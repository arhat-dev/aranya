// +build !noperfhelper_metrics
// +build !noconfhelper_metrics

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
	"net/http"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	otprom "go.opentelemetry.io/otel/exporters/metric/prometheus"
	otexporterotlp "go.opentelemetry.io/otel/exporters/otlp"
	otelmetric "go.opentelemetry.io/otel/metric"
	otsdkmetricspull "go.opentelemetry.io/otel/sdk/metric/controller/pull"
	"go.opentelemetry.io/otel/sdk/metric/controller/push"
	"go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"google.golang.org/grpc/credentials"
)

func (c *MetricsConfig) CreateIfEnabled(setGlobal bool) (otelmetric.MeterProvider, http.Handler, error) {
	if !c.Enabled {
		return nil, nil, nil
	}

	var (
		metricsProvider otelmetric.MeterProvider
		httpHandler     http.Handler
	)

	switch c.Format {
	case "otlp":
		// get client tls config
		tlsConfig, err := c.TLS.GetTLSConfig(false)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create tls config: %w", err)
		}

		opts := []otexporterotlp.ExporterOption{
			otexporterotlp.WithAddress(c.Endpoint),
		}

		if tlsConfig != nil {
			opts = append(opts, otexporterotlp.WithTLSCredentials(credentials.NewTLS(tlsConfig)))
		} else {
			opts = append(opts, otexporterotlp.WithInsecure())
		}

		var exporter *otexporterotlp.Exporter
		exporter, err = otexporterotlp.NewExporter(opts...)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create otlp exporter: %w", err)
		}

		pusher := push.New(
			basic.New(simple.NewWithExactDistribution(), exporter),
			exporter,
			push.WithPeriod(5*time.Second),
		)
		pusher.Start()

		metricsProvider = pusher.MeterProvider()
	case "prometheus":
		promCfg := otprom.Config{Registry: prom.NewRegistry()}

		var exporter *otprom.Exporter
		exporter, err := otprom.NewExportPipeline(promCfg,
			otsdkmetricspull.WithCachePeriod(5*time.Second),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to install global metrics collector")
		}

		httpHandler = exporter
		metricsProvider = exporter.MeterProvider()
	default:
		return nil, nil, fmt.Errorf("unsupported metrics format %q", c.Format)
	}

	if setGlobal {
		otel.SetMeterProvider(metricsProvider)
	}

	return metricsProvider, httpHandler, nil
}
