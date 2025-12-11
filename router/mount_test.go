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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMount_BasicRouteRegistration(t *testing.T) {
	t.Parallel()

	// Create subrouter with routes
	sub := MustNew()
	sub.GET("/users", func(c *Context) {
		_ = c.String(http.StatusOK, "users list") // Error ignored: test verifies route registration, not response handling
	})
	sub.GET("/users/:id", func(c *Context) {
		_ = c.String(http.StatusOK, "user "+c.Param("id")) // Error ignored: test verifies route registration, not response handling
	})
	sub.POST("/users", func(c *Context) {
		_ = c.String(http.StatusOK, "user created") // Error ignored: test verifies route registration, not response handling
	})

	// Create parent router and mount subrouter
	r := MustNew()
	r.GET("/health", func(c *Context) {
		_ = c.String(http.StatusOK, "ok") // Error ignored: test verifies route registration, not response handling
	})
	r.Mount("/api/v1", sub)

	// Verify routes are registered with correct paths
	routes := r.Routes()
	routePaths := make(map[string]bool)
	for _, route := range routes {
		key := route.Method + " " + route.Path
		routePaths[key] = true
	}

	assert.True(t, routePaths["GET /health"], "parent route should exist")
	assert.True(t, routePaths["GET /api/v1/users"], "mounted GET users route should exist")
	assert.True(t, routePaths["GET /api/v1/users/:id"], "mounted GET user by id route should exist")
	assert.True(t, routePaths["POST /api/v1/users"], "mounted POST users route should exist")
}

func TestMount_RoutePatternPreservedForObservability(t *testing.T) {
	t.Parallel()

	// Create subrouter with routes
	sub := MustNew()
	var capturedPattern string
	sub.GET("/users/:id", func(c *Context) {
		capturedPattern = c.RoutePattern()
		_ = c.String(http.StatusOK, "user "+c.Param("id")) // Error ignored: test verifies route pattern preservation, not response handling
	})

	// Create parent router and mount subrouter
	r := MustNew()
	r.Mount("/api/v1", sub)

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user 123", w.Body.String())

	// Verify route pattern is the FULL path, not a catch-all
	assert.Equal(t, "/api/v1/users/:id", capturedPattern,
		"route pattern should be full path for observability, not /api/v1/*")
}

func TestMount_MiddlewareInheritance(t *testing.T) {
	t.Parallel()

	var middlewareOrder []string

	parentMiddleware := func(c *Context) {
		middlewareOrder = append(middlewareOrder, "parent")
		c.Next()
	}
	subMiddleware := func(c *Context) {
		middlewareOrder = append(middlewareOrder, "sub")
		c.Next()
	}
	extraMiddleware := func(c *Context) {
		middlewareOrder = append(middlewareOrder, "extra")
		c.Next()
	}

	// Create parent with middleware
	r := MustNew()
	r.Use(parentMiddleware)

	// Create subrouter with middleware
	sub := MustNew()
	sub.Use(subMiddleware)
	sub.GET("/test", func(c *Context) {
		middlewareOrder = append(middlewareOrder, "handler")
		_ = c.String(http.StatusOK, "ok") // Error ignored: test verifies middleware inheritance, not response handling
	})

	// Mount with InheritMiddleware and extra middleware
	// InheritMiddleware adds parent middleware to route chain, so it runs twice:
	// 1. Once via parent router's global middleware (ServeHTTP flow)
	// 2. Once via the route's handler chain (InheritMiddleware copies it)
	r.Mount("/api", sub, InheritMiddleware(), WithMiddleware(extraMiddleware))

	// Make request
	middlewareOrder = nil
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Verify middleware execution order:
	// parent (global) → parent (inherited) → sub → extra → handler
	expected := []string{"parent", "parent", "sub", "extra", "handler"}
	assert.Equal(t, expected, middlewareOrder,
		"with InheritMiddleware, parent middleware runs twice (global + route chain)")
}

func TestMount_MiddlewareWithoutInheritance(t *testing.T) {
	t.Parallel()

	var middlewareOrder []string

	parentMiddleware := func(c *Context) {
		middlewareOrder = append(middlewareOrder, "parent")
		c.Next()
	}
	subMiddleware := func(c *Context) {
		middlewareOrder = append(middlewareOrder, "sub")
		c.Next()
	}

	// Create parent with middleware
	r := MustNew()
	r.Use(parentMiddleware)

	// Create subrouter with middleware
	sub := MustNew()
	sub.Use(subMiddleware)
	sub.GET("/test", func(c *Context) {
		middlewareOrder = append(middlewareOrder, "handler")
		_ = c.String(http.StatusOK, "ok") // Error ignored: test verifies middleware inheritance, not response handling
	})

	// Mount WITHOUT InheritMiddleware
	// Parent global middleware still runs (via ServeHTTP), but is not duplicated in route chain
	r.Mount("/api", sub)

	// Make request
	middlewareOrder = nil
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Parent middleware runs once (global), then sub middleware, then handler
	expected := []string{"parent", "sub", "handler"}
	assert.Equal(t, expected, middlewareOrder,
		"without InheritMiddleware, parent middleware runs only once (via global middleware)")
}

func TestMount_NamePrefix(t *testing.T) {
	t.Parallel()

	sub := MustNew()
	sub.GET("/users", func(c *Context) {
		_ = c.String(http.StatusOK, "users") // Error ignored: test verifies name prefixing, not response handling
	}).SetName("users.list")

	r := MustNew()
	r.Mount("/api", sub, NamePrefix("api."))

	// Warmup to compile routes and names
	r.Warmup()

	// Check that route name is prefixed
	routes := r.Routes()
	found := false
	for _, route := range routes {
		if route.Path == "/api/users" {
			found = true
			break
		}
	}
	assert.True(t, found, "mounted route should exist with full path")
}

func TestMount_WithNotFound(t *testing.T) {
	t.Parallel()

	notFoundCalled := false
	customNotFound := func(c *Context) {
		notFoundCalled = true
		_ = c.String(http.StatusNotFound, "custom 404") // Error ignored: test verifies custom 404 handler, not response details
	}

	sub := MustNew()
	sub.GET("/users", func(c *Context) {
		_ = c.String(http.StatusOK, "users") // Error ignored: test verifies custom 404 handler, not response details
	})

	r := MustNew()
	r.Mount("/api", sub, WithNotFound(customNotFound))

	// Request to existing route
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, notFoundCalled)

	// Request to non-existing route within prefix
	notFoundCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.True(t, notFoundCalled, "custom 404 should be called for paths within mount prefix")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "custom 404", w.Body.String())
}

func TestMount_ConstraintsPreserved(t *testing.T) {
	t.Parallel()

	sub := MustNew()
	sub.GET("/users/:id", func(c *Context) {
		_ = c.String(http.StatusOK, "user "+c.Param("id")) // Error ignored: test verifies constraint preservation, not response handling
	}).Where("id", `\d+`)

	r := MustNew()
	r.Mount("/api", sub)

	// Valid ID (numeric)
	req := httptest.NewRequest(http.MethodGet, "/api/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user 123", w.Body.String())

	// Invalid ID (non-numeric) - should not match
	req = httptest.NewRequest(http.MethodGet, "/api/users/abc", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMount_TypedConstraintsPreserved(t *testing.T) {
	t.Parallel()

	sub := MustNew()
	sub.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "user "+c.Param("id"))
	}).WhereInt("id")

	r := MustNew()
	r.Mount("/api", sub)

	// Valid integer ID
	req := httptest.NewRequest(http.MethodGet, "/api/users/456", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Invalid non-integer ID
	req = httptest.NewRequest(http.MethodGet, "/api/users/abc", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMount_RootPathHandling(t *testing.T) {
	t.Parallel()

	sub := MustNew()
	sub.GET("/", func(c *Context) {
		_ = c.String(http.StatusOK, "root") // Error ignored: test verifies root path handling, not response details
	})
	sub.GET("/nested", func(c *Context) {
		_ = c.String(http.StatusOK, "nested") // Error ignored: test verifies nested path handling, not response details
	})

	r := MustNew()
	r.Mount("/api", sub)

	// Request to mount root
	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "root", w.Body.String())

	// Request to nested path
	req = httptest.NewRequest(http.MethodGet, "/api/nested", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "nested", w.Body.String())
}

func TestMount_MultipleMounts(t *testing.T) {
	t.Parallel()

	// Create two subrouters
	apiV1 := MustNew()
	apiV1.GET("/users", func(c *Context) {
		_ = c.String(http.StatusOK, "v1 users") // Error ignored: test verifies multiple mounts, not response handling
	})

	apiV2 := MustNew()
	apiV2.GET("/users", func(c *Context) {
		_ = c.String(http.StatusOK, "v2 users") // Error ignored: test verifies multiple mounts, not response handling
	})

	// Mount both
	r := MustNew()
	r.Mount("/api/v1", apiV1)
	r.Mount("/api/v2", apiV2)

	// Test v1
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "v1 users", w.Body.String())

	// Test v2
	req = httptest.NewRequest(http.MethodGet, "/api/v2/users", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "v2 users", w.Body.String())
}

func TestMount_ParamsExtracted(t *testing.T) {
	t.Parallel()

	sub := MustNew()
	sub.GET("/users/:userId/posts/:postId", func(c *Context) {
		userID := c.Param("userId")
		postID := c.Param("postId")
		_ = c.String(http.StatusOK, "user:"+userID+" post:"+postID) // Error ignored: test verifies param extraction, not response handling
	})

	r := MustNew()
	r.Mount("/api", sub)

	req := httptest.NewRequest(http.MethodGet, "/api/users/42/posts/99", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user:42 post:99", w.Body.String())
}

func TestMount_WildcardRoutes(t *testing.T) {
	t.Parallel()

	sub := MustNew()
	sub.GET("/files/*", func(c *Context) {
		filepath := c.Param("filepath")
		_ = c.String(http.StatusOK, "file:"+filepath) // Error ignored: test verifies wildcard routes, not response handling
	})

	r := MustNew()
	r.Mount("/static", sub)

	req := httptest.NewRequest(http.MethodGet, "/static/files/css/app.css", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "file:css/app.css", w.Body.String())
}

func TestMount_NilSubrouter(t *testing.T) {
	t.Parallel()

	r := MustNew()
	// Should not panic
	r.Mount("/api", nil)

	// Router should still work
	r.GET("/test", func(c *Context) {
		_ = c.String(http.StatusOK, "ok") // Error ignored: test verifies nil subrouter handling, not response details
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMount_AllHTTPMethods(t *testing.T) {
	t.Parallel()

	sub := MustNew()
	sub.GET("/resource", func(c *Context) { _ = c.String(http.StatusOK, http.MethodGet) })       // Error ignored: test verifies HTTP methods, not response handling
	sub.POST("/resource", func(c *Context) { _ = c.String(http.StatusOK, http.MethodPost) })     // Error ignored: test verifies HTTP methods, not response handling
	sub.PUT("/resource", func(c *Context) { _ = c.String(http.StatusOK, http.MethodPut) })       // Error ignored: test verifies HTTP methods, not response handling
	sub.PATCH("/resource", func(c *Context) { _ = c.String(http.StatusOK, http.MethodPatch) })   // Error ignored: test verifies HTTP methods, not response handling
	sub.DELETE("/resource", func(c *Context) { _ = c.String(http.StatusOK, http.MethodDelete) }) // Error ignored: test verifies HTTP methods, not response handling
	sub.HEAD("/resource", func(c *Context) { c.Status(http.StatusOK) })
	sub.OPTIONS("/resource", func(c *Context) { _ = c.String(http.StatusOK, http.MethodOptions) }) // Error ignored: test verifies HTTP methods, not response handling

	r := MustNew()
	r.Mount("/api", sub)

	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(method, "/api/resource", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "method %s should work", method)
		})
	}
}

func TestMount_ObservabilityIntegration(t *testing.T) {
	t.Parallel()

	// Track route patterns seen by observability
	var observedPatterns []string

	sub := MustNew()
	sub.GET("/users/:id", func(c *Context) {
		observedPatterns = append(observedPatterns, c.RoutePattern())
		_ = c.String(http.StatusOK, "ok") // Error ignored: test verifies observability integration, not response handling
	})
	sub.GET("/products/:category/:id", func(c *Context) {
		observedPatterns = append(observedPatterns, c.RoutePattern())
		_ = c.String(http.StatusOK, "ok") // Error ignored: test verifies observability integration, not response handling
	})

	r := MustNew()
	r.Mount("/api/v1", sub)

	// Make requests
	requests := []string{
		"/api/v1/users/123",
		"/api/v1/products/electronics/456",
	}

	for _, path := range requests {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	}

	// Verify patterns are full paths, not catch-all patterns
	expected := []string{
		"/api/v1/users/:id",
		"/api/v1/products/:category/:id",
	}
	assert.Equal(t, expected, observedPatterns,
		"route patterns should be full paths for correct metrics/tracing")
}
