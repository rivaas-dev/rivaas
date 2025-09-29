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

package tracing

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

// BenchmarkTracingOverhead measures the overhead of tracing operations
func BenchmarkTracingOverhead(b *testing.B) {
	b.Run("NoTracing", func(b *testing.B) {
		config := MustNew(WithSampleRate(0.0))
		handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))

		req := httptest.NewRequest("GET", "/test", nil)

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		}
	})

	b.Run("WithTracing100Percent", func(b *testing.B) {
		config := MustNew(WithSampleRate(1.0))
		handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))

		req := httptest.NewRequest("GET", "/test", nil)

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		}
	})

	b.Run("WithTracing50Percent", func(b *testing.B) {
		config := MustNew(WithSampleRate(0.5))
		handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))

		req := httptest.NewRequest("GET", "/test", nil)

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		}
	})
}

// BenchmarkSpanOperations measures the cost of individual span operations
func BenchmarkSpanOperations(b *testing.B) {
	config := MustNew(WithServiceName("benchmark"))
	ctx := context.Background()

	b.Run("StartFinishSpan", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, span := config.StartSpan(ctx, "test-span")
			config.FinishSpan(span, http.StatusOK)
		}
	})

	b.Run("SetStringAttribute", func(b *testing.B) {
		_, span := config.StartSpan(ctx, "test-span")
		defer config.FinishSpan(span, http.StatusOK)

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			config.SetSpanAttribute(span, "key", "value")
		}
	})

	b.Run("SetIntAttribute", func(b *testing.B) {
		_, span := config.StartSpan(ctx, "test-span")
		defer config.FinishSpan(span, http.StatusOK)

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			config.SetSpanAttribute(span, "key", 42)
		}
	})

	b.Run("SetBoolAttribute", func(b *testing.B) {
		_, span := config.StartSpan(ctx, "test-span")
		defer config.FinishSpan(span, http.StatusOK)

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			config.SetSpanAttribute(span, "key", true)
		}
	})

	b.Run("AddEvent", func(b *testing.B) {
		_, span := config.StartSpan(ctx, "test-span")
		defer config.FinishSpan(span, http.StatusOK)

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			config.AddSpanEvent(span, "test-event")
		}
	})

	b.Run("AddEventWithAttributes", func(b *testing.B) {
		_, span := config.StartSpan(ctx, "test-span")
		defer config.FinishSpan(span, http.StatusOK)

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			config.AddSpanEvent(span, "test-event",
				attribute.String("key1", "value1"),
				attribute.Int("key2", 42),
			)
		}
	})
}

// BenchmarkContextPropagation measures trace context propagation overhead
func BenchmarkContextPropagation(b *testing.B) {
	config := MustNew(WithServiceName("benchmark"))
	ctx := context.Background()
	headers := http.Header{}

	b.Run("ExtractTraceContext", func(b *testing.B) {
		headers.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			config.ExtractTraceContext(ctx, headers)
		}
	})

	b.Run("InjectTraceContext", func(b *testing.B) {
		newCtx, span := config.StartSpan(ctx, "test-span")
		defer config.FinishSpan(span, http.StatusOK)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			h := http.Header{}
			config.InjectTraceContext(newCtx, h)
		}
	})
}

// BenchmarkResponseWriterConcurrency measures responseWriter mutex contention
func BenchmarkResponseWriterConcurrency(b *testing.B) {
	config := MustNew(WithSampleRate(1.0))
	handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		}
	})
}

// BenchmarkExcludedPaths measures path exclusion overhead
func BenchmarkExcludedPaths(b *testing.B) {
	b.Run("NoExclusions", func(b *testing.B) {
		config := MustNew()

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			config.ShouldExcludePath("/api/users")
		}
	})

	b.Run("With10Exclusions", func(b *testing.B) {
		config := MustNew(WithExcludePaths(
			"/health", "/metrics", "/ready", "/live", "/debug",
			"/status", "/ping", "/version", "/info", "/admin",
		))

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			config.ShouldExcludePath("/api/users")
		}
	})

	b.Run("With100Exclusions", func(b *testing.B) {
		paths := make([]string, 100)
		for i := 0; i < 100; i++ {
			paths[i] = fmt.Sprintf("/excluded%d", i)
		}
		config := MustNew(WithExcludePaths(paths...))

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			config.ShouldExcludePath("/api/users")
		}
	})
}

// BenchmarkSamplingDecision measures sampling decision overhead
func BenchmarkSamplingDecision(b *testing.B) {
	config := MustNew(WithSampleRate(0.5))
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, span := config.StartRequestSpan(ctx, req, "/test", false)
		config.FinishRequestSpan(span, http.StatusOK)
	}
}
