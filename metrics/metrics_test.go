package metrics

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivaas-dev/rivaas/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitForMetricsServer waits for the metrics server to be ready
func waitForMetricsServer(address string, timeout time.Duration) error {
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

func TestMetricsConfig(t *testing.T) {
	config := MustNew(
		WithServiceName("test-service"),
		WithServiceVersion("v1.0.0"),
		WithPort(":9091"),
		WithStrictPort(), // Require exact port for deterministic test
	)
	defer config.Shutdown(context.Background())

	assert.True(t, config.IsEnabled())
	assert.Equal(t, "test-service", config.GetServiceName())
	assert.Equal(t, "v1.0.0", config.GetServiceVersion())
	assert.Equal(t, ":9091", config.GetServerAddress())
	assert.Equal(t, PrometheusProvider, config.GetProvider())
}

func TestMetricsWithRouter(t *testing.T) {
	// Create metrics config
	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":9092"), // Use unique port to avoid conflicts
	)

	// Wait for server to be ready
	err := waitForMetricsServer("localhost:9092", 1*time.Second)
	require.NoError(t, err, "Metrics server should start")

	// Create router
	r := router.New()
	r.SetMetricsRecorder(config)

	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMetricsProviders(t *testing.T) {
	t.Run("Prometheus", func(t *testing.T) {
		config := MustNew(
			WithProvider(PrometheusProvider),
			WithPort(":9093"),
		)
		assert.Equal(t, PrometheusProvider, config.GetProvider())
	})

	t.Run("OTLP", func(t *testing.T) {
		config := MustNew(
			WithProvider(OTLPProvider),
			WithOTLPEndpoint("http://localhost:4318"),
		)
		assert.Equal(t, OTLPProvider, config.GetProvider())
	})

	t.Run("Stdout", func(t *testing.T) {
		config := MustNew(
			WithProvider(StdoutProvider),
		)
		assert.Equal(t, StdoutProvider, config.GetProvider())
	})
}

func TestCustomMetrics(t *testing.T) {
	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":9094"),
	)

	ctx := context.Background()

	// Test custom metrics recording
	config.RecordMetric(ctx, "test_histogram", 1.5)
	config.IncrementCounter(ctx, "test_counter")
	config.SetGauge(ctx, "test_gauge", 42.0)

	// These should not panic
	assert.True(t, config.IsEnabled())
}

func TestMetricsMiddleware(t *testing.T) {
	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":9095"),
	)

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with metrics middleware
	middleware := Middleware(config)
	wrappedHandler := middleware(handler)

	// Test the wrapped handler
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestMetricsExcludePaths(t *testing.T) {
	config := MustNew(
		WithServiceName("test-service"),
		WithExcludePaths("/health", "/metrics"),
	)

	// Test that excluded paths are properly configured
	// This is an internal test - in real usage, the router would check this
	assert.True(t, config.IsEnabled())
}

func TestMetricsOptions(t *testing.T) {
	t.Run("WithHeaders", func(t *testing.T) {
		config := MustNew(
			WithHeaders("Authorization", "X-Request-ID"),
		)
		assert.True(t, config.IsEnabled())
	})

	t.Run("WithDisableParams", func(t *testing.T) {
		config := MustNew(
			WithDisableParams(),
		)
		assert.True(t, config.IsEnabled())
	})

	t.Run("WithMaxCustomMetrics", func(t *testing.T) {
		config := MustNew(
			WithMaxCustomMetrics(500),
		)
		assert.True(t, config.IsEnabled())
	})

	t.Run("WithServerDisabled", func(t *testing.T) {
		config := MustNew(
			WithServerDisabled(),
		)
		assert.True(t, config.IsEnabled())
		assert.Equal(t, "", config.GetServerAddress())
	})
}

func TestMetricsIntegration(t *testing.T) {
	// Test full integration with router
	config := MustNew(
		WithServiceName("integration-test"),
		WithPort(":9096"),
		WithExcludePaths("/health"),
	)

	r := router.New()
	r.SetMetricsRecorder(config)

	// Add routes
	r.GET("/", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "Hello"})
	})

	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})

	// Test normal route
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Test health route (should be excluded from metrics)
	req = httptest.NewRequest("GET", "/health", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestMetricsHandler(t *testing.T) {
	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":9097"),
	)

	// Create a router with metrics to generate some data
	r := router.New()
	r.SetMetricsRecorder(config)

	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Make a request to generate metrics
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Now test the metrics handler
	handler, err := config.GetHandler()
	require.NoError(t, err)
	require.NotNil(t, handler)

	// Test that the handler responds
	req = httptest.NewRequest("GET", "/metrics", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Should return 200 and contain some metrics
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "http_requests_total")
}

func TestGetHandlerErrors(t *testing.T) {
	t.Run("ErrorWhenNotPrometheusProvider", func(t *testing.T) {
		config := MustNew(
			WithServiceName("test-service"),
			WithProvider(OTLPProvider),
			WithOTLPEndpoint("http://localhost:4318"),
		)
		defer config.Shutdown(context.Background())

		handler, err := config.GetHandler()
		assert.Error(t, err)
		assert.Nil(t, handler)
		assert.Contains(t, err.Error(), "only available with Prometheus provider")
		assert.Contains(t, err.Error(), "otlp")
	})

	t.Run("ErrorWhenStdoutProvider", func(t *testing.T) {
		config := MustNew(
			WithServiceName("test-service"),
			WithProvider(StdoutProvider),
		)
		defer config.Shutdown(context.Background())

		handler, err := config.GetHandler()
		assert.Error(t, err)
		assert.Nil(t, handler)
		assert.Contains(t, err.Error(), "only available with Prometheus provider")
		assert.Contains(t, err.Error(), "stdout")
	})
}

func TestRecordContextPoolMetrics(t *testing.T) {
	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":9101"),
	)

	ctx := context.Background()

	// Record some pool hits and misses
	config.RecordContextPoolHit(ctx)
	config.RecordContextPoolHit(ctx)
	config.RecordContextPoolMiss(ctx)

	// Verify atomic counters were updated
	assert.Equal(t, int64(2), config.getAtomicContextPoolHits())
	assert.Equal(t, int64(1), config.getAtomicContextPoolMisses())
}

func TestRecordConstraintFailure(t *testing.T) {
	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":9102"),
	)

	ctx := context.Background()

	// Record constraint failures
	config.RecordConstraintFailure(ctx, "regex")
	config.RecordConstraintFailure(ctx, "int")

	// Should not panic
	assert.True(t, config.IsEnabled())
}

func TestShutdown(t *testing.T) {
	t.Run("Prometheus", func(t *testing.T) {
		config := MustNew(
			WithServiceName("test-service"),
			WithProvider(PrometheusProvider),
			WithPort(":9103"),
		)

		// Wait for server to be ready
		err := waitForMetricsServer("localhost:9103", 1*time.Second)
		require.NoError(t, err, "Metrics server should start")

		// Shutdown should not error
		ctx := context.Background()
		err = config.Shutdown(ctx)
		assert.NoError(t, err)
	})

	t.Run("OTLP", func(t *testing.T) {
		config := MustNew(
			WithServiceName("test-service"),
			WithProvider(OTLPProvider),
			WithOTLPEndpoint("http://localhost:4318"),
		)

		// Shutdown may error if OTLP collector is not running (expected in tests)
		ctx := context.Background()
		err := config.Shutdown(ctx)
		// We don't assert no error here because OTLP requires a running collector
		// The important thing is that Shutdown() doesn't panic
		_ = err
	})

	t.Run("Stdout", func(t *testing.T) {
		config := MustNew(
			WithServiceName("test-service"),
			WithProvider(StdoutProvider),
		)

		// Shutdown should not error
		ctx := context.Background()
		err := config.Shutdown(ctx)
		assert.NoError(t, err)
	})

	t.Run("IdempotentShutdown", func(t *testing.T) {
		config := MustNew(
			WithServiceName("test-service"),
			WithProvider(PrometheusProvider),
			WithPort(":9106"),
		)

		// Wait for server to be ready
		err := waitForMetricsServer("localhost:9106", 1*time.Second)
		require.NoError(t, err, "Metrics server should start")

		ctx := context.Background()

		// First shutdown
		err = config.Shutdown(ctx)
		assert.NoError(t, err)

		// Second shutdown should also succeed (idempotent)
		err = config.Shutdown(ctx)
		assert.NoError(t, err)

		// Third shutdown for good measure
		err = config.Shutdown(ctx)
		assert.NoError(t, err)

		// Verify shutdown flag is still true
		assert.True(t, config.isShuttingDown.Load())
	})
}

func TestCustomMetricsLimitRaceCondition(t *testing.T) {
	// Test that the limit is enforced correctly under concurrent access
	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":9104"),
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
				config.IncrementCounter(ctx, metricName)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for range numGoroutines {
		<-done
	}

	// Get total metrics count from atomic counter
	totalMetrics := atomic.LoadInt64(&config.atomicCustomMetricsCount)

	// Should not exceed the limit
	assert.LessOrEqual(t, int(totalMetrics), 10, "Total metrics should not exceed limit")

	// Should have created some metrics (not zero)
	assert.Greater(t, int(totalMetrics), 0, "Should have created some metrics")
}

func TestNewReturnsError(t *testing.T) {
	// Test that New() returns errors properly
	config, err := New(
		WithServiceName("test-service"),
		WithProvider(PrometheusProvider),
		WithPort(":9100"),
	)
	require.NoError(t, err)
	require.NotNil(t, config)
	assert.True(t, config.IsEnabled())

	// Shutdown the config
	ctx := context.Background()
	err = config.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestMustNewPanics(t *testing.T) {
	// Test that MustNew panics on error
	// We can't easily test this without creating an invalid config
	// Just verify it works normally
	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":9099"),
	)
	require.NotNil(t, config)
	assert.True(t, config.IsEnabled())

	// Shutdown the config
	ctx := context.Background()
	err := config.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestCustomMetricsLimitEnforcement(t *testing.T) {
	// Test that limit is enforced and errors are recorded
	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":9105"),
		WithMaxCustomMetrics(3),
	)

	ctx := context.Background()

	// Create 3 metrics (should succeed)
	config.IncrementCounter(ctx, "counter1")
	config.RecordMetric(ctx, "histogram1", 1.0)
	config.SetGauge(ctx, "gauge1", 1.0)

	// Try to create a 4th metric (should fail silently)
	config.IncrementCounter(ctx, "counter2")

	// Verify we have exactly 3 metrics
	totalMetrics := atomic.LoadInt64(&config.atomicCustomMetricsCount)
	assert.Equal(t, int64(3), totalMetrics, "Should have exactly 3 metrics")

	// Verify failure was recorded
	failures := config.getAtomicCustomMetricFailures()
	assert.Greater(t, failures, int64(0), "Should have recorded at least one failure")
}
