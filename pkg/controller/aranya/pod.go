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

package aranya

import (
	"context"
	"fmt"
	"sync"

	"arhat.dev/pkg/envhelper"
	"arhat.dev/pkg/kubehelper"
	"arhat.dev/pkg/log"
	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	kubecache "k8s.io/client-go/tools/cache"

	"arhat.dev/aranya/pkg/constant"
)

type podController struct {
	podCtx context.Context

	podLogger   log.Interface
	podClient   clientcorev1.PodInterface
	podInformer kubecache.SharedIndexInformer
	podRec      *kubehelper.KubeInformerReconciler
}

func (c *podController) init(
	appCtx context.Context,
	logger log.Interface,
	kubeClient kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) {
	c.podCtx = appCtx
	c.podLogger = logger.WithName("aranya:pod")
	c.podClient = kubeClient.CoreV1().Pods(envhelper.ThisPodNS())
	c.podInformer = informerFactory.Core().V1().Pods().Informer()

	c.podRec = kubehelper.NewKubeInformerReconciler(appCtx, c.podInformer, reconcile.Options{
		Logger:          c.podLogger,
		BackoffStrategy: nil,
		Workers:         1,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded:    nextActionUpdate,
			OnUpdated:  c.onPodUpdated,
			OnDeleting: nil,
			OnDeleted:  nil,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	})
}

func (c *podController) start() error {
	return c.podRec.Start()
}

func (c *podController) reconcile(wg *sync.WaitGroup, stop <-chan struct{}) {
	err := c.podRec.Schedule(queue.Job{
		Action: queue.ActionUpdate,
		// we are sure this pod is cached
		Key: envhelper.ThisPodNS() + "/" + envhelper.ThisPodName(),
	}, 0)
	if err != nil {
		panic(err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		c.podRec.ReconcileUntil(stop)
	}()
}

func (c *podController) onPodUpdated(oldObj, newObj interface{}) *reconcile.Result {
	// ensure no other pod has aranya leadership in this namespace
	for _, obj := range c.podInformer.GetStore().List() {
		po, ok := obj.(*corev1.Pod)
		if !ok {
			continue
		}
		po = po.DeepCopy()

		switch {
		case po.Name == envhelper.ThisPodName():
			continue
		case len(po.Labels) == 0:
			continue
		}

		leadership, ok := po.Labels[constant.LabelAranyaLeadership]
		if !ok || leadership != constant.LabelAranyaLeadershipLeader {
			continue
		}

		delete(po.Labels, constant.LabelAranyaLeadership)
		_, err := c.podClient.Update(c.podCtx, po, metav1.UpdateOptions{})
		if err != nil {
			c.podLogger.I("failed to delete pod label", log.Error(err))
			// try next time
			return &reconcile.Result{Err: err}
		}
	}

	thisPod, err := c.podClient.Get(c.podCtx, envhelper.ThisPodName(), metav1.GetOptions{})
	if err != nil {
		c.podLogger.I("failed to find aranya pod itself", log.Error(err))

		if kubeerrors.IsNotFound(err) {
			panic(fmt.Errorf("failed to find aranya pod itself: %w", err))
		}

		// try next time
		return &reconcile.Result{Err: err}
	}

	hasLeadershipLabel := true
	if len(thisPod.Labels) == 0 {
		thisPod.Labels = make(map[string]string)
		hasLeadershipLabel = false
	} else {
		leadership, ok := thisPod.Labels[constant.LabelAranyaLeadership]
		if !ok || leadership != constant.LabelAranyaLeadershipLeader {
			hasLeadershipLabel = false
		}
	}

	if hasLeadershipLabel {
		return nil
	}

	_, err = c.podClient.Update(c.podCtx, thisPod, metav1.UpdateOptions{})
	if err != nil {
		c.podLogger.I("failed to update aranya pod with leadership label", log.Error(err))

		if kubeerrors.IsNotFound(err) {
			panic(fmt.Errorf("failed to update aranya pod itself: %w", err))
		}

		// try next time
		return &reconcile.Result{Err: err}
	}

	return nil
}
