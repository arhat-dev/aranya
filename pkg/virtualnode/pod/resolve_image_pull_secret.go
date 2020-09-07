package pod

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/credentialprovider/secrets"
	"k8s.io/kubernetes/pkg/util/parsers"

	"arhat.dev/aranya-proto/gopb"
)

func (m *Manager) resolveImagePullAuthConfig(pod *corev1.Pod) (map[string]*gopb.AuthConfig, error) {
	secret := make([]corev1.Secret, len(pod.Spec.ImagePullSecrets))
	for i, secretRef := range pod.Spec.ImagePullSecrets {
		s := m.options.GetSecret(secretRef.Name)
		if s == nil {
			return nil, fmt.Errorf("failed to get secret %q", secretRef.Name)
		}
		secret[i] = *s
	}

	imageNameToAuthConfigMap := make(map[string]*gopb.AuthConfig)

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
			imageNameToAuthConfigMap[apiCtr.Image] = &gopb.AuthConfig{
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
