package logging

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMiddleware_Basic(t *testing.T) {
	th := NewTestHelper(t)
	mw := Middleware(th.Logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	wrapped := mw(handler)
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	// Should have request started and completed
	if !th.ContainsLog("request started") {
		t.Error("missing 'request started' log")
	}
	if !th.ContainsLog("request completed") {
		t.Error("missing 'request completed' log")
	}

	// Should have status code
	if !th.ContainsAttr("status", 200) {
		t.Error("missing status attribute")
	}
}

func TestMiddleware_SkipPaths(t *testing.T) {
	th := NewTestHelper(t)
	mw := Middleware(th.Logger, WithSkipPaths("/health", "/metrics"))

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)

	// Test skip path
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if th.ContainsLog("request started") {
		t.Error("/health should be skipped from logging")
	}

	// Test non-skip path
	th.Reset()
	req = httptest.NewRequest("GET", "/api", nil)
	w = httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if !th.ContainsLog("request started") {
		t.Error("/api should be logged")
	}
}

func TestMiddleware_WithHeaders(t *testing.T) {
	th := NewTestHelper(t)
	mw := Middleware(th.Logger, WithLogHeaders(true))

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("User-Agent", "Test/1.0")

	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	// Headers should be logged
	if !th.ContainsAttr("hdr.User-Agent", "Test/1.0") {
		t.Error("User-Agent header should be logged")
	}
}

func TestMiddleware_StatusLevels(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantLevel  string
	}{
		{"2xx OK", http.StatusOK, "INFO"},
		{"3xx Redirect", http.StatusMovedPermanently, "INFO"},
		{"4xx Client Error", http.StatusNotFound, "WARN"},
		{"4xx Bad Request", http.StatusBadRequest, "WARN"},
		{"5xx Server Error", http.StatusInternalServerError, "ERROR"},
		{"5xx Service Unavailable", http.StatusServiceUnavailable, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := NewTestHelper(t)
			mw := Middleware(th.Logger)

			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			wrapped := mw(handler)
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			wrapped.ServeHTTP(w, req)

			count := th.CountLevel(tt.wantLevel)
			if count == 0 {
				t.Errorf("expected at least one %s log for status %d", tt.wantLevel, tt.statusCode)
			}
		})
	}
}

func TestMiddleware_CapturesSize(t *testing.T) {
	th := NewTestHelper(t)
	mw := Middleware(th.Logger)

	responseBody := "Hello, World!"
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(responseBody))
	})

	wrapped := mw(handler)
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	expectedSize := len(responseBody)
	if !th.ContainsAttr("size", expectedSize) {
		t.Errorf("expected size=%d in logs", expectedSize)
	}
}

func TestMiddleware_CapturesDuration(t *testing.T) {
	th := NewTestHelper(t)
	mw := Middleware(th.Logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	entries, _ := th.Logs()
	found := false
	for _, entry := range entries {
		if entry.Message == "request completed" {
			if _, ok := entry.Attrs["duration"]; ok {
				found = true
				break
			}
		}
	}

	if !found {
		t.Error("expected duration attribute in completed log")
	}
}

func TestMiddleware_QueryParameters(t *testing.T) {
	th := NewTestHelper(t)
	mw := Middleware(th.Logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)
	req := httptest.NewRequest("GET", "/test?foo=bar&baz=qux", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if !th.ContainsAttr("query", "foo=bar&baz=qux") {
		t.Error("query parameters should be logged")
	}
}

func TestMiddleware_NoQueryParameters(t *testing.T) {
	th := NewTestHelper(t)
	mw := Middleware(th.Logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	entries, _ := th.Logs()
	for _, entry := range entries {
		if _, hasQuery := entry.Attrs["query"]; hasQuery {
			t.Error("query should not be present when no query params")
		}
	}
}

func TestMiddleware_MultipleRequests(t *testing.T) {
	th := NewTestHelper(t)
	mw := Middleware(th.Logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)

	// Make 5 requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)
	}

	entries, _ := th.Logs()

	// Should have 10 entries (start + complete for each request)
	if len(entries) != 10 {
		t.Errorf("expected 10 log entries, got %d", len(entries))
	}
}

func TestMiddleware_WithDifferentMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			th := NewTestHelper(t)
			mw := Middleware(th.Logger)

			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			wrapped := mw(handler)
			req := httptest.NewRequest(method, "/test", nil)
			w := httptest.NewRecorder()

			wrapped.ServeHTTP(w, req)

			if !th.ContainsAttr("method", method) {
				t.Errorf("method %s should be logged", method)
			}
		})
	}
}

// Test responseWriter
func TestResponseWriter_StatusCode(t *testing.T) {
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w}

	// Default status is 200
	if rw.StatusCode() != http.StatusOK {
		t.Errorf("expected default status 200, got %d", rw.StatusCode())
	}

	// Set status
	rw.WriteHeader(http.StatusNotFound)
	if rw.StatusCode() != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rw.StatusCode())
	}

	// Multiple WriteHeader calls should not change status
	rw.WriteHeader(http.StatusInternalServerError)
	if rw.StatusCode() != http.StatusNotFound {
		t.Errorf("status should remain 404, got %d", rw.StatusCode())
	}
}

func TestResponseWriter_Size(t *testing.T) {
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w}

	data := []byte("Hello, World!")
	n, err := rw.Write(data)

	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if n != len(data) {
		t.Errorf("expected to write %d bytes, wrote %d", len(data), n)
	}

	if rw.Size() != len(data) {
		t.Errorf("expected size %d, got %d", len(data), rw.Size())
	}
}

func TestResponseWriter_Reset(t *testing.T) {
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w}

	rw.WriteHeader(http.StatusNotFound)
	rw.Write([]byte("test"))

	// Reset
	w2 := httptest.NewRecorder()
	rw.reset(w2)

	if rw.statusCode != 0 {
		t.Errorf("expected statusCode to be 0 after reset, got %d", rw.statusCode)
	}

	if rw.size != 0 {
		t.Errorf("expected size to be 0 after reset, got %d", rw.size)
	}

	if rw.written {
		t.Error("expected written to be false after reset")
	}
}

func TestResponseWriter_WriteImplicitStatus(t *testing.T) {
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w}

	// Write without explicit WriteHeader
	rw.Write([]byte("test"))

	// Should default to 200
	if rw.StatusCode() != http.StatusOK {
		t.Errorf("expected implicit status 200, got %d", rw.StatusCode())
	}
}

// Test pool reuse
func TestMiddleware_PoolReuse(_ *testing.T) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	mw := Middleware(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)

	// Make multiple requests to test pool reuse
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)
	}

	// No specific assertions, just ensure no panics
}

// Test middleware with query string
func TestMiddleware_WithQuery(t *testing.T) {
	th := NewTestHelper(t)
	mw := Middleware(th.Logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)
	req := httptest.NewRequest("GET", "/api/users?page=1&limit=10", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if !th.ContainsAttr("path", "/api/users") {
		t.Error("expected path to be /api/users")
	}

	if !th.ContainsAttr("query", "page=1&limit=10") {
		t.Error("expected query string to be logged")
	}
}

// Test middleware captures all status codes
func TestMiddleware_AllStatusCodes(t *testing.T) {
	statusCodes := []int{
		http.StatusOK,
		http.StatusCreated,
		http.StatusMovedPermanently,
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusNotFound,
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
	}

	for _, code := range statusCodes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			th := NewTestHelper(t)
			mw := Middleware(th.Logger)

			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
			})

			wrapped := mw(handler)
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			wrapped.ServeHTTP(w, req)

			if !th.ContainsAttr("status", code) {
				t.Errorf("expected status %d to be logged", code)
			}
		})
	}
}

// Test middleware with different HTTP methods
func TestMiddleware_HTTPMethods(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
		http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			th := NewTestHelper(t)
			mw := Middleware(th.Logger)

			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			wrapped := mw(handler)
			req := httptest.NewRequest(method, "/test", nil)
			w := httptest.NewRecorder()

			wrapped.ServeHTTP(w, req)

			if !th.ContainsAttr("method", method) {
				t.Errorf("expected method %s to be logged", method)
			}
		})
	}
}

// Test pool allocation patterns
func TestPools_NoAllocation(_ *testing.T) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	mw := Middleware(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)
	req := httptest.NewRequest("GET", "/test", nil)

	// Warm up pools
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)
	}

	// This test just ensures pools work without panicking
	// Actual allocation testing is in benchmarks with -benchmem
}

// Test constants are defined correctly
func TestConstants(t *testing.T) {
	if defaultAttrCapacity != 32 {
		t.Errorf("expected defaultAttrCapacity=32, got %d", defaultAttrCapacity)
	}

	if statusOKStart != 200 {
		t.Errorf("expected statusOKStart=200, got %d", statusOKStart)
	}

	if statusErrorStart != 500 {
		t.Errorf("expected statusErrorStart=500, got %d", statusErrorStart)
	}
}

// Test context logger with trace
func TestContextLogger_WithTrace(t *testing.T) {
	th := NewTestHelper(t)

	// Create a basic context (no actual tracing for unit test)
	ctx := context.Background()

	cl := NewContextLogger(ctx, th.Logger)
	cl.Info("traced message", "key", "value")

	if !th.ContainsLog("traced message") {
		t.Error("message should be logged")
	}
}

// Test middleware user agent
func TestMiddleware_UserAgent(t *testing.T) {
	th := NewTestHelper(t)
	mw := Middleware(th.Logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "CustomAgent/1.0")

	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if !th.ContainsAttr("user_agent", "CustomAgent/1.0") {
		t.Error("user agent should be logged")
	}
}

// Test middleware remote addr
func TestMiddleware_RemoteAddr(t *testing.T) {
	th := NewTestHelper(t)
	mw := Middleware(th.Logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if !th.ContainsAttr("remote", "192.168.1.1:12345") {
		t.Error("remote address should be logged")
	}
}

// Test skip paths with multiple paths
func TestMiddleware_MultipleSkipPaths(t *testing.T) {
	th := NewTestHelper(t)
	skipPaths := []string{"/health", "/metrics", "/ready", "/alive"}
	mw := Middleware(th.Logger, WithSkipPaths(skipPaths...))

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)

	for _, path := range skipPaths {
		th.Reset()
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)

		if th.ContainsLog("request started") {
			t.Errorf("path %s should be skipped", path)
		}
	}

	// Test non-skip path
	th.Reset()
	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if !th.ContainsLog("request started") {
		t.Error("/api should be logged")
	}
}

// Test middleware with headers containing sensitive data
func TestMiddleware_HeadersWithSensitiveData(t *testing.T) {
	th := NewTestHelper(t)
	mw := Middleware(th.Logger, WithLogHeaders(true))

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("X-Api-Key", "secret-key")

	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	// Headers should be logged (redaction happens in buildReplaceAttr if keys match)
	entries, _ := th.Logs()
	found := false
	for _, entry := range entries {
		if entry.Message == "request started" {
			if _, ok := entry.Attrs["hdr.Authorization"]; ok {
				found = true
			}
		}
	}

	if !found {
		t.Error("Authorization header should be logged when WithLogHeaders is enabled")
	}
}

// Test response writer multiple writes
func TestResponseWriter_MultipleWrites(t *testing.T) {
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w}

	writes := [][]byte{
		[]byte("Hello"),
		[]byte(", "),
		[]byte("World"),
		[]byte("!"),
	}

	totalSize := 0
	for _, data := range writes {
		n, err := rw.Write(data)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		totalSize += n
	}

	if rw.Size() != totalSize {
		t.Errorf("expected total size %d, got %d", totalSize, rw.Size())
	}

	expectedBody := "Hello, World!"
	if w.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, w.Body.String())
	}
}

// Test middleware integration with context logger pool
func TestMiddleware_ContextLoggerPooling(_ *testing.T) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	mw := Middleware(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Simulate some work
		time.Sleep(1 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)

	// Concurrent requests to test pool safety
	done := make(chan struct{})
	requests := 50
	workers := 10

	for i := 0; i < workers; i++ {
		go func() {
			for j := 0; j < requests/workers; j++ {
				req := httptest.NewRequest("GET", "/test", nil)
				w := httptest.NewRecorder()
				wrapped.ServeHTTP(w, req)
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < workers; i++ {
		<-done
	}

	// Should complete without panics
}

// Test middleware with panic recovery (ensure pools are returned)
func TestMiddleware_PanicRecovery(t *testing.T) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	mw := Middleware(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		panic("test panic")
	})

	wrapped := mw(handler)
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic to propagate")
		}
	}()

	wrapped.ServeHTTP(w, req)
}

// Test middleware options combinations
func TestMiddleware_OptionCombinations(t *testing.T) {
	tests := []struct {
		name string
		opts []MiddlewareOption
	}{
		{
			name: "skip paths + headers",
			opts: []MiddlewareOption{
				WithSkipPaths("/health"),
				WithLogHeaders(true),
			},
		},
		{
			name: "skip paths only",
			opts: []MiddlewareOption{
				WithSkipPaths("/health", "/metrics"),
			},
		},
		{
			name: "all options",
			opts: []MiddlewareOption{
				WithSkipPaths("/health"),
				WithLogHeaders(true),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
			mw := Middleware(logger, tt.opts...)

			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			wrapped := mw(handler)
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			wrapped.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}
		})
	}
}

// Test that middleware preserves response body
func TestMiddleware_PreservesResponse(t *testing.T) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	mw := Middleware(logger)

	expectedBody := `{"message":"success","data":{"id":123}}`
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedBody))
	})

	wrapped := mw(handler)
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, w.Body.String())
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
}

// Test middleware with large response
func TestMiddleware_LargeResponse(t *testing.T) {
	th := NewTestHelper(t)
	mw := Middleware(th.Logger)

	// Create 1MB response
	largeBody := strings.Repeat("x", 1024*1024)
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(largeBody))
	})

	wrapped := mw(handler)
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if !th.ContainsAttr("size", 1024*1024) {
		t.Error("expected size to be 1MB")
	}

	if w.Body.Len() != 1024*1024 {
		t.Errorf("expected body length 1MB, got %d", w.Body.Len())
	}
}

// Test middleware preserves headers
func TestMiddleware_PreservesHeaders(t *testing.T) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	mw := Middleware(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Custom-Header", "custom-value")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Header().Get("X-Custom-Header") != "custom-value" {
		t.Error("custom header should be preserved")
	}

	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Error("cache-control header should be preserved")
	}
}
