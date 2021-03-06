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
	"fmt"
	"net/http"
	"strings"
	"time"

	"arhat.dev/abbot-proto/abbotgopb"
	"arhat.dev/aranya-proto/aranyagopb/runtimepb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
)

func newContainerErrorStatus(pod *corev1.Pod) (corev1.PodPhase, []corev1.ContainerStatus) {
	status := make([]corev1.ContainerStatus, len(pod.Spec.Containers))
	for i, ctr := range pod.Spec.Containers {
		status[i] = corev1.ContainerStatus{
			Name:  ctr.Name,
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerErrored"}},
			Image: ctr.Image,
		}
	}

	return corev1.PodFailed, status
}

func newContainerInitializingStatus(pod *corev1.Pod) (corev1.PodPhase, []corev1.ContainerStatus) {
	status := make([]corev1.ContainerStatus, len(pod.Spec.Containers))
	for i, ctr := range pod.Spec.InitContainers {
		status[i] = corev1.ContainerStatus{
			Name:  ctr.Name,
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "PodInitializing"}},
			Image: ctr.Image,
		}
	}

	return corev1.PodPending, status
}

func newContainerCreatingStatus(pod *corev1.Pod) (corev1.PodPhase, []corev1.ContainerStatus) {
	status := make([]corev1.ContainerStatus, len(pod.Spec.Containers))
	for i, ctr := range pod.Spec.Containers {
		status[i] = corev1.ContainerStatus{
			Name:  ctr.Name,
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}},
			Image: ctr.Image,
		}
	}

	return corev1.PodPending, status
}

func resolveContainerStatus(
	containers []corev1.Container,
	devicePodStatus *runtimepb.PodStatusMsg,
) (corev1.PodPhase, []corev1.ContainerStatus) {
	ctrStatusMap := devicePodStatus.GetContainers()
	if ctrStatusMap == nil {
		// generalize to avoid panic
		ctrStatusMap = make(map[string]*runtimepb.ContainerStatus)
	}

	podPhase := corev1.PodRunning
	statuses := make([]corev1.ContainerStatus, len(containers))
	for i, ctr := range containers {
		if s, ok := ctrStatusMap[ctr.Name]; ok {
			status := &corev1.ContainerStatus{
				Name:        ctr.Name,
				ContainerID: s.ContainerId,
				Image:       ctr.Image,
				ImageID:     s.ImageId,
			}

			containerExited := false
			switch s.GetState() {
			case runtimepb.POD_STATE_UNKNOWN:
			case runtimepb.POD_STATE_PENDING:
				podPhase = corev1.PodPending
				status.State.Waiting = &corev1.ContainerStateWaiting{
					Reason:  s.Reason,
					Message: s.Message,
				}
			case runtimepb.POD_STATE_RUNNING:
				status.Ready = true
				status.State.Running = &corev1.ContainerStateRunning{
					StartedAt: metav1.NewTime(s.GetTimeStartedAt()),
				}
			case runtimepb.POD_STATE_SUCCEEDED:
				containerExited = true
				podPhase = corev1.PodSucceeded
			case runtimepb.POD_STATE_FAILED:
				containerExited = true
				podPhase = corev1.PodFailed
			}

			if containerExited {
				status.State.Terminated = &corev1.ContainerStateTerminated{
					ExitCode:    s.ExitCode,
					Reason:      s.Reason,
					Message:     s.Message,
					StartedAt:   metav1.NewTime(s.GetTimeStartedAt()),
					FinishedAt:  metav1.NewTime(s.GetTimeFinishedAt()),
					ContainerID: s.ContainerId,
				}
			}

			statuses[i] = *status
		} else {
			statuses[i] = corev1.ContainerStatus{
				Name: ctr.Name,
			}
		}
	}

	return podPhase, statuses
}

// nolint:gocyclo
func (m *Manager) translatePodCreateOptions(
	pod *corev1.Pod,
	envs map[string]map[string]string,
	authConfigs map[string]*runtimepb.ImageAuthConfig,
	volumeData map[string]*runtimepb.NamedData,
	volNames map[corev1.UniqueVolumeName]volumeNamePathPair,
	dnsConfig *aranyaapi.PodDNSConfig,
) (
	_ *runtimepb.ImageEnsureCmd,
	initOpts, workOpts *runtimepb.PodEnsureCmd,
	initHostExec, workHostExec bool,
	_ error,
) {
	var (
		sharePid             bool
		hosts                = make(map[string]string)
		containers           = make([]*runtimepb.ContainerSpec, len(pod.Spec.Containers))
		sysctls              = make(map[string]string)
		hostPaths            = make(map[string]string)
		imagePull            = make(map[string]*runtimepb.ImagePullSpec)
		volumeDataForWorkCtr = make(map[string]*runtimepb.NamedData)
		hostPathsForWorkCtr  = make(map[string]string)
	)

	netOpts := translatePodNetworkOptions(pod, m.netMgr.GetPodCIDR(false), m.netMgr.GetPodCIDR(true), dnsConfig)
	netReq, err := abbotgopb.NewRequest(netOpts)
	if err != nil {
		return nil, nil, nil, false, false, fmt.Errorf("failed to create network request: %w", err)
	}

	netReqBytes, err := netReq.Marshal()
	if err != nil {
		return nil, nil, nil, false, false, fmt.Errorf("failed to encode network request: %w", err)
	}

	for _, alias := range pod.Spec.HostAliases {
		hosts[alias.IP] = strings.Join(alias.Hostnames, " ")
	}

	for _, vol := range pod.Spec.Volumes {
		switch {
		case vol.HostPath != nil:
			hostPaths[vol.Name] = vol.HostPath.Path
		case vol.EmptyDir != nil:
			hostPaths[vol.Name] = ""
		case vol.PersistentVolumeClaim != nil:
			fallthrough
		case vol.CSI != nil:
			if !m.StorageEnabled() {
				return nil, nil, nil, false, false, fmt.Errorf("storage not enabled")
			}

			var found bool
			for _, v := range volNames {
				if v.name == vol.Name {
					found = true
					hostPaths[vol.Name] = v.path
				}
			}

			if !found {
				return nil, nil, nil, false, false, fmt.Errorf("no remote path found for volume %q", vol.Name)
			}
		}
	}

	if pod.Spec.SecurityContext != nil {
		for _, s := range pod.Spec.SecurityContext.Sysctls {
			sysctls[s.Name] = s.Value
		}
	}

	if pod.Spec.ShareProcessNamespace != nil {
		sharePid = *pod.Spec.ShareProcessNamespace
	}

	hostExecImageFound := 0
	for i, ctr := range pod.Spec.Containers {
		containers[i] = translateContainerSpec(pod, envs, &pod.Spec.Containers[i])

		// check if is virtual image
		if ctr.Image == constant.VirtualImageNameHostExec {
			workHostExec = true
			hostExecImageFound++
		} else {
			// real image to pull
			imagePull[ctr.Image] = &runtimepb.ImagePullSpec{
				AuthConfig: authConfigs[ctr.Image],
				PullPolicy: translateImagePullPolicy(ctr.ImagePullPolicy),
			}
		}

		for _, vol := range ctr.VolumeMounts {
			if namedData, ok := volumeData[vol.Name]; ok {
				volumeDataForWorkCtr[vol.Name] = namedData
			}

			if hostPath, ok := hostPaths[vol.Name]; ok {
				hostPathsForWorkCtr[vol.Name] = hostPath
			}
		}
	}

	if !(hostExecImageFound == 0 || hostExecImageFound == len(pod.Spec.Containers)) {
		// only valid if all container image is virtual image or not at the same time
		return nil, nil, nil, false, false, fmt.Errorf("invalid work container images")
	}

	if len(pod.Spec.InitContainers) != 0 {
		var (
			hostExecImageFound   = 0
			initContainers       = make([]*runtimepb.ContainerSpec, len(pod.Spec.InitContainers))
			volumeDataForInitCtr = make(map[string]*runtimepb.NamedData)
			hostPathsForInitCtr  = make(map[string]string)
		)

		for i, ctr := range pod.Spec.InitContainers {
			initContainers[i] = translateContainerSpec(pod, envs, &pod.Spec.InitContainers[i])

			if ctr.Image == constant.VirtualImageNameHostExec {
				initHostExec = true
				hostExecImageFound++
			} else {
				// real image to pull
				imagePull[ctr.Image] = &runtimepb.ImagePullSpec{
					AuthConfig: authConfigs[ctr.Image],
					PullPolicy: translateImagePullPolicy(ctr.ImagePullPolicy),
				}
			}

			for _, vol := range ctr.VolumeMounts {
				if namedData, ok := volumeData[vol.Name]; ok {
					volumeDataForInitCtr[vol.Name] = namedData
				}

				if hostPath, ok := hostPaths[vol.Name]; ok {
					hostPathsForInitCtr[vol.Name] = hostPath
				}
			}
		}

		if !(hostExecImageFound == 0 || hostExecImageFound == len(pod.Spec.InitContainers)) {
			// only valid if all container image is virtual image or not at the same time
			return nil, nil, nil, false, false, fmt.Errorf("invalid init container images")
		}

		initOpts = &runtimepb.PodEnsureCmd{
			PodUid:    string(pod.UID),
			Namespace: pod.Namespace,
			Name:      pod.Name,

			Labels: getPodLabels(pod.Labels),

			RestartPolicy: runtimepb.RESTART_NEVER,

			// kernel namespaces
			HostIpc:     pod.Spec.HostIPC,
			HostNetwork: pod.Spec.HostNetwork,
			HostPid:     pod.Spec.HostPID,
			Hostname:    pod.Spec.Hostname,
			SharePid:    sharePid,

			// network options

			Network: &runtimepb.PodNetworkSpec{
				Nameservers: dnsConfig.Servers,
				DnsSearches: dnsConfig.Searches,
				DnsOptions:  dnsConfig.Options,
				Hosts:       hosts,

				AbbotRequestBytes: netReqBytes,
			},

			Containers: initContainers,
			Wait:       true,

			Volumes: &runtimepb.PodVolumeSpec{
				HostPaths:  hostPathsForInitCtr,
				VolumeData: volumeDataForInitCtr,
			},
			Security: &runtimepb.PodSecuritySpec{
				Sysctls: sysctls,
			},
		}

		workOpts = &runtimepb.PodEnsureCmd{
			PodUid:    string(pod.UID),
			Namespace: pod.Namespace,
			Name:      pod.Name,

			RestartPolicy: translateRestartPolicy(pod.Spec.RestartPolicy),

			Containers: containers,
			Wait:       false,

			Volumes: &runtimepb.PodVolumeSpec{
				HostPaths:  hostPathsForWorkCtr,
				VolumeData: volumeDataForWorkCtr,
			},

			Security: &runtimepb.PodSecuritySpec{
				Sysctls: sysctls,
			},
		}
	} else {
		// need to create pause when creating work containers
		// require all options applied to pause container
		workOpts = &runtimepb.PodEnsureCmd{
			PodUid:    string(pod.UID),
			Namespace: pod.Namespace,
			Name:      pod.Name,

			Labels: getPodLabels(pod.Labels),

			RestartPolicy: translateRestartPolicy(pod.Spec.RestartPolicy),

			// kernel namespaces
			HostIpc:     pod.Spec.HostIPC,
			HostNetwork: pod.Spec.HostNetwork,
			HostPid:     pod.Spec.HostPID,
			Hostname:    pod.Spec.Hostname,
			SharePid:    sharePid,

			// network options

			Network: &runtimepb.PodNetworkSpec{
				Nameservers: dnsConfig.Servers,
				DnsSearches: dnsConfig.Searches,
				DnsOptions:  dnsConfig.Options,
				Hosts:       hosts,

				AbbotRequestBytes: netReqBytes,
			},

			Containers: containers,
			Wait:       false,

			Volumes: &runtimepb.PodVolumeSpec{
				HostPaths:  hostPathsForWorkCtr,
				VolumeData: volumeDataForWorkCtr,
			},
			Security: &runtimepb.PodSecuritySpec{
				Sysctls: sysctls,
			},
		}
	}

	return &runtimepb.ImageEnsureCmd{Images: imagePull}, initOpts, workOpts, initHostExec, workHostExec, nil
}

func getNamedContainerPorts(ctr *corev1.Container) map[string]int32 {
	ctrPorts := make(map[string]int32)
	for _, p := range ctr.Ports {
		if p.Name != "" {
			ctrPorts[p.Name] = p.ContainerPort
		}
	}
	return ctrPorts
}

func translateContainerSpec(
	pod *corev1.Pod,
	envs map[string]map[string]string,
	ctr *corev1.Container,
) *runtimepb.ContainerSpec {
	var (
		ctrPorts = make(map[string]int32)
	)

	for _, p := range ctr.Ports {
		if p.Name != "" {
			ctrPorts[p.Name] = p.ContainerPort
		}
	}

	mounts := make(map[string]*runtimepb.ContainerMountSpec)
	for _, volMount := range ctr.VolumeMounts {
		var (
			fileMode uint32
			remote   bool
			readOnly = volMount.ReadOnly
		)

		for _, vol := range pod.Spec.Volumes {
			if vol.Name != volMount.Name {
				continue
			}

			switch {
			case vol.ConfigMap != nil && vol.ConfigMap.DefaultMode != nil:
				fileMode = uint32(*vol.ConfigMap.DefaultMode)
			case vol.Secret != nil && vol.Secret.DefaultMode != nil:
				fileMode = uint32(*vol.Secret.DefaultMode)
			case vol.PersistentVolumeClaim != nil:
				readOnly = vol.PersistentVolumeClaim.ReadOnly
				remote = true
			case vol.CSI != nil:
				if vol.CSI.ReadOnly != nil {
					readOnly = *vol.CSI.ReadOnly
				}
				remote = true
			}
			break
		}

		mounts[volMount.Name] = &runtimepb.ContainerMountSpec{
			MountPath: volMount.MountPath,
			SubPath:   volMount.SubPath,
			ReadOnly:  readOnly,
			Type:      "",
			Options:   nil,
			FileMode:  fileMode,
			Remote:    remote,
		}
	}

	spec := &runtimepb.ContainerSpec{
		Name:  ctr.Name,
		Image: ctr.Image,

		Command: ctr.Command,
		Args:    ctr.Args,

		WorkingDir: ctr.WorkingDir,
		Stdin:      ctr.Stdin,
		Tty:        ctr.TTY,

		Envs:   envs[ctr.Name],
		Mounts: mounts,

		ReadinessCheck: translateProbe(ctr.ReadinessProbe, ctrPorts),
		LivenessCheck:  translateProbe(ctr.LivenessProbe, ctrPorts),

		Security: translateContainerSecOpts(pod.Spec.SecurityContext, ctr.SecurityContext),
	}

	if ctr.Lifecycle != nil && ctr.Lifecycle.PostStart != nil {
		spec.HookPostStart = translateHandler(ctr.Lifecycle.PostStart, ctrPorts)
	}

	return spec
}

func translateProbe(p *corev1.Probe, ports map[string]int32) *runtimepb.ContainerProbeSpec {
	if p == nil {
		return nil
	}

	return &runtimepb.ContainerProbeSpec{
		Method:           translateHandler(&p.Handler, ports),
		InitialDelay:     int64(time.Second) * int64(p.InitialDelaySeconds),
		ProbeTimeout:     int64(time.Second) * int64(p.TimeoutSeconds),
		ProbeInterval:    int64(time.Second) * int64(p.PeriodSeconds),
		SuccessThreshold: p.SuccessThreshold,
		FailureThreshold: p.FailureThreshold,
	}
}

func translateHandler(h *corev1.Handler, namedPorts map[string]int32) *runtimepb.ContainerAction {
	if h == nil {
		return nil
	}

	switch {
	case h.Exec != nil:
		return &runtimepb.ContainerAction{
			Action: &runtimepb.ContainerAction_Exec_{
				Exec: &runtimepb.ContainerAction_Exec{
					Command: h.Exec.Command,
				},
			},
		}
	case h.HTTPGet != nil:
		port := getPortValue(namedPorts, h.HTTPGet.Port)
		if port < 1 {
			return nil
		}

		var kvPair []*runtimepb.KeyValuePair
		for _, header := range h.HTTPGet.HTTPHeaders {
			kvPair = append(kvPair, &runtimepb.KeyValuePair{Key: header.Name, Value: header.Value})
		}

		return &runtimepb.ContainerAction{
			Action: &runtimepb.ContainerAction_Http{
				Http: &runtimepb.ContainerAction_HTTP{
					Method:  http.MethodGet,
					Url:     fmt.Sprintf("%s://%s:%d%s", h.HTTPGet.Scheme, h.HTTPGet.Host, port, h.HTTPGet.Path),
					Headers: kvPair,
				},
			},
		}
	case h.TCPSocket != nil:
		port := getPortValue(namedPorts, h.HTTPGet.Port)
		if port < 1 {
			return nil
		}

		return &runtimepb.ContainerAction{
			Action: &runtimepb.ContainerAction_Socket_{
				Socket: &runtimepb.ContainerAction_Socket{
					Address: fmt.Sprintf("tcp://%s:%d", h.TCPSocket.Host, port),
				},
			},
		}
	}

	return nil
}

func getPortValue(ports map[string]int32, port intstr.IntOrString) int32 {
	switch port.Type {
	case intstr.Int:
		return port.IntVal
	case intstr.String:
		if ports == nil {
			return -1
		}

		if p, ok := ports[port.StrVal]; ok {
			return p
		}
		return -1
	}
	return -1
}

func translateContainerSecOpts(
	podSecOpts *corev1.PodSecurityContext,
	ctrSecOpts *corev1.SecurityContext,
) *runtimepb.ContainerSecuritySpec {
	result := resolveCommonSecOpts(podSecOpts, ctrSecOpts)
	if result == nil || ctrSecOpts == nil {
		return result
	}

	if ctrSecOpts.AllowPrivilegeEscalation != nil {
		result.AllowNewPrivileges = *ctrSecOpts.AllowPrivilegeEscalation
	}

	if ctrSecOpts.Privileged != nil {
		result.Privileged = *ctrSecOpts.Privileged
		result.AllowNewPrivileges = true
	}

	if ctrSecOpts.ReadOnlyRootFilesystem != nil {
		result.ReadOnlyRootfs = *ctrSecOpts.ReadOnlyRootFilesystem
	}

	if ctrSecOpts.ProcMount != nil {
		switch *ctrSecOpts.ProcMount {
		case corev1.UnmaskedProcMount:
			result.ProcMountKind = runtimepb.PROC_MOUNT_UNMASKED
		default:
			result.ProcMountKind = runtimepb.PROC_MOUNT_DEFAULT
		}
	}

	if ctrSecOpts.Capabilities != nil {
		for _, capAdd := range ctrSecOpts.Capabilities.Add {
			if capAdd == "SYS_ADMIN" {
				result.AllowNewPrivileges = true
			}
			result.CapsAdd = append(result.CapsAdd, string(capAdd))
		}

		for _, capDrop := range ctrSecOpts.Capabilities.Drop {
			result.CapsDrop = append(result.CapsDrop, string(capDrop))
		}
	}

	return result
}

func resolveCommonSecOpts(
	podSecOpts *corev1.PodSecurityContext,
	ctrSecOpts *corev1.SecurityContext,
) *runtimepb.ContainerSecuritySpec {
	if podSecOpts == nil && ctrSecOpts != nil {
		return nil
	}

	return &runtimepb.ContainerSecuritySpec{
		NonRoot: func() bool {
			switch {
			case ctrSecOpts != nil && ctrSecOpts.RunAsNonRoot != nil:
				return *ctrSecOpts.RunAsNonRoot
			case podSecOpts != nil && podSecOpts.RunAsNonRoot != nil:
				return *podSecOpts.RunAsNonRoot
			}
			return false
		}(),
		User: func() int64 {
			switch {
			case ctrSecOpts != nil && ctrSecOpts.RunAsUser != nil:
				return *ctrSecOpts.RunAsUser
			case podSecOpts != nil && podSecOpts.RunAsUser != nil:
				return *podSecOpts.RunAsUser
			}
			return -1
		}(),
		Group: func() int64 {
			switch {
			case ctrSecOpts != nil && ctrSecOpts.RunAsGroup != nil:
				return *ctrSecOpts.RunAsGroup
			case podSecOpts != nil && podSecOpts.RunAsGroup != nil:
				return *podSecOpts.RunAsGroup
			}
			return -1
		}(),
		SelinuxOptions: func() *runtimepb.SELinuxOptions {
			switch {
			case ctrSecOpts != nil && ctrSecOpts.SELinuxOptions != nil:
				return &runtimepb.SELinuxOptions{
					Type:  ctrSecOpts.SELinuxOptions.Type,
					Level: ctrSecOpts.SELinuxOptions.Level,
					Role:  ctrSecOpts.SELinuxOptions.Role,
					User:  ctrSecOpts.SELinuxOptions.User,
				}
			case podSecOpts != nil && podSecOpts.SELinuxOptions != nil:
				return &runtimepb.SELinuxOptions{
					Type:  podSecOpts.SELinuxOptions.Type,
					Level: podSecOpts.SELinuxOptions.Level,
					Role:  podSecOpts.SELinuxOptions.Role,
					User:  podSecOpts.SELinuxOptions.User,
				}
			}
			return nil
		}(),
	}
}

func translateImagePullPolicy(policy corev1.PullPolicy) runtimepb.ImagePullPolicy {
	switch policy {
	case corev1.PullNever:
		return runtimepb.IMAGE_PULL_NEVER
	case corev1.PullIfNotPresent:
		return runtimepb.IMAGE_PULL_IF_NOT_PRESENT
	case corev1.PullAlways:
		return runtimepb.IMAGE_PULL_ALWAYS
	default:
		return runtimepb.IMAGE_PULL_NEVER
	}
}

func translateRestartPolicy(policy corev1.RestartPolicy) runtimepb.RestartPolicy {
	switch policy {
	case corev1.RestartPolicyAlways:
		return runtimepb.RESTART_ALWAYS
	case corev1.RestartPolicyNever:
		return runtimepb.RESTART_NEVER
	case corev1.RestartPolicyOnFailure:
		return runtimepb.RESTART_ON_FAILURE
	}
	return runtimepb.RESTART_ALWAYS
}

func getPodLabels(allLabels map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range allLabels {
		// exclude non-public labels (key without a slash)
		if strings.Contains(k, "/") {
			result[k] = v
		}
	}

	return result
}

// nolint:unused,deadcode
func getPodAnnotations(allAnnotations map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range allAnnotations {
		switch k {
		case "kubernetes.io/ingress-bandwidth",
			"kubernetes.io/egress-bandwidth",
			"kubectl.kubernetes.io/last-applied-configuration":
		default:
			result[k] = v
		}
	}

	return result
}
