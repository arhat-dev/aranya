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

package conf

import (
	"strconv"
	"time"

	"arhat.dev/pkg/confhelper"
	"arhat.dev/pkg/log"
	"github.com/spf13/pflag"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
)

type AranyaConfig struct {
	Aranya      AranyaAppConfig   `json:"aranya" yaml:"aranya"`
	VirtualNode VirtualnodeConfig `json:"virtualnode" yaml:"virtualnode"`
}

func (c *AranyaConfig) GetLogConfig() log.ConfigSet {
	return c.Aranya.Log
}

func (c *AranyaConfig) SetLogConfig(l log.ConfigSet) {
	c.Aranya.Log = l
}

// nolint:lll
type AranyaAppConfig struct {
	confhelper.ControllerConfig `json:",inline" yaml:",inline"`

	MaxVirtualnodeCreatingInParallel int  `json:"maxVirtualnodeCreatingInParallel" yaml:"maxVirtualnodeCreatingInParallel"`
	RunAsCloudProvider               bool `json:"runAsCloudProvider" yaml:"runAsCloudProvider"`

	Managed struct {
		ConnectivityService struct {
			// aranya will only manage Service.spec.{selector, ports[].{name, port, targetPort}}
			Name string `json:"name" yaml:"name"`
		} `json:"connectivityService" yaml:"connectivityService"`

		SFTPService struct {
			// aranya will only manage Service.spec.{selector, ports[].{name, port, targetPort}}
			Name string `json:"name" yaml:"name"`
		} `json:"sftpService" yaml:"sftpService"`

		// NodeClusterRoles managed for node access
		// aranya will add nodes related to the edgedevices in watch namespace to this cluster role
		NodeClusterRoles map[string]aranyaapi.NodeClusterRolePermissions `json:"nodeClusterRoles" yaml:"nodeClusterRoles"`
		PodRoles         map[string]aranyaapi.PodRolePermissions         `json:"podRoles" yaml:"podRoles"`
		VirtualPodRoles  map[string]aranyaapi.PodRolePermissions         `json:"virtualPodRoles" yaml:"virtualPodRoles"`
	} `json:"managed" yaml:"managed"`
}

func FlagsForAranyaAppConfig(prefix string, config *AranyaAppConfig) *pflag.FlagSet {
	flags := pflag.NewFlagSet("aranya.app", pflag.ExitOnError)

	flags.IntVar(&config.MaxVirtualnodeCreatingInParallel, "maxVirtualnodeCreatingInParallel", 0,
		"set how many virtualnode can be creating in parallel, values <= 0 means no limit")

	flags.StringVar(&config.Managed.SFTPService.Name, prefix+"managed.sftpService.name",
		"edgedevice-sftp", "set sftp service resource name managed by aranya (for remote csi storage feature)")
	flags.StringVar(&config.Managed.ConnectivityService.Name, prefix+"managed.connectivity.name",
		"edgedevice-connectivity",
		"set the grpc service resource name managed by aranya (for EdgeDevices with grpc connectivity method)")

	return flags
}

type VirtualnodeConnectivityConfig struct {
	Timers struct {
		UnarySessionTimeout time.Duration `json:"unarySessionTimeout" yaml:"unarySessionTimeout"`
	} `json:"timers" yaml:"timers"`

	Backoff struct {
		InitialDelay time.Duration `json:"initialDelay" yaml:"initialDelay"`
		MaxDelay     time.Duration `json:"maxDelay" yaml:"maxDelay"`
		Factor       float64       `json:"factor" yaml:"factor"`
	} `json:"backoff" yaml:"backoff"`
}

func FlagsForVirtualnodeConnectivityConfig(prefix string, config *VirtualnodeConnectivityConfig) *pflag.FlagSet {
	flags := pflag.NewFlagSet("virtualnode.conn", pflag.ExitOnError)

	flags.DurationVar(&config.Timers.UnarySessionTimeout, prefix+"unarySessionTimeout",
		constant.DefaultUnarySessionTimeout, "timeout duration for unary session")

	flags.DurationVar(&config.Backoff.InitialDelay, prefix+"backoff.initialDelay",
		constant.DefaultBackoffInitialDelay, "")
	flags.DurationVar(&config.Backoff.MaxDelay, prefix+"backoff.maxDelay",
		constant.DefaultBackoffMaxDelay, "")
	flags.Float64Var(&config.Backoff.Factor, prefix+"backoff.factor",
		constant.DefaultBackoffFactor, "")

	return flags
}

// VirtualnodePodConfig of pod manager
type VirtualnodePodConfig struct {
	LogDir string `json:"logDir" yaml:"logDir"`
	Timers struct {
		ForceSyncInterval     time.Duration `json:"forceSyncInterval" yaml:"forceSyncInterval"`
		StreamCreationTimeout time.Duration `json:"streamCreationTimeout" yaml:"streamCreationTimeout"`
		StreamIdleTimeout     time.Duration `json:"streamIdleTimeout" yaml:"streamIdleTimeout"`
	} `json:"timers" yaml:"timers"`

	DNS struct {
		aranyaapi.PodDNSConfig `json:",inline" yaml:",inline"`
		ClusterDomain          string `json:"clusterDomain" yaml:"clusterDomain"`
	} `json:"dns" yaml:"dns"`

	Allocatable int `json:"allocatable" yaml:"allocatable"`

	Metrics aranyaapi.MetricsConfig `json:"metrics" yaml:"metrics"`
}

func FlagsForVirtualnodePodConfig(prefix string, config *VirtualnodePodConfig) *pflag.FlagSet {
	flags := pflag.NewFlagSet("virtualnode.pod", pflag.ExitOnError)

	flags.StringVar(&config.LogDir, prefix+"logDir", constant.DefaultAranyaPodLogDir,
		"log dir for virtual pod logs")
	flags.DurationVar(&config.Timers.ForceSyncInterval, prefix+"forceSyncInterval", 0,
		"device pod status sync interval, reject device if operation failed")
	flags.DurationVar(&config.Timers.StreamCreationTimeout, prefix+"streamCreationTimeout",
		constant.DefaultStreamCreationTimeout, "kubectl stream creation timeout (exec, attach, port.forward)")
	flags.DurationVar(&config.Timers.StreamIdleTimeout, prefix+"streamIdleTimeout",
		constant.DefaultStreamIdleTimeout, "kubectl stream idle timeout (exec, attach, port.forward)")

	flags.IntVar(&config.Allocatable, prefix+"allocatable", 10, "set max pods assigned to this node")

	flags.AddFlagSet(aranyaapi.FlagsForPodDNSConfig(prefix+"dns.", &config.DNS.PodDNSConfig))
	flags.StringVar(&config.DNS.ClusterDomain, prefix+"clusterDomain", "cluster.local",
		"set cluster domain, should be the same with your kubelets' clusterDomain value")

	flags.AddFlagSet(aranyaapi.FlagsForMetricsConfig(prefix+"metrics.", &config.Metrics))

	return flags
}

type VirtualnodeNodeStorageConfig struct {
	Enabled                bool   `json:"enabled" yaml:"enabled"`
	RootDir                string `json:"rootDir" yaml:"rootDir"`
	KubeletPluginsDir      string `json:"kubeletPluginsDir" yaml:"kubeletPluginsDir"`
	KubeletRegistrationDir string `json:"kubeletRegistrationDir" yaml:"kubeletRegistrationDir"`
	SFTP                   struct {
		Enabled     bool   `json:"enabled" yaml:"enabled"`
		HostKeyFile string `json:"hostKey" yaml:"hostKey"`
	} `json:"sftp" yaml:"sftp"`
}

func FlagsForVirtualnodeNodeStorageConfig(prefix string, config *VirtualnodeNodeStorageConfig) *pflag.FlagSet {
	flags := pflag.NewFlagSet("virtualnode.node.storage", pflag.ExitOnError)

	flags.BoolVar(&config.Enabled, prefix+"enabled", false,
		"enable storage support in edge device by default")
	flags.StringVar(&config.RootDir, prefix+"rootDir",
		constant.DefaultAranyaStorageDir, "set dir to host pods volume mount")
	flags.StringVar(&config.KubeletPluginsDir, prefix+"kubeletPluginsDir",
		constant.DefaultKubeletPluginsDir, "set kubelet plugins dir")
	flags.StringVar(&config.KubeletRegistrationDir, prefix+"kubeletRegDir",
		constant.DefaultKubeletRegDir, "set kubelet plugin registration dir")

	// virtualnode.node.storage.sftp
	flags.BoolVar(&config.SFTP.Enabled, prefix+"sftp", true, "enable sftp server")
	flags.StringVar(&config.SFTP.HostKeyFile, prefix+"sftp.hostKeyFile",
		constant.DefaultSFTPHostKeyFile, "path to sftp host private key")

	return flags
}

// VirtualnodeNodeConfig the virtual node status update config
// nolint:maligned
type VirtualnodeNodeConfig struct {
	RecreateIfPatchFailed bool `json:"recreateIfPatchFailed" yaml:"recreateIfPatchFailed"`

	Timers struct {
		MirrorSyncInterval time.Duration `json:"mirrorSyncInterval" yaml:"mirrorSyncInterval"`
		ForceSyncInterval  time.Duration `json:"forceSyncInterval" yaml:"forceSyncInterval"`
	} `json:"timers" yaml:"timers"`

	Cert struct {
		AutoApprove bool `json:"autoApprove" yaml:"autoApprove"`
	} `json:"cert" yaml:"cert"`

	Lease struct {
		Enabled bool `json:"enabled" yaml:"enabled"`

		Duration       time.Duration `json:"duration" yaml:"duration"`
		UpdateInterval time.Duration `json:"updateInterval" yaml:"updateInterval"`
	} `json:"lease" yaml:"lease"`

	Storage VirtualnodeNodeStorageConfig `json:"storage" yaml:"storage"`
	Metrics aranyaapi.MetricsConfig      `json:"metrics" yaml:"metrics"`
}

func FlagsForVirtualnodeNodeConfig(prefix string, config *VirtualnodeNodeConfig) *pflag.FlagSet {
	flags := pflag.NewFlagSet("virtualnode.node", pflag.ExitOnError)

	flags.BoolVar(&config.Cert.AutoApprove, prefix+"cert.autoApprove", true,
		"enable node certificate auto approve")

	flags.BoolVar(&config.Lease.Enabled, prefix+"lease.enable", false,
		"use node lease instead of updating node status periodically")
	flags.DurationVar(&config.Lease.Duration, prefix+"lease.duration",
		constant.DefaultNodeLeaseDuration, "lease duration")
	flags.DurationVar(&config.Lease.UpdateInterval, prefix+"lease.updateInterval",
		constant.DefaultNodeLeaseUpdateInterval, "time interval used for node lease renew")

	flags.BoolVar(&config.RecreateIfPatchFailed, prefix+"recreateIfPatchFailed", false,
		"delete then create node object if patch failed")
	flags.DurationVar(&config.Timers.ForceSyncInterval, prefix+"forceSyncInterval", 0,
		"device node status sync interval, reject device if operation failed")
	flags.DurationVar(&config.Timers.MirrorSyncInterval, prefix+"mirrorSyncInterval",
		constant.DefaultMirrorNodeSyncInterval, "cluster node status update interval")

	flags.AddFlagSet(FlagsForVirtualnodeNodeStorageConfig(prefix+"storage.", &config.Storage))
	flags.AddFlagSet(aranyaapi.FlagsForMetricsConfig(prefix+"metrics.", &config.Metrics))

	return flags
}

// VirtualnodeConfig the virtual node config
type VirtualnodeConfig struct {
	KubeClient   confhelper.KubeClientConfig   `json:"kubeClient" yaml:"kubeClient"`
	Node         VirtualnodeNodeConfig         `json:"node" yaml:"node"`
	Pod          VirtualnodePodConfig          `json:"pod" yaml:"pod"`
	Connectivity VirtualnodeConnectivityConfig `json:"connectivity" yaml:"connectivity"`
}

func FlagsForVirtualnode(prefix string, config *VirtualnodeConfig) *pflag.FlagSet {
	flags := pflag.NewFlagSet("virtualnode", pflag.ExitOnError)

	flags.AddFlagSet(confhelper.FlagsForKubeClient("vn.", &config.KubeClient))

	flags.AddFlagSet(FlagsForVirtualnodeConnectivityConfig("conn.", &config.Connectivity))
	flags.AddFlagSet(FlagsForVirtualnodePodConfig("pod.", &config.Pod))
	flags.AddFlagSet(FlagsForVirtualnodeNodeConfig("node.", &config.Node))

	return flags
}

// OverrideWith a config with higher priority (config from EdgeDevices)
func (c *VirtualnodeConfig) OverrideWith(spec aranyaapi.EdgeDeviceSpec) *VirtualnodeConfig {
	newConfig := &VirtualnodeConfig{
		Node: c.Node,
		Pod:  c.Pod,
		Connectivity: VirtualnodeConnectivityConfig{
			Timers:  c.Connectivity.Timers,
			Backoff: c.Connectivity.Backoff,
		},
	}

	if interval := spec.Node.Timers.ForceSyncInterval; interval != nil {
		newConfig.Node.Timers.ForceSyncInterval = interval.Duration
	}

	if interval := spec.Pod.Timers.ForceSyncInterval; interval != nil {
		newConfig.Pod.Timers.ForceSyncInterval = interval.Duration
	}

	if timeout := spec.Connectivity.Timers.UnarySessionTimeout; timeout != nil && timeout.Duration >= time.Millisecond {
		newConfig.Connectivity.Timers.UnarySessionTimeout = timeout.Duration
	}

	backoff := spec.Connectivity.Backoff
	if backoff.Factor != "" {
		factor, _ := strconv.ParseFloat(backoff.Factor, 64)
		if factor > 1 {
			newConfig.Connectivity.Backoff.Factor = factor
		}
	}

	if backoff.InitialDelay != nil && backoff.InitialDelay.Duration > 100*time.Millisecond {
		newConfig.Connectivity.Backoff.InitialDelay = backoff.InitialDelay.Duration
	}

	if backoff.MaxDelay != nil && backoff.MaxDelay.Duration > 100*time.Millisecond {
		newConfig.Connectivity.Backoff.MaxDelay = backoff.MaxDelay.Duration
	}

	// enable storage support only when configured in both EdgeDevice CRD and aranya
	newConfig.Node.Storage.Enabled = spec.Node.Storage.Enabled && newConfig.Node.Storage.Enabled

	if spec.Node.Metrics != nil {
		newConfig.Node.Metrics = *spec.Node.Metrics
	}

	if dnsSpec := spec.Pod.DNS; dnsSpec != nil {
		newConfig.Pod.DNS.Servers = dnsSpec.Servers
		newConfig.Pod.DNS.Searches = dnsSpec.Searches
		newConfig.Pod.DNS.Options = dnsSpec.Options
	}

	if spec.Pod.Metrics != nil {
		newConfig.Pod.Metrics = *spec.Pod.Metrics
	}

	if spec.Pod.Allocatable > 0 {
		newConfig.Pod.Allocatable = spec.Pod.Allocatable
	}

	return newConfig
}
