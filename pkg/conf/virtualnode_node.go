package conf

import (
	"time"

	"github.com/spf13/pflag"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
)

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

	Metrics []VirtualnodeNodeMetricsConfig `json:"metrics" yaml:"metrics"`
}

type VirtualnodeNodeMetricsConfig struct {
	// OS name, metrics differs from different OSes
	OS string `json:"os" yaml:"os"`

	aranyaapi.MetricsConfig `json:",inline" yaml:",inline"`
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

	return flags
}
