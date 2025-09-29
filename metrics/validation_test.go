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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
)

// TestValidation tests the configuration validation
func TestValidation(t *testing.T) {
	t.Parallel()

	t.Run("EmptyServiceName", func(t *testing.T) {
		t.Parallel()
		config := &Config{
			enabled:          true,
			serviceName:      "", // Invalid
			serviceVersion:   "1.0.0",
			provider:         PrometheusProvider,
			metricsPort:      ":9090",
			metricsPath:      "/metrics",
			maxCustomMetrics: 1000,
		}
		err := config.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "service name cannot be empty")
	})

	t.Run("EmptyServiceVersion", func(t *testing.T) {
		t.Parallel()

		config := &Config{
			enabled:          true,
			serviceName:      "test-service",
			serviceVersion:   "", // Invalid
			provider:         PrometheusProvider,
			metricsPort:      ":9090",
			metricsPath:      "/metrics",
			maxCustomMetrics: 1000,
		}
		err := config.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "service version cannot be empty")
	})

	t.Run("InvalidMaxCustomMetrics", func(t *testing.T) {
		t.Parallel()

		config := &Config{
			enabled:          true,
			serviceName:      "test-service",
			serviceVersion:   "1.0.0",
			provider:         PrometheusProvider,
			metricsPort:      ":9090",
			metricsPath:      "/metrics",
			maxCustomMetrics: 0, // Invalid
		}
		err := config.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "maxCustomMetrics must be at least 1")
	})

	t.Run("EmptyMetricsPortForPrometheus", func(t *testing.T) {
		t.Parallel()

		config := &Config{
			enabled:          true,
			serviceName:      "test-service",
			serviceVersion:   "1.0.0",
			provider:         PrometheusProvider,
			metricsPort:      "", // Invalid for Prometheus
			metricsPath:      "/metrics",
			maxCustomMetrics: 1000,
		}
		err := config.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "metrics port cannot be empty")
	})

	t.Run("EmptyMetricsPathForPrometheus", func(t *testing.T) {
		t.Parallel()

		config := &Config{
			enabled:          true,
			serviceName:      "test-service",
			serviceVersion:   "1.0.0",
			provider:         PrometheusProvider,
			metricsPort:      ":9090",
			metricsPath:      "", // Invalid for Prometheus
			maxCustomMetrics: 1000,
		}
		err := config.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "metrics path cannot be empty")
	})

	t.Run("UnsupportedProvider", func(t *testing.T) {
		t.Parallel()

		config := &Config{
			enabled:          true,
			serviceName:      "test-service",
			serviceVersion:   "1.0.0",
			provider:         "invalid", // Invalid
			maxCustomMetrics: 1000,
		}
		err := config.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported metrics provider")
	})

	t.Run("ValidConfiguration", func(t *testing.T) {
		t.Parallel()

		config := &Config{
			enabled:          true,
			serviceName:      "test-service",
			serviceVersion:   "1.0.0",
			provider:         PrometheusProvider,
			metricsPort:      ":9090",
			metricsPath:      "/metrics",
			exportInterval:   30 * time.Second,
			maxCustomMetrics: 1000,
		}
		err := config.validate()
		require.NoError(t, err)
	})
}

// TestCustomMetricsLimitRaceConditionFixed tests that the race condition is properly fixed
func TestCustomMetricsLimitRaceConditionFixed(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":19201"),
		WithMaxCustomMetrics(100), // Moderate limit
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()

	const numGoroutines = 50
	const metricsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Create metrics concurrently
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			for j := range metricsPerGoroutine {
				metricName := fmt.Sprintf("counter_%d_%d", id, j)
				// This should not panic or cause data races
				config.IncrementCounter(ctx, metricName)
			}
		}(i)
	}

	wg.Wait()

	// Get final count
	totalMetrics := atomic.LoadInt64(&config.atomicCustomMetricsCount)

	// Should not exceed the limit
	assert.LessOrEqual(t, int(totalMetrics), 100, "Total metrics should not exceed limit")

	// Should have created some metrics (at least the limit)
	assert.Greater(t, int(totalMetrics), 0, "Should have created some metrics")

	// Check that failures were recorded if limit was hit
	if totalMetrics >= 100 {
		failures := config.getAtomicCustomMetricFailures()
		assert.Greater(t, failures, int64(0), "Should have recorded failures when limit hit")
	}
}

// TestCustomMetricsDoubleCheckRace tests that double-checking doesn't cause races
func TestCustomMetricsDoubleCheckRace(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":19202"),
		WithMaxCustomMetrics(10),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()

	const numGoroutines = 20
	const metricName = "shared_counter"

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Multiple goroutines try to create the same metric
	for range numGoroutines {
		go func() {
			defer wg.Done()
			// All goroutines try to create the same counter
			config.IncrementCounter(ctx, metricName)
		}()
	}

	wg.Wait()

	// Should have created exactly 1 metric (not 20)
	totalMetrics := atomic.LoadInt64(&config.atomicCustomMetricsCount)
	assert.Equal(t, int64(1), totalMetrics, "Should have created exactly one metric")

	// Verify the counter exists and is usable
	counters := config.getAtomicCustomCounters()
	_, exists := counters[metricName]
	assert.True(t, exists, "Counter should exist in the map")
}

// TestShutdownPreventsServerRestart tests that shutdown flag prevents server restart
func TestShutdownPreventsServerRestart(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":19203"),
	)

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown
	ctx := context.Background()
	err := config.Shutdown(ctx)
	require.NoError(t, err)

	// Verify shutdown flag is set
	assert.True(t, config.isShuttingDown.Load(), "Shutdown flag should be set")

	// Try to start server again (should be prevented)
	config.startMetricsServer()

	// Server should not be running
	config.serverMutex.Lock()
	server := config.metricsServer
	config.serverMutex.Unlock()
	assert.Nil(t, server, "Server should not be started after shutdown")
}

// TestContextCancellationInStartRequest tests that StartRequest handles cancelled contexts.
// StartRequest does not check ctx.Done() upfront. The OpenTelemetry SDK handles cancellation
// internally during metric recording. This test verifies that StartRequest doesn't panic
// with cancelled context.
func TestContextCancellationInStartRequest(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":19204"),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// StartRequest should not panic with cancelled context
	// It returns a metrics object; the OTel SDK handles cancellation internally
	result := config.StartRequest(ctx, "/test", false)
	assert.NotNil(t, result, "StartRequest returns metrics object even with cancelled context (OTel SDK handles cancellation)")

	// FinishRequest should also not panic
	config.FinishRequest(ctx, result, 200, 1024, "/test")
}

// TestContextCancellationInCustomMetrics tests that custom metrics respect context cancellation
func TestContextCancellationInCustomMetrics(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":19205"),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// These should not panic or create metrics
	config.RecordMetric(ctx, "test_histogram", 1.5)
	config.IncrementCounter(ctx, "test_counter")
	config.SetGauge(ctx, "test_gauge", 42.0)

	// Verify no metrics were created
	totalMetrics := atomic.LoadInt64(&config.atomicCustomMetricsCount)
	assert.Equal(t, int64(0), totalMetrics, "No metrics should be created with cancelled context")
}

// TestAtomicMapOperationsSafety tests the safety of atomic map operations
func TestAtomicMapOperationsSafety(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":19206"),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()

	// Verify pointer safety
	t.Run("CounterPointerSafety", func(t *testing.T) {
		t.Parallel()

		ptr := atomic.LoadPointer(&config.atomicCustomCounters)
		require.NotNil(t, ptr)

		m := *(*map[string]metric.Int64Counter)(ptr)
		require.NotNil(t, m)
	})

	t.Run("HistogramPointerSafety", func(t *testing.T) {
		t.Parallel()

		ptr := atomic.LoadPointer(&config.atomicCustomHistograms)
		require.NotNil(t, ptr)

		m := *(*map[string]metric.Float64Histogram)(ptr)
		require.NotNil(t, m)
	})

	t.Run("GaugePointerSafety", func(t *testing.T) {
		t.Parallel()

		ptr := atomic.LoadPointer(&config.atomicCustomGauges)
		require.NotNil(t, ptr)

		m := *(*map[string]metric.Float64Gauge)(ptr)
		require.NotNil(t, m)
	})

	// Test concurrent reads and writes
	t.Run("ConcurrentReadsWrites", func(t *testing.T) {
		t.Parallel()

		var wg sync.WaitGroup
		const numReaders = 10
		const numWriters = 10

		// Readers
		wg.Add(numReaders)
		for range numReaders {
			go func() {
				defer wg.Done()
				for j := range 100 {
					_ = config.getAtomicCustomCounters()
					_ = config.getAtomicCustomHistograms()
					_ = config.getAtomicCustomGauges()
					time.Sleep(time.Microsecond * time.Duration(j%10))
				}
			}()
		}

		// Writers
		wg.Add(numWriters)
		for i := range numWriters {
			go func(id int) {
				defer wg.Done()
				for j := range 10 {
					metricName := fmt.Sprintf("metric_%d_%d", id, j)
					config.IncrementCounter(ctx, metricName)
				}
			}(i)
		}

		wg.Wait()
	})
}

// TestMetricsCreationErrorHandling tests error handling during metric creation
func TestMetricsCreationErrorHandling(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":19207"),
		WithMaxCustomMetrics(5),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()

	// Create metrics up to the limit
	for i := range 5 {
		config.IncrementCounter(ctx, fmt.Sprintf("counter_%d", i))
	}

	// Verify we're at the limit
	totalMetrics := atomic.LoadInt64(&config.atomicCustomMetricsCount)
	assert.Equal(t, int64(5), totalMetrics)

	// Try to create one more (should fail)
	config.IncrementCounter(ctx, "counter_overflow")

	// Should still be at limit
	totalMetrics = atomic.LoadInt64(&config.atomicCustomMetricsCount)
	assert.Equal(t, int64(5), totalMetrics, "Should not exceed limit")

	// Verify failure was recorded
	failures := config.getAtomicCustomMetricFailures()
	assert.Greater(t, failures, int64(0), "Should have recorded failure")
}

// TestMetricNameValidation tests metric name validation including reserved prefixes
func TestMetricNameValidation(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":19210"),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()

	tests := []struct {
		name          string
		metricName    string
		shouldError   bool
		errorContains string
	}{
		{
			name:        "ValidName",
			metricName:  "my_custom_metric",
			shouldError: false,
		},
		{
			name:        "ValidWithDots",
			metricName:  "my.custom.metric",
			shouldError: false,
		},
		{
			name:        "ValidWithHyphens",
			metricName:  "my-custom-metric",
			shouldError: false,
		},
		{
			name:          "EmptyName",
			metricName:    "",
			shouldError:   true,
			errorContains: "cannot be empty",
		},
		{
			name:          "StartsWithNumber",
			metricName:    "1_invalid",
			shouldError:   true,
			errorContains: "must start with letter",
		},
		{
			name:          "ReservedPrometheusPrefix",
			metricName:    "__prometheus_internal",
			shouldError:   true,
			errorContains: "reserved prefix '__'",
		},
		{
			name:          "ReservedHTTPPrefix",
			metricName:    "http_custom_metric",
			shouldError:   true,
			errorContains: "reserved prefix 'http_'",
		},
		{
			name:          "ReservedRouterPrefix",
			metricName:    "router_my_metric",
			shouldError:   true,
			errorContains: "reserved prefix 'router_'",
		},
		{
			name:          "TooLongName",
			metricName:    string(make([]byte, 256)),
			shouldError:   true,
			errorContains: "too long",
		},
		{
			name:          "InvalidCharacters",
			metricName:    "my@invalid#metric",
			shouldError:   true,
			errorContains: "must start with letter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: Cannot use t.Parallel() here because test cases share the same config
			// instance and check a shared atomic failure counter, which would cause race conditions.

			// Try to increment a counter with this name
			// It will validate internally
			initialFailures := config.getAtomicCustomMetricFailures()
			config.IncrementCounter(ctx, tt.metricName)
			newFailures := config.getAtomicCustomMetricFailures()

			if tt.shouldError {
				// Should have recorded a failure
				assert.Greater(t, newFailures, initialFailures,
					"Should have recorded failure for invalid metric name: %s", tt.metricName)
			} else {
				// Should not have recorded a failure
				assert.Equal(t, initialFailures, newFailures,
					"Should not have recorded failure for valid metric name: %s", tt.metricName)
			}
		})
	}
}

// TestCASRetriesMetric tests that CAS retries are properly tracked
func TestCASRetriesMetric(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
		WithPort(":19209"),
		WithMaxCustomMetrics(50),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()

	// Create many metrics concurrently to induce some CAS retries
	const numGoroutines = 20
	const metricsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := range numGoroutines {
		go func(_ int) {
			defer wg.Done()
			for j := range metricsPerGoroutine {
				// Use shared metric names to increase contention
				metricName := fmt.Sprintf("shared_metric_%d", j%5)
				config.IncrementCounter(ctx, metricName)
			}
		}(i)
	}

	wg.Wait()

	// Get CAS retry count
	retries := config.getAtomicCASRetries()

	// Under high concurrency, we expect some retries (not necessarily zero)
	// But we don't assert a specific value because it's timing-dependent
	t.Logf("CAS retries observed: %d", retries)

	// The metric should exist
	assert.NotNil(t, config.casRetriesCounter, "CAS retries counter should be initialized")
}
