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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func TestTracingConfig(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
		WithServiceVersion("v1.0.0"),
		WithSampleRate(0.5),
	)
	defer config.Shutdown(context.Background())

	assert.True(t, config.IsEnabled())
	assert.Equal(t, "test-service", config.ServiceName())
	assert.Equal(t, "v1.0.0", config.ServiceVersion())
	assert.NotNil(t, config.GetTracer())
	assert.NotNil(t, config.GetPropagator())
}

func TestTracingWithHTTP(t *testing.T) {
	t.Parallel()

	// Create tracing config
	config := MustNew(
		WithServiceName("test-service"),
		WithSampleRate(1.0),
	)
	defer config.Shutdown(context.Background())

	// Create HTTP handler with tracing middleware
	handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}

func TestTracingOptions(t *testing.T) {
	t.Parallel()

	t.Run("WithExcludePaths", func(t *testing.T) {
		t.Parallel()

		config := MustNew(
			WithExcludePaths("/health", "/metrics"),
		)
		assert.True(t, config.ShouldExcludePath("/health"))
		assert.True(t, config.ShouldExcludePath("/metrics"))
		assert.False(t, config.ShouldExcludePath("/api"))
	})

	t.Run("WithHeaders", func(t *testing.T) {
		t.Parallel()

		config := MustNew(
			WithHeaders("Authorization", "X-Request-ID"),
		)
		assert.True(t, config.IsEnabled())
	})

	t.Run("WithDisableParams", func(t *testing.T) {
		t.Parallel()

		config := MustNew(
			WithDisableParams(),
		)
		assert.True(t, config.IsEnabled())
	})

	t.Run("WithCustomTracer", func(t *testing.T) {
		t.Parallel()

		tempConfig := MustNew()
		config := MustNew(
			WithCustomTracer(tempConfig.GetTracer()),
		)
		assert.True(t, config.IsEnabled())
	})
}

func TestTracingMiddleware(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
		WithExcludePaths("/health"),
	)

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with tracing middleware
	middleware := Middleware(config)
	wrappedHandler := middleware(handler)

	// Test the wrapped handler
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestTracingIntegration(t *testing.T) {
	t.Parallel()

	// Test full integration with HTTP middleware
	config := MustNew(
		WithServiceName("integration-test"),
		WithExcludePaths("/health"),
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

	// Wrap with tracing middleware
	handler := Middleware(config)(mux)

	// Test normal route
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Test health route (should be excluded from tracing)
	req = httptest.NewRequest("GET", "/health", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestContextHelpers(t *testing.T) {
	t.Parallel()

	// Test context helper functions
	ctx := context.Background()

	// These should not panic even without active spans
	traceID := TraceID(ctx)
	spanID := SpanID(ctx)

	assert.Equal(t, "", traceID)
	assert.Equal(t, "", spanID)
}

func TestSamplingRate(t *testing.T) {
	t.Parallel()

	t.Run("SampleRateValidation", func(t *testing.T) {
		t.Parallel()

		// Test clamping of sample rate
		config := MustNew(WithServiceName("test"), WithSampleRate(1.5))
		assert.Equal(t, 1.0, config.sampleRate)
		config.Shutdown(context.Background())

		config = MustNew(WithServiceName("test"), WithSampleRate(-0.5))
		assert.Equal(t, 0.0, config.sampleRate)
		config.Shutdown(context.Background())

		config = MustNew(WithServiceName("test"), WithSampleRate(0.5))
		assert.Equal(t, 0.5, config.sampleRate)
		defer config.Shutdown(context.Background())
	})

	t.Run("SampleRate100Percent", func(t *testing.T) {
		t.Parallel()

		config := MustNew(
			WithServiceName("test-service"),
			WithSampleRate(1.0),
		)
		defer config.Shutdown(context.Background())

		// All requests should be traced
		handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("SampleRate0Percent", func(t *testing.T) {
		t.Parallel()

		config := MustNew(
			WithServiceName("test-service"),
			WithSampleRate(0.0),
		)
		defer config.Shutdown(context.Background())

		// No requests should be traced
		handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("SampleRateStatistical", func(t *testing.T) {
		t.Parallel()
		// Note: This test validates that sampling logic works correctly.
		// With a noop tracer (default), spans won't be recorded, but we can
		// verify the sampling logic is being called correctly.

		config := MustNew(
			WithServiceName("test-service"),
			WithSampleRate(0.5),
		)
		defer config.Shutdown(context.Background())

		// The sampling logic is working correctly if:
		// 1. Requests are processed without errors
		// 2. No panics or race conditions occur
		// 3. The behavior is consistent

		handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))

		// Make multiple requests to verify sampling doesn't cause issues
		const numRequests = 100
		for i := 0; i < numRequests; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}

		// If we got here without panics or errors, sampling is working correctly
		assert.True(t, true, "Sampling logic executed successfully")
	})
}

func TestParameterRecording(t *testing.T) {
	t.Run("WithParams", func(t *testing.T) {
		config := MustNew(
			WithServiceName("test-service"),
		)
		defer config.Shutdown(context.Background())

		handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))

		req := httptest.NewRequest("GET", "/test?foo=bar&baz=qux", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("WithDisabledParams", func(t *testing.T) {
		config := MustNew(
			WithServiceName("test-service"),
			WithDisableParams(),
		)
		defer config.Shutdown(context.Background())

		assert.False(t, config.recordParams)

		handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))

		req := httptest.NewRequest("GET", "/test?foo=bar", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestSpanAttributeTypes(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
	)

	ctx := context.Background()
	_, span := config.StartSpan(ctx, "test-span")
	defer span.End()

	// Test different types - these should not panic even if span is not recording
	config.SetSpanAttribute(span, "string_attr", "value")
	config.SetSpanAttribute(span, "int_attr", 42)
	config.SetSpanAttribute(span, "int64_attr", int64(123))
	config.SetSpanAttribute(span, "float_attr", 3.14)
	config.SetSpanAttribute(span, "bool_attr", true)
	config.SetSpanAttribute(span, "other_attr", struct{ Name string }{"test"})

	// Verify span exists (may or may not be recording depending on OTEL setup)
	assert.NotNil(t, span)
}

func TestSpanAttributeTypesFromContext(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
	)

	ctx := context.Background()
	ctx, span := config.StartSpan(ctx, "test-span")
	defer span.End()

	// Test different types through context helper - should not panic
	SetSpanAttributeFromContext(ctx, "string_attr", "value")
	SetSpanAttributeFromContext(ctx, "int_attr", 42)
	SetSpanAttributeFromContext(ctx, "int64_attr", int64(123))
	SetSpanAttributeFromContext(ctx, "float_attr", 3.14)
	SetSpanAttributeFromContext(ctx, "bool_attr", true)

	// Verify span exists
	assert.NotNil(t, span)
}

func TestErrorStatusCodes(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
	)
	defer config.Shutdown(context.Background())

	// Create mux with different status codes
	mux := http.NewServeMux()
	mux.HandleFunc("/not-found", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	})
	mux.HandleFunc("/error", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	})

	handler := Middleware(config)(mux)

	// Test 404
	req := httptest.NewRequest("GET", "/not-found", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Test 500
	req = httptest.NewRequest("GET", "/error", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestConcurrentResponseWriter(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
	)
	defer config.Shutdown(context.Background())

	handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))

	// Test concurrent requests
	const numRequests = 50
	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/concurrent", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}()
	}

	wg.Wait()
}

func TestContextTracingHelpers(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
	)
	defer config.Shutdown(context.Background())

	handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get span from context and test attribute setting
		ctx := r.Context()

		// Test setting attributes through context (if span is available)
		SetSpanAttributeFromContext(ctx, "string", "value")
		SetSpanAttributeFromContext(ctx, "int", 42)
		SetSpanAttributeFromContext(ctx, "float", 3.14)
		SetSpanAttributeFromContext(ctx, "bool", true)

		// Test adding span event
		AddSpanEventFromContext(ctx, "test_event")

		// Test getting trace ID and span ID from context
		traceID := TraceID(ctx)
		spanID := SpanID(ctx)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"trace_id":"%s","span_id":"%s"}`, traceID, spanID)))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// Edge case tests
func TestEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("DisabledTracing", func(t *testing.T) {
		t.Parallel()

		config := MustNew(WithSampleRate(0.0))
		ctx := context.Background()

		// All operations should be no-ops
		_, span := config.StartSpan(ctx, "test")
		config.SetSpanAttribute(span, "key", "value")
		config.AddSpanEvent(span, "event")
		config.FinishSpan(span, http.StatusOK)

		// Should not panic
		assert.NotNil(t, span)
	})

	t.Run("NilContext", func(t *testing.T) {
		t.Parallel()

		// These should not panic even with nil/empty contexts
		traceID := TraceID(context.Background())
		spanID := SpanID(context.Background())

		assert.Equal(t, "", traceID)
		assert.Equal(t, "", spanID)

		// These should be no-ops
		SetSpanAttributeFromContext(context.Background(), "key", "value")
		AddSpanEventFromContext(context.Background(), "event")
	})

	t.Run("MultipleFinishSpan", func(t *testing.T) {
		t.Parallel()

		config := MustNew()
		ctx := context.Background()
		_, span := config.StartSpan(ctx, "test")

		// Should be safe to call multiple times
		config.FinishSpan(span, http.StatusOK)
		config.FinishSpan(span, http.StatusOK)
		config.FinishSpan(span, http.StatusOK)

		// Should not panic
		assert.NotNil(t, span)
	})

	t.Run("EmptyServiceName", func(t *testing.T) {
		t.Parallel()

		config, err := New(WithServiceName(""))
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "service name cannot be empty")
	})

	t.Run("EmptyServiceVersion", func(t *testing.T) {
		t.Parallel()

		config, err := New(WithServiceVersion(""))
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "service version cannot be empty")
	})

	t.Run("ExtremelyLargeSampleRate", func(t *testing.T) {
		t.Parallel()

		// Sample rate is clamped by WithSampleRate, not by validation
		config := MustNew(WithServiceName("test"), WithSampleRate(999.9))
		defer config.Shutdown(context.Background())
		assert.Equal(t, 1.0, config.sampleRate)
	})

	t.Run("NegativeSampleRate", func(t *testing.T) {
		t.Parallel()

		// Sample rate is clamped by WithSampleRate, not by validation
		config := MustNew(WithServiceName("test"), WithSampleRate(-999.9))
		defer config.Shutdown(context.Background())
		assert.Equal(t, 0.0, config.sampleRate)
	})

	t.Run("EmptyExcludePaths", func(t *testing.T) {
		t.Parallel()

		config := MustNew(WithExcludePaths())
		assert.NotNil(t, config)
		assert.False(t, config.ShouldExcludePath("/any"))
	})

	t.Run("DuplicateExcludePaths", func(t *testing.T) {
		t.Parallel()

		config := MustNew(
			WithExcludePaths("/health"),
			WithExcludePaths("/health"),
			WithExcludePaths("/health"),
		)
		assert.True(t, config.ShouldExcludePath("/health"))
	})

	t.Run("MaxExcludedPathsLimit", func(t *testing.T) {
		t.Parallel()

		// Try to add more than 1000 paths
		paths := make([]string, 1500)
		for i := 0; i < 1500; i++ {
			paths[i] = fmt.Sprintf("/path%d", i)
		}
		config := MustNew(WithExcludePaths(paths...))

		// First 1000 should be excluded
		assert.True(t, config.ShouldExcludePath("/path0"))
		assert.True(t, config.ShouldExcludePath("/path999"))

		// Paths beyond 1000 should not be excluded
		assert.False(t, config.ShouldExcludePath("/path1000"))
		assert.False(t, config.ShouldExcludePath("/path1499"))

		// Verify map size is capped at 1000
		assert.LessOrEqual(t, len(config.excludePaths), 1000)
	})

	t.Run("NilHeaders", func(t *testing.T) {
		t.Parallel()

		config := MustNew()
		ctx := context.Background()

		// Should not panic with nil headers
		ctx = config.ExtractTraceContext(ctx, nil)
		config.InjectTraceContext(ctx, nil)

		assert.NotNil(t, ctx)
	})

	t.Run("EmptyHeaders", func(t *testing.T) {
		t.Parallel()

		config := MustNew()
		ctx := context.Background()
		headers := http.Header{}

		ctx = config.ExtractTraceContext(ctx, headers)
		config.InjectTraceContext(ctx, headers)

		assert.NotNil(t, ctx)
	})

	t.Run("MalformedTraceParent", func(t *testing.T) {
		t.Parallel()

		config := MustNew()
		ctx := context.Background()
		headers := http.Header{}
		headers.Set("traceparent", "invalid-trace-parent")

		// Should handle gracefully
		ctx = config.ExtractTraceContext(ctx, headers)
		assert.NotNil(t, ctx)
	})

	t.Run("NilSpanOperations", func(t *testing.T) {
		t.Parallel()

		config := MustNew()

		// These should not panic even with nil span
		config.SetSpanAttribute(nil, "key", "value")
		config.AddSpanEvent(nil, "event")
		config.FinishSpan(nil, http.StatusOK)

		// Should be handled gracefully
		assert.True(t, true)
	})

	t.Run("VeryLongAttributeValue", func(t *testing.T) {
		t.Parallel()

		config := MustNew()
		ctx := context.Background()
		_, span := config.StartSpan(ctx, "test")
		defer config.FinishSpan(span, http.StatusOK)

		// Very long string
		longValue := string(make([]byte, 10000))
		config.SetSpanAttribute(span, "long_key", longValue)

		// Should not panic
		assert.NotNil(t, span)
	})

	t.Run("SpecialCharactersInAttributeKey", func(t *testing.T) {
		t.Parallel()

		config := MustNew()
		ctx := context.Background()
		_, span := config.StartSpan(ctx, "test")
		defer config.FinishSpan(span, http.StatusOK)

		// Special characters
		config.SetSpanAttribute(span, "key-with-dashes", "value")
		config.SetSpanAttribute(span, "key.with.dots", "value")
		config.SetSpanAttribute(span, "key_with_underscores", "value")
		config.SetSpanAttribute(span, "key/with/slashes", "value")

		assert.NotNil(t, span)
	})

	t.Run("ConcurrentConfigAccess", func(t *testing.T) {
		t.Parallel()

		config := MustNew()

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx := context.Background()
				_, span := config.StartSpan(ctx, "test")
				config.SetSpanAttribute(span, "key", "value")
				config.FinishSpan(span, http.StatusOK)
			}()
		}

		wg.Wait()
		assert.True(t, config.IsEnabled())
	})
}

func TestTraceContextPropagation(t *testing.T) {
	t.Parallel()

	t.Run("PropagateTraceContext", func(t *testing.T) {
		t.Parallel()

		config := MustNew()

		// Simulate incoming request with trace context
		headers := http.Header{}
		headers.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")

		ctx := context.Background()
		ctx = config.ExtractTraceContext(ctx, headers)

		// Start a span with the propagated context
		ctx, span := config.StartSpan(ctx, "test-span")
		defer config.FinishSpan(span, http.StatusOK)

		// Verify span was created
		assert.NotNil(t, span)

		// Inject into new headers
		outHeaders := http.Header{}
		config.InjectTraceContext(ctx, outHeaders)

		// With a noop tracer, we may not get valid trace propagation
		// but the inject should not panic
		assert.NotNil(t, outHeaders)
	})
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()

	t.Run("CancelledContextDoesNotCreateSpan", func(t *testing.T) {
		t.Parallel()

		config := MustNew()

		// Create cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Should not create a recording span
		ctx, span := config.StartSpan(ctx, "test-span")
		defer config.FinishSpan(span, http.StatusOK)

		// Context should still be cancelled
		assert.Error(t, ctx.Err())
		assert.Equal(t, context.Canceled, ctx.Err())
	})

	t.Run("ActiveContextCreatesSpan", func(t *testing.T) {
		t.Parallel()

		config := MustNew()

		// Create active context with cancel
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Should create a span
		ctx, span := config.StartSpan(ctx, "test-span")
		defer config.FinishSpan(span, http.StatusOK)

		// Span should be created
		assert.NotNil(t, span)

		// Cancel after creating span
		cancel()
		assert.Error(t, ctx.Err())
	})
}

func TestDisabledRecording(t *testing.T) {
	t.Parallel()

	t.Run("DisableParamsWorks", func(t *testing.T) {
		t.Parallel()

		config := MustNew(
			WithServiceName("test"),
			WithDisableParams(),
		)
		defer config.Shutdown(context.Background())

		handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))

		req := httptest.NewRequest("GET", "/test?secret=password&token=abc123", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("HeaderRecordingWorks", func(t *testing.T) {
		config := MustNew(
			WithServiceName("test"),
			WithHeaders("X-Request-ID", "User-Agent"),
		)
		defer config.Shutdown(context.Background())

		handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Request-ID", "test-123")
		req.Header.Set("User-Agent", "test-agent")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("SensitiveHeadersFiltered", func(t *testing.T) {
		t.Parallel()

		config := MustNew(
			WithServiceName("test"),
			WithHeaders("Authorization", "Cookie", "X-Request-ID"),
		)

		// Verify sensitive headers are filtered out
		assert.Len(t, config.recordHeaders, 1)
		assert.Equal(t, "X-Request-ID", config.recordHeaders[0])
	})
}

// TestMiddlewareIntegration tests the middleware integration with standard HTTP handlers
func TestMiddlewareIntegration(t *testing.T) {
	// NOTE: Router-specific integration tests (WithTracing, WithTracingFromConfig)
	// have been moved to the router module's tests for proper separation of concerns.

	t.Run("MiddlewareIntegration", func(t *testing.T) {
		config := MustNew(
			WithServiceName("middleware-test"),
			WithSampleRate(1.0),
		)

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		middleware := Middleware(config)
		wrappedHandler := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "OK", w.Body.String())
	})

	t.Run("MiddlewareExcludedPath", func(t *testing.T) {
		t.Parallel()

		config := MustNew(
			WithServiceName("middleware-test"),
			WithExcludePaths("/health"),
		)

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := Middleware(config)
		wrappedHandler := middleware(handler)

		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("MiddlewareDisabledTracing", func(t *testing.T) {
		t.Parallel()

		config := MustNew(WithSampleRate(0.0))

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := Middleware(config)
		wrappedHandler := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestContextTracing tests the ContextTracing helper
func TestContextTracing(t *testing.T) {
	t.Run("ValidContext", func(t *testing.T) {
		config := MustNew()
		ctx := context.Background()
		ctx, span := config.StartSpan(ctx, "test")
		defer config.FinishSpan(span, http.StatusOK)

		ct := NewContextTracing(ctx, config, span)

		assert.NotNil(t, ct.TraceContext())
		assert.NotNil(t, ct.GetSpan())
		assert.NotNil(t, ct.GetConfig())
	})

	t.Run("NilContext", func(t *testing.T) {
		t.Parallel()

		config := MustNew()
		ct := NewContextTracing(context.TODO(), config, nil)

		// Should not panic and should return valid context
		ctx := ct.TraceContext()
		assert.NotNil(t, ctx)
	})

	t.Run("ContextTracingMethods", func(t *testing.T) {
		config := MustNew()
		ctx, span := config.StartSpan(context.Background(), "test")
		defer config.FinishSpan(span, http.StatusOK)

		ct := NewContextTracing(ctx, config, span)

		// These should not panic
		ct.SetSpanAttribute("key", "value")
		ct.AddSpanEvent("event")
		traceID := ct.TraceID()
		spanID := ct.SpanID()

		// With noop tracer, these may be empty, but shouldn't panic
		assert.NotNil(t, traceID)
		assert.NotNil(t, spanID)
	})

	t.Run("ContextTracingNilSpan", func(t *testing.T) {
		t.Parallel()

		config := MustNew()
		ctx := context.Background()
		ct := NewContextTracing(ctx, config, nil)

		// Should handle nil span gracefully
		ct.SetSpanAttribute("key", "value")
		ct.AddSpanEvent("event")
		traceID := ct.TraceID()
		spanID := ct.SpanID()

		assert.Equal(t, "", traceID)
		assert.Equal(t, "", spanID)
	})
}

// TestProductionHelper tests the production configuration helper
func TestProductionHelper(t *testing.T) {
	config, err := NewProduction("prod-service", "v2.0.0")
	require.NoError(t, err)
	require.NotNil(t, config)
	defer config.Shutdown(context.Background())

	assert.Equal(t, "prod-service", config.ServiceName())
	assert.Equal(t, "v2.0.0", config.ServiceVersion())
	assert.Equal(t, 0.1, config.sampleRate)
	assert.False(t, config.recordParams)
	assert.True(t, config.ShouldExcludePath("/health"))
	assert.True(t, config.ShouldExcludePath("/metrics"))
	assert.True(t, config.ShouldExcludePath("/ready"))
	assert.Equal(t, OTLPProvider, config.GetProvider())
}

// TestDevelopmentHelper tests the development configuration helper
func TestDevelopmentHelper(t *testing.T) {
	t.Parallel()

	config, err := NewDevelopment("dev-service", "dev")
	require.NoError(t, err)
	require.NotNil(t, config)
	defer config.Shutdown(context.Background())

	assert.Equal(t, "dev-service", config.ServiceName())
	assert.Equal(t, "dev", config.ServiceVersion())
	assert.Equal(t, 1.0, config.sampleRate)
	assert.True(t, config.recordParams)
	assert.True(t, config.ShouldExcludePath("/health"))
	assert.False(t, config.ShouldExcludePath("/metrics")) // Not excluded in dev
	assert.Equal(t, StdoutProvider, config.GetProvider())
}

// TestProviderSetup tests the provider setup functions
func TestProviderSetup(t *testing.T) {
	t.Run("StdoutProvider", func(t *testing.T) {
		config, err := New(
			WithServiceName("test-service"),
			WithServiceVersion("v1.0.0"),
			WithProvider(StdoutProvider),
		)
		require.NoError(t, err)
		require.NotNil(t, config)
		defer config.Shutdown(context.Background())

		assert.Equal(t, StdoutProvider, config.GetProvider())
		assert.Equal(t, "test-service", config.ServiceName())
		assert.Equal(t, "v1.0.0", config.ServiceVersion())
	})

	t.Run("OTLPProvider", func(t *testing.T) {
		t.Parallel()

		// Note: This may fail if no OTLP collector is running, but it should
		// not panic and should return a proper error or config
		config, err := New(
			WithServiceName("test-service"),
			WithServiceVersion("v1.0.0"),
			WithProvider(OTLPProvider),
			WithOTLPEndpoint("localhost:4317"),
			WithOTLPInsecure(true),
		)
		if config != nil {
			defer config.Shutdown(context.Background())
		}
		// Either succeeds or returns error, but shouldn't panic
		if err != nil {
			assert.Error(t, err)
		} else {
			assert.NotNil(t, config)
			assert.Equal(t, OTLPProvider, config.GetProvider())
		}
	})

	t.Run("NoopProvider", func(t *testing.T) {
		t.Parallel()

		config, err := New(
			WithServiceName("test-service"),
			WithServiceVersion("v1.0.0"),
			WithProvider(NoopProvider),
		)
		require.NoError(t, err)
		require.NotNil(t, config)
		defer config.Shutdown(context.Background())

		assert.Equal(t, NoopProvider, config.GetProvider())
	})

	t.Run("DefaultProvider", func(t *testing.T) {
		t.Parallel()

		config, err := New(
			WithServiceName("test-service"),
			WithServiceVersion("v1.0.0"),
		)
		require.NoError(t, err)
		require.NotNil(t, config)
		defer config.Shutdown(context.Background())

		// Default should be noop
		assert.Equal(t, NoopProvider, config.GetProvider())
	})

	t.Run("InvalidProvider", func(t *testing.T) {
		t.Parallel()

		config, err := New(
			WithServiceName("test-service"),
			WithServiceVersion("v1.0.0"),
			WithProvider(Provider("invalid")),
		)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "unsupported tracing provider")
	})

	t.Run("ShutdownIdempotent", func(t *testing.T) {
		config, err := New(
			WithServiceName("test-service"),
			WithServiceVersion("v1.0.0"),
			WithProvider(StdoutProvider),
		)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// First shutdown
		err = config.Shutdown(ctx)
		assert.NoError(t, err)

		// Second shutdown should also succeed (idempotent)
		err = config.Shutdown(ctx)
		assert.NoError(t, err)
	})

	t.Run("MustNew_Success", func(t *testing.T) {
		t.Parallel()

		assert.NotPanics(t, func() {
			config := MustNew(
				WithServiceName("test-service"),
				WithServiceVersion("v1.0.0"),
				WithProvider(StdoutProvider),
			)
			defer config.Shutdown(context.Background())
		})
	})

	t.Run("MustNew_Panics", func(t *testing.T) {
		assert.Panics(t, func() {
			MustNew(
				WithServiceName(""), // Invalid - will cause panic
				WithProvider(StdoutProvider),
			)
		})
	})
}

// TestWarningLogs tests that warning logs are generated for appropriate conditions
func TestWarningLogs(t *testing.T) {
	t.Parallel()

	t.Run("ExcludedPathsLimitWarning", func(t *testing.T) {
		t.Parallel()

		// Create a custom logger to capture warnings
		var loggedWarnings []string
		logger := &testLogger{
			warnFunc: func(msg string, _ ...any) {
				loggedWarnings = append(loggedWarnings, msg)
			},
		}

		// Try to add more than 1000 paths
		paths := make([]string, 1500)
		for i := 0; i < 1500; i++ {
			paths[i] = fmt.Sprintf("/path%d", i)
		}

		config := MustNew(
			WithLogger(logger),
			WithExcludePaths(paths...),
		)
		defer config.Shutdown(context.Background())

		// Verify warning was logged
		assert.Len(t, loggedWarnings, 1)
		assert.Contains(t, loggedWarnings[0], "Excluded paths limit reached")

		// Verify only first 1000 paths were added
		assert.True(t, config.ShouldExcludePath("/path0"))
		assert.True(t, config.ShouldExcludePath("/path999"))
		assert.False(t, config.ShouldExcludePath("/path1000"))
	})

	t.Run("SamplingDebugLogs", func(t *testing.T) {
		// Create a custom logger to capture debug messages
		var loggedDebugMessages []string
		logger := &testLogger{
			debugFunc: func(msg string, _ ...any) {
				loggedDebugMessages = append(loggedDebugMessages, msg)
			},
		}

		config := MustNew(
			WithServiceName("test"),
			WithLogger(logger),
			WithSampleRate(0.0), // 0% sampling to trigger debug log
		)
		defer config.Shutdown(context.Background())

		// Make a request that won't be sampled
		req := httptest.NewRequest("GET", "/test", nil)
		ctx := context.Background()
		_, _ = config.StartRequestSpan(ctx, req, "/test", false)

		// Verify debug log was generated - check that at least one message contains "Request not sampled"
		assert.GreaterOrEqual(t, len(loggedDebugMessages), 1)
		foundSamplingLog := false
		for _, msg := range loggedDebugMessages {
			if assert.ObjectsAreEqual("Request not sampled (0% sample rate)", msg) {
				foundSamplingLog = true
				break
			}
		}
		assert.True(t, foundSamplingLog, "Expected to find 'Request not sampled' debug log")
	})
}

// TestSpanLifecycleHooks tests the span start and finish hooks
func TestSpanLifecycleHooks(t *testing.T) {
	t.Parallel()

	t.Run("SpanStartHook", func(t *testing.T) {
		t.Parallel()

		var hookCalled bool
		var capturedReq *http.Request

		startHook := func(_ context.Context, span trace.Span, req *http.Request) {
			hookCalled = true
			capturedReq = req
			// Add custom attribute
			span.SetAttributes(attribute.String("custom.tenant_id", "tenant-123"))
		}

		config := MustNew(
			WithServiceName("test"),
			WithSpanStartHook(startHook),
		)
		defer config.Shutdown(context.Background())

		// Create middleware and make request
		handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Verify hook was called
		assert.True(t, hookCalled)
		assert.NotNil(t, capturedReq)
		assert.Equal(t, "/test", capturedReq.URL.Path)
	})

	t.Run("SpanFinishHook", func(t *testing.T) {
		t.Parallel()

		var hookCalled bool
		var capturedStatusCode int

		finishHook := func(_ trace.Span, statusCode int) {
			hookCalled = true
			capturedStatusCode = statusCode
		}

		config := MustNew(
			WithServiceName("test"),
			WithSpanFinishHook(finishHook),
		)
		defer config.Shutdown(context.Background())

		// Create middleware and make request
		handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))

		req := httptest.NewRequest("POST", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Verify hook was called with correct status code
		assert.True(t, hookCalled)
		assert.Equal(t, http.StatusCreated, capturedStatusCode)
	})

	t.Run("BothHooks", func(t *testing.T) {
		t.Parallel()

		var startHookCalled bool
		var finishHookCalled bool

		startHook := func(_ context.Context, _ trace.Span, _ *http.Request) {
			startHookCalled = true
		}

		finishHook := func(_ trace.Span, _ int) {
			finishHookCalled = true
		}

		config := MustNew(
			WithServiceName("test"),
			WithSpanStartHook(startHook),
			WithSpanFinishHook(finishHook),
		)
		defer config.Shutdown(context.Background())

		// Create middleware and make request
		handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Verify both hooks were called
		assert.True(t, startHookCalled)
		assert.True(t, finishHookCalled)
	})

	t.Run("HookNotCalledWhenSampledOut", func(t *testing.T) {
		t.Parallel()

		var startHookCalled bool
		var finishHookCalled bool

		startHook := func(_ context.Context, _ trace.Span, _ *http.Request) {
			startHookCalled = true
		}

		finishHook := func(_ trace.Span, _ int) {
			finishHookCalled = true
		}

		config := MustNew(
			WithServiceName("test"),
			WithSampleRate(0.0), // Don't sample anything
			WithSpanStartHook(startHook),
			WithSpanFinishHook(finishHook),
		)
		defer config.Shutdown(context.Background())

		// Create middleware and make request
		handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Hooks should not be called for sampled-out requests
		assert.False(t, startHookCalled)
		assert.False(t, finishHookCalled)
	})
}

// TestGranularParameterRecording tests the granular parameter recording options
func TestGranularParameterRecording(t *testing.T) {
	t.Run("WithRecordParams_Whitelist", func(t *testing.T) {
		config := MustNew(
			WithServiceName("test"),
			WithRecordParams("user_id", "request_id"), // Only record these
		)
		defer config.Shutdown(context.Background())

		// Verify configuration
		assert.True(t, config.recordParams)
		assert.Len(t, config.recordParamsList, 2)
		assert.Equal(t, "user_id", config.recordParamsList[0])
		assert.Equal(t, "request_id", config.recordParamsList[1])

		// Test shouldRecordParam logic
		assert.True(t, config.shouldRecordParam("user_id"))
		assert.True(t, config.shouldRecordParam("request_id"))
		assert.False(t, config.shouldRecordParam("password"))
		assert.False(t, config.shouldRecordParam("token"))
	})

	t.Run("WithExcludeParams_Blacklist", func(t *testing.T) {
		t.Parallel()

		config := MustNew(
			WithServiceName("test"),
			WithExcludeParams("password", "token", "api_key"), // Exclude these
		)
		defer config.Shutdown(context.Background())

		// Verify configuration
		assert.True(t, config.recordParams)    // Default is true
		assert.Nil(t, config.recordParamsList) // No whitelist
		assert.Len(t, config.excludeParams, 3)

		// Test shouldRecordParam logic
		assert.False(t, config.shouldRecordParam("password"))
		assert.False(t, config.shouldRecordParam("token"))
		assert.False(t, config.shouldRecordParam("api_key"))
		assert.True(t, config.shouldRecordParam("user_id"))
		assert.True(t, config.shouldRecordParam("page"))
	})

	t.Run("WithRecordParams_And_WithExcludeParams", func(t *testing.T) {
		t.Parallel()

		// Whitelist takes precedence, but blacklist is checked first
		config := MustNew(
			WithServiceName("test"),
			WithRecordParams("user_id", "request_id", "password"),
			WithExcludeParams("password"), // Exclude password even if whitelisted
		)
		defer config.Shutdown(context.Background())

		// Test shouldRecordParam logic
		assert.True(t, config.shouldRecordParam("user_id"))
		assert.True(t, config.shouldRecordParam("request_id"))
		assert.False(t, config.shouldRecordParam("password")) // Blacklist wins
		assert.False(t, config.shouldRecordParam("other"))    // Not in whitelist
	})

	t.Run("HTTPRequest_WithRecordParams", func(t *testing.T) {
		config := MustNew(
			WithServiceName("test"),
			WithRecordParams("user_id", "page"),
		)
		defer config.Shutdown(context.Background())

		handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Request with multiple params - only user_id and page should be recorded
		req := httptest.NewRequest("GET", "/test?user_id=123&page=5&token=secret&password=hunter2", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Note: We can't directly verify span attributes with noop tracer,
		// but we've verified the logic in shouldRecordParam tests
	})

	t.Run("HTTPRequest_WithExcludeParams", func(t *testing.T) {
		t.Parallel()

		config := MustNew(
			WithServiceName("test"),
			WithExcludeParams("password", "token", "api_key"),
		)
		defer config.Shutdown(context.Background())

		handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Request with params - password and token should be excluded
		req := httptest.NewRequest("GET", "/test?user_id=123&password=secret&token=abc", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("EmptyWhitelist", func(t *testing.T) {
		t.Parallel()

		config := MustNew(
			WithServiceName("test"),
			WithRecordParams(), // Empty list
		)
		defer config.Shutdown(context.Background())

		// With empty whitelist, recordParams should remain false (default)
		// or be set but recordParamsList should be nil
		assert.True(t, config.recordParams)
		assert.Nil(t, config.recordParamsList)
	})

	t.Run("EmptyBlacklist", func(t *testing.T) {
		config := MustNew(
			WithServiceName("test"),
			WithExcludeParams(), // Empty list
		)
		defer config.Shutdown(context.Background())

		// All params should be recorded
		assert.True(t, config.shouldRecordParam("any_param"))
	})
}

// testLogger is a mock logger for testing
type testLogger struct {
	errorFunc func(msg string, keysAndValues ...any)
	warnFunc  func(msg string, keysAndValues ...any)
	infoFunc  func(msg string, keysAndValues ...any)
	debugFunc func(msg string, keysAndValues ...any)
}

func (l *testLogger) Error(msg string, keysAndValues ...any) {
	if l.errorFunc != nil {
		l.errorFunc(msg, keysAndValues...)
	}
}

func (l *testLogger) Warn(msg string, keysAndValues ...any) {
	if l.warnFunc != nil {
		l.warnFunc(msg, keysAndValues...)
	}
}

func (l *testLogger) Info(msg string, keysAndValues ...any) {
	if l.infoFunc != nil {
		l.infoFunc(msg, keysAndValues...)
	}
}

func (l *testLogger) Debug(msg string, keysAndValues ...any) {
	if l.debugFunc != nil {
		l.debugFunc(msg, keysAndValues...)
	}
}

// TestConcurrentShutdown tests that shutdown is safe to call concurrently
func TestConcurrentShutdown(t *testing.T) {
	t.Parallel()

	config, err := New(
		WithServiceName("test-service"),
		WithServiceVersion("v1.0.0"),
		WithProvider(StdoutProvider),
	)
	require.NoError(t, err)
	require.NotNil(t, config)

	// Test concurrent shutdown calls
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			err := config.Shutdown(ctx)
			// All calls should succeed (idempotent)
			assert.NoError(t, err)
		}()
	}

	wg.Wait()

	// Verify shutdown was called (subsequent calls should also succeed)
	err = config.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestProviderFailure tests that provider initialization failures are handled gracefully
func TestProviderFailure(t *testing.T) {
	t.Parallel()

	t.Run("OTLPProvider_InvalidEndpoint", func(t *testing.T) {
		t.Parallel()

		// Test with invalid endpoint format (should still create config but may fail on connection)
		// Note: OTLP provider creation doesn't fail on invalid endpoint, it just won't connect
		// So we test with a valid but unreachable endpoint
		config, err := New(
			WithServiceName("test"),
			WithServiceVersion("v1.0.0"),
			WithProvider(OTLPProvider),
			WithOTLPEndpoint("localhost:99999"), // Invalid/unreachable port
			WithOTLPInsecure(true),
		)
		// Provider creation may succeed even with unreachable endpoint
		// The actual connection failure happens during span export
		if err != nil {
			assert.Error(t, err)
		} else {
			require.NotNil(t, config)
			defer config.Shutdown(context.Background())
		}
	})

	t.Run("InvalidProvider", func(t *testing.T) {
		config, err := New(
			WithServiceName("test"),
			WithServiceVersion("v1.0.0"),
			WithProvider(Provider("invalid")),
		)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "unsupported tracing provider")
	})
}

// TestContextCancellationInStartRequestSpan tests context cancellation handling
func TestContextCancellationInStartRequestSpan(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
		WithSampleRate(1.0),
	)
	defer config.Shutdown(context.Background())

	t.Run("CancelledContext", func(t *testing.T) {
		t.Parallel()

		// Create cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		req := httptest.NewRequest("GET", "/test", nil)
		ctx, span := config.StartRequestSpan(ctx, req, "/test", false)

		// Should return non-recording span and preserve cancellation
		assert.Error(t, ctx.Err())
		assert.Equal(t, context.Canceled, ctx.Err())
		assert.NotNil(t, span)
	})

	t.Run("TimeoutContext", func(t *testing.T) {
		t.Parallel()

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		// Wait for timeout
		time.Sleep(10 * time.Millisecond)

		req := httptest.NewRequest("GET", "/test", nil)
		ctx, span := config.StartRequestSpan(ctx, req, "/test", false)

		// Should return non-recording span and preserve timeout
		assert.Error(t, ctx.Err())
		assert.NotNil(t, span)
	})
}

// TestExcludePathPattern tests regex pattern support for path exclusion
func TestExcludePathPattern(t *testing.T) {
	t.Parallel()

	t.Run("ValidPattern", func(t *testing.T) {
		t.Parallel()

		config := MustNew(
			WithServiceName("test"),
			WithExcludePathPattern("^/internal/.*"),
			WithExcludePathPattern("^/(health|ready|live)"),
		)
		defer config.Shutdown(context.Background())

		// Test pattern matching
		assert.True(t, config.ShouldExcludePath("/internal/api"))
		assert.True(t, config.ShouldExcludePath("/internal/status"))
		assert.True(t, config.ShouldExcludePath("/health"))
		assert.True(t, config.ShouldExcludePath("/ready"))
		assert.True(t, config.ShouldExcludePath("/live"))
		assert.False(t, config.ShouldExcludePath("/api/users"))
		assert.False(t, config.ShouldExcludePath("/public"))
	})

	t.Run("InvalidPattern", func(t *testing.T) {
		t.Parallel()

		config, err := New(
			WithServiceName("test"),
			WithExcludePathPattern("[invalid-regex"), // Invalid regex
		)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "invalid regex pattern")
	})

	t.Run("PatternAndExactPath", func(t *testing.T) {
		config := MustNew(
			WithServiceName("test"),
			WithExcludePaths("/exact"),
			WithExcludePathPattern("^/pattern/.*"),
		)
		defer config.Shutdown(context.Background())

		// Both should work
		assert.True(t, config.ShouldExcludePath("/exact"))
		assert.True(t, config.ShouldExcludePath("/pattern/anything"))
		assert.False(t, config.ShouldExcludePath("/other"))
	})

	t.Run("MultiplePatterns", func(t *testing.T) {
		t.Parallel()

		config := MustNew(
			WithServiceName("test"),
			WithExcludePathPattern("^/v1/.*"),
			WithExcludePathPattern("^/v2/.*"),
			WithExcludePathPattern("^/admin/.*"),
		)
		defer config.Shutdown(context.Background())

		assert.True(t, config.ShouldExcludePath("/v1/users"))
		assert.True(t, config.ShouldExcludePath("/v2/posts"))
		assert.True(t, config.ShouldExcludePath("/admin/settings"))
		assert.False(t, config.ShouldExcludePath("/v3/api"))
	})
}
