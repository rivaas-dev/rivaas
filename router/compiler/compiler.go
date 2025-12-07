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
	"hash/fnv"
	"regexp"
	"strings"
	"sync"
	"unsafe"
)

// HandlerFunc defines the handler function signature.
// This is a copy of router.HandlerFunc to avoid import cycles.
type HandlerFunc interface{}

// RouteConstraint represents a compiled constraint for route parameters.
// This is a copy of router.RouteConstraint to avoid import cycles.
type RouteConstraint struct {
	Param   string
	Pattern *regexp.Regexp
}

const (
	// minRoutesForIndexing is the minimum number of dynamic routes required
	// before building the first-segment index for filtering.
	// Below this threshold, the index is not built.
	minRoutesForIndexing = 10
)

// CompiledRoute represents a pre-compiled route with metadata for matching.
// It pre-computes route structure information during registration,
// including segment positions, parameter names, and constraint patterns.
type CompiledRoute struct {
	// Route identification
	method  string // HTTP method (GET, POST, etc.)
	pattern string // Original pattern (/users/:id/posts/:pid)
	hash    uint64 // Pre-computed hash for lookup

	// Route structure (pre-computed during registration)
	segmentCount   int32    // Total number of segments
	staticSegments []string // Static segments that must match exactly
	staticPos      []int32  // Positions of static segments
	paramNames     []string // Parameter names in extraction order
	paramPos       []int32  // Positions where parameters are extracted

	// Validation (pre-compiled)
	constraints []*regexp.Regexp // Constraints indexed by parameter position

	// Handler chain
	handlers []HandlerFunc

	// Cached converted handlers (set by router after compilation)
	// This avoids creating a new slice on every request
	// The pointer points to []router.HandlerFunc, stored as unsafe.Pointer
	// to avoid import cycles
	cachedHandlers unsafe.Pointer

	// Flags
	isStatic       bool // True if route has no parameters
	hasWildcard    bool // True if route has wildcard
	hasConstraints bool // True if route has parameter constraints
}

// RouteCompiler manages compiled routes for lookup.
// It organizes routes into static routes (exact path matches) and
// dynamic routes (routes with parameters). Routes are matched using
// the compiled metadata stored in CompiledRoute.
type RouteCompiler struct {
	// Static route table: method+path → handlers
	staticRoutes map[uint64]*CompiledRoute
	staticBloom  *BloomFilter

	// Dynamic routes: ordered by specificity
	dynamicRoutes []*CompiledRoute

	// First-segment index for filtering (ASCII-only)
	// Maps first character after '/' to routes that start with that character.
	// Example: 'u' → ["/users/:id", "/user/profile"]
	// UTF-8 paths beyond ASCII fall back to scanning all routes.
	firstSegmentIndex    [128][]*CompiledRoute // ASCII lookup (0-127)
	hasFirstSegmentIndex bool                  // Whether index is built

	// Mutex protects route compiler during updates
	mu sync.RWMutex
}

// NewRouteCompiler creates a new route compiler
func NewRouteCompiler(bloomSize uint64, numHashFuncs int) *RouteCompiler {
	return &RouteCompiler{
		staticRoutes:  make(map[uint64]*CompiledRoute, 64),
		dynamicRoutes: make([]*CompiledRoute, 0, 32),
		staticBloom:   NewBloomFilter(bloomSize, numHashFuncs),
	}
}

// CompileRoute compiles a route pattern into a compiled route for matching.
// It pre-computes route structure information during registration,
// including segment positions, parameter names, and constraint patterns.
func CompileRoute(method, pattern string, handlers []HandlerFunc, constraints []RouteConstraint) *CompiledRoute {
	// Normalize pattern
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		pattern = "/"
	}

	h := fnv.New64a()
	h.Write([]byte(method + pattern))
	route := &CompiledRoute{
		method:   method,
		pattern:  pattern,
		handlers: handlers,
		hash:     h.Sum64(),
	}

	// Handle root path
	if pattern == "/" {
		route.isStatic = true
		route.segmentCount = 0
		return route
	}

	// Parse pattern into segments
	segments := strings.Split(strings.Trim(pattern, "/"), "/")
	route.segmentCount = int32(len(segments))

	// Check for wildcard
	if len(segments) > 0 && strings.HasSuffix(segments[len(segments)-1], "*") {
		route.hasWildcard = true
		// Wildcard routes use tree fallback
		return route
	}

	// Pre-allocate slices with known capacity
	staticSegs := make([]string, 0, len(segments))
	staticPositions := make([]int32, 0, len(segments))
	paramNames := make([]string, 0, len(segments)/2)
	paramPositions := make([]int32, 0, len(segments)/2)
	constraintsList := make([]*regexp.Regexp, 0, len(segments)/2)

	// Analyze each segment
	for i, seg := range segments {
		if strings.HasPrefix(seg, ":") {
			// Parameter segment
			paramName := seg[1:]
			paramNames = append(paramNames, paramName)
			paramPositions = append(paramPositions, int32(i))

			// Find matching constraint
			var constraint *regexp.Regexp
			for _, c := range constraints {
				if c.Param == paramName {
					constraint = c.Pattern
					route.hasConstraints = true

					break
				}
			}
			constraintsList = append(constraintsList, constraint)
		} else {
			// Static segment
			staticSegs = append(staticSegs, seg)
			staticPositions = append(staticPositions, int32(i))
		}
	}

	// Store compiled metadata
	route.staticSegments = staticSegs
	route.staticPos = staticPositions
	route.paramNames = paramNames
	route.paramPos = paramPositions
	route.constraints = constraintsList

	// Mark as static if no parameters
	route.isStatic = len(paramNames) == 0

	return route
}

// Pattern returns the route pattern (e.g., "/users/:id")
func (r *CompiledRoute) Pattern() string {
	return r.pattern
}

// Handlers returns the handler chain for this route
func (r *CompiledRoute) Handlers() []HandlerFunc {
	return r.handlers
}

// SetCachedHandlers stores the converted handler slice.
// The handlers parameter should be a pointer to []router.HandlerFunc.
// This is called once during route compilation by the router.
func (r *CompiledRoute) SetCachedHandlers(handlers unsafe.Pointer) {
	r.cachedHandlers = handlers
}

// CachedHandlers returns the cached converted handlers, or nil if not set.
// Returns unsafe.Pointer to []router.HandlerFunc.
func (r *CompiledRoute) CachedHandlers() unsafe.Pointer {
	return r.cachedHandlers
}

// Method returns the HTTP method for this route
func (r *CompiledRoute) Method() string {
	return r.method
}

// AddRoute adds a compiled route to the compiler
func (rc *RouteCompiler) AddRoute(route *CompiledRoute) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if route.isStatic {
		// Add to static table
		rc.staticRoutes[route.hash] = route
		rc.staticBloom.Add([]byte(route.method + route.pattern))
	} else if !route.hasWildcard {
		// Add to dynamic routes (sorted by specificity)
		rc.dynamicRoutes = append(rc.dynamicRoutes, route)

		// Sort by specificity (more static segments = higher priority)
		// This ensures more specific routes match first
		rc.sortRoutesBySpecificity()

		// Invalidate first-segment index (will be rebuilt on next lookup)
		rc.hasFirstSegmentIndex = false
	}
	// Wildcard routes fall back to tree
}

// RemoveRoute removes a route from the compiler (used when updating constraints)
func (rc *RouteCompiler) RemoveRoute(method, pattern string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Calculate hash
	h := fnv.New64a()
	h.Write([]byte(method + pattern))
	hash := h.Sum64()

	// Remove from static routes
	delete(rc.staticRoutes, hash)

	// Remove from dynamic routes
	for i, route := range rc.dynamicRoutes {
		if route.method == method && route.pattern == pattern {
			// Remove by swapping with last element and slicing
			rc.dynamicRoutes[i] = rc.dynamicRoutes[len(rc.dynamicRoutes)-1]
			rc.dynamicRoutes = rc.dynamicRoutes[:len(rc.dynamicRoutes)-1]
			rc.hasFirstSegmentIndex = false

			break
		}
	}
}

// sortRoutesBySpecificity sorts routes by specificity (most specific first).
// Specificity is determined by the number of static segments.
// Routes with more static segments are considered more specific.
func (rc *RouteCompiler) sortRoutesBySpecificity() {
	routes := rc.dynamicRoutes

	// Insertion sort
	for i := 1; i < len(routes); i++ {
		key := routes[i]
		keySpecificity := len(key.staticSegments)

		j := i - 1
		for j >= 0 && len(routes[j].staticSegments) < keySpecificity {
			routes[j+1] = routes[j]
			j--
		}
		routes[j+1] = key
	}
}
