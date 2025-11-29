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
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitForMetricsServer waits for the metrics server to be ready
func waitForMetricsServer(t *testing.T, address string, timeout time.Duration) error {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("metrics server not ready after %v", timeout)
}

func TestRecorderConfig(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithPrometheus(":9091", "/metrics"),
		WithServiceName("test-service"),
		WithServiceVersion("v1.0.0"),
		WithStrictPort(), // Require exact port for deterministic test
	)
	defer recorder.Shutdown(context.Background())

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

	// Wait for server to be ready
	err := waitForMetricsServer(t, "localhost:9092", 1*time.Second)
	require.NoError(t, err, "Metrics server should start")

	// Create HTTP handler with metrics middleware
	handler := Middleware(recorder)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
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
		assert.Equal(t, PrometheusProvider, recorder.Provider())
	})

	t.Run("OTLP", func(t *testing.T) {
		t.Parallel()

		recorder := MustNew(
			WithOTLP("http://localhost:4318"),
		)
		assert.Equal(t, OTLPProvider, recorder.Provider())
	})

	t.Run("Stdout", func(t *testing.T) {
		t.Parallel()
		recorder := MustNew(
			WithStdout(),
		)
		assert.Equal(t, StdoutProvider, recorder.Provider())
	})
}

func TestCustomMetrics(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithPrometheus(":9094", "/metrics"),
		WithServiceName("test-service"),
	)

	ctx := context.Background()

	// Test custom metrics recording - now returns errors
	err := recorder.RecordHistogram(ctx, "test_histogram", 1.5)
	assert.NoError(t, err)

	err = recorder.IncrementCounter(ctx, "test_counter")
	assert.NoError(t, err)

	err = recorder.SetGauge(ctx, "test_gauge", 42.0)
	assert.NoError(t, err)

	assert.True(t, recorder.IsEnabled())
}

func TestRecorderMiddleware(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithPrometheus(":9095", "/metrics"),
		WithServiceName("test-service"),
	)

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with metrics middleware
	middleware := Middleware(recorder)
	wrappedHandler := middleware(handler)

	// Test the wrapped handler
	req := httptest.NewRequest("GET", "/test", nil)
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
		assert.Equal(t, "", recorder.ServerAddress())
	})
}

func TestMiddlewareOptions(t *testing.T) {
	t.Parallel()

	t.Run("WithHeaders", func(t *testing.T) {
		t.Parallel()
		cfg := newMiddlewareConfig()
		WithHeaders("X-Request-ID", "X-Custom-Header")(cfg)
		assert.Equal(t, 2, len(cfg.recordHeaders))
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

	// Create HTTP mux
	mux := http.NewServeMux()

	// Add routes
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Hello"}`))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	// Wrap with metrics middleware (path exclusion is now a middleware option)
	handler := Middleware(recorder, WithExcludePaths("/health"))(mux)

	// Test normal route
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Test health route (should be excluded from metrics)
	req = httptest.NewRequest("GET", "/health", nil)
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

	// Create HTTP handler with metrics to generate some data
	handler := Middleware(recorder)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))

	// Make a request to generate metrics
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Now test the metrics handler
	metricsHandler, err := recorder.Handler()
	require.NoError(t, err)
	require.NotNil(t, metricsHandler)

	// Test that the handler responds
	req = httptest.NewRequest("GET", "/metrics", nil)
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
		defer recorder.Shutdown(context.Background())

		handler, err := recorder.Handler()
		assert.Error(t, err)
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
		defer recorder.Shutdown(context.Background())

		handler, err := recorder.Handler()
		assert.Error(t, err)
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

		// Wait for server to be ready
		err := waitForMetricsServer(t, "localhost:9103", 1*time.Second)
		require.NoError(t, err, "Metrics server should start")

		// Shutdown should not error
		ctx := context.Background()
		err = recorder.Shutdown(ctx)
		assert.NoError(t, err)
	})

	t.Run("OTLP", func(t *testing.T) {
		t.Parallel()

		recorder := MustNew(
			WithOTLP("http://localhost:4318"),
			WithServiceName("test-service"),
		)

		// Shutdown may error if OTLP collector is not running (expected in tests)
		ctx := context.Background()
		err := recorder.Shutdown(ctx)
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
		ctx := context.Background()
		err := recorder.Shutdown(ctx)
		assert.NoError(t, err)
	})

	t.Run("IdempotentShutdown", func(t *testing.T) {
		t.Parallel()

		recorder := MustNew(
			WithPrometheus(":9106", "/metrics"),
			WithServiceName("test-service"),
		)

		// Wait for server to be ready
		err := waitForMetricsServer(t, "localhost:9106", 1*time.Second)
		require.NoError(t, err, "Metrics server should start")

		ctx := context.Background()

		// First shutdown
		err = recorder.Shutdown(ctx)
		assert.NoError(t, err)

		// Second shutdown should also succeed (idempotent)
		err = recorder.Shutdown(ctx)
		assert.NoError(t, err)

		// Third shutdown for good measure
		err = recorder.Shutdown(ctx)
		assert.NoError(t, err)

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

	ctx := context.Background()

	// Create metrics concurrently
	const numGoroutines = 20
	const metricsPerGoroutine = 5

	done := make(chan bool, numGoroutines)
	for i := range numGoroutines {
		go func(id int) {
			for j := range metricsPerGoroutine {
				metricName := fmt.Sprintf("metric_%d_%d", id, j)
				_ = recorder.IncrementCounter(ctx, metricName)
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
	assert.Greater(t, totalMetrics, 0, "Should have created some metrics")
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
	ctx := context.Background()
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
	ctx := context.Background()
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

	ctx := context.Background()

	// Create 3 metrics (should succeed)
	err := recorder.IncrementCounter(ctx, "counter1")
	assert.NoError(t, err)

	err = recorder.RecordHistogram(ctx, "histogram1", 1.0)
	assert.NoError(t, err)

	err = recorder.SetGauge(ctx, "gauge1", 1.0)
	assert.NoError(t, err)

	// Try to create a 4th metric (should fail with error)
	err = recorder.IncrementCounter(ctx, "counter2")
	assert.Error(t, err)
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
	assert.Equal(t, 2, len(cfg.recordHeaders))
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
