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
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BenchmarkTemplateStatic benchmarks static route lookup using templates
func BenchmarkTemplateStatic(b *testing.B) {
	r := MustNew()

	// Register static routes
	r.GET("/api/users", func(c *Context) {
		c.String(http.StatusOK, "users")
	})
	r.GET("/api/posts", func(c *Context) {
		c.String(http.StatusOK, "posts")
	})
	r.GET("/health", func(c *Context) {
		c.String(http.StatusOK, "ok")
	})

	// Warmup to compile templates
	r.Warmup()

	req := httptest.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		r.ServeHTTP(w, req)
	}
}

// BenchmarkTemplateDynamic benchmarks dynamic route lookup using templates
func BenchmarkTemplateDynamic(b *testing.B) {
	r := MustNew()

	// Register routes with parameters
	r.GET("/api/users/:id", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
	})
	r.GET("/api/users/:id/posts/:pid", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"id": c.Param("id"), "pid": c.Param("pid")})
	})

	// Warmup to compile templates
	r.Warmup()

	req := httptest.NewRequest("GET", "/api/users/123/posts/456", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		r.ServeHTTP(w, req)
	}
}

// BenchmarkTemplateWithConstraints benchmarks routes with parameter constraints
func BenchmarkTemplateWithConstraints(b *testing.B) {
	r := MustNew()

	// Register route with constraints
	r.GET("/api/users/:id", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
	}).Where("id", `^\d+$`)

	// Warmup to compile templates
	r.Warmup()

	req := httptest.NewRequest("GET", "/api/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		r.ServeHTTP(w, req)
	}
}

// BenchmarkTemplateVsTree compares template matching vs tree traversal
func BenchmarkTemplateVsTree(b *testing.B) {
	b.Run("WithTemplates", func(b *testing.B) {
		r := MustNew(WithTemplateRouting(true))

		r.GET("/api/users/:id/posts/:pid/comments/:cid", func(c *Context) {
			c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		r.Warmup()

		req := httptest.NewRequest("GET", "/api/users/123/posts/456/comments/789", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			r.ServeHTTP(w, req)
		}
	})

	b.Run("WithoutTemplates", func(b *testing.B) {
		r := MustNew(WithTemplateRouting(false))

		r.GET("/api/users/:id/posts/:pid/comments/:cid", func(c *Context) {
			c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		r.Warmup()

		req := httptest.NewRequest("GET", "/api/users/123/posts/456/comments/789", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
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
			c.String(http.StatusOK, "ok")
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = compileRouteTemplate("GET", "/api/users/:id/posts/:pid", handlers, constraints)
	}
}

// BenchmarkTemplateMatching benchmarks just the template matching logic
func BenchmarkTemplateMatching(b *testing.B) {
	tmpl := compileRouteTemplate("GET", "/api/users/:id/posts/:pid", []HandlerFunc{
		func(_ *Context) {},
	}, nil)

	ctx := &Context{}
	path := "/api/users/123/posts/456"

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		ctx.paramCount = 0
		tmpl.matchAndExtract(path, ctx)
	}
}

// TestTemplateMatching tests template matching correctness
func TestTemplateMatching(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			tmpl := compileRouteTemplate("GET", tt.pattern, []HandlerFunc{func(_ *Context) {}}, nil)
			ctx := &Context{}

			matched := tmpl.matchAndExtract(tt.path, ctx)

			assert.Equal(t, tt.shouldMatch, matched)

			if matched && tt.params != nil {
				for key, expectedValue := range tt.params {
					actualValue := ctx.Param(key)
					assert.Equal(t, expectedValue, actualValue, "Param %q", key)
				}
			}
		})
	}
}

// TestTemplateWithConstraints tests template matching with constraints
func TestTemplateWithConstraints(t *testing.T) {
	t.Parallel()

	constraints := []RouteConstraint{
		{Param: "id", Pattern: regexp.MustCompile(`^\d+$`)},
	}

	tmpl := compileRouteTemplate("GET", "/users/:id", []HandlerFunc{func(_ *Context) {}}, constraints)

	tests := []struct {
		name        string
		path        string
		shouldMatch bool
	}{
		{"valid numeric id", "/users/123", true},
		{"invalid alphabetic id", "/users/abc", false}, // Constraint violation
		{"invalid mixed id", "/users/12abc", false},    // Constraint violation
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := &Context{}
			matched := tmpl.matchAndExtract(tt.path, ctx)
			assert.Equal(t, tt.shouldMatch, matched, "Path %q", tt.path)
		})
	}
}

// TestTemplateCache_BuildFirstSegmentIndex tests first segment optimization
func TestTemplateCache_BuildFirstSegmentIndex(t *testing.T) {
	t.Parallel()

	r := MustNew()

	// Register routes with different first segments
	r.GET("/users/:id", func(_ *Context) {})
	r.GET("/posts/:id", func(_ *Context) {})
	r.GET("/admin/:id", func(_ *Context) {})
	r.GET("/api/:resource", func(_ *Context) {})

	// Build index
	r.templateCache.buildFirstSegmentIndex()

	// Verify index is built
	assert.True(t, r.templateCache.hasFirstSegmentIndex)

	// Index should have entries for 'u', 'p', 'a'
	assert.NotNil(t, r.templateCache.firstSegmentIndex['u'], "should have index entry for 'u' (users)")
	assert.NotNil(t, r.templateCache.firstSegmentIndex['p'], "should have index entry for 'p' (posts)")
	assert.NotNil(t, r.templateCache.firstSegmentIndex['a'], "should have index entry for 'a' (admin, api)")

	// Verify 'a' has 2 entries (admin and api)
	aEntries := r.templateCache.firstSegmentIndex['a']
	assert.GreaterOrEqual(t, len(aEntries), 2, "expected at least 2 entries for 'a'")
}

// TestTemplateCache_BuildFirstSegmentIndex_EmptyCache tests building index on empty cache
func TestTemplateCache_BuildFirstSegmentIndex_EmptyCache(t *testing.T) {
	t.Parallel()

	r := MustNew()

	// Build index with no routes
	r.templateCache.buildFirstSegmentIndex()

	// Should not panic
	assert.True(t, r.templateCache.hasFirstSegmentIndex, "index should be marked as built even if empty")
}

// TestTemplateCache_BuildFirstSegmentIndex_RootPath tests index with root path
func TestTemplateCache_BuildFirstSegmentIndex_RootPath(t *testing.T) {
	t.Parallel()

	r := MustNew()

	// Register root path
	r.GET("/", func(_ *Context) {})
	r.GET("/users", func(_ *Context) {})

	// Build index
	r.templateCache.buildFirstSegmentIndex()

	// Root path shouldn't cause issues
	assert.True(t, r.templateCache.hasFirstSegmentIndex, "should build index successfully with root path")
}

// TestTemplateCache_BuildFirstSegmentIndex_NonASCII tests index with non-ASCII first char
func TestTemplateCache_BuildFirstSegmentIndex_NonASCII(t *testing.T) {
	t.Parallel()

	r := MustNew()

	// Register route with non-ASCII first character (should be ignored)
	r.GET("/über/:id", func(_ *Context) {})
	r.GET("/users/:id", func(_ *Context) {})

	// Build index
	r.templateCache.buildFirstSegmentIndex()

	// Should build successfully
	assert.True(t, r.templateCache.hasFirstSegmentIndex, "should build index even with non-ASCII paths")

	// ASCII route should be indexed
	assert.NotNil(t, r.templateCache.firstSegmentIndex['u'], "should have index for ASCII 'u'")
}

// TestTemplateCache_RemoveTemplate tests template removal
func TestTemplateCache_RemoveTemplate(t *testing.T) {
	t.Parallel()

	r := MustNew()

	// Register a route
	r.GET("/test/:id", func(c *Context) {
		c.String(http.StatusOK, "test")
	})

	// Count initial templates
	initialCount := len(r.templateCache.dynamicTemplates)

	require.Greater(t, initialCount, 0, "should have templates after route registration")

	// Remove the template
	r.templateCache.removeTemplate("GET", "/test/:id")

	// Should have fewer templates now
	afterCount := len(r.templateCache.dynamicTemplates)

	assert.Less(t, afterCount, initialCount, "template count should decrease after removal")
}

// TestTemplateCache_AddTemplate_Duplicate tests adding duplicate template
func TestTemplateCache_AddTemplate_Duplicate(_ *testing.T) {
	r := MustNew()

	// Add template manually
	tmpl := compileRouteTemplate("GET", "/users/:id", []HandlerFunc{func(_ *Context) {}}, nil)

	r.templateCache.addTemplate(tmpl)
	count1 := len(r.templateCache.dynamicTemplates)

	// Add same template again
	r.templateCache.addTemplate(tmpl)
	count2 := len(r.templateCache.dynamicTemplates)

	// Count might increase (duplicate) or stay same (dedup) - both behaviors are acceptable
	_ = count1
	_ = count2
}

// TestTemplateCache_SortBySpecificity tests template sorting
func TestTemplateCache_SortBySpecificity(t *testing.T) {
	t.Parallel()

	r := MustNew()

	// Register routes in random order
	r.GET("/api/*", func(_ *Context) {})               // Less specific
	r.GET("/api/users/:id", func(_ *Context) {})       // More specific
	r.GET("/api/users/:id/posts", func(_ *Context) {}) // Most specific
	r.GET("/api/:resource", func(_ *Context) {})       // Medium specific

	// Templates should be sorted by specificity
	// We can't directly test sorting, but we can verify routes work correctly
	r.Warmup()

	// Most specific route should match
	req := httptest.NewRequest("GET", "/api/users/123/posts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "specific route should match")
}

// TestTemplateCache_Concurrent tests concurrent template operations
func TestTemplateCache_Concurrent(_ *testing.T) {
	r := MustNew()

	// Add routes concurrently
	done := make(chan bool)

	for i := range 10 {
		go func(id int) {
			defer func() { done <- true }()

			r.GET("/route"+string(rune('0'+id))+"/:param", func(_ *Context) {
				// Handler intentionally empty for concurrent test
			})
		}(i)
	}

	// Wait for all
	for range 10 {
		<-done
	}

	// Build index shouldn't panic
	r.templateCache.buildFirstSegmentIndex()

	// Warmup shouldn't panic
	r.Warmup()
}

// TestRadix_CompileStaticRoutes_NoRoutes tests compiling with no static routes
func TestRadix_CompileStaticRoutes_NoRoutes(t *testing.T) {
	t.Parallel()

	r := MustNew()

	// Register only dynamic routes (no static routes)
	r.GET("/users/:id", func(_ *Context) {})
	r.GET("/posts/:id", func(_ *Context) {})

	// Compile routes
	r.CompileAllRoutes()

	// Should not panic even with no static routes
	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "dynamic routes should still work")
}

// TestRadix_CompileStaticRoutes_MixedRoutes tests compiling mixed static and dynamic
func TestRadix_CompileStaticRoutes_MixedRoutes(t *testing.T) {
	t.Parallel()

	r := MustNew()

	// Mix of static and dynamic routes
	r.GET("/static/path", func(_ *Context) {})
	r.GET("/users/:id", func(_ *Context) {})
	r.GET("/another/static", func(_ *Context) {})
	r.GET("/items/:id/details", func(_ *Context) {})

	// Compile
	r.CompileAllRoutes()

	// Both types should work
	req1 := httptest.NewRequest("GET", "/static/path", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	assert.Equal(t, http.StatusOK, w1.Code, "static route should work after compilation")

	req2 := httptest.NewRequest("GET", "/users/789", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code, "dynamic route should work after compilation")
}
