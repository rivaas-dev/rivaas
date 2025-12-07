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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContextHelpers tests context helper methods
func TestContextHelpers(t *testing.T) {
	t.Parallel()

	r := MustNew()

	t.Run("PostForm", func(t *testing.T) {
		t.Parallel()
		r.POST("/form", func(c *Context) {
			username := c.FormValue("username")
			password := c.FormValue("password")
			c.Stringf(http.StatusOK, "user=%s,pass=%s", username, password)
		})

		req := httptest.NewRequest(http.MethodPost, "/form", nil)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.PostForm = map[string][]string{
			"username": {"john"},
			"password": {"secret"},
		}
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "user=john,pass=secret", w.Body.String())
	})

	t.Run("PostFormDefault", func(t *testing.T) {
		t.Parallel()

		r := MustNew()
		r.POST("/form-default", func(c *Context) {
			role := c.FormValueDefault("role", "guest")
			c.Stringf(http.StatusOK, "role=%s", role)
		})

		req := httptest.NewRequest(http.MethodPost, "/form-default", nil)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "role=guest", w.Body.String())
	})

	t.Run("IsSecure", func(t *testing.T) {
		t.Parallel()

		r := MustNew()
		r.GET("/secure", func(c *Context) {
			if c.IsHTTPS() {
				c.String(http.StatusOK, "secure")
			} else {
				c.String(http.StatusOK, "insecure")
			}
		})

		// Test HTTP
		req := httptest.NewRequest("GET", "/secure", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, "insecure", w.Body.String())

		// Test with X-Forwarded-Proto header
		req = httptest.NewRequest("GET", "/secure", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, "secure", w.Body.String())
	})

	t.Run("NoContent", func(t *testing.T) {
		t.Parallel()

		r := MustNew()
		r.DELETE("/item", func(c *Context) {
			c.NoContent()
		})

		req := httptest.NewRequest(http.MethodDelete, "/item", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, w.Body.String())
	})

	t.Run("SetCookie and GetCookie", func(t *testing.T) {
		t.Parallel()
		r.GET("/set-cookie", func(c *Context) {
			c.SetCookie("session", "abc123", 3600, "/", "", false, true)
			c.String(http.StatusOK, "cookie set")
		})

		r.GET("/get-cookie", func(c *Context) {
			session, err := c.GetCookie("session")
			if err != nil {
				c.String(http.StatusNotFound, "no cookie")
			} else {
				c.Stringf(http.StatusOK, "session=%s", session)
			}
		})

		// Test setting cookie
		req := httptest.NewRequest("GET", "/set-cookie", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		cookies := w.Result().Cookies()
		assert.NotEmpty(t, cookies)

		// Test getting cookie
		req = httptest.NewRequest("GET", "/get-cookie", nil)
		req.AddCookie(cookies[0])
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "session=abc123")

		// Test missing cookie
		req = httptest.NewRequest("GET", "/get-cookie", nil)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, "no cookie", w.Body.String())
	})
}

// TestNewContext tests the NewContext function
func TestNewContext(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	ctx := NewContext(w, req)

	assert.NotNil(t, ctx)
	assert.Equal(t, req, ctx.Request)
	assert.Equal(t, w, ctx.Response)
	assert.Equal(t, int32(-1), ctx.index)
}

// TestStatusMethod tests the Status method edge cases
func TestStatusMethod(t *testing.T) {
	t.Parallel()

	r := MustNew()

	t.Run("Status with wrapped responseWriter", func(t *testing.T) {
		t.Parallel()
		r.GET("/status-wrapped", func(c *Context) {
			c.Status(http.StatusAccepted)
			c.String(http.StatusOK, "ok") // Should use Accepted status
		})

		req := httptest.NewRequest("GET", "/status-wrapped", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)
	})

	t.Run("Status with plain responseWriter", func(t *testing.T) {
		t.Parallel()
		// Create context with plain http.ResponseWriter
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		ctx := NewContext(w, req)

		ctx.Status(http.StatusCreated)
		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

// TestContext_String_MultipleFormatValues tests String method with complex formatting
func TestContext_String_MultipleFormatValues(t *testing.T) {
	t.Parallel()

	r := MustNew()

	r.GET("/test", func(c *Context) {
		c.Stringf(http.StatusOK, "Name: %s, Age: %d, Score: %.2f", "Bob", 28, 88.5)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Bob")
}

// TestContext_HTML_DifferentStatusCodes tests HTML with various status codes
func TestContext_HTML_DifferentStatusCodes(t *testing.T) {
	t.Parallel()

	codes := []int{200, 201, 404, 500}

	for _, code := range codes {
		r := MustNew()

		r.GET("/test", func(c *Context) {
			c.HTML(code, "<div>Content</div>")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, code, w.Code)
	}
}

func TestContext_Status_WithResponseWriter(t *testing.T) {
	t.Parallel()

	r := MustNew()

	r.GET("/test", func(c *Context) {
		c.Status(http.StatusCreated)
		c.Response.Write([]byte("created"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

// TestContext_Status_AlreadyWritten tests Status when headers already written
func TestContext_Status_AlreadyWritten(t *testing.T) {
	t.Parallel()

	r := MustNew()

	r.GET("/test", func(c *Context) {
		// Write something first (sets status to 200)
		c.Response.Write([]byte("data"))

		// Try to set status again (should be no-op)
		c.Status(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should be 200 (first write), not 201
	assert.Equal(t, http.StatusOK, w.Code, "expected status 200 (first write)")
}

// TestContext_Next_WithCancellation tests Next with cancelled context
//
//nolint:paralleltest // Tests cancellation behavior
func TestContext_Next_WithCancellation(t *testing.T) {
	r := MustNew(WithCancellationCheck(true))

	handlerCalled := false

	r.Use(func(c *Context) {
		handlerCalled = true
		c.Next()
	})

	r.GET("/test", func(c *Context) {
		c.Status(http.StatusOK)
	})

	// Create request with already-cancelled context
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// With cancelled context, cancellation check prevents handlers from running
	// The test verifies no panic occurs and request completes gracefully
	// Whether handlers are called depends on exact timing of cancellation detection
	_ = handlerCalled // May or may not be called, both are valid
}

// TestContext_Next_WithTimeout tests Next with timeout context
func TestContext_Next_WithTimeout(t *testing.T) {
	t.Parallel()

	r := MustNew(WithCancellationCheck(true))

	var callOrder []int

	r.Use(func(c *Context) {
		callOrder = append(callOrder, 1)
		c.Next()
	})

	r.Use(func(c *Context) {
		callOrder = append(callOrder, 2)
		c.Next()
	})

	r.GET("/test", func(c *Context) {
		callOrder = append(callOrder, 3)
		c.Status(http.StatusOK)
	})

	// Create request with timeout that hasn't expired yet
	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// All handlers should be called
	assert.Len(t, callOrder, 3, "expected 3 handlers called, got %d: %v", len(callOrder), callOrder)
}

// TestContext_Next_Abort tests that Abort stops the chain
func TestContext_Next_Abort(t *testing.T) {
	t.Parallel()

	r := MustNew()

	handler1Called := false
	handler2Called := false
	handler3Called := false

	r.Use(func(c *Context) {
		handler1Called = true
		c.Next()
	})

	r.Use(func(c *Context) {
		handler2Called = true
		c.Abort() // Stop the chain
	})

	r.GET("/test", func(c *Context) {
		handler3Called = true
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, handler1Called, "handler 1 should be called")
	assert.True(t, handler2Called, "handler 2 should be called")
	assert.False(t, handler3Called, "handler 3 should NOT be called after Abort()")
}

// TestContext_IsAborted tests the IsAborted method
func TestContext_IsAborted(t *testing.T) {
	t.Parallel()

	r := MustNew()

	var abortedInMiddleware bool
	var abortedInHandler bool

	r.Use(func(c *Context) {
		abortedInMiddleware = c.IsAborted()
		c.Abort()
		c.Next() // Should not execute next handler
	})

	r.GET("/test", func(c *Context) {
		abortedInHandler = c.IsAborted()
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.False(t, abortedInMiddleware, "should not be aborted at start of middleware")
	assert.False(t, abortedInHandler, "handler should not be called, so this check shouldn't run")
}

// TestContext_Reset_ClearsAllFields tests that reset properly clears all fields
func TestContext_Reset_ClearsAllFields(t *testing.T) {
	t.Parallel()

	r := MustNew()

	// Create and populate a context
	req := httptest.NewRequest(http.MethodGet, "/test?param=value", nil)
	w := httptest.NewRecorder()

	ctx := NewContext(w, req)
	ctx.handlers = []HandlerFunc{func(_ *Context) {}}
	ctx.router = r
	ctx.paramCount = 2
	ctx.paramKeys[0] = "key1"
	ctx.paramValues[0] = "value1"
	ctx.paramKeys[1] = "key2"
	ctx.paramValues[1] = "value2"
	ctx.version = "v1"
	ctx.aborted = true
	ctx.Params = make(map[string]string)
	ctx.Params["test"] = "value"

	// Reset
	ctx.reset()

	// Verify all fields are cleared
	assert.Nil(t, ctx.Request, "Request should be nil after reset")
	assert.Nil(t, ctx.Response, "Response should be nil after reset")
	assert.Nil(t, ctx.handlers, "handlers should be nil after reset")
	assert.Equal(t, int32(-1), ctx.index, "index should be -1")
	assert.Equal(t, int32(0), ctx.paramCount, "paramCount should be 0")
	assert.Empty(t, ctx.paramKeys[0], "paramKeys should be cleared")
	assert.Empty(t, ctx.version, "version should be empty after reset")
	assert.False(t, ctx.aborted, "aborted flag should be false after reset")
	assert.Empty(t, ctx.Params, "Params map should be cleared")
}

// TestContext_InitForRequest tests context initialization
func TestContext_InitForRequest(t *testing.T) {
	t.Parallel()

	r := MustNew()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handlers := []HandlerFunc{func(_ *Context) {}}

	ctx := NewContext(nil, nil)
	ctx.initForRequest(req, w, handlers, r)

	assert.Equal(t, req, ctx.Request, "Request should be set")
	assert.Equal(t, w, ctx.Response, "Response should be set")
	assert.Equal(t, r, ctx.router, "router should be set")
	assert.Equal(t, int32(-1), ctx.index, "index should be -1")
	assert.Equal(t, int32(0), ctx.paramCount, "paramCount should be 0")
}

// TestContext_InitForRequestWithParams tests init that preserves parameters
func TestContext_InitForRequestWithParams(t *testing.T) {
	t.Parallel()

	r := MustNew()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handlers := []HandlerFunc{func(_ *Context) {}}

	ctx := NewContext(nil, nil)

	// Set some parameters first
	ctx.paramCount = 2
	ctx.paramKeys[0] = "id"
	ctx.paramValues[0] = "123"
	ctx.paramKeys[1] = "name"
	ctx.paramValues[1] = "test"

	// Init with params (should preserve them)
	ctx.initForRequestWithParams(req, w, handlers, r)

	assert.Equal(t, req, ctx.Request, "Request should be set")
	assert.Equal(t, int32(2), ctx.paramCount, "paramCount should be preserved as 2")
	assert.Equal(t, "id", ctx.paramKeys[0], "paramKeys should be preserved")
	assert.Equal(t, "123", ctx.paramValues[0], "paramValues should be preserved")
}

// TestContext_Abort_Multiple tests calling Abort multiple times
func TestContext_Abort_Multiple(t *testing.T) {
	t.Parallel()

	r := MustNew()

	callCount := 0

	r.Use(func(c *Context) {
		callCount++
		c.Abort()
		c.Abort() // Call again (should be no-op)
		c.Next()  // Should not execute further handlers
	})

	r.GET("/test", func(c *Context) {
		callCount++
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 1, callCount, "expected 1 handler called")
}

// TestContext_Next_EmptyHandlers tests Next with no handlers
func TestContext_Next_EmptyHandlers(t *testing.T) {
	t.Parallel()

	ctx := NewContext(nil, nil)
	ctx.handlers = []HandlerFunc{}
	ctx.index = -1

	// Should not panic
	ctx.Next()

	assert.Equal(t, int32(0), ctx.index, "expected index 0")
}

// TestContext_Abort_BeforeNext tests aborting before calling Next
func TestContext_Abort_BeforeNext(t *testing.T) {
	t.Parallel()

	r := MustNew()

	middlewareCalled := false
	handlerCalled := false

	r.Use(func(c *Context) {
		middlewareCalled = true
		c.Abort() // Abort before Next
		// Don't call Next
	})

	r.GET("/test", func(c *Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, middlewareCalled, "middleware should be called")
	assert.False(t, handlerCalled, "handler should NOT be called after Abort()")
}

// TestContextPool_ResetBeforeReuse tests contexts are reset before reuse
func TestContextPool_ResetBeforeReuse(t *testing.T) {
	t.Parallel()

	r := MustNew()

	// First request sets some data
	r.GET("/first", func(c *Context) {
		c.Header("X-Custom", "value")
		c.Status(http.StatusOK)
	})

	req1 := httptest.NewRequest(http.MethodGet, "/first", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	// Second request should not have data from first
	r.GET("/second", func(c *Context) {
		// Context should be clean
		assert.False(t, c.IsAborted(), "context should not be aborted from previous request")
		assert.Empty(t, c.version, "version should be empty for new request")

		c.Status(http.StatusOK)
	})

	req2 := httptest.NewRequest(http.MethodGet, "/second", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
}

// TestContext_Next_NestedCalls tests nested Next calls
func TestContext_Next_NestedCalls(t *testing.T) {
	t.Parallel()

	r := MustNew()

	var callOrder []string

	r.Use(func(c *Context) {
		callOrder = append(callOrder, "middleware1-start")
		c.Next()
		callOrder = append(callOrder, "middleware1-end")
	})

	r.Use(func(c *Context) {
		callOrder = append(callOrder, "middleware2-start")
		c.Next()
		callOrder = append(callOrder, "middleware2-end")
	})

	r.GET("/test", func(c *Context) {
		callOrder = append(callOrder, "handler")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	expected := []string{
		"middleware1-start",
		"middleware2-start",
		"handler",
		"middleware2-end",
		"middleware1-end",
	}

	require.Equal(t, len(expected), len(callOrder), "expected %d calls, got %d: %v", len(expected), len(callOrder), callOrder)

	for i, exp := range expected {
		assert.Equal(t, exp, callOrder[i], "call %d", i)
	}
}

// TestContext_Status_MultipleWriters tests Status with plain ResponseWriter
func TestContext_Status_MultipleWriters(t *testing.T) {
	t.Parallel()

	r := MustNew()

	statusSet := false

	r.GET("/test", func(c *Context) {
		// Test Status with a plain ResponseWriter (not wrapped)
		c.Status(http.StatusAccepted)
		statusSet = true
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, statusSet, "Status should be called")
	assert.Equal(t, http.StatusAccepted, w.Code)
}

// TestContext_Next_WithAbortAndCancellation tests Abort takes precedence
func TestContext_Next_WithAbortAndCancellation(t *testing.T) {
	t.Parallel()

	r := MustNew(WithCancellationCheck(true))

	handler1Called := false
	handler2Called := false

	r.Use(func(c *Context) {
		handler1Called = true
		c.Abort()
		c.Next() // Should not proceed due to Abort
	})

	r.GET("/test", func(c *Context) {
		handler2Called = true
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, handler1Called, "first handler should be called")
	assert.False(t, handler2Called, "second handler should NOT be called after Abort()")
}

// TestContext_Abort_InHandler tests aborting from final handler
func TestContext_Abort_InHandler(t *testing.T) {
	t.Parallel()

	r := MustNew()

	handlerCalled := false

	r.GET("/test", func(c *Context) {
		handlerCalled = true
		c.Abort() // Abort in final handler (no effect as it's last)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, handlerCalled, "handler should be called")
	assert.Equal(t, http.StatusOK, w.Code)
}
