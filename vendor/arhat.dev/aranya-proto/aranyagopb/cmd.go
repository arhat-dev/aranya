// +build !arhat

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
	"crypto/sha256"
	"encoding/hex"
	"math"
	"sort"
	"time"

	"arhat.dev/aranya-proto/aranyagopb/aranyagoconst"
)

var (
	EmptyCmdSize int
)

func init() {
	emptyBody, _ := (&Empty{}).Marshal()
	emptyCmd := &Cmd{
		Kind:      math.MaxInt32,
		Sid:       math.MaxUint64,
		Seq:       math.MaxUint64,
		Completed: true,
		Body:      emptyBody,
	}

	EmptyCmdSize = emptyCmd.Size()
	_ = EmptyCmdSize
}

// NewCmd to create cmd object to be marshaled and sent to agent
func NewCmd(kind CmdType, sid, seq uint64, completed bool, payload []byte) *Cmd {
	return &Cmd{
		Kind:      kind,
		Sid:       sid,
		Seq:       seq,
		Completed: completed,
		Body:      payload,
	}
}

func NewRejectCmd(reason RejectionReason, message string) *RejectCmd {
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

func NewImageListCmd(refs ...string) *ImageListCmd {
	return &ImageListCmd{
		Refs: refs,
	}
}

func NewImageDeleteCmd(refs ...string) *ImageDeleteCmd {
	return &ImageDeleteCmd{
		Refs: refs,
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

func NewExecCmd(
	podUID, container string,
	command []string,
	stdin, stdout, stderr, tty bool,
	env map[string]string,
) *ExecOrAttachCmd {
	return &ExecOrAttachCmd{
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

func NewAttachCmd(podUID, container string, stdin, stdout, stderr, tty bool) *ExecOrAttachCmd {
	return &ExecOrAttachCmd{
		PodUid:    podUID,
		Container: container,
		Stdin:     stdin,
		Stderr:    stderr,
		Stdout:    stdout,
		Tty:       tty,
	}
}

func NewHostLogCmd(path string) *LogsCmd {
	return &LogsCmd{
		Path: path,
	}
}

func NewLogsCmd(
	podUID, container string,
	follow, timestamp, previous bool,
	since time.Time, tailLines, bytesLimit int64,
) *LogsCmd {
	return &LogsCmd{
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

func NewPortForwardCmd(podUID string, port int32, protocol string) *PortForwardCmd {
	return &PortForwardCmd{
		PodUid:   podUID,
		Port:     port,
		Protocol: protocol,
	}
}

func NewTerminalResizeCmd(cols uint16, rows uint16) *TerminalResizeCmd {
	return &TerminalResizeCmd{
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
	return &MetricsConfigCmd{
		Target:    t,
		Collect:   collect,
		ExtraArgs: extraArgs,
	}
}

func NewContainerNetworkListCmd() *ContainerNetworkListCmd {
	return &ContainerNetworkListCmd{}
}

func NewContainerNetworkEnsureCmd(ipv4CIDR, ipv6CIDR string) *ContainerNetworkEnsureCmd {
	return &ContainerNetworkEnsureCmd{
		Ipv4Cidr: ipv4CIDR,
		Ipv6Cidr: ipv6CIDR,
	}
}

func NewDeviceListCmd(ids ...string) *DeviceListCmd {
	return &DeviceListCmd{Ids: ids}
}

func NewDeviceEnsureCmd(
	kind DeviceType,
	connectorHashHex, deviceID string,
	connector *Connectivity,
	operations []*DeviceOperation,
	metrics []*DeviceMetric,
) *DeviceEnsureCmd {
	return &DeviceEnsureCmd{
		Kind:             kind,
		ConnectorHashHex: connectorHashHex,
		Connector:        connector,

		DeviceId:   deviceID,
		Operations: operations,
		Metrics:    metrics,
	}
}

func NewDeviceDeleteCmd(ids ...string) *DeviceDeleteCmd {
	return &DeviceDeleteCmd{Ids: ids}
}

func HexHashOfConnectivity(c *Connectivity) string {
	h := sha256.New()

	_, _ = h.Write([]byte(c.Method))
	_, _ = h.Write([]byte(c.Target))

	if tls := c.Tls; tls != nil {
		_, _ = h.Write(tls.CaCert)
		_, _ = h.Write(tls.Cert)
		_, _ = h.Write(tls.Key)
	}

	var keys []string
	for k := range c.Params {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	for _, k := range keys {
		_, _ = h.Write([]byte(k))
		_, _ = h.Write([]byte(c.Params[k]))
	}

	return hex.EncodeToString(h.Sum(nil))
}

func NewDeviceOperateCmd(deviceID, operationID string, data []byte) *DeviceOperateCmd {
	return &DeviceOperateCmd{
		DeviceId:    deviceID,
		OperationId: operationID,
		Data:        data,
	}
}

func NewDeviceMetricsCollectCmd(deviceIDs ...string) *DeviceMetricsCollectCmd {
	return &DeviceMetricsCollectCmd{
		DeviceIds: deviceIDs,
	}
}
