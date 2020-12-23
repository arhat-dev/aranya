// +build !noperfhelper_pprof

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

package perfhelper

import (
	"arhat.dev/pkg/tlshelper"
)

// nolint:maligned
type PProfConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Listen   string `json:"listen" yaml:"listen"`
	HTTPPath string `json:"httpPath" yaml:"httpPath"`

	ApplyProfileConfig    bool `json:"applyProfileConfig" yaml:"applyProfileConfig"`
	CPUProfileFrequencyHz int  `json:"cpuProfileFrequencyHz" yaml:"cpuProfileFrequencyHz"`
	MutexProfileFraction  int  `json:"mutexProfileFraction" yaml:"mutexProfileFraction"`
	BlockProfileFraction  int  `json:"blockProfileFraction" yaml:"blockProfileFraction"`

	TLS tlshelper.TLSConfig `json:"tls" yaml:"tls"`
}
