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

type tenantPodRoleController struct {
	tenantPodRoleInformer kubecache.SharedIndexInformer
	tenantPodRoleClient   clientrbacv1.RoleInterface
	tenantPodRoleReqRec   *reconcile.Core
	tenantPodRoles        map[string]aranyaapi.PodRolePermissions
}

func (c *tenantPodRoleController) init(
	ctrl *Controller,
	config *conf.Config,
	kubeClient kubeclient.Interface,
	tenantInformerFactory informers.SharedInformerFactory,
) error {
	if len(config.Aranya.Managed.PodRoles) == 0 && len(config.Aranya.Managed.VirtualPodRoles) == 0 {
		return nil
	}

	c.tenantPodRoles = config.Aranya.Managed.PodRoles
	if len(c.tenantPodRoles) != 0 {
		delete(c.tenantPodRoles, "")
	} else {
		c.tenantPodRoles = make(map[string]aranyaapi.PodRolePermissions)
	}

	c.tenantPodRoleClient = kubeClient.RbacV1().Roles(constant.TenantNS())

	c.tenantPodRoleInformer = informersrbacv1.New(tenantInformerFactory, constant.TenantNS(),
		newTweakListOptionsFunc(
			labels.SelectorFromSet(map[string]string{
				constant.LabelRole: constant.LabelRoleValuePodRole,
			}),
		),
	).Roles().Informer()

	ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs, c.tenantPodRoleInformer.HasSynced)
	ctrl.listActions = append(ctrl.listActions, func() error {
		_, err2 := listersrbacv1.NewRoleLister(c.tenantPodRoleInformer.GetIndexer()).List(labels.Everything())
		if err2 != nil {
			return fmt.Errorf("failed to list tenant roles: %w", err2)
		}

		return nil
	})

	roleRec := kubehelper.NewKubeInformerReconciler(ctrl.Context(), c.tenantPodRoleInformer, reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:podrole"),
		BackoffStrategy: nil,
		Workers:         0,
		RequireCache:    true,
		Handlers: reconcile.HandleFuncs{
			OnAdded:    nextActionUpdate,
			OnUpdated:  ctrl.onTenantPodRoleUpdated,
			OnDeleting: ctrl.onTenantPodRoleDeleting,
			OnDeleted:  ctrl.onTenantPodRoleDeleted,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	})
	ctrl.recStart = append(ctrl.recStart, roleRec.Start)
	ctrl.recReconcile = append(ctrl.recReconcile, roleRec.Reconcile)

	c.tenantPodRoleReqRec = reconcile.NewCore(ctrl.Context(), reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:podrole_req"),
		BackoffStrategy: nil,
		Workers:         1,
		RequireCache:    false,
		Handlers: reconcile.HandleFuncs{
			OnAdded: ctrl.onTenantPodRoleEnsureRequested,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	}.ResolveNil())
	ctrl.recStart = append(ctrl.recStart, c.tenantPodRoleReqRec.Start)
	ctrl.recReconcile = append(ctrl.recReconcile, c.tenantPodRoleReqRec.Reconcile)

	return nil
}

// nolint:gocyclo
func (c *Controller) checkTenantPodRoleUpToDate(obj *rbacv1.Role) bool {
	crSpec, ok := c.tenantPodRoles[obj.Name]
	if !ok {
		// not managed by this controller
		return true
	}

	if !ok {
		// not managed by us
		return true
	}

	switch {
	case len(obj.Labels) == 0 || obj.Labels[constant.LabelRole] != constant.LabelRoleValuePodRole:
		return false
	case len(obj.Labels) == 0 || obj.Labels[constant.LabelNamespace] != constant.TenantNS():
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
			_, ok := c.getEdgeDeviceObject(podName)
			if ok {
				// ignore virtual pods
				return false
			}

			// retrieve real edge device from pod spec, so get pod spec first
			pod, ok := c.getTenantPodObject(podName)
			if !ok {
				// no pod with this resource name
				return false
			}

			edgeDevice, ok := c.getEdgeDeviceObject(pod.Spec.NodeName)
			if !ok {
				// no edge device is managing this pod, this resource name entry needs to be removed
				return false
			}

			tenantPodRoles := edgeDevice.Spec.Pod.RBAC.RolePermissions

			// check if verbs matched

			if len(tenantPodRoles) == 0 {
				// check default verbs, which is the default option
				// do nothing
			} else {
				// check edgedevice specific roles
				spec, ok := tenantPodRoles[obj.Name]
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

	requiredResourceNames := c.getTenantPodNames()
	if podNames.Len() != len(requiredResourceNames) {
		return false
	}

	return podNames.HasAll(requiredResourceNames...)
}

func (c *Controller) requestTenantPodRoleEnsure() error {
	if c.tenantPodRoleReqRec == nil {
		return nil
	}

	err := c.tenantPodRoleReqRec.Schedule(queue.Job{Action: queue.ActionAdd, Key: ""}, 0)
	if err != nil && !errors.Is(err, queue.ErrJobDuplicated) {
		return fmt.Errorf("failed to schedule pod role update: %w", err)
	}

	return nil
}

func (c *Controller) onTenantPodRoleEnsureRequested(_ interface{}) *reconcile.Result {
	for n := range c.tenantPodRoles {
		err := c.ensureTenantPodRole(n)
		if err != nil {
			c.Log.I("failed to ensure pod role", log.String("name", n), log.Error(err))
			return &reconcile.Result{Err: err}
		}
	}

	return nil
}

func (c *Controller) onTenantPodRoleUpdated(oldObj, newObj interface{}) *reconcile.Result {
	return nil
}

func (c *Controller) onTenantPodRoleDeleting(obj interface{}) *reconcile.Result {
	return nil
}

func (c *Controller) onTenantPodRoleDeleted(obj interface{}) *reconcile.Result {
	return nil
}

func (c *Controller) ensureTenantPodRole(name string) error {
	roleSpec, ok := c.tenantPodRoles[name]
	if !ok {
		return nil
	}

	if !ok {
		// not managed by us, skip
		return nil
	}

	var (
		create bool
		err    error
		role   = c.newTenantPodRoleForAllEdgeDevices(name, roleSpec)
	)

	oldRole, found := c.getTenantRoleObject(name)
	if found {
		if c.checkTenantPodRoleUpToDate(oldRole) {
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
		_, err = c.tenantPodRoleClient.Update(c.Context(), clone, metav1.UpdateOptions{})
		if err != nil {
			if kubeerrors.IsConflict(err) {
				return err
			}

			c.Log.I("failed to update pod role, deleting", log.Error(err))
			err = c.tenantPodRoleClient.Delete(c.Context(), name, *deleteAtOnce)
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
		_, err = c.tenantPodRoleClient.Create(c.Context(), role, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

// nolint:gocyclo
func (c *Controller) newTenantPodRoleForAllEdgeDevices(
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

	for _, obj := range c.tenantPodInformer.GetStore().List() {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			continue
		}

		_, ok = c.getEdgeDeviceObject(pod.Name)
		if ok {
			// ignore virtual pod
			continue
		}

		// is normal pod
		edgeDevice, ok := c.getEdgeDeviceObject(pod.Spec.NodeName)
		if !ok {
			// not managed by us
			continue
		}

		tenantPodRoles := edgeDevice.Spec.Pod.RBAC.RolePermissions
		if tenantPodRoles == nil {
			tenantPodRoles = make(map[string]aranyaapi.PodRolePermissions)
		}

		podPermissions, ok := tenantPodRoles[roleName]
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
			Namespace: constant.TenantNS(),
			Labels: map[string]string{
				constant.LabelRole: constant.LabelRoleValuePodRole,
			},
		},
		Rules: policies,
	}
}

func (c *Controller) getTenantRoleObject(name string) (*rbacv1.Role, bool) {
	obj, found, err := c.tenantPodRoleInformer.GetIndexer().GetByKey(constant.TenantNS() + "/" + name)
	if err != nil || !found {
		role, err := c.tenantPodRoleClient.Get(c.Context(), name, metav1.GetOptions{})
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
