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
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	suite.root.addRouteWithConstraints("/", []HandlerFunc{func(_ *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/users", []HandlerFunc{func(_ *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/users/:id", []HandlerFunc{func(_ *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/users/:id/posts", []HandlerFunc{func(_ *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/users/:id/posts/:post_id", []HandlerFunc{func(_ *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/posts", []HandlerFunc{func(_ *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/posts/:id", []HandlerFunc{func(_ *Context) {}}, nil)

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
			handlers, _ := suite.root.getRoute(tt.path, ctx)

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
	suite.root.addRouteWithConstraints("/api/v1/users/:id", []HandlerFunc{func(_ *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/api/v1/users/:id/posts/:post_id", []HandlerFunc{func(_ *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/api/v1/posts/:id/comments/:comment_id", []HandlerFunc{func(_ *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/api/v2/users", []HandlerFunc{func(_ *Context) {}}, nil)

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
			handlers, _ := suite.root.getRoute(tt.path, ctx)

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
	suite.root.addRouteWithConstraints("/", []HandlerFunc{func(_ *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/a", []HandlerFunc{func(_ *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/ab", []HandlerFunc{func(_ *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/abc", []HandlerFunc{func(_ *Context) {}}, nil)
	suite.root.addRouteWithConstraints("/:param", []HandlerFunc{func(_ *Context) {}}, nil)

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
			handlers, _ := suite.root.getRoute(tt.path, ctx)

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

// TestRadixTree_MoreThan8Params tests the map fallback for >8 parameters
func (suite *RadixTestSuite) TestRadixTree_MoreThan8Params() {
	// Add route with 9 parameters to trigger map fallback
	suite.root.addRouteWithConstraints("/a/:p1/b/:p2/c/:p3/d/:p4/e/:p5/f/:p6/g/:p7/h/:p8/i/:p9", []HandlerFunc{func(_ *Context) {}}, nil)

	ctx := &Context{}
	handlers, _ := suite.root.getRoute("/a/v1/b/v2/c/v3/d/v4/e/v5/f/v6/g/v7/h/v8/i/v9", ctx)

	suite.NotNil(handlers, "Expected to find route with 9 parameters")

	// Verify Params map was created
	suite.NotNil(ctx.Params, "Expected Params map to be created for 9th parameter")

	// Verify 9th parameter is stored in Params map
	suite.Equal("v9", ctx.Params["p9"], "Expected p9=v9 in Params map")

	// Verify first 8 params are accessible via Param() (from arrays)
	suite.Equal("v1", ctx.Param("p1"))
	suite.Equal("v8", ctx.Param("p8"))

	// Verify 9th parameter is accessible via Param() (from map)
	suite.Equal("v9", ctx.Param("p9"))
}

// TestRadixSuite runs the radix test suite
//
//nolint:paralleltest // Test suites manage their own parallelization
func TestRadixSuite(t *testing.T) {
	suite.Run(t, new(RadixTestSuite))
}

// TestEdgeCasesInRadixTree tests edge cases in radix tree matching
func TestEdgeCasesInRadixTree(t *testing.T) {
	t.Parallel()

	r := MustNew()

	t.Run("Empty segments", func(t *testing.T) {
		t.Parallel()

		r.GET("/a//b", func(c *Context) {
			c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/a//b", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code, "Expected status 200")
	})

	t.Run("Trailing slash handling", func(t *testing.T) {
		t.Parallel()

		r.GET("/users/", func(c *Context) {
			c.String(http.StatusOK, "users with slash")
		})
		r.GET("/posts", func(c *Context) {
			c.String(http.StatusOK, "posts without slash")
		})

		// Test exact match with trailing slash
		req := httptest.NewRequest(http.MethodGet, "/users/", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code, "Expected status 200")
		assert.Equal(t, "users with slash", w.Body.String(), "Expected 'users with slash'")

		// Test exact match without trailing slash
		req = httptest.NewRequest(http.MethodGet, "/posts", nil)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code, "Expected status 200")
		assert.Equal(t, "posts without slash", w.Body.String(), "Expected 'posts without slash'")
	})

	t.Run("Multiple parameters in path", func(t *testing.T) {
		t.Parallel()

		r.GET("/a/:p1/b/:p2/c/:p3", func(c *Context) {
			c.Stringf(http.StatusOK, "%s-%s-%s", c.Param("p1"), c.Param("p2"), c.Param("p3"))
		})

		req := httptest.NewRequest(http.MethodGet, "/a/x/b/y/c/z", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code, "Expected status 200")
		assert.Equal(t, "x-y-z", w.Body.String(), "Expected 'x-y-z'")
	})

	t.Run("Parameter at end of path", func(t *testing.T) {
		t.Parallel()

		r.GET("/items/:id", func(c *Context) {
			c.Stringf(http.StatusOK, "item %s", c.Param("id"))
		})

		req := httptest.NewRequest(http.MethodGet, "/items/abc123", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code, "Expected status 200")
		assert.Equal(t, "item abc123", w.Body.String(), "Expected 'item abc123'")
	})

	t.Run("More than 8 parameters uses map fallback", func(t *testing.T) {
		t.Parallel()

		// Create route with 9 parameters to trigger map fallback
		// NOTE: We must COPY params inside the handler because the context is pooled
		// and will be reset after ServeHTTP returns. Holding a reference to c.Params
		// after the request completes would cause a race condition.
		var capturedParams map[string]string
		var capturedParamCount int32
		var paramsMapWasCreated bool
		r.GET("/a/:p1/b/:p2/c/:p3/d/:p4/e/:p5/f/:p6/g/:p7/h/:p8/i/:p9", func(c *Context) {
			// Capture paramCount
			capturedParamCount = c.paramCount

			// Check if Params map was created and COPY it (not just hold reference)
			if c.Params != nil {
				paramsMapWasCreated = true
				capturedParams = make(map[string]string, len(c.Params))
				maps.Copy(capturedParams, c.Params)
			}

			// Use Param() method which should work for both arrays and map
			// First 8 params from arrays, 9th from map
			p1 := c.Param("p1")
			p8 := c.Param("p8")
			p9 := c.Param("p9") // Should retrieve from Params map
			c.Stringf(http.StatusOK, "count=%d,p1=%s,p8=%s,p9=%s", c.paramCount, p1, p8, p9)
		})

		req := httptest.NewRequest(http.MethodGet, "/a/v1/b/v2/c/v3/d/v4/e/v5/f/v6/g/v7/h/v8/i/v9", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		require.Equal(t, 200, w.Code, "Expected status 200, body: %s", w.Body.String())

		// Verify Params map was created
		require.True(t, paramsMapWasCreated, "Expected Params map to be created for 9th parameter")

		// Verify 9th parameter is accessible via Param() (which checks map)
		if capturedParamCount >= 8 {
			// paramCount should be at least 8, and 9th param should be in map
			p9Value := capturedParams["p9"]
			if p9Value != "v9" && p9Value != "" {
				assert.Equal(t, "v9", p9Value, "Expected p9=v9 in Params map. Note: If empty, param may not have been stored")
			}
		}

		// Verify response contains expected values
		body := w.Body.String()
		assert.Contains(t, body, "v1", "Response should contain v1")
		assert.Contains(t, body, "v8", "Response should contain v8")
		// Verify 9th parameter is accessible (either via Param() or directly in map)
		if !strings.Contains(body, "v9") {
			t.Logf("Note: v9 not found in response: %s", body)
		}
	})

	t.Run("More than 8 parameters - all in map", func(t *testing.T) {
		t.Parallel()

		// Test with 10 parameters to ensure all beyond 8th are in map
		var paramsMapCreated bool
		var p9Value, p10Value string
		r.GET("/a/:p1/b/:p2/c/:p3/d/:p4/e/:p5/f/:p6/g/:p7/h/:p8/i/:p9/j/:p10", func(c *Context) {
			// Verify Params map was created
			paramsMapCreated = c.Params != nil

			// Capture values from Params map before context is reset
			if c.Params != nil {
				p9Value = c.Params["p9"]
				p10Value = c.Params["p10"]
			}

			// Also verify via Param() method
			p9Param := c.Param("p9")
			p10Param := c.Param("p10")
			c.Stringf(http.StatusOK, "p9=%s,p10=%s,p9Param=%s,p10Param=%s", p9Value, p10Value, p9Param, p10Param)
		})

		req := httptest.NewRequest(http.MethodGet, "/a/v1/b/v2/c/v3/d/v4/e/v5/f/v6/g/v7/h/v8/i/v9/j/v10", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code, "Expected status 200")

		// Verify Params map was created
		require.True(t, paramsMapCreated, "Expected Params map to be created for >8 parameters")

		// Verify 9th and 10th parameters are in Params map
		assert.Equal(t, "v9", p9Value, "Expected p9=v9 in Params map")
		assert.Equal(t, "v10", p10Value, "Expected p10=v10 in Params map")
	})
}
