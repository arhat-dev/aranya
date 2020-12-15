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

	"arhat.dev/pkg/envhelper"
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

type podRoleController struct {
	roleInformer    kubecache.SharedIndexInformer
	roleClient      clientrbacv1.RoleInterface
	roleReqRec      *reconcile.Core
	podRoles        map[string]aranyaapi.PodRolePermissions
	virtualPodRoles map[string]aranyaapi.PodRolePermissions
}

func (c *podRoleController) init(
	ctrl *Controller,
	config *conf.Config,
	kubeClient kubeclient.Interface,
	watchInformerFactory informers.SharedInformerFactory,
) error {
	if len(config.Aranya.Managed.PodRoles) == 0 && len(config.Aranya.Managed.VirtualPodRoles) == 0 {
		return nil
	}

	c.podRoles = config.Aranya.Managed.PodRoles
	c.virtualPodRoles = config.Aranya.Managed.VirtualPodRoles

	if len(c.podRoles) != 0 {
		delete(c.podRoles, "")
	} else {
		c.podRoles = make(map[string]aranyaapi.PodRolePermissions)
	}

	if len(c.virtualPodRoles) != 0 {
		delete(c.virtualPodRoles, "")
	} else {
		c.virtualPodRoles = make(map[string]aranyaapi.PodRolePermissions)
	}

	c.roleClient = kubeClient.RbacV1().Roles(constant.WatchNS())

	c.roleInformer = informersrbacv1.New(watchInformerFactory, constant.WatchNS(),
		newTweakListOptionsFunc(
			labels.SelectorFromSet(map[string]string{
				constant.LabelRole: constant.LabelRoleValuePodRole,
			}),
		),
	).Roles().Informer()

	ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs, c.roleInformer.HasSynced)
	ctrl.listActions = append(ctrl.listActions, func() error {
		_, err2 := listersrbacv1.NewRoleLister(c.roleInformer.GetIndexer()).List(labels.Everything())
		if err2 != nil {
			return fmt.Errorf("failed to list watched roles: %w", err2)
		}

		return nil
	})

	roleRec := kubehelper.NewKubeInformerReconciler(ctrl.Context(), c.roleInformer, reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:role"),
		BackoffStrategy: nil,
		Workers:         1,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded:    nextActionUpdate,
			OnUpdated:  ctrl.onPodRoleUpdated,
			OnDeleting: ctrl.onPodRoleDeleting,
			OnDeleted:  ctrl.onPodRoleDeleted,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	})
	ctrl.recStart = append(ctrl.recStart, roleRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, roleRec.ReconcileUntil)

	c.roleReqRec = reconcile.NewCore(ctrl.Context(), reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:role_req"),
		BackoffStrategy: nil,
		Workers:         1,
		RequireCache:    false,
		Handlers: reconcile.HandleFuncs{
			OnAdded: ctrl.onPodRoleEnsureRequested,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	}.ResolveNil())
	ctrl.recStart = append(ctrl.recStart, c.roleReqRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, c.roleReqRec.ReconcileUntil)

	return nil
}

// nolint:gocyclo
func (c *Controller) checkPodRoleUpToDate(obj *rbacv1.Role) bool {
	var (
		isVirtualPodRole = false
	)

	crSpec, ok := c.podRoles[obj.Name]
	if !ok {
		isVirtualPodRole = true
		crSpec, ok = c.virtualPodRoles[obj.Name]
	}

	if !ok {
		// not managed by us
		return true
	}

	switch {
	case len(obj.Labels) == 0 || obj.Labels[constant.LabelRole] != constant.LabelRoleValuePodRole:
		return false
	case len(obj.Labels) == 0 || obj.Labels[constant.LabelNamespace] != constant.WatchNS():
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
			if isVirtualPodRole {
				if !ok {
					// no edge device with this name
					return false
				}
			} else {
				if ok {
					// roles mixed
					return false
				}

				// retrieve real edge device from pod spec, so get pod spec first
				pod, ok := c.getWatchPodObject(podName)
				if !ok {
					// no pod with this resource name
					return false
				}

				edgeDevice, ok = c.getEdgeDeviceObject(pod.Spec.NodeName)
				if !ok {
					// no edge device is managing this pod, this resource name entry needs to be removed
					return false
				}
			}

			var podRoles map[string]aranyaapi.PodRolePermissions
			if isVirtualPodRole {
				podRoles = edgeDevice.Spec.Pod.RBAC.VirtualPodRolePermissions
			} else {
				podRoles = edgeDevice.Spec.Pod.RBAC.RolePermissions
			}

			// check if verbs matched

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

	var requiredResourceNames []string
	if isVirtualPodRole {
		// set resource names for virtual pod names (edge device name)
		requiredResourceNames = c.getVirtualPodNames()
	} else {
		// set resource names for all other pod names
		requiredResourceNames = c.getManagedPodNames()
	}

	if podNames.Len() != len(requiredResourceNames) {
		return false
	}

	return podNames.HasAll(requiredResourceNames...)
}

func (c *Controller) requestPodRoleEnsure() error {
	if c.roleReqRec == nil {
		return nil
	}

	err := c.roleReqRec.Schedule(queue.Job{Action: queue.ActionAdd, Key: ""}, 0)
	if err != nil && !errors.Is(err, queue.ErrJobDuplicated) {
		return fmt.Errorf("failed to schedule pod role update: %w", err)
	}

	return nil
}

func (c *Controller) onPodRoleEnsureRequested(_ interface{}) *reconcile.Result {
	for n := range c.podRoles {
		err := c.ensurePodRole(n)
		if err != nil {
			c.Log.I("failed to ensure pod role", log.String("name", n), log.Error(err))
			return &reconcile.Result{Err: err}
		}
	}

	for n := range c.virtualPodRoles {
		err := c.ensurePodRole(n)
		if err != nil {
			c.Log.I("failed to ensure virtual pod role", log.String("name", n), log.Error(err))
			return &reconcile.Result{Err: err}
		}
	}

	return nil
}

func (c *Controller) onPodRoleUpdated(oldObj, newObj interface{}) *reconcile.Result {
	return nil
}

func (c *Controller) onPodRoleDeleting(obj interface{}) *reconcile.Result {
	return nil
}

func (c *Controller) onPodRoleDeleted(obj interface{}) *reconcile.Result {
	return nil
}

func (c *Controller) ensurePodRole(name string) error {
	isVirtualPodRole := false
	roleSpec, ok := c.podRoles[name]
	if !ok {
		isVirtualPodRole = true
		roleSpec, ok = c.virtualPodRoles[name]
	}

	if !ok {
		// not managed by us, skip
		return nil
	}

	var (
		create bool
		err    error
		role   = c.newPodRoleForAllEdgeDevices(name, roleSpec, isVirtualPodRole)
	)

	oldRole, found := c.getSysRoleObject(name)
	if found {
		if c.checkPodRoleUpToDate(oldRole) {
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
		_, err = c.roleClient.Update(c.Context(), clone, metav1.UpdateOptions{})
		if err != nil {
			if kubeerrors.IsConflict(err) {
				return err
			}

			c.Log.I("failed to update pod role, deleting", log.Error(err))
			err = c.roleClient.Delete(c.Context(), name, *deleteAtOnce)
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
		_, err = c.roleClient.Create(c.Context(), role, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

// nolint:gocyclo
func (c *Controller) newPodRoleForAllEdgeDevices(
	roleName string,
	roleSpec aranyaapi.PodRolePermissions,
	forVirtualPods bool,
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

	for _, obj := range c.podInformer.GetStore().List() {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			continue
		}

		edgeDevice, ok := c.getEdgeDeviceObject(pod.Name)
		if ok {
			// is virtual pod
			if !forVirtualPods {
				// not for this role
				continue
			}
		} else {
			// is normal pod
			if forVirtualPods {
				// not for this role
				continue
			}

			edgeDevice, ok = c.getEdgeDeviceObject(pod.Spec.NodeName)
			if !ok {
				// not managed by us
				continue
			}
		}

		var podRoles map[string]aranyaapi.PodRolePermissions
		if forVirtualPods {
			podRoles = edgeDevice.Spec.Pod.RBAC.VirtualPodRolePermissions
		} else {
			podRoles = edgeDevice.Spec.Pod.RBAC.RolePermissions
		}

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
			Namespace: envhelper.ThisPodNS(),
			Labels: map[string]string{
				constant.LabelRole: constant.LabelRoleValuePodRole,
			},
		},
		Rules: policies,
	}
}
