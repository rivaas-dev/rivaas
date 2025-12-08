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
	"sync/atomic"

	"rivaas.dev/router/compiler"
)

// countStaticRoutesForMethod counts the number of static routes (no parameters) in a method tree.
// This is used to determine optimal bloom filter size.
func (r *Router) countStaticRoutesForMethod(method string) int {
	tree := r.getTreeForMethodDirect(method)
	if tree == nil {
		return 0
	}

	return tree.countStaticRoutes()
}

// optimalBloomFilterSize calculates the bloom filter size based on route count.
// Uses the formula: m = -n*ln(p) / (ln(2)^2) where:
//   - n = number of routes
//   - p = desired false positive rate (0.01 = 1%)
//   - m = bits needed
//
// Uses 10 bits per route for approximately 1% false positive rate.
func optimalBloomFilterSize(routeCount int) uint64 {
	if routeCount <= 0 {
		return defaultBloomFilterSize
	}
	// Calculate size based on route count
	// Minimum size of 100 to avoid degenerate cases
	size := uint64(routeCount * 10)
	if size < 100 {
		return 100
	}
	// Cap at maximum size
	if size > 1000000 {
		return 1000000
	}

	return size
}

func (r *Router) compileRoutesForMethod(method string) {
	tree := r.getTreeForMethodDirect(method)
	if tree == nil {
		return
	}

	// Calculate optimal bloom filter size based on route count
	// If user hasn't explicitly set a size, auto-size based on routes
	bloomSize := r.bloomFilterSize
	if bloomSize == defaultBloomFilterSize {
		// Count static routes in this tree to determine optimal size
		routeCount := r.countStaticRoutesForMethod(method)
		bloomSize = optimalBloomFilterSize(routeCount)
	}

	// Compile routes
	_ = tree.compileStaticRoutes(bloomSize, r.bloomHashFunctions)
}

// CompileAllRoutes pre-compiles all static routes.
// This should be called after all routes are registered.
func (r *Router) CompileAllRoutes() {
	treesPtr := atomic.LoadPointer(&r.routeTree.trees)
	trees := (*map[string]*node)(treesPtr)

	for method := range *trees {
		r.compileRoutesForMethod(method)
	}
}

// Warmup registers all pending routes and pre-compiles them for optimal request handling.
// This should be called after all routes are registered and before serving requests.
//
// Warmup phases:
// 1. Register all pending routes to their appropriate trees (standard or version-specific)
// 2. Compile all static routes into hash tables with bloom filters
// 3. Compile version-specific routes if versioning is enabled
//
// Warmup prepares the router for handling requests by registering routes,
// compiling data structures, and initializing caches before traffic arrives.
//
// Calling Warmup() multiple times is safe - routes are only registered once.
func (r *Router) Warmup() {
	r.warmupOnce.Do(r.doWarmup)
}

// doWarmup performs the actual warmup work (called via sync.Once).
func (r *Router) doWarmup() {
	// CRITICAL: Set warmedUp=true BEFORE clearing pendingRoutes to avoid race condition
	// Without this, routes added between clearing pendingRoutes and setting warmedUp=true
	// would be lost (added to empty pendingRoutes, but warmedUp still false, warmup done)
	r.pendingRoutesMu.Lock()
	r.warmedUp = true
	routes := r.pendingRoutes
	r.pendingRoutes = nil // Clear pending routes
	r.pendingRoutesMu.Unlock()

	// Phase 1: Register all pending routes to their appropriate trees
	for _, rt := range routes {
		rt.RegisterRoute()
	}

	// Phase 2: Compile all standard (non-versioned) routes
	r.CompileAllRoutes()

	// Phase 3: Compile version-specific routes if versioning is enabled
	if r.versionEngine != nil {
		r.compileVersionRoutes()
	}
}

// compileVersionRoutes compiles static routes for all version-specific trees
// and stores them in the version cache (sync.Map).
// This enables lookup for versioned static routes.
// Cache key format: "version:method" (e.g., "v1:GET")
func (r *Router) compileVersionRoutes() {
	// Load version trees atomically
	versionTreesPtr := atomic.LoadPointer(&r.versionTrees.trees)
	if versionTreesPtr == nil {
		return // No version-specific routes registered
	}

	versionTrees := *(*map[string]map[string]*node)(versionTreesPtr)

	// Compile static routes for each version AND method
	// Each method gets its own compiled table to avoid handler conflicts
	for version, methodTrees := range versionTrees {
		for method, tree := range methodTrees {
			if tree == nil {
				continue
			}

			// Count static routes for this method tree
			staticRoutes := tree.countStaticRoutes()
			if staticRoutes == 0 {
				continue
			}

			// Calculate optimal bloom filter size
			bloomSize := r.bloomFilterSize
			if bloomSize == defaultBloomFilterSize {
				bloomSize = optimalBloomFilterSize(staticRoutes)
			}

			// Create compiled table for this version+method combination
			compiled := &CompiledRouteTable{
				routes: make(map[uint64]*CompiledRoute),
				bloom:  compiler.NewBloomFilter(bloomSize, r.bloomHashFunctions),
			}

			// Compile routes from this method's tree
			tree.compileStaticRoutesRecursive(compiled, "")

			// Store with version:method key
			if len(compiled.routes) > 0 {
				cacheKey := version + ":" + method
				r.versionCache.Store(cacheKey, compiled)
			}
		}
	}
}

// recordRouteRegistration is a hook for route registration tracking.
// Currently a no-op; route registration is tracked via RouteInfo in the route tree.
// Diagnostic events are reserved for runtime anomalies (security, performance),
// not routine setup events which would be too noisy.
func (r *Router) recordRouteRegistration(method, path string) {
	// Intentionally empty - route registration is tracked via r.routeTree.routes
	// Diagnostic events are for runtime anomalies, not routine setup
	_ = method
	_ = path
}
