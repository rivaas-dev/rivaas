package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// BenchmarkAtomicRouteRegistration benchmarks atomic route registration performance
func BenchmarkAtomicRouteRegistration(b *testing.B) {
	r := New()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			path := "/benchmark" + string(rune('0'+i%10)) + "/" + string(rune('0'+i%100))
			r.GET(path, func(c *Context) {
				c.String(http.StatusOK, "OK")
			})
			i++
		}
	})
}

// BenchmarkAtomicRouteLookup benchmarks the performance of atomic route lookup
func BenchmarkAtomicRouteLookup(b *testing.B) {
	r := New()

	// Register test routes
	routes := []string{
		"/",
		"/users",
		"/users/:id",
		"/users/:id/posts",
		"/users/:id/posts/:post_id",
		"/posts",
		"/posts/:id",
		"/api/v1/users",
		"/api/v1/users/:id",
		"/api/v1/posts",
		"/api/v1/posts/:id",
	}

	for _, route := range routes {
		r.GET(route, func(c *Context) {
			c.String(http.StatusOK, "OK")
		})
	}

	// Test paths
	testPaths := []string{
		"/",
		"/users",
		"/users/123",
		"/users/123/posts",
		"/users/123/posts/456",
		"/posts",
		"/posts/123",
		"/api/v1/users",
		"/api/v1/users/123",
		"/api/v1/posts",
		"/api/v1/posts/123",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, path := range testPaths {
				req := httptest.NewRequest("GET", path, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
			}
		}
	})
}

// BenchmarkConcurrentRegistrationAndLookup benchmarks concurrent route registration and lookup
func BenchmarkConcurrentRegistrationAndLookup(b *testing.B) {
	r := New()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Alternate between registration and lookup
			if i%2 == 0 {
				// Register a route
				path := "/concurrent" + string(rune('0'+i%10)) + "/" + string(rune('0'+i%100))
				r.GET(path, func(c *Context) {
					c.String(http.StatusOK, "OK")
				})
			} else {
				// Lookup a route
				req := httptest.NewRequest("GET", "/", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
			}
			i++
		}
	})
}
