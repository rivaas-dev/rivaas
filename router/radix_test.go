package router

import (
	"testing"
)

func TestRadixTree(t *testing.T) {
	root := &node{}

	// Add routes
	root.addRoute("/", []HandlerFunc{func(c *Context) {}})
	root.addRoute("/users", []HandlerFunc{func(c *Context) {}})
	root.addRoute("/users/:id", []HandlerFunc{func(c *Context) {}})
	root.addRoute("/users/:id/posts", []HandlerFunc{func(c *Context) {}})
	root.addRoute("/users/:id/posts/:post_id", []HandlerFunc{func(c *Context) {}})
	root.addRoute("/posts", []HandlerFunc{func(c *Context) {}})
	root.addRoute("/posts/:id", []HandlerFunc{func(c *Context) {}})

	// Test cases
	tests := []struct {
		path     string
		expected bool
		params   map[string]string
	}{
		{"/", true, map[string]string{}},
		{"/users", true, map[string]string{}},
		{"/users/123", true, map[string]string{"id": "123"}},
		{"/users/123/posts", true, map[string]string{"id": "123"}},
		{"/users/123/posts/456", true, map[string]string{"id": "123", "post_id": "456"}},
		{"/posts", true, map[string]string{}},
		{"/posts/789", true, map[string]string{"id": "789"}},
		{"/nonexistent", false, map[string]string{}},
		{"/users/123/posts/456/comments", false, map[string]string{}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			ctx := &Context{}
			handlers := root.getRoute(tt.path, ctx)

			if tt.expected {
				if handlers == nil {
					t.Errorf("Expected to find route for %s", tt.path)
				}

				// Check parameters
				for key, expectedValue := range tt.params {
					actualValue := ctx.Param(key)
					if actualValue != expectedValue {
						t.Errorf("Expected param %s=%s, got %s", key, expectedValue, actualValue)
					}
				}
			} else {
				if handlers != nil {
					t.Errorf("Expected no route for %s", tt.path)
				}
			}
		})
	}
}

func TestRadixTreeComplex(t *testing.T) {
	root := &node{}

	// Add complex routes
	root.addRoute("/api/v1/users/:id", []HandlerFunc{func(c *Context) {}})
	root.addRoute("/api/v1/users/:id/posts/:post_id", []HandlerFunc{func(c *Context) {}})
	root.addRoute("/api/v1/posts/:id/comments/:comment_id", []HandlerFunc{func(c *Context) {}})
	root.addRoute("/api/v2/users", []HandlerFunc{func(c *Context) {}})

	// Test cases
	tests := []struct {
		path     string
		expected bool
		params   map[string]string
	}{
		{"/api/v1/users/123", true, map[string]string{"id": "123"}},
		{"/api/v1/users/123/posts/456", true, map[string]string{"id": "123", "post_id": "456"}},
		{"/api/v1/posts/789/comments/101", true, map[string]string{"id": "789", "comment_id": "101"}},
		{"/api/v2/users", true, map[string]string{}},
		{"/api/v1/users", false, map[string]string{}},
		{"/api/v1/posts/789", false, map[string]string{}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			ctx := &Context{}
			handlers := root.getRoute(tt.path, ctx)

			if tt.expected {
				if handlers == nil {
					t.Errorf("Expected to find route for %s", tt.path)
				}

				// Check parameters
				for key, expectedValue := range tt.params {
					actualValue := ctx.Param(key)
					if actualValue != expectedValue {
						t.Errorf("Expected param %s=%s, got %s", key, expectedValue, actualValue)
					}
				}
			} else {
				if handlers != nil {
					t.Errorf("Expected no route for %s", tt.path)
				}
			}
		})
	}
}

func TestRadixTreeEdgeCases(t *testing.T) {
	root := &node{}

	// Add edge case routes
	root.addRoute("/", []HandlerFunc{func(c *Context) {}})
	root.addRoute("/a", []HandlerFunc{func(c *Context) {}})
	root.addRoute("/ab", []HandlerFunc{func(c *Context) {}})
	root.addRoute("/abc", []HandlerFunc{func(c *Context) {}})
	root.addRoute("/:param", []HandlerFunc{func(c *Context) {}})

	// Test cases
	tests := []struct {
		path     string
		expected bool
		params   map[string]string
	}{
		{"/", true, map[string]string{}},
		{"/a", true, map[string]string{}},
		{"/ab", true, map[string]string{}},
		{"/abc", true, map[string]string{}},
		{"/xyz", true, map[string]string{"param": "xyz"}},
		{"/", true, map[string]string{}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			ctx := &Context{}
			handlers := root.getRoute(tt.path, ctx)

			if tt.expected {
				if handlers == nil {
					t.Errorf("Expected to find route for %s", tt.path)
				}

				// Check parameters
				for key, expectedValue := range tt.params {
					actualValue := ctx.Param(key)
					if actualValue != expectedValue {
						t.Errorf("Expected param %s=%s, got %s", key, expectedValue, actualValue)
					}
				}
			} else {
				if handlers != nil {
					t.Errorf("Expected no route for %s", tt.path)
				}
			}
		})
	}
}
