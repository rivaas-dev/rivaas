// Copyright 2025 The Rivaas Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"fmt"

	"rivaas.dev/logging"
	"rivaas.dev/metrics"
	"rivaas.dev/tracing"
)

// TracingProvider defines available tracing backends.
type TracingProvider string

const (
	// TracingStdout exports traces to stdout (development/testing).
	TracingStdout TracingProvider = "stdout"
	// TracingOTLP exports traces via OTLP gRPC protocol.
	TracingOTLP TracingProvider = "otlp"
	// TracingOTLPHTTP exports traces via OTLP HTTP protocol.
	TracingOTLPHTTP TracingProvider = "otlp-http"
	// TracingNoop is a no-op provider (no traces exported).
	TracingNoop TracingProvider = "noop"
)

// MetricsProvider defines available metrics backends.
type MetricsProvider string

const (
	// MetricsPrometheus uses Prometheus exporter for metrics.
	MetricsPrometheus MetricsProvider = "prometheus"
	// MetricsOTLP uses OTLP HTTP exporter for metrics.
	MetricsOTLP MetricsProvider = "otlp"
	// MetricsStdout uses stdout exporter for metrics (development/testing).
	MetricsStdout MetricsProvider = "stdout"
)

// LoggingHandler defines log output formats.
type LoggingHandler string

const (
	// LoggingConsole uses console handler (human-readable).
	LoggingConsole LoggingHandler = "console"
	// LoggingJSON uses JSON handler (machine-readable).
	LoggingJSON LoggingHandler = "json"
)

// LoggingLevel defines log levels.
type LoggingLevel string

const (
	// LoggingDebug enables debug-level logging.
	LoggingDebug LoggingLevel = "debug"
	// LoggingInfo enables info-level logging.
	LoggingInfo LoggingLevel = "info"
	// LoggingWarn enables warn-level logging.
	LoggingWarn LoggingLevel = "warn"
	// LoggingError enables error-level logging.
	LoggingError LoggingLevel = "error"
)

// TracingConfig configures distributed tracing.
// This struct can be loaded from configuration files (YAML, JSON, etc.).
//
// Example YAML:
//
//	tracing:
//	  provider: otlp
//	  endpoint: localhost:4317
//	  sampleRate: 0.1
//	  insecure: true
type TracingConfig struct {
	Provider   TracingProvider `config:"provider" json:"provider" yaml:"provider"`
	Endpoint   string          `config:"endpoint" json:"endpoint" yaml:"endpoint"`
	SampleRate float64         `config:"sampleRate" json:"sampleRate" yaml:"sampleRate"`
	Insecure   bool            `config:"insecure" json:"insecure" yaml:"insecure"`
}

// options converts TracingConfig to tracing.Option slice.
// This is the bridge between declarative config and the functional options API.
func (c TracingConfig) options() ([]tracing.Option, error) {
	var opts []tracing.Option

	switch c.Provider {
	case TracingStdout:
		opts = append(opts, tracing.WithStdout())
	case TracingOTLP:
		endpoint := c.Endpoint
		if endpoint == "" {
			endpoint = "localhost:4317"
		}
		if c.Insecure {
			opts = append(opts, tracing.WithOTLP(endpoint, tracing.OTLPInsecure()))
		} else {
			opts = append(opts, tracing.WithOTLP(endpoint))
		}
	case TracingOTLPHTTP:
		endpoint := c.Endpoint
		if endpoint == "" {
			endpoint = "http://localhost:4318"
		}
		opts = append(opts, tracing.WithOTLPHTTP(endpoint))
	case TracingNoop, "":
		opts = append(opts, tracing.WithNoop())
	default:
		return nil, fmt.Errorf("unknown provider %q (valid: stdout, otlp, otlp-http, noop)", c.Provider)
	}

	if c.SampleRate > 0 && c.SampleRate <= 1.0 {
		opts = append(opts, tracing.WithSampleRate(c.SampleRate))
	}

	return opts, nil
}

// MetricsConfig configures metrics collection.
// This struct can be loaded from configuration files (YAML, JSON, etc.).
//
// Example YAML:
//
//	metrics:
//	  provider: prometheus
//	  endpoint: ":9090"
//	  path: /metrics
type MetricsConfig struct {
	Provider MetricsProvider `config:"provider" json:"provider" yaml:"provider"`
	Endpoint string          `config:"endpoint" json:"endpoint" yaml:"endpoint"`
	Path     string          `config:"path" json:"path" yaml:"path"`
}

// options converts MetricsConfig to metrics.Option slice.
// This is the bridge between declarative config and the functional options API.
func (c MetricsConfig) options() ([]metrics.Option, error) {
	switch c.Provider {
	case MetricsPrometheus, "":
		endpoint := c.Endpoint
		if endpoint == "" {
			endpoint = ":9090"
		}
		path := c.Path
		if path == "" {
			path = "/metrics"
		}
		return []metrics.Option{metrics.WithPrometheus(endpoint, path)}, nil
	case MetricsOTLP:
		endpoint := c.Endpoint
		if endpoint == "" {
			endpoint = "localhost:4318"
		}
		return []metrics.Option{metrics.WithOTLP(endpoint)}, nil
	case MetricsStdout:
		return []metrics.Option{metrics.WithStdout()}, nil
	default:
		return nil, fmt.Errorf("unknown provider %q (valid: prometheus, otlp, stdout)", c.Provider)
	}
}

// LoggingConfig configures structured logging.
// This struct can be loaded from configuration files (YAML, JSON, etc.).
//
// Example YAML:
//
//	logging:
//	  handler: json
//	  level: info
type LoggingConfig struct {
	Handler LoggingHandler `config:"handler" json:"handler" yaml:"handler"`
	Level   LoggingLevel   `config:"level" json:"level" yaml:"level"`
}

// options converts LoggingConfig to logging.Option slice.
// This is the bridge between declarative config and the functional options API.
func (c LoggingConfig) options() ([]logging.Option, error) {
	var opts []logging.Option

	switch c.Handler {
	case LoggingJSON:
		opts = append(opts, logging.WithJSONHandler())
	case LoggingConsole, "":
		opts = append(opts, logging.WithConsoleHandler())
	default:
		return nil, fmt.Errorf("unknown handler %q (valid: console, json)", c.Handler)
	}

	switch c.Level {
	case LoggingDebug:
		opts = append(opts, logging.WithDebugLevel())
	case LoggingInfo, "":
		opts = append(opts, logging.WithLevel(logging.LevelInfo))
	case LoggingWarn:
		opts = append(opts, logging.WithLevel(logging.LevelWarn))
	case LoggingError:
		opts = append(opts, logging.WithLevel(logging.LevelError))
	default:
		return nil, fmt.Errorf("unknown level %q (valid: debug, info, warn, error)", c.Level)
	}

	return opts, nil
}

// ObservabilityConfig is the unified observability configuration.
// Embed this in your app config struct for seamless config loading.
//
// Example YAML:
//
//	observability:
//	  tracing:
//	    provider: otlp
//	    endpoint: localhost:4317
//	  metrics:
//	    provider: prometheus
//	  logging:
//	    handler: json
//	    level: info
//	  excludePaths:
//	    - /healthz
//	    - /readyz
//
// Example usage:
//
//	type AppConfig struct {
//	    Server        ServerConfig           `config:"server"`
//	    Observability app.ObservabilityConfig `config:"observability"`
//	}
//
//	app.New(
//	    app.WithServiceName("my-api"),
//	    app.WithObservabilityConfig(cfg.Observability),
//	)
type ObservabilityConfig struct {
	Tracing TracingConfig `config:"tracing" json:"tracing" yaml:"tracing"`
	Metrics MetricsConfig `config:"metrics" json:"metrics" yaml:"metrics"`
	Logging LoggingConfig `config:"logging" json:"logging" yaml:"logging"`

	ExcludePaths    []string `config:"excludePaths" json:"excludePaths" yaml:"excludePaths"`
	ExcludePrefixes []string `config:"excludePrefixes" json:"excludePrefixes" yaml:"excludePrefixes"`
}

// WithObservabilityFromConfig configures all observability from a single config struct.
// This is a convenience method that converts declarative configuration
// into functional options and applies them via the existing WithObservability function.
//
// This function is ideal for loading observability configuration from files (YAML, JSON, etc.).
//
// Example:
//
//	app.New(
//	    app.WithServiceName("blog-api"),
//	    app.WithObservabilityFromConfig(cfg.Observability),
//	)
func WithObservabilityFromConfig(cfg ObservabilityConfig) Option {
	return func(c *config) {
		var obsOpts []ObservabilityOption

		// Tracing
		if cfg.Tracing.Provider != "" {
			tracingOpts, err := cfg.Tracing.options()
			if err != nil {
				panic(fmt.Sprintf("tracing config error: %v", err))
			}
			obsOpts = append(obsOpts, WithTracing(tracingOpts...))
		}

		// Metrics
		if cfg.Metrics.Provider != "" {
			metricsOpts, err := cfg.Metrics.options()
			if err != nil {
				panic(fmt.Sprintf("metrics config error: %v", err))
			}
			obsOpts = append(obsOpts, WithMetrics(metricsOpts...))
		}

		// Logging
		if cfg.Logging.Handler != "" {
			loggingOpts, err := cfg.Logging.options()
			if err != nil {
				panic(fmt.Sprintf("logging config error: %v", err))
			}
			obsOpts = append(obsOpts, WithLogging(loggingOpts...))
		}

		// Path exclusions
		if len(cfg.ExcludePaths) > 0 {
			obsOpts = append(obsOpts, WithExcludePaths(cfg.ExcludePaths...))
		}
		if len(cfg.ExcludePrefixes) > 0 {
			obsOpts = append(obsOpts, WithExcludePrefixes(cfg.ExcludePrefixes...))
		}

		// Apply all observability options
		WithObservability(obsOpts...)(c)
	}
}
