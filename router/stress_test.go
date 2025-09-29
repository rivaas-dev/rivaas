package router

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestRouterStress tests the router under high concurrent load
func TestRouterStress(t *testing.T) {
	r := New()

	// Add many routes to test scalability
	for i := 0; i < 100; i++ {
		route := "/api/v1/users/" + string(rune('a'+i%26))
		r.GET(route, func(c *Context) {
			c.String(http.StatusOK, "User")
		})
	}

	// Add parameter routes
	for i := 0; i < 50; i++ {
		route := "/api/v1/users/:id/posts/:post_id"
		r.GET(route, func(c *Context) {
			c.String(http.StatusOK, "User: %s, Post: %s", c.Param("id"), c.Param("post_id"))
		})
	}

	// Test concurrent requests
	var wg sync.WaitGroup
	concurrency := 100
	requests := 1000

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requests/concurrency; j++ {
				req := httptest.NewRequest("GET", "/api/v1/users/123/posts/456", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Errorf("Expected status 200, got %d", w.Code)
				}
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	t.Logf("Processed %d requests with %d concurrent goroutines in %v",
		requests, concurrency, duration)
	t.Logf("Average: %v per request", duration/time.Duration(requests))
	t.Logf("Throughput: %.2f requests/second",
		float64(requests)/duration.Seconds())
}

// BenchmarkRouterConcurrent benchmarks concurrent requests
func BenchmarkRouterConcurrent(b *testing.B) {
	r := New()
	r.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "User: %s", c.Param("id"))
	})

	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest("GET", "/users/123", nil)
		w := httptest.NewRecorder()

		for pb.Next() {
			r.ServeHTTP(w, req)
		}
	})
}

// BenchmarkRouterMemoryAllocations tests memory efficiency
func BenchmarkRouterMemoryAllocations(b *testing.B) {
	r := New()
	r.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "User: %s", c.Param("id"))
	})

	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}
