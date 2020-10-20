package conf

import (
	"strconv"
	"time"

	"arhat.dev/pkg/confhelper"
	"github.com/spf13/pflag"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
)

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
