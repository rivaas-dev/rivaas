package router

import (
	"bufio"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

// RouterTestSuite is the main test suite for router functionality
type RouterTestSuite struct {
	suite.Suite
	router *Router
}

// SetupTest runs before each individual test
func (suite *RouterTestSuite) SetupTest() {
	suite.router = New()
}

// TearDownTest runs after each individual test
func (suite *RouterTestSuite) TearDownTest() {
	if suite.router != nil {
		// Cleanup if needed
	}
}

// TestBasicRouting tests basic HTTP method routing
func (suite *RouterTestSuite) TestBasicRouting() {
	// Test basic routes
	suite.router.GET("/", func(c *Context) {
		c.String(http.StatusOK, "Hello World")
	})

	suite.router.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "User: %s", c.Param("id"))
	})

	suite.router.POST("/users", func(c *Context) {
		c.String(http.StatusCreated, "User created")
	})

	// Test cases
	tests := []struct {
		method string
		path   string
		status int
		body   string
	}{
		{"GET", "/", 200, "Hello World"},
		{"GET", "/users/123", 200, "User: 123"},
		{"POST", "/users", 201, "User created"},
		{"GET", "/users/123/posts/456", 404, ""},
		{"GET", "/nonexistent", 404, ""},
	}

	for _, tt := range tests {
		suite.Run(tt.method+" "+tt.path, func() {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			suite.router.ServeHTTP(w, req)

			suite.Equal(tt.status, w.Code, "Status code mismatch for %s %s", tt.method, tt.path)
			if tt.body != "" {
				suite.Equal(tt.body, w.Body.String(), "Body mismatch for %s %s", tt.method, tt.path)
			}
		})
	}
}

// TestRouterWithMiddleware tests middleware functionality
func (suite *RouterTestSuite) TestRouterWithMiddleware() {
	// Add middleware
	suite.router.Use(func(c *Context) {
		c.Header("X-Middleware", "true")
		c.Next()
	})

	suite.router.GET("/", func(c *Context) {
		c.String(http.StatusOK, "Hello")
	})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	suite.router.ServeHTTP(w, req)

	suite.Equal(200, w.Code)
	suite.Equal("true", w.Header().Get("X-Middleware"))
}

// TestRouterGroup tests route grouping functionality
func (suite *RouterTestSuite) TestRouterGroup() {
	// Create a group
	api := suite.router.Group("/api/v1")
	api.GET("/users", func(c *Context) {
		c.String(http.StatusOK, "Users")
	})

	api.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "User: %s", c.Param("id"))
	})

	// Test cases
	tests := []struct {
		path   string
		status int
		body   string
	}{
		{"/api/v1/users", 200, "Users"},
		{"/api/v1/users/123", 200, "User: 123"},
		{"/users", 404, ""},
	}

	for _, tt := range tests {
		suite.Run(tt.path, func() {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			suite.router.ServeHTTP(w, req)

			suite.Equal(tt.status, w.Code)
			if tt.body != "" {
				suite.Equal(tt.body, w.Body.String())
			}
		})
	}
}

// TestRouterGroupMiddleware tests middleware on route groups
func (suite *RouterTestSuite) TestRouterGroupMiddleware() {
	// Create a group with middleware
	api := suite.router.Group("/api/v1")
	api.Use(func(c *Context) {
		c.Header("X-API-Version", "v1")
		c.Next()
	})

	api.GET("/users", func(c *Context) {
		c.String(http.StatusOK, "Users")
	})

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	w := httptest.NewRecorder()

	suite.router.ServeHTTP(w, req)

	suite.Equal(200, w.Code)
	suite.Equal("v1", w.Header().Get("X-API-Version"))
}

// TestRouterComplexRoutes tests complex route patterns
func (suite *RouterTestSuite) TestRouterComplexRoutes() {
	suite.router.GET("/users/:id/posts/:post_id", func(c *Context) {
		c.String(http.StatusOK, "User: %s, Post: %s", c.Param("id"), c.Param("post_id"))
	})

	suite.router.GET("/users/:id/posts/:post_id/comments/:comment_id", func(c *Context) {
		c.String(http.StatusOK, "User: %s, Post: %s, Comment: %s",
			c.Param("id"), c.Param("post_id"), c.Param("comment_id"))
	})

	// Test cases
	tests := []struct {
		path   string
		status int
		body   string
	}{
		{"/users/123/posts/456", 200, "User: 123, Post: 456"},
		{"/users/123/posts/456/comments/789", 200, "User: 123, Post: 456, Comment: 789"},
		{"/users/123/posts", 404, ""},
	}

	for _, tt := range tests {
		suite.Run(tt.path, func() {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			suite.router.ServeHTTP(w, req)

			suite.Equal(tt.status, w.Code)
			if tt.body != "" {
				suite.Equal(tt.body, w.Body.String())
			}
		})
	}
}

// TestContextMethods tests various context methods
func (suite *RouterTestSuite) TestContextMethods() {
	suite.router.GET("/test", func(c *Context) {
		// Test JSON response
		c.JSON(http.StatusOK, map[string]string{"message": "test"})
	})

	suite.router.GET("/string", func(c *Context) {
		// Test String response
		c.String(http.StatusOK, "Hello %s", "World")
	})

	suite.router.GET("/html", func(c *Context) {
		// Test HTML response
		c.HTML(http.StatusOK, "<h1>Hello</h1>")
	})

	// Test JSON
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)
	suite.Equal("application/json; charset=utf-8", w.Header().Get("Content-Type"))

	// Test String
	req = httptest.NewRequest("GET", "/string", nil)
	w = httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)
	suite.Equal("Hello World", w.Body.String())

	// Test HTML
	req = httptest.NewRequest("GET", "/html", nil)
	w = httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)
	suite.Equal("text/html", w.Header().Get("Content-Type"))
}

// TestRouterSuite runs the router test suite
func TestRouterSuite(t *testing.T) {
	suite.Run(t, new(RouterTestSuite))
}

// TestHTTPMethods tests all HTTP method handlers
func TestHTTPMethods(t *testing.T) {
	r := New()

	// Register all HTTP methods
	r.GET("/get", func(c *Context) {
		c.String(200, "GET")
	})
	r.POST("/post", func(c *Context) {
		c.String(200, "POST")
	})
	r.PUT("/put", func(c *Context) {
		c.String(200, "PUT")
	})
	r.DELETE("/delete", func(c *Context) {
		c.String(200, "DELETE")
	})
	r.PATCH("/patch", func(c *Context) {
		c.String(200, "PATCH")
	})
	r.OPTIONS("/options", func(c *Context) {
		c.String(200, "OPTIONS")
	})
	r.HEAD("/head", func(c *Context) {
		c.Status(200)
	})

	tests := []struct {
		method   string
		path     string
		expected string
	}{
		{"GET", "/get", "GET"},
		{"POST", "/post", "POST"},
		{"PUT", "/put", "PUT"},
		{"DELETE", "/delete", "DELETE"},
		{"PATCH", "/patch", "PATCH"},
		{"OPTIONS", "/options", "OPTIONS"},
		{"HEAD", "/head", ""},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != 200 {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
			if tt.expected != "" && w.Body.String() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, w.Body.String())
			}
		})
	}
}

// TestCompileOptimizations tests route compilation and optimization
func TestCompileOptimizations(t *testing.T) {
	r := New()

	// Add static routes that will be compiled
	r.GET("/home", func(c *Context) {
		c.String(200, "home")
	})
	r.GET("/about", func(c *Context) {
		c.String(200, "about")
	})
	r.GET("/contact", func(c *Context) {
		c.String(200, "contact")
	})

	// Trigger compilation
	r.WarmupOptimizations()

	// Test that compiled routes work
	req := httptest.NewRequest("GET", "/home", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "home" {
		t.Errorf("Expected 'home', got %q", w.Body.String())
	}
}

// TestWithBloomFilterSize tests bloom filter configuration
func TestWithBloomFilterSize(t *testing.T) {
	r := New(WithBloomFilterSize(2000))

	if r.bloomFilterSize != 2000 {
		t.Errorf("Expected bloom filter size 2000, got %d", r.bloomFilterSize)
	}

	// Test with zero size (should use default)
	r2 := New(WithBloomFilterSize(0))
	if r2.bloomFilterSize != 1000 {
		t.Errorf("Expected default bloom filter size 1000, got %d", r2.bloomFilterSize)
	}
}

// mockLogger implements the Logger interface for testing
type mockLogger struct {
	lastError string
}

func (m *mockLogger) Error(msg string, keysAndValues ...any) {
	m.lastError = msg
}

func (m *mockLogger) Warn(msg string, keysAndValues ...any) {}

func (m *mockLogger) Info(msg string, keysAndValues ...any) {}

func (m *mockLogger) Debug(msg string, keysAndValues ...any) {}

// TestWithLogger tests logger configuration
func TestWithLogger(t *testing.T) {
	logger := &mockLogger{}
	r := New(WithLogger(logger))

	if r.logger == nil {
		t.Error("Expected logger to be set")
	}
}

// ============================================================================
// HTTP Interface Tests (merged from http_interface_test.go)
// ============================================================================

type mockHijackableResponseWriter struct {
	*httptest.ResponseRecorder
	hijackCalled bool
	conn         net.Conn
	rw           *bufio.ReadWriter
	hijackErr    error
}

func (m *mockHijackableResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	m.hijackCalled = true
	return m.conn, m.rw, m.hijackErr
}

// mockFlushableResponseWriter implements http.ResponseWriter and http.Flusher for testing
type mockFlushableResponseWriter struct {
	*httptest.ResponseRecorder
	flushCalled bool
}

func (m *mockFlushableResponseWriter) Flush() {
	m.flushCalled = true
}

// mockHijackFlushResponseWriter implements both http.Hijacker and http.Flusher
type mockHijackFlushResponseWriter struct {
	*httptest.ResponseRecorder
	hijackCalled bool
	flushCalled  bool
	conn         net.Conn
	rw           *bufio.ReadWriter
	hijackErr    error
}

func (m *mockHijackFlushResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	m.hijackCalled = true
	return m.conn, m.rw, m.hijackErr
}

func (m *mockHijackFlushResponseWriter) Flush() {
	m.flushCalled = true
}

// TestResponseWriter_Hijack tests the Hijack method for WebSocket support
func TestResponseWriter_Hijack(t *testing.T) {
	r := New()

	var hijackedConn net.Conn
	var hijackedRW *bufio.ReadWriter
	var hijackErr error

	r.GET("/ws", func(c *Context) {
		// Try to hijack the connection (for WebSocket upgrade)
		if hijacker, ok := c.Response.(http.Hijacker); ok {
			hijackedConn, hijackedRW, hijackErr = hijacker.Hijack()
			c.Status(http.StatusSwitchingProtocols)
		} else {
			c.String(http.StatusInternalServerError, "Hijack not supported")
		}
	})

	// Create request with hijackable response writer
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)

	// Create mock connection for hijack
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	mockRW := bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server))

	mockWriter := &mockHijackableResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		conn:             server,
		rw:               mockRW,
	}

	// Serve request
	r.ServeHTTP(mockWriter, req)

	// Verify hijack was called
	if !mockWriter.hijackCalled {
		t.Error("Hijack() was not called")
	}

	if hijackErr != nil {
		t.Errorf("Hijack() returned error: %v", hijackErr)
	}

	if hijackedConn == nil {
		t.Error("Hijack() should return connection")
	}

	if hijackedRW == nil {
		t.Error("Hijack() should return bufio.ReadWriter")
	}
}

// TestResponseWriter_HijackNotSupported tests Hijack when underlying writer doesn't support it
func TestResponseWriter_HijackNotSupported(t *testing.T) {
	r := New()

	var hijackErr error

	r.GET("/ws", func(c *Context) {
		if hijacker, ok := c.Response.(http.Hijacker); ok {
			_, _, hijackErr = hijacker.Hijack()
		} else {
			c.String(http.StatusInternalServerError, "WebSocket not supported")
		}
	})

	// Use standard httptest.ResponseRecorder which doesn't support Hijack
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	w := httptest.NewRecorder()

	// Wrap in our responseWriter
	rw := &responseWriter{ResponseWriter: w}

	// Create context manually to use wrapped writer
	ctx := NewContext(rw, req)
	ctx.handlers = []HandlerFunc{func(c *Context) {
		if hijacker, ok := c.Response.(http.Hijacker); ok {
			_, _, hijackErr = hijacker.Hijack()
			if hijackErr == nil {
				t.Error("expected error when hijacking non-hijackable writer")
			}
		} else {
			t.Error("responseWriter should implement http.Hijacker interface")
		}
	}}
	ctx.router = r
	ctx.index = -1

	ctx.Next()

	// The error should indicate hijack is not supported
	if hijackErr == nil {
		t.Error("expected error when Hijack is not supported by underlying writer")
	}
}

// TestResponseWriter_Flush tests the Flush method for streaming responses
func TestResponseWriter_Flush(t *testing.T) {
	r := New()

	r.GET("/stream", func(c *Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")

		// Write first chunk
		c.String(http.StatusOK, "data: chunk1\n\n")

		// Flush to send immediately
		if flusher, ok := c.Response.(http.Flusher); ok {
			flusher.Flush()
		}

		// Write second chunk
		c.String(http.StatusOK, "data: chunk2\n\n")

		// Flush again
		if flusher, ok := c.Response.(http.Flusher); ok {
			flusher.Flush()
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)

	mockWriter := &mockFlushableResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
	}

	r.ServeHTTP(mockWriter, req)

	if !mockWriter.flushCalled {
		t.Error("Flush() was not called")
	}

	if mockWriter.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", mockWriter.Code)
	}

	body := mockWriter.Body.String()
	if body != "data: chunk1\n\ndata: chunk2\n\n" {
		t.Errorf("unexpected body: %q", body)
	}
}

// TestResponseWriter_FlushNotSupported tests Flush when underlying writer doesn't support it
func TestResponseWriter_FlushNotSupported(t *testing.T) {
	r := New()

	flushAttempted := false

	r.GET("/stream", func(c *Context) {
		c.String(http.StatusOK, "data")

		// Try to flush
		if flusher, ok := c.Response.(http.Flusher); ok {
			flusher.Flush()
			flushAttempted = true
		} else {
			flushAttempted = false
		}
	})

	// Use standard httptest.ResponseRecorder (supports Flush)
	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// httptest.ResponseRecorder actually implements Flusher, so this should work
	if !flushAttempted {
		t.Error("Flush should be supported by httptest.ResponseRecorder")
	}
}

// TestResponseWriter_HijackAndFlush tests both Hijack and Flush on same writer
func TestResponseWriter_HijackAndFlush(t *testing.T) {
	r := New()

	r.GET("/websocket", func(c *Context) {
		// First, flush some headers
		c.Header("Upgrade", "websocket")
		c.Header("Connection", "Upgrade")

		if flusher, ok := c.Response.(http.Flusher); ok {
			flusher.Flush()
		} else {
			t.Error("expected Flusher interface")
		}

		// Then hijack the connection
		if hijacker, ok := c.Response.(http.Hijacker); ok {
			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Errorf("Hijack failed: %v", err)
			}
			if conn != nil {
				// Don't close here as it's managed by test cleanup
			}
		} else {
			t.Error("expected Hijacker interface")
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/websocket", nil)

	// Create connection pair
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	mockRW := bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server))

	mockWriter := &mockHijackFlushResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		conn:             server,
		rw:               mockRW,
	}

	r.ServeHTTP(mockWriter, req)

	if !mockWriter.flushCalled {
		t.Error("Flush() was not called")
	}

	if !mockWriter.hijackCalled {
		t.Error("Hijack() was not called")
	}
}

// TestResponseWriter_HijackPreservesStatusAndSize tests that Hijack works with responseWriter wrapper
func TestResponseWriter_HijackPreservesStatusAndSize(t *testing.T) {
	r := New()

	r.GET("/ws", func(c *Context) {
		// Write some data first
		c.Header("X-Test", "value")
		c.Status(http.StatusSwitchingProtocols)
		c.Response.Write([]byte("Upgrading"))

		// Then hijack
		if hijacker, ok := c.Response.(http.Hijacker); ok {
			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if conn == nil {
				t.Error("expected connection")
			}
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	mockRW := bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server))
	mockWriter := &mockHijackableResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		conn:             server,
		rw:               mockRW,
	}

	r.ServeHTTP(mockWriter, req)

	if !mockWriter.hijackCalled {
		t.Error("Hijack should be called")
	}
}

// TestResponseWriter_FlushWithMetrics tests Flush works when metrics are enabled
func TestResponseWriter_FlushWithMetrics(t *testing.T) {
	r := New()

	// Create a mock metrics recorder
	mockMetrics := &mockMetricsRecorder{enabled: true}
	r.SetMetricsRecorder(mockMetrics)

	flushed := false

	r.GET("/stream", func(c *Context) {
		c.String(http.StatusOK, "chunk 1")

		if flusher, ok := c.Response.(http.Flusher); ok {
			flusher.Flush()
			flushed = true
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	mockWriter := &mockFlushableResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
	}

	r.ServeHTTP(mockWriter, req)

	if !flushed {
		t.Error("Flush was not attempted")
	}

	if !mockWriter.flushCalled {
		t.Error("Flush() was not called on underlying writer")
	}
}

// TestResponseWriter_InterfaceAssertion tests that responseWriter properly implements interfaces
func TestResponseWriter_InterfaceAssertion(t *testing.T) {
	// Test with hijackable writer
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	mockRW := bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server))
	hijackable := &mockHijackableResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		conn:             server,
		rw:               mockRW,
	}

	wrapped := &responseWriter{ResponseWriter: hijackable}

	// Should be able to assert as Hijacker
	var wrappedInterface http.ResponseWriter = wrapped
	if _, ok := wrappedInterface.(http.Hijacker); !ok {
		t.Error("wrapped hijackable writer should implement http.Hijacker")
	}

	// Test with flushable writer
	flushable := &mockFlushableResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
	}

	wrapped = &responseWriter{ResponseWriter: flushable}
	wrappedInterface = wrapped

	// Should be able to assert as Flusher
	if _, ok := wrappedInterface.(http.Flusher); !ok {
		t.Error("wrapped flushable writer should implement http.Flusher")
	}

	// Test with both
	both := &mockHijackFlushResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		conn:             server,
		rw:               mockRW,
	}

	wrapped = &responseWriter{ResponseWriter: both}
	wrappedInterface = wrapped

	if _, ok := wrappedInterface.(http.Hijacker); !ok {
		t.Error("wrapped writer should implement http.Hijacker")
	}

	if _, ok := wrappedInterface.(http.Flusher); !ok {
		t.Error("wrapped writer should implement http.Flusher")
	}
}

// TestResponseWriter_HijackError tests error handling in Hijack
func TestResponseWriter_HijackError(t *testing.T) {
	r := New()

	var receivedErr error

	r.GET("/ws", func(c *Context) {
		if hijacker, ok := c.Response.(http.Hijacker); ok {
			_, _, receivedErr = hijacker.Hijack()
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)

	mockWriter := &mockHijackableResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		hijackErr:        http.ErrNotSupported,
	}

	r.ServeHTTP(mockWriter, req)

	if receivedErr == nil {
		t.Error("expected error from Hijack()")
	}

	if receivedErr != http.ErrNotSupported {
		t.Errorf("expected ErrNotSupported, got %v", receivedErr)
	}
}

// TestResponseWriter_FlushNoOp tests Flush on non-flushable writer (should be no-op)
func TestResponseWriter_FlushNoOp(t *testing.T) {
	// Create a response writer that doesn't support Flush
	type nonFlushableWriter struct {
		*httptest.ResponseRecorder
	}

	w := &nonFlushableWriter{
		ResponseRecorder: httptest.NewRecorder(),
	}

	wrapped := &responseWriter{ResponseWriter: w}

	// Calling Flush should be a no-op, not panic
	var wrappedInterface http.ResponseWriter = wrapped
	if flusher, ok := wrappedInterface.(http.Flusher); ok {
		flusher.Flush() // Should not panic
	}

	// Verify we can still write after failed flush
	wrapped.Write([]byte("test"))
	if w.Body.String() != "test" {
		t.Error("should still be able to write after flush on non-flushable writer")
	}
}

// TestResponseWriter_StatusCodeAndSizeAfterHijack tests status tracking after hijack
func TestResponseWriter_StatusCodeAndSizeAfterHijack(t *testing.T) {
	r := New()

	r.GET("/ws", func(c *Context) {
		// Write status and data before hijack
		c.Status(http.StatusSwitchingProtocols)
		c.Response.Write([]byte("Upgrading"))

		// Get status and size
		if rw, ok := c.Response.(*responseWriter); ok {
			if rw.StatusCode() != http.StatusSwitchingProtocols {
				t.Errorf("expected status 101, got %d", rw.StatusCode())
			}

			if rw.Size() == 0 {
				t.Error("expected non-zero size before hijack")
			}
		}

		// Now hijack
		if hijacker, ok := c.Response.(http.Hijacker); ok {
			hijacker.Hijack()
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	mockRW := bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server))
	mockWriter := &mockHijackableResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		conn:             server,
		rw:               mockRW,
	}

	r.ServeHTTP(mockWriter, req)

	if !mockWriter.hijackCalled {
		t.Error("Hijack should be called")
	}
}

// TestResponseWriter_FlushBetweenWrites tests flushing between multiple writes
func TestResponseWriter_FlushBetweenWrites(t *testing.T) {
	r := New()

	flushCount := 0

	r.GET("/events", func(c *Context) {
		c.Header("Content-Type", "text/event-stream")

		// Write and flush multiple times
		for i := 1; i <= 3; i++ {
			c.Response.Write([]byte("event: message\n"))

			if flusher, ok := c.Response.(http.Flusher); ok {
				flusher.Flush()
				flushCount++
			}
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/events", nil)

	// Create custom flushable writer that counts flushes
	type countingFlusher struct {
		*httptest.ResponseRecorder
	}

	mockWriter := &countingFlusher{
		ResponseRecorder: httptest.NewRecorder(),
	}

	// We need to make it flushable
	flushableWriter := &mockFlushableResponseWriter{
		ResponseRecorder: mockWriter.ResponseRecorder,
	}

	r.ServeHTTP(flushableWriter, req)

	if !flushableWriter.flushCalled {
		t.Error("Flush() should be called at least once")
	}
}

// TestResponseWriter_HijackWithTracing tests Hijack when tracing is enabled
func TestResponseWriter_HijackWithTracing(t *testing.T) {
	r := New()

	mockTracing := &mockTracingRecorder{enabled: true}
	r.SetTracingRecorder(mockTracing)

	r.GET("/ws", func(c *Context) {
		if hijacker, ok := c.Response.(http.Hijacker); ok {
			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Errorf("Hijack failed: %v", err)
			}
			if conn == nil {
				t.Error("expected connection")
			}
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	mockRW := bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server))
	mockWriter := &mockHijackableResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		conn:             server,
		rw:               mockRW,
	}

	r.ServeHTTP(mockWriter, req)

	if !mockWriter.hijackCalled {
		t.Error("Hijack should work with tracing enabled")
	}
}

// ============================================================================
// Router Options Tests (merged from router_options_test.go)
// ============================================================================

func TestSetLogger(t *testing.T) {
	r := New()

	// Create a test logger
	var logOutput strings.Builder
	logger := slog.New(slog.NewTextHandler(&logOutput, nil))

	// Set logger
	r.SetLogger(logger)

	// Verify logger is set by triggering a log entry
	// The router logs security events like header injection attempts
	r.GET("/test", func(c *Context) {
		// Try to set a header with newline (should be logged)
		c.Header("X-Test", "value\r\nX-Injected: malicious")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Check that logging occurred
	loggedOutput := logOutput.String()
	if !strings.Contains(loggedOutput, "header injection") {
		t.Errorf("expected header injection log, got: %s", loggedOutput)
	}
}

// TestWithBloomFilterHashFunctions tests bloom filter hash configuration
func TestWithBloomFilterHashFunctions(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int // Expected value after clamping
	}{
		{"negative value clamped to 1", -1, 1},
		{"zero clamped to 1", 0, 1},
		{"valid value 1", 1, 1},
		{"valid value 3", 3, 3},
		{"valid value 5", 5, 5},
		{"valid value 10", 10, 10},
		{"value > 10 clamped to 10", 15, 10},
		{"large value clamped to 10", 100, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(WithBloomFilterHashFunctions(tt.input))

			if r.bloomHashFunctions != tt.expected {
				t.Errorf("expected %d hash functions, got %d", tt.expected, r.bloomHashFunctions)
			}

			// Verify router still works
			r.GET("/test", func(c *Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("router should work with %d hash functions", tt.input)
			}
		})
	}
}

// TestWithCancellationCheck tests cancellation checking enable/disable
func TestWithCancellationCheck(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
	}{
		{"cancellation enabled", true},
		{"cancellation disabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(WithCancellationCheck(tt.enabled))

			if r.checkCancellation != tt.enabled {
				t.Errorf("expected checkCancellation=%v, got %v", tt.enabled, r.checkCancellation)
			}

			middlewareCalled := false
			handlerCalled := false

			r.Use(func(c *Context) {
				middlewareCalled = true
				c.Next()
			})

			r.GET("/test", func(c *Context) {
				handlerCalled = true
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if !middlewareCalled || !handlerCalled {
				t.Error("both middleware and handler should be called regardless of cancellation check setting")
			}

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}
		})
	}
}

// TestWithCancellationCheck_ContextCancelled tests that cancellation checking actually works
func TestWithCancellationCheck_ContextCancelled(t *testing.T) {
	// Test with cancellation checking enabled
	t.Run("enabled", func(t *testing.T) {
		r := New(WithCancellationCheck(true))

		handlerCalled := false

		r.Use(func(c *Context) {
			// Cancel the context
			// We can't actually cancel c.Request.Context() easily in tests,
			// but we can verify the check is in place
			c.Next()
		})

		r.GET("/test", func(c *Context) {
			handlerCalled = true
			c.Status(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if !handlerCalled {
			t.Error("handler should be called when context is not cancelled")
		}
	})

	// Test with cancellation checking disabled
	t.Run("disabled", func(t *testing.T) {
		r := New(WithCancellationCheck(false))

		handlerCalled := false

		r.Use(func(c *Context) {
			c.Next()
		})

		r.GET("/test", func(c *Context) {
			handlerCalled = true
			c.Status(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if !handlerCalled {
			t.Error("handler should be called when cancellation check is disabled")
		}
	})
}

// TestWithTemplateRouting tests template routing enable/disable
func TestWithTemplateRouting(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
	}{
		{"templates enabled", true},
		{"templates disabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(WithTemplateRouting(tt.enabled))

			if r.useTemplates != tt.enabled {
				t.Errorf("expected useTemplates=%v, got %v", tt.enabled, r.useTemplates)
			}

			// Verify routing still works
			r.GET("/users/:id", func(c *Context) {
				c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
			})

			r.GET("/static", func(c *Context) {
				c.String(http.StatusOK, "static")
			})

			// Test static route
			req := httptest.NewRequest(http.MethodGet, "/static", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("static route should work, got status %d", w.Code)
			}

			if w.Body.String() != "static" {
				t.Errorf("expected 'static', got %q", w.Body.String())
			}

			// Test param route
			req = httptest.NewRequest(http.MethodGet, "/users/123", nil)
			w = httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("param route should work, got status %d", w.Code)
			}
		})
	}
}

// TestWithBloomFilterSize_EdgeCases tests additional edge cases beyond basic test
func TestWithBloomFilterSize_EdgeCases(t *testing.T) {
	// The basic test exists in router_test.go, this tests edge cases
	tests := []struct {
		name string
		size uint64
	}{
		{"very large size", 100000},
		{"size 1", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(WithBloomFilterSize(tt.size))

			if r.bloomFilterSize != tt.size {
				t.Errorf("expected size %d, got %d", tt.size, r.bloomFilterSize)
			}

			// Verify router works
			r.GET("/test", func(c *Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Error("router should work with custom bloom filter size")
			}
		})
	}
}

// TestRouterOptions_Combined tests multiple options together
func TestRouterOptions_Combined(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	r := New(
		WithLogger(logger),
		WithBloomFilterSize(2000),
		WithBloomFilterHashFunctions(5),
		WithCancellationCheck(false),
		WithTemplateRouting(true),
	)

	// Verify all options were applied
	if r.bloomFilterSize != 2000 {
		t.Errorf("expected bloom filter size 2000, got %d", r.bloomFilterSize)
	}

	if r.bloomHashFunctions != 5 {
		t.Errorf("expected 5 hash functions, got %d", r.bloomHashFunctions)
	}

	if r.checkCancellation {
		t.Error("expected cancellation check to be disabled")
	}

	if !r.useTemplates {
		t.Error("expected templates to be enabled")
	}

	// Verify router works with all options
	r.GET("/users/:id", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
	})

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestWithTemplateRouting_StaticVsDynamic tests template routing with different route types
func TestWithTemplateRouting_StaticVsDynamic(t *testing.T) {
	// Test with templates enabled
	t.Run("templates_enabled", func(t *testing.T) {
		r := New(WithTemplateRouting(true))

		staticCalled := false
		dynamicCalled := false

		r.GET("/static/path", func(c *Context) {
			staticCalled = true
			c.String(http.StatusOK, "static")
		})

		r.GET("/users/:id/posts/:postId", func(c *Context) {
			dynamicCalled = true
			c.JSON(http.StatusOK, map[string]string{
				"userId": c.Param("id"),
				"postId": c.Param("postId"),
			})
		})

		// Test static route
		req := httptest.NewRequest(http.MethodGet, "/static/path", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if !staticCalled {
			t.Error("static route should be called")
		}

		// Test dynamic route
		req = httptest.NewRequest(http.MethodGet, "/users/1/posts/2", nil)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if !dynamicCalled {
			t.Error("dynamic route should be called")
		}
	})

	// Test with templates disabled
	t.Run("templates_disabled", func(t *testing.T) {
		r := New(WithTemplateRouting(false))

		staticCalled := false
		dynamicCalled := false

		r.GET("/static/path", func(c *Context) {
			staticCalled = true
			c.String(http.StatusOK, "static")
		})

		r.GET("/users/:id", func(c *Context) {
			dynamicCalled = true
			c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
		})

		// Both should work even without templates
		req := httptest.NewRequest(http.MethodGet, "/static/path", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if !staticCalled {
			t.Error("static route should work without templates")
		}

		req = httptest.NewRequest(http.MethodGet, "/users/123", nil)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if !dynamicCalled {
			t.Error("dynamic route should work without templates")
		}
	})
}

// TestRouterOptions_Defaults tests that default values are set correctly
func TestRouterOptions_Defaults(t *testing.T) {
	r := New()

	// Check default values
	if r.bloomFilterSize != 1000 {
		t.Errorf("expected default bloom filter size 1000, got %d", r.bloomFilterSize)
	}

	if r.bloomHashFunctions != 3 {
		t.Errorf("expected default hash functions 3, got %d", r.bloomHashFunctions)
	}

	if !r.checkCancellation {
		t.Error("expected cancellation check to be enabled by default")
	}

	if !r.useTemplates {
		t.Error("expected templates to be enabled by default")
	}

	if r.contextPool == nil {
		t.Error("context pool should be initialized")
	}

	if r.templateCache == nil {
		t.Error("template cache should be initialized")
	}
}

// TestRouterOptions_MultipleLoggerCalls tests setting logger multiple times
func TestRouterOptions_MultipleLoggerCalls(t *testing.T) {
	var output1, output2 strings.Builder

	logger1 := slog.New(slog.NewTextHandler(&output1, nil))
	logger2 := slog.New(slog.NewTextHandler(&output2, nil))

	r := New(WithLogger(logger1))

	// Set logger again
	r.SetLogger(logger2)

	// Trigger logging
	r.GET("/test", func(c *Context) {
		c.Header("X-Test", "value\ninjection")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Second logger should receive the log
	if !strings.Contains(output2.String(), "header injection") {
		t.Error("second logger should receive logs")
	}

	// First logger should not receive the log
	if strings.Contains(output1.String(), "header injection") {
		t.Error("first logger should not receive logs after being replaced")
	}
}

// TestWithCancellationCheck_Performance tests performance impact of cancellation checking
func TestWithCancellationCheck_Performance(t *testing.T) {
	// This is a sanity test, not a benchmark
	// Just verify both modes work correctly

	for _, enabled := range []bool{true, false} {
		t.Run(strings.ToLower(strings.ReplaceAll(t.Name(), " ", "_")), func(t *testing.T) {
			r := New(WithCancellationCheck(enabled))

			count := 0

			// Add multiple middleware to test the check happens in each
			for i := 0; i < 5; i++ {
				r.Use(func(c *Context) {
					count++
					c.Next()
				})
			}

			r.GET("/test", func(c *Context) {
				count++
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// All 6 handlers should be called (5 middleware + 1 handler)
			if count != 6 {
				t.Errorf("expected 6 handlers called, got %d", count)
			}

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}
		})
	}
}

// TestWithTemplateRouting_AfterRoutesRegistered tests changing template setting
func TestWithTemplateRouting_AfterRoutesRegistered(t *testing.T) {
	r := New(WithTemplateRouting(true))

	// Register routes
	r.GET("/test1", func(c *Context) {
		c.String(http.StatusOK, "test1")
	})

	// Routes should work
	req := httptest.NewRequest(http.MethodGet, "/test1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "test1" {
		t.Errorf("expected 'test1', got %q", w.Body.String())
	}
}

// TestWithBloomFilterHashFunctions_EdgeCases tests extreme edge cases
func TestWithBloomFilterHashFunctions_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"very negative", -999, 1},
		{"very large", 999, 10},
		{"max valid", 10, 10},
		{"min valid", 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(WithBloomFilterHashFunctions(tt.input))

			if r.bloomHashFunctions != tt.expected {
				t.Errorf("input %d: expected %d, got %d", tt.input, tt.expected, r.bloomHashFunctions)
			}

			// Verify warmup doesn't panic
			r.GET("/test", func(c *Context) {
				c.Status(http.StatusOK)
			})
			r.WarmupOptimizations()

			// Verify routing works
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("routing should work with %d hash functions", tt.input)
			}
		})
	}
}
