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
	corev1 "k8s.io/api/core/v1"
)

func hasStringInSlice(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}

func (m *Manager) ensureFinalizer(pod *corev1.Pod, finalizer string) (*corev1.Pod, error) {
	if hasStringInSlice(pod.ObjectMeta.Finalizers, finalizer) {
		return pod, nil
	}

	newPod := pod.DeepCopy()
	newPod.Finalizers = append(newPod.Finalizers, finalizer)

	return m.patch(pod, newPod)
}

func (m *Manager) removeFinalizer(pod *corev1.Pod, finalizer string) (*corev1.Pod, error) {
	if !hasStringInSlice(pod.ObjectMeta.Finalizers, finalizer) {
		return pod, nil
	}

	var finalizers []string
	for _, f := range pod.Finalizers {
		if f == finalizer {
			continue
		}

		finalizers = append(finalizers, f)
	}
	newPod := pod.DeepCopy()
	newPod.Finalizers = finalizers

	return m.patch(pod, newPod)
}
