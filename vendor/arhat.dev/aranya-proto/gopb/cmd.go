// +build !arhat,!abbot

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

package gopb

import (
	"math"
	"time"

	"github.com/gogo/protobuf/proto"

	"arhat.dev/aranya-proto/gopb/protoconst"
)

var (
	EmptyInputCmdSize int
)

func init() {
	emptyInputCmd := newCmd(CMD_POD_OPERATION, math.MaxUint64, &PodOperationCmd{
		Action: WRITE_TO_CONTAINER,
		Options: &PodOperationCmd_InputOptions{
			InputOptions: &InputOptions{
				Data:      make([]byte, 0),
				Completed: true,
				Seq:       math.MaxUint64,
			},
		},
	})

	EmptyInputCmdSize = emptyInputCmd.Size()
}

func NewCmd(sid uint64, cmd proto.Marshaler) *Cmd {
	var kind CmdType
	switch cmd.(type) {
	case *RejectCmd:
		kind = CMD_REJECTION
	case *NodeCmd:
		kind = CMD_NODE
	case *StorageCmd:
		kind = CMD_STORAGE
	case *PodCmd:
		kind = CMD_POD
	case *PodOperationCmd:
		kind = CMD_POD_OPERATION
	case *NetworkCmd:
		kind = CMD_NETWORK
	case *CredentialCmd:
		kind = CMD_CRED
	case *SessionCmd:
		kind = CMD_SESSION
	case *DeviceCmd:
		kind = CMD_DEVICE
	case *MetricsCmd:
		kind = CMD_METRICS
	default:
		return nil
	}

	return newCmd(kind, sid, cmd)
}

func newCmd(kind CmdType, sid uint64, cmd proto.Marshaler) *Cmd {
	body, _ := cmd.Marshal()
	return &Cmd{
		Kind:      kind,
		SessionId: sid,
		Body:      body,
	}
}

func NewRejectCmd(reason RejectCmd_Reason, message string) *RejectCmd {
	return &RejectCmd{
		Reason:  reason,
		Message: message,
	}
}

func NewNodeCmd(action NodeCmd_Action) *NodeCmd {
	return &NodeCmd{
		Action: action,
	}
}

func NewStorageCredentialUpdateCmd(sshPrivateKey []byte) *CredentialCmd {
	return &CredentialCmd{
		Action: UPDATE_STORAGE_CREDENTIAL,
		Options: &CredentialCmd_Storage{
			Storage: &StorageCredentialOptions{
				SshPrivateKey: sshPrivateKey,
			},
		},
	}
}

func newPodCmd(action PodCmd_Action, options isPodCmd_Options) *PodCmd {
	return &PodCmd{
		Action:  action,
		Options: options,
	}
}

func NewPodImageEnsureCmd(options *ImageEnsureOptions) *PodCmd {
	return newPodCmd(ENSURE_IMAGES, &PodCmd_ImageEnsureOptions{
		ImageEnsureOptions: options,
	})
}

func NewPodCreateCmd(options *CreateOptions) *PodCmd {
	return newPodCmd(CREATE_CONTAINERS, &PodCmd_CreateOptions{
		CreateOptions: options,
	})
}

func NewPodDeleteCmd(podUID string, graceTime time.Duration, preStopHooks map[string]*ActionMethod) *PodCmd {
	return newPodCmd(DELETE_POD, &PodCmd_DeleteOptions{
		DeleteOptions: &DeleteOptions{
			PodUid:      podUID,
			GraceTime:   int64(graceTime),
			HookPreStop: preStopHooks,
		},
	})
}

func NewPodContainerDeleteCmd(podUID string, containers []string) *PodCmd {
	return newPodCmd(DELETE_CONTAINERS, &PodCmd_DeleteOptions{
		DeleteOptions: &DeleteOptions{
			PodUid:     podUID,
			Containers: containers,
		},
	})
}

func NewPodListCmd(namespace, name string, all bool) *PodCmd {
	return newPodCmd(LIST_PODS, &PodCmd_ListOptions{
		ListOptions: &ListOptions{
			Namespace: namespace,
			Name:      name,
			All:       all,
		},
	})
}

func newPodOperationCmd(action PodOperationCmd_Action, options isPodOperationCmd_Options) *PodOperationCmd {
	return &PodOperationCmd{
		Action:  action,
		Options: options,
	}
}

func NewPodExecCmd(
	podUID, container string,
	command []string,
	stdin, stdout, stderr, tty bool,
	env map[string]string,
) *PodOperationCmd {
	return newPodOperationCmd(EXEC_IN_CONTAINER, &PodOperationCmd_ExecOptions{
		ExecOptions: &ExecOptions{
			PodUid:    podUID,
			Container: container,
			Command:   command,
			Stdin:     stdin,
			Stderr:    stderr,
			Stdout:    stdout,
			Tty:       tty,
			Envs:      env,
		},
	})
}

func NewPodAttachCmd(podUID, container string, stdin, stdout, stderr, tty bool) *PodOperationCmd {
	return newPodOperationCmd(ATTACH_TO_CONTAINER, &PodOperationCmd_ExecOptions{
		ExecOptions: &ExecOptions{
			PodUid:    podUID,
			Container: container,
			Stdin:     stdin,
			Stderr:    stderr,
			Stdout:    stdout,
			Tty:       tty,
		},
	})
}

func NewHostLogCmd(path string) *PodOperationCmd {
	return newPodOperationCmd(RETRIEVE_CONTAINER_LOG, &PodOperationCmd_LogOptions{
		LogOptions: &LogOptions{
			Path: path,
		},
	})
}

func NewPodLogCmd(
	podUID, container string,
	follow, timestamp, previous bool,
	since time.Time, tailLines, bytesLimit int64,
) *PodOperationCmd {
	return newPodOperationCmd(RETRIEVE_CONTAINER_LOG, &PodOperationCmd_LogOptions{
		LogOptions: &LogOptions{
			PodUid:     podUID,
			Container:  container,
			Follow:     follow,
			Timestamp:  timestamp,
			Since:      since.Format(protoconst.TimeLayout),
			TailLines:  tailLines,
			BytesLimit: bytesLimit,
			Previous:   previous,
		},
	})
}

func NewPodPortForwardCmd(podUID string, port int32, protocol string) *PodOperationCmd {
	return newPodOperationCmd(PORT_FORWARD_TO_CONTAINER, &PodOperationCmd_PortForwardOptions{
		PortForwardOptions: &PortForwardOptions{
			PodUid:   podUID,
			Port:     port,
			Protocol: protocol,
		},
	})
}

func NewPodInputCmd(completed bool, data []byte) *PodOperationCmd {
	return newPodOperationCmd(WRITE_TO_CONTAINER, &PodOperationCmd_InputOptions{
		InputOptions: &InputOptions{
			Data:      data,
			Completed: completed,
		},
	})
}

func NewPodResizeCmd(cols uint16, rows uint16) *PodOperationCmd {
	return newPodOperationCmd(RESIZE_CONTAINER_TTY, &PodOperationCmd_ResizeOptions{
		ResizeOptions: &TtyResizeOptions{
			Cols: uint32(cols),
			Rows: uint32(rows),
		},
	})
}

func NewSessionCloseCmd(sessionToClose uint64) *SessionCmd {
	return &SessionCmd{
		Action:  CLOSE_SESSION,
		Options: &SessionCmd_SessionId{SessionId: sessionToClose},
	}
}

func NewMetricsCollectCmd(action MetricsCmd_Action) *MetricsCmd {
	return &MetricsCmd{
		Action: action,
	}
}

func NewMetricsConfigureCmd(action MetricsCmd_Action, config *MetricsConfigOptions) *MetricsCmd {
	return &MetricsCmd{
		Action:  action,
		Options: &MetricsCmd_Config{Config: config},
	}
}

func newNetworkCmd(action NetworkCmd_Action, options *NetworkCmd_NetworkOptions) *NetworkCmd {
	return &NetworkCmd{
		Action:  action,
		Options: options,
	}
}

func NewPodNetworkUpdateCmd(ipv4PodCIDR, ipv6PodCIDR string) *NetworkCmd {
	return newNetworkCmd(UPDATE_NETWORK, &NetworkCmd_NetworkOptions{
		NetworkOptions: &NetworkOptions{
			Ipv4PodCidr: ipv4PodCIDR,
			Ipv6PodCidr: ipv6PodCIDR,
		},
	})
}

func newDeviceCmd(action DeviceCmd_Action, options *DeviceCmd_DeviceSpec, id string) *DeviceCmd {
	var opt isDeviceCmd_Options
	if options == nil {
		opt = &DeviceCmd_DeviceId{DeviceId: id}
	} else {
		opt = options
	}

	return &DeviceCmd{
		Action:  action,
		Options: opt,
	}
}

func NewDeviceListCmd() *DeviceCmd {
	return newDeviceCmd(LIST_DEVICES, nil, "")
}

func NewDeviceRemoveCmd(id string) *DeviceCmd {
	return newDeviceCmd(REMOVE_DEVICE, nil, id)
}

func NewDeviceEnsureCmd(deviceSpec *Device) *DeviceCmd {
	return newDeviceCmd(ENSURE_DEVICE, &DeviceCmd_DeviceSpec{
		DeviceSpec: deviceSpec,
	}, "")
}
