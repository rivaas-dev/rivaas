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
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkRouter(b *testing.B) {
	r := MustNew()

	routes := []string{
		"/",
		"/users",
		"/users/:id",
		"/users/:id/posts",
		"/users/:id/posts/:post_id",
		"/users/:id/posts/:post_id/comments",
		"/users/:id/posts/:post_id/comments/:comment_id",
		"/posts",
		"/posts/:id",
		"/posts/:id/comments",
		"/posts/:id/comments/:comment_id",
		"/api/v1/users",
		"/api/v1/users/:id",
		"/api/v1/posts",
		"/api/v1/posts/:id",
		"/api/v2/users",
		"/api/v2/posts",
		"/admin/users",
		"/admin/posts",
		"/admin/settings",
	}

	for _, route := range routes {
		r.GET(route, func(c *Context) {
			c.String(http.StatusOK, "OK")
		})
	}

	r.Warmup()

	// Test paths
	testPaths := []string{
		"/",
		"/users",
		"/users/123",
		"/users/123/posts",
		"/users/123/posts/456",
		"/users/123/posts/456/comments",
		"/users/123/posts/456/comments/789",
		"/posts",
		"/posts/123",
		"/posts/123/comments",
		"/posts/123/comments/456",
		"/api/v1/users",
		"/api/v1/users/123",
		"/api/v1/posts",
		"/api/v1/posts/123",
		"/api/v2/users",
		"/api/v2/posts",
		"/admin/users",
		"/admin/posts",
		"/admin/settings",
	}

	b.ResetTimer()
	b.ReportAllocs()
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

func BenchmarkRouterWithMiddleware(b *testing.B) {
	r := MustNew()

	// Add middleware
	r.Use(func(c *Context) {
		c.Next()
	})

	// Add routes
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
	b.ReportAllocs()
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

func BenchmarkRouterGroup(b *testing.B) {
	r := MustNew()

	// Create groups
	api := r.Group("/api/v1")
	api.GET("/users", func(c *Context) {
		c.String(http.StatusOK, "OK")
	})
	api.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "OK")
	})
	api.GET("/posts", func(c *Context) {
		c.String(http.StatusOK, "OK")
	})
	api.GET("/posts/:id", func(c *Context) {
		c.String(http.StatusOK, "OK")
	})

	admin := r.Group("/admin")
	admin.GET("/users", func(c *Context) {
		c.String(http.StatusOK, "OK")
	})
	admin.GET("/posts", func(c *Context) {
		c.String(http.StatusOK, "OK")
	})

	// Test paths
	testPaths := []string{
		"/api/v1/users",
		"/api/v1/users/123",
		"/api/v1/posts",
		"/api/v1/posts/123",
		"/admin/users",
		"/admin/posts",
	}

	b.ResetTimer()
	b.ReportAllocs()
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

func BenchmarkRadixTree(b *testing.B) {
	root := &node{}

	// Add routes
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
		root.addRouteWithConstraints(route, []HandlerFunc{func(_ *Context) {}}, nil)
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
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		ctx := &Context{}
		for pb.Next() {
			for _, path := range testPaths {
				ctx.paramCount = 0 // Reset context for reuse
				_, _ = root.getRoute(path, ctx)
			}
		}
	})
}

// BenchmarkStaticRoutes tests compiled static routes.
func BenchmarkStaticRoutes(b *testing.B) {
	r := MustNew()

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

	r.Warmup()

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
	b.ReportAllocs()
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

// BenchmarkContextPool tests context pooling.
func BenchmarkContextPool(b *testing.B) {
	r := MustNew()

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

	r.Warmup()

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
	b.ReportAllocs()
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

// BenchmarkMemoryUsage tests memory usage.
func BenchmarkMemoryUsage(b *testing.B) {
	r := MustNew()

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

	r.Warmup()

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
	b.ReportAllocs()
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

// BenchmarkBloomFilter tests bloom filter for negative lookups.
func BenchmarkBloomFilter(b *testing.B) {
	r := MustNew()

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

	r.Warmup()

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
	b.ReportAllocs()
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

func BenchmarkAtomicRouteRegistration(b *testing.B) {
	r := MustNew()

	b.ResetTimer()
	b.ReportAllocs()
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

// BenchmarkAtomicRouteLookup benchmarks atomic route lookup.
func BenchmarkAtomicRouteLookup(b *testing.B) {
	r := MustNew()

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
	b.ReportAllocs()
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

// BenchmarkConcurrentRegistrationAndLookup benchmarks concurrent route registration and lookup.
func BenchmarkConcurrentRegistrationAndLookup(b *testing.B) {
	r := MustNew()

	b.ResetTimer()
	b.ReportAllocs()
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
