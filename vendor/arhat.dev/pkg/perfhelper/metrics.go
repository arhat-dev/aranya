// +build !noperfhelper_metrics

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

type MetricsConfig struct {
	// Enabled metrics collection
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Format of exposed metrics, one of [prometheus, otlp]
	Format string `json:"format" yaml:"format"`

	// Endpoint address for metrics/tracing collection,
	// when format is prometheus: it's a listen address (SHOULD NOT be empty or use random port (:0))
	// when format is otlp: it's the otlp collector address
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// HTTPPath for metrics collection, used when format is prometheus
	HTTPPath string `json:"httpPath" yaml:"httpPath"`

	// TLS config for client/server
	TLS tlshelper.TLSConfig `json:"tls" yaml:"tls"`
}
