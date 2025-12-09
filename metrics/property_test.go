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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProperty_MetricCountNeverExceedsLimit verifies the invariant that
// the custom metric count never exceeds the configured limit, even under
// high concurrent load.
func TestProperty_MetricCountNeverExceedsLimit(t *testing.T) {
	t.Parallel()

	const limit = 50
	const numGoroutines = 100
	const metricsPerGoroutine = 20

	recorder := MustNew(
		WithStdout(),
		WithServiceName("property-test"),
		WithMaxCustomMetrics(limit),
		WithServerDisabled(),
	)
	t.Cleanup(func() { recorder.Shutdown(context.Background()) })

	ctx := t.Context()
	var violations atomic.Int64

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			for j := range metricsPerGoroutine {
				metricName := fmt.Sprintf("metric_%d_%d", id, j)
				_ = recorder.IncrementCounter(ctx, metricName)

				// PROPERTY: Count should NEVER exceed limit
				count := recorder.CustomMetricCount()
				if count > limit {
					violations.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()

	// Final verification
	finalCount := recorder.CustomMetricCount()
	assert.LessOrEqual(t, finalCount, limit,
		"Final metric count %d exceeds limit %d", finalCount, limit)
	assert.Zero(t, violations.Load(),
		"Detected %d violations during concurrent metric creation", violations.Load())
}

// TestProperty_SameMetricNameReturnsSameInstance verifies the invariant that
// requesting the same metric name multiple times returns the same instance
// (idempotent creation).
func TestProperty_SameMetricNameReturnsSameInstance(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithStdout(),
		WithServiceName("idempotent-test"),
		WithServerDisabled(),
	)
	t.Cleanup(func() { recorder.Shutdown(context.Background()) })

	ctx := t.Context()
	const metricName = "shared_counter"
	const numGoroutines = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// All goroutines try to create the same metric concurrently
	for range numGoroutines {
		go func() {
			defer wg.Done()
			_ = recorder.IncrementCounter(ctx, metricName)
		}()
	}

	wg.Wait()

	// PROPERTY: Only one metric should be created despite concurrent attempts
	count := recorder.CustomMetricCount()
	assert.Equal(t, 1, count,
		"Should have exactly 1 metric, but got %d", count)
}

// TestProperty_FailuresAreTracked verifies the invariant that all metric
// creation failures are tracked in the failure counter.
func TestProperty_FailuresAreTracked(t *testing.T) {
	t.Parallel()

	const limit = 5
	const totalAttempts = 20

	recorder := MustNew(
		WithStdout(),
		WithServiceName("failure-tracking-test"),
		WithMaxCustomMetrics(limit),
		WithServerDisabled(),
	)
	t.Cleanup(func() { recorder.Shutdown(context.Background()) })

	ctx := t.Context()

	// Create metrics until limit, then continue to trigger failures
	var successCount int
	var failureCount int
	for i := range totalAttempts {
		err := recorder.IncrementCounter(ctx, fmt.Sprintf("counter_%d", i))
		if err != nil {
			failureCount++
		} else {
			successCount++
		}
	}

	// PROPERTY: success + failure should equal total attempts
	assert.Equal(t, totalAttempts, successCount+failureCount,
		"success (%d) + failure (%d) should equal total attempts (%d)",
		successCount, failureCount, totalAttempts)

	// PROPERTY: success count should equal limit (we created unique metrics)
	assert.Equal(t, limit, successCount,
		"should have created exactly %d metrics, got %d", limit, successCount)

	// PROPERTY: failure count should equal attempts beyond limit
	expectedFailures := totalAttempts - limit
	assert.Equal(t, expectedFailures, failureCount,
		"should have %d failures, got %d", expectedFailures, failureCount)

	// PROPERTY: atomic failure counter should match our counted failures
	atomicFailures := recorder.getAtomicCustomMetricFailures()
	assert.Equal(t, int64(failureCount), atomicFailures,
		"atomic failures (%d) should match counted failures (%d)",
		atomicFailures, failureCount)
}

// TestProperty_ShutdownIsIdempotent verifies the invariant that
// calling Shutdown multiple times is safe and doesn't cause errors.
func TestProperty_ShutdownIsIdempotent(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithStdout(),
		WithServiceName("idempotent-shutdown-test"),
		WithServerDisabled(),
	)

	ctx := t.Context()

	// Call shutdown multiple times from multiple goroutines
	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errors := make(chan error, numGoroutines)

	for range numGoroutines {
		go func() {
			defer wg.Done()
			err := recorder.Shutdown(ctx)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// PROPERTY: All shutdown calls should succeed
	var errorCount int
	for err := range errors {
		t.Logf("Shutdown error: %v", err)
		errorCount++
	}
	assert.Zero(t, errorCount, "Shutdown should be idempotent and not return errors")
}

// TestProperty_DisabledRecorderNeverRecords verifies the invariant that
// a disabled recorder never records metrics and always returns nil/no-error.
func TestProperty_DisabledRecorderNeverRecords(t *testing.T) {
	t.Parallel()

	// Create a disabled recorder
	recorder := &Recorder{
		enabled: false,
	}

	ctx := t.Context()

	// All metric operations should be no-ops
	err := recorder.IncrementCounter(ctx, "test_counter")
	require.NoError(t, err, "IncrementCounter on disabled recorder should not error")

	err = recorder.RecordHistogram(ctx, "test_histogram", 1.0)
	require.NoError(t, err, "RecordHistogram on disabled recorder should not error")

	err = recorder.SetGauge(ctx, "test_gauge", 1.0)
	require.NoError(t, err, "SetGauge on disabled recorder should not error")

	// Start should return nil
	result := recorder.BeginRequest(ctx)
	assert.Nil(t, result, "Start on disabled recorder should return nil")

	// Shutdown should succeed
	err = recorder.Shutdown(ctx)
	assert.NoError(t, err, "Shutdown on disabled recorder should not error")
}

// TestProperty_ValidMetricNamesAreAccepted verifies that all metric names
// matching the expected format are accepted.
func TestProperty_ValidMetricNamesAreAccepted(t *testing.T) {
	t.Parallel()

	validNames := []string{
		"a",
		"metric",
		"my_metric",
		"my.metric",
		"my-metric",
		"MyMetric",
		"metric123",
		"my_metric.name-v2",
		"A123_test.value-count",
	}

	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := validateMetricName(name)
			assert.NoError(t, err, "Valid metric name %q should be accepted", name)
		})
	}
}

// TestProperty_InvalidMetricNamesAreRejected verifies that all invalid
// metric name patterns are rejected with appropriate errors.
func TestProperty_InvalidMetricNamesAreRejected(t *testing.T) {
	t.Parallel()

	invalidNames := []struct {
		name        string
		description string
	}{
		{"", "empty name"},
		{"123abc", "starts with number"},
		{"_underscore", "starts with underscore"},
		{"__double_underscore", "reserved prometheus prefix"},
		{"http_custom", "reserved http_ prefix"},
		{"router_custom", "reserved router_ prefix"},
		{"metric@invalid", "contains @ symbol"},
		{"metric#invalid", "contains # symbol"},
		{"metric invalid", "contains space"},
		{"metric\tinvalid", "contains tab"},
	}

	for _, tc := range invalidNames {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			err := validateMetricName(tc.name)
			assert.Error(t, err, "Invalid metric name %q (%s) should be rejected",
				tc.name, tc.description)
		})
	}
}

// TestProperty_MetricLimitEnforcementIsAtomic verifies that limit enforcement
// is atomic and doesn't allow even temporary violations under high contention.
func TestProperty_MetricLimitEnforcementIsAtomic(t *testing.T) {
	t.Parallel()

	const limit = 10
	const numGoroutines = 100

	recorder := MustNew(
		WithStdout(),
		WithServiceName("atomic-limit-test"),
		WithMaxCustomMetrics(limit),
		WithServerDisabled(),
	)
	t.Cleanup(func() { recorder.Shutdown(context.Background()) })

	ctx := t.Context()
	var successCount atomic.Int64
	var maxObserved atomic.Int64

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// All goroutines try to create unique metrics simultaneously
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			err := recorder.IncrementCounter(ctx, fmt.Sprintf("atomic_metric_%d", id))
			if err == nil {
				successCount.Add(1)
			}

			// Track the maximum observed count
			count := recorder.CustomMetricCount()
			for {
				current := maxObserved.Load()
				if count <= int(current) || maxObserved.CompareAndSwap(current, int64(count)) {
					break
				}
			}
		}(i)
	}

	wg.Wait()

	// PROPERTY: Exactly 'limit' metrics should have been created
	assert.Equal(t, int64(limit), successCount.Load(),
		"Exactly %d metrics should succeed, got %d", limit, successCount.Load())

	// PROPERTY: The maximum observed count should never exceed the limit
	assert.LessOrEqual(t, int(maxObserved.Load()), limit,
		"Maximum observed count %d should not exceed limit %d",
		maxObserved.Load(), limit)

	// PROPERTY: Final count should equal limit
	finalCount := recorder.CustomMetricCount()
	assert.Equal(t, limit, finalCount,
		"Final count %d should equal limit %d", finalCount, limit)
}
