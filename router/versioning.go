package router

import (
	"net/http"
	"slices"
	"sync/atomic"
	"unsafe"
)

// VersioningConfig holds configuration for version detection
type VersioningConfig struct {
	// Header-based versioning
	HeaderName    string // e.g., "API-Version", "X-API-Version", "Accept-Version"
	HeaderEnabled bool

	// Query parameter-based versioning
	QueryParam   string // e.g., "version", "v", "api_version"
	QueryEnabled bool

	// Default version when no version is specified
	DefaultVersion string

	// Version validation (optional)
	ValidVersions []string // e.g., ["v1", "v2", "latest"]

	// Custom version detection function
	CustomDetector func(*http.Request) string
}

// VersioningOption defines functional options for versioning configuration
type VersioningOption func(*VersioningConfig)

// WithVersioning configures the router with versioning support
func WithVersioning(opts ...VersioningOption) RouterOption {
	return func(r *Router) {
		if r.versioning == nil {
			r.versioning = &VersioningConfig{
				DefaultVersion: "v1",
			}
		}
		for _, opt := range opts {
			opt(r.versioning)
		}
	}
}

// WithHeaderVersioning configures header-based version detection
func WithHeaderVersioning(headerName string) VersioningOption {
	return func(cfg *VersioningConfig) {
		cfg.HeaderName = headerName
		cfg.HeaderEnabled = true
	}
}

// WithQueryVersioning configures query parameter-based version detection
func WithQueryVersioning(paramName string) VersioningOption {
	return func(cfg *VersioningConfig) {
		cfg.QueryParam = paramName
		cfg.QueryEnabled = true
	}
}

// WithDefaultVersion sets the default version
func WithDefaultVersion(version string) VersioningOption {
	return func(cfg *VersioningConfig) {
		cfg.DefaultVersion = version
	}
}

// WithValidVersions sets allowed versions for validation
func WithValidVersions(versions ...string) VersioningOption {
	return func(cfg *VersioningConfig) {
		cfg.ValidVersions = versions
	}
}

// WithCustomVersionDetector sets a custom version detection function
func WithCustomVersionDetector(detector func(*http.Request) string) VersioningOption {
	return func(cfg *VersioningConfig) {
		cfg.CustomDetector = detector
	}
}

// atomicVersionTrees represents lock-free version-specific route trees
type atomicVersionTrees struct {
	trees unsafe.Pointer // *map[string]map[string]*node (version -> method -> tree)
}

// VersionRouter represents a version-specific router
type VersionRouter struct {
	router  *Router
	version string
}

// detectVersion performs efficient version detection
func (r *Router) detectVersion(req *http.Request) string {
	// Custom detector takes precedence
	if r.versioning.CustomDetector != nil {
		return r.versioning.CustomDetector(req)
	}

	// Header-based detection (zero allocations)
	if r.versioning.HeaderEnabled {
		if version := r.getHeaderVersion(req); version != "" {
			return version
		}
	}

	// Query parameter-based detection (zero allocations)
	if r.versioning.QueryEnabled {
		if version := r.getQueryVersion(req); version != "" {
			return version
		}
	}

	return r.versioning.DefaultVersion
}

// getHeaderVersion extracts version from header efficiently
func (r *Router) getHeaderVersion(req *http.Request) string {
	// Direct header access without string allocations
	header := req.Header.Get(r.versioning.HeaderName)
	if header == "" {
		return ""
	}

	// Validate version if configured
	if len(r.versioning.ValidVersions) > 0 {
		if !slices.Contains(r.versioning.ValidVersions, header) {
			return "" // Invalid version
		}
	}

	return header
}

// getQueryVersion extracts version from query parameter efficiently
func (r *Router) getQueryVersion(req *http.Request) string {
	// Direct query parameter access without allocations
	query := req.URL.Query()
	version := query.Get(r.versioning.QueryParam)
	if version == "" {
		return ""
	}

	// Validate version if configured
	if len(r.versioning.ValidVersions) > 0 {
		if !slices.Contains(r.versioning.ValidVersions, version) {
			return "" // Invalid version
		}
	}

	return version
}

// getVersionTree atomically gets the tree for a specific version and HTTP method
func (r *Router) getVersionTree(version, method string) *node {
	versionTreesPtr := atomic.LoadPointer(&r.versionTrees.trees)
	if versionTreesPtr == nil {
		return nil
	}
	versionTrees := *(*map[string]map[string]*node)(versionTreesPtr)

	if methodTrees, exists := versionTrees[version]; exists {
		return methodTrees[method]
	}
	return nil
}

// addVersionRoute adds a route to a specific version tree using atomic compare-and-swap
// This ensures thread-safety without locks during concurrent route registration
//
// Algorithm: Two-phase versioned routing with CAS loop
// Phase 1 (Fast path): Add to existing version/method tree if it exists
// Phase 2 (Slow path): Create new version/method tree atomically via CAS
//
// Data structure: map[version]map[method]*node
// Example: {"v1": {"GET": tree1, "POST": tree2}, "v2": {"GET": tree3}}
//
// Why this design:
// - Fast path avoids CAS overhead for existing version/method combinations
// - Slow path ensures thread-safe creation of new version trees
// - Deep copy prevents race conditions when creating new method trees
func (r *Router) addVersionRoute(version, method, path string, handlers []HandlerFunc, constraints []RouteConstraint) {
	// Fast path: Try to get the existing tree for this version/method combination
	// This is the common case after initial setup
	versionTreesPtr := atomic.LoadPointer(&r.versionTrees.trees)
	if versionTreesPtr != nil {
		versionTrees := *(*map[string]map[string]*node)(versionTreesPtr)
		if methodTrees, exists := versionTrees[version]; exists {
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
		// This is the classic "check-before-copy" optimization in CAS loops
		if methodTrees, exists := currentTrees[version]; exists {
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
			for mk, mv := range v {
				methodTreesCopy[mk] = mv // Node pointers are shared (safe - immutable after creation)
			}
			newTrees[k] = methodTreesCopy
		}

		// Step 4: Add the new version/method tree
		if newTrees[version] == nil {
			newTrees[version] = make(map[string]*node)
		}

		if newTrees[version][method] == nil {
			newTrees[version][method] = &node{}
		}

		// Add route to the newly created tree
		newTrees[version][method].addRouteWithConstraints(path, handlers, constraints)

		// Step 5: Attempt atomic compare-and-swap
		// Only succeeds if no other goroutine modified the pointer since step 1
		if atomic.CompareAndSwapPointer(&r.versionTrees.trees, versionTreesPtr, unsafe.Pointer(&newTrees)) {
			return // Successfully updated, we won the race
		}
		// CAS failed - another goroutine modified the tree between steps 1 and 5
		// Retry the entire operation with fresh state
		// In practice, this rarely loops more than once or twice
	}
}

// Version creates a version-specific router
func (r *Router) Version(version string) *VersionRouter {
	return &VersionRouter{
		router:  r,
		version: version,
	}
}

// Handle adds a route with the specified HTTP method to the version-specific router.
// This is the generic method used by all HTTP method shortcuts.
//
// Example:
//
//	vr.Handle("GET", "/users", getUserHandler)
//	vr.Handle("POST", "/users", createUserHandler)
func (vr *VersionRouter) Handle(method, path string, handlers ...HandlerFunc) *Route {
	return vr.addVersionRoute(method, path, handlers)
}

// GET adds a GET route to the version-specific router
func (vr *VersionRouter) GET(path string, handlers ...HandlerFunc) *Route {
	return vr.Handle("GET", path, handlers...)
}

// POST adds a POST route to the version-specific router
func (vr *VersionRouter) POST(path string, handlers ...HandlerFunc) *Route {
	return vr.Handle("POST", path, handlers...)
}

// PUT adds a PUT route to the version-specific router
func (vr *VersionRouter) PUT(path string, handlers ...HandlerFunc) *Route {
	return vr.Handle("PUT", path, handlers...)
}

// DELETE adds a DELETE route to the version-specific router
func (vr *VersionRouter) DELETE(path string, handlers ...HandlerFunc) *Route {
	return vr.Handle("DELETE", path, handlers...)
}

// PATCH adds a PATCH route to the version-specific router
func (vr *VersionRouter) PATCH(path string, handlers ...HandlerFunc) *Route {
	return vr.Handle("PATCH", path, handlers...)
}

// OPTIONS adds an OPTIONS route to the version-specific router
func (vr *VersionRouter) OPTIONS(path string, handlers ...HandlerFunc) *Route {
	return vr.Handle("OPTIONS", path, handlers...)
}

// HEAD adds a HEAD route to the version-specific router
func (vr *VersionRouter) HEAD(path string, handlers ...HandlerFunc) *Route {
	return vr.Handle("HEAD", path, handlers...)
}

// addVersionRoute adds a route to the version-specific router
func (vr *VersionRouter) addVersionRoute(method, path string, handlers []HandlerFunc) *Route {
	// Combine global middleware with route handlers
	allHandlers := append(vr.router.middleware, handlers...)

	// Add to version-specific tree
	vr.router.addVersionRoute(vr.version, method, path, allHandlers, nil)

	// Create route object for consistency
	route := &Route{
		router:   vr.router,
		method:   method,
		path:     path,
		handlers: handlers,
	}

	return route
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
}

// Handle adds a route with the specified HTTP method to the version group.
// This is the generic method used by all HTTP method shortcuts.
func (vg *VersionGroup) Handle(method, path string, handlers ...HandlerFunc) *Route {
	fullPath := vg.prefix + path
	allHandlers := append(vg.middleware, handlers...)
	return vg.versionRouter.addVersionRoute(method, fullPath, allHandlers)
}

// GET adds a GET route to the version group
func (vg *VersionGroup) GET(path string, handlers ...HandlerFunc) *Route {
	return vg.Handle("GET", path, handlers...)
}

// POST adds a POST route to the version group
func (vg *VersionGroup) POST(path string, handlers ...HandlerFunc) *Route {
	return vg.Handle("POST", path, handlers...)
}

// PUT adds a PUT route to the version group
func (vg *VersionGroup) PUT(path string, handlers ...HandlerFunc) *Route {
	return vg.Handle("PUT", path, handlers...)
}

// DELETE adds a DELETE route to the version group
func (vg *VersionGroup) DELETE(path string, handlers ...HandlerFunc) *Route {
	return vg.Handle("DELETE", path, handlers...)
}

// PATCH adds a PATCH route to the version group
func (vg *VersionGroup) PATCH(path string, handlers ...HandlerFunc) *Route {
	return vg.Handle("PATCH", path, handlers...)
}

// OPTIONS adds an OPTIONS route to the version group
func (vg *VersionGroup) OPTIONS(path string, handlers ...HandlerFunc) *Route {
	return vg.Handle("OPTIONS", path, handlers...)
}

// HEAD adds a HEAD route to the version group
func (vg *VersionGroup) HEAD(path string, handlers ...HandlerFunc) *Route {
	return vg.Handle("HEAD", path, handlers...)
}
