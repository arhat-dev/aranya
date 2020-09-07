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

package storage

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"

	"arhat.dev/pkg/kubehelper"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/kubernetes/pkg/kubelet/config"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/csi/nodeinfomanager"
	"k8s.io/kubernetes/pkg/volume/util/hostutil"
	"k8s.io/kubernetes/pkg/volume/util/subpath"
	utilexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"

	"arhat.dev/aranya/pkg/virtualnode/storage/csi"
)

// type check
var (
	_ csi.VolumeHost = &volumeHost{}
)

func newVolumeHost(
	nodeName, hostname, hostIP, podsDir, kubeletPluginsDir string,
	kubeClient kubeclient.Interface,
	csiDriverLister *kubehelper.CSIDriverLister,
	recorder record.EventRecorder,
) *volumeHost {
	mounterPath, _ := exec.LookPath("mount")
	mounter := mount.New(mounterPath)

	vh := &volumeHost{
		nodeName:   types.NodeName(nodeName),
		kubeClient: kubeClient,

		nim:             nil, // see below
		csiPlugin:       nil, // see below
		driverStore:     new(csi.DriversStore),
		csiDriverLister: csiDriverLister,

		kubeletPluginsDir: kubeletPluginsDir,
		podsDir:           podsDir,
		hostname:          hostname,
		hostIP:            net.ParseIP(hostIP),

		recorder: recorder,

		mounter:   mounter,
		subPather: subpath.New(mounter),
		hu:        hostutil.NewHostUtil(),
		exec:      utilexec.New(),
	}

	vh.nim = nodeinfomanager.NewNodeInfoManager(vh.nodeName, vh, map[string]func() bool{})
	plugins := csi.ProbeVolumePlugins(vh.nim, vh.csiDriverLister, vh.driverStore)
	vh.csiPlugin = plugins[0].(volume.AttachableVolumePlugin)

	return vh
}

type volumeHost struct {
	nodeName   types.NodeName
	kubeClient kubeclient.Interface

	nim             nodeinfomanager.Interface
	csiPlugin       volume.AttachableVolumePlugin
	driverStore     *csi.DriversStore
	csiDriverLister *kubehelper.CSIDriverLister

	kubeletPluginsDir string
	podsDir           string
	hostname          string
	hostIP            net.IP

	recorder record.EventRecorder

	mounter   mount.Interface
	subPather subpath.Interface
	hu        hostutil.HostUtils
	exec      utilexec.Interface
}

func (m *volumeHost) getNode() *corev1.Node {
	n, err := m.kubeClient.CoreV1().Nodes().Get(context.TODO(), string(m.nodeName), metav1.GetOptions{})
	if err != nil {
		return new(corev1.Node)
	}

	return n
}

func (m *volumeHost) Plugins() []volume.VolumePlugin {
	return []volume.VolumePlugin{m.csiPlugin}
}

func (m *volumeHost) DriverStore() *csi.DriversStore {
	return m.driverStore
}

func (m *volumeHost) NodeInfoManager() nodeinfomanager.Interface {
	return m.nim
}

// SetKubeletError lets plugins set an error on the Kubelet runtime status
// that will cause the Kubelet to post NotReady status with the error message provided
func (m *volumeHost) SetKubeletError(err error) {
	// TODO: implement
}

// GetInformerFactory returns the informer factory for CSIDriverLister
func (m *volumeHost) GetInformerFactory() informers.SharedInformerFactory {
	// TODO: if uncommented vpm.Run(m.ctx.Done()), implement this
	return nil
}

// CSIDriverLister returns the informer lister for the CSIDriver API Object
func (m *volumeHost) CSIDriverLister() *kubehelper.CSIDriverLister {
	return m.csiDriverLister
}

// CSIDriverSynced returns the informer synced for the CSIDriver API Object
func (m *volumeHost) CSIDriversSynced() cache.InformerSynced {
	// csi drivers are always synced by controller
	return func() bool {
		return true
	}
}

// WaitForCacheSync is a helper function that waits for cache sync for CSIDriverLister
func (m *volumeHost) WaitForCacheSync() error {
	return nil
}

// Returns hostutil.HostUtils
func (m *volumeHost) GetHostUtil() hostutil.HostUtils {
	return m.hu
}

// GetPluginDir returns the absolute path to a directory under which
// a given plugin may store data.  This directory might not actually
// exist on disk yet.  For plugin data that is per-pod, see
// GetPodPluginDir().
func (m *volumeHost) GetPluginDir(pluginName string) string {
	return filepath.Join(m.kubeletPluginsDir, pluginName)
}

// GetVolumeDevicePluginDir returns the absolute path to a directory
// under which a given plugin may store data.
// ex. plugins/kubernetes.io/{PluginName}/{DefaultKubeletVolumeDevicesDirName}/{volumePluginDependentPath}/
func (m *volumeHost) GetVolumeDevicePluginDir(pluginName string) string {
	return filepath.Join(m.kubeletPluginsDir, pluginName, config.DefaultKubeletVolumeDevicesDirName)
}

// GetPodsDir returns the absolute path to a directory where all the pods
// information is stored
func (m *volumeHost) GetPodsDir() string {
	return m.podsDir
}

// GetPodVolumeDir returns the absolute path a directory which
// represents the named volume under the named plugin for the given
// pod.  If the specified pod does not exist, the result of this call
// might not exist.
func (m *volumeHost) GetPodVolumeDir(podUID types.UID, pluginName, volumeName string) string {
	return filepath.Join(m.podsDir, string(podUID), config.DefaultKubeletVolumesDirName, pluginName, volumeName)
}

// GetPodPluginDir returns the absolute path to a directory under which
// a given plugin may store data for a given pod.  If the specified pod
// does not exist, the result of this call might not exist.  This
// directory might not actually exist on disk yet.
func (m *volumeHost) GetPodPluginDir(podUID types.UID, pluginName string) string {
	return filepath.Join(m.podsDir, string(podUID), config.DefaultKubeletPluginsDirName, pluginName)
}

// GetPodVolumeDeviceDir returns the absolute path a directory which
// represents the named plugin for the given pod.
// If the specified pod does not exist, the result of this call
// might not exist.
// ex. pods/{podUid}/{DefaultKubeletVolumeDevicesDirName}/{escapeQualifiedPluginName}/
func (m *volumeHost) GetPodVolumeDeviceDir(podUID types.UID, pluginName string) string {
	return filepath.Join(m.podsDir, string(podUID), config.DefaultKubeletVolumeDevicesDirName, pluginName)
}

// GetKubeClient returns a client interface
func (m *volumeHost) GetKubeClient() kubeclient.Interface {
	return m.kubeClient
}

// NewWrapperMounter finds an appropriate plugin with which to handle
// the provided spec.  This is used to implement volume plugins which
// "wrap" other plugins.  For example, the "secret" volume is
// implemented in terms of the "emptyDir" volume.
func (m *volumeHost) NewWrapperMounter(volName string, spec volume.Spec, pod *corev1.Pod, opts volume.VolumeOptions) (volume.Mounter, error) {
	hasCSISpec := (spec.PersistentVolume != nil && spec.PersistentVolume.Spec.CSI != nil) ||
		(spec.Volume != nil && spec.Volume.CSI != nil)

	if !hasCSISpec {
		return nil, fmt.Errorf("unable to handle this volume in csi")
	}

	// The name of wrapper volume is set to "wrapped_{wrapped_volume_name}"
	wrapperVolumeName := "wrapped_" + volName
	if spec.Volume != nil {
		spec.Volume.Name = wrapperVolumeName
	}

	return m.csiPlugin.NewMounter(&spec, pod, opts)
}

// NewWrapperUnmounter finds an appropriate plugin with which to handle
// the provided spec.  See comments on NewWrapperMounter for more
// context.
func (m *volumeHost) NewWrapperUnmounter(volName string, spec volume.Spec, podUID types.UID) (volume.Unmounter, error) {
	// The name of wrapper volume is set to "wrapped_{wrapped_volume_name}"
	wrapperVolumeName := "wrapped_" + volName
	if spec.Volume != nil {
		spec.Volume.Name = wrapperVolumeName
	}

	return m.csiPlugin.NewUnmounter(spec.Name(), podUID)
}

// Get cloud provider from kubelet.
func (m *volumeHost) GetCloudProvider() cloudprovider.Interface {
	return nil
}

// Get mounter interface.
func (m *volumeHost) GetMounter(pluginName string) mount.Interface {
	return m.mounter
}

// Returns the hostname of the host kubelet is running on
func (m *volumeHost) GetHostName() string {
	return m.hostname
}

// Returns host IP or nil in the case of error.
func (m *volumeHost) GetHostIP() (net.IP, error) {
	return m.hostIP, nil
}

// Returns node allocatable.
func (m *volumeHost) GetNodeAllocatable() (corev1.ResourceList, error) {
	return m.getNode().Status.Allocatable, nil
}

// Returns a function that returns a secret.
func (m *volumeHost) GetSecretFunc() func(namespace, name string) (*corev1.Secret, error) {
	// TODO: check if used
	return nil
}

// Returns a function that returns a configmap.
func (m *volumeHost) GetConfigMapFunc() func(namespace, name string) (*corev1.ConfigMap, error) {
	// TODO: check if used
	return nil
}

func (m *volumeHost) GetServiceAccountTokenFunc() func(namespace, name string, tr *authv1.TokenRequest) (*authv1.TokenRequest, error) {
	// TODO: check if used
	return nil
}

func (m *volumeHost) DeleteServiceAccountTokenFunc() func(podUID types.UID) {
	// TODO: check if used
	return nil
}

// Returns an interface that should be used to execute any utilities in volume plugins
func (m *volumeHost) GetExec(pluginName string) utilexec.Interface {
	return m.exec
}

// Returns the labels on the node
func (m *volumeHost) GetNodeLabels() (map[string]string, error) {
	return m.getNode().Labels, nil
}

// Returns the name of the node
func (m *volumeHost) GetNodeName() types.NodeName {
	return m.nodeName
}

// Returns the event recorder of kubelet.
func (m *volumeHost) GetEventRecorder() record.EventRecorder {
	return m.recorder
}

// Returns an interface that should be used to execute subpath operations
func (m *volumeHost) GetSubpather() subpath.Interface {
	return m.subPather
}
