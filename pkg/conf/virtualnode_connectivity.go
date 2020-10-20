package conf

import (
	"time"

	"github.com/spf13/pflag"

	"arhat.dev/aranya/pkg/constant"
)

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
