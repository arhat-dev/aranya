// +build !nocloud,!notelemetry

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

package confhelper

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/pflag"
	otapiglobal "go.opentelemetry.io/otel/api/global"
	otapimetric "go.opentelemetry.io/otel/api/metric"
	otprom "go.opentelemetry.io/otel/exporters/metric/prometheus"
	otexporterotlp "go.opentelemetry.io/otel/exporters/otlp"
	otsdkmetricspull "go.opentelemetry.io/otel/sdk/metric/controller/pull"
	"go.opentelemetry.io/otel/sdk/metric/controller/push"
	"go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"google.golang.org/grpc/credentials"

	"arhat.dev/pkg/log"
)

func FlagsForMetrics(prefix string, c *MetricsConfig) *pflag.FlagSet {
	fs := pflag.NewFlagSet("metrics", pflag.ExitOnError)

	fs.BoolVar(&c.Enabled, prefix+"enabled", true, "enable metrics collection")
	fs.StringVar(&c.Endpoint, prefix+"listen", ":9876", "set address:port for telemetry endpoint")
	fs.StringVar(&c.HTTPPath, prefix+"httpPath", "/metrics", "set http path for metrics collection")
	fs.StringVar(&c.Format, prefix+"format", "prometheus", "set metrics format")
	fs.AddFlagSet(FlagsForTLSConfig(prefix+"tls.", &c.TLS))

	return fs
}

type MetricsConfig struct {
	// Enabled metrics collection
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Endpoint address for metrics/tracing collection,
	// for prometheus, it's a listen address (SHOULD NOT be empty or use random port (:0))
	// for otlp, it's the otlp collector address
	Endpoint string `json:"listen" yaml:"listen"`

	// Format of exposed metrics
	Format string `json:"format" yaml:"format"`

	// HTTPPath for metrics collection
	HTTPPath string `json:"httpPath" yaml:"httpPath"`

	// TLS config for client/server
	TLS TLSConfig `json:"tls" yaml:"tls"`
}

func (c *MetricsConfig) RegisterIfEnabled(ctx context.Context, logger log.Interface) (err error) {
	if !c.Enabled {
		return nil
	}

	var (
		metricsProvider otapimetric.MeterProvider
	)

	tlsConfig, err := c.TLS.GetTLSConfig(true)
	if err != nil {
		return fmt.Errorf("failed to create tls config: %w", err)
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

		var exporter *otexporterotlp.Exporter
		exporter, err = otexporterotlp.NewExporter(opts...)
		if err != nil {
			return fmt.Errorf("failed to create otlp exporter: %w", err)
		}

		pusher := push.New(
			basic.New(simple.NewWithExactDistribution(), exporter),
			exporter,
			push.WithPeriod(5*time.Second),
		)
		pusher.Start()

		metricsProvider = pusher.MeterProvider()
	case "prometheus":
		var metricsListener net.Listener
		metricsListener, err = net.Listen("tcp", c.Endpoint)
		if err != nil {
			return fmt.Errorf("failed to create listener for metrics: %w", err)
		}

		defer func() {
			if err != nil {
				_ = metricsListener.Close()
			}
		}()

		promCfg := otprom.Config{Registry: prom.NewRegistry()}

		var exporter *otprom.Exporter
		exporter, err = otprom.NewExportPipeline(promCfg,
			otsdkmetricspull.WithCachePeriod(5*time.Second),
		)
		if err != nil {
			return fmt.Errorf("failed to install global metrics collector")
		}

		mux := http.NewServeMux()
		mux.HandleFunc(c.HTTPPath, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				http.Error(w, fmt.Sprintf("method %q not allowed", r.Method), http.StatusMethodNotAllowed)
				return
			}
			exporter.ServeHTTP(w, r)
		})

		if tlsConfig != nil {
			tlsConfig.NextProtos = []string{"h2", "http/1.1"}
		}

		srv := http.Server{
			Handler:   mux,
			TLSConfig: tlsConfig,
			BaseContext: func(net.Listener) context.Context {
				return ctx
			},
		}

		go func() {
			var err error
			defer func() {
				_ = srv.Close()
				_ = metricsListener.Close()

				if err != nil {
					os.Exit(1)
				}
			}()

			if err = srv.Serve(metricsListener); err != nil {
				if errors.Is(err, http.ErrServerClosed) {
					err = nil
				} else {
					logger.E("failed to serve metrics", log.Error(err))
				}
			}
		}()
	default:
		return fmt.Errorf("unsupported metrics format %q", c.Format)
	}

	otapiglobal.SetMeterProvider(metricsProvider)

	return nil
}
