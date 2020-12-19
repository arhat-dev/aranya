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
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"arhat.dev/pkg/encodehelper"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	kubeclient "k8s.io/client-go/kubernetes"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	kubecache "k8s.io/client-go/tools/cache"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
)

type sysSecretController struct {
	sysSecretCtx       context.Context
	meshNetworkEnabled bool

	sysSecretClient clientcorev1.SecretInterface

	sysSecretInformer kubecache.SharedIndexInformer
}

func (c *sysSecretController) init(
	ctrl *Controller,
	config *conf.Config,
	kubeClient kubeclient.Interface,
	sysInformerFactory informers.SharedInformerFactory,
) error {
	c.sysSecretCtx = ctrl.Context()
	c.meshNetworkEnabled = config.VirtualNode.Network.Enabled

	c.sysSecretClient = kubeClient.CoreV1().Secrets(constant.SysNS())
	c.sysSecretInformer = sysInformerFactory.Core().V1().Secrets().Informer()

	ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs, c.sysSecretInformer.HasSynced)
	ctrl.listActions = append(ctrl.listActions, func() error {
		_, err := listerscorev1.NewSecretLister(c.sysSecretInformer.GetIndexer()).List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list secrets in namespace %q: %w", constant.SysNS(), err)
		}

		return nil
	})

	return nil
}

// nolint:unparam
func (c *sysSecretController) ensureMeshConfig(name string, config aranyaapi.NetworkSpec) (*corev1.Secret, error) {
	if !c.meshNetworkEnabled || !config.Enabled {
		return nil, nil
	}

	var (
		secretName = fmt.Sprintf("mesh-config.%s", name)
		create     = false
	)

	meshSecret, err := c.sysSecretClient.Get(c.sysSecretCtx, secretName, metav1.GetOptions{})
	if err != nil {
		if !kubeerrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to check existing mesh secret: %w", err)
		}

		create = true
	}

	if create {
		meshSecret, err = newSecretForMesh(secretName)
		if err != nil {
			return nil, fmt.Errorf("failed to generate mesh secret: %w", err)
		}

		meshSecret, err = c.sysSecretClient.Create(c.sysSecretCtx, meshSecret, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create mesh secret: %w", err)
		}

		return meshSecret, nil
	}

	// check and update

	update := false
	wgPk, ok := accessMap(meshSecret.StringData, meshSecret.Data, constant.MeshConfigKeyWireguardPrivateKey)
	if !ok || len(wgPk) != constant.WireguardKeyLength {
		update = true
	}

	if update {
		var newSecret *corev1.Secret
		newSecret, err = newSecretForMesh(secretName)
		if err != nil {
			return nil, fmt.Errorf("failed to generate mesh secret for update: %w", err)
		}

		meshSecret.Data = newSecret.Data
		meshSecret.StringData = newSecret.StringData

		meshSecret, err = c.sysSecretClient.Update(c.sysSecretCtx, meshSecret, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to update invalid mesh secret: %w", err)
		}
	}

	return meshSecret, nil
}

func newSecretForMesh(secretName string) (*corev1.Secret, error) {
	wgPk, err := generateWireguardPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate wireguard private key: %w", err)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: constant.SysNS(),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			constant.MeshConfigKeyWireguardPrivateKey: wgPk,
		},
	}, nil
}

func generateWireguardPrivateKey() ([]byte, error) {
	key := make([]byte, constant.WireguardKeyLength)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to read random bytes: %v", err)
	}

	key[0] &= 248
	key[31] &= 127
	key[31] |= 64
	return key, nil
}

// nolint:deadcode,unused
func generateOpenSSHKeyPair() (sshPrivateKey []byte, _ ssh.PublicKey, err error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate ed25519 key pair: %w", err)
	}

	publicKey, _ := ssh.NewPublicKey(pubKey)
	// TODO: send ssh private key
	sshPrivateKey = pem.EncodeToMemory(&pem.Block{
		Type:  "OPENSSH PRIVATE KEY",
		Bytes: encodehelper.MarshalED25519PrivateKeyForOpenSSH(privKey),
	})
	_ = sshPrivateKey

	return sshPrivateKey, publicKey, nil
}

func (c *sysSecretController) getUserPassFromSecret(name string) (username, password []byte, err error) {
	secret, ok := c.getSysSecretObject(name)
	if !ok {
		return nil, nil, fmt.Errorf("failed to get secret %q", name)
	}

	if secret.Type != corev1.SecretTypeBasicAuth {
		return nil, nil, fmt.Errorf("non basic auth secret found by userPassRef")
	}

	return secret.Data[corev1.BasicAuthUsernameKey], secret.Data[corev1.BasicAuthPasswordKey], nil
}

func (c *sysSecretController) createTLSConfigFromSysSecret(
	tlsSecretRef *corev1.LocalObjectReference,
) (*tls.Config, error) {
	// no tls required
	if tlsSecretRef == nil {
		return nil, nil
	}

	// use system default tls config
	if tlsSecretRef.Name == "" {
		return &tls.Config{}, nil
	}

	tlsSecret, ok := c.getSysSecretObject(tlsSecretRef.Name)
	if !ok {
		return nil, fmt.Errorf("failed to get secret %q", tlsSecretRef.Name)
	}

	_, insecureSkipVerify := tlsSecret.Data["insecureSkipVerify"]
	caPEM, hasCACert := tlsSecret.Data[constant.TLSCaCertKey]
	certPEM, hasCert := tlsSecret.Data[corev1.TLSCertKey]
	keyPEM, hasKey := tlsSecret.Data[corev1.TLSPrivateKeyKey]
	clientCaPEM, hasClientCACert := tlsSecret.Data["client_ca.crt"]

	tlsConfig := &tls.Config{
		ServerName:         string(tlsSecret.Data["serverName"]),
		InsecureSkipVerify: insecureSkipVerify,
	}

	if hasCACert {
		tlsConfig.RootCAs = x509.NewCertPool()
		tlsConfig.RootCAs.AppendCertsFromPEM(caPEM)
	}

	if hasClientCACert {
		tlsConfig.ClientCAs = x509.NewCertPool()
		tlsConfig.ClientCAs.AppendCertsFromPEM(clientCaPEM)
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	if hasCert {
		if hasKey {
			cert, err := tls.X509KeyPair(certPEM, keyPEM)
			if err != nil {
				return nil, fmt.Errorf("failed to load x509 key pair: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		} else {
			certPool := x509.NewCertPool()
			if !certPool.AppendCertsFromPEM(certPEM) {
				return nil, fmt.Errorf("failed to append certificates")
			}
			tlsConfig.RootCAs = certPool
		}
	}

	return tlsConfig, nil
}

func (c *sysSecretController) getSysSecretObject(name string) (*corev1.Secret, bool) {
	obj, found, err := c.sysSecretInformer.GetIndexer().GetByKey(constant.SysNS() + "/" + name)
	if err != nil || !found {
		return nil, false
	}

	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil, false
	}

	return secret, true
}
