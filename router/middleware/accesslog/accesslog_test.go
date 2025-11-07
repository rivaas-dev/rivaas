package accesslog

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rivaas.dev/router"
	"rivaas.dev/router/middleware/requestid"
)

// mockLogger implements the logging.Logger interface for testing
type mockLogger struct {
	errorCalls []call
	warnCalls  []call
	infoCalls  []call
	debugCalls []call
}

type call struct {
	msg    string
	fields []any
}

func (m *mockLogger) Error(msg string, args ...any) {
	m.errorCalls = append(m.errorCalls, call{msg: msg, fields: args})
}

func (m *mockLogger) Warn(msg string, args ...any) {
	m.warnCalls = append(m.warnCalls, call{msg: msg, fields: args})
}

func (m *mockLogger) Info(msg string, args ...any) {
	m.infoCalls = append(m.infoCalls, call{msg: msg, fields: args})
}

func (m *mockLogger) Debug(msg string, args ...any) {
	m.debugCalls = append(m.debugCalls, call{msg: msg, fields: args})
}

func (m *mockLogger) reset() {
	m.errorCalls = nil
	m.warnCalls = nil
	m.infoCalls = nil
	m.debugCalls = nil
}

func (m *mockLogger) getFields(calls []call) map[string]any {
	if len(calls) == 0 {
		return nil
	}
	// Convert key-value pairs to map
	fields := make(map[string]any)
	for i := 0; i < len(calls[0].fields); i += 2 {
		if i+1 < len(calls[0].fields) {
			key, ok := calls[0].fields[i].(string)
			if ok {
				fields[key] = calls[0].fields[i+1]
			}
		}
	}
	return fields
}

func TestAccessLog_BasicLogging(t *testing.T) {
	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
	r.Use(New())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if len(logger.infoCalls) != 1 {
		t.Fatalf("Expected 1 info log call, got %d", len(logger.infoCalls))
	}

	if logger.infoCalls[0].msg != "access" {
		t.Errorf("Expected log message 'access', got %s", logger.infoCalls[0].msg)
	}

	fields := logger.getFields(logger.infoCalls)
	if fields["method"] != "GET" {
		t.Errorf("Expected method 'GET', got %v", fields["method"])
	}
	if fields["path"] != "/test" {
		t.Errorf("Expected path '/test', got %v", fields["path"])
	}
	if fields["status"] != http.StatusOK {
		t.Errorf("Expected status 200, got %v", fields["status"])
	}
}

func TestAccessLog_ExcludePaths(t *testing.T) {
	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
	r.Use(New(
		WithExcludePaths("/health", "/metrics"),
	))

	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})
	r.GET("/api", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	// Request to excluded path
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if len(logger.infoCalls) > 0 || len(logger.warnCalls) > 0 || len(logger.errorCalls) > 0 {
		t.Errorf("Excluded path should not be logged, got %d info, %d warn, %d error calls",
			len(logger.infoCalls), len(logger.warnCalls), len(logger.errorCalls))
	}

	// Request to non-excluded path
	logger.reset()
	req = httptest.NewRequest(http.MethodGet, "/api", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if len(logger.infoCalls) == 0 {
		t.Error("Non-excluded path should be logged")
	}
}

func TestAccessLog_ExcludePrefixes(t *testing.T) {
	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
	r.Use(New(
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
			logger.reset()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			hasLogs := len(logger.infoCalls) > 0 || len(logger.warnCalls) > 0 || len(logger.errorCalls) > 0
			if tc.shouldLog && !hasLogs {
				t.Errorf("Path %s should be logged, but wasn't", tc.path)
			}
			if !tc.shouldLog && hasLogs {
				t.Errorf("Path %s should not be logged, but was", tc.path)
			}
		})
	}
}

func TestAccessLog_StatusCodes(t *testing.T) {
	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
	r.Use(New())

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
			logger.reset()
			r.GET("/test", func(c *router.Context) {
				c.Status(tc.statusCode)
				c.JSON(tc.statusCode, map[string]string{"status": "test"})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			var calls []call
			switch tc.expectedLevel {
			case "error":
				calls = logger.errorCalls
			case "warn":
				calls = logger.warnCalls
			case "info":
				calls = logger.infoCalls
			}

			if len(calls) != 1 {
				t.Errorf("Expected 1 %s log call, got %d", tc.expectedLevel, len(calls))
			}

			if len(calls) > 0 {
				fields := logger.getFields(calls)
				if fields["status"] != tc.statusCode {
					t.Errorf("Expected status %d, got %v", tc.statusCode, fields["status"])
				}
			}
		})
	}
}

func TestAccessLog_SlowRequest(t *testing.T) {
	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
	r.Use(New(
		WithSlowThreshold(100 * time.Millisecond),
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

	if len(logger.infoCalls) != 1 {
		t.Errorf("Fast request should log at info level, got %d calls", len(logger.infoCalls))
	}
	if len(logger.warnCalls) > 0 {
		t.Error("Fast request should not log at warn level")
	}

	// Slow request
	logger.reset()
	req = httptest.NewRequest(http.MethodGet, "/slow", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if len(logger.warnCalls) != 1 {
		t.Errorf("Slow request should log at warn level, got %d warn calls", len(logger.warnCalls))
	}

	if len(logger.warnCalls) > 0 {
		fields := logger.getFields(logger.warnCalls)
		if fields["slow"] != true {
			t.Error("Slow request should have 'slow' field set to true")
		}
	}
}

func TestAccessLog_ErrorsOnly(t *testing.T) {
	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
	r.Use(New(
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

	if len(logger.infoCalls) > 0 || len(logger.warnCalls) > 0 || len(logger.errorCalls) > 0 {
		t.Error("Success request should not be logged when errorsOnly is enabled")
	}

	// Error request should be logged
	logger.reset()
	req = httptest.NewRequest(http.MethodGet, "/error", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if len(logger.warnCalls) == 0 {
		t.Error("Error request should be logged when errorsOnly is enabled")
	}
}

func TestAccessLog_Sampling(t *testing.T) {
	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
	r.Use(New(
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
		logger.reset()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		decisions[i] = len(logger.infoCalls) > 0
	}

	// All decisions should be the same (deterministic)
	firstDecision := decisions[0]
	for i, decision := range decisions {
		if decision != firstDecision {
			t.Errorf("Sampling decision %d differs from first decision (expected deterministic)", i)
		}
	}
}

func TestAccessLog_SlowRequestBypassesSampling(t *testing.T) {
	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
	r.Use(New(
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
	if len(logger.warnCalls) == 0 {
		t.Error("Slow request should bypass sampling and be logged")
	}
}

func TestAccessLog_ErrorBypassesSampling(t *testing.T) {
	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
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
	if len(logger.warnCalls) == 0 {
		t.Error("Error request should bypass sampling and be logged")
	}
}

func TestAccessLog_RouteTemplate(t *testing.T) {
	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
	r.Use(New())

	r.GET("/users/:id", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"user_id": c.Param("id")})
	})

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if len(logger.infoCalls) != 1 {
		t.Fatalf("Expected 1 info log call, got %d", len(logger.infoCalls))
	}

	fields := logger.getFields(logger.infoCalls)
	if fields["route"] != "/users/:id" {
		t.Errorf("Expected route template '/users/:id', got %v", fields["route"])
	}
}

func TestAccessLog_ClientIP(t *testing.T) {
	logger := &mockLogger{}
	// Configure trusted proxies to test X-Forwarded-For header trust
	// 10.0.0.0/8 covers the test proxy IPs (10.0.0.1)
	r := router.New(
		router.WithLogger(logger),
		router.WithTrustedProxies(
			router.WithProxies("10.0.0.0/8", "192.168.0.0/16"),
		),
	)
	r.Use(New())

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
			logger.reset()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tc.remoteAddr != "" {
				req.RemoteAddr = tc.remoteAddr
			}
			if tc.forwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tc.forwardedFor)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			fields := logger.getFields(logger.infoCalls)
			if fields["client_ip"] != tc.expectedIP {
				t.Errorf("Expected client_ip '%s', got '%v'", tc.expectedIP, fields["client_ip"])
			}
		})
	}
}

func TestAccessLog_AllFields(t *testing.T) {
	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
	r.Use(requestid.New())
	r.Use(New())

	r.GET("/users/:id", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"user_id": c.Param("id")})
	})

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	req.Header.Set("User-Agent", "test-agent/1.0")
	req.Header.Set("X-Request-ID", "test-request-id")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if len(logger.infoCalls) != 1 {
		t.Fatalf("Expected 1 info log call, got %d", len(logger.infoCalls))
	}

	fields := logger.getFields(logger.infoCalls)
	requiredFields := []string{"method", "path", "status", "duration_ms", "bytes_sent", "user_agent", "client_ip", "host", "proto", "route"}

	for _, field := range requiredFields {
		if _, ok := fields[field]; !ok {
			t.Errorf("Expected field '%s' in log entry, but it was missing", field)
		}
	}

	// Verify some specific values
	if fields["method"] != "GET" {
		t.Errorf("Expected method 'GET', got %v", fields["method"])
	}
	if fields["path"] != "/users/123" {
		t.Errorf("Expected path '/users/123', got %v", fields["path"])
	}
	if fields["route"] != "/users/:id" {
		t.Errorf("Expected route '/users/:id', got %v", fields["route"])
	}
	if fields["user_agent"] != "test-agent/1.0" {
		t.Errorf("Expected user_agent 'test-agent/1.0', got %v", fields["user_agent"])
	}
}

func TestAccessLog_NoLogger(t *testing.T) {
	// Test that middleware works even when no logger is configured
	r := router.New() // No logger set
	r.Use(New())

	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	// Should not panic
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAccessLog_ResponseWriterInterfaces(t *testing.T) {
	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
	r.Use(New())

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

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAccessLog_BytesSent(t *testing.T) {
	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
	r.Use(New())

	responseBody := `{"message": "hello world"}`
	r.GET("/test", func(c *router.Context) {
		c.Response.WriteHeader(http.StatusOK)
		c.Response.Write([]byte(responseBody))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	fields := logger.getFields(logger.infoCalls)
	if fields["bytes_sent"] != len(responseBody) {
		t.Errorf("Expected bytes_sent %d, got %v", len(responseBody), fields["bytes_sent"])
	}
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
			if result != tt.expected && tt.name != "50% rate deterministic" {
				t.Errorf("sampleByHash(%q, %f) = %v, want %v", tt.id, tt.rate, result, tt.expected)
			}

			// Test deterministic behavior (same ID should give same result)
			if tt.id != "" && tt.rate > 0 && tt.rate < 1 {
				for i := 0; i < 10; i++ {
					if sampleByHash(tt.id, tt.rate) != result {
						t.Errorf("sampleByHash should be deterministic for same ID, iteration %d differed", i)
					}
				}
			}
		})
	}
}

// TestExtractClientIP was removed - the middleware now uses c.ClientIP() which
// has proper proxy trust checking. See router/proxies_test.go for comprehensive
// tests of the trusted proxy functionality.

func TestAccessLog_CombinedOptions(t *testing.T) {
	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
	r.Use(New(
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
			logger.reset()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			hasLogs := len(logger.infoCalls) > 0 || len(logger.warnCalls) > 0 || len(logger.errorCalls) > 0
			if tc.shouldLog && !hasLogs {
				t.Errorf("Path %s should be logged, but wasn't", tc.path)
			}
			if !tc.shouldLog && hasLogs {
				t.Errorf("Path %s should not be logged, but was", tc.path)
			}
		})
	}
}

func TestAccessLog_Duration(t *testing.T) {
	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
	r.Use(New())

	r.GET("/test", func(c *router.Context) {
		time.Sleep(50 * time.Millisecond)
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	fields := logger.getFields(logger.infoCalls)
	durationMs, ok := fields["duration_ms"].(int64)
	if !ok {
		t.Fatal("duration_ms field should be present and be int64")
	}

	// Should be approximately 50ms (allow some tolerance)
	if durationMs < 40 || durationMs > 100 {
		t.Errorf("Expected duration_ms around 50, got %d", durationMs)
	}
}

func TestAccessLog_ResponseWriterPreservation(t *testing.T) {
	// Test that responseWriter properly implements all optional interfaces
	var (
		_ http.ResponseWriter           = (*responseWriter)(nil)
		_ http.Flusher                  = (*responseWriter)(nil)
		_ http.Hijacker                 = (*responseWriter)(nil)
		_ http.Pusher                   = (*responseWriter)(nil)
		_ interface{ StatusCode() int } = (*responseWriter)(nil)
		_ interface{ Size() int }       = (*responseWriter)(nil)
	)

	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
	r.Use(New())

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

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAccessLog_StatusCodeTracking(t *testing.T) {
	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
	r.Use(New())

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
			logger.reset()
			r.GET("/test", func(c *router.Context) {
				c.Status(tc.statusCode)
				c.Response.Write([]byte("test response"))
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.statusCode {
				t.Errorf("Expected HTTP status %d, got %d", tc.statusCode, w.Code)
			}

			// Check that status was logged correctly
			var calls []call
			if tc.statusCode >= 500 {
				calls = logger.errorCalls
			} else if tc.statusCode >= 400 {
				calls = logger.warnCalls
			} else {
				calls = logger.infoCalls
			}

			if len(calls) > 0 {
				fields := logger.getFields(calls)
				if fields["status"] != tc.statusCode {
					t.Errorf("Expected logged status %d, got %v", tc.statusCode, fields["status"])
				}
			}
		})
	}
}

func TestAccessLog_RequestIDSampling(t *testing.T) {
	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
	r.Use(requestid.New())
	r.Use(New(
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
	logger.reset()
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	decision1 := len(logger.infoCalls) > 0

	logger.reset()
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	decision2 := len(logger.infoCalls) > 0

	if decision1 != decision2 {
		t.Error("Same request ID should produce same sampling decision")
	}
}

func TestAccessLog_NoRequestIDSampling(t *testing.T) {
	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
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
	if len(logger.infoCalls) == 0 {
		t.Error("Request without request ID should always log (no sampling)")
	}
}

func TestAccessLog_HostAndProto(t *testing.T) {
	logger := &mockLogger{}
	r := router.New(router.WithLogger(logger))
	r.Use(New())

	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.com:8080/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	fields := logger.getFields(logger.infoCalls)
	if fields["host"] != "example.com:8080" {
		t.Errorf("Expected host 'example.com:8080', got %v", fields["host"])
	}
	if fields["proto"] != "HTTP/1.1" {
		t.Errorf("Expected proto 'HTTP/1.1', got %v", fields["proto"])
	}
}
