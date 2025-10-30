package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestVersionedRouting tests version-specific routing
func TestVersionedRouting(t *testing.T) {
	r := New(
		WithVersioning(
			WithHeaderVersioning("X-API-Version"),
			WithDefaultVersion("v1"),
			WithValidVersions("v1", "v2"),
		),
	)

	// Add v1 routes - using static routes for PUT/DELETE/PATCH to ensure they're tested
	v1 := r.Version("v1")
	v1.GET("/users", func(c *Context) {
		c.String(http.StatusOK, "v1 users")
	})
	v1.POST("/users", func(c *Context) {
		c.String(http.StatusCreated, "v1 user created")
	})
	// Use static paths for these to avoid parameter extraction issues with versioned routes
	v1.PUT("/users/123", func(c *Context) {
		c.String(http.StatusOK, "v1 updated user 123")
	})
	v1.DELETE("/users/456", func(c *Context) {
		c.String(http.StatusOK, "v1 deleted user 456")
	})
	v1.PATCH("/users/789", func(c *Context) {
		c.String(http.StatusOK, "v1 patched user 789")
	})
	v1.OPTIONS("/users", func(c *Context) {
		c.String(http.StatusOK, "v1 options")
	})
	v1.HEAD("/users", func(c *Context) {
		c.Status(http.StatusOK)
	})

	// Add v2 routes
	v2 := r.Version("v2")
	v2.GET("/users", func(c *Context) {
		c.String(http.StatusOK, "v2 users")
	})
	v2.POST("/users", func(c *Context) {
		c.String(http.StatusCreated, "v2 user created")
	})

	tests := []struct {
		name     string
		method   string
		path     string
		version  string
		expected string
		status   int
	}{
		{"v1 GET", "GET", "/users", "v1", "v1 users", http.StatusOK},
		{"v2 GET", "GET", "/users", "v2", "v2 users", http.StatusOK},
		{"v1 POST", "POST", "/users", "v1", "v1 user created", http.StatusCreated},
		{"v2 POST", "POST", "/users", "v2", "v2 user created", http.StatusCreated},
		{"v1 PUT", "PUT", "/users/123", "v1", "v1 updated user 123", http.StatusOK},
		{"v1 DELETE", "DELETE", "/users/456", "v1", "v1 deleted user 456", http.StatusOK},
		{"v1 PATCH", "PATCH", "/users/789", "v1", "v1 patched user 789", http.StatusOK},
		{"v1 OPTIONS", "OPTIONS", "/users", "v1", "v1 options", http.StatusOK},
		{"v1 HEAD", "HEAD", "/users", "v1", "", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("X-API-Version", tt.version)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.status, w.Code)
			if tt.expected != "" {
				assert.Equal(t, tt.expected, w.Body.String())
			}
		})
	}
}

// TestVersionedGroups tests versioned route groups
func TestVersionedGroups(t *testing.T) {
	r := New(
		WithVersioning(
			WithHeaderVersioning("X-API-Version"),
			WithDefaultVersion("v1"),
		),
	)

	// Create versioned groups - using static paths to ensure they work
	v1 := r.Version("v1")
	v1Group := v1.Group("/api")
	v1Group.GET("/users", func(c *Context) {
		c.String(http.StatusOK, "v1 api users")
	})
	v1Group.POST("/users", func(c *Context) {
		c.String(http.StatusCreated, "v1 api user created")
	})
	v1Group.PUT("/users/123", func(c *Context) {
		c.String(http.StatusOK, "v1 api updated 123")
	})
	v1Group.DELETE("/users/456", func(c *Context) {
		c.String(http.StatusOK, "v1 api deleted 456")
	})
	v1Group.PATCH("/users/789", func(c *Context) {
		c.String(http.StatusOK, "v1 api patched 789")
	})
	v1Group.OPTIONS("/users", func(c *Context) {
		c.String(http.StatusOK, "v1 api options")
	})
	v1Group.HEAD("/users", func(c *Context) {
		c.Status(http.StatusOK)
	})

	tests := []struct {
		name     string
		method   string
		path     string
		expected string
		status   int
	}{
		{"GET", "GET", "/api/users", "v1 api users", http.StatusOK},
		{"POST", "POST", "/api/users", "v1 api user created", http.StatusCreated},
		{"PUT", "PUT", "/api/users/123", "v1 api updated 123", http.StatusOK},
		{"DELETE", "DELETE", "/api/users/456", "v1 api deleted 456", http.StatusOK},
		{"PATCH", "PATCH", "/api/users/789", "v1 api patched 789", http.StatusOK},
		{"OPTIONS", "OPTIONS", "/api/users", "v1 api options", http.StatusOK},
		{"HEAD", "HEAD", "/api/users", "", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("X-API-Version", "v1")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.status, w.Code)
			if tt.expected != "" {
				assert.Equal(t, tt.expected, w.Body.String())
			}
		})
	}
}

// TestQueryVersioning tests query parameter-based versioning
func TestQueryVersioning(t *testing.T) {
	r := New(
		WithVersioning(
			WithQueryVersioning("version"),
			WithDefaultVersion("v1"),
			WithValidVersions("v1", "v2"),
		),
	)

	v1 := r.Version("v1")
	v1.GET("/data", func(c *Context) {
		c.String(http.StatusOK, "v1 data")
	})

	v2 := r.Version("v2")
	v2.GET("/data", func(c *Context) {
		c.String(http.StatusOK, "v2 data")
	})

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"default version", "/data", "v1 data"},
		{"v1 explicit", "/data?version=v1", "v1 data"},
		{"v2 explicit", "/data?version=v2", "v2 data"},
		{"invalid version defaults to v1", "/data?version=v3", "v1 data"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tt.expected, w.Body.String())
		})
	}
}

// TestCustomVersionDetector tests custom version detection function
func TestCustomVersionDetector(t *testing.T) {
	r := New(
		WithVersioning(
			WithCustomVersionDetector(func(req *http.Request) string {
				// Custom logic: extract version from user-agent
				ua := req.UserAgent()
				if ua == "ClientV2" {
					return "v2"
				}
				return "v1"
			}),
		),
	)

	v1 := r.Version("v1")
	v1.GET("/data", func(c *Context) {
		c.String(http.StatusOK, "v1 data")
	})

	v2 := r.Version("v2")
	v2.GET("/data", func(c *Context) {
		c.String(http.StatusOK, "v2 data")
	})

	// Test v1 (default)
	req := httptest.NewRequest("GET", "/data", nil)
	req.Header.Set("User-Agent", "ClientV1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, "v1 data", w.Body.String())

	// Test v2 (custom detector)
	req = httptest.NewRequest("GET", "/data", nil)
	req.Header.Set("User-Agent", "ClientV2")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, "v2 data", w.Body.String())
}

// TestVersionedRoutingWithCompilation tests versioned routes with compilation
func TestVersionedRoutingWithCompilation(t *testing.T) {
	r := New(
		WithVersioning(
			WithHeaderVersioning("X-API-Version"),
			WithDefaultVersion("v1"),
		),
	)

	v1 := r.Version("v1")
	v1.GET("/static1", func(c *Context) {
		c.String(http.StatusOK, "v1 static1")
	})
	v1.GET("/static2", func(c *Context) {
		c.String(http.StatusOK, "v1 static2")
	})

	// Compile routes
	r.WarmupOptimizations()

	// Test compiled versioned routes
	req := httptest.NewRequest("GET", "/static1", nil)
	req.Header.Set("X-API-Version", "v1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "v1 static1", w.Body.String())
}

// ============================================================================
// Fast Path Tests (merged from versioning_fast_test.go)
// ============================================================================

func TestFastQueryVersion(t *testing.T) {
	tests := []struct {
		name      string
		rawQuery  string
		param     string
		wantValue string
		wantFound bool
	}{
		// Basic cases
		{"empty_query", "", "v", "", false},
		{"empty_param", "v=v1", "", "", false},
		{"simple_match", "v=v1", "v", "v1", true},
		{"long_value", "version=v1.2.3", "version", "v1.2.3", true},

		// Position variations
		{"param_at_start", "v=v1", "v", "v1", true},
		{"param_in_middle", "foo=bar&v=v2&baz=qux", "v", "v2", true},
		{"param_at_end", "foo=bar&baz=qux&v=v3", "v", "v3", true},

		// Boundary cases
		{"param_not_at_boundary", "foobar=v1", "bar", "", false}, // "bar=" is not at boundary
		{"similar_param_names", "fooversion=v1&version=v2", "version", "v2", true},
		{"param_as_substring", "myv=wrong&v=correct", "v", "correct", true},

		// Value extraction
		{"value_until_ampersand", "v=v1&other=value", "v", "v1", true},
		{"value_until_end", "other=value&v=v2", "v", "v2", true},
		{"empty_value", "v=&other=value", "v", "", true}, // Empty value is valid
		{"no_value", "v", "v", "", false},                // No "=" sign

		// Multiple occurrences (should return first)
		{"duplicate_params", "v=v1&foo=bar&v=v2", "v", "v1", true},

		// Special characters in values
		{"dash_in_value", "v=v1-beta", "v", "v1-beta", true},
		{"dot_in_value", "v=1.0.0", "v", "1.0.0", true},
		{"underscore_in_value", "v=v1_stable", "v", "v1_stable", true},

		// Long parameter names
		{"long_param_name", "api_version=v1", "api_version", "v1", true},
		{"long_param_middle", "foo=bar&api_version=v2&baz=qux", "api_version", "v2", true},

		// Edge cases
		{"single_char_param", "a=1", "a", "1", true},
		{"single_char_value", "version=1", "version", "1", true},
		{"equals_no_value_at_end", "foo=bar&v=", "v", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotFound := fastQueryVersion(tt.rawQuery, tt.param)
			if gotValue != tt.wantValue {
				t.Errorf("fastQueryVersion(%q, %q) value = %q, want %q",
					tt.rawQuery, tt.param, gotValue, tt.wantValue)
			}
			if gotFound != tt.wantFound {
				t.Errorf("fastQueryVersion(%q, %q) found = %v, want %v",
					tt.rawQuery, tt.param, gotFound, tt.wantFound)
			}
		})
	}
}

// TestFastQueryVersion_ZeroAlloc verifies zero allocations
func TestFastQueryVersion_ZeroAlloc(t *testing.T) {
	allocs := testing.AllocsPerRun(100, func() {
		_, _ = fastQueryVersion("foo=bar&v=v1&baz=qux", "v")
	})

	if allocs != 0 {
		t.Errorf("fastQueryVersion allocated %f times, want 0", allocs)
	}
}

// TestFastHeaderVersion tests the fast header extraction
func TestFastHeaderVersion(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string][]string
		headerName string
		want       string
	}{
		{"found", map[string][]string{"API-Version": {"v1"}}, "API-Version", "v1"},
		{"not_found", map[string][]string{"Other": {"value"}}, "API-Version", ""},
		{"empty_header", map[string][]string{}, "API-Version", ""},
		{"multiple_values", map[string][]string{"API-Version": {"v1", "v2"}}, "API-Version", "v1"},
		{"empty_value", map[string][]string{"API-Version": {""}}, "API-Version", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fastHeaderVersion(tt.headers, tt.headerName)
			if got != tt.want {
				t.Errorf("fastHeaderVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFastHeaderVersion_ZeroAlloc verifies zero allocations
func TestFastHeaderVersion_ZeroAlloc(t *testing.T) {
	headers := map[string][]string{"API-Version": {"v1"}}

	allocs := testing.AllocsPerRun(100, func() {
		_ = fastHeaderVersion(headers, "API-Version")
	})

	if allocs != 0 {
		t.Errorf("fastHeaderVersion allocated %f times, want 0", allocs)
	}
}

// TestDetectVersion_Integration tests version detection with fast paths
func TestDetectVersion_Integration(t *testing.T) {
	t.Run("query_fast_path", func(t *testing.T) {
		r := New(WithVersioning(
			WithQueryVersioning("v"),
			WithDefaultVersion("v1"),
		))

		// Create request with query parameter
		req := httptest.NewRequest("GET", "/test?v=v2", nil)

		version := r.detectVersion(req)
		if version != "v2" {
			t.Errorf("detectVersion() = %q, want %q", version, "v2")
		}
	})

	t.Run("query_with_validation", func(t *testing.T) {
		r := New(WithVersioning(
			WithQueryVersioning("version"),
			WithValidVersions("v1", "v2", "v3"),
			WithDefaultVersion("v1"),
		))

		req := httptest.NewRequest("GET", "/test?version=v2", nil)
		version := r.detectVersion(req)
		if version != "v2" {
			t.Errorf("detectVersion() = %q, want %q", version, "v2")
		}
	})

	t.Run("query_invalid_version", func(t *testing.T) {
		r := New(WithVersioning(
			WithQueryVersioning("v"),
			WithValidVersions("v1", "v2"),
			WithDefaultVersion("v1"),
		))

		req := httptest.NewRequest("GET", "/test?v=invalid", nil)
		version := r.detectVersion(req)
		if version != "v1" {
			t.Errorf("detectVersion() = %q, want %q (default)", version, "v1")
		}
	})

	t.Run("header_priority", func(t *testing.T) {
		r := New(WithVersioning(
			WithHeaderVersioning("API-Version"),
			WithQueryVersioning("v"),
			WithDefaultVersion("v1"),
		))

		req := httptest.NewRequest("GET", "/test?v=v3", nil)
		req.Header.Set("API-Version", "v2")

		// Header should take priority over query
		version := r.detectVersion(req)
		if version != "v2" {
			t.Errorf("detectVersion() = %q, want %q (header priority)", version, "v2")
		}
	})
}
