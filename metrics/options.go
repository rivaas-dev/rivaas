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

package metrics

import (
	"log/slog"
	"strings"
	"time"

	"go.opentelemetry.io/otel/metric"
)

// Option defines functional options for Recorder configuration.
type Option func(*Recorder)

// WithMeterProvider allows you to provide a custom OpenTelemetry [metric.MeterProvider].
// When using this option, the package will NOT set the global otel.SetMeterProvider()
// by default. Use [WithGlobalMeterProvider] if you want global registration.
//
// This is useful when:
//   - You want to manage the meter provider lifecycle yourself
//   - You need multiple independent metrics configurations
//   - You want to avoid global state in your application
//
// Example:
//
//	mp := sdkmetric.NewMeterProvider(...)
//	recorder := metrics.New(
//	    metrics.WithMeterProvider(mp),
//	    metrics.WithServiceName("my-service"),
//	)
//	defer mp.Shutdown(context.Background())
//
// Note: When using WithMeterProvider, provider options ([WithPrometheus], [WithOTLP], etc.)
// are ignored since you're managing the provider yourself.
func WithMeterProvider(provider metric.MeterProvider) Option {
	return func(r *Recorder) {
		r.meterProvider = provider
		r.customMeterProvider = true
		// Note: registerGlobal stays false unless explicitly set
	}
}

// WithGlobalMeterProvider registers the meter provider as the global
// OpenTelemetry meter provider via otel.SetMeterProvider().
// By default, meter providers are not registered globally to allow multiple
// metrics configurations to coexist in the same process.
//
// Example:
//
//	recorder := metrics.New(
//	    metrics.WithPrometheus(":9090", "/metrics"),
//	    metrics.WithGlobalMeterProvider(), // Register as global default
//	)
func WithGlobalMeterProvider() Option {
	return func(r *Recorder) {
		r.registerGlobal = true
	}
}

// WithServiceName sets the service name for metrics.
func WithServiceName(name string) Option {
	return func(r *Recorder) {
		r.serviceName = name
	}
}

// WithServiceVersion sets the service version for metrics.
func WithServiceVersion(version string) Option {
	return func(r *Recorder) {
		r.serviceVersion = version
	}
}

// WithExportInterval sets the export interval for OTLP and stdout metrics.
func WithExportInterval(interval time.Duration) Option {
	return func(r *Recorder) {
		r.exportInterval = interval
	}
}

// WithDurationBuckets sets custom histogram bucket boundaries for request duration metrics.
// Buckets are specified in seconds. If not set, DefaultDurationBuckets is used.
//
// Example:
//
//	recorder := metrics.MustNew(
//	    metrics.WithDurationBuckets(0.01, 0.05, 0.1, 0.5, 1, 5), // in seconds
//	)
func WithDurationBuckets(buckets ...float64) Option {
	return func(r *Recorder) {
		r.durationBuckets = buckets
	}
}

// WithSizeBuckets sets custom histogram bucket boundaries for request/response size metrics.
// Buckets are specified in bytes. If not set, DefaultSizeBuckets is used.
//
// Example:
//
//	recorder := metrics.MustNew(
//	    metrics.WithSizeBuckets(1000, 10000, 100000, 1000000), // in bytes
//	)
func WithSizeBuckets(buckets ...float64) Option {
	return func(r *Recorder) {
		r.sizeBuckets = buckets
	}
}

// WithServerDisabled disables the automatic metrics server for Prometheus.
// Use this if you want to manually serve metrics via [Recorder.Handler].
func WithServerDisabled() Option {
	return func(r *Recorder) {
		r.autoStartServer = false
	}
}

// WithStrictPort requires the metrics server to use the exact port specified.
// If the port is unavailable, initialization will fail instead of finding an alternative port.
// This is useful when you need metrics on a specific port for monitoring integrations.
func WithStrictPort() Option {
	return func(r *Recorder) {
		r.strictPort = true
	}
}

// WithMaxCustomMetrics sets the maximum number of custom metrics allowed.
func WithMaxCustomMetrics(maxLimit int) Option {
	return func(r *Recorder) {
		r.maxCustomMetrics = maxLimit
	}
}

// WithEventHandler sets a custom [EventHandler] for internal operational events.
// Use this for advanced use cases like sending errors to Sentry, custom alerting,
// or integrating with non-slog logging systems.
//
// Example:
//
//	metrics.New(metrics.WithEventHandler(func(e metrics.Event) {
//	    if e.Type == metrics.EventError {
//	        sentry.CaptureMessage(e.Message)
//	    }
//	    myLogger.Log(e.Type, e.Message, e.Args...)
//	}))
func WithEventHandler(handler EventHandler) Option {
	return func(r *Recorder) {
		r.eventHandler = handler
	}
}

// WithLogger sets the logger for internal operational events using the default event handler.
// This is a convenience wrapper around [WithEventHandler] that logs events to the provided [slog.Logger].
//
// Example:
//
//	// Use stdlib slog
//	metrics.New(metrics.WithLogger(slog.Default()))
//
//	// Use custom slog logger
//	:= slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	metrics.New(metrics.WithLogger(logger))
func WithLogger(logger *slog.Logger) Option {
	return WithEventHandler(DefaultEventHandler(logger))
}

// WithPrometheus configures Prometheus provider with port and path.
// This is the recommended way to configure Prometheus metrics.
//
// Example:
//
//	recorder := metrics.MustNew(
//	    metrics.WithPrometheus(":9090", "/metrics"),
//	    metrics.WithServiceName("my-api"),
//	)
func WithPrometheus(port, path string) Option {
	return func(r *Recorder) {
		r.provider = PrometheusProvider
		r.providerSetCount++
		// Normalize and set port
		if port != "" && !strings.HasPrefix(port, ":") {
			port = ":" + port
		}
		r.metricsPort = port
		// Normalize and set path
		if path != "" && !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		r.metricsPath = path
	}
}

// WithOTLP configures OTLP HTTP provider with endpoint.
//
// Example:
//
//	recorder := metrics.MustNew(
//	    metrics.WithOTLP("http://localhost:4318"),
//	    metrics.WithServiceName("my-api"),
//	)
func WithOTLP(endpoint string) Option {
	return func(r *Recorder) {
		r.provider = OTLPProvider
		r.providerSetCount++
		r.otlpEndpoint = endpoint
	}
}

// WithStdout configures stdout provider for development/debugging.
//
// Example:
//
//	recorder := metrics.MustNew(
//	    metrics.WithStdout(),
//	    metrics.WithExportInterval(time.Second),
//	)
func WithStdout() Option {
	return func(r *Recorder) {
		r.provider = StdoutProvider
		r.providerSetCount++
	}
}
