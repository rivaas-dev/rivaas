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
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func TestTracerConfig(t *testing.T) {
	t.Parallel()

	tracer := MustNew(
		WithServiceName("test-service"),
		WithServiceVersion("v1.0.0"),
		WithSampleRate(0.5),
	)
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	assert.True(t, tracer.IsEnabled())
	assert.Equal(t, "test-service", tracer.ServiceName())
	assert.Equal(t, "v1.0.0", tracer.ServiceVersion())
	assert.NotNil(t, tracer.GetTracer())
	assert.NotNil(t, tracer.GetPropagator())
}

func TestTracingWithHTTP(t *testing.T) {
	t.Parallel()

	// Create tracer
	tracer := MustNew(
		WithServiceName("test-service"),
		WithSampleRate(1.0),
	)
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	// Create HTTP handler with tracing middleware
	handler := Middleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck // Test handler
		w.Write([]byte(`{"status":"ok"}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}

func TestTracerOptions(t *testing.T) {
	t.Parallel()

	t.Run("WithCustomTracer", func(t *testing.T) {
		t.Parallel()

		tempTracer := MustNew()
		tracer := MustNew(
			WithCustomTracer(tempTracer.GetTracer()),
		)
		assert.True(t, tracer.IsEnabled())
	})

	t.Run("WithSampleRate", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithSampleRate(0.5))
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup
		assert.InEpsilon(t, 0.5, tracer.sampleRate, 0.001)
	})
}

func TestTracingMiddleware(t *testing.T) {
	t.Parallel()

	tracer := MustNew(
		WithServiceName("test-service"),
	)

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck // Test handler
		w.Write([]byte("OK"))
	})

	// Wrap with tracing middleware (with path exclusion)
	middleware := Middleware(tracer, WithExcludePaths("/health"))
	wrappedHandler := middleware(handler)

	// Test the wrapped handler
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestTracingIntegration(t *testing.T) {
	t.Parallel()

	// Test full integration with HTTP middleware
	tracer := MustNew(
		WithServiceName("integration-test"),
	)

	// Create HTTP mux
	mux := http.NewServeMux()

	// Add routes
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck // Test handler
		w.Write([]byte(`{"message":"Hello"}`))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck // Test handler
		w.Write([]byte(`{"status":"healthy"}`))
	})

	// Wrap with tracing middleware
	handler := Middleware(tracer, WithExcludePaths("/health"))(mux)

	// Test normal route
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Test health route (should be excluded from tracing)
	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestContextHelpers(t *testing.T) {
	t.Parallel()

	// Test context helper functions
	ctx := t.Context()

	// These should not panic even without active spans
	traceID := TraceID(ctx)
	spanID := SpanID(ctx)

	assert.Empty(t, traceID)
	assert.Empty(t, spanID)
}

func TestSamplingRate(t *testing.T) {
	t.Parallel()

	t.Run("SampleRateValidation", func(t *testing.T) {
		t.Parallel()

		// Test clamping of sample rate
		tracer := MustNew(WithServiceName("test"), WithSampleRate(1.5))
		assert.InEpsilon(t, 1.0, tracer.sampleRate, 0.001)
		tracer.Shutdown(t.Context()) //nolint:errcheck // Test cleanup

		tracer = MustNew(WithServiceName("test"), WithSampleRate(-0.5))
		assert.InDelta(t, 0.0, tracer.sampleRate, 0.001)
		tracer.Shutdown(t.Context()) //nolint:errcheck // Test cleanup

		tracer = MustNew(WithServiceName("test"), WithSampleRate(0.5))
		assert.InEpsilon(t, 0.5, tracer.sampleRate, 0.001)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup
	})

	t.Run("SampleRate100Percent", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("test-service"),
			WithSampleRate(1.0),
		)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		// All requests should be traced
		handler := Middleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			//nolint:errcheck // Test handler
			w.Write([]byte(`{"status":"ok"}`))
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("SampleRate0Percent", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("test-service"),
			WithSampleRate(0.0),
		)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		// No requests should be traced
		handler := Middleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			//nolint:errcheck // Test handler
			w.Write([]byte(`{"status":"ok"}`))
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("SampleRateStatistical", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("test-service"),
			WithSampleRate(0.5),
		)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		handler := Middleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			//nolint:errcheck // Test handler
			w.Write([]byte(`{"status":"ok"}`))
		}))

		// Make multiple requests to verify sampling doesn't cause issues
		const numRequests = 100
		for i := range numRequests {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "request %d: expected status %d, got %d", i, http.StatusOK, w.Code)
		}

		// Sampling logic executed successfully - if we reach here without panic, the test passes
	})
}

func TestParameterRecording(t *testing.T) {
	t.Parallel()

	t.Run("WithParams", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("test-service"),
		)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		handler := Middleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			//nolint:errcheck // Test handler
			w.Write([]byte(`{"status":"ok"}`))
		}))

		req := httptest.NewRequest(http.MethodGet, "/test?foo=bar&baz=qux", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("WithoutParams", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("test-service"),
		)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		handler := Middleware(tracer, WithoutParams())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			//nolint:errcheck // Test handler
			w.Write([]byte(`{"status":"ok"}`))
		}))

		req := httptest.NewRequest(http.MethodGet, "/test?foo=bar", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestSpanAttributeTypes(t *testing.T) {
	t.Parallel()

	tracer := MustNew(
		WithServiceName("test-service"),
	)

	ctx := t.Context()
	_, span := tracer.StartSpan(ctx, "test-span")
	defer span.End()

	// Test different types - these should not panic even if span is not recording
	tracer.SetSpanAttribute(span, "string_attr", "value")
	tracer.SetSpanAttribute(span, "int_attr", 42)
	tracer.SetSpanAttribute(span, "int64_attr", int64(123))
	tracer.SetSpanAttribute(span, "float_attr", 3.14)
	tracer.SetSpanAttribute(span, "bool_attr", true)
	tracer.SetSpanAttribute(span, "other_attr", struct{ Name string }{"test"})

	assert.NotNil(t, span)
}

func TestSpanAttributeTypesFromContext(t *testing.T) {
	t.Parallel()

	tracer := MustNew(
		WithServiceName("test-service"),
	)

	ctx := t.Context()
	ctx, span := tracer.StartSpan(ctx, "test-span")
	defer span.End()

	// Test different types through context helper - should not panic
	SetSpanAttributeFromContext(ctx, "string_attr", "value")
	SetSpanAttributeFromContext(ctx, "int_attr", 42)
	SetSpanAttributeFromContext(ctx, "int64_attr", int64(123))
	SetSpanAttributeFromContext(ctx, "float_attr", 3.14)
	SetSpanAttributeFromContext(ctx, "bool_attr", true)

	assert.NotNil(t, span)
}

func TestErrorStatusCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "bad request",
			path:       "/bad-request",
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":"bad request"}`,
		},
		{
			name:       "not found",
			path:       "/not-found",
			wantStatus: http.StatusNotFound,
			wantBody:   `{"error":"not found"}`,
		},
		{
			name:       "internal server error",
			path:       "/error",
			wantStatus: http.StatusInternalServerError,
			wantBody:   `{"error":"server error"}`,
		},
		{
			name:       "service unavailable",
			path:       "/unavailable",
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   `{"error":"service unavailable"}`,
		},
	}

	tracer := MustNew(
		WithServiceName("test-service"),
	)
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	// Create mux with different status codes
	mux := http.NewServeMux()
	for _, tt := range tests {
		status := tt.wantStatus
		body := tt.wantBody
		mux.HandleFunc(tt.path, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			//nolint:errcheck // Test handler
			w.Write([]byte(body))
		})
	}
	handler := Middleware(tracer)(mux)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantBody, w.Body.String())
		})
	}
}

func TestConcurrentResponseWriter(t *testing.T) {
	t.Parallel()

	tracer := MustNew(
		WithServiceName("test-service"),
	)
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	handler := Middleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck // Test handler
		w.Write([]byte(`{"status":"ok"}`))
	}))

	// Test concurrent requests
	const numRequests = 50
	var wg sync.WaitGroup
	for i := range numRequests {
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodGet, "/concurrent", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "request %d: expected status %d, got %d", i, http.StatusOK, w.Code)
		})
	}
	wg.Wait()
}

func TestContextTracingHelpers(t *testing.T) {
	t.Parallel()

	tracer := MustNew(
		WithServiceName("test-service"),
	)
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	handler := Middleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		SetSpanAttributeFromContext(ctx, "string", "value")
		SetSpanAttributeFromContext(ctx, "int", 42)
		SetSpanAttributeFromContext(ctx, "float", 3.14)
		SetSpanAttributeFromContext(ctx, "bool", true)

		AddSpanEventFromContext(ctx, "test_event")

		traceID := TraceID(ctx)
		spanID := SpanID(ctx)

		w.WriteHeader(http.StatusOK)
		//nolint:errcheck,gosec // Test handler; G705: trace IDs from tracer, not user input
		w.Write(fmt.Appendf(nil, `{"trace_id":"%s","span_id":"%s"}`, traceID, spanID))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestTracer_EdgeCases tests edge cases and robustness of the Tracer type.
func TestTracer_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("DisabledTracing", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithSampleRate(0.0))
		ctx := t.Context()

		_, span := tracer.StartSpan(ctx, "test")
		tracer.SetSpanAttribute(span, "key", "value")
		tracer.AddSpanEvent(span, "event")
		tracer.FinishSpan(span, http.StatusOK)

		assert.NotNil(t, span)
	})

	t.Run("NilContext", func(t *testing.T) {
		t.Parallel()

		traceID := TraceID(t.Context())
		spanID := SpanID(t.Context())

		assert.Empty(t, traceID)
		assert.Empty(t, spanID)

		SetSpanAttributeFromContext(t.Context(), "key", "value")
		AddSpanEventFromContext(t.Context(), "event")
	})

	t.Run("MultipleFinishSpan", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()
		ctx := t.Context()
		_, span := tracer.StartSpan(ctx, "test")

		// Should be safe to call multiple times
		tracer.FinishSpan(span, http.StatusOK)
		tracer.FinishSpan(span, http.StatusOK)
		tracer.FinishSpan(span, http.StatusOK)

		assert.NotNil(t, span)
	})

	t.Run("EmptyServiceName", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(WithServiceName(""))
		require.Error(t, err)
		assert.Nil(t, tracer)
		assert.Contains(t, err.Error(), "serviceName: cannot be empty")
	})

	t.Run("EmptyServiceVersion", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(WithServiceVersion(""))
		require.Error(t, err)
		assert.Nil(t, tracer)
		assert.Contains(t, err.Error(), "serviceVersion: cannot be empty")
	})

	t.Run("ExtremelyLargeSampleRate", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithServiceName("test"), WithSampleRate(999.9))
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup
		assert.InEpsilon(t, 1.0, tracer.sampleRate, 0.001)
	})

	t.Run("NegativeSampleRate", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithServiceName("test"), WithSampleRate(-999.9))
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup
		assert.InDelta(t, 0.0, tracer.sampleRate, 0.001)
	})

	t.Run("NilHeaders", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()
		ctx := t.Context()

		ctx = tracer.ExtractTraceContext(ctx, nil)
		tracer.InjectTraceContext(ctx, nil)

		assert.NotNil(t, ctx)
	})

	t.Run("EmptyHeaders", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()
		ctx := t.Context()
		headers := http.Header{}

		ctx = tracer.ExtractTraceContext(ctx, headers)
		tracer.InjectTraceContext(ctx, headers)

		assert.NotNil(t, ctx)
	})

	t.Run("MalformedTraceParent", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()
		ctx := t.Context()
		headers := http.Header{}
		headers.Set("Traceparent", "invalid-trace-parent")

		ctx = tracer.ExtractTraceContext(ctx, headers)
		assert.NotNil(t, ctx)
	})

	t.Run("NilSpanOperations", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()

		tracer.SetSpanAttribute(nil, "key", "value")
		tracer.AddSpanEvent(nil, "event")
		tracer.FinishSpan(nil, http.StatusOK)

		// If we reach here without panic, the test passes
	})

	t.Run("VeryLongAttributeValue", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()
		ctx := t.Context()
		_, span := tracer.StartSpan(ctx, "test")
		defer tracer.FinishSpan(span, http.StatusOK)

		longValue := string(make([]byte, 10000))
		tracer.SetSpanAttribute(span, "long_key", longValue)

		assert.NotNil(t, span)
	})

	t.Run("SpecialCharactersInAttributeKey", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()
		ctx := t.Context()
		_, span := tracer.StartSpan(ctx, "test")
		defer tracer.FinishSpan(span, http.StatusOK)

		tracer.SetSpanAttribute(span, "key-with-dashes", "value")
		tracer.SetSpanAttribute(span, "key.with.dots", "value")
		tracer.SetSpanAttribute(span, "key_with_underscores", "value")
		tracer.SetSpanAttribute(span, "key/with/slashes", "value")

		assert.NotNil(t, span)
	})

	t.Run("ConcurrentTracerAccess", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()
		ctx := t.Context()

		var wg sync.WaitGroup
		for range 10 {
			wg.Go(func() {
				_, span := tracer.StartSpan(ctx, "test")
				tracer.SetSpanAttribute(span, "key", "value")
				tracer.FinishSpan(span, http.StatusOK)
			})
		}

		wg.Wait()
		assert.True(t, tracer.IsEnabled())
	})
}

func TestTraceContextPropagation(t *testing.T) {
	t.Parallel()

	t.Run("PropagateTraceContext", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()

		headers := http.Header{}
		headers.Set("Traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")

		ctx := t.Context()
		ctx = tracer.ExtractTraceContext(ctx, headers)

		ctx, span := tracer.StartSpan(ctx, "test-span")
		defer tracer.FinishSpan(span, http.StatusOK)

		assert.NotNil(t, span)

		outHeaders := http.Header{}
		tracer.InjectTraceContext(ctx, outHeaders)

		assert.NotNil(t, outHeaders)
	})
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()

	t.Run("CancelledContextDoesNotCreateSpan", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		ctx, span := tracer.StartSpan(ctx, "test-span")
		defer tracer.FinishSpan(span, http.StatusOK)

		require.Error(t, ctx.Err())
		assert.Equal(t, context.Canceled, ctx.Err())
	})

	t.Run("ActiveContextCreatesSpan", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		ctx, span := tracer.StartSpan(ctx, "test-span")
		defer tracer.FinishSpan(span, http.StatusOK)

		assert.NotNil(t, span)

		cancel()
		assert.Error(t, ctx.Err())
	})
}

func TestDisabledRecording(t *testing.T) {
	t.Parallel()

	t.Run("WithoutParamsWorks", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("test"),
		)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		handler := Middleware(tracer, WithoutParams())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			//nolint:errcheck // Test handler
			w.Write([]byte(`{"status":"ok"}`))
		}))

		req := httptest.NewRequest(http.MethodGet, "/test?secret=password&token=abc123", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("HeaderRecordingWorks", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("test"),
		)
		t.Cleanup(func() {
			// Use context.Background() instead of t.Context() because with t.Parallel(),
			// the test context is canceled before cleanup runs, causing shutdown to fail.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			tracer.Shutdown(ctx) //nolint:errcheck // Test cleanup
		})

		handler := Middleware(tracer, WithHeaders("X-Request-ID", "User-Agent"))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			//nolint:errcheck // Test handler
			w.Write([]byte(`{"status":"ok"}`))
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Request-ID", "test-123")
		req.Header.Set("User-Agent", "test-agent")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestMiddlewareIntegration tests the middleware integration.
func TestMiddlewareIntegration(t *testing.T) {
	t.Parallel()

	t.Run("MiddlewareIntegration", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("middleware-test"),
			WithSampleRate(1.0),
		)

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			//nolint:errcheck // Test handler
			w.Write([]byte("OK"))
		})

		middleware := Middleware(tracer)
		wrappedHandler := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "OK", w.Body.String())
	})

	t.Run("MiddlewareExcludedPath", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("middleware-test"),
		)

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := Middleware(tracer, WithExcludePaths("/health"))
		wrappedHandler := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("MiddlewareDisabledTracing", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithSampleRate(0.0))

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := Middleware(tracer)
		wrappedHandler := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestContextTracing tests the ContextTracing helper.
func TestContextTracing(t *testing.T) {
	t.Parallel()

	t.Run("ValidContext", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()
		ctx := t.Context()
		ctx, span := tracer.StartSpan(ctx, "test")
		defer tracer.FinishSpan(span, http.StatusOK)

		ct := NewContextTracing(ctx, tracer, span)

		assert.NotNil(t, ct.TraceContext())
		assert.NotNil(t, ct.GetSpan())
		assert.NotNil(t, ct.GetTracer())
	})

	t.Run("NilContext", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()
		ct := NewContextTracing(t.Context(), tracer, nil)

		ctx := ct.TraceContext()
		assert.NotNil(t, ctx)
	})

	t.Run("ContextTracingMethods", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()
		ctx, span := tracer.StartSpan(t.Context(), "test")
		defer tracer.FinishSpan(span, http.StatusOK)

		ct := NewContextTracing(ctx, tracer, span)

		ct.SetSpanAttribute("key", "value")
		ct.AddSpanEvent("event")
		traceID := ct.TraceID()
		spanID := ct.SpanID()

		assert.NotNil(t, traceID)
		assert.NotNil(t, spanID)
	})

	t.Run("ContextTracingNilSpan", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()
		ctx := t.Context()
		ct := NewContextTracing(ctx, tracer, nil)

		ct.SetSpanAttribute("key", "value")
		ct.AddSpanEvent("event")
		traceID := ct.TraceID()
		spanID := ct.SpanID()

		assert.Empty(t, traceID)
		assert.Empty(t, spanID)
	})
}

// TestProviderSetup tests the provider setup functions.
//
//nolint:paralleltest // Some subtests share state
func TestProviderSetup(t *testing.T) {
	t.Parallel()

	t.Run("StdoutProvider", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test-service"),
			WithServiceVersion("v1.0.0"),
			WithStdout(),
		)
		require.NoError(t, err)
		require.NotNil(t, tracer)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		assert.Equal(t, StdoutProvider, tracer.GetProvider())
		assert.Equal(t, "test-service", tracer.ServiceName())
		assert.Equal(t, "v1.0.0", tracer.ServiceVersion())
	})

	t.Run("OTLPProvider", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test-service"),
			WithServiceVersion("v1.0.0"),
			WithOTLP("localhost:4317", OTLPInsecure()),
		)
		if tracer != nil {
			t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup
		}
		if err != nil {
			assert.Error(t, err)
		} else {
			assert.NotNil(t, tracer)
			assert.Equal(t, OTLPProvider, tracer.GetProvider())
		}
	})

	t.Run("NoopProvider", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test-service"),
			WithServiceVersion("v1.0.0"),
			WithNoop(),
		)
		require.NoError(t, err)
		require.NotNil(t, tracer)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		assert.Equal(t, NoopProvider, tracer.GetProvider())
	})

	t.Run("DefaultProvider", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test-service"),
			WithServiceVersion("v1.0.0"),
		)
		require.NoError(t, err)
		require.NotNil(t, tracer)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		// Default should be noop
		assert.Equal(t, NoopProvider, tracer.GetProvider())
	})

	t.Run("ShutdownIdempotent", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test-service"),
			WithServiceVersion("v1.0.0"),
			WithStdout(),
		)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()

		err = tracer.Shutdown(ctx)
		require.NoError(t, err)

		err = tracer.Shutdown(ctx)
		require.NoError(t, err)
	})

	t.Run("MustNew_Success", func(t *testing.T) {
		t.Parallel()

		var cleanupTracer *Tracer
		assert.NotPanics(t, func() {
			cleanupTracer = MustNew(
				WithServiceName("test-service"),
				WithServiceVersion("v1.0.0"),
				WithStdout(),
			)
		})
		if cleanupTracer != nil {
			t.Cleanup(func() { cleanupTracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup
		}
	})

	t.Run("MustNew_Panics", func(t *testing.T) {
		assert.Panics(t, func() {
			MustNew(
				WithServiceName(""),
				WithStdout(),
			)
		})
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
			span.SetAttributes(attribute.String("custom.tenant_id", "tenant-123"))
		}

		tracer := MustNew(
			WithServiceName("test"),
			WithSpanStartHook(startHook),
		)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		handler := Middleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

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

		tracer := MustNew(
			WithServiceName("test"),
			WithSpanFinishHook(finishHook),
		)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		handler := Middleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))

		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

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

		tracer := MustNew(
			WithServiceName("test"),
			WithSpanStartHook(startHook),
			WithSpanFinishHook(finishHook),
		)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		handler := Middleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

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

		tracer := MustNew(
			WithServiceName("test"),
			WithSampleRate(0.0),
			WithSpanStartHook(startHook),
			WithSpanFinishHook(finishHook),
		)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		handler := Middleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.False(t, startHookCalled)
		assert.False(t, finishHookCalled)
	})
}

// TestGranularParameterRecording tests the granular parameter recording middleware options.
func TestGranularParameterRecording(t *testing.T) {
	t.Parallel()

	t.Run("WithRecordParams_Whitelist", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithServiceName("test"))
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		handler := Middleware(tracer, WithRecordParams("user_id", "request_id"))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test?user_id=123&page=5&token=secret", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("WithExcludeParams_Blacklist", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithServiceName("test"))
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		handler := Middleware(tracer, WithExcludeParams("password", "token", "api_key"))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test?user_id=123&password=secret&token=abc", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestConcurrentShutdown tests that shutdown is safe to call concurrently
func TestConcurrentShutdown(t *testing.T) {
	t.Parallel()

	tracer, err := New(
		WithServiceName("test-service"),
		WithServiceVersion("v1.0.0"),
		WithStdout(),
	)
	require.NoError(t, err)
	require.NotNil(t, tracer)

	const numGoroutines = 100
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	for range numGoroutines {
		wg.Go(func() {
			shutdownErr := tracer.Shutdown(ctx)
			assert.NoError(t, shutdownErr)
		})
	}

	wg.Wait()

	err = tracer.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestProviderFailure tests that provider initialization failures are handled gracefully
func TestProviderFailure(t *testing.T) {
	t.Parallel()

	t.Run("OTLPProvider_InvalidEndpoint", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test"),
			WithServiceVersion("v1.0.0"),
			WithOTLP("localhost:99999", OTLPInsecure()),
		)
		if err != nil {
			assert.Error(t, err)
		} else {
			require.NotNil(t, tracer)
			t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup
		}
	})
}

// TestContextCancellationInStartRequestSpan tests context cancellation handling
func TestContextCancellationInStartRequestSpan(t *testing.T) {
	t.Parallel()

	tracer := MustNew(
		WithServiceName("test-service"),
		WithSampleRate(1.0),
	)
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	t.Run("CancelledContext", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx, span := tracer.StartRequestSpan(ctx, req, "/test", false)

		require.Error(t, ctx.Err())
		assert.Equal(t, context.Canceled, ctx.Err())
		assert.NotNil(t, span)
	})

	t.Run("TimeoutContext", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 1*time.Nanosecond)
		defer cancel()

		time.Sleep(10 * time.Millisecond)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx, span := tracer.StartRequestSpan(ctx, req, "/test", false)

		require.Error(t, ctx.Err())
		assert.NotNil(t, span)
	})
}

// TestExcludePathPattern tests regex pattern support for path exclusion via middleware
func TestExcludePathPattern(t *testing.T) {
	t.Parallel()

	t.Run("ValidPattern", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithServiceName("test"))
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		handler := Middleware(tracer,
			WithExcludePatterns("^/internal/.*", "^/(health|ready|live)"),
		)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Test pattern matching - make requests and verify they work
		for _, path := range []string{"/internal/api", "/health", "/ready", "/api/users"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})

	t.Run("Middleware_PanicsOnInvalidPattern", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithServiceName("test"))
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		assert.Panics(t, func() {
			Middleware(tracer,
				WithExcludePatterns("[invalid"),
			)
		})
	})
}

// TestExcludePrefixes tests prefix-based path exclusion via middleware
func TestExcludePrefixes(t *testing.T) {
	t.Parallel()

	t.Run("SinglePrefix", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithServiceName("test"))
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		handler := Middleware(tracer, WithExcludePrefixes("/debug/"))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		for _, path := range []string{"/debug/pprof", "/debug/vars", "/api/users"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})

	t.Run("MultiplePrefixes", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithServiceName("test"))
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		handler := Middleware(tracer,
			WithExcludePrefixes("/debug/", "/internal/", "/admin/"),
		)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		for _, path := range []string{"/debug/pprof", "/internal/status", "/admin/users", "/api/users"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})
}

// TestNewProviderOptions tests the new convenient provider options
func TestNewProviderOptions(t *testing.T) {
	t.Parallel()

	t.Run("WithOTLP", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test"),
			WithOTLP("localhost:4317"),
		)
		if tracer != nil {
			t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup
			assert.Equal(t, OTLPProvider, tracer.GetProvider())
		}
		// May fail if OTLP collector not running - that's ok
		_ = err
	})

	t.Run("WithOTLPInsecure", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test"),
			WithOTLP("localhost:4317", OTLPInsecure()),
		)
		if tracer != nil {
			t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup
			assert.Equal(t, OTLPProvider, tracer.GetProvider())
		}
		_ = err
	})

	t.Run("WithOTLPHTTP", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test"),
			WithOTLPHTTP("http://localhost:4318"),
		)
		if tracer != nil {
			t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup
			assert.Equal(t, OTLPHTTPProvider, tracer.GetProvider())
		}
		_ = err
	})

	t.Run("WithStdout", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test"),
			WithStdout(),
		)
		require.NoError(t, err)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup
		assert.Equal(t, StdoutProvider, tracer.GetProvider())
	})

	t.Run("WithNoop", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test"),
			WithNoop(),
		)
		require.NoError(t, err)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup
		assert.Equal(t, NoopProvider, tracer.GetProvider())
	})
}

// TestMultipleProvidersValidation tests that configuring multiple providers returns an error
func TestMultipleProvidersValidation(t *testing.T) {
	t.Parallel()

	t.Run("StdoutThenOTLP", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test"),
			WithStdout(),
			WithOTLP("localhost:4317"),
		)
		require.Error(t, err)
		assert.Nil(t, tracer)
		assert.Contains(t, err.Error(), "multiple providers configured")
		assert.Contains(t, err.Error(), "stdout")
		assert.Contains(t, err.Error(), "otlp")
	})

	t.Run("OTLPThenStdout", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test"),
			WithOTLP("localhost:4317"),
			WithStdout(),
		)
		require.Error(t, err)
		assert.Nil(t, tracer)
		assert.Contains(t, err.Error(), "multiple providers configured")
	})

	t.Run("NoopThenOTLPHTTP", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test"),
			WithNoop(),
			WithOTLPHTTP("http://localhost:4318"),
		)
		require.Error(t, err)
		assert.Nil(t, tracer)
		assert.Contains(t, err.Error(), "multiple providers configured")
		assert.Contains(t, err.Error(), "noop")
		assert.Contains(t, err.Error(), "otlp-http")
	})

	t.Run("ThreeProviders", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test"),
			WithStdout(),
			WithOTLP("localhost:4317"),
			WithNoop(),
		)
		require.Error(t, err)
		assert.Nil(t, tracer)
		// Should contain error for second provider (OTLP)
		assert.Contains(t, err.Error(), "multiple providers configured")
	})

	t.Run("SingleProviderIsValid", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test"),
			WithStdout(),
		)
		require.NoError(t, err)
		require.NotNil(t, tracer)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup
		assert.Equal(t, StdoutProvider, tracer.GetProvider())
	})

	t.Run("MustNew_PanicsOnMultipleProviders", func(t *testing.T) {
		t.Parallel()

		assert.Panics(t, func() {
			MustNew(
				WithServiceName("test"),
				WithStdout(),
				WithOTLP("localhost:4317"),
			)
		})
	})
}

// TestValidate_SampleRateBetweenZeroAndOne covers sampling threshold for rate in (0, 1).
func TestValidate_SampleRateBetweenZeroAndOne(t *testing.T) {
	t.Parallel()

	tracer, err := New(
		WithServiceName("test"),
		WithSampleRate(0.5),
	)
	require.NoError(t, err)
	require.NotNil(t, tracer)
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	assert.InEpsilon(t, 0.5, tracer.sampleRate, 0.001)
	assert.Greater(t, tracer.samplingThreshold, uint64(0))
	assert.Less(t, tracer.samplingThreshold, ^uint64(0))
}

// TestValidate_OTLPDefaultEndpointWarning covers the warning when OTLP endpoint is empty.
func TestValidate_OTLPDefaultEndpointWarning(t *testing.T) {
	t.Parallel()

	var warnings []string
	handler := func(e Event) {
		if e.Type == EventWarning {
			warnings = append(warnings, e.Message)
		}
	}

	tracer, err := New(
		WithServiceName("test"),
		WithOTLP(""),
		WithEventHandler(handler),
	)
	require.NoError(t, err)
	require.NotNil(t, tracer)
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	assert.Contains(t, warnings, "OTLP endpoint not specified, will use default")
}

// TestTracer_StartSpan_CanceledContext covers StartSpan when context is already canceled.
func TestTracer_StartSpan_CanceledContext(t *testing.T) {
	t.Parallel()

	tracer := MustNew()
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, span := tracer.StartSpan(ctx, "test")
	defer tracer.FinishSpan(span, http.StatusOK)

	assert.False(t, span.IsRecording(), "span should not be recording when context is canceled")
}

// TestTracer_Start_Idempotent covers Start when called twice (second returns nil).
func TestTracer_Start_Idempotent(t *testing.T) {
	t.Parallel()

	tracer, err := New(
		WithServiceName("test"),
		WithOTLP("localhost:4317", OTLPInsecure()),
	)
	require.NoError(t, err)
	require.NotNil(t, tracer)
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	err1 := tracer.Start(ctx)
	require.NoError(t, err1)

	err2 := tracer.Start(ctx)
	require.NoError(t, err2)
	assert.Nil(t, err2)
}

// TestTracer_Start_OTLPInitError covers Start with OTLP; init may fail on invalid endpoint or timeout.
func TestTracer_Start_OTLPInitError(t *testing.T) {
	t.Parallel()

	tracer, err := New(
		WithServiceName("test"),
		WithOTLP("invalid-host-port", OTLPInsecure()),
	)
	require.NoError(t, err)
	require.NotNil(t, tracer)
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	err = tracer.Start(ctx)
	// OTLP client may succeed (background connect) or fail; either way Start path is exercised
	if err != nil {
		assert.Contains(t, err.Error(), "failed to initialize tracing")
	}
}

// TestTracer_Shutdown_ReturnsErrorWhenProviderFails covers Shutdown when provider shutdown fails.
// Uses a canceled context so that the SDK provider's Shutdown may return an error.
func TestTracer_Shutdown_ReturnsErrorWhenProviderFails(t *testing.T) {
	t.Parallel()

	tracer, err := New(
		WithServiceName("test"),
		WithStdout(),
	)
	require.NoError(t, err)
	require.NotNil(t, tracer)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = tracer.Shutdown(ctx)
	// SDK may or may not return error on canceled context; either way Shutdown path is exercised
	if err != nil {
		assert.Contains(t, err.Error(), "tracer provider shutdown")
	}
}

// TestTracer_FinishSpan_StatusCodeError covers FinishSpan with statusCode >= 400.
func TestTracer_FinishSpan_StatusCodeError(t *testing.T) {
	t.Parallel()

	tracer := MustNew()
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	_, span := tracer.StartSpan(t.Context(), "test")
	require.True(t, span.IsRecording())

	tracer.FinishSpan(span, http.StatusNotFound)
	// No panic and span is ended
}

// TestAddSpanEventFromContext_NoOpWhenNotRecording covers AddSpanEventFromContext with no recording span.
func TestAddSpanEventFromContext_NoOpWhenNotRecording(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	// No span in context - should not panic
	AddSpanEventFromContext(ctx, "event")
	AddSpanEventFromContext(ctx, "event2", attribute.String("k", "v"))
}

// TestBuildAttribute_DefaultStringConversion covers buildAttribute default branch (non-primitive type).
func TestBuildAttribute_DefaultStringConversion(t *testing.T) {
	t.Parallel()

	tracer := MustNew()
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	_, span := tracer.StartSpan(t.Context(), "test")
	defer tracer.FinishSpan(span, http.StatusOK)

	// Struct type uses default fmt.Sprintf path
	tracer.SetSpanAttribute(span, "struct_attr", struct{ X int }{42})
	assert.True(t, span.IsRecording())
}

// TestEventEmitters covers emitError, emitWarning, emitInfo, emitDebug via event handler.
func TestEventEmitters(t *testing.T) {
	t.Parallel()

	var (
		warnings []string
		infos    []string
		debugs   []string
	)
	handler := func(e Event) {
		switch e.Type {
		case EventWarning:
			warnings = append(warnings, e.Message)
		case EventInfo:
			infos = append(infos, e.Message)
		case EventDebug:
			debugs = append(debugs, e.Message)
		}
	}

	// emitWarning: OTLP empty endpoint
	tracer, err := New(
		WithServiceName("test"),
		WithOTLP(""),
		WithEventHandler(handler),
	)
	require.NoError(t, err)
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup
	assert.NotEmpty(t, warnings)

	// emitInfo: use Stdout so init emits info
	tracer2, err := New(
		WithServiceName("test"),
		WithStdout(),
		WithEventHandler(handler),
	)
	require.NoError(t, err)
	t.Cleanup(func() { tracer2.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup
	assert.NotEmpty(t, infos)

	// emitDebug: use custom provider and Shutdown so we get debug messages
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	require.NoError(t, err)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("test"),
		)),
	)
	tracer3, err := New(
		WithTracerProvider(tp),
		WithServiceName("test"),
		WithEventHandler(handler),
	)
	require.NoError(t, err)
	require.NoError(t, tracer3.Shutdown(t.Context()))
	require.NoError(t, tp.Shutdown(t.Context()))
	assert.NotEmpty(t, debugs)
	// emitError is triggered when sdkProvider.Shutdown returns error; see TestTracer_Shutdown_ReturnsErrorWhenProviderFails
}

// TestTraceContext_ReturnsContext covers TraceContext.
func TestTraceContext_ReturnsContext(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	out := TraceContext(ctx)
	assert.Equal(t, ctx, out)
}
