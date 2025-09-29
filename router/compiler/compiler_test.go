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
	"sync"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockContextParamWriter is a mock implementation of ContextParamWriter for testing.
type mockContextParamWriter struct {
	mu     sync.Mutex
	params map[string]string
	count  int32
}

func (m *mockContextParamWriter) SetParam(index int, key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.params == nil {
		m.params = make(map[string]string)
	}
	m.params[key] = value
}

func (m *mockContextParamWriter) SetParamMap(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.params == nil {
		m.params = make(map[string]string)
	}
	m.params[key] = value
}

func (m *mockContextParamWriter) SetParamCount(count int32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count = count
}

// TestCompileRoute tests route compilation with various patterns.
func TestCompileRoute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		method          string
		pattern         string
		wantSegments    int32
		wantStatic      bool
		wantWildcard    bool
		wantConstraints bool
	}{
		{
			name:         "simple static route",
			method:       "GET",
			pattern:      "/users",
			wantSegments: 1,
			wantStatic:   true,
			wantWildcard: false,
		},
		{
			name:         "multi-segment static route",
			method:       "GET",
			pattern:      "/api/v1/users",
			wantSegments: 3,
			wantStatic:   true,
			wantWildcard: false,
		},
		{
			name:         "route with single parameter",
			method:       "GET",
			pattern:      "/users/:id",
			wantSegments: 2,
			wantStatic:   false,
			wantWildcard: false,
		},
		{
			name:         "route with multiple parameters",
			method:       "GET",
			pattern:      "/users/:id/posts/:pid",
			wantSegments: 4,
			wantStatic:   false,
			wantWildcard: false,
		},
		{
			name:         "wildcard route",
			method:       "GET",
			pattern:      "/static/*",
			wantSegments: 2, // "static" and "*"
			wantStatic:   false,
			wantWildcard: true,
		},
		{
			name:         "root path",
			method:       "GET",
			pattern:      "/",
			wantSegments: 0,
			wantStatic:   true,
			wantWildcard: false,
		},
		{
			name:            "route with constraint",
			method:          "GET",
			pattern:         "/users/:id",
			wantSegments:    2,
			wantStatic:      false,
			wantWildcard:    false,
			wantConstraints: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var constraints []RouteConstraint
			if tt.wantConstraints {
				constraints = []RouteConstraint{
					{Param: "id", Pattern: regexp.MustCompile(`^\d+$`)},
				}
			}

			route := CompileRoute(tt.method, tt.pattern, nil, constraints)

			require.NotNil(t, route)
			assert.Equal(t, tt.method, route.method)
			assert.Equal(t, tt.pattern, route.pattern)
			assert.Equal(t, tt.wantSegments, route.segmentCount)
			assert.Equal(t, tt.wantStatic, route.isStatic)
			assert.Equal(t, tt.wantWildcard, route.hasWildcard)

			if tt.wantConstraints {
				assert.NotEmpty(t, route.constraints, "should have constraints")
			}
		})
	}
}

// TestCompiledRoute_Getters tests the getter methods of CompiledRoute.
func TestCompiledRoute_Getters(t *testing.T) {
	t.Parallel()

	pattern := "/users/:id"
	method := "GET"
	handlers := []HandlerFunc{func() {}}

	route := CompileRoute(method, pattern, handlers, nil)

	// Test Pattern()
	assert.Equal(t, pattern, route.Pattern())

	// Test Method()
	assert.Equal(t, method, route.Method())

	// Test Handlers()
	assert.Equal(t, handlers, route.Handlers())
	assert.Len(t, route.Handlers(), 1)
}

// TestCompiledRoute_CachedHandlers tests cached handler management.
func TestCompiledRoute_CachedHandlers(t *testing.T) {
	t.Parallel()

	route := CompileRoute("GET", "/users", nil, nil)

	// Initially should be nil
	assert.Nil(t, route.CachedHandlers())

	// Set cached handlers
	mockHandlers := []int{1, 2, 3} // Just some data
	ptr := unsafe.Pointer(&mockHandlers)
	route.SetCachedHandlers(ptr)

	// Should now return the pointer
	assert.Equal(t, ptr, route.CachedHandlers())
}

// TestRouteCompiler_AddRoute tests adding routes to the compiler.
func TestRouteCompiler_AddRoute(t *testing.T) {
	t.Parallel()

	rc := NewRouteCompiler(1000, 3)

	// Add static routes
	staticRoute1 := CompileRoute("GET", "/users", nil, nil)
	staticRoute2 := CompileRoute("GET", "/posts", nil, nil)
	rc.AddRoute(staticRoute1)
	rc.AddRoute(staticRoute2)

	// Add dynamic routes
	dynamicRoute1 := CompileRoute("GET", "/users/:id", nil, nil)
	dynamicRoute2 := CompileRoute("GET", "/posts/:pid", nil, nil)
	rc.AddRoute(dynamicRoute1)
	rc.AddRoute(dynamicRoute2)

	// Add wildcard route (should not be added to static or dynamic)
	wildcardRoute := CompileRoute("GET", "/files/*", nil, nil)
	rc.AddRoute(wildcardRoute)

	rc.mu.RLock()
	defer rc.mu.RUnlock()

	assert.Len(t, rc.staticRoutes, 2, "should have 2 static routes")
	assert.Len(t, rc.dynamicRoutes, 2, "should have 2 dynamic routes")
}

// TestRouteCompiler_RemoveRoute tests route removal.
func TestRouteCompiler_RemoveRoute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		addRoutes     []struct{ method, pattern string }
		removeMethod  string
		removePattern string
		wantStatic    int
		wantDynamic   int
	}{
		{
			name: "remove static route",
			addRoutes: []struct{ method, pattern string }{
				{"GET", "/users"},
				{"GET", "/posts"},
			},
			removeMethod:  "GET",
			removePattern: "/users",
			wantStatic:    1,
			wantDynamic:   0,
		},
		{
			name: "remove dynamic route",
			addRoutes: []struct{ method, pattern string }{
				{"GET", "/users/:id"},
				{"GET", "/posts/:pid"},
			},
			removeMethod:  "GET",
			removePattern: "/users/:id",
			wantStatic:    0,
			wantDynamic:   1,
		},
		{
			name: "remove non-existent route",
			addRoutes: []struct{ method, pattern string }{
				{"GET", "/users"},
			},
			removeMethod:  "GET",
			removePattern: "/posts",
			wantStatic:    1,
			wantDynamic:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rc := NewRouteCompiler(1000, 3)

			for _, r := range tt.addRoutes {
				route := CompileRoute(r.method, r.pattern, nil, nil)
				rc.AddRoute(route)
			}

			rc.RemoveRoute(tt.removeMethod, tt.removePattern)

			rc.mu.RLock()
			defer rc.mu.RUnlock()

			assert.Len(t, rc.staticRoutes, tt.wantStatic)
			assert.Len(t, rc.dynamicRoutes, tt.wantDynamic)
		})
	}
}

// TestRouteCompiler_LookupStatic tests static route lookup.
func TestRouteCompiler_LookupStatic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		addRoutes  []struct{ method, pattern string }
		lookupPath string
		wantFound  bool
		wantRoute  string
	}{
		{
			name: "find existing route",
			addRoutes: []struct{ method, pattern string }{
				{"GET", "/users"},
				{"GET", "/posts"},
			},
			lookupPath: "/users",
			wantFound:  true,
			wantRoute:  "/users",
		},
		{
			name: "route not found",
			addRoutes: []struct{ method, pattern string }{
				{"GET", "/users"},
			},
			lookupPath: "/posts",
			wantFound:  false,
		},
		{
			name: "method mismatch",
			addRoutes: []struct{ method, pattern string }{
				{"POST", "/users"},
			},
			lookupPath: "/users", // Will be looked up with GET
			wantFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rc := NewRouteCompiler(1000, 3)

			for _, r := range tt.addRoutes {
				route := CompileRoute(r.method, r.pattern, nil, nil)
				rc.AddRoute(route)
			}

			found := rc.LookupStatic("GET", tt.lookupPath)

			if tt.wantFound {
				require.NotNil(t, found, "route should be found")
				assert.Equal(t, tt.wantRoute, found.pattern)
			} else {
				assert.Nil(t, found, "route should not be found")
			}
		})
	}
}

// TestRouteCompiler_MatchDynamic tests dynamic route matching.
func TestRouteCompiler_MatchDynamic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		routes     []string
		testPath   string
		wantMatch  bool
		wantParams map[string]string
	}{
		{
			name:      "simple parameter match",
			routes:    []string{"/users/:id"},
			testPath:  "/users/123",
			wantMatch: true,
			wantParams: map[string]string{
				"id": "123",
			},
		},
		{
			name:      "multiple parameters",
			routes:    []string{"/users/:id/posts/:pid"},
			testPath:  "/users/123/posts/456",
			wantMatch: true,
			wantParams: map[string]string{
				"id":  "123",
				"pid": "456",
			},
		},
		{
			name:      "segment count mismatch",
			routes:    []string{"/users/:id"},
			testPath:  "/users/123/extra",
			wantMatch: false,
		},
		{
			name:      "static segment mismatch",
			routes:    []string{"/users/:id"},
			testPath:  "/posts/123",
			wantMatch: false,
		},
		{
			name:      "empty path",
			routes:    []string{"/users/:id"},
			testPath:  "",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rc := NewRouteCompiler(1000, 3)

			for _, pattern := range tt.routes {
				route := CompileRoute("GET", pattern, nil, nil)
				rc.AddRoute(route)
			}

			ctx := &mockContextParamWriter{}
			matched := rc.MatchDynamic("GET", tt.testPath, ctx)

			if tt.wantMatch {
				require.NotNil(t, matched, "route should match")
				for key, expectedValue := range tt.wantParams {
					actualValue, exists := ctx.params[key]
					assert.True(t, exists, "parameter %q should exist", key)
					assert.Equal(t, expectedValue, actualValue, "parameter %q value mismatch", key)
				}
			} else {
				assert.Nil(t, matched, "route should not match")
			}
		})
	}
}

// TestRouteCompiler_MatchDynamic_FirstSegmentIndex tests first segment index optimization.
func TestRouteCompiler_MatchDynamic_FirstSegmentIndex(t *testing.T) {
	t.Parallel()

	rc := NewRouteCompiler(1000, 3)

	// Add enough routes to trigger index building (>= minRoutesForIndexing)
	patterns := []string{
		"/users/:id",
		"/posts/:id",
		"/admin/:id",
		"/api/:resource",
		"/products/:id",
		"/categories/:id",
		"/orders/:id",
		"/customers/:id",
		"/items/:id",
		"/services/:id",
		"/resources/:id", // 11 routes total
	}

	for _, pattern := range patterns {
		route := CompileRoute("GET", pattern, nil, nil)
		rc.AddRoute(route)
	}

	ctx := &mockContextParamWriter{}

	// This should trigger index building
	matched := rc.MatchDynamic("GET", "/users/123", ctx)
	require.NotNil(t, matched)
	assert.Equal(t, "123", ctx.params["id"])

	// Verify index was built
	rc.mu.RLock()
	hasIndex := rc.hasFirstSegmentIndex
	rc.mu.RUnlock()
	assert.True(t, hasIndex, "first segment index should be built")

	// Test matching with index
	ctx = &mockContextParamWriter{}
	matched = rc.MatchDynamic("GET", "/products/456", ctx)
	require.NotNil(t, matched)
	assert.Equal(t, "456", ctx.params["id"])
}

// TestRouteCompiler_MatchDynamic_Constraints tests constraint validation.
func TestRouteCompiler_MatchDynamic_Constraints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		pattern     string
		constraints []RouteConstraint
		testPath    string
		wantMatch   bool
	}{
		{
			name:    "numeric constraint match",
			pattern: "/users/:id",
			constraints: []RouteConstraint{
				{Param: "id", Pattern: regexp.MustCompile(`^\d+$`)},
			},
			testPath:  "/users/123",
			wantMatch: true,
		},
		{
			name:    "numeric constraint mismatch",
			pattern: "/users/:id",
			constraints: []RouteConstraint{
				{Param: "id", Pattern: regexp.MustCompile(`^\d+$`)},
			},
			testPath:  "/users/abc",
			wantMatch: false,
		},
		{
			name:    "multiple constraints",
			pattern: "/users/:id/posts/:pid",
			constraints: []RouteConstraint{
				{Param: "id", Pattern: regexp.MustCompile(`^\d+$`)},
				{Param: "pid", Pattern: regexp.MustCompile(`^\d+$`)},
			},
			testPath:  "/users/123/posts/456",
			wantMatch: true,
		},
		{
			name:    "first constraint fails",
			pattern: "/users/:id/posts/:pid",
			constraints: []RouteConstraint{
				{Param: "id", Pattern: regexp.MustCompile(`^\d+$`)},
				{Param: "pid", Pattern: regexp.MustCompile(`^\d+$`)},
			},
			testPath:  "/users/abc/posts/456",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rc := NewRouteCompiler(1000, 3)
			route := CompileRoute("GET", tt.pattern, nil, tt.constraints)
			rc.AddRoute(route)

			ctx := &mockContextParamWriter{}
			matched := rc.MatchDynamic("GET", tt.testPath, ctx)

			if tt.wantMatch {
				assert.NotNil(t, matched, "route should match")
			} else {
				assert.Nil(t, matched, "route should not match due to constraint")
			}
		})
	}
}

// TestRouteCompiler_Concurrent tests concurrent access to the compiler.
func TestRouteCompiler_Concurrent(t *testing.T) {
	t.Parallel()

	rc := NewRouteCompiler(1000, 3)

	// Add some initial routes
	for i := range 5 {
		pattern := "/route" + string(rune('0'+i)) + "/:param"
		route := CompileRoute("GET", pattern, nil, nil)
		rc.AddRoute(route)
	}

	var wg sync.WaitGroup
	ctx := &mockContextParamWriter{}

	// Concurrent reads
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = rc.MatchDynamic("GET", "/route0/test", ctx)
		}()
	}

	// Concurrent static lookups
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = rc.LookupStatic("GET", "/nonexistent")
		}()
	}

	wg.Wait()
}

// TestBloomFilter tests bloom filter operations.
func TestBloomFilter(t *testing.T) {
	t.Parallel()

	bf := NewBloomFilter(1000, 3)

	// Add some items
	bf.Add([]byte("test1"))
	bf.Add([]byte("test2"))
	bf.Add([]byte("test3"))

	// Test membership (should return true for added items)
	assert.True(t, bf.Test([]byte("test1")), "should contain test1")
	assert.True(t, bf.Test([]byte("test2")), "should contain test2")
	assert.True(t, bf.Test([]byte("test3")), "should contain test3")

	// Test non-membership (may return false positives, but unlikely for small sets)
	// We can't assert false here as bloom filters have false positives
	bf.Test([]byte("nonexistent"))
}

// TestBloomFilter_FalsePositives tests bloom filter false positive behavior.
func TestBloomFilter_FalsePositives(t *testing.T) {
	t.Parallel()

	bf := NewBloomFilter(100, 3) // Small size to increase false positive rate

	// Add items
	for i := range 50 {
		bf.Add([]byte("/route" + string(rune('0'+i))))
	}

	// Test that added items are all positive
	for i := range 50 {
		assert.True(t, bf.Test([]byte("/route"+string(rune('0'+i)))), "added items should test positive")
	}

	// Test items not added - may have false positives
	falsePositives := 0
	for i := range 100 {
		if bf.Test([]byte("/nonexistent" + string(rune('0'+i)))) {
			falsePositives++
		}
	}

	// With 100 tests, we expect some false positives but not all
	assert.Less(t, falsePositives, 100, "should have some true negatives")
}

// BenchmarkCompileRoute benchmarks route compilation.
func BenchmarkCompileRoute(b *testing.B) {
	b.Run("StaticRoute", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			_ = CompileRoute("GET", "/api/users", nil, nil)
		}
	})

	b.Run("DynamicRoute", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			_ = CompileRoute("GET", "/api/users/:id/posts/:pid", nil, nil)
		}
	})

	b.Run("WithConstraints", func(b *testing.B) {
		constraints := []RouteConstraint{
			{Param: "id", Pattern: regexp.MustCompile(`^\d+$`)},
			{Param: "pid", Pattern: regexp.MustCompile(`^\d+$`)},
		}

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			_ = CompileRoute("GET", "/api/users/:id/posts/:pid", nil, constraints)
		}
	})
}

// BenchmarkRouteCompiler_LookupStatic benchmarks static route lookup.
func BenchmarkRouteCompiler_LookupStatic(b *testing.B) {
	rc := NewRouteCompiler(1000, 3)

	// Add some static routes
	for _, path := range []string{"/users", "/posts", "/comments", "/admin", "/api"} {
		route := CompileRoute("GET", path, nil, nil)
		rc.AddRoute(route)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = rc.LookupStatic("GET", "/users")
	}
}

// BenchmarkRouteCompiler_MatchDynamic benchmarks dynamic route matching.
func BenchmarkRouteCompiler_MatchDynamic(b *testing.B) {
	rc := NewRouteCompiler(1000, 3)

	// Add some dynamic routes
	for _, pattern := range []string{
		"/users/:id",
		"/posts/:pid",
		"/users/:id/posts/:pid",
		"/api/:version/users/:id",
	} {
		route := CompileRoute("GET", pattern, nil, nil)
		rc.AddRoute(route)
	}

	ctx := &mockContextParamWriter{}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		ctx.params = nil
		ctx.count = 0
		_ = rc.MatchDynamic("GET", "/users/123/posts/456", ctx)
	}
}

// BenchmarkBloomFilter benchmarks bloom filter operations.
func BenchmarkBloomFilter(b *testing.B) {
	bf := NewBloomFilter(10000, 3)

	// Add items
	for i := range 100 {
		bf.Add([]byte("/route" + string(rune('0'+i))))
	}

	key := []byte("/route50")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = bf.Test(key)
	}
}
