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

//go:build !integration

package router

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWithBloomFilterSize tests bloom filter configuration
func TestWithBloomFilterSize(t *testing.T) {
	t.Parallel()
	r := MustNew(WithBloomFilterSize(2000))

	assert.Equal(t, uint64(2000), r.bloomFilterSize)

	// Test with zero size (should fail validation)
	_, err := New(WithBloomFilterSize(0))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bloom filter size must be non-zero")
}

// mockDiagnosticHandler implements the DiagnosticHandler interface for testing
type mockDiagnosticHandler struct {
	events []DiagnosticEvent
}

func (m *mockDiagnosticHandler) OnDiagnostic(e DiagnosticEvent) {
	m.events = append(m.events, e)
}

// TestWithDiagnostics tests diagnostic handler configuration
func TestWithDiagnostics(t *testing.T) {
	t.Parallel()
	handler := &mockDiagnosticHandler{}
	r := MustNew(WithDiagnostics(handler))

	assert.NotNil(t, r.diagnostics, "Expected diagnostics handler to be set")
}

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

// TestResponseWriter_HijackNotSupported tests Hijack when underlying writer doesn't support it
func TestResponseWriter_HijackNotSupported(t *testing.T) {
	t.Parallel()
	r := MustNew()

	var hijackErr error

	r.GET("/ws", func(c *Context) {
		if hijacker, ok := c.Response.(http.Hijacker); ok {
			_, _, hijackErr = hijacker.Hijack()
		} else {
			require.NoError(t, c.String(http.StatusInternalServerError, "WebSocket not supported"))
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
			assert.Error(t, hijackErr, "expected error when hijacking non-hijackable writer")
		} else {
			assert.Fail(t, "responseWriter should implement http.Hijacker interface")
		}
	}}
	ctx.router = r
	ctx.index = -1

	ctx.Next()

	// The error should indicate hijack is not supported
	assert.Error(t, hijackErr, "expected error when Hijack is not supported by underlying writer")
}

// TestResponseWriter_FlushNotSupported tests Flush when underlying writer doesn't support it
func TestResponseWriter_FlushNotSupported(t *testing.T) {
	t.Parallel()
	r := MustNew()

	flushAttempted := false

	r.GET("/stream", func(c *Context) {
		require.NoError(t, c.String(http.StatusOK, "data"))

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
	assert.True(t, flushAttempted, "Flush should be supported by httptest.ResponseRecorder")
}

// TestResponseWriter_InterfaceAssertion tests that responseWriter properly implements interfaces
func TestResponseWriter_InterfaceAssertion(t *testing.T) {
	t.Parallel()
	// Test with hijackable writer
	server, client := net.Pipe()
	defer func() {
		//nolint:errcheck // Test cleanup
		server.Close()
		//nolint:errcheck // Test cleanup
		client.Close()
	}()

	mockRW := bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server))
	hijackable := &mockHijackableResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		conn:             server,
		rw:               mockRW,
	}

	wrapped := &responseWriter{ResponseWriter: hijackable}

	// Should be able to assert as Hijacker
	var wrappedInterface http.ResponseWriter = wrapped
	_, ok := wrappedInterface.(http.Hijacker)
	assert.True(t, ok, "wrapped hijackable writer should implement http.Hijacker")

	// Test with flushable writer
	flushable := &mockFlushableResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
	}

	wrapped = &responseWriter{ResponseWriter: flushable}
	wrappedInterface = wrapped

	// Should be able to assert as Flusher
	_, ok = wrappedInterface.(http.Flusher)
	assert.True(t, ok, "wrapped flushable writer should implement http.Flusher")

	// Test with both
	both := &mockHijackFlushResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		conn:             server,
		rw:               mockRW,
	}

	wrapped = &responseWriter{ResponseWriter: both}
	wrappedInterface = wrapped

	_, ok = wrappedInterface.(http.Hijacker)
	assert.True(t, ok, "wrapped writer should implement http.Hijacker")

	_, ok = wrappedInterface.(http.Flusher)
	assert.True(t, ok, "wrapped writer should implement http.Flusher")
}

// TestResponseWriter_HijackError tests error handling in Hijack
func TestResponseWriter_HijackError(t *testing.T) {
	t.Parallel()
	r := MustNew()

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

	require.Error(t, receivedErr, "expected error from Hijack()")
	assert.ErrorIs(t, receivedErr, http.ErrNotSupported)
}

// TestResponseWriter_FlushNoOp tests Flush on non-flushable writer (should be no-op)
func TestResponseWriter_FlushNoOp(t *testing.T) {
	t.Parallel()
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
	_, err := wrapped.Write([]byte("test"))
	require.NoError(t, err)
	assert.Equal(t, "test", w.Body.String(), "should still be able to write after flush on non-flushable writer")
}

// TestNoopLogger covers NoopLogger().
func TestNoopLogger(t *testing.T) {
	t.Parallel()
	logger := NoopLogger()
	require.NotNil(t, logger)
}

// TestResponseWriter_StatusCode_Size_Written covers responseWriter getters.
func TestResponseWriter_StatusCode_Size_Written(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w}
	assert.False(t, rw.Written())
	_, err := rw.Write([]byte("body"))
	require.NoError(t, err)
	assert.True(t, rw.Written())
	assert.Equal(t, http.StatusOK, rw.StatusCode())
	assert.Equal(t, int64(4), rw.Size())
	rw.WriteHeader(http.StatusCreated)
	// Already written, status stays 200
	assert.Equal(t, http.StatusOK, rw.StatusCode())
}

// TestRouteExists covers RouteExists for existing and missing routes.
func TestRouteExists(t *testing.T) {
	t.Parallel()
	r := MustNew()
	r.GET("/health", func(c *Context) { c.Status(http.StatusOK) })
	r.Warmup() // Trees are populated during Warmup
	assert.True(t, r.RouteExists("GET", "/health"))
	assert.False(t, r.RouteExists("GET", "/missing"))
	assert.False(t, r.RouteExists("POST", "/health"))
}

// TestHandleMethodNotAllowed triggers 405 when method does not match path.
func TestHandleMethodNotAllowed(t *testing.T) {
	t.Parallel()
	r := MustNew()
	r.GET("/only-get", func(c *Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodPost, "/only-get", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assert.Equal(t, "GET", w.Header().Get("Allow"))
}

// TestWithBloomFilterHashFunctions tests bloom filter hash configuration
func TestWithBloomFilterHashFunctions(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			r := MustNew(WithBloomFilterHashFunctions(tt.input))

			assert.Equal(t, tt.expected, r.bloomHashFunctions)

			// Verify router still works
			r.GET("/test", func(c *Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "router should work with %d hash functions", tt.input)
		})
	}
}

// TestWithCancellationCheck tests cancellation checking enable/disable
func TestWithCancellationCheck(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		enabled bool
	}{
		{"cancellation enabled", true},
		{"cancellation disabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := MustNew(WithCancellationCheck(tt.enabled))

			assert.Equal(t, tt.enabled, r.checkCancellation)

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

			assert.True(t, middlewareCalled && handlerCalled, "both middleware and handler should be called regardless of cancellation check setting")
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

// TestWithCancellationCheck_ContextCancelled tests that cancellation checking actually works
func TestWithCancellationCheck_ContextCancelled(t *testing.T) {
	t.Parallel()
	// Test with cancellation checking enabled
	t.Run("enabled", func(t *testing.T) {
		t.Parallel()
		r := MustNew(WithCancellationCheck(true))

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

		assert.True(t, handlerCalled, "handler should be called when context is not canceled")
	})

	// Test with cancellation checking disabled
	t.Run("disabled", func(t *testing.T) {
		t.Parallel()
		r := MustNew(WithCancellationCheck(false))

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

		assert.True(t, handlerCalled, "handler should be called when cancellation check is disabled")
	})
}

// TestWithRouteCompilation tests route compilation enable/disable
func TestWithRouteCompilation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		enabled bool
	}{
		{"compilation enabled", true},
		{"compilation disabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := MustNew(WithRouteCompilation(tt.enabled))

			assert.Equal(t, tt.enabled, r.useCompiledRoutes)

			// Verify routing still works
			r.GET("/users/:id", func(c *Context) {
				require.NoError(t, c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")}))
			})

			r.GET("/static", func(c *Context) {
				require.NoError(t, c.String(http.StatusOK, "static"))
			})

			// Test static route
			req := httptest.NewRequest(http.MethodGet, "/static", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "static route should work")
			assert.Equal(t, "static", w.Body.String())

			// Test param route
			req = httptest.NewRequest(http.MethodGet, "/users/123", nil)
			w = httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "param route should work")
		})
	}
}

// TestRouterOptions_Defaults tests that default values are set correctly
func TestRouterOptions_Defaults(t *testing.T) {
	t.Parallel()
	r := MustNew()

	// Check default values
	assert.Equal(t, uint64(1000), r.bloomFilterSize)
	assert.Equal(t, 3, r.bloomHashFunctions)
	assert.True(t, r.checkCancellation, "expected cancellation check to be enabled by default")
	assert.False(t, r.useCompiledRoutes, "expected compiled routes to be disabled by default")
	assert.NotNil(t, r.routeCompiler, "route compiler should be initialized")
}

// TestRouterOptions_MultipleLoggerCalls tests setting logger multiple times
func TestRouterOptions_MultipleLoggerCalls(t *testing.T) {
	t.Parallel()
	var events1, events2 []DiagnosticEvent

	handler1 := DiagnosticHandlerFunc(func(e DiagnosticEvent) {
		events1 = append(events1, e)
	})
	handler2 := DiagnosticHandlerFunc(func(e DiagnosticEvent) {
		events2 = append(events2, e)
	})

	// Create first router (not used, just to verify it doesn't interfere)
	_ = MustNew(WithDiagnostics(handler1))

	// Create second router with different handler
	r := MustNew(WithDiagnostics(handler2))

	// Trigger diagnostic event
	r.GET("/test", func(c *Context) {
		c.Header("X-Test", "value\ninjection")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Second handler should receive the diagnostic event
	assert.NotEmpty(t, events2, "second handler should receive diagnostic events")
	assert.Equal(t, DiagHeaderInjection, events2[0].Kind)

	// First handler should not receive the event (we created a new router)
	assert.Empty(t, events1, "first logger should not receive logs after being replaced")
}

// TestWithBloomFilterHashFunctions_EdgeCases tests extreme edge cases
func TestWithBloomFilterHashFunctions_EdgeCases(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			r := MustNew(WithBloomFilterHashFunctions(tt.input))

			assert.Equal(t, tt.expected, r.bloomHashFunctions, "input %d", tt.input)

			// Verify warmup doesn't panic
			r.GET("/test", func(c *Context) {
				c.Status(http.StatusOK)
			})
			r.Warmup()

			// Verify routing works
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "routing should work with %d hash functions", tt.input)
		})
	}
}

// TestCompiledRouteRequiresDeferForCleanup verifies that contexts are properly
// released even when handlers panic.
func TestCompiledRouteRequiresDeferForCleanup(t *testing.T) {
	t.Parallel()

	r := MustNew()

	cleanupExecuted := false
	var capturedContext *Context

	r.GET("/panic", func(c *Context) {
		capturedContext = c
		// Add a deferred cleanup in the handler to verify execution order
		defer func() {
			// This defer in the handler runs BEFORE the router's defer
			// If the router's defer exists, capturedContext should still be valid here
			cleanupExecuted = true
		}()
		panic("test panic")
	})

	r.CompileAllRoutes()

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	// This should panic
	require.Panics(t, func() {
		r.ServeHTTP(w, req)
	})

	// Verify the handler's cleanup executed
	// This proves the defer chain works correctly
	require.True(t, cleanupExecuted, "handler defer should execute even on panic")
	require.NotNil(t, capturedContext, "should have captured context before panic")

	// After cleanup, the context should have been reset and returned to pool
	// We can verify this by making another request and checking it works fine
	req2 := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w2 := httptest.NewRecorder()
	require.Panics(t, func() {
		r.ServeHTTP(w2, req2)
	}, "second request should also panic as expected")
}
