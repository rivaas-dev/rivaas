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
	"os"
	"strconv"
	"strings"
	"time"

	"rivaas.dev/logging"
	"rivaas.dev/metrics"
	"rivaas.dev/tracing"
)

// EnvPrefix is the environment variable prefix for Rivaas framework settings.
const EnvPrefix = "RIVAAS_"

// Environment variable names for framework configuration.
// These are used when [WithEnv] or [WithEnvPrefix] is called.
const (
	// Core application settings
	EnvMode           = "ENV"             // Environment mode: "development" or "production"
	EnvServiceName    = "SERVICE_NAME"    // Service name for observability
	EnvServiceVersion = "SERVICE_VERSION" // Service version

	// Server settings
	EnvPort            = "PORT"             // HTTP server port (e.g., "8080")
	EnvHost            = "HOST"             // HTTP server host/interface (e.g., "127.0.0.1")
	EnvReadTimeout     = "READ_TIMEOUT"     // Request read timeout (e.g., "10s")
	EnvWriteTimeout    = "WRITE_TIMEOUT"    // Response write timeout (e.g., "10s")
	EnvShutdownTimeout = "SHUTDOWN_TIMEOUT" // Graceful shutdown timeout (e.g., "30s")

	// Logging settings
	EnvLogLevel  = "LOG_LEVEL"  // Log level: "debug", "info", "warn", "error"
	EnvLogFormat = "LOG_FORMAT" // Log format: "json", "text", or "console"

	// Observability settings
	EnvMetricsExporter = "METRICS_EXPORTER" // Metrics exporter: "prometheus", "otlp", or "stdout"
	EnvMetricsAddr     = "METRICS_ADDR"     // Prometheus address (e.g., ":9090")
	EnvMetricsPath     = "METRICS_PATH"     // Prometheus path (e.g., "/metrics")
	EnvMetricsEndpoint = "METRICS_ENDPOINT" // OTLP endpoint (e.g., "http://localhost:4318")

	EnvTracingExporter = "TRACING_EXPORTER" // Tracing exporter: "otlp", "otlp-http", or "stdout"
	EnvTracingEndpoint = "TRACING_ENDPOINT" // OTLP endpoint (e.g., "localhost:4317")

	// Debug settings
	EnvPprofEnabled = "PPROF_ENABLED" // Enable pprof: "true" or "false"
)

// envConfig holds parsed environment variable values and any errors encountered.
type envConfig struct {
	errors []error
}

// addError records a parsing error for later reporting.
func (e *envConfig) addError(envVar string, err error) {
	e.errors = append(e.errors, fmt.Errorf("invalid environment variable %s: %w", envVar, err))
}

// WithEnv enables environment variable overrides for framework configuration.
// Environment variables use the RIVAAS_ prefix and take precedence over
// programmatic configuration.
//
// Supported variables:
//
//	Core:
//	  RIVAAS_ENV                    - Environment mode: "development" or "production"
//	  RIVAAS_SERVICE_NAME           - Service name for observability
//	  RIVAAS_SERVICE_VERSION        - Service version
//
//	Server:
//	  RIVAAS_PORT                   - HTTP server port (e.g., "8080")
//	  RIVAAS_HOST                   - HTTP server host/interface (e.g., "127.0.0.1")
//	  RIVAAS_READ_TIMEOUT           - Request read timeout (e.g., "10s")
//	  RIVAAS_WRITE_TIMEOUT          - Response write timeout (e.g., "10s")
//	  RIVAAS_SHUTDOWN_TIMEOUT       - Graceful shutdown timeout (e.g., "30s")
//
//	Logging:
//	  RIVAAS_LOG_LEVEL              - Log level: "debug", "info", "warn", "error"
//	  RIVAAS_LOG_FORMAT             - Log format: "json", "text", or "console"
//
//	Observability:
//	  RIVAAS_METRICS_EXPORTER       - Metrics exporter: "prometheus", "otlp", or "stdout"
//	  RIVAAS_METRICS_ADDR           - Prometheus address (default: ":9090")
//	  RIVAAS_METRICS_PATH           - Prometheus path (default: "/metrics")
//	  RIVAAS_METRICS_ENDPOINT       - OTLP metrics endpoint (e.g., "http://localhost:4318")
//	  RIVAAS_TRACING_EXPORTER       - Tracing exporter: "otlp", "otlp-http", or "stdout"
//	  RIVAAS_TRACING_ENDPOINT       - OTLP tracing endpoint (e.g., "localhost:4317")
//
//	Debug:
//	  RIVAAS_PPROF_ENABLED          - Enable pprof: "true" or "false"
//
// Example:
//
//	export RIVAAS_ENV=production
//	export RIVAAS_PORT=3000
//	export RIVAAS_LOG_LEVEL=warn
//
//	app := app.MustNew(
//	    app.WithServiceName("orders-api"),
//	    app.WithEnv(),  // Applies environment overrides
//	)
func WithEnv() Option {
	return WithEnvPrefix(EnvPrefix)
}

// WithEnvPrefix enables environment variable overrides with a custom prefix.
// Use this when deploying multiple Rivaas services that need different configurations.
//
// The prefix is prepended to the standard variable names. For example, with prefix "ORDERS_":
//   - ORDERS_ENV instead of RIVAAS_ENV
//   - ORDERS_PORT instead of RIVAAS_PORT
//
// Example:
//
//	// Service 1: uses ORDERS_ENV, ORDERS_PORT, etc.
//	app.MustNew(
//	    app.WithServiceName("orders-api"),
//	    app.WithEnvPrefix("ORDERS_"),
//	)
//
//	// Service 2: uses PAYMENTS_ENV, PAYMENTS_PORT, etc.
//	app.MustNew(
//	    app.WithServiceName("payments-api"),
//	    app.WithEnvPrefix("PAYMENTS_"),
//	)
func WithEnvPrefix(prefix string) Option {
	return func(c *config) {
		env := &envConfig{}
		applyEnvOverrides(c, prefix, env)

		// Collect errors for validation phase
		if len(env.errors) > 0 {
			if c.envErrors == nil {
				c.envErrors = make([]error, 0, len(env.errors))
			}
			c.envErrors = append(c.envErrors, env.errors...)
		}
	}
}

// applyEnvOverrides applies environment variable values to the configuration.
func applyEnvOverrides(c *config, prefix string, env *envConfig) {
	// Core settings
	applyEnvString(prefix, EnvMode, &c.environment)
	applyEnvString(prefix, EnvServiceName, &c.serviceName)
	applyEnvString(prefix, EnvServiceVersion, &c.serviceVersion)

	// Server settings
	applyEnvInt(prefix, EnvPort, &c.server.port, env)
	applyEnvString(prefix, EnvHost, &c.server.host)
	applyEnvDuration(prefix, EnvReadTimeout, &c.server.readTimeout, env)
	applyEnvDuration(prefix, EnvWriteTimeout, &c.server.writeTimeout, env)
	applyEnvDuration(prefix, EnvShutdownTimeout, &c.server.shutdownTimeout, env)

	// Logging settings
	applyEnvLogging(c, prefix, env)

	// Observability settings
	applyEnvObservability(c, prefix, env)

	// Debug settings
	applyEnvDebug(c, prefix, env)
}

// applyEnvString sets a string value from environment if present.
func applyEnvString(prefix, key string, target *string) {
	if v := os.Getenv(prefix + key); v != "" {
		*target = v
	}
}

// applyEnvInt sets an int value from environment if present.
func applyEnvInt(prefix, key string, target *int, env *envConfig) {
	fullKey := prefix + key
	if v := os.Getenv(fullKey); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			env.addError(fullKey, err)
			return
		}
		*target = parsed
	}
}

// applyEnvDuration sets a duration value from environment if present.
func applyEnvDuration(prefix, key string, target *time.Duration, env *envConfig) {
	fullKey := prefix + key
	if v := os.Getenv(fullKey); v != "" {
		parsed, err := time.ParseDuration(v)
		if err != nil {
			env.addError(fullKey, err)
			return
		}
		*target = parsed
	}
}

// applyEnvBool parses a boolean value from environment.
func applyEnvBool(prefix, key string) (value, isSet bool) {
	fullKey := prefix + key
	v := os.Getenv(fullKey)
	if v == "" {
		return false, false
	}
	v = strings.ToLower(v)
	return v == "true" || v == "1" || v == "yes", true
}

// applyEnvLogging configures logging from environment variables.
func applyEnvLogging(c *config, prefix string, _ *envConfig) {
	level := os.Getenv(prefix + EnvLogLevel)
	format := os.Getenv(prefix + EnvLogFormat)

	if level == "" && format == "" {
		return
	}

	// Ensure observability settings exist
	if c.observability == nil {
		c.observability = defaultObservabilitySettings()
	}
	if c.observability.logging == nil {
		c.observability.logging = &loggingConfig{enabled: true}
	} else {
		c.observability.logging.enabled = true
	}

	// Apply logging options
	if level != "" {
		var logLevel logging.Level
		switch strings.ToLower(level) {
		case "debug":
			logLevel = logging.LevelDebug
		case "info":
			logLevel = logging.LevelInfo
		case "warn", "warning":
			logLevel = logging.LevelWarn
		case "error":
			logLevel = logging.LevelError
		default:
			logLevel = logging.LevelInfo
		}
		c.observability.logging.options = append(c.observability.logging.options, logging.WithLevel(logLevel))
	}

	if format != "" {
		switch strings.ToLower(format) {
		case "json":
			c.observability.logging.options = append(c.observability.logging.options, logging.WithJSONHandler())
		case "text":
			c.observability.logging.options = append(c.observability.logging.options, logging.WithTextHandler())
		case "console":
			c.observability.logging.options = append(c.observability.logging.options, logging.WithConsoleHandler())
		}
	}
}

// applyEnvObservability configures metrics and tracing from environment variables.
func applyEnvObservability(c *config, prefix string, env *envConfig) {
	metricsExporter := os.Getenv(prefix + EnvMetricsExporter)
	tracingExporter := os.Getenv(prefix + EnvTracingExporter)

	// No observability env vars set - nothing to do
	if metricsExporter == "" && tracingExporter == "" {
		return
	}

	// Ensure observability settings exist
	if c.observability == nil {
		c.observability = defaultObservabilitySettings()
	}

	// Configure metrics exporter
	if metricsExporter != "" {
		if err := applyMetricsExporter(c, prefix, metricsExporter, env); err != nil {
			env.addError(prefix+EnvMetricsExporter, err)
		}
	}

	// Configure tracing exporter
	if tracingExporter != "" {
		if err := applyTracingExporter(c, prefix, tracingExporter, env); err != nil {
			env.addError(prefix+EnvTracingExporter, err)
		}
	}
}

// applyMetricsExporter configures metrics based on the exporter type.
func applyMetricsExporter(c *config, prefix, exporter string, env *envConfig) error {
	exporterLower := strings.ToLower(exporter)

	// Create new metrics config (env overrides code)
	c.observability.metrics = &metricsConfig{
		enabled: true,
		options: []metrics.Option{},
	}

	switch exporterLower {
	case "prometheus":
		addr := os.Getenv(prefix + EnvMetricsAddr)
		if addr == "" {
			addr = ":9090" // Default Prometheus address
		}

		path := os.Getenv(prefix + EnvMetricsPath)
		if path == "" {
			path = "/metrics" // Default Prometheus path
		}

		c.observability.metrics.options = append(
			c.observability.metrics.options,
			metrics.WithPrometheus(addr, path),
		)

	case "otlp":
		endpoint := os.Getenv(prefix + EnvMetricsEndpoint)
		if endpoint == "" {
			return fmt.Errorf("requires %s%s to be set", prefix, EnvMetricsEndpoint)
		}

		c.observability.metrics.options = append(
			c.observability.metrics.options,
			metrics.WithOTLP(endpoint),
		)

	case "stdout":
		c.observability.metrics.options = append(
			c.observability.metrics.options,
			metrics.WithStdout(),
		)

	default:
		return fmt.Errorf("must be one of: prometheus, otlp, stdout (got: %s)", exporter)
	}

	return nil
}

// applyTracingExporter configures tracing based on the exporter type.
func applyTracingExporter(c *config, prefix, exporter string, env *envConfig) error {
	exporterLower := strings.ToLower(exporter)

	// Create new tracing config (env overrides code)
	c.observability.tracing = &tracingConfig{
		enabled: true,
		options: []tracing.Option{},
	}

	switch exporterLower {
	case "otlp":
		endpoint := os.Getenv(prefix + EnvTracingEndpoint)
		if endpoint == "" {
			return fmt.Errorf("requires %s%s to be set", prefix, EnvTracingEndpoint)
		}

		c.observability.tracing.options = append(
			c.observability.tracing.options,
			tracing.WithOTLP(endpoint),
		)

	case "otlp-http":
		endpoint := os.Getenv(prefix + EnvTracingEndpoint)
		if endpoint == "" {
			return fmt.Errorf("requires %s%s to be set", prefix, EnvTracingEndpoint)
		}

		c.observability.tracing.options = append(
			c.observability.tracing.options,
			tracing.WithOTLPHTTP(endpoint),
		)

	case "stdout":
		c.observability.tracing.options = append(
			c.observability.tracing.options,
			tracing.WithStdout(),
		)

	default:
		return fmt.Errorf("must be one of: otlp, otlp-http, stdout (got: %s)", exporter)
	}

	return nil
}

// applyEnvDebug configures debug endpoints from environment variables.
func applyEnvDebug(c *config, prefix string, _ *envConfig) {
	pprofEnabled, isSet := applyEnvBool(prefix, EnvPprofEnabled)
	if !isSet {
		return
	}

	if pprofEnabled {
		if c.debug == nil {
			c.debug = defaultDebugSettings()
		}
		c.debug.pprofEnabled = true
	}
}
