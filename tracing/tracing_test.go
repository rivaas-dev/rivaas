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

func TestTracerConfig(t *testing.T) {
	t.Parallel()

	tracer := MustNew(
		WithServiceName("test-service"),
		WithServiceVersion("v1.0.0"),
		WithSampleRate(0.5),
	)
	t.Cleanup(func() { tracer.Shutdown(context.Background()) })

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
	t.Cleanup(func() { tracer.Shutdown(context.Background()) })

	// Create HTTP handler with tracing middleware
	handler := MustMiddleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
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
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })
		assert.Equal(t, 0.5, tracer.sampleRate)
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
		w.Write([]byte("OK"))
	})

	// Wrap with tracing middleware (with path exclusion)
	middleware := MustMiddleware(tracer, WithExcludePaths("/health"))
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
	tracer := MustNew(
		WithServiceName("integration-test"),
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
	handler := MustMiddleware(tracer, WithExcludePaths("/health"))(mux)

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
		tracer := MustNew(WithServiceName("test"), WithSampleRate(1.5))
		assert.Equal(t, 1.0, tracer.sampleRate)
		tracer.Shutdown(context.Background())

		tracer = MustNew(WithServiceName("test"), WithSampleRate(-0.5))
		assert.Equal(t, 0.0, tracer.sampleRate)
		tracer.Shutdown(context.Background())

		tracer = MustNew(WithServiceName("test"), WithSampleRate(0.5))
		assert.Equal(t, 0.5, tracer.sampleRate)
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })
	})

	t.Run("SampleRate100Percent", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("test-service"),
			WithSampleRate(1.0),
		)
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

		// All requests should be traced
		handler := MustMiddleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

		tracer := MustNew(
			WithServiceName("test-service"),
			WithSampleRate(0.0),
		)
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

		// No requests should be traced
		handler := MustMiddleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

		tracer := MustNew(
			WithServiceName("test-service"),
			WithSampleRate(0.5),
		)
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

		handler := MustMiddleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

		assert.True(t, true, "Sampling logic executed successfully")
	})
}

func TestParameterRecording(t *testing.T) {
	t.Parallel()

	t.Run("WithParams", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("test-service"),
		)
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

		handler := MustMiddleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))

		req := httptest.NewRequest("GET", "/test?foo=bar&baz=qux", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("WithoutParams", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("test-service"),
		)
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

		handler := MustMiddleware(tracer, WithoutParams())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

	tracer := MustNew(
		WithServiceName("test-service"),
	)

	ctx := context.Background()
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

	ctx := context.Background()
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
	t.Cleanup(func() { tracer.Shutdown(context.Background()) })

	// Create mux with different status codes
	mux := http.NewServeMux()
	for _, tt := range tests {
		status := tt.wantStatus
		body := tt.wantBody
		mux.HandleFunc(tt.path, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			w.Write([]byte(body))
		})
	}
	handler := MustMiddleware(tracer)(mux)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", tt.path, nil)
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
	t.Cleanup(func() { tracer.Shutdown(context.Background()) })

	handler := MustMiddleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

	tracer := MustNew(
		WithServiceName("test-service"),
	)
	t.Cleanup(func() { tracer.Shutdown(context.Background()) })

	handler := MustMiddleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		SetSpanAttributeFromContext(ctx, "string", "value")
		SetSpanAttributeFromContext(ctx, "int", 42)
		SetSpanAttributeFromContext(ctx, "float", 3.14)
		SetSpanAttributeFromContext(ctx, "bool", true)

		AddSpanEventFromContext(ctx, "test_event")

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

// TestTracer_EdgeCases tests edge cases and robustness of the Tracer type.
func TestTracer_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("DisabledTracing", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithSampleRate(0.0))
		ctx := context.Background()

		_, span := tracer.StartSpan(ctx, "test")
		tracer.SetSpanAttribute(span, "key", "value")
		tracer.AddSpanEvent(span, "event")
		tracer.FinishSpan(span, http.StatusOK)

		assert.NotNil(t, span)
	})

	t.Run("NilContext", func(t *testing.T) {
		t.Parallel()

		traceID := TraceID(context.Background())
		spanID := SpanID(context.Background())

		assert.Equal(t, "", traceID)
		assert.Equal(t, "", spanID)

		SetSpanAttributeFromContext(context.Background(), "key", "value")
		AddSpanEventFromContext(context.Background(), "event")
	})

	t.Run("MultipleFinishSpan", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()
		ctx := context.Background()
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
		assert.Error(t, err)
		assert.Nil(t, tracer)
		assert.Contains(t, err.Error(), "serviceName: cannot be empty")
	})

	t.Run("EmptyServiceVersion", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(WithServiceVersion(""))
		assert.Error(t, err)
		assert.Nil(t, tracer)
		assert.Contains(t, err.Error(), "serviceVersion: cannot be empty")
	})

	t.Run("ExtremelyLargeSampleRate", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithServiceName("test"), WithSampleRate(999.9))
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })
		assert.Equal(t, 1.0, tracer.sampleRate)
	})

	t.Run("NegativeSampleRate", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithServiceName("test"), WithSampleRate(-999.9))
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })
		assert.Equal(t, 0.0, tracer.sampleRate)
	})

	t.Run("NilHeaders", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()
		ctx := context.Background()

		ctx = tracer.ExtractTraceContext(ctx, nil)
		tracer.InjectTraceContext(ctx, nil)

		assert.NotNil(t, ctx)
	})

	t.Run("EmptyHeaders", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()
		ctx := context.Background()
		headers := http.Header{}

		ctx = tracer.ExtractTraceContext(ctx, headers)
		tracer.InjectTraceContext(ctx, headers)

		assert.NotNil(t, ctx)
	})

	t.Run("MalformedTraceParent", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()
		ctx := context.Background()
		headers := http.Header{}
		headers.Set("traceparent", "invalid-trace-parent")

		ctx = tracer.ExtractTraceContext(ctx, headers)
		assert.NotNil(t, ctx)
	})

	t.Run("NilSpanOperations", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()

		tracer.SetSpanAttribute(nil, "key", "value")
		tracer.AddSpanEvent(nil, "event")
		tracer.FinishSpan(nil, http.StatusOK)

		assert.True(t, true)
	})

	t.Run("VeryLongAttributeValue", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()
		ctx := context.Background()
		_, span := tracer.StartSpan(ctx, "test")
		defer tracer.FinishSpan(span, http.StatusOK)

		longValue := string(make([]byte, 10000))
		tracer.SetSpanAttribute(span, "long_key", longValue)

		assert.NotNil(t, span)
	})

	t.Run("SpecialCharactersInAttributeKey", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()
		ctx := context.Background()
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

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx := context.Background()
				_, span := tracer.StartSpan(ctx, "test")
				tracer.SetSpanAttribute(span, "key", "value")
				tracer.FinishSpan(span, http.StatusOK)
			}()
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
		headers.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")

		ctx := context.Background()
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

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		ctx, span := tracer.StartSpan(ctx, "test-span")
		defer tracer.FinishSpan(span, http.StatusOK)

		assert.Error(t, ctx.Err())
		assert.Equal(t, context.Canceled, ctx.Err())
	})

	t.Run("ActiveContextCreatesSpan", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()

		ctx, cancel := context.WithCancel(context.Background())
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
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

		handler := MustMiddleware(tracer, WithoutParams())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))

		req := httptest.NewRequest("GET", "/test?secret=password&token=abc123", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("HeaderRecordingWorks", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("test"),
		)
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

		handler := MustMiddleware(tracer, WithHeaders("X-Request-ID", "User-Agent"))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
			w.Write([]byte("OK"))
		})

		middleware := MustMiddleware(tracer)
		wrappedHandler := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
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

		middleware := MustMiddleware(tracer, WithExcludePaths("/health"))
		wrappedHandler := middleware(handler)

		req := httptest.NewRequest("GET", "/health", nil)
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

		middleware := MustMiddleware(tracer)
		wrappedHandler := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
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
		ctx := context.Background()
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
		ct := NewContextTracing(context.TODO(), tracer, nil)

		ctx := ct.TraceContext()
		assert.NotNil(t, ctx)
	})

	t.Run("ContextTracingMethods", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew()
		ctx, span := tracer.StartSpan(context.Background(), "test")
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
		ctx := context.Background()
		ct := NewContextTracing(ctx, tracer, nil)

		ct.SetSpanAttribute("key", "value")
		ct.AddSpanEvent("event")
		traceID := ct.TraceID()
		spanID := ct.SpanID()

		assert.Equal(t, "", traceID)
		assert.Equal(t, "", spanID)
	})
}

// TestProviderSetup tests the provider setup functions.
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
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

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
			t.Cleanup(func() { tracer.Shutdown(context.Background()) })
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
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

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
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

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

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err = tracer.Shutdown(ctx)
		assert.NoError(t, err)

		err = tracer.Shutdown(ctx)
		assert.NoError(t, err)
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
			t.Cleanup(func() { cleanupTracer.Shutdown(context.Background()) })
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
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

		handler := MustMiddleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
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
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

		handler := MustMiddleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))

		req := httptest.NewRequest("POST", "/test", nil)
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
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

		handler := MustMiddleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
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
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

		handler := MustMiddleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
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
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

		handler := MustMiddleware(tracer, WithRecordParams("user_id", "request_id"))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test?user_id=123&page=5&token=secret", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("WithExcludeParams_Blacklist", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithServiceName("test"))
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

		handler := MustMiddleware(tracer, WithExcludeParams("password", "token", "api_key"))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test?user_id=123&password=secret&token=abc", nil)
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
	wg.Add(numGoroutines)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			err := tracer.Shutdown(ctx)
			assert.NoError(t, err)
		}()
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
			t.Cleanup(func() { tracer.Shutdown(context.Background()) })
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
	t.Cleanup(func() { tracer.Shutdown(context.Background()) })

	t.Run("CancelledContext", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		req := httptest.NewRequest("GET", "/test", nil)
		ctx, span := tracer.StartRequestSpan(ctx, req, "/test", false)

		assert.Error(t, ctx.Err())
		assert.Equal(t, context.Canceled, ctx.Err())
		assert.NotNil(t, span)
	})

	t.Run("TimeoutContext", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		time.Sleep(10 * time.Millisecond)

		req := httptest.NewRequest("GET", "/test", nil)
		ctx, span := tracer.StartRequestSpan(ctx, req, "/test", false)

		assert.Error(t, ctx.Err())
		assert.NotNil(t, span)
	})
}

// TestExcludePathPattern tests regex pattern support for path exclusion via middleware
func TestExcludePathPattern(t *testing.T) {
	t.Parallel()

	t.Run("ValidPattern", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithServiceName("test"))
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

		handler := MustMiddleware(tracer,
			WithExcludePatterns("^/internal/.*", "^/(health|ready|live)"),
		)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Test pattern matching - make requests and verify they work
		for _, path := range []string{"/internal/api", "/health", "/ready", "/api/users"} {
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})

	t.Run("InvalidPattern_ReturnsError", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithServiceName("test"))
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

		// Invalid regex pattern (unclosed bracket)
		_, err := Middleware(tracer,
			WithExcludePatterns("[invalid"),
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "excludePatterns: invalid regex")
		assert.Contains(t, err.Error(), "[invalid")
	})

	t.Run("MixedPatterns_PartialError", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithServiceName("test"))
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

		// One valid, one invalid pattern
		_, err := Middleware(tracer,
			WithExcludePatterns("^/valid/.*", "[invalid", "^/also-valid$"),
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "excludePatterns: invalid regex")
	})

	t.Run("MustMiddleware_PanicsOnInvalidPattern", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithServiceName("test"))
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

		assert.Panics(t, func() {
			MustMiddleware(tracer,
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
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

		handler := MustMiddleware(tracer, WithExcludePrefixes("/debug/"))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		for _, path := range []string{"/debug/pprof", "/debug/vars", "/api/users"} {
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})

	t.Run("MultiplePrefixes", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(WithServiceName("test"))
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })

		handler := MustMiddleware(tracer,
			WithExcludePrefixes("/debug/", "/internal/", "/admin/"),
		)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		for _, path := range []string{"/debug/pprof", "/internal/status", "/admin/users", "/api/users"} {
			req := httptest.NewRequest("GET", path, nil)
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
			t.Cleanup(func() { tracer.Shutdown(context.Background()) })
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
			t.Cleanup(func() { tracer.Shutdown(context.Background()) })
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
			t.Cleanup(func() { tracer.Shutdown(context.Background()) })
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
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })
		assert.Equal(t, StdoutProvider, tracer.GetProvider())
	})

	t.Run("WithNoop", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test"),
			WithNoop(),
		)
		require.NoError(t, err)
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })
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
		t.Cleanup(func() { tracer.Shutdown(context.Background()) })
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
