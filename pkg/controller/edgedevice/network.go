package edgedevice

import (
	"crypto/rand"
	"errors"
	"fmt"

	"arhat.dev/pkg/envhelper"
	corev1 "k8s.io/api/core/v1"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"

	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	informerscorev1 "k8s.io/client-go/informers/core/v1"
	kubecache "k8s.io/client-go/tools/cache"

	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/util/ipam"
)

// network management
type networkController struct {
	meshIPAMv4 *ipam.IPAddressManager
	meshIPAMv6 *ipam.IPAddressManager

	abbotEndpointsInformer kubecache.SharedIndexInformer
	abbotEndpointsRec      *reconcile.KubeInformerReconciler

	netSvcInformer       kubecache.SharedIndexInformer
	netSvcRec            *reconcile.KubeInformerReconciler
	netEndpointsInformer kubecache.SharedIndexInformer
	netEndpointsRec      *reconcile.KubeInformerReconciler

	// nolint:structcheck
	netReqRec *reconcile.Core
}

func (c *networkController) init(
	ctrl *Controller,
	config *conf.Config,
	watchInformerFactory informers.SharedInformerFactory,
) error {
	netConf := config.VirtualNode.Network
	if !netConf.Enabled {
		return nil
	}

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
	c.abbotEndpointsRec = reconcile.NewKubeInformerReconciler(ctrl.Context(), c.abbotEndpointsInformer,
		reconcile.Options{
			Logger:          ctrl.Log.WithName("rec:net:abbot"),
			BackoffStrategy: nil,
			Workers:         1,
			RequireCache:    true,
			Handlers: reconcile.HandleFuncs{
				OnAdded: nil,
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
	).Endpoints().Informer()
	c.netSvcRec = reconcile.NewKubeInformerReconciler(ctrl.Context(), c.netSvcInformer,
		reconcile.Options{
			Logger:          ctrl.Log.WithName("rec:net:svc"),
			BackoffStrategy: nil,
			Workers:         1,
			RequireCache:    true,
			Handlers: reconcile.HandleFuncs{
				OnAdded: nil,
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
	c.netEndpointsRec = reconcile.NewKubeInformerReconciler(ctrl.Context(), c.netEndpointsInformer,
		reconcile.Options{
			Logger:          ctrl.Log.WithName("rec:net:ep"),
			BackoffStrategy: nil,
			Workers:         1,
			RequireCache:    true,
			Handlers: reconcile.HandleFuncs{
				OnAdded: nil,
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
			OnAdded: ctrl.onNetworkEnsureRequested,
			OnUpdated: func(old, newObj interface{}) *reconcile.Result {
				return ctrl.onNetworkEnsureRequested(newObj)
			},
			OnDeleted: ctrl.onNetworkDeleteRequested,
		},
		OnBackoffStart: nil,
		OnBackoffReset: nil,
	}.ResolveNil())

	ctrl.recStart = append(ctrl.recStart, ctrl.netReqRec.Start)
	ctrl.recReconcileUntil = append(ctrl.recReconcileUntil, ctrl.netReqRec.ReconcileUntil)

	return nil
}

// check existing abbot endpoints, EdgeDevices' network config (enabled or not)
func (c *Controller) onNetworkEnsureRequested(obj interface{}) *reconcile.Result {
	return nil
}

func (c *Controller) onNetworkDeleteRequested(obj interface{}) *reconcile.Result {
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
	// TODO: generate and store config for mesh network

	wgPk, err := generateWireguardPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate wireguard private key for %q: %w", name, err)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("mesh-config.%s", name),
			Namespace: envhelper.ThisPodNS(),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			constant.MeshConfigKeyWireguardPrivateKey: wgPk,
		},
	}, nil
}

func generateWireguardPrivateKey() ([]byte, error) {
	const (
		KeyLen = 32
	)

	key := make([]byte, KeyLen)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to read random bytes: %v", err)
	}

	key[0] &= 248
	key[31] &= 127
	key[31] |= 64
	return key, nil
}
