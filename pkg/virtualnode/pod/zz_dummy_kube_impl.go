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
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/flowcontrol"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/pod"
	"k8s.io/kubernetes/pkg/kubelet/status"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"

	"arhat.dev/aranya/pkg/util/cache"
)

func (m *Manager) KubeRuntimeForVolumeManager() kubecontainer.Runtime {
	return &runtimeForVolumeManager{
		store: m.podCache,
	}
}

func (m *Manager) KubePodManagerForVolumeManager() pod.Manager {
	return &podManagerForVolumeManager{
		store: m.podCache,
	}
}

func (m *Manager) KubePodStatusProviderForVolumeManager() status.PodStatusProvider {
	return &podStatusProviderForVolumeManager{
		store: m.podCache,
	}
}

type podStatusProviderForVolumeManager struct {
	store *cache.PodCache
}

func (m *podStatusProviderForVolumeManager) GetPodStatus(uid types.UID) (corev1.PodStatus, bool) {
	po, ok := m.store.GetByID(uid)
	if !ok {
		return corev1.PodStatus{}, false
	}

	// default to running as we need volume being mounted for remote access
	phase := corev1.PodRunning
	switch po.Status.Phase {
	case corev1.PodFailed, corev1.PodSucceeded:
		phase = po.Status.Phase
	}

	// here we don't report real status to make sure volumes are prepared
	// DesiredStateOfWorldPopulator only checks pod Phase and container status
	result := &corev1.PodStatus{Phase: phase}
	if !m.store.IsDeletedInRemote(uid) {
		// if no containerStatus provided or provided with non-nil state.Waiting
		// or state.Terminated, DesiredStateOfWorldPopulator will mark this pod
		// terminated and check runtime provided pod info, we want this behavior
		// only for pods has been deleted in remote device
		result.ContainerStatuses = make([]corev1.ContainerStatus, 1)
	}

	return *result, true
}

type runtimeForVolumeManager struct {
	store *cache.PodCache
}

// GetPods is the only method used by volume manager in Runtime interface
// the caller only checks its ID and the count of containers
func (m *runtimeForVolumeManager) GetPods(all bool) ([]*kubecontainer.Pod, error) {
	// all is always false and is meant to show running pods only
	var pods []*kubecontainer.Pod
	for _, po := range m.store.GetAll() {
		if m.store.IsDeletedInRemote(po.UID) {
			// pod already deleted in remote device, we no longer need its volumes
			continue
		}

		pods = append(pods, &kubecontainer.Pod{ID: po.UID, Containers: make([]*kubecontainer.Container, 1)})
	}

	return pods, nil
}

func (m *runtimeForVolumeManager) SupportsSingleFileMapping() bool                 { return true }
func (m *runtimeForVolumeManager) Type() string                                    { return "" }
func (m *runtimeForVolumeManager) Version() (kubecontainer.Version, error)         { return nil, nil }
func (m *runtimeForVolumeManager) APIVersion() (kubecontainer.Version, error)      { return nil, nil }
func (m *runtimeForVolumeManager) Status() (*kubecontainer.RuntimeStatus, error)   { return nil, nil }
func (m *runtimeForVolumeManager) UpdatePodCIDR(podCIDR string) error              { return nil }
func (m *runtimeForVolumeManager) ListImages() ([]kubecontainer.Image, error)      { return nil, nil }
func (m *runtimeForVolumeManager) RemoveImage(image kubecontainer.ImageSpec) error { return nil }
func (m *runtimeForVolumeManager) ImageStats() (*kubecontainer.ImageStats, error)  { return nil, nil }
func (m *runtimeForVolumeManager) GarbageCollect(gcPolicy kubecontainer.ContainerGCPolicy, allSourcesReady bool, evictNonDeletedPods bool) error {
	return nil
}
func (m *runtimeForVolumeManager) SyncPod(pod *corev1.Pod, podStatus *kubecontainer.PodStatus, pullSecrets []corev1.Secret, backOff *flowcontrol.Backoff) kubecontainer.PodSyncResult {
	return kubecontainer.PodSyncResult{}
}
func (m *runtimeForVolumeManager) KillPod(pod *corev1.Pod, runningPod kubecontainer.Pod, gracePeriodOverride *int64) error {
	return nil
}
func (m *runtimeForVolumeManager) GetPodStatus(uid types.UID, name, namespace string) (*kubecontainer.PodStatus, error) {
	return nil, nil
}
func (m *runtimeForVolumeManager) GetContainerLogs(ctx context.Context, pod *corev1.Pod, containerID kubecontainer.ContainerID, logOptions *corev1.PodLogOptions, stdout, stderr io.Writer) (err error) {
	return nil
}
func (m *runtimeForVolumeManager) DeleteContainer(containerID kubecontainer.ContainerID) error {
	return nil
}
func (m *runtimeForVolumeManager) PullImage(image kubecontainer.ImageSpec, pullSecrets []corev1.Secret, podSandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	return "", nil
}
func (m *runtimeForVolumeManager) GetImageRef(image kubecontainer.ImageSpec) (string, error) {
	return "", nil
}

type podManagerForVolumeManager struct {
	store *cache.PodCache
}

func (m *podManagerForVolumeManager) GetPods() []*corev1.Pod {
	var pods []*corev1.Pod
	for _, po := range m.store.GetAll() {
		pods = append(pods, keepPVCAndInlineCSIOnly(po))
	}
	return pods
}

func (m *podManagerForVolumeManager) GetPodByUID(podUID types.UID) (*corev1.Pod, bool) {
	po, ok := m.store.GetByID(podUID)
	if !ok {
		return nil, false
	}
	return keepPVCAndInlineCSIOnly(po), false
}

func (m *podManagerForVolumeManager) CreateMirrorPod(pod *corev1.Pod) error         { return nil }
func (m *podManagerForVolumeManager) IsMirrorPodOf(mirrorPod, pod *corev1.Pod) bool { return false }
func (m *podManagerForVolumeManager) SetPods(pods []*corev1.Pod)                    {}
func (m *podManagerForVolumeManager) AddPod(pod *corev1.Pod)                        {}
func (m *podManagerForVolumeManager) UpdatePod(pod *corev1.Pod)                     {}
func (m *podManagerForVolumeManager) DeletePod(pod *corev1.Pod)                     {}
func (m *podManagerForVolumeManager) DeleteOrphanedMirrorPods()                     {}
func (m *podManagerForVolumeManager) TranslatePodUID(uid types.UID) kubetypes.ResolvedPodUID {
	return ""
}
func (m *podManagerForVolumeManager) GetPodByFullName(podFullName string) (*corev1.Pod, bool) {
	return nil, false
}
func (m *podManagerForVolumeManager) GetPodByName(namespace, name string) (*corev1.Pod, bool) {
	return nil, false
}
func (m *podManagerForVolumeManager) GetUIDTranslations() (podToMirror map[kubetypes.ResolvedPodUID]kubetypes.MirrorPodUID, mirrorToPod map[kubetypes.MirrorPodUID]kubetypes.ResolvedPodUID) {
	return
}
func (m *podManagerForVolumeManager) DeleteMirrorPod(podFullName string, uid *types.UID) (bool, error) {
	return false, nil
}
func (m *podManagerForVolumeManager) GetPodByMirrorPod(*corev1.Pod) (*corev1.Pod, bool) {
	return nil, false
}
func (m *podManagerForVolumeManager) GetMirrorPodByPod(*corev1.Pod) (*corev1.Pod, bool) {
	return nil, false
}
func (m *podManagerForVolumeManager) GetPodsAndMirrorPods() ([]*corev1.Pod, []*corev1.Pod) {
	return nil, nil
}

func keepPVCAndInlineCSIOnly(p *corev1.Pod) *corev1.Pod {
	po := p.DeepCopy()

	var (
		volumes    []corev1.Volume
		validNames = make(map[string]struct{})
	)
	for i, vol := range po.Spec.Volumes {
		switch {
		case vol.PersistentVolumeClaim != nil, vol.CSI != nil:
			validNames[vol.Name] = struct{}{}
			volumes = append(volumes, po.Spec.Volumes[i])
		}
	}

	po.Spec.Volumes = volumes

	for i, ctr := range po.Spec.InitContainers {
		var mounts []corev1.VolumeMount
		for j, vm := range ctr.VolumeMounts {
			if _, valid := validNames[vm.Name]; valid {
				// remove unwanted
				mounts = append(mounts, po.Spec.Containers[i].VolumeMounts[j])
			}
		}

		po.Spec.Containers[i].VolumeMounts = mounts
	}

	for i, ctr := range po.Spec.Containers {
		var mounts []corev1.VolumeMount
		for j, vm := range ctr.VolumeMounts {
			if _, valid := validNames[vm.Name]; valid {
				// remove unwanted
				mounts = append(mounts, po.Spec.Containers[i].VolumeMounts[j])
			}
		}

		po.Spec.Containers[i].VolumeMounts = mounts
	}

	return po
}
