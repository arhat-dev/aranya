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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"arhat.dev/pkg/kubehelper"
	"arhat.dev/pkg/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	csitrans "k8s.io/csi-translation-lib"
	pluginwatcherapi "k8s.io/kubelet/pkg/apis/pluginregistration/v1"
	"k8s.io/kubernetes/pkg/kubelet/config"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/pluginmanager"
	plugincache "k8s.io/kubernetes/pkg/kubelet/pluginmanager/cache"
	kubeletpod "k8s.io/kubernetes/pkg/kubelet/pod"
	kubeletstatus "k8s.io/kubernetes/pkg/kubelet/status"
	volcache "k8s.io/kubernetes/pkg/kubelet/volumemanager/cache"
	"k8s.io/kubernetes/pkg/kubelet/volumemanager/populator"
	"k8s.io/kubernetes/pkg/kubelet/volumemanager/reconciler"
	"k8s.io/kubernetes/pkg/util/removeall"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/csimigration"
	"k8s.io/kubernetes/pkg/volume/util"
	"k8s.io/kubernetes/pkg/volume/util/operationexecutor"
	voltypes "k8s.io/kubernetes/pkg/volume/util/types"
	"k8s.io/kubernetes/pkg/volume/util/volumepathhandler"
	utilstrings "k8s.io/utils/strings"

	"arhat.dev/aranya/pkg/virtualnode/storage/csi"
)

const (
	pollInterval = 300 * time.Millisecond
	pollTimeout  = 2 * time.Minute
)

const (
	volumeReconcilerLoopMinInterval      = time.Second
	volumeReconcilerWaitForAttachTimeout = 10 * time.Minute

	volumeDesiredStateOfWorldMinLoopInterval           = time.Second
	volumeDesiredStateOfWorldGetPodStatusRetryInterval = 4 * time.Second

	volumeCleanupWaitTimeout = 5 * volumeDesiredStateOfWorldMinLoopInterval
)

type Options struct {
	SSHPrivateKey []byte

	StorageRootDir         string
	KubeletRegistrationDir string
	KubeletPluginsDir      string
	CSIDriverLister        *kubehelper.CSIDriverLister
	EventRecorder          record.EventRecorder
}

func NewManager(
	ctx context.Context,
	kubeClient kubeclient.Interface,
	nodeName, hostname, hostIP string,
	runtime kubecontainer.Runtime,
	podManager kubeletpod.Manager,
	podStatusProvider kubeletstatus.PodStatusProvider,
	options *Options,
) *Manager {
	podsDir := filepath.Join(options.StorageRootDir, nodeName)

	return &Manager{
		log: log.Log.WithName(fmt.Sprintf("storage.%s", nodeName)),
		ctx: ctx,

		kubeletRegDir: options.KubeletRegistrationDir,

		podManager:        podManager,
		podStatusProvider: podStatusProvider,
		runtime:           runtime,

		vh: newVolumeHost(
			nodeName, hostname, hostIP, podsDir,
			options.KubeletPluginsDir, kubeClient,
			options.CSIDriverLister, options.EventRecorder,
		),
		mu: new(sync.RWMutex),
	}
}

type Manager struct {
	log log.Interface
	ctx context.Context

	kubeletRegDir string

	podManager        kubeletpod.Manager
	podStatusProvider kubeletstatus.PodStatusProvider
	runtime           kubecontainer.Runtime

	vh *volumeHost
	mu *sync.RWMutex
	// initialized after start
	dswp populator.DesiredStateOfWorldPopulator
	asw  volcache.ActualStateOfWorld
	dsw  volcache.DesiredStateOfWorld
}

func (m *Manager) Start() error {
	err := os.MkdirAll(m.vh.GetPodsDir(), 0750)
	if err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create pods dir: %w", err)
	}

	vpm := new(volume.VolumePluginMgr)

	err = vpm.InitPlugins(m.vh.Plugins(), nil, m.vh)
	if err != nil {
		return fmt.Errorf("failed to init volume plugins: %w", err)
	}

	pluginManager := pluginmanager.NewPluginManager(m.kubeletRegDir, m.vh.GetEventRecorder())

	rh := &csi.RegistrationHandler{CSIDrivers: m.vh.DriverStore(), NodeInfoManager: m.vh.NodeInfoManager()}
	pluginManager.AddHandler(pluginwatcherapi.CSIPlugin, plugincache.PluginHandler(rh))

	// VolumePluginMgr.Run is basically a wrapper for informerFactory.Run for now
	// uncomment this and we need to implement GetInformerFactory
	//vpm.Run(m.ctx.Done())

	sourcesReady := config.NewSourcesReady(func(sourcesSeen sets.String) bool {
		return true
	})
	// parameter sourcesReady is not used, so we can just pass nil
	go pluginManager.Run(sourcesReady, m.ctx.Done())

	asw := volcache.NewActualStateOfWorld(m.vh.GetNodeName(), vpm)
	dsw := volcache.NewDesiredStateOfWorld(vpm)
	intreeToCSITranslator := csitrans.New()
	csiMigratedPluginManager := csimigration.NewPluginManager(intreeToCSITranslator)

	dswp := populator.NewDesiredStateOfWorldPopulator(
		m.vh.GetKubeClient(),
		volumeDesiredStateOfWorldMinLoopInterval,
		volumeDesiredStateOfWorldGetPodStatusRetryInterval,
		m.podManager,
		m.podStatusProvider,
		dsw,
		asw,
		m.runtime,
		false,
		csiMigratedPluginManager,
		intreeToCSITranslator,
	)

	r := reconciler.NewReconciler(
		m.vh.GetKubeClient(),
		false,
		volumeReconcilerLoopMinInterval,
		volumeReconcilerWaitForAttachTimeout,
		m.vh.GetNodeName(),
		dsw,
		asw,
		dswp.HasAddedPods,
		operationexecutor.NewOperationExecutor(
			operationexecutor.NewOperationGenerator(
				m.vh.GetKubeClient(),
				vpm,
				m.vh.GetEventRecorder(),
				false,
				volumepathhandler.NewBlockVolumePathHandler(),
			),
		),
		m.vh.GetMounter(csi.CSIPluginName),
		m.vh.GetHostUtil(),
		vpm,
		m.vh.GetPodsDir(),
	)

	go dswp.Run(sourcesReady, m.ctx.Done())

	m.mu.Lock()
	m.dswp = dswp
	m.asw = asw
	m.dsw = dsw
	m.mu.Unlock()

	r.Run(m.ctx.Done())

	return nil
}

func (m *Manager) Close() error {
	return nil
}

func (m *Manager) Prepare(pod *corev1.Pod, volToMount []corev1.UniqueVolumeName) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.asw == nil || m.dsw == nil || m.dswp == nil {
		return fmt.Errorf("not started")
	}

	if pod == nil {
		return nil
	}

	logger := m.log.WithFields(log.String("action", "prepare"), log.String("podUID", string(pod.UID)))

	mounts, devices := util.GetPodVolumeNames(pod)
	expectedVolumes := mounts.Union(devices).UnsortedList()

	if len(expectedVolumes) == 0 {
		// No volumes to verify
		logger.D("no volume to mount")
		return nil
	}

	m.dsw.MarkVolumesReportedInUse(volToMount)

	uniquePodName := util.GetUniquePodName(pod)

	// Some pods expect to have Setup called over and over again to update.
	// Remount plugins for which this is true. (Atomically updating volumes,
	// like Downward API, depend on this to update the contents of the volume).
	m.dswp.ReprocessPod(uniquePodName)

	logger.D("polling volumes mounted")
	err := wait.PollImmediate(pollInterval, pollTimeout, m.verifyVolumesMountedFunc(uniquePodName, expectedVolumes))
	if err != nil {
		unmountedVolumes := m.getUnmountedVolumes(uniquePodName, expectedVolumes)
		// Also get unattached volumes for error message
		unattachedVolumes := m.getUnattachedVolumes(expectedVolumes)

		if len(unmountedVolumes) == 0 {
			logger.D("all volumes mounted")
			return nil
		}

		logger.I("failed to verify volume mounted",
			log.Error(err),
			log.Any("unmounted", unmountedVolumes),
			log.Any("unattached", unattachedVolumes))

		err = fmt.Errorf(
			"unmounted volumes=%v, unattached volumes=%v: %s",
			unmountedVolumes,
			unattachedVolumes,
			err)
		return err
	}

	logger.D("all volumes mounted")
	return nil
}

// Cleanup pod volumes
func (m *Manager) Cleanup(pod *corev1.Pod) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.asw == nil || m.dsw == nil || m.dswp == nil {
		return fmt.Errorf("not started")
	}

	if pod == nil {
		return nil
	}

	logger := m.log.WithFields(log.String("action", "cleanup"), log.String("podUID", string(pod.UID)))
	uniquePodName := util.GetUniquePodName(pod)

	// TODO: find a better way to make sure volume unmounted
	// we need sleep to wait due to the native of volume reconciler working mode
	time.Sleep(volumeCleanupWaitTimeout)

	logger.D("polling volumes unmounted")
	err := wait.PollImmediate(pollInterval, pollTimeout, m.verifyVolumesUnmountedFunc(uniquePodName, pod.UID))

	if err != nil {
		logger.I("failed to verify volumes unmounted", log.Error(err))
		return err
	}

	logger.D("removing pod dir")
	return removeall.RemoveAllOneFilesystem(m.vh.GetMounter(""), filepath.Join(m.vh.GetPodsDir(), string(pod.UID)))
}

func (m *Manager) GetPersistentVolumeMountPath(podUID types.UID, volumeName string) string {
	return filepath.Join(
		m.vh.GetPodVolumeDir(
			podUID,
			utilstrings.EscapeQualifiedName(m.vh.csiPlugin.GetPluginName()),
			volumeName,
		),
		"mount",
	)
}

// getUnattachedVolumes returns a list of the volumes that are expected to be attached but
// are not currently attached to the node
func (m *Manager) getUnattachedVolumes(expectedVolumes []string) []string {
	var unattachedVolumes []string
	for _, vol := range expectedVolumes {
		if !m.asw.VolumeExists(corev1.UniqueVolumeName(vol)) {
			unattachedVolumes = append(unattachedVolumes, vol)
		}
	}
	return unattachedVolumes
}

func (m *Manager) getUnmountedVolumes(podName voltypes.UniquePodName, expectedVolumes []string) []string {
	mounted := sets.NewString()
	for _, vol := range m.asw.GetMountedVolumesForPod(podName) {
		mounted.Insert(vol.OuterVolumeSpecName)
	}

	var unmountedVolumes []string
	for _, expectedVolume := range expectedVolumes {
		if !mounted.Has(expectedVolume) {
			unmountedVolumes = append(unmountedVolumes, expectedVolume)
		}
	}

	return unmountedVolumes
}

// verifyVolumesMountedFunc returns a method that returns true when all expected
// volumes are mounted.
func (m *Manager) verifyVolumesMountedFunc(
	podName voltypes.UniquePodName, expectedVolumes []string,
) wait.ConditionFunc {
	return func() (done bool, err error) {
		if errs := m.dsw.PopPodErrors(podName); len(errs) > 0 {
			return true, errors.New(strings.Join(errs, "; "))
		}
		return len(m.getUnmountedVolumes(podName, expectedVolumes)) == 0, nil
	}
}

func (m *Manager) verifyVolumesUnmountedFunc(podName voltypes.UniquePodName, podUID types.UID) wait.ConditionFunc {
	podCSIVolumesDir := m.vh.GetPodVolumeDir(
		podUID, utilstrings.EscapeQualifiedName(m.vh.csiPlugin.GetPluginName()), "")

	return func() (done bool, err error) {
		if errs := m.dsw.PopPodErrors(podName); len(errs) > 0 {
			return true, errors.New(strings.Join(errs, "; "))
		}

		files, err := ioutil.ReadDir(podCSIVolumesDir)
		if err != nil {
			return true, err
		}

		if len(files) == 0 {
			return true, nil
		}

		return false, nil
	}
}
