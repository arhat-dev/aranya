package pod

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"arhat.dev/aranya-proto/aranyagopb"
)

// ResolveVolumeData resolves volume data for pod from ConfigMap and Secret
func (m *Manager) resolveVolumeData(pod *corev1.Pod) (volumeData map[string]*aranyagopb.NamedData, err error) {
	volumeData = make(map[string]*aranyagopb.NamedData)

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

			namedData := &aranyagopb.NamedData{DataMap: make(map[string][]byte)}

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

			namedData := &aranyagopb.NamedData{DataMap: make(map[string][]byte)}
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
