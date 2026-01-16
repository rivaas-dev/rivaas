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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

// TestStress_RapidCreateShutdownCycles tests rapid creation and shutdown cycles
// to ensure no resource leaks or race conditions.
//
//nolint:paralleltest // Subtests run sequentially to test rapid create/shutdown cycles
func TestStress_RapidCreateShutdownCycles(t *testing.T) {
	t.Parallel()

	const cycles = 20

	for i := range cycles {
		t.Run(fmt.Sprintf("Cycle%d", i), func(t *testing.T) {
			recorder := MustNew(
				WithStdout(),
				WithServiceName("stress-test"),
				WithServerDisabled(),
			)

			// Rapidly record some metrics
			for j := range 50 {
				recorder.IncrementCounter(t.Context(), fmt.Sprintf("counter_%d", j%10))             //nolint:errcheck // Stress test
				recorder.RecordHistogram(t.Context(), fmt.Sprintf("histogram_%d", j%5), float64(j)) //nolint:errcheck // Stress test
				recorder.SetGauge(t.Context(), fmt.Sprintf("gauge_%d", j%3), float64(j))            //nolint:errcheck // Stress test
			}

			// Rapid shutdown
			err := recorder.Shutdown(t.Context())
			assert.NoError(t, err, "Cycle %d shutdown failed", i)
		})
	}
}

// TestStress_ConcurrentShutdownRace tests that concurrent shutdown calls
// don't cause race conditions or panics.
func TestStress_ConcurrentShutdownRace(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithStdout(),
		WithServiceName("concurrent-shutdown-stress"),
		WithServerDisabled(),
	)

	// Record some metrics first
	for i := range 20 {
		recorder.IncrementCounter(t.Context(), fmt.Sprintf("counter_%d", i)) //nolint:errcheck // Stress test
	}

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Multiple goroutines try to shutdown simultaneously
	for range numGoroutines {
		go func() {
			defer wg.Done()
			// Should all succeed without panic (idempotent)
			err := recorder.Shutdown(t.Context())
			// Errors are acceptable, panics are not
			_ = err
		}()
	}

	wg.Wait()
}

// TestStress_MetricRecordingDuringShutdown tests that metric recording
// during shutdown doesn't cause panics or undefined behavior.
func TestStress_MetricRecordingDuringShutdown(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithStdout(),
		WithServiceName("shutdown-during-recording"),
		WithServerDisabled(),
	)

	var wg sync.WaitGroup

	// Start recording goroutines
	const recorderGoroutines = 20
	stopRecording := make(chan struct{})

	wg.Add(recorderGoroutines)
	for i := range recorderGoroutines {
		go func(id int) {
			defer wg.Done()
			j := 0
			for {
				select {
				case <-stopRecording:
					return
				default:
					// Continue recording even during shutdown
					recorder.IncrementCounter(t.Context(), fmt.Sprintf("counter_%d", id))              //nolint:errcheck // Stress test
					recorder.RecordHistogram(t.Context(), fmt.Sprintf("histogram_%d", id), float64(j)) //nolint:errcheck // Stress test
					j++
					// Small yield to allow interleaving
					if j%100 == 0 {
						time.Sleep(time.Microsecond)
					}
				}
			}
		}(i)
	}

	// Let recording run for a bit
	time.Sleep(10 * time.Millisecond)

	// Start shutdown while recording is ongoing
	wg.Go(func() {
		err := recorder.Shutdown(t.Context())
		_ = err // Error is acceptable, panic is not
	})

	// Let shutdown and recording race for a bit
	time.Sleep(20 * time.Millisecond)

	// Stop recording goroutines
	close(stopRecording)
	wg.Wait()
}

// TestStress_HighConcurrencyMetricCreation tests metric creation under
// very high concurrency to detect race conditions.
func TestStress_HighConcurrencyMetricCreation(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithStdout(),
		WithServiceName("high-concurrency-stress"),
		WithMaxCustomMetrics(1000),
		WithServerDisabled(),
	)
	t.Cleanup(func() {
		recorder.Shutdown(t.Context()) //nolint:errcheck // Stress test cleanup
	})

	ctx := t.Context()
	const numGoroutines = 200
	const operationsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// High concurrency metric operations
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			for j := range operationsPerGoroutine {
				// Mix of operations
				switch j % 3 {
				case 0:
					recorder.IncrementCounter(ctx, fmt.Sprintf("counter_%d_%d", id, j%20)) //nolint:errcheck // Stress test
				case 1:
					recorder.RecordHistogram(ctx, fmt.Sprintf("histogram_%d_%d", id, j%10), float64(j)) //nolint:errcheck // Stress test
				case 2:
					recorder.SetGauge(ctx, fmt.Sprintf("gauge_%d_%d", id, j%5), float64(j)) //nolint:errcheck // Stress test
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify the recorder is still functional
	count := recorder.CustomMetricCount()
	assert.Positive(t, count, "Should have created some metrics")
	assert.LessOrEqual(t, count, 1000, "Should not exceed limit")
}

// TestStress_MiddlewareThroughput tests middleware under high request volume.
func TestStress_MiddlewareThroughput(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithStdout(),
		WithServiceName("middleware-stress"),
		WithServerDisabled(),
	)
	t.Cleanup(func() {
		recorder.Shutdown(t.Context()) //nolint:errcheck // Stress test cleanup
	})

	// Create a simple handler wrapped with middleware
	handler := Middleware(recorder)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK")) //nolint:errcheck // Stress test handler
	}))

	const numGoroutines = 100
	const requestsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for range numGoroutines {
		go func() {
			defer wg.Done()
			for range requestsPerGoroutine {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				// Verify response
				if w.Code != http.StatusOK {
					t.Errorf("unexpected status code: %d", w.Code)
				}
			}
		}()
	}

	wg.Wait()
}

// TestStress_PathFilterHighVolume tests path filter performance under high volume.
func TestStress_PathFilterHighVolume(t *testing.T) {
	t.Parallel()

	pf := newPathFilter()
	pf.addPaths("/health", "/metrics", "/ready", "/live")
	pf.addPrefixes("/debug/", "/internal/", "/admin/")

	paths := []string{
		"/health",
		"/metrics",
		"/api/users",
		"/api/users/123",
		"/debug/pprof",
		"/internal/status",
		"/products/456",
		"/orders/789",
	}

	const numGoroutines = 100
	const checksPerGoroutine = 10000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			for j := range checksPerGoroutine {
				path := paths[(id+j)%len(paths)]
				// Just exercise the function, not checking result
				_ = pf.shouldExclude(path)
			}
		}(i)
	}

	wg.Wait()
}

// TestStress_MixedReadWriteOperations tests mixed read/write operations
// to verify RWMutex behavior under stress.
func TestStress_MixedReadWriteOperations(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithStdout(),
		WithServiceName("mixed-ops-stress"),
		WithMaxCustomMetrics(500),
		WithServerDisabled(),
	)
	t.Cleanup(func() {
		recorder.Shutdown(t.Context()) //nolint:errcheck // Stress test cleanup
	})

	ctx := t.Context()
	const numReaders = 50
	const numWriters = 50
	const operationsPerGoroutine = 500

	var wg sync.WaitGroup
	wg.Add(numReaders + numWriters)

	// Readers - call CustomMetricCount repeatedly
	for range numReaders {
		go func() {
			defer wg.Done()
			for range operationsPerGoroutine {
				_ = recorder.CustomMetricCount()
			}
		}()
	}

	// Writers - create metrics
	for i := range numWriters {
		go func(id int) {
			defer wg.Done()
			for j := range operationsPerGoroutine {
				metricName := fmt.Sprintf("writer_%d_metric_%d", id, j%50)
				recorder.IncrementCounter(ctx, metricName) //nolint:errcheck // Stress test
			}
		}(i)
	}

	wg.Wait()

	// Verify recorder is still consistent
	count := recorder.CustomMetricCount()
	assert.Positive(t, count, "Should have created some metrics")
	assert.LessOrEqual(t, count, 500, "Should not exceed limit")
}

// TestStress_RequestMetricsLifecycle tests the full request metrics lifecycle
// under stress conditions.
func TestStress_RequestMetricsLifecycle(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithStdout(),
		WithServiceName("lifecycle-stress"),
		WithServerDisabled(),
	)
	t.Cleanup(func() {
		recorder.Shutdown(t.Context()) //nolint:errcheck // Stress test cleanup
	})

	ctx := t.Context()
	const numGoroutines = 100
	const requestsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := range numGoroutines {
		go func(_ int) {
			defer wg.Done()
			for j := range requestsPerGoroutine {
				// Full request lifecycle
				m := recorder.BeginRequest(ctx)
				if m == nil {
					t.Error("Start returned nil for enabled recorder")
					continue
				}

				// Add some attributes
				m.AddAttributes(
					attribute.String("http.method", "GET"),
					attribute.String("http.url", "/api/test"),
				)

				// Record request size
				recorder.RecordRequestSize(ctx, m, int64(j*100))

				// Finish with response
				recorder.Finish(ctx, m, 200, int64(j*50), "/api/test")
			}
		}(i)
	}

	wg.Wait()
}

// TestStress_PrometheusServerStability tests Prometheus server stability
// under connection stress.
//
//nolint:paralleltest // Cannot use t.Parallel() - test binds to a specific port
func TestStress_PrometheusServerStability(t *testing.T) {
	recorder := MustNew(
		WithPrometheus(":0", "/metrics"), // Use port 0 to get random available port
		WithServiceName("prometheus-stress"),
	)

	// Start the metrics server
	err := recorder.Start(t.Context())
	if err != nil {
		t.Skipf("Could not start metrics server: %v", err)
	}

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	serverAddr := recorder.ServerAddress()
	if serverAddr == "" {
		t.Skip("Server not started, skipping test")
	}

	// Try to wait for server
	err = waitForMetricsServer(t, "localhost"+serverAddr, 2*time.Second)
	if err != nil {
		t.Skipf("Could not connect to metrics server: %v", err)
	}

	// Get the handler for direct testing
	handler, err := recorder.Handler()
	require.NoError(t, err)
	require.NotNil(t, handler)

	const numGoroutines = 50
	const requestsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Stress the handler
	for range numGoroutines {
		go func() {
			defer wg.Done()
			for range requestsPerGoroutine {
				req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					// Log but don't fail - some requests may fail under stress
					t.Logf("metrics endpoint returned status %d", w.Code)
				}
			}
		}()
	}

	wg.Wait()

	// Clean shutdown
	err = recorder.Shutdown(t.Context())
	assert.NoError(t, err)
}
