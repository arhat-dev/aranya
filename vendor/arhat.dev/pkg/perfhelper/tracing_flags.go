// +build !noperfhelper_tracing
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
	"fmt"

	"github.com/spf13/pflag"

	"arhat.dev/pkg/envhelper"
	"arhat.dev/pkg/tlshelper"
)

func FlagsForTracing(prefix string, c *TracingConfig) *pflag.FlagSet {
	fs := pflag.NewFlagSet("tracing", pflag.ExitOnError)

	fs.BoolVar(&c.Enabled, prefix+"enabled", false, "enable tracing")
	fs.StringVar(&c.Format, prefix+"format", "jaeger", "set tracing stats format")
	fs.StringVar(&c.EndpointType, prefix+"endpointType", "agent",
		"set endpoint type of collector (only used for jaeger)")
	fs.StringVar(&c.Endpoint, prefix+"endpoint", "", "set collector endpoint for tracing stats collection")
	fs.Float64Var(&c.SampleRate, prefix+"sampleRate", 1.0, "set tracing sample rate")
	fs.StringVar(&c.ServiceName, prefix+"reportedServiceName",
		fmt.Sprintf("%s.%s", envhelper.ThisPodName(), envhelper.ThisPodNS()), "set service name used for tracing stats")
	fs.AddFlagSet(tlshelper.FlagsForTLSConfig(prefix+"tls", &c.TLS))

	return fs
}
