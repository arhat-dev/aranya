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
	"arhat.dev/pkg/log"
	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	"arhat.dev/aranya/pkg/controller/edgedevice"
)

func NewController(
	appCtx context.Context,
	logger log.Interface,
	kubeClient kubernetes.Interface,
	edCtrl *edgedevice.Controller,
) (*Controller, error) {
	informerFactory := informers.NewSharedInformerFactoryWithOptions(
		kubeClient, 0, informers.WithNamespace(envhelper.ThisPodNS()),
	)

	ctrl := &Controller{
		logger: logger,
		edCtrl: edCtrl,
	}

	ctrl.podController.init(appCtx, logger, kubeClient, informerFactory)

	return ctrl, nil
}

// Controller to reconcile aranya self related resouces in POD_NAMESPACE when elected as leader
type Controller struct {
	logger log.Interface
	edCtrl *edgedevice.Controller

	podController
}

func (c *Controller) Start() error {
	err := c.podController.start()
	if err != nil {
		return fmt.Errorf("failed to start aranya pod controller: %w", err)
	}

	err = c.edCtrl.Start()
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) OnElected(ctx context.Context) {
	c.logger.I("elected as leader")

	wg := new(sync.WaitGroup)

	c.podController.reconcile(wg, ctx.Done())
	c.edCtrl.Reconcile(wg, ctx.Done())

	wg.Wait()
}

func (c *Controller) OnEjected() {
	c.logger.E("lost leader-election")
}

func (c *Controller) OnNewLeader(podName string) {
	// TODO: distributed EdgeDevice creation with coordination with leader
}

func nextActionUpdate(obj interface{}) *reconcile.Result {
	return &reconcile.Result{NextAction: queue.ActionUpdate}
}
