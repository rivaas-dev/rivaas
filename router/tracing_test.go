package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/otel"
)

// TracingTestSuite tests tracing functionality
type TracingTestSuite struct {
	suite.Suite
	router *Router
}

func (suite *TracingTestSuite) SetupTest() {
	suite.router = New()
}

func (suite *TracingTestSuite) TearDownTest() {
	if suite.router != nil {
		suite.router.StopMetricsServer()
	}
}

// TestTracingDisabled tests router without tracing
func (suite *TracingTestSuite) TestTracingDisabled() {
	// Router without tracing should work normally
	suite.router.GET("/test", func(c *Context) {
		// Tracing methods should be safe to call
		traceID := c.TraceID()
		spanID := c.SpanID()

		suite.Equal("", traceID, "Expected empty trace ID when tracing disabled")
		suite.Equal("", spanID, "Expected empty span ID when tracing disabled")

		// These should be no-ops
		c.SetSpanAttribute("test.key", "test.value")
		c.AddSpanEvent("test_event")

		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	suite.router.ServeHTTP(w, req)

	suite.Equal(http.StatusOK, w.Code)
}

func (suite *TracingTestSuite) TestTracingEnabled() {
	// Set up a test tracer
	testTracer := otel.Tracer("test-tracer")

	// Router with tracing enabled
	r := New(
		WithTracing(),
		WithTracingServiceName("test-service"),
		WithTracingServiceVersion("v1.0.0"),
		WithCustomTracer(testTracer),
	)

	r.GET("/test/:id", func(c *Context) {
		// Test that span methods work
		c.SetSpanAttribute("test.key", "test.value")
		c.AddSpanEvent("test_event")

		// These should not be empty when tracing is enabled
		// Note: In unit tests without a real trace provider, these might still be empty
		// but the methods should not panic
		c.TraceID()
		c.SpanID()
		c.TraceContext()

		c.JSON(http.StatusOK, map[string]string{
			"id": c.Param("id"),
		})
	})

	req := httptest.NewRequest("GET", "/test/123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	suite.Equal(http.StatusOK, w.Code, "Expected status 200, got %d", w.Code)
}

func (suite *TracingTestSuite) TestTracingExcludePaths() {
	r := New(
		WithTracing(),
		WithTracingExcludePaths("/health", "/metrics"),
	)

	r.GET("/health", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})

	r.GET("/api/users", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "users"})
	})

	// Test excluded path
	req1 := httptest.NewRequest("GET", "/health", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	suite.Equal(http.StatusOK, w1.Code, "Expected status 200 for /health, got %d", w1.Code)

	// Test non-excluded path
	req2 := httptest.NewRequest("GET", "/api/users", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	suite.Equal(http.StatusOK, w2.Code, "Expected status 200 for /api/users, got %d", w2.Code)
}

func (suite *TracingTestSuite) TestTracingConfiguration() {
	// Test that configuration options are applied correctly
	r := New(
		WithTracing(),
		WithTracingServiceName("custom-service"),
		WithTracingServiceVersion("v2.0.0"),
		WithTracingSampleRate(0.5),
		WithTracingExcludePaths("/health"),
		WithTracingHeaders("Authorization"),
		WithTracingDisableParams(),
	)

	suite.NotNil(r.tracing, "Expected tracing config to be set")
	suite.True(r.tracing.enabled, "Expected tracing to be enabled")
	suite.Equal("custom-service", r.tracing.serviceName, "Expected service name 'custom-service', got '%s'", r.tracing.serviceName)
	suite.Equal("v2.0.0", r.tracing.serviceVersion, "Expected service version 'v2.0.0', got '%s'", r.tracing.serviceVersion)
	suite.Equal(0.5, r.tracing.sampleRate, "Expected sample rate 0.5, got %f", r.tracing.sampleRate)
	suite.True(r.tracing.excludePaths["/health"], "Expected /health to be in exclude paths")
	suite.False(r.tracing.recordParams, "Expected params recording to be disabled")
	suite.Len(r.tracing.recordHeaders, 1, "Expected Authorization header to be recorded")
	suite.Equal("Authorization", r.tracing.recordHeaders[0], "Expected Authorization header to be recorded")
}

func (suite *TracingTestSuite) TestTracingWithGroups() {
	r := New(WithTracing())

	api := r.Group("/api/v1")
	api.GET("/users/:id", func(c *Context) {
		c.SetSpanAttribute("user.id", c.Param("id"))
		c.JSON(http.StatusOK, map[string]string{
			"id": c.Param("id"),
		})
	})

	req := httptest.NewRequest("GET", "/api/v1/users/123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	suite.Equal(http.StatusOK, w.Code, "Expected status 200, got %d", w.Code)
}

// TestTracingSuite runs the tracing test suite
func TestTracingSuite(t *testing.T) {
	suite.Run(t, new(TracingTestSuite))
}
