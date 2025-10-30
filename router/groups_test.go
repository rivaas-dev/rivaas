package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGroupPUT tests the PUT method on route groups
func TestGroupPUT(t *testing.T) {
	r := New()
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

	if !called {
		t.Error("PUT handler was not called")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	expected := `{"action":"updated","id":"123"}`
	got := w.Body.String()
	if got != expected+"\n" {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// TestGroupDELETE tests the DELETE method on route groups
func TestGroupDELETE(t *testing.T) {
	r := New()
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

	if !called {
		t.Error("DELETE handler was not called")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	expected := `{"action":"deleted","id":"456"}`
	got := w.Body.String()
	if got != expected+"\n" {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// TestGroupPATCH tests the PATCH method on route groups
func TestGroupPATCH(t *testing.T) {
	r := New()
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

	if !called {
		t.Error("PATCH handler was not called")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	expected := `{"action":"patched","id":"789"}`
	got := w.Body.String()
	if got != expected+"\n" {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// TestGroupOPTIONS tests the OPTIONS method on route groups
func TestGroupOPTIONS(t *testing.T) {
	r := New()
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

	if !called {
		t.Error("OPTIONS handler was not called")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Allow") == "" {
		t.Error("expected Allow header to be set")
	}
}

// TestGroupHEAD tests the HEAD method on route groups
func TestGroupHEAD(t *testing.T) {
	r := New()
	api := r.Group("/api")

	called := false
	api.HEAD("/users/:id", func(c *Context) {
		called = true
		id := c.Param("id")
		c.Header("X-User-ID", id)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodHead, "/api/users/999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !called {
		t.Error("HEAD handler was not called")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// HEAD should not return body
	if w.Body.Len() > 0 {
		t.Errorf("HEAD request should not have body, got %d bytes", w.Body.Len())
	}

	if w.Header().Get("X-User-ID") != "999" {
		t.Errorf("expected X-User-ID header to be 999, got %s", w.Header().Get("X-User-ID"))
	}
}

// TestGroupNestedGroups tests nested route groups with all HTTP methods
func TestGroupNestedGroups(t *testing.T) {
	r := New()

	api := r.Group("/api")
	v1 := api.Group("/v1")
	users := v1.Group("/users")

	tests := []struct {
		method  string
		path    string
		handler HandlerFunc
		action  string
	}{
		{http.MethodGet, "", func(c *Context) {
			c.JSON(http.StatusOK, map[string]string{"action": "list"})
		}, "list"},
		{http.MethodPost, "", func(c *Context) {
			c.JSON(http.StatusCreated, map[string]string{"action": "created"})
		}, "created"},
		{http.MethodPut, "/:id", func(c *Context) {
			c.JSON(http.StatusOK, map[string]string{"action": "updated", "id": c.Param("id")})
		}, "updated"},
		{http.MethodDelete, "/:id", func(c *Context) {
			c.JSON(http.StatusOK, map[string]string{"action": "deleted", "id": c.Param("id")})
		}, "deleted"},
		{http.MethodPatch, "/:id", func(c *Context) {
			c.JSON(http.StatusOK, map[string]string{"action": "patched", "id": c.Param("id")})
		}, "patched"},
		{http.MethodOptions, "", func(c *Context) {
			c.Status(http.StatusOK)
		}, "options"},
		{http.MethodHead, "/:id", func(c *Context) {
			c.Status(http.StatusOK)
		}, "head"},
	}

	for _, tt := range tests {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			switch tt.method {
			case http.MethodGet:
				users.GET(tt.path, tt.handler)
			case http.MethodPost:
				users.POST(tt.path, tt.handler)
			case http.MethodPut:
				users.PUT(tt.path, tt.handler)
			case http.MethodDelete:
				users.DELETE(tt.path, tt.handler)
			case http.MethodPatch:
				users.PATCH(tt.path, tt.handler)
			case http.MethodOptions:
				users.OPTIONS(tt.path, tt.handler)
			case http.MethodHead:
				users.HEAD(tt.path, tt.handler)
			}

			var url string
			if tt.path == "" {
				url = "/api/v1/users"
			} else {
				url = "/api/v1/users/100"
			}

			req := httptest.NewRequest(tt.method, url, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			expectedStatus := http.StatusOK
			if tt.method == http.MethodPost {
				expectedStatus = http.StatusCreated
			}

			if w.Code != expectedStatus {
				t.Errorf("expected status %d, got %d", expectedStatus, w.Code)
			}
		})
	}
}

// TestGroupMiddlewareInheritance tests that group middleware is properly inherited
func TestGroupMiddlewareInheritance(t *testing.T) {
	r := New()

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
	if len(executionOrder) != len(expected) {
		t.Fatalf("expected %d middleware/handlers, got %d: %v", len(expected), len(executionOrder), executionOrder)
	}

	for i, name := range expected {
		if executionOrder[i] != name {
			t.Errorf("execution order[%d]: expected %s, got %s", i, name, executionOrder[i])
		}
	}
}

// TestGroupWithAllHTTPMethods tests all HTTP methods work on groups
func TestGroupWithAllHTTPMethods(t *testing.T) {
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
			r := New()
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

			if !called {
				t.Errorf("%s handler was not called", m.name)
			}

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}
		})
	}
}

// TestGroupPUTWithMiddleware tests PUT with group-specific middleware
func TestGroupPUTWithMiddleware(t *testing.T) {
	r := New()

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

	if !middlewareCalled {
		t.Error("group middleware was not called")
	}

	if !handlerCalled {
		t.Error("PUT handler was not called")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestGroupDELETEWithMiddleware tests DELETE with group-specific middleware
func TestGroupDELETEWithMiddleware(t *testing.T) {
	r := New()

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

	if !authCalled {
		t.Error("auth middleware was not called")
	}

	if !handlerCalled {
		t.Error("DELETE handler was not called")
	}

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

// TestGroupPATCHWithParams tests PATCH with route parameters
func TestGroupPATCHWithParams(t *testing.T) {
	r := New()
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

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	expected := `{"id":"123","type":"users"}`
	got := w.Body.String()
	if got != expected+"\n" {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// TestGroupOPTIONSForCORS tests OPTIONS method for CORS preflight
func TestGroupOPTIONSForCORS(t *testing.T) {
	r := New()
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

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS headers to be set")
	}
}

// TestGroupHEADMatchesGET tests that HEAD requests work for HEAD-specific routes
func TestGroupHEADMatchesGET(t *testing.T) {
	r := New()
	api := r.Group("/api")

	api.HEAD("/status", func(c *Context) {
		c.Header("X-Status", "healthy")
		c.Header("X-Version", "1.0.0")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodHead, "/api/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Header().Get("X-Status") != "healthy" {
		t.Error("expected custom headers to be set")
	}

	// HEAD should not have response body
	if w.Body.Len() > 0 {
		t.Errorf("HEAD should have no body, got %d bytes", w.Body.Len())
	}
}

// TestGroupMultipleHTTPMethodsSamePath tests registering multiple methods on same path
func TestGroupMultipleHTTPMethodsSamePath(t *testing.T) {
	r := New()
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
	if !getCalled || putCalled || deleteCalled {
		t.Error("Only GET handler should be called for GET request")
	}
	getCalled = false

	// Test PUT
	req = httptest.NewRequest(http.MethodPut, "/api/item", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if getCalled || !putCalled || deleteCalled {
		t.Error("Only PUT handler should be called for PUT request")
	}
	putCalled = false

	// Test DELETE
	req = httptest.NewRequest(http.MethodDelete, "/api/item", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if getCalled || putCalled || !deleteCalled {
		t.Error("Only DELETE handler should be called for DELETE request")
	}
}

// TestGroupEmptyPath tests group with empty path parameter
func TestGroupEmptyPath(t *testing.T) {
	r := New()
	group := r.Group("/api")

	// Test all methods with empty path
	methods := []string{
		http.MethodGet, http.MethodPost, http.MethodPut,
		http.MethodDelete, http.MethodPatch, http.MethodOptions, http.MethodHead,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			handler := func(c *Context) {
				c.Status(http.StatusOK)
			}

			switch method {
			case http.MethodGet:
				group.GET("", handler)
			case http.MethodPost:
				group.POST("", handler)
			case http.MethodPut:
				group.PUT("", handler)
			case http.MethodDelete:
				group.DELETE("", handler)
			case http.MethodPatch:
				group.PATCH("", handler)
			case http.MethodOptions:
				group.OPTIONS("", handler)
			case http.MethodHead:
				group.HEAD("", handler)
			}

			req := httptest.NewRequest(method, "/api", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}
		})
	}
}

// TestGroupMethodsWithConstraints tests HTTP methods with route constraints
func TestGroupMethodsWithConstraints(t *testing.T) {
	r := New()
	api := r.Group("/api")

	// Test PUT with numeric constraint
	api.PUT("/users/:id", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
	}).WhereNumber("id")

	// Valid numeric ID
	req := httptest.NewRequest(http.MethodPut, "/api/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("numeric ID should match, got status %d", w.Code)
	}

	// Invalid non-numeric ID should 404
	req = httptest.NewRequest(http.MethodPut, "/api/users/abc", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("non-numeric ID should not match, got status %d", w.Code)
	}

	// Test DELETE with UUID constraint
	api.DELETE("/items/:uuid", func(c *Context) {
		c.NoContent()
	}).WhereUUID("uuid")

	// Valid UUID
	req = httptest.NewRequest(http.MethodDelete, "/api/items/550e8400-e29b-41d4-a716-446655440000", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("valid UUID should match, got status %d", w.Code)
	}

	// Invalid UUID should 404
	req = httptest.NewRequest(http.MethodDelete, "/api/items/not-a-uuid", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("invalid UUID should not match, got status %d", w.Code)
	}
}

// TestGroup_EmptyPrefix tests creating group with empty prefix
func TestGroup_EmptyPrefix(t *testing.T) {
	r := New()

	// Group with empty prefix
	g := r.Group("")

	g.GET("/test", func(c *Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Error("empty prefix group should work")
	}
}

// TestGroup_NestedEmptyPrefixes tests nested groups with empty prefixes
func TestGroup_NestedEmptyPrefixes(t *testing.T) {
	r := New()

	g1 := r.Group("")
	g2 := g1.Group("")
	g3 := g2.Group("/api")

	g3.GET("/test", func(c *Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Error("nested groups with empty prefixes should work")
	}
}

// TestGroup_WithMiddlewareInheritance tests middleware inheritance through nesting
func TestGroup_WithMiddlewareInheritance(t *testing.T) {
	r := New()

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

	if len(order) != 3 {
		t.Errorf("expected 3 executions, got %d: %v", len(order), order)
	}

	if order[0] != "g1" || order[1] != "g2" || order[2] != "handler" {
		t.Errorf("wrong execution order: %v", order)
	}
}
