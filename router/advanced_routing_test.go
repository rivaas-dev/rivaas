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
	"strings"
	"testing"

	"rivaas.dev/router/version"

	"github.com/stretchr/testify/suite"
)

// AdvancedRoutingTestSuite tests advanced routing functionality
type AdvancedRoutingTestSuite struct {
	suite.Suite
	router *Router
}

func (suite *AdvancedRoutingTestSuite) SetupTest() {
	suite.router = MustNew()
}

func (suite *AdvancedRoutingTestSuite) TearDownTest() {
	// Cleanup if needed
}

func (suite *AdvancedRoutingTestSuite) TestWildcardRoutes() {
	r := MustNew()

	// Test default wildcard parameter
	r.GET("/files/*", func(c *Context) {
		filepath := c.Param("filepath")
		c.JSON(http.StatusOK, map[string]string{"filepath": filepath})
	})

	// Test custom wildcard parameter (still uses filepath parameter name)
	r.GET("/static/*", func(c *Context) {
		asset := c.Param("filepath") // Still uses "filepath" parameter name
		c.JSON(http.StatusOK, map[string]string{"asset": asset})
	})

	// Test requests
	tests := []struct {
		path     string
		expected string
		param    string
	}{
		{"/files/image.jpg", "image.jpg", "filepath"},
		{"/files/docs/readme.txt", "docs/readme.txt", "filepath"},
		{"/static/css/style.css", "css/style.css", "filepath"},
		{"/static/js/app.js", "js/app.js", "filepath"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		suite.Equal(200, w.Code, "Expected status 200, got %d", w.Code)

		// Check response contains the expected parameter value
		body := w.Body.String()
		suite.Contains(body, tt.expected, "Expected response to contain %s, got %s", tt.expected, body)
	}
}

func (suite *AdvancedRoutingTestSuite) TestRouteVersioning() {
	r := MustNew(
		WithVersioning(
			version.WithHeaderDetection("API-Version"),
			version.WithQueryDetection("version"),
			version.WithDefault("v1"),
			version.WithValidVersions("v1", "v2"),
		),
	)

	// Register version-specific routes
	v1 := r.Version("v1")
	v1.GET("/users", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"version": "v1", "endpoint": "users"})
	})

	v2 := r.Version("v2")
	v2.GET("/users", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"version": "v2", "endpoint": "users"})
	})

	// Test header-based versioning
	suite.Run("HeaderVersioning", func() {
		req := httptest.NewRequest("GET", "/users", nil)
		req.Header.Set("Api-Version", "v2")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		suite.Equal(200, w.Code, "Expected status 200, got %d", w.Code)

		body := w.Body.String()
		suite.Contains(body, "v2", "Expected v2 version, got %s", body)
	})

	// Test query parameter versioning
	suite.Run("QueryVersioning", func() {
		req := httptest.NewRequest("GET", "/users?version=v1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		suite.Equal(200, w.Code, "Expected status 200, got %d", w.Code)

		body := w.Body.String()
		suite.Contains(body, "v1", "Expected v1 version, got %s", body)
	})

	// Test default version
	suite.Run("DefaultVersion", func() {
		req := httptest.NewRequest("GET", "/users", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		suite.Equal(200, w.Code, "Expected status 200, got %d", w.Code)

		body := w.Body.String()
		suite.Contains(body, "v1", "Expected default v1 version, got %s", body)
	})
}

func (suite *AdvancedRoutingTestSuite) TestVersionGroups() {
	r := MustNew(
		WithVersioning(
			version.WithHeaderDetection("API-Version"),
			version.WithDefault("v1"),
		),
	)

	// Create version-specific groups
	v1API := r.Version("v1").Group("/api", func(c *Context) {
		c.Header("X-Api-Version", "v1")
		c.Next()
	})
	v1API.GET("/profile", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"version": "v1", "endpoint": "profile"})
	})

	v2API := r.Version("v2").Group("/api", func(c *Context) {
		c.Header("X-Api-Version", "v2")
		c.Next()
	})
	v2API.GET("/profile", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"version": "v2", "endpoint": "profile"})
	})

	// Test v1 group
	req := httptest.NewRequest("GET", "/api/profile", nil)
	req.Header.Set("Api-Version", "v1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	suite.Equal(200, w.Code, "Expected status 200, got %d", w.Code)

	// Check version header was set
	suite.Equal("v1", w.Header().Get("X-Api-Version"), "Expected X-Api-Version header to be v1, got %s", w.Header().Get("X-Api-Version"))
}

func (suite *AdvancedRoutingTestSuite) TestCustomVersionDetection() {
	r := MustNew(
		WithVersioning(
			version.WithCustomDetection(func(req *http.Request) string {
				// Custom logic: check subdomain
				host := req.Host
				if strings.HasPrefix(host, "v2.") {
					return "v2"
				}
				if strings.HasPrefix(host, "v1.") {
					return "v1"
				}
				return "v1" // default
			}),
			version.WithDefault("v1"),
		),
	)

	// Register version-specific routes
	v1 := r.Version("v1")
	v1.GET("/test", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"version": "v1"})
	})

	v2 := r.Version("v2")
	v2.GET("/test", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"version": "v2"})
	})

	// Test subdomain-based versioning
	req := httptest.NewRequest("GET", "/test", nil)
	req.Host = "v2.example.com"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	suite.Equal(200, w.Code, "Expected status 200, got %d", w.Code)

	body := w.Body.String()
	suite.Contains(body, "v2", "Expected v2 version, got %s", body)
}

func (suite *AdvancedRoutingTestSuite) TestContextVersionMethods() {
	r := MustNew(
		WithVersioning(
			version.WithHeaderDetection("API-Version"),
			version.WithDefault("v1"),
		),
	)

	// Routes must be registered via Version() to get version detection
	// Non-versioned routes (r.GET) bypass version detection entirely
	v1 := r.Version("v1")
	v1.GET("/version-test", func(c *Context) {
		version := c.Version()
		isV1 := c.IsVersion("v1")
		isV2 := c.IsVersion("v2")

		c.JSON(http.StatusOK, map[string]any{
			"version": version,
			"is_v1":   isV1,
			"is_v2":   isV2,
		})
	})

	// Test with v1 header
	req := httptest.NewRequest("GET", "/version-test", nil)
	req.Header.Set("Api-Version", "v1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	suite.Equal(200, w.Code, "Expected status 200, got %d", w.Code)

	body := w.Body.String()
	suite.Contains(body, "v1", "Expected v1 version, got %s", body)
	suite.Contains(body, "true", "Expected true for is_v1, got %s", body)
}

func (suite *AdvancedRoutingTestSuite) TestPerformance() {
	r := MustNew(
		WithVersioning(
			version.WithHeaderDetection("API-Version"),
			version.WithDefault("v1"),
		),
	)

	// Register many routes
	for i := range 100 {
		v1 := r.Version("v1")
		v1.GET("/test"+string(rune(i)), func(c *Context) {
			c.JSON(http.StatusOK, map[string]string{"version": "v1"})
		})
	}

	// Test that routing is still fast
	req := httptest.NewRequest("GET", "/test0", nil)
	req.Header.Set("Api-Version", "v1")
	w := httptest.NewRecorder()

	// This should be fast even with many routes
	r.ServeHTTP(w, req)

	suite.Equal(200, w.Code, "Expected status 200, got %d", w.Code)
}

func (suite *AdvancedRoutingTestSuite) TestWildcardParameterNames() {
	r := MustNew()

	// Test different wildcard routes (all use "filepath" parameter name)
	routes := []struct {
		path     string
		param    string
		expected string
	}{
		{"/files/*", "filepath", "image.jpg"},
		{"/static/*", "filepath", "css/style.css"},
		{"/uploads/*", "filepath", "document.pdf"},
		{"/docs/*", "filepath", "api/guide.md"},
	}

	for _, route := range routes {
		r.GET(route.path, func(c *Context) {
			param := c.Param(route.param)
			c.JSON(http.StatusOK, map[string]string{route.param: param})
		})
	}

	// Test each route
	testPaths := []struct {
		path     string
		param    string
		expected string
	}{
		{"/files/image.jpg", "filepath", "image.jpg"},
		{"/static/css/style.css", "filepath", "css/style.css"},
		{"/uploads/document.pdf", "filepath", "document.pdf"},
		{"/docs/api/guide.md", "filepath", "api/guide.md"},
	}

	for _, tt := range testPaths {
		req := httptest.NewRequest("GET", tt.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		suite.Equal(200, w.Code, "Expected status 200 for %s, got %d", tt.path, w.Code)

		body := w.Body.String()
		suite.Contains(body, tt.expected, "Expected response to contain %s for %s, got %s", tt.expected, tt.path, body)
	}
}

func (suite *AdvancedRoutingTestSuite) TestVersioningConfiguration() {
	// Test different versioning configurations
	configs := []struct {
		name     string
		config   []version.Option
		request  func() *http.Request
		expected string
	}{
		{
			name: "HeaderVersioning",
			config: []version.Option{
				version.WithHeaderDetection("X-API-Version"),
				version.WithDefault("v1"),
			},
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Api-Version", "v2")
				return req
			},
			expected: "v2",
		},
		{
			name: "QueryVersioning",
			config: []version.Option{
				version.WithQueryDetection("v"),
				version.WithDefault("v1"),
			},
			request: func() *http.Request {
				return httptest.NewRequest("GET", "/test?v=v2", nil)
			},
			expected: "v2",
		},
		{
			name: "DefaultVersion",
			config: []version.Option{
				version.WithDefault("v3"),
			},
			request: func() *http.Request {
				return httptest.NewRequest("GET", "/test", nil)
			},
			expected: "v3",
		},
	}

	for _, cfg := range configs {
		suite.Run(cfg.name, func() {
			r := MustNew(WithVersioning(cfg.config...))

			// Register version-specific route
			version := r.Version(cfg.expected)
			version.GET("/test", func(c *Context) {
				c.JSON(http.StatusOK, map[string]string{"version": c.Version()})
			})

			req := cfg.request()
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			suite.Equal(200, w.Code, "Expected status 200, got %d", w.Code)

			body := w.Body.String()
			suite.Contains(body, cfg.expected, "Expected version %s, got %s", cfg.expected, body)
		})
	}
}

// TestAdvancedRoutingSuite runs the advanced routing test suite
//
//nolint:paralleltest // Test suites manage their own parallelization
func TestAdvancedRoutingSuite(t *testing.T) {
	suite.Run(t, new(AdvancedRoutingTestSuite))
}
