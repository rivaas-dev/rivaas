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

// addVersionRoute adds a route to a specific version tree
func (r *Router) addVersionRoute(version, method, path string, handlers []HandlerFunc, constraints []RouteConstraint) {
	// Get or create version trees
	versionTreesPtr := atomic.LoadPointer(&r.versionTrees.trees)
	var versionTrees map[string]map[string]*node

	if versionTreesPtr == nil {
		versionTrees = make(map[string]map[string]*node)
	} else {
		versionTrees = *(*map[string]map[string]*node)(versionTreesPtr)
	}

	// Get or create method trees for this version
	if versionTrees[version] == nil {
		versionTrees[version] = make(map[string]*node)
	}

	if versionTrees[version][method] == nil {
		versionTrees[version][method] = &node{}
	}

	// Add route to the version-specific tree
	versionTrees[version][method].addRouteWithConstraints(path, handlers, constraints)

	// Atomically update the version trees
	atomic.StorePointer(&r.versionTrees.trees, unsafe.Pointer(&versionTrees))
}

// Version creates a version-specific router
func (r *Router) Version(version string) *VersionRouter {
	return &VersionRouter{
		router:  r,
		version: version,
	}
}

// GET adds a GET route to the version-specific router
func (vr *VersionRouter) GET(path string, handlers ...HandlerFunc) *Route {
	return vr.addVersionRoute("GET", path, handlers)
}

// POST adds a POST route to the version-specific router
func (vr *VersionRouter) POST(path string, handlers ...HandlerFunc) *Route {
	return vr.addVersionRoute("POST", path, handlers)
}

// PUT adds a PUT route to the version-specific router
func (vr *VersionRouter) PUT(path string, handlers ...HandlerFunc) *Route {
	return vr.addVersionRoute("PUT", path, handlers)
}

// DELETE adds a DELETE route to the version-specific router
func (vr *VersionRouter) DELETE(path string, handlers ...HandlerFunc) *Route {
	return vr.addVersionRoute("DELETE", path, handlers)
}

// PATCH adds a PATCH route to the version-specific router
func (vr *VersionRouter) PATCH(path string, handlers ...HandlerFunc) *Route {
	return vr.addVersionRoute("PATCH", path, handlers)
}

// OPTIONS adds an OPTIONS route to the version-specific router
func (vr *VersionRouter) OPTIONS(path string, handlers ...HandlerFunc) *Route {
	return vr.addVersionRoute("OPTIONS", path, handlers)
}

// HEAD adds a HEAD route to the version-specific router
func (vr *VersionRouter) HEAD(path string, handlers ...HandlerFunc) *Route {
	return vr.addVersionRoute("HEAD", path, handlers)
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

// GET adds a GET route to the version group
func (vg *VersionGroup) GET(path string, handlers ...HandlerFunc) *Route {
	fullPath := vg.prefix + path
	allHandlers := append(vg.middleware, handlers...)
	return vg.versionRouter.addVersionRoute("GET", fullPath, allHandlers)
}

// POST adds a POST route to the version group
func (vg *VersionGroup) POST(path string, handlers ...HandlerFunc) *Route {
	fullPath := vg.prefix + path
	allHandlers := append(vg.middleware, handlers...)
	return vg.versionRouter.addVersionRoute("POST", fullPath, allHandlers)
}

// PUT adds a PUT route to the version group
func (vg *VersionGroup) PUT(path string, handlers ...HandlerFunc) *Route {
	fullPath := vg.prefix + path
	allHandlers := append(vg.middleware, handlers...)
	return vg.versionRouter.addVersionRoute("PUT", fullPath, allHandlers)
}

// DELETE adds a DELETE route to the version group
func (vg *VersionGroup) DELETE(path string, handlers ...HandlerFunc) *Route {
	fullPath := vg.prefix + path
	allHandlers := append(vg.middleware, handlers...)
	return vg.versionRouter.addVersionRoute("DELETE", fullPath, allHandlers)
}

// PATCH adds a PATCH route to the version group
func (vg *VersionGroup) PATCH(path string, handlers ...HandlerFunc) *Route {
	fullPath := vg.prefix + path
	allHandlers := append(vg.middleware, handlers...)
	return vg.versionRouter.addVersionRoute("PATCH", fullPath, allHandlers)
}

// OPTIONS adds an OPTIONS route to the version group
func (vg *VersionGroup) OPTIONS(path string, handlers ...HandlerFunc) *Route {
	fullPath := vg.prefix + path
	allHandlers := append(vg.middleware, handlers...)
	return vg.versionRouter.addVersionRoute("OPTIONS", fullPath, allHandlers)
}

// HEAD adds a HEAD route to the version group
func (vg *VersionGroup) HEAD(path string, handlers ...HandlerFunc) *Route {
	fullPath := vg.prefix + path
	allHandlers := append(vg.middleware, handlers...)
	return vg.versionRouter.addVersionRoute("HEAD", fullPath, allHandlers)
}
