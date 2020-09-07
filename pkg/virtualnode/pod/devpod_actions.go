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
	"errors"
	"fmt"
	"time"

	"arhat.dev/pkg/log"
	"arhat.dev/pkg/queue"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"arhat.dev/aranya-proto/gopb"
	"arhat.dev/aranya/pkg/constant"
)

// UpdateMirrorPod update Kubernetes pod object status according to device pod status
func (m *Manager) UpdateMirrorPod(pod *corev1.Pod, devicePodStatus *gopb.PodStatus, forInitContainers bool) error {
	logger := m.Log.WithFields(log.String("type", "cloud"), log.String("action", "update"))

	if pod == nil {
		if devicePodStatus != nil {
			var ok bool

			podUID := types.UID(devicePodStatus.Uid)
			pod, ok = m.podCache.GetByID(types.UID(devicePodStatus.Uid))
			if !ok {
				logger.D("device pod not cached, delete")
				m.devPodRec.Update(podUID, nil, podUID)
				err := m.devPodRec.Schedule(queue.Job{Key: podUID, Action: queue.ActionDelete}, 0)
				if err != nil {
					logger.I("failed to schedule pod delete work", log.NamedError("reason", err))
				}
				return nil
			}
		} else {
			return fmt.Errorf("pod cache not found for device pod status")
		}
	}

	if devicePodStatus != nil {
		if forInitContainers {
			_, pod.Status.InitContainerStatuses = resolveContainerStatus(
				pod.Spec.InitContainers, devicePodStatus,
			)
		} else {
			pod.Status.Phase, pod.Status.ContainerStatuses = resolveContainerStatus(
				pod.Spec.Containers, devicePodStatus,
			)
		}

		if devicePodStatus.PodIp != "" {
			pod.Status.PodIP = devicePodStatus.PodIp
		}
		logger.D("resolved device container status", log.String("podIP", pod.Status.PodIP))
	}

	_, err2 := m.UpdatePodStatus(pod)
	if err2 != nil {
		logger.I("failed to update pod status", log.Error(err2))
		return err2
	}
	return nil
}

// CreateDevicePod handle both pod resource resolution and create pod in edge device
// nolint:gocyclo
func (m *Manager) CreateDevicePod(pod *corev1.Pod) error {
	if !m.hasPodCIDR() && !pod.Spec.HostNetwork {
		return fmt.Errorf("pod cidr does not exists")
	}

	envs := make(map[string]map[string]string)
	for i, ctr := range pod.Spec.Containers {
		ctrEnv, err := m.resolveEnv(pod, &pod.Spec.Containers[i])
		if err != nil {
			m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, "FailedToResolveEnv", err.Error())
			return err
		}
		envs[ctr.Name] = ctrEnv
	}

	imagePullAuthConfig, err := m.resolveImagePullAuthConfig(pod)
	if err != nil {
		m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, "FailedToResolveImagePullAuthConfig", err.Error())
		return err
	}

	volumeData, err := m.resolveVolumeData(pod)
	if err != nil {
		m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, "FailedToResolveVolumeData", err.Error())
		return err
	}

	dnsConfig, err := m.resolveDNSSettings(pod)
	if err != nil {
		m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, "FailedToResolveDNSSettings", err.Error())
		return fmt.Errorf("failed to resolve dns settings: %w", err)
	}

	var volNames map[corev1.UniqueVolumeName]volumeNamePathPair
	if m.StorageEnabled() {
		// storage enabled
		volNames, err = m.getUniqueVolumeNamesAndPaths(pod)
		if err != nil {
			return fmt.Errorf("failed to get unique volume names: %w", err)
		}

		if len(volNames) > 0 {
			node := m.options.GetNode()
			if node == nil {
				return fmt.Errorf("failed to get node")
			}

			node.Status.VolumesInUse, node.Status.VolumesAttached = m.addVolumesInUse(
				node.Status.VolumesInUse, node.Status.VolumesAttached, volNames,
			)

			node, err = m.nodeClient.UpdateStatus(m.Context(), node, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update node volume in use: %w", err)
			}

			// mount required volumes
			if err = m.PrepareStorage(keepPVCAndInlineCSIOnly(pod), node.Status.VolumesInUse); err != nil {
				return fmt.Errorf("failed to setup storage: %w", err)
			}
		}
	}

	imgOpts, initOpts, workOpts, initHostExec, workHostExec, err := m.translatePodCreateOptions(
		pod,
		m.netMgr.GetPodCIDR(false),
		m.netMgr.GetPodCIDR(true),
		envs,
		imagePullAuthConfig,
		volumeData,
		volNames,
		dnsConfig,
	)
	if err != nil {
		m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, "FailedToGeneratePodCreationOptions", err.Error())
		return fmt.Errorf("failed to generate pod creation options: %w", err)
	}

	if initOpts == nil {
		// mark pod status ContainerCreating just like what the kubelet will do
		latestPod, err2 := m.getPod(pod.Name)
		if err2 != nil {
			return err2
		}
		pod = latestPod
		pod.Status.Phase, pod.Status.ContainerStatuses = newContainerCreatingStatus(pod)
		pod, err2 = m.UpdatePodStatus(pod)
		if err2 != nil {
			return err2
		}
	}

	// pull images if contains non virtual images
	if len(imgOpts.ImagePull) > 0 {
		podImageEnsureCmd := gopb.NewPodImageEnsureCmd(imgOpts)
		msgCh, _, err2 := m.ConnectivityManager.PostCmd(0, podImageEnsureCmd)
		if err2 != nil {
			m.options.EventRecorder.Event(pod, corev1.EventTypeNormal,
				"FailedToPostImageEnsureCmd", err.Error())
			return err2
		}
		m.options.EventRecorder.Event(pod, corev1.EventTypeNormal,
			"Pulling", "Pulling all images")

		gopb.HandleMessages(msgCh, func(msg *gopb.Msg) (exit bool) {
			if msgErr := msg.GetError(); msgErr != nil {
				m.options.EventRecorder.Event(pod, corev1.EventTypeNormal,
					"ImagePullError", msgErr.Description)
				err = msgErr
				return true
			}

			il := msg.GetImageList()
			if il == nil {
				return true
			}

			if len(il.Images) != 0 {
				m.options.EventRecorder.Event(pod, corev1.EventTypeNormal,
					"Pulled", "Successfully pulled all images")
			}
			return false
		}, nil, gopb.HandleUnknownMessage(m.Log))

		if err != nil {
			return err
		}
	}

	// create init containers if any
	if initOpts != nil {
		// mark pod status PodInitializing just like what the kubelet will do
		latestPod, err2 := m.getPod(pod.Name)
		if err2 != nil {
			return err2
		}
		pod = latestPod
		pod.Status.Phase, pod.Status.InitContainerStatuses = newContainerInitializingStatus(pod)
		pod, err2 = m.UpdatePodStatus(pod)
		if err2 != nil {
			return err2
		}

		if initHostExec {
			// init containers are host exec commands
			podStatus, err3 := m.handleContainerAsHostExec(initOpts)
			if err3 != nil {
				// failed during creating log files, not actual command executed, retry!
				return fmt.Errorf("failed to handle work container as host exec: %w", err3)
			}

			latestPod, err3 = m.getPod(pod.Name)
			if err3 == nil {
				pod = latestPod
			}

			err3 = m.UpdateMirrorPod(pod, podStatus, false)
			if err3 != nil {
				m.Log.I("failed to update work container status for host exec", log.Error(err3))
				// this is not real pod creation and we do not want to retry command execution
				return nil
			}
		} else {
			// normal init containers

			initCreateCmd := gopb.NewPodCreateCmd(initOpts)
			msgCh, _, err3 := m.ConnectivityManager.PostCmd(0, initCreateCmd)
			if err3 != nil {
				m.options.EventRecorder.Event(pod, corev1.EventTypeNormal,
					"PostCreateInitCmdError", err3.Error())
				return err3
			}

			gopb.HandleMessages(msgCh, func(msg *gopb.Msg) (exit bool) {
				if msgErr := msg.GetError(); msgErr != nil {
					m.options.EventRecorder.Event(pod, corev1.EventTypeNormal,
						"RunInitContainerError", msgErr.Description)
					pod.Status.Phase, pod.Status.InitContainerStatuses = newContainerErrorStatus(pod)
					_ = m.UpdateMirrorPod(pod, nil, false)
					err = msgErr
					return true
				}

				ps := msg.GetPodStatus()
				if ps == nil {
					return true
				}

				m.options.EventRecorder.Event(pod, corev1.EventTypeNormal,
					"PodInitialized", "Init container(s) created and finished")
				_ = m.UpdateMirrorPod(pod, ps, true)
				return false
			}, nil, gopb.HandleUnknownMessage(m.Log))

			if err != nil {
				return err
			}
		}
	}

	// mark pod status ContainerCreating just like what the kubelet will do
	latestPod, err := m.getPod(pod.Name)
	if err != nil {
		return err
	}
	pod = latestPod
	pod.Status.Phase, pod.Status.ContainerStatuses = newContainerCreatingStatus(pod)
	pod, err = m.UpdatePodStatus(pod)
	if err != nil {
		return err
	}

	if workHostExec {
		// work containers are host exec commands
		podStatus, err2 := m.handleContainerAsHostExec(workOpts)
		if err2 != nil {
			// failed during creating log files, not actual command executed, retry!
			return fmt.Errorf("failed to handle work container as host exec: %w", err2)
		}

		latestPod, err2 = m.getPod(pod.Name)
		if err2 == nil {
			pod = latestPod
		}

		err2 = m.UpdateMirrorPod(pod, podStatus, false)
		if err2 != nil {
			m.Log.I("failed to update work container status for host exec", log.Error(err2))
			// this is not real pod creation and we do not want to retry command execution
			return nil
		}
	} else {
		// create work containers
		workCreateCmd := gopb.NewPodCreateCmd(workOpts)
		msgCh, _, err2 := m.ConnectivityManager.PostCmd(0, workCreateCmd)
		if err2 != nil {
			m.options.EventRecorder.Event(pod, corev1.EventTypeNormal,
				"PostCreateWorkCmdError", err2.Error())
			return err2
		}

		gopb.HandleMessages(msgCh, func(msg *gopb.Msg) (exit bool) {
			if msgErr := msg.GetError(); msgErr != nil {
				if msgErr.Kind == gopb.ERR_ALREADY_EXISTS {
					m.options.EventRecorder.Event(pod, corev1.EventTypeNormal,
						"Started", "Reused existing container(s)")
				} else {
					m.options.EventRecorder.Event(pod, corev1.EventTypeNormal,
						"RunContainerError", msgErr.Description)

					pod.Status.Phase, pod.Status.ContainerStatuses = newContainerErrorStatus(pod)
					_ = m.UpdateMirrorPod(pod, nil, false)

					err = msgErr
				}

				return true
			}

			ps := msg.GetPodStatus()
			if ps == nil {
				return true
			}

			m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, "Started", "Container(s) created and started")
			_ = m.UpdateMirrorPod(pod, ps, false)
			return false
		}, nil, gopb.HandleUnknownMessage(m.Log))
	}

	// do not return nil here, we may have collected err from message channel
	return err
}

// DeleteDevicePod delete pod in edge device
func (m *Manager) DeleteDevicePod(podUID types.UID) error {
	var (
		graceTime    = time.Duration(0)
		preStopHooks map[string]*gopb.ActionMethod
	)

	// usually we should have pod object for pod deletion, but when we are trying to delete a stale pod
	// unknown to the cluster (deployed in the remote device before, but somehow we lost the connection
	// to the device and the pod got deleted in cluster later), there will be no pod cache at all
	podToDelete, hasPodObj := m.podCache.GetByID(podUID)
	if hasPodObj {
		m.options.EventRecorder.Event(podToDelete, corev1.EventTypeNormal, "Killing", "Killing containers")

		if g := podToDelete.Spec.TerminationGracePeriodSeconds; g != nil && *g > 0 {
			graceTime = time.Duration(*g) * time.Second
		}

		// collect pre-stop hook spec
		preStopHooks = make(map[string]*gopb.ActionMethod)
		for i, ctr := range podToDelete.Spec.Containers {
			// ignore pods using virtual images, they do not actually create containers in device
			if ctr.Image == constant.VirtualImageNameHostExec {
				return nil
			}

			if ctr.Lifecycle != nil && ctr.Lifecycle.PreStop != nil {
				preStopHooks[ctr.Name] = translateHandler(
					ctr.Lifecycle.PreStop,
					getNamedContainerPorts(&podToDelete.Spec.Containers[i]),
				)
			}
		}
	}

	// post cmd anyway, check pod cache when processing message
	podDeleteCmd := gopb.NewPodDeleteCmd(string(podUID), graceTime, preStopHooks)
	msgCh, _, err := m.ConnectivityManager.PostCmd(0, podDeleteCmd)
	if err != nil {
		if hasPodObj {
			m.options.EventRecorder.Event(podToDelete, corev1.EventTypeNormal, "PostDeleteCmdError", err.Error())
		}
		return err
	}

	gopb.HandleMessages(msgCh, func(msg *gopb.Msg) (exit bool) {
		if msgErr := msg.GetError(); msgErr != nil {
			switch msgErr.Kind {
			case gopb.ERR_NOT_SUPPORTED:
				// ignore
				err = nil
			case gopb.ERR_NOT_FOUND:
				// successful
				err = nil
				if hasPodObj {
					m.options.EventRecorder.Event(podToDelete, corev1.EventTypeNormal, "Deleted", msgErr.Description)
				}
			default:
				if hasPodObj {
					m.options.EventRecorder.Event(podToDelete, corev1.EventTypeNormal,
						"DeletePodError", msgErr.Description)
				}
				err = errors.New(msgErr.Description)
			}
			return true
		}

		if hasPodObj && err == nil {
			m.options.EventRecorder.Event(podToDelete, corev1.EventTypeNormal, "Killed", "Delete pod succeeded")
		}
		return false
	}, nil, gopb.HandleUnknownMessage(m.Log))

	return err
}

// SyncDevicePods sync all pods in device with Kubernetes cluster
func (m *Manager) SyncDevicePods() error {
	logger := m.Log.WithFields(log.String("type", "device"), log.String("action", "sync"))

	logger.I("syncing device pods")
	msgCh, _, err := m.ConnectivityManager.PostCmd(0, gopb.NewPodListCmd("", "", true))
	if err != nil {
		logger.I("failed to post pod list cmd", log.Error(err))
		return err
	}

	var (
		volumeInUse    []corev1.UniqueVolumeName
		volumeAttached []corev1.AttachedVolume
	)

	gopb.HandleMessages(msgCh, func(msg *gopb.Msg) (exit bool) {
		if msgErr := msg.GetError(); msgErr != nil {
			logger.I("failed to list device pods", log.Error(msgErr))
			err = msgErr
			return true
		}

		psl := msg.GetPodStatusList()
		if psl == nil {
			logger.I("unexpected non pod status list")
			return true
		}

		pods := psl.Pods

		for _, ps := range pods {
			podUID := types.UID(ps.GetUid())
			if podUID == "" {
				logger.I("device pod uid empty, discard")
				return true
			}

			pod, ok := m.podCache.GetByID(podUID)
			if !ok {
				logger.D("device pod not cached, delete")
				// no pod cache means no mirror pod for it, should be deleted
				m.devPodRec.Update(podUID, nil, podUID)
				err2 := m.devPodRec.Schedule(queue.Job{Key: podUID, Action: queue.ActionDelete}, 0)
				if err2 != nil {
					logger.I("failed to schedule pod delete work", log.NamedError("reason", err2))
				}
				return true
			}

			logger.I("device pod exists, update status")
			_ = m.UpdateMirrorPod(pod, ps, false)

			volNames, err2 := m.getUniqueVolumeNamesAndPaths(pod)
			if err2 != nil {
				err = fmt.Errorf("failed to get unique volume names: %w", err2)
				return true
			}

			if len(volNames) > 0 {
				volumeInUse, volumeAttached = m.addVolumesInUse(volumeInUse, volumeAttached, volNames)
			}
		}

		return false
	}, nil, gopb.HandleUnknownMessage(logger))

	if err != nil {
		return err
	}

	if len(volumeInUse) > 0 {
		node := m.options.GetNode()
		if node == nil {
			return fmt.Errorf("failed to get node")
		}

		node.Status.VolumesInUse = volumeInUse
		node.Status.VolumesAttached = volumeAttached

		_, err = m.nodeClient.UpdateStatus(m.Context(), node, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update node volume in use: %w", err)
		}
	}

	return err
}
