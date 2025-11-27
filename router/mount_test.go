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
		c.String(http.StatusOK, "users list")
	})
	sub.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "user "+c.Param("id"))
	})
	sub.POST("/users", func(c *Context) {
		c.String(http.StatusOK, "user created")
	})

	// Create parent router and mount subrouter
	r := MustNew()
	r.GET("/health", func(c *Context) {
		c.String(http.StatusOK, "ok")
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

func TestMount_RouteTemplatePreservedForObservability(t *testing.T) {
	t.Parallel()

	// Create subrouter with routes
	sub := MustNew()
	var capturedTemplate string
	sub.GET("/users/:id", func(c *Context) {
		capturedTemplate = c.RouteTemplate()
		c.String(http.StatusOK, "user "+c.Param("id"))
	})

	// Create parent router and mount subrouter
	r := MustNew()
	r.Mount("/api/v1", sub)

	// Make request
	req := httptest.NewRequest("GET", "/api/v1/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user 123", w.Body.String())

	// Verify route template is the FULL path, not a catch-all
	assert.Equal(t, "/api/v1/users/:id", capturedTemplate,
		"route template should be full path for observability, not /api/v1/*")
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
		c.String(http.StatusOK, "ok")
	})

	// Mount with InheritMiddleware and extra middleware
	// InheritMiddleware adds parent middleware to route chain, so it runs twice:
	// 1. Once via parent router's global middleware (ServeHTTP flow)
	// 2. Once via the route's handler chain (InheritMiddleware copies it)
	r.Mount("/api", sub, InheritMiddleware(), WithMiddleware(extraMiddleware))

	// Make request
	middlewareOrder = nil
	req := httptest.NewRequest("GET", "/api/test", nil)
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
		c.String(http.StatusOK, "ok")
	})

	// Mount WITHOUT InheritMiddleware
	// Parent global middleware still runs (via ServeHTTP), but is not duplicated in route chain
	r.Mount("/api", sub)

	// Make request
	middlewareOrder = nil
	req := httptest.NewRequest("GET", "/api/test", nil)
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
		c.String(http.StatusOK, "users")
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
		c.String(http.StatusNotFound, "custom 404")
	}

	sub := MustNew()
	sub.GET("/users", func(c *Context) {
		c.String(http.StatusOK, "users")
	})

	r := MustNew()
	r.Mount("/api", sub, WithNotFound(customNotFound))

	// Request to existing route
	req := httptest.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, notFoundCalled)

	// Request to non-existing route within prefix
	notFoundCalled = false
	req = httptest.NewRequest("GET", "/api/nonexistent", nil)
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
		c.String(http.StatusOK, "user "+c.Param("id"))
	}).Where("id", `\d+`)

	r := MustNew()
	r.Mount("/api", sub)

	// Valid ID (numeric)
	req := httptest.NewRequest("GET", "/api/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user 123", w.Body.String())

	// Invalid ID (non-numeric) - should not match
	req = httptest.NewRequest("GET", "/api/users/abc", nil)
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
	req := httptest.NewRequest("GET", "/api/users/456", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Invalid non-integer ID
	req = httptest.NewRequest("GET", "/api/users/abc", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMount_RootPathHandling(t *testing.T) {
	t.Parallel()

	sub := MustNew()
	sub.GET("/", func(c *Context) {
		c.String(http.StatusOK, "root")
	})
	sub.GET("/nested", func(c *Context) {
		c.String(http.StatusOK, "nested")
	})

	r := MustNew()
	r.Mount("/api", sub)

	// Request to mount root
	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "root", w.Body.String())

	// Request to nested path
	req = httptest.NewRequest("GET", "/api/nested", nil)
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
		c.String(http.StatusOK, "v1 users")
	})

	apiV2 := MustNew()
	apiV2.GET("/users", func(c *Context) {
		c.String(http.StatusOK, "v2 users")
	})

	// Mount both
	r := MustNew()
	r.Mount("/api/v1", apiV1)
	r.Mount("/api/v2", apiV2)

	// Test v1
	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "v1 users", w.Body.String())

	// Test v2
	req = httptest.NewRequest("GET", "/api/v2/users", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "v2 users", w.Body.String())
}

func TestMount_ParamsExtracted(t *testing.T) {
	t.Parallel()

	sub := MustNew()
	sub.GET("/users/:userId/posts/:postId", func(c *Context) {
		userId := c.Param("userId")
		postId := c.Param("postId")
		c.String(http.StatusOK, "user:"+userId+" post:"+postId)
	})

	r := MustNew()
	r.Mount("/api", sub)

	req := httptest.NewRequest("GET", "/api/users/42/posts/99", nil)
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
		c.String(http.StatusOK, "file:"+filepath)
	})

	r := MustNew()
	r.Mount("/static", sub)

	req := httptest.NewRequest("GET", "/static/files/css/app.css", nil)
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
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMount_AllHTTPMethods(t *testing.T) {
	t.Parallel()

	sub := MustNew()
	sub.GET("/resource", func(c *Context) { c.String(http.StatusOK, "GET") })
	sub.POST("/resource", func(c *Context) { c.String(http.StatusOK, "POST") })
	sub.PUT("/resource", func(c *Context) { c.String(http.StatusOK, "PUT") })
	sub.PATCH("/resource", func(c *Context) { c.String(http.StatusOK, "PATCH") })
	sub.DELETE("/resource", func(c *Context) { c.String(http.StatusOK, "DELETE") })
	sub.HEAD("/resource", func(c *Context) { c.Status(http.StatusOK) })
	sub.OPTIONS("/resource", func(c *Context) { c.String(http.StatusOK, "OPTIONS") })

	r := MustNew()
	r.Mount("/api", sub)

	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/resource", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "method %s should work", method)
		})
	}
}

func TestMount_ObservabilityIntegration(t *testing.T) {
	t.Parallel()

	// Track route templates seen by observability
	var observedTemplates []string

	sub := MustNew()
	sub.GET("/users/:id", func(c *Context) {
		observedTemplates = append(observedTemplates, c.RouteTemplate())
		c.String(http.StatusOK, "ok")
	})
	sub.GET("/products/:category/:id", func(c *Context) {
		observedTemplates = append(observedTemplates, c.RouteTemplate())
		c.String(http.StatusOK, "ok")
	})

	r := MustNew()
	r.Mount("/api/v1", sub)

	// Make requests
	requests := []string{
		"/api/v1/users/123",
		"/api/v1/products/electronics/456",
	}

	for _, path := range requests {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	}

	// Verify templates are full paths, not catch-all patterns
	expected := []string{
		"/api/v1/users/:id",
		"/api/v1/products/:category/:id",
	}
	assert.Equal(t, expected, observedTemplates,
		"route templates should be full paths for correct metrics/tracing")
}
