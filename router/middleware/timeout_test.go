package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rivaas.dev/router"
)

func TestTimeout_CompletesWithinTimeout(t *testing.T) {
	r := router.New()
	r.Use(Timeout(100 * time.Millisecond))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestTimeout_ExceedsTimeout(t *testing.T) {
	r := router.New()
	r.Use(Timeout(50 * time.Millisecond))
	r.GET("/slow", func(c *router.Context) {
		// Properly respect context cancellation
		select {
		case <-time.After(200 * time.Millisecond):
			c.JSON(http.StatusOK, map[string]string{"message": "ok"})
		case <-c.Request.Context().Done():
			// Context cancelled due to timeout - don't write response
			return
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestTimeout {
		t.Errorf("Expected status 408, got %d", w.Code)
	}
}

func TestTimeout_RespectsContextCancellation(t *testing.T) {
	r := router.New()
	r.Use(Timeout(50 * time.Millisecond))

	contextCancelled := make(chan bool, 1)
	r.GET("/test", func(c *router.Context) {
		// Capture context at start
		ctx := c.Request.Context()

		// Check context multiple times during a long operation
		for i := 0; i < 10; i++ {
			select {
			case <-ctx.Done():
				contextCancelled <- true
				return
			default:
				time.Sleep(20 * time.Millisecond)
			}
		}
		// Only write response if not cancelled
		select {
		case <-ctx.Done():
			contextCancelled <- true
			return
		default:
			c.JSON(http.StatusOK, map[string]string{"message": "ok"})
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Wait for handler to signal cancellation or timeout
	select {
	case <-contextCancelled:
		// Good - context was cancelled
	case <-time.After(200 * time.Millisecond):
		t.Error("Handler should detect context cancellation")
	}

	if w.Code != http.StatusRequestTimeout {
		t.Errorf("Expected status 408, got %d", w.Code)
	}
}

func TestTimeout_SkipPaths(t *testing.T) {
	r := router.New()
	r.Use(Timeout(50*time.Millisecond, WithTimeoutSkipPaths([]string{"/long-running"})))

	r.GET("/long-running", func(c *router.Context) {
		time.Sleep(100 * time.Millisecond)
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	r.GET("/fast", func(c *router.Context) {
		// Respect context cancellation
		select {
		case <-time.After(100 * time.Millisecond):
			c.JSON(http.StatusOK, map[string]string{"message": "ok"})
		case <-c.Request.Context().Done():
			return
		}
	})

	// Skipped path should complete even if slow
	req := httptest.NewRequest(http.MethodGet, "/long-running", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for skipped path, got %d", w.Code)
	}

	// Non-skipped path should timeout
	req = httptest.NewRequest(http.MethodGet, "/fast", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestTimeout {
		t.Errorf("Expected status 408 for non-skipped path, got %d", w.Code)
	}
}

func TestTimeout_CustomHandler(t *testing.T) {
	customHandlerCalled := false

	r := router.New()
	r.Use(Timeout(30*time.Millisecond,
		WithTimeoutHandler(func(c *router.Context) {
			customHandlerCalled = true
			c.JSON(http.StatusRequestTimeout, map[string]any{
				"error":   "Custom timeout message",
				"timeout": "30ms",
			})
		}),
	))

	r.GET("/slow", func(c *router.Context) {
		// Respect context cancellation
		select {
		case <-time.After(150 * time.Millisecond):
			c.JSON(http.StatusOK, map[string]string{"message": "ok"})
		case <-c.Request.Context().Done():
			return
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Give enough time for timeout to trigger
	time.Sleep(50 * time.Millisecond)

	if !customHandlerCalled {
		t.Error("Custom timeout handler should be called")
	}

	if w.Code != http.StatusRequestTimeout {
		t.Errorf("Expected status 408, got %d", w.Code)
	}
}

func TestTimeout_ContextPropagation(t *testing.T) {
	r := router.New()
	r.Use(Timeout(100 * time.Millisecond))

	var ctxWithTimeout context.Context
	r.GET("/test", func(c *router.Context) {
		ctxWithTimeout = c.Request.Context()
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if ctxWithTimeout == nil {
		t.Fatal("Context should be set")
	}

	// Check that context has deadline
	if _, ok := ctxWithTimeout.Deadline(); !ok {
		t.Error("Context should have deadline set")
	}
}

func TestTimeout_MultipleRequests(t *testing.T) {
	r := router.New()
	r.Use(Timeout(100 * time.Millisecond))

	fastPath := func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "fast"})
	}

	slowPath := func(c *router.Context) {
		// Respect context cancellation
		select {
		case <-time.After(200 * time.Millisecond):
			c.JSON(http.StatusOK, map[string]string{"message": "slow"})
		case <-c.Request.Context().Done():
			return
		}
	}

	r.GET("/fast", fastPath)
	r.GET("/slow", slowPath)

	// Test fast request
	req := httptest.NewRequest(http.MethodGet, "/fast", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Fast request: expected status 200, got %d", w.Code)
	}

	// Test slow request
	req = httptest.NewRequest(http.MethodGet, "/slow", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestTimeout {
		t.Errorf("Slow request: expected status 408, got %d", w.Code)
	}
}

// Benchmark tests
func BenchmarkTimeout_NoTimeout(b *testing.B) {
	r := router.New()
	r.Use(Timeout(1 * time.Second))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkTimeout_WithContextCheck(b *testing.B) {
	r := router.New()
	r.Use(Timeout(1 * time.Second))
	r.GET("/test", func(c *router.Context) {
		// Simulate handler checking context
		select {
		case <-c.Request.Context().Done():
			return
		default:
			c.JSON(http.StatusOK, map[string]string{"message": "ok"})
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
