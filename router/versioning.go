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
	"strings"
	"sync/atomic"
	"unsafe"

	"rivaas.dev/router/route"
	"rivaas.dev/router/version"
)

// WithVersioning configures the router with API versioning support using functional options.
// This enables version detection from headers, query parameters, paths, or Accept headers.
//
// Returns an error through New() if the versioning configuration is invalid.
// This follows the fail-fast design principle by validating configuration at startup.
//
// Example:
//
//	router := router.MustNew(
//	    router.WithVersioning(
//	        version.WithHeaderDetection("X-API-Version"),
//	        version.WithDefault("v1"),
//	    ),
//	)
//
// With multiple detection strategies:
//
//	router := router.MustNew(
//	    router.WithVersioning(
//	        version.WithPathDetection("/api/v{version}"),
//	        version.WithHeaderDetection("X-API-Version"),
//	        version.WithQueryDetection("v"),
//	        version.WithDefault("v2"),
//	        version.WithResponseHeaders(),
//	        version.WithSunsetEnforcement(),
//	    ),
//	)
//
// Error handling with New():
//
//	router, err := router.New(router.WithVersioning(invalidOpts...))
//	if err != nil {
//	    // Handle versioning configuration error
//	}
func WithVersioning(opts ...version.Option) Option {
	return func(r *Router) {
		// Store options for deferred validation in validate()
		// This ensures errors are returned through New() instead of panicking
		r.versionOpts = opts
	}
}

// versionContext holds the result of pre-routing version detection and path processing.
// This is computed once before routing to determine which tree to use and how to match paths.
type versionContext struct {
	version     string // Detected version (e.g., "v1", "v2")
	routingPath string // Path after version stripping (for matching routes)
	tree        *node  // Version-specific tree, or nil to use standard tree
}

// atomicVersionTrees represents lock-free version-specific route trees.
//
// FIELD ORDER REQUIREMENTS:
//   - `trees` MUST be the first (and only) field for 8-byte alignment
//   - Atomic operations on unsafe.Pointer require 8-byte alignment
//   - DO NOT add fields before `trees`
//
// Alignment is verified at runtime in routes.go init() - the program will panic if misaligned.
type atomicVersionTrees struct {
	// trees is an atomic pointer to version-specific route trees
	// CRITICAL: Must be first field for 8-byte alignment (verified in init())
	trees unsafe.Pointer // *map[string]map[string]*node (version -> method -> tree)
}

// VersionRouter represents a version-specific router with lifecycle configuration.
type VersionRouter struct {
	router    *Router
	version   string
	lifecycle *version.LifecycleConfig
}

// selectRoutingTree selects the appropriate route tree based on version and method.
// Returns nil if no version-specific tree exists, indicating standard routing should be used.
//
// If the requested version's tree doesn't exist, falls back to the default version tree.
func (r *Router) selectRoutingTree(method, ver string) *node {
	if r.versionEngine == nil || ver == "" {
		return nil
	}

	// Try to get version-specific tree
	tree := r.getVersionTree(ver, method)
	if tree != nil {
		return tree
	}

	// Fallback to default version tree if available
	cfg := r.versionEngine.Config()
	if cfg.DefaultVersion() != "" && ver != cfg.DefaultVersion() {
		tree = r.getVersionTree(cfg.DefaultVersion(), method)
		if tree != nil {
			return tree
		}
	}

	return nil
}

// processVersioning orchestrates version detection, tree selection, and path transformation.
// This runs before routing to determine how the request should be routed.
//
// The method handles all versioning logic in one place:
// 1. Determines if versioning should be applied
// 2. Detects version from request (path/header/query/accept/custom)
// 3. Prepares routing path (strips version prefix if needed)
// 4. Selects appropriate route tree
//
// Returns versionContext with routing details.
// The version will be passed down to handlers and stored in Context during request execution.
func (r *Router) processVersioning(req *http.Request, path string) versionContext {
	// Path: versioning disabled
	if r.versionEngine == nil {
		return versionContext{
			version:     "",
			routingPath: path,
			tree:        nil,
		}
	}

	// Check if we should use versioning for this request
	if !r.versionEngine.ShouldApplyVersioning(path) {
		return versionContext{
			version:     "",
			routingPath: path,
			tree:        nil,
		}
	}

	// Step 1: Detect version from request using the engine
	ver := r.versionEngine.DetectVersion(req)

	// Step 2: Prepare routing path (strip version if needed)
	// For path-based versioning, we need to strip the actual segment from the path,
	// even if it's invalid (e.g., "/v99/users" should strip "/v99/" and route to default version)
	routingPath := path
	if toStrip, ok := r.versionEngine.ExtractPathSegment(path); ok {
		// Strip using the actual segment from the path, not the validated version
		routingPath = r.versionEngine.StripPathVersion(path, toStrip)
	}

	// Step 3: Select appropriate tree
	tree := r.selectRoutingTree(req.Method, ver)

	return versionContext{
		version:     ver,
		routingPath: routingPath,
		tree:        tree,
	}
}

// getVersionTree atomically gets the tree for a specific version and HTTP method
func (r *Router) getVersionTree(ver, method string) *node {
	versionTreesPtr := atomic.LoadPointer(&r.versionTrees.trees)
	if versionTreesPtr == nil {
		return nil
	}
	versionTrees := *(*map[string]map[string]*node)(versionTreesPtr)

	if methodTrees, exists := versionTrees[ver]; exists {
		return methodTrees[method]
	}

	return nil
}

// addVersionRoute adds a route to a specific version tree using atomic compare-and-swap
// This ensures thread-safety without locks during concurrent route registration
//
// Data structure: map[version]map[method]*node
// Example: {"v1": {"GET": tree1, "POST": tree2}, "v2": {"GET": tree3}}
//
// Why this design:
// - Phase 1 handles existing version/method combinations directly
// - Phase 2 ensures thread-safe creation of new version trees
// - Deep copy prevents race conditions when creating new method trees
func (r *Router) addVersionRoute(ver, method, path string, handlers []HandlerFunc, constraints []route.Constraint) {
	// Try to get the existing tree for this version/method combination
	// This is the common case after initial setup
	versionTreesPtr := atomic.LoadPointer(&r.versionTrees.trees)
	if versionTreesPtr != nil {
		versionTrees := *(*map[string]map[string]*node)(versionTreesPtr)
		if methodTrees, exists := versionTrees[ver]; exists {
			if tree, exists := methodTrees[method]; exists {
				// Tree exists, add route directly (thread-safe due to per-node mutex)
				// No CAS needed - we're only modifying the tree structure, not replacing pointers
				tree.addRouteWithConstraints(path, handlers, constraints)
				return
			}
		}
	}

	// Slow path: Tree doesn't exist for this version/method, need to create it atomically
	// Use CAS loop to handle concurrent creation attempts
	for {
		// Step 1: Load current version trees atomically
		versionTreesPtr := atomic.LoadPointer(&r.versionTrees.trees)
		var currentTrees map[string]map[string]*node

		if versionTreesPtr == nil {
			// No version trees exist yet, start with empty map
			currentTrees = make(map[string]map[string]*node)
		} else {
			// Version trees exist, use current snapshot
			currentTrees = *(*map[string]map[string]*node)(versionTreesPtr)
		}

		// Step 2: Double-check if another goroutine created the tree during retry
		// This is the classic "check-before-copy" pattern in CAS loops
		if methodTrees, exists := currentTrees[ver]; exists {
			if tree, exists := methodTrees[method]; exists {
				// Another goroutine won the race and created it, use it directly
				tree.addRouteWithConstraints(path, handlers, constraints)
				return
			}
		}

		// Step 3: Create a deep copy with the new method tree
		// Deep copy is required because:
		// - We share node pointers from the old tree (they're immutable after creation)
		// - But we need new method tree map to add our new tree
		// - Shallow copy would cause race: another goroutine could modify shared map
		newTrees := make(map[string]map[string]*node, len(currentTrees))
		for k, v := range currentTrees {
			// Deep copy method trees map for each version
			methodTreesCopy := make(map[string]*node, len(v))
			maps.Copy(methodTreesCopy, v) // Node pointers are shared (safe - immutable after creation)
			newTrees[k] = methodTreesCopy
		}

		// Step 4: Add the new version/method tree
		if newTrees[ver] == nil {
			newTrees[ver] = make(map[string]*node)
		}

		if newTrees[ver][method] == nil {
			newTrees[ver][method] = &node{}
		}

		// Add route to the newly created tree
		newTrees[ver][method].addRouteWithConstraints(path, handlers, constraints)

		// Step 5: Attempt atomic compare-and-swap
		// IMPORTANT: Allocate on heap to avoid storing pointer to stack variable
		// The map needs to outlive this function call
		heapTrees := new(map[string]map[string]*node)
		*heapTrees = newTrees
		if atomic.CompareAndSwapPointer(&r.versionTrees.trees, versionTreesPtr, unsafe.Pointer(heapTrees)) {
			return // Successfully updated, we won the race
		}
		// CAS failed - another goroutine modified the tree between steps 1 and 5
		// Retry the entire operation with fresh state
		// In practice, this rarely loops more than once or twice
	}
}

// Version creates a version-specific router with optional lifecycle configuration.
// Lifecycle options configure deprecation, sunset dates, and migration documentation.
//
// Example without lifecycle:
//
//	v1 := r.Version("v1")
//	v1.GET("/users", listUsersV1)
//
// Example with lifecycle:
//
//	v1 := r.Version("v1",
//	    version.Deprecated(),
//	    version.Sunset(time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)),
//	    version.MigrationDocs("https://docs.example.com/v1-to-v2"),
//	)
//	v1.GET("/users", listUsersV1)
func (r *Router) Version(ver string, opts ...version.LifecycleOption) *VersionRouter {
	vr := &VersionRouter{
		router:  r,
		version: ver,
	}

	// Apply lifecycle options if any
	if len(opts) > 0 {
		vr.lifecycle = version.ApplyLifecycleOptions(opts...)

		// Register lifecycle with the engine
		if r.versionEngine != nil {
			r.versionEngine.Config().SetLifecycle(ver, vr.lifecycle)
		}
	}

	return vr
}

// Configure applies lifecycle options to an existing version router.
// Use this when you need to configure lifecycle after defining routes.
//
// Example:
//
//	v1 := r.Version("v1")
//	v1.GET("/users", listUsersV1)
//
//	// Later, configure lifecycle
//	v1.Configure(
//	    version.Deprecated(),
//	    version.Sunset(sunsetDate),
//	)
func (vr *VersionRouter) Configure(opts ...version.LifecycleOption) *VersionRouter {
	if len(opts) == 0 {
		return vr
	}

	if vr.lifecycle == nil {
		vr.lifecycle = &version.LifecycleConfig{}
	}

	// Apply options to existing lifecycle config
	for _, opt := range opts {
		opt(vr.lifecycle)
	}

	// Update engine's lifecycle config
	if vr.router.versionEngine != nil {
		vr.router.versionEngine.Config().SetLifecycle(vr.version, vr.lifecycle)
	}

	return vr
}

// Handle adds a route with the specified HTTP method to the version-specific router.
// This is the generic method used by all HTTP method shortcuts.
//
// Example:
//
//	vr.Handle("GET", "/users", getUserHandler)
//	vr.Handle("POST", "/users", createUserHandler)
func (vr *VersionRouter) Handle(method, path string, handlers ...HandlerFunc) *route.Route {
	return vr.addVersionRoute(method, path, handlers)
}

// GET adds a GET route to the version-specific router
func (vr *VersionRouter) GET(path string, handlers ...HandlerFunc) *route.Route {
	return vr.Handle("GET", path, handlers...)
}

// POST adds a POST route to the version-specific router
func (vr *VersionRouter) POST(path string, handlers ...HandlerFunc) *route.Route {
	return vr.Handle("POST", path, handlers...)
}

// PUT adds a PUT route to the version-specific router
func (vr *VersionRouter) PUT(path string, handlers ...HandlerFunc) *route.Route {
	return vr.Handle("PUT", path, handlers...)
}

// DELETE adds a DELETE route to the version-specific router
func (vr *VersionRouter) DELETE(path string, handlers ...HandlerFunc) *route.Route {
	return vr.Handle("DELETE", path, handlers...)
}

// PATCH adds a PATCH route to the version-specific router
func (vr *VersionRouter) PATCH(path string, handlers ...HandlerFunc) *route.Route {
	return vr.Handle("PATCH", path, handlers...)
}

// OPTIONS adds an OPTIONS route to the version-specific router
func (vr *VersionRouter) OPTIONS(path string, handlers ...HandlerFunc) *route.Route {
	return vr.Handle("OPTIONS", path, handlers...)
}

// HEAD adds a HEAD route to the version-specific router
func (vr *VersionRouter) HEAD(path string, handlers ...HandlerFunc) *route.Route {
	return vr.Handle("HEAD", path, handlers...)
}

// addVersionRoute adds a route to the version-specific router using deferred registration.
// The route's version field is set so it will be registered to the correct tree during Warmup().
func (vr *VersionRouter) addVersionRoute(method, path string, handlers []HandlerFunc) *route.Route {
	// Analyze route for introspection
	handlerName := "anonymous"
	if len(handlers) > 0 {
		handlerName = getHandlerName(handlers[len(handlers)-1])
	}

	// Extract middleware names (all handlers except the last one)
	var middlewareNames []string
	if len(handlers) > 1 {
		middlewareNames = make([]string, 0, len(handlers)-1)
		for i := 0; i < len(handlers)-1; i++ {
			middlewareNames = append(middlewareNames, getHandlerName(handlers[i]))
		}
	}

	// Count parameters in path
	paramCount := strings.Count(path, ":")

	// Check if route is static (no parameters)
	isStatic := !strings.Contains(path, ":") && !strings.HasSuffix(path, "*")

	// Store route info for introspection
	vr.router.routeTree.routesMutex.Lock()
	vr.router.routeTree.routes = append(vr.router.routeTree.routes, route.Info{
		Method:      method,
		Path:        path,
		HandlerName: handlerName,
		Middleware:  middlewareNames,
		Constraints: make(map[string]string), // Will be populated when constraints are added
		IsStatic:    isStatic,
		Version:     vr.version, // Version-specific route
		ParamCount:  paramCount,
	})
	vr.router.routeTree.routesMutex.Unlock()

	// Record route registration for metrics
	vr.router.recordRouteRegistration(method, path)

	// Convert handlers to route.Handler
	routeHandlers := make([]route.Handler, len(handlers))
	for i, h := range handlers {
		routeHandlers[i] = h
	}

	// Create route object with version field set for deferred registration
	rt := route.NewRoute(vr.router, vr.version, method, path, routeHandlers)

	// Add to pending routes for deferred registration during Warmup()
	// If warmup has already been called, register immediately
	vr.router.pendingRoutesMu.Lock()
	if vr.router.warmedUp {
		// Warmup already happened, register immediately
		vr.router.pendingRoutesMu.Unlock()
		rt.RegisterRoute()
	} else {
		vr.router.pendingRoutes = append(vr.router.pendingRoutes, rt)
		vr.router.pendingRoutesMu.Unlock()
	}

	return rt
}

// Group creates a version-specific route group
func (vr *VersionRouter) Group(prefix string, middleware ...HandlerFunc) *VersionGroup {
	return &VersionGroup{
		versionRouter: vr,
		prefix:        prefix,
		middleware:    middleware,
	}
}

// VersionGroup represents a group of routes within a specific version
type VersionGroup struct {
	versionRouter *VersionRouter
	prefix        string
	middleware    []HandlerFunc
	namePrefix    string // Name prefix for all routes in this group (e.g., "api.v1.")
}

// SetNamePrefix sets a prefix for all route names in this version group.
// The prefix is appended to any existing name prefix,
// enabling hierarchical route naming.
// Returns the group for method chaining.
//
// Example:
//
//	v1 := r.Version("v1")
//	api := v1.Group("/api").SetNamePrefix("api.")
//	api.GET("/users", handler).SetName("users.list")
//	// Full route name becomes: "api.users.list"
func (vg *VersionGroup) SetNamePrefix(prefix string) *VersionGroup {
	vg.namePrefix = vg.namePrefix + prefix
	return vg
}

// Handle adds a route with the specified HTTP method to the version group.
// This is the generic method used by all HTTP method shortcuts.
func (vg *VersionGroup) Handle(method, path string, handlers ...HandlerFunc) *route.Route {
	fullPath := vg.prefix + path
	// Create a new slice to avoid modifying vg.middleware's backing array
	allHandlers := make([]HandlerFunc, 0, len(vg.middleware)+len(handlers))
	allHandlers = append(allHandlers, vg.middleware...)
	allHandlers = append(allHandlers, handlers...)
	rt := vg.versionRouter.addVersionRoute(method, fullPath, allHandlers)
	// Set version group reference for name prefixing
	rt.SetVersionGroup(vg)

	return rt
}

// GET adds a GET route to the version group
func (vg *VersionGroup) GET(path string, handlers ...HandlerFunc) *route.Route {
	return vg.Handle("GET", path, handlers...)
}

// POST adds a POST route to the version group
func (vg *VersionGroup) POST(path string, handlers ...HandlerFunc) *route.Route {
	return vg.Handle("POST", path, handlers...)
}

// PUT adds a PUT route to the version group
func (vg *VersionGroup) PUT(path string, handlers ...HandlerFunc) *route.Route {
	return vg.Handle("PUT", path, handlers...)
}

// DELETE adds a DELETE route to the version group
func (vg *VersionGroup) DELETE(path string, handlers ...HandlerFunc) *route.Route {
	return vg.Handle("DELETE", path, handlers...)
}

// PATCH adds a PATCH route to the version group
func (vg *VersionGroup) PATCH(path string, handlers ...HandlerFunc) *route.Route {
	return vg.Handle("PATCH", path, handlers...)
}

// OPTIONS adds an OPTIONS route to the version group
func (vg *VersionGroup) OPTIONS(path string, handlers ...HandlerFunc) *route.Route {
	return vg.Handle("OPTIONS", path, handlers...)
}

// HEAD adds a HEAD route to the version group
func (vg *VersionGroup) HEAD(path string, handlers ...HandlerFunc) *route.Route {
	return vg.Handle("HEAD", path, handlers...)
}

// NamePrefix returns the name prefix for this version group.
func (vg *VersionGroup) NamePrefix() string {
	return vg.namePrefix
}
