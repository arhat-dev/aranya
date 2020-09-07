package csi

import (
	"arhat.dev/pkg/kubehelper"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/util/hostutil"
)

// VolumeHost is a AttachDetach Controller specific interface that plugins can use
// to access methods on the Attach Detach Controller.
type VolumeHost interface {
	//volume.KubeletVolumeHost

	// SetKubeletError lets plugins set an error on the Kubelet runtime status
	// that will cause the Kubelet to post NotReady status with the error message provided
	SetKubeletError(err error)
	// GetInformerFactory returns the informer factory for CSIDriverLister
	GetInformerFactory() informers.SharedInformerFactory
	// CSIDriverLister returns the informer lister for the CSIDriver API Object
	CSIDriverLister() *kubehelper.CSIDriverLister
	// CSIDriverSynced returns the informer synced for the CSIDriver API Object
	CSIDriversSynced() cache.InformerSynced
	// WaitForCacheSync is a helper function that waits for cache sync for CSIDriverLister
	WaitForCacheSync() error
	// Returns hostutil.HostUtils
	GetHostUtil() hostutil.HostUtils

	volume.VolumeHost
}
