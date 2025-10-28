package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

// ExtendedTestSuite tests extended router functionality
type ExtendedTestSuite struct {
	suite.Suite
	router *Router
}

func (suite *ExtendedTestSuite) SetupTest() {
	suite.router = New()
}

func (suite *ExtendedTestSuite) TearDownTest() {
	if suite.router != nil {
		// Cleanup if needed
	}
}

// TestRouteIntrospection tests route introspection functionality
func (suite *ExtendedTestSuite) TestRouteIntrospection() {
	// Add some routes
	suite.router.GET("/", func(c *Context) { c.String(200, "home") })
	suite.router.GET("/users/:id", func(c *Context) { c.String(200, "user") })
	suite.router.POST("/users", func(c *Context) { c.String(200, "create") })

	// Test Routes() method
	routes := suite.router.Routes()
	suite.Equal(3, len(routes), "Expected 3 routes")

	// Check if routes are sorted
	suite.Equal("GET", routes[0].Method, "Expected first route method to be GET")
	suite.Equal("/", routes[0].Path, "Expected first route path to be /")

	// Test that we can find our routes
	found := false
	for _, route := range routes {
		if route.Method == "POST" && route.Path == "/users" {
			found = true
			break
		}
	}
	suite.True(found, "Expected to find POST /users route")
}

func (suite *ExtendedTestSuite) TestRequestHelpers() {
	r := New()

	r.GET("/test", func(c *Context) {
		// Test content type detection
		if c.IsJSON() {
			c.String(200, "json")
		} else if c.IsXML() {
			c.String(200, "xml")
		} else {
			c.String(200, "other")
		}
	})

	tests := []struct {
		contentType string
		expected    string
	}{
		{"application/json", "json"},
		{"application/xml", "xml"},
		{"text/xml", "xml"},
		{"text/plain", "other"},
	}

	for _, test := range tests {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Content-Type", test.contentType)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		suite.Equal(test.expected, w.Body.String(), "Expected %s for content type %s, got %s", test.expected, test.contentType, w.Body.String())
	}
}

func (suite *ExtendedTestSuite) TestAcceptsHelpers() {
	r := New()

	r.GET("/test", func(c *Context) {
		if c.AcceptsJSON() {
			c.JSON(200, map[string]string{"type": "json"})
		} else if c.AcceptsHTML() {
			c.HTML(200, "<h1>html</h1>")
		} else {
			c.String(200, "other")
		}
	})

	tests := []struct {
		accept      string
		contentType string
	}{
		{"application/json", "application/json"},
		{"text/html", "text/html"},
		{"*/*", "application/json"}, // Should default to JSON for */*
	}

	for _, test := range tests {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Accept", test.accept)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		suite.Contains(w.Header().Get("Content-Type"), test.contentType, "Expected content type %s for accept %s, got %s", test.contentType, test.accept, w.Header().Get("Content-Type"))
	}
}

func (suite *ExtendedTestSuite) TestClientIP() {
	r := New()

	r.GET("/ip", func(c *Context) {
		c.String(200, "%s", c.ClientIP())
	})

	tests := []struct {
		name           string
		remoteAddr     string
		xForwardedFor  string
		xRealIP        string
		expectedPrefix string
	}{
		{
			name:           "Direct connection",
			remoteAddr:     "192.168.1.1:8080",
			expectedPrefix: "192.168.1.1",
		},
		{
			name:           "X-Forwarded-For single",
			remoteAddr:     "10.0.0.1:8080",
			xForwardedFor:  "203.0.113.195",
			expectedPrefix: "203.0.113.195",
		},
		{
			name:           "X-Forwarded-For multiple",
			remoteAddr:     "10.0.0.1:8080",
			xForwardedFor:  "203.0.113.195, 70.41.3.18, 150.172.238.178",
			expectedPrefix: "203.0.113.195",
		},
		{
			name:           "X-Real-IP",
			remoteAddr:     "10.0.0.1:8080",
			xRealIP:        "203.0.113.200",
			expectedPrefix: "203.0.113.200",
		},
	}

	for _, test := range tests {
		suite.Run(test.name, func() {
			req := httptest.NewRequest("GET", "/ip", nil)
			req.RemoteAddr = test.remoteAddr
			if test.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", test.xForwardedFor)
			}
			if test.xRealIP != "" {
				req.Header.Set("X-Real-IP", test.xRealIP)
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			suite.True(strings.HasPrefix(w.Body.String(), test.expectedPrefix), "Expected IP to start with %s, got %s", test.expectedPrefix, w.Body.String())
		})
	}
}

func (suite *ExtendedTestSuite) TestRedirect() {
	r := New()

	r.GET("/redirect", func(c *Context) {
		c.Redirect(http.StatusFound, "/target")
	})

	req := httptest.NewRequest("GET", "/redirect", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	suite.Equal(http.StatusFound, w.Code, "Expected status %d, got %d", http.StatusFound, w.Code)
	suite.Equal("/target", w.Header().Get("Location"), "Expected Location header '/target', got '%s'", w.Header().Get("Location"))
}

func (suite *ExtendedTestSuite) TestRouteConstraints() {
	r := New()

	// Add route with numeric constraint
	r.GET("/users/:id", func(c *Context) {
		c.String(200, "user %s", c.Param("id"))
	}).WhereNumber("id")

	// Add route with custom constraint
	r.GET("/files/:name", func(c *Context) {
		c.String(200, "file %s", c.Param("name"))
	}).Where("name", `[a-zA-Z0-9._-]+`)

	tests := []struct {
		path       string
		statusCode int
		contains   string
	}{
		{"/users/123", 200, "user 123"},      // Valid numeric
		{"/users/abc", 404, ""},              // Invalid numeric
		{"/files/document.pdf", 200, "file"}, // Valid filename
		{"/files/bad@file.txt", 404, ""},     // Invalid filename (contains @)
	}

	for _, test := range tests {
		req := httptest.NewRequest("GET", test.path, nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		suite.Equal(test.statusCode, w.Code, "Path %s: expected status %d, got %d", test.path, test.statusCode, w.Code)

		if test.contains != "" {
			suite.Contains(w.Body.String(), test.contains, "Path %s: expected body to contain '%s', got '%s'", test.path, test.contains, w.Body.String())
		}
	}
}

func (suite *ExtendedTestSuite) TestMultipleConstraints() {
	r := New()

	r.GET("/posts/:id/:slug", func(c *Context) {
		c.String(200, "post %s %s", c.Param("id"), c.Param("slug"))
	}).WhereNumber("id").WhereAlphaNumeric("slug")

	tests := []struct {
		path       string
		statusCode int
	}{
		{"/posts/123/mypost123", 200}, // Valid: numeric id, alphanumeric slug
		{"/posts/abc/mypost123", 404}, // Invalid: non-numeric id
		{"/posts/123/my@post", 404},   // Invalid: slug with special char
	}

	for _, test := range tests {
		req := httptest.NewRequest("GET", test.path, nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		suite.Equal(test.statusCode, w.Code, "Path %s: expected status %d, got %d", test.path, test.statusCode, w.Code)
	}
}

func (suite *ExtendedTestSuite) TestStaticFileRoute() {
	r := New()

	// Test that static file routes are registered correctly
	r.StaticFile("/favicon.ico", "./favicon.ico")

	routes := r.Routes()
	found := false
	for _, route := range routes {
		if route.Path == "/favicon.ico" && route.Method == "GET" {
			found = true
			break
		}
	}

	suite.True(found, "Expected to find static file route for /favicon.ico")
}

func (suite *ExtendedTestSuite) TestQueryDefaults() {
	r := New()

	r.GET("/search", func(c *Context) {
		limit := c.QueryDefault("limit", "10")
		page := c.QueryDefault("page", "1")
		query := c.QueryDefault("q", "")

		c.JSON(200, map[string]string{
			"limit": limit,
			"page":  page,
			"query": query,
		})
	})

	tests := []struct {
		path     string
		expected map[string]string
	}{
		{
			"/search",
			map[string]string{"limit": "10", "page": "1", "query": ""},
		},
		{
			"/search?limit=20&page=2&q=golang",
			map[string]string{"limit": "20", "page": "2", "query": "golang"},
		},
		{
			"/search?q=test",
			map[string]string{"limit": "10", "page": "1", "query": "test"},
		},
	}

	for _, test := range tests {
		req := httptest.NewRequest("GET", test.path, nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		suite.Equal(200, w.Code, "Path %s: expected status 200, got %d", test.path, w.Code)

		for key, expectedValue := range test.expected {
			suite.Contains(w.Body.String(), expectedValue, "Path %s: expected %s=%s in response", test.path, key, expectedValue)
		}
	}
}

func TestExtendedSuite(t *testing.T) {
	suite.Run(t, new(ExtendedTestSuite))
}
