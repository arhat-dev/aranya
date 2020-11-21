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
