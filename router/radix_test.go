package router

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

// RadixTestSuite tests radix tree functionality
type RadixTestSuite struct {
	suite.Suite
	root *node
}

func (suite *RadixTestSuite) SetupTest() {
	suite.root = &node{}
}

// TestRadixTree tests basic radix tree functionality
func (suite *RadixTestSuite) TestRadixTree() {
	// Add routes
	suite.root.addRouteWithConstraints("/", []HandlerFunc{func(c *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/users", []HandlerFunc{func(c *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/users/:id", []HandlerFunc{func(c *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/users/:id/posts", []HandlerFunc{func(c *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/users/:id/posts/:post_id", []HandlerFunc{func(c *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/posts", []HandlerFunc{func(c *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/posts/:id", []HandlerFunc{func(c *Context) {}}, nil)

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
		suite.Run(tt.path, func() {
			ctx := &Context{}
			handlers := suite.root.getRoute(tt.path, ctx)

			if tt.expected {
				suite.NotNil(handlers, "Expected to find route for %s", tt.path)

				// Check parameters
				for key, expectedValue := range tt.params {
					actualValue := ctx.Param(key)
					suite.Equal(expectedValue, actualValue, "Expected param %s=%s, got %s", key, expectedValue, actualValue)
				}
			} else {
				suite.Nil(handlers, "Expected no route for %s", tt.path)
			}
		})
	}
}

// TestRadixTreeComplex tests complex radix tree scenarios
func (suite *RadixTestSuite) TestRadixTreeComplex() {
	// Add complex routes
	suite.root.addRouteWithConstraints("/api/v1/users/:id", []HandlerFunc{func(c *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/api/v1/users/:id/posts/:post_id", []HandlerFunc{func(c *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/api/v1/posts/:id/comments/:comment_id", []HandlerFunc{func(c *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/api/v2/users", []HandlerFunc{func(c *Context) {}}, nil)

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
		suite.Run(tt.path, func() {
			ctx := &Context{}
			handlers := suite.root.getRoute(tt.path, ctx)

			if tt.expected {
				suite.NotNil(handlers, "Expected to find route for %s", tt.path)

				// Check parameters
				for key, expectedValue := range tt.params {
					actualValue := ctx.Param(key)
					suite.Equal(expectedValue, actualValue, "Expected param %s=%s, got %s", key, expectedValue, actualValue)
				}
			} else {
				suite.Nil(handlers, "Expected no route for %s", tt.path)
			}
		})
	}
}

// TestRadixTreeEdgeCases tests edge cases for radix tree
func (suite *RadixTestSuite) TestRadixTreeEdgeCases() {
	// Add edge case routes
	suite.root.addRouteWithConstraints("/", []HandlerFunc{func(c *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/a", []HandlerFunc{func(c *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/ab", []HandlerFunc{func(c *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/abc", []HandlerFunc{func(c *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/:param", []HandlerFunc{func(c *Context) {}}, nil)

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
		suite.Run(tt.path, func() {
			ctx := &Context{}
			handlers := suite.root.getRoute(tt.path, ctx)

			if tt.expected {
				suite.NotNil(handlers, "Expected to find route for %s", tt.path)

				// Check parameters
				for key, expectedValue := range tt.params {
					actualValue := ctx.Param(key)
					suite.Equal(expectedValue, actualValue, "Expected param %s=%s, got %s", key, expectedValue, actualValue)
				}
			} else {
				suite.Nil(handlers, "Expected no route for %s", tt.path)
			}
		})
	}
}

// TestRadixSuite runs the radix test suite
func TestRadixSuite(t *testing.T) {
	suite.Run(t, new(RadixTestSuite))
}

// TestEdgeCasesInRadixTree tests edge cases in radix tree matching
func TestEdgeCasesInRadixTree(t *testing.T) {
	r := New()

	t.Run("Empty segments", func(t *testing.T) {
		r.GET("/a//b", func(c *Context) {
			c.String(200, "ok")
		})

		req := httptest.NewRequest("GET", "/a//b", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("Trailing slash handling", func(t *testing.T) {
		r.GET("/users/", func(c *Context) {
			c.String(200, "users with slash")
		})
		r.GET("/posts", func(c *Context) {
			c.String(200, "posts without slash")
		})

		// Test exact match with trailing slash
		req := httptest.NewRequest("GET", "/users/", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		if w.Body.String() != "users with slash" {
			t.Errorf("Expected 'users with slash', got %q", w.Body.String())
		}

		// Test exact match without trailing slash
		req = httptest.NewRequest("GET", "/posts", nil)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		if w.Body.String() != "posts without slash" {
			t.Errorf("Expected 'posts without slash', got %q", w.Body.String())
		}
	})

	t.Run("Multiple parameters in path", func(t *testing.T) {
		r.GET("/a/:p1/b/:p2/c/:p3", func(c *Context) {
			c.String(200, "%s-%s-%s", c.Param("p1"), c.Param("p2"), c.Param("p3"))
		})

		req := httptest.NewRequest("GET", "/a/x/b/y/c/z", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		if w.Body.String() != "x-y-z" {
			t.Errorf("Expected 'x-y-z', got %q", w.Body.String())
		}
	})

	t.Run("Parameter at end of path", func(t *testing.T) {
		r.GET("/items/:id", func(c *Context) {
			c.String(200, "item %s", c.Param("id"))
		})

		req := httptest.NewRequest("GET", "/items/abc123", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		if w.Body.String() != "item abc123" {
			t.Errorf("Expected 'item abc123', got %q", w.Body.String())
		}
	})
}
