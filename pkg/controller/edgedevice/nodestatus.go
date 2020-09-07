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
func (c *Controller) onNodeStatusUpdated(oldObj, newObj interface{}) *reconcile.Result {
	var (
		err            error
		node           = newObj.(*corev1.Node).DeepCopy()
		name           = node.Name
		updateStatus   = false
		updateMetadata = false
		logger         = c.Log.WithFields(log.String("name", name))
		now            = metav1.Now()
		ret            *reconcile.Result
	)

	vn, ignore := c.doNodeResourcePreCheck(name)
	if ignore {
		return nil
	}

	// ensure node status up to date
	if vn.Connected() {
		clone := node.DeepCopy()

		actualStatus := vn.ActualNodeStatus(clone.Status)
		requiredLabels, requiredAnnotations := vn.ExtInfo()

		// check its labels annotations
		var (
			arch   = actualStatus.NodeInfo.Architecture
			os     = actualStatus.NodeInfo.OperatingSystem
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
			requiredLabels[k] = v
		}

		for k, v := range clone.Labels {
			if requiredLabels[k] == v {
				delete(requiredLabels, k)
			}
		}

		for k, v := range clone.Annotations {
			if requiredAnnotations[k] == v {
				delete(requiredAnnotations, k)
			}
		}

		if len(requiredLabels) != 0 {
			if logger.Enabled(log.LevelVerbose) {
				var labels []string
				for k, v := range requiredLabels {
					labels = append(labels, k+"="+v)
				}
				sort.Strings(labels)

				logger.V("node labels need to be updated", log.Strings("labels", labels))
			}

			updateMetadata = true

			if clone.Labels == nil {
				clone.Labels = requiredLabels
			} else {
				for k, v := range requiredLabels {
					clone.Labels[k] = v
				}
			}
		}

		if len(requiredAnnotations) != 0 {
			if logger.Enabled(log.LevelVerbose) {
				var annotations []string
				for k, v := range requiredAnnotations {
					annotations = append(annotations, k+"="+v)
				}
				sort.Strings(annotations)

				logger.V("node annotations need to be updated", log.Strings("annotations", annotations))
			}

			updateMetadata = true

			if clone.Annotations == nil {
				clone.Annotations = requiredAnnotations
			} else {
				for k, v := range requiredAnnotations {
					clone.Annotations[k] = v
				}
			}
		}

		if updateMetadata {
			// need to update metadata first, update node status next round
			logger.V("updating node metadata")
			err = patchhelper.TwoWayMergePatch(node, clone, &corev1.Node{}, func(patchData []byte) error {
				_, err2 := c.nodeClient.Patch(
					c.Context(), name, types.StrategicMergePatchType, patchData, metav1.PatchOptions{})
				return err2
			})
			if err != nil {
				logger.I("failed to update node metadata", log.Error(err))
				return &reconcile.Result{Err: err}
			}
			logger.V("node metadata updated")

			// should trigger update automatically since we are watching, just in case
			return &reconcile.Result{
				NextAction: queue.ActionUpdate,
				ScheduleAfter: func() time.Duration {
					if c.vnConfig.Node.Lease.Enabled {
						return c.vnConfig.Node.Lease.UpdateInterval / 5
					}

					return c.vnConfig.Node.Timers.MirrorSyncInterval / 5
				}(),
			}
		}

		actualStatus.Allocatable = getAllocatable(actualStatus.Capacity, c.getWatchPodsForNode(name))

		if !checkNodeResourcesEqual(actualStatus.Capacity, clone.Status.Capacity) {
			updateStatus = true
			logger.V("node capacity needs to be updated")
			clone.Status.Capacity = actualStatus.Capacity
		}

		if !checkNodeResourcesEqual(actualStatus.Allocatable, clone.Status.Allocatable) {
			updateStatus = true
			logger.V("node allocatable needs to be updated")
			clone.Status.Allocatable = actualStatus.Allocatable
		}

		if !checkNodeInfoEqual(actualStatus.NodeInfo, clone.Status.NodeInfo) {
			updateStatus = true
			logger.V("node system info needs to be updated")
			clone.Status.NodeInfo = actualStatus.NodeInfo
		}

		if clone.Status.Phase != corev1.NodeRunning {
			updateStatus = true
			logger.V("node phase need to be updated")
			clone.Status.Phase = corev1.NodeRunning
		}

		var heartBeatRemain time.Duration

		for _, expectedCond := range actualStatus.Conditions {
			found := false
			for i, currentCond := range clone.Status.Conditions {
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

						clone.Status.Conditions[i].LastHeartbeatTime = now
					}

					continue
				}

				updateStatus = true
				clone.Status.Conditions[i].Status = expectedCond.Status
				clone.Status.Conditions[i].LastTransitionTime = now
				clone.Status.Conditions[i].LastHeartbeatTime = now
				clone.Status.Conditions[i].Message = expectedCond.Message
				clone.Status.Conditions[i].Reason = expectedCond.Reason

				logger.V("node condition need to be updated",
					log.Any("old", currentCond), log.Any("new", clone.Status.Conditions[i]))
			}

			if !found {
				updateStatus = true

				// add missing condition
				c := expectedCond.DeepCopy()
				c.LastTransitionTime = now
				c.LastHeartbeatTime = now

				clone.Status.Conditions = append(clone.Status.Conditions, *c)
			}
		}
		logger.V("resolved node conditions")

		if !c.vnConfig.Node.Lease.Enabled {
			if heartBeatRemain < (c.vnConfig.Node.Timers.MirrorSyncInterval / 10) {
				logger.V("node conditions need to be updated for heart beat")
				updateStatus = true
			} else {
				logger.V("node conditions will be updated", log.Duration("after", heartBeatRemain))
				// ensure update action for heart beat
				ret = &reconcile.Result{NextAction: queue.ActionUpdate, ScheduleAfter: heartBeatRemain}
			}
		}

		node = clone
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
		return ret
	}

	logger.V("updating node status")
	_, err = c.nodeClient.UpdateStatus(c.Context(), node, metav1.UpdateOptions{})
	if err != nil {
		logger.I("failed to update node status", log.Error(err))
		return &reconcile.Result{Err: err}
	}
	logger.V("node status synced")

	return ret
}

func checkNodeResourcesEqual(a, b corev1.ResourceList) bool {
	switch {
	case !a.Cpu().Equal(*b.Cpu()):
	case !a.Pods().Equal(*b.Pods()):
	case !a.Memory().Equal(*b.Memory()):
	case !a.Storage().Equal(*b.Storage()):
	case !a.StorageEphemeral().Equal(*b.Storage()):
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
