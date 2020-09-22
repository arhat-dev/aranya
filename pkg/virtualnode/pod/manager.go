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

package pod

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/pkg/backoff"
	"arhat.dev/pkg/log"
	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeclient "k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/csi"

	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/util/cache"
	"arhat.dev/aranya/pkg/util/manager"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
	"arhat.dev/aranya/pkg/virtualnode/network"
)

var (
	deleteAtOnce = metav1.NewDeleteOptions(0)
)

type Options struct {
	Config        *conf.VirtualnodePodConfig
	GetNode       func() *corev1.Node
	GetPod        func(name string) *corev1.Pod
	GetConfigMap  func(name string) *corev1.ConfigMap
	GetSecret     func(name string) *corev1.Secret
	ListServices  func() []*corev1.Service
	EventRecorder record.EventRecorder
}

// NewManager creates a new pod manager for virtual node
func NewManager(
	parentCtx context.Context,
	name, hostIP string,
	client kubeclient.Interface,
	networkManager *network.Manager,
	connectivityManager connectivity.Manager,
	options *Options,
) *Manager {
	mgr := &Manager{
		BaseManager: manager.NewBaseManager(parentCtx, fmt.Sprintf("pod.%s", name), connectivityManager),

		nodeName: name,
		hostIP:   hostIP,

		kubeClient: client,
		nodeClient: client.CoreV1().Nodes(),
		podClient:  client.CoreV1().Pods(constant.WatchNS()),
		options:    options,

		netMgr:   networkManager,
		podCache: cache.NewPodCache(),

		csiPlugin:        csi.ProbeVolumePlugins()[0],
		abbotPodUIDStore: new(atomic.Value),
	}
	mgr.abbotPodUIDStore.Store(types.UID(""))

	mgr.resPodRec = reconcile.NewCore(mgr.Context(), reconcile.Options{
		Logger:          mgr.Log.WithFields(log.String("type", "resPod")),
		BackoffStrategy: backoff.NewStrategy(time.Second, 15*time.Minute, 1.5, 5),
		Workers:         1,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded:    nil,
			OnUpdated:  mgr.onResourcePodUpdated,
			OnDeleting: mgr.onResourcePodDeleting,
			OnDeleted:  mgr.onResourcePodDeleted,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	}.ResolveNil())

	mgr.devPodRec = reconcile.NewCore(mgr.Context(), reconcile.Options{
		Logger:          mgr.Log.WithFields(log.String("type", "devicePod")),
		BackoffStrategy: backoff.NewStrategy(time.Second, time.Minute, 2, 1),
		Workers:         1,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded:    mgr.onRemotePodCreationRequested,
			OnUpdated:  mgr.onRemotePodUpdateRequested,
			OnDeleting: mgr.onRemotePodDeletionRequested,
			OnDeleted:  mgr.onRemotePodDeleted,
		},
		OnBackoffStart: func(key interface{}, err error) {
			podUID := key.(types.UID)

			pod, ok := mgr.podCache.GetByID(podUID)
			if ok {
				mgr.options.EventRecorder.Eventf(pod, corev1.EventTypeNormal,
					"CrashLoopBackOff", "Backoff due to error: %w", err)
			}
		},
	}.ResolveNil())

	return mgr
}

// Manager the pod manager, controls pod workload assignment and pod command execution
type Manager struct {
	*manager.BaseManager

	nodeName string
	hostIP   string

	kubeClient kubeclient.Interface
	nodeClient typedcorev1.NodeInterface
	podClient  typedcorev1.PodInterface
	options    *Options

	devPodRec *reconcile.Core
	resPodRec *reconcile.Core

	netMgr   *network.Manager
	podCache *cache.PodCache

	csiPlugin        volume.VolumePlugin
	abbotPodUIDStore *atomic.Value

	// storage related funcs
	PrepareStorage               func(pod *corev1.Pod, volumeInUse []corev1.UniqueVolumeName) error
	CleanupStorage               func(pod *corev1.Pod) error
	GetPersistentVolumeMountPath func(podUID types.UID, pvName string) string
}

func (m *Manager) StorageEnabled() bool {
	return m.PrepareStorage != nil && m.CleanupStorage != nil && m.GetPersistentVolumeMountPath != nil
}

type JobScheduleFunc func(action queue.JobAction, oldPod, newPod *corev1.Pod) error

func (m *Manager) SchedulePodJob(action queue.JobAction, oldPod, newPod *corev1.Pod) error {
	m.resPodRec.Update(newPod.UID, oldPod, newPod)

	switch action {
	case queue.ActionAdd:
	case queue.ActionUpdate:
		// TODO: do we really need to freeze old pod cache for update?
		//m.resPodRec.Freeze(newPod.UID, true)
	case queue.ActionDelete:
	case queue.ActionCleanup:
	}

	return m.resPodRec.Schedule(queue.Job{Action: action, Key: newPod.UID}, 0)
}

func (m *Manager) hasPodCIDR() bool {
	return m.netMgr.GetPodCIDR(false) != "" || m.netMgr.GetPodCIDR(true) != ""
}

func (m *Manager) getPod(name string) (*corev1.Pod, error) {
	pod := m.options.GetPod(name)
	if pod == nil {
		return nil, fmt.Errorf("pod %q not found", name)
	}

	if pod.Name == m.nodeName {
		// do not record virtual pod
		m.podCache.Delete(pod.UID)
	} else {
		m.podCache.Update(pod)
	}
	return pod, nil
}

// UpdatePodStatus update podStatus and store the latest cache
func (m *Manager) UpdatePodStatus(pod *corev1.Pod) (*corev1.Pod, error) {
	const tryCount = 5
	for i := 0; i < tryCount; i++ {
		_, err := m.podClient.UpdateStatus(m.Context(), pod, metav1.UpdateOptions{})
		if err == nil {
			break
		}

		// failed to update
		switch {
		case kubeerrors.IsNotFound(err):
			return pod, err
		default:
			time.Sleep(time.Second)

			newPod, err := m.getPod(pod.Name)
			if err == nil {
				newPod.Status = pod.Status
				pod = newPod
			}
		}
	}

	newPod, err := m.getPod(pod.Name)
	if err != nil {
		return pod, err
	}

	return newPod, nil
}

// Start pod manager
func (m *Manager) Start() error {
	return m.OnStart(func() error {
		err := m.resPodRec.Start()
		if err != nil {
			return fmt.Errorf("failed to start resource pod reconciler: %w", err)
		}

		err = m.devPodRec.Start()
		if err != nil {
			return fmt.Errorf("failed to start device pod reconciler: %w", err)
		}

		m.Log.D("reconciling resource pods")
		go m.resPodRec.ReconcileUntil(m.Context().Done())

		for !m.Closing() {
			select {
			case <-m.ConnectivityManager.Connected():
				// we are good to go, ensure virtual pod running
				// (best effort)
				m.Log.D("ensuring virtual pod running phase")
				for err = m.updateVirtualPodToRunningPhase(); err != nil; err = m.updateVirtualPodToRunningPhase() {
					m.Log.I("failed to update virtual pod to running phase", log.Error(err))
					time.Sleep(time.Second)
				}
			case <-m.Context().Done():
				return nil
			}

			m.Log.D("reconciling device pods")

			// reconcile until lost device connection
			m.devPodRec.ReconcileUntil(m.ConnectivityManager.Disconnected())
		}

		return nil
	})
}

// Close pod manager
func (m *Manager) Close() {
	m.OnClose(nil)
}

// nolint:unused
func (m *Manager) updateDeviceNetwork() error {
	cmd := aranyagopb.NewNetworkUpdatePodNetCmd(m.netMgr.GetPodCIDR(false), m.netMgr.GetPodCIDR(true))
	msgCh, _, err := m.ConnectivityManager.PostCmd(0, aranyagopb.CMD_NET_UPDATE_POD_NET, cmd)
	if err != nil {
		return fmt.Errorf("failed to post network update cmd: %w", err)
	}

	var retErr error
	connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
		// if there is no abbot container running in the edge device (ERR_NOT_SUPPORTED)
		// we can assume the edge device is not using cluster network, no more action required
		if msgErr := msg.GetError(); msgErr != nil && msgErr.Kind != aranyagopb.ERR_NOT_SUPPORTED {
			retErr = multierr.Append(retErr, msgErr)
		}

		return false
	}, nil, connectivity.HandleUnknownMessage(m.Log))
	return nil
}
