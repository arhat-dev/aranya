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

	"arhat.dev/pkg/backoff"
	"arhat.dev/pkg/envhelper"
	"arhat.dev/pkg/kubehelper"
	"arhat.dev/pkg/log"
	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	informerscorev1 "k8s.io/client-go/informers/core/v1"
	informersrbacv1 "k8s.io/client-go/informers/rbac/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	clientcodv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	clientrbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	listerscodv1 "k8s.io/client-go/listers/coordination/v1"
	listerscodv1b1 "k8s.io/client-go/listers/coordination/v1beta1"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	listersrbacv1 "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/rest"
	kubecache "k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/controller/cloud"

	aranyaclient "arhat.dev/aranya/pkg/apis/aranya/generated/clientset/versioned"
	clientaranyav1a1 "arhat.dev/aranya/pkg/apis/aranya/generated/clientset/versioned/typed/aranya/v1alpha1"
	aranyainformers "arhat.dev/aranya/pkg/apis/aranya/generated/informers/externalversions"
	listersaranyav1a1 "arhat.dev/aranya/pkg/apis/aranya/generated/listers/aranya/v1alpha1"
	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/util/manager"
	"arhat.dev/aranya/pkg/virtualnode"
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
	sshPrivateKey []byte,
	hostNodeName, hostname, hostIP string,
	hostNodeAddresses []corev1.NodeAddress,
	thisPodLabels map[string]string,
	preferredResources []*metav1.APIResourceList,
) (*Controller, error) {
	kubeClient, kubeConfig, err := config.Aranya.KubeClient.NewKubeClient(nil, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create kube client for controller: %w", err)
	}

	aranyaClient, err := aranyaclient.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create aranya client for controller: %w", err)
	}

	_, kubeConfigForVirtualNode, err := config.VirtualNode.KubeClient.NewKubeClient(nil, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig for virtualnode: %w", err)
	}

	// informer factory for all managed Service, Secret
	sysInformerFactory := informers.NewSharedInformerFactoryWithOptions(
		kubeClient, 0, informers.WithNamespace(envhelper.ThisPodNS()))

	// informer factory for all managed NodeClusterRoles, NodeVerbs
	clusterInformerFactory := informers.NewSharedInformerFactoryWithOptions(kubeClient, 0)
	nodeInformerTyped := informerscorev1.New(clusterInformerFactory, corev1.NamespaceAll,
		newTweakListOptionsFunc(
			labels.SelectorFromSet(map[string]string{
				constant.LabelRole:      constant.LabelRoleValueNode,
				constant.LabelNamespace: constant.WatchNS(),
			}),
		),
	).Nodes()
	nodeInformer := nodeInformerTyped.Informer()

	// informer factory for all watched pods
	watchInformerFactory := informers.NewSharedInformerFactoryWithOptions(
		kubeClient, 0, informers.WithNamespace(constant.WatchNS()))
	podInformer := watchInformerFactory.Core().V1().Pods().Informer()
	secretInformer := watchInformerFactory.Core().V1().Secrets().Informer()
	cmInformer := watchInformerFactory.Core().V1().ConfigMaps().Informer()
	svcInformer := watchInformerFactory.Core().V1().Services().Informer()

	// informer factory for EdgeDevices
	edgeDeviceInformerFactory := aranyainformers.NewSharedInformerFactoryWithOptions(
		aranyaClient, 0, aranyainformers.WithNamespace(constant.WatchNS()))
	edgeDeviceInformer := edgeDeviceInformerFactory.Aranya().V1alpha1().EdgeDevices().Informer()

	ctrl := &Controller{
		BaseManager: manager.NewBaseManager(appCtx, "controller", nil),

		connectivityService: config.Aranya.Managed.ConnectivityService.Name,
		nodeClusterRoles:    config.Aranya.Managed.NodeClusterRoles,
		podRoles:            config.Aranya.Managed.PodRoles,
		virtualPodRoles:     config.Aranya.Managed.VirtualPodRoles,

		vnKubeconfig:     kubeConfigForVirtualNode,
		kubeClient:       kubeClient,
		edgeDeviceClient: aranyaClient.AranyaV1alpha1().EdgeDevices(constant.WatchNS()),
		nodeClient:       kubeClient.CoreV1().Nodes(),
		podClient:        kubeClient.CoreV1().Pods(constant.WatchNS()),
		csrClient:        kubehelper.CreateCertificateSigningRequestClient(preferredResources, kubeClient),
		certSecretClient: kubeClient.CoreV1().Secrets(envhelper.ThisPodNS()),
		nodeLeaseClient:  kubeClient.CoordinationV1().Leases(corev1.NamespaceNodeLease),

		watchSecretInformer: secretInformer,
		watchCMInformer:     cmInformer,
		watchSvcInformer:    svcInformer,

		hostNodeName:      hostNodeName,
		hostname:          hostname,
		hostIP:            hostIP,
		hostNodeAddresses: hostNodeAddresses,
		thisPodLabels:     thisPodLabels,

		nodeInformer:       nodeInformer,
		podInformer:        podInformer,
		edgeDeviceInformer: edgeDeviceInformer,

		cacheSyncWaitFuncs: []kubecache.InformerSynced{
			edgeDeviceInformer.HasSynced,
			podInformer.HasSynced,
			nodeInformer.HasSynced,
			cmInformer.HasSynced,
			secretInformer.HasSynced,
			svcInformer.HasSynced,
		},

		informerFactoryStart: []func(<-chan struct{}){
			edgeDeviceInformerFactory.Start,
			clusterInformerFactory.Start,
			sysInformerFactory.Start,
			watchInformerFactory.Start,
		},
		listActions: []func() error{
			func() error {
				_, err2 := listersaranyav1a1.
					NewEdgeDeviceLister(edgeDeviceInformer.GetIndexer()).List(labels.Everything())
				if err2 != nil {
					return fmt.Errorf("failed to list edgedevice in namespace %q: %w", constant.WatchNS(), err2)
				}

				_, err2 = listerscorev1.NewNodeLister(nodeInformer.GetIndexer()).List(labels.Everything())
				if err2 != nil {
					return fmt.Errorf("failed to list nodes: %w", err2)
				}

				_, err2 = listerscorev1.NewPodLister(podInformer.GetIndexer()).List(labels.Everything())
				if err2 != nil {
					return fmt.Errorf("failed to list pods in namespace %q: %w", constant.WatchNS(), err2)
				}

				_, err2 = listerscorev1.NewConfigMapLister(cmInformer.GetIndexer()).List(labels.Everything())
				if err2 != nil {
					return fmt.Errorf("failed to list configmaps in namespace %q: %w", constant.WatchNS(), err2)
				}

				_, err2 = listerscorev1.NewSecretLister(secretInformer.GetIndexer()).List(labels.Everything())
				if err2 != nil {
					return fmt.Errorf("failed to list secrets in namespace %q: %w", constant.WatchNS(), err2)
				}

				_, err2 = listerscorev1.NewServiceLister(svcInformer.GetIndexer()).List(labels.Everything())
				if err2 != nil {
					return fmt.Errorf("failed to list services in namespace %q: %w", constant.WatchNS(), err2)
				}

				return nil
			},
		},

		sshPrivateKey: sshPrivateKey,
		vnConfig:      &config.VirtualNode,

		managedPods: sets.NewString(),
		podsMu:      new(sync.RWMutex),
	}

	var getLeaseFunc func(name string) *coordinationv1.Lease
	if config.VirtualNode.Node.Lease.Enabled {
		// informer and sync
		nodeLeaseInformerFactory := informers.NewSharedInformerFactoryWithOptions(kubeClient, 0,
			informers.WithNamespace(corev1.NamespaceNodeLease),
			informers.WithTweakListOptions(
				newTweakListOptionsFunc(
					labels.SelectorFromSet(map[string]string{
						constant.LabelRole:      constant.LabelRoleValueNodeLease,
						constant.LabelNamespace: constant.WatchNS(),
					}),
				),
			),
		)

		ctrl.informerFactoryStart = append(ctrl.informerFactoryStart, nodeLeaseInformerFactory.Start)

		leaseClient := kubehelper.CreateLeaseClient(preferredResources, kubeClient, corev1.NamespaceNodeLease)
		switch {
		case leaseClient.V1Client != nil:
			ctrl.nodeLeaseInformer = nodeLeaseInformerFactory.Coordination().V1().Leases().Informer()
			ctrl.listActions = append(ctrl.listActions, func() error {
				_, err2 := listerscodv1.NewLeaseLister(ctrl.nodeLeaseInformer.GetIndexer()).List(labels.Everything())
				if err2 != nil {
					return fmt.Errorf("failed to list cluster roles: %w", err2)
				}

				return nil
			})
		case leaseClient.V1b1Client != nil:
			ctrl.nodeLeaseInformer = nodeLeaseInformerFactory.Coordination().V1beta1().Leases().Informer()
			ctrl.listActions = append(ctrl.listActions, func() error {
				_, err2 := listerscodv1b1.NewLeaseLister(ctrl.nodeLeaseInformer.GetIndexer()).List(labels.Everything())
				if err2 != nil {
					return fmt.Errorf("failed to list cluster roles: %w", err2)
				}

				return nil
			})
		default:
			return nil, fmt.Errorf("no lease api support in kubernetes cluster")
		}

		ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs, ctrl.nodeLeaseInformer.HasSynced)

		getLeaseFunc = func(name string) *coordinationv1.Lease {
			obj, ok, err2 := ctrl.nodeLeaseInformer.GetIndexer().GetByKey(corev1.NamespaceNodeLease + "/" + name)
			if err2 != nil {
				// ignore this error
				return nil
			}

			if !ok {
				return nil
			}

			lease, ok := obj.(*coordinationv1.Lease)
			if !ok {
				return nil
			}

			return lease
		}

		nodeLeaseRec := reconcile.NewKubeInformerReconciler(appCtx, ctrl.nodeLeaseInformer, reconcile.Options{
			Logger:          ctrl.Log.WithName("rec:nl"),
			BackoffStrategy: nil,
			Workers:         1,
			RequireCache:    true,
			Handlers: reconcile.HandleFuncs{
				OnAdded:    nextActionUpdate,
				OnUpdated:  ctrl.onNodeLeaseUpdated,
				OnDeleting: ctrl.onNodeLeaseDeleting,
				OnDeleted:  ctrl.onNodeLeaseDeleted,
			},
			OnBackoffStart: nil,
			OnBackoffReset: nil,
		})
		ctrl.recStart = append(ctrl.recStart, nodeLeaseRec.Start)
		ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, nodeLeaseRec.ReconcileUntil)

		ctrl.nodeLeaseReqRec = reconcile.NewCore(appCtx, reconcile.Options{
			Logger:          ctrl.Log.WithName("rec:nl_req"),
			BackoffStrategy: nil,
			Workers:         1,
			RequireCache:    true,
			Handlers: reconcile.HandleFuncs{
				OnAdded: ctrl.onNodeLeaseEnsureRequest,
			},
			OnBackoffStart: nil,
			OnBackoffReset: nil,
		}.ResolveNil())
		ctrl.recStart = append(ctrl.recStart, ctrl.nodeLeaseReqRec.Start)
		ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, ctrl.nodeLeaseReqRec.ReconcileUntil)
	} else {
		getLeaseFunc = func(name string) *coordinationv1.Lease {
			return nil
		}
	}

	ctrl.virtualNodes = virtualnode.NewVirtualNodeManager(
		appCtx,
		&config.VirtualNode,
		getLeaseFunc,
		ctrl.nodeLeaseClient,
	)

	edgeDeviceRec := reconcile.NewKubeInformerReconciler(appCtx, edgeDeviceInformer, reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:ed"),
		BackoffStrategy: nil,
		Workers:         1,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded:    ctrl.onEdgeDeviceAdded,
			OnUpdated:  ctrl.onEdgeDeviceUpdated,
			OnDeleting: ctrl.onEdgeDeviceDeleting,
			OnDeleted:  ctrl.onEdgeDeviceDeleted,
		},
	})
	ctrl.recStart = append(ctrl.recStart, edgeDeviceRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, edgeDeviceRec.ReconcileUntil)

	nodeRec := reconcile.NewKubeInformerReconciler(appCtx, nodeInformer, reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:node"),
		BackoffStrategy: nil,
		Workers:         1,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded:    ctrl.onNodeAdd,
			OnUpdated:  ctrl.onNodeUpdated,
			OnDeleting: ctrl.onNodeDeleting,
			OnDeleted:  ctrl.onNodeDeleted,
		},
	})
	ctrl.recStart = append(ctrl.recStart, nodeRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, nodeRec.ReconcileUntil)

	ctrl.nodeReqRec = reconcile.NewCore(appCtx, reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:node_req"),
		BackoffStrategy: nil,
		Workers:         1,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded: ctrl.onNodeEnsureRequested,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	}.ResolveNil())
	ctrl.recStart = append(ctrl.recStart, ctrl.nodeReqRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, ctrl.nodeReqRec.ReconcileUntil)

	// start a standalone node status reconciler in addition to node reconciler to make it clear for node
	ctrl.nodeStatusRec = reconcile.NewKubeInformerReconciler(appCtx, nodeInformer, reconcile.Options{
		Logger: ctrl.Log.WithName("rec:nodestatus"),
		// no backoff
		BackoffStrategy: backoff.NewStrategy(0, 0, 1, 0),
		Workers:         1,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnUpdated: ctrl.onNodeStatusUpdated,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	})
	ctrl.recStart = append(ctrl.recStart, ctrl.nodeStatusRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, ctrl.nodeStatusRec.ReconcileUntil)

	podRec := reconcile.NewKubeInformerReconciler(appCtx, podInformer, reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:pod"),
		BackoffStrategy: nil,
		Workers:         1,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded:    ctrl.onPodAdded,
			OnUpdated:  ctrl.onPodUpdated,
			OnDeleting: ctrl.onPodDeleting,
			OnDeleted:  ctrl.onPodDeleted,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	})
	ctrl.recStart = append(ctrl.recStart, podRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, podRec.ReconcileUntil)

	ctrl.vpReqRec = reconcile.NewCore(appCtx, reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:pod_req"),
		BackoffStrategy: nil,
		Workers:         1,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded: ctrl.onVirtualPodEnsueRequested,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	}.ResolveNil())
	ctrl.recStart = append(ctrl.recStart, ctrl.vpReqRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, ctrl.vpReqRec.ReconcileUntil)

	ctrl.vnRec = reconcile.NewCore(appCtx, reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:vn"),
		BackoffStrategy: backoff.NewStrategy(time.Second, time.Minute, 2, 1),
		Workers:         config.Aranya.MaxVirtualnodeCreatingInParallel,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded:    ctrl.onEdgeDeviceCreationRequested,
			OnUpdated:  ctrl.onEdgeDeviceUpdateRequested,
			OnDeleting: ctrl.onEdgeDeviceDeletionRequested,
			OnDeleted:  ctrl.onEdgeDeviceCleanup,
		},
	}.ResolveNil())
	ctrl.recStart = append(ctrl.recStart, ctrl.vnRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, ctrl.vnRec.ReconcileUntil)

	if config.VirtualNode.Node.Storage.Enabled {
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
	}

	if config.Aranya.RunAsCloudProvider {
		var err2 error
		ctrl.nodeController, err2 = cloud.NewCloudNodeController(nodeInformerTyped, kubeClient, ctrl, 10*time.Second)
		if err2 != nil {
			return nil, fmt.Errorf("failed to create cloud node controller: %w", err2)
		}

		ctrl.nodeLifecycleController, err2 = cloud.NewCloudNodeLifecycleController(
			nodeInformerTyped, kubeClient, ctrl, 5*time.Second)
		if err2 != nil {
			return nil, fmt.Errorf("failed to create cloud lifecycle controller: %w", err2)
		}
	}

	// watch service for device connectivity (gRPC)
	if ctrl.connectivityService != "" {
		// client
		ctrl.sysSvcClient = kubeClient.CoreV1().Services(envhelper.ThisPodNS())

		// informer and sync
		setLabelSelector := newTweakListOptionsFunc(
			labels.SelectorFromSet(map[string]string{
				constant.LabelRole: constant.LabelRoleValueConnectivity,
			}),
		)

		fieldSelector := fields.OneTermEqualSelector("metadata.name", ctrl.connectivityService).String()
		informer := informerscorev1.New(sysInformerFactory, envhelper.ThisPodNS(), func(options *metav1.ListOptions) {
			setLabelSelector(options)
			options.FieldSelector = fieldSelector
		}).Services().Informer()
		ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs, informer.HasSynced)

		ctrl.listActions = append(ctrl.listActions, func() error {
			_, err2 := listerscorev1.NewServiceLister(informer.GetIndexer()).List(labels.Everything())
			if err2 != nil {
				return fmt.Errorf("failed to list services in namespace %q: %w", envhelper.ThisPodNS(), err2)
			}
			return nil
		})

		// reconciler
		serviceRec := reconcile.NewKubeInformerReconciler(appCtx, informer, reconcile.Options{
			Logger:          ctrl.Log.WithName("rec:svc"),
			BackoffStrategy: nil,
			Workers:         0,
			RequireCache:    true,
			Handlers: reconcile.HandleFuncs{
				OnAdded:    nextActionUpdate,
				OnUpdated:  ctrl.onServiceUpdated,
				OnDeleting: ctrl.onServiceDeleting,
				OnDeleted:  ctrl.onServiceDeleted,
			},
		})
		ctrl.recStart = append(ctrl.recStart, serviceRec.Start)
		ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, serviceRec.ReconcileUntil)

		ctrl.svcReqRec = reconcile.NewCore(appCtx, reconcile.Options{
			Logger:          ctrl.Log.WithName("rec:svc_req"),
			BackoffStrategy: nil,
			Workers:         1,
			RequireCache:    false,
			Handlers: reconcile.HandleFuncs{
				OnAdded: ctrl.onServiceEnsureRequested,
			},
			OnBackoffStart: nil,
			OnBackoffReset: nil,
		}.ResolveNil())
		ctrl.recStart = append(ctrl.recStart, ctrl.svcReqRec.Start)
		ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, ctrl.svcReqRec.ReconcileUntil)
	}

	if len(ctrl.podRoles) != 0 || len(ctrl.virtualPodRoles) != 0 {
		if len(ctrl.podRoles) != 0 {
			delete(ctrl.podRoles, "")
		} else {
			ctrl.podRoles = make(map[string]aranyaapi.PodRolePermissions)
		}

		if len(ctrl.virtualPodRoles) != 0 {
			delete(ctrl.virtualPodRoles, "")
		} else {
			ctrl.virtualPodRoles = make(map[string]aranyaapi.PodRolePermissions)
		}

		ctrl.roleClient = kubeClient.RbacV1().Roles(envhelper.ThisPodNS())

		ctrl.roleInformer = informersrbacv1.New(watchInformerFactory, envhelper.ThisPodNS(),
			newTweakListOptionsFunc(
				labels.SelectorFromSet(map[string]string{
					constant.LabelRole: constant.LabelRoleValuePodRole,
				}),
			),
		).Roles().Informer()

		ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs, ctrl.roleInformer.HasSynced)
		ctrl.listActions = append(ctrl.listActions, func() error {
			_, err2 := listersrbacv1.NewRoleLister(ctrl.roleInformer.GetIndexer()).List(labels.Everything())
			if err2 != nil {
				return fmt.Errorf("failed to list watched roles: %w", err2)
			}

			return nil
		})

		roleRec := reconcile.NewKubeInformerReconciler(appCtx, ctrl.roleInformer, reconcile.Options{
			Logger:          ctrl.Log.WithName("rec:role"),
			BackoffStrategy: nil,
			Workers:         1,
			RequireCache:    true,
			Handlers: reconcile.HandleFuncs{
				OnAdded:    nextActionUpdate,
				OnUpdated:  ctrl.onPodRoleUpdated,
				OnDeleting: ctrl.onPodRoleDeleting,
				OnDeleted:  ctrl.onPodRoleDeleted,
			},
			OnBackoffStart: nil,
			OnBackoffReset: nil,
		})
		ctrl.recStart = append(ctrl.recStart, roleRec.Start)
		ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, roleRec.ReconcileUntil)

		ctrl.roleReqRec = reconcile.NewCore(appCtx, reconcile.Options{
			Logger:          ctrl.Log.WithName("rec:role_req"),
			BackoffStrategy: nil,
			Workers:         1,
			RequireCache:    false,
			Handlers: reconcile.HandleFuncs{
				OnAdded: ctrl.onPodRoleEnsureRequested,
			},
			OnBackoffStart: nil,
			OnBackoffReset: nil,
		}.ResolveNil())
		ctrl.recStart = append(ctrl.recStart, ctrl.roleReqRec.Start)
		ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, ctrl.roleReqRec.ReconcileUntil)
	}

	// watch cluster roles managed by us
	if len(ctrl.nodeClusterRoles) != 0 {
		// ensure no empty name
		delete(ctrl.nodeClusterRoles, "")

		// client
		ctrl.crClient = kubeClient.RbacV1().ClusterRoles()

		// informer and sync
		ctrl.crInformer = informersrbacv1.New(clusterInformerFactory, corev1.NamespaceAll,
			newTweakListOptionsFunc(
				labels.SelectorFromSet(map[string]string{
					constant.LabelRole:      constant.LabelRoleValueNodeClusterRole,
					constant.LabelNamespace: constant.WatchNS(),
				}),
			),
		).ClusterRoles().Informer()
		ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs, ctrl.crInformer.HasSynced)

		ctrl.listActions = append(ctrl.listActions, func() error {
			_, err2 := listersrbacv1.NewClusterRoleLister(ctrl.crInformer.GetIndexer()).List(labels.Everything())
			if err2 != nil {
				return fmt.Errorf("failed to list cluster roles: %w", err2)
			}

			return nil
		})

		// reconciler for cluster role resources
		crRec := reconcile.NewKubeInformerReconciler(appCtx, ctrl.crInformer, reconcile.Options{
			Logger:          ctrl.Log.WithName("rec:cr"),
			BackoffStrategy: nil,
			Workers:         1,
			RequireCache:    true,
			Handlers: reconcile.HandleFuncs{
				OnAdded:    nextActionUpdate,
				OnUpdated:  ctrl.onNodeClusterRoleUpdated,
				OnDeleting: ctrl.onNodeClusterRoleDeleting,
				OnDeleted:  ctrl.onNodeClusterRoleDeleted,
			},
		})
		ctrl.recStart = append(ctrl.recStart, crRec.Start)
		ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, crRec.ReconcileUntil)

		ctrl.crReqRec = reconcile.NewCore(appCtx, reconcile.Options{
			Logger:          ctrl.Log.WithName("rec:cr_req"),
			BackoffStrategy: nil,
			Workers:         1,
			RequireCache:    false,
			Handlers: reconcile.HandleFuncs{
				OnAdded: ctrl.onNodeClusterRoleEnsureRequested,
			},
			OnBackoffStart: nil,
			OnBackoffReset: nil,
		}.ResolveNil())
		ctrl.recStart = append(ctrl.recStart, ctrl.crReqRec.Start)
		ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, ctrl.crReqRec.ReconcileUntil)
	}

	return ctrl, nil
}

type Controller struct {
	*manager.BaseManager

	kubeClient kubeclient.Interface

	hostNodeName      string
	hostname, hostIP  string
	hostNodeAddresses []corev1.NodeAddress
	thisPodLabels     map[string]string

	cacheSyncWaitFuncs   []kubecache.InformerSynced
	informerFactoryStart []func(<-chan struct{})
	listActions          []func() error
	recStart             []func() error
	recReconcileUntil    []func(<-chan struct{})

	// EdgeDevice management
	edgeDeviceClient   clientaranyav1a1.EdgeDeviceInterface
	edgeDeviceInformer kubecache.SharedIndexInformer

	vnRec         *reconcile.Core
	virtualNodes  *virtualnode.Manager
	sshPrivateKey []byte
	vnConfig      *conf.VirtualnodeConfig
	vnKubeconfig  *rest.Config

	// Node management
	nodeClient    clientcorev1.NodeInterface
	nodeInformer  kubecache.SharedIndexInformer
	nodeStatusRec *reconcile.KubeInformerReconciler
	nodeReqRec    *reconcile.Core

	nodeLeaseInformer kubecache.SharedIndexInformer
	nodeLeaseClient   clientcodv1.LeaseInterface
	nodeLeaseReqRec   *reconcile.Core

	// Pod management
	podClient   clientcorev1.PodInterface
	podInformer kubecache.SharedIndexInformer
	managedPods sets.String
	podsMu      *sync.RWMutex
	vpReqRec    *reconcile.Core

	watchSecretInformer kubecache.SharedIndexInformer
	watchCMInformer     kubecache.SharedIndexInformer
	watchSvcInformer    kubecache.SharedIndexInformer

	// Service management
	svcReqRec           *reconcile.Core
	sysSvcClient        clientcorev1.ServiceInterface
	connectivityService string

	// Node Certificate management
	certSecretClient clientcorev1.SecretInterface
	csrClient        *kubehelper.CertificateSigningRequestClient

	// RBAC management
	crInformer       kubecache.SharedIndexInformer
	crClient         clientrbacv1.ClusterRoleInterface
	crReqRec         *reconcile.Core
	nodeClusterRoles map[string]aranyaapi.NodeClusterRolePermissions

	roleInformer    kubecache.SharedIndexInformer
	roleClient      clientrbacv1.RoleInterface
	roleReqRec      *reconcile.Core
	podRoles        map[string]aranyaapi.PodRolePermissions
	virtualPodRoles map[string]aranyaapi.PodRolePermissions

	// Storage management
	csiDriverLister *kubehelper.CSIDriverLister

	// unused fields
	nodeController          *cloud.CloudNodeController
	nodeLifecycleController *cloud.CloudNodeLifecycleController
}

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

		for _, startReconcile := range c.recReconcileUntil {
			go startReconcile(stopCh)
		}

		if c.nodeController != nil && c.nodeLifecycleController != nil {
			go c.nodeController.Run(stopCh)
			go c.nodeLifecycleController.Run(stopCh)
		}

		err = c.virtualNodes.Start()
		if err != nil {
			return fmt.Errorf("failed to start virtual node manager: %w", err)
		}

		return nil
	})
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
	err := c.instantiationEdgeDevice(name)
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
