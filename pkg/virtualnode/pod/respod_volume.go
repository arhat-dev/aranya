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
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/util"

	"arhat.dev/aranya/pkg/constant"
)

type volumeNamePathPair struct {
	name string
	path string
}

func (m *Manager) getUniqueVolumeNamesAndPaths(pod *corev1.Pod) (map[corev1.UniqueVolumeName]volumeNamePathPair, error) {
	volNamePathPairs := make(map[corev1.UniqueVolumeName]volumeNamePathPair)

	mounts, devices := util.GetPodVolumeNames(pod)
	for _, vol := range pod.Spec.Volumes {
		if !mounts.Has(vol.Name) && !devices.Has(vol.Name) {
			// volume/device not used
			continue
		}

		switch {
		case vol.CSI != nil:
			volName, err := m.csiPlugin.GetVolumeName(volume.NewSpecFromVolume(&vol))
			if err != nil {
				return nil, fmt.Errorf("failed to generate volume name with csi plugin: %w", err)
			}

			volNamePathPairs[util.GetUniqueVolumeName(m.csiPlugin.GetPluginName(), volName)] = volumeNamePathPair{
				name: vol.Name,
				path: "",
			}
		case vol.PersistentVolumeClaim != nil:
			pvc, err := m.kubeClient.CoreV1().PersistentVolumeClaims(constant.WatchNS()).Get(m.Context(), vol.PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to get pvc spec: %w", err)
			}

			pv, err := m.kubeClient.CoreV1().PersistentVolumes().Get(m.Context(), pvc.Spec.VolumeName, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to get pv referenced in pvc spec: %w", err)
			}

			volName, err := m.csiPlugin.GetVolumeName(volume.NewSpecFromPersistentVolume(pv, false))
			if err != nil {
				return nil, fmt.Errorf("failed to generate volume name with csi plugin: %w", err)
			}

			volNamePathPairs[util.GetUniqueVolumeName(m.csiPlugin.GetPluginName(), volName)] = volumeNamePathPair{
				name: vol.Name,
				path: m.GetPersistentVolumeMountPath(pod.UID, pvc.Spec.VolumeName),
			}
		}
	}

	return volNamePathPairs, nil
}

func (m *Manager) removeVolumeInUse(oldVolumeInUse []corev1.UniqueVolumeName, oldVolumeAttached []corev1.AttachedVolume, volNames map[corev1.UniqueVolumeName]volumeNamePathPair) ([]corev1.UniqueVolumeName, []corev1.AttachedVolume) {
	var (
		volInUse    []corev1.UniqueVolumeName
		volAttached []corev1.AttachedVolume
	)
	for i, n := range oldVolumeInUse {
		if _, toRemove := volNames[n]; toRemove {
			continue
		}

		volInUse = append(volInUse, oldVolumeInUse[i])
		volAttached = append(volAttached, oldVolumeAttached[i])
	}

	return volInUse, volAttached
}

func (m *Manager) addVolumesInUse(oldVolumeInUse []corev1.UniqueVolumeName, oldVolumeAttached []corev1.AttachedVolume, volNames map[corev1.UniqueVolumeName]volumeNamePathPair) ([]corev1.UniqueVolumeName, []corev1.AttachedVolume) {
	var (
		volumesInUse    []corev1.UniqueVolumeName
		volumesAttached []corev1.AttachedVolume
	)

	for vol := range volNames {
		var marked bool
		for _, volName := range oldVolumeInUse {
			if volName == vol {
				marked = true
				break
			}
		}

		if !marked {
			volumesInUse = append(volumesInUse, vol)
			volumesAttached = append(volumesAttached, corev1.AttachedVolume{Name: vol})
		}
	}

	// append old volume in use
	for i := range oldVolumeInUse {
		volumesInUse = append(volumesInUse, oldVolumeInUse[i])
		volumesAttached = append(volumesAttached, oldVolumeAttached[i])
	}

	sort.Slice(volumesInUse, func(i, j int) bool {
		return volumesInUse[i] < volumesInUse[j]
	})

	sort.Slice(volumesAttached, func(i, j int) bool {
		return volumesAttached[i].Name < volumesAttached[j].Name
	})

	return volumesInUse, volumesAttached
}
