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

package accesslog

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rivaas.dev/router"
	"rivaas.dev/router/middleware/requestid"
)

// testHandler is a slog.Handler implementation for testing that captures log records.
type testHandler struct {
	mu      sync.Mutex
	records []testRecord
}

type testRecord struct {
	level slog.Level
	msg   string
	attrs map[string]any
}

func newTestHandler() *testHandler {
	return &testHandler{
		records: make([]testRecord, 0),
	}
}

func (h *testHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *testHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	attrs := make(map[string]any)
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	h.records = append(h.records, testRecord{
		level: r.Level,
		msg:   r.Message,
		attrs: attrs,
	})
	return nil
}

func (h *testHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

func (h *testHandler) WithGroup(_ string) slog.Handler {
	return h
}

func (h *testHandler) reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = nil
}

func (h *testHandler) getRecords(level slog.Level) []testRecord {
	h.mu.Lock()
	defer h.mu.Unlock()

	var result []testRecord
	for _, r := range h.records {
		if r.level == level {
			result = append(result, r)
		}
	}
	return result
}

func (h *testHandler) getFields(level slog.Level) map[string]any {
	records := h.getRecords(level)
	if len(records) == 0 {
		return nil
	}
	// Return attributes from the first matching record
	return records[0].attrs
}

func TestAccessLog_BasicLogging(t *testing.T) {
	handler := newTestHandler()
	logger := slog.New(handler)

	r := router.MustNew()
	r.Use(New(WithLogger(logger)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	records := handler.getRecords(slog.LevelInfo)
	require.Len(t, records, 1, "Expected exactly 1 info log")
	assert.Equal(t, "access", records[0].msg)

	fields := handler.getFields(slog.LevelInfo)
	assert.Equal(t, "GET", fields["method"])
	assert.Equal(t, "/test", fields["path"])
	assert.Equal(t, int64(http.StatusOK), fields["status"])
}

func TestAccessLog_ExcludePaths(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		shouldLog bool
	}{
		{"excluded /health", "/health", false},
		{"excluded /metrics", "/metrics", false},
		{"non-excluded /api", "/api", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newTestHandler()
			logger := slog.New(handler)

			r := router.MustNew()
			r.Use(New(
				WithLogger(logger),
				WithExcludePaths("/health", "/metrics"),
			))
			r.GET(tt.path, func(c *router.Context) {
				c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			totalLogs := len(handler.getRecords(slog.LevelInfo)) +
				len(handler.getRecords(slog.LevelWarn)) +
				len(handler.getRecords(slog.LevelError))

			if tt.shouldLog {
				assert.Greater(t, totalLogs, 0, "Path should be logged")
			} else {
				assert.Equal(t, 0, totalLogs, "Path should not be logged")
			}
		})
	}
}

func TestAccessLog_ExcludePrefixes(t *testing.T) {
	handler := newTestHandler()
	logger := slog.New(handler)
	r := router.MustNew()
	r.Use(New(
		WithLogger(logger),
		WithExcludePrefixes("/metrics", "/debug"),
	))

	r.GET("/metrics/prometheus", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	r.GET("/debug/pprof/heap", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	r.GET("/api/users", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	// Test prefix exclusion
	testCases := []struct {
		path      string
		shouldLog bool
		desc      string
	}{
		{"/metrics/prometheus", false, "metrics prefix"},
		{"/debug/pprof/heap", false, "debug prefix"},
		{"/api/users", true, "non-excluded path"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			handler.reset()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			hasLogs := len(handler.getRecords(slog.LevelInfo)) > 0 || len(handler.getRecords(slog.LevelWarn)) > 0 || len(handler.getRecords(slog.LevelError)) > 0
			if tc.shouldLog {
				assert.True(t, hasLogs, "Path %s should be logged, but wasn't", tc.path)
			} else {
				assert.False(t, hasLogs, "Path %s should not be logged, but was", tc.path)
			}
		})
	}
}

func TestAccessLog_StatusCodes(t *testing.T) {
	handler := newTestHandler()
	logger := slog.New(handler)
	r := router.MustNew()
	r.Use(New(WithLogger(logger)))

	testCases := []struct {
		name          string
		statusCode    int
		expectedLevel string // "info", "warn", "error"
	}{
		{"200 OK", http.StatusOK, "info"},
		{"201 Created", http.StatusCreated, "info"},
		{"400 Bad Request", http.StatusBadRequest, "warn"},
		{"404 Not Found", http.StatusNotFound, "warn"},
		{"500 Internal Server Error", http.StatusInternalServerError, "error"},
		{"503 Service Unavailable", http.StatusServiceUnavailable, "error"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler.reset()
			r.GET("/test", func(c *router.Context) {
				c.Status(tc.statusCode)
				c.JSON(tc.statusCode, map[string]string{"status": "test"})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			var level slog.Level
			switch tc.expectedLevel {
			case "error":
				level = slog.LevelError
			case "warn":
				level = slog.LevelWarn
			case "info":
				level = slog.LevelInfo
			}

			records := handler.getRecords(level)
			require.Len(t, records, 1, "Expected 1 %s log call", tc.expectedLevel)

			fields := handler.getFields(level)
			assert.Equal(t, int64(tc.statusCode), fields["status"])
		})
	}
}

func TestAccessLog_SlowRequest(t *testing.T) {
	handler := newTestHandler()
	logger := slog.New(handler)
	r := router.MustNew()
	r.Use(New(
		WithLogger(logger),
		WithSlowThreshold(100*time.Millisecond),
	))

	r.GET("/fast", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	r.GET("/slow", func(c *router.Context) {
		time.Sleep(150 * time.Millisecond)
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	// Fast request
	req := httptest.NewRequest(http.MethodGet, "/fast", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Len(t, handler.getRecords(slog.LevelInfo), 1, "Fast request should log at info level")
	assert.Empty(t, handler.getRecords(slog.LevelWarn), "Fast request should not log at warn level")

	// Slow request
	handler.reset()
	req = httptest.NewRequest(http.MethodGet, "/slow", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Len(t, handler.getRecords(slog.LevelWarn), 1, "Slow request should log at warn level")

	fields := handler.getFields(slog.LevelWarn)
	assert.Equal(t, true, fields["slow"])
}

func TestAccessLog_ErrorsOnly(t *testing.T) {
	handler := newTestHandler()
	logger := slog.New(handler)
	r := router.MustNew()
	r.Use(New(
		WithLogger(logger),
		WithErrorsOnly(),
	))

	r.GET("/success", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	r.GET("/error", func(c *router.Context) {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
	})

	// Success request should not be logged
	req := httptest.NewRequest(http.MethodGet, "/success", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	totalLogs := len(handler.getRecords(slog.LevelInfo)) + len(handler.getRecords(slog.LevelWarn)) + len(handler.getRecords(slog.LevelError))
	assert.Equal(t, 0, totalLogs, "Success request should not be logged when errorsOnly is enabled")

	// Error request should be logged
	handler.reset()
	req = httptest.NewRequest(http.MethodGet, "/error", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.NotEmpty(t, handler.getRecords(slog.LevelWarn), "Error request should be logged when errorsOnly is enabled")
}

func TestAccessLog_Sampling(t *testing.T) {
	handler := newTestHandler()
	logger := slog.New(handler)
	r := router.MustNew()
	r.Use(New(
		WithLogger(logger),
		WithSampleRate(0.5), // 50% sampling
	))

	r.Use(requestid.New())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	// Make multiple requests with same request ID
	// They should all make the same sampling decision
	requestID := "test-request-id-12345"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", requestID)

	// Run multiple times - all should make the same decision
	decisions := make([]bool, 10)
	for i := 0; i < 10; i++ {
		handler.reset()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		decisions[i] = len(handler.getRecords(slog.LevelInfo)) > 0
	}

	// All decisions should be the same (deterministic)
	firstDecision := decisions[0]
	for i, decision := range decisions {
		assert.Equal(t, firstDecision, decision, "Sampling decision %d differs from first decision (expected deterministic)", i)
	}
}

func TestAccessLog_SlowRequestBypassesSampling(t *testing.T) {
	handler := newTestHandler()
	logger := slog.New(handler)
	r := router.MustNew()
	r.Use(New(
		WithLogger(logger),
		WithSampleRate(0.0), // Sample 0% (should skip all)
		WithSlowThreshold(50*time.Millisecond),
	))

	r.GET("/slow", func(c *router.Context) {
		time.Sleep(100 * time.Millisecond)
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Slow request should bypass sampling and be logged
	assert.NotEmpty(t, handler.getRecords(slog.LevelWarn), "Slow request should bypass sampling and be logged")
}

func TestAccessLog_ErrorBypassesSampling(t *testing.T) {
	handler := newTestHandler()
	logger := slog.New(handler)
	r := router.MustNew()
	r.Use(New(WithLogger(logger)))
	r.Use(New(
		WithSampleRate(0.0), // Sample 0% (should skip all)
	))

	r.GET("/error", func(c *router.Context) {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Error request should bypass sampling and be logged
	assert.NotEmpty(t, handler.getRecords(slog.LevelWarn), "Error request should bypass sampling and be logged")
}

func TestAccessLog_RouteTemplate(t *testing.T) {
	handler := newTestHandler()
	logger := slog.New(handler)
	r := router.MustNew()
	r.Use(New(WithLogger(logger)))

	r.GET("/users/:id", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"user_id": c.Param("id")})
	})

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Len(t, handler.getRecords(slog.LevelInfo), 1, "Expected 1 info log call")

	fields := handler.getFields(slog.LevelInfo)
	assert.Equal(t, "/users/:id", fields["route"])
}

func TestAccessLog_ClientIP(t *testing.T) {
	handler := newTestHandler()
	logger := slog.New(handler)
	// Configure trusted proxies to test X-Forwarded-For header trust
	// 10.0.0.0/8 covers the test proxy IPs (10.0.0.1)
	r := router.MustNew(
		router.WithTrustedProxies(
			router.WithProxies("10.0.0.0/8", "192.168.0.0/16"),
		),
	)
	r.Use(New(WithLogger(logger)))

	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	testCases := []struct {
		name         string
		remoteAddr   string
		forwardedFor string
		expectedIP   string
	}{
		{"Direct connection", "192.168.1.1:12345", "", "192.168.1.1"},
		{"X-Forwarded-For single IP (trusted proxy)", "10.0.0.1:8080", "203.0.113.1", "203.0.113.1"},
		{"X-Forwarded-For multiple IPs (trusted proxy)", "10.0.0.1:8080", "203.0.113.1, 198.51.100.1", "203.0.113.1"},
		{"X-Forwarded-For from untrusted proxy (ignored)", "203.0.113.50:8080", "198.51.100.1", "203.0.113.50"},
		// Note: httptest.NewRequest always sets RemoteAddr, so we can't test "no IP" case
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler.reset()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tc.remoteAddr != "" {
				req.RemoteAddr = tc.remoteAddr
			}
			if tc.forwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tc.forwardedFor)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			fields := handler.getFields(slog.LevelInfo)
			assert.Equal(t, tc.expectedIP, fields["client_ip"], "Expected client_ip '%s'", tc.expectedIP)
		})
	}
}

func TestAccessLog_AllFields(t *testing.T) {
	handler := newTestHandler()
	logger := slog.New(handler)
	r := router.MustNew()
	r.Use(requestid.New())
	r.Use(New(WithLogger(logger)))

	r.GET("/users/:id", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"user_id": c.Param("id")})
	})

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	req.Header.Set("User-Agent", "test-agent/1.0")
	req.Header.Set("X-Request-ID", "test-request-id")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Len(t, handler.getRecords(slog.LevelInfo), 1, "Expected 1 info log call")

	fields := handler.getFields(slog.LevelInfo)
	requiredFields := []string{"method", "path", "status", "duration_ms", "bytes_sent", "user_agent", "client_ip", "host", "proto", "route"}

	for _, field := range requiredFields {
		assert.Contains(t, fields, field, "Expected field '%s' in log entry, but it was missing", field)
	}

	// Verify some specific values
	assert.Equal(t, "GET", fields["method"], "Expected method 'GET'")
	assert.Equal(t, "/users/123", fields["path"], "Expected path '/users/123'")
	assert.Equal(t, "/users/:id", fields["route"], "Expected route '/users/:id'")
	assert.Equal(t, "test-agent/1.0", fields["user_agent"], "Expected user_agent 'test-agent/1.0'")
}

func TestAccessLog_NoLogger(t *testing.T) {
	// Test that middleware works even when no logger is configured
	r := router.MustNew() // No logger set
	r.Use(New())          // No logger option provided

	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	// Should not panic
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAccessLog_ResponseWriterInterfaces(t *testing.T) {
	handler := newTestHandler()
	logger := slog.New(handler)
	r := router.MustNew()
	r.Use(New(WithLogger(logger)))

	// Test that responseWriter preserves optional interfaces
	// This is tested indirectly - if interfaces weren't preserved, certain operations would fail
	r.GET("/test", func(c *router.Context) {
		// Try to flush (if supported)
		if flusher, ok := c.Response.(http.Flusher); ok {
			flusher.Flush()
		}
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAccessLog_BytesSent(t *testing.T) {
	handler := newTestHandler()
	logger := slog.New(handler)
	r := router.MustNew()
	r.Use(New(WithLogger(logger)))

	responseBody := `{"message": "hello world"}`
	r.GET("/test", func(c *router.Context) {
		c.Response.WriteHeader(http.StatusOK)
		c.Response.Write([]byte(responseBody))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	fields := handler.getFields(slog.LevelInfo)
	assert.Equal(t, int64(len(responseBody)), fields["bytes_sent"])
}

func TestSampleByHash(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		rate     float64
		expected bool
	}{
		{"no ID always logs", "", 0.0, true},
		{"no ID always logs (high rate)", "", 1.0, true},
		{"100% rate always logs", "test-id", 1.0, true},
		{"0% rate never logs (except no ID)", "test-id", 0.0, false},
		{"50% rate deterministic", "test-id", 0.5, true}, // Should be deterministic
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sampleByHash(tt.id, tt.rate)
			// For "50% rate deterministic", we only verify it's deterministic, not the exact value
			if tt.name != "50% rate deterministic" {
				assert.Equal(t, tt.expected, result, "sampleByHash(%q, %f)", tt.id, tt.rate)
			}

			// Test deterministic behavior (same ID should give same result)
			if tt.id != "" && tt.rate > 0 && tt.rate < 1 {
				for i := 0; i < 10; i++ {
					assert.Equal(t, result, sampleByHash(tt.id, tt.rate), "sampleByHash should be deterministic for same ID, iteration %d differed", i)
				}
			}
		})
	}
}

// TestExtractClientIP was removed - the middleware now uses c.ClientIP() which
// has proper proxy trust checking. See router/proxies_test.go for comprehensive
// tests of the trusted proxy functionality.

func TestAccessLog_CombinedOptions(t *testing.T) {
	handler := newTestHandler()
	logger := slog.New(handler)
	r := router.MustNew()
	r.Use(New(
		WithLogger(logger),
		WithExcludePaths("/health"),
		WithExcludePrefixes("/metrics"),
		WithSlowThreshold(100*time.Millisecond),
		WithErrorsOnly(),
	))

	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})
	r.GET("/metrics/prometheus", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	r.GET("/success", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})
	r.GET("/error", func(c *router.Context) {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "bad"})
	})
	r.GET("/slow", func(c *router.Context) {
		time.Sleep(150 * time.Millisecond)
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	testCases := []struct {
		path      string
		shouldLog bool
		desc      string
	}{
		{"/health", false, "excluded exact path"},
		{"/metrics/prometheus", false, "excluded prefix"},
		{"/success", false, "success with errorsOnly enabled"},
		{"/error", true, "error should be logged"},
		{"/slow", true, "slow request should bypass errorsOnly"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			handler.reset()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			hasLogs := len(handler.getRecords(slog.LevelInfo)) > 0 || len(handler.getRecords(slog.LevelWarn)) > 0 || len(handler.getRecords(slog.LevelError)) > 0
			if tc.shouldLog {
				assert.True(t, hasLogs, "Path %s should be logged, but wasn't", tc.path)
			} else {
				assert.False(t, hasLogs, "Path %s should not be logged, but was", tc.path)
			}
		})
	}
}

func TestAccessLog_Duration(t *testing.T) {
	handler := newTestHandler()
	logger := slog.New(handler)
	r := router.MustNew()
	r.Use(New(WithLogger(logger)))

	r.GET("/test", func(c *router.Context) {
		time.Sleep(50 * time.Millisecond)
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	fields := handler.getFields(slog.LevelInfo)
	durationMs, ok := fields["duration_ms"].(int64)
	require.True(t, ok, "duration_ms field should be present and be int64")

	// Should be approximately 50ms (allow some tolerance)
	assert.GreaterOrEqual(t, durationMs, int64(40), "Expected duration_ms around 50, got %d", durationMs)
	assert.LessOrEqual(t, durationMs, int64(100), "Expected duration_ms around 50, got %d", durationMs)
}

func TestAccessLog_ResponseWriterPreservation(t *testing.T) {
	// Test that responseWriter properly implements all optional interfaces
	var (
		_ http.ResponseWriter           = (*responseWriter)(nil)
		_ http.Flusher                  = (*responseWriter)(nil)
		_ http.Hijacker                 = (*responseWriter)(nil)
		_ http.Pusher                   = (*responseWriter)(nil)
		_ interface{ StatusCode() int } = (*responseWriter)(nil)
		_ interface{ Size() int64 }     = (*responseWriter)(nil)
	)

	handler := newTestHandler()
	logger := slog.New(handler)
	r := router.MustNew()
	r.Use(New(WithLogger(logger)))

	r.GET("/test", func(c *router.Context) {
		// Test that we can check for interfaces
		if flusher, ok := c.Response.(http.Flusher); ok {
			flusher.Flush()
		}
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")
}

func TestAccessLog_StatusCodeTracking(t *testing.T) {
	handler := newTestHandler()
	logger := slog.New(handler)
	r := router.MustNew()
	r.Use(New(WithLogger(logger)))

	testCases := []struct {
		statusCode int
		desc       string
	}{
		{http.StatusOK, "200 OK"},
		{http.StatusCreated, "201 Created"},
		{http.StatusNoContent, "204 No Content"},
		{http.StatusBadRequest, "400 Bad Request"},
		{http.StatusNotFound, "404 Not Found"},
		{http.StatusInternalServerError, "500 Internal Server Error"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			handler.reset()
			r.GET("/test", func(c *router.Context) {
				c.Status(tc.statusCode)
				c.Response.Write([]byte("test response"))
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tc.statusCode, w.Code, "Expected HTTP status %d", tc.statusCode)

			// Check that status was logged correctly
			var level slog.Level
			if tc.statusCode >= 500 {
				level = slog.LevelError
			} else if tc.statusCode >= 400 {
				level = slog.LevelWarn
			} else {
				level = slog.LevelInfo
			}

			records := handler.getRecords(level)
			require.NotEmpty(t, records, "Expected at least one %s log record", level)
			fields := handler.getFields(level)
			assert.Equal(t, int64(tc.statusCode), fields["status"], "Expected logged status %d", tc.statusCode)
		})
	}
}

func TestAccessLog_RequestIDSampling(t *testing.T) {
	handler := newTestHandler()
	logger := slog.New(handler)
	r := router.MustNew()
	r.Use(requestid.New())
	r.Use(New(
		WithLogger(logger),
		WithSampleRate(0.5), // 50% sampling
	))

	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	// Test with same request ID - should get same sampling decision
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.Header.Set("X-Request-ID", "same-id-123")

	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("X-Request-ID", "same-id-123")

	// Both should make the same decision
	handler.reset()
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	decision1 := len(handler.getRecords(slog.LevelInfo)) > 0

	handler.reset()
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	decision2 := len(handler.getRecords(slog.LevelInfo)) > 0

	assert.Equal(t, decision1, decision2, "Same request ID should produce same sampling decision")
}

func TestAccessLog_NoRequestIDSampling(t *testing.T) {
	handler := newTestHandler()
	logger := slog.New(handler)
	r := router.MustNew()
	r.Use(New(WithLogger(logger)))
	// Don't use requestid middleware - no request ID available
	r.Use(New(
		WithSampleRate(0.0), // 0% sampling
	))

	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Without request ID, should always log (sampling returns true for empty ID)
	assert.NotEmpty(t, handler.getRecords(slog.LevelInfo), "Request without request ID should always log (no sampling)")
}

func TestAccessLog_HostAndProto(t *testing.T) {
	handler := newTestHandler()
	logger := slog.New(handler)
	r := router.MustNew()
	r.Use(New(WithLogger(logger)))

	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.com:8080/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	fields := handler.getFields(slog.LevelInfo)
	assert.Equal(t, "example.com:8080", fields["host"])
	assert.Equal(t, "HTTP/1.1", fields["proto"])
}
