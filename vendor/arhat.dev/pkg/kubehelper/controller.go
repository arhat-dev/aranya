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

package kubehelper

import (
	"arhat.dev/pkg/log"
	"arhat.dev/pkg/perfhelper"
)

type ControllerConfig struct {
	Log            log.ConfigSet            `json:"log" yaml:"log"`
	KubeClient     KubeClientConfig         `json:"kubeClient" yaml:"kubeClient"`
	Metrics        perfhelper.MetricsConfig `json:"metrics" yaml:"metrics"`
	Tracing        perfhelper.TracingConfig `json:"tracing" yaml:"tracing"`
	LeaderElection LeaderElectionConfig     `json:"leaderElection" yaml:"leaderElection"`
	PProf          perfhelper.PProfConfig   `json:"pprof" yaml:"pprof"`
}
