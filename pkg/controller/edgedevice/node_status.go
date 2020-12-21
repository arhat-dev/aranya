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
	"sort"
	"strings"
	"time"

	"arhat.dev/pkg/log"
	"arhat.dev/pkg/patchhelper"
	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeletapis "k8s.io/kubernetes/pkg/kubelet/apis"

	"arhat.dev/aranya/pkg/constant"
)

// nolint:gocyclo
func (c *Controller) onNodeStatusUpdated(oldObj, newObj interface{}) (ret *reconcile.Result) {
	var (
		node   = newObj.(*corev1.Node).DeepCopy()
		name   = node.Name
		logger = c.Log.WithFields(log.String("name", name))
	)

	vn, ignore := c.doNodeResourcePreCheck(name)
	if ignore {
		logger.V("node status update ignored")
		return nil
	}

	expectedStatus := vn.ActualNodeStatus(node.Status)
	extLabels, extAnnotations := vn.ExtInfo()

	// check its labels and annotations
	var (
		arch   = expectedStatus.NodeInfo.Architecture
		os     = expectedStatus.NodeInfo.OperatingSystem
		goArch = convertToGOARCH(arch)
	)

	for k, v := range map[string]string{
		constant.LabelArch:     arch,
		corev1.LabelArchStable: goArch,
		kubeletapis.LabelArch:  goArch,
		corev1.LabelOSStable:   os,
		kubeletapis.LabelOS:    os,
	} {
		// MUST not override these labels in ext info
		extLabels[k] = v
	}

	for k, v := range node.Labels {
		if extLabels[k] == v {
			delete(extLabels, k)
		}
	}

	for k, v := range node.Annotations {
		if extAnnotations[k] == v {
			delete(extAnnotations, k)
		}
	}

	if len(extLabels) != 0 || len(extAnnotations) != 0 {
		if logger.Enabled(log.LevelVerbose) {
			var (
				labels      []string
				annotations []string
			)

			for k, v := range extLabels {
				labels = append(labels, k+"="+v)
			}

			for k, v := range extAnnotations {
				annotations = append(annotations, k+"="+v)
			}

			sort.Strings(labels)
			sort.Strings(annotations)

			logger.V("node metadata requires update",
				log.Strings("labels", labels),
				log.Strings("annotations", annotations),
			)
		}

		updateNode := node.DeepCopy()
		if updateNode.Labels == nil {
			updateNode.Labels = extLabels
		} else {
			for k, v := range extLabels {
				updateNode.Labels[k] = v
			}
		}

		if updateNode.Annotations == nil {
			updateNode.Annotations = extAnnotations
		} else {
			for k, v := range extAnnotations {
				updateNode.Annotations[k] = v
			}
		}

		// need to update metadata first, update node status next round
		logger.V("updating node metadata")
		err := patchhelper.TwoWayMergePatch(node, updateNode, &corev1.Node{}, func(patchData []byte) error {
			_, err2 := c.nodeClient.Patch(
				c.Context(), name, types.StrategicMergePatchType, patchData, metav1.PatchOptions{},
			)
			return err2
		})
		if err != nil {
			logger.I("failed to update node metadata", log.Error(err))
			return &reconcile.Result{Err: err}
		}
		logger.V("node metadata updated")
	}

	var (
		now = metav1.Now()

		updateStatus = false
	)
	// ensure node status up to date
	if vn.Connected() {
		expectedStatus.Allocatable = getAllocatable(expectedStatus.Capacity, c.getTenantPodsForNode(name))

		if !checkNodeResourcesEqual(expectedStatus.Capacity, node.Status.Capacity) {
			updateStatus = true
			logger.V("node capacity requires updated")
			node.Status.Capacity = expectedStatus.Capacity
		}

		if !checkNodeResourcesEqual(expectedStatus.Allocatable, node.Status.Allocatable) {
			updateStatus = true
			logger.V("node allocatable requires updated")
			node.Status.Allocatable = expectedStatus.Allocatable
		}

		if !checkNodeInfoEqual(expectedStatus.NodeInfo, node.Status.NodeInfo) {
			updateStatus = true
			logger.V("node system info requires updated")
			node.Status.NodeInfo = expectedStatus.NodeInfo
		}

		if node.Status.Phase != corev1.NodeRunning {
			updateStatus = true
			logger.V("node phase requires update")
			node.Status.Phase = corev1.NodeRunning
		}

		var heartBeatRemain time.Duration

		for _, expectedCond := range expectedStatus.Conditions {
			found := false
			for i, currentCond := range node.Status.Conditions {
				if expectedCond.Type != currentCond.Type {
					continue
				}

				found = true
				if expectedCond.Status == currentCond.Status {
					// check if not using node lease for heart beat, then we need to ensure heart beat time
					if !c.vnConfig.Node.Lease.Enabled {
						// select the min heart beat remain
						remain := currentCond.LastHeartbeatTime.
							Add(c.vnConfig.Node.Timers.MirrorSyncInterval).Sub(now.Time)
						if remain < heartBeatRemain {
							heartBeatRemain = remain
						}

						node.Status.Conditions[i].LastHeartbeatTime = now
					}

					continue
				}

				updateStatus = true
				node.Status.Conditions[i].Status = expectedCond.Status
				node.Status.Conditions[i].LastTransitionTime = now
				node.Status.Conditions[i].LastHeartbeatTime = now
				node.Status.Conditions[i].Message = expectedCond.Message
				node.Status.Conditions[i].Reason = expectedCond.Reason

				logger.V("node condition requires update",
					log.Any("old", currentCond),
					log.Any("new", node.Status.Conditions[i]),
				)
			}

			if !found {
				updateStatus = true

				// add missing condition
				c := expectedCond.DeepCopy()
				c.LastTransitionTime = now
				c.LastHeartbeatTime = now

				node.Status.Conditions = append(node.Status.Conditions, *c)
			}
		}
		logger.V("resolved node conditions")

		if !c.vnConfig.Node.Lease.Enabled {
			if heartBeatRemain < (c.vnConfig.Node.Timers.MirrorSyncInterval / 5) {
				logger.V("node conditions requires update for heart beat")
				updateStatus = true
			} else {
				after := heartBeatRemain * 4 / 5
				logger.V("node conditions will be updated", log.Duration("after", after))
				// ensure update action for heart beat
				ret = &reconcile.Result{NextAction: queue.ActionUpdate, ScheduleAfter: after}
			}
		}
	} else {
		// node disconnected, ensure node status contains condition Ready=False
		var foundReadyCondition bool
		for i, c := range node.Status.Conditions {
			if c.Type != corev1.NodeReady {
				continue
			}

			foundReadyCondition = true

			if c.Status == corev1.ConditionUnknown || c.Status == corev1.ConditionFalse {
				break
			}

			updateStatus = true
			node.Status.Conditions[i] = newNodeNotReadyCondition(now)
			break
		}

		if !foundReadyCondition {
			updateStatus = true
			node.Status.Conditions = append(node.Status.Conditions, newNodeNotReadyCondition(now))
		}
	}

	if !updateStatus {
		logger.V("no node status update requested")
		return
	}

	logger.V("updating node status")
	_, err := c.nodeClient.UpdateStatus(c.Context(), node, metav1.UpdateOptions{})
	if err != nil {
		logger.I("failed to update node status", log.Error(err))
		return &reconcile.Result{Err: err}
	}
	logger.V("node status updated")

	return
}

func checkNodeResourcesEqual(a, b corev1.ResourceList) bool {
	switch {
	case !a.Cpu().Equal(*b.Cpu()):
	case !a.Pods().Equal(*b.Pods()):
	case !a.Memory().Equal(*b.Memory()):
	case !a.StorageEphemeral().Equal(*b.StorageEphemeral()):
	default:
		return true
	}

	return false
}

func checkNodeInfoEqual(a, b corev1.NodeSystemInfo) bool {
	switch {
	case a.MachineID != b.MachineID:
	case a.SystemUUID != b.SystemUUID:
	case a.BootID != b.BootID:
	case a.KernelVersion != b.KernelVersion:
	case a.OSImage != b.OSImage:
	case a.ContainerRuntimeVersion != b.ContainerRuntimeVersion:
	case a.KubeletVersion != b.KubeletVersion:
	case a.KubeProxyVersion != b.KubeProxyVersion:
	case a.OperatingSystem != b.OperatingSystem:
	case a.Architecture != b.Architecture:
	default:
		return true
	}

	return false
}

func getAllocatable(capacity corev1.ResourceList, pods []*corev1.Pod) corev1.ResourceList {
	cpuAvail := capacity.Cpu().MilliValue()
	memAvail := capacity.Memory().Value()
	storageAvail := capacity.StorageEphemeral().Value()
	podAvail := capacity.Pods().Value() - int64(len(pods))

	for _, pod := range pods {
		for _, ctr := range pod.Spec.Containers {
			cpuUsed := ctr.Resources.Requests.Cpu().MilliValue()
			if ctr.Resources.Limits.Cpu().Value() != 0 {
				cpuUsed = ctr.Resources.Limits.Cpu().MilliValue()
			}

			memUsed := ctr.Resources.Requests.Memory().Value()
			if ctr.Resources.Limits.Memory().Value() != 0 {
				memUsed = ctr.Resources.Limits.Memory().Value()
			}

			storageUsed := ctr.Resources.Requests.StorageEphemeral().Value()
			if ctr.Resources.Limits.StorageEphemeral().Value() != 0 {
				storageUsed = ctr.Resources.Limits.StorageEphemeral().Value()
			}

			cpuAvail -= cpuUsed
			memAvail -= memUsed
			storageAvail -= storageUsed
		}
	}

	if cpuAvail < 0 {
		cpuAvail = 0
	}

	if memAvail < 0 {
		memAvail = 0
	}

	if storageAvail < 0 {
		storageAvail = 0
	}

	if podAvail < 0 {
		podAvail = 0
	}

	return corev1.ResourceList{
		corev1.ResourceCPU:              *resource.NewMilliQuantity(cpuAvail, resource.DecimalSI),
		corev1.ResourceMemory:           *resource.NewQuantity(memAvail, resource.DecimalSI),
		corev1.ResourceEphemeralStorage: *resource.NewQuantity(storageAvail, resource.DecimalSI),
		corev1.ResourcePods:             *resource.NewQuantity(podAvail, resource.DecimalSI),
	}
}

func convertToGOARCH(arch string) string {
	switch {
	case arch == "x86":
		return "386"
	case strings.HasPrefix(arch, "armv"):
		// armv5/armv6/armv7 -> arm
		return "arm"
	case strings.HasPrefix(arch, "mips"):
		// mipshf/mips64hf -> mips/mips64 (runtime.GOARCH)
		return strings.TrimSuffix(arch, "hf")
	}

	return arch
}

func newNodeNotReadyCondition(t metav1.Time) corev1.NodeCondition {
	return corev1.NodeCondition{
		Type:               corev1.NodeReady,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: t,
		LastHeartbeatTime:  t,
		Reason:             "EdgeDevice disconnected",
		Message:            "connectivity lost",
	}
}
