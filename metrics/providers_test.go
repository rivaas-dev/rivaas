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
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProviderInitialization tests provider initialization edge cases
func TestProviderInitialization(t *testing.T) {
	t.Run("PrometheusProviderSuccess", func(t *testing.T) {
		config, err := New(
			WithServiceName("test-service"),
			WithProvider(PrometheusProvider),
			WithPort(":19301"),
			WithServerDisabled(),
		)
		require.NoError(t, err)
		require.NotNil(t, config)
		assert.Equal(t, PrometheusProvider, config.GetProvider())

		// Verify handler is available
		handler, err := config.GetHandler()
		require.NoError(t, err)
		require.NotNil(t, handler)

		// Cleanup
		err = config.Shutdown(context.Background())
		assert.NoError(t, err)
	})

	t.Run("OTLPProviderWithEndpoint", func(t *testing.T) {
		config, err := New(
			WithServiceName("test-service"),
			WithProvider(OTLPProvider),
			WithOTLPEndpoint("http://localhost:4318"),
		)
		require.NoError(t, err)
		require.NotNil(t, config)
		assert.Equal(t, OTLPProvider, config.GetProvider())

		// Cleanup (may error if collector not running)
		_ = config.Shutdown(context.Background())
	})

	t.Run("OTLPProviderWithHTTPS", func(t *testing.T) {
		config, err := New(
			WithServiceName("test-service"),
			WithProvider(OTLPProvider),
			WithOTLPEndpoint("https://collector.example.com:4318"),
		)
		require.NoError(t, err)
		require.NotNil(t, config)

		_ = config.Shutdown(context.Background())
	})

	t.Run("OTLPProviderWithPath", func(t *testing.T) {
		config, err := New(
			WithServiceName("test-service"),
			WithProvider(OTLPProvider),
			WithOTLPEndpoint("http://localhost:4318/v1/metrics"),
		)
		require.NoError(t, err)
		require.NotNil(t, config)

		_ = config.Shutdown(context.Background())
	})

	t.Run("OTLPProviderDefaultEndpoint", func(t *testing.T) {
		config, err := New(
			WithServiceName("test-service"),
			WithProvider(OTLPProvider),
			// No endpoint specified, should use default
		)
		require.NoError(t, err)
		require.NotNil(t, config)
		assert.Equal(t, OTLPProvider, config.GetProvider())

		_ = config.Shutdown(context.Background())
	})

	t.Run("StdoutProvider", func(t *testing.T) {
		config, err := New(
			WithServiceName("test-service"),
			WithProvider(StdoutProvider),
		)
		require.NoError(t, err)
		require.NotNil(t, config)
		assert.Equal(t, StdoutProvider, config.GetProvider())

		// Test that metrics can be recorded
		ctx := context.Background()
		config.IncrementCounter(ctx, "test_counter")
		config.RecordMetric(ctx, "test_histogram", 1.5)
		config.SetGauge(ctx, "test_gauge", 42.0)

		// Cleanup
		err = config.Shutdown(context.Background())
		assert.NoError(t, err)
	})

	t.Run("StdoutProviderWithCustomInterval", func(t *testing.T) {
		config, err := New(
			WithServiceName("test-service"),
			WithProvider(StdoutProvider),
			WithExportInterval(5*time.Second),
		)
		require.NoError(t, err)
		require.NotNil(t, config)

		err = config.Shutdown(context.Background())
		assert.NoError(t, err)
	})
}

// TestEnvironmentVariables tests environment variable configuration
func TestEnvironmentVariables(t *testing.T) {
	t.Run("OTEL_METRICS_EXPORTER_Prometheus", func(t *testing.T) {
		os.Setenv("OTEL_METRICS_EXPORTER", "prometheus")
		defer os.Unsetenv("OTEL_METRICS_EXPORTER")

		config, err := New(
			WithServiceName("test-service"),
			WithPort(":19302"),
			WithServerDisabled(),
		)
		require.NoError(t, err)
		assert.Equal(t, PrometheusProvider, config.GetProvider())

		config.Shutdown(context.Background())
	})

	t.Run("OTEL_METRICS_EXPORTER_OTLP", func(t *testing.T) {
		os.Setenv("OTEL_METRICS_EXPORTER", "otlp")
		defer os.Unsetenv("OTEL_METRICS_EXPORTER")

		config, err := New(
			WithServiceName("test-service"),
		)
		require.NoError(t, err)
		assert.Equal(t, OTLPProvider, config.GetProvider())

		config.Shutdown(context.Background())
	})

	t.Run("OTEL_SERVICE_NAME", func(t *testing.T) {
		os.Setenv("OTEL_SERVICE_NAME", "env-service")
		defer os.Unsetenv("OTEL_SERVICE_NAME")

		config, err := New(
			WithProvider(PrometheusProvider),
			WithPort(":19303"),
			WithServerDisabled(),
		)
		require.NoError(t, err)
		assert.Equal(t, "env-service", config.ServiceName())

		config.Shutdown(context.Background())
	})

	t.Run("OTEL_SERVICE_VERSION", func(t *testing.T) {
		os.Setenv("OTEL_SERVICE_VERSION", "v2.0.0")
		defer os.Unsetenv("OTEL_SERVICE_VERSION")

		config, err := New(
			WithServiceName("test-service"),
			WithProvider(PrometheusProvider),
			WithPort(":19304"),
			WithServerDisabled(),
		)
		require.NoError(t, err)
		assert.Equal(t, "v2.0.0", config.ServiceVersion())

		config.Shutdown(context.Background())
	})

	t.Run("RIVAAS_METRICS_PORT", func(t *testing.T) {
		os.Setenv("RIVAAS_METRICS_PORT", "19305")
		defer os.Unsetenv("RIVAAS_METRICS_PORT")

		config, err := New(
			WithServiceName("test-service"),
			WithServerDisabled(),
		)
		require.NoError(t, err)
		// Should add : prefix automatically
		assert.Equal(t, ":19305", config.metricsPort)

		config.Shutdown(context.Background())
	})

	t.Run("RIVAAS_METRICS_PATH", func(t *testing.T) {
		os.Setenv("RIVAAS_METRICS_PATH", "/custom/metrics")
		defer os.Unsetenv("RIVAAS_METRICS_PATH")

		config, err := New(
			WithServiceName("test-service"),
			WithPort(":19306"),
			WithServerDisabled(),
		)
		require.NoError(t, err)
		assert.Equal(t, "/custom/metrics", config.metricsPath)

		config.Shutdown(context.Background())
	})
}

// TestPortDiscovery tests port auto-discovery behavior
func TestPortDiscovery(t *testing.T) {
	t.Run("StrictPortFailsOnUnavailable", func(t *testing.T) {
		// First, occupy a port
		config1, err := New(
			WithServiceName("test-service-1"),
			WithPort(":19307"),
			WithStrictPort(),
		)
		require.NoError(t, err)
		defer config1.Shutdown(context.Background())

		// Wait for server to start
		err = waitForMetricsServer("localhost:19307", 1*time.Second)
		require.NoError(t, err, "First server should start successfully")

		// Try to use the same port in strict mode - should fail to start server
		// Note: New() still succeeds, but the server goroutine will fail
		config2, err := New(
			WithServiceName("test-service-2"),
			WithPort(":19307"),
			WithStrictPort(),
		)
		require.NoError(t, err) // New() succeeds

		// Wait a bit for server start attempt
		time.Sleep(100 * time.Millisecond)

		// In strict mode, GetServerAddress still returns configured port
		// (even if server failed to start - this is current behavior)
		// The failure is logged but doesn't prevent New() from succeeding
		assert.Equal(t, ":19307", config2.GetServerAddress())

		config2.Shutdown(context.Background())
	})

	t.Run("FlexiblePortFindsAlternative", func(t *testing.T) {
		// First, occupy a port
		config1, err := New(
			WithServiceName("test-service-1"),
			WithPort(":19308"),
		)
		require.NoError(t, err)
		defer config1.Shutdown(context.Background())

		// Wait for server to start
		time.Sleep(100 * time.Millisecond)

		// Try to use the same port without strict mode - should find alternative
		config2, err := New(
			WithServiceName("test-service-2"),
			WithPort(":19308"),
			// No WithStrictPort() - should auto-discover
		)
		require.NoError(t, err)
		defer config2.Shutdown(context.Background())

		// Should have found a different port
		assert.NotEqual(t, ":19308", config2.GetServerAddress())
		// Should have a port assigned
		assert.NotEmpty(t, config2.GetServerAddress())
	})
}

// TestValidationEdgeCases tests configuration validation edge cases
func TestValidationEdgeCases(t *testing.T) {
	t.Run("VeryLowExportInterval", func(t *testing.T) {
		// Should warn but not error
		config, err := New(
			WithServiceName("test-service"),
			WithProvider(StdoutProvider),
			WithExportInterval(500*time.Millisecond), // Very low
		)
		require.NoError(t, err) // Should succeed despite warning
		require.NotNil(t, config)

		config.Shutdown(context.Background())
	})

	t.Run("CustomPrometheusPath", func(t *testing.T) {
		config, err := New(
			WithServiceName("test-service"),
			WithProvider(PrometheusProvider),
			WithPath("/custom-metrics"),
			WithPort(":19309"),
			WithServerDisabled(),
		)
		require.NoError(t, err)
		assert.Equal(t, "/custom-metrics", config.metricsPath)

		config.Shutdown(context.Background())
	})

	t.Run("ExcludeMultiplePaths", func(t *testing.T) {
		config, err := New(
			WithServiceName("test-service"),
			WithExcludePaths("/health", "/metrics", "/debug"),
			WithPort(":19310"),
			WithServerDisabled(),
		)
		require.NoError(t, err)
		assert.True(t, config.excludePaths["/health"])
		assert.True(t, config.excludePaths["/metrics"])
		assert.True(t, config.excludePaths["/debug"])

		config.Shutdown(context.Background())
	})

	t.Run("RecordSpecificHeaders", func(t *testing.T) {
		config, err := New(
			WithServiceName("test-service"),
			WithHeaders("Authorization", "X-Request-ID", "User-Agent"),
			WithPort(":19311"),
			WithServerDisabled(),
		)
		require.NoError(t, err)
		assert.Len(t, config.recordHeaders, 3)

		config.Shutdown(context.Background())
	})
}

// TestShutdownEdgeCases tests shutdown behavior in various scenarios
func TestShutdownEdgeCases(t *testing.T) {
	t.Run("ShutdownWithoutServer", func(t *testing.T) {
		config, err := New(
			WithServiceName("test-service"),
			WithServerDisabled(),
		)
		require.NoError(t, err)

		err = config.Shutdown(context.Background())
		assert.NoError(t, err)
	})

	t.Run("ShutdownWithCancelledContext", func(t *testing.T) {
		config, err := New(
			WithServiceName("test-service"),
			WithPort(":19312"),
		)
		require.NoError(t, err)

		// Wait for server to start
		time.Sleep(100 * time.Millisecond)

		// Create cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Shutdown with cancelled context - should still work
		err = config.Shutdown(ctx)
		// May error due to cancelled context, but shouldn't panic
		_ = err
	})

	t.Run("ShutdownWithTimeout", func(t *testing.T) {
		config, err := New(
			WithServiceName("test-service"),
			WithPort(":19313"),
		)
		require.NoError(t, err)

		// Wait for server to start
		time.Sleep(100 * time.Millisecond)

		// Create context with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		// Shutdown might timeout but shouldn't panic
		_ = config.Shutdown(ctx)
	})
}

// TestMetricNameValidationEdgeCases tests additional validation scenarios
func TestMetricNameValidationEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		metricName    string
		shouldSucceed bool
	}{
		{"SingleCharacter", "a", true},
		{"WithNumbers", "metric123", true},
		{"MixedCase", "MyMetric", true},
		{"WithUnderscores", "my_metric_name", true},
		{"WithDots", "my.metric.name", true},
		{"WithHyphens", "my-metric-name", true},
		{"Complex", "my_metric.name-v2", true},
		{"LeadingUnderscore", "_metric", false},  // Invalid: doesn't start with letter
		{"DoubleDash", "__metric", false},        // Invalid: reserved prefix
		{"HTTPPrefix", "http_metric", false},     // Invalid: reserved prefix
		{"RouterPrefix", "router_metric", false}, // Invalid: reserved prefix
	}

	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":19314"),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialFailures := config.getAtomicCustomMetricFailures()
			config.IncrementCounter(ctx, tt.metricName)
			newFailures := config.getAtomicCustomMetricFailures()

			if tt.shouldSucceed {
				assert.Equal(t, initialFailures, newFailures,
					"Should not fail for valid metric name: %s", tt.metricName)
			} else {
				assert.Greater(t, newFailures, initialFailures,
					"Should fail for invalid metric name: %s", tt.metricName)
			}
		})
	}
}

// TestResponseWriterEdgeCases tests the responseWriter wrapper
func TestResponseWriterEdgeCases(t *testing.T) {
	t.Run("DefaultStatusCode", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec}
		assert.Equal(t, http.StatusOK, rw.StatusCode())
	})

	t.Run("WriteWithoutWriteHeader", func(t *testing.T) {
		// This tests that Write sets status code to 200 if not set
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec}
		rw.Write([]byte("test"))
		assert.Equal(t, http.StatusOK, rw.StatusCode())
		assert.Equal(t, 4, rw.Size())
	})

	t.Run("MultipleWriteHeaderCalls", func(t *testing.T) {
		// Tests that duplicate WriteHeader calls are prevented
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec}
		rw.WriteHeader(http.StatusNotFound)
		rw.WriteHeader(http.StatusOK) // Should be ignored
		assert.Equal(t, http.StatusNotFound, rw.StatusCode())
		assert.True(t, rw.written)
	})

	t.Run("MultipleWrites", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec}

		n1, err := rw.Write([]byte("Hello "))
		assert.NoError(t, err)
		assert.Equal(t, 6, n1)

		n2, err := rw.Write([]byte("World"))
		assert.NoError(t, err)
		assert.Equal(t, 5, n2)

		assert.Equal(t, 11, rw.Size())
		assert.Equal(t, http.StatusOK, rw.StatusCode())
	})
}

// TestGetStatusClass tests HTTP status code classification
func TestGetStatusClass(t *testing.T) {
	tests := []struct {
		code     int
		expected string
	}{
		{200, "2xx"},
		{299, "2xx"},
		{300, "3xx"},
		{399, "3xx"},
		{400, "4xx"},
		{499, "4xx"},
		{500, "5xx"},
		{599, "5xx"},
		{100, "unknown"},
		{600, "5xx"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Status%d", tt.code), func(t *testing.T) {
			result := getStatusClass(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestContextMetrics tests the ContextMetrics helper
func TestContextMetrics(t *testing.T) {
	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":19315"),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	cm := NewContextMetrics(config)
	ctx := context.Background()

	// Test all methods
	cm.RecordMetric(ctx, "test_metric", 1.5)
	cm.IncrementCounter(ctx, "test_counter")
	cm.SetGauge(ctx, "test_gauge", 42.0)

	// Test GetConfig
	assert.Equal(t, config, cm.GetConfig())
}

// TestMetricsWithDisabledState tests behavior when metrics are disabled
func TestMetricsWithDisabledState(t *testing.T) {
	config := &Config{
		enabled: false,
	}

	ctx := context.Background()

	// These should all be no-ops
	config.IncrementCounter(ctx, "test")
	config.RecordMetric(ctx, "test", 1.0)
	config.SetGauge(ctx, "test", 1.0)
	config.RecordRouteRegistration(ctx, "GET", "/test")
	config.RecordContextPoolHit(ctx)
	config.RecordContextPoolMiss(ctx)
	config.RecordConstraintFailure(ctx, "test")

	result := config.StartRequest(ctx, "/test", false)
	assert.Nil(t, result)

	// Shutdown should succeed
	err := config.Shutdown(context.Background())
	assert.NoError(t, err)

	// IsEnabled should return false
	assert.False(t, config.IsEnabled())

	// GetProvider should return empty string
	assert.Equal(t, Provider(""), config.GetProvider())

	// GetServerAddress should return empty string
	assert.Equal(t, "", config.GetServerAddress())
}

// TestNewStandalone tests the standalone constructor
func TestNewStandalone(t *testing.T) {
	config := NewStandalone(
		WithServiceName("standalone-service"),
		WithProvider(StdoutProvider),
	)
	require.NotNil(t, config)
	assert.True(t, config.IsEnabled())

	config.Shutdown(context.Background())
}
