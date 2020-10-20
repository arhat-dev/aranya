package edgedevice

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"arhat.dev/pkg/log"
	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
)

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

// nolint:unparam
func (c *Controller) addManagedPod(name string) (added bool) {
	c.podsMu.Lock()
	defer c.podsMu.Unlock()

	oldLen := c.managedPods.Len()
	c.managedPods.Insert(name)

	return oldLen < c.managedPods.Len()
}

func (c *Controller) removeManagedPod(name string) (removed bool) {
	c.podsMu.Lock()
	defer c.podsMu.Unlock()

	oldLen := c.managedPods.Len()
	c.managedPods.Insert(name)
	return oldLen > c.managedPods.Len()
}

func (c *Controller) getManagedPodNames() []string {
	c.podsMu.RLock()
	defer c.podsMu.RUnlock()

	return c.managedPods.List()
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

func (c *Controller) onPodAdded(obj interface{}) *reconcile.Result {
	var (
		pod    = obj.(*corev1.Pod)
		name   = pod.Name
		logger = c.Log.WithFields(log.String("name", name))
	)

	_, managed := c.getEdgeDeviceObject(name)
	if managed {
		logger.V("virtual pod added")
	} else {
		_, managed = c.getEdgeDeviceObject(pod.Spec.NodeName)
		if managed {
			c.addManagedPod(name)
		}
	}

	if managed {
		logger.V("requesting managed pod role update")
		err := c.requestPodRoleEnsure()
		if err != nil {
			logger.I("failed to request pod role update", log.Error(err))
			return &reconcile.Result{Err: err}
		}
	}

	return &reconcile.Result{NextAction: queue.ActionUpdate, ScheduleAfter: 3 * time.Second}
}

func (c *Controller) onPodUpdated(oldObj, newObj interface{}) *reconcile.Result {
	var (
		oldPod, newPod = oldObj.(*corev1.Pod), newObj.(*corev1.Pod)
		name           = newPod.Name
		logger         = c.Log.WithFields(log.String("name", name))
		isVirtualPod   = false
	)

	ed, managed := c.getEdgeDeviceObject(name)
	if managed {
		logger.V("virtual pod being updated")
		isVirtualPod = true
	} else {
		ed, managed = c.getEdgeDeviceObject(newPod.Spec.NodeName)
	}

	if !managed {
		// new host node is not managed by us

		if oldPod.Spec.NodeName == newPod.Spec.NodeName {
			// host node not changed, ignore it
			return nil
		}

		oldHost, ok := c.virtualNodes.Get(oldPod.Spec.NodeName)
		if !ok {
			// old host node is also not managed by us, so ignore it, just some random pod updated
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

	if isVirtualPod {
		// TODO: handle virtual pod update (TBD)
		//		 virtual pod is managed by the virtual node pod manager
		return nil
	}

	logger = logger.WithFields(log.String("node", newPod.Spec.NodeName))

	c.addManagedPod(name)

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

func (c *Controller) onPodDeleting(obj interface{}) *reconcile.Result {
	var (
		pod          = obj.(*corev1.Pod)
		name         = pod.Name
		node         = pod.Spec.NodeName
		logger       = c.Log.WithFields(log.String("name", name), log.String("node", node))
		isVirtualPod = false
	)

	ed, managed := c.getEdgeDeviceObject(name)
	if managed {
		logger.V("virtual pod being deleted")
		isVirtualPod = true
	} else {
		ed, managed = c.getEdgeDeviceObject(pod.Spec.NodeName)
	}

	if !managed {
		// not managed by us
		return nil
	}

	if isVirtualPod {
		logger.D("deleting virtual pod immediately")
		err := c.podClient.Delete(c.Context(), name, *deleteAtOnce)
		if err != nil {
			logger.I("failed to delete virtual pod immediately", log.Error(err))
			return &reconcile.Result{Err: err}
		}
		logger.V("deleted virtual pod")

		return &reconcile.Result{NextAction: queue.ActionCleanup}
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

func (c *Controller) onPodDeleted(obj interface{}) *reconcile.Result {
	var (
		pod          = obj.(*corev1.Pod)
		name         = pod.Name
		node         = pod.Spec.NodeName
		logger       = c.Log.WithFields(log.String("name", name), log.String("node", node))
		isVirtualPod = false
	)

	ed, managed := c.getEdgeDeviceObject(name)
	if managed {
		logger.V("virtual pod deleted")
		isVirtualPod = true
	} else {
		ed, managed = c.getEdgeDeviceObject(pod.Spec.NodeName)
	}

	if !managed {
		// not managed by us
		return nil
	}

	if c.removeManagedPod(name) {
		logger.V("requesting pod role update")
		err := c.requestPodRoleEnsure()
		if err != nil {
			logger.I("failed to request pod role update", log.Error(err))
			return &reconcile.Result{Err: err}
		}
	}

	if isVirtualPod {
		logger.D("ensuring virtual pod after being deleted")
		err := c.ensureVirtualPod(ed)
		if err != nil {
			logger.I("failed to ensure virtual pod", log.Error(err))
			return &reconcile.Result{Err: err}
		}
		logger.V("ensured virtual pod")

		return nil
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

func (c *Controller) ensureVirtualPod(edgeDevice *aranyaapi.EdgeDevice) error {
	var (
		createPod bool
		name      = edgeDevice.Name
	)

	pod, err := c.newVirtualPodForEdgeDevice(edgeDevice)
	if err != nil {
		return fmt.Errorf("failed to generate virtual pod object: %w", err)
	}

	oldPod, found := c.getWatchPodObject(name)
	if found {
		if c.checkVirtualPodUpToDate(pod, oldPod) {
			return nil
		}

		// pod spec has a lot of update limits, so just recreate it if not correct
		err = c.podClient.Delete(c.Context(), name, *deleteAtOnce)
		if err != nil && !kubeerrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete invalid virtual pod at once: %w", err)
		}

		createPod = true
	} else {
		createPod = true
	}

	if createPod {
		_, err = c.podClient.Create(c.Context(), pod, metav1.CreateOptions{})
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
			Image:           constant.VirtualImageNameDevice,
			ImagePullPolicy: corev1.PullIfNotPresent,
			// just list available commands as side notes
			Command: commands,
		})
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      device.Name,
			Namespace: constant.WatchNS(),
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
