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

package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProperty_RouteMatchingCommutativity tests that route matching is commutative:
// registering routes in different orders should produce the same results.
func TestProperty_RouteMatchingCommutativity(t *testing.T) {
	t.Parallel()


	// Generate test routes
	routes := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/users/:id", "user"},
		{"GET", "/posts/:id", "post"},
		{"GET", "/api/v1/health", "health"},
		{"POST", "/users", "create"},
		{"PUT", "/users/:id", "update"},
	}

	// Test all permutations of route registration order
	permutations := generatePermutations(len(routes))

	for _, perm := range permutations {
		t.Run(fmt.Sprintf("permutation_%v", perm), func(t *testing.T) {
			t.Parallel()

			app := MustNew(
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
			)

			// Register routes in this permutation order
			for _, idx := range perm {
				route := routes[idx]
				body := route.body // Capture for closure
				switch route.method {
				case "GET":
					app.GET(route.path, func(c *Context) {
						if err := c.Stringf(http.StatusOK, "%s", body); err != nil {
							c.Logger().Error("failed to write response", "err", err)
						}
					})
				case "POST":
					app.POST(route.path, func(c *Context) {
						if err := c.Stringf(http.StatusOK, "%s", body); err != nil {
							c.Logger().Error("failed to write response", "err", err)
						}
					})
				case "PUT":
					app.PUT(route.path, func(c *Context) {
						if err := c.Stringf(http.StatusOK, "%s", body); err != nil {
							c.Logger().Error("failed to write response", "err", err)
						}
					})
				default:
					app.GET(route.path, func(c *Context) {
						if err := c.Stringf(http.StatusOK, "%s", body); err != nil {
							c.Logger().Error("failed to write response", "err", err)
						}
					})
				}
			}

			// Test that all routes still work regardless of registration order
			for _, route := range routes {
				req := httptest.NewRequest(route.method, route.path, nil)
				w := httptest.NewRecorder()
				app.Router().ServeHTTP(w, req)

				// Replace :id with a test value for matching
				testPath := strings.Replace(route.path, ":id", "123", 1)
				req = httptest.NewRequest(route.method, testPath, nil)
				w = httptest.NewRecorder()
				app.Router().ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code,
					"route %s %s should work in any registration order",
					route.method, route.path)
			}
		})
	}
}

// TestProperty_MiddlewareIdempotency tests that adding the same middleware
// multiple times produces consistent results (idempotency property).
func TestProperty_MiddlewareIdempotency(t *testing.T) {
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	var callCount int

	middleware := func(c *Context) {
		callCount++
		c.Next()
	}

	// Add same middleware multiple times
	for range 5 {
		app.Use(middleware)
	}

	app.GET("/test", func(c *Context) {
		if err := c.Stringf(http.StatusOK, "ok"); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	app.Router().ServeHTTP(w, req)

	// Middleware should be called exactly 5 times
	assert.Equal(t, 5, callCount, "middleware should be called for each registration")
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestProperty_ConfigurationDefaults tests that default configuration values
// satisfy all validation constraints (defaults should always be valid).
func TestProperty_ConfigurationDefaults(t *testing.T) {
	t.Parallel()

	// Test that default config is always valid
	cfg := defaultConfig()

	err := cfg.validate()
	require.NoError(t, err, "default configuration should always be valid")

	// Verify defaults satisfy constraints
	assert.Greater(t, cfg.server.readTimeout, time.Duration(0))
	assert.Greater(t, cfg.server.writeTimeout, time.Duration(0))
	assert.GreaterOrEqual(t, cfg.server.readTimeout, cfg.server.writeTimeout,
		"default read timeout should not exceed write timeout")
	assert.GreaterOrEqual(t, cfg.server.shutdownTimeout, time.Second,
		"default shutdown timeout should be at least 1 second")
	assert.GreaterOrEqual(t, cfg.server.maxHeaderBytes, 1024,
		"default max header bytes should be at least 1KB")
}

// TestProperty_ErrorMessagesCompleteness tests that all validation errors
// provide complete information (field, value, message, constraint).
func TestProperty_ErrorMessagesCompleteness(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		opts  []Option
		check func(*testing.T, error)
	}{
		{
			name: "empty service name",
			opts: []Option{
				WithServiceName(""),
				WithServiceVersion("1.0.0"),
			},
			check: func(t *testing.T, err error) {
				t.Helper()
				var ve *ValidationError
				require.ErrorAs(t, err, &ve)
				for _, e := range ve.Errors {
					assert.NotEmpty(t, e.Field, "error should have field name")
					assert.NotEmpty(t, e.Message, "error should have message")
				}
			},
		},
		{
			name: "invalid timeout",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServer(WithReadTimeout(-1 * time.Second)),
			},
			check: func(t *testing.T, err error) {
				t.Helper()
				var ve *ValidationError
				require.ErrorAs(t, err, &ve)
				for _, e := range ve.Errors {
					if e.Field == "server.readTimeout" {
						assert.NotNil(t, e.Value, "error should include invalid value")
						assert.NotEmpty(t, e.Constraint, "error should include constraint")
					}
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := New(tc.opts...)
			require.Error(t, err)
			if tc.check != nil {
				tc.check(t, err)
			}
		})
	}
}

// TestProperty_RoutePathEquivalence tests that equivalent route paths
// (e.g., "/users/:id" and "/users/123") match correctly.
func TestProperty_RoutePathEquivalence(t *testing.T) {
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	// Register parameter route
	app.GET("/users/:id", func(c *Context) {
		id := c.Param("id")
		if err := c.Stringf(http.StatusOK, "user-%s", id); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	// Test various equivalent paths
	testPaths := []string{
		"/users/123",
		"/users/abc",
		"/users/123-456",
		"/users/user_123",
	}

	for _, path := range testPaths {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			app.Router().ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code,
				"path %s should match /users/:id", path)
			assert.Contains(t, w.Body.String(), strings.TrimPrefix(path, "/users/"),
				"response should contain parameter value")
		})
	}
}

// Helper function to generate all permutations of indices
func generatePermutations(n int) [][]int {
	if n == 0 {
		return [][]int{{}}
	}
	if n == 1 {
		return [][]int{{0}}
	}

	// Generate permutations recursively
	smaller := generatePermutations(n - 1)
	result := make([][]int, 0, len(smaller)*n)

	for _, perm := range smaller {
		for i := 0; i <= len(perm); i++ {
			newPerm := make([]int, 0, len(perm)+1)
			newPerm = append(newPerm, perm[:i]...)
			newPerm = append(newPerm, n-1)
			newPerm = append(newPerm, perm[i:]...)
			result = append(result, newPerm)
		}
	}

	return result
}

// TestProperty_ConfigurationComposition tests that configuration options
// can be composed in any order (commutativity of options).
func TestProperty_ConfigurationComposition(t *testing.T) {
	t.Parallel()

	opts1 := []Option{
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithEnvironment(EnvironmentDevelopment),
	}

	opts2 := []Option{
		WithEnvironment(EnvironmentDevelopment),
		WithServiceVersion("1.0.0"),
		WithServiceName("test"),
	}

	app1, err1 := New(opts1...)
	app2, err2 := New(opts2...)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NotNil(t, app1)
	assert.NotNil(t, app2)

	// Both should have same configuration
	assert.Equal(t, app1.ServiceName(), app2.ServiceName())
	assert.Equal(t, app1.ServiceVersion(), app2.ServiceVersion())
	assert.Equal(t, app1.Environment(), app2.Environment())
}
