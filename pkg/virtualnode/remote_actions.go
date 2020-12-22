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
		0, aranyagopb.CMD_NODE_INFO_GET, &aranyagopb.NodeInfoGetCmd{Kind: action},
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

		vn.updateNodeCache(ns, action)
		return false
	})

	if err != nil {
		return fmt.Errorf("failed to sync node status: %w", err)
	}

	return nil
}

// nolint:gocyclo
func (vn *VirtualNode) handleGlobalMsg(msg *aranyagopb.Msg) {
	sid := msg.Sid
	logger := vn.log.WithFields(log.String("source", "global"), log.Uint64("sid", sid))

	switch msg.Kind {
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

		action := aranyagopb.NODE_INFO_DYN
		if len(ns.ExtInfo) != 0 {
			action = aranyagopb.NODE_INFO_ALL
		}
		vn.updateNodeCache(ns, action)

		err := vn.opt.ScheduleNodeSync()
		if err != nil {
			logger.I("failed to sync mirror node status", log.Error(err))
		}
	case aranyagopb.MSG_NET:
	case aranyagopb.MSG_PERIPHERAL_STATUS:
	case aranyagopb.MSG_PERIPHERAL_STATUS_LIST:
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
			0, aranyagopb.CMD_SESSION_CLOSE, &aranyagopb.SessionCloseCmd{Sid: msg.Sid},
		)
	case aranyagopb.MSG_RUNTIME:
		if ps := msg.GetPodStatus(); ps != nil {
			logger.V("received global pod status")

			err := vn.podManager.UpdateMirrorPod(nil, ps, false)
			if err != nil {
				logger.I("failed to update pod status", log.Error(err))
			}
		}
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

func (vn *VirtualNode) updateNodeCache(msg *aranyagopb.NodeStatusMsg, action aranyagopb.NodeInfoGetCmd_Kind) {
	if sysInfo := msg.GetSystemInfo(); sysInfo != nil {
		vn.nodeStatusCache.UpdateSystemInfo(&corev1.NodeSystemInfo{
			OperatingSystem: sysInfo.GetOs(),
			Architecture:    sysInfo.GetArch(),
			OSImage:         sysInfo.GetOsImage(),
			KernelVersion:   sysInfo.GetKernelVersion(),
			MachineID:       sysInfo.GetMachineId(),
			SystemUUID:      sysInfo.GetSystemUuid(),
			BootID:          sysInfo.GetBootId(),
			// TODO: handle runtime version in pod manager
			ContainerRuntimeVersion: "",
			// TODO: how could we report kubelet and kube-proxy version?
			//       be the same with host node?
			KubeletVersion:   "",
			KubeProxyVersion: "",
		})
	}

	if conditions := msg.GetConditions(); conditions != nil {
		vn.nodeStatusCache.UpdateConditions(
			translateNodeConditions(
				vn.nodeStatusCache.RetrieveStatus(corev1.NodeStatus{}).Conditions,
				conditions,
			),
		)
	}

	if capacity := msg.GetCapacity(); capacity != nil {
		vn.nodeStatusCache.UpdateCapacity(
			translateNodeResourcesCapacity(
				capacity,
				vn.maxPods,
			),
		)
	}

	if action == aranyagopb.NODE_INFO_ALL {
		// only update node ext info for full update
		vn.log.V("updating node ext info", log.Any("ext_info", msg.GetExtInfo()))
		err := vn.nodeStatusCache.UpdateExtInfo(msg.GetExtInfo())
		if err != nil {
			vn.log.I("failed to update node ext info", log.Error(err))
		}
	}

	vn.nodeStatusCache.UpdatePhase(corev1.NodeRunning)
}

func translateNodeResourcesCapacity(res *aranyagopb.NodeResources, maxPods int64) corev1.ResourceList {
	return corev1.ResourceList{
		corev1.ResourceCPU:              *resource.NewQuantity(int64(res.GetCpuCount()), resource.DecimalSI),
		corev1.ResourceMemory:           *resource.NewQuantity(int64(res.GetMemoryBytes()), resource.BinarySI),
		corev1.ResourcePods:             *resource.NewQuantity(maxPods, resource.DecimalSI),
		corev1.ResourceEphemeralStorage: *resource.NewQuantity(int64(res.GetStorageBytes()), resource.BinarySI),
	}
}

func translateNodeConditions(prev []corev1.NodeCondition, cond *aranyagopb.NodeConditions) []corev1.NodeCondition {
	translate := func(condition aranyagopb.NodeCondition) corev1.ConditionStatus {
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
