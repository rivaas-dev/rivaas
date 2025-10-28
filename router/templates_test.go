package router

import (
	"net/http/httptest"
	"regexp"
	"testing"
)

// BenchmarkTemplateStatic benchmarks static route lookup using templates
func BenchmarkTemplateStatic(b *testing.B) {
	r := New()

	// Register static routes
	r.GET("/api/users", func(c *Context) {
		c.String(200, "users")
	})
	r.GET("/api/posts", func(c *Context) {
		c.String(200, "posts")
	})
	r.GET("/health", func(c *Context) {
		c.String(200, "ok")
	})

	// Warmup to compile templates
	r.WarmupOptimizations()

	req := httptest.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

// BenchmarkTemplateDynamic benchmarks dynamic route lookup using templates
func BenchmarkTemplateDynamic(b *testing.B) {
	r := New()

	// Register routes with parameters
	r.GET("/api/users/:id", func(c *Context) {
		_ = c.JSON(200, map[string]string{"id": c.Param("id")})
	})
	r.GET("/api/users/:id/posts/:pid", func(c *Context) {
		_ = c.JSON(200, map[string]string{"id": c.Param("id"), "pid": c.Param("pid")})
	})

	// Warmup to compile templates
	r.WarmupOptimizations()

	req := httptest.NewRequest("GET", "/api/users/123/posts/456", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

// BenchmarkTemplateWithConstraints benchmarks routes with parameter constraints
func BenchmarkTemplateWithConstraints(b *testing.B) {
	r := New()

	// Register route with constraints
	r.GET("/api/users/:id", func(c *Context) {
		_ = c.JSON(200, map[string]string{"id": c.Param("id")})
	}).Where("id", `^\d+$`)

	// Warmup to compile templates
	r.WarmupOptimizations()

	req := httptest.NewRequest("GET", "/api/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

// BenchmarkTemplateVsTree compares template matching vs tree traversal
func BenchmarkTemplateVsTree(b *testing.B) {
	b.Run("WithTemplates", func(b *testing.B) {
		r := New(WithTemplateRouting(true))

		r.GET("/api/users/:id/posts/:pid/comments/:cid", func(c *Context) {
			_ = c.JSON(200, map[string]string{"status": "ok"})
		})

		r.WarmupOptimizations()

		req := httptest.NewRequest("GET", "/api/users/123/posts/456/comments/789", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, req)
		}
	})

	b.Run("WithoutTemplates", func(b *testing.B) {
		r := New(WithTemplateRouting(false))

		r.GET("/api/users/:id/posts/:pid/comments/:cid", func(c *Context) {
			_ = c.JSON(200, map[string]string{"status": "ok"})
		})

		r.WarmupOptimizations()

		req := httptest.NewRequest("GET", "/api/users/123/posts/456/comments/789", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, req)
		}
	})
}

// BenchmarkTemplateCompilation benchmarks the template compilation process
func BenchmarkTemplateCompilation(b *testing.B) {
	constraints := []RouteConstraint{
		{Param: "id", Pattern: regexp.MustCompile(`^\d+$`)},
	}

	handlers := []HandlerFunc{
		func(c *Context) {
			c.String(200, "ok")
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = compileRouteTemplate("GET", "/api/users/:id/posts/:pid", handlers, constraints)
	}
}

// BenchmarkTemplateMatching benchmarks just the template matching logic
func BenchmarkTemplateMatching(b *testing.B) {
	tmpl := compileRouteTemplate("GET", "/api/users/:id/posts/:pid", []HandlerFunc{
		func(c *Context) {},
	}, nil)

	ctx := &Context{}
	path := "/api/users/123/posts/456"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx.paramCount = 0
		tmpl.matchAndExtract(path, ctx)
	}
}

// TestTemplateMatching tests template matching correctness
func TestTemplateMatching(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		path        string
		shouldMatch bool
		params      map[string]string
	}{
		{
			name:        "Simple parameter",
			pattern:     "/users/:id",
			path:        "/users/123",
			shouldMatch: true,
			params:      map[string]string{"id": "123"},
		},
		{
			name:        "Multiple parameters",
			pattern:     "/users/:id/posts/:pid",
			path:        "/users/123/posts/456",
			shouldMatch: true,
			params:      map[string]string{"id": "123", "pid": "456"},
		},
		{
			name:        "Mixed static and parameters",
			pattern:     "/api/v1/users/:id/posts/:pid",
			path:        "/api/v1/users/123/posts/456",
			shouldMatch: true,
			params:      map[string]string{"id": "123", "pid": "456"},
		},
		{
			name:        "Segment count mismatch",
			pattern:     "/users/:id",
			path:        "/users/123/extra",
			shouldMatch: false,
		},
		{
			name:        "Static segment mismatch",
			pattern:     "/users/:id",
			path:        "/posts/123",
			shouldMatch: false,
		},
		{
			name:        "Root path",
			pattern:     "/",
			path:        "/",
			shouldMatch: true,
			params:      map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := compileRouteTemplate("GET", tt.pattern, []HandlerFunc{func(c *Context) {}}, nil)
			ctx := &Context{}

			matched := tmpl.matchAndExtract(tt.path, ctx)

			if matched != tt.shouldMatch {
				t.Errorf("Expected match=%v, got %v", tt.shouldMatch, matched)
			}

			if matched && tt.params != nil {
				for key, expectedValue := range tt.params {
					actualValue := ctx.Param(key)
					if actualValue != expectedValue {
						t.Errorf("Param %q: expected %q, got %q", key, expectedValue, actualValue)
					}
				}
			}
		})
	}
}

// TestTemplateWithConstraints tests template matching with constraints
func TestTemplateWithConstraints(t *testing.T) {
	constraints := []RouteConstraint{
		{Param: "id", Pattern: regexp.MustCompile(`^\d+$`)},
	}

	tmpl := compileRouteTemplate("GET", "/users/:id", []HandlerFunc{func(c *Context) {}}, constraints)

	tests := []struct {
		path        string
		shouldMatch bool
	}{
		{"/users/123", true},
		{"/users/abc", false},   // Constraint violation
		{"/users/12abc", false}, // Constraint violation
	}

	for _, tt := range tests {
		ctx := &Context{}
		matched := tmpl.matchAndExtract(tt.path, ctx)
		if matched != tt.shouldMatch {
			t.Errorf("Path %q: expected match=%v, got %v", tt.path, tt.shouldMatch, matched)
		}
	}
}
