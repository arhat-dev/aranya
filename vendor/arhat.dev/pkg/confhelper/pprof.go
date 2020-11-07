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

import "github.com/spf13/pflag"

type PProfConfig struct {
	Enabled              bool   `json:"enabled" yaml:"enabled"`
	Listen               string `json:"listen" yaml:"listen"`
	HTTPPath             string `json:"httpPath" yaml:"httpPath"`
	MutexProfileFraction int    `json:"mutexProfileFraction" yaml:"mutexProfileFraction"`
	BlockProfileRate     int    `json:"blockProfileRate" yaml:"blockProfileRate"`
}

func FlagsForPProfConfig(prefix string, c *PProfConfig) *pflag.FlagSet {
	fs := pflag.NewFlagSet("pprof", pflag.ExitOnError)

	fs.BoolVar(&c.Enabled, prefix+"enabled", false, "enable pprof")
	fs.StringVar(&c.Listen, prefix+"listen", "", "set pprof http server listen address")
	fs.StringVar(&c.HTTPPath, prefix+"httpPath", "/debug/pprof", "set pprof server http path")
	fs.IntVar(&c.MutexProfileFraction, prefix+"mutexProfileFraction", 0, "set go/runtime mutex profile fraction")
	fs.IntVar(&c.BlockProfileRate, prefix+"blockProfileRate", 0, "set go/runtime block profile rate")

	return fs
}

func (c *PProfConfig) RunIfEnabled() error {
	if !c.Enabled {
		return nil
	}

	return nil
}
