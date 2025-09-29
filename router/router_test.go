package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouter(t *testing.T) {
	r := New()

	// Test basic routes
	r.GET("/", func(c *Context) {
		c.String(http.StatusOK, "Hello World")
	})

	r.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "User: %s", c.Param("id"))
	})

	r.POST("/users", func(c *Context) {
		c.String(http.StatusCreated, "User created")
	})

	// Test cases
	tests := []struct {
		method string
		path   string
		status int
		body   string
	}{
		{"GET", "/", 200, "Hello World"},
		{"GET", "/users/123", 200, "User: 123"},
		{"POST", "/users", 201, "User created"},
		{"GET", "/users/123/posts/456", 404, ""},
		{"GET", "/nonexistent", 404, ""},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.status {
				t.Errorf("Expected status %d, got %d", tt.status, w.Code)
			}

			if tt.body != "" && w.Body.String() != tt.body {
				t.Errorf("Expected body %q, got %q", tt.body, w.Body.String())
			}
		})
	}
}

func TestRouterWithMiddleware(t *testing.T) {
	r := New()

	// Add middleware
	r.Use(func(c *Context) {
		c.Header("X-Middleware", "true")
		c.Next()
	})

	r.GET("/", func(c *Context) {
		c.String(http.StatusOK, "Hello")
	})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("X-Middleware") != "true" {
		t.Errorf("Expected X-Middleware header")
	}
}

func TestRouterGroup(t *testing.T) {
	r := New()

	// Create a group
	api := r.Group("/api/v1")
	api.GET("/users", func(c *Context) {
		c.String(http.StatusOK, "Users")
	})

	api.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "User: %s", c.Param("id"))
	})

	// Test cases
	tests := []struct {
		path   string
		status int
		body   string
	}{
		{"/api/v1/users", 200, "Users"},
		{"/api/v1/users/123", 200, "User: 123"},
		{"/users", 404, ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.status {
				t.Errorf("Expected status %d, got %d", tt.status, w.Code)
			}

			if tt.body != "" && w.Body.String() != tt.body {
				t.Errorf("Expected body %q, got %q", tt.body, w.Body.String())
			}
		})
	}
}

func TestRouterGroupMiddleware(t *testing.T) {
	r := New()

	// Create a group with middleware
	api := r.Group("/api/v1")
	api.Use(func(c *Context) {
		c.Header("X-API-Version", "v1")
		c.Next()
	})

	api.GET("/users", func(c *Context) {
		c.String(http.StatusOK, "Users")
	})

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("X-API-Version") != "v1" {
		t.Errorf("Expected X-API-Version header")
	}
}

func TestRouterComplexRoutes(t *testing.T) {
	r := New()

	r.GET("/users/:id/posts/:post_id", func(c *Context) {
		c.String(http.StatusOK, "User: %s, Post: %s", c.Param("id"), c.Param("post_id"))
	})

	r.GET("/users/:id/posts/:post_id/comments/:comment_id", func(c *Context) {
		c.String(http.StatusOK, "User: %s, Post: %s, Comment: %s",
			c.Param("id"), c.Param("post_id"), c.Param("comment_id"))
	})

	// Test cases
	tests := []struct {
		path   string
		status int
		body   string
	}{
		{"/users/123/posts/456", 200, "User: 123, Post: 456"},
		{"/users/123/posts/456/comments/789", 200, "User: 123, Post: 456, Comment: 789"},
		{"/users/123/posts", 404, ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.status {
				t.Errorf("Expected status %d, got %d", tt.status, w.Code)
			}

			if tt.body != "" && w.Body.String() != tt.body {
				t.Errorf("Expected body %q, got %q", tt.body, w.Body.String())
			}
		})
	}
}

func TestContextMethods(t *testing.T) {
	r := New()

	r.GET("/test", func(c *Context) {
		// Test JSON response
		c.JSON(http.StatusOK, map[string]string{"message": "test"})
	})

	r.GET("/string", func(c *Context) {
		// Test String response
		c.String(http.StatusOK, "Hello %s", "World")
	})

	r.GET("/html", func(c *Context) {
		// Test HTML response
		c.HTML(http.StatusOK, "<h1>Hello</h1>")
	})

	// Test JSON
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected JSON content type")
	}

	// Test String
	req = httptest.NewRequest("GET", "/string", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Body.String() != "Hello World" {
		t.Errorf("Expected 'Hello World', got %q", w.Body.String())
	}

	// Test HTML
	req = httptest.NewRequest("GET", "/html", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Header().Get("Content-Type") != "text/html" {
		t.Errorf("Expected HTML content type")
	}
}
