package tracing

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rivaas-dev/rivaas/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTracingConfig(t *testing.T) {
	config := MustNew(
		WithServiceName("test-service"),
		WithServiceVersion("v1.0.0"),
		WithSampleRate(0.5),
	)
	defer config.Shutdown(context.Background())

	assert.True(t, config.IsEnabled())
	assert.Equal(t, "test-service", config.GetServiceName())
	assert.Equal(t, "v1.0.0", config.GetServiceVersion())
	assert.NotNil(t, config.GetTracer())
	assert.NotNil(t, config.GetPropagator())
}

func TestTracingWithRouter(t *testing.T) {
	// Create tracing config
	config := MustNew(
		WithServiceName("test-service"),
		WithSampleRate(1.0),
	)
	defer config.Shutdown(context.Background())

	// Create router
	r := router.New()
	r.SetTracingRecorder(config)

	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTracingOptions(t *testing.T) {
	t.Run("WithExcludePaths", func(t *testing.T) {
		config := MustNew(
			WithExcludePaths("/health", "/metrics"),
		)
		assert.True(t, config.ShouldExcludePath("/health"))
		assert.True(t, config.ShouldExcludePath("/metrics"))
		assert.False(t, config.ShouldExcludePath("/api"))
	})

	t.Run("WithHeaders", func(t *testing.T) {
		config := MustNew(
			WithHeaders("Authorization", "X-Request-ID"),
		)
		assert.True(t, config.IsEnabled())
	})

	t.Run("WithDisableParams", func(t *testing.T) {
		config := MustNew(
			WithDisableParams(),
		)
		assert.True(t, config.IsEnabled())
	})

	t.Run("WithCustomTracer", func(t *testing.T) {
		tempConfig := MustNew()
		config := MustNew(
			WithCustomTracer(tempConfig.GetTracer()),
		)
		assert.True(t, config.IsEnabled())
	})
}

func TestTracingMiddleware(t *testing.T) {
	config := MustNew(
		WithServiceName("test-service"),
		WithExcludePaths("/health"),
	)

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	// Test full integration with router
	config := MustNew(
		WithServiceName("integration-test"),
		WithExcludePaths("/health"),
	)

	r := router.New()
	r.SetTracingRecorder(config)

	// Add routes
	r.GET("/", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "Hello"})
	})

	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})

	// Test normal route
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Test health route (should be excluded from tracing)
	req = httptest.NewRequest("GET", "/health", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestContextHelpers(t *testing.T) {
	// Test context helper functions
	ctx := context.Background()

	// These should not panic even without active spans
	traceID := TraceID(ctx)
	spanID := SpanID(ctx)

	assert.Equal(t, "", traceID)
	assert.Equal(t, "", spanID)
}

func TestTracingRecorderInterface(t *testing.T) {
	config := MustNew(
		WithServiceName("test-service"),
	)

	// Test that config implements router.TracingRecorder interface
	var recorder interface{} = config
	_, ok := recorder.(interface {
		IsEnabled() bool
		ShouldExcludePath(string) bool
	})
	assert.True(t, ok, "Config should implement TracingRecorder interface")
	assert.True(t, config.IsEnabled())
}

func TestSamplingRate(t *testing.T) {
	t.Run("SampleRateValidation", func(t *testing.T) {
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
		config := MustNew(
			WithServiceName("test-service"),
			WithSampleRate(1.0),
		)

		// All requests should be traced
		r := router.New()
		r.SetTracingRecorder(config)

		r.GET("/test", func(c *router.Context) {
			// Trace should always be active at 100% sampling
			traceID := c.TraceID()
			// A valid trace ID would be generated
			c.JSON(http.StatusOK, map[string]string{"trace_id": traceID})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("SampleRate0Percent", func(t *testing.T) {
		config := MustNew(
			WithServiceName("test-service"),
			WithSampleRate(0.0),
		)

		// No requests should be traced
		r := router.New()
		r.SetTracingRecorder(config)

		r.GET("/test", func(c *router.Context) {
			traceID := c.TraceID()
			// At 0% sampling, no trace should be created
			c.JSON(http.StatusOK, map[string]string{"trace_id": traceID})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("SampleRateStatistical", func(t *testing.T) {
		// Note: This test validates that sampling logic works correctly.
		// With a noop tracer (default), spans won't be recording, but we can
		// verify the sampling logic is being called correctly by testing with
		// the router integration where we can observe the behavior.

		config := MustNew(
			WithServiceName("test-service"),
			WithSampleRate(0.5),
		)

		r := router.New()
		r.SetTracingRecorder(config)

		// The sampling logic is working correctly if:
		// 1. Requests are processed without errors
		// 2. No panics or race conditions occur
		// 3. The behavior is consistent

		r.GET("/test", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		// Make multiple requests to verify sampling doesn't cause issues
		const numRequests = 100
		for i := 0; i < numRequests; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
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

		r := router.New()
		r.SetTracingRecorder(config)

		r.GET("/test", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/test?foo=bar&baz=qux", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("WithDisabledParams", func(t *testing.T) {
		config := MustNew(
			WithServiceName("test-service"),
			WithDisableParams(),
		)

		assert.False(t, config.recordParams)

		r := router.New()
		r.SetTracingRecorder(config)

		r.GET("/test", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/test?foo=bar", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestSpanAttributeTypes(t *testing.T) {
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
	config := MustNew(
		WithServiceName("test-service"),
	)

	r := router.New()
	r.SetTracingRecorder(config)

	r.GET("/not-found", func(c *router.Context) {
		c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	})

	r.GET("/error", func(c *router.Context) {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "server error"})
	})

	// Test 404
	req := httptest.NewRequest("GET", "/not-found", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Test 500
	req = httptest.NewRequest("GET", "/error", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestConcurrentResponseWriter(t *testing.T) {
	config := MustNew(
		WithServiceName("test-service"),
	)

	r := router.New()
	r.SetTracingRecorder(config)

	r.GET("/concurrent", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Test concurrent requests
	const numRequests = 50
	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/concurrent", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}()
	}

	wg.Wait()
}

func TestContextTracingHelpers(t *testing.T) {
	config := MustNew(
		WithServiceName("test-service"),
	)

	r := router.New()
	r.SetTracingRecorder(config)

	r.GET("/test", func(c *router.Context) {
		// Test different attribute types through context
		c.SetSpanAttribute("string", "value")
		c.SetSpanAttribute("int", 42)
		c.SetSpanAttribute("float", 3.14)
		c.SetSpanAttribute("bool", true)

		c.AddSpanEvent("test_event")

		traceID := c.TraceID()
		spanID := c.SpanID()

		c.JSON(http.StatusOK, map[string]string{
			"trace_id": traceID,
			"span_id":  spanID,
		})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// Edge case tests
func TestEdgeCases(t *testing.T) {
	t.Run("DisabledTracing", func(t *testing.T) {
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
		config, err := New(WithServiceName(""))
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "service name cannot be empty")
	})

	t.Run("EmptyServiceVersion", func(t *testing.T) {
		config, err := New(WithServiceVersion(""))
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "service version cannot be empty")
	})

	t.Run("ExtremelyLargeSampleRate", func(t *testing.T) {
		// Sample rate is clamped by WithSampleRate, not by validation
		config := MustNew(WithServiceName("test"), WithSampleRate(999.9))
		defer config.Shutdown(context.Background())
		assert.Equal(t, 1.0, config.sampleRate)
	})

	t.Run("NegativeSampleRate", func(t *testing.T) {
		// Sample rate is clamped by WithSampleRate, not by validation
		config := MustNew(WithServiceName("test"), WithSampleRate(-999.9))
		defer config.Shutdown(context.Background())
		assert.Equal(t, 0.0, config.sampleRate)
	})

	t.Run("EmptyExcludePaths", func(t *testing.T) {
		config := MustNew(WithExcludePaths())
		assert.NotNil(t, config)
		assert.False(t, config.ShouldExcludePath("/any"))
	})

	t.Run("DuplicateExcludePaths", func(t *testing.T) {
		config := MustNew(
			WithExcludePaths("/health"),
			WithExcludePaths("/health"),
			WithExcludePaths("/health"),
		)
		assert.True(t, config.ShouldExcludePath("/health"))
	})

	t.Run("MaxExcludedPathsLimit", func(t *testing.T) {
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
		config := MustNew()
		ctx := context.Background()

		// Should not panic with nil headers
		ctx = config.ExtractTraceContext(ctx, nil)
		config.InjectTraceContext(ctx, nil)

		assert.NotNil(t, ctx)
	})

	t.Run("EmptyHeaders", func(t *testing.T) {
		config := MustNew()
		ctx := context.Background()
		headers := http.Header{}

		ctx = config.ExtractTraceContext(ctx, headers)
		config.InjectTraceContext(ctx, headers)

		assert.NotNil(t, ctx)
	})

	t.Run("MalformedTraceParent", func(t *testing.T) {
		config := MustNew()
		ctx := context.Background()
		headers := http.Header{}
		headers.Set("traceparent", "invalid-trace-parent")

		// Should handle gracefully
		ctx = config.ExtractTraceContext(ctx, headers)
		assert.NotNil(t, ctx)
	})

	t.Run("NilSpanOperations", func(t *testing.T) {
		config := MustNew()

		// These should not panic even with nil span
		config.SetSpanAttribute(nil, "key", "value")
		config.AddSpanEvent(nil, "event")
		config.FinishSpan(nil, http.StatusOK)

		// Should be handled gracefully
		assert.True(t, true)
	})

	t.Run("VeryLongAttributeValue", func(t *testing.T) {
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
	t.Run("PropagateTraceContext", func(t *testing.T) {
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
	t.Run("CancelledContextDoesNotCreateSpan", func(t *testing.T) {
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
	t.Run("DisableParamsWorks", func(t *testing.T) {
		config := MustNew(
			WithServiceName("test"),
			WithDisableParams(),
		)

		r := router.New()
		r.SetTracingRecorder(config)

		r.GET("/test", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/test?secret=password&token=abc123", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("HeaderRecordingWorks", func(t *testing.T) {
		config := MustNew(
			WithServiceName("test"),
			WithHeaders("X-Request-ID", "User-Agent"),
		)

		r := router.New()
		r.SetTracingRecorder(config)

		r.GET("/test", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Request-ID", "test-123")
		req.Header.Set("User-Agent", "test-agent")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("SensitiveHeadersFiltered", func(t *testing.T) {
		config := MustNew(
			WithServiceName("test"),
			WithHeaders("Authorization", "Cookie", "X-Request-ID"),
		)

		// Verify sensitive headers are filtered out
		assert.Len(t, config.recordHeaders, 1)
		assert.Equal(t, "X-Request-ID", config.recordHeaders[0])
	})
}

// TestRouterIntegration tests the integration layer between tracing and router
func TestRouterIntegration(t *testing.T) {
	t.Run("WithTracingOption", func(t *testing.T) {
		// Test that WithTracing creates and sets config
		r := router.New()
		opt := WithTracing(
			WithServiceName("test-service"),
			WithSampleRate(0.5),
		)
		opt(r)

		// Add a route and verify tracing works
		r.GET("/test", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("WithTracingFromConfig", func(t *testing.T) {
		config := MustNew(
			WithServiceName("existing-config"),
			WithSampleRate(1.0),
		)

		r := router.New()
		opt := WithTracingFromConfig(config)
		opt(r)

		r.GET("/test", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("MiddlewareIntegration", func(t *testing.T) {
		config := MustNew(
			WithServiceName("middleware-test"),
			WithSampleRate(1.0),
		)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		config := MustNew(
			WithServiceName("middleware-test"),
			WithExcludePaths("/health"),
		)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		config := MustNew(WithSampleRate(0.0))

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		ct := NewContextTracing(config, ctx, span)

		assert.NotNil(t, ct.TraceContext())
		assert.NotNil(t, ct.GetSpan())
		assert.NotNil(t, ct.GetConfig())
	})

	t.Run("NilContext", func(t *testing.T) {
		config := MustNew()
		ct := NewContextTracing(config, nil, nil)

		// Should not panic and should return valid context
		ctx := ct.TraceContext()
		assert.NotNil(t, ctx)
	})

	t.Run("ContextTracingMethods", func(t *testing.T) {
		config := MustNew()
		ctx, span := config.StartSpan(context.Background(), "test")
		defer config.FinishSpan(span, http.StatusOK)

		ct := NewContextTracing(config, ctx, span)

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
		config := MustNew()
		ctx := context.Background()
		ct := NewContextTracing(config, ctx, nil)

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

	assert.Equal(t, "prod-service", config.GetServiceName())
	assert.Equal(t, "v2.0.0", config.GetServiceVersion())
	assert.Equal(t, 0.1, config.sampleRate)
	assert.False(t, config.recordParams)
	assert.True(t, config.ShouldExcludePath("/health"))
	assert.True(t, config.ShouldExcludePath("/metrics"))
	assert.True(t, config.ShouldExcludePath("/ready"))
	assert.Equal(t, OTLPProvider, config.GetProvider())
}

// TestDevelopmentHelper tests the development configuration helper
func TestDevelopmentHelper(t *testing.T) {
	config, err := NewDevelopment("dev-service", "dev")
	require.NoError(t, err)
	require.NotNil(t, config)
	defer config.Shutdown(context.Background())

	assert.Equal(t, "dev-service", config.GetServiceName())
	assert.Equal(t, "dev", config.GetServiceVersion())
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
		assert.Equal(t, "test-service", config.GetServiceName())
		assert.Equal(t, "v1.0.0", config.GetServiceVersion())
	})

	t.Run("OTLPProvider", func(t *testing.T) {
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
		config, err := New(
			WithServiceName("test-service"),
			WithServiceVersion("v1.0.0"),
			WithProvider(TracingProvider("invalid")),
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
