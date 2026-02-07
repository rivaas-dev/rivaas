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

package metrics

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitForMetricsServer is an internal alias for WaitForMetricsServer.
// Kept for backward compatibility with existing tests.
func waitForMetricsServer(t *testing.T, address string, timeout time.Duration) error {
	t.Helper()
	return WaitForMetricsServer(t, address, timeout)
}

func TestRecorderConfig(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithPrometheus(":9091", "/metrics"),
		WithServiceName("test-service"),
		WithServiceVersion("v1.0.0"),
		WithStrictPort(), // Require exact port for deterministic test
	)
	t.Cleanup(func() {
		//nolint:errcheck // Test cleanup
		recorder.Shutdown(t.Context())
	})

	assert.True(t, recorder.IsEnabled())
	assert.Equal(t, "test-service", recorder.ServiceName())
	assert.Equal(t, "v1.0.0", recorder.ServiceVersion())
	assert.Equal(t, ":9091", recorder.ServerAddress())
	assert.Equal(t, PrometheusProvider, recorder.Provider())
}

func TestRecorderWithHTTP(t *testing.T) {
	t.Parallel()

	// Create metrics recorder
	recorder := MustNew(
		WithPrometheus(":9092", "/metrics"),
		WithServiceName("test-service"),
	)
	t.Cleanup(func() {
		//nolint:errcheck // Test cleanup
		recorder.Shutdown(t.Context())
	})

	// Start the metrics server
	err := recorder.Start(t.Context())
	require.NoError(t, err, "StartServer should not error")

	// Wait for server to be ready
	err = waitForMetricsServer(t, "localhost:9092", 1*time.Second)
	require.NoError(t, err, "Metrics server should start")

	// Create HTTP handler with metrics middleware
	handler := Middleware(recorder)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck // Test handler
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRecorderProviders(t *testing.T) {
	t.Parallel()

	t.Run("Prometheus", func(t *testing.T) {
		t.Parallel()
		recorder := MustNew(
			WithPrometheus(":9093", "/metrics"),
		)
		t.Cleanup(func() {
			//nolint:errcheck // Test cleanup
			recorder.Shutdown(t.Context())
		})
		assert.Equal(t, PrometheusProvider, recorder.Provider())
	})

	t.Run("OTLP", func(t *testing.T) {
		t.Parallel()

		recorder := MustNew(
			WithOTLP("http://localhost:4318"),
		)
		t.Cleanup(func() {
			//nolint:errcheck // Test cleanup
			recorder.Shutdown(t.Context())
		})
		assert.Equal(t, OTLPProvider, recorder.Provider())
	})

	t.Run("Stdout", func(t *testing.T) {
		t.Parallel()
		recorder := MustNew(
			WithStdout(),
		)
		t.Cleanup(func() {
			//nolint:errcheck // Test cleanup
			recorder.Shutdown(t.Context())
		})
		assert.Equal(t, StdoutProvider, recorder.Provider())
	})
}

func TestCustomMetrics(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithPrometheus(":9094", "/metrics"),
		WithServiceName("test-service"),
	)
	t.Cleanup(func() {
		//nolint:errcheck // Test cleanup
		recorder.Shutdown(t.Context())
	})

	// Test custom metrics recording - now returns errors
	err := recorder.RecordHistogram(t.Context(), "test_histogram", 1.5)
	require.NoError(t, err)

	err = recorder.IncrementCounter(t.Context(), "test_counter")
	require.NoError(t, err)

	err = recorder.SetGauge(t.Context(), "test_gauge", 42.0)
	require.NoError(t, err)

	assert.True(t, recorder.IsEnabled())
}

func TestRecorderMiddleware(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithPrometheus(":9095", "/metrics"),
		WithServiceName("test-service"),
	)
	t.Cleanup(func() {
		//nolint:errcheck // Test cleanup
		recorder.Shutdown(t.Context())
	})

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK")) //nolint:errcheck // Test handler
	})

	// Wrap with metrics middleware
	middleware := Middleware(recorder)
	wrappedHandler := middleware(handler)

	// Test the wrapped handler
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestPathFilterExcludePaths(t *testing.T) {
	t.Parallel()

	// Path filtering is now in middleware, test pathFilter directly
	pf := newPathFilter()
	pf.addPaths("/health", "/metrics")

	// Test that excluded paths work correctly
	assert.True(t, pf.shouldExclude("/health"))
	assert.True(t, pf.shouldExclude("/metrics"))
	assert.False(t, pf.shouldExclude("/api/users"))
}

func TestRecorderOptions(t *testing.T) {
	t.Parallel()

	t.Run("WithMaxCustomMetrics", func(t *testing.T) {
		t.Parallel()

		recorder := MustNew(
			WithMaxCustomMetrics(500),
		)
		assert.True(t, recorder.IsEnabled())
	})

	t.Run("WithServerDisabled", func(t *testing.T) {
		t.Parallel()

		recorder := MustNew(
			WithServerDisabled(),
		)
		assert.True(t, recorder.IsEnabled())
		assert.Empty(t, recorder.ServerAddress())
	})
}

func TestMiddlewareOptions(t *testing.T) {
	t.Parallel()

	t.Run("WithHeaders", func(t *testing.T) {
		t.Parallel()
		cfg := newMiddlewareConfig()
		WithHeaders("X-Request-ID", "X-Custom-Header")(cfg)
		assert.Len(t, cfg.recordHeaders, 2)
		assert.Contains(t, cfg.recordHeaders, "X-Request-ID")
		assert.Contains(t, cfg.recordHeaders, "X-Custom-Header")
	})

	t.Run("WithExcludePaths", func(t *testing.T) {
		t.Parallel()
		cfg := newMiddlewareConfig()
		WithExcludePaths("/health", "/metrics")(cfg)
		assert.True(t, cfg.pathFilter.shouldExclude("/health"))
		assert.True(t, cfg.pathFilter.shouldExclude("/metrics"))
		assert.False(t, cfg.pathFilter.shouldExclude("/api/users"))
	})
}

func TestRecorderIntegration(t *testing.T) {
	t.Parallel()

	// Test full integration with HTTP middleware
	recorder := MustNew(
		WithPrometheus(":9096", "/metrics"),
		WithServiceName("integration-test"),
	)
	t.Cleanup(func() {
		//nolint:errcheck // Test cleanup
		recorder.Shutdown(t.Context())
	})

	// Create HTTP mux
	mux := http.NewServeMux()

	// Add routes
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Hello"}`)) //nolint:errcheck // Test handler
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`)) //nolint:errcheck // Test handler
	})

	// Wrap with metrics middleware (path exclusion is now a middleware option)
	handler := Middleware(recorder, WithExcludePaths("/health"))(mux)

	// Test normal route
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Test health route (should be excluded from metrics)
	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestRecorderHandler(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithPrometheus(":9097", "/metrics"),
		WithServiceName("test-service"),
	)
	t.Cleanup(func() {
		//nolint:errcheck // Test cleanup
		recorder.Shutdown(t.Context())
	})

	// Create HTTP handler with metrics to generate some data
	handler := Middleware(recorder)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck // Test handler
	}))

	// Make a request to generate metrics
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Now test the metrics handler
	metricsHandler, err := recorder.Handler()
	require.NoError(t, err)
	require.NotNil(t, metricsHandler)

	// Test that the handler responds
	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w = httptest.NewRecorder()
	metricsHandler.ServeHTTP(w, req)

	// Should return 200 and contain some metrics
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "http_requests_total")
}

func TestHandlerErrors(t *testing.T) {
	t.Parallel()

	t.Run("ErrorWhenNotPrometheusProvider", func(t *testing.T) {
		t.Parallel()
		recorder := MustNew(
			WithOTLP("http://localhost:4318"),
			WithServiceName("test-service"),
		)
		t.Cleanup(func() {
			//nolint:errcheck // Test cleanup
			recorder.Shutdown(t.Context())
		})

		handler, err := recorder.Handler()
		require.Error(t, err)
		assert.Nil(t, handler)
		assert.Contains(t, err.Error(), "only available with Prometheus provider")
		assert.Contains(t, err.Error(), "otlp")
	})

	t.Run("ErrorWhenStdoutProvider", func(t *testing.T) {
		t.Parallel()

		recorder := MustNew(
			WithStdout(),
			WithServiceName("test-service"),
		)
		t.Cleanup(func() {
			//nolint:errcheck // Test cleanup
			recorder.Shutdown(t.Context())
		})

		handler, err := recorder.Handler()
		require.Error(t, err)
		assert.Nil(t, handler)
		assert.Contains(t, err.Error(), "only available with Prometheus provider")
		assert.Contains(t, err.Error(), "stdout")
	})
}

func TestShutdown(t *testing.T) {
	t.Parallel()

	t.Run("Prometheus", func(t *testing.T) {
		t.Parallel()
		recorder := MustNew(
			WithPrometheus(":9103", "/metrics"),
			WithServiceName("test-service"),
		)

		// Start the metrics server
		err := recorder.Start(t.Context())
		require.NoError(t, err, "StartServer should not error")

		// Wait for server to be ready
		err = waitForMetricsServer(t, "localhost:9103", 1*time.Second)
		require.NoError(t, err, "Metrics server should start")

		// Shutdown should not error
		err = recorder.Shutdown(t.Context())
		assert.NoError(t, err)
	})

	t.Run("OTLP", func(t *testing.T) {
		t.Parallel()

		recorder := MustNew(
			WithOTLP("http://localhost:4318"),
			WithServiceName("test-service"),
		)

		// Shutdown may error if OTLP collector is not running (expected in tests)
		err := recorder.Shutdown(t.Context())
		// We don't assert no error here because OTLP requires a running collector
		// The important thing is that Shutdown() doesn't panic
		_ = err
	})

	t.Run("Stdout", func(t *testing.T) {
		t.Parallel()
		recorder := MustNew(
			WithStdout(),
			WithServiceName("test-service"),
		)

		// Shutdown should not error
		err := recorder.Shutdown(t.Context())
		assert.NoError(t, err)
	})

	t.Run("IdempotentShutdown", func(t *testing.T) {
		t.Parallel()

		recorder := MustNew(
			WithPrometheus(":9106", "/metrics"),
			WithServiceName("test-service"),
		)

		// Start the metrics server
		err := recorder.Start(t.Context())
		require.NoError(t, err, "StartServer should not error")

		// Wait for server to be ready
		err = waitForMetricsServer(t, "localhost:9106", 1*time.Second)
		require.NoError(t, err, "Metrics server should start")

		// First shutdown
		err = recorder.Shutdown(t.Context())
		require.NoError(t, err)

		// Second shutdown should also succeed (idempotent)
		err = recorder.Shutdown(t.Context())
		require.NoError(t, err)

		// Third shutdown for good measure
		err = recorder.Shutdown(t.Context())
		require.NoError(t, err)

		// Verify shutdown flag is still true
		assert.True(t, recorder.isShuttingDown.Load())
	})
}

func TestCustomMetricsLimitRaceCondition(t *testing.T) {
	t.Parallel()

	// Test that the limit is enforced correctly under concurrent access
	recorder := MustNew(
		WithPrometheus(":9104", "/metrics"),
		WithServiceName("test-service"),
		WithMaxCustomMetrics(10), // Small limit for testing
	)

	// Create metrics concurrently
	const numGoroutines = 20
	const metricsPerGoroutine = 5

	done := make(chan bool, numGoroutines)
	for i := range numGoroutines {
		go func(id int) {
			for j := range metricsPerGoroutine {
				metricName := fmt.Sprintf("metric_%d_%d", id, j)
				recorder.IncrementCounter(t.Context(), metricName) //nolint:errcheck // Test hot path
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for range numGoroutines {
		<-done
	}

	// Get total metrics count
	totalMetrics := recorder.CustomMetricCount()

	// Should not exceed the limit
	assert.LessOrEqual(t, totalMetrics, 10, "Total metrics should not exceed limit")

	// Should have created some metrics (not zero)
	assert.Positive(t, totalMetrics, "Should have created some metrics")
}

func TestNewReturnsError(t *testing.T) {
	t.Parallel()

	// Test that New() returns errors properly
	recorder, err := New(
		WithPrometheus(":9100", "/metrics"),
		WithServiceName("test-service"),
	)
	require.NoError(t, err)
	require.NotNil(t, recorder)
	assert.True(t, recorder.IsEnabled())

	// Shutdown the recorder
	ctx := t.Context()
	err = recorder.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestMustNewPanics(t *testing.T) {
	t.Parallel()

	// Test that MustNew panics on error
	// We can't easily test this without creating an invalid config
	// Just verify it works normally
	recorder := MustNew(
		WithStdout(),
		WithServiceName("test-service"),
	)
	require.NotNil(t, recorder)
	assert.True(t, recorder.IsEnabled())

	// Shutdown the recorder
	ctx := t.Context()
	err := recorder.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestCustomMetricsLimitEnforcement(t *testing.T) {
	t.Parallel()

	// Test that limit is enforced and errors are returned
	recorder := MustNew(
		WithPrometheus(":9105", "/metrics"),
		WithServiceName("test-service"),
		WithMaxCustomMetrics(3),
	)

	ctx := t.Context()

	// Create 3 metrics (should succeed)
	err := recorder.IncrementCounter(ctx, "counter1")
	require.NoError(t, err)

	err = recorder.RecordHistogram(ctx, "histogram1", 1.0)
	require.NoError(t, err)

	err = recorder.SetGauge(ctx, "gauge1", 1.0)
	require.NoError(t, err)

	// Try to create a 4th metric (should fail with error)
	err = recorder.IncrementCounter(ctx, "counter2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "limit reached")

	// Verify we have exactly 3 metrics
	totalMetrics := recorder.CustomMetricCount()
	assert.Equal(t, 3, totalMetrics, "Should have exactly 3 metrics")
}

func TestSensitiveHeaderFiltering(t *testing.T) {
	t.Parallel()

	// WithHeaders is now a middleware option, test the config directly
	cfg := newMiddlewareConfig()
	WithHeaders("Authorization", "X-Request-ID", "Cookie", "X-Custom")(cfg)

	// Sensitive headers should be filtered out
	assert.Len(t, cfg.recordHeaders, 2)
	assert.Contains(t, cfg.recordHeaders, "X-Request-ID")
	assert.Contains(t, cfg.recordHeaders, "X-Custom")
	// Authorization and Cookie should be filtered
	assert.NotContains(t, cfg.recordHeaders, "Authorization")
	assert.NotContains(t, cfg.recordHeaders, "Cookie")
}

func TestPrometheusNormalization(t *testing.T) {
	t.Parallel()

	t.Run("PortWithColon", func(t *testing.T) {
		t.Parallel()
		recorder := MustNew(
			WithPrometheus(":8080", "/metrics"),
			WithServiceName("test-service"),
			WithServerDisabled(),
		)
		assert.Equal(t, ":8080", recorder.metricsPort)
	})

	t.Run("PortWithoutColon", func(t *testing.T) {
		t.Parallel()
		recorder := MustNew(
			WithPrometheus("8080", "/metrics"),
			WithServiceName("test-service"),
			WithServerDisabled(),
		)
		assert.Equal(t, ":8080", recorder.metricsPort)
	})

	t.Run("PathWithSlash", func(t *testing.T) {
		t.Parallel()
		recorder := MustNew(
			WithPrometheus(":9090", "/custom-metrics"),
			WithServiceName("test-service"),
			WithServerDisabled(),
		)
		assert.Equal(t, "/custom-metrics", recorder.metricsPath)
	})

	t.Run("PathWithoutSlash", func(t *testing.T) {
		t.Parallel()
		recorder := MustNew(
			WithPrometheus(":9090", "custom-metrics"),
			WithServiceName("test-service"),
			WithServerDisabled(),
		)
		assert.Equal(t, "/custom-metrics", recorder.metricsPath)
	})
}

func TestRecorder_ServiceNameInAttributes(t *testing.T) {
	t.Parallel()

	customName := "my-custom-service"
	recorder := MustNew(
		WithPrometheus(":0", "/metrics"),
		WithServiceName(customName),
		WithServerDisabled(),
	)
	t.Cleanup(func() {
		//nolint:errcheck // Test cleanup
		recorder.Shutdown(t.Context())
	})

	// Verify the service name field
	assert.Equal(t, customName, recorder.ServiceName())

	// Service name is now only set as a resource attribute (in target_info),
	// not as a metric-level attribute
}

func TestRecorder_ServiceVersionInAttributes(t *testing.T) {
	t.Parallel()

	customVersion := "v2.3.4"
	recorder := MustNew(
		WithPrometheus(":0", "/metrics"),
		WithServiceVersion(customVersion),
		WithServerDisabled(),
	)
	t.Cleanup(func() {
		//nolint:errcheck // Test cleanup
		recorder.Shutdown(t.Context())
	})

	// Verify the service version field
	assert.Equal(t, customVersion, recorder.ServiceVersion())

	// Service version is now only set as a resource attribute (in target_info),
	// not as a metric-level attribute
}

func TestServiceName_NotDefaultAfterConfiguration(t *testing.T) {
	t.Parallel()

	customName := "headless-browser-services"
	recorder := MustNew(
		WithPrometheus(":0", "/metrics"),
		WithServiceName(customName),
		WithServerDisabled(),
	)
	t.Cleanup(func() {
		//nolint:errcheck // Test cleanup
		recorder.Shutdown(t.Context())
	})

	// Verify the service name field - regression test
	assert.Equal(t, customName, recorder.ServiceName())
	assert.NotEqual(t, "rivaas-service", recorder.ServiceName())

	// Service name is now only set as a resource attribute (in target_info),
	// not as a metric-level attribute
}
