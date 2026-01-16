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

//go:build !integration

package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// ExtendedTestSuite tests extended router functionality
type ExtendedTestSuite struct {
	suite.Suite

	router *Router
}

func (suite *ExtendedTestSuite) SetupTest() {
	suite.router = MustNew()
}

func (suite *ExtendedTestSuite) TearDownTest() {
	// Cleanup if needed
	_ = suite.router
}

// TestRouteIntrospection tests route introspection functionality
func (suite *ExtendedTestSuite) TestRouteIntrospection() {
	// Add some routes
	suite.router.GET("/", func(c *Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "home")
	})
	suite.router.GET("/users/:id", func(c *Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "user")
	})
	suite.router.POST("/users", func(c *Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "create")
	})

	// Test Routes() method
	routes := suite.router.Routes()
	suite.Len(routes, 3, "Expected 3 routes")

	// Check if routes are sorted
	suite.Equal("GET", routes[0].Method, "Expected first route method to be GET")
	suite.Equal("/", routes[0].Path, "Expected first route path to be /")

	// Test that we can find our routes
	found := false
	for _, route := range routes {
		if route.Method == http.MethodPost && route.Path == "/users" {
			found = true
			break
		}
	}
	suite.True(found, "Expected to find POST /users route")
}

func (suite *ExtendedTestSuite) TestRequestHelpers() {
	r := MustNew()

	r.GET("/test", func(c *Context) {
		// Test content type detection
		if c.IsJSON() {
			//nolint:errcheck // Test handler
			c.String(http.StatusOK, "json")
		} else if c.IsXML() {
			//nolint:errcheck // Test handler
			c.String(http.StatusOK, "xml")
		} else {
			//nolint:errcheck // Test handler
			c.String(http.StatusOK, "other")
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
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Content-Type", test.contentType)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		suite.Equal(test.expected, w.Body.String(), "Expected %s for content type %s, got %s", test.expected, test.contentType, w.Body.String())
	}
}

func (suite *ExtendedTestSuite) TestAcceptsHelpers() {
	r := MustNew()

	r.GET("/test", func(c *Context) {
		if c.AcceptsJSON() {
			//nolint:errcheck // Test handler
			c.JSON(http.StatusOK, map[string]string{"type": "json"})
		} else if c.AcceptsHTML() {
			//nolint:errcheck // Test handler
			c.HTML(http.StatusOK, "<h1>html</h1>")
		} else {
			//nolint:errcheck // Test handler
			c.String(http.StatusOK, "other")
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
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Accept", test.accept)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		suite.Contains(w.Header().Get("Content-Type"), test.contentType, "Expected content type %s for accept %s, got %s", test.contentType, test.accept, w.Header().Get("Content-Type"))
	}
}

func (suite *ExtendedTestSuite) TestClientIP() {
	r := MustNew(WithTrustedProxies(
		WithProxies("10.0.0.0/8", "192.168.0.0/16"),
	))

	r.GET("/ip", func(c *Context) {
		//nolint:errcheck // Test handler
		c.Stringf(http.StatusOK, "%s", c.ClientIP())
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
			req := httptest.NewRequest(http.MethodGet, "/ip", nil)
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

func (suite *ExtendedTestSuite) TestClientIP_CustomHeaders() {
	// Test custom header support (e.g., Fastly, Akamai, etc.)
	r := MustNew(WithTrustedProxies(
		WithProxies("10.0.0.0/8", "192.168.0.0/16"),
		WithProxyHeaders(
			HeaderXFF,
			RealIPHeader("Fastly-Client-IP"), // Custom header
			RealIPHeader("True-Client-IP"),   // Another custom header
		),
	))

	r.GET("/ip", func(c *Context) {
		//nolint:errcheck // Test handler
		c.Stringf(http.StatusOK, "%s", c.ClientIP())
	})

	tests := []struct {
		name           string
		remoteAddr     string
		headerName     string
		headerValue    string
		expectedPrefix string
	}{
		{
			name:           "Fastly-Client-IP custom header",
			remoteAddr:     "10.0.0.1:8080",
			headerName:     "Fastly-Client-IP",
			headerValue:    "203.0.113.50",
			expectedPrefix: "203.0.113.50",
		},
		{
			name:           "True-Client-IP custom header",
			remoteAddr:     "10.0.0.1:8080",
			headerName:     "True-Client-IP",
			headerValue:    "198.51.100.25",
			expectedPrefix: "198.51.100.25",
		},
		{
			name:           "Custom header from untrusted proxy (ignored)",
			remoteAddr:     "203.0.113.50:8080", // Not in trusted CIDR
			headerName:     "Fastly-Client-IP",
			headerValue:    "198.51.100.25",
			expectedPrefix: "203.0.113.50", // Should return RemoteAddr, not header
		},
	}

	for _, test := range tests {
		suite.Run(test.name, func() {
			req := httptest.NewRequest(http.MethodGet, "/ip", nil)
			req.RemoteAddr = test.remoteAddr
			req.Header.Set(test.headerName, test.headerValue)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			suite.True(strings.HasPrefix(w.Body.String(), test.expectedPrefix),
				"Expected IP to start with %s, got %s", test.expectedPrefix, w.Body.String())
		})
	}
}

func (suite *ExtendedTestSuite) TestRedirect() {
	r := MustNew()

	r.GET("/redirect", func(c *Context) {
		c.Redirect(http.StatusFound, "/target")
	})

	req := httptest.NewRequest(http.MethodGet, "/redirect", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	suite.Equal(http.StatusFound, w.Code, "Expected status %d, got %d", http.StatusFound, w.Code)
	suite.Equal("/target", w.Header().Get("Location"), "Expected Location header '/target', got '%s'", w.Header().Get("Location"))
}

func (suite *ExtendedTestSuite) TestRouteConstraints() {
	r := MustNew()

	// Add route with integer constraint
	r.GET("/users/:id", func(c *Context) {
		//nolint:errcheck // Test handler
		c.Stringf(http.StatusOK, "user %s", c.Param("id"))
	}).WhereInt("id")

	// Add route with custom constraint
	r.GET("/files/:name", func(c *Context) {
		//nolint:errcheck // Test handler
		c.Stringf(http.StatusOK, "file %s", c.Param("name"))
	}).Where("name", `[a-zA-Z0-9._-]+`)

	tests := []struct {
		path       string
		statusCode int
		contains   string
	}{
		{"/users/123", http.StatusOK, "user 123"},        // Valid numeric
		{"/users/abc", http.StatusNotFound, ""},          // Invalid numeric
		{"/files/document.pdf", http.StatusOK, "file"},   // Valid filename
		{"/files/bad@file.txt", http.StatusNotFound, ""}, // Invalid filename (contains @)
	}

	for _, test := range tests {
		req := httptest.NewRequest(http.MethodGet, test.path, nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		suite.Equal(test.statusCode, w.Code, "Path %s: expected status %d, got %d", test.path, test.statusCode, w.Code)

		if test.contains != "" {
			suite.Contains(w.Body.String(), test.contains, "Path %s: expected body to contain '%s', got '%s'", test.path, test.contains, w.Body.String())
		}
	}
}

func (suite *ExtendedTestSuite) TestMultipleConstraints() {
	r := MustNew()

	r.GET("/posts/:id/:slug", func(c *Context) {
		//nolint:errcheck // Test handler
		c.Stringf(http.StatusOK, "post %s %s", c.Param("id"), c.Param("slug"))
	}).WhereInt("id").WhereRegex("slug", `[a-zA-Z0-9]+`)

	tests := []struct {
		path       string
		statusCode int
	}{
		{"/posts/123/mypost123", http.StatusOK},       // Valid: numeric id, alphanumeric slug
		{"/posts/abc/mypost123", http.StatusNotFound}, // Invalid: non-numeric id
		{"/posts/123/my@post", http.StatusNotFound},   // Invalid: slug with special char
	}

	for _, test := range tests {
		req := httptest.NewRequest(http.MethodGet, test.path, nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		suite.Equal(test.statusCode, w.Code, "Path %s: expected status %d, got %d", test.path, test.statusCode, w.Code)
	}
}

func (suite *ExtendedTestSuite) TestStaticFileRoute() {
	r := MustNew()

	// Test that static file routes are registered correctly
	r.StaticFile("/favicon.ico", "./favicon.ico")

	routes := r.Routes()
	found := false
	for _, route := range routes {
		if route.Path == "/favicon.ico" && route.Method == http.MethodGet {
			found = true
			break
		}
	}

	suite.True(found, "Expected to find static file route for /favicon.ico")
}

func (suite *ExtendedTestSuite) TestQueryDefaults() {
	r := MustNew()

	r.GET("/search", func(c *Context) {
		limit := c.QueryDefault("limit", "10")
		page := c.QueryDefault("page", "1")
		query := c.QueryDefault("q", "")

		//nolint:errcheck // Test handler
		c.JSON(http.StatusOK, map[string]string{
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
		req := httptest.NewRequest(http.MethodGet, test.path, nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		suite.Equal(http.StatusOK, w.Code, "Path %s: expected status 200, got %d", test.path, w.Code)

		for key, expectedValue := range test.expected {
			suite.Contains(w.Body.String(), expectedValue, "Path %s: expected %s=%s in response", test.path, key, expectedValue)
		}
	}
}

//nolint:paralleltest // Test suites manage their own parallelization
func TestExtendedSuite(t *testing.T) {
	suite.Run(t, new(ExtendedTestSuite))
}

// TestRoutes tests the Routes introspection function
func TestRoutes(t *testing.T) {
	t.Parallel()

	r := MustNew()

	r.GET("/users", func(_ *Context) {})
	r.POST("/users", func(_ *Context) {})
	r.GET("/users/:id", func(_ *Context) {})

	routes := r.Routes()
	assert.Len(t, routes, 3, "Expected 3 routes")

	// Verify routes are sorted correctly
	expectedMethods := []string{"GET", "GET", "POST"}
	for i, route := range routes {
		assert.Equal(t, expectedMethods[i], route.Method, "Route %d: expected method %s", i, expectedMethods[i])
	}
}
