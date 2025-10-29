package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

// RouterTestSuite is the main test suite for router functionality
type RouterTestSuite struct {
	suite.Suite
	router *Router
}

// SetupTest runs before each individual test
func (suite *RouterTestSuite) SetupTest() {
	suite.router = New()
}

// TearDownTest runs after each individual test
func (suite *RouterTestSuite) TearDownTest() {
	if suite.router != nil {
		// Cleanup if needed
	}
}

// TestBasicRouting tests basic HTTP method routing
func (suite *RouterTestSuite) TestBasicRouting() {
	// Test basic routes
	suite.router.GET("/", func(c *Context) {
		c.String(http.StatusOK, "Hello World")
	})

	suite.router.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "User: %s", c.Param("id"))
	})

	suite.router.POST("/users", func(c *Context) {
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
		suite.Run(tt.method+" "+tt.path, func() {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			suite.router.ServeHTTP(w, req)

			suite.Equal(tt.status, w.Code, "Status code mismatch for %s %s", tt.method, tt.path)
			if tt.body != "" {
				suite.Equal(tt.body, w.Body.String(), "Body mismatch for %s %s", tt.method, tt.path)
			}
		})
	}
}

// TestRouterWithMiddleware tests middleware functionality
func (suite *RouterTestSuite) TestRouterWithMiddleware() {
	// Add middleware
	suite.router.Use(func(c *Context) {
		c.Header("X-Middleware", "true")
		c.Next()
	})

	suite.router.GET("/", func(c *Context) {
		c.String(http.StatusOK, "Hello")
	})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	suite.router.ServeHTTP(w, req)

	suite.Equal(200, w.Code)
	suite.Equal("true", w.Header().Get("X-Middleware"))
}

// TestRouterGroup tests route grouping functionality
func (suite *RouterTestSuite) TestRouterGroup() {
	// Create a group
	api := suite.router.Group("/api/v1")
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
		suite.Run(tt.path, func() {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			suite.router.ServeHTTP(w, req)

			suite.Equal(tt.status, w.Code)
			if tt.body != "" {
				suite.Equal(tt.body, w.Body.String())
			}
		})
	}
}

// TestRouterGroupMiddleware tests middleware on route groups
func (suite *RouterTestSuite) TestRouterGroupMiddleware() {
	// Create a group with middleware
	api := suite.router.Group("/api/v1")
	api.Use(func(c *Context) {
		c.Header("X-API-Version", "v1")
		c.Next()
	})

	api.GET("/users", func(c *Context) {
		c.String(http.StatusOK, "Users")
	})

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	w := httptest.NewRecorder()

	suite.router.ServeHTTP(w, req)

	suite.Equal(200, w.Code)
	suite.Equal("v1", w.Header().Get("X-API-Version"))
}

// TestRouterComplexRoutes tests complex route patterns
func (suite *RouterTestSuite) TestRouterComplexRoutes() {
	suite.router.GET("/users/:id/posts/:post_id", func(c *Context) {
		c.String(http.StatusOK, "User: %s, Post: %s", c.Param("id"), c.Param("post_id"))
	})

	suite.router.GET("/users/:id/posts/:post_id/comments/:comment_id", func(c *Context) {
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
		suite.Run(tt.path, func() {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			suite.router.ServeHTTP(w, req)

			suite.Equal(tt.status, w.Code)
			if tt.body != "" {
				suite.Equal(tt.body, w.Body.String())
			}
		})
	}
}

// TestContextMethods tests various context methods
func (suite *RouterTestSuite) TestContextMethods() {
	suite.router.GET("/test", func(c *Context) {
		// Test JSON response
		c.JSON(http.StatusOK, map[string]string{"message": "test"})
	})

	suite.router.GET("/string", func(c *Context) {
		// Test String response
		c.String(http.StatusOK, "Hello %s", "World")
	})

	suite.router.GET("/html", func(c *Context) {
		// Test HTML response
		c.HTML(http.StatusOK, "<h1>Hello</h1>")
	})

	// Test JSON
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)
	suite.Equal("application/json; charset=utf-8", w.Header().Get("Content-Type"))

	// Test String
	req = httptest.NewRequest("GET", "/string", nil)
	w = httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)
	suite.Equal("Hello World", w.Body.String())

	// Test HTML
	req = httptest.NewRequest("GET", "/html", nil)
	w = httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)
	suite.Equal("text/html", w.Header().Get("Content-Type"))
}

// TestRouterSuite runs the router test suite
func TestRouterSuite(t *testing.T) {
	suite.Run(t, new(RouterTestSuite))
}

// TestHTTPMethods tests all HTTP method handlers
func TestHTTPMethods(t *testing.T) {
	r := New()

	// Register all HTTP methods
	r.GET("/get", func(c *Context) {
		c.String(200, "GET")
	})
	r.POST("/post", func(c *Context) {
		c.String(200, "POST")
	})
	r.PUT("/put", func(c *Context) {
		c.String(200, "PUT")
	})
	r.DELETE("/delete", func(c *Context) {
		c.String(200, "DELETE")
	})
	r.PATCH("/patch", func(c *Context) {
		c.String(200, "PATCH")
	})
	r.OPTIONS("/options", func(c *Context) {
		c.String(200, "OPTIONS")
	})
	r.HEAD("/head", func(c *Context) {
		c.Status(200)
	})

	tests := []struct {
		method   string
		path     string
		expected string
	}{
		{"GET", "/get", "GET"},
		{"POST", "/post", "POST"},
		{"PUT", "/put", "PUT"},
		{"DELETE", "/delete", "DELETE"},
		{"PATCH", "/patch", "PATCH"},
		{"OPTIONS", "/options", "OPTIONS"},
		{"HEAD", "/head", ""},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != 200 {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
			if tt.expected != "" && w.Body.String() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, w.Body.String())
			}
		})
	}
}

// TestCompileOptimizations tests route compilation and optimization
func TestCompileOptimizations(t *testing.T) {
	r := New()

	// Add static routes that will be compiled
	r.GET("/home", func(c *Context) {
		c.String(200, "home")
	})
	r.GET("/about", func(c *Context) {
		c.String(200, "about")
	})
	r.GET("/contact", func(c *Context) {
		c.String(200, "contact")
	})

	// Trigger compilation
	r.WarmupOptimizations()

	// Test that compiled routes work
	req := httptest.NewRequest("GET", "/home", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "home" {
		t.Errorf("Expected 'home', got %q", w.Body.String())
	}
}

// TestWithBloomFilterSize tests bloom filter configuration
func TestWithBloomFilterSize(t *testing.T) {
	r := New(WithBloomFilterSize(2000))

	if r.bloomFilterSize != 2000 {
		t.Errorf("Expected bloom filter size 2000, got %d", r.bloomFilterSize)
	}

	// Test with zero size (should use default)
	r2 := New(WithBloomFilterSize(0))
	if r2.bloomFilterSize != 1000 {
		t.Errorf("Expected default bloom filter size 1000, got %d", r2.bloomFilterSize)
	}
}

// mockLogger implements the Logger interface for testing
type mockLogger struct {
	lastError string
}

func (m *mockLogger) Error(msg string, keysAndValues ...any) {
	m.lastError = msg
}

func (m *mockLogger) Warn(msg string, keysAndValues ...any) {}

func (m *mockLogger) Info(msg string, keysAndValues ...any) {}

func (m *mockLogger) Debug(msg string, keysAndValues ...any) {}

// TestWithLogger tests logger configuration
func TestWithLogger(t *testing.T) {
	logger := &mockLogger{}
	r := New(WithLogger(logger))

	if r.logger == nil {
		t.Error("Expected logger to be set")
	}
}
