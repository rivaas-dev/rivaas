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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConstraintValidationDebug(t *testing.T) {
	t.Parallel()

	r := MustNew()

	// Register route with integer constraint
	r.GET("/users/:id", func(c *Context) {
		t.Logf("Handler executed with id=%s", c.Param("id"))
	}).WhereInt("id")

	// Test valid request (should match)
	req1 := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	t.Logf("Valid request /users/123: status=%d (expected 200)", w1.Code)
	assert.Equal(t, 200, w1.Code, "Valid request failed")

	// Test invalid request (should NOT match - return 404)
	req2 := httptest.NewRequest(http.MethodGet, "/users/invalid123", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	t.Logf("Invalid request /users/invalid123: status=%d (expected 404)", w2.Code)
	assert.Equal(t, 404, w2.Code, "Invalid request should return 404")

	// Check route compiler (just verify it exists, can't access unexported fields)
	if r.routeCompiler != nil {
		t.Logf("Route compiler is initialized")
		// Note: Can't access internal fields for debugging in tests
		// The route compiler is working if the route matches correctly
	}
}

func TestFastPathConstraintValidation(t *testing.T) {
	t.Parallel()

	// Test the fast path for constraint validation through the router
	r := MustNew()

	// Register route with integer constraint
	r.GET("/users/:id", func(c *Context) {
		//nolint:errcheck // Test handler
		c.Stringf(http.StatusOK, "id=%s", c.Param("id"))
	}).WhereInt("id")

	// Test valid value
	req1 := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	t.Logf("Valid /users/123: status=%d", w1.Code)
	assert.Equal(t, http.StatusOK, w1.Code, "Valid request should match")

	// Test invalid value
	req2 := httptest.NewRequest(http.MethodGet, "/users/abc123", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	t.Logf("Invalid /users/abc123: status=%d", w2.Code)
	assert.Equal(t, http.StatusNotFound, w2.Code, "Invalid request should NOT match (should return 404)")
}

// TestRouteConstraints tests additional constraint validators
func TestRouteConstraints(t *testing.T) {
	t.Parallel()

	r := MustNew()

	r.GET("/alpha/:name", func(c *Context) {
		//nolint:errcheck // Test handler
		c.Stringf(http.StatusOK, "name=%s", c.Param("name"))
	}).WhereRegex("name", `[a-zA-Z]+`)

	r.GET("/uuid/:id", func(c *Context) {
		//nolint:errcheck // Test handler
		c.Stringf(http.StatusOK, "id=%s", c.Param("id"))
	}).WhereUUID("id")

	tests := []struct {
		name       string
		path       string
		shouldPass bool
		expected   string
	}{
		{"valid alpha", "/alpha/john", true, "name=john"},
		{"invalid alpha with numbers", "/alpha/john123", false, ""},
		{"valid UUID", "/uuid/123e4567-e89b-12d3-a456-426614174000", true, "id=123e4567-e89b-12d3-a456-426614174000"},
		{"invalid UUID", "/uuid/not-a-uuid", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if tt.shouldPass {
				assert.Equal(t, http.StatusOK, w.Code, "Expected status %d", http.StatusOK)
				assert.Equal(t, tt.expected, w.Body.String(), "Expected %q", tt.expected)
			} else {
				assert.Equal(t, http.StatusNotFound, w.Code, "Expected status %d", http.StatusNotFound)
			}
		})
	}
}
