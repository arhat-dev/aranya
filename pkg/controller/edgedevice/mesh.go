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
	"errors"
	"fmt"

	"arhat.dev/pkg/kubehelper"
	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	informerscorev1 "k8s.io/client-go/informers/core/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	kubecache "k8s.io/client-go/tools/cache"

	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/util/ipam"
)

// network management
type meshController struct {
	meshIPAMv4 *ipam.IPAddressManager
	meshIPAMv6 *ipam.IPAddressManager

	abbotSvcName    string
	abbotEpInformer kubecache.SharedIndexInformer
	abbotEpRec      *kubehelper.KubeInformerReconciler

	netSvcName     string
	netSvcInformer kubecache.SharedIndexInformer
	netSvcRec      *kubehelper.KubeInformerReconciler
	netEpInformer  kubecache.SharedIndexInformer
	netEpRec       *kubehelper.KubeInformerReconciler

	netReqRec *reconcile.Core
}

func (c *meshController) init(
	ctrl *Controller,
	config *conf.Config,
	kubeClient kubeclient.Interface,
	tenantInformerFactory informers.SharedInformerFactory,
) error {
	netConf := config.VirtualNode.Network
	if !netConf.Enabled {
		return nil
	}

	c.abbotSvcName = config.VirtualNode.Network.AbbotService.Name
	c.netSvcName = config.VirtualNode.Network.NetworkService.Name

	if len(c.abbotSvcName) == 0 {
		return fmt.Errorf("abbot service name not set")
	}

	if len(c.netSvcName) == 0 {
		return fmt.Errorf("network service name not set")
	}

	if blocks := netConf.Mesh.IPv4Blocks; len(blocks) != 0 {
		c.meshIPAMv4 = ipam.NewIPAddressManager()
		for _, b := range blocks {
			err := c.meshIPAMv4.AddAddressBlock(b.CIDR, b.Start, b.End)
			if err != nil {
				return fmt.Errorf("failed to add ipv4 address block %q: %s", b.CIDR, err)
			}
		}
	}

	if blocks := netConf.Mesh.IPv6Blocks; len(blocks) != 0 {
		c.meshIPAMv6 = ipam.NewIPAddressManager()
		for _, b := range blocks {
			err := c.meshIPAMv6.AddAddressBlock(b.CIDR, b.Start, b.End)
			if err != nil {
				return fmt.Errorf("failed to add ipv6 address block %q: %s", b.CIDR, err)
			}
		}
	}

	if c.meshIPAMv4 == nil && c.meshIPAMv6 == nil {
		return fmt.Errorf("no address block available for mesh network")
	}

	// watch abbot endpoints
	filterAbbotSvcName := fields.OneTermEqualSelector("metadata.name", c.abbotSvcName).String()
	c.abbotEpInformer = informerscorev1.New(tenantInformerFactory, constant.TenantNS(),
		func(options *metav1.ListOptions) {
			options.FieldSelector = filterAbbotSvcName
		},
	).Endpoints().Informer()

	ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs, c.abbotEpInformer.HasSynced)
	ctrl.listActions = append(ctrl.listActions, func() error {
		_, err := listerscorev1.NewEndpointsLister(c.abbotEpInformer.GetIndexer()).List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list abbot endpoints in namespace %q: %w", constant.TenantNS(), err)
		}

		return nil
	})

	c.abbotEpRec = kubehelper.NewKubeInformerReconciler(ctrl.Context(), c.abbotEpInformer,
		reconcile.Options{
			Logger:          ctrl.Log.WithName("rec:abbot:ep"),
			BackoffStrategy: nil,
			Workers:         0,
			RequireCache:    true,
			Handlers: reconcile.HandleFuncs{
				OnAdded:   nextActionUpdate,
				OnUpdated: ctrl.onAbbotEndpointUpdated,
			},
			OnBackoffStart: nil,
			OnBackoffReset: nil,
		},
	)

	ctrl.recStart = append(ctrl.recStart, c.abbotEpRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, c.abbotEpRec.ReconcileUntil)

	// monitor managed network service
	filterNetSvcName := fields.OneTermEqualSelector("metadata.name", c.netSvcName).String()
	c.netSvcInformer = informerscorev1.New(tenantInformerFactory, constant.TenantNS(),
		func(options *metav1.ListOptions) {
			options.FieldSelector = filterNetSvcName
		},
	).Services().Informer()

	ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs, c.netSvcInformer.HasSynced)
	ctrl.listActions = append(ctrl.listActions, func() error {
		_, err := listerscorev1.NewServiceLister(c.netSvcInformer.GetIndexer()).List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list network services in namespace %q: %w", constant.TenantNS(), err)
		}

		return nil
	})

	c.netSvcRec = kubehelper.NewKubeInformerReconciler(ctrl.Context(), c.netSvcInformer,
		reconcile.Options{
			Logger:          ctrl.Log.WithName("rec:net:svc"),
			BackoffStrategy: nil,
			Workers:         0,
			RequireCache:    true,
			Handlers: reconcile.HandleFuncs{
				OnAdded:   nextActionUpdate,
				OnUpdated: ctrl.onNetworkServiceUpdated,
			},
			OnBackoffStart: nil,
			OnBackoffReset: nil,
		},
	)
	ctrl.recStart = append(ctrl.recStart, c.netSvcRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, c.netSvcRec.ReconcileUntil)

	// monitor managed network service endpoints
	c.netEpInformer = informerscorev1.New(tenantInformerFactory, constant.TenantNS(),
		func(options *metav1.ListOptions) {
			options.FieldSelector = filterNetSvcName
		},
	).Endpoints().Informer()

	ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs, c.netEpInformer.HasSynced)
	ctrl.listActions = append(ctrl.listActions, func() error {
		_, err := listerscorev1.NewEndpointsLister(c.netEpInformer.GetIndexer()).List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list network endpoints in namespace %q: %w", constant.TenantNS(), err)
		}

		return nil
	})

	c.netEpRec = kubehelper.NewKubeInformerReconciler(ctrl.Context(), c.netEpInformer,
		reconcile.Options{
			Logger:          ctrl.Log.WithName("rec:net:ep"),
			BackoffStrategy: nil,
			Workers:         0,
			RequireCache:    true,
			Handlers: reconcile.HandleFuncs{
				OnAdded:   nextActionUpdate,
				OnUpdated: ctrl.onNetworkEndpointUpdated,
			},
			OnBackoffStart: nil,
			OnBackoffReset: nil,
		},
	)
	ctrl.recStart = append(ctrl.recStart, c.netEpRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, c.netEpRec.ReconcileUntil)

	// handle EdgeDevice add/delete
	ctrl.netReqRec = reconcile.NewCore(ctrl.Context(), reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:net:req"),
		BackoffStrategy: nil,
		Workers:         1,
		RequireCache:    false,
		Handlers: reconcile.HandleFuncs{
			OnAdded: ctrl.onMeshMemberEnsureRequested,
			OnUpdated: func(old, newObj interface{}) *reconcile.Result {
				return ctrl.onMeshMemberEnsureRequested(newObj)
			},
			OnDeleted: ctrl.onMeshMemberDeleteRequested,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	}.ResolveNil())

	ctrl.recStart = append(ctrl.recStart, ctrl.netReqRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, ctrl.netReqRec.ReconcileUntil)

	return nil
}

func (c *Controller) onAbbotEndpointUpdated(oldObj, newObj interface{}) *reconcile.Result {
	return nil
}

func (c *Controller) onNetworkServiceUpdated(oldObj, newObj interface{}) *reconcile.Result {
	return nil
}

func (c *Controller) onNetworkEndpointUpdated(oldObj, newObj interface{}) *reconcile.Result {
	return nil
}

// check existing abbot endpoints, EdgeDevices' network config (enabled or not)
func (c *Controller) onMeshMemberEnsureRequested(obj interface{}) *reconcile.Result {
	return nil
}

func (c *Controller) onMeshMemberDeleteRequested(obj interface{}) *reconcile.Result {
	return nil
}

func (c *meshController) requestNetworkEnsure(name string) error {
	if c.netReqRec == nil {
		return fmt.Errorf("network ensure not supported")
	}

	c.netReqRec.Update(name, name, name)
	err := c.netReqRec.Schedule(queue.Job{Action: queue.ActionAdd, Key: name}, 0)
	if err != nil && !errors.Is(err, queue.ErrJobDuplicated) {
		return fmt.Errorf("failed to schedule network ensure: %w", err)
	}

	return nil
}
