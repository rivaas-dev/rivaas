package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// BenchmarkOptimizedRouter tests the router with all performance optimizations
func BenchmarkOptimizedRouter(b *testing.B) {
	r := New()

	// Add many routes to test performance
	routes := []string{
		"/",
		"/health",
		"/status",
		"/metrics",
		"/api",
		"/api/v1",
		"/api/v2",
		"/users",
		"/users/profile",
		"/users/settings",
		"/posts",
		"/posts/latest",
		"/posts/popular",
		"/admin",
		"/admin/users",
		"/admin/posts",
		"/admin/settings",
		"/auth",
		"/auth/login",
		"/auth/logout",
		"/auth/register",
		"/static",
		"/static/css",
		"/static/js",
		"/static/images",
		"/docs",
		"/docs/api",
		"/docs/guide",
		"/help",
		"/help/faq",
		"/help/contact",
	}

	for _, route := range routes {
		r.GET(route, func(c *Context) {
			c.String(http.StatusOK, "OK")
		})
	}

	// Warm up all optimizations for maximum performance
	r.WarmupOptimizations()

	// Test paths (mix of static and dynamic)
	testPaths := []string{
		"/",
		"/health",
		"/status",
		"/metrics",
		"/api",
		"/api/v1",
		"/api/v2",
		"/users",
		"/users/profile",
		"/users/settings",
		"/posts",
		"/posts/latest",
		"/posts/popular",
		"/admin",
		"/admin/users",
		"/admin/posts",
		"/admin/settings",
		"/auth",
		"/auth/login",
		"/auth/logout",
		"/auth/register",
		"/static",
		"/static/css",
		"/static/js",
		"/static/images",
		"/docs",
		"/docs/api",
		"/docs/guide",
		"/help",
		"/help/faq",
		"/help/contact",
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

// BenchmarkCompiledRoutes tests the performance of compiled static routes
func BenchmarkCompiledRoutes(b *testing.B) {
	r := New()

	// Add static routes only
	staticRoutes := []string{
		"/",
		"/health",
		"/status",
		"/metrics",
		"/api",
		"/api/v1",
		"/api/v2",
		"/users",
		"/users/profile",
		"/users/settings",
		"/posts",
		"/posts/latest",
		"/posts/popular",
		"/admin",
		"/admin/users",
		"/admin/posts",
		"/admin/settings",
		"/auth",
		"/auth/login",
		"/auth/logout",
		"/auth/register",
		"/static",
		"/static/css",
		"/static/js",
		"/static/images",
		"/docs",
		"/docs/api",
		"/docs/guide",
		"/help",
		"/help/faq",
		"/help/contact",
	}

	for _, route := range staticRoutes {
		r.GET(route, func(c *Context) {
			c.String(http.StatusOK, "OK")
		})
	}

	// Warm up all optimizations for maximum performance
	r.WarmupOptimizations()

	// Test paths
	testPaths := []string{
		"/",
		"/health",
		"/status",
		"/metrics",
		"/api",
		"/api/v1",
		"/api/v2",
		"/users",
		"/users/profile",
		"/users/settings",
		"/posts",
		"/posts/latest",
		"/posts/popular",
		"/admin",
		"/admin/users",
		"/admin/posts",
		"/admin/settings",
		"/auth",
		"/auth/login",
		"/auth/logout",
		"/auth/register",
		"/static",
		"/static/css",
		"/static/js",
		"/static/images",
		"/docs",
		"/docs/api",
		"/docs/guide",
		"/help",
		"/help/faq",
		"/help/contact",
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

// BenchmarkEnhancedContextPool tests the performance of enhanced context pooling
func BenchmarkEnhancedContextPool(b *testing.B) {
	r := New()

	// Add routes with different parameter counts
	routes := []string{
		"/",                                  // 0 params
		"/users",                             // 0 params
		"/users/:id",                         // 1 param
		"/users/:id/posts",                   // 1 param
		"/users/:id/posts/:post_id",          // 2 params
		"/users/:id/posts/:post_id/comments", // 2 params
		"/users/:id/posts/:post_id/comments/:comment_id",                          // 3 params
		"/api/v1/users/:id/posts/:post_id/comments/:comment_id/replies/:reply_id", // 4 params
	}

	for _, route := range routes {
		r.GET(route, func(c *Context) {
			c.String(http.StatusOK, "OK")
		})
	}

	// Warm up all optimizations for maximum performance
	r.WarmupOptimizations()

	// Test paths with different parameter counts
	testPaths := []string{
		"/",
		"/users",
		"/users/123",
		"/users/123/posts",
		"/users/123/posts/456",
		"/users/123/posts/456/comments",
		"/users/123/posts/456/comments/789",
		"/api/v1/users/123/posts/456/comments/789/replies/101",
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

// BenchmarkMemoryOptimizations tests memory usage with optimizations
func BenchmarkMemoryOptimizations(b *testing.B) {
	r := New()

	// Add many routes to test memory efficiency
	routes := []string{
		"/",
		"/health",
		"/status",
		"/metrics",
		"/api",
		"/api/v1",
		"/api/v2",
		"/users",
		"/users/profile",
		"/users/settings",
		"/posts",
		"/posts/latest",
		"/posts/popular",
		"/admin",
		"/admin/users",
		"/admin/posts",
		"/admin/settings",
		"/auth",
		"/auth/login",
		"/auth/logout",
		"/auth/register",
		"/static",
		"/static/css",
		"/static/js",
		"/static/images",
		"/docs",
		"/docs/api",
		"/docs/guide",
		"/help",
		"/help/faq",
		"/help/contact",
		"/users/:id",
		"/users/:id/posts",
		"/users/:id/posts/:post_id",
		"/posts/:id",
		"/posts/:id/comments",
		"/posts/:id/comments/:comment_id",
	}

	for _, route := range routes {
		r.GET(route, func(c *Context) {
			c.String(http.StatusOK, "OK")
		})
	}

	// Warm up all optimizations for maximum performance
	r.WarmupOptimizations()

	// Test paths
	testPaths := []string{
		"/",
		"/health",
		"/status",
		"/metrics",
		"/api",
		"/api/v1",
		"/api/v2",
		"/users",
		"/users/profile",
		"/users/settings",
		"/posts",
		"/posts/latest",
		"/posts/popular",
		"/admin",
		"/admin/users",
		"/admin/posts",
		"/admin/settings",
		"/auth",
		"/auth/login",
		"/auth/logout",
		"/auth/register",
		"/static",
		"/static/css",
		"/static/js",
		"/static/images",
		"/docs",
		"/docs/api",
		"/docs/guide",
		"/help",
		"/help/faq",
		"/help/contact",
		"/users/123",
		"/users/123/posts",
		"/users/123/posts/456",
		"/posts/123",
		"/posts/123/comments",
		"/posts/123/comments/456",
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

// BenchmarkBloomFilter tests the performance of bloom filter for negative lookups
func BenchmarkBloomFilter(b *testing.B) {
	r := New()

	// Add many static routes
	routes := []string{
		"/",
		"/health",
		"/status",
		"/metrics",
		"/api",
		"/api/v1",
		"/api/v2",
		"/users",
		"/users/profile",
		"/users/settings",
		"/posts",
		"/posts/latest",
		"/posts/popular",
		"/admin",
		"/admin/users",
		"/admin/posts",
		"/admin/settings",
		"/auth",
		"/auth/login",
		"/auth/logout",
		"/auth/register",
		"/static",
		"/static/css",
		"/static/js",
		"/static/images",
		"/docs",
		"/docs/api",
		"/docs/guide",
		"/help",
		"/help/faq",
		"/help/contact",
	}

	for _, route := range routes {
		r.GET(route, func(c *Context) {
			c.String(http.StatusOK, "OK")
		})
	}

	// Warm up all optimizations for maximum performance
	r.WarmupOptimizations()

	// Test paths (mix of existing and non-existing routes)
	testPaths := []string{
		"/",               // exists
		"/health",         // exists
		"/status",         // exists
		"/metrics",        // exists
		"/api",            // exists
		"/api/v1",         // exists
		"/api/v2",         // exists
		"/users",          // exists
		"/users/profile",  // exists
		"/users/settings", // exists
		"/posts",          // exists
		"/posts/latest",   // exists
		"/posts/popular",  // exists
		"/admin",          // exists
		"/admin/users",    // exists
		"/admin/posts",    // exists
		"/admin/settings", // exists
		"/auth",           // exists
		"/auth/login",     // exists
		"/auth/logout",    // exists
		"/auth/register",  // exists
		"/static",         // exists
		"/static/css",     // exists
		"/static/js",      // exists
		"/static/images",  // exists
		"/docs",           // exists
		"/docs/api",       // exists
		"/docs/guide",     // exists
		"/help",           // exists
		"/help/faq",       // exists
		"/help/contact",   // exists
		"/nonexistent",    // doesn't exist
		"/api/v3",         // doesn't exist
		"/users/123",      // doesn't exist
		"/posts/123",      // doesn't exist
		"/admin/123",      // doesn't exist
		"/auth/123",       // doesn't exist
		"/static/123",     // doesn't exist
		"/docs/123",       // doesn't exist
		"/help/123",       // doesn't exist
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
