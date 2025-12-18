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

// TestGroupPUT tests the PUT method on route groups
func TestGroupPUT(t *testing.T) {
	t.Parallel()
	r := MustNew()
	api := r.Group("/api")

	called := false
	api.PUT("/users/:id", func(c *Context) {
		called = true
		id := c.Param("id")
		c.JSON(http.StatusOK, map[string]string{"id": id, "action": "updated"})
	})

	req := httptest.NewRequest(http.MethodPut, "/api/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, called, "PUT handler was not called")
	assert.Equal(t, http.StatusOK, w.Code, "expected status 200")

	expected := `{"action":"updated","id":"123"}`
	assert.Equal(t, expected+"\n", w.Body.String(), "expected %q", expected)
}

// TestGroupDELETE tests the DELETE method on route groups
func TestGroupDELETE(t *testing.T) {
	t.Parallel()
	r := MustNew()
	api := r.Group("/api")

	called := false
	api.DELETE("/users/:id", func(c *Context) {
		called = true
		id := c.Param("id")
		c.JSON(http.StatusOK, map[string]string{"id": id, "action": "deleted"})
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/users/456", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, called, "DELETE handler was not called")
	assert.Equal(t, http.StatusOK, w.Code, "expected status 200")

	expected := `{"action":"deleted","id":"456"}`
	assert.Equal(t, expected+"\n", w.Body.String(), "expected %q", expected)
}

// TestGroupPATCH tests the PATCH method on route groups
func TestGroupPATCH(t *testing.T) {
	t.Parallel()
	r := MustNew()
	api := r.Group("/api")

	called := false
	api.PATCH("/users/:id", func(c *Context) {
		called = true
		id := c.Param("id")
		c.JSON(http.StatusOK, map[string]string{"id": id, "action": "patched"})
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/users/789", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, called, "PATCH handler was not called")
	assert.Equal(t, http.StatusOK, w.Code, "expected status 200")

	expected := `{"action":"patched","id":"789"}`
	assert.Equal(t, expected+"\n", w.Body.String(), "expected %q", expected)
}

// TestGroupOPTIONS tests the OPTIONS method on route groups
func TestGroupOPTIONS(t *testing.T) {
	t.Parallel()
	r := MustNew()
	api := r.Group("/api")

	called := false
	api.OPTIONS("/users", func(c *Context) {
		called = true
		c.Header("Allow", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, called, "OPTIONS handler was not called")
	assert.Equal(t, http.StatusOK, w.Code, "expected status 200")
	assert.NotEmpty(t, w.Header().Get("Allow"), "expected Allow header to be set")
}

// TestGroupHEAD tests the HEAD method on route groups
func TestGroupHEAD(t *testing.T) {
	t.Parallel()
	r := MustNew()
	api := r.Group("/api")

	called := false
	api.HEAD("/users/:id", func(c *Context) {
		called = true
		id := c.Param("id")
		c.Header("X-User-Id", id)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodHead, "/api/users/999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, called, "HEAD handler was not called")
	assert.Equal(t, http.StatusOK, w.Code, "expected status 200")
	assert.Equal(t, 0, w.Body.Len(), "HEAD request should not have body")
	assert.Equal(t, "999", w.Header().Get("X-User-Id"), "expected X-User-Id header to be 999")
}

// TestGroupNestedGroups tests nested route groups with all HTTP methods
func TestGroupNestedGroups(t *testing.T) {
	t.Parallel()
	r := MustNew()

	api := r.Group("/api")
	v1 := api.Group("/v1")
	users := v1.Group("/users")

	tests := []struct {
		method         string
		path           string
		action         string
		expectedStatus int
	}{
		{http.MethodGet, "", "list", http.StatusOK},
		{http.MethodPost, "", "created", http.StatusCreated},
		{http.MethodPut, "/:id", "updated", http.StatusOK},
		{http.MethodDelete, "/:id", "deleted", http.StatusOK},
		{http.MethodPatch, "/:id", "patched", http.StatusOK},
		{http.MethodOptions, "", "options", http.StatusOK},
		{http.MethodHead, "/:id", "head", http.StatusOK},
	}

	// Register all routes first (configuration phase)
	users.GET("", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"action": "list"})
	})
	users.POST("", func(c *Context) {
		c.JSON(http.StatusCreated, map[string]string{"action": "created"})
	})
	users.PUT("/:id", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"action": "updated", "id": c.Param("id")})
	})
	users.DELETE("/:id", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"action": "deleted", "id": c.Param("id")})
	})
	users.PATCH("/:id", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"action": "patched", "id": c.Param("id")})
	})
	users.OPTIONS("", func(c *Context) {
		c.Status(http.StatusOK)
	})
	users.HEAD("/:id", func(c *Context) {
		c.Status(http.StatusOK)
	})

	// Freeze before running parallel tests
	r.Freeze()

	// Now run tests (serving phase - can be parallel)
	for _, tt := range tests {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			t.Parallel()

			var url string
			if tt.path == "" {
				url = "/api/v1/users"
			} else {
				url = "/api/v1/users/100"
			}

			req := httptest.NewRequest(tt.method, url, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "expected status %d", tt.expectedStatus)
		})
	}
}

// TestGroupMiddlewareInheritance tests that group middleware is properly inherited
func TestGroupMiddlewareInheritance(t *testing.T) {
	t.Parallel()
	r := MustNew()

	var executionOrder []string

	globalMiddleware := func(c *Context) {
		executionOrder = append(executionOrder, "global")
		c.Next()
	}

	apiMiddleware := func(c *Context) {
		executionOrder = append(executionOrder, "api")
		c.Next()
	}

	v1Middleware := func(c *Context) {
		executionOrder = append(executionOrder, "v1")
		c.Next()
	}

	handler := func(c *Context) {
		executionOrder = append(executionOrder, "handler")
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	}

	r.Use(globalMiddleware)
	api := r.Group("/api", apiMiddleware)
	v1 := api.Group("/v1", v1Middleware)

	v1.PUT("/resource", handler)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/resource", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	expected := []string{"global", "api", "v1", "handler"}
	require.Len(t, executionOrder, len(expected), "expected %d middleware/handlers, got %d: %v", len(expected), len(executionOrder), executionOrder)

	for i, name := range expected {
		assert.Equal(t, name, executionOrder[i], "execution order[%d]", i)
	}
}

// TestGroupWithAllHTTPMethods tests all HTTP methods work on groups
func TestGroupWithAllHTTPMethods(t *testing.T) {
	t.Parallel()
	methods := []struct {
		name   string
		method string
		path   string
	}{
		{"GET", http.MethodGet, "/items"},
		{"POST", http.MethodPost, "/items"},
		{"PUT", http.MethodPut, "/items/1"},
		{"DELETE", http.MethodDelete, "/items/1"},
		{"PATCH", http.MethodPatch, "/items/1"},
		{"OPTIONS", http.MethodOptions, "/items"},
		{"HEAD", http.MethodHead, "/items/1"},
	}

	for _, m := range methods {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			r := MustNew()
			group := r.Group("/api")

			called := false
			handler := func(c *Context) {
				called = true
				c.Status(http.StatusOK)
			}

			// Register route based on method
			switch m.method {
			case http.MethodGet:
				group.GET(m.path, handler)
			case http.MethodPost:
				group.POST(m.path, handler)
			case http.MethodPut:
				group.PUT(m.path, handler)
			case http.MethodDelete:
				group.DELETE(m.path, handler)
			case http.MethodPatch:
				group.PATCH(m.path, handler)
			case http.MethodOptions:
				group.OPTIONS(m.path, handler)
			case http.MethodHead:
				group.HEAD(m.path, handler)
			}

			req := httptest.NewRequest(m.method, "/api"+m.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.True(t, called, "%s handler was not called", m.name)
			assert.Equal(t, http.StatusOK, w.Code, "expected status 200")
		})
	}
}

// TestGroupPUTWithMiddleware tests PUT with group-specific middleware
func TestGroupPUTWithMiddleware(t *testing.T) {
	t.Parallel()
	r := MustNew()

	middlewareCalled := false
	handlerCalled := false

	middleware := func(c *Context) {
		middlewareCalled = true
		c.Next()
	}

	api := r.Group("/api", middleware)
	api.PUT("/resource/:id", func(c *Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
	})

	req := httptest.NewRequest(http.MethodPut, "/api/resource/42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, middlewareCalled, "group middleware was not called")
	assert.True(t, handlerCalled, "PUT handler was not called")
	assert.Equal(t, http.StatusOK, w.Code, "expected status 200")
}

// TestGroupDELETEWithMiddleware tests DELETE with group-specific middleware
func TestGroupDELETEWithMiddleware(t *testing.T) {
	t.Parallel()
	r := MustNew()

	authCalled := false
	handlerCalled := false

	authMiddleware := func(c *Context) {
		authCalled = true
		c.Next()
	}

	admin := r.Group("/admin", authMiddleware)
	admin.DELETE("/posts/:id", func(c *Context) {
		handlerCalled = true
		c.NoContent()
	})

	req := httptest.NewRequest(http.MethodDelete, "/admin/posts/99", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, authCalled, "auth middleware was not called")
	assert.True(t, handlerCalled, "DELETE handler was not called")
	assert.Equal(t, http.StatusNoContent, w.Code, "expected status 204")
}

// TestGroupPATCHWithParams tests PATCH with route parameters
func TestGroupPATCHWithParams(t *testing.T) {
	t.Parallel()
	r := MustNew()
	resources := r.Group("/resources")

	resources.PATCH("/:type/:id", func(c *Context) {
		resourceType := c.Param("type")
		id := c.Param("id")
		c.JSON(http.StatusOK, map[string]string{
			"type": resourceType,
			"id":   id,
		})
	})

	req := httptest.NewRequest(http.MethodPatch, "/resources/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "expected status 200")

	expected := `{"id":"123","type":"users"}`
	assert.Equal(t, expected+"\n", w.Body.String(), "expected %q", expected)
}

// TestGroupOPTIONSForCORS tests OPTIONS method for CORS preflight
func TestGroupOPTIONSForCORS(t *testing.T) {
	t.Parallel()
	r := MustNew()
	api := r.Group("/api")

	api.OPTIONS("/*", func(c *Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/anything/here", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "expected status 200")
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"), "expected CORS headers to be set")
}

// TestGroupHEADMatchesGET tests that HEAD requests work for HEAD-specific routes
func TestGroupHEADMatchesGET(t *testing.T) {
	t.Parallel()
	r := MustNew()
	api := r.Group("/api")

	api.HEAD("/status", func(c *Context) {
		c.Header("X-Status", "healthy")
		c.Header("X-Version", "1.0.0")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodHead, "/api/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "expected status 200")
	assert.Equal(t, "healthy", w.Header().Get("X-Status"), "expected custom headers to be set")
	assert.Equal(t, 0, w.Body.Len(), "HEAD should have no body")
}

// TestGroupMultipleHTTPMethodsSamePath tests registering multiple methods on same path
func TestGroupMultipleHTTPMethodsSamePath(t *testing.T) {
	t.Parallel()
	r := MustNew()
	api := r.Group("/api")

	getCalled := false
	putCalled := false
	deleteCalled := false

	api.GET("/item", func(c *Context) {
		getCalled = true
		c.JSON(http.StatusOK, map[string]string{"action": "get"})
	})

	api.PUT("/item", func(c *Context) {
		putCalled = true
		c.JSON(http.StatusOK, map[string]string{"action": "put"})
	})

	api.DELETE("/item", func(c *Context) {
		deleteCalled = true
		c.NoContent()
	})

	// Test GET
	req := httptest.NewRequest(http.MethodGet, "/api/item", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.True(t, getCalled && !putCalled && !deleteCalled, "Only GET handler should be called for GET request")
	getCalled = false

	// Test PUT
	req = httptest.NewRequest(http.MethodPut, "/api/item", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.True(t, !getCalled && putCalled && !deleteCalled, "Only PUT handler should be called for PUT request")
	putCalled = false

	// Test DELETE
	req = httptest.NewRequest(http.MethodDelete, "/api/item", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.True(t, !getCalled && !putCalled && deleteCalled, "Only DELETE handler should be called for DELETE request")
}

// TestGroupEmptyPath tests group with empty path parameter
func TestGroupEmptyPath(t *testing.T) {
	t.Parallel()
	r := MustNew()
	group := r.Group("/api")

	// Test all methods with empty path
	methods := []string{
		http.MethodGet, http.MethodPost, http.MethodPut,
		http.MethodDelete, http.MethodPatch, http.MethodOptions, http.MethodHead,
	}

	handler := func(c *Context) {
		c.Status(http.StatusOK)
	}

	// Register all routes first (configuration phase)
	group.GET("", handler)
	group.POST("", handler)
	group.PUT("", handler)
	group.DELETE("", handler)
	group.PATCH("", handler)
	group.OPTIONS("", handler)
	group.HEAD("", handler)

	// Freeze the router before running parallel subtests
	r.Freeze()

	// Now test each method (serving phase - can run in parallel)
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(method, "/api", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "expected status 200")
		})
	}
}

// TestGroupMethodsWithConstraints tests HTTP methods with route constraints
func TestGroupMethodsWithConstraints(t *testing.T) {
	t.Parallel()
	r := MustNew()
	api := r.Group("/api")

	// Register all routes before serving (two-phase design)
	// Test PUT with numeric constraint
	api.PUT("/users/:id", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
	}).WhereInt("id")

	// Test DELETE with UUID constraint
	api.DELETE("/items/:uuid", func(c *Context) {
		c.NoContent()
	}).WhereUUID("uuid")

	// Now test the routes
	t.Run("PUT with valid numeric ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/users/123", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "numeric ID should match")
	})

	t.Run("PUT with invalid non-numeric ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/users/abc", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code, "non-numeric ID should not match")
	})

	t.Run("DELETE with valid UUID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/items/550e8400-e29b-41d4-a716-446655440000", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNoContent, w.Code, "valid UUID should match")
	})

	t.Run("DELETE with invalid UUID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/items/not-a-uuid", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code, "invalid UUID should not match")
	})
}

// TestGroup_EmptyPrefix tests creating group with empty prefix
func TestGroup_EmptyPrefix(t *testing.T) {
	t.Parallel()
	r := MustNew()

	// Group with empty prefix
	g := r.Group("")

	g.GET("/test", func(c *Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "empty prefix group should work")
}

// TestGroup_NestedEmptyPrefixes tests nested groups with empty prefixes
func TestGroup_NestedEmptyPrefixes(t *testing.T) {
	t.Parallel()
	r := MustNew()

	g1 := r.Group("")
	g2 := g1.Group("")
	g3 := g2.Group("/api")

	g3.GET("/test", func(c *Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "nested groups with empty prefixes should work")
}

// TestGroup_WithMiddlewareInheritance tests middleware inheritance through nesting
func TestGroup_WithMiddlewareInheritance(t *testing.T) {
	t.Parallel()
	r := MustNew()

	var order []string

	g1 := r.Group("/g1", func(c *Context) {
		order = append(order, "g1")
		c.Next()
	})

	g2 := g1.Group("/g2", func(c *Context) {
		order = append(order, "g2")
		c.Next()
	})

	g2.GET("/test", func(c *Context) {
		order = append(order, "handler")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/g1/g2/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Len(t, order, 3, "expected 3 executions, got %d: %v", len(order), order)
	assert.Equal(t, []string{"g1", "g2", "handler"}, order, "wrong execution order")
}

// TestGroup_EmptyPrefixOnNestedGroup tests creating a nested group with empty prefix
// when parent has a prefix. This covers the case where fullPrefix = g.prefix.
func TestGroup_EmptyPrefixOnNestedGroup(t *testing.T) {
	t.Parallel()
	r := MustNew()

	// Parent group with prefix
	api := r.Group("/api")

	// Nested group with empty prefix - should inherit parent's prefix
	nested := api.Group("")

	nested.GET("/users", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"path": "users"})
	})

	// Test that routes added to nested group work correctly
	// Note: All routes must be registered before the first ServeHTTP call
	nested.POST("/posts", func(c *Context) {
		c.JSON(http.StatusCreated, map[string]string{"path": "posts"})
	})

	// Now test both routes after all registration is complete
	t.Run("GET /api/users", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "expected status 200")
	})

	t.Run("POST /api/posts", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/posts", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code, "expected status 201")
	})
}
