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
	"regexp"
	"time"

	"rivaas.dev/logging"
	"rivaas.dev/metrics"
	"rivaas.dev/tracing"

	stderrors "errors"
)

// ObservabilityOption configures unified observability settings.
// These options configure metrics, tracing, logging, and shared settings like path exclusions.
type ObservabilityOption func(*observabilitySettings)

// loggingConfig holds logging configuration settings.
type loggingConfig struct {
	enabled bool
	options []logging.Option
}

// observabilitySettings holds unified observability configuration.
type observabilitySettings struct {
	// Component configurations
	metrics *metricsConfig
	tracing *tracingConfig
	logging *loggingConfig

	// Metrics server configuration (mutually exclusive)
	metricsOnMainRouter   bool   // If true, mount metrics on main router
	metricsMainRouterPath string // Path on main router (default: /metrics)
	metricsSeparateServer bool   // If true, use custom separate server config
	metricsSeparateAddr   string // Address for separate server (e.g., ":9091")
	metricsSeparatePath   string // Path on separate server (default: /metrics)

	// Shared settings
	pathFilter    *pathFilter
	accessLogging bool
	logErrorsOnly bool
	slowThreshold time.Duration

	// Validation errors collected during option application
	validationErrors []error
}

// defaultObservabilitySettings creates observability settings with sensible defaults.
func defaultObservabilitySettings() *observabilitySettings {
	return &observabilitySettings{
		pathFilter:    newPathFilterWithDefaults(),
		accessLogging: true,
		slowThreshold: time.Second,
	}
}

// WithMetrics enables metrics collection with the given options.
// Service name and version are automatically injected from app-level configuration.
//
// Example:
//
//	app.MustNew(
//	    app.WithServiceName("orders-api"),
//	    app.WithObservability(
//	        app.WithMetrics(), // Prometheus is default
//	    ),
//	)
func WithMetrics(opts ...metrics.Option) ObservabilityOption {
	return func(s *observabilitySettings) {
		s.metrics = &metricsConfig{
			enabled: true,
			options: opts,
		}
	}
}

// WithTracing enables distributed tracing with the given options.
// Service name and version are automatically injected from app-level configuration.
//
// Example:
//
//	app.MustNew(
//	    app.WithServiceName("orders-api"),
//	    app.WithObservability(
//	        app.WithTracing(tracing.WithOTLP("localhost:4317")),
//	    ),
//	)
func WithTracing(opts ...tracing.Option) ObservabilityOption {
	return func(s *observabilitySettings) {
		s.tracing = &tracingConfig{
			enabled: true,
			options: opts,
		}
	}
}

// WithLogging enables structured logging with the given options.
// Service name and version are automatically injected from app-level configuration.
//
// If not provided, a no-op logger is used (logs are discarded).
//
// The app automatically derives request-scoped loggers that include:
//   - HTTP metadata (method, route, target path, client IP)
//   - Request ID (if X-Request-ID header is present)
//   - Trace/span IDs (if OpenTelemetry tracing is enabled)
//
// Example:
//
//	app.MustNew(
//	    app.WithServiceName("orders-api"),
//	    app.WithServiceVersion("v1.0.0"),
//	    app.WithObservability(
//	        app.WithLogging(
//	            logging.WithJSONHandler(),
//	            logging.WithDebugLevel(),
//	            // Service name/version auto-injected from app config
//	        ),
//	    ),
//	)
func WithLogging(opts ...logging.Option) ObservabilityOption {
	return func(s *observabilitySettings) {
		s.logging = &loggingConfig{
			enabled: true,
			options: opts,
		}
	}
}

// WithMetricsOnMainRouter mounts the metrics endpoint on the main application router
// instead of running a separate metrics server.
//
// By default, the metrics package runs a separate server on :9090.
// Use this option when you need metrics on the same port as the application,
// such as in Kubernetes environments with strict ingress rules.
//
// The separate metrics server is automatically disabled when this option is used.
//
// Example:
//
//	app.MustNew(
//	    app.WithServiceName("orders-api"),
//	    app.WithObservability(
//	        app.WithMetrics(), // Prometheus is default
//	        app.WithMetricsOnMainRouter("/metrics"),
//	    ),
//	)
func WithMetricsOnMainRouter(path string) ObservabilityOption {
	return func(s *observabilitySettings) {
		// Check for conflict
		if s.metricsSeparateServer {
			s.validationErrors = append(s.validationErrors,
				stderrors.New("WithMetricsOnMainRouter and WithMetricsSeparateServer are mutually exclusive; use only one to configure where metrics are served"))
		}
		s.metricsOnMainRouter = true
		if path == "" {
			path = "/metrics"
		}
		s.metricsMainRouterPath = path
	}
}

// WithMetricsSeparateServer configures the separate metrics server with custom address and path.
//
// By default, metrics run on a separate server at :9090/metrics.
// Use this option to customize the port or endpoint path.
//
// This is mutually exclusive with WithMetricsOnMainRouter.
//
// Example:
//
//	app.MustNew(
//	    app.WithServiceName("orders-api"),
//	    app.WithObservability(
//	        app.WithMetrics(), // Prometheus is default
//	        app.WithMetricsSeparateServer(":9091", "/custom-metrics"),
//	    ),
//	)
func WithMetricsSeparateServer(addr, path string) ObservabilityOption {
	return func(s *observabilitySettings) {
		// Check for conflict
		if s.metricsOnMainRouter {
			s.validationErrors = append(s.validationErrors,
				stderrors.New("WithMetricsOnMainRouter and WithMetricsSeparateServer are mutually exclusive; use only one to configure where metrics are served"))
		}
		s.metricsSeparateServer = true
		s.metricsSeparateAddr = addr
		if path == "" {
			path = "/metrics"
		}
		s.metricsSeparatePath = path
	}
}

// WithoutDefaultExclusions clears the default path exclusions.
// By default, common health/probe paths are excluded (/health, /healthz, /ready, etc.).
// Use this option to start with an empty exclusion list, then add your own paths.
//
// Example:
//
//	app.WithObservability(
//	    app.WithoutDefaultExclusions(),
//	    app.WithExcludePaths("/only-this", "/and-that"),
//	)
func WithoutDefaultExclusions() ObservabilityOption {
	return func(s *observabilitySettings) {
		s.pathFilter = newPathFilter()
	}
}

// WithExcludePaths adds exact paths to exclude from ALL observability
// (metrics, tracing, access logging).
// Multiple calls accumulate paths. Default exclusions are preserved unless
// WithoutDefaultExclusions() is called first.
//
// Example:
//
//	app.WithObservability(
//	    app.WithExcludePaths("/custom-health", "/k8s-probe"),
//	)
func WithExcludePaths(paths ...string) ObservabilityOption {
	return func(s *observabilitySettings) {
		if s.pathFilter == nil {
			s.pathFilter = newPathFilterWithDefaults()
		}
		s.pathFilter.addPaths(paths...)
	}
}

// WithExcludePrefixes adds path prefixes to exclude from ALL observability.
// Paths starting with any of these prefixes will be excluded.
//
// Example:
//
//	app.WithObservability(
//	    app.WithExcludePrefixes("/internal/", "/admin/", "/debug/"),
//	)
func WithExcludePrefixes(prefixes ...string) ObservabilityOption {
	return func(s *observabilitySettings) {
		if s.pathFilter == nil {
			s.pathFilter = newPathFilterWithDefaults()
		}
		s.pathFilter.addPrefixes(prefixes...)
	}
}

// WithExcludePatterns adds regex patterns to exclude from ALL observability.
// Paths matching any pattern will be excluded.
//
// Example:
//
//	app.WithObservability(
//	    app.WithExcludePatterns(`^/v[0-9]+/internal/.*`, `^/debug/.*`),
//	)
func WithExcludePatterns(patterns ...string) ObservabilityOption {
	return func(s *observabilitySettings) {
		if s.pathFilter == nil {
			s.pathFilter = newPathFilterWithDefaults()
		}
		for _, pattern := range patterns {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				if s.validationErrors == nil {
					s.validationErrors = make([]error, 0, 1)
				}
				s.validationErrors = append(s.validationErrors,
					fmt.Errorf("invalid regex pattern for path exclusion %q: %w", pattern, err))

				continue
			}
			s.pathFilter.addPatterns(compiled)
		}
	}
}

// WithAccessLogging enables or disables access logging.
// Default is true.
//
// Example:
//
//	app.WithObservability(
//	    app.WithAccessLogging(false), // Disable access logs
//	)
func WithAccessLogging(enabled bool) ObservabilityOption {
	return func(s *observabilitySettings) {
		s.accessLogging = enabled
	}
}

// WithLogOnlyErrors logs only errors (status >= 400) and slow requests.
// Normal successful requests are not logged.
//
// Example:
//
//	app.WithObservability(
//	    app.WithLogOnlyErrors(),
//	)
func WithLogOnlyErrors() ObservabilityOption {
	return func(s *observabilitySettings) {
		s.logErrorsOnly = true
	}
}

// WithSlowThreshold sets the duration threshold for marking requests as "slow".
// Slow requests are always logged, even when using WithLogOnlyErrors.
// Default is 1 second.
//
// Example:
//
//	app.WithObservability(
//	    app.WithSlowThreshold(500 * time.Millisecond),
//	)
func WithSlowThreshold(d time.Duration) ObservabilityOption {
	return func(s *observabilitySettings) {
		s.slowThreshold = d
	}
}
