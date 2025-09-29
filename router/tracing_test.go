package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
)

func TestTracingDisabled(t *testing.T) {
	// Router without tracing should work normally
	r := New()

	r.GET("/test", func(c *Context) {
		// Tracing methods should be safe to call
		traceID := c.TraceID()
		spanID := c.SpanID()

		if traceID != "" {
			t.Error("Expected empty trace ID when tracing disabled")
		}
		if spanID != "" {
			t.Error("Expected empty span ID when tracing disabled")
		}

		// These should be no-ops
		c.SetSpanAttribute("test.key", "test.value")
		c.AddSpanEvent("test_event")

		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestTracingEnabled(t *testing.T) {
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

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestTracingExcludePaths(t *testing.T) {
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

	if w1.Code != http.StatusOK {
		t.Errorf("Expected status 200 for /health, got %d", w1.Code)
	}

	// Test non-excluded path
	req2 := httptest.NewRequest("GET", "/api/users", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 for /api/users, got %d", w2.Code)
	}
}

func TestTracingConfiguration(t *testing.T) {
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

	if r.tracing == nil {
		t.Fatal("Expected tracing config to be set")
	}

	if !r.tracing.enabled {
		t.Error("Expected tracing to be enabled")
	}

	if r.tracing.serviceName != "custom-service" {
		t.Errorf("Expected service name 'custom-service', got '%s'", r.tracing.serviceName)
	}

	if r.tracing.serviceVersion != "v2.0.0" {
		t.Errorf("Expected service version 'v2.0.0', got '%s'", r.tracing.serviceVersion)
	}

	if r.tracing.sampleRate != 0.5 {
		t.Errorf("Expected sample rate 0.5, got %f", r.tracing.sampleRate)
	}

	if !r.tracing.excludePaths["/health"] {
		t.Error("Expected /health to be in exclude paths")
	}

	if r.tracing.recordParams {
		t.Error("Expected params recording to be disabled")
	}

	if len(r.tracing.recordHeaders) != 1 || r.tracing.recordHeaders[0] != "Authorization" {
		t.Error("Expected Authorization header to be recorded")
	}
}

func TestTracingWithGroups(t *testing.T) {
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

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
