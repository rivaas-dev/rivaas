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
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

// BenchmarkRecordHistogram_Cached benchmarks recording to an existing (cached) histogram.
// This is the hot path - metric already exists, just recording values.
func BenchmarkRecordHistogram_Cached(b *testing.B) {
	recorder := MustNew(
		WithServiceName("bench-service"),
		WithStdout(),
		WithServerDisabled(),
	)
	b.Cleanup(func() {
		//nolint:errcheck // Benchmark cleanup
		recorder.Shutdown(b.Context())
	})

	// Pre-create the metric so it's cached
	//nolint:errcheck // Benchmark setup
	recorder.RecordHistogram(b.Context(), "test.cached.metric", 1.0)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark hot path
		recorder.RecordHistogram(b.Context(), "test.cached.metric", 1.0)
	}
}

// BenchmarkRecordHistogram_CachedWithAttributes benchmarks recording to cached histogram with attributes.
func BenchmarkRecordHistogram_CachedWithAttributes(b *testing.B) {
	recorder := MustNew(
		WithServiceName("bench-service"),
		WithStdout(),
		WithServerDisabled(),
	)
	b.Cleanup(func() {
		//nolint:errcheck // Benchmark cleanup
		recorder.Shutdown(b.Context())
	})

	attrs := []attribute.KeyValue{
		attribute.String("endpoint", "/api/users"),
		attribute.String("method", "GET"),
		attribute.Int("status", 200),
	}

	// Pre-create the metric
	//nolint:errcheck // Benchmark setup
	recorder.RecordHistogram(b.Context(), "test.cached.with.attrs", 1.0, attrs...)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark hot path
		recorder.RecordHistogram(b.Context(), "test.cached.with.attrs", 1.0, attrs...)
	}
}

// BenchmarkRecordHistogram_New benchmarks creating new histogram metrics.
// This is the slow path - metric doesn't exist yet.
func BenchmarkRecordHistogram_New(b *testing.B) {
	recorder := MustNew(
		WithServiceName("bench-service"),
		WithStdout(),
		WithServerDisabled(),
		WithMaxCustomMetrics(100000), // Allow many metrics
	)
	b.Cleanup(func() {
		//nolint:errcheck // Benchmark cleanup
		recorder.Shutdown(b.Context())
	})

	b.ResetTimer()
	b.ReportAllocs()

	i := 0
	for b.Loop() {
		// Each iteration creates a new metric
		//nolint:errcheck // Benchmark hot path
		recorder.RecordHistogram(b.Context(), "test.new.metric."+string(rune(i)), float64(i))
		i++
	}
}

// BenchmarkIncrementCounter_Cached benchmarks incrementing an existing (cached) counter.
func BenchmarkIncrementCounter_Cached(b *testing.B) {
	recorder := MustNew(
		WithServiceName("bench-service"),
		WithStdout(),
		WithServerDisabled(),
	)
	b.Cleanup(func() {
		//nolint:errcheck // Benchmark cleanup
		recorder.Shutdown(b.Context())
	})

	// Pre-create the counter
	//nolint:errcheck // Benchmark setup
	recorder.IncrementCounter(b.Context(), "test.cached.counter")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark hot path
		recorder.IncrementCounter(b.Context(), "test.cached.counter")
	}
}

// BenchmarkIncrementCounter_CachedWithAttributes benchmarks counter increment with attributes.
func BenchmarkIncrementCounter_CachedWithAttributes(b *testing.B) {
	recorder := MustNew(
		WithServiceName("bench-service"),
		WithStdout(),
		WithServerDisabled(),
	)
	b.Cleanup(func() {
		//nolint:errcheck // Benchmark cleanup
		recorder.Shutdown(b.Context())
	})

	attrs := []attribute.KeyValue{
		attribute.String("status", "success"),
		attribute.String("operation", "create"),
	}

	// Pre-create the counter
	//nolint:errcheck // Benchmark setup
	recorder.IncrementCounter(b.Context(), "test.cached.counter.attrs", attrs...)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark hot path
		recorder.IncrementCounter(b.Context(), "test.cached.counter.attrs", attrs...)
	}
}

// BenchmarkSetGauge_Cached benchmarks setting an existing (cached) gauge.
func BenchmarkSetGauge_Cached(b *testing.B) {
	recorder := MustNew(
		WithServiceName("bench-service"),
		WithStdout(),
		WithServerDisabled(),
	)
	b.Cleanup(func() {
		//nolint:errcheck // Benchmark cleanup
		recorder.Shutdown(b.Context())
	})

	// Pre-create the gauge
	//nolint:errcheck // Benchmark setup
	recorder.SetGauge(b.Context(), "test.cached.gauge", 1.0)

	b.ResetTimer()
	b.ReportAllocs()

	i := 0
	for b.Loop() {
		//nolint:errcheck // Benchmark hot path
		recorder.SetGauge(b.Context(), "test.cached.gauge", float64(i))
		i++
	}
}

// BenchmarkStart benchmarks the Start operation.
func BenchmarkStart(b *testing.B) {
	recorder := MustNew(
		WithServiceName("bench-service"),
		WithStdout(),
		WithServerDisabled(),
	)
	b.Cleanup(func() {
		//nolint:errcheck // Benchmark cleanup
		recorder.Shutdown(b.Context())
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		recorder.BeginRequest(b.Context())
	}
}

// BenchmarkStart_WithAttributes benchmarks Start with added attributes.
func BenchmarkStart_WithAttributes(b *testing.B) {
	recorder := MustNew(
		WithServiceName("bench-service"),
		WithStdout(),
		WithServerDisabled(),
	)
	b.Cleanup(func() {
		//nolint:errcheck // Benchmark cleanup
		recorder.Shutdown(b.Context())
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		m := recorder.BeginRequest(b.Context())
		m.AddAttributes(
			attribute.String("http.method", "GET"),
			attribute.String("http.url", "http://localhost/api/users"),
			attribute.String("http.scheme", "http"),
			attribute.String("http.host", "localhost"),
			attribute.String("http.user_agent", "Go-http-client/1.1"),
		)
	}
}

// BenchmarkStartFinish benchmarks full request lifecycle.
func BenchmarkStartFinish(b *testing.B) {
	recorder := MustNew(
		WithServiceName("bench-service"),
		WithStdout(),
		WithServerDisabled(),
	)
	b.Cleanup(func() {
		//nolint:errcheck // Benchmark cleanup
		recorder.Shutdown(b.Context())
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		m := recorder.BeginRequest(b.Context())
		m.AddAttributes(
			attribute.String("http.method", "GET"),
			attribute.String("http.url", "http://localhost/api/users"),
		)
		recorder.RecordRequestSize(b.Context(), m, 1024)
		recorder.Finish(b.Context(), m, 200, 2048, "/api/users")
	}
}

// BenchmarkAttributeAllocation benchmarks attribute slice allocation strategies.
func BenchmarkAttributeAllocation(b *testing.B) {
	b.Run("NoPreallocation", func(b *testing.B) {
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			attrs := []attribute.KeyValue{
				attribute.String("key1", "value1"),
				attribute.String("key2", "value2"),
				attribute.String("key3", "value3"),
			}
			if i%2 == 0 {
				attrs = append(attrs, attribute.String("key4", "value4"))
			}
			attrs = append(attrs, attribute.String("key5", "value5"))
			_ = attrs
			i++
		}
	})

	b.Run("WithPreallocation", func(b *testing.B) {
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			capacity := 3
			if i%2 == 0 {
				capacity++
			}
			capacity++ // for key5

			attrs := make([]attribute.KeyValue, 3, capacity)
			attrs[0] = attribute.String("key1", "value1")
			attrs[1] = attribute.String("key2", "value2")
			attrs[2] = attribute.String("key3", "value3")
			if i%2 == 0 {
				attrs = append(attrs, attribute.String("key4", "value4"))
			}
			attrs = append(attrs, attribute.String("key5", "value5"))
			_ = attrs
			i++
		}
	})
}

// BenchmarkPrecomputedAttributes benchmarks pre-computed vs dynamic attributes.
func BenchmarkPrecomputedAttributes(b *testing.B) {
	serviceName := "bench-service"
	serviceVersion := "1.0.0"

	b.Run("DynamicAttributes", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			attrs := []attribute.KeyValue{
				attribute.String("service.name", serviceName),
				attribute.String("service.version", serviceVersion),
				attribute.Bool("static", true),
			}
			_ = attrs
		}
	})

	b.Run("PrecomputedAttributes", func(b *testing.B) {
		serviceNameAttr := attribute.String("service.name", serviceName)
		serviceVersionAttr := attribute.String("service.version", serviceVersion)
		staticAttr := attribute.Bool("static", true)

		b.ReportAllocs()
		for b.Loop() {
			attrs := []attribute.KeyValue{
				serviceNameAttr,
				serviceVersionAttr,
				staticAttr,
			}
			_ = attrs
		}
	})
}

// BenchmarkCASContention benchmarks CAS operations under different contention levels.
func BenchmarkCASContention(b *testing.B) {
	b.Run("LowContention_1Goroutine", func(b *testing.B) {
		recorder := MustNew(
			WithServiceName("bench-service"),
			WithStdout(),
			WithServerDisabled(),
			WithMaxCustomMetrics(100000),
		)
		b.Cleanup(func() {
			//nolint:errcheck // Benchmark cleanup
			recorder.Shutdown(b.Context())
		})

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			//nolint:errcheck // Benchmark hot path
			recorder.IncrementCounter(b.Context(), "counter.low.contention")
		}
	})

	b.Run("MediumContention_4Goroutines", func(b *testing.B) {
		recorder := MustNew(
			WithServiceName("bench-service"),
			WithStdout(),
			WithServerDisabled(),
			WithMaxCustomMetrics(100000),
		)
		b.Cleanup(func() {
			//nolint:errcheck // Benchmark cleanup
			recorder.Shutdown(b.Context())
		})

		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				// Each goroutine creates different metrics
				//nolint:errcheck // Benchmark hot path
				recorder.IncrementCounter(b.Context(), "counter.medium.contention."+string(rune(i%4)))
				i++
			}
		})
	})

	b.Run("HighContention_Parallel", func(b *testing.B) {
		recorder := MustNew(
			WithServiceName("bench-service"),
			WithStdout(),
			WithServerDisabled(),
			WithMaxCustomMetrics(100000),
		)
		b.Cleanup(func() {
			//nolint:errcheck // Benchmark cleanup
			recorder.Shutdown(b.Context())
		})

		b.ResetTimer()
		b.ReportAllocs()

		// All goroutines try to create new metrics simultaneously
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				//nolint:errcheck // Benchmark hot path
				recorder.IncrementCounter(b.Context(), "counter.high.contention."+string(rune(i)))
				i++
			}
		})
	})
}

// BenchmarkValidateMetricName benchmarks metric name validation.
func BenchmarkValidateMetricName(b *testing.B) {
	b.Run("ValidName", func(b *testing.B) {
		name := "http.request.duration"
		b.ReportAllocs()
		for b.Loop() {
			//nolint:errcheck // Benchmark hot path
			validateMetricName(name)
		}
	})

	b.Run("InvalidName_Empty", func(b *testing.B) {
		name := ""
		b.ReportAllocs()
		for b.Loop() {
			//nolint:errcheck // Benchmark hot path
			validateMetricName(name)
		}
	})

	b.Run("InvalidName_Reserved", func(b *testing.B) {
		name := "http_reserved_metric"
		b.ReportAllocs()
		for b.Loop() {
			//nolint:errcheck // Benchmark hot path
			validateMetricName(name)
		}
	})
}

// BenchmarkMetricCreation_Parallel benchmarks concurrent metric creation.
func BenchmarkMetricCreation_Parallel(b *testing.B) {
	recorder := MustNew(
		WithServiceName("bench-service"),
		WithStdout(),
		WithServerDisabled(),
		WithMaxCustomMetrics(1000000),
	)
	b.Cleanup(func() {
		//nolint:errcheck // Benchmark cleanup
		recorder.Shutdown(b.Context())
	})

	var counter int64

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Each goroutine creates unique metrics
			i := counter
			counter++
			//nolint:errcheck // Benchmark hot path
			recorder.RecordHistogram(b.Context(), "parallel.metric."+string(rune(i)), float64(i))
		}
	})
}

// BenchmarkNoopProvider benchmarks with a custom noop provider for baseline.
func BenchmarkNoopProvider(b *testing.B) {
	// Create recorder with noop meter provider
	noopProvider := noop.NewMeterProvider()
	recorder := &Recorder{
		enabled:          true,
		serviceName:      "bench-service",
		serviceVersion:   "1.0.0",
		meterProvider:    noopProvider,
		meter:            noopProvider.Meter("bench"),
		maxCustomMetrics: 1000,
		durationBuckets:  DefaultDurationBuckets,
		sizeBuckets:      DefaultSizeBuckets,
		customCounters:   make(map[string]metric.Int64Counter),
		customHistograms: make(map[string]metric.Float64Histogram),
		customGauges:     make(map[string]metric.Float64Gauge),
	}

	// Initialize all required noop metrics
	//nolint:errcheck // Noop metrics
	recorder.requestDuration, _ = recorder.meter.Float64Histogram("http_request_duration_seconds")
	//nolint:errcheck // Noop metrics
	recorder.requestCount, _ = recorder.meter.Int64Counter("http_requests_total")
	//nolint:errcheck // Noop metrics
	recorder.activeRequests, _ = recorder.meter.Int64UpDownCounter("http_requests_active")
	//nolint:errcheck // Noop metrics
	recorder.requestSize, _ = recorder.meter.Int64Histogram("http_request_size_bytes")
	//nolint:errcheck // Noop metrics
	recorder.responseSize, _ = recorder.meter.Int64Histogram("http_response_size_bytes")
	//nolint:errcheck // Noop metrics
	recorder.errorCount, _ = recorder.meter.Int64Counter("http_errors_total")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		m := recorder.BeginRequest(b.Context())
		m.AddAttributes(
			attribute.String("http.method", "GET"),
			attribute.String("http.url", "http://localhost/api/users"),
		)
		recorder.Finish(b.Context(), m, 200, 2048, "/api/users")
	}
}

// BenchmarkHeaderProcessing benchmarks header recording with pre-lowercased names.
func BenchmarkHeaderProcessing(b *testing.B) {
	headers := []string{"Authorization", "Content-Type", "X-Request-ID"}

	b.Run("WithLowercasing", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			attrs := make([]attribute.KeyValue, 0, len(headers))
			for _, header := range headers {
				key := "http.request.header." + toLower(header)
				attrs = append(attrs, attribute.String(key, "value"))
			}
			_ = attrs
		}
	})

	b.Run("WithPreLowercased", func(b *testing.B) {
		headersLower := []string{"authorization", "content-type", "x-request-id"}
		b.ReportAllocs()
		for b.Loop() {
			attrs := make([]attribute.KeyValue, 0, len(headers))
			for _, headerLower := range headersLower {
				key := "http.request.header." + headerLower
				attrs = append(attrs, attribute.String(key, "value"))
			}
			_ = attrs
		}
	})
}

// toLower is a simple lowercase helper for benchmarking.
func toLower(s string) string {
	result := make([]byte, 0, len(s))
	for i := range len(s) {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result = append(result, c)
	}

	return string(result)
}

// BenchmarkExcludedPath benchmarks the path filter check.
func BenchmarkExcludedPath(b *testing.B) {
	// Path filtering is now in middleware, benchmark the pathFilter directly
	pf := newPathFilter()
	pf.addPaths("/health", "/metrics", "/debug")

	b.Run("ExcludedPath", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			//nolint:errcheck // Benchmark hot path
			pf.shouldExclude("/health")
		}
	})

	b.Run("NonExcludedPath", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			//nolint:errcheck // Benchmark hot path
			pf.shouldExclude("/api/users")
		}
	})
}

// BenchmarkRWMutexOperations benchmarks RWMutex-based metric operations.
func BenchmarkRWMutexOperations(b *testing.B) {
	recorder := MustNew(
		WithServiceName("bench-service"),
		WithStdout(),
		WithServerDisabled(),
	)
	b.Cleanup(func() {
		//nolint:errcheck // Benchmark cleanup
		recorder.Shutdown(b.Context())
	})

	// Pre-create a counter to test the read path
	//nolint:errcheck // Benchmark setup
	recorder.IncrementCounter(b.Context(), "rwmutex_test_counter")

	b.Run("SingleThreaded", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			//nolint:errcheck // Benchmark hot path
			recorder.IncrementCounter(b.Context(), "rwmutex_test_counter")
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				//nolint:errcheck // Benchmark hot path
				recorder.IncrementCounter(b.Context(), "rwmutex_test_counter")
			}
		})
	})
}

// BenchmarkCASRetryBackoff benchmarks different backoff strategies.
func BenchmarkCASRetryBackoff(b *testing.B) {
	recorder := MustNew(
		WithServiceName("bench-service"),
		WithStdout(),
		WithServerDisabled(),
		WithMaxCustomMetrics(1000000),
	)

	b.Cleanup(func() {
		//nolint:errcheck // Benchmark cleanup
		recorder.Shutdown(b.Context())
	})

	b.Run("SequentialCreation", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			//nolint:errcheck // Benchmark hot path
			recorder.IncrementCounter(b.Context(), "seq.counter."+string(rune(i)))
			i++
		}
	})

	b.Run("ParallelCreation_8Goroutines", func(b *testing.B) {
		var wg sync.WaitGroup
		b.ReportAllocs()
		b.ResetTimer()

		goroutines := 8
		itemsPerGoroutine := b.N / goroutines

		for g := range goroutines {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for i := range itemsPerGoroutine {
					//nolint:errcheck // Benchmark hot path
					recorder.IncrementCounter(b.Context(), "parallel.counter."+string(rune(goroutineID*itemsPerGoroutine+i)))
				}
			}(g)
		}
		wg.Wait()
	})
}

// BenchmarkCustomMetricsCreation benchmarks custom metrics creation
func BenchmarkCustomMetricsCreation(b *testing.B) {
	recorder := MustNew(
		WithPrometheus(":19208", "/metrics"),
		WithServiceName("benchmark-service"),
		WithMaxCustomMetrics(10000),
		WithServerDisabled(),
	)
	b.Cleanup(func() {
		//nolint:errcheck // Benchmark cleanup
		recorder.Shutdown(b.Context())
	})

	b.Run("SequentialCreation", func(b *testing.B) {
		b.ResetTimer()
		for i := range b.N {
			metricName := fmt.Sprintf("counter_%d", i%1000)
			//nolint:errcheck // Benchmark hot path
			recorder.IncrementCounter(b.Context(), metricName)
		}
	})

	b.Run("ParallelCreation", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				metricName := fmt.Sprintf("parallel_counter_%d", i%1000)
				//nolint:errcheck // Benchmark hot path
				recorder.IncrementCounter(b.Context(), metricName)
				i++
			}
		})
	})
}

// TestRecordHistogram_ZeroAlloc verifies that recording to a cached histogram
// has minimal allocations.
//
//nolint:paralleltest // Cannot use t.Parallel() with testing.AllocsPerRun
func TestRecordHistogram_ZeroAlloc(t *testing.T) {
	recorder := MustNew(
		WithStdout(),
		WithServiceName("alloc-test"),
		WithServerDisabled(),
	)
	t.Cleanup(func() {
		//nolint:errcheck // Test cleanup
		recorder.Shutdown(t.Context())
	})

	// Pre-create the metric so it's cached
	//nolint:errcheck // Test setup
	recorder.RecordHistogram(t.Context(), "prewarmed_histogram", 1.0)

	allocs := testing.AllocsPerRun(100, func() {
		//nolint:errcheck // Test hot path
		recorder.RecordHistogram(t.Context(), "prewarmed_histogram", 1.0)
	})

	// Recording to existing histogram should have minimal allocations
	// The OpenTelemetry SDK may have some internal allocations
	if allocs > 5 {
		t.Errorf("RecordHistogram to cached metric allocated %.1f times, want <= 5", allocs)
	}
}

// TestIncrementCounter_ZeroAlloc verifies that incrementing a cached counter
// has minimal allocations.
//
//nolint:paralleltest // Cannot use t.Parallel() with testing.AllocsPerRun
func TestIncrementCounter_ZeroAlloc(t *testing.T) {
	recorder := MustNew(
		WithStdout(),
		WithServiceName("alloc-test"),
		WithServerDisabled(),
	)
	t.Cleanup(func() {
		//nolint:errcheck // Test cleanup
		recorder.Shutdown(t.Context())
	})

	// Pre-create the counter
	//nolint:errcheck // Test setup
	recorder.IncrementCounter(t.Context(), "prewarmed_counter")

	allocs := testing.AllocsPerRun(100, func() {
		//nolint:errcheck // Test hot path
		recorder.IncrementCounter(t.Context(), "prewarmed_counter")
	})

	// Incrementing existing counter should have minimal allocations
	if allocs > 5 {
		t.Errorf("IncrementCounter to cached metric allocated %.1f times, want <= 5", allocs)
	}
}

// TestSetGauge_ZeroAlloc verifies that setting a cached gauge
// has minimal allocations.
//
//nolint:paralleltest // Cannot use t.Parallel() with testing.AllocsPerRun
func TestSetGauge_ZeroAlloc(t *testing.T) {
	recorder := MustNew(
		WithStdout(),
		WithServiceName("alloc-test"),
		WithServerDisabled(),
	)
	t.Cleanup(func() {
		//nolint:errcheck // Test cleanup
		recorder.Shutdown(t.Context())
	})

	// Pre-create the gauge
	//nolint:errcheck // Test setup
	recorder.SetGauge(t.Context(), "prewarmed_gauge", 1.0)

	allocs := testing.AllocsPerRun(100, func() {
		//nolint:errcheck // Test hot path
		recorder.SetGauge(t.Context(), "prewarmed_gauge", 42.0)
	})

	// Setting existing gauge should have minimal allocations
	if allocs > 5 {
		t.Errorf("SetGauge to cached metric allocated %.1f times, want <= 5", allocs)
	}
}

// TestPathFilter_ZeroAlloc verifies that path filtering has zero allocations.
//
//nolint:paralleltest // Cannot use t.Parallel() with testing.AllocsPerRun
func TestPathFilter_ZeroAlloc(t *testing.T) {
	pf := newPathFilter()
	pf.addPaths("/health", "/metrics", "/ready")
	pf.addPrefixes("/debug/", "/internal/")

	// Test exact path match
	allocsExact := testing.AllocsPerRun(100, func() {
		_ = pf.shouldExclude("/health")
	})
	if allocsExact > 0 {
		t.Errorf("shouldExclude (exact match) allocated %.1f times, want 0", allocsExact)
	}

	// Test non-excluded path
	allocsNonExcluded := testing.AllocsPerRun(100, func() {
		_ = pf.shouldExclude("/api/users")
	})
	if allocsNonExcluded > 0 {
		t.Errorf("shouldExclude (non-excluded) allocated %.1f times, want 0", allocsNonExcluded)
	}

	// Test prefix match
	allocsPrefix := testing.AllocsPerRun(100, func() {
		_ = pf.shouldExclude("/debug/pprof")
	})
	if allocsPrefix > 0 {
		t.Errorf("shouldExclude (prefix match) allocated %.1f times, want 0", allocsPrefix)
	}
}

// TestValidateMetricName_ZeroAlloc verifies that metric name validation
// has minimal allocations for valid names.
//
//nolint:paralleltest // Cannot use t.Parallel() with testing.AllocsPerRun
func TestValidateMetricName_ZeroAlloc(t *testing.T) {
	validName := "my_valid_metric_name"

	allocs := testing.AllocsPerRun(100, func() {
		//nolint:errcheck // Test hot path
		validateMetricName(validName)
	})

	// Validation of valid names should have minimal allocations
	if allocs > 0 {
		t.Errorf("validateMetricName for valid name allocated %.1f times, want 0", allocs)
	}
}

// TestStart_Allocations verifies the allocation count for Start operation.
//
//nolint:paralleltest // Cannot use t.Parallel() with testing.AllocsPerRun
func TestStart_Allocations(t *testing.T) {
	recorder := MustNew(
		WithStdout(),
		WithServiceName("alloc-test"),
		WithServerDisabled(),
	)
	t.Cleanup(func() {
		//nolint:errcheck // Test cleanup
		recorder.Shutdown(t.Context())
	})

	allocs := testing.AllocsPerRun(100, func() {
		m := recorder.BeginRequest(t.Context())
		// Must use m to prevent compiler optimization
		if m == nil {
			t.Fatal("unexpected nil")
		}
	})

	// Start allocates a RequestMetrics struct, attribute slice, and may have
	// OpenTelemetry SDK internal allocations. The actual count varies slightly
	// based on SDK version and Go version.
	if allocs > 10 {
		t.Errorf("Start allocated %.1f times, want <= 10", allocs)
	}
}
