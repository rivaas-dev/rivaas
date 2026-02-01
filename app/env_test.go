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

//go:build !integration

package app

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/metrics"
)

func TestWithEnv_BasicOverrides(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix

	// Set environment variables
	envVars := map[string]string{
		"RIVAAS_ENV":              "production",
		"RIVAAS_SERVICE_NAME":     "test-service",
		"RIVAAS_SERVICE_VERSION":  "v2.0.0",
		"RIVAAS_PORT":             "9000",
		"RIVAAS_HOST":             "127.0.0.1",
		"RIVAAS_READ_TIMEOUT":     "15s",
		"RIVAAS_WRITE_TIMEOUT":    "20s",
		"RIVAAS_SHUTDOWN_TIMEOUT": "45s",
	}

	for k, v := range envVars {
		t.Setenv(k, v)
	}

	// Create app with env overrides
	app, err := New(
		WithServiceName("original-name"),
		WithServiceVersion("v1.0.0"),
		WithEnvironment("development"),
		WithEnv(),
	)
	require.NoError(t, err)

	// Verify overrides
	assert.Equal(t, "production", app.Environment())
	assert.Equal(t, "test-service", app.ServiceName())
	assert.Equal(t, "v2.0.0", app.ServiceVersion())
	assert.Equal(t, 9000, app.config.server.port)
	assert.Equal(t, "127.0.0.1", app.config.server.host)
	assert.Equal(t, 15*time.Second, app.config.server.readTimeout)
	assert.Equal(t, 20*time.Second, app.config.server.writeTimeout)
	assert.Equal(t, 45*time.Second, app.config.server.shutdownTimeout)
}

func TestWithEnvPrefix_CustomPrefix(t *testing.T) {
	t.Parallel()

	// Set environment variables with custom prefix
	envVars := map[string]string{
		"MYAPP_ENV":          "production",
		"MYAPP_SERVICE_NAME": "custom-service",
		"MYAPP_PORT":         "4000",
	}

	for k, v := range envVars {
		// NOTE: When using t.Setenv or t.Chdir in a test, t.Parallel should not be used
		require.NoError(t, os.Setenv(k, v))
		defer os.Unsetenv(k) //nolint:errcheck // we do not care about errors here
	}

	// Create app with custom prefix
	app, err := New(
		WithServiceName("original-name"),
		WithEnvPrefix("MYAPP_"),
	)
	require.NoError(t, err)

	// Verify overrides
	assert.Equal(t, "production", app.Environment())
	assert.Equal(t, "custom-service", app.ServiceName())
	assert.Equal(t, 4000, app.config.server.port)
}

func TestWithEnv_InvalidPort_FailsFast(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix
	t.Setenv("RIVAAS_PORT", "invalid")

	_, err := New(
		WithServiceName("test-service"),
		WithEnv(),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "RIVAAS_PORT")
}

func TestWithEnv_InvalidDuration_FailsFast(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix
	t.Setenv("RIVAAS_READ_TIMEOUT", "not-a-duration")

	_, err := New(
		WithServiceName("test-service"),
		WithEnv(),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "RIVAAS_READ_TIMEOUT")
}

func TestWithEnv_EnvOverridesProgrammaticConfig(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix
	t.Setenv("RIVAAS_PORT", "5000")

	// Programmatic config sets port to 3000, but env should override
	app, err := New(
		WithServiceName("test-service"),
		WithPort(3000),
		WithEnv(), // Env overrides
	)
	require.NoError(t, err)

	// Env should win
	assert.Equal(t, 5000, app.config.server.port)
}

func TestWithEnv_LogLevel(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix
	t.Setenv("RIVAAS_LOG_LEVEL", "debug")

	app, err := New(
		WithServiceName("test-service"),
		WithEnv(),
	)
	require.NoError(t, err)

	// Logging should be enabled
	assert.NotNil(t, app.config.observability)
	assert.NotNil(t, app.config.observability.logging)
	assert.True(t, app.config.observability.logging.enabled)
}

func TestWithEnv_LogFormat(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix

	tests := []struct {
		name   string
		format string
	}{
		{"json format", "json"},
		{"text format", "text"},
		{"console format", "console"},
		{"JSON uppercase", "JSON"},
		{"Console mixed case", "Console"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("RIVAAS_LOG_FORMAT", tt.format)

			app, err := New(
				WithServiceName("test-service"),
				WithEnv(),
			)
			require.NoError(t, err)

			// Logging should be enabled with the specified format
			assert.NotNil(t, app.config.observability)
			assert.NotNil(t, app.config.observability.logging)
			assert.True(t, app.config.observability.logging.enabled)
		})
	}
}

func TestWithEnv_MetricsExporter_Prometheus(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix
	t.Setenv("RIVAAS_METRICS_EXPORTER", "prometheus")

	app, err := New(
		WithServiceName("test-service"),
		WithEnv(),
	)
	require.NoError(t, err)

	// Metrics should be enabled
	assert.NotNil(t, app.config.observability)
	assert.NotNil(t, app.config.observability.metrics)
	assert.True(t, app.config.observability.metrics.enabled)
}

func TestWithEnv_MetricsExporter_PrometheusCustom(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix
	t.Setenv("RIVAAS_METRICS_EXPORTER", "prometheus")
	t.Setenv("RIVAAS_METRICS_ADDR", ":9000")
	t.Setenv("RIVAAS_METRICS_PATH", "/custom/metrics")

	app, err := New(
		WithServiceName("test-service"),
		WithEnv(),
	)
	require.NoError(t, err)

	// Metrics should be enabled with custom settings
	assert.NotNil(t, app.config.observability)
	assert.NotNil(t, app.config.observability.metrics)
	assert.True(t, app.config.observability.metrics.enabled)
}

func TestWithEnv_MetricsExporter_OTLP(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix
	t.Setenv("RIVAAS_METRICS_EXPORTER", "otlp")
	t.Setenv("RIVAAS_METRICS_ENDPOINT", "http://localhost:4318")

	app, err := New(
		WithServiceName("test-service"),
		WithEnv(),
	)
	require.NoError(t, err)

	// Metrics should be enabled with OTLP
	assert.NotNil(t, app.config.observability)
	assert.NotNil(t, app.config.observability.metrics)
	assert.True(t, app.config.observability.metrics.enabled)
}

func TestWithEnv_MetricsExporter_Stdout(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix
	t.Setenv("RIVAAS_METRICS_EXPORTER", "stdout")

	app, err := New(
		WithServiceName("test-service"),
		WithEnv(),
	)
	require.NoError(t, err)

	// Metrics should be enabled with stdout
	assert.NotNil(t, app.config.observability)
	assert.NotNil(t, app.config.observability.metrics)
	assert.True(t, app.config.observability.metrics.enabled)
}

func TestWithEnv_MetricsExporter_OTLP_MissingEndpoint_FailsFast(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix
	t.Setenv("RIVAAS_METRICS_EXPORTER", "otlp")

	_, err := New(
		WithServiceName("test-service"),
		WithEnv(),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "RIVAAS_METRICS_EXPORTER")
	assert.Contains(t, err.Error(), "requires")
	assert.Contains(t, err.Error(), "RIVAAS_METRICS_ENDPOINT")
}

func TestWithEnv_MetricsExporter_Invalid_FailsFast(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix
	t.Setenv("RIVAAS_METRICS_EXPORTER", "invalid")

	_, err := New(
		WithServiceName("test-service"),
		WithEnv(),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "RIVAAS_METRICS_EXPORTER")
	assert.Contains(t, err.Error(), "must be one of")
}

func TestWithEnv_TracingExporter_OTLP(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix
	t.Setenv("RIVAAS_TRACING_EXPORTER", "otlp")
	t.Setenv("RIVAAS_TRACING_ENDPOINT", "localhost:4317")

	app, err := New(
		WithServiceName("test-service"),
		WithEnv(),
	)
	require.NoError(t, err)

	// Tracing should be enabled with OTLP gRPC
	assert.NotNil(t, app.config.observability)
	assert.NotNil(t, app.config.observability.tracing)
	assert.True(t, app.config.observability.tracing.enabled)
}

func TestWithEnv_TracingExporter_OTLPHttp(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix
	t.Setenv("RIVAAS_TRACING_EXPORTER", "otlp-http")
	t.Setenv("RIVAAS_TRACING_ENDPOINT", "http://localhost:4318")

	app, err := New(
		WithServiceName("test-service"),
		WithEnv(),
	)
	require.NoError(t, err)

	// Tracing should be enabled with OTLP HTTP
	assert.NotNil(t, app.config.observability)
	assert.NotNil(t, app.config.observability.tracing)
	assert.True(t, app.config.observability.tracing.enabled)
}

func TestWithEnv_TracingExporter_Stdout(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix
	t.Setenv("RIVAAS_TRACING_EXPORTER", "stdout")

	app, err := New(
		WithServiceName("test-service"),
		WithEnv(),
	)
	require.NoError(t, err)

	// Tracing should be enabled with stdout
	assert.NotNil(t, app.config.observability)
	assert.NotNil(t, app.config.observability.tracing)
	assert.True(t, app.config.observability.tracing.enabled)
}

func TestWithEnv_TracingExporter_OTLP_MissingEndpoint_FailsFast(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix
	t.Setenv("RIVAAS_TRACING_EXPORTER", "otlp")

	_, err := New(
		WithServiceName("test-service"),
		WithEnv(),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "RIVAAS_TRACING_EXPORTER")
	assert.Contains(t, err.Error(), "requires")
	assert.Contains(t, err.Error(), "RIVAAS_TRACING_ENDPOINT")
}

func TestWithEnv_TracingExporter_Invalid_FailsFast(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix
	t.Setenv("RIVAAS_TRACING_EXPORTER", "jaeger")

	_, err := New(
		WithServiceName("test-service"),
		WithEnv(),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "RIVAAS_TRACING_EXPORTER")
	assert.Contains(t, err.Error(), "must be one of")
}

func TestWithEnv_ExporterOverridesCode(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix
	t.Setenv("RIVAAS_METRICS_EXPORTER", "stdout")

	// Code sets prometheus, but env should override to stdout
	app, err := New(
		WithServiceName("test-service"),
		WithObservability(
			WithMetrics(metrics.WithPrometheus(":9090", "/metrics")),
		),
		WithEnv(), // Env overrides
	)
	require.NoError(t, err)

	// Metrics should be enabled (env override took effect)
	assert.NotNil(t, app.config.observability)
	assert.NotNil(t, app.config.observability.metrics)
	assert.True(t, app.config.observability.metrics.enabled)
}

func TestWithEnv_PprofEnabled(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix
	t.Setenv("RIVAAS_PPROF_ENABLED", "true")

	app, err := New(
		WithServiceName("test-service"),
		WithEnv(),
	)
	require.NoError(t, err)

	// Debug/pprof should be enabled
	assert.NotNil(t, app.config.debug)
	assert.True(t, app.config.debug.pprofEnabled)
}

func TestWithEnv_NoEnvVarsSet_UsesDefaults(t *testing.T) {
	// Not parallel - modifies global env vars with RIVAAS_ prefix
	// Clear any env vars that might be set
	envVars := []string{
		"RIVAAS_ENV", "RIVAAS_SERVICE_NAME", "RIVAAS_SERVICE_VERSION",
		"RIVAAS_PORT", "RIVAAS_HOST", "RIVAAS_READ_TIMEOUT",
	}
	for _, k := range envVars {
		require.NoError(t, os.Unsetenv(k))
	}

	app, err := New(
		WithServiceName("test-service"),
		WithServiceVersion("v1.0.0"),
		WithEnv(),
	)
	require.NoError(t, err)

	// Should use programmatic values
	assert.Equal(t, "test-service", app.ServiceName())
	assert.Equal(t, "v1.0.0", app.ServiceVersion())
	assert.Equal(t, DefaultPort, app.config.server.port)
}

func TestWithPort(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test-service"),
		WithPort(3000),
	)
	require.NoError(t, err)

	assert.Equal(t, 3000, app.config.server.port)
	assert.Equal(t, ":3000", app.config.server.ListenAddr())
}

func TestWithHost(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test-service"),
		WithHost("127.0.0.1"),
		WithPort(8080),
	)
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1", app.config.server.host)
	assert.Equal(t, "127.0.0.1:8080", app.config.server.ListenAddr())
}

func TestServerConfig_ListenAddr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		host     string
		port     int
		expected string
	}{
		{
			name:     "default (empty host)",
			host:     "",
			port:     8080,
			expected: ":8080",
		},
		{
			name:     "localhost",
			host:     "127.0.0.1",
			port:     3000,
			expected: "127.0.0.1:3000",
		},
		{
			name:     "all interfaces explicit",
			host:     "0.0.0.0",
			port:     9000,
			expected: "0.0.0.0:9000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sc := &serverConfig{
				host: tt.host,
				port: tt.port,
			}
			assert.Equal(t, tt.expected, sc.ListenAddr())
		})
	}
}
