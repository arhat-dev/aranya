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
	"fmt"
	"sync"
	"time"

	"arhat.dev/pkg/kubehelper"
	"arhat.dev/pkg/log"
	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	kubeclient "k8s.io/client-go/kubernetes"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	kubecache "k8s.io/client-go/tools/cache"

	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
)

type tenantPodController struct {
	tenantPodCtx context.Context

	tenantPodClient   clientcorev1.PodInterface
	tenantPodInformer kubecache.SharedIndexInformer
	tenantPods        sets.String
	tenantPodsMu      *sync.RWMutex

	tenantSecretInformer kubecache.SharedIndexInformer
	tenantCMInformer     kubecache.SharedIndexInformer
	tenantSvcInformer    kubecache.SharedIndexInformer
}

func (c *tenantPodController) init(
	ctrl *Controller,
	config *conf.Config,
	kubeClient kubeclient.Interface,
	tenantInformerFactory informers.SharedInformerFactory,
) error {
	c.tenantPodCtx = ctrl.Context()
	c.tenantPods = sets.NewString()
	c.tenantPodsMu = new(sync.RWMutex)

	c.tenantPodClient = kubeClient.CoreV1().Pods(constant.TenantNS())

	c.tenantPodInformer = tenantInformerFactory.Core().V1().Pods().Informer()
	c.tenantSecretInformer = tenantInformerFactory.Core().V1().Secrets().Informer()
	c.tenantCMInformer = tenantInformerFactory.Core().V1().ConfigMaps().Informer()
	c.tenantSvcInformer = tenantInformerFactory.Core().V1().Services().Informer()

	ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs,
		c.tenantPodInformer.HasSynced,
		c.tenantCMInformer.HasSynced,
		c.tenantSecretInformer.HasSynced,
		c.tenantSvcInformer.HasSynced,
	)

	ctrl.listActions = append(ctrl.listActions, func() error {
		_, err := listerscorev1.NewPodLister(c.tenantPodInformer.GetIndexer()).List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list pods in namespace %q: %w", constant.TenantNS(), err)
		}

		_, err = listerscorev1.NewConfigMapLister(c.tenantCMInformer.GetIndexer()).List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list configmaps in namespace %q: %w", constant.TenantNS(), err)
		}

		_, err = listerscorev1.NewSecretLister(c.tenantSecretInformer.GetIndexer()).List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list secrets in namespace %q: %w", constant.TenantNS(), err)
		}

		_, err = listerscorev1.NewServiceLister(c.tenantSvcInformer.GetIndexer()).List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list services in namespace %q: %w", constant.TenantNS(), err)
		}

		return nil
	})

	podRec := kubehelper.NewKubeInformerReconciler(ctrl.Context(), c.tenantPodInformer, reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:pod"),
		BackoffStrategy: nil,
		Workers:         0,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded:    ctrl.onTenantPodAdded,
			OnUpdated:  ctrl.onTenantPodUpdated,
			OnDeleting: ctrl.onTenantPodDeleting,
			OnDeleted:  ctrl.onTenantPodDeleted,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	})
	ctrl.recStart = append(ctrl.recStart, podRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, podRec.ReconcileUntil)

	return nil
}

// nolint:unparam
func (c *tenantPodController) addTenantPod(name string) (added bool) {
	c.tenantPodsMu.Lock()
	defer c.tenantPodsMu.Unlock()

	oldLen := c.tenantPods.Len()
	c.tenantPods.Insert(name)

	return oldLen < c.tenantPods.Len()
}

func (c *tenantPodController) removeTenantPod(name string) (removed bool) {
	c.tenantPodsMu.Lock()
	defer c.tenantPodsMu.Unlock()

	oldLen := c.tenantPods.Len()
	c.tenantPods.Insert(name)
	return oldLen > c.tenantPods.Len()
}

func (c *tenantPodController) getTenantPodNames() []string {
	c.tenantPodsMu.RLock()
	defer c.tenantPodsMu.RUnlock()

	return c.tenantPods.List()
}

func (c *Controller) onTenantPodAdded(obj interface{}) *reconcile.Result {
	var (
		pod    = obj.(*corev1.Pod)
		name   = pod.Name
		logger = c.Log.WithFields(log.String("name", name))
	)

	_, isVirtualPod := c.getEdgeDeviceObject(name)
	if isVirtualPod {
		return nil
	}

	_, managed := c.getEdgeDeviceObject(pod.Spec.NodeName)
	if !managed {
		return nil
	}

	c.addTenantPod(name)
	logger.V("requesting managed pod role update")
	err := c.requestTenantPodRoleEnsure()
	if err != nil {
		logger.I("failed to request pod role update", log.Error(err))
		return &reconcile.Result{Err: err}
	}

	return &reconcile.Result{NextAction: queue.ActionUpdate, ScheduleAfter: 3 * time.Second}
}

func (c *Controller) onTenantPodUpdated(oldObj, newObj interface{}) *reconcile.Result {
	var (
		oldPod, newPod = oldObj.(*corev1.Pod), newObj.(*corev1.Pod)
		name           = newPod.Name
		logger         = c.Log.WithFields(log.String("name", name))
	)

	_, isVirtualPod := c.getEdgeDeviceObject(name)
	if isVirtualPod {
		return nil
	}

	ed, managed := c.getEdgeDeviceObject(newPod.Spec.NodeName)
	if !managed {
		// new host node is not managed by us

		if oldPod.Spec.NodeName == newPod.Spec.NodeName {
			// host node not changed, ignore it
			return nil
		}

		oldHost, ok := c.virtualNodes.Get(oldPod.Spec.NodeName)
		if !ok {
			// old host node was also not managed by us, so ignore it, just some random pod updated
			return nil
		}

		// this pod has a new host different from than the old one and the old host node
		// is currently managed by us

		logger = logger.WithFields(
			log.String("node", oldPod.Spec.NodeName),
			log.String("newNode", newPod.Spec.NodeName),
		)

		logger.D("scheduling pod deletion due to node change")
		err := oldHost.SchedulePodJob(queue.ActionDelete, oldPod, newPod)
		if err != nil {
			logger.I("failed to schedule pod deletion on old host node", log.Error(err))
			return &reconcile.Result{Err: err}
		}

		return nil
	}

	logger = logger.WithFields(log.String("node", newPod.Spec.NodeName))

	c.addTenantPod(name)

	host, ok := c.virtualNodes.Get(ed.Name)
	if !ok {
		logger.D("failed to find required virtual node")
		return &reconcile.Result{Err: fmt.Errorf("failed to find virtual node")}
	}

	logger.V("pod being updated")
	err := host.SchedulePodJob(queue.ActionUpdate, oldPod, newPod)
	if err != nil {
		logger.I("failed to schedule pod update", log.Error(err))
		return &reconcile.Result{Err: err}
	}
	logger.V("scheduled pod update")

	return nil
}

func (c *Controller) onTenantPodDeleting(obj interface{}) *reconcile.Result {
	var (
		pod    = obj.(*corev1.Pod)
		name   = pod.Name
		node   = pod.Spec.NodeName
		logger = c.Log.WithFields(log.String("name", name), log.String("node", node))
	)

	_, isVirtualPod := c.getEdgeDeviceObject(name)
	if isVirtualPod {
		return nil
	}

	ed, managed := c.getEdgeDeviceObject(pod.Spec.NodeName)
	if !managed {
		// not managed by us
		return nil
	}

	host, ok := c.virtualNodes.Get(ed.Name)
	if !ok {
		logger.D("failed to find required virtual node")
		return &reconcile.Result{Err: fmt.Errorf("failed to find virtual node")}
	}

	logger.I("scheduling pod deletion")
	err := host.SchedulePodJob(queue.ActionDelete, nil, pod)
	if err != nil {
		logger.I("failed to schedule pod clean up", log.Error(err))
		return &reconcile.Result{Err: err}
	}
	logger.V("scheduled pod cleanup")

	return nil
}

func (c *Controller) onTenantPodDeleted(obj interface{}) *reconcile.Result {
	var (
		pod    = obj.(*corev1.Pod)
		name   = pod.Name
		node   = pod.Spec.NodeName
		logger = c.Log.WithFields(log.String("name", name), log.String("node", node))
	)

	_, isVirtualPod := c.getEdgeDeviceObject(name)
	if isVirtualPod {
		return nil
	}

	ed, managed := c.getEdgeDeviceObject(pod.Spec.NodeName)
	if !managed {
		// not managed by us
		return nil
	}

	if c.removeTenantPod(name) {
		logger.V("requesting pod role update")
		err := c.requestTenantPodRoleEnsure()
		if err != nil {
			logger.I("failed to request pod role update", log.Error(err))
			return &reconcile.Result{Err: err}
		}
	}

	host, ok := c.virtualNodes.Get(ed.Name)
	if !ok {
		logger.D("failed to find required virtual node")
		return &reconcile.Result{Err: fmt.Errorf("failed to find virtual node")}
	}

	logger.V("scheduling pod cleanup")
	err := host.SchedulePodJob(queue.ActionCleanup, nil, pod)
	if err != nil {
		logger.I("failed to schedule pod clean up", log.Error(err))
		return &reconcile.Result{Err: err}
	}
	logger.V("scheduled pod cleanup")

	return nil
}

func (c *tenantPodController) getTenantConfigMapObject(name string) (*corev1.ConfigMap, bool) {
	obj, found, err := c.tenantCMInformer.GetIndexer().GetByKey(constant.TenantNS() + "/" + name)
	if err != nil || !found {
		return nil, false
	}

	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return nil, false
	}

	return cm, true
}

func (c *tenantPodController) getTenantSecretObject(name string) (*corev1.Secret, bool) {
	obj, found, err := c.tenantSecretInformer.GetIndexer().GetByKey(constant.TenantNS() + "/" + name)
	if err != nil || !found {
		return nil, false
	}

	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil, false
	}

	return secret, true
}

func (c *tenantPodController) listTenantServiceObjects() []*corev1.Service {
	var ret []*corev1.Service

	for _, obj := range c.tenantSvcInformer.GetStore().List() {
		svc, ok := obj.(*corev1.Service)
		if !ok {
			continue
		}

		ret = append(ret, svc)
	}

	return ret
}

func (c *tenantPodController) getTenantPodsForNode(name string) []*corev1.Pod {
	var result []*corev1.Pod
	for _, obj := range c.tenantPodInformer.GetStore().List() {
		po, ok := obj.(*corev1.Pod)
		if !ok {
			continue
		}

		if po.Spec.NodeName == name {
			result = append(result, po)
		}
	}

	return result
}

func (c *tenantPodController) getTenantPodObject(name string) (*corev1.Pod, bool) {
	obj, found, err := c.tenantPodInformer.GetIndexer().GetByKey(constant.TenantNS() + "/" + name)
	if err != nil || !found {
		pod, err := c.tenantPodClient.Get(c.tenantPodCtx, name, metav1.GetOptions{})
		if err != nil {
			return nil, false
		}

		return pod, true
	}

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil, false
	}

	return pod, true
}
