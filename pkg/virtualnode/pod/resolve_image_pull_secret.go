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
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/credentialprovider/secrets"
	"k8s.io/kubernetes/pkg/util/parsers"
)

func (m *Manager) resolveImagePullAuthConfig(pod *corev1.Pod) (map[string]*runtimepb.ImageAuthConfig, error) {
	secret := make([]corev1.Secret, len(pod.Spec.ImagePullSecrets))
	for i, secretRef := range pod.Spec.ImagePullSecrets {
		s := m.options.GetSecret(secretRef.Name)
		if s == nil {
			return nil, fmt.Errorf("failed to get secret %q", secretRef.Name)
		}
		secret[i] = *s
	}

	imageNameToAuthConfigMap := make(map[string]*runtimepb.ImageAuthConfig)

	keyring, err := secrets.MakeDockerKeyring(secret, credentialprovider.NewDockerKeyring())
	if err != nil {
		return nil, err
	}

	for _, apiCtr := range pod.Spec.Containers {
		repoToPull, _, _, err := parsers.ParseImageName(apiCtr.Image)
		if err != nil {
			return nil, err
		}

		creds, withCredentials := keyring.Lookup(repoToPull)
		if !withCredentials {
			// pull without credentials
			continue
		}

		for _, currentCreds := range creds {
			imageNameToAuthConfigMap[apiCtr.Image] = &runtimepb.ImageAuthConfig{
				Username:      currentCreds.Username,
				Password:      currentCreds.Password,
				Auth:          currentCreds.Auth,
				ServerAddress: currentCreds.ServerAddress,
				IdentityToken: currentCreds.IdentityToken,
				RegistryToken: currentCreds.RegistryToken,
				Email:         currentCreds.Email,
			}
		}
	}

	return imageNameToAuthConfigMap, nil
}
