/*
Copyright 2020 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package virtualnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"arhat.dev/pkg/log"
	"arhat.dev/pkg/queue"
	coordinationv1 "k8s.io/api/coordination/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientcodv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"

	"arhat.dev/aranya-proto/gopb"
	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
)

func NewVirtualNodeManager(
	ctx context.Context,
	config *conf.VirtualnodeConfig,
	getLease func(name string) *coordinationv1.Lease,
	leaseClient clientcodv1.LeaseInterface,
) *Manager {
	nodeUpdateInterval := config.Node.Timers.MirrorSyncInterval
	if nodeUpdateInterval < time.Second {
		nodeUpdateInterval = constant.DefaultMirrorNodeSyncInterval
	}

	leaseUpdateInterval := config.Node.Lease.UpdateInterval
	if leaseUpdateInterval < time.Second {
		leaseUpdateInterval = constant.DefaultNodeLeaseUpdateInterval
	}

	return &Manager{
		ctx: ctx,
		log: log.Log.WithName("virtualnode"),

		nodeUpdateInterval:  nodeUpdateInterval,
		leaseUpdateInterval: leaseUpdateInterval,
		nodes:               make(map[string]*VirtualNode),

		nodeForceSyncQ: queue.NewTimeoutQueue(),
		podForceSyncQ:  queue.NewTimeoutQueue(),
		mirrorUpdateQ:  queue.NewTimeoutQueue(),
		leaseUpdateQ:   queue.NewTimeoutQueue(),

		getLeaseFromCache: getLease,
		leaseClient:       leaseClient,
		leaseEnabled:      config.Node.Lease.Enabled,

		mu: new(sync.RWMutex),
	}
}

type Manager struct {
	ctx context.Context
	log log.Interface

	nodeUpdateInterval  time.Duration
	leaseUpdateInterval time.Duration
	nodes               map[string]*VirtualNode

	nodeForceSyncQ *queue.TimeoutQueue
	podForceSyncQ  *queue.TimeoutQueue
	mirrorUpdateQ  *queue.TimeoutQueue
	leaseUpdateQ   *queue.TimeoutQueue

	getLeaseFromCache func(name string) *coordinationv1.Lease
	leaseClient       clientcodv1.LeaseInterface
	leaseEnabled      bool

	mu *sync.RWMutex
}

func (m *Manager) doPerQueue(do func(q *queue.TimeoutQueue)) {
	for _, q := range []*queue.TimeoutQueue{m.nodeForceSyncQ, m.podForceSyncQ, m.mirrorUpdateQ, m.leaseUpdateQ} {
		do(q)
	}
}

func (m *Manager) Start() error {
	m.doPerQueue(func(q *queue.TimeoutQueue) {
		q.Start(m.ctx.Done())
	})

	go m.consumeForceNodeSync()
	go m.consumeForcePodSync()

	if m.leaseEnabled {
		m.consumeLeaseUpdate()
	} else {
		m.consumeMirrorNodeSync()
	}

	return m.ctx.Err()
}

func (m *Manager) OnVirtualNodeDisconnected(vn *VirtualNode) {
	m.doPerQueue(func(q *queue.TimeoutQueue) {
		q.Forbid(vn.name)
		q.Remove(vn.name)
	})

	err := vn.opt.ScheduleNodeSync()
	if err != nil {
		m.log.E("failed to sync node status for offline", log.Error(err))
	}
}

func (m *Manager) OnVirtualNodeConnected(vn *VirtualNode) (allow bool) {
	m.doPerQueue(func(q *queue.TimeoutQueue) {
		q.Allow(vn.name)
	})

	defer func() {
		if !allow {
			m.OnVirtualNodeDisconnected(vn)
		}
	}()

	vn.log.D("syncing device info for the first time")
	if err := vn.SyncDeviceNodeStatus(gopb.GET_NODE_INFO_ALL); err != nil {
		vn.log.I("failed to sync device node info, reject", log.Error(err))
		vn.opt.ConnectivityManager.Reject(gopb.REJECTION_NODE_STATUS_SYNC_ERROR, "failed to pass initial node sync")
		return false
	}

	var supportPod bool
	vn.log.D("syncing device pods for the first time")
	if err := vn.podManager.SyncDevicePods(); err != nil {
		if e, ok := err.(*gopb.Error); ok && e.Kind == gopb.ERR_NOT_SUPPORTED {
			// ignore unsupported error
			supportPod = false
			vn.log.I("pod not supported in remote device")
		} else {
			vn.log.I("failed to sync device pods", log.Error(err))
			vn.opt.ConnectivityManager.Reject(gopb.REJECTION_POD_STATUS_SYNC_ERROR, "failed to pass initial pod sync")
			return false
		}
	} else {
		supportPod = true
	}

	// start force node status check if configured
	if interval := vn.opt.NodeOptions.ForceSyncInterval; interval > 0 {
		vn.log.D("scheduling force node sync", log.Duration("interval", interval))
		if err := m.nodeForceSyncQ.OfferWithDelay(vn.name, interval, interval); err != nil {
			vn.log.E("failed to schedule force node sync", log.Error(err))
			vn.opt.ConnectivityManager.Reject(gopb.REJECTION_INTERNAL_SERVER_ERROR, "force pod sync schedule failure")
			return false
		}

		vn.log.V("scheduled force node sync")
	} else {
		vn.log.I("no node force sync scheduled")
	}

	// start force pod check if configured and supported
	if interval := vn.opt.PodOptions.Config.Timers.ForceSyncInterval; interval > 0 && supportPod {
		vn.log.D("scheduling force pod sync", log.Duration("interval", interval))
		if err := m.podForceSyncQ.OfferWithDelay(vn.name, interval, interval); err != nil {
			vn.log.E("failed to schedule force pod sync", log.Error(err))
			vn.opt.ConnectivityManager.Reject(gopb.REJECTION_INTERNAL_SERVER_ERROR, "force pod sync schedule failure")
			return false
		}

		vn.log.V("scheduled force pod sync")
	} else {
		vn.log.I("no pod force sync scheduled")
	}

	if m.leaseEnabled {
		m.mirrorUpdateQ.Forbid(vn.name)
		m.mirrorUpdateQ.Remove(vn.name)

		// using node lease, update lease object periodically
		vn.log.D("scheduling lease update", log.Duration("interval", m.leaseUpdateInterval))
		err := m.leaseUpdateQ.OfferWithDelay(vn.name, m.leaseUpdateInterval, wait.Jitter(m.leaseUpdateInterval, .2))
		if err != nil {
			vn.log.E("failed to schedule node lease update", log.Error(err))
			vn.opt.ConnectivityManager.Reject(gopb.REJECTION_INTERNAL_SERVER_ERROR, "lease update failure")
			return false
		}

		vn.log.V("scheduled node lease update")
	} else {
		m.leaseUpdateQ.Forbid(vn.name)
		m.leaseUpdateQ.Remove(vn.name)

		// not using node lease, we need to update node status periodically
		vn.log.D("scheduling mirror node update", log.Duration("interval", m.nodeUpdateInterval))
		err := m.mirrorUpdateQ.OfferWithDelay(vn.name, m.nodeUpdateInterval, wait.Jitter(m.nodeUpdateInterval, .2))
		if err != nil {
			vn.log.E("failed to schedule mirror node update", log.Error(err))
			vn.opt.ConnectivityManager.Reject(gopb.REJECTION_INTERNAL_SERVER_ERROR, "mirror node sync failure")
			return false
		}

		vn.log.V("scheduled mirror node update")
	}

	if vn.storageManager != nil {
		// send storage credentials when storage enabled
		vn.log.D("sending storage credentials to device")
		msgCh, _, err := vn.opt.ConnectivityManager.PostCmd(0, gopb.NewStorageCredentialUpdateCmd(vn.opt.SSHPrivateKey))
		if err != nil {
			vn.log.E("failed to send storage credentials", log.Error(err))
			vn.opt.ConnectivityManager.Reject(gopb.REJECTION_CREDENTIAL_FAILURE, "failed to send node credentials")
			return false
		}

		gopb.HandleMessages(msgCh, func(msg *gopb.Msg) (exit bool) {
			if msgErr := msg.GetError(); msgErr != nil {
				err = msgErr
				return true
			}

			credStatus := msg.GetCredentialStatus()
			if credStatus == nil {
				vn.log.I("unexpected non credential status message", log.Any("msg", msg))
				return true
			}

			sum := sha256.New()
			_, _ = sum.Write(vn.opt.SSHPrivateKey)
			if credStatus.SshPrivateKeySha256Hex != hex.EncodeToString(sum.Sum(nil)) {
				err = fmt.Errorf("ssh identity corrupted")
				return true
			}

			return false
		}, nil, gopb.HandleUnknownMessage(vn.log))

		if err != nil {
			if e, ok := err.(*gopb.Error); ok && e.Kind == gopb.ERR_NOT_SUPPORTED {
				vn.log.I("node credentials not supported")
			} else {
				vn.log.I("failed to set node credentials", log.Error(err))
				vn.opt.ConnectivityManager.Reject(gopb.REJECTION_CREDENTIAL_FAILURE, "failed to set node credentials")
				return false
			}
		}
	}

	return true
}

// Add a virtual node to the active virtual node list
func (m *Manager) Add(vn *VirtualNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.nodes[vn.name]; ok {
		return errors.New("node with same name already exists")
	}

	m.nodes[vn.name] = vn

	m.doPerQueue(func(q *queue.TimeoutQueue) {
		q.Forbid(vn.name)
		q.Remove(vn.name)
	})

	return nil
}

// Get an active virtual node
func (m *Manager) Get(name string) (*VirtualNode, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, ok := m.nodes[name]
	if ok {
		return node, true
	}
	return nil, false
}

func (m *Manager) All() map[string]*VirtualNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clone := make(map[string]*VirtualNode)
	for k, v := range m.nodes {
		clone[k] = v
	}

	return clone
}

// Delete close and delete an active virtual node
func (m *Manager) Delete(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if srv, ok := m.nodes[name]; ok {
		srv.ForceClose()
		delete(m.nodes, name)
	} else {
		return
	}
}

func (m *Manager) getVirtualNodeFromKey(key interface{}) (*VirtualNode, bool) {
	nodeName, ok := key.(string)
	if !ok {
		return nil, false
	}

	return m.Get(nodeName)
}

func (m *Manager) consumeForceNodeSync() {
	syncLogger := m.log.WithName("force-node-sync")
	syncLogger.I("provisioned")
	defer syncLogger.I("exited")

	ch := m.nodeForceSyncQ.TakeCh()
	for t := range ch {
		vn, ok := m.getVirtualNodeFromKey(t.Key)
		if !ok {
			syncLogger.I("virtualnode not found", log.Any("key", t.Key))
			continue
		}

		logger := syncLogger.WithFields(log.String("node", vn.name))

		select {
		case <-vn.opt.ConnectivityManager.Connected():
			go func() {
				logger.V("syncing")
				if err := vn.SyncDeviceNodeStatus(gopb.GET_NODE_INFO_DYN); err != nil {
					logger.I("failed", log.Error(err))
					vn.opt.ConnectivityManager.Reject(gopb.REJECTION_NODE_STATUS_SYNC_ERROR, err.Error())

					logger.I("rejected")
					return
				}

				timeout := wait.Jitter(t.Data.(time.Duration), .2)
				logger.V("scheduling next", log.Duration("after", timeout))
				err := m.nodeForceSyncQ.OfferWithDelay(t.Key, t.Data, timeout)
				if err == nil {
					logger.V("scheduled next")
				} else {
					logger.E("failed to schedule next", log.NamedError("reason", err))
				}
			}()
		default:
			logger.I("disconnected, do nothing")
		}
	}
}

func (m *Manager) consumeForcePodSync() {
	syncLogger := m.log.WithName("force-pod-sync")
	syncLogger.I("provisioned")
	defer syncLogger.I("exited")

	ch := m.podForceSyncQ.TakeCh()
	for t := range ch {
		vn, ok := m.getVirtualNodeFromKey(t.Key)
		if !ok {
			syncLogger.I("virtualnode not found", log.Any("key", t.Key))
			continue
		}

		logger := syncLogger.WithFields(log.String("node", vn.name))

		select {
		case <-vn.opt.ConnectivityManager.Connected():
			go func() {
				logger.V("syncing")
				if err := vn.podManager.SyncDevicePods(); err != nil {
					logger.I("failed", log.Error(err))
					vn.opt.ConnectivityManager.Reject(gopb.REJECTION_POD_STATUS_SYNC_ERROR, err.Error())

					logger.I("rejected")
					return
				}

				timeout := wait.Jitter(t.Data.(time.Duration), .2)
				logger.V("scheduling next", log.Duration("after", timeout))
				err := m.podForceSyncQ.OfferWithDelay(t.Key, t.Data, timeout)
				if err == nil {
					logger.V("scheduled next")
				} else {
					logger.E("failed to schedule next", log.NamedError("reason", err))
				}
			}()
		default:
			logger.I("disconnected, do nothing")
		}
	}
}

func (m *Manager) consumeMirrorNodeSync() {
	syncLogger := m.log.WithName("mirror-node-sync")
	syncLogger.I("provisioned")
	defer syncLogger.I("exited")

	ch := m.mirrorUpdateQ.TakeCh()
	for t := range ch {
		vn, ok := m.getVirtualNodeFromKey(t.Key)
		if !ok {
			syncLogger.I("virtualnode not found", log.Any("key", t.Key))
			continue
		}

		logger := syncLogger.WithFields(log.String("node", vn.name))

		select {
		case <-vn.opt.ConnectivityManager.Connected():
			go func() {
				logger.V("syncing")
				err := vn.opt.ScheduleNodeSync()
				if err != nil {
					logger.I("failed", log.Error(err))
					vn.opt.ConnectivityManager.Reject(gopb.REJECTION_INTERNAL_SERVER_ERROR, err.Error())

					logger.I("rejected")
					return
				}

				timeout := wait.Jitter(m.nodeUpdateInterval, .2)
				logger.V("scheduling next", log.Duration("after", timeout))
				err = m.mirrorUpdateQ.OfferWithDelay(vn.name, m.nodeUpdateInterval, timeout)
				if err == nil {
					logger.V("scheduled next")
				} else {
					logger.E("failed to schedule next", log.NamedError("reason", err))
				}
			}()
		default:
			logger.I("disconnected, do nothing")
		}
	}
}

func (m *Manager) consumeLeaseUpdate() {
	syncLogger := m.log.WithName("node-lease-update")
	syncLogger.I("provisioned")
	defer syncLogger.I("exited")

	ch := m.leaseUpdateQ.TakeCh()
	for t := range ch {
		vn, ok := m.getVirtualNodeFromKey(t.Key)
		if !ok {
			syncLogger.I("virtualnode not found", log.Any("key", t.Key))
			continue
		}

		logger := syncLogger.WithFields(log.String("node", vn.name))

		select {
		case <-vn.opt.ConnectivityManager.Connected():
			go func() {
				logger.V("syncing")

				err := m.updateLeaseWithRetry(vn.name)
				if err != nil {
					logger.I("failed", log.Error(err))
					vn.opt.ConnectivityManager.Reject(gopb.REJECTION_INTERNAL_SERVER_ERROR, "lease update failure")

					logger.I("rejected")
					return
				}

				timeout := wait.Jitter(m.leaseUpdateInterval, .2)
				logger.V("scheduling next", log.Duration("after", timeout))
				err = m.leaseUpdateQ.OfferWithDelay(vn.name, m.leaseUpdateInterval, timeout)
				if err == nil {
					logger.V("scheduled next")
				} else {
					logger.E("failed to schedule next", log.NamedError("reason", err))
				}
			}()
		default:
			logger.I("disconnected, do nothing")
		}
	}
}

func (m *Manager) updateLeaseWithRetry(name string) error {
	var (
		err           error
		lease         *coordinationv1.Lease
		retrieveLease bool
	)
	for i := 0; i < 5; i++ {
		func() {
			defer func() {
				if err != nil {
					time.Sleep(m.leaseUpdateInterval / 7)
				}
			}()

			lease = m.getLeaseFromCache(name)
			if lease == nil || retrieveLease {
				lease, err = m.leaseClient.Get(m.ctx, name, metav1.GetOptions{})
				if err != nil {
					return
				}
			}

			lease = lease.DeepCopy()

			lease.Spec.RenewTime = &metav1.MicroTime{Time: time.Now()}
			_, err = m.leaseClient.Update(m.ctx, lease, metav1.UpdateOptions{})
			if err != nil {
				if kubeerrors.IsConflict(err) {
					retrieveLease = true
				} else {
					retrieveLease = false
				}
			}
		}()

		if err == nil {
			return nil
		}
	}

	return err
}
