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

// Package compiler provides route compilation for the Rivaas router.
//
// The compiler package implements a route matching system through pre-compiled
// route tables. It improves routing by:
//   - Pre-computing route structure information
//   - Using bloom filters for negative lookups
//   - Building first-segment indexes for route narrowing
//   - Enabling single-pass validation and parameter extraction
//
// # Architecture
//
// The compiler uses a three-tier strategy for route matching:
//
//  1. Static routes: Hash table lookup
//  2. Dynamic routes: Pre-compiled patterns with segment-based matching
//  3. Complex routes: Tree fallback
//
// This hybrid approach handles the most common cases
// while maintaining correctness for complex routing scenarios.
//
// # Route Compilation
//
// Routes are compiled through the CompileRoute function, which:
//
//  1. Analyzes route structure (static/dynamic/wildcard segments)
//  2. Counts segments for length validation
//  3. Stores parameter positions for extraction
//  4. Attaches constraint patterns for validation
//  5. Computes specificity score for sorting
//
// Example:
//
//	route := compiler.CompileRoute(
//	    "GET",
//	    "/api/users/:id/posts/:pid",
//	    handlers,
//	    []compiler.RouteConstraint{
//	        {Param: "id", Pattern: regexp.MustCompile(`^\d+$`)},
//	        {Param: "pid", Pattern: regexp.MustCompile(`^\d+$`)},
//	    },
//	)
//
// # Bloom Filter
//
// The bloom filter provides negative lookups for static routes:
//
//   - Membership tests
//   - Zero false negatives (if not in bloom filter, definitely not in routes)
//   - Rare false positives (< 1% with proper sizing)
//   - Checks
//
// The bloom filter eliminates map lookups for non-existent routes.
//
// # First-Segment Index
//
// The first-segment index groups dynamic routes by their first static segment:
//
//	/users/:id        -> index["users"]
//	/posts/:pid       -> index["posts"]
//	/api/:resource    -> index["api"]
//
// This approach:
//
//   - Reduces candidate routes for typical workloads
//   - Enables route narrowing before pattern matching
//   - Lazy-initializes on first dynamic route match
//   - Only indexes ASCII segments (non-ASCII routes skip indexing)
//
// # Lookup Details
//
// Implementation:
//
//   - Static routes: Hash table lookup
//   - Dynamic routes: Segment-based matching
//   - Bloom filter size depends on route count
//
// Typical behavior:
//
//   - Static lookup: Hash table lookup
//   - Dynamic match: Segment-based matching
//   - Constrained match: Pattern validation with matching
//
// # Design Decisions
//
// ## ASCII-Only First-Segment Index
//
// The first-segment index only handles ASCII characters for simplicity:
//
//   - Avoids UTF-8 decoding in hot path
//   - Simplifies case-insensitive comparisons (single tolower operation)
//   - Non-ASCII routes fall back to linear scan (acceptable trade-off)
//   - Most web APIs use ASCII paths anyway
//
// ## Import Cycle Prevention
//
// The compiler package is designed to avoid import cycles with the router package:
//
//   - Defines its own HandlerFunc type (function, not interface)
//   - Uses []HandlerFunc instead of HandlerChain
//   - Defines minimal RouteConstraint type
//   - No dependencies on router internals
//
// This allows the router package to import compiler without circular dependencies.
//
// ## Storage
//
// Compiled routes store minimal metadata:
//
//   - Fixed-size arrays for parameter positions (max 8 params)
//   - String slices are shared, not copied
//   - Constraints stored as slice of structs (not pointers)
//   - Handler functions shared across router and compiler
//
// # Thread Safety
//
// All operations are thread-safe:
//
//   - RouteCompiler uses RWMutex for concurrent access
//   - Bloom filter is read-only after initialization
//   - First-segment index uses sync.Once for lazy initialization
//   - Route compilation is a pure function
//
// Concurrent reads (route matching) do not block each other.
// Route additions (compilation) are synchronized with a mutex.
//
// # Usage Example
//
// Basic usage in the router:
//
//	// Initialize compiler
//	rc := compiler.NewRouteCompiler(1000, 3) // 1000 routes, 3 hash funcs
//
//	// Register routes
//	r.GET("/users/:id", handler)
//	r.GET("/posts/:pid", handler)
//
//	// Compile routes (called by router.Warmup)
//	rc.CompileAllRoutes()
//
//	// Route matching (called by router.ServeHTTP)
//	if route := rc.LookupStatic("GET", "/users"); route != nil {
//	    // Found static route
//	}
//
//	params := make(map[string]string)
//	if route := rc.MatchDynamic("GET", "/users/123", params); route != nil {
//	    // Found dynamic route, params["id"] = "123"
//	}
//
// # See Also
//
//   - bloom.go: Bloom filter implementation for negative lookups
//   - static.go: Static route compilation and lookup
//   - dynamic.go: Dynamic route compilation and matching
//   - compiler.go: Main RouteCompiler and route compilation logic
package compiler
