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
	"arhat.dev/pkg/patchhelper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (m *Manager) patch(oldPod, newPod *corev1.Pod) (*corev1.Pod, error) {
	err := patchhelper.TwoWayMergePatch(oldPod, newPod, &corev1.Pod{}, func(patchData []byte) error {
		var err error
		newPod, err = m.podClient.Patch(m.Context(), oldPod.Name,
			types.StrategicMergePatchType, patchData, metav1.PatchOptions{})
		return err
	})

	if err != nil {
		return oldPod, err
	}

	return newPod, nil
}
