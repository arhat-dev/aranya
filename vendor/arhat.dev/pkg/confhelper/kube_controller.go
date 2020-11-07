// +build !nocloud,!nokube

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

package confhelper

import (
	"github.com/spf13/pflag"

	"arhat.dev/pkg/log"
)

func FlagsForControllerConfig(name, prefix string, l *log.Config, c *ControllerConfig) *pflag.FlagSet {
	fs := pflag.NewFlagSet("kube.controller", pflag.ExitOnError)

	// logging
	fs.AddFlagSet(log.FlagsForLogConfig("log.", l))

	// kube client
	fs.AddFlagSet(FlagsForKubeClient(prefix+"kubeClient", &c.KubeClient))

	// metrics
	fs.AddFlagSet(FlagsForMetrics(prefix+"metrics.", &c.Metrics))

	// tracing
	fs.AddFlagSet(FlagsForTracing(prefix+"tracing.", &c.Tracing))

	// leader-election
	fs.AddFlagSet(FlagsForLeaderElection(name, prefix+"leaderElection.", &c.LeaderElection))

	return fs
}

type ControllerConfig struct {
	Log            log.ConfigSet        `json:"log" yaml:"log"`
	KubeClient     KubeClientConfig     `json:"kubeClient" yaml:"kubeClient"`
	Metrics        MetricsConfig        `json:"metrics" yaml:"metrics"`
	Tracing        TracingConfig        `json:"tracing" yaml:"tracing"`
	LeaderElection LeaderElectionConfig `json:"leaderElection" yaml:"leaderElection"`
	PProf          PProfConfig          `json:"pprof" yaml:"pprof"`
}
