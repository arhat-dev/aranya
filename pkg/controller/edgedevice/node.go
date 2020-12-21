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
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"arhat.dev/pkg/backoff"
	"arhat.dev/pkg/kubehelper"
	"arhat.dev/pkg/log"
	"arhat.dev/pkg/patchhelper"
	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	"arhat.dev/pkg/textquery"
	"github.com/itchyny/gojq"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	informerscorev1 "k8s.io/client-go/informers/core/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	clientcodv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	listerscodv1 "k8s.io/client-go/listers/coordination/v1"
	listerscodv1b1 "k8s.io/client-go/listers/coordination/v1beta1"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	kubecache "k8s.io/client-go/tools/cache"
	utilnode "k8s.io/kubernetes/pkg/util/node"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/util/convert"
	"arhat.dev/aranya/pkg/virtualnode"
)

type nodeController struct {
	nodeCtx context.Context

	vnRec        *reconcile.Core
	virtualNodes *virtualnode.Manager
	vnConfig     *conf.VirtualnodeConfig
	vnKubeconfig *rest.Config

	// Node management
	nodeClient    clientcorev1.NodeInterface
	nodeInformer  kubecache.SharedIndexInformer
	nodeStatusRec *kubehelper.KubeInformerReconciler
	nodeReqRec    *reconcile.Core

	nodeLeaseInformer kubecache.SharedIndexInformer
	nodeLeaseClient   clientcodv1.LeaseInterface
	nodeLeaseReqRec   *reconcile.Core
}

func (c *nodeController) init(
	ctrl *Controller,
	config *conf.Config,
	kubeClient kubeclient.Interface,
	nodeInformerTyped informerscorev1.NodeInformer,
	preferredResources []*metav1.APIResourceList,
) error {
	c.nodeCtx = ctrl.Context()
	_, kubeConfigForVirtualNode, err := config.VirtualNode.KubeClient.NewKubeClient(nil, true)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig for virtualnode: %w", err)
	}

	c.nodeInformer = nodeInformerTyped.Informer()

	ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs, c.nodeInformer.HasSynced)
	ctrl.listActions = append(ctrl.listActions, func() error {
		_, err := listerscorev1.NewNodeLister(c.nodeInformer.GetIndexer()).List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list nodes: %w", err)
		}

		return nil
	})

	c.vnKubeconfig = kubeConfigForVirtualNode
	c.nodeClient = kubeClient.CoreV1().Nodes()
	c.nodeLeaseClient = kubeClient.CoordinationV1().Leases(corev1.NamespaceNodeLease)
	c.vnConfig = &config.VirtualNode

	var getLeaseFunc func(name string) *coordinationv1.Lease
	if config.VirtualNode.Node.Lease.Enabled {
		// informer and sync
		nodeLeaseInformerFactory := informers.NewSharedInformerFactoryWithOptions(kubeClient, 0,
			informers.WithNamespace(corev1.NamespaceNodeLease),
			informers.WithTweakListOptions(
				newTweakListOptionsFunc(
					labels.SelectorFromSet(map[string]string{
						constant.LabelRole:      constant.LabelRoleValueNodeLease,
						constant.LabelNamespace: constant.SysNS(),
					}),
				),
			),
		)

		ctrl.informerFactoryStart = append(ctrl.informerFactoryStart, nodeLeaseInformerFactory.Start)

		leaseClient := kubehelper.CreateLeaseClient(preferredResources, kubeClient, corev1.NamespaceNodeLease)
		switch {
		case leaseClient.V1Client != nil:
			c.nodeLeaseInformer = nodeLeaseInformerFactory.Coordination().V1().Leases().Informer()
			ctrl.listActions = append(ctrl.listActions, func() error {
				_, err2 := listerscodv1.NewLeaseLister(c.nodeLeaseInformer.GetIndexer()).List(labels.Everything())
				if err2 != nil {
					return fmt.Errorf("failed to list cluster roles: %w", err2)
				}

				return nil
			})
		case leaseClient.V1b1Client != nil:
			c.nodeLeaseInformer = nodeLeaseInformerFactory.Coordination().V1beta1().Leases().Informer()
			ctrl.listActions = append(ctrl.listActions, func() error {
				_, err2 := listerscodv1b1.NewLeaseLister(c.nodeLeaseInformer.GetIndexer()).List(labels.Everything())
				if err2 != nil {
					return fmt.Errorf("failed to list cluster roles: %w", err2)
				}

				return nil
			})
		default:
			return fmt.Errorf("no lease api support in kubernetes cluster")
		}

		ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs, c.nodeLeaseInformer.HasSynced)

		getLeaseFunc = func(name string) *coordinationv1.Lease {
			obj, ok, err2 := c.nodeLeaseInformer.GetIndexer().GetByKey(corev1.NamespaceNodeLease + "/" + name)
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

		nodeLeaseRec := kubehelper.NewKubeInformerReconciler(ctrl.Context(), c.nodeLeaseInformer, reconcile.Options{
			Logger:          ctrl.Log.WithName("rec:nl"),
			BackoffStrategy: nil,
			Workers:         0,
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

		c.nodeLeaseReqRec = reconcile.NewCore(ctrl.Context(), reconcile.Options{
			Logger:          ctrl.Log.WithName("rec:nl:req"),
			BackoffStrategy: nil,
			Workers:         0,
			RequireCache:    true,
			Handlers: reconcile.HandleFuncs{
				OnAdded: ctrl.onNodeLeaseEnsureRequest,
			},
			OnBackoffStart: nil,
			OnBackoffReset: nil,
		}.ResolveNil())
		ctrl.recStart = append(ctrl.recStart, c.nodeLeaseReqRec.Start)
		ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, c.nodeLeaseReqRec.ReconcileUntil)
	} else {
		getLeaseFunc = func(name string) *coordinationv1.Lease {
			return nil
		}
	}

	c.virtualNodes = virtualnode.NewVirtualNodeManager(
		ctrl.Context(),
		&config.VirtualNode,
		getLeaseFunc,
		c.nodeLeaseClient,
	)

	nodeRec := kubehelper.NewKubeInformerReconciler(ctrl.Context(), c.nodeInformer, reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:node"),
		BackoffStrategy: nil,
		Workers:         0,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded:    nextActionUpdate,
			OnUpdated:  ctrl.onNodeUpdated,
			OnDeleting: ctrl.onNodeDeleting,
			OnDeleted:  ctrl.onNodeDeleted,
		},
	})
	ctrl.recStart = append(ctrl.recStart, nodeRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, nodeRec.ReconcileUntil)

	c.nodeReqRec = reconcile.NewCore(ctrl.Context(), reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:node:req"),
		BackoffStrategy: nil,
		Workers:         0,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded: ctrl.onNodeEnsureRequested,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	}.ResolveNil())
	ctrl.recStart = append(ctrl.recStart, c.nodeReqRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, c.nodeReqRec.ReconcileUntil)

	// start a standalone node status reconciler in addition to node reconciler to make it clear for node
	c.nodeStatusRec = kubehelper.NewKubeInformerReconciler(ctrl.Context(), c.nodeInformer, reconcile.Options{
		Logger: ctrl.Log.WithName("rec:node:status"),
		// no backoff, always wait for 1s
		BackoffStrategy: backoff.NewStrategy(time.Second, time.Second, 1, 0),
		Workers:         0,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnUpdated: ctrl.onNodeStatusUpdated,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	})
	ctrl.recStart = append(ctrl.recStart, c.nodeStatusRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, c.nodeStatusRec.ReconcileUntil)

	c.vnRec = reconcile.NewCore(ctrl.Context(), reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:vn"),
		BackoffStrategy: backoff.NewStrategy(time.Second, time.Minute, 2, 1),
		Workers:         0,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded:    ctrl.onEdgeDeviceCreationRequested,
			OnUpdated:  ctrl.onEdgeDeviceUpdateRequested,
			OnDeleting: ctrl.onEdgeDeviceDeletionRequested,
			OnDeleted:  ctrl.onEdgeDeviceCleanup,
		},
	}.ResolveNil())
	ctrl.recStart = append(ctrl.recStart, c.vnRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, c.vnRec.ReconcileUntil)

	return nil
}

func (c *nodeController) checkNodeQualified(node *corev1.Node, kubeletListener net.Listener) bool {
	port, err := getListenerPort(kubeletListener)
	if err != nil {
		return false
	}

	if len(node.Labels) == 0 || node.Labels[constant.LabelRole] != constant.LabelRoleValueNode {
		return false
	}

	hasRequiredTaint := false
	for _, t := range node.Spec.Taints {
		if t.Key == constant.TaintKeyNamespace &&
			t.Value == constant.SysNS() &&
			t.Effect == corev1.TaintEffectNoSchedule {
			hasRequiredTaint = true
		}
	}

	return hasRequiredTaint && node.Status.DaemonEndpoints.KubeletEndpoint.Port == port
}

func (c *nodeController) requestNodeEnsure(name string) error {
	if c.nodeReqRec == nil {
		return nil
	}

	c.nodeReqRec.Update(name, name, name)
	err := c.nodeReqRec.Schedule(queue.Job{Action: queue.ActionAdd, Key: name}, 0)
	if err != nil && !errors.Is(err, queue.ErrJobDuplicated) {
		return fmt.Errorf("failed to schedule node ensure: %w", err)
	}

	return nil
}

func (c *Controller) onNodeEnsureRequested(obj interface{}) *reconcile.Result {
	var (
		name   = obj.(string)
		logger = c.Log.WithFields(log.String("name", name))
	)

	vn, ok := c.virtualNodes.Get(name)
	if !ok {
		_, shouldEnsure := c.getEdgeDeviceObject(name)
		if !shouldEnsure {
			return nil
		}

		return &reconcile.Result{Err: fmt.Errorf("waiting for virtual node")}
	}

	logger.V("ensuring node")
	err := c.ensureNodeObject(name, vn.KubeletServerListener())
	if err != nil {
		logger.I("failed to ensure node")
		return &reconcile.Result{Err: err}
	}
	logger.V("ensured node")

	return nil
}

func (c *Controller) onNodeUpdated(oldObj, newObj interface{}) *reconcile.Result {
	var (
		node   = newObj.(*corev1.Node).DeepCopy()
		name   = node.Name
		logger = c.Log.WithFields(log.String("name", name))
	)

	vn, ignore := c.doNodeResourcePreCheck(name)
	if ignore {
		return nil
	}

	listener := vn.KubeletServerListener()
	if listener == nil {
		logger.I("no kubelet listener found, error in virtualnode creation")
		return nil
	}

	if c.checkNodeQualified(node, listener) {
		logger.V("matches virtual node, setting pod cidr(s)")
		// TODO: recognize cni plugin specific CIDR annotations (e.g. calico, cilium)
		vn.SetPodCIDRs(convert.GetPodCIDRWithIPVersion(node.Spec.PodCIDR, node.Spec.PodCIDRs))
	} else {
		logger.D("updating outdated node")
		err := c.ensureNodeObject(name, listener)
		if err != nil {
			logger.I("failed to ensure node object")
			return &reconcile.Result{Err: err}
		}

		logger.V("scheduling node status update for updated node")
		err = c.nodeStatusRec.Schedule(queue.Job{
			Action: queue.ActionUpdate,
			Key:    name,
		}, 0)
		if err != nil {
			logger.I("failed to schedule node status update for updated node", log.Error(err))
		}
	}

	logger.D("evaluating node field hooks")
	err := c.evalNodeFiledHooksAndUpdateNode(logger, node, vn)
	if err != nil {
		logger.I("failed to eval and update node field hooks", log.Error(err))
		return &reconcile.Result{Err: err}
	}

	return nil
}

func (c *Controller) onNodeDeleting(obj interface{}) *reconcile.Result {
	// TODO: add finalizer
	err := c.nodeClient.Delete(c.Context(), obj.(*corev1.Node).Name, *deleteAtOnce)
	if err != nil && !kubeerrors.IsNotFound(err) {
		return &reconcile.Result{Err: err}
	}

	return &reconcile.Result{NextAction: queue.ActionCleanup}
}

func (c *Controller) onNodeDeleted(obj interface{}) *reconcile.Result {
	var (
		name   = obj.(*corev1.Node).Name
		logger = c.Log.WithFields(log.String("name", name))
	)

	// TODO: Node could be removed from cache by changing the label, which wont trigger onNodeDeleting
	vn, ignore := c.doNodeResourcePreCheck(name)
	if ignore {
		return nil
	}

	listener := vn.KubeletServerListener()
	if listener == nil {
		logger.I("no kubelet listener found, error in virtualnode creation")
		return nil
	}

	// virtualnode exists, this node should present
	err := c.ensureNodeObject(name, listener)
	if err != nil {
		logger.I("failed to restore node deleted", log.Error(err))
		return &reconcile.Result{Err: err}
	}

	return nil
}

func (c *Controller) evalNodeFiledHooksAndUpdateNode(
	logger log.Interface,
	node *corev1.Node,
	vn *virtualnode.VirtualNode,
) error {
	device, ok := c.getEdgeDeviceObject(node.Name)
	if !ok {
		logger.I("no EdgeDevice for this node")
		return nil
	}

	if len(device.Spec.Node.FieldHooks) == 0 {
		logger.V("field hooks not set, skipping")
		return nil
	}

	var (
		updateNode *corev1.Node

		extLabels, extAnnotations = vn.ExtInfo()
	)

	// get labels & annotations not applied
	setLabels, setAnnotations := getNodeLabelsAndAnnotationsToSet(
		node.Labels, node.Annotations,
		&node.Status.NodeInfo,
		extLabels, extAnnotations,
	)

	if len(setLabels) != 0 || len(setAnnotations) != 0 {
		updateNode = node.DeepCopy()

		if updateNode.Labels == nil {
			updateNode.Labels = setLabels
		} else {
			for k, v := range setLabels {
				updateNode.Labels[k] = v
			}
		}

		if updateNode.Annotations == nil {
			updateNode.Annotations = setAnnotations
		} else {
			for k, v := range setAnnotations {
				node.Annotations[k] = v
			}
		}
	}

	toMarshal := node
	if updateNode != nil {
		// labels/annotations not up to date, but we should eval with the latest info
		toMarshal = updateNode
	}

	nodeJSONBytes, err := json.Marshal(toMarshal)
	if err != nil {
		return fmt.Errorf("failed to marshal node object: %w", err)
	}

	data := make(map[string]interface{})
	err = json.Unmarshal(nodeJSONBytes, &data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal node object as map: %w", err)
	}

	for _, h := range device.Spec.Node.FieldHooks {
		var query *gojq.Query
		query, err = gojq.Parse(h.Query)
		if err != nil {
			logger.I("bad field query", log.String("query", h.Query), log.Error(err))
			continue
		}

		result, found, err2 := textquery.RunQuery(query, data, nil)
		if err2 != nil {
			logger.I("failed to run query over node object", log.String("query", h.Query), log.Error(err2))
			continue
		}

		if !found {
			// nothing found, which means query failed
			continue
		}

		// found something, proceed

		targetValue := h.Value
		switch {
		case targetValue != "":
			// user already set value, do nothing
		case h.ValueExpression != "":
			targetValue, err = textquery.JQ(h.ValueExpression, result)
			if err != nil {
				logger.I("failed to eval value expression", log.String("expression", h.ValueExpression), log.Error(err))
			}
		}

		fp := h.TargetFieldPath

		switch {
		case strings.HasPrefix(fp, "metadata.labels['") && strings.HasSuffix(fp, "']"):
			key := strings.TrimSuffix(strings.TrimPrefix(fp, "metadata.labels['"), "']")

			if node.Labels[key] != targetValue {
				if updateNode == nil {
					updateNode = node.DeepCopy()
				}

				updateNode.Labels[key] = targetValue
			}
		case strings.HasPrefix(fp, "metadata.annotations['") && strings.HasSuffix(fp, "']"):
			key := strings.TrimSuffix(strings.TrimPrefix(fp, "metadata.annotations['"), "']")

			if node.Annotations[key] != targetValue {
				if updateNode == nil {
					updateNode = node.DeepCopy()
				}

				updateNode.Annotations[key] = targetValue
			}
		default:
			logger.I("unsupported target field", log.String("targetField", fp))
			continue
		}
	}

	if updateNode == nil {
		return nil
	}

	logger.D("updating node object metadata for field hooks")
	err = patchhelper.TwoWayMergePatch(node, updateNode, &corev1.Node{}, func(patchData []byte) error {
		_, err2 := c.nodeClient.Patch(
			c.Context(), node.Name, types.StrategicMergePatchType, patchData, metav1.PatchOptions{},
		)
		return err2
	})

	if err != nil {
		logger.I("failed to update node object", log.Error(err))
		return err
	}

	return nil
}

func (c *Controller) ensureNodeObject(name string, kubeletListener net.Listener) error {
	var create bool
	edgeDeviceObj, ok := c.getEdgeDeviceObject(name)
	if !ok {
		// EdgeDevice is the controller of node, and once edge device get deleted,
		// node will be deleted as well (by garbage-collector in kube-controller-manager)
		// user should use `kubectl delete --cascade=false` when deleting edge device
		// if they want to preserve node object
		return nil
	}

	kubeletPort, err := getListenerPort(kubeletListener)
	if err != nil {
		return err
	}

	node := c.newNodeForEdgeDevice(edgeDeviceObj, kubeletPort)

	oldNode, err := c.nodeClient.Get(c.Context(), name, metav1.GetOptions{})
	if err == nil {
		c.Log.I("found old node, patch to update")
		err = func() error {
			clone := oldNode.DeepCopy()
			if clone.Annotations == nil {
				clone.Annotations = node.Annotations
			} else {
				// keep old annotations
				for k, v := range node.Annotations {
					clone.Annotations[k] = v
				}
			}

			if clone.Labels == nil {
				clone.Labels = node.Labels
			} else {
				// keep old labels
				for k, v := range node.Labels {
					if k == constant.LabelKubeRole {
						// keep label kubernetes.io/role
						continue
					}

					clone.Labels[k] = v
				}
			}

			var appliedTaints []corev1.Taint
			if len(clone.Spec.Taints) == 0 {
				appliedTaints = node.Spec.Taints
			} else {
				for i, t := range node.Spec.Taints {
					if t.Key == constant.TaintKeyNamespace {
						// ensure default taint
						appliedTaints = append(appliedTaints, node.Spec.Taints[i])
						continue
					}

					found := false
					for j, oldT := range clone.Spec.Taints {
						if t.Key == oldT.Key {
							found = true
							// do not override user settings for node
							appliedTaints = append(appliedTaints, clone.Spec.Taints[j])
							break
						}
					}

					if !found {
						appliedTaints = append(appliedTaints, node.Spec.Taints[i])
					}
				}
			}

			clone.ClusterName = node.ClusterName
			clone.Spec.Taints = appliedTaints
			clone.Spec.ProviderID = node.Spec.ProviderID
			clone.OwnerReferences = node.OwnerReferences

			err = patchhelper.TwoWayMergePatch(oldNode, clone, &corev1.Node{}, func(patchData []byte) error {
				oldNode, err = c.nodeClient.Patch(
					c.Context(), name, types.StrategicMergePatchType, patchData, metav1.PatchOptions{})
				return err
			})
			if err != nil {
				return fmt.Errorf("failed to patch node spec: %w", err)
			}

			oldNode, err = c.nodeClient.Get(c.Context(), name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get patched node: %w", err)
			}

			clone = oldNode.DeepCopy()
			clone.Status.Addresses = node.Status.Addresses
			clone.Status.DaemonEndpoints = node.Status.DaemonEndpoints

			_, _, err = utilnode.PatchNodeStatus(c.kubeClient.CoreV1(), types.NodeName(name), oldNode, clone)
			if err != nil {
				return fmt.Errorf("failed to patch node status: %w", err)
			}

			if len(node.Spec.PodCIDRs) > 0 {
				for i := 0; i < 3; i++ {
					err = utilnode.PatchNodeCIDRs(c.kubeClient, types.NodeName(name), node.Spec.PodCIDRs)
					if err == nil {
						return nil
					}
				}

				return fmt.Errorf("failed to update pod cidr: %w", err)
			}

			return nil
		}()

		if err != nil {
			c.Log.I("failed to patch update node object", log.Error(err))

			if c.vnConfig.Node.RecreateIfPatchFailed {
				c.Log.I("deleting node after patch failed to recreate")
				err = c.nodeClient.Delete(c.Context(), name, *deleteAtOnce)
				if err != nil && !kubeerrors.IsNotFound(err) {
					return fmt.Errorf("failed to delete old node: %w", err)
				}

				create = true
			}
		}
	} else {
		if !kubeerrors.IsNotFound(err) {
			return fmt.Errorf("failed to get old node: %w", err)
		}

		// old node not found
		create = true
	}

	if create {
		node, err = c.nodeClient.Create(c.Context(), node, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

// nolint:lll
func (c *Controller) newNodeForEdgeDevice(edgeDevice *aranyaapi.EdgeDevice, kubeletPort int32) *corev1.Node {
	labels := map[string]string{
		// this label can be overridden
		constant.LabelKubeRole: constant.SysNS(),
	}

	for k, v := range edgeDevice.Spec.Node.Labels {
		labels[k] = v
	}

	// ensure following labels not overridden by user, they are special labels
	labels[constant.LabelName] = edgeDevice.Name
	labels[constant.LabelRole] = constant.LabelRoleValueNode
	labels[constant.LabelNamespace] = constant.SysNS()
	labels[corev1.LabelHostname] = c.hostname

	// ensure taints
	taints := []corev1.Taint{{
		Key:    constant.TaintKeyNamespace,
		Value:  edgeDevice.Namespace,
		Effect: corev1.TaintEffectNoSchedule,
	}}
	for _, t := range edgeDevice.Spec.Node.Taints {
		if t.Key == constant.TaintKeyNamespace {
			// do not override default taints
			continue
		}

		taints = append(taints, corev1.Taint{
			Key:    t.Key,
			Value:  t.Value,
			Effect: t.Effect,
		})
	}

	podCIDRs := []string{edgeDevice.Spec.Pod.IPv4CIDR, edgeDevice.Spec.Pod.IPv6CIDR}
	switch {
	case podCIDRs[0] == "" && podCIDRs[1] == "":
		// no cidr
		podCIDRs = nil
	case podCIDRs[0] == "" && podCIDRs[1] != "":
		// has ipv6 cidr
		podCIDRs = podCIDRs[1:]
	case podCIDRs[0] != "" && podCIDRs[1] == "":
		// has ipv4 cidr
		podCIDRs = podCIDRs[:1]
	}
	var firstPodCIDR string
	if len(podCIDRs) != 0 {
		firstPodCIDR = podCIDRs[0]
	}

	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        edgeDevice.Name,
			Annotations: edgeDevice.Spec.Node.Annotations,
			Labels:      labels,
			ClusterName: edgeDevice.ClusterName,
			// TODO: currently kube-controller-manager (v1.18.5) claims the owner doesn't exist
			//       even if we are sure the EdgeDevice exists and the uid is correct
			//OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(edgeDevice, controllerKind)},
		},
		Spec: corev1.NodeSpec{
			Taints:   taints,
			PodCIDR:  firstPodCIDR,
			PodCIDRs: podCIDRs,
			// providerID is the cloud resource id with vendor name prefix, it is usually used by cloud controller
			// manager (CCM) to determine whether the related resource exists
			// format: <cloud-vendor-name>://<resource-id>
			//
			// cloud managed computing resources usually derive its resource id via some cloud internal service
			// 		e.g. digital-ocean droplets can retrieve its resource id by requesting http://169.254.169.254/metadata/v1/id
			//			 (ref: https://github.com/digitalocean/digitalocean-cloud-controller-manager/blob/master/docs/getting-started.md#--provider-iddigitaloceandroplet-id)
			//
			// if we do not provide it at all, chances are that, the CCM will determine it automatically and delete
			// this node object automatically if related resource not found
			//
			// if we do not provide it with a prefix unknown to the CCM (e.g. `gce://`), it will be translated
			// into cloud provider specific resource id to look up such resource
			//
			// so here we just set it to `aranya://<edgedevice-namespace>/<edgedevice-name>`
			//
			// working CCM environment:
			//	- digital-ocean
			ProviderID: c.getNodeProviderID(edgeDevice.Namespace, edgeDevice.Name),
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				// in kubernetes v1.18.5, there is an assumption for Node Resource object in kube-controller-manager
				// that all node have condition NodeReady present, or will cause nil pointer panic in the controller
				// and this node will get deleted
				{Type: corev1.NodeReady, Status: corev1.ConditionUnknown},
				{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionUnknown},
				{Type: corev1.NodeDiskPressure, Status: corev1.ConditionUnknown},
				{Type: corev1.NodePIDPressure, Status: corev1.ConditionUnknown},
				{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionUnknown},
			},
			Addresses: c.hostNodeAddresses,
			DaemonEndpoints: corev1.NodeDaemonEndpoints{
				KubeletEndpoint: corev1.DaemonEndpoint{
					Port: kubeletPort,
				},
			},
		},
	}
}

func (c *Controller) doNodeResourcePreCheck(name string) (_ *virtualnode.VirtualNode, ignore bool) {
	// check if virtual node exist (relatively cheap operation)
	vn, ok := c.virtualNodes.Get(name)
	if !ok {
		// do not manage pending node
		return nil, true
	}

	_, ok = c.getEdgeDeviceObject(name)
	if !ok {
		// do not manage unknown node
		return nil, true
	}

	return vn, false
}

func (c *nodeController) getNodeObject(name string) (*corev1.Node, bool, error) {
	obj, found, err := c.nodeInformer.GetIndexer().GetByKey(name)
	if err != nil || !found {
		node, err2 := c.nodeClient.Get(c.nodeCtx, name, metav1.GetOptions{})
		if err2 != nil {
			return nil, false, err2
		}

		return node, true, nil
	}

	node, ok := obj.(*corev1.Node)
	if !ok {
		return nil, false, err
	}

	return node, true, nil
}
