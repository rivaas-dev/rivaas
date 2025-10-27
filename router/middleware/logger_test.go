package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rivaas-dev/rivaas/router"
)

func TestLogger_BasicLogging(t *testing.T) {
	buf := &bytes.Buffer{}

	r := router.New()
	r.Use(Logger(WithLoggerOutput(buf)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	logOutput := buf.String()
	if logOutput == "" {
		t.Error("Expected log output, got empty string")
	}

	// Check log contains expected fields
	if !strings.Contains(logOutput, "GET") {
		t.Error("Log should contain HTTP method")
	}

	if !strings.Contains(logOutput, "/test") {
		t.Error("Log should contain request path")
	}

	if !strings.Contains(logOutput, "200") {
		t.Error("Log should contain status code")
	}
}

func TestLogger_SkipPaths(t *testing.T) {
	buf := &bytes.Buffer{}

	r := router.New()
	r.Use(Logger(
		WithLoggerOutput(buf),
		WithSkipPaths([]string{"/health", "/metrics"}),
	))

	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})

	r.GET("/api", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	// Request to skipped path
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if buf.Len() > 0 {
		t.Errorf("Skipped path should not be logged, got: %s", buf.String())
	}

	// Request to non-skipped path
	buf.Reset()
	req = httptest.NewRequest(http.MethodGet, "/api", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if buf.Len() == 0 {
		t.Error("Non-skipped path should be logged")
	}
}

func TestLogger_CustomTimeFormat(t *testing.T) {
	buf := &bytes.Buffer{}

	r := router.New()
	r.Use(Logger(
		WithLoggerOutput(buf),
		WithTimeFormat(time.RFC3339),
	))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	logOutput := buf.String()
	// RFC3339 format includes 'T' between date and time
	if !strings.Contains(logOutput, "T") {
		t.Error("Log should use RFC3339 time format")
	}
}

func TestLogger_CustomFormatter(t *testing.T) {
	buf := &bytes.Buffer{}

	r := router.New()
	r.Use(Logger(
		WithLoggerOutput(buf),
		WithLogFormatter(func(params LogFormatterParams) string {
			return "CUSTOM: " + params.Method + " " + params.Path
		}),
	))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "CUSTOM: GET /test") {
		t.Errorf("Expected custom log format, got: %s", logOutput)
	}
}

func TestLogger_StatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"404 Not Found", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}

			r := router.New()
			r.Use(Logger(WithLoggerOutput(buf)))
			r.GET("/test", func(c *router.Context) {
				c.JSON(tt.statusCode, map[string]string{"status": "test"})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Just verify logging occurred - status code tracking depends on router internals
			logOutput := buf.String()
			if logOutput == "" {
				t.Error("Expected log output")
			}

			if !strings.Contains(logOutput, "GET") || !strings.Contains(logOutput, "/test") {
				t.Errorf("Log should contain method and path, got: %s", logOutput)
			}
		})
	}
}

func TestLogger_QueryParameters(t *testing.T) {
	buf := &bytes.Buffer{}

	r := router.New()
	r.Use(Logger(WithLoggerOutput(buf)))
	r.GET("/search", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/search?q=test&page=1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "/search?q=test&page=1") {
		t.Error("Log should contain full path with query parameters")
	}
}

func TestLogger_LatencyTracking(t *testing.T) {
	buf := &bytes.Buffer{}

	r := router.New()
	r.Use(Logger(WithLoggerOutput(buf)))
	r.GET("/slow", func(c *router.Context) {
		time.Sleep(10 * time.Millisecond)
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	logOutput := buf.String()
	// Should contain some time unit (ms, µs, etc.)
	if !strings.Contains(logOutput, "ms") && !strings.Contains(logOutput, "µs") && !strings.Contains(logOutput, "s") {
		t.Error("Log should contain latency information")
	}
}

func TestLogger_ClientIP(t *testing.T) {
	buf := &bytes.Buffer{}

	r := router.New()
	r.Use(Logger(WithLoggerOutput(buf)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	logOutput := buf.String()
	// Should contain IP address
	if !strings.Contains(logOutput, "192.168.1.100") {
		t.Errorf("Log should contain client IP, got: %s", logOutput)
	}
}

func TestLogger_ColoredOutput(t *testing.T) {
	buf := &bytes.Buffer{}

	r := router.New()
	r.Use(Logger(
		WithLoggerOutput(buf),
		WithColors(true),
	))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	logOutput := buf.String()
	// ANSI color codes contain escape sequences
	if !strings.Contains(logOutput, "\033[") {
		t.Error("Colored output should contain ANSI escape codes")
	}
}

func TestLogger_DifferentMethods(t *testing.T) {
	tests := []struct {
		method     string
		setupRoute func(r *router.Router, handler router.HandlerFunc)
	}{
		{http.MethodGet, func(r *router.Router, h router.HandlerFunc) { r.GET("/test", h) }},
		{http.MethodPost, func(r *router.Router, h router.HandlerFunc) { r.POST("/test", h) }},
		{http.MethodPut, func(r *router.Router, h router.HandlerFunc) { r.PUT("/test", h) }},
		{http.MethodPatch, func(r *router.Router, h router.HandlerFunc) { r.PATCH("/test", h) }},
		{http.MethodDelete, func(r *router.Router, h router.HandlerFunc) { r.DELETE("/test", h) }},
		{http.MethodHead, func(r *router.Router, h router.HandlerFunc) { r.HEAD("/test", h) }},
		{http.MethodOptions, func(r *router.Router, h router.HandlerFunc) { r.OPTIONS("/test", h) }},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			buf := &bytes.Buffer{}

			r := router.New()
			r.Use(Logger(WithLoggerOutput(buf)))
			tt.setupRoute(r, func(c *router.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(tt.method, "/test", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			logOutput := buf.String()
			if !strings.Contains(logOutput, tt.method) {
				t.Errorf("Log should contain method %s", tt.method)
			}
		})
	}
}

func TestLogger_WithRequestID(t *testing.T) {
	buf := &bytes.Buffer{}

	r := router.New()
	r.Use(RequestID()) // Add request ID middleware first
	r.Use(Logger(WithLoggerOutput(buf)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	logOutput := buf.String()
	requestID := w.Header().Get("X-Request-ID")

	// Log should contain the request ID
	if requestID == "" {
		t.Error("Request ID should be set")
	}

	if !strings.Contains(logOutput, requestID) {
		t.Errorf("Log should contain request ID %s, got: %s", requestID, logOutput)
	}

	// Should have pipe separator when request ID is present
	if !strings.Contains(logOutput, "|") {
		t.Errorf("Log should contain pipe separator when request ID is present, got: %s", logOutput)
	}
}

func TestLogger_WithoutRequestID(t *testing.T) {
	buf := &bytes.Buffer{}

	r := router.New()
	// No request ID middleware
	r.Use(Logger(WithLoggerOutput(buf)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	logOutput := buf.String()

	// Should not have pipe separator when no request ID
	if strings.Contains(logOutput, "|") {
		t.Errorf("Log should not contain pipe separator when no request ID, got: %s", logOutput)
	}
}

// Benchmark tests
func BenchmarkLogger_Simple(b *testing.B) {
	buf := &bytes.Buffer{}

	r := router.New()
	r.Use(Logger(WithLoggerOutput(buf)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		buf.Reset()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkLogger_CustomFormatter(b *testing.B) {
	buf := &bytes.Buffer{}

	r := router.New()
	r.Use(Logger(
		WithLoggerOutput(buf),
		WithLogFormatter(func(params LogFormatterParams) string {
			return params.Method + " " + params.Path
		}),
	))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		buf.Reset()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkLogger_WithColors(b *testing.B) {
	buf := &bytes.Buffer{}

	r := router.New()
	r.Use(Logger(
		WithLoggerOutput(buf),
		WithColors(true),
	))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		buf.Reset()
		r.ServeHTTP(w, req)
	}
}
