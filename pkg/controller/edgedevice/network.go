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
	"crypto/rand"
	"errors"
	"fmt"

	"arhat.dev/pkg/envhelper"
	"arhat.dev/pkg/kubehelper"
	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	informerscorev1 "k8s.io/client-go/informers/core/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	kubecache "k8s.io/client-go/tools/cache"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/util/ipam"
)

// network management
type networkController struct {
	meshSecretClient clientcorev1.SecretInterface

	meshIPAMv4 *ipam.IPAddressManager
	meshIPAMv6 *ipam.IPAddressManager

	abbotEndpointsInformer kubecache.SharedIndexInformer
	abbotEndpointsRec      *kubehelper.KubeInformerReconciler

	netSvcInformer       kubecache.SharedIndexInformer
	netSvcRec            *kubehelper.KubeInformerReconciler
	netEndpointsInformer kubecache.SharedIndexInformer
	netEndpointsRec      *kubehelper.KubeInformerReconciler

	// nolint:structcheck
	netReqRec *reconcile.Core
}

func (c *networkController) init(
	ctrl *Controller,
	config *conf.Config,
	kubeClient kubeclient.Interface,
	watchInformerFactory informers.SharedInformerFactory,
) error {
	netConf := config.VirtualNode.Network
	if !netConf.Enabled {
		return nil
	}

	c.meshSecretClient = kubeClient.CoreV1().Secrets(envhelper.ThisPodNS())

	if blocks := netConf.Mesh.IPv4Blocks; len(blocks) != 0 {
		c.meshIPAMv4 = ipam.NewIPAddressManager()
		for _, b := range blocks {
			err := c.meshIPAMv4.AddAddressBlock(b.CIDR, b.Start, b.End)
			if err != nil {
				return fmt.Errorf("failed to add ipv4 address block %q: %s", b.CIDR, err)
			}
		}
	}

	if blocks := netConf.Mesh.IPv6Blocks; len(blocks) != 0 {
		c.meshIPAMv6 = ipam.NewIPAddressManager()
		for _, b := range blocks {
			err := c.meshIPAMv6.AddAddressBlock(b.CIDR, b.Start, b.End)
			if err != nil {
				return fmt.Errorf("failed to add ipv6 address block %q: %s", b.CIDR, err)
			}
		}
	}

	// watch abbot endpoints
	c.abbotEndpointsInformer = informerscorev1.New(watchInformerFactory, constant.WatchNS(),
		func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector(
				"metadata.name", config.VirtualNode.Network.AbbotService.Name,
			).String()
		},
	).Endpoints().Informer()
	c.abbotEndpointsRec = kubehelper.NewKubeInformerReconciler(ctrl.Context(), c.abbotEndpointsInformer,
		reconcile.Options{
			Logger:          ctrl.Log.WithName("rec:net:abbot"),
			BackoffStrategy: nil,
			Workers:         1,
			RequireCache:    true,
			Handlers: reconcile.HandleFuncs{
				OnAdded:   nextActionUpdate,
				OnUpdated: ctrl.onAbbotEndpointUpdated,
			},
			OnBackoffStart: nil,
			OnBackoffReset: nil,
		},
	)
	ctrl.recStart = append(ctrl.recStart, c.abbotEndpointsRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, c.abbotEndpointsRec.ReconcileUntil)

	// monitor managed network service
	c.netSvcInformer = informerscorev1.New(watchInformerFactory, constant.WatchNS(),
		func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector(
				"metadata.name", config.VirtualNode.Network.NetworkService.Name,
			).String()
		},
	).Services().Informer()
	c.netSvcRec = kubehelper.NewKubeInformerReconciler(ctrl.Context(), c.netSvcInformer,
		reconcile.Options{
			Logger:          ctrl.Log.WithName("rec:net:svc"),
			BackoffStrategy: nil,
			Workers:         1,
			RequireCache:    true,
			Handlers: reconcile.HandleFuncs{
				OnAdded:   nextActionUpdate,
				OnUpdated: ctrl.onNetworkServiceUpdated,
			},
			OnBackoffStart: nil,
			OnBackoffReset: nil,
		},
	)
	ctrl.recStart = append(ctrl.recStart, c.netSvcRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, c.netSvcRec.ReconcileUntil)

	// monitor managed network service endpoints
	c.netEndpointsInformer = informerscorev1.New(watchInformerFactory, constant.WatchNS(),
		func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector(
				"metadata.name", config.VirtualNode.Network.NetworkService.Name,
			).String()
		},
	).Endpoints().Informer()
	c.netEndpointsRec = kubehelper.NewKubeInformerReconciler(ctrl.Context(), c.netEndpointsInformer,
		reconcile.Options{
			Logger:          ctrl.Log.WithName("rec:net:ep"),
			BackoffStrategy: nil,
			Workers:         1,
			RequireCache:    true,
			Handlers: reconcile.HandleFuncs{
				OnAdded:   nextActionUpdate,
				OnUpdated: ctrl.onNetworkEndpointUpdated,
			},
			OnBackoffStart: nil,
			OnBackoffReset: nil,
		},
	)
	ctrl.recStart = append(ctrl.recStart, c.netEndpointsRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, c.netEndpointsRec.ReconcileUntil)

	// handle EdgeDevice add/delete
	ctrl.netReqRec = reconcile.NewCore(ctrl.Context(), reconcile.Options{
		Logger:          ctrl.Log.WithName("rec:net:req"),
		BackoffStrategy: nil,
		Workers:         1,
		RequireCache:    false,
		Handlers: reconcile.HandleFuncs{
			OnAdded: ctrl.onMeshMemberEnsureRequested,
			OnUpdated: func(old, newObj interface{}) *reconcile.Result {
				return ctrl.onMeshMemberEnsureRequested(newObj)
			},
			OnDeleted: ctrl.onMeshMemberDeleteRequested,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	}.ResolveNil())

	ctrl.recStart = append(ctrl.recStart, ctrl.netReqRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, ctrl.netReqRec.ReconcileUntil)

	return nil
}

func (c *Controller) onAbbotEndpointUpdated(oldObj, newObj interface{}) *reconcile.Result {
	return nil
}

func (c *Controller) onNetworkServiceUpdated(oldObj, newObj interface{}) *reconcile.Result {
	return nil
}

func (c *Controller) onNetworkEndpointUpdated(oldObj, newObj interface{}) *reconcile.Result {
	return nil
}

// check existing abbot endpoints, EdgeDevices' network config (enabled or not)
func (c *Controller) onMeshMemberEnsureRequested(obj interface{}) *reconcile.Result {
	return nil
}

func (c *Controller) onMeshMemberDeleteRequested(obj interface{}) *reconcile.Result {
	return nil
}

func (c *Controller) requestNetworkEnsure(name string) error {
	if c.netReqRec == nil {
		return fmt.Errorf("network ensure not supported")
	}

	c.netReqRec.Update(name, name, name)
	err := c.netReqRec.Schedule(queue.Job{Action: queue.ActionAdd, Key: name}, 0)
	if err != nil && !errors.Is(err, queue.ErrJobDuplicated) {
		return fmt.Errorf("failed to schedule network ensure: %w", err)
	}

	return nil
}

// nolint:unparam
func (c *Controller) ensureMeshConfig(name string, config aranyaapi.NetworkSpec) (*corev1.Secret, error) {
	if !c.vnConfig.Network.Enabled || !config.Enabled {
		return nil, nil
	}

	var (
		secretName = fmt.Sprintf("mesh-config.%s", name)
		create     = false
	)

	meshSecret, err := c.meshSecretClient.Get(c.Context(), secretName, metav1.GetOptions{})
	if err != nil {
		if !kubeerrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to retrieve mesh secret: %w", err)
		}

		create = true
	}

	if create {
		meshSecret, err = newSecretForMesh(secretName)
		if err != nil {
			return nil, fmt.Errorf("failed to generate mesh secret: %w", err)
		}

		meshSecret, err = c.meshSecretClient.Create(c.Context(), meshSecret, metav1.CreateOptions{})
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

		meshSecret, err = c.meshSecretClient.Update(c.Context(), meshSecret, metav1.UpdateOptions{})
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
			Namespace: envhelper.ThisPodNS(),
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
