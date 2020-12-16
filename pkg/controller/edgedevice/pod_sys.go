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
	"strings"

	"arhat.dev/pkg/kubehelper"
	"arhat.dev/pkg/log"
	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/informers"
	kubeclient "k8s.io/client-go/kubernetes"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	kubecache "k8s.io/client-go/tools/cache"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
)

// sysPodController to reconcile virtual pods
type sysPodController struct {
	sysPodClient   clientcorev1.PodInterface
	sysPodInformer kubecache.SharedIndexInformer
	managedPods    sets.String
	vpReqRec       *reconcile.Core

	sysSecretInformer kubecache.SharedIndexInformer
	sysCMInformer     kubecache.SharedIndexInformer
}

func (c *sysPodController) init(
	ctrl *Controller,
	config *conf.Config,
	kubeClient kubeclient.Interface,
	sysInformerFactory informers.SharedInformerFactory,
) error {
	c.managedPods = sets.NewString()

	c.sysPodClient = kubeClient.CoreV1().Pods(constant.SysNS())

	c.sysPodInformer = sysInformerFactory.Core().V1().Pods().Informer()
	c.sysSecretInformer = sysInformerFactory.Core().V1().Secrets().Informer()
	c.sysCMInformer = sysInformerFactory.Core().V1().ConfigMaps().Informer()

	ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs,
		c.sysPodInformer.HasSynced,
		c.sysCMInformer.HasSynced,
		c.sysSecretInformer.HasSynced,
	)

	ctrl.listActions = append(ctrl.listActions, func() error {
		_, err := listerscorev1.NewPodLister(c.sysPodInformer.GetIndexer()).List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list pods in namespace %q: %w", constant.SysNS(), err)
		}

		_, err = listerscorev1.NewConfigMapLister(c.sysCMInformer.GetIndexer()).List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list configmaps in namespace %q: %w", constant.SysNS(), err)
		}

		_, err = listerscorev1.NewSecretLister(c.sysSecretInformer.GetIndexer()).List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list secrets in namespace %q: %w", constant.SysNS(), err)
		}

		return nil
	})

	virtualPodRec := kubehelper.NewKubeInformerReconciler(ctrl.Context(), c.sysPodInformer, reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:vp"),
		BackoffStrategy: nil,
		Workers:         1,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded:    ctrl.onSysPodAdded,
			OnUpdated:  ctrl.onSysPodUpdated,
			OnDeleting: ctrl.onSysPodDeleting,
			OnDeleted:  ctrl.onSysPodDeleted,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	})
	ctrl.recStart = append(ctrl.recStart, virtualPodRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, virtualPodRec.ReconcileUntil)

	c.vpReqRec = reconcile.NewCore(ctrl.Context(), reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:vp_req"),
		BackoffStrategy: nil,
		Workers:         1,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded: ctrl.onVirtualPodEnsueRequested,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	}.ResolveNil())
	ctrl.recStart = append(ctrl.recStart, c.vpReqRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, c.vpReqRec.ReconcileUntil)

	return nil
}

func (c *Controller) checkVirtualPodUpToDate(realPod, expectedPod *corev1.Pod) (match bool) {
	switch {
	case len(realPod.Spec.InitContainers) != 0:
	case len(realPod.Spec.Containers) != len(expectedPod.Spec.Containers):
	default:
		match = true
	}

	if !match {
		return false
	}

	for i, expectedCtr := range expectedPod.Spec.Containers {
		realCtr := realPod.Spec.Containers[i]

		match = false
		switch {
		case realCtr.Name != expectedCtr.Name:
		case realCtr.Image != expectedCtr.Image:
		case realCtr.Stdin != expectedCtr.Stdin:
		case realCtr.TTY != expectedCtr.TTY:
		case len(realCtr.Command) != len(expectedCtr.Command) || !containsAll(realCtr.Command, expectedCtr.Command):
		case len(realCtr.Args) != len(expectedCtr.Args) || !containsAll(realCtr.Args, expectedCtr.Args):
		default:
			match = true
		}
		if !match {
			return false
		}
	}

	return true
}

func (c *Controller) getVirtualPodNames() []string {
	result := sets.NewString()
	edgeDevices := c.edgeDeviceInformer.GetIndexer().ListKeys()
	for _, namespacedName := range edgeDevices {
		parts := strings.SplitN(namespacedName, "/", 2)
		if len(parts) != 2 {
			//panic("invalid edge device cache key")
			continue
		}

		result.Insert(parts[1])
	}

	return result.List()
}

func (c *Controller) requestVirtualPodEnsure(name string) error {
	if c.vpReqRec == nil {
		return fmt.Errorf("virtual pod ensure not supported")
	}

	c.vpReqRec.Update(name, name, name)
	err := c.vpReqRec.Schedule(queue.Job{Action: queue.ActionAdd, Key: name}, 0)
	if err != nil && !errors.Is(err, queue.ErrJobDuplicated) {
		return fmt.Errorf("failed to schedule virtual pod ensure: %w", err)
	}

	return nil
}

func (c *Controller) onVirtualPodEnsueRequested(obj interface{}) *reconcile.Result {
	var (
		name   = obj.(string)
		logger = c.Log.WithFields(log.String("name", name))
	)

	ed, ok := c.getEdgeDeviceObject(name)
	if !ok {
		return nil
	}

	logger.V("ensuring virtual pod")
	err := c.ensureVirtualPod(ed)
	if err != nil {
		logger.I("failed to ensure virtual pod")
		return &reconcile.Result{Err: err}
	}
	logger.V("ensured virtual pod")

	return nil
}

func (c *Controller) onSysPodAdded(obj interface{}) *reconcile.Result {
	var (
		pod    = obj.(*corev1.Pod)
		name   = pod.Name
		logger = c.Log.WithFields(log.String("name", name))
	)

	_, isVirtualPod := c.getEdgeDeviceObject(name)
	if !isVirtualPod {
		return nil
	}

	logger.V("requesting managed pod role update on virtual pod add")
	err := c.requestVirtualPodRoleEnsure()
	if err != nil {
		logger.I("failed to request pod role update", log.Error(err))
		return &reconcile.Result{Err: err}
	}

	return nil
}

func (c *Controller) onSysPodUpdated(oldObj, newObj interface{}) *reconcile.Result {
	var (
		name   = newObj.(*corev1.Pod).Name
		logger = c.Log.WithFields(log.String("name", name))
	)

	_, isVirtualPod := c.getEdgeDeviceObject(name)
	if !isVirtualPod {
		return nil
	}

	logger.V("virtual pod being updated")

	return nil
}

func (c *Controller) onSysPodDeleting(obj interface{}) *reconcile.Result {
	var (
		name   = obj.(*corev1.Pod).Name
		logger = c.Log.WithFields(log.String("name", name))
	)

	_, isVirtualPod := c.getEdgeDeviceObject(name)
	if !isVirtualPod {
		return nil
	}

	logger.D("deleting virtual pod immediately")
	err := c.sysPodClient.Delete(c.Context(), name, *deleteAtOnce)
	if err != nil {
		logger.I("failed to delete virtual pod immediately", log.Error(err))
		return &reconcile.Result{Err: err}
	}
	logger.V("deleted virtual pod")

	return &reconcile.Result{NextAction: queue.ActionCleanup}
}

func (c *Controller) onSysPodDeleted(obj interface{}) *reconcile.Result {
	var (
		name   = obj.(*corev1.Pod).Name
		logger = c.Log.WithFields(log.String("name", name))
	)

	ed, isVirtualPod := c.getEdgeDeviceObject(name)
	if !isVirtualPod {
		return nil
	}

	logger.D("ensuring virtual pod after being deleted")
	err := c.ensureVirtualPod(ed)
	if err != nil {
		logger.I("failed to ensure virtual pod", log.Error(err))
		return &reconcile.Result{Err: err}
	}
	logger.V("ensured virtual pod")

	return nil
}

func (c *Controller) ensureVirtualPod(edgeDevice *aranyaapi.EdgeDevice) error {
	var (
		createPod bool
		name      = edgeDevice.Name
	)

	pod, err := c.newVirtualPodForEdgeDevice(edgeDevice)
	if err != nil {
		return fmt.Errorf("failed to generate virtual pod object: %w", err)
	}

	oldPod, found := c.getSysPodObject(name)
	if found {
		if c.checkVirtualPodUpToDate(pod, oldPod) {
			return nil
		}

		// pod spec has a lot of update limits, so just recreate it if not correct
		err = c.sysPodClient.Delete(c.Context(), name, *deleteAtOnce)
		if err != nil && !kubeerrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete invalid virtual pod at once: %w", err)
		}

		createPod = true
	} else {
		createPod = true
	}

	if createPod {
		_, err = c.sysPodClient.Create(c.Context(), pod, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create virtual pod: %w", err)
		}
	}

	return nil
}

func (c *Controller) newVirtualPodForEdgeDevice(device *aranyaapi.EdgeDevice) (*corev1.Pod, error) {
	var virtualCtrs []corev1.Container

	// map devices as virtual containers
	for _, d := range device.Spec.Peripherals {
		if d.Name == "" || d.Name == constant.VirtualContainerNameHost {
			return nil, fmt.Errorf("invalid device name %q", d.Name)
		}

		if msgs := validation.IsDNS1123Label(d.Name); len(msgs) > 0 {
			return nil, fmt.Errorf("device name %q is not a valid dns label: %s", d.Name, strings.Join(msgs, ", "))
		}

		var commands []string
		for _, op := range d.Operations {
			cmd := op.PseudoCommand
			if cmd == "" {
				cmd = op.Name
			} else {
				cmd = fmt.Sprintf("%s (%s)", op.PseudoCommand, op.Name)
			}
			commands = append(commands, cmd)
		}

		virtualCtrs = append(virtualCtrs, corev1.Container{
			Name:            d.Name,
			Image:           constant.VirtualImageNamePeripheral,
			ImagePullPolicy: corev1.PullIfNotPresent,
			// just list available commands as side notes
			Command: commands,
		})
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      device.Name,
			Namespace: constant.SysNS(),
		},
		Spec: corev1.PodSpec{
			Containers: append([]corev1.Container{{
				Name:            constant.VirtualContainerNameHost,
				Image:           constant.VirtualImageNameHost,
				ImagePullPolicy: corev1.PullIfNotPresent,
				TTY:             true,
				Stdin:           true,
			}}, virtualCtrs...),
			Tolerations: []corev1.Toleration{{
				// schedule this pod anyway
				Operator: corev1.TolerationOpExists,
			}},
			NodeName: device.Name,
		},
	}, nil
}

func (c *sysPodController) getSysPodObject(name string) (*corev1.Pod, bool) {
	obj, found, err := c.sysPodInformer.GetIndexer().GetByKey(constant.SysNS() + "/" + name)
	if err != nil || !found {
		return nil, false
	}

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil, false
	}

	return pod, true
}

func (c *sysPodController) getUserPassFromSecret(name string) (username, password []byte, err error) {
	secret, ok := c.getSysSecretObject(name)
	if !ok {
		return nil, nil, fmt.Errorf("failed to get secret %q", name)
	}

	if secret.Type != corev1.SecretTypeBasicAuth {
		return nil, nil, fmt.Errorf("non basic auth secret found by userPassRef")
	}

	return secret.Data[corev1.BasicAuthUsernameKey], secret.Data[corev1.BasicAuthPasswordKey], nil
}

func (c *sysPodController) getSysSecretObject(name string) (*corev1.Secret, bool) {
	obj, found, err := c.sysSecretInformer.GetIndexer().GetByKey(constant.SysNS() + "/" + name)
	if err != nil || !found {
		return nil, false
	}

	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil, false
	}

	return secret, true
}
