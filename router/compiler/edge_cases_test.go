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

package compiler

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMatchAndExtract_EdgeCases tests edge cases in matchAndExtract function.
func TestMatchAndExtract_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pattern    string
		testPath   string
		wantMatch  bool
		wantParams map[string]string
	}{
		{
			name:      "root path match",
			pattern:   "/",
			testPath:  "/",
			wantMatch: true,
		},
		{
			name:      "root path with empty string",
			pattern:   "/",
			testPath:  "",
			wantMatch: true,
		},
		{
			name:      "path without leading slash",
			pattern:   "/users/:id",
			testPath:  "users/123",
			wantMatch: false,
		},
		{
			name:      "path too short",
			pattern:   "/users/:id",
			testPath:  "/u",
			wantMatch: false,
		},
		{
			name:      "path with trailing slash mismatch",
			pattern:   "/users/:id",
			testPath:  "/users/123/",
			wantMatch: false,
		},
		{
			name:      "single parameter fast path - exact match",
			pattern:   "/users/:id",
			testPath:  "/users/123",
			wantMatch: true,
			wantParams: map[string]string{
				"id": "123",
			},
		},
		{
			name:      "single parameter fast path - no second segment",
			pattern:   "/users/:id",
			testPath:  "/users",
			wantMatch: false,
		},
		{
			name:      "single parameter fast path - too many segments",
			pattern:   "/users/:id",
			testPath:  "/users/123/extra",
			wantMatch: false,
		},
		{
			name:      "single parameter fast path - static segment mismatch",
			pattern:   "/users/:id",
			testPath:  "/posts/123",
			wantMatch: false,
		},
		{
			name:      "path with many segments",
			pattern:   "/a/:b/c/:d/e/:f/g/:h",
			testPath:  "/a/1/c/2/e/3/g/4",
			wantMatch: true,
			wantParams: map[string]string{
				"b": "1",
				"d": "2",
				"f": "3",
				"h": "4",
			},
		},
		{
			name:      "path with 9 parameters (>8)",
			pattern:   "/:p1/:p2/:p3/:p4/:p5/:p6/:p7/:p8/:p9",
			testPath:  "/1/2/3/4/5/6/7/8/9",
			wantMatch: true,
			wantParams: map[string]string{
				"p1": "1", "p2": "2", "p3": "3", "p4": "4",
				"p5": "5", "p6": "6", "p7": "7", "p8": "8", "p9": "9",
			},
		},
		{
			name:      "UTF-8 characters in path",
			pattern:   "/users/:id",
			testPath:  "/users/ユーザー",
			wantMatch: true,
			wantParams: map[string]string{
				"id": "ユーザー",
			},
		},
		{
			name:      "empty parameter value",
			pattern:   "/users/:id/posts",
			testPath:  "/users//posts",
			wantMatch: true,
			wantParams: map[string]string{
				"id": "",
			},
		},
		{
			name:      "segment count exact match required",
			pattern:   "/a/:b/c",
			testPath:  "/a/1/c/extra",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			route := CompileRoute("GET", tt.pattern, nil, nil)
			ctx := &mockContextParamWriter{}
			matched := route.matchAndExtract(tt.testPath, ctx)

			if tt.wantMatch {
				assert.True(t, matched, "route should match")
				if tt.wantParams != nil {
					for key, expectedValue := range tt.wantParams {
						actualValue, exists := ctx.params[key]
						assert.True(t, exists, "parameter %q should exist", key)
						assert.Equal(t, expectedValue, actualValue, "parameter %q value mismatch", key)
					}
				}
			} else {
				assert.False(t, matched, "route should not match")
			}
		})
	}
}

// TestMatchAndExtract_Constraints tests parameter constraint validation in matchAndExtract.
func TestMatchAndExtract_Constraints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		pattern     string
		constraints []RouteConstraint
		testPath    string
		wantMatch   bool
	}{
		{
			name:    "constraint passes - single param fast path",
			pattern: "/users/:id",
			constraints: []RouteConstraint{
				{Param: "id", Pattern: regexp.MustCompile(`^\d+$`)},
			},
			testPath:  "/users/123",
			wantMatch: true,
		},
		{
			name:    "constraint fails - single param fast path",
			pattern: "/users/:id",
			constraints: []RouteConstraint{
				{Param: "id", Pattern: regexp.MustCompile(`^\d+$`)},
			},
			testPath:  "/users/abc",
			wantMatch: false,
		},
		{
			name:    "constraint passes - multiple params",
			pattern: "/users/:id/posts/:pid",
			constraints: []RouteConstraint{
				{Param: "id", Pattern: regexp.MustCompile(`^\d+$`)},
				{Param: "pid", Pattern: regexp.MustCompile(`^\d+$`)},
			},
			testPath:  "/users/123/posts/456",
			wantMatch: true,
		},
		{
			name:    "constraint fails - first param",
			pattern: "/users/:id/posts/:pid",
			constraints: []RouteConstraint{
				{Param: "id", Pattern: regexp.MustCompile(`^\d+$`)},
				{Param: "pid", Pattern: regexp.MustCompile(`^\d+$`)},
			},
			testPath:  "/users/abc/posts/456",
			wantMatch: false,
		},
		{
			name:    "constraint fails - second param",
			pattern: "/users/:id/posts/:pid",
			constraints: []RouteConstraint{
				{Param: "id", Pattern: regexp.MustCompile(`^\d+$`)},
				{Param: "pid", Pattern: regexp.MustCompile(`^\d+$`)},
			},
			testPath:  "/users/123/posts/abc",
			wantMatch: false,
		},
		{
			name:    "nil constraint pattern - no validation",
			pattern: "/users/:id",
			constraints: []RouteConstraint{
				{Param: "id", Pattern: nil},
			},
			testPath:  "/users/anything",
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			route := CompileRoute("GET", tt.pattern, nil, tt.constraints)
			ctx := &mockContextParamWriter{}
			matched := route.matchAndExtract(tt.testPath, ctx)

			assert.Equal(t, tt.wantMatch, matched)
		})
	}
}

// TestMatchDynamic_MethodMismatch tests that MatchDynamic respects HTTP method.
func TestMatchDynamic_MethodMismatch(t *testing.T) {
	t.Parallel()

	rc := NewRouteCompiler(1000, 3)

	// Add GET route
	route := CompileRoute("GET", "/users/:id", nil, nil)
	rc.AddRoute(route)

	tests := []struct {
		name      string
		method    string
		wantMatch bool
	}{
		{"matching method", "GET", true},
		{"different method", "POST", false},
		{"another method", "PUT", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &mockContextParamWriter{}
			matched := rc.MatchDynamic(tt.method, "/users/123", ctx)

			if tt.wantMatch {
				assert.NotNil(t, matched, "should match %s method", tt.method)
			} else {
				assert.Nil(t, matched, "should not match %s method", tt.method)
			}
		})
	}
}

// TestMatchDynamic_NonASCIIPaths tests handling of non-ASCII paths.
func TestMatchDynamic_NonASCIIPaths(t *testing.T) {
	t.Parallel()

	rc := NewRouteCompiler(1000, 3)

	// Add routes with various first characters
	patterns := []string{
		"/users/:id",
		"/ユーザー/:id", // Japanese characters
		"/用户/:id",   // Chinese characters
		"/사용자/:id",  // Korean characters
	}

	for _, pattern := range patterns {
		route := CompileRoute("GET", pattern, nil, nil)
		rc.AddRoute(route)
	}

	tests := []struct {
		name      string
		path      string
		wantMatch bool
	}{
		{"ASCII path", "/users/123", true},
		{"Japanese path", "/ユーザー/456", true},
		{"Chinese path", "/用户/789", true},
		{"Korean path", "/사용자/012", true},
		{"non-matching non-ASCII", "/άλλο/123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &mockContextParamWriter{}
			matched := rc.MatchDynamic("GET", tt.path, ctx)

			if tt.wantMatch {
				assert.NotNil(t, matched, "should match path %s", tt.path)
			} else {
				assert.Nil(t, matched, "should not match path %s", tt.path)
			}
		})
	}
}

// TestBuildFirstSegmentIndex_EdgeCases tests edge cases in first segment index building.
func TestBuildFirstSegmentIndex_EdgeCases(t *testing.T) {
	t.Parallel()

	rc := NewRouteCompiler(1000, 3)

	// Add routes with various patterns
	patterns := []string{
		"/",          // root route
		"/users/:id", // normal ASCII
		"/ユーザー/:id",  // non-ASCII (shouldn't be indexed)
		"/:param",    // parameter at start
	}

	for _, pattern := range patterns {
		route := CompileRoute("GET", pattern, nil, nil)
		rc.AddRoute(route)
	}

	// Force index building
	rc.buildFirstSegmentIndex()

	// Verify index was built
	assert.True(t, rc.hasFirstSegmentIndex, "index should be built")

	// Check that ASCII routes are indexed
	assert.NotEmpty(t, rc.firstSegmentIndex['u'], "ASCII 'u' should be indexed")

	// The index only covers 0-127 ASCII range (array size 128)
	// Non-ASCII paths (byte values 128-255) fall back to linear scan
	// and are correctly handled by the fallback path in MatchDynamic
}

// TestLookupStatic_EdgeCases tests edge cases in static route lookup.
func TestLookupStatic_EdgeCases(t *testing.T) {
	t.Parallel()

	rc := NewRouteCompiler(1000, 3)

	tests := []struct {
		name      string
		method    string
		path      string
		wantMatch bool
	}{
		{
			name:      "empty method and path",
			method:    "",
			path:      "",
			wantMatch: false,
		},
		{
			name:      "empty path",
			method:    "GET",
			path:      "",
			wantMatch: false,
		},
		{
			name:      "empty method",
			method:    "",
			path:      "/users",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := rc.LookupStatic(tt.method, tt.path)

			if tt.wantMatch {
				assert.NotNil(t, matched)
			} else {
				assert.Nil(t, matched)
			}
		})
	}
}

// TestMatchDynamic_PathEdgeCases tests various path edge cases.
func TestMatchDynamic_PathEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		pattern   string
		testPath  string
		wantMatch bool
	}{
		{
			name:      "path with special characters in param",
			pattern:   "/files/:name",
			testPath:  "/files/file%20name",
			wantMatch: true,
		},
		{
			name:      "path with dots in param",
			pattern:   "/files/:name",
			testPath:  "/files/file.txt",
			wantMatch: true,
		},
		{
			name:      "path with dashes in param",
			pattern:   "/users/:id",
			testPath:  "/users/user-123",
			wantMatch: true,
		},
		{
			name:      "path with underscores in param",
			pattern:   "/users/:id",
			testPath:  "/users/user_123",
			wantMatch: true,
		},
		{
			name:      "very long parameter value",
			pattern:   "/users/:id",
			testPath:  "/users/" + string(make([]byte, 1000)),
			wantMatch: true,
		},
		{
			name:      "single character parameter",
			pattern:   "/users/:id",
			testPath:  "/users/a",
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rc := NewRouteCompiler(1000, 3)
			route := CompileRoute("GET", tt.pattern, nil, nil)
			rc.AddRoute(route)

			ctx := &mockContextParamWriter{}
			matched := rc.MatchDynamic("GET", tt.testPath, ctx)

			if tt.wantMatch {
				assert.NotNil(t, matched, "should match path %s", tt.testPath)
			} else {
				assert.Nil(t, matched, "should not match path %s", tt.testPath)
			}
		})
	}
}
