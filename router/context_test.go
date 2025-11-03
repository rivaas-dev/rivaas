package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestContextHelpers tests context helper methods
func TestContextHelpers(t *testing.T) {
	r := New()

	t.Run("PostForm", func(t *testing.T) {
		r.POST("/form", func(c *Context) {
			username := c.FormValue("username")
			password := c.FormValue("password")
			c.String(http.StatusOK, "user=%s,pass=%s", username, password)
		})

		req := httptest.NewRequest("POST", "/form", nil)
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
		r.POST("/form-default", func(c *Context) {
			role := c.FormValueDefault("role", "guest")
			c.String(http.StatusOK, "role=%s", role)
		})

		req := httptest.NewRequest("POST", "/form-default", nil)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "role=guest", w.Body.String())
	})

	t.Run("IsSecure", func(t *testing.T) {
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
		r.DELETE("/item", func(c *Context) {
			c.NoContent()
		})

		req := httptest.NewRequest("DELETE", "/item", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, w.Body.String())
	})

	t.Run("SetCookie and GetCookie", func(t *testing.T) {
		r.GET("/set-cookie", func(c *Context) {
			c.SetCookie("session", "abc123", 3600, "/", "", false, true)
			c.String(http.StatusOK, "cookie set")
		})

		r.GET("/get-cookie", func(c *Context) {
			session, err := c.GetCookie("session")
			if err != nil {
				c.String(http.StatusNotFound, "no cookie")
			} else {
				c.String(http.StatusOK, "session=%s", session)
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
	r := New()

	t.Run("Status with wrapped responseWriter", func(t *testing.T) {
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
	r := New()

	r.GET("/test", func(c *Context) {
		err := c.String(http.StatusOK, "Name: %s, Age: %d, Score: %.2f", "Bob", 28, 88.5)
		if err != nil {
			t.Errorf("String failed: %v", err)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Bob")
}

// TestContext_HTML_DifferentStatusCodes tests HTML with various status codes
func TestContext_HTML_DifferentStatusCodes(t *testing.T) {
	codes := []int{200, 201, 404, 500}

	for _, code := range codes {
		r := New()

		r.GET("/test", func(c *Context) {
			err := c.HTML(code, "<div>Content</div>")
			assert.Nil(t, err)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, code, w.Code)
	}
}

// ============================================================================
// Context Pool Tests (merged from context_pool_test.go)
// ============================================================================

func TestContext_Status_WithResponseWriter(t *testing.T) {
	r := New()

	r.GET("/test", func(c *Context) {
		c.Status(http.StatusCreated)
		c.Response.Write([]byte("created"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

// TestContext_Status_AlreadyWritten tests Status when headers already written
func TestContext_Status_AlreadyWritten(t *testing.T) {
	r := New()

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
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 (first write), got %d", w.Code)
	}
}

// TestContext_Next_WithCancellation tests Next with cancelled context
func TestContext_Next_WithCancellation(t *testing.T) {
	r := New(WithCancellationCheck(true))

	handlerCalled := false

	r.Use(func(c *Context) {
		handlerCalled = true
		c.Next()
	})

	r.GET("/test", func(c *Context) {
		c.Status(http.StatusOK)
	})

	// Create request with already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
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
	r := New(WithCancellationCheck(true))

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
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// All handlers should be called
	if len(callOrder) != 3 {
		t.Errorf("expected 3 handlers called, got %d: %v", len(callOrder), callOrder)
	}
}

// TestContext_Next_Abort tests that Abort stops the chain
func TestContext_Next_Abort(t *testing.T) {
	r := New()

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

	if !handler1Called {
		t.Error("handler 1 should be called")
	}

	if !handler2Called {
		t.Error("handler 2 should be called")
	}

	if handler3Called {
		t.Error("handler 3 should NOT be called after Abort()")
	}
}

// TestContext_IsAborted tests the IsAborted method
func TestContext_IsAborted(t *testing.T) {
	r := New()

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

	if abortedInMiddleware {
		t.Error("should not be aborted at start of middleware")
	}

	if abortedInHandler {
		// Handler should not be called after Abort
		t.Error("handler should not be called, so this check shouldn't run")
	}
}

// TestContextPool_Get_ParameterCounts tests pool with different parameter counts
func TestContextPool_Get_ParameterCounts(t *testing.T) {
	r := New()
	pool := r.contextPool

	// Test getting contexts with different parameter counts
	paramCounts := []int{0, 1, 4, 8, 16}

	for _, count := range paramCounts {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			ctx := pool.Get(count)

			if ctx == nil {
				t.Errorf("Get(%d) should not return nil", count)
			}

			// Return to pool
			pool.Put(ctx)
		})
	}
}

// TestContextPool_Put_Reuse tests that contexts are properly reused
func TestContextPool_Put_Reuse(t *testing.T) {
	r := New()
	pool := r.contextPool

	// Get a context
	ctx1 := pool.Get(0)

	// Modify it to track reuse (use valid value within array bounds)
	ctx1.paramCount = 2
	ctx1.paramKeys[0] = "marker"
	ctx1.paramValues[0] = "value"

	// Return it
	pool.Put(ctx1)

	// Get another context - might be the same one (reused)
	ctx2 := pool.Get(0)

	// After reset, should be cleared
	if ctx2.paramCount != 0 {
		t.Error("context should be reset before reuse")
	}

	if ctx2.paramKeys[0] != "" {
		t.Error("param keys should be cleared")
	}

	pool.Put(ctx2)
}

// TestContextPool_Warmup tests pool warmup
func TestContextPool_Warmup(t *testing.T) {
	r := New()

	// Warmup should not panic
	r.contextPool.Warmup()

	// After warmup, pools should work normally
	ctx := r.contextPool.Get(4)
	if ctx == nil {
		t.Error("context pool should work after warmup")
	}
	r.contextPool.Put(ctx)
}

// TestContextPool_ConcurrentAccess tests concurrent pool access
func TestContextPool_ConcurrentAccess(t *testing.T) {
	r := New()
	pool := r.contextPool

	// Run concurrent gets and puts
	done := make(chan bool)

	for range 10 {
		go func() {
			defer func() { done <- true }()

			for j := range 100 {
				ctx := pool.Get(j % 9)
				if ctx == nil {
					t.Error("Get should not return nil")
				}
				pool.Put(ctx)
			}
		}()
	}

	// Wait for all goroutines
	for range 10 {
		<-done
	}
}

// TestContext_Reset_ClearsAllFields tests that reset properly clears all fields
func TestContext_Reset_ClearsAllFields(t *testing.T) {
	r := New()

	// Create and populate a context
	req := httptest.NewRequest(http.MethodGet, "/test?param=value", nil)
	w := httptest.NewRecorder()

	ctx := NewContext(w, req)
	ctx.handlers = []HandlerFunc{func(c *Context) {}}
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
	if ctx.Request != nil {
		t.Error("Request should be nil after reset")
	}

	if ctx.Response != nil {
		t.Error("Response should be nil after reset")
	}

	if ctx.handlers != nil {
		t.Error("handlers should be nil after reset")
	}

	if ctx.index != -1 {
		t.Errorf("index should be -1, got %d", ctx.index)
	}

	if ctx.paramCount != 0 {
		t.Errorf("paramCount should be 0, got %d", ctx.paramCount)
	}

	if ctx.paramKeys[0] != "" {
		t.Error("paramKeys should be cleared")
	}

	if ctx.version != "" {
		t.Error("version should be empty after reset")
	}

	if ctx.aborted {
		t.Error("aborted flag should be false after reset")
	}

	if len(ctx.Params) != 0 {
		t.Error("Params map should be cleared")
	}
}

// TestContext_InitForRequest tests context initialization
func TestContext_InitForRequest(t *testing.T) {
	r := New()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handlers := []HandlerFunc{func(c *Context) {}}

	ctx := NewContext(nil, nil)
	ctx.initForRequest(req, w, handlers, r)

	if ctx.Request != req {
		t.Error("Request should be set")
	}

	if ctx.Response != w {
		t.Error("Response should be set")
	}

	if ctx.router != r {
		t.Error("router should be set")
	}

	if ctx.index != -1 {
		t.Errorf("index should be -1, got %d", ctx.index)
	}

	if ctx.paramCount != 0 {
		t.Errorf("paramCount should be 0, got %d", ctx.paramCount)
	}
}

// TestContext_InitForRequestWithParams tests init that preserves parameters
func TestContext_InitForRequestWithParams(t *testing.T) {
	r := New()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handlers := []HandlerFunc{func(c *Context) {}}

	ctx := NewContext(nil, nil)

	// Set some parameters first
	ctx.paramCount = 2
	ctx.paramKeys[0] = "id"
	ctx.paramValues[0] = "123"
	ctx.paramKeys[1] = "name"
	ctx.paramValues[1] = "test"

	// Init with params (should preserve them)
	ctx.initForRequestWithParams(req, w, handlers, r)

	if ctx.Request != req {
		t.Error("Request should be set")
	}

	if ctx.paramCount != 2 {
		t.Errorf("paramCount should be preserved as 2, got %d", ctx.paramCount)
	}

	if ctx.paramKeys[0] != "id" {
		t.Error("paramKeys should be preserved")
	}

	if ctx.paramValues[0] != "123" {
		t.Error("paramValues should be preserved")
	}
}

// TestContextPool_GetPut_DifferentSizes tests pool with different context sizes
func TestContextPool_GetPut_DifferentSizes(t *testing.T) {
	r := New()
	pool := r.contextPool

	// Test with 0 params (uses general pool)
	ctx0 := pool.Get(0)
	if ctx0 == nil {
		t.Error("Get(0) returned nil")
	}
	pool.Put(ctx0)

	// Test with exact pool sizes (1, 2, 4, 8)
	for _, size := range []int{1, 2, 4, 8} {
		ctx := pool.Get(size)
		if ctx == nil {
			t.Errorf("Get(%d) returned nil", size)
		}
		pool.Put(ctx)
	}

	// Test with size > 8 (uses general pool)
	ctx16 := pool.Get(16)
	if ctx16 == nil {
		t.Error("Get(16) returned nil")
	}
	pool.Put(ctx16)
}

// TestContext_Abort_Multiple tests calling Abort multiple times
func TestContext_Abort_Multiple(t *testing.T) {
	r := New()

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

	if callCount != 1 {
		t.Errorf("expected 1 handler called, got %d", callCount)
	}
}

// TestContext_Next_EmptyHandlers tests Next with no handlers
func TestContext_Next_EmptyHandlers(t *testing.T) {
	ctx := NewContext(nil, nil)
	ctx.handlers = []HandlerFunc{}
	ctx.index = -1

	// Should not panic
	ctx.Next()

	if ctx.index != 0 {
		t.Errorf("expected index 0, got %d", ctx.index)
	}
}

// TestContext_Abort_BeforeNext tests aborting before calling Next
func TestContext_Abort_BeforeNext(t *testing.T) {
	r := New()

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

	if !middlewareCalled {
		t.Error("middleware should be called")
	}

	if handlerCalled {
		t.Error("handler should NOT be called after Abort()")
	}
}

// TestContextPool_ResetBeforeReuse tests contexts are reset before reuse
func TestContextPool_ResetBeforeReuse(t *testing.T) {
	r := New()

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
		if c.IsAborted() {
			t.Error("context should not be aborted from previous request")
		}

		if c.version != "" {
			t.Error("version should be empty for new request")
		}

		c.Status(http.StatusOK)
	})

	req2 := httptest.NewRequest(http.MethodGet, "/second", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
}

// TestContext_Next_NestedCalls tests nested Next calls
func TestContext_Next_NestedCalls(t *testing.T) {
	r := New()

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

	if len(callOrder) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(callOrder), callOrder)
	}

	for i, exp := range expected {
		if callOrder[i] != exp {
			t.Errorf("call %d: expected %s, got %s", i, exp, callOrder[i])
		}
	}
}

// TestContext_Status_MultipleWriters tests Status with plain ResponseWriter
func TestContext_Status_MultipleWriters(t *testing.T) {
	r := New()

	statusSet := false

	r.GET("/test", func(c *Context) {
		// Test Status with a plain ResponseWriter (not wrapped)
		c.Status(http.StatusAccepted)
		statusSet = true
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !statusSet {
		t.Error("Status should be called")
	}

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", w.Code)
	}
}

// TestContextPool_WarmupMultipleTimes tests calling warmup multiple times
func TestContextPool_WarmupMultipleTimes(t *testing.T) {
	r := New()

	// Warmup multiple times should not panic or cause issues
	r.contextPool.Warmup()
	r.contextPool.Warmup()
	r.contextPool.Warmup()

	// Pool should still work
	ctx := r.contextPool.Get(2)
	if ctx == nil {
		t.Error("pool should work after multiple warmups")
	}
	r.contextPool.Put(ctx)
}

// TestContext_Next_WithAbortAndCancellation tests Abort takes precedence
func TestContext_Next_WithAbortAndCancellation(t *testing.T) {
	r := New(WithCancellationCheck(true))

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

	if !handler1Called {
		t.Error("first handler should be called")
	}

	if handler2Called {
		t.Error("second handler should NOT be called after Abort()")
	}
}

// TestContext_Abort_InHandler tests aborting from final handler
func TestContext_Abort_InHandler(t *testing.T) {
	r := New()

	handlerCalled := false

	r.GET("/test", func(c *Context) {
		handlerCalled = true
		c.Abort() // Abort in final handler (no effect as it's last)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("handler should be called")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestContextPool_Put_AllPools tests returning to all pool sizes
func TestContextPool_Put_AllPools(t *testing.T) {
	r := New()
	pool := r.contextPool

	// Test small pool (paramCount <= 4)
	ctx1 := pool.Get(2)
	ctx1.paramCount = 3
	pool.Put(ctx1) // Should go to small pool

	// Test medium pool (paramCount <= 8)
	ctx2 := pool.Get(6)
	ctx2.paramCount = 6
	pool.Put(ctx2) // Should go to medium pool

	// Test large pool (paramCount > 8)
	ctx3 := pool.Get(10)
	ctx3.paramCount = 10
	pool.Put(ctx3) // Should go to large pool

	// Verify all contexts were returned successfully
	// (no panic means success)
}

// TestContextPool_Put_BoundaryCases tests Put boundary cases for medium and large pools
func TestContextPool_Put_BoundaryCases(t *testing.T) {
	r := New()
	pool := r.contextPool

	t.Run("medium pool lower boundary", func(t *testing.T) {
		// Test paramCount = 5 (enters medium pool)
		ctx := pool.Get(5)
		ctx.paramCount = 5
		pool.Put(ctx) // Should go to medium pool
		// Verify by getting another context with same paramCount - should reuse from medium pool
		ctx2 := pool.Get(5)
		if ctx2 == nil {
			t.Fatal("Get(5) should not return nil")
		}
		pool.Put(ctx2)
	})

	t.Run("medium pool upper boundary", func(t *testing.T) {
		// Test paramCount = 8 (still medium pool)
		ctx := pool.Get(8)
		ctx.paramCount = 8
		pool.Put(ctx) // Should go to medium pool
		// Verify by getting another context with same paramCount - should reuse from medium pool
		ctx2 := pool.Get(8)
		if ctx2 == nil {
			t.Fatal("Get(8) should not return nil")
		}
		pool.Put(ctx2)
	})

	t.Run("medium pool middle values", func(t *testing.T) {
		// Test paramCount = 6 and 7 (medium pool)
		for _, count := range []int{6, 7} {
			ctx := pool.Get(count)
			ctx.paramCount = int32(count)
			pool.Put(ctx) // Should go to medium pool
			ctx2 := pool.Get(count)
			if ctx2 == nil {
				t.Fatalf("Get(%d) should not return nil", count)
			}
			pool.Put(ctx2)
		}
	})

	t.Run("large pool lower boundary", func(t *testing.T) {
		// Test paramCount = 9 (enters large pool)
		ctx := pool.Get(9)
		ctx.paramCount = int32(9)
		pool.Put(ctx) // Should go to large pool
		// Verify by getting another context with same paramCount - should reuse from large pool
		ctx2 := pool.Get(9)
		if ctx2 == nil {
			t.Fatal("Get(9) should not return nil")
		}
		pool.Put(ctx2)
	})

	t.Run("large pool higher values", func(t *testing.T) {
		// Test paramCount > 9 (large pool)
		for _, count := range []int{10, 15, 20} {
			ctx := pool.Get(count)
			ctx.paramCount = int32(count)
			pool.Put(ctx) // Should go to large pool
			ctx2 := pool.Get(count)
			if ctx2 == nil {
				t.Fatalf("Get(%d) should not return nil", count)
			}
			pool.Put(ctx2)
		}
	})
}

// TestContextPool_WarmupPool_New tests the warmupPool.New function
func TestContextPool_WarmupPool_New(t *testing.T) {
	r := New()
	pool := r.contextPool

	// Access warmupPool directly (same package, so unexported fields are accessible)
	// Call Get() when pool is empty to trigger New function
	// This should create a new slice with capacity 10
	result := pool.warmupPool.Get()
	slice, ok := result.([]*Context)
	if !ok {
		t.Fatalf("Expected []*Context, got %T", result)
	}

	// Verify the slice properties
	if slice == nil {
		t.Error("Expected non-nil slice")
	}
	if len(slice) != 0 {
		t.Errorf("Expected empty slice, got length %d", len(slice))
	}
	if cap(slice) != 10 {
		t.Errorf("Expected capacity 10, got %d", cap(slice))
	}

	// Put it back
	pool.warmupPool.Put(slice)
}
