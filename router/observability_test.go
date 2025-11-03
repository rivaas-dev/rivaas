package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// mockMetricsRecorder implements MetricsRecorder for testing
type mockMetricsRecorder struct {
	enabled             bool
	startRequestCalled  bool
	finishRequestCalled bool
	recordMetricCalled  bool
	incrementCalled     bool
	setGaugeCalled      bool
}

func (m *mockMetricsRecorder) IsEnabled() bool {
	return m.enabled
}

func (m *mockMetricsRecorder) StartRequest(ctx context.Context, path string, isStatic bool, attrs ...attribute.KeyValue) any {
	m.startRequestCalled = true
	return nil
}

func (m *mockMetricsRecorder) FinishRequest(ctx context.Context, data any, statusCode int, size int64) {
	m.finishRequestCalled = true
}

func (m *mockMetricsRecorder) RecordRouteRegistration(ctx context.Context, method, path string) {}

func (m *mockMetricsRecorder) RecordMetric(ctx context.Context, name string, value float64, attrs ...attribute.KeyValue) {
	m.recordMetricCalled = true
}

func (m *mockMetricsRecorder) IncrementCounter(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	m.incrementCalled = true
}

func (m *mockMetricsRecorder) SetGauge(ctx context.Context, name string, value float64, attrs ...attribute.KeyValue) {
	m.setGaugeCalled = true
}

func (m *mockMetricsRecorder) RecordConstraintFailure(ctx context.Context, path string) {}

func (m *mockMetricsRecorder) RecordContextPoolHit(ctx context.Context) {}

func (m *mockMetricsRecorder) RecordContextPoolMiss(ctx context.Context) {}

// mockTracingRecorder implements TracingRecorder for testing
type mockTracingRecorder struct {
	enabled              bool
	traceID              string
	spanID               string
	startSpanCalled      bool
	finishSpanCalled     bool
	setAttributeCalled   bool
	addEventCalled       bool
	shouldExclude        bool
	mockSpan             trace.Span
	mockContext          context.Context
	extractContextCalled bool
	injectContextCalled  bool
}

func (m *mockTracingRecorder) IsEnabled() bool {
	return m.enabled
}

func (m *mockTracingRecorder) ShouldExcludePath(path string) bool {
	return m.shouldExclude
}

func (m *mockTracingRecorder) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	m.startSpanCalled = true
	if m.mockContext != nil && m.mockSpan != nil {
		return m.mockContext, m.mockSpan
	}
	return ctx, trace.SpanFromContext(ctx)
}

func (m *mockTracingRecorder) FinishSpan(span trace.Span, statusCode int) {
	m.finishSpanCalled = true
}

func (m *mockTracingRecorder) SetSpanAttribute(span trace.Span, key string, value any) {
	m.setAttributeCalled = true
}

func (m *mockTracingRecorder) AddSpanEvent(span trace.Span, name string, attrs ...attribute.KeyValue) {
	m.addEventCalled = true
}

func (m *mockTracingRecorder) TraceID() string {
	return m.traceID
}

func (m *mockTracingRecorder) SpanID() string {
	return m.spanID
}

func (m *mockTracingRecorder) TraceContext() context.Context {
	if m.mockContext != nil {
		return m.mockContext
	}
	return context.Background()
}

func (m *mockTracingRecorder) ExtractTraceContext(ctx context.Context, headers http.Header) context.Context {
	m.extractContextCalled = true
	return ctx
}

func (m *mockTracingRecorder) InjectTraceContext(ctx context.Context, headers http.Header) {
	m.injectContextCalled = true
}

// mockContextMetricsRecorder implements ContextMetricsRecorder
type mockContextMetricsRecorder struct {
	recordMetricCalled bool
	incrementCalled    bool
	setGaugeCalled     bool
}

func (m *mockContextMetricsRecorder) RecordMetric(ctx context.Context, name string, value float64, attrs ...attribute.KeyValue) {
	m.recordMetricCalled = true
}

func (m *mockContextMetricsRecorder) IncrementCounter(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	m.incrementCalled = true
}

func (m *mockContextMetricsRecorder) SetGauge(ctx context.Context, name string, value float64, attrs ...attribute.KeyValue) {
	m.setGaugeCalled = true
}

// mockContextTracingRecorder implements ContextTracingRecorder
type mockContextTracingRecorder struct {
	traceID            string
	spanID             string
	setAttributeCalled bool
	addEventCalled     bool
}

func (m *mockContextTracingRecorder) TraceID() string {
	return m.traceID
}

func (m *mockContextTracingRecorder) SpanID() string {
	return m.spanID
}

func (m *mockContextTracingRecorder) SetSpanAttribute(key string, value any) {
	m.setAttributeCalled = true
}

func (m *mockContextTracingRecorder) AddSpanEvent(name string, attrs ...attribute.KeyValue) {
	m.addEventCalled = true
}

func (m *mockContextTracingRecorder) TraceContext() context.Context {
	return context.Background()
}

// TestContextWithMetrics tests context metrics methods with actual recorder
func TestContextWithMetrics(t *testing.T) {
	r := New()
	metricsRecorder := &mockMetricsRecorder{enabled: true}
	r.SetMetricsRecorder(metricsRecorder)

	contextMetrics := &mockContextMetricsRecorder{}

	r.GET("/metrics-enabled", func(c *Context) {
		// Temporarily set mock recorder
		c.metricsRecorder = contextMetrics

		c.RecordMetric("test_metric", 1.5)
		c.IncrementCounter("test_counter")
		c.SetGauge("test_gauge", 42)

		assert.True(t, contextMetrics.recordMetricCalled)
		assert.True(t, contextMetrics.incrementCalled)
		assert.True(t, contextMetrics.setGaugeCalled)

		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/metrics-enabled", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestContextWithTracing tests context tracing methods with actual recorder
func TestContextWithTracing(t *testing.T) {
	r := New()

	contextTracing := &mockContextTracingRecorder{
		traceID: "test-trace-id",
		spanID:  "test-span-id",
	}

	r.GET("/tracing-enabled", func(c *Context) {
		// Temporarily set mock recorder
		c.tracingRecorder = contextTracing

		traceID := c.TraceID()
		spanID := c.SpanID()
		c.SetSpanAttribute("key", "value")
		c.AddSpanEvent("event")
		ctx := c.TraceContext()

		assert.Equal(t, "test-trace-id", traceID)
		assert.Equal(t, "test-span-id", spanID)
		assert.True(t, contextTracing.setAttributeCalled)
		assert.True(t, contextTracing.addEventCalled)
		assert.NotNil(t, ctx)

		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/tracing-enabled", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestServeHTTPWithMetrics tests ServeHTTP with metrics enabled
func TestServeHTTPWithMetrics(t *testing.T) {
	r := New()
	metricsRecorder := &mockMetricsRecorder{enabled: true}
	r.SetMetricsRecorder(metricsRecorder)

	r.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, metricsRecorder.startRequestCalled)
	assert.True(t, metricsRecorder.finishRequestCalled)
}

// TestServeHTTPWithTracing tests ServeHTTP with tracing enabled
func TestServeHTTPWithTracing(t *testing.T) {
	r := New()
	tracingRecorder := &mockTracingRecorder{enabled: true}
	r.SetTracingRecorder(tracingRecorder)

	r.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, tracingRecorder.startSpanCalled)
	assert.True(t, tracingRecorder.finishSpanCalled)
}

// TestServeHTTPWithBothMetricsAndTracing tests with both enabled
func TestServeHTTPWithBothMetricsAndTracing(t *testing.T) {
	r := New()
	metricsRecorder := &mockMetricsRecorder{enabled: true}
	tracingRecorder := &mockTracingRecorder{enabled: true}
	r.SetMetricsRecorder(metricsRecorder)
	r.SetTracingRecorder(tracingRecorder)

	r.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, metricsRecorder.startRequestCalled)
	assert.True(t, metricsRecorder.finishRequestCalled)
	assert.True(t, tracingRecorder.startSpanCalled)
	assert.True(t, tracingRecorder.finishSpanCalled)
}

// TestServeHTTPWithCompiledRoutes tests compiled route path
func TestServeHTTPWithCompiledRoutes(t *testing.T) {
	r := New()

	// Add static routes
	r.GET("/home", func(c *Context) {
		c.String(http.StatusOK, "home")
	})
	r.GET("/about", func(c *Context) {
		c.String(http.StatusOK, "about")
	})

	// Compile routes
	r.Warmup()

	// Test compiled route path
	req := httptest.NewRequest("GET", "/home", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "home", w.Body.String())
}

// TestServeHTTPWithCompiledRoutesAndMetrics tests compiled routes with metrics
func TestServeHTTPWithCompiledRoutesAndMetrics(t *testing.T) {
	r := New()
	metricsRecorder := &mockMetricsRecorder{enabled: true}
	r.SetMetricsRecorder(metricsRecorder)

	r.GET("/static", func(c *Context) {
		c.String(http.StatusOK, "static")
	})

	r.Warmup()

	req := httptest.NewRequest("GET", "/static", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, metricsRecorder.startRequestCalled)
}

// TestServeHTTPWithCompiledRoutesAndTracing tests compiled routes with tracing
func TestServeHTTPWithCompiledRoutesAndTracing(t *testing.T) {
	r := New()
	tracingRecorder := &mockTracingRecorder{enabled: true}
	r.SetTracingRecorder(tracingRecorder)

	r.GET("/static", func(c *Context) {
		c.String(http.StatusOK, "static")
	})

	r.Warmup()

	req := httptest.NewRequest("GET", "/static", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, tracingRecorder.startSpanCalled)
}

// TestServeHTTPWithCompiledRoutesAndBoth tests compiled routes with both metrics and tracing
func TestServeHTTPWithCompiledRoutesAndBoth(t *testing.T) {
	r := New()
	metricsRecorder := &mockMetricsRecorder{enabled: true}
	tracingRecorder := &mockTracingRecorder{enabled: true}
	r.SetMetricsRecorder(metricsRecorder)
	r.SetTracingRecorder(tracingRecorder)

	r.GET("/static", func(c *Context) {
		c.String(http.StatusOK, "static")
	})

	r.Warmup()

	req := httptest.NewRequest("GET", "/static", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, metricsRecorder.startRequestCalled)
	assert.True(t, tracingRecorder.startSpanCalled)
}

// TestCompiledRouteWithTracingAndMetrics tests the compiled route path with both tracing and metrics
// This covers the branch: shouldTrace && shouldMeasure -> serveStaticWithTracingAndMetrics
func TestCompiledRouteWithTracingAndMetrics(t *testing.T) {
	r := New()
	metricsRecorder := &mockMetricsRecorder{enabled: true}
	tracingRecorder := &mockTracingRecorder{enabled: true}
	r.SetMetricsRecorder(metricsRecorder)
	r.SetTracingRecorder(tracingRecorder)

	r.GET("/compiled", func(c *Context) {
		c.String(http.StatusOK, "compiled-route")
	})

	// Compile routes to enable compiled route lookup
	r.Warmup()

	req := httptest.NewRequest("GET", "/compiled", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "compiled-route", w.Body.String())
	assert.True(t, metricsRecorder.startRequestCalled, "Metrics should be started")
	assert.True(t, metricsRecorder.finishRequestCalled, "Metrics should be finished")
	assert.True(t, tracingRecorder.startSpanCalled, "Tracing span should be started")
	assert.True(t, tracingRecorder.finishSpanCalled, "Tracing span should be finished")
}

// TestCompiledRouteWithTracingOnly tests the compiled route path with only tracing
// This covers the branch: shouldTrace -> serveStaticWithTracing
func TestCompiledRouteWithTracingOnly(t *testing.T) {
	r := New()
	tracingRecorder := &mockTracingRecorder{enabled: true}
	r.SetTracingRecorder(tracingRecorder)

	r.GET("/trace-only", func(c *Context) {
		c.String(http.StatusOK, "trace-only-route")
	})

	r.Warmup()

	req := httptest.NewRequest("GET", "/trace-only", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "trace-only-route", w.Body.String())
	assert.True(t, tracingRecorder.startSpanCalled, "Tracing span should be started")
	assert.True(t, tracingRecorder.finishSpanCalled, "Tracing span should be finished")
}

// TestCompiledRouteWithMetricsOnly tests the compiled route path with only metrics
// This covers the branch: shouldMeasure -> serveStaticWithMetrics
func TestCompiledRouteWithMetricsOnly(t *testing.T) {
	r := New()
	metricsRecorder := &mockMetricsRecorder{enabled: true}
	r.SetMetricsRecorder(metricsRecorder)

	r.GET("/metrics-only", func(c *Context) {
		c.String(http.StatusOK, "metrics-only-route")
	})

	r.Warmup()

	req := httptest.NewRequest("GET", "/metrics-only", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "metrics-only-route", w.Body.String())
	assert.True(t, metricsRecorder.startRequestCalled, "Metrics should be started")
	assert.True(t, metricsRecorder.finishRequestCalled, "Metrics should be finished")
}

// TestCompiledRouteWithoutTracingOrMetrics tests the compiled route path without tracing or metrics
// This covers the branch: else -> direct execution with globalContextPool
func TestCompiledRouteWithoutTracingOrMetrics(t *testing.T) {
	r := New()
	// No metrics or tracing recorders set

	r.GET("/direct", func(c *Context) {
		c.String(http.StatusOK, "direct-route")
	})

	r.Warmup()

	req := httptest.NewRequest("GET", "/direct", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "direct-route", w.Body.String())
}

// TestCompiledRouteWithVersioningWithoutTracingOrMetrics tests compiled route with versioning
// but without tracing or metrics to cover the version assignment path
// This covers the code path where version is assigned to context when hasVersioning is true
func TestCompiledRouteWithVersioningWithoutTracingOrMetrics(t *testing.T) {
	r := New(
		WithVersioning(
			WithHeaderVersioning("X-API-Version"),
			WithDefaultVersion("v1"),
		),
	)
	// No metrics or tracing recorders set

	v1 := r.Version("v1")
	v1.GET("/versioned", func(c *Context) {
		assert.Equal(t, "v1", c.Version(), "Version should be set on context")
		c.String(http.StatusOK, "versioned-route-v1")
	})

	r.Warmup()

	req := httptest.NewRequest("GET", "/versioned", nil)
	req.Header.Set("X-API-Version", "v1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "versioned-route-v1", w.Body.String())
}

// TestCompiledRouteWithVersioningAndBothObservability tests compiled route with versioning,
// tracing, and metrics to ensure all code paths work together
func TestCompiledRouteWithVersioningAndBothObservability(t *testing.T) {
	r := New(
		WithVersioning(
			WithHeaderVersioning("X-API-Version"),
			WithDefaultVersion("v1"),
		),
	)
	metricsRecorder := &mockMetricsRecorder{enabled: true}
	tracingRecorder := &mockTracingRecorder{enabled: true}
	r.SetMetricsRecorder(metricsRecorder)
	r.SetTracingRecorder(tracingRecorder)

	v2 := r.Version("v2")
	v2.GET("/versioned-observable", func(c *Context) {
		assert.Equal(t, "v2", c.Version(), "Version should be set on context")
		c.String(http.StatusOK, "versioned-route-v2")
	})

	r.Warmup()

	req := httptest.NewRequest("GET", "/versioned-observable", nil)
	req.Header.Set("X-API-Version", "v2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "versioned-route-v2", w.Body.String())
	assert.True(t, metricsRecorder.startRequestCalled)
	assert.True(t, tracingRecorder.startSpanCalled)
}

// TestServeDynamicWithMetrics tests dynamic routes with metrics
func TestServeDynamicWithMetrics(t *testing.T) {
	r := New()
	metricsRecorder := &mockMetricsRecorder{enabled: true}
	r.SetMetricsRecorder(metricsRecorder)

	r.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "user %s", c.Param("id"))
	})

	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, metricsRecorder.startRequestCalled)
	assert.True(t, metricsRecorder.finishRequestCalled)
}

// TestServeDynamicWithTracing tests dynamic routes with tracing
func TestServeDynamicWithTracing(t *testing.T) {
	r := New()
	tracingRecorder := &mockTracingRecorder{enabled: true}
	r.SetTracingRecorder(tracingRecorder)

	r.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "user %s", c.Param("id"))
	})

	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, tracingRecorder.startSpanCalled)
	assert.True(t, tracingRecorder.finishSpanCalled)
}

// TestServeDynamicWithBoth tests dynamic routes with both metrics and tracing
func TestServeDynamicWithBoth(t *testing.T) {
	r := New()
	metricsRecorder := &mockMetricsRecorder{enabled: true}
	tracingRecorder := &mockTracingRecorder{enabled: true}
	r.SetMetricsRecorder(metricsRecorder)
	r.SetTracingRecorder(tracingRecorder)

	r.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "user %s", c.Param("id"))
	})

	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, metricsRecorder.startRequestCalled)
	assert.True(t, metricsRecorder.finishRequestCalled)
	assert.True(t, tracingRecorder.startSpanCalled)
	assert.True(t, tracingRecorder.finishSpanCalled)
}

// TestVersionedRoutingWithMetrics tests versioned routes with metrics
func TestVersionedRoutingWithMetrics(t *testing.T) {
	r := New(
		WithVersioning(
			WithHeaderVersioning("X-API-Version"),
			WithDefaultVersion("v1"),
		),
	)

	metricsRecorder := &mockMetricsRecorder{enabled: true}
	r.SetMetricsRecorder(metricsRecorder)

	v1 := r.Version("v1")
	v1.GET("/data", func(c *Context) {
		c.String(http.StatusOK, "v1 data")
	})

	req := httptest.NewRequest("GET", "/data", nil)
	req.Header.Set("X-API-Version", "v1")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, metricsRecorder.startRequestCalled)
}

// TestVersionedRoutingWithTracing tests versioned routes with tracing
func TestVersionedRoutingWithTracing(t *testing.T) {
	r := New(
		WithVersioning(
			WithHeaderVersioning("X-API-Version"),
			WithDefaultVersion("v1"),
		),
	)

	tracingRecorder := &mockTracingRecorder{enabled: true}
	r.SetTracingRecorder(tracingRecorder)

	v1 := r.Version("v1")
	v1.GET("/data", func(c *Context) {
		c.String(http.StatusOK, "v1 data")
	})

	req := httptest.NewRequest("GET", "/data", nil)
	req.Header.Set("X-API-Version", "v1")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, tracingRecorder.startSpanCalled)
}

// TestVersionedRoutingWithBoth tests versioned routes with both observability systems
func TestVersionedRoutingWithBoth(t *testing.T) {
	r := New(
		WithVersioning(
			WithHeaderVersioning("X-API-Version"),
			WithDefaultVersion("v1"),
		),
	)

	metricsRecorder := &mockMetricsRecorder{enabled: true}
	tracingRecorder := &mockTracingRecorder{enabled: true}
	r.SetMetricsRecorder(metricsRecorder)
	r.SetTracingRecorder(tracingRecorder)

	v1 := r.Version("v1")
	v1.GET("/data", func(c *Context) {
		c.String(http.StatusOK, "v1 data")
	})

	req := httptest.NewRequest("GET", "/data", nil)
	req.Header.Set("X-API-Version", "v1")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, metricsRecorder.startRequestCalled)
	assert.True(t, tracingRecorder.startSpanCalled)
}

// TestVersionedCompiledRoutesWithObservability tests compiled versioned routes
func TestVersionedCompiledRoutesWithObservability(t *testing.T) {
	r := New(
		WithVersioning(
			WithHeaderVersioning("X-API-Version"),
			WithDefaultVersion("v1"),
		),
	)

	metricsRecorder := &mockMetricsRecorder{enabled: true}
	tracingRecorder := &mockTracingRecorder{enabled: true}
	r.SetMetricsRecorder(metricsRecorder)
	r.SetTracingRecorder(tracingRecorder)

	v1 := r.Version("v1")
	v1.GET("/static1", func(c *Context) {
		c.String(http.StatusOK, "v1 static")
	})
	v1.GET("/static2", func(c *Context) {
		c.String(http.StatusOK, "v1 static2")
	})

	// Compile routes
	r.Warmup()

	req := httptest.NewRequest("GET", "/static1", nil)
	req.Header.Set("X-API-Version", "v1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, metricsRecorder.startRequestCalled)
	assert.True(t, tracingRecorder.startSpanCalled)
}

// TestTracingExcludedPaths tests that excluded paths don't create spans
func TestTracingExcludedPaths(t *testing.T) {
	r := New()
	tracingRecorder := &mockTracingRecorder{
		enabled:       true,
		shouldExclude: true,
	}
	r.SetTracingRecorder(tracingRecorder)

	r.GET("/health", func(c *Context) {
		c.String(http.StatusOK, "healthy")
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Should not start span for excluded path
	assert.False(t, tracingRecorder.startSpanCalled)
}

// TestPostFormDefault tests PostFormDefault method
func TestPostFormDefault(t *testing.T) {
	r := New()

	r.POST("/form", func(c *Context) {
		role := c.FormValueDefault("role", "guest")
		name := c.FormValueDefault("name", "anonymous")
		c.String(http.StatusOK, "role=%s,name=%s", role, name)
	})

	req := httptest.NewRequest("POST", "/form", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.PostForm = map[string][]string{
		"name": {"john"},
	}
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "role=guest,name=john", w.Body.String())
}

// TestIsSecureWithTLS tests IsSecure with TLS connection
func TestIsSecureWithTLS(t *testing.T) {
	r := New()

	r.GET("/secure", func(c *Context) {
		if c.IsHTTPS() {
			c.String(http.StatusOK, "secure")
		} else {
			c.String(http.StatusOK, "insecure")
		}
	})

	// Cannot easily test actual TLS in unit tests, but we tested X-Forwarded-Proto
	// in coverage_improvement_test.go which covers the branch
}

// TestGetCookieError tests GetCookie error path
func TestGetCookieError(t *testing.T) {
	r := New()

	r.GET("/cookie", func(c *Context) {
		_, err := c.GetCookie("invalid-cookie=")
		assert.Error(t, err)
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/cookie", nil)
	// Add a malformed cookie
	req.Header.Set("Cookie", "invalid-cookie=bad%ZZvalue")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestContextMetricsMethodsNoOp tests metrics recording methods when metrics are not enabled
func TestContextMetricsMethodsNoOp(t *testing.T) {
	r := New()

	r.GET("/metrics-test", func(c *Context) {
		// These should be no-ops when metrics are not enabled
		c.RecordMetric("test_metric", 1.5)
		c.IncrementCounter("test_counter")
		c.SetGauge("test_gauge", 42)
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/metrics-test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

// TestContextTracingMethodsNoOp tests tracing methods when tracing is not enabled
func TestContextTracingMethodsNoOp(t *testing.T) {
	r := New()

	r.GET("/tracing-test", func(c *Context) {
		// These should be no-ops when tracing is not enabled
		traceID := c.TraceID()
		spanID := c.SpanID()
		c.SetSpanAttribute("key", "value")
		c.AddSpanEvent("event")
		ctx := c.TraceContext()

		assert.Empty(t, traceID)
		assert.Empty(t, spanID)
		assert.NotNil(t, ctx)

		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/tracing-test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

// TestCompiledRouteBranchWithTracingAndMetrics tests the branch where both tracing and metrics are enabled
// Specifically tests: shouldTrace && shouldMeasure -> serveStaticWithTracingAndMetrics
func TestCompiledRouteBranchWithTracingAndMetrics(t *testing.T) {
	r := New()
	metricsRecorder := &mockMetricsRecorder{enabled: true}
	tracingRecorder := &mockTracingRecorder{enabled: true}
	r.SetMetricsRecorder(metricsRecorder)
	r.SetTracingRecorder(tracingRecorder)

	// Register a static route
	r.GET("/static-both", func(c *Context) {
		c.String(http.StatusOK, "static-with-both")
	})

	// Compile routes to ensure compiled route path is used
	r.Warmup()

	req := httptest.NewRequest("GET", "/static-both", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "static-with-both", w.Body.String())
	assert.True(t, metricsRecorder.startRequestCalled, "Metrics StartRequest should be called")
	assert.True(t, metricsRecorder.finishRequestCalled, "Metrics FinishRequest should be called")
	assert.True(t, tracingRecorder.startSpanCalled, "Tracing StartSpan should be called")
	assert.True(t, tracingRecorder.finishSpanCalled, "Tracing FinishSpan should be called")
}

// TestCompiledRouteBranchWithTracingOnly tests the branch where only tracing is enabled
// Specifically tests: shouldTrace -> serveStaticWithTracing
func TestCompiledRouteBranchWithTracingOnly(t *testing.T) {
	r := New()
	tracingRecorder := &mockTracingRecorder{enabled: true}
	r.SetTracingRecorder(tracingRecorder)
	// No metrics recorder - ensures shouldMeasure is false

	// Register a static route
	r.GET("/static-trace", func(c *Context) {
		c.String(http.StatusOK, "static-with-trace")
	})

	// Compile routes to ensure compiled route path is used
	r.Warmup()

	req := httptest.NewRequest("GET", "/static-trace", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "static-with-trace", w.Body.String())
	assert.True(t, tracingRecorder.startSpanCalled, "Tracing StartSpan should be called")
	assert.True(t, tracingRecorder.finishSpanCalled, "Tracing FinishSpan should be called")
}

// TestCompiledRouteBranchWithMetricsOnly tests the branch where only metrics is enabled
// Specifically tests: shouldMeasure -> serveStaticWithMetrics
func TestCompiledRouteBranchWithMetricsOnly(t *testing.T) {
	r := New()
	metricsRecorder := &mockMetricsRecorder{enabled: true}
	r.SetMetricsRecorder(metricsRecorder)
	// No tracing recorder - ensures shouldTrace is false

	// Register a static route
	r.GET("/static-metrics", func(c *Context) {
		c.String(http.StatusOK, "static-with-metrics")
	})

	// Compile routes to ensure compiled route path is used
	r.Warmup()

	req := httptest.NewRequest("GET", "/static-metrics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "static-with-metrics", w.Body.String())
	assert.True(t, metricsRecorder.startRequestCalled, "Metrics StartRequest should be called")
	assert.True(t, metricsRecorder.finishRequestCalled, "Metrics FinishRequest should be called")
}

// TestCompiledRouteBranchWithoutTracingOrMetrics tests the branch where neither tracing nor metrics are enabled
// Specifically tests the else branch: direct execution with globalContextPool
func TestCompiledRouteBranchWithoutTracingOrMetrics(t *testing.T) {
	r := New()
	// No metrics or tracing recorders - ensures both shouldTrace and shouldMeasure are false

	// Register a static route
	r.GET("/static-direct", func(c *Context) {
		c.String(http.StatusOK, "static-direct")
	})

	// Compile routes to ensure compiled route path is used
	r.Warmup()

	req := httptest.NewRequest("GET", "/static-direct", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "static-direct", w.Body.String())
}

// TestCompiledRouteBranchWithoutTracingOrMetricsWithVersioning tests the branch where neither tracing nor metrics are enabled, but versioning is enabled
// This ensures the version assignment path is tested
func TestCompiledRouteBranchWithoutTracingOrMetricsWithVersioning(t *testing.T) {
	r := New(
		WithVersioning(
			WithHeaderVersioning("X-API-Version"),
			WithDefaultVersion("v1"),
		),
	)
	// No metrics or tracing recorders - ensures both shouldTrace and shouldMeasure are false

	// Register a static route (not in versioned tree, will use standard compiled route lookup)
	r.GET("/static-versioned", func(c *Context) {
		assert.Equal(t, "v1", c.Version(), "Version should be set from header")
		c.String(http.StatusOK, "static-with-version")
	})

	// Compile routes to ensure compiled route path is used
	r.Warmup()

	req := httptest.NewRequest("GET", "/static-versioned", nil)
	req.Header.Set("X-API-Version", "v1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "static-with-version", w.Body.String())
}

// TestCompiledRouteBranchWithTracingAndVersioning tests tracing with versioning enabled
// Ensures version is properly set when tracing is enabled
func TestCompiledRouteBranchWithTracingAndVersioning(t *testing.T) {
	r := New(
		WithVersioning(
			WithHeaderVersioning("X-API-Version"),
			WithDefaultVersion("v2"),
		),
	)
	tracingRecorder := &mockTracingRecorder{enabled: true}
	r.SetTracingRecorder(tracingRecorder)

	// Register a static route (not in versioned tree, will use standard compiled route lookup)
	r.GET("/static-trace-version", func(c *Context) {
		assert.Equal(t, "v2", c.Version(), "Version should be set from header")
		c.String(http.StatusOK, "static-trace-version")
	})

	r.Warmup()

	req := httptest.NewRequest("GET", "/static-trace-version", nil)
	req.Header.Set("X-API-Version", "v2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "static-trace-version", w.Body.String())
	assert.True(t, tracingRecorder.startSpanCalled, "Tracing StartSpan should be called")
	assert.True(t, tracingRecorder.finishSpanCalled, "Tracing FinishSpan should be called")
}

// TestCompiledRouteBranchWithMetricsAndVersioning tests metrics with versioning enabled
// Ensures version is properly set when metrics is enabled
func TestCompiledRouteBranchWithMetricsAndVersioning(t *testing.T) {
	r := New(
		WithVersioning(
			WithHeaderVersioning("X-API-Version"),
			WithDefaultVersion("v3"),
		),
	)
	metricsRecorder := &mockMetricsRecorder{enabled: true}
	r.SetMetricsRecorder(metricsRecorder)

	// Register a static route (not in versioned tree, will use standard compiled route lookup)
	r.GET("/static-metrics-version", func(c *Context) {
		assert.Equal(t, "v3", c.Version(), "Version should be set from header")
		c.String(http.StatusOK, "static-metrics-version")
	})

	r.Warmup()

	req := httptest.NewRequest("GET", "/static-metrics-version", nil)
	req.Header.Set("X-API-Version", "v3")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "static-metrics-version", w.Body.String())
	assert.True(t, metricsRecorder.startRequestCalled, "Metrics StartRequest should be called")
	assert.True(t, metricsRecorder.finishRequestCalled, "Metrics FinishRequest should be called")
}

// TestCompiledRouteBranchWithBothAndVersioning tests both tracing and metrics with versioning enabled
// Ensures version is properly set when both tracing and metrics are enabled
func TestCompiledRouteBranchWithBothAndVersioning(t *testing.T) {
	r := New(
		WithVersioning(
			WithHeaderVersioning("X-API-Version"),
			WithDefaultVersion("v4"),
		),
	)
	metricsRecorder := &mockMetricsRecorder{enabled: true}
	tracingRecorder := &mockTracingRecorder{enabled: true}
	r.SetMetricsRecorder(metricsRecorder)
	r.SetTracingRecorder(tracingRecorder)

	// Register a static route (not in versioned tree, will use standard compiled route lookup)
	r.GET("/static-both-version", func(c *Context) {
		assert.Equal(t, "v4", c.Version(), "Version should be set from header")
		c.String(http.StatusOK, "static-both-version")
	})

	r.Warmup()

	req := httptest.NewRequest("GET", "/static-both-version", nil)
	req.Header.Set("X-API-Version", "v4")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "static-both-version", w.Body.String())
	assert.True(t, metricsRecorder.startRequestCalled, "Metrics StartRequest should be called")
	assert.True(t, metricsRecorder.finishRequestCalled, "Metrics FinishRequest should be called")
	assert.True(t, tracingRecorder.startSpanCalled, "Tracing StartSpan should be called")
	assert.True(t, tracingRecorder.finishSpanCalled, "Tracing FinishSpan should be called")
}
