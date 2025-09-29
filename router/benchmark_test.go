package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkRouter(b *testing.B) {
	r := New()

	// Add many routes to test performance
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
	r := New()

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
	r := New()

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
		root.addRoute(route, []HandlerFunc{func(c *Context) {}})
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
		ctx := &Context{}
		for pb.Next() {
			for _, path := range testPaths {
				ctx.paramCount = 0 // Reset context for reuse
				root.getRoute(path, ctx)
			}
		}
	})
}
