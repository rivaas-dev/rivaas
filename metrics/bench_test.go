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

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/noop"
)

// BenchmarkRecordMetric_Cached benchmarks recording to an existing (cached) histogram.
// This is the hot path - metric already exists, just recording values.
func BenchmarkRecordMetric_Cached(b *testing.B) {
	config := MustNew(
		WithServiceName("bench-service"),
		WithProvider(StdoutProvider),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()
	// Pre-create the metric so it's cached
	config.RecordMetric(ctx, "test.cached.metric", 1.0)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		config.RecordMetric(ctx, "test.cached.metric", 1.0)
	}
}

// BenchmarkRecordMetric_CachedWithAttributes benchmarks recording to cached histogram with attributes.
func BenchmarkRecordMetric_CachedWithAttributes(b *testing.B) {
	config := MustNew(
		WithServiceName("bench-service"),
		WithProvider(StdoutProvider),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()
	attrs := []attribute.KeyValue{
		attribute.String("endpoint", "/api/users"),
		attribute.String("method", "GET"),
		attribute.Int("status", 200),
	}

	// Pre-create the metric
	config.RecordMetric(ctx, "test.cached.with.attrs", 1.0, attrs...)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		config.RecordMetric(ctx, "test.cached.with.attrs", 1.0, attrs...)
	}
}

// BenchmarkRecordMetric_New benchmarks creating new histogram metrics.
// This is the slow path - metric doesn't exist yet.
func BenchmarkRecordMetric_New(b *testing.B) {
	config := MustNew(
		WithServiceName("bench-service"),
		WithProvider(StdoutProvider),
		WithServerDisabled(),
		WithMaxCustomMetrics(100000), // Allow many metrics
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	i := 0
	for b.Loop() {
		// Each iteration creates a new metric
		config.RecordMetric(ctx, "test.new.metric."+string(rune(i)), float64(i))
		i++
	}
}

// BenchmarkIncrementCounter_Cached benchmarks incrementing an existing (cached) counter.
func BenchmarkIncrementCounter_Cached(b *testing.B) {
	config := MustNew(
		WithServiceName("bench-service"),
		WithProvider(StdoutProvider),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()
	// Pre-create the counter
	config.IncrementCounter(ctx, "test.cached.counter")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		config.IncrementCounter(ctx, "test.cached.counter")
	}
}

// BenchmarkIncrementCounter_CachedWithAttributes benchmarks counter increment with attributes.
func BenchmarkIncrementCounter_CachedWithAttributes(b *testing.B) {
	config := MustNew(
		WithServiceName("bench-service"),
		WithProvider(StdoutProvider),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()
	attrs := []attribute.KeyValue{
		attribute.String("status", "success"),
		attribute.String("operation", "create"),
	}

	// Pre-create the counter
	config.IncrementCounter(ctx, "test.cached.counter.attrs", attrs...)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		config.IncrementCounter(ctx, "test.cached.counter.attrs", attrs...)
	}
}

// BenchmarkSetGauge_Cached benchmarks setting an existing (cached) gauge.
func BenchmarkSetGauge_Cached(b *testing.B) {
	config := MustNew(
		WithServiceName("bench-service"),
		WithProvider(StdoutProvider),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()
	// Pre-create the gauge
	config.SetGauge(ctx, "test.cached.gauge", 1.0)

	b.ResetTimer()
	b.ReportAllocs()

	i := 0
	for b.Loop() {
		config.SetGauge(ctx, "test.cached.gauge", float64(i))
		i++
	}
}

// BenchmarkStartRequest benchmarks the StartRequest operation.
func BenchmarkStartRequest(b *testing.B) {
	config := MustNew(
		WithServiceName("bench-service"),
		WithProvider(StdoutProvider),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		config.StartRequest(ctx, "/api/users", false)
	}
}

// BenchmarkStartRequest_WithAttributes benchmarks StartRequest with multiple attributes.
func BenchmarkStartRequest_WithAttributes(b *testing.B) {
	config := MustNew(
		WithServiceName("bench-service"),
		WithProvider(StdoutProvider),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()
	attrs := []attribute.KeyValue{
		attribute.String("http.method", "GET"),
		attribute.String("http.url", "http://localhost/api/users"),
		attribute.String("http.scheme", "http"),
		attribute.String("http.host", "localhost"),
		attribute.String("http.user_agent", "Go-http-client/1.1"),
		attribute.Int64("http.request.size", 1024),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		config.StartRequest(ctx, "/api/users", false, attrs...)
	}
}

// BenchmarkStartFinishRequest benchmarks full request lifecycle.
func BenchmarkStartFinishRequest(b *testing.B) {
	config := MustNew(
		WithServiceName("bench-service"),
		WithProvider(StdoutProvider),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()
	attrs := []attribute.KeyValue{
		attribute.String("http.method", "GET"),
		attribute.String("http.url", "http://localhost/api/users"),
		attribute.Int64("http.request.size", 1024),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		metrics := config.StartRequest(ctx, "/api/users", false, attrs...)
		config.FinishRequest(ctx, metrics, 200, 2048, "/api/users")
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
		config := MustNew(
			WithServiceName("bench-service"),
			WithProvider(StdoutProvider),
			WithServerDisabled(),
			WithMaxCustomMetrics(100000),
		)
		defer config.Shutdown(context.Background())

		ctx := context.Background()
		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			config.IncrementCounter(ctx, "counter.low.contention")
		}
	})

	b.Run("MediumContention_4Goroutines", func(b *testing.B) {
		config := MustNew(
			WithServiceName("bench-service"),
			WithProvider(StdoutProvider),
			WithServerDisabled(),
			WithMaxCustomMetrics(100000),
		)
		defer config.Shutdown(context.Background())

		ctx := context.Background()
		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				// Each goroutine creates different metrics
				config.IncrementCounter(ctx, "counter.medium.contention."+string(rune(i%4)))
				i++
			}
		})
	})

	b.Run("HighContention_Parallel", func(b *testing.B) {
		config := MustNew(
			WithServiceName("bench-service"),
			WithProvider(StdoutProvider),
			WithServerDisabled(),
			WithMaxCustomMetrics(100000),
		)
		defer config.Shutdown(context.Background())

		ctx := context.Background()
		b.ResetTimer()
		b.ReportAllocs()

		// All goroutines try to create new metrics simultaneously
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				config.IncrementCounter(ctx, "counter.high.contention."+string(rune(i)))
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
			_ = validateMetricName(name)
		}
	})

	b.Run("InvalidName_Empty", func(b *testing.B) {
		name := ""
		b.ReportAllocs()
		for b.Loop() {
			_ = validateMetricName(name)
		}
	})

	b.Run("InvalidName_Reserved", func(b *testing.B) {
		name := "http_reserved_metric"
		b.ReportAllocs()
		for b.Loop() {
			_ = validateMetricName(name)
		}
	})
}

// BenchmarkRecordRouteRegistration benchmarks route registration recording.
func BenchmarkRecordRouteRegistration(b *testing.B) {
	config := MustNew(
		WithServiceName("bench-service"),
		WithProvider(StdoutProvider),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		config.RecordRouteRegistration(ctx, "GET", "/api/users/:id")
	}
}

// BenchmarkRecordConstraintFailure benchmarks constraint failure recording.
func BenchmarkRecordConstraintFailure(b *testing.B) {
	config := MustNew(
		WithServiceName("bench-service"),
		WithProvider(StdoutProvider),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()
	attrs := []attribute.KeyValue{
		attribute.String("constraint.value", "abc"),
		attribute.String("expected", "numeric"),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		config.RecordConstraintFailure(ctx, "numeric", attrs...)
	}
}

// BenchmarkMetricCreation_Parallel benchmarks concurrent metric creation.
func BenchmarkMetricCreation_Parallel(b *testing.B) {
	config := MustNew(
		WithServiceName("bench-service"),
		WithProvider(StdoutProvider),
		WithServerDisabled(),
		WithMaxCustomMetrics(1000000),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()
	var counter int64

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Each goroutine creates unique metrics
			i := counter
			counter++
			config.RecordMetric(ctx, "parallel.metric."+string(rune(i)), float64(i))
		}
	})
}

// BenchmarkNoopProvider benchmarks with a custom noop provider for baseline.
func BenchmarkNoopProvider(b *testing.B) {
	// Create config with noop meter provider
	noopProvider := noop.NewMeterProvider()
	config := &Config{
		enabled:          true,
		serviceName:      "bench-service",
		serviceVersion:   "1.0.0",
		meterProvider:    noopProvider,
		meter:            noopProvider.Meter("bench"),
		excludePaths:     make(map[string]bool),
		maxCustomMetrics: 1000,
	}
	config.initAtomicMaps()
	config.initCommonAttributes()

	// Initialize all required noop metrics
	config.requestDuration, _ = config.meter.Float64Histogram("http_request_duration_seconds")
	config.requestCount, _ = config.meter.Int64Counter("http_requests_total")
	config.activeRequests, _ = config.meter.Int64UpDownCounter("http_requests_active")
	config.requestSize, _ = config.meter.Int64Histogram("http_request_size_bytes")
	config.responseSize, _ = config.meter.Int64Histogram("http_response_size_bytes")
	config.errorCount, _ = config.meter.Int64Counter("http_errors_total")

	ctx := context.Background()
	attrs := []attribute.KeyValue{
		attribute.String("http.method", "GET"),
		attribute.String("http.url", "http://localhost/api/users"),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		metrics := config.StartRequest(ctx, "/api/users", false, attrs...)
		config.FinishRequest(ctx, metrics, 200, 2048, "/api/users")
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
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// BenchmarkExcludedPath benchmarks the excluded path check.
func BenchmarkExcludedPath(b *testing.B) {
	config := MustNew(
		WithServiceName("bench-service"),
		WithProvider(StdoutProvider),
		WithServerDisabled(),
		WithExcludePaths("/health", "/metrics", "/debug"),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()

	b.Run("ExcludedPath", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			metrics := config.StartRequest(ctx, "/health", false)
			_ = metrics
		}
	})

	b.Run("NonExcludedPath", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			metrics := config.StartRequest(ctx, "/api/users", false)
			config.FinishRequest(ctx, metrics, 200, 1024, "/test")
		}
	})
}

// BenchmarkContextPoolOperations benchmarks context pool hit/miss recording.
func BenchmarkContextPoolOperations(b *testing.B) {
	config := MustNew(
		WithServiceName("bench-service"),
		WithProvider(StdoutProvider),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()

	b.Run("PoolHit", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			config.RecordContextPoolHit(ctx)
		}
	})

	b.Run("PoolMiss", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			config.RecordContextPoolMiss(ctx)
		}
	})
}

// BenchmarkAtomicOperations benchmarks atomic counter operations.
func BenchmarkAtomicOperations(b *testing.B) {
	config := MustNew(
		WithServiceName("bench-service"),
		WithProvider(StdoutProvider),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	b.Run("SingleThreaded", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			config.recordRequestCountAtomically()
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				config.recordRequestCountAtomically()
			}
		})
	})
}

// BenchmarkCASRetryBackoff benchmarks different backoff strategies.
func BenchmarkCASRetryBackoff(b *testing.B) {
	config := MustNew(
		WithServiceName("bench-service"),
		WithProvider(StdoutProvider),
		WithServerDisabled(),
		WithMaxCustomMetrics(1000000),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()

	b.Run("SequentialCreation", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			config.IncrementCounter(ctx, "seq.counter."+string(rune(i)))
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
					config.IncrementCounter(ctx, "parallel.counter."+string(rune(goroutineID*itemsPerGoroutine+i)))
				}
			}(g)
		}
		wg.Wait()
	})
}

// BenchmarkCustomMetricsCreation benchmarks custom metrics creation
func BenchmarkCustomMetricsCreation(b *testing.B) {
	config := MustNew(
		WithServiceName("benchmark-service"),
		WithPort(":19208"),
		WithMaxCustomMetrics(10000),
		WithServerDisabled(),
	)
	defer config.Shutdown(context.Background())

	ctx := context.Background()

	b.Run("SequentialCreation", func(b *testing.B) {
		b.ResetTimer()
		for i := range b.N {
			metricName := fmt.Sprintf("counter_%d", i%1000)
			config.IncrementCounter(ctx, metricName)
		}
	})

	b.Run("ParallelCreation", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				metricName := fmt.Sprintf("parallel_counter_%d", i%1000)
				config.IncrementCounter(ctx, metricName)
				i++
			}
		})
	})
}
