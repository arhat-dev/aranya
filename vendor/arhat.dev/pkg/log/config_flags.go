// +build !noflaghelper

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

import "github.com/spf13/pflag"

func FlagsForLogConfig(prefix string, c *Config) *pflag.FlagSet {
	fs := pflag.NewFlagSet("log", pflag.ExitOnError)

	fs.StringVarP(&c.Level, prefix+"level", "v", "error", "log level, one of [verbose, debug, info, error, silent]")
	fs.StringVar(&c.Format, prefix+"format", "console", "log output format, one of [console, json]")
	fs.StringVar(&c.File, prefix+"file", "stderr", "log to this file")

	return fs
}
