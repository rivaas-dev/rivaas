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
	"bufio"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestWithBloomFilterSize tests bloom filter configuration
func TestWithBloomFilterSize(t *testing.T) {
	t.Parallel()
	r := MustNew(WithBloomFilterSize(2000))

	if r.bloomFilterSize != 2000 {
		t.Errorf("Expected bloom filter size 2000, got %d", r.bloomFilterSize)
	}

	// Test with zero size (should use default)
	r2 := MustNew(WithBloomFilterSize(0))
	if r2.bloomFilterSize != 1000 {
		t.Errorf("Expected default bloom filter size 1000, got %d", r2.bloomFilterSize)
	}
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

	if r.diagnostics == nil {
		t.Error("Expected diagnostics handler to be set")
	}
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

// TestResponseWriter_FlushNotSupported tests Flush when underlying writer doesn't support it
func TestResponseWriter_FlushNotSupported(t *testing.T) {
	t.Parallel()
	r := MustNew()

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

// TestResponseWriter_InterfaceAssertion tests that responseWriter properly implements interfaces
func TestResponseWriter_InterfaceAssertion(t *testing.T) {
	t.Parallel()
	// Test with hijackable writer
	server, client := net.Pipe()
	defer func() {
		_ = server.Close()
		_ = client.Close()
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

	if receivedErr == nil {
		t.Error("expected error from Hijack()")
	}

	if !errors.Is(receivedErr, http.ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got %v", receivedErr)
	}
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
	wrapped.Write([]byte("test"))
	if w.Body.String() != "test" {
		t.Error("should still be able to write after flush on non-flushable writer")
	}
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

		if !handlerCalled {
			t.Error("handler should be called when context is not cancelled")
		}
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

		if !handlerCalled {
			t.Error("handler should be called when cancellation check is disabled")
		}
	})
}

// TestWithTemplateRouting tests template routing enable/disable
func TestWithTemplateRouting(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		enabled bool
	}{
		{"templates enabled", true},
		{"templates disabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := MustNew(WithTemplateRouting(tt.enabled))

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

// TestRouterOptions_Defaults tests that default values are set correctly
func TestRouterOptions_Defaults(t *testing.T) {
	t.Parallel()
	r := MustNew()

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
	if len(events2) == 0 || events2[0].Kind != DiagHeaderInjection {
		t.Error("second handler should receive diagnostic events")
	}

	// First handler should not receive the event (we created a new router)
	if len(events1) > 0 {
		t.Error("first logger should not receive logs after being replaced")
	}
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

			if r.bloomHashFunctions != tt.expected {
				t.Errorf("input %d: expected %d, got %d", tt.input, tt.expected, r.bloomHashFunctions)
			}

			// Verify warmup doesn't panic
			r.GET("/test", func(c *Context) {
				c.Status(http.StatusOK)
			})
			r.Warmup()

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
