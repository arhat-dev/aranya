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
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/pkg/log"
	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/server/healthz"
	kubeclient "k8s.io/client-go/kubernetes"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/util/middleware"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
	"arhat.dev/aranya/pkg/virtualnode/metrics"
	"arhat.dev/aranya/pkg/virtualnode/network"
	"arhat.dev/aranya/pkg/virtualnode/peripheral"
	"arhat.dev/aranya/pkg/virtualnode/pod"
	"arhat.dev/aranya/pkg/virtualnode/storage"
)

// CreationOptions the options used to create a new virtual node
type CreationOptions struct {
	// node info
	NodeName string
	HostIP   string
	Hostname string

	SSHPrivateKey         []byte
	ScheduleNodeSync      func() error
	KubeClient            kubeclient.Interface
	KubeletServerListener net.Listener
	EventBroadcaster      record.EventBroadcaster
	VirtualnodeManager    *Manager

	ConnectivityManager connectivity.Manager

	ConnectivityOptions *connectivity.Options
	NodeOptions         *Options
	PodOptions          *pod.Options
	PeripheralOptions   *peripheral.Options
	MetricsOptions      *metrics.Options
	NetworkOptions      *network.Options
	StorageOptions      *storage.Options
}

type Options struct {
	ForceSyncInterval      time.Duration
	MirrorNodeSyncInterval time.Duration
}

type muxWrapper struct {
	m *mux.Router
}

func (m *muxWrapper) Handle(pattern string, handler http.Handler) {
	m.m.Handle(pattern, handler)
}

// CreateVirtualNode creates a new virtual node with options
func CreateVirtualNode(ctx context.Context, cancel context.CancelFunc, opt *CreationOptions) (*VirtualNode, error) {
	logger := log.Log.WithName(fmt.Sprintf("node.%s", opt.NodeName))

	networkManager := network.NewManager(
		ctx,
		opt.NodeName,
		opt.ConnectivityManager,
		opt.NetworkOptions,
	)

	peripheralManager := peripheral.NewManager(ctx, opt.NodeName, opt.ConnectivityManager, opt.PeripheralOptions)

	opt.PodOptions.OperateDevice = peripheralManager.Operate
	opt.PodOptions.CollectDeviceMetrics = peripheralManager.CollectMetrics
	podManager := pod.NewManager(
		ctx,
		opt.NodeName,
		opt.HostIP,
		opt.KubeClient,
		networkManager,
		opt.ConnectivityManager,
		opt.PodOptions,
	)

	m := &mux.Router{NotFoundHandler: middleware.NotFoundHandler(logger)}
	m.Use(middleware.DeviceOnlineCheck(opt.ConnectivityManager.Disconnected, logger))
	// use strict slash for redirection, or will not handle certain requests
	m.StrictSlash(true)

	//
	// routes for pod
	//
	// health check
	healthz.InstallHandler(&muxWrapper{m: m},
		healthz.PingHealthz,
		healthz.LogHealthz,
		healthz.NamedCheck("syncloop", func(r *http.Request) error {
			duration := opt.NodeOptions.MirrorNodeSyncInterval * 2
			minDuration := time.Minute * 5
			if duration < minDuration {
				duration = minDuration
			}
			_ = duration
			// TODO: finish sync loop health check
			//enterLoopTime := s.host.LatestLoopEntryTime()
			//if !enterLoopTime.IsZero() && time.Now().After(enterLoopTime.Add(duration)) {
			//	return fmt.Errorf("sync Loop took longer than expected")
			//}
			return nil
		}),
	)
	// host logs
	m.HandleFunc("/logs/.*",
		podManager.HandleHostLog).Methods(http.MethodGet)
	// containerLogs (kubectl logs)
	m.HandleFunc("/containerLogs/{namespace}/{name}/{container}",
		podManager.HandlePodLog).Methods(http.MethodGet)
	// exec (kubectl exec/cp)
	m.HandleFunc("/exec/{namespace}/{name}/{container}",
		podManager.HandlePodExec).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/exec/{namespace}/{name}/{uid}/{container}",
		podManager.HandlePodExec).Methods(http.MethodPost, http.MethodGet)
	// attach (kubectl attach)
	m.HandleFunc("/attach/{namespace}/{name}/{container}",
		podManager.HandlePodAttach).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/attach/{namespace}/{name}/{uid}/{container}",
		podManager.HandlePodAttach).Methods(http.MethodPost, http.MethodGet)
	// run
	m.HandleFunc("/run/{namespace}/{name}/{container}",
		podManager.HandlePodExec).Methods(http.MethodPost)
	m.HandleFunc("/run/{namespace}/{name}/{uid}/{container}",
		podManager.HandlePodExec).Methods(http.MethodPost)
	// portForward (kubectl proxy)
	m.HandleFunc("/portForward/{namespace}/{name}",
		podManager.HandlePodPortForward).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/portForward/{namespace}/{name}/{uid}",
		podManager.HandlePodPortForward).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/pods", podManager.HandleGetPods).Methods(http.MethodGet)
	m.HandleFunc("/runningpods", podManager.HandleGetRunningPods).Methods(http.MethodGet)

	vn := &VirtualNode{
		ctx:    ctx,
		cancel: cancel,
		opt:    opt,

		log:  logger,
		name: opt.NodeName,

		nodeClient: opt.KubeClient.CoreV1().Nodes(),

		maxPods:           int64(opt.PodOptions.Config.Allocatable) + 1,
		kubeletSrv:        &http.Server{BaseContext: func(net.Listener) context.Context { return ctx }, Handler: m},
		networkManager:    networkManager,
		podManager:        podManager,
		storageManager:    nil, // initialized later
		metricsManager:    nil, // initialized later
		peripheralManager: peripheralManager,
		nodeStatusCache:   newNodeCache(),

		SchedulePodJob: podManager.SchedulePodJob,

		eventLoggingWatch: opt.EventBroadcaster.StartLogging(func(format string, args ...interface{}) {
			logger.I(fmt.Sprintf(format, args...), log.String("source", "event"))
		}),
		eventRecordingWatch: opt.EventBroadcaster.StartRecordingToSink(&clientcorev1.EventSinkImpl{
			Interface: opt.KubeClient.CoreV1().Events(constant.WatchNS()),
		}),
	}

	opt.MetricsOptions.GetOS = vn.OS
	vn.metricsManager = metrics.NewManager(
		ctx,
		opt.NodeName,
		opt.ConnectivityManager,
		opt.MetricsOptions,
	)
	//
	// routes for metrics and stats
	//
	// metrics
	m.HandleFunc("/metrics", vn.metricsManager.HandleNodeMetrics).Methods(http.MethodGet)
	m.HandleFunc("/metrics/cadvisor", vn.metricsManager.HandleContainerMetrics).Methods(http.MethodGet)
	m.HandleFunc("/metrics/probes", vn.metricsManager.HandleProbesMetrics).Methods(http.MethodGet)
	m.HandleFunc("/metrics/resource", vn.metricsManager.HandleResourceMetrics).Methods(http.MethodGet)
	// stats
	m.HandleFunc("/stats", vn.metricsManager.HandleStats).Methods(http.MethodGet, http.MethodPost)
	m.HandleFunc("/stats/summary", vn.metricsManager.HandleStatsSummary).Methods(http.MethodGet, http.MethodPost)
	m.HandleFunc("/stats/container",
		vn.metricsManager.HandleStatsSystemContainer).Methods(http.MethodGet, http.MethodPost)
	m.HandleFunc("/stats/{name}/{container}",
		vn.metricsManager.HandleStatsContainer).Methods(http.MethodGet, http.MethodPost)
	m.HandleFunc("/stats/{namespace}/{name}/{uid}/{container}",
		vn.metricsManager.HandleStatsContainer).Methods(http.MethodGet, http.MethodPost)
	// stats spec
	m.HandleFunc("/spec", vn.metricsManager.HandleStatsSpec).Methods(http.MethodGet)
	// pprof
	m.HandleFunc("/debug/pprof", vn.metricsManager.HandlePprof).Methods(http.MethodGet)

	// TODO: evaluate /cri
	//m.HandleFunc("/cri", nil)

	if opt.StorageOptions != nil {
		vn.storageManager = storage.NewManager(
			ctx,
			opt.KubeClient,
			opt.NodeName,
			opt.Hostname,
			opt.HostIP,
			vn.podManager.KubeRuntimeForVolumeManager(),
			vn.podManager.KubePodManagerForVolumeManager(),
			vn.podManager.KubePodStatusProviderForVolumeManager(),
			opt.StorageOptions,
		)

		vn.podManager.PrepareStorage = vn.storageManager.Prepare
		vn.podManager.CleanupStorage = vn.storageManager.Cleanup
		vn.podManager.GetPersistentVolumeMountPath = vn.storageManager.GetPersistentVolumeMountPath
	}

	return vn, nil
}

// VirtualNode the virtual node implementation
type VirtualNode struct {
	once   sync.Once
	ctx    context.Context
	cancel context.CancelFunc
	opt    *CreationOptions

	log  log.Interface
	name string

	nodeClient clientcorev1.NodeInterface

	maxPods           int64
	kubeletSrv        *http.Server
	networkManager    *network.Manager
	podManager        *pod.Manager
	storageManager    *storage.Manager
	metricsManager    *metrics.Manager
	peripheralManager *peripheral.Manager
	nodeStatusCache   *NodeCache

	SchedulePodJob pod.JobScheduleFunc

	eventLoggingWatch   watch.Interface
	eventRecordingWatch watch.Interface
}

// Start the virtual node with all required managers
func (vn *VirtualNode) Start() error {
	vn.once.Do(func() {
		// start the kubelet http server
		go func() {
			vn.log.I("starting kubelet http server")
			defer func() {
				vn.log.I("kubelet http server exited")

				// once kubelet server exited, delete this virtual node
				vn.opt.VirtualnodeManager.Delete(vn.name)
			}()

			if err := vn.kubeletSrv.Serve(vn.opt.KubeletServerListener); err != nil && err != http.ErrServerClosed {
				vn.log.I("failed to start kubelet http server", log.Error(err))
				return
			}
		}()

		go func() {
			vn.log.I("starting connectivity manager")
			defer func() {
				vn.log.I("connectivity manager exited")

				// once connectivity manager exited, delete this virtual node
				vn.opt.VirtualnodeManager.Delete(vn.name)
			}()

			if err := vn.opt.ConnectivityManager.Start(); err != nil {
				vn.log.I("failed to start connectivity manager", log.Error(err))
				return
			}
		}()

		// start storage manager if enabled
		if vn.storageManager != nil {
			go func() {
				vn.log.I("starting storage manager")
				defer func() {
					vn.log.I("storage manager exited")

					// once storage manager exited, delete this virtual node
					vn.opt.VirtualnodeManager.Delete(vn.name)
				}()

				if err := vn.storageManager.Start(); err != nil {
					vn.log.I("failed to start storage manager", log.Error(err))
					return
				}
			}()
		}

		go func() {
			vn.log.I("starting network manager")
			defer func() {
				vn.log.I("network manager exited")

				// once network manager exited, delete this virtual node
				vn.opt.VirtualnodeManager.Delete(vn.name)
			}()

			if err := vn.networkManager.Start(); err != nil {
				vn.log.I("failed to start network manager", log.Error(err))
				return
			}
		}()

		go func() {
			vn.log.I("starting pod manager")
			defer func() {
				vn.log.I("pod manager exited")

				// once pod manager exited, delete this virtual node
				vn.opt.VirtualnodeManager.Delete(vn.name)
			}()

			if err := vn.podManager.Start(); err != nil {
				vn.log.I("failed to start pod manager", log.Error(err))
				return
			}
		}()

		go func() {
			vn.log.I("starting metrics manager")
			defer func() {
				vn.log.I("metrics manager exited")

				// once metrics manager exited, delete this virtual node
				vn.opt.VirtualnodeManager.Delete(vn.name)
			}()

			if err := vn.metricsManager.Start(); err != nil {
				vn.log.I("failed to start metrics manager", log.Error(err))
				return
			}
		}()

		go func() {
			vn.log.I("starting peripheral manager")
			defer func() {
				vn.log.I("peripheral manager exited")

				// once peripheral manager exited, delete this virtual node
				vn.opt.VirtualnodeManager.Delete(vn.name)
			}()

			if err := vn.peripheralManager.Start(); err != nil {
				vn.log.I("failed to start peripheral manager", log.Error(err))
				return
			}
		}()

		go func() {
			vn.log.D("starting to handle global messages")
			defer func() {
				vn.log.D("global message handler exited")

				// delete this virtual node once global message handle exited
				vn.opt.VirtualnodeManager.Delete(vn.name)
			}()

			// receive all global messages
			for msg := range vn.opt.ConnectivityManager.GlobalMessages() {
				vn.handleGlobalMsg(msg)
			}
		}()

		// serve remote device
		go func() {
			vn.log.D("starting to handle device connect")
			defer func() {
				vn.log.D("stopped waiting for device connect")

				// once virtual node exited, delete this virtual node and the according node object
				vn.opt.VirtualnodeManager.Delete(vn.name)
			}()

			for !vn.closing() {
				select {
				case <-vn.opt.ConnectivityManager.Connected():
					vn.log.I("device connected, starting to handle")
				case <-vn.ctx.Done():
					return
				}

				vn.log.I("syncing mirror node status for the first time")
				if err := vn.opt.ScheduleNodeSync(); err != nil {
					vn.log.I("failed to schedule mirror node sync, reject", log.Error(err))
					vn.opt.ConnectivityManager.Reject(
						aranyagopb.REJECTION_INTERNAL_SERVER_ERROR, "mirror node sync failure")
				}

				select {
				case <-vn.opt.ConnectivityManager.Disconnected():
					vn.log.I("device disconnected, waiting for next connection")
				case <-vn.ctx.Done():
				}

				vn.opt.VirtualnodeManager.OnVirtualNodeDisconnected(vn)
			}
		}()
	})

	return nil
}

// ForceClose shutdown this virtual node immediately
func (vn *VirtualNode) ForceClose() {
	vn.log.I("force close virtual node")

	_ = vn.kubeletSrv.Close()
	_ = vn.opt.KubeletServerListener.Close()
	// connectivity manager will handle gRPC server and listener close
	vn.opt.ConnectivityManager.Close()
	vn.podManager.Close()
	vn.metricsManager.Close()
	vn.peripheralManager.Close()
	vn.networkManager.Close()

	if vn.storageManager != nil {
		_ = vn.storageManager.Close()
	}

	vn.eventRecordingWatch.Stop()
	vn.eventLoggingWatch.Stop()

	vn.cancel()
}

func (vn *VirtualNode) KubeletServerListener() net.Listener {
	return vn.opt.KubeletServerListener
}

func (vn *VirtualNode) ConnectivityServerListener() net.Listener {
	if grpcOpts := vn.opt.ConnectivityOptions.GRPCOpts; grpcOpts != nil {
		return grpcOpts.Listener
	}

	return nil
}

func (vn *VirtualNode) Connected() bool {
	select {
	case <-vn.opt.ConnectivityManager.Connected():
		return true
	case <-vn.opt.ConnectivityManager.Disconnected():
		return false
	default:
		return false
	}
}

func (vn *VirtualNode) ExtInfo() (labels, annotations map[string]string) {
	return vn.nodeStatusCache.RetrieveExtInfo()
}

func (vn *VirtualNode) ActualNodeStatus(resNodeStatus corev1.NodeStatus) corev1.NodeStatus {
	return vn.nodeStatusCache.RetrieveStatus(resNodeStatus)
}

func (vn *VirtualNode) OS() string {
	return vn.nodeStatusCache.RetrieveStatus(corev1.NodeStatus{}).NodeInfo.OperatingSystem
}

func (vn *VirtualNode) SetPodCIDRs(ipv4, ipv6 string) {
	vn.networkManager.SetPodCIDRs(ipv4, ipv6)
}

func (vn *VirtualNode) closing() bool {
	select {
	case <-vn.ctx.Done():
		return true
	default:
		return false
	}
}
