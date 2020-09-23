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

package aranyagopb

import (
	"math"
	"time"

	"arhat.dev/aranya-proto/aranyagopb/aranyagoconst"
)

var (
	EmptyCmdSize int
)

func init() {
	emptyBody, _ := (&Empty{}).Marshal()
	emptyCmd := &Cmd{
		Header: &Header{
			Kind:      math.MaxInt32,
			Sid:       math.MaxUint64,
			Seq:       math.MaxUint64,
			Completed: true,
		},
		Body: emptyBody,
	}

	EmptyCmdSize = emptyCmd.Size()
	_ = EmptyCmdSize
}

// NewCmd to create cmd object to be marshaled and sent to agent
func NewCmd(kind Kind, sid, seq uint64, completed bool, payload []byte) *Cmd {
	return &Cmd{
		Header: &Header{
			Kind:      kind,
			Sid:       sid,
			Seq:       seq,
			Completed: completed,
		},
		Body: payload,
	}
}

func NewRejectCmd(reason RejectCmd_Reason, message string) *RejectCmd {
	return &RejectCmd{
		Reason:  reason,
		Message: message,
	}
}

func NewNodeCmd(k NodeInfoGetCmd_Kind) *NodeInfoGetCmd {
	return &NodeInfoGetCmd{
		Kind: k,
	}
}

func NewCredentialEnsureCmd(sshPrivateKey []byte) *CredentialEnsureCmd {
	return &CredentialEnsureCmd{
		SshPrivateKey: sshPrivateKey,
	}
}

func NewPodDeleteCmd(podUID string, graceTime time.Duration, preStopHooks map[string]*ContainerAction) *PodDeleteCmd {
	return &PodDeleteCmd{
		PodUid:      podUID,
		GraceTime:   int64(graceTime),
		HookPreStop: preStopHooks,
	}
}

func NewPodContainerDeleteCmd(podUID string, containers []string) *PodDeleteCmd {
	return &PodDeleteCmd{
		PodUid:     podUID,
		Containers: containers,
	}
}

func NewPodListCmd(namespace, name string, all bool) *PodListCmd {
	return &PodListCmd{
		Namespace: namespace,
		Name:      name,
		All:       all,
	}
}

func NewPodContainerExecCmd(
	podUID, container string,
	command []string,
	stdin, stdout, stderr, tty bool,
	env map[string]string,
) *ContainerExecOrAttachCmd {
	return &ContainerExecOrAttachCmd{
		PodUid:    podUID,
		Container: container,
		Command:   command,
		Stdin:     stdin,
		Stderr:    stderr,
		Stdout:    stdout,
		Tty:       tty,
		Envs:      env,
	}
}

func NewPodContainerAttachCmd(podUID, container string, stdin, stdout, stderr, tty bool) *ContainerExecOrAttachCmd {
	return &ContainerExecOrAttachCmd{
		PodUid:    podUID,
		Container: container,
		Stdin:     stdin,
		Stderr:    stderr,
		Stdout:    stdout,
		Tty:       tty,
	}
}

func NewHostLogCmd(path string) *ContainerLogsCmd {
	return &ContainerLogsCmd{
		Path: path,
	}
}

func NewPodContainerLogsCmd(
	podUID, container string,
	follow, timestamp, previous bool,
	since time.Time, tailLines, bytesLimit int64,
) *ContainerLogsCmd {
	return &ContainerLogsCmd{
		PodUid:     podUID,
		Container:  container,
		Follow:     follow,
		Timestamp:  timestamp,
		Since:      since.Format(aranyagoconst.TimeLayout),
		TailLines:  tailLines,
		BytesLimit: bytesLimit,
		Previous:   previous,
	}
}

func NewPodPortForwardCmd(podUID string, port int32, protocol string) *PodPortForwardCmd {
	return &PodPortForwardCmd{
		PodUid:   podUID,
		Port:     port,
		Protocol: protocol,
	}
}

func NewPodContainerTerminalResizeCmd(cols uint16, rows uint16) *ContainerTerminalResizeCmd {
	return &ContainerTerminalResizeCmd{
		Cols: uint32(cols),
		Rows: uint32(rows),
	}
}

func NewSessionCloseCmd(sessionToClose uint64) *SessionCloseCmd {
	return &SessionCloseCmd{Sid: sessionToClose}
}

func NewMetricsCollectCmd(t MetricsTarget) *MetricsCollectCmd {
	return &MetricsCollectCmd{
		Target: t,
	}
}

func NewMetricsConfigCmd(t MetricsTarget, collect, extraArgs []string) *MetricsConfigCmd {
	return &MetricsConfigCmd{Target: t, Collect: collect, ExtraArgs: extraArgs}
}

func NewContainerNetworkEnsureCmd(ipv4CIDR, ipv6CIDR string) *ContainerNetworkEnsureCmd {
	return &ContainerNetworkEnsureCmd{
		CidrIpv4: ipv4CIDR,
		CidrIpv6: ipv6CIDR,
	}
}

func NewDeviceListCmd() *DeviceListCmd {
	return &DeviceListCmd{}
}

func NewDeviceEnsureCmd(
	deviceID string,
	deviceConnectivity, uploadConnectivity *DeviceConnectivity,
	deviceOperations []*DeviceOperation,
	deviceMetrics []*DeviceMetrics,
) *DeviceEnsureCmd {
	return &DeviceEnsureCmd{
		DeviceId:           deviceID,
		DeviceConnectivity: deviceConnectivity,
		UploadConnectivity: uploadConnectivity,
		DeviceOperations:   deviceOperations,
		DeviceMetrics:      deviceMetrics,
	}
}

func NewDeviceDeleteCmd(ids ...string) *DeviceDeleteCmd {
	return &DeviceDeleteCmd{DeviceIds: ids}
}
