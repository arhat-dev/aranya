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
	"sort"

	"arhat.dev/pkg/envhelper"
	"arhat.dev/pkg/kubehelper"
	"arhat.dev/pkg/log"
	"arhat.dev/pkg/patchhelper"
	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/informers"
	informerscorev1 "k8s.io/client-go/informers/core/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	listerscorev1 "k8s.io/client-go/listers/core/v1"

	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
)

type connectivityServiceController struct {
	connSvcReqRec       *reconcile.Core
	connSvcClient       clientcorev1.ServiceInterface
	connectivityService string
}

func (c *connectivityServiceController) init(
	ctrl *Controller,
	config *conf.Config,
	kubeClient kubeclient.Interface,
	thisPodNSInformerFactory informers.SharedInformerFactory,
) error {
	c.connectivityService = config.Aranya.Managed.ConnectivityService.Name
	if c.connectivityService == "" {
		return nil
	}

	// client
	c.connSvcClient = kubeClient.CoreV1().Services(envhelper.ThisPodNS())

	// informer and sync
	setLabelSelector := newTweakListOptionsFunc(
		labels.SelectorFromSet(map[string]string{
			constant.LabelRole: constant.LabelRoleValueConnectivity,
		}),
	)

	fieldSelector := fields.OneTermEqualSelector("metadata.name", c.connectivityService).String()
	informer := informerscorev1.New(thisPodNSInformerFactory, envhelper.ThisPodNS(), func(options *metav1.ListOptions) {
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
	serviceRec := kubehelper.NewKubeInformerReconciler(ctrl.Context(), informer, reconcile.Options{
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
	ctrl.recReconcile = append(ctrl.recReconcile, serviceRec.Reconcile)

	c.connSvcReqRec = reconcile.NewCore(ctrl.Context(), reconcile.Options{
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
	ctrl.recStart = append(ctrl.recStart, c.connSvcReqRec.Start)
	ctrl.recReconcile = append(ctrl.recReconcile, c.connSvcReqRec.Reconcile)

	return nil
}

func (c *Controller) checkServiceUpToDate(
	svc *corev1.Service,
	allPorts []corev1.ServicePort,
) (hasUnknownPort, upToDate bool) {
	// assume the best condition (no action required)
	hasUnknownPort, upToDate = false, true

	if c.connectivityService != svc.Name {
		// not managed by us
		return
	}

	if len(svc.Labels) == 0 || svc.Labels[constant.LabelRole] != constant.LabelRoleValueConnectivity {
		upToDate = false
	}

	if len(svc.Spec.Ports) != len(allPorts) {
		upToDate = false
	}

	requiredPorts := make(map[string]int32)
	for _, p := range allPorts {
		requiredPorts[p.Name] = p.Port
	}

	for _, p := range svc.Spec.Ports {
		rp, ok := requiredPorts[p.Name]
		if !ok {
			// this port is unknown to us
			hasUnknownPort = true
		}

		if p.Port != rp || p.TargetPort.IntValue() != int(rp) {
			upToDate = false
		}

		delete(requiredPorts, p.Name)
	}

	if len(requiredPorts) > 0 {
		upToDate = false
	}

	return
}

func (c *connectivityServiceController) requestConnectivityServiceEnsure() error {
	switch {
	case c.connectivityService == "":
		return nil
	case c.connSvcReqRec == nil:
		return nil
	}

	err := c.connSvcReqRec.Schedule(queue.Job{Action: queue.ActionAdd, Key: ""}, 0)
	if err != nil && !errors.Is(err, queue.ErrJobDuplicated) {
		return fmt.Errorf("failed to schedule connectivity service ensure: %w", err)
	}

	return nil
}

func (c *Controller) onServiceEnsureRequested(_ interface{}) *reconcile.Result {
	if c.connectivityService == "" {
		return nil
	}

	err := c.ensureConnectivityService()
	if err != nil {
		c.Log.I("failed to ensure connectivity service", log.String("name", c.connectivityService), log.Error(err))
		return &reconcile.Result{Err: err}
	}

	return nil
}

func (c *Controller) onServiceUpdated(oldObj, newObj interface{}) *reconcile.Result {
	var (
		err    error
		newSvc = newObj.(*corev1.Service)
		name   = newSvc.Name
		logger = c.Log.WithFields(log.String("name", name))
	)

	logger.V("service updated")
	if c.connectivityService != name {
		logger.V("ignored")
		return nil
	}

	logger.D("checking if service up to date")
	hasUnknownPort, upToDate := c.checkServiceUpToDate(newSvc, c.getAllRequiredServicePorts())
	if upToDate && !hasUnknownPort {
		logger.D("service already up to date")
		return nil
	}

	logger.I("service outdated, updating")
	err = c.ensureConnectivityService()
	if err != nil {
		logger.I("failed to ensure service up to date", log.Error(err))
		return &reconcile.Result{Err: err}
	}

	return nil
}

func (c *Controller) onServiceDeleting(obj interface{}) *reconcile.Result {
	// TODO: add finalizer
	err := c.connSvcClient.Delete(c.Context(), obj.(*corev1.Service).Name, *deleteAtOnce)
	if err != nil && !kubeerrors.IsNotFound(err) {
		return &reconcile.Result{Err: err}
	}

	return &reconcile.Result{NextAction: queue.ActionCleanup}
}

func (c *Controller) onServiceDeleted(obj interface{}) *reconcile.Result {
	var (
		name   = obj.(*corev1.Service).Name
		logger = c.Log.WithFields(log.String("name", name))
	)

	logger.D("service deleted")
	if c.connectivityService != name {
		return nil
	}

	// this svc should present
	logger.I("deleted service is required, recreating")
	err := c.ensureConnectivityService()
	if err != nil {
		logger.I("failed to ensure required service for connectivity", log.Error(err))
		return &reconcile.Result{Err: err}
	}

	return nil
}

func (c *Controller) ensureConnectivityService() error {
	if c.connectivityService == "" {
		return nil
	}

	var (
		create bool
		logger = c.Log.WithFields(log.String("name", c.connectivityService))
	)

	svc := c.newServiceForAllEdgeDevices()

	// get latest service object, not from informer cache
	oldSvc, err := c.connSvcClient.Get(c.Context(), c.connectivityService, metav1.GetOptions{})
	if err == nil {
		logger.D("found old svc, checking if up to date")

		hasUnknownPort, upToDate := c.checkServiceUpToDate(oldSvc, svc.Spec.Ports)

		clone := oldSvc.DeepCopy()

		// ensure labels contain role label
		if clone.Labels == nil {
			clone.Labels = svc.Labels
		} else {
			for k, v := range svc.Labels {
				clone.Labels[k] = v
			}
		}
		clone.Spec.Ports = svc.Spec.Ports
		clone.Spec.Selector = svc.Spec.Selector

		if hasUnknownPort {
			// update to remove unknown port(s)
			oldSvc, err = c.connSvcClient.Update(c.Context(), clone, metav1.UpdateOptions{})
		} else if !upToDate {
			// patch to update ports
			err = patchhelper.TwoWayMergePatch(oldSvc, clone, &corev1.Service{}, func(patchData []byte) error {
				oldSvc, err = c.connSvcClient.Patch(
					c.Context(),
					c.connectivityService,
					types.StrategicMergePatchType,
					patchData,
					metav1.PatchOptions{},
				)
				return err
			})
		}

		if err != nil {
			if kubeerrors.IsConflict(err) {
				return err
			}

			logger.I("failed to update svc object, will create after delete", log.Error(err))
			err = c.connSvcClient.Delete(c.Context(), c.connectivityService, *deleteAtOnce)
			if err != nil && !kubeerrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete old service: %w", err)
			}

			create = true
		}
	} else {
		if !kubeerrors.IsNotFound(err) {
			return fmt.Errorf("failed to get old service: %w", err)
		}

		// old svc not found
		create = true
	}

	if create && len(svc.Spec.Ports) != 0 {
		logger.D("creating connectivity service object")
		_, err = c.connSvcClient.Create(c.Context(), svc, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create connectivity service object: %w", err)
		}
	}

	return nil
}

func (c *Controller) getAllRequiredServicePorts() []corev1.ServicePort {
	var allPorts []corev1.ServicePort
	for n, v := range c.virtualNodes.All() {
		listener := v.ConnectivityServerListener()
		if listener == nil {
			continue
		}

		port, _ := getListenerPort(listener)
		allPorts = append(allPorts, corev1.ServicePort{
			Name:       n,
			Protocol:   corev1.ProtocolTCP,
			Port:       port,
			TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: port},
		})
	}

	sort.Slice(allPorts, func(i, j int) bool {
		return allPorts[i].Name < allPorts[j].Name
	})

	return allPorts
}

func (c *Controller) newServiceForAllEdgeDevices() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.connectivityService,
			Namespace: envhelper.ThisPodNS(),
			Labels:    map[string]string{constant.LabelRole: constant.LabelRoleValueConnectivity},
		},
		Spec: corev1.ServiceSpec{
			Ports: c.getAllRequiredServicePorts(),
		},
	}
}
