// +build !noperfhelper_pprof
// +build !noflaghelper

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

import "github.com/spf13/pflag"

func FlagsForPProfConfig(prefix string, c *PProfConfig) *pflag.FlagSet {
	fs := pflag.NewFlagSet("pprof", pflag.ExitOnError)

	fs.BoolVar(&c.Enabled, prefix+"enabled", false, "enable pprof")
	fs.StringVar(&c.Listen, prefix+"listen", "", "set pprof http server listen address")
	fs.StringVar(&c.HTTPPath, prefix+"httpPath", "/debug/pprof", "set pprof server http path")

	fs.IntVar(&c.CPUProfileFrequencyHz, prefix+"cpuProfileFrequencyHz", 100, "set go/runtime cpu profile frequency")
	fs.IntVar(&c.MutexProfileFraction, prefix+"mutexProfileFraction", 100, "set go/runtime mutex profile fraction")
	fs.IntVar(&c.BlockProfileFraction, prefix+"blockProfileFraction", 100, "set go/runtime block profile fraction")

	return fs
}
