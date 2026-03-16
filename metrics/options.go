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
// Options apply to an internal config struct; the constructor builds the Recorder from the validated config.
type Option func(*config)

// config holds construction-time metrics configuration.
type config struct {
	meterProvider       metric.MeterProvider
	serviceName         string
	serviceVersion      string
	exportInterval      time.Duration
	durationBuckets     []float64
	sizeBuckets         []float64
	autoStartServer     bool
	strictPort          bool
	maxCustomMetrics    int
	logger              *slog.Logger
	registerGlobal      bool
	withoutScopeInfo    bool
	withoutTargetInfo   bool
	provider            Provider
	providerSetCount    int
	metricsPort         string
	metricsPath         string
	otlpEndpoint        string
	customMeterProvider bool
	validationErrors    []error
}

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
	return func(c *config) {
		c.meterProvider = provider
		c.customMeterProvider = true
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
	return func(c *config) {
		c.registerGlobal = true
	}
}

// WithServiceName sets the service name for metrics.
func WithServiceName(name string) Option {
	return func(c *config) {
		c.serviceName = name
	}
}

// WithServiceVersion sets the service version for metrics.
func WithServiceVersion(version string) Option {
	return func(c *config) {
		c.serviceVersion = version
	}
}

// WithExportInterval sets the export interval for OTLP and stdout metrics.
func WithExportInterval(interval time.Duration) Option {
	return func(c *config) {
		c.exportInterval = interval
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
	return func(c *config) {
		c.durationBuckets = buckets
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
	return func(c *config) {
		c.sizeBuckets = buckets
	}
}

// WithServerDisabled disables the automatic metrics server for Prometheus.
// Use this if you want to manually serve metrics via [Recorder.Handler].
func WithServerDisabled() Option {
	return func(c *config) {
		c.autoStartServer = false
	}
}

// WithStrictPort requires the metrics server to use the exact port specified.
// If the port is unavailable, initialization will fail instead of finding an alternative port.
// This is useful when you need metrics on a specific port for monitoring integrations.
func WithStrictPort() Option {
	return func(c *config) {
		c.strictPort = true
	}
}

// WithMaxCustomMetrics sets the maximum number of custom metrics allowed.
func WithMaxCustomMetrics(maxLimit int) Option {
	return func(c *config) {
		c.maxCustomMetrics = maxLimit
	}
}

// WithLogger sets the logger for internal operational events (errors, warnings, info, debug).
// Internal events are logged at the appropriate slog level. If logger is nil or WithLogger is not called,
// a discard logger is used and no internal output is produced.
//
// Example:
//
//	metrics.New(metrics.WithLogger(slog.Default()))
//
//	// No internal logging:
//	metrics.New(metrics.WithPrometheus(":9090", "/metrics")) // omit WithLogger, or pass WithLogger(nil)
func WithLogger(logger *slog.Logger) Option {
	return func(c *config) {
		c.logger = logger
	}
}

// WithoutScopeInfo configures the Prometheus exporter to omit instrumentation scope labels
// (otel_scope_name, otel_scope_version, otel_scope_schema_url) from all metric points.
//
// By default, OpenTelemetry adds scope information to identify which instrumentation library
// produced each metric. This can be disabled to reduce label cardinality when you only have
// a single instrumentation scope or when the scope information is not useful.
//
// This option only affects the Prometheus provider. Other providers ignore it.
//
// Example:
//
//	recorder := metrics.MustNew(
//	    metrics.WithPrometheus(":9090", "/metrics"),
//	    metrics.WithoutScopeInfo(), // Remove otel_scope_* labels
//	)
func WithoutScopeInfo() Option {
	return func(c *config) {
		c.withoutScopeInfo = true
	}
}

// WithoutTargetInfo configures the Prometheus exporter to omit the target_info metric.
//
// By default, OpenTelemetry creates a target_info metric containing the resource attributes
// (service.name, service.version, etc.). This can be disabled if you manage service
// identification through other means (e.g., external labels in Prometheus configuration).
//
// This option only affects the Prometheus provider. Other providers ignore it.
//
// Example:
//
//	recorder := metrics.MustNew(
//	    metrics.WithPrometheus(":9090", "/metrics"),
//	    metrics.WithoutTargetInfo(), // Remove target_info metric
//	)
func WithoutTargetInfo() Option {
	return func(c *config) {
		c.withoutTargetInfo = true
	}
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
	return func(c *config) {
		c.provider = PrometheusProvider
		c.providerSetCount++
		if port != "" && !strings.HasPrefix(port, ":") {
			port = ":" + port
		}
		c.metricsPort = port
		if path != "" && !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		c.metricsPath = path
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
	return func(c *config) {
		c.provider = OTLPProvider
		c.providerSetCount++
		c.otlpEndpoint = endpoint
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
	return func(c *config) {
		c.provider = StdoutProvider
		c.providerSetCount++
	}
}
