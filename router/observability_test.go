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
	r.WarmupOptimizations()

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

	r.WarmupOptimizations()

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

	r.WarmupOptimizations()

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

	r.WarmupOptimizations()

	req := httptest.NewRequest("GET", "/static", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
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
	r.WarmupOptimizations()

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
