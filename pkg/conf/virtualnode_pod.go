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
	"time"

	"github.com/spf13/pflag"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
)

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
		constant.DefaultStreamCreationTimeout, "kubectl stream creation timeout (exec, attach, port-forward)")
	flags.DurationVar(&config.Timers.StreamIdleTimeout, prefix+"streamIdleTimeout",
		constant.DefaultStreamIdleTimeout, "kubectl stream idle timeout (exec, attach, port-forward)")

	flags.IntVar(&config.Allocatable, prefix+"allocatable", 10, "set max pods assigned to this node")

	flags.AddFlagSet(aranyaapi.FlagsForPodDNSConfig(prefix+"dns.", &config.DNS.PodDNSConfig))
	flags.StringVar(&config.DNS.ClusterDomain, prefix+"clusterDomain", "cluster.local",
		"set cluster domain, should be the same with your kubelets' clusterDomain value")

	flags.AddFlagSet(aranyaapi.FlagsForMetricsConfig(prefix+"metrics.", &config.Metrics))

	return flags
}
