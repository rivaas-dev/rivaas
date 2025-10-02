package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

// AdvancedRoutingTestSuite tests advanced routing functionality
type AdvancedRoutingTestSuite struct {
	suite.Suite
	router *Router
}

func (suite *AdvancedRoutingTestSuite) SetupTest() {
	suite.router = New()
}

func (suite *AdvancedRoutingTestSuite) TearDownTest() {
	if suite.router != nil {
		suite.router.StopMetricsServer()
	}
}

func (suite *AdvancedRoutingTestSuite) TestWildcardRoutes() {
	r := New()

	// Test default wildcard parameter
	r.GET("/files/*", func(c *Context) {
		filepath := c.Param("filepath")
		c.JSON(200, map[string]string{"filepath": filepath})
	})

	// Test custom wildcard parameter (still uses filepath parameter name)
	r.GET("/static/*", func(c *Context) {
		asset := c.Param("filepath") // Still uses "filepath" parameter name
		c.JSON(200, map[string]string{"asset": asset})
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
	r := New(
		WithVersioning(
			WithHeaderVersioning("API-Version"),
			WithQueryVersioning("version"),
			WithDefaultVersion("v1"),
			WithValidVersions("v1", "v2"),
		),
	)

	// Register version-specific routes
	v1 := r.Version("v1")
	v1.GET("/users", func(c *Context) {
		c.JSON(200, map[string]string{"version": "v1", "endpoint": "users"})
	})

	v2 := r.Version("v2")
	v2.GET("/users", func(c *Context) {
		c.JSON(200, map[string]string{"version": "v2", "endpoint": "users"})
	})

	// Test header-based versioning
	suite.Run("HeaderVersioning", func() {
		req := httptest.NewRequest("GET", "/users", nil)
		req.Header.Set("API-Version", "v2")
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
	r := New(
		WithVersioning(
			WithHeaderVersioning("API-Version"),
			WithDefaultVersion("v1"),
		),
	)

	// Create version-specific groups
	v1API := r.Version("v1").Group("/api", func(c *Context) {
		c.Header("X-API-Version", "v1")
		c.Next()
	})
	v1API.GET("/profile", func(c *Context) {
		c.JSON(200, map[string]string{"version": "v1", "endpoint": "profile"})
	})

	v2API := r.Version("v2").Group("/api", func(c *Context) {
		c.Header("X-API-Version", "v2")
		c.Next()
	})
	v2API.GET("/profile", func(c *Context) {
		c.JSON(200, map[string]string{"version": "v2", "endpoint": "profile"})
	})

	// Test v1 group
	req := httptest.NewRequest("GET", "/api/profile", nil)
	req.Header.Set("API-Version", "v1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	suite.Equal(200, w.Code, "Expected status 200, got %d", w.Code)

	// Check version header was set
	suite.Equal("v1", w.Header().Get("X-API-Version"), "Expected X-API-Version header to be v1, got %s", w.Header().Get("X-API-Version"))
}

func (suite *AdvancedRoutingTestSuite) TestCustomVersionDetection() {
	r := New(
		WithVersioning(
			WithCustomVersionDetector(func(req *http.Request) string {
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
			WithDefaultVersion("v1"),
		),
	)

	// Register version-specific routes
	v1 := r.Version("v1")
	v1.GET("/test", func(c *Context) {
		c.JSON(200, map[string]string{"version": "v1"})
	})

	v2 := r.Version("v2")
	v2.GET("/test", func(c *Context) {
		c.JSON(200, map[string]string{"version": "v2"})
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
	r := New(
		WithVersioning(
			WithHeaderVersioning("API-Version"),
			WithDefaultVersion("v1"),
		),
	)

	r.GET("/version-test", func(c *Context) {
		version := c.Version()
		isV1 := c.IsVersion("v1")
		isV2 := c.IsVersion("v2")

		c.JSON(200, map[string]interface{}{
			"version": version,
			"is_v1":   isV1,
			"is_v2":   isV2,
		})
	})

	// Test with v1 header
	req := httptest.NewRequest("GET", "/version-test", nil)
	req.Header.Set("API-Version", "v1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	suite.Equal(200, w.Code, "Expected status 200, got %d", w.Code)

	body := w.Body.String()
	suite.Contains(body, "v1", "Expected v1 version, got %s", body)
	suite.Contains(body, "true", "Expected true for is_v1, got %s", body)
}

func (suite *AdvancedRoutingTestSuite) TestPerformance() {
	r := New(
		WithVersioning(
			WithHeaderVersioning("API-Version"),
			WithDefaultVersion("v1"),
		),
	)

	// Register many routes
	for i := 0; i < 100; i++ {
		v1 := r.Version("v1")
		v1.GET("/test"+string(rune(i)), func(c *Context) {
			c.JSON(200, map[string]string{"version": "v1"})
		})
	}

	// Test that routing is still fast
	req := httptest.NewRequest("GET", "/test0", nil)
	req.Header.Set("API-Version", "v1")
	w := httptest.NewRecorder()

	// This should be fast even with many routes
	r.ServeHTTP(w, req)

	suite.Equal(200, w.Code, "Expected status 200, got %d", w.Code)
}

func (suite *AdvancedRoutingTestSuite) TestWildcardParameterNames() {
	r := New()

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
			c.JSON(200, map[string]string{route.param: param})
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
		config   []VersioningOption
		request  func() *http.Request
		expected string
	}{
		{
			name: "HeaderVersioning",
			config: []VersioningOption{
				WithHeaderVersioning("X-API-Version"),
				WithDefaultVersion("v1"),
			},
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-API-Version", "v2")
				return req
			},
			expected: "v2",
		},
		{
			name: "QueryVersioning",
			config: []VersioningOption{
				WithQueryVersioning("v"),
				WithDefaultVersion("v1"),
			},
			request: func() *http.Request {
				return httptest.NewRequest("GET", "/test?v=v2", nil)
			},
			expected: "v2",
		},
		{
			name: "DefaultVersion",
			config: []VersioningOption{
				WithDefaultVersion("v3"),
			},
			request: func() *http.Request {
				return httptest.NewRequest("GET", "/test", nil)
			},
			expected: "v3",
		},
	}

	for _, cfg := range configs {
		suite.Run(cfg.name, func() {
			r := New(WithVersioning(cfg.config...))

			// Register version-specific route
			version := r.Version(cfg.expected)
			version.GET("/test", func(c *Context) {
				c.JSON(200, map[string]string{"version": c.Version()})
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
func TestAdvancedRoutingSuite(t *testing.T) {
	suite.Run(t, new(AdvancedRoutingTestSuite))
}
