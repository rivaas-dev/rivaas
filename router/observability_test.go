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

package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"rivaas.dev/router/version"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

// Use mockObservabilityRecorder (see observability_test_helpers.go) instead.
//
// Migration guide:
//   OLD: r.SetMetricsRecorder(&mockMetricsRecorder{enabled: true})
//   NEW: r.SetObservabilityRecorder(newMockObservabilityRecorder(true))
//
//   OLD: r.SetTracingRecorder(&mockTracingRecorder{enabled: true})
//   NEW: r.SetObservabilityRecorder(newMockObservabilityRecorder(true))

// mockContextMetricsRecorder implements ContextMetricsRecorder
type mockContextMetricsRecorder struct {
	recordMetricCalled bool
	incrementCalled    bool
	setGaugeCalled     bool
}

func (m *mockContextMetricsRecorder) RecordMetric(_ context.Context, _ string, _ float64, _ ...attribute.KeyValue) {
	m.recordMetricCalled = true
}

func (m *mockContextMetricsRecorder) IncrementCounter(_ context.Context, _ string, _ ...attribute.KeyValue) {
	m.incrementCalled = true
}

func (m *mockContextMetricsRecorder) SetGauge(_ context.Context, _ string, _ float64, _ ...attribute.KeyValue) {
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

func (m *mockContextTracingRecorder) SetSpanAttribute(_ string, _ any) {
	m.setAttributeCalled = true
}

func (m *mockContextTracingRecorder) AddSpanEvent(_ string, _ ...attribute.KeyValue) {
	m.addEventCalled = true
}

func (m *mockContextTracingRecorder) TraceContext() context.Context {
	return context.Background()
}

// TestContextWithMetrics tests context metrics methods with actual recorder
func TestContextWithMetrics(t *testing.T) {
	t.Parallel()
	r := MustNew()
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

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
	t.Parallel()
	r := MustNew()

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
	t.Parallel()
	r := MustNew()
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	r.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Positive(t, mockObs.startCalls.Load())
	assert.Positive(t, mockObs.endCalls.Load())
}

// TestServeHTTPWithTracing tests ServeHTTP with tracing enabled
func TestServeHTTPWithTracing(t *testing.T) {
	t.Parallel()
	r := MustNew()
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	r.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Positive(t, mockObs.startCalls.Load())
	assert.Positive(t, mockObs.endCalls.Load())
}

// TestServeHTTPWithBothMetricsAndTracing tests with both enabled
func TestServeHTTPWithBothMetricsAndTracing(t *testing.T) {
	t.Parallel()
	r := MustNew()
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	r.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Positive(t, mockObs.startCalls.Load())
	assert.Positive(t, mockObs.endCalls.Load())
	assert.Positive(t, mockObs.startCalls.Load())
	assert.Positive(t, mockObs.endCalls.Load())
}

// TestServeHTTPWithCompiledRoutes tests compiled route path
func TestServeHTTPWithCompiledRoutes(t *testing.T) {
	t.Parallel()
	r := MustNew()

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
	t.Parallel()
	r := MustNew()
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	r.GET("/static", func(c *Context) {
		c.String(http.StatusOK, "static")
	})

	r.Warmup()

	req := httptest.NewRequest("GET", "/static", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Positive(t, mockObs.startCalls.Load())
}

// TestServeHTTPWithCompiledRoutesAndTracing tests compiled routes with tracing
func TestServeHTTPWithCompiledRoutesAndTracing(t *testing.T) {
	t.Parallel()
	r := MustNew()
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	r.GET("/static", func(c *Context) {
		c.String(http.StatusOK, "static")
	})

	r.Warmup()

	req := httptest.NewRequest("GET", "/static", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Positive(t, mockObs.startCalls.Load())
}

// TestServeHTTPWithCompiledRoutesAndBoth tests compiled routes with both metrics and tracing
func TestServeHTTPWithCompiledRoutesAndBoth(t *testing.T) {
	t.Parallel()
	r := MustNew()
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	r.GET("/static", func(c *Context) {
		c.String(http.StatusOK, "static")
	})

	r.Warmup()

	req := httptest.NewRequest("GET", "/static", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Positive(t, mockObs.startCalls.Load())
	assert.Positive(t, mockObs.startCalls.Load())
}

// TestCompiledRouteWithTracingAndMetrics tests the compiled route path with both tracing and metrics
// This covers the branch: shouldTrace && shouldMeasure -> serveStaticWithTracingAndMetrics
func TestCompiledRouteWithTracingAndMetrics(t *testing.T) {
	t.Parallel()
	r := MustNew()
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

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
	assert.Positive(t, mockObs.startCalls.Load(), "Metrics should be started")
	assert.Positive(t, mockObs.endCalls.Load(), "Metrics should be finished")
	assert.Positive(t, mockObs.startCalls.Load(), "Tracing span should be started")
	assert.Positive(t, mockObs.endCalls.Load(), "Tracing span should be finished")
}

// TestCompiledRouteWithTracingOnly tests the compiled route path with only tracing
// This covers the branch: shouldTrace -> serveStaticWithTracing
func TestCompiledRouteWithTracingOnly(t *testing.T) {
	t.Parallel()
	r := MustNew()
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	r.GET("/trace-only", func(c *Context) {
		c.String(http.StatusOK, "trace-only-route")
	})

	r.Warmup()

	req := httptest.NewRequest("GET", "/trace-only", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "trace-only-route", w.Body.String())
	assert.Positive(t, mockObs.startCalls.Load(), "Tracing span should be started")
	assert.Positive(t, mockObs.endCalls.Load(), "Tracing span should be finished")
}

// TestCompiledRouteWithMetricsOnly tests the compiled route path with only metrics
// This covers the branch: shouldMeasure -> serveStaticWithMetrics
func TestCompiledRouteWithMetricsOnly(t *testing.T) {
	t.Parallel()
	r := MustNew()
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	r.GET("/metrics-only", func(c *Context) {
		c.String(http.StatusOK, "metrics-only-route")
	})

	r.Warmup()

	req := httptest.NewRequest("GET", "/metrics-only", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "metrics-only-route", w.Body.String())
	assert.Positive(t, mockObs.startCalls.Load(), "Metrics should be started")
	assert.Positive(t, mockObs.endCalls.Load(), "Metrics should be finished")
}

// TestCompiledRouteWithoutTracingOrMetrics tests the compiled route path without tracing or metrics
// This covers the branch: else -> direct execution with globalContextPool
func TestCompiledRouteWithoutTracingOrMetrics(t *testing.T) {
	t.Parallel()
	r := MustNew()
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
	t.Parallel()
	r := MustNew(
		WithVersioning(
			version.WithHeaderDetection("X-API-Version"),
			version.WithDefault("v1"),
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
	t.Parallel()
	r := MustNew(
		WithVersioning(
			version.WithHeaderDetection("X-API-Version"),
			version.WithDefault("v1"),
		),
	)
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

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
	assert.Positive(t, mockObs.startCalls.Load())
	assert.Positive(t, mockObs.endCalls.Load())
}

// TestServeDynamicWithMetrics tests dynamic routes with metrics
func TestServeDynamicWithMetrics(t *testing.T) {
	t.Parallel()
	r := MustNew()
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	r.GET("/users/:id", func(c *Context) {
		c.Stringf(http.StatusOK, "user %s", c.Param("id"))
	})

	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Positive(t, mockObs.startCalls.Load())
	assert.Positive(t, mockObs.endCalls.Load())
}

// TestServeDynamicWithTracing tests dynamic routes with tracing
func TestServeDynamicWithTracing(t *testing.T) {
	t.Parallel()
	r := MustNew()
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	r.GET("/users/:id", func(c *Context) {
		c.Stringf(http.StatusOK, "user %s", c.Param("id"))
	})

	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Positive(t, mockObs.startCalls.Load())
	assert.Positive(t, mockObs.endCalls.Load())
}

// TestServeDynamicWithBoth tests dynamic routes with both metrics and tracing
func TestServeDynamicWithBoth(t *testing.T) {
	t.Parallel()
	r := MustNew()
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	r.GET("/users/:id", func(c *Context) {
		c.Stringf(http.StatusOK, "user %s", c.Param("id"))
	})

	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Positive(t, mockObs.startCalls.Load())
	assert.Positive(t, mockObs.endCalls.Load())
}

// TestVersionedRoutingWithMetrics tests versioned routes with metrics
//
//nolint:paralleltest // Tests observability recorder state
func TestVersionedRoutingWithMetrics(t *testing.T) {
	r := MustNew(
		WithVersioning(
			version.WithHeaderDetection("X-API-Version"),
			version.WithDefault("v1"),
		),
	)

	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	v1 := r.Version("v1")
	v1.GET("/data", func(c *Context) {
		c.String(http.StatusOK, "v1 data")
	})

	req := httptest.NewRequest("GET", "/data", nil)
	req.Header.Set("X-API-Version", "v1")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Positive(t, mockObs.startCalls.Load())
}

// TestVersionedRoutingWithTracing tests versioned routes with tracing
func TestVersionedRoutingWithTracing(t *testing.T) {
	t.Parallel()
	r := MustNew(
		WithVersioning(
			version.WithHeaderDetection("X-API-Version"),
			version.WithDefault("v1"),
		),
	)

	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	v1 := r.Version("v1")
	v1.GET("/data", func(c *Context) {
		c.String(http.StatusOK, "v1 data")
	})

	req := httptest.NewRequest("GET", "/data", nil)
	req.Header.Set("X-API-Version", "v1")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Positive(t, mockObs.startCalls.Load())
}

// TestVersionedRoutingWithBoth tests versioned routes with both observability systems
func TestVersionedRoutingWithBoth(t *testing.T) {
	t.Parallel()
	r := MustNew(
		WithVersioning(
			version.WithHeaderDetection("X-API-Version"),
			version.WithDefault("v1"),
		),
	)

	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	v1 := r.Version("v1")
	v1.GET("/data", func(c *Context) {
		c.String(http.StatusOK, "v1 data")
	})

	req := httptest.NewRequest("GET", "/data", nil)
	req.Header.Set("X-API-Version", "v1")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Positive(t, mockObs.startCalls.Load())
	assert.Positive(t, mockObs.endCalls.Load())
}

// TestVersionedCompiledRoutesWithObservability tests compiled versioned routes
func TestVersionedCompiledRoutesWithObservability(t *testing.T) {
	t.Parallel()
	r := MustNew(
		WithVersioning(
			version.WithHeaderDetection("X-API-Version"),
			version.WithDefault("v1"),
		),
	)

	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

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
	assert.Positive(t, mockObs.startCalls.Load())
	assert.Positive(t, mockObs.endCalls.Load())
}

// TestTracingExcludedPaths tests that excluded paths don't create spans
//
//nolint:paralleltest // Tests observability recorder state
func TestTracingExcludedPaths(t *testing.T) {
	r := MustNew()
	mockObs := newMockObservabilityWithExclusion("/health")
	r.SetObservabilityRecorder(mockObs)

	r.GET("/health", func(c *Context) {
		c.String(http.StatusOK, "healthy")
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Should not start observability for excluded path
	assert.Equal(t, int32(0), mockObs.endCalls.Load(), "excluded path should not trigger OnRequestEnd")
}

// TestPostFormDefault tests PostFormDefault method
func TestPostFormDefault(t *testing.T) {
	t.Parallel()
	r := MustNew()

	r.POST("/form", func(c *Context) {
		role := c.FormValueDefault("role", "guest")
		name := c.FormValueDefault("name", "anonymous")
		c.Stringf(http.StatusOK, "role=%s,name=%s", role, name)
	})

	req := httptest.NewRequest(http.MethodPost, "/form", nil)
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
//
//nolint:paralleltest // Tests TLS state
func TestIsSecureWithTLS(_ *testing.T) {
	r := MustNew()

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
func TestGetCookieError(t *testing.T) { //nolint:paralleltest // Tests error handling behavior
	r := MustNew()

	r.GET("/cookie", func(c *Context) {
		_, err := c.GetCookie("invalid-cookie=")
		require.Error(t, err)
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
	t.Parallel()
	r := MustNew()

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
	t.Parallel()
	r := MustNew()

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
//
//nolint:paralleltest // Tests observability recorder state
func TestCompiledRouteBranchWithTracingAndMetrics(t *testing.T) {
	r := MustNew()
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

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
	assert.Positive(t, mockObs.startCalls.Load(), "StartRequest should be called")
	assert.Positive(t, mockObs.endCalls.Load(), "FinishRequest should be called")
}

// TestCompiledRouteBranchWithTracingOnly tests the branch where only tracing is enabled
// Specifically tests: shouldTrace -> serveStaticWithTracing
func TestCompiledRouteBranchWithTracingOnly(t *testing.T) {
	t.Parallel()
	r := MustNew()
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)
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
	assert.Positive(t, mockObs.startCalls.Load(), "Tracing StartSpan should be called")
	assert.Positive(t, mockObs.endCalls.Load(), "Tracing FinishSpan should be called")
}

// TestCompiledRouteBranchWithMetricsOnly tests the branch where only metrics is enabled
// Specifically tests: shouldMeasure -> serveStaticWithMetrics
//
//nolint:paralleltest // Tests observability recorder state
func TestCompiledRouteBranchWithMetricsOnly(t *testing.T) {
	r := MustNew()
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)
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
	assert.Positive(t, mockObs.startCalls.Load(), "Metrics StartRequest should be called")
	assert.Positive(t, mockObs.endCalls.Load(), "Metrics FinishRequest should be called")
}

// TestCompiledRouteBranchWithoutTracingOrMetrics tests the branch where neither tracing nor metrics are enabled
// Specifically tests the else branch: direct execution with globalContextPool
func TestCompiledRouteBranchWithoutTracingOrMetrics(t *testing.T) {
	t.Parallel()
	r := MustNew()
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
	t.Parallel()
	r := MustNew(
		WithVersioning(
			version.WithHeaderDetection("X-API-Version"),
			version.WithDefault("v1"),
		),
	)
	// No metrics or tracing recorders - ensures both shouldTrace and shouldMeasure are false

	// Register route via Version() to enable version detection
	// Non-versioned routes (r.GET) bypass version detection entirely
	v1 := r.Version("v1")
	v1.GET("/static-versioned", func(c *Context) {
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
	t.Parallel()
	r := MustNew(
		WithVersioning(
			version.WithHeaderDetection("X-API-Version"),
			version.WithDefault("v2"),
		),
	)
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	// Register route via Version() to enable version detection
	// Non-versioned routes (r.GET) bypass version detection entirely
	v2 := r.Version("v2")
	v2.GET("/static-trace-version", func(c *Context) {
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
	assert.Positive(t, mockObs.startCalls.Load(), "Tracing StartSpan should be called")
	assert.Positive(t, mockObs.endCalls.Load(), "Tracing FinishSpan should be called")
}

// TestCompiledRouteBranchWithMetricsAndVersioning tests metrics with versioning enabled
// Ensures version is properly set when metrics is enabled
func TestCompiledRouteBranchWithMetricsAndVersioning(t *testing.T) {
	t.Parallel()
	r := MustNew(
		WithVersioning(
			version.WithHeaderDetection("X-API-Version"),
			version.WithDefault("v3"),
		),
	)
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	// Register route via Version() to enable version detection
	// Non-versioned routes (r.GET) bypass version detection entirely
	v3 := r.Version("v3")
	v3.GET("/static-metrics-version", func(c *Context) {
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
	assert.Positive(t, mockObs.startCalls.Load(), "Metrics StartRequest should be called")
	assert.Positive(t, mockObs.endCalls.Load(), "Metrics FinishRequest should be called")
}

// TestCompiledRouteBranchWithBothAndVersioning tests both tracing and metrics with versioning enabled
// Ensures version is properly set when both tracing and metrics are enabled
func TestCompiledRouteBranchWithBothAndVersioning(t *testing.T) {
	t.Parallel()
	r := MustNew(
		WithVersioning(
			version.WithHeaderDetection("X-API-Version"),
			version.WithDefault("v4"),
		),
	)
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	// Register route via Version() to enable version detection
	// Non-versioned routes (r.GET) bypass version detection entirely
	v4 := r.Version("v4")
	v4.GET("/static-both-version", func(c *Context) {
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
	assert.Positive(t, mockObs.startCalls.Load(), "Metrics StartRequest should be called")
	assert.Positive(t, mockObs.endCalls.Load(), "Metrics FinishRequest should be called")
	assert.Positive(t, mockObs.startCalls.Load(), "Tracing StartSpan should be called")
	assert.Positive(t, mockObs.endCalls.Load(), "Tracing FinishSpan should be called")
}

// TestContextCustomMetricsFromHandler tests that custom metrics called from handlers
// are properly recorded when metricsRecorder is initialized during context setup.
// This test verifies the fix for the bug where c.metricsRecorder was nil,
// causing custom metrics to silently fail.
func TestContextCustomMetricsFromHandler(t *testing.T) {
	t.Parallel()
	r := MustNew()

	// Create a mock metrics recorder that implements both MetricsRecorder and ContextMetricsRecorder
	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	// Track if custom metrics were called
	var (
		counterCalled   bool
		histogramCalled bool
		gaugeCalled     bool
	)

	// Register a handler that calls custom metrics
	r.GET("/users/:id", func(c *Context) {
		// These calls should work because c.metricsRecorder is properly initialized
		c.IncrementCounter("user_lookups_total",
			attribute.String("user_id", c.Param("id")),
			attribute.String("result", "success"),
		)
		counterCalled = true

		c.RecordMetric("user_lookup_duration_seconds", 0.123,
			attribute.String("user_id", c.Param("id")),
		)
		histogramCalled = true

		c.SetGauge("active_user_sessions", 42,
			attribute.String("user_id", c.Param("id")),
		)
		gaugeCalled = true

		c.JSON(http.StatusOK, map[string]any{
			"id":   c.Param("id"),
			"name": "John Doe",
		})
	})

	// Make a request to the handler
	req := httptest.NewRequest("GET", "/users/5", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Verify the handler executed successfully
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify custom metrics were called
	assert.True(t, counterCalled, "IncrementCounter should have been called")
	assert.True(t, histogramCalled, "RecordMetric should have been called")
	assert.True(t, gaugeCalled, "SetGauge should have been called")

	// Verify metrics recorder received the calls
}

// TestContextCustomMetricsWithoutMetricsRecorder tests that custom metrics
// gracefully handle the case when no metrics recorder is configured.
func TestContextCustomMetricsWithoutMetricsRecorder(t *testing.T) {
	t.Parallel()
	r := MustNew()
	// No metrics recorder set

	r.GET("/test", func(c *Context) {
		// These should not panic even when metricsRecorder is nil
		c.IncrementCounter("test_counter")
		c.RecordMetric("test_metric", 1.5)
		c.SetGauge("test_gauge", 42)

		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Should not panic
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestContextCustomMetricsWithCompiledRoutes tests that custom metrics work
// when routes are matched via compiled routes.
func TestContextCustomMetricsWithCompiledRoutes(t *testing.T) {
	t.Parallel()
	r := MustNew()

	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	var metricsCalled bool

	// Register a static route that will be compiled
	r.GET("/api/v1/products/:id", func(c *Context) {
		c.IncrementCounter("product_views_total",
			attribute.String("product_id", c.Param("id")),
		)
		metricsCalled = true
		c.Stringf(http.StatusOK, "product: %s", c.Param("id"))
	})

	// Compile routes for optimized matching
	r.CompileAllRoutes()

	// Make request
	req := httptest.NewRequest("GET", "/api/v1/products/123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, metricsCalled, "Custom metrics should work with compiled routes")
}

// TestContextCustomMetricsWithVersionedRoutes tests that custom metrics work
// in versioned route handlers.
func TestContextCustomMetricsWithVersionedRoutes(t *testing.T) {
	t.Parallel()
	r := MustNew(
		WithVersioning(
			version.WithPathDetection("/v{version}/"),
			version.WithDefault("v1"),
			version.WithValidVersions("v1", "v2"),
		),
	)

	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	var metricsCalled bool

	// Register versioned route
	v1 := r.Version("v1")
	v1.GET("/products/:id", func(c *Context) {
		c.IncrementCounter("versioned_product_views_total",
			attribute.String("version", c.Version()),
			attribute.String("product_id", c.Param("id")),
		)
		metricsCalled = true
		c.JSON(http.StatusOK, map[string]any{
			"id":      c.Param("id"),
			"version": c.Version(),
		})
	})

	// Make request to versioned route
	req := httptest.NewRequest("GET", "/v1/products/456", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, metricsCalled, "Custom metrics should work in versioned routes")
}

// TestVersionedRouteParameterExtraction tests that path parameters are properly
// extracted and available in versioned route handlers.
// This test verifies the fix where parameters were lost when a new context
// was created in serveVersionedRequest.
func TestVersionedRouteParameterExtraction(t *testing.T) {
	t.Parallel()
	r := MustNew(
		WithVersioning(
			version.WithPathDetection("/v{version}/"),
			version.WithDefault("v1"),
			version.WithValidVersions("v1", "v2"),
		),
	)

	var (
		handlerCalled bool
		capturedID    string
		capturedName  string
	)

	// Register a versioned route with multiple parameters
	v1 := r.Version("v1")
	v1.GET("/users/:id/posts/:postId", func(c *Context) {
		handlerCalled = true
		capturedID = c.Param("id")
		capturedName = c.Param("postId")
		c.JSON(http.StatusOK, map[string]any{
			"user_id": c.Param("id"),
			"post_id": c.Param("postId"),
		})
	})

	// Test with versioned route
	req := httptest.NewRequest("GET", "/v1/users/123/posts/456", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Verify handler was called
	assert.True(t, handlerCalled, "Handler should have been called")

	// Verify parameters were extracted correctly
	assert.Equal(t, "123", capturedID, "User ID parameter should be extracted")
	assert.Equal(t, "456", capturedName, "Post ID parameter should be extracted")
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify JSON response contains correct parameters
	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "123", response["user_id"])
	assert.Equal(t, "456", response["post_id"])
}

// TestVersionedRouteParameterExtractionWithMetrics tests parameter extraction
// in versioned routes when metrics are enabled (different code path).
func TestVersionedRouteParameterExtractionWithMetrics(t *testing.T) {
	t.Parallel()
	r := MustNew(
		WithVersioning(
			version.WithPathDetection("/v{version}/"),
			version.WithDefault("v1"),
			version.WithValidVersions("v1"),
		),
	)

	mockObs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(mockObs)

	var capturedParam string

	v1 := r.Version("v1")
	v1.GET("/products/:id", func(c *Context) {
		capturedParam = c.Param("id")
		c.Stringf(http.StatusOK, "product: %s", c.Param("id"))
	})

	req := httptest.NewRequest("GET", "/v1/products/abc123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "abc123", capturedParam, "Parameter should be extracted with metrics enabled")
	assert.Contains(t, w.Body.String(), "abc123")
}

// TestNonVersionedPathRoutingToMainTree tests that requests without version
// prefixes are routed to the main tree, not versioned trees.
// This verifies the fix where all requests were incorrectly matched
// against versioned trees.
//
//nolint:paralleltest // Tests version routing state
func TestNonVersionedPathRoutingToMainTree(t *testing.T) {
	r := MustNew(
		WithVersioning(
			version.WithPathDetection("/v{version}/"),
			version.WithDefault("v1"),
			version.WithValidVersions("v1"),
		),
	)

	var (
		mainHandlerCalled      bool
		versionedHandlerCalled bool
	)

	// Register route in main tree (non-versioned)
	r.GET("/users/:id", func(c *Context) {
		mainHandlerCalled = true
		c.JSON(http.StatusOK, map[string]any{
			"source": "main",
			"id":     c.Param("id"),
		})
	})

	// Register route in versioned tree
	v1 := r.Version("v1")
	v1.GET("/users/:id", func(c *Context) {
		versionedHandlerCalled = true
		c.JSON(http.StatusOK, map[string]any{
			"source": "versioned",
			"id":     c.Param("id"),
		})
	})

	// Test 1: Request WITHOUT version prefix should use main handler
	req1 := httptest.NewRequest("GET", "/users/100", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	assert.True(t, mainHandlerCalled, "Main handler should be called for non-versioned path")
	assert.False(t, versionedHandlerCalled, "Versioned handler should NOT be called")
	assert.Equal(t, http.StatusOK, w1.Code)

	var response1 map[string]any
	json.Unmarshal(w1.Body.Bytes(), &response1)
	assert.Equal(t, "main", response1["source"])

	// Reset flags
	mainHandlerCalled = false
	versionedHandlerCalled = false

	// Test 2: Request WITH version prefix should use versioned handler
	req2 := httptest.NewRequest("GET", "/v1/users/200", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	assert.False(t, mainHandlerCalled, "Main handler should NOT be called for versioned path")
	assert.True(t, versionedHandlerCalled, "Versioned handler should be called")
	assert.Equal(t, http.StatusOK, w2.Code)

	var response2 map[string]any
	json.Unmarshal(w2.Body.Bytes(), &response2)
	assert.Equal(t, "versioned", response2["source"])
}

// TestVersionedAndNonVersionedRoutesSeparation tests that versioned and
// non-versioned routes with the same paths are kept separate and route correctly.
func TestVersionedAndNonVersionedRoutesSeparation(t *testing.T) {
	t.Parallel()
	r := MustNew(
		WithVersioning(
			version.WithPathDetection("/v{version}/"),
			version.WithDefault("v1"),
		),
	)

	var (
		usersHandlerCalled    bool
		ordersHandlerCalled   bool
		productsHandlerCalled bool
	)

	// Register multiple non-versioned routes
	r.GET("/users/:id", func(c *Context) {
		usersHandlerCalled = true
		c.Stringf(http.StatusOK, "user-%s", c.Param("id"))
	})

	r.GET("/orders/:id", func(c *Context) {
		ordersHandlerCalled = true
		c.Stringf(http.StatusOK, "order-%s", c.Param("id"))
	})

	r.GET("/products/:id", func(c *Context) {
		productsHandlerCalled = true
		c.Stringf(http.StatusOK, "product-%s", c.Param("id"))
	})

	// Register versioned routes with same paths
	v1 := r.Version("v1")
	v1.GET("/products/:id", func(c *Context) {
		c.Stringf(http.StatusOK, "v1-product-%s", c.Param("id"))
	})

	// Test each non-versioned route
	tests := []struct {
		path         string
		expectedFlag *bool
		expectedBody string
	}{
		{"/users/1", &usersHandlerCalled, "user-1"},
		{"/orders/2", &ordersHandlerCalled, "order-2"},
		{"/products/3", &productsHandlerCalled, "product-3"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.True(t, *tt.expectedFlag, "Expected handler should be called")
			assert.Equal(t, tt.expectedBody, w.Body.String())
		})
	}
}

// TestMainTreeIgnoresVersionHeader tests that routes registered directly on the router
// (non-versioned routes) are matched BEFORE version detection, even when a version header
// is present. This confirms the design where:
// - r.GET("/path", h) → Main tree, bypasses version detection
// - r.Version("v1").GET("/path", h) → Version tree, subject to version detection
func TestMainTreeIgnoresVersionHeader(t *testing.T) {
	t.Parallel()

	// Helper to create a router with versioning and tracking handlers
	setupRouter := func() (*Router, *bool, *bool, *bool, *bool) {
		r := MustNew(
			WithVersioning(
				version.WithHeaderDetection("X-API-Version"),
				version.WithDefault("v1"),
				version.WithValidVersions("v1", "v2"),
			),
		)

		var (
			healthHandlerCalled  bool
			metricsHandlerCalled bool
			v1UsersHandlerCalled bool
			v2UsersHandlerCalled bool
		)

		// Register non-versioned routes (should bypass version detection)
		r.GET("/health", func(c *Context) {
			healthHandlerCalled = true
			c.String(http.StatusOK, "healthy")
		})

		r.GET("/metrics", func(c *Context) {
			metricsHandlerCalled = true
			c.String(http.StatusOK, "metrics")
		})

		// Register versioned routes
		r.Version("v1").GET("/users", func(c *Context) {
			v1UsersHandlerCalled = true
			c.String(http.StatusOK, "v1 users")
		})

		r.Version("v2").GET("/users", func(c *Context) {
			v2UsersHandlerCalled = true
			c.String(http.StatusOK, "v2 users")
		})

		return r, &healthHandlerCalled, &metricsHandlerCalled, &v1UsersHandlerCalled, &v2UsersHandlerCalled
	}

	// Test 1: /health should work WITHOUT version header
	t.Run("health without header", func(t *testing.T) {
		t.Parallel()
		r, healthCalled, _, _, _ := setupRouter()

		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.True(t, *healthCalled, "Health handler should be called")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "healthy", w.Body.String())
	})

	// Test 2: /health should STILL work WITH version header (ignored)
	t.Run("health with v2 header", func(t *testing.T) {
		t.Parallel()
		r, healthCalled, _, _, _ := setupRouter()

		req := httptest.NewRequest("GET", "/health", nil)
		req.Header.Set("X-API-Version", "v2")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.True(t, *healthCalled, "Health handler should be called even with version header")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "healthy", w.Body.String())
	})

	// Test 3: /metrics should work with any version header
	t.Run("metrics with v1 header", func(t *testing.T) {
		t.Parallel()
		r, _, metricsCalled, _, _ := setupRouter()

		req := httptest.NewRequest("GET", "/metrics", nil)
		req.Header.Set("X-API-Version", "v1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.True(t, *metricsCalled, "Metrics handler should be called")
		assert.Equal(t, http.StatusOK, w.Code)
	})

	// Test 4: /users with v1 header should use v1 handler
	t.Run("users with v1 header", func(t *testing.T) {
		t.Parallel()
		r, _, _, v1Called, v2Called := setupRouter()

		req := httptest.NewRequest("GET", "/users", nil)
		req.Header.Set("X-API-Version", "v1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.True(t, *v1Called, "V1 users handler should be called")
		assert.False(t, *v2Called, "V2 users handler should NOT be called")
		assert.Equal(t, "v1 users", w.Body.String())
	})

	// Test 5: /users with v2 header should use v2 handler
	t.Run("users with v2 header", func(t *testing.T) {
		t.Parallel()
		r, _, _, v1Called, v2Called := setupRouter()

		req := httptest.NewRequest("GET", "/users", nil)
		req.Header.Set("X-API-Version", "v2")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.False(t, *v1Called, "V1 users handler should NOT be called")
		assert.True(t, *v2Called, "V2 users handler should be called")
		assert.Equal(t, "v2 users", w.Body.String())
	})

	// Test 6: /users without header should use default (v1)
	t.Run("users without header uses default", func(t *testing.T) {
		t.Parallel()
		r, _, _, v1Called, v2Called := setupRouter()

		req := httptest.NewRequest("GET", "/users", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.True(t, *v1Called, "V1 users handler should be called (default)")
		assert.False(t, *v2Called, "V2 users handler should NOT be called")
		assert.Equal(t, "v1 users", w.Body.String())
	})
}

// TestRouteHandlerIsolationWithMiddleware tests that multiple routes with
// middleware don't share handlers due to slice aliasing.
func TestRouteHandlerIsolationWithMiddleware(t *testing.T) {
	t.Parallel()
	r := MustNew()

	// Add global middleware
	r.Use(func(c *Context) {
		c.Next()
	})

	var (
		handler1Called  bool
		handler2Called  bool
		handler3Called  bool
		handler1Context string
		handler2Context string
		handler3Context string
	)

	// Register multiple routes
	r.GET("/route1", func(c *Context) {
		handler1Called = true
		handler1Context = "handler1"
		c.String(http.StatusOK, "route1")
	})

	r.GET("/route2", func(c *Context) {
		handler2Called = true
		handler2Context = "handler2"
		c.String(http.StatusOK, "route2")
	})

	r.GET("/route3", func(c *Context) {
		handler3Called = true
		handler3Context = "handler3"
		c.String(http.StatusOK, "route3")
	})

	// Test route 1
	req1 := httptest.NewRequest("GET", "/route1", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	assert.True(t, handler1Called, "Handler 1 should be called")
	assert.False(t, handler2Called, "Handler 2 should NOT be called")
	assert.False(t, handler3Called, "Handler 3 should NOT be called")
	assert.Equal(t, "handler1", handler1Context)
	assert.Equal(t, "route1", w1.Body.String())

	// Reset
	handler1Called = false

	// Test route 2
	req2 := httptest.NewRequest("GET", "/route2", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	assert.False(t, handler1Called, "Handler 1 should NOT be called")
	assert.True(t, handler2Called, "Handler 2 should be called")
	assert.False(t, handler3Called, "Handler 3 should NOT be called")
	assert.Equal(t, "handler2", handler2Context)
	assert.Equal(t, "route2", w2.Body.String())

	// Reset
	handler2Called = false

	// Test route 3
	req3 := httptest.NewRequest("GET", "/route3", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)

	assert.False(t, handler1Called, "Handler 1 should NOT be called")
	assert.False(t, handler2Called, "Handler 2 should NOT be called")
	assert.True(t, handler3Called, "Handler 3 should be called")
	assert.Equal(t, "handler3", handler3Context)
	assert.Equal(t, "route3", w3.Body.String())
}

// TestRouteHandlerIsolationWithDistinctResponses tests that each route returns
// its own response and doesn't execute other routes' handlers.
func TestRouteHandlerIsolationWithDistinctResponses(t *testing.T) {
	t.Parallel()
	r := MustNew()

	// Add middleware
	r.Use(func(c *Context) {
		c.Next()
	})

	// Register routes with distinct responses
	r.GET("/users/:id", func(c *Context) {
		c.JSON(http.StatusOK, map[string]any{
			"type": "user",
			"id":   c.Param("id"),
		})
	})

	r.GET("/products/:id", func(c *Context) {
		c.JSON(http.StatusOK, map[string]any{
			"type": "product",
			"id":   c.Param("id"),
		})
	})

	r.GET("/orders/:id", func(c *Context) {
		c.JSON(http.StatusOK, map[string]any{
			"type": "order",
			"id":   c.Param("id"),
		})
	})

	// Test each route returns correct type
	tests := []struct {
		path         string
		expectedType string
		expectedID   string
	}{
		{"/users/1", "user", "1"},
		{"/products/2", "product", "2"},
		{"/orders/3", "order", "3"},
		{"/users/4", "user", "4"}, // Test users again to ensure no cross-contamination
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]any
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedType, response["type"], "Route should return its own type")
			assert.Equal(t, tt.expectedID, response["id"], "Route should return correct ID")
		})
	}
}

// TestRouteHandlerIsolationWithConstraints tests that routes with constraints
// don't share handlers due to slice aliasing (constraints trigger finalizeRoute twice).
func TestRouteHandlerIsolationWithConstraints(t *testing.T) {
	t.Parallel()
	r := MustNew()

	r.Use(func(c *Context) {
		c.Next()
	})

	var (
		usersHandlerCalled bool
		postsHandlerCalled bool
	)

	// Register routes with constraints (triggers finalizeRoute twice)
	r.GET("/users/:id", func(c *Context) {
		usersHandlerCalled = true
		c.String(http.StatusOK, "user")
	}).WhereInt("id")

	r.GET("/posts/:id", func(c *Context) {
		postsHandlerCalled = true
		c.String(http.StatusOK, "post")
	}).WhereInt("id")

	// Test users route
	req1 := httptest.NewRequest("GET", "/users/123", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	assert.True(t, usersHandlerCalled, "Users handler should be called")
	assert.False(t, postsHandlerCalled, "Posts handler should NOT be called")
	assert.Equal(t, "user", w1.Body.String())

	// Reset
	usersHandlerCalled = false

	// Test posts route
	req2 := httptest.NewRequest("GET", "/posts/456", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	assert.False(t, usersHandlerCalled, "Users handler should NOT be called")
	assert.True(t, postsHandlerCalled, "Posts handler should be called")
	assert.Equal(t, "post", w2.Body.String())
}
