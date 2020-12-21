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
	"fmt"

	"arhat.dev/pkg/envhelper"
	"arhat.dev/pkg/kubehelper"
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
)

type storageServiceController struct {
	storageSvcName string

	storageSvcInformer kubecache.SharedIndexInformer
	storageEpInformer  kubecache.SharedIndexInformer

	storageSvcRec *kubehelper.KubeInformerReconciler
	storageEpRec  *kubehelper.KubeInformerReconciler
}

func (c *storageServiceController) init(
	ctrl *Controller,
	config *conf.Config,
	kubeClient kubeclient.Interface,
	thisPodNSInformerFactory informers.SharedInformerFactory,
) error {
	c.storageSvcName = config.Aranya.Managed.StorageService.Name
	if config.VirtualNode.Storage.Enabled || len(c.storageSvcName) == 0 {
		return nil
	}

	c.storageSvcInformer = informerscorev1.New(thisPodNSInformerFactory, envhelper.ThisPodNS(),
		func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector(
				"metadata.name", c.storageSvcName,
			).String()
		},
	).Services().Informer()

	ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs, c.storageSvcInformer.HasSynced)
	ctrl.listActions = append(ctrl.listActions, func() error {
		_, err := listerscorev1.NewServiceLister(c.storageSvcInformer.GetIndexer()).List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list storage service in namespace %q: %w", envhelper.ThisPodNS(), err)
		}

		return nil
	})

	c.storageEpInformer = informerscorev1.New(thisPodNSInformerFactory, envhelper.ThisPodNS(),
		func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector(
				"metadata.name", c.storageSvcName,
			).String()
		},
	).Endpoints().Informer()

	ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs, c.storageEpInformer.HasSynced)
	ctrl.listActions = append(ctrl.listActions, func() error {
		_, err := listerscorev1.NewEndpointsLister(c.storageEpInformer.GetIndexer()).List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list storage endpoints in namespace %q: %w", envhelper.ThisPodNS(), err)
		}

		return nil
	})

	c.storageSvcRec = kubehelper.NewKubeInformerReconciler(ctrl.Context(), c.storageSvcInformer, reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:storage:svc"),
		BackoffStrategy: nil,
		Workers:         0,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded:    nextActionUpdate,
			OnUpdated:  c.onSvcUpdated,
			OnDeleting: c.onSvcDeleting,
			OnDeleted:  c.onSvcDeleted,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	})

	ctrl.recStart = append(ctrl.recStart, c.storageSvcRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, c.storageSvcRec.ReconcileUntil)

	c.storageEpRec = kubehelper.NewKubeInformerReconciler(ctrl.Context(), c.storageEpInformer, reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:storage:ep"),
		BackoffStrategy: nil,
		Workers:         0,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded:    nextActionUpdate,
			OnUpdated:  c.onEndpointsUpdated,
			OnDeleting: c.onEndpointsDeleting,
			OnDeleted:  c.onEndpointsDeleted,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	})

	ctrl.recStart = append(ctrl.recStart, c.storageEpRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, c.storageEpRec.ReconcileUntil)

	return nil
}

func (c *storageServiceController) onSvcUpdated(oldObj, newObj interface{}) *reconcile.Result {
	return nil
}

func (c *storageServiceController) onSvcDeleting(obj interface{}) *reconcile.Result {
	return nil
}

func (c *storageServiceController) onSvcDeleted(obj interface{}) *reconcile.Result {
	return nil
}

func (c *storageServiceController) onEndpointsUpdated(oldObj, newObj interface{}) *reconcile.Result {
	return nil
}

func (c *storageServiceController) onEndpointsDeleting(obj interface{}) *reconcile.Result {
	return nil
}

func (c *storageServiceController) onEndpointsDeleted(obj interface{}) *reconcile.Result {
	return nil
}
