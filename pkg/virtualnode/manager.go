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
	"errors"
	"sync"
	"time"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/pkg/log"
	"arhat.dev/pkg/queue"
	coordinationv1 "k8s.io/api/coordination/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientcodv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"

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

// Manager of virtualnodes
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
	for _, q := range []*queue.TimeoutQueue{
		m.nodeForceSyncQ,
		m.podForceSyncQ,
		m.mirrorUpdateQ,
		m.leaseUpdateQ,
	} {
		do(q)
	}
}

func (m *Manager) Start(wg *sync.WaitGroup, stop <-chan struct{}) {
	m.doPerQueue(func(q *queue.TimeoutQueue) {
		q.Start(stop)
	})

	wg.Add(2)
	go func() {
		defer wg.Done()

		m.consumeForceNodeSync()
	}()

	go func() {
		defer wg.Done()

		m.consumeForcePodSync()
	}()

	wg.Add(1)
	if m.leaseEnabled {
		go func() {
			defer wg.Done()

			m.consumeLeaseUpdate()
		}()
	} else {
		go func() {
			defer wg.Done()

			m.consumeMirrorNodeSync()
		}()
	}
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

	vn.log.D("syncing node info for the first time")
	if err := vn.SyncDeviceNodeStatus(aranyagopb.NODE_INFO_ALL); err != nil {
		vn.log.I("failed to sync node info, reject", log.Error(err))
		vn.opt.ConnectivityManager.Reject(
			aranyagopb.REJECTION_INITIAL_CHECK_FAILURE, "failed to pass initial node sync")
		return false
	}

	var supportPod bool
	vn.log.D("syncing pods for the first time")
	if err := vn.podManager.SyncDevicePods(); err != nil {
		if e, ok := err.(*aranyagopb.ErrorMsg); ok && e.Kind == aranyagopb.ERR_NOT_SUPPORTED {
			// ignore unsupported error
			supportPod = false
			vn.log.I("pod not supported in remote node")
		} else {
			vn.log.I("failed to sync pods", log.Error(err))
			vn.opt.ConnectivityManager.Reject(
				aranyagopb.REJECTION_INITIAL_CHECK_FAILURE, "failed to pass initial pod sync")
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
			vn.opt.ConnectivityManager.Reject(
				aranyagopb.REJECTION_INTERNAL_SERVER_ERROR, "force pod sync schedule failure")
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
			vn.opt.ConnectivityManager.Reject(
				aranyagopb.REJECTION_INTERNAL_SERVER_ERROR, "force pod sync schedule failure")
			return false
		}

		vn.log.V("scheduled force pod sync")
	} else {
		vn.log.I("no pod force sync scheduled")
	}

	if m.leaseEnabled {
		// using node lease, disable node status update
		m.mirrorUpdateQ.Forbid(vn.name)
		m.mirrorUpdateQ.Remove(vn.name)

		// update lease object periodically
		vn.log.D("scheduling lease update", log.Duration("interval", m.leaseUpdateInterval))
		err := m.leaseUpdateQ.OfferWithDelay(vn.name, m.leaseUpdateInterval, wait.Jitter(m.leaseUpdateInterval, .2))
		if err != nil {
			vn.log.E("failed to schedule node lease update", log.Error(err))
			vn.opt.ConnectivityManager.Reject(aranyagopb.REJECTION_INTERNAL_SERVER_ERROR, "lease update failure")
			return false
		}

		vn.log.V("scheduled node lease update")
	} else {
		// not using node lease, disable lease object update
		m.leaseUpdateQ.Forbid(vn.name)
		m.leaseUpdateQ.Remove(vn.name)

		// need to update node status periodically
		vn.log.D("scheduling mirror node update", log.Duration("interval", m.nodeUpdateInterval))
		err := m.mirrorUpdateQ.OfferWithDelay(vn.name, m.nodeUpdateInterval, wait.Jitter(m.nodeUpdateInterval, .2))
		if err != nil {
			vn.log.E("failed to schedule mirror node update", log.Error(err))
			vn.opt.ConnectivityManager.Reject(aranyagopb.REJECTION_INTERNAL_SERVER_ERROR, "mirror node sync failure")
			return false
		}

		vn.log.V("scheduled mirror node update")
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
				if err := vn.SyncDeviceNodeStatus(aranyagopb.NODE_INFO_DYN); err != nil {
					logger.I("failed", log.Error(err))
					vn.opt.ConnectivityManager.Reject(aranyagopb.REJECTION_INTERNAL_SERVER_ERROR, err.Error())

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
					vn.opt.ConnectivityManager.Reject(aranyagopb.REJECTION_INTERNAL_SERVER_ERROR, err.Error())

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
					vn.opt.ConnectivityManager.Reject(aranyagopb.REJECTION_INTERNAL_SERVER_ERROR, err.Error())

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
					vn.opt.ConnectivityManager.Reject(
						aranyagopb.REJECTION_INTERNAL_SERVER_ERROR, "lease update failure")

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
