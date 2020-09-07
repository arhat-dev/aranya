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

package pod

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
)

func podRequiresRecreation(oldPod, newPod *corev1.Pod) bool {
	if !reflect.DeepEqual(oldPod.Spec.Volumes, newPod.Spec.Volumes) {
		return true
	}

	// init containers, containers cannot be updated for now
	//if !reflect.DeepEqual(oldPod.Spec.InitContainers, newPod.Spec.InitContainers) {
	//	return true
	//}

	//if !reflect.DeepEqual(oldPod.Spec.Containers, newPod.Spec.Containers) {
	//	return true
	//}

	if !reflect.DeepEqual(oldPod.Spec.EphemeralContainers, newPod.Spec.EphemeralContainers) {
		return true
	}

	if !reflect.DeepEqual(oldPod.Spec.SecurityContext, newPod.Spec.SecurityContext) {
		return true
	}

	if oldPod.Spec.RestartPolicy != newPod.Spec.RestartPolicy {
		return true
	}

	if oldPod.Spec.ServiceAccountName != newPod.Spec.ServiceAccountName {
		return true
	}

	if oldPod.Spec.HostNetwork != newPod.Spec.HostNetwork {
		return true
	}

	if oldPod.Spec.HostIPC != newPod.Spec.HostIPC {
		return true
	}

	if oldPod.Spec.HostPID != newPod.Spec.HostPID {
		return true
	}

	if !reflect.DeepEqual(oldPod.Spec.ImagePullSecrets, newPod.Spec.ImagePullSecrets) {
		return true
	}

	if oldPod.Spec.Hostname != newPod.Spec.Hostname {
		return true
	}

	if !reflect.DeepEqual(oldPod.Spec.HostAliases, newPod.Spec.HostAliases) {
		return true
	}

	if !reflect.DeepEqual(oldPod.Spec.DNSConfig, newPod.Spec.DNSConfig) {
		return true
	}

	if oldPod.Spec.DNSPolicy != newPod.Spec.DNSPolicy {
		return true
	}

	return false
}
