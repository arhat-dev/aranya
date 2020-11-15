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
	"net/http"
	pprofweb "net/http/pprof"
	"runtime"
)

type PProfConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Listen   string `json:"listen" yaml:"listen"`
	HTTPPath string `json:"httpPath" yaml:"httpPath"`

	CPUProfileFrequencyHz int `json:"cpuProfileFrequencyHz" yaml:"cpuProfileFrequencyHz"`
	MutexProfileFraction  int `json:"mutexProfileFraction" yaml:"mutexProfileFraction"`
	BlockProfileFraction  int `json:"blockProfileFraction" yaml:"blockProfileFraction"`
}

func (c *PProfConfig) CreateHTTPHandlerIfEnabled(applyProfileConfig bool) http.Handler {
	if !c.Enabled {
		return nil
	}

	if applyProfileConfig {
		runtime.SetCPUProfileRate(c.CPUProfileFrequencyHz)
		runtime.SetMutexProfileFraction(c.MutexProfileFraction)
		runtime.SetBlockProfileRate(c.BlockProfileFraction)
	}

	mux := http.NewServeMux()

	// index
	mux.HandleFunc("/", pprofweb.Index)

	// category
	mux.HandleFunc("/cmdline", pprofweb.Cmdline)
	mux.HandleFunc("/symbol", pprofweb.Symbol)
	mux.HandleFunc("/trace", pprofweb.Trace)

	// actual profile stats

	// cpu profile
	mux.HandleFunc("/profile", pprofweb.Profile)

	mux.Handle("/allocs", pprofweb.Handler("allocs"))
	mux.Handle("/block", pprofweb.Handler("block"))
	mux.Handle("/goroutine", pprofweb.Handler("goroutine"))
	mux.Handle("/heap", pprofweb.Handler("heap"))
	mux.Handle("/mutex", pprofweb.Handler("mutex"))
	mux.Handle("/threadcreate", pprofweb.Handler("threadcreate"))

	return mux
}
