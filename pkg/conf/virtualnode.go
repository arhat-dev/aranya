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

	"arhat.dev/pkg/kubehelper"
	"github.com/spf13/pflag"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
)

// VirtualnodeConfig the virtual node config
type VirtualnodeConfig struct {
	KubeClient   kubehelper.KubeClientConfig   `json:"kubeClient" yaml:"kubeClient"`
	Node         VirtualnodeNodeConfig         `json:"node" yaml:"node"`
	Pod          VirtualnodePodConfig          `json:"pod" yaml:"pod"`
	Storage      VirtualnodeStorageConfig      `json:"storage" yaml:"storage"`
	Network      VirtualnodeNetworkConfig      `json:"network" yaml:"network"`
	Connectivity VirtualnodeConnectivityConfig `json:"connectivity" yaml:"connectivity"`
}

func FlagsForVirtualnode(prefix string, config *VirtualnodeConfig) *pflag.FlagSet {
	flags := pflag.NewFlagSet("virtualnode", pflag.ExitOnError)

	flags.AddFlagSet(kubehelper.FlagsForKubeClient(prefix+"kube.", &config.KubeClient))

	flags.AddFlagSet(FlagsForVirtualnodeConnectivityConfig(prefix+"conn.", &config.Connectivity))
	flags.AddFlagSet(FlagsForVirtualnodePodConfig(prefix+"pod.", &config.Pod))
	flags.AddFlagSet(FlagsForVirtualnodeNodeConfig(prefix+"node.", &config.Node))
	flags.AddFlagSet(FlagsForVirtualnodeNodeStorageConfig(prefix+"storage.", &config.Storage))

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

	// enable storage support only when configured in both EdgeDevice CR and aranya
	newConfig.Storage.Enabled = spec.Storage.Enabled && newConfig.Storage.Enabled
	// enabled network support only when configured in both EdgeDevice CR and aranya
	newConfig.Network.Enabled = spec.Network.Enabled && newConfig.Network.Enabled

	if spec.Node.Metrics != nil {
		newConfig.Node.Metrics = []VirtualnodeNodeMetricsConfig{{
			OS:            "",
			MetricsConfig: *spec.Node.Metrics,
		}}
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
