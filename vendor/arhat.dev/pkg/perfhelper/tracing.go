// +build !noperfhelper_tracing

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

type TracingConfig struct {
	// Enabled tracing stats
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Format of exposed tracing stats, one of [otlp, zipkin, jaeger]
	Format string `json:"format" yaml:"format"`

	// EndpointType the type of collector used by jaeger, can be one of [agent, collector]
	EndpointType string `json:"endpointType" yaml:"endpointType"`

	// Endpoint to report tracing stats
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// SampleRate
	SampleRate float64 `json:"sampleRate" yaml:"sampleRate"`

	// ServiceName used when reporting tracing stats
	ServiceName string `json:"serviceName" yaml:"serviceName"`

	// TLS config for client/server
	TLS tlshelper.TLSConfig `json:"tls" yaml:"tls"`
}
