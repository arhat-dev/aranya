// +build !noperfhelper_pprof
// +build !noconfhelper_pprof

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

func (c *PProfConfig) CreateHTTPHandlersIfEnabled() map[string]http.Handler {
	if !c.Enabled {
		return nil
	}

	if c.ApplyProfileConfig {
		runtime.SetCPUProfileRate(c.CPUProfileFrequencyHz)
		runtime.SetMutexProfileFraction(c.MutexProfileFraction)
		runtime.SetBlockProfileRate(c.BlockProfileFraction)
	}

	return map[string]http.Handler{
		"":             http.HandlerFunc(pprofweb.Index),
		"cmdline":      http.HandlerFunc(pprofweb.Cmdline),
		"symbol":       http.HandlerFunc(pprofweb.Symbol),
		"trace":        http.HandlerFunc(pprofweb.Trace),
		"profile":      http.HandlerFunc(pprofweb.Profile),
		"allocs":       pprofweb.Handler("allocs"),
		"block":        pprofweb.Handler("block"),
		"goroutine":    pprofweb.Handler("goroutine"),
		"heap":         pprofweb.Handler("heap"),
		"mutex":        pprofweb.Handler("mutex"),
		"threadcreate": pprofweb.Handler("threadcreate"),
	}
}
