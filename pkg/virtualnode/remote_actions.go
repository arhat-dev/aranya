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

package virtualnode

import (
	"fmt"
	"strconv"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/pkg/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

// generate in cluster node cache for remote node
func (vn *VirtualNode) SyncDeviceNodeStatus(action aranyagopb.NodeInfoGetCmd_Kind) error {
	msgCh, _, err := vn.opt.ConnectivityManager.PostCmd(
		0, aranyagopb.CMD_NODE_INFO_GET, aranyagopb.NewNodeCmd(action),
	)
	if err != nil {
		return err
	}

	connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
		if msgErr := msg.GetError(); msgErr != nil {
			err = msgErr
			return true
		}

		ns := msg.GetNodeStatus()
		if ns == nil {
			vn.log.I("unexpected non node status msg", log.Any("msg", msg))
			return true
		}

		vn.updateNodeCache(ns)
		return false
	}, nil, connectivity.HandleUnknownMessage(vn.log))

	if err != nil {
		return fmt.Errorf("failed to sync node status: %w", err)
	}

	return nil
}

func (vn *VirtualNode) handleGlobalMsg(msg *aranyagopb.Msg) {
	sid := msg.Header.Sid
	logger := vn.log.WithFields(log.String("source", "global"), log.Uint64("sid", sid))

	switch msg.Header.Kind {
	case aranyagopb.MSG_STATE:
		s := msg.GetState()
		if s == nil {
			// TODO: handle invalid msg
			return
		}
		switch s.Kind {
		case aranyagopb.STATE_ONLINE:
			logger.I("node connected", log.String("id", s.DeviceId))

			vn.opt.ConnectivityManager.OnConnected(func() (id string) {
				// initialize node and setup node/pod sync
				logger.D("processing node online message")
				if !vn.opt.VirtualnodeManager.OnVirtualNodeConnected(vn) {
					// initialization failed (rejected)
					logger.I("node failed to pass initial online check")
					return ""
				}

				logger.D("initialized node")

				if s.DeviceId == "" {
					return "default"
				}

				return s.DeviceId
			})
		case aranyagopb.STATE_OFFLINE:
			logger.I("node disconnected", log.String("id", s.DeviceId))

			vn.opt.ConnectivityManager.OnDisconnected(func() (id string, all bool) {
				return s.DeviceId, false
			})
		}
	case aranyagopb.MSG_NODE_STATUS:
		ns := msg.GetNodeStatus()
		if ns == nil {
			// TODO: handle invalid msg
			return
		}

		logger.V("received node status")

		vn.updateNodeCache(ns)

		err := vn.opt.ScheduleNodeSync()
		if err != nil {
			logger.I("failed to sync mirror node status", log.Error(err))
		}
	case aranyagopb.MSG_NETWORK_STATUS:
	case aranyagopb.MSG_DEVICE_STATUS:

	case aranyagopb.MSG_STORAGE_STATUS:
		if ss := msg.GetStorageStatus(); ss != nil {
			logger.V("received global storage status")
		}
	case aranyagopb.MSG_STORAGE_STATUS_LIST:
		if ssl := msg.GetStorageStatusList(); ssl != nil {
			logger.V("received global storage status list")
		}
	case aranyagopb.MSG_ERROR:
		logger.D("received global error", log.Error(msg.GetError()))
	case aranyagopb.MSG_DATA_DEFAULT, aranyagopb.MSG_DATA_STDERR:
		data := msg.GetData()
		if data == nil {
			// TODO: handle invalid msg
			return
		}

		logger.D("received orphan data", log.Binary("data", data))
		// close previous session, best effort
		_, _, _ = vn.opt.ConnectivityManager.PostCmd(
			0, aranyagopb.CMD_SESSION_CLOSE, aranyagopb.NewSessionCloseCmd(msg.Header.Sid),
		)
	case aranyagopb.MSG_CRED_STATUS:
	case aranyagopb.MSG_POD_STATUS:
		if ps := msg.GetPodStatus(); ps != nil {
			logger.V("received global pod status")

			err := vn.podManager.UpdateMirrorPod(nil, ps, false)
			if err != nil {
				logger.I("failed to update pod status", log.Error(err))
			}
		}
	case aranyagopb.MSG_POD_STATUS_LIST:
		if psl := msg.GetPodStatusList(); psl != nil {
			logger.V("received global pod status list")

			for _, status := range psl.Pods {
				err := vn.podManager.UpdateMirrorPod(nil, status, false)
				if err != nil {
					logger.I("failed to update pod status", log.Error(err))
				}
			}
		}
	default:
		// we don't know how to handle this kind of messages, discard
		logger.I("received unknown msg", log.Any("msg", msg))
	}
}

func (vn *VirtualNode) updateNodeCache(msg *aranyagopb.NodeStatusMsg) {
	if sysInfo := msg.GetSystemInfo(); sysInfo != nil {
		vn.nodeStatusCache.UpdateSystemInfo(&corev1.NodeSystemInfo{
			OperatingSystem: sysInfo.GetOs(),
			Architecture:    sysInfo.GetArch(),
			OSImage:         sysInfo.GetOsImage(),
			KernelVersion:   sysInfo.GetKernelVersion(),
			MachineID:       sysInfo.GetMachineId(),
			SystemUUID:      sysInfo.GetSystemUuid(),
			BootID:          sysInfo.GetBootId(),
			ContainerRuntimeVersion: fmt.Sprintf("%s://%s",
				sysInfo.GetRuntimeInfo().GetName(), sysInfo.GetRuntimeInfo().GetVersion()),
			// TODO: how should we report kubelet and kube-proxy version?
			//       be the same with host node?
			KubeletVersion:   "",
			KubeProxyVersion: "",
		})
	}

	if conditions := msg.GetConditions(); conditions != nil {
		prevConditions := vn.nodeStatusCache.RetrieveStatus(corev1.NodeStatus{}).Conditions
		vn.nodeStatusCache.UpdateConditions(translateDeviceCondition(prevConditions, conditions))
	}

	if capacity := msg.GetCapacity(); capacity != nil {
		vn.nodeStatusCache.UpdateCapacity(translateDeviceResourcesCapacity(capacity, vn.maxPods))
	}

	labels := make(map[string]string)
	annotations := make(map[string]string)
	oldLabels, oldAnnotations := vn.nodeStatusCache.RetrieveExtInfo()

	if oldLabels == nil {
		oldLabels = make(map[string]string)
	}

	if oldAnnotations == nil {
		oldAnnotations = make(map[string]string)
	}

	for _, info := range msg.GetExtInfo() {
		var (
			target, oldTarget map[string]string
			key               = info.TargetKey
		)

		switch info.Target {
		case aranyagopb.NODE_EXT_INFO_TARGET_ANNOTATION:
			target, oldTarget = annotations, oldAnnotations
		case aranyagopb.NODE_EXT_INFO_TARGET_LABEL:
			target, oldTarget = labels, oldLabels
		default:
			// TODO: report unsupported
			return
		}

		switch info.Operator {
		case aranyagopb.NODE_EXT_INFO_OPERATOR_SET:
			target[key] = info.Value
		case aranyagopb.NODE_EXT_INFO_OPERATOR_ADD,
			aranyagopb.NODE_EXT_INFO_OPERATOR_MINUS:
			oldVal := oldTarget[key]

			switch info.ValueType {
			case aranyagopb.NODE_EXT_INFO_TYPE_STRING:
				target[key] = oldTarget[key] + info.Value
			case aranyagopb.NODE_EXT_INFO_TYPE_INTEGER:
				oldIntVal, _ := strconv.ParseInt(oldVal, 0, 64)
				val, _ := strconv.ParseInt(info.Value, 0, 64)

				switch info.Operator {
				case aranyagopb.NODE_EXT_INFO_OPERATOR_ADD:
					target[key] = strconv.FormatInt(oldIntVal+val, 10)
				case aranyagopb.NODE_EXT_INFO_OPERATOR_MINUS:
					target[key] = strconv.FormatInt(oldIntVal-val, 10)
				}
			case aranyagopb.NODE_EXT_INFO_TYPE_FLOAT:
				oldFloatVal, _ := strconv.ParseFloat(oldVal, 64)
				val, _ := strconv.ParseFloat(info.Value, 64)

				switch info.Operator {
				case aranyagopb.NODE_EXT_INFO_OPERATOR_ADD:
					target[key] = strconv.FormatFloat(oldFloatVal+val, 'f', -1, 64)
				case aranyagopb.NODE_EXT_INFO_OPERATOR_MINUS:
					target[key] = strconv.FormatFloat(oldFloatVal-val, 'f', -1, 64)
				}
			}
		}
	}
	vn.nodeStatusCache.UpdateExtInfo(labels, annotations)

	vn.nodeStatusCache.UpdatePhase(corev1.NodeRunning)
}

func translateDeviceResourcesCapacity(res *aranyagopb.NodeResources, maxPods int64) corev1.ResourceList {
	return corev1.ResourceList{
		corev1.ResourceCPU:              *resource.NewQuantity(int64(res.GetCpuCount()), resource.DecimalSI),
		corev1.ResourceMemory:           *resource.NewQuantity(int64(res.GetMemoryBytes()), resource.BinarySI),
		corev1.ResourcePods:             *resource.NewQuantity(maxPods, resource.DecimalSI),
		corev1.ResourceEphemeralStorage: *resource.NewQuantity(int64(res.GetStorageBytes()), resource.BinarySI),
	}
}

func translateDeviceCondition(prev []corev1.NodeCondition, cond *aranyagopb.NodeConditions) []corev1.NodeCondition {
	translate := func(condition aranyagopb.NodeConditions_Condition) corev1.ConditionStatus {
		switch condition {
		case aranyagopb.NODE_CONDITION_HEALTHY:
			return corev1.ConditionFalse
		case aranyagopb.NODE_CONDITION_UNHEALTHY:
			return corev1.ConditionTrue
		default:
			return corev1.ConditionUnknown
		}
	}

	result := []corev1.NodeCondition{
		{
			Type: corev1.NodeReady,
			Status: func() corev1.ConditionStatus {
				switch cond.GetReady() {
				case aranyagopb.NODE_CONDITION_HEALTHY:
					return corev1.ConditionTrue
				case aranyagopb.NODE_CONDITION_UNHEALTHY:
					return corev1.ConditionFalse
				default:
					return corev1.ConditionUnknown
				}
			}(),
		},
		{Type: corev1.NodeMemoryPressure, Status: translate(cond.GetMemory())},
		{Type: corev1.NodeDiskPressure, Status: translate(cond.GetDisk())},
		{Type: corev1.NodePIDPressure, Status: translate(cond.GetPid())},
		{Type: corev1.NodeNetworkUnavailable, Status: translate(cond.GetNetwork())},
	}

	now := metav1.Now()
	for i, current := range result {
		for j, last := range prev {
			if last.Type != current.Type {
				continue
			}

			if last.Status == current.Status && !last.LastTransitionTime.IsZero() {
				result[i].LastTransitionTime = prev[j].LastTransitionTime
				continue
			}

			result[i].LastTransitionTime = now
		}

		if result[i].LastTransitionTime.IsZero() {
			result[i].LastTransitionTime = now
		}
		result[i].LastHeartbeatTime = now
	}

	return result
}
