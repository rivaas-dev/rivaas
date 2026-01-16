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

package compiler

import (
	"regexp"
	"testing"
)

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

// BenchmarkRouteCompiler_LookupStatic_Miss benchmarks static route lookup for non-existent routes.
func BenchmarkRouteCompiler_LookupStatic_Miss(b *testing.B) {
	rc := NewRouteCompiler(1000, 3)

	// Add some static routes
	for _, path := range []string{"/users", "/posts", "/comments", "/admin", "/api"} {
		route := CompileRoute("GET", path, nil, nil)
		rc.AddRoute(route)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = rc.LookupStatic("GET", "/nonexistent")
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

	ctx := &testContextParamWriter{}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		ctx.params = nil
		ctx.count = 0
		_ = rc.MatchDynamic("GET", "/users/123/posts/456", ctx)
	}
}

// BenchmarkRouteCompiler_MatchDynamic_SingleParam benchmarks matching routes with a single parameter.
func BenchmarkRouteCompiler_MatchDynamic_SingleParam(b *testing.B) {
	rc := NewRouteCompiler(1000, 3)

	route := CompileRoute("GET", "/users/:id", nil, nil)
	rc.AddRoute(route)

	ctx := &testContextParamWriter{}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		ctx.params = nil
		ctx.count = 0
		_ = rc.MatchDynamic("GET", "/users/123", ctx)
	}
}

// BenchmarkRouteCompiler_MatchDynamic_WithConstraints benchmarks matching with constraints.
func BenchmarkRouteCompiler_MatchDynamic_WithConstraints(b *testing.B) {
	rc := NewRouteCompiler(1000, 3)

	constraints := []RouteConstraint{
		{Param: "id", Pattern: regexp.MustCompile(`^\d+$`)},
	}
	route := CompileRoute("GET", "/users/:id", nil, constraints)
	rc.AddRoute(route)

	ctx := &testContextParamWriter{}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		ctx.params = nil
		ctx.count = 0
		_ = rc.MatchDynamic("GET", "/users/123", ctx)
	}
}

// BenchmarkRouteCompiler_MatchDynamic_ManyRoutes benchmarks matching with many routes.
func BenchmarkRouteCompiler_MatchDynamic_ManyRoutes(b *testing.B) {
	rc := NewRouteCompiler(10000, 3)

	// Add many routes to trigger first-segment index
	prefixes := []string{"users", "posts", "comments", "articles", "products", "categories", "orders", "customers", "items", "services", "resources", "data"}
	for _, prefix := range prefixes {
		route := CompileRoute("GET", "/"+prefix+"/:id", nil, nil)
		rc.AddRoute(route)
	}

	ctx := &testContextParamWriter{}

	// Warm up to build index
	_ = rc.MatchDynamic("GET", "/users/123", ctx)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		ctx.params = nil
		ctx.count = 0
		_ = rc.MatchDynamic("GET", "/products/456", ctx)
	}
}

// BenchmarkBloomFilter benchmarks bloom filter operations.
func BenchmarkBloomFilter(b *testing.B) {
	b.Run("Add", func(b *testing.B) {
		bf := NewBloomFilter(10000, 3)
		key := []byte("/users/123")

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			bf.Add(key)
		}
	})

	b.Run("Test_Hit", func(b *testing.B) {
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
	})

	b.Run("Test_Miss", func(b *testing.B) {
		bf := NewBloomFilter(10000, 3)

		// Add items
		for i := range 100 {
			bf.Add([]byte("/route" + string(rune('0'+i))))
		}

		key := []byte("/nonexistent")

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			_ = bf.Test(key)
		}
	})
}

// BenchmarkMatchAndExtract benchmarks the internal matchAndExtract function.
func BenchmarkMatchAndExtract(b *testing.B) {
	b.Run("SingleParam_FastPath", func(b *testing.B) {
		route := CompileRoute("GET", "/users/:id", nil, nil)
		ctx := &testContextParamWriter{}

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			ctx.params = nil
			ctx.count = 0
			_ = route.matchAndExtract("/users/123", ctx)
		}
	})

	b.Run("MultipleParams", func(b *testing.B) {
		route := CompileRoute("GET", "/users/:id/posts/:pid/comments/:cid", nil, nil)
		ctx := &testContextParamWriter{}

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			ctx.params = nil
			ctx.count = 0
			_ = route.matchAndExtract("/users/123/posts/456/comments/789", ctx)
		}
	})

	b.Run("WithConstraints", func(b *testing.B) {
		constraints := []RouteConstraint{
			{Param: "id", Pattern: regexp.MustCompile(`^\d+$`)},
		}
		route := CompileRoute("GET", "/users/:id", nil, constraints)
		ctx := &testContextParamWriter{}

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			ctx.params = nil
			ctx.count = 0
			_ = route.matchAndExtract("/users/123", ctx)
		}
	})
}
