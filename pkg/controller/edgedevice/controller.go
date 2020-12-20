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
	"errors"
	"fmt"
	"sync"
	"time"

	"arhat.dev/pkg/envhelper"
	"arhat.dev/pkg/kubehelper"
	"arhat.dev/pkg/log"
	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	informerscorev1 "k8s.io/client-go/informers/core/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	kubecache "k8s.io/client-go/tools/cache"
	cloudnodecontroller "k8s.io/cloud-provider/controllers/node"
	cloudnodelifecyclecontroller "k8s.io/cloud-provider/controllers/nodelifecycle"

	aranyaclient "arhat.dev/aranya/pkg/apis/aranya/generated/clientset/versioned"
	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/util/manager"
)

var (
	deleteAtOnce = metav1.NewDeleteOptions(0)
)

// CheckAPIVersionFallback is just a static library check for the api resource fallback if discovery failed,
// you should only use the result returned by this function when you have failed to discover api resources
// via kubernetes api discovery, and the result only contains the most legacy apis wanted by the controller
// and supported by the client library
func CheckAPIVersionFallback(kubeClient kubeclient.Interface) []*metav1.APIResourceList {
	var ret []*metav1.APIResourceList

	_ = kubeClient.CoordinationV1beta1().Leases("")
	_ = kubeClient.CoordinationV1().Leases("")

	ret = append(ret, &metav1.APIResourceList{
		GroupVersion: "coordination.k8s.io/v1beta1",
		APIResources: []metav1.APIResource{{
			Name:         "leases",
			SingularName: "",
			Namespaced:   true,
			Kind:         "Lease",
		}},
	})

	_ = kubeClient.StorageV1().CSIDrivers()
	_ = kubeClient.StorageV1beta1().CSIDrivers()

	ret = append(ret, &metav1.APIResourceList{
		GroupVersion: "storage.k8s.io/v1beta1",
		APIResources: []metav1.APIResource{{
			Name:         "csidrivers",
			SingularName: "",
			Namespaced:   false,
			Kind:         "CSIDriver",
		}},
	})

	return ret
}

// nolint:gocyclo
func NewController(
	appCtx context.Context,
	config *conf.Config,
	hostNodeName, hostname, hostIP string,
	hostNodeAddresses []corev1.NodeAddress,
	preferredResources []*metav1.APIResourceList,
) (*Controller, error) {
	kubeClient, kubeConfig, err := config.Aranya.KubeClient.NewKubeClient(nil, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create kube client for controller: %w", err)
	}

	aranyaClient, err := aranyaclient.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create aranya client: %w", err)
	}

	// informer factory for all managed Service, Secret
	thisPodNSInformerFactory := informers.NewSharedInformerFactoryWithOptions(
		kubeClient, 0, informers.WithNamespace(envhelper.ThisPodNS()),
	)

	// informer factory for all managed NodeClusterRoles, NodeVerbs
	clusterInformerFactory := informers.NewSharedInformerFactoryWithOptions(kubeClient, 0)
	nodeInformerTyped := informerscorev1.New(clusterInformerFactory, corev1.NamespaceAll,
		newTweakListOptionsFunc(
			labels.SelectorFromSet(map[string]string{
				constant.LabelRole:      constant.LabelRoleValueNode,
				constant.LabelNamespace: constant.SysNS(),
			}),
		),
	).Nodes()

	// informer factory for all sys resouces
	sysInformerFactory := informers.NewSharedInformerFactoryWithOptions(
		kubeClient, 0, informers.WithNamespace(constant.SysNS()),
	)

	// informer factory for all tenant resources (reuse if possible)
	var tenantInformerFactory informers.SharedInformerFactory
	if constant.SysNS() == constant.TenantNS() {
		tenantInformerFactory = sysInformerFactory
	} else {
		tenantInformerFactory = informers.NewSharedInformerFactoryWithOptions(
			kubeClient, 0, informers.WithNamespace(constant.TenantNS()),
		)
	}

	// informer factory for EdgeDevices

	ctrl := &Controller{
		BaseManager: manager.NewBaseManager(appCtx, "controller", nil),

		kubeClient:        kubeClient,
		hostNodeName:      hostNodeName,
		hostname:          hostname,
		hostIP:            hostIP,
		hostNodeAddresses: hostNodeAddresses,

		informerFactoryStart: []func(<-chan struct{}){
			clusterInformerFactory.Start,
			thisPodNSInformerFactory.Start,
			sysInformerFactory.Start,
			tenantInformerFactory.Start,
		},
	}

	err = ctrl.edgeDeviceController.init(ctrl, config, aranyaClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create edgedevice controller: %w", err)
	}

	err = ctrl.connectivityServiceController.init(ctrl, config, kubeClient, thisPodNSInformerFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create connectivity service controller: %w", err)
	}

	err = ctrl.nodeController.init(ctrl, config, kubeClient, nodeInformerTyped, preferredResources)
	if err != nil {
		return nil, fmt.Errorf("failed to create node controller: %w", err)
	}

	err = ctrl.nodeCertController.init(
		ctrl, &config.VirtualNode.Node, kubeClient, preferredResources, thisPodNSInformerFactory,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create node cert controller: %w", err)
	}

	err = ctrl.sysSecretController.init(ctrl, config, kubeClient, sysInformerFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create sys secret controller: %w", err)
	}

	err = ctrl.sysPodController.init(ctrl, config, kubeClient, sysInformerFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create sys pod controller: %w", err)
	}

	err = ctrl.sysPodRoleController.init(ctrl, config, kubeClient, sysInformerFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create sys pod role controller: %w", err)
	}

	err = ctrl.tenantPodController.init(ctrl, config, kubeClient, tenantInformerFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create pod controller: %w", err)
	}

	err = ctrl.tenantPodRoleController.init(ctrl, config, kubeClient, tenantInformerFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create pod role controller: %w", err)
	}

	err = ctrl.nodeClusterRoleController.init(ctrl, config, kubeClient, clusterInformerFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create node cluster role controller: %w", err)
	}

	err = ctrl.meshController.init(ctrl, config, kubeClient, tenantInformerFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create network controller: %w", err)
	}

	err = ctrl.storageServiceController.init(ctrl, config, kubeClient, thisPodNSInformerFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage service controller: %w", err)
	}

	if config.VirtualNode.Storage.Enabled {
		csiDriverClient := kubehelper.CreateCSIDriverClient(preferredResources, kubeClient)

		var csiDriverInformer kubecache.SharedIndexInformer
		switch {
		case csiDriverClient.V1Client != nil:
			csiDriverInformer = clusterInformerFactory.Storage().V1().CSIDrivers().Informer()
		case csiDriverClient.V1b1Client != nil:
			csiDriverInformer = clusterInformerFactory.Storage().V1beta1().CSIDrivers().Informer()
		default:
			return nil, fmt.Errorf("no csidriver api support in kubernetes cluster")
		}

		ctrl.csiDriverLister = kubehelper.CreateCSIDriverLister(csiDriverInformer.GetIndexer())
		ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs, csiDriverInformer.HasSynced)

		ctrl.listActions = append(ctrl.listActions, func() error {
			_, err2 := ctrl.csiDriverLister.List(labels.Everything())
			if err2 != nil {
				return fmt.Errorf("failed to list csi drivers: %w", err2)
			}
			return nil
		})

		// // create sftp server
		// sshListener, err2 := net.Listen("tcp", ":0")
		// if err2 != nil {
		// 	return nil, fmt.Errorf("failed to listen for ssh server: %w", err2)
		// }

		// hostKeyBytes, err2 := ioutil.ReadFile(config.VirtualNode.Storage.SFTP.HostKeyFile)
		// if err2 != nil {
		// 	return nil, fmt.Errorf("failed to read sftp host key: %w", err2)
		// }

		// hostKey, err2 := ssh.ParsePrivateKey(hostKeyBytes)
		// if err2 != nil {
		// 	return nil, fmt.Errorf("failed to parse sftp host key: %w", err2)
		// }

		// sftpManager := manager.NewSFTPManager(
		// 	appCtx, config.VirtualNode.Storage.RootDir,
		// 	sshListener, hostKey, publicKey.Marshal(),
		// )

		// go func() {
		// 	err2 := sftpManager.Start()
		// 	if err2 != nil {
		// 		panic(fmt.Errorf("failed to start sftp manager: %w", err2))
		// 	}
		// }()
	}

	if config.Aranya.RunAsCloudProvider {
		var err2 error
		ctrl.cloudNodeController, err2 = cloudnodecontroller.NewCloudNodeController(
			nodeInformerTyped, kubeClient, ctrl, 10*time.Second,
		)
		if err2 != nil {
			return nil, fmt.Errorf("failed to create cloud node controller: %w", err2)
		}

		ctrl.cloudNodeLifecycleController, err2 = cloudnodelifecyclecontroller.NewCloudNodeLifecycleController(
			nodeInformerTyped, kubeClient, ctrl, 5*time.Second)
		if err2 != nil {
			return nil, fmt.Errorf("failed to create cloud lifecycle controller: %w", err2)
		}
	}

	return ctrl, nil
}

type Controller struct {
	*manager.BaseManager

	kubeClient kubeclient.Interface

	hostNodeName      string
	hostname, hostIP  string
	hostNodeAddresses []corev1.NodeAddress

	cacheSyncWaitFuncs   []kubecache.InformerSynced
	informerFactoryStart []func(<-chan struct{})
	listActions          []func() error
	recStart             []func() error
	recReconcileUntil    []func(<-chan struct{})

	edgeDeviceController
	connectivityServiceController

	nodeController
	nodeCertController
	nodeClusterRoleController

	sysSecretController

	sysPodController
	sysPodRoleController

	tenantPodController
	tenantPodRoleController

	meshController

	storageServiceController

	// Storage management
	csiDriverLister *kubehelper.CSIDriverLister

	// unused fields
	cloudNodeController          *cloudnodecontroller.CloudNodeController
	cloudNodeLifecycleController *cloudnodelifecyclecontroller.CloudNodeLifecycleController
}

// Start caching but do not reconcile
func (c *Controller) Start() error {
	return c.OnStart(func() error {
		var (
			err    error
			stopCh = c.Context().Done()
		)

		for _, startInformerFactory := range c.informerFactoryStart {
			startInformerFactory(stopCh)
		}

		for _, prepareReconcile := range c.recStart {
			if err = prepareReconcile(); err != nil {
				return fmt.Errorf("failed to start reconciler: %w", err)
			}
		}

		for _, doList := range c.listActions {
			if err = doList(); err != nil {
				return fmt.Errorf("failed to do list for cache: %w", err)
			}
		}

		ok := kubecache.WaitForCacheSync(stopCh, c.cacheSyncWaitFuncs...)
		if !ok {
			return fmt.Errorf("failed to sync resource cache")
		}

		return nil
	})
}

// Reconcile resouces objects
func (c *Controller) Reconcile(wg *sync.WaitGroup, stop <-chan struct{}) {
	wg.Add(len(c.recReconcileUntil))

	for i := range c.recReconcileUntil {
		startReconcile := c.recReconcileUntil[i]
		go func() {
			defer wg.Done()

			startReconcile(stop)
		}()
	}

	if c.cloudNodeController != nil && c.cloudNodeLifecycleController != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()

			c.cloudNodeController.Run(stop)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()

			c.cloudNodeLifecycleController.Run(stop)
		}()
	}

	c.virtualNodes.Start(wg, stop)
}

func (c *Controller) Stop() error {
	c.OnClose(func() {
	})
	return nil
}

func (c *Controller) onEdgeDeviceCreationRequested(obj interface{}) *reconcile.Result {
	var (
		edgeDevice = obj.(*aranyaapi.EdgeDevice)
		name       = edgeDevice.Name
		logger     = c.Log.WithFields(log.String("name", name))
	)

	logger.I("instantiating edge device to virtual node")
	err := c.instantiateEdgeDevice(name)
	if err != nil {
		logger.I("failed to instantiate edge device", log.Error(err))
		return &reconcile.Result{Err: err}
	}

	return nil
}

func (c *Controller) onEdgeDeviceUpdateRequested(oldObj, newObj interface{}) *reconcile.Result {
	var (
		edgeDevice = newObj.(*aranyaapi.EdgeDevice)
		name       = edgeDevice.Name
		logger     = c.Log.WithFields(log.String("name", name))
	)

	// Update resources in sequence
	logger.D("deleting virtual node for edge device update")
	c.virtualNodes.Delete(name)
	err := c.vnRec.Schedule(queue.Job{Action: queue.ActionAdd, Key: name}, 0)
	if err != nil {
		return &reconcile.Result{Err: err}
	}

	return nil
}

func (c *Controller) onEdgeDeviceDeletionRequested(obj interface{}) *reconcile.Result {
	var (
		edgeDevice = obj.(*aranyaapi.EdgeDevice)
		name       = edgeDevice.Name
		logger     = c.Log.WithFields(log.String("name", name))
	)

	logger.D("deleting virtual node for edge device deletion")
	c.virtualNodes.Delete(name)

	return &reconcile.Result{NextAction: queue.ActionCleanup}
}

func (c *Controller) onEdgeDeviceCleanup(obj interface{}) *reconcile.Result {
	var (
		edgeDevice = obj.(*aranyaapi.EdgeDevice)
		name       = edgeDevice.Name
		logger     = c.Log.WithFields(log.String("name", name))
	)

	logger.D("deleting resource objects for EdgeDevice deletion")

	err := c.requestConnectivityServiceEnsure()
	if err != nil && !errors.Is(err, queue.ErrJobDuplicated) {
		return &reconcile.Result{Err: fmt.Errorf("failed to ensure connectivity service object: %w", err)}
	}

	err = c.nodeClient.Delete(c.Context(), name, *deleteAtOnce)
	if err != nil && !kubeerrors.IsNotFound(err) {
		return &reconcile.Result{Err: fmt.Errorf("failed to delete node object: %w", err)}
	}

	err = c.edgeDeviceClient.Delete(c.Context(), name, *deleteAtOnce)
	if err != nil && !kubeerrors.IsNotFound(err) {
		return &reconcile.Result{Err: fmt.Errorf("failed to delete edge device object: %w", err)}
	}

	return nil
}
