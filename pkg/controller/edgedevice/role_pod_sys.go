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
	"errors"
	"fmt"

	"arhat.dev/pkg/kubehelper"
	"arhat.dev/pkg/log"
	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
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

type sysPodRoleController struct {
	vpRoleCtx context.Context

	vpRoleInformer kubecache.SharedIndexInformer
	vpRoleClient   clientrbacv1.RoleInterface
	vpRoleReqRec   *reconcile.Core

	vpRoles map[string]aranyaapi.PodRolePermissions
}

func (c *sysPodRoleController) init(
	ctrl *Controller,
	config *conf.Config,
	kubeClient kubeclient.Interface,
	sysInformerFactory informers.SharedInformerFactory,
) error {
	if len(config.Aranya.Managed.PodRoles) == 0 && len(config.Aranya.Managed.VirtualPodRoles) == 0 {
		return nil
	}

	c.vpRoleCtx = ctrl.Context()
	c.vpRoles = config.Aranya.Managed.VirtualPodRoles
	if len(c.vpRoles) != 0 {
		delete(c.vpRoles, "")
	} else {
		c.vpRoles = make(map[string]aranyaapi.PodRolePermissions)
	}

	c.vpRoleClient = kubeClient.RbacV1().Roles(constant.SysNS())

	c.vpRoleInformer = informersrbacv1.New(sysInformerFactory, constant.SysNS(),
		newTweakListOptionsFunc(
			labels.SelectorFromSet(map[string]string{
				constant.LabelRole: constant.LabelRoleValuePodRole,
			}),
		),
	).Roles().Informer()

	ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs, c.vpRoleInformer.HasSynced)
	ctrl.listActions = append(ctrl.listActions, func() error {
		_, err2 := listersrbacv1.NewRoleLister(c.vpRoleInformer.GetIndexer()).List(labels.Everything())
		if err2 != nil {
			return fmt.Errorf("failed to list virtual pod roles: %w", err2)
		}

		return nil
	})

	roleRec := kubehelper.NewKubeInformerReconciler(ctrl.Context(), c.vpRoleInformer, reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:role"),
		BackoffStrategy: nil,
		Workers:         0,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded:    nextActionUpdate,
			OnUpdated:  ctrl.onVirtualPodRoleUpdated,
			OnDeleting: ctrl.onVirtualPodRoleDeleting,
			OnDeleted:  ctrl.onVirtualPodRoleDeleted,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	})
	ctrl.recStart = append(ctrl.recStart, roleRec.Start)
	ctrl.recReconcile = append(ctrl.recReconcile, roleRec.Reconcile)

	c.vpRoleReqRec = reconcile.NewCore(ctrl.Context(), reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:role_req"),
		BackoffStrategy: nil,
		Workers:         1,
		RequireCache:    false,
		Handlers: reconcile.HandleFuncs{
			OnAdded: ctrl.onVirtualPodRoleEnsureRequested,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	}.ResolveNil())
	ctrl.recStart = append(ctrl.recStart, c.vpRoleReqRec.Start)
	ctrl.recReconcile = append(ctrl.recReconcile, c.vpRoleReqRec.Reconcile)

	return nil
}

// nolint:gocyclo
func (c *Controller) checkVirtualPodRoleUpToDate(obj *rbacv1.Role) bool {
	crSpec, ok := c.vpRoles[obj.Name]
	if !ok {
		return true
	}

	if !ok {
		// not managed by us
		return true
	}

	switch {
	case len(obj.Labels) == 0 || obj.Labels[constant.LabelRole] != constant.LabelRoleValuePodRole:
		return false
	case len(obj.Labels) == 0 || obj.Labels[constant.LabelNamespace] != constant.SysNS():
		return false
	}

	// 	rules:
	//  - apiGroups: [""] # must only contains core api group ("")
	//	- resources:
	//    - pod/exec # must only have one resource per rule
	//  - resourceNames: # pods using default or edge device specific permissions
	//    - foo
	//    - bar
	//	  ...
	//  - verbs: # default verbs or edge device specific verbs
	//    - create
	podNames := sets.NewString()
	for _, p := range obj.Rules {
		switch {
		case len(p.APIGroups) != 1 || p.APIGroups[0] != "":
			return false
		case len(p.Resources) != 1:
			return false
		}

		var (
			podVerbs         = crSpec.PodVerbs
			statusVerbs      = crSpec.StatusVerbs
			allowAttach      = crSpec.AllowAttach
			allowExec        = crSpec.AllowExec
			allowLog         = crSpec.AllowLog
			allowPortForward = crSpec.AllowPortForward
		)

		for _, podName := range p.ResourceNames {
			// ensure not mixing virtual pod roles with normal pod roles by trying to get edge device with pod name
			edgeDevice, ok := c.getEdgeDeviceObject(podName)
			if !ok {
				// no edge device with this name
				return false
			}

			// check if verbs matched

			podRoles := edgeDevice.Spec.Pod.RBAC.VirtualPodRolePermissions
			if len(podRoles) == 0 {
				// check default verbs, which is the default option
				// do nothing
			} else {
				// check edgedevice specific roles
				spec, ok := podRoles[obj.Name]
				if ok {
					if len(spec.PodVerbs) != 0 {
						podVerbs = spec.PodVerbs
					}

					if len(spec.StatusVerbs) != 0 {
						statusVerbs = spec.StatusVerbs
					}

					if spec.AllowAttach != nil {
						allowAttach = spec.AllowAttach
					}

					if spec.AllowExec != nil {
						allowExec = spec.AllowExec
					}

					if spec.AllowLog != nil {
						allowLog = spec.AllowLog
					}

					if spec.AllowPortForward != nil {
						allowPortForward = spec.AllowPortForward
					}
				}
			}

			var requiredVerbs []string
			switch p.Resources[0] {
			case "pods":
				requiredVerbs = podVerbs
			case "pods/status":
				requiredVerbs = statusVerbs
			case "pods/log":
				if allowLog != nil && *allowLog {
					requiredVerbs = []string{"create"}
				}
			case "pods/exec":
				if allowExec != nil && *allowExec {
					requiredVerbs = []string{"create"}
				}
			case "pods/attach":
				if allowAttach != nil && *allowAttach {
					requiredVerbs = []string{"create"}
				}
			case "pods/portforward":
				if allowPortForward != nil && *allowPortForward {
					requiredVerbs = []string{"create"}
				}
			default:
				// unknown resource for pod role
				return false
			}

			if len(p.Verbs) != len(requiredVerbs) {
				return false
			}

			// check if all verbs included
			if !containsAll(p.Verbs, requiredVerbs) {
				return false
			}

			podNames.Insert(podName)
		}
	}

	// set resource names for virtual pod names (edge device name)
	requiredResourceNames := c.getVirtualPodNames()
	if podNames.Len() != len(requiredResourceNames) {
		return false
	}

	return podNames.HasAll(requiredResourceNames...)
}

func (c *Controller) requestVirtualPodRoleEnsure() error {
	if c.vpRoleReqRec == nil {
		return nil
	}

	err := c.vpRoleReqRec.Schedule(queue.Job{Action: queue.ActionAdd, Key: ""}, 0)
	if err != nil && !errors.Is(err, queue.ErrJobDuplicated) {
		return fmt.Errorf("failed to schedule pod role update: %w", err)
	}

	return nil
}

func (c *Controller) onVirtualPodRoleEnsureRequested(_ interface{}) *reconcile.Result {
	for n := range c.vpRoles {
		err := c.ensureVirtualPodRole(n)
		if err != nil {
			c.Log.I("failed to ensure virtual pod role", log.String("name", n), log.Error(err))
			return &reconcile.Result{Err: err}
		}
	}

	return nil
}

func (c *Controller) onVirtualPodRoleUpdated(oldObj, newObj interface{}) *reconcile.Result {
	return nil
}

func (c *Controller) onVirtualPodRoleDeleting(obj interface{}) *reconcile.Result {
	return nil
}

func (c *Controller) onVirtualPodRoleDeleted(obj interface{}) *reconcile.Result {
	return nil
}

func (c *Controller) ensureVirtualPodRole(name string) error {
	roleSpec, ok := c.vpRoles[name]
	if !ok {
		return nil
	}

	var (
		create bool
		err    error
		role   = c.newVirtualPodRole(name, roleSpec)
	)

	oldRole, found := c.getSysRoleObject(name)
	if found {
		if c.checkVirtualPodRoleUpToDate(oldRole) {
			return nil
		}

		clone := oldRole.DeepCopy()
		if clone.Labels == nil {
			clone.Labels = make(map[string]string)
		}

		for k, v := range role.Labels {
			clone.Labels[k] = v
		}

		clone.Rules = role.Rules

		c.Log.D("updating pod role", log.String("name", name))
		_, err = c.vpRoleClient.Update(c.Context(), clone, metav1.UpdateOptions{})
		if err != nil {
			if kubeerrors.IsConflict(err) {
				return err
			}

			c.Log.I("failed to update pod role, deleting", log.Error(err))
			err = c.vpRoleClient.Delete(c.Context(), name, *deleteAtOnce)
			if err != nil && !kubeerrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete old cluster role: %w", err)
			}

			create = true
		}
	} else {
		// old role not found
		create = true
	}

	if create {
		c.Log.I("creating new managed pod role")
		_, err = c.vpRoleClient.Create(c.Context(), role, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

// nolint:gocyclo
func (c *Controller) newVirtualPodRole(
	roleName string,
	roleSpec aranyaapi.PodRolePermissions,
) *rbacv1.Role {
	var (
		policies []rbacv1.PolicyRule

		podsWithDefaultPodVerbs    = sets.NewString()
		podsWithDefaultStatusVerbs = sets.NewString()

		podsAllowExec        = sets.NewString()
		podsAllowAttach      = sets.NewString()
		podsAllowLog         = sets.NewString()
		podsAllowPortForward = sets.NewString()

		defaultAllowExec        = (roleSpec.AllowExec != nil) && *roleSpec.AllowExec
		defaultAllowAttach      = (roleSpec.AllowAttach != nil) && *roleSpec.AllowAttach
		defaultAllowLog         = (roleSpec.AllowLog != nil) && *roleSpec.AllowLog
		defaultAllowPortForward = (roleSpec.AllowPortForward != nil) && *roleSpec.AllowPortForward
	)

	for _, obj := range c.sysPodInformer.GetStore().List() {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			continue
		}

		edgeDevice, ok := c.getEdgeDeviceObject(pod.Name)
		if !ok {
			continue
		}

		podRoles := edgeDevice.Spec.Pod.RBAC.VirtualPodRolePermissions
		if podRoles == nil {
			podRoles = make(map[string]aranyaapi.PodRolePermissions)
		}

		podPermissions, ok := podRoles[roleName]
		if !ok {
			podsWithDefaultPodVerbs.Insert(pod.Name)
			podsWithDefaultStatusVerbs.Insert(pod.Name)

			if defaultAllowPortForward {
				podsAllowPortForward.Insert(pod.Name)
			}

			if defaultAllowLog {
				podsAllowLog.Insert(pod.Name)
			}

			if defaultAllowExec {
				podsAllowExec.Insert(pod.Name)
			}

			if defaultAllowAttach {
				podsAllowAttach.Insert(pod.Name)
			}
		} else {
			if len(podPermissions.PodVerbs) != 0 {
				policies = append(policies, rbacv1.PolicyRule{
					APIGroups:     []string{""},
					Resources:     []string{"pods"},
					Verbs:         podPermissions.PodVerbs,
					ResourceNames: []string{pod.Name},
				})
			} else {
				podsWithDefaultPodVerbs.Insert(edgeDevice.Name)
			}

			if len(podPermissions.StatusVerbs) != 0 {
				policies = append(policies, rbacv1.PolicyRule{
					APIGroups:     []string{""},
					Resources:     []string{"pods/status"},
					Verbs:         podPermissions.StatusVerbs,
					ResourceNames: []string{pod.Name},
				})
			} else {
				podsWithDefaultStatusVerbs.Insert(edgeDevice.Name)
			}

			if a := podPermissions.AllowAttach; a != nil {
				if *a {
					podsAllowAttach.Insert(pod.Name)
				}
			} else if defaultAllowAttach {
				podsAllowAttach.Insert(pod.Name)
			}

			if a := podPermissions.AllowExec; a != nil {
				if *a {
					podsAllowExec.Insert(pod.Name)
				}
			} else if defaultAllowExec {
				podsAllowExec.Insert(pod.Name)
			}

			if a := podPermissions.AllowPortForward; a != nil {
				if *a {
					podsAllowPortForward.Insert(pod.Name)
				}
			} else if defaultAllowPortForward {
				podsAllowPortForward.Insert(pod.Name)
			}

			if a := podPermissions.AllowLog; a != nil {
				if *a {
					podsAllowLog.Insert(pod.Name)
				}
			} else if defaultAllowLog {
				podsAllowLog.Insert(pod.Name)
			}
		}
	}

	if len(podsWithDefaultPodVerbs) != 0 {
		policies = append(policies, rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"pods"},
			Verbs:         roleSpec.PodVerbs,
			ResourceNames: podsWithDefaultPodVerbs.List(),
		})
	}

	if len(podsWithDefaultStatusVerbs) != 0 {
		policies = append(policies, rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"pods/status"},
			Verbs:         roleSpec.StatusVerbs,
			ResourceNames: podsWithDefaultStatusVerbs.List(),
		})
	}

	if len(podsAllowAttach) != 0 {
		policies = append(policies, rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"pods/attach"},
			Verbs:         []string{"create"},
			ResourceNames: podsAllowAttach.List(),
		})
	}

	if len(podsAllowExec) != 0 {
		policies = append(policies, rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"pods/exec"},
			Verbs:         []string{"create"},
			ResourceNames: podsAllowExec.List(),
		})
	}

	if len(podsAllowPortForward) != 0 {
		policies = append(policies, rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"pods/portforward"},
			Verbs:         []string{"create"},
			ResourceNames: podsAllowPortForward.List(),
		})
	}

	if len(podsAllowLog) != 0 {
		policies = append(policies, rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"pods/log"},
			Verbs:         []string{"get"},
			ResourceNames: podsAllowLog.List(),
		})
	}

	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: constant.SysNS(),
			Labels: map[string]string{
				constant.LabelRole: constant.LabelRoleValuePodRole,
			},
		},
		Rules: policies,
	}
}

func (c *sysPodRoleController) getSysRoleObject(name string) (*rbacv1.Role, bool) {
	obj, found, err := c.vpRoleInformer.GetIndexer().GetByKey(constant.SysNS() + "/" + name)
	if err != nil || !found {
		role, err := c.vpRoleClient.Get(c.vpRoleCtx, name, metav1.GetOptions{})
		if err != nil {
			return nil, false
		}

		return role, true
	}

	role, ok := obj.(*rbacv1.Role)
	if !ok {
		return nil, false
	}

	return role, true
}
