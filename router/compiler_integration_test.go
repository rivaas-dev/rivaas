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

package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"rivaas.dev/router"
)

// TestCompilerIntegration_StaticRoutes tests compiler integration with static routes.
//
//nolint:tparallel // False positive: t.Parallel() is called at both top level and in subtests
func TestCompilerIntegration_StaticRoutes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tests := []struct {
		name         string
		routes       []string
		testPath     string
		wantCode     int
		wantResponse string
	}{
		{
			name:         "simple static route",
			routes:       []string{"/api/users"},
			testPath:     "/api/users",
			wantCode:     http.StatusOK,
			wantResponse: "users",
		},
		{
			name:         "multiple static routes",
			routes:       []string{"/api/users", "/api/posts", "/health"},
			testPath:     "/api/posts",
			wantCode:     http.StatusOK,
			wantResponse: "posts",
		},
		{
			name:     "non-existent route",
			routes:   []string{"/api/users"},
			testPath: "/api/posts",
			wantCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := router.MustNew()

			for _, route := range tt.routes {
				path := route
				r.GET(route, func(c *router.Context) {
					// Extract response from path
					switch path {
					case "/api/users":
						c.String(http.StatusOK, "users")
					case "/api/posts":
						c.String(http.StatusOK, "posts")
					case "/health":
						c.String(http.StatusOK, "ok")
					}
				})
			}

			// Warmup to compile routes
			r.Warmup()

			req := httptest.NewRequest(http.MethodGet, tt.testPath, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.wantCode, w.Code)
			if tt.wantResponse != "" {
				assert.Equal(t, tt.wantResponse, w.Body.String())
			}
		})
	}
}

// TestCompilerIntegration_DynamicRoutes tests compiler integration with dynamic routes.
//
//nolint:tparallel // False positive: t.Parallel() is called at both top level and in subtests
func TestCompilerIntegration_DynamicRoutes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tests := []struct {
		name       string
		pattern    string
		testPath   string
		wantCode   int
		wantParams map[string]string
	}{
		{
			name:     "single parameter",
			pattern:  "/api/users/:id",
			testPath: "/api/users/123",
			wantCode: http.StatusOK,
			wantParams: map[string]string{
				"id": "123",
			},
		},
		{
			name:     "multiple parameters",
			pattern:  "/api/users/:id/posts/:pid",
			testPath: "/api/users/123/posts/456",
			wantCode: http.StatusOK,
			wantParams: map[string]string{
				"id":  "123",
				"pid": "456",
			},
		},
		{
			name:     "mixed static and parameters",
			pattern:  "/api/v1/users/:id/posts/:pid",
			testPath: "/api/v1/users/123/posts/456",
			wantCode: http.StatusOK,
			wantParams: map[string]string{
				"id":  "123",
				"pid": "456",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := router.MustNew()

			r.GET(tt.pattern, func(c *router.Context) {
				params := make(map[string]string)
				for key := range tt.wantParams {
					params[key] = c.Param(key)
				}
				c.JSON(http.StatusOK, params)
			})

			r.Warmup()

			req := httptest.NewRequest(http.MethodGet, tt.testPath, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.wantCode, w.Code)

			// Verify parameters were extracted correctly
			for key, expectedValue := range tt.wantParams {
				assert.Contains(t, w.Body.String(), `"`+key+`":"`+expectedValue+`"`)
			}
		})
	}
}

// TestCompilerIntegration_Constraints tests compiler integration with parameter constraints.
//
//nolint:tparallel // False positive: t.Parallel() is called at both top level and in subtests
func TestCompilerIntegration_Constraints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tests := []struct {
		name        string
		path        string
		shouldMatch bool
	}{
		{
			name:        "valid numeric id",
			path:        "/users/123",
			shouldMatch: true,
		},
		{
			name:        "invalid alphabetic id",
			path:        "/users/abc",
			shouldMatch: false,
		},
		{
			name:        "invalid mixed id",
			path:        "/users/12abc",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := router.MustNew()
			r.GET("/users/:id", func(c *router.Context) {
				c.Status(http.StatusOK)
			}).WhereInt("id")
			r.Warmup()

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if tt.shouldMatch {
				assert.Equal(t, http.StatusOK, w.Code, "Path %q should match", tt.path)
			} else {
				assert.Equal(t, http.StatusNotFound, w.Code, "Path %q should not match", tt.path)
			}
		})
	}
}

// TestCompilerIntegration_MatchingCorrectness tests route pattern matching correctness.
//
//nolint:tparallel // False positive: t.Parallel() is called at both top level and in subtests
func TestCompilerIntegration_MatchingCorrectness(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tests := []struct {
		name        string
		pattern     string
		path        string
		shouldMatch bool
		params      map[string]string
	}{
		{
			name:        "simple parameter",
			pattern:     "/users/:id",
			path:        "/users/123",
			shouldMatch: true,
			params:      map[string]string{"id": "123"},
		},
		{
			name:        "multiple parameters",
			pattern:     "/users/:id/posts/:pid",
			path:        "/users/123/posts/456",
			shouldMatch: true,
			params:      map[string]string{"id": "123", "pid": "456"},
		},
		{
			name:        "mixed static and parameters",
			pattern:     "/api/v1/users/:id/posts/:pid",
			path:        "/api/v1/users/123/posts/456",
			shouldMatch: true,
			params:      map[string]string{"id": "123", "pid": "456"},
		},
		{
			name:        "segment count mismatch",
			pattern:     "/users/:id",
			path:        "/users/123/extra",
			shouldMatch: false,
		},
		{
			name:        "static segment mismatch",
			pattern:     "/users/:id",
			path:        "/posts/123",
			shouldMatch: false,
		},
		{
			name:        "root path",
			pattern:     "/",
			path:        "/",
			shouldMatch: true,
			params:      map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := router.MustNew()
			var capturedParams map[string]string
			r.GET(tt.pattern, func(c *router.Context) {
				capturedParams = make(map[string]string)
				for key := range tt.params {
					capturedParams[key] = c.Param(key)
				}
				c.Status(http.StatusOK)
			})
			r.Warmup()

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if tt.shouldMatch {
				assert.Equal(t, http.StatusOK, w.Code, "Route should match")
				if tt.params != nil {
					for key, expectedValue := range tt.params {
						actualValue := capturedParams[key]
						assert.Equal(t, expectedValue, actualValue, "Param %q", key)
					}
				}
			} else {
				assert.Equal(t, http.StatusNotFound, w.Code, "Route should not match")
			}
		})
	}
}

// TestCompilerIntegration_FirstSegmentIndex tests first segment optimization.
func TestCompilerIntegration_FirstSegmentIndex(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	r := router.MustNew()

	// Register routes with different first segments
	r.GET("/users/:id", func(c *router.Context) {
		c.String(http.StatusOK, "users")
	})
	r.GET("/posts/:id", func(c *router.Context) {
		c.String(http.StatusOK, "posts")
	})
	r.GET("/admin/:id", func(c *router.Context) {
		c.String(http.StatusOK, "admin")
	})
	r.GET("/api/:resource", func(c *router.Context) {
		c.String(http.StatusOK, "api")
	})

	// Warmup triggers index building internally
	r.Warmup()

	// Verify routes work (indirectly tests index is working)
	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "route should match")
	assert.Equal(t, "users", w.Body.String())
}

// TestCompilerIntegration_Sorting tests route sorting by specificity.
func TestCompilerIntegration_Sorting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	r := router.MustNew()

	// Register routes in random order
	r.GET("/api/*", func(c *router.Context) {
		c.String(http.StatusOK, "wildcard")
	}) // Less specific
	r.GET("/api/users/:id", func(c *router.Context) {
		c.String(http.StatusOK, "user")
	}) // More specific
	r.GET("/api/users/:id/posts", func(c *router.Context) {
		c.String(http.StatusOK, "posts")
	}) // Most specific
	r.GET("/api/:resource", func(c *router.Context) {
		c.String(http.StatusOK, "resource")
	}) // Medium specific

	r.Warmup()

	// Most specific route should match
	req := httptest.NewRequest(http.MethodGet, "/api/users/123/posts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "specific route should match")
	assert.Equal(t, "posts", w.Body.String())
}

// TestCompilerIntegration_Concurrent tests concurrent route compilation operations.
func TestCompilerIntegration_Concurrent(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	r := router.MustNew()

	// Add routes concurrently
	done := make(chan bool)

	for i := range 10 {
		go func(id int) {
			defer func() { done <- true }()

			r.GET("/route"+string(rune('0'+id))+"/:param", func(c *router.Context) {
				c.Status(http.StatusOK)
			})
		}(i)
	}

	// Wait for all
	for range 10 {
		<-done
	}

	// Warmup shouldn't panic (triggers index building internally)
	r.Warmup()

	// Verify a route works
	req := httptest.NewRequest(http.MethodGet, "/route0/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestCompilerIntegration_NoStaticRoutes tests compiling with no static routes.
func TestCompilerIntegration_NoStaticRoutes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	r := router.MustNew()

	// Register only dynamic routes (no static routes)
	r.GET("/users/:id", func(c *router.Context) {
		c.String(http.StatusOK, "user")
	})
	r.GET("/posts/:id", func(c *router.Context) {
		c.String(http.StatusOK, "post")
	})

	// Compile routes
	r.CompileAllRoutes()

	// Should not panic even with no static routes
	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "dynamic routes should still work")
	assert.Equal(t, "user", w.Body.String())
}

// TestCompilerIntegration_MixedRoutes tests compiling mixed static and dynamic routes.
func TestCompilerIntegration_MixedRoutes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	r := router.MustNew()

	// Mix of static and dynamic routes
	r.GET("/static/path", func(c *router.Context) {
		c.String(http.StatusOK, "static1")
	})
	r.GET("/users/:id", func(c *router.Context) {
		c.String(http.StatusOK, "dynamic1")
	})
	r.GET("/another/static", func(c *router.Context) {
		c.String(http.StatusOK, "static2")
	})
	r.GET("/items/:id/details", func(c *router.Context) {
		c.String(http.StatusOK, "dynamic2")
	})

	// Compile
	r.CompileAllRoutes()

	// Test static route
	req1 := httptest.NewRequest(http.MethodGet, "/static/path", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	assert.Equal(t, http.StatusOK, w1.Code, "static route should work after compilation")
	assert.Equal(t, "static1", w1.Body.String())

	// Test dynamic route
	req2 := httptest.NewRequest("GET", "/users/789", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code, "dynamic route should work after compilation")
	assert.Equal(t, "dynamic1", w2.Body.String())
}

// BenchmarkCompilerIntegration_StaticRoute benchmarks compiled static route matching.
func BenchmarkCompilerIntegration_StaticRoute(b *testing.B) {
	r := router.MustNew()
	r.GET("/api/users", func(c *router.Context) {
		c.String(http.StatusOK, "users")
	})
	r.Warmup()

	req := httptest.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w.Body.Reset()
		w.Code = 0
		r.ServeHTTP(w, req)
	}
}

// BenchmarkCompilerIntegration_DynamicRoute benchmarks compiled dynamic route matching.
func BenchmarkCompilerIntegration_DynamicRoute(b *testing.B) {
	r := router.MustNew()
	r.GET("/api/users/:id/posts/:pid", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Warmup()

	req := httptest.NewRequest("GET", "/api/users/123/posts/456", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w.Body.Reset()
		w.Code = 0
		r.ServeHTTP(w, req)
	}
}

// BenchmarkCompilerIntegration_WithVsWithoutCompiler compares performance.
func BenchmarkCompilerIntegration_WithVsWithoutCompiler(b *testing.B) {
	b.Run("WithCompiler", func(b *testing.B) {
		r := router.MustNew(router.WithRouteCompilation(true))
		r.GET("/api/users/:id/posts/:pid/comments/:cid", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})
		r.Warmup()

		req := httptest.NewRequest("GET", "/api/users/123/posts/456/comments/789", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			w.Body.Reset()
			w.Code = 0
			r.ServeHTTP(w, req)
		}
	})

	b.Run("WithoutCompiler", func(b *testing.B) {
		r := router.MustNew(router.WithRouteCompilation(false))
		r.GET("/api/users/:id/posts/:pid/comments/:cid", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})
		r.Warmup()

		req := httptest.NewRequest("GET", "/api/users/123/posts/456/comments/789", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			w.Body.Reset()
			w.Code = 0
			r.ServeHTTP(w, req)
		}
	})
}

// BenchmarkCompilerIntegration_Matching benchmarks compiled route matching logic.
func BenchmarkCompilerIntegration_Matching(b *testing.B) {
	r := router.MustNew()
	r.GET("/api/users/:id/posts/:pid", func(_ *router.Context) {})
	r.Warmup()

	req := httptest.NewRequest("GET", "/api/users/123/posts/456", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w.Body.Reset()
		w.Code = 0
		r.ServeHTTP(w, req)
	}
}

// BenchmarkCompilerIntegration_ConstrainedRoute benchmarks routes with constraints.
func BenchmarkCompilerIntegration_ConstrainedRoute(b *testing.B) {
	r := router.MustNew()

	// Register route with constraints
	r.GET("/api/users/:id", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
	}).Where("id", `^\d+$`)

	// Warmup to compile routes
	r.Warmup()

	req := httptest.NewRequest("GET", "/api/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w.Body.Reset()
		w.Code = 0
		r.ServeHTTP(w, req)
	}
}
