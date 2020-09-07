package edgedevice

import (
	"errors"
	"fmt"

	"arhat.dev/pkg/log"
	"arhat.dev/pkg/patchhelper"
	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"arhat.dev/aranya/pkg/constant"
)

func (c *Controller) checkNodeLeaseUpToDate(
	lease *coordinationv1.Lease,
	nodeMeta metav1.ObjectMeta,
) (ownerOk, generalOk bool) {
	_, ignore := c.doNodeResourcePreCheck(lease.Name)
	if ignore {
		return true, true
	}

	if len(lease.ObjectMeta.OwnerReferences) != 1 {
		return false, false
	}

	ref := lease.ObjectMeta.OwnerReferences[0]
	switch {
	case ref.Name != lease.Name:
	case ref.Controller != nil:
	case ref.UID != nodeMeta.UID:
	case ref.Kind != "Node":
	case ref.APIVersion != "v1":
	default:
		ownerOk = true
	}

	if len(lease.Labels) == 0 {
		return ownerOk, false
	}

	switch {
	case lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity != nodeMeta.Name:
	case lease.Spec.LeaseDurationSeconds == nil ||
		*lease.Spec.LeaseDurationSeconds != int32(c.vnConfig.Node.Lease.Duration.Seconds()):
	case lease.Labels[constant.LabelRole] != constant.LabelRoleValueNodeLease:
	case lease.Labels[constant.LabelNamespace] != constant.WatchNS():
	default:
		generalOk = true
	}

	return
}

func (c *Controller) requestNodeLeaseEnsure(name string) error {
	if c.nodeLeaseReqRec == nil {
		return nil
	}

	c.nodeLeaseReqRec.Update(name, name, name)
	err := c.nodeLeaseReqRec.Schedule(queue.Job{Action: queue.ActionAdd, Key: name}, 0)
	if err != nil && !errors.Is(err, queue.ErrJobDuplicated) {
		return fmt.Errorf("failed to schedule node lease ensure")
	}

	return nil
}

func (c *Controller) onNodeLeaseEnsureRequest(obj interface{}) *reconcile.Result {
	var (
		name   = obj.(string)
		logger = c.Log.WithFields(log.String("name", name))
	)

	node, ok := c.getNodeObject(name)
	if !ok {
		_, shouldEnsure := c.getEdgeDeviceObject(name)
		if !shouldEnsure {
			return nil
		}

		return &reconcile.Result{Err: fmt.Errorf("waiting for node object")}
	}

	logger.V("ensuring node lease")
	err := c.ensureNodeLease(node.ObjectMeta)
	if err != nil {
		logger.I("failed to ensure node lease")
		return &reconcile.Result{Err: err}
	}

	return nil
}

func (c *Controller) onNodeLeaseUpdated(oldObj, newObj interface{}) *reconcile.Result {
	var (
		lease  = newObj.(*coordinationv1.Lease)
		name   = lease.Name
		logger = c.Log.WithFields(log.String("name", name))
	)

	_, ignore := c.doNodeResourcePreCheck(name)
	if ignore {
		return nil
	}

	// find node since the lease object requires node as owner
	node, found := c.getNodeObject(name)
	if !found {
		return nil
	}

	ownerOk, generalOk := c.checkNodeLeaseUpToDate(lease, node.ObjectMeta)
	if ownerOk && generalOk {
		return nil
	}

	logger.D("ensuring node lease resource up to date")
	err := c.ensureNodeLease(node.ObjectMeta)
	if err != nil {
		logger.I("failed to ensure node lease resource up to date", log.Error(err))
		return &reconcile.Result{Err: err}
	}
	logger.V("ensured node lease resource up to date")

	return nil
}

func (c *Controller) onNodeLeaseDeleting(obj interface{}) *reconcile.Result {
	var (
		lease  = obj.(*coordinationv1.Lease)
		name   = lease.Name
		logger = c.Log.WithFields(log.String("name", name))
	)

	logger.V("deleting node lease marked to be deleted immediately")
	err := c.nodeLeaseClient.Delete(c.Context(), name, *deleteAtOnce)
	if err != nil && !kubeerrors.IsNotFound(err) {
		logger.I("failed to delete node lease, will retry")
		return &reconcile.Result{Err: err}
	}
	logger.V("deleted node lease")

	return nil
}

func (c *Controller) onNodeLeaseDeleted(obj interface{}) *reconcile.Result {
	var (
		lease  = obj.(*coordinationv1.Lease)
		name   = lease.Name
		logger = c.Log.WithFields(log.String("name", name))
	)

	_, ignore := c.doNodeResourcePreCheck(name)
	if ignore {
		return nil
	}

	// find node since the lease object requires node as owner
	node, found := c.getNodeObject(name)
	if !found {
		return nil
	}

	logger.D("ensuring deleted but required node lease resource")
	err := c.ensureNodeLease(node.ObjectMeta)
	if err != nil {
		logger.I("failed to ensure node lease resource", log.Error(err))
		return &reconcile.Result{Err: err}
	}
	logger.V("ensured node lease resource")

	return nil
}

func (c *Controller) ensureNodeLease(nodeMeta metav1.ObjectMeta) error {
	if !c.vnConfig.Node.Lease.Enabled {
		return nil
	}

	var (
		create bool
		err    error
		name   = nodeMeta.Name
		logger = c.Log.WithFields(log.String("name", name))
	)

	lease := c.newLeaseForNode(nodeMeta)

	oldLease, found := c.getNodeLeaseObject(name)
	if found {
		ownerOk, ok := c.checkNodeLeaseUpToDate(oldLease, nodeMeta)
		if ownerOk && ok {
			return nil
		}

		clone := oldLease.DeepCopy()
		if clone.Labels == nil {
			clone.Labels = lease.Labels
		} else {
			for k, v := range lease.Labels {
				clone.Labels[k] = v
			}
		}
		clone.Spec = lease.Spec

		if ownerOk {
			err = patchhelper.TwoWayMergePatch(oldLease, clone, &coordinationv1.Lease{}, func(patchData []byte) error {
				_, err2 := c.nodeLeaseClient.Patch(
					c.Context(), name, types.StrategicMergePatchType, patchData, metav1.PatchOptions{})
				return err2
			})
		} else {
			_, err = c.nodeLeaseClient.Update(c.Context(), clone, metav1.UpdateOptions{})
		}

		if err != nil {
			if kubeerrors.IsConflict(err) {
				return err
			}

			logger.D("failed to update node lease, deleting", log.Error(err))
			err = c.nodeLeaseClient.Delete(c.Context(), name, *deleteAtOnce)
			if err != nil && !kubeerrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete old node lease: %w", err)
			}
			create = true
		}
	} else {
		create = true
	}

	if create {
		logger.D("creating node lease")
		_, err = c.nodeLeaseClient.Create(c.Context(), lease, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create node lease: %w", err)
		}
	}

	return nil
}

func (c *Controller) newLeaseForNode(nodeMeta metav1.ObjectMeta) *coordinationv1.Lease {
	identity := nodeMeta.Name
	leaseSeconds := int32(c.vnConfig.Node.Lease.Duration.Seconds())

	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeMeta.Name,
			Namespace: corev1.NamespaceNodeLease,
			Labels: map[string]string{
				constant.LabelRole:      constant.LabelRoleValueNodeLease,
				constant.LabelNamespace: constant.WatchNS(),
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "Node",
				Name:       nodeMeta.Name,
				UID:        nodeMeta.UID,
			}},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &identity,
			LeaseDurationSeconds: &leaseSeconds,
		},
	}
}
