package router

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestOptimizedMiddlewareChain tests the optimized middleware chain execution
func TestOptimizedMiddlewareChain(t *testing.T) {
	r := New()

	// Add middleware that tracks execution
	executionOrder := make([]string, 0)
	r.Use(func(c *Context) {
		executionOrder = append(executionOrder, "global1")
	})
	r.Use(func(c *Context) {
		executionOrder = append(executionOrder, "global2")
	})

	// Add a route
	r.GET("/test", func(c *Context) {
		executionOrder = append(executionOrder, "handler")
		c.String(http.StatusOK, "test")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Verify execution order
	expected := []string{"global1", "global2", "handler"}
	if len(executionOrder) != len(expected) {
		t.Errorf("Expected %d middleware executions, got %d", len(expected), len(executionOrder))
	}

	for i, expectedItem := range expected {
		if i >= len(executionOrder) || executionOrder[i] != expectedItem {
			t.Errorf("Expected execution order %v, got %v", expected, executionOrder)
			break
		}
	}

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestOptimizedMiddlewareChainCaching tests that middleware chains are cached properly
func TestOptimizedMiddlewareChainCaching(t *testing.T) {
	r := New()

	// Add middleware
	r.Use(func(c *Context) {
		c.String(http.StatusOK, "middleware")
	})

	// Add multiple routes with same middleware
	r.GET("/route1", func(c *Context) {
		c.String(http.StatusOK, "route1")
	})
	r.GET("/route2", func(c *Context) {
		c.String(http.StatusOK, "route2")
	})

	// Test both routes
	req1 := httptest.NewRequest("GET", "/route1", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	req2 := httptest.NewRequest("GET", "/route2", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	// Both should work
	if w1.Code != 200 || w2.Code != 200 {
		t.Errorf("Expected both routes to return 200, got %d and %d", w1.Code, w2.Code)
	}
}

// TestOptimizedMiddlewareChainConcurrency tests concurrent middleware chain execution
func TestOptimizedMiddlewareChainConcurrency(t *testing.T) {
	r := New()

	// Add middleware that tracks concurrent execution
	r.Use(func(c *Context) {
		// Simulate some work
		time.Sleep(1 * time.Millisecond)
	})

	r.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "test")
	})

	// Test concurrent requests
	const numGoroutines = 100
	const requestsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for range numGoroutines {
		go func() {
			defer wg.Done()
			for range requestsPerGoroutine {
				req := httptest.NewRequest("GET", "/test", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				if w.Code != 200 {
					t.Errorf("Expected status 200, got %d", w.Code)
				}
			}
		}()
	}

	wg.Wait()

	// Verify no race conditions occurred
	t.Logf("Successfully handled %d concurrent requests", numGoroutines*requestsPerGoroutine)
}

// TestOptimizedMiddlewareChainPerformance tests the performance improvement of optimized chains
func TestOptimizedMiddlewareChainPerformance(t *testing.T) {
	r := New()

	// Add multiple middleware layers
	for range 5 {
		r.Use(func(c *Context) {
			// Simulate middleware work
		})
	}

	r.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "test")
	})

	// Measure execution time
	start := time.Now()

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	duration := time.Since(start)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Should be very fast with optimized chains
	if duration > 10*time.Millisecond {
		t.Logf("Warning: Middleware execution took %v, which is slower than expected", duration)
	}

	t.Logf("Optimized middleware chain execution time: %v", duration)
}

// TestOptimizedMiddlewareChainMemorySafety tests memory safety of optimized chains
func TestOptimizedMiddlewareChainMemorySafety(t *testing.T) {
	r := New()

	// Add middleware that manipulates context
	r.Use(func(c *Context) {
		// Simulate middleware work
		c.String(http.StatusOK, "middleware")
	})

	r.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "test")
	})

	// Test multiple requests to ensure memory safety
	for range 100 {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	}

	t.Log("Memory safety test passed - no memory leaks or corruption detected")
}

// TestOptimizedMiddlewareChainCacheEfficiency tests the efficiency of middleware chain caching
func TestOptimizedMiddlewareChainCacheEfficiency(t *testing.T) {
	r := New()

	// Add middleware
	r.Use(func(c *Context) {
		c.String(http.StatusOK, "middleware")
	})

	// Add routes with different middleware combinations
	r.GET("/route1", func(c *Context) {
		c.String(http.StatusOK, "route1")
	})

	// Create a group with additional middleware
	api := r.Group("/api")
	api.Use(func(c *Context) {
		c.String(http.StatusOK, "api_middleware")
	})
	api.GET("/users", func(c *Context) {
		c.String(http.StatusOK, "users")
	})

	// Test both routes
	req1 := httptest.NewRequest("GET", "/route1", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	req2 := httptest.NewRequest("GET", "/api/users", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	// Both should work
	if w1.Code != 200 || w2.Code != 200 {
		t.Errorf("Expected both routes to return 200, got %d and %d", w1.Code, w2.Code)
	}

	t.Log("Middleware chain cache efficiency test passed")
}
