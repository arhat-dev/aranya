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

	"arhat.dev/aranya-proto/aranyagopb/runtimepb"
	corev1 "k8s.io/api/core/v1"
)

// ResolveVolumeData resolves volume data for pod from ConfigMap and Secret
func (m *Manager) resolveVolumeData(pod *corev1.Pod) (volumeData map[string]*runtimepb.NamedData, err error) {
	volumeData = make(map[string]*runtimepb.NamedData)

	for _, vol := range pod.Spec.Volumes {
		switch {
		case vol.ConfigMap != nil:
			optional := vol.ConfigMap.Optional != nil && *vol.ConfigMap.Optional

			configMap := m.options.GetConfigMap(vol.ConfigMap.Name)
			if configMap == nil {
				if optional {
					continue
				}
				return nil, fmt.Errorf("failed to get configmap %q", vol.ConfigMap.Name)
			}

			namedData := &runtimepb.NamedData{DataMap: make(map[string][]byte)}

			for dataName, data := range configMap.Data {
				namedData.DataMap[dataName] = []byte(data)
			}

			for dataName, data := range configMap.BinaryData {
				namedData.DataMap[dataName] = data
			}

			volumeData[vol.Name] = namedData
		case vol.Secret != nil:
			optional := vol.Secret.Optional != nil && *vol.Secret.Optional

			secret := m.options.GetSecret(vol.Secret.SecretName)
			if secret == nil {
				if optional {
					continue
				}
				return nil, fmt.Errorf("failed to get secret %q", vol.Secret.SecretName)
			}

			namedData := &runtimepb.NamedData{DataMap: make(map[string][]byte)}
			for dataName, data := range secret.StringData {
				namedData.DataMap[dataName] = []byte(data)
			}

			for dataName, dataVal := range secret.Data {
				namedData.DataMap[dataName] = dataVal
			}

			volumeData[vol.Name] = namedData
		}
	}

	return volumeData, nil
}
