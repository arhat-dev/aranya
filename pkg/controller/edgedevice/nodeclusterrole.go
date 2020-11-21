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
	rbacv1 "k8s.io/api/rbac/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	informersrbacv1 "k8s.io/client-go/informers/rbac/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	clientrbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	listersrbacv1 "k8s.io/client-go/listers/rbac/v1"
	kubecache "k8s.io/client-go/tools/cache"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
)

type nodeClusterRoleController struct {
	crInformer       kubecache.SharedIndexInformer
	crClient         clientrbacv1.ClusterRoleInterface
	crReqRec         *reconcile.Core
	nodeClusterRoles map[string]aranyaapi.NodeClusterRolePermissions
}

func (c *nodeClusterRoleController) init(
	ctrl *Controller,
	config *conf.Config,
	kubeClient kubeclient.Interface,
	clusterInformerFactory informers.SharedInformerFactory,
) error {
	// watch cluster roles managed by us
	if len(config.Aranya.Managed.NodeClusterRoles) > 0 {
		return nil
	}

	c.nodeClusterRoles = config.Aranya.Managed.NodeClusterRoles
	// client
	c.crClient = kubeClient.RbacV1().ClusterRoles()

	// informer and sync
	c.crInformer = informersrbacv1.New(clusterInformerFactory, corev1.NamespaceAll,
		newTweakListOptionsFunc(
			labels.SelectorFromSet(map[string]string{
				constant.LabelRole:      constant.LabelRoleValueNodeClusterRole,
				constant.LabelNamespace: constant.WatchNS(),
			}),
		),
	).ClusterRoles().Informer()
	ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs, c.crInformer.HasSynced)

	ctrl.listActions = append(ctrl.listActions, func() error {
		_, err2 := listersrbacv1.NewClusterRoleLister(c.crInformer.GetIndexer()).List(labels.Everything())
		if err2 != nil {
			return fmt.Errorf("failed to list cluster roles: %w", err2)
		}

		return nil
	})

	// reconciler for cluster role resources
	crRec := kubehelper.NewKubeInformerReconciler(ctrl.Context(), c.crInformer, reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:cr"),
		BackoffStrategy: nil,
		Workers:         1,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded:    nextActionUpdate,
			OnUpdated:  ctrl.onNodeClusterRoleUpdated,
			OnDeleting: ctrl.onNodeClusterRoleDeleting,
			OnDeleted:  ctrl.onNodeClusterRoleDeleted,
		},
	})
	ctrl.recStart = append(ctrl.recStart, crRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, crRec.ReconcileUntil)

	c.crReqRec = reconcile.NewCore(ctrl.Context(), reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:cr_req"),
		BackoffStrategy: nil,
		Workers:         1,
		RequireCache:    false,
		Handlers: reconcile.HandleFuncs{
			OnAdded: ctrl.onNodeClusterRoleEnsureRequested,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	}.ResolveNil())
	ctrl.recStart = append(ctrl.recStart, c.crReqRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, c.crReqRec.ReconcileUntil)

	return nil
}

func (c *Controller) checkNodeClusterRoleUpToDate(obj *rbacv1.ClusterRole) bool {
	crSpec, ok := c.nodeClusterRoles[obj.Name]
	if !ok {
		// not managed by us
		return true
	}

	switch {
	case len(obj.Labels) == 0 || obj.Labels[constant.LabelRole] != constant.LabelRoleValueNodeClusterRole:
		return false
	case len(obj.Labels) == 0 || obj.Labels[constant.LabelNamespace] != constant.WatchNS():
		return false
	}

	nodeNames := make(map[string]struct{})
	for _, p := range obj.Rules {
		if len(p.ResourceNames) == 0 {
			return false
		}

		switch {
		case len(p.APIGroups) != 1 || p.APIGroups[0] != "":
			return false
		case len(p.Resources) != 1:
			return false
		}

		nodeVerbs := crSpec.NodeVerbs
		nodeStatusVerbs := crSpec.StatusVerbs

		// check edge device override if only one resource name found
		if len(p.ResourceNames) == 1 {
			name := p.ResourceNames[0]

			edgeDevice, ok := c.getEdgeDeviceObject(name)
			if !ok {
				// no edge device with this name, this entry needs to be removed
				return false
			}

			nodePermissions := edgeDevice.Spec.Node.RBAC.ClusterRolePermissions
			if len(nodePermissions) != 0 {
				if spec, ok := nodePermissions[obj.Name]; ok {
					if len(spec.NodeVerbs) != 0 {
						nodeVerbs = spec.NodeVerbs
					}

					if len(spec.StatusVerbs) != 0 {
						nodeStatusVerbs = spec.StatusVerbs
					}
				}
			}
		}

		var requiredVerbs []string
		switch p.Resources[0] {
		case "nodes":
			requiredVerbs = nodeVerbs
		case "nodes/status":
			requiredVerbs = nodeStatusVerbs
		default:
			return false
		}

		// check if all verbs included
		if !containsAll(p.Verbs, requiredVerbs) {
			return false
		}

		// prepare for resource name check
		for _, n := range p.ResourceNames {
			nodeNames[n] = struct{}{}
		}
	}

	// check if all nodes are exactly included (resource name check)

	edgeDevices := c.edgeDeviceInformer.GetIndexer().ListKeys()
	if len(edgeDevices) != len(nodeNames) {
		// current cluster role contains more/less than expected nodes
		return false
	}

	for _, namespacedName := range edgeDevices {
		parts := strings.SplitN(namespacedName, "/", 2)
		if len(parts) != 2 {
			// not possible, just in case
			return false
		}

		delete(nodeNames, parts[1])
	}

	// nolint:gosimple
	if len(nodeNames) != 0 {
		return false
	}

	return true
}

func (c *Controller) requestNodeClusterRoleEnsure() error {
	if c.crReqRec == nil {
		return nil
	}

	err := c.crReqRec.Schedule(queue.Job{Action: queue.ActionAdd, Key: ""}, 0)
	if err != nil && !errors.Is(err, queue.ErrJobDuplicated) {
		return fmt.Errorf("failed to schedule node cluster role ensure: %w", err)
	}

	return nil
}

func (c *Controller) onNodeClusterRoleEnsureRequested(_ interface{}) *reconcile.Result {
	for n := range c.nodeClusterRoles {
		err := c.ensureNodeClusterRole(n)
		if err != nil {
			c.Log.I("failed to ensure node cluster role", log.String("name", n), log.Error(err))
			return &reconcile.Result{Err: err}
		}
	}

	return nil
}

func (c *Controller) onNodeClusterRoleUpdated(oldObj, newObj interface{}) *reconcile.Result {
	var (
		err      error
		newCRObj = newObj.(*rbacv1.ClusterRole)
		name     = newCRObj.Name
		logger   = c.Log.WithFields(log.String("name", name))
	)

	logger.V("cluster role updated")
	if c.checkNodeClusterRoleUpToDate(newCRObj) {
		logger.V("cluster role is up to date")
		return nil
	}

	logger.D("updating outdated cluster role")
	err = c.ensureNodeClusterRole(name)
	if err != nil {
		logger.I("failed to ensure cluster role up to date: %w", log.Error(err))
		return &reconcile.Result{Err: err}
	}

	return nil
}

func (c *Controller) onNodeClusterRoleDeleting(obj interface{}) *reconcile.Result {
	err := c.crClient.Delete(c.Context(), obj.(*rbacv1.ClusterRole).Name, *deleteAtOnce)
	if err != nil && !kubeerrors.IsNotFound(err) {
		return &reconcile.Result{Err: err}
	}

	return &reconcile.Result{NextAction: queue.ActionCleanup}
}

func (c *Controller) onNodeClusterRoleDeleted(obj interface{}) *reconcile.Result {
	var (
		name   = obj.(*rbacv1.ClusterRole).Name
		logger = c.Log.WithFields(log.String("name", name))
	)

	logger.V("cluster role deleted")
	err := c.ensureNodeClusterRole(name)
	if err != nil {
		logger.I("failed to ensure cluster role after being deleted", log.Error(err))
		return &reconcile.Result{Err: err}
	}

	return nil
}

func (c *Controller) ensureNodeClusterRole(name string) error {
	crSpec, ok := c.nodeClusterRoles[name]
	if !ok {
		// not managed by us, skip
		return nil
	}

	var (
		create bool
		err    error
		cr     = c.newNodeClusterRoleForAllEdgeDevices(name, crSpec)
	)

	oldCR, found := c.getClusterRoleObject(name)
	if found {
		if c.checkNodeClusterRoleUpToDate(oldCR) {
			return nil
		}

		clone := oldCR.DeepCopy()
		if clone.Labels == nil {
			clone.Labels = make(map[string]string)
		}

		for k, v := range cr.Labels {
			clone.Labels[k] = v
		}

		clone.Rules = cr.Rules

		c.Log.D("updating cluster node role")
		_, err = c.crClient.Update(c.Context(), clone, metav1.UpdateOptions{})
		if err != nil {
			if kubeerrors.IsConflict(err) {
				return err
			}

			c.Log.I("failed to update cluster role, deleting", log.Error(err))
			err = c.crClient.Delete(c.Context(), name, *deleteAtOnce)
			if err != nil && !kubeerrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete old cluster role: %w", err)
			}

			create = true
		}
	} else {
		// old cluster role not found
		create = true
	}

	if create {
		c.Log.D("creating new managed cluster role")
		_, err = c.crClient.Create(c.Context(), cr, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) newNodeClusterRoleForAllEdgeDevices(
	crName string,
	crSpec aranyaapi.NodeClusterRolePermissions,
) *rbacv1.ClusterRole {
	var (
		policies                   []rbacv1.PolicyRule
		nodeWithDefaultNodeVerbs   []string
		nodeWithDefaultStatusVerbs []string
	)

	for _, obj := range c.edgeDeviceInformer.GetStore().List() {
		edgeDevice, ok := obj.(*aranyaapi.EdgeDevice)
		if !ok {
			continue
		}

		if nodeCRs := edgeDevice.Spec.Node.RBAC.ClusterRolePermissions; len(nodeCRs) == 0 {
			nodeWithDefaultNodeVerbs = append(nodeWithDefaultNodeVerbs, edgeDevice.Name)
			nodeWithDefaultStatusVerbs = append(nodeWithDefaultStatusVerbs, edgeDevice.Name)
		} else if nodePermissions, ok := nodeCRs[crName]; ok {
			if len(nodePermissions.NodeVerbs) != 0 {
				policies = append(policies, rbacv1.PolicyRule{
					APIGroups:     []string{""},
					Resources:     []string{"nodes"},
					Verbs:         nodePermissions.NodeVerbs,
					ResourceNames: []string{edgeDevice.Name},
				})
			} else {
				nodeWithDefaultNodeVerbs = append(nodeWithDefaultNodeVerbs, edgeDevice.Name)
			}

			if len(nodePermissions.StatusVerbs) != 0 {
				policies = append(policies, rbacv1.PolicyRule{
					APIGroups:     []string{""},
					Resources:     []string{"nodes/status"},
					Verbs:         nodePermissions.StatusVerbs,
					ResourceNames: []string{edgeDevice.Name},
				})
			} else {
				nodeWithDefaultStatusVerbs = append(nodeWithDefaultStatusVerbs, edgeDevice.Name)
			}
		}
	}

	if len(nodeWithDefaultNodeVerbs) != 0 {
		policies = append(policies, rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"nodes"},
			Verbs:         crSpec.NodeVerbs,
			ResourceNames: nodeWithDefaultNodeVerbs,
		})
	}

	if len(nodeWithDefaultStatusVerbs) != 0 {
		policies = append(policies, rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"nodes/status"},
			Verbs:         crSpec.StatusVerbs,
			ResourceNames: nodeWithDefaultStatusVerbs,
		})
	}

	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: crName,
			Labels: map[string]string{
				constant.LabelRole:      constant.LabelRoleValueNodeClusterRole,
				constant.LabelNamespace: constant.WatchNS(),
			},
		},
		Rules: policies,
	}
}
