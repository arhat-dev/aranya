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

package edgedevice

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/pkg/kubehelper"
	"arhat.dev/pkg/log"
	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes/scheme"
	kubecache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	aranyaclient "arhat.dev/aranya/pkg/apis/aranya/generated/clientset/versioned"
	clientaranyav1a1 "arhat.dev/aranya/pkg/apis/aranya/generated/clientset/versioned/typed/aranya/v1alpha1"
	aranyainformers "arhat.dev/aranya/pkg/apis/aranya/generated/informers/externalversions"
	listersaranyav1a1 "arhat.dev/aranya/pkg/apis/aranya/generated/listers/aranya/v1alpha1"
	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/mesh"
	"arhat.dev/aranya/pkg/virtualnode"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
	"arhat.dev/aranya/pkg/virtualnode/metrics"
	"arhat.dev/aranya/pkg/virtualnode/network"
	"arhat.dev/aranya/pkg/virtualnode/peripheral"
	"arhat.dev/aranya/pkg/virtualnode/pod"
	"arhat.dev/aranya/pkg/virtualnode/storage"
)

type sessionTimeoutKey struct {
	name string
	sid  uint64
}

type sessionTimeoutValue struct {
	handleSessionTimeout connectivity.SessionTimeoutHandleFunc
}

type edgeDeviceController struct {
	timedSessions *sync.Map
	// shared timeout queue for session timeout
	timedSessionsTQ *queue.TimeoutQueue

	edgeDeviceClient   clientaranyav1a1.EdgeDeviceInterface
	edgeDeviceInformer kubecache.SharedIndexInformer
}

func (c *edgeDeviceController) init(
	ctrl *Controller,
	config *conf.Config,
	aranyaClient aranyaclient.Interface,
) error {
	c.timedSessions = new(sync.Map)

	c.timedSessionsTQ = queue.NewTimeoutQueue()
	c.timedSessionsTQ.Start(ctrl.Context().Done())

	timeoutCh := c.timedSessionsTQ.TakeCh()
	go func() {
		for {
			select {
			case session, more := <-timeoutCh:
				if !more {
					return
				}

				session.Data.(sessionTimeoutValue).handleSessionTimeout()

				c.timedSessions.Delete(session.Key.(sessionTimeoutKey))
			case <-ctrl.Context().Done():
				// controller exited, no timeout event will be handled
				return
			}
		}
	}()

	edgeDeviceInformerFactory := aranyainformers.NewSharedInformerFactoryWithOptions(
		aranyaClient, 0, aranyainformers.WithNamespace(constant.WatchNS()))

	ctrl.informerFactoryStart = append(ctrl.informerFactoryStart, edgeDeviceInformerFactory.Start)

	c.edgeDeviceClient = aranyaClient.AranyaV1alpha1().EdgeDevices(constant.WatchNS())
	c.edgeDeviceInformer = edgeDeviceInformerFactory.Aranya().V1alpha1().EdgeDevices().Informer()

	ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs, c.edgeDeviceInformer.HasSynced)

	ctrl.listActions = append(ctrl.listActions, func() error {
		_, err := listersaranyav1a1.
			NewEdgeDeviceLister(c.edgeDeviceInformer.GetIndexer()).List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list edgedevice in namespace %q: %w", constant.WatchNS(), err)
		}

		return nil
	})

	edgeDeviceRec := kubehelper.NewKubeInformerReconciler(ctrl.Context(), c.edgeDeviceInformer, reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:ed"),
		BackoffStrategy: nil,
		Workers:         1,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded:    ctrl.onEdgeDeviceResAdded,
			OnUpdated:  ctrl.onEdgeDeviceResUpdated,
			OnDeleting: ctrl.onEdgeDeviceResDeleting,
			OnDeleted:  ctrl.onEdgeDeviceResDeleted,
		},
	})
	ctrl.recStart = append(ctrl.recStart, edgeDeviceRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, edgeDeviceRec.ReconcileUntil)

	return nil
}

func (c *Controller) onEdgeDeviceResAdded(obj interface{}) *reconcile.Result {
	var (
		err        error
		edgeDevice = obj.(*aranyaapi.EdgeDevice)
		name       = edgeDevice.Name
		logger     = c.Log.WithFields(log.String("name", name))
	)

	if edgeDevice.Status.HostNode != c.hostNodeName {
		ed := edgeDevice.DeepCopy()
		ed.Status.HostNode = c.hostNodeName

		logger.V("updating edge device status")
		_, err = c.edgeDeviceClient.UpdateStatus(c.Context(), ed, metav1.UpdateOptions{})
		if err != nil {
			logger.I("failed to update edge device status", log.Error(err))
			return &reconcile.Result{Err: err}
		}
	}

	err = c.requestNodeClusterRoleEnsure()
	if err != nil {
		logger.I("failed to request node cluster role ensure", log.Error(err))
		return &reconcile.Result{Err: err}
	}

	logger.V("scheduling edge device instantiation work")
	c.vnRec.Update(name, nil, edgeDevice.DeepCopy())
	err = c.vnRec.Schedule(queue.Job{Action: queue.ActionUpdate, Key: name}, 0)
	if err != nil {
		logger.I("failed to schedule edge device instantiation work", log.NamedError("reason", err))
		// trigger reschedule to make sure it will be created
		return &reconcile.Result{Err: err}
	}

	return nil
}

func (c *Controller) onEdgeDeviceResUpdated(oldObj, newObj interface{}) *reconcile.Result {
	var (
		err           error
		oldEdgeDevice = oldObj.(*aranyaapi.EdgeDevice)
		newEdgeDevice = newObj.(*aranyaapi.EdgeDevice)
		name          = oldEdgeDevice.Name
		logger        = c.Log.WithFields(log.String("name", name))
	)

	if !c.checkEdgeDeviceUpdated(oldEdgeDevice, newEdgeDevice) {
		// no need to update virtualnode and related resources
		return nil
	}

	logger.V("scheduling edge device update work")
	c.vnRec.Update(name, oldEdgeDevice.DeepCopy(), newEdgeDevice.DeepCopy())
	err = c.vnRec.Schedule(queue.Job{Action: queue.ActionUpdate, Key: name}, 0)
	if err != nil {
		logger.I("failed to schedule edge device update work", log.NamedError("reason", err))
		return &reconcile.Result{Err: err}
	}

	return nil
}

func (c *Controller) onEdgeDeviceResDeleting(obj interface{}) *reconcile.Result {
	var (
		edgeDevice = obj.(*aranyaapi.EdgeDevice)
		name       = edgeDevice.Name
		logger     = c.Log.WithFields(log.String("name", name))
	)

	logger.D("scheduling edge device deletion work", log.String("name", name))

	c.vnRec.Update(name, nil, edgeDevice.DeepCopy())
	err := c.vnRec.Schedule(queue.Job{Action: queue.ActionDelete, Key: name}, 0)
	if err != nil {
		logger.I("failed to schedule edge device deletion work", log.NamedError("reason", err))
		return &reconcile.Result{Err: err}
	}

	return nil
}

func (c *Controller) onEdgeDeviceResDeleted(obj interface{}) *reconcile.Result {
	var (
		edgeDevice = obj.(*aranyaapi.EdgeDevice)
		name       = edgeDevice.Name
		logger     = c.Log.WithFields(log.String("name", name))
	)

	logger.V("requesting node cluster role update")
	err := c.requestNodeClusterRoleEnsure()
	if err != nil {
		logger.I("failed to request node cluster role update", log.Error(err))
		return &reconcile.Result{Err: err}
	}

	if _, ok := c.virtualNodes.Get(name); !ok {
		return nil
	}

	logger.V("scheduling edge device deletion work")
	c.vnRec.Update(name, nil, edgeDevice.DeepCopy())
	err = c.vnRec.Schedule(queue.Job{Action: queue.ActionDelete, Key: name}, 0)
	if err != nil {
		logger.I("failed to schedule edge device deletion work")
		// do not trigger reschedule since we can rely on job queue's deduplication behavior
		return nil
	}

	return nil
}

// instantiateEdgeDevice create required resources in sequence
//   - Ensure kubelet serving certificate valid (secret object contains valid certificate for this node)
//   - Create connectivity manager
//   - Create virtualnode with all options prepared
//   - Ensure NodeVerbs object
//	 - Start virtualnode
// nolint:gocyclo
func (c *Controller) instantiateEdgeDevice(name string) (err error) {
	var (
		logger = c.Log.WithFields(log.String("name", name))
		opts   = &virtualnode.CreationOptions{
			NodeName: name,
			Hostname: c.hostname,
			HostIP:   c.hostIP,

			SSHPrivateKey:      make([]byte, len(c.sshPrivateKey)),
			EventBroadcaster:   record.NewBroadcaster(),
			VirtualnodeManager: c.virtualNodes,

			ScheduleNodeSync: func() error {
				err2 := c.nodeStatusRec.Schedule(queue.Job{Action: queue.ActionUpdate, Key: name}, 0)
				if err2 != nil && !errors.Is(err2, queue.ErrJobDuplicated) {
					return err2
				}

				return nil
			},

			PodOptions: nil,
		}
	)
	_ = copy(opts.SSHPrivateKey, c.sshPrivateKey)

	ed, ok := c.getEdgeDeviceObject(name)
	if !ok {
		logger.I("failed to get edge device object for creation")
		return nil
	}

	meshSecret, err := c.ensureMeshConfig(ed.Name, ed.Spec.Network)
	if err != nil {
		logger.I("failed to ensure mesh config for edge device", log.Error(err))
		return err
	}

	logger.D("ensuring kubelet cert for edge device node")
	kubeletCert, err := c.ensureKubeletServerCert(
		name, c.hostNodeName, ed.Spec.Node.Cert, c.hostNodeAddresses,
	)
	if err != nil {
		logger.I("failed to get kubelet cert for edge device node", log.Error(err))
		return err
	}

	logger.D("creating listener for kubelet server")
	httpListener, err := net.Listen("tcp", ":0")
	if err != nil {
		logger.I("failed to create listener for kubelet server", log.Error(err))
		return err
	}
	defer func() {
		if err != nil {
			_ = httpListener.Close()
		}
	}()

	// create a new kube client just in case of rate limiting affected by other client
	opts.KubeClient, _, err = c.vnConfig.KubeClient.NewKubeClient(c.vnKubeconfig, true)
	if err != nil {
		logger.I("failed to create kube client", log.Error(err))
		return err
	}

	opts.KubeletServerListener = tls.NewListener(
		httpListener, &tls.Config{Certificates: []tls.Certificate{*kubeletCert}},
	)
	defer func() {
		if err != nil {
			logger.V("closing kubelet server listener due to error")
			_ = opts.KubeletServerListener.Close()
		}
	}()

	// prepare all required options
	vnConfig := c.vnConfig.OverrideWith(ed.Spec)
	{
		logger.D("preparing connectivity options")
		opts.ConnectivityOptions, err = c.prepareConnectivityOptions(ed, vnConfig)
		if err != nil {
			logger.I("failed to prepare connectivity options", log.Error(err))
			return err
		}

		logger.D("preparing node options")
		opts.NodeOptions, err = c.prepareNodeOptions(ed, vnConfig)
		if err != nil {
			logger.I("failed to prepare node options", log.Error(err))
			return err
		}

		logger.D("preparing associated peripheral options")
		opts.PeripheralOptions, err = c.preparePeripheralOptions(ed, vnConfig)
		if err != nil {
			logger.I("failed to prepare associated peripheral options", log.Error(err))
			return err
		}

		logger.D("preparing metrics options")
		opts.MetricsOptions, err = c.prepareMetricsOptions(ed, vnConfig)
		if err != nil {
			logger.I("failed to prepare metrics options", log.Error(err))
			return err
		}

		logger.D("preparing pod options")
		opts.PodOptions, err = c.preparePodOptions(ed, vnConfig, opts.EventBroadcaster)
		if err != nil {
			logger.I("failed to prepare pod options", log.Error(err))
			return err
		}

		logger.D("preparing storage options")
		opts.StorageOptions, err = c.prepareStorageOptions(ed, vnConfig, opts.EventBroadcaster)
		if err != nil {
			logger.I("failed to prepare storage options", log.Error(err))
			return err
		}

		logger.D("preparing network options")
		opts.NetworkOptions, err = c.prepareNetworkOptions(ed, vnConfig, meshSecret)
		if err != nil {
			logger.I("failed to prepare network options", log.Error(err))
			return err
		}
	}

	virtualNodeCtx, cancelVirtualNode := context.WithCancel(c.Context())
	logger.D("creating connectivity manager")
	opts.ConnectivityManager, err = connectivity.NewManager(virtualNodeCtx, name, opts.ConnectivityOptions)
	if err != nil {
		cancelVirtualNode()
		logger.I("failed to create connectivity manager", log.Error(err))
		return err
	}
	defer func() {
		if err != nil {
			logger.I("closing connectivity manager due to error")
			opts.ConnectivityManager.Close()
		}
	}()

	logger.D("creating virtual node")
	vn, err := virtualnode.CreateVirtualNode(virtualNodeCtx, cancelVirtualNode, opts)
	if err != nil {
		logger.I("failed to create virtual node", log.Error(err))
		return err
	}

	defer func() {
		if err != nil {
			vn.ForceClose()
		}
	}()

	// MUST add virtual node before request node ensure
	err = c.virtualNodes.Add(vn)
	if err != nil {
		logger.I("failed to add virtual node, removing existing one", log.Error(err))
		c.virtualNodes.Delete(name)

		if err = c.virtualNodes.Add(vn); err != nil {
			logger.E("failed to add virtual node, unrecoverable", log.Error(err))
			return err
		}
	}

	defer func() {
		if err != nil {
			c.virtualNodes.Delete(name)
		}
	}()

	logger.D("requesting node ensure")
	err = c.requestNodeEnsure(name)
	if err != nil {
		logger.I("failed to request node ensure", log.Error(err))
		return err
	}

	if c.vnConfig.Node.Lease.Enabled {
		logger.D("requesting node lease ensure")
		err = c.requestNodeLeaseEnsure(name)
		if err != nil {
			logger.I("failed to request node lease ensure", log.Error(err))
			return err
		}
	}

	logger.D("requesting virtual pod ensure")
	err = c.requestVirtualPodEnsure(name)
	if err != nil {
		logger.I("failed to request virtual pod ensure", log.Error(err))
		return err
	}

	if vnConfig.Network.Enabled {
		logger.D("requesting network ensure")
		err = c.requestNetworkEnsure(name)
		if err != nil {
			logger.I("failed to request network ensure", log.Error(err))
			return err
		}
	}

	if vn.ConnectivityServerListener() != nil && c.svcReqRec != nil {
		logger.D("requesting connectivity service ensure")
		err = c.requestConnectivityServiceEnsure()
		if err != nil {
			logger.I("failed to request connectivity service ensure", log.Error(err))
			return err
		}
	}

	logger.D("starting virtual node")
	err = vn.Start()
	if err != nil {
		logger.I("failed to start virtual node", log.Error(err))
		return err
	}

	return nil
}

func (c *Controller) checkEdgeDeviceUpdated(old, new *aranyaapi.EdgeDevice) bool {
	return !reflect.DeepEqual(old.Spec, new.Spec)
}

func (c *Controller) prepareConnectivityOptions(
	edgeDevice *aranyaapi.EdgeDevice,
	vnConfig *conf.VirtualnodeConfig,
) (config *connectivity.Options, err error) {
	config = &connectivity.Options{
		AddTimedStreamCreation: func(sid uint64, onCreationTimeout connectivity.SessionTimeoutHandleFunc) {
			key := sessionTimeoutKey{
				name: edgeDevice.Name,
				sid:  sid,
			}
			_ = c.timedSessionsTQ.OfferWithDelay(
				key,
				sessionTimeoutValue{
					handleSessionTimeout: onCreationTimeout,
				},
				vnConfig.Pod.Timers.StreamCreationTimeout,
			)
		},

		AddTimedSession: func(sid uint64, onSessionTimeout connectivity.SessionTimeoutHandleFunc) {
			key := sessionTimeoutKey{
				name: edgeDevice.Name,
				sid:  sid,
			}
			_ = c.timedSessionsTQ.OfferWithDelay(
				key,
				sessionTimeoutValue{
					handleSessionTimeout: onSessionTimeout,
				},
				vnConfig.Connectivity.Timers.UnarySessionTimeout,
			)

			c.timedSessions.Store(key, nil)
		},
		DelTimedSession: func(sid uint64) {
			c.timedSessionsTQ.Remove(sid)
			c.timedSessions.Delete(sessionTimeoutKey{
				name: edgeDevice.Name,
				sid:  sid,
			})
		},
		GetTimedSessions: func() (timedSessionIDs []uint64) {
			c.timedSessions.Range(func(key, value interface{}) bool {
				k := key.(sessionTimeoutKey)
				if k.name == edgeDevice.Name {
					timedSessionIDs = append(timedSessionIDs, k.sid)
				}

				return true
			})

			return
		},

		GRPCOpts:        nil,
		MQTTOpts:        nil,
		AMQPOpts:        nil,
		AzureIoTHubOpts: nil,
		GCPIoTCoreOpts:  nil,
	}

	switch edgeDevice.Spec.Connectivity.Method {
	case aranyaapi.GCPIoTCore:
		opts, err2 := c.generateGCPPubSubOpts(edgeDevice.Spec.Connectivity.GCPIoTCore)
		if err2 != nil {
			return nil, fmt.Errorf("failed to generate gcp pub/sub opts: %w", err2)
		}

		config.GCPIoTCoreOpts = opts
	case aranyaapi.AzureIoTHub:
		opts, err2 := c.generateAzureIoTHubOpts(edgeDevice.Spec.Connectivity.AzureIoTHub)
		if err2 != nil {
			return nil, fmt.Errorf("failed to generate azure iot hub opts: %w", err2)
		}

		config.AzureIoTHubOpts = opts
	case aranyaapi.GRPC:
		var (
			grpcConfig     = edgeDevice.Spec.Connectivity.GRPC
			grpcSrvOptions []grpc.ServerOption
			tlsConfig      *tls.Config
		)

		tlsConfig, err = c.createTLSConfigFromSecret(grpcConfig.TLSSecretRef)
		if err != nil {
			c.Log.I("failed to get grpc server tls secret", log.Error(err))
			return nil, err
		}

		if tlsConfig != nil {
			grpcSrvOptions = append(grpcSrvOptions, grpc.Creds(credentials.NewTLS(tlsConfig)))
		}

		var grpcListener net.Listener
		grpcListener, err = net.Listen("tcp", ":0")
		if err != nil {
			return nil, err
		}

		defer func() {
			if err != nil {
				_ = grpcListener.Close()
			}
		}()

		config.GRPCOpts = &connectivity.GRPCOpts{
			Server:   grpc.NewServer(grpcSrvOptions...),
			Listener: grpcListener,
		}
	case aranyaapi.MQTT:
		var (
			mqttConfig = edgeDevice.Spec.Connectivity.MQTT
			tlsConfig  *tls.Config
			username   []byte
			password   []byte
		)

		tlsConfig, err = c.createTLSConfigFromSecret(mqttConfig.TLSSecretRef)
		if err != nil {
			c.Log.I("failed to get mqtt client tls secret", log.Error(err))
			return nil, err
		}

		if userPassRef := mqttConfig.UserPassRef; userPassRef != nil && userPassRef.Name != "" {
			c.Log.D("fetching mqtt user pass")
			username, password, err = c.getUserPassFromSecret(userPassRef.Name)
			if err != nil {
				c.Log.I("failed to get mqtt client user pass", log.Error(err))
				return nil, err
			}
		}

		config.MQTTOpts = &connectivity.MQTTOpts{
			TLSConfig: tlsConfig,
			Username:  username,
			Password:  password,
			Config:    mqttConfig,
		}
	case aranyaapi.AMQP:
		var (
			amqpConfig = edgeDevice.Spec.Connectivity.AMQP
			tlsConfig  *tls.Config
			username   []byte
			password   []byte
		)

		tlsConfig, err = c.createTLSConfigFromSecret(amqpConfig.TLSSecretRef)
		if err != nil {
			c.Log.I("failed to create amqp client tls config", log.Error(err))
			return nil, err
		}

		if userPassRef := amqpConfig.UserPassRef; userPassRef != nil && userPassRef.Name != "" {
			c.Log.D("fetching amqp username and password")
			username, password, err = c.getUserPassFromSecret(userPassRef.Name)
			if err != nil {
				c.Log.I("failed to get amqp username and password", log.Error(err))
				return nil, err
			}
		}

		config.AMQPOpts = &connectivity.AMQPOpts{
			TLSConfig: tlsConfig,
			Username:  username,
			Password:  password,
			Config:    amqpConfig,
		}
	default:
		err = fmt.Errorf("unsupported connectivity method: %s", edgeDevice.Spec.Connectivity.Method)
		return
	}

	return
}

// nolint:unparam
func (c *Controller) prepareNodeOptions(
	edgeDevice *aranyaapi.EdgeDevice,
	vnConfig *conf.VirtualnodeConfig,
) (_ *virtualnode.Options, err error) {
	_ = edgeDevice
	opts := &virtualnode.Options{
		ForceSyncInterval:      vnConfig.Node.Timers.ForceSyncInterval,
		MirrorNodeSyncInterval: vnConfig.Node.Timers.MirrorSyncInterval,
	}

	return opts, nil
}

// nolint:unparam
func (c *Controller) preparePodOptions(
	edgeDevice *aranyaapi.EdgeDevice,
	vnConfig *conf.VirtualnodeConfig,
	evb record.EventBroadcaster,
) (_ *pod.Options, err error) {
	opts := &pod.Options{
		Config: &vnConfig.Pod,
		GetNode: func() *corev1.Node {
			node, ok := c.getNodeObject(edgeDevice.Name)
			if !ok {
				return nil
			}

			return node
		},
		GetPod: func(name string) *corev1.Pod {
			po, ok := c.getWatchPodObject(name)
			if !ok {
				return nil
			}

			return po
		},
		GetConfigMap: func(name string) *corev1.ConfigMap {
			cm, ok := c.getWatchConfigMapObject(name)
			if !ok {
				return nil
			}

			return cm
		},
		GetSecret: func(name string) *corev1.Secret {
			secret, ok := c.getWatchSecretObject(name)
			if !ok {
				return nil
			}

			return secret
		},
		ListServices: c.listWatchServiceObjects,
		EventRecorder: evb.NewRecorder(
			scheme.Scheme,
			corev1.EventSource{Component: fmt.Sprintf("aranya-pod-manager,%s", edgeDevice.Name)},
		),
	}

	return opts, nil
}

func (c *Controller) preparePeripheralOptions(
	edgeDevice *aranyaapi.EdgeDevice,
	vnConfig *conf.VirtualnodeConfig,
) (_ *peripheral.Options, err error) {
	_ = vnConfig
	var (
		mrs         = make(map[string]*aranyagopb.PeripheralEnsureCmd)
		peripherals = make(map[string]*aranyagopb.PeripheralEnsureCmd)
		devs        = make(map[string]struct{})
	)

	for _, mr := range edgeDevice.Spec.MetricsReporters {
		if mr.Name == "" {
			err = fmt.Errorf("invalid empty metrics reporter name")
			return
		}

		if _, ok := mrs[mr.Name]; ok {
			err = fmt.Errorf("duplicate metrics reporter name %q", mr.Name)
			return
		}

		var conn *aranyagopb.Connectivity
		conn, err = c.createPeripheralConnector(mr.Connector)
		if err != nil {
			return nil, fmt.Errorf("failed to create peripheral connector: %w", err)
		}

		mrs[mr.Name] = &aranyagopb.PeripheralEnsureCmd{
			Kind:      aranyagopb.PERIPHERAL_TYPE_METRICS_REPORTER,
			Name:      mr.Name,
			Connector: conn,
		}
	}

	for _, d := range edgeDevice.Spec.Peripherals {
		if d.Name == "" || d.Name == constant.VirtualContainerNameHost {
			err = fmt.Errorf("invalid peripheral name %q", d.Name)
			return
		}

		if msgs := validation.IsDNS1123Label(d.Name); len(msgs) > 0 {
			err = fmt.Errorf("peripheral name %q is not a valid dns label: %s", d.Name, strings.Join(msgs, ", "))
			return
		}

		if _, ok := devs[d.Name]; ok {
			err = fmt.Errorf("duplicate peripheral name %q", d.Name)
			return
		}

		devs[d.Name] = struct{}{}

		var operations []*aranyagopb.PeripheralOperation
		for _, op := range d.Operations {
			operations = append(operations, &aranyagopb.PeripheralOperation{
				OperationId: op.Name,
				Params:      op.Params,
			})
		}

		var peripheralMetrics []*aranyagopb.PeripheralMetric
		for _, m := range d.Metrics {
			var reportMethod aranyagopb.PeripheralMetric_ReportMethod
			switch m.ReportMethod {
			case aranyaapi.ReportViaArhatConnectivity:
				reportMethod = aranyagopb.REPORT_WITH_ARHAT_CONNECTIVITY
			case aranyaapi.ReportViaNodeMetrics, "":
				reportMethod = aranyagopb.REPORT_WITH_NODE_METRICS
			case aranyaapi.ReportViaStandaloneClient:
				reportMethod = aranyagopb.REPORT_WITH_STANDALONE_CLIENT
			}

			var valueType aranyagopb.PeripheralMetric_ValueType
			switch m.ValueType {
			case aranyaapi.PeripheralMetricValueTypeGauge:
				valueType = aranyagopb.METRICS_VALUE_TYPE_GAUGE
			case aranyaapi.PeripheralMetricValueTypeCounter:
				valueType = aranyagopb.METRICS_VALUE_TYPE_COUNTER
			case aranyaapi.PeripheralMetricValueTypeUnknown, "":
				valueType = aranyagopb.METRICS_VALUE_TYPE_UNTYPED
			}

			peripheralMetrics = append(peripheralMetrics, &aranyagopb.PeripheralMetric{
				Name:             m.Name,
				PeripheralParams: m.Params,
				ValueType:        valueType,
				ReportMethod:     reportMethod,
				ReporterName:     m.ReporterName,
				ReporterParams:   m.ReporterParams,
			})
		}

		conn, err := c.createPeripheralConnector(d.Connector)
		if err != nil {
			return nil, fmt.Errorf("failed to create peripheral connector: %w", err)
		}

		peripherals[d.Name] = &aranyagopb.PeripheralEnsureCmd{
			Kind:       aranyagopb.PERIPHERAL_TYPE_NORMAL,
			Name:       d.Name,
			Connector:  conn,
			Operations: operations,
			Metrics:    peripheralMetrics,
		}
	}

	return &peripheral.Options{
		MetricsReporters: mrs,
		Peripherals:      peripherals,
	}, nil
}

// nolint:unparam
func (c *Controller) prepareMetricsOptions(
	edgeDevice *aranyaapi.EdgeDevice,
	vnConfig *conf.VirtualnodeConfig,
) (_ *metrics.Options, err error) {
	_ = edgeDevice
	opts := &metrics.Options{
		NodeMetrics:    vnConfig.Node.Metrics,
		RuntimeMetrics: &vnConfig.Pod.Metrics,
	}

	return opts, nil
}

// nolint:unparam
func (c *Controller) prepareStorageOptions(
	edgeDevice *aranyaapi.EdgeDevice,
	vnConfig *conf.VirtualnodeConfig,
	evb record.EventBroadcaster,
) (_ *storage.Options, err error) {
	if !vnConfig.Storage.Enabled {
		return nil, nil
	}

	opts := &storage.Options{
		StorageRootDir:         vnConfig.Storage.RootDir,
		KubeletRegistrationDir: vnConfig.Storage.KubeletRegistrationDir,
		KubeletPluginsDir:      vnConfig.Storage.KubeletPluginsDir,
		CSIDriverLister:        c.csiDriverLister,
		EventRecorder: evb.NewRecorder(
			scheme.Scheme,
			corev1.EventSource{Component: fmt.Sprintf("aranya-storage-manager,%s", edgeDevice.Name)},
		),
	}

	return opts, nil
}

// nolint:unparam
func (c *Controller) prepareNetworkOptions(
	edgeDevice *aranyaapi.EdgeDevice,
	vnConfig *conf.VirtualnodeConfig,
	secret *corev1.Secret,
) (_ *network.Options, err error) {
	if !vnConfig.Network.Enabled {
		return &network.Options{}, nil
	}

	if c.meshIPAMv4 == nil && c.meshIPAMv6 == nil {
		return nil, fmt.Errorf("no mesh ip cidr configured")
	}

	var (
		ipv4Req = edgeDevice.Spec.Network.Mesh.IPv4Addr
		ipv6Req = edgeDevice.Spec.Network.Mesh.IPv6Addr
	)

	if ipv4Req == "" {
		ipv4Req = edgeDevice.Status.Network.MeshIPv4Addr
	}

	if ipv6Req == "" {
		ipv6Req = edgeDevice.Status.Network.MeshIPv6Addr
	}

	if ipv4Req != "" {
		ipv4, _, err := net.ParseCIDR(ipv4Req)
		if err == nil {
			ipv4Req = ipv4.String()
		}
	}

	if ipv6Req != "" {
		ipv6, _, err := net.ParseCIDR(ipv6Req)
		if err == nil {
			ipv6Req = ipv6.String()
		}
	}

	var (
		ipv4, ipv6 string
	)
	// allocate dynamic ipv4 address
	if c.meshIPAMv4 != nil {
		ip, err := c.meshIPAMv4.Allocate(net.ParseIP(ipv4Req))
		if err != nil {
			return nil, fmt.Errorf("failed to allocate ipv4 address: %s", err)
		}
		ipv4 = ip.String()
	}

	// allocate dynamic ipv6 address
	if c.meshIPAMv6 != nil {
		ip, err := c.meshIPAMv6.Allocate(net.ParseIP(ipv6Req))
		if err != nil {
			return nil, fmt.Errorf("failed to allocate ipv6 address: %s", err)
		}
		ipv6 = ip.String()
	}

	var addresses []string
	if ipv4 != "" {
		addresses = append(addresses, ipv4)
	}

	if ipv6 != "" {
		addresses = append(addresses, ipv6)
	}

	if len(addresses) == 0 {
		return nil, fmt.Errorf("no ip address allocated")
	}

	opts := &network.Options{
		PublicAddresses: c.vnConfig.Network.NetworkService.Addresses,

		InterfaceName:     edgeDevice.Spec.Network.Mesh.InterfaceName,
		MTU:               edgeDevice.Spec.Network.Mesh.MTU,
		Provider:          constant.PrefixMeshInterfaceProviderAranya + edgeDevice.Name,
		ExtraAllowedCIDRs: edgeDevice.Spec.Network.Mesh.ExtraAllowedCIDRs,
		Addresses:         addresses,

		WireguardOpts: nil,
	}

	switch d := vnConfig.Network.Backend.Driver; d {
	case constant.NetworkMeshDriverWireguard:
		cfg := vnConfig.Network.Backend.Wireguard

		privateKey, ok := accessMap(secret.StringData, secret.Data, constant.MeshConfigKeyWireguardPrivateKey)
		if !ok {
			return nil, fmt.Errorf("no private key for wireguard mesh")
		}

		opts.WireguardOpts = &mesh.WireguardOpts{
			LogLevel:         "debug",
			PreSharedKey:     cfg.PreSharedKey,
			PrivateKey:       base64.StdEncoding.EncodeToString(privateKey),
			KeepaliveSeconds: int32(cfg.Keepalive.Seconds()),
			ListenPort:       int32(edgeDevice.Spec.Network.Mesh.ListenPort),
			RoutingTable:     int32(edgeDevice.Spec.Network.Mesh.RoutingTable),
			FirewallMark:     int32(edgeDevice.Spec.Network.Mesh.FirewallMark),
		}
	default:
		return nil, fmt.Errorf("unknown backend driver: %s", d)
	}

	return opts, nil
}
