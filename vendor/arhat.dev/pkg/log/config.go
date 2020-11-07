/*
Copyright 2019 The arhat.dev Authors.

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

package log

import (
	"github.com/spf13/pflag"
)

type Config struct {
	Level       string `json:"level" yaml:"level"`
	Format      string `json:"format" yaml:"format"`
	KubeLog     bool   `json:"kubeLog" yaml:"kubeLog"`
	Destination `json:",inline" yaml:",inline"`
}

func FlagsForLogConfig(prefix string, c *Config) *pflag.FlagSet {
	fs := pflag.NewFlagSet("log", pflag.ExitOnError)

	fs.StringVarP(&c.Level, prefix+"level", "v", "error", "log level, one of [verbose, debug, info, error, silent]")
	fs.StringVar(&c.Format, prefix+"format", "console", "log output format, one of [console, json]")
	fs.StringVar(&c.File, prefix+"file", "stderr", "log to this file")

	return fs
}

type Destination struct {
	File string `json:"file" yaml:"file"`
}

type ConfigSet []Config

func (cs ConfigSet) GetUnique() ConfigSet {
	existingDest := make(map[Destination]Config)
	for _, c := range cs {
		if prev, ok := existingDest[c.Destination]; ok {
			prevLevel := parseZapLevel(prev.Level)
			myLevel := parseZapLevel(c.Level)
			if myLevel < prevLevel {
				existingDest[c.Destination] = c
			}
		} else {
			existingDest[c.Destination] = c
		}
	}

	var finalConfig ConfigSet
	for _, c := range existingDest {
		finalConfig = append(finalConfig, c)
	}

	return finalConfig
}

func (cs ConfigSet) KubeLogFile() string {
	finalConfig := cs.GetUnique()
	for _, c := range finalConfig {
		if c.KubeLog {
			switch c.File {
			case "stderr", "stdout":
			default:
				return c.File
			}
		}
	}
	return ""
}
