// +build !noperfhelper_metrics
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

import (
	"github.com/spf13/pflag"

	"arhat.dev/pkg/tlshelper"
)

func FlagsForMetrics(prefix string, c *MetricsConfig) *pflag.FlagSet {
	fs := pflag.NewFlagSet("metrics", pflag.ExitOnError)

	fs.BoolVar(&c.Enabled, prefix+"enabled", true, "enable metrics collection")
	fs.StringVar(&c.Endpoint, prefix+"listen", ":9876", "set address:port for telemetry endpoint")
	fs.StringVar(&c.HTTPPath, prefix+"httpPath", "/metrics", "set http path for metrics collection")
	fs.StringVar(&c.Format, prefix+"format", "prometheus", "set metrics format")
	fs.AddFlagSet(tlshelper.FlagsForTLSConfig(prefix+"tls.", &c.TLS))

	return fs
}
