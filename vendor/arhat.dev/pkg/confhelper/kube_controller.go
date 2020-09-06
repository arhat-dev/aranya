// +build !nocloud,!nokube

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
