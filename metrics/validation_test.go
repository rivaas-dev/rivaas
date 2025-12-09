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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidation tests the configuration validation
func TestValidation(t *testing.T) {
	t.Parallel()

	t.Run("ConflictingProviders", func(t *testing.T) {
		t.Parallel()

		// Test WithPrometheus + WithStdout
		_, err := New(
			WithPrometheus(":9090", "/metrics"),
			WithStdout(),
			WithServiceName("test"),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflicting provider options")

		// Test WithOTLP + WithPrometheus
		_, err = New(
			WithOTLP("http://localhost:4318"),
			WithPrometheus(":9090", "/metrics"),
			WithServiceName("test"),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflicting provider options")

		// Test WithStdout + WithOTLP
		_, err = New(
			WithStdout(),
			WithOTLP("http://localhost:4318"),
			WithServiceName("test"),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflicting provider options")
	})

	t.Run("SingleProviderAllowed", func(t *testing.T) {
		t.Parallel()

		// Single provider should work
		recorder, err := New(
			WithPrometheus(":19500", "/metrics"),
			WithServiceName("test"),
			WithServerDisabled(),
		)
		require.NoError(t, err)
		assert.Equal(t, PrometheusProvider, recorder.Provider())
		recorder.Shutdown(t.Context())

		// WithStdout alone should work
		recorder, err = New(
			WithStdout(),
			WithServiceName("test"),
		)
		require.NoError(t, err)
		assert.Equal(t, StdoutProvider, recorder.Provider())
		recorder.Shutdown(t.Context())
	})

	t.Run("EmptyServiceName", func(t *testing.T) {
		t.Parallel()
		recorder := &Recorder{
			enabled:          true,
			serviceName:      "", // Invalid
			serviceVersion:   "1.0.0",
			provider:         PrometheusProvider,
			metricsPort:      ":9090",
			metricsPath:      "/metrics",
			maxCustomMetrics: 1000,
		}
		err := recorder.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "service name cannot be empty")
	})

	t.Run("EmptyServiceVersion", func(t *testing.T) {
		t.Parallel()

		recorder := &Recorder{
			enabled:          true,
			serviceName:      "test-service",
			serviceVersion:   "", // Invalid
			provider:         PrometheusProvider,
			metricsPort:      ":9090",
			metricsPath:      "/metrics",
			maxCustomMetrics: 1000,
		}
		err := recorder.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "service version cannot be empty")
	})

	t.Run("InvalidMaxCustomMetrics", func(t *testing.T) {
		t.Parallel()

		recorder := &Recorder{
			enabled:          true,
			serviceName:      "test-service",
			serviceVersion:   "1.0.0",
			provider:         PrometheusProvider,
			metricsPort:      ":9090",
			metricsPath:      "/metrics",
			maxCustomMetrics: 0, // Invalid
		}
		err := recorder.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "maxCustomMetrics must be at least 1")
	})

	t.Run("EmptyMetricsPortForPrometheus", func(t *testing.T) {
		t.Parallel()

		recorder := &Recorder{
			enabled:          true,
			serviceName:      "test-service",
			serviceVersion:   "1.0.0",
			provider:         PrometheusProvider,
			metricsPort:      "", // Invalid for Prometheus
			metricsPath:      "/metrics",
			maxCustomMetrics: 1000,
		}
		err := recorder.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "metrics port cannot be empty")
	})

	t.Run("EmptyMetricsPathForPrometheus", func(t *testing.T) {
		t.Parallel()

		recorder := &Recorder{
			enabled:          true,
			serviceName:      "test-service",
			serviceVersion:   "1.0.0",
			provider:         PrometheusProvider,
			metricsPort:      ":9090",
			metricsPath:      "", // Invalid for Prometheus
			maxCustomMetrics: 1000,
		}
		err := recorder.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "metrics path cannot be empty")
	})

	t.Run("UnsupportedProvider", func(t *testing.T) {
		t.Parallel()

		recorder := &Recorder{
			enabled:          true,
			serviceName:      "test-service",
			serviceVersion:   "1.0.0",
			provider:         "invalid", // Invalid
			maxCustomMetrics: 1000,
		}
		err := recorder.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported metrics provider")
	})

	t.Run("ValidConfiguration", func(t *testing.T) {
		t.Parallel()

		recorder := &Recorder{
			enabled:          true,
			serviceName:      "test-service",
			serviceVersion:   "1.0.0",
			provider:         PrometheusProvider,
			metricsPort:      ":9090",
			metricsPath:      "/metrics",
			exportInterval:   30 * time.Second,
			maxCustomMetrics: 1000,
		}
		err := recorder.validate()
		require.NoError(t, err)
	})
}

// TestCustomMetricsLimitRaceConditionFixed tests that the race condition is properly fixed
func TestCustomMetricsLimitRaceConditionFixed(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithPrometheus(":19201", "/metrics"),
		WithServiceName("test-service"),
		WithMaxCustomMetrics(100), // Moderate limit
		WithServerDisabled(),
	)
	t.Cleanup(func() {
		recorder.Shutdown(t.Context())
	})

	ctx := t.Context()

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
				_ = recorder.IncrementCounter(ctx, metricName)
			}
		}(i)
	}

	wg.Wait()

	// Get final count
	totalMetrics := recorder.CustomMetricCount()

	// Should not exceed the limit
	assert.LessOrEqual(t, totalMetrics, 100, "Total metrics should not exceed limit")

	// Should have created some metrics (at least the limit)
	assert.Positive(t, totalMetrics, "Should have created some metrics")

	// Check that failures were recorded if limit was hit
	if totalMetrics >= 100 {
		failures := recorder.getAtomicCustomMetricFailures()
		assert.Positive(t, failures, "Should have recorded failures when limit hit")
	}
}

// TestCustomMetricsDoubleCheckRace tests that double-checking doesn't cause races
func TestCustomMetricsDoubleCheckRace(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithPrometheus(":19202", "/metrics"),
		WithServiceName("test-service"),
		WithMaxCustomMetrics(10),
		WithServerDisabled(),
	)
	t.Cleanup(func() {
		recorder.Shutdown(t.Context())
	})

	ctx := t.Context()

	const numGoroutines = 20
	const metricName = "shared_counter"

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Multiple goroutines try to create the same metric
	for range numGoroutines {
		go func() {
			defer wg.Done()
			// All goroutines try to create the same counter
			_ = recorder.IncrementCounter(ctx, metricName)
		}()
	}

	wg.Wait()

	// Should have created exactly 1 metric (not 20)
	totalMetrics := recorder.CustomMetricCount()
	assert.Equal(t, 1, totalMetrics, "Should have created exactly one metric")
}

// TestShutdownPreventsServerRestart tests that shutdown flag prevents server restart
func TestShutdownPreventsServerRestart(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithPrometheus(":19203", "/metrics"),
		WithServiceName("test-service"),
	)

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown
	ctx := t.Context()
	err := recorder.Shutdown(ctx)
	require.NoError(t, err)

	// Verify shutdown flag is set
	assert.True(t, recorder.isShuttingDown.Load(), "Shutdown flag should be set")

	// Try to start server again (should be prevented)
	recorder.startMetricsServer(t.Context())

	// Server should not be running
	recorder.serverMutex.Lock()
	server := recorder.metricsServer
	recorder.serverMutex.Unlock()
	assert.Nil(t, server, "Server should not be started after shutdown")
}

// TestContextCancellationInStart tests that Start handles canceled contexts.
// Start does not check ctx.Done() upfront. The OpenTelemetry SDK handles cancellation
// internally during metric recording. This test verifies that Start doesn't panic
// with canceled context.
func TestContextCancellationInStart(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithPrometheus(":19204", "/metrics"),
		WithServiceName("test-service"),
		WithServerDisabled(),
	)
	t.Cleanup(func() {
		recorder.Shutdown(t.Context())
	})

	// Create canceled context
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	// Start should not panic with canceled context
	// It returns a metrics object; the OTel SDK handles cancellation internally
	result := recorder.BeginRequest(ctx)
	assert.NotNil(t, result, "Start returns metrics object even with canceled context (OTel SDK handles cancellation)")

	// Finish should also not panic
	recorder.Finish(ctx, result, 200, 1024, "/test")
}

// TestContextCancellationInCustomMetrics tests that custom metrics returns error with canceled context
func TestContextCancellationInCustomMetrics(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithPrometheus(":19205", "/metrics"),
		WithServiceName("test-service"),
		WithServerDisabled(),
	)
	t.Cleanup(func() {
		recorder.Shutdown(t.Context())
	})

	// Create canceled context
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	// These should not panic - they will still record (OTel handles cancellation)
	_ = recorder.RecordHistogram(ctx, "test_histogram", 1.5)
	_ = recorder.IncrementCounter(ctx, "test_counter")
	_ = recorder.SetGauge(ctx, "test_gauge", 42.0)
}

// TestRWMutexOperationsSafety tests the safety of RWMutex operations
func TestRWMutexOperationsSafety(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithPrometheus(":19206", "/metrics"),
		WithServiceName("test-service"),
		WithServerDisabled(),
	)
	t.Cleanup(func() {
		recorder.Shutdown(t.Context())
	})

	ctx := t.Context()

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
					_ = recorder.CustomMetricCount()
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
					_ = recorder.IncrementCounter(ctx, metricName)
				}
			}(i)
		}

		wg.Wait()
	})
}

// TestMetricsCreationErrorHandling tests error handling during metric creation
func TestMetricsCreationErrorHandling(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithPrometheus(":19207", "/metrics"),
		WithServiceName("test-service"),
		WithMaxCustomMetrics(5),
		WithServerDisabled(),
	)
	t.Cleanup(func() {
		recorder.Shutdown(t.Context())
	})

	ctx := t.Context()

	// Create metrics up to the limit
	for i := range 5 {
		err := recorder.IncrementCounter(ctx, fmt.Sprintf("counter_%d", i))
		require.NoError(t, err)
	}

	// Verify we're at the limit
	totalMetrics := recorder.CustomMetricCount()
	assert.Equal(t, 5, totalMetrics)

	// Try to create one more (should fail with error)
	err := recorder.IncrementCounter(ctx, "counter_overflow")
	require.Error(t, err)

	// Should still be at limit
	totalMetrics = recorder.CustomMetricCount()
	assert.Equal(t, 5, totalMetrics, "Should not exceed limit")

	// Verify failure was recorded
	failures := recorder.getAtomicCustomMetricFailures()
	assert.Positive(t, failures, "Should have recorded failure")
}

// TestMetricNameValidation tests metric name validation including reserved prefixes
//
//nolint:paralleltest,tparallel // Subtests share recorder state; parallel execution would cause race conditions
func TestMetricNameValidation(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithPrometheus(":19210", "/metrics"),
		WithServiceName("test-service"),
		WithServerDisabled(),
	)
	t.Cleanup(func() {
		recorder.Shutdown(t.Context())
	})

	ctx := t.Context()

	tests := []struct {
		name        string
		metricName  string
		shouldError bool
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
			name:        "EmptyName",
			metricName:  "",
			shouldError: true,
		},
		{
			name:        "StartsWithNumber",
			metricName:  "1_invalid",
			shouldError: true,
		},
		{
			name:        "ReservedPrometheusPrefix",
			metricName:  "__prometheus_internal",
			shouldError: true,
		},
		{
			name:        "ReservedHTTPPrefix",
			metricName:  "http_custom_metric",
			shouldError: true,
		},
		{
			name:        "ReservedRouterPrefix",
			metricName:  "router_my_metric",
			shouldError: true,
		},
		{
			name:        "TooLongName",
			metricName:  string(make([]byte, 256)),
			shouldError: true,
		},
		{
			name:        "InvalidCharacters",
			metricName:  "my@invalid#metric",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: Cannot use t.Parallel() here because test cases share the same recorder
			// instance and check a shared atomic failure counter, which would cause race conditions.

			err := recorder.IncrementCounter(ctx, tt.metricName)

			if tt.shouldError {
				assert.Error(t, err, "Should return error for invalid metric name: %s", tt.metricName)
			} else {
				assert.NoError(t, err, "Should not return error for valid metric name: %s", tt.metricName)
			}
		})
	}
}

// =============================================================================
// Error Message Quality Tests
// =============================================================================
// These tests ensure error messages are descriptive and helpful for debugging.

// TestErrorMessages_AreDescriptive verifies that error messages contain
// relevant information for debugging.
func TestErrorMessages_AreDescriptive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setup          func() error
		wantSubstrings []string
	}{
		{
			name: "EmptyMetricName",
			setup: func() error {
				return validateMetricName("")
			},
			wantSubstrings: []string{"empty"},
		},
		{
			name: "MetricNameTooLong",
			setup: func() error {
				return validateMetricName(string(make([]byte, 300)))
			},
			wantSubstrings: []string{"too long", "255"},
		},
		{
			name: "InvalidMetricNameFormat",
			setup: func() error {
				return validateMetricName("123invalid")
			},
			wantSubstrings: []string{"invalid", "123invalid", "letter"},
		},
		{
			name: "ReservedPrefixHTTP",
			setup: func() error {
				return validateMetricName("http_custom_metric")
			},
			wantSubstrings: []string{"reserved", "http_"},
		},
		{
			name: "ReservedPrefixRouter",
			setup: func() error {
				return validateMetricName("router_my_metric")
			},
			wantSubstrings: []string{"reserved", "router_"},
		},
		{
			name: "ReservedPrefixPrometheus",
			setup: func() error {
				// Note: Names starting with __ fail regex before reserved check
				// because they don't start with a letter
				return validateMetricName("__prometheus_internal")
			},
			wantSubstrings: []string{"invalid", "__prometheus_internal"},
		},
		{
			name: "InvalidCharacters",
			setup: func() error {
				return validateMetricName("metric@invalid")
			},
			wantSubstrings: []string{"invalid", "metric@invalid"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.setup()
			require.Error(t, err, "Expected error for %s", tt.name)

			errMsg := err.Error()
			for _, substr := range tt.wantSubstrings {
				assert.Contains(t, errMsg, substr,
					"Error message should contain %q, got: %s", substr, errMsg)
			}
		})
	}
}

// TestErrorMessages_MetricLimitReached verifies that the limit error message
// contains useful debugging information.
func TestErrorMessages_MetricLimitReached(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithStdout(),
		WithServiceName("error-message-test"),
		WithMaxCustomMetrics(3),
		WithServerDisabled(),
	)
	t.Cleanup(func() { recorder.Shutdown(t.Context()) })

	ctx := t.Context()

	// Fill up the limit
	for i := range 3 {
		err := recorder.IncrementCounter(ctx, fmt.Sprintf("counter_%d", i))
		require.NoError(t, err)
	}

	// Try to create one more
	err := recorder.IncrementCounter(ctx, "overflow_counter")
	require.Error(t, err)

	errMsg := err.Error()

	// Error should contain:
	// - The metric name that failed
	assert.Contains(t, errMsg, "overflow_counter",
		"Error should contain the metric name that failed")
	// - The word "limit"
	assert.Contains(t, errMsg, "limit",
		"Error should indicate it's a limit issue")
	// - Current count or limit value
	assert.Contains(t, errMsg, "3",
		"Error should contain the limit value")
}

// TestErrorMessages_ProviderConflict verifies that provider conflict errors
// are clear about which options conflict.
func TestErrorMessages_ProviderConflict(t *testing.T) {
	t.Parallel()

	_, err := New(
		WithPrometheus(":9090", "/metrics"),
		WithStdout(),
		WithServiceName("conflict-test"),
	)
	require.Error(t, err)

	errMsg := err.Error()
	assert.Contains(t, errMsg, "conflicting",
		"Error should indicate options are conflicting")
	assert.Contains(t, errMsg, "provider",
		"Error should mention 'provider'")
}

// TestErrorMessages_ValidationErrors verifies that validation errors are clear.
func TestErrorMessages_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		recorder       *Recorder
		wantSubstrings []string
	}{
		{
			name: "EmptyServiceName",
			recorder: &Recorder{
				enabled:          true,
				serviceName:      "",
				serviceVersion:   "1.0.0",
				provider:         PrometheusProvider,
				metricsPort:      ":9090",
				metricsPath:      "/metrics",
				maxCustomMetrics: 1000,
			},
			wantSubstrings: []string{"service name", "empty"},
		},
		{
			name: "EmptyServiceVersion",
			recorder: &Recorder{
				enabled:          true,
				serviceName:      "test-service",
				serviceVersion:   "",
				provider:         PrometheusProvider,
				metricsPort:      ":9090",
				metricsPath:      "/metrics",
				maxCustomMetrics: 1000,
			},
			wantSubstrings: []string{"service version", "empty"},
		},
		{
			name: "InvalidMaxCustomMetrics",
			recorder: &Recorder{
				enabled:          true,
				serviceName:      "test-service",
				serviceVersion:   "1.0.0",
				provider:         PrometheusProvider,
				metricsPort:      ":9090",
				metricsPath:      "/metrics",
				maxCustomMetrics: 0,
			},
			wantSubstrings: []string{"maxCustomMetrics", "at least 1"},
		},
		{
			name: "EmptyMetricsPort",
			recorder: &Recorder{
				enabled:          true,
				serviceName:      "test-service",
				serviceVersion:   "1.0.0",
				provider:         PrometheusProvider,
				metricsPort:      "",
				metricsPath:      "/metrics",
				maxCustomMetrics: 1000,
			},
			wantSubstrings: []string{"metrics port", "empty"},
		},
		{
			name: "EmptyMetricsPath",
			recorder: &Recorder{
				enabled:          true,
				serviceName:      "test-service",
				serviceVersion:   "1.0.0",
				provider:         PrometheusProvider,
				metricsPort:      ":9090",
				metricsPath:      "",
				maxCustomMetrics: 1000,
			},
			wantSubstrings: []string{"metrics path", "empty"},
		},
		{
			name: "UnsupportedProvider",
			recorder: &Recorder{
				enabled:          true,
				serviceName:      "test-service",
				serviceVersion:   "1.0.0",
				provider:         "invalid_provider",
				maxCustomMetrics: 1000,
			},
			wantSubstrings: []string{"unsupported", "provider", "invalid_provider"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.recorder.validate()
			require.Error(t, err, "Expected validation error for %s", tt.name)

			errMsg := err.Error()
			for _, substr := range tt.wantSubstrings {
				assert.Contains(t, errMsg, substr,
					"Error message should contain %q, got: %s", substr, errMsg)
			}
		})
	}
}

// TestErrorMessages_HandlerNotAvailable verifies that Handler() errors are clear.
func TestErrorMessages_HandlerNotAvailable(t *testing.T) {
	t.Parallel()

	t.Run("NotPrometheusProvider", func(t *testing.T) {
		t.Parallel()

		recorder := MustNew(
			WithOTLP("http://localhost:4318"),
			WithServiceName("handler-error-test"),
		)
		t.Cleanup(func() { recorder.Shutdown(t.Context()) })

		_, err := recorder.Handler()
		require.Error(t, err)

		errMsg := err.Error()
		assert.Contains(t, errMsg, "Prometheus",
			"Error should mention Prometheus")
		assert.Contains(t, errMsg, "otlp",
			"Error should mention current provider")
	})

	t.Run("DisabledRecorder", func(t *testing.T) {
		t.Parallel()

		recorder := &Recorder{enabled: false}

		_, err := recorder.Handler()
		require.Error(t, err)

		errMsg := err.Error()
		assert.Contains(t, errMsg, "not enabled",
			"Error should indicate metrics are not enabled")
	})
}
