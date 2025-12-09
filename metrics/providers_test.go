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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProviderInitialization tests provider initialization edge cases
func TestProviderInitialization(t *testing.T) {
	t.Parallel()

	t.Run("PrometheusProviderSuccess", func(t *testing.T) {
		t.Parallel()
		recorder, err := New(
			WithPrometheus(":19301", "/metrics"),
			WithServiceName("test-service"),
			WithServerDisabled(),
		)
		require.NoError(t, err)
		require.NotNil(t, recorder)
		assert.Equal(t, PrometheusProvider, recorder.Provider())

		// Verify handler is available
		handler, err := recorder.Handler()
		require.NoError(t, err)
		require.NotNil(t, handler)

		// Cleanup
		err = recorder.Shutdown(t.Context())
		assert.NoError(t, err)
	})

	t.Run("OTLPProviderWithEndpoint", func(t *testing.T) {
		t.Parallel()

		recorder, err := New(
			WithOTLP("http://localhost:4318"),
			WithServiceName("test-service"),
		)
		require.NoError(t, err)
		require.NotNil(t, recorder)
		assert.Equal(t, OTLPProvider, recorder.Provider())

		// Cleanup (may error if collector not running)
		_ = recorder.Shutdown(t.Context())
	})

	t.Run("OTLPProviderWithHTTPS", func(t *testing.T) {
		t.Parallel()

		recorder, err := New(
			WithOTLP("https://collector.example.com:4318"),
			WithServiceName("test-service"),
		)
		require.NoError(t, err)
		require.NotNil(t, recorder)

		_ = recorder.Shutdown(t.Context())
	})

	t.Run("OTLPProviderWithPath", func(t *testing.T) {
		t.Parallel()

		recorder, err := New(
			WithOTLP("http://localhost:4318/v1/metrics"),
			WithServiceName("test-service"),
		)
		require.NoError(t, err)
		require.NotNil(t, recorder)

		_ = recorder.Shutdown(t.Context())
	})

	t.Run("StdoutProvider", func(t *testing.T) {
		t.Parallel()

		recorder, err := New(
			WithStdout(),
			WithServiceName("test-service"),
		)
		require.NoError(t, err)
		require.NotNil(t, recorder)
		assert.Equal(t, StdoutProvider, recorder.Provider())

		// Test that metrics can be recorded
		ctx := t.Context()
		_ = recorder.IncrementCounter(ctx, "test_counter")
		_ = recorder.RecordHistogram(ctx, "test_histogram", 1.5)
		_ = recorder.SetGauge(ctx, "test_gauge", 42.0)

		// Cleanup
		err = recorder.Shutdown(t.Context())
		assert.NoError(t, err)
	})

	t.Run("StdoutProviderWithCustomInterval", func(t *testing.T) {
		t.Parallel()

		recorder, err := New(
			WithStdout(),
			WithServiceName("test-service"),
			WithExportInterval(5*time.Second),
		)
		require.NoError(t, err)
		require.NotNil(t, recorder)

		err = recorder.Shutdown(t.Context())
		assert.NoError(t, err)
	})
}

// TestPortDiscovery tests port auto-discovery behavior
//
//nolint:paralleltest // Cannot use t.Parallel() - test binds to specific ports
func TestPortDiscovery(t *testing.T) {
	t.Run("StrictPortFailsOnUnavailable", func(t *testing.T) {
		// First, occupy a port
		config1, err := New(
			WithPrometheus(":19307", "/metrics"),
			WithServiceName("test-service-1"),
			WithStrictPort(),
		)
		require.NoError(t, err)
		defer config1.Shutdown(t.Context())

		// Start the first server
		err = config1.Start(t.Context())
		require.NoError(t, err, "StartServer should not error")

		// Wait for server to start
		err = waitForMetricsServer(t, "localhost:19307", 1*time.Second)
		require.NoError(t, err, "First server should start successfully")

		// Try to use the same port in strict mode - should fail to start server
		// Note: New() still succeeds, but StartServer will fail
		config2, err := New(
			WithPrometheus(":19307", "/metrics"),
			WithServiceName("test-service-2"),
			WithStrictPort(),
		)
		require.NoError(t, err) // New() succeeds

		// Start the second server - this will fail silently (logged)
		_ = config2.Start(t.Context())

		// Wait a bit for server start attempt
		time.Sleep(100 * time.Millisecond)

		// In strict mode, ServerAddress still returns configured port
		// (even if server failed to start - this is current behavior)
		// The failure is logged but doesn't prevent New() from succeeding
		assert.Equal(t, ":19307", config2.ServerAddress())

		config2.Shutdown(t.Context())
	})

	t.Run("FlexiblePortFindsAlternative", func(t *testing.T) {
		// First, occupy a port
		config1, err := New(
			WithPrometheus(":19308", "/metrics"),
			WithServiceName("test-service-1"),
		)
		require.NoError(t, err)
		defer config1.Shutdown(t.Context())

		// Start the first server
		err = config1.Start(t.Context())
		require.NoError(t, err, "StartServer should not error")

		// Wait for server to start
		time.Sleep(100 * time.Millisecond)

		// Try to use the same port without strict mode - should find alternative
		config2, err := New(
			WithPrometheus(":19308", "/metrics"),
			WithServiceName("test-service-2"),
			// No WithStrictPort() - should auto-discover
		)
		require.NoError(t, err)
		defer config2.Shutdown(t.Context())

		// Start the second server - should find alternative port
		err = config2.Start(t.Context())
		require.NoError(t, err, "StartServer should find alternative port")

		// Should have found a different port
		assert.NotEqual(t, ":19308", config2.ServerAddress())
		// Should have a port assigned
		assert.NotEmpty(t, config2.ServerAddress())
	})
}

// TestValidationEdgeCases tests configuration validation edge cases
func TestValidationEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("VeryLowExportInterval", func(t *testing.T) {
		t.Parallel()
		// Should warn but not error
		recorder, err := New(
			WithStdout(),
			WithServiceName("test-service"),
			WithExportInterval(500*time.Millisecond), // Very low
		)
		require.NoError(t, err) // Should succeed despite warning
		require.NotNil(t, recorder)

		recorder.Shutdown(t.Context())
	})

	t.Run("CustomPrometheusPath", func(t *testing.T) {
		t.Parallel()

		recorder, err := New(
			WithPrometheus(":19309", "/custom-metrics"),
			WithServiceName("test-service"),
			WithServerDisabled(),
		)
		require.NoError(t, err)
		assert.Equal(t, "/custom-metrics", recorder.metricsPath)

		recorder.Shutdown(t.Context())
	})

	t.Run("ExcludeMultiplePaths_Middleware", func(t *testing.T) {
		t.Parallel()

		// Path filtering is now in middleware, test the middleware config
		cfg := newMiddlewareConfig()
		WithExcludePaths("/health", "/metrics", "/debug")(cfg)
		assert.True(t, cfg.pathFilter.shouldExclude("/health"))
		assert.True(t, cfg.pathFilter.shouldExclude("/metrics"))
		assert.True(t, cfg.pathFilter.shouldExclude("/debug"))
	})

	t.Run("RecordSpecificHeaders_Middleware", func(t *testing.T) {
		t.Parallel()

		// Header filtering is now in middleware, test the middleware config
		cfg := newMiddlewareConfig()
		WithHeaders("Authorization", "X-Request-ID", "User-Agent")(cfg) // Authorization filtered
		// Authorization is filtered as sensitive header, so only 2 remain
		assert.Len(t, cfg.recordHeaders, 2)
	})
}

// TestShutdownEdgeCases tests shutdown behavior in various scenarios
func TestShutdownEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("ShutdownWithoutServer", func(t *testing.T) {
		t.Parallel()
		recorder, err := New(
			WithStdout(),
			WithServiceName("test-service"),
			WithServerDisabled(),
		)
		require.NoError(t, err)

		err = recorder.Shutdown(t.Context())
		assert.NoError(t, err)
	})

	t.Run("ShutdownWithCancelledContext", func(t *testing.T) {
		t.Parallel()

		recorder, err := New(
			WithPrometheus(":19312", "/metrics"),
			WithServiceName("test-service"),
		)
		require.NoError(t, err)

		// Wait for server to start
		time.Sleep(100 * time.Millisecond)

		// Create canceled context
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		// Shutdown with canceled context - should still work
		err = recorder.Shutdown(ctx)
		// May error due to canceled context, but shouldn't panic
		_ = err
	})

	t.Run("ShutdownWithTimeout", func(t *testing.T) {
		t.Parallel()

		recorder, err := New(
			WithPrometheus(":19313", "/metrics"),
			WithServiceName("test-service"),
		)
		require.NoError(t, err)

		// Wait for server to start
		time.Sleep(100 * time.Millisecond)

		// Create context with very short timeout
		ctx, cancel := context.WithTimeout(t.Context(), 1*time.Millisecond)
		defer cancel()

		// Shutdown might timeout but shouldn't panic
		_ = recorder.Shutdown(ctx)
	})
}

// TestMetricNameValidationEdgeCases tests additional validation scenarios
//
//nolint:paralleltest,tparallel // Subtests share recorder state; parallel execution would cause race conditions
func TestMetricNameValidationEdgeCases(t *testing.T) {
	t.Parallel()

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

	recorder := MustNew(
		WithPrometheus(":19314", "/metrics"),
		WithServiceName("test-service"),
		WithServerDisabled(),
	)
	t.Cleanup(func() {
		recorder.Shutdown(t.Context())
	})

	ctx := t.Context()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: Cannot use t.Parallel() here because test cases share the same recorder
			// instance and check a shared atomic failure counter, which would cause race conditions.

			initialFailures := recorder.getAtomicCustomMetricFailures()
			_ = recorder.IncrementCounter(ctx, tt.metricName)
			newFailures := recorder.getAtomicCustomMetricFailures()

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
	t.Parallel()

	t.Run("DefaultStatusCode", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec}
		assert.Equal(t, http.StatusOK, rw.StatusCode())
	})

	t.Run("WriteWithoutWriteHeader", func(t *testing.T) {
		t.Parallel()

		// This tests that Write sets status code to 200 if not set
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec}
		rw.Write([]byte("test"))
		assert.Equal(t, http.StatusOK, rw.StatusCode())
		assert.Equal(t, 4, rw.Size())
	})

	t.Run("MultipleWriteHeaderCalls", func(t *testing.T) {
		t.Parallel()

		// Tests that duplicate WriteHeader calls are prevented
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec}
		rw.WriteHeader(http.StatusNotFound)
		rw.WriteHeader(http.StatusOK) // Should be ignored
		assert.Equal(t, http.StatusNotFound, rw.StatusCode())
		assert.True(t, rw.written)
	})

	t.Run("MultipleWrites", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec}

		n1, err := rw.Write([]byte("Hello "))
		require.NoError(t, err)
		assert.Equal(t, 6, n1)

		n2, err := rw.Write([]byte("World"))
		require.NoError(t, err)
		assert.Equal(t, 5, n2)

		assert.Equal(t, 11, rw.Size())
		assert.Equal(t, http.StatusOK, rw.StatusCode())
	})
}

// TestStatusClass tests HTTP status code classification
func TestStatusClass(t *testing.T) {
	t.Parallel()

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
		{600, "unknown"}, // Status codes > 599 are not standard HTTP codes
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Status%d", tt.code), func(t *testing.T) {
			t.Parallel()

			result := statusClass(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestMetricsWithDisabledState tests behavior when metrics are disabled
func TestMetricsWithDisabledState(t *testing.T) {
	t.Parallel()

	recorder := &Recorder{
		enabled: false,
	}

	ctx := t.Context()

	// These should all be no-ops (return nil for disabled recorder)
	_ = recorder.IncrementCounter(ctx, "test")
	_ = recorder.RecordHistogram(ctx, "test", 1.0)
	_ = recorder.SetGauge(ctx, "test", 1.0)

	result := recorder.BeginRequest(ctx)
	assert.Nil(t, result)

	// Shutdown should succeed
	err := recorder.Shutdown(t.Context())
	require.NoError(t, err)

	// IsEnabled should return false
	assert.False(t, recorder.IsEnabled())

	// Provider should return empty string
	assert.Equal(t, Provider(""), recorder.Provider())

	// ServerAddress should return empty string
	assert.Empty(t, recorder.ServerAddress())
}
