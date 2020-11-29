// +build !noperfhelper_tracing
// +build !noconfhelper_tracing

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
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	otexporterotlp "go.opentelemetry.io/otel/exporters/otlp"
	otexporterjaeger "go.opentelemetry.io/otel/exporters/trace/jaeger"
	otexporterzipkin "go.opentelemetry.io/otel/exporters/trace/zipkin"
	otsdkresource "go.opentelemetry.io/otel/sdk/resource"
	otsdktrace "go.opentelemetry.io/otel/sdk/trace"
	otsemconv "go.opentelemetry.io/otel/semconv"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
)

func (c *TracingConfig) CreateIfEnabled(setGlobal bool, client *http.Client) (oteltrace.TracerProvider, error) {
	if !c.Enabled {
		return nil, nil
	}

	var (
		traceProvider oteltrace.TracerProvider
	)

	tlsConfig, err := c.TLS.GetTLSConfig(true)
	if err != nil {
		return nil, fmt.Errorf("failed to create tls config: %w", err)
	}

	switch c.Format {
	case "otlp":
		opts := []otexporterotlp.ExporterOption{
			otexporterotlp.WithAddress(c.Endpoint),
		}

		if tlsConfig != nil {
			opts = append(opts, otexporterotlp.WithTLSCredentials(credentials.NewTLS(tlsConfig)))
		} else {
			opts = append(opts, otexporterotlp.WithInsecure())
		}

		exporter, err2 := otexporterotlp.NewExporter(opts...)
		if err2 != nil {
			return nil, fmt.Errorf("failed to create otlp exporter: %w", err2)
		}

		bsp := otsdktrace.NewBatchSpanProcessor(exporter)

		svcNameRes, err2 := otsdkresource.New(context.TODO(),
			otsdkresource.WithAttributes(otsemconv.ServiceNameKey.String(c.ServiceName)),
		)
		if err2 != nil {
			return nil, fmt.Errorf("failed to create service name resource: %w", err2)
		}

		traceProvider = otsdktrace.NewTracerProvider(
			otsdktrace.WithResource(svcNameRes),
			otsdktrace.WithConfig(otsdktrace.Config{DefaultSampler: otsdktrace.TraceIDRatioBased(c.SampleRate)}),
			otsdktrace.WithSyncer(exporter),
			otsdktrace.WithSpanProcessor(bsp),
		)
	case "zipkin":
		exporter, err2 := otexporterzipkin.NewRawExporter(c.Endpoint, c.ServiceName,
			otexporterzipkin.WithClient(c.newHTTPClient(client, tlsConfig)),
			otexporterzipkin.WithLogger(nil),
		)
		if err2 != nil {
			return nil, fmt.Errorf("failed to create zipkin exporter: %w", err2)
		}

		traceProvider = otsdktrace.NewTracerProvider(
			otsdktrace.WithBatcher(exporter,
				otsdktrace.WithBatchTimeout(5*time.Second),
			),
			otsdktrace.WithConfig(otsdktrace.Config{
				DefaultSampler: otsdktrace.TraceIDRatioBased(c.SampleRate),
			}),
		)
	case "jaeger":
		var endpoint otexporterjaeger.EndpointOption
		switch c.EndpointType {
		case "agent":
			endpoint = otexporterjaeger.WithAgentEndpoint(
				c.Endpoint,
				otexporterjaeger.WithAttemptReconnectingInterval(5*time.Second),
			)
		case "collector":
			endpoint = otexporterjaeger.WithCollectorEndpoint(
				c.Endpoint,
				// use environ JAEGER_USERNAME and JAEGER_PASSWORD
				otexporterjaeger.WithCollectorEndpointOptionFromEnv(),
				otexporterjaeger.WithHTTPClient(c.newHTTPClient(client, tlsConfig)),
			)
		default:
			return nil, fmt.Errorf("unsupported tracing endpoint type %q", c.EndpointType)
		}

		var flush func()
		traceProvider, flush, err = otexporterjaeger.NewExportPipeline(
			endpoint,
			otexporterjaeger.WithProcess(otexporterjaeger.Process{
				ServiceName: c.ServiceName,
			}),
			otexporterjaeger.WithSDK(&otsdktrace.Config{
				DefaultSampler: otsdktrace.TraceIDRatioBased(c.SampleRate),
			}),
		)
		_ = flush
	default:
		return nil, fmt.Errorf("unsupported tracing format %q", c.Format)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create %q tracing provider: %w", c.Format, err)
	}

	if setGlobal {
		otel.SetTracerProvider(traceProvider)
	}

	return traceProvider, nil
}

func (c *TracingConfig) newHTTPClient(client *http.Client, tlsConfig *tls.Config) *http.Client {
	if tlsConfig != nil {
		tlsConfig.NextProtos = []string{"h2", "http/1.1"}
	}

	if client == nil {
		client = &http.Client{
			// TODO: set reasonable defaults, currently using default client and transport
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:       30 * time.Second,
					KeepAlive:     30 * time.Second,
					FallbackDelay: 300 * time.Millisecond,
				}).DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				TLSClientConfig:       tlsConfig,

				DisableKeepAlives:      false,
				DisableCompression:     false,
				MaxIdleConnsPerHost:    0,
				MaxConnsPerHost:        0,
				ResponseHeaderTimeout:  0,
				TLSNextProto:           nil,
				ProxyConnectHeader:     nil,
				MaxResponseHeaderBytes: 0,
				WriteBufferSize:        0,
				ReadBufferSize:         0,
			},
		}
	}

	return &http.Client{
		Transport:     client.Transport,
		CheckRedirect: client.CheckRedirect,
		Jar:           client.Jar,
		Timeout:       client.Timeout,
	}
}
