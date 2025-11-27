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
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"rivaas.dev/router/compiler"
)

// RouteConstraint represents a compiled constraint for route parameters.
// Constraints are compiled for validation during routing.
type RouteConstraint struct {
	Param   string         // Parameter name
	Pattern *regexp.Regexp // Compiled regex pattern
}

// ConstraintKind represents the type of constraint applied to a route parameter.
type ConstraintKind uint8

const (
	ConstraintNone ConstraintKind = iota
	ConstraintInt
	ConstraintFloat
	ConstraintUUID
	ConstraintRegex
	ConstraintEnum
	ConstraintDate     // RFC3339 full-date
	ConstraintDateTime // RFC3339 date-time
)

// ParamConstraint represents a typed constraint for a route parameter.
// This provides semantic constraint types that map directly to OpenAPI schema types.
type ParamConstraint struct {
	Kind    ConstraintKind
	Pattern string         // for ConstraintRegex
	Enum    []string       // for ConstraintEnum
	re      *regexp.Regexp // compiled regex for ConstraintRegex (lazy)
}

// Route represents a registered route with optional constraints.
// This provides a fluent interface for adding constraints and metadata.
//
// Routes use deferred registration - they are collected when created but only
// added to the routing tree during Warmup() or on first request. This allows
// constraints to be added via fluent API without re-registration issues.
type Route struct {
	router           *Router
	version          string // API version (empty = standard tree, non-empty = version-specific tree)
	method           string
	path             string
	handlers         []HandlerFunc
	constraints      []RouteConstraint          // Legacy regex-based constraints
	typedConstraints map[string]ParamConstraint // New typed constraints
	registered       bool                       // Whether route has been registered to a tree
	compiled         bool                       // Whether typed constraints have been compiled

	// Route metadata (immutable after registration)
	name         string         // Human-readable name for reverse routing
	description  string         // Optional description
	tags         []string       // Optional tags for categorization
	template     *routeTemplate // Template for reverse routing
	group        *Group         // Reference to group for name prefixing
	versionGroup *VersionGroup  // Reference to version group for name prefixing

	mu sync.Mutex // Protects route modifications during constraint addition
}

// RouteInfo contains comprehensive information about a registered route for introspection.
// This is used for debugging, documentation generation, API documentation, and monitoring.
//
// Enhanced fields provide deep insights into route configuration:
//   - Middleware: Full middleware chain for this route
//   - Constraints: Parameter validation rules
//   - IsStatic: Whether the route is static
//   - Version: API versioning information
type RouteInfo struct {
	Method      string            // HTTP method (GET, POST, etc.)
	Path        string            // Route path pattern (/users/:id)
	HandlerName string            // Name of the handler function
	Middleware  []string          // Middleware chain names (in execution order)
	Constraints map[string]string // Parameter constraints (param -> regex pattern)
	IsStatic    bool              // True if route has no dynamic parameters
	Version     string            // API version (e.g., "v1", "v2"), empty if not versioned
	ParamCount  int               // Number of URL parameters in this route
}

// atomicRouteTree represents a route tree with thread-safe operations.
// This structure enables concurrent reads and writes.
//
// SAFETY REQUIREMENTS:
//   - Requires 64-bit architecture for pointer operations
//   - Pointer must be properly aligned (guaranteed by Go runtime)
//   - Copy-on-write ensures readers never see partial updates
//
// FIELD ORDER REQUIREMENTS:
//   - `trees` MUST be the first field (offset 0) to guarantee 8-byte alignment
//   - `version` MUST follow immediately after for 8-byte alignment
//   - DO NOT reorder these fields or insert fields between them
//   - Operations on uint64/unsafe.Pointer require 8-byte alignment
//
// Platform Support:
//   - amd64: ✓ Fully supported
//   - arm64: ✓ Fully supported
//   - 386:   ✗ Not supported (32-bit)
//   - arm:   ✗ Not supported (32-bit)
//
// Alignment is verified at runtime in init() - the program will panic if misaligned.
type atomicRouteTree struct {
	// trees is a pointer to the current route tree map
	// This allows thread-safe reads and updates during route registration
	// WARNING: Must only be accessed via atomic operations (Load/Store/CompareAndSwap)
	// CRITICAL: Must be first field for 8-byte alignment (verified in init())
	trees unsafe.Pointer // *map[string]*node

	// version is incremented on each tree update
	// CRITICAL: Must immediately follow trees for 8-byte alignment (verified in init())
	version uint64

	// routes is protected by a separate mutex for introspection (low-frequency access)
	routes      []RouteInfo
	routesMutex sync.RWMutex
}

func init() {
	// Runtime safety check: Verify platform support for atomic pointer operations
	// This ensures the router only runs on supported 64-bit architectures
	if unsafe.Sizeof(unsafe.Pointer(nil)) != 8 {
		panic("router: requires 64-bit architecture for atomic pointer operations (unsafe.Pointer must be 8 bytes)")
	}

	// Verify atomic field alignment at runtime
	// On 64-bit systems, atomic operations on uint64 and unsafe.Pointer require 8-byte alignment.
	// The Go compiler guarantees this for the first field and for fields following 8-byte aligned fields.
	// This check ensures our struct layout remains correct even if refactored.
	var tree atomicRouteTree
	treesOffset := unsafe.Offsetof(tree.trees)
	versionOffset := unsafe.Offsetof(tree.version)

	if treesOffset != 0 {
		panic("router: atomicRouteTree.trees must be first field for proper atomic alignment")
	}
	if versionOffset%8 != 0 {
		panic("router: atomicRouteTree.version is not 8-byte aligned (misaligned atomic operations will panic on some architectures)")
	}

	// Verify atomicVersionTrees alignment
	var vt atomicVersionTrees
	vtTreesOffset := unsafe.Offsetof(vt.trees)
	if vtTreesOffset != 0 {
		panic("router: atomicVersionTrees.trees must be first field for proper atomic alignment")
	}
}

// getTreeForMethodDirect atomically gets the tree for a specific HTTP method without copying.
// This method uses direct pointer access.
func (r *Router) getTreeForMethodDirect(method string) *node {
	treesPtr := atomic.LoadPointer(&r.routeTree.trees)
	trees := (*map[string]*node)(treesPtr)
	return (*trees)[method]
}

// loadTrees atomically loads the route trees map.
// Returns nil if trees haven't been initialized.
func (rt *atomicRouteTree) loadTrees() *map[string]*node {
	treesPtr := atomic.LoadPointer(&rt.trees)
	if treesPtr == nil {
		return nil
	}
	return (*map[string]*node)(treesPtr)
}

// GET adds a route that matches GET requests to the specified path.
// The path can contain parameters using the :param syntax.
// Returns a Route object for adding constraints and metadata.
//
// Example:
//
//	r.GET("/users/:id", getUserHandler)
//	r.GET("/health", healthCheckHandler)
//	r.GET("/users/:id", getUserHandler).Where("id", `\d+`) // With constraint
func (r *Router) GET(path string, handlers ...HandlerFunc) *Route {
	return r.addRouteWithConstraints(http.MethodGet, path, handlers)
}

// POST adds a route that matches POST requests to the specified path.
// Commonly used for creating resources and handling form submissions.
//
// Example:
//
//	r.POST("/users", createUserHandler)
//	r.POST("/login", loginHandler)
func (r *Router) POST(path string, handlers ...HandlerFunc) *Route {
	return r.addRouteWithConstraints(http.MethodPost, path, handlers)
}

// PUT adds a route that matches PUT requests to the specified path.
// Typically used for updating or replacing entire resources.
//
// Example:
//
//	r.PUT("/users/:id", updateUserHandler)
func (r *Router) PUT(path string, handlers ...HandlerFunc) *Route {
	return r.addRouteWithConstraints(http.MethodPut, path, handlers)
}

// DELETE adds a route that matches DELETE requests to the specified path.
// Used for removing resources from the server.
//
// Example:
//
//	r.DELETE("/users/:id", deleteUserHandler)
func (r *Router) DELETE(path string, handlers ...HandlerFunc) *Route {
	return r.addRouteWithConstraints(http.MethodDelete, path, handlers)
}

// PATCH adds a route that matches PATCH requests to the specified path.
// Used for partial updates to existing resources.
//
// Example:
//
//	r.PATCH("/users/:id", patchUserHandler)
func (r *Router) PATCH(path string, handlers ...HandlerFunc) *Route {
	return r.addRouteWithConstraints(http.MethodPatch, path, handlers)
}

// OPTIONS adds a route that matches OPTIONS requests to the specified path.
// Commonly used for CORS preflight requests and API discovery.
//
// Example:
//
//	r.OPTIONS("/api/*", corsHandler)
func (r *Router) OPTIONS(path string, handlers ...HandlerFunc) *Route {
	return r.addRouteWithConstraints(http.MethodOptions, path, handlers)
}

// HEAD adds a route that matches HEAD requests to the specified path.
// HEAD requests are like GET requests but return only headers without the response body.
//
// Example:
//
//	r.HEAD("/users/:id", checkUserExistsHandler)
func (r *Router) HEAD(path string, handlers ...HandlerFunc) *Route {
	return r.addRouteWithConstraints(http.MethodHead, path, handlers)
}

// addRouteWithConstraints adds a route with support for parameter constraints.
// Returns a Route object that can be used to add constraints and metadata.
//
// Routes use deferred registration - they are added to pendingRoutes and only
// registered to the routing tree during Warmup() or on first request. This allows
// the fluent Where* API to work correctly without re-registration issues.
func (r *Router) addRouteWithConstraints(method, path string, handlers []HandlerFunc) *Route {
	// Check if router is frozen
	if r.frozen.Load() {
		panic("cannot register routes after router is frozen (call Freeze() before serving)")
	}

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

	// Runtime warning for routes with excessive parameters (>8)
	if paramCount > 8 {
		r.emit(DiagHighParamCount, "route has more than 8 parameters, using map storage instead of array", map[string]any{
			"method":         method,
			"path":           path,
			"param_count":    paramCount,
			"recommendation": "consider restructuring route to use query parameters or request body for additional data",
		})
	}

	// Check if route is static (no parameters)
	isStatic := !strings.Contains(path, ":") && !strings.HasSuffix(path, "*")

	// Store route info for introspection
	r.routeTree.routesMutex.Lock()
	r.routeTree.routes = append(r.routeTree.routes, RouteInfo{
		Method:      method,
		Path:        path,
		HandlerName: handlerName,
		Middleware:  middlewareNames,
		Constraints: make(map[string]string), // Will be populated when constraints are added
		IsStatic:    isStatic,
		Version:     "", // Standard tree (non-versioned)
		ParamCount:  paramCount,
	})
	r.routeTree.routesMutex.Unlock()

	// Create route object for constraint support (deferred registration)
	route := &Route{
		router:   r,
		version:  "", // Empty = standard tree
		method:   method,
		path:     path,
		handlers: handlers,
	}

	// Record route registration for metrics
	r.recordRouteRegistration(method, path)

	// Add to pending routes for deferred registration during Warmup()
	// If warmup has already been called, register immediately
	r.pendingRoutesMu.Lock()
	if r.warmedUp {
		// Warmup already happened, register immediately
		r.pendingRoutesMu.Unlock()
		route.registerRoute()
	} else {
		r.pendingRoutes = append(r.pendingRoutes, route)
		r.pendingRoutesMu.Unlock()
	}

	return route
}

// Routes returns a list of all registered routes for introspection.
// This is useful for debugging, documentation generation, and monitoring.
// The returned slice is sorted by method and then by path for consistency.
//
// Example:
//
//	routes := r.Routes()
//	for _, route := range routes {
//	    fmt.Printf("%s %s -> %s\n", route.Method, route.Path, route.HandlerName)
//	}
func (r *Router) Routes() []RouteInfo {
	// Create a copy to avoid exposing internal slice
	r.routeTree.routesMutex.RLock()
	routes := make([]RouteInfo, len(r.routeTree.routes))
	copy(routes, r.routeTree.routes)
	r.routeTree.routesMutex.RUnlock()

	// Sort by method, then by path for consistent output
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Method == routes[j].Method {
			return routes[i].Path < routes[j].Path
		}
		return routes[i].Method < routes[j].Method
	})

	return routes
}

// registerRoute adds the route to the appropriate radix tree with its constraints.
// This is called during Warmup() for deferred route registration.
// The route.version field determines which tree to use:
//   - Empty string: standard tree
//   - Non-empty: version-specific tree
//
// This method is thread-safe and uses a mutex to prevent double registration.
func (route *Route) registerRoute() {
	route.mu.Lock()
	defer route.mu.Unlock()

	if route.registered {
		return // Already registered, skip
	}
	route.registered = true

	// Combine global middleware with route handlers
	// IMPORTANT: Create a new slice to avoid aliasing bugs with append
	route.router.middlewareMu.RLock()
	allHandlers := make([]HandlerFunc, 0, len(route.router.middleware)+len(route.handlers))
	allHandlers = append(allHandlers, route.router.middleware...)
	route.router.middlewareMu.RUnlock()
	allHandlers = append(allHandlers, route.handlers...)

	// Convert typed constraints to regex constraints for validation
	allConstraints := route.convertTypedConstraintsToRegex()
	allConstraints = append(allConstraints, route.constraints...)

	// Add route to appropriate tree based on version
	if route.version != "" {
		// Version-specific tree - do NOT add to global route compiler
		// Versioned routes use version-specific trees and caches
		route.router.addVersionRoute(route.version, route.method, route.path, allHandlers, allConstraints)
	} else {
		// Standard tree - update compiler FIRST, then radix tree
		//
		// IMPORTANT: Order matters for constraint validation during re-registration.
		// When Where() is called after initial registration, we must update the
		// compiler before the radix tree to avoid a race condition where:
		// 1. Radix tree is updated with new constraints
		// 2. Compiler still has old route without constraints
		// 3. Request matches old route in compiler, bypassing constraint validation
		//
		// By updating compiler first:
		// - Compiler gets new constraints immediately
		// - During brief window before radix tree update, requests either:
		//   a) Match in compiler with new constraints (correct)
		//   b) Fall through to radix tree with old state (acceptable for brief window)

		// Compile route for matching (if enabled)
		if route.router.useTemplates && route.router.routeCompiler != nil {
			// Convert constraints for compiler
			var compilerConstraints []compiler.RouteConstraint
			if len(allConstraints) > 0 {
				compilerConstraints = make([]compiler.RouteConstraint, len(allConstraints))
				for i, c := range allConstraints {
					compilerConstraints[i] = compiler.RouteConstraint{
						Param:   c.Param,
						Pattern: c.Pattern,
					}
				}
			}

			// Convert HandlerFunc to compiler.HandlerFunc
			compilerHandlers := make([]compiler.HandlerFunc, len(allHandlers))
			for i, h := range allHandlers {
				compilerHandlers[i] = compiler.HandlerFunc(h)
			}
			compiledRoute := compiler.CompileRoute(route.method, route.path, compilerHandlers, compilerConstraints)

			// Cache the converted handlers
			compiledRoute.SetCachedHandlers(unsafe.Pointer(&allHandlers))

			// Remove any existing route then add new one
			// This ensures constraints are enforced before radix tree is updated
			route.router.routeCompiler.RemoveRoute(route.method, route.path)
			route.router.routeCompiler.AddRoute(compiledRoute)
		}

		// Update radix tree (fallback path)
		route.router.addRouteToTree(route.method, route.path, allHandlers, allConstraints)
	}
}

// Where adds a constraint to a route parameter using a regular expression.
// The constraint is pre-compiled for validation during routing.
// This method provides a fluent interface for building routes with validation.
//
// IMPORTANT: This method panics if the regex pattern is invalid. This is intentional
// for validation during application startup. Ensure patterns are tested.
//
// Common patterns:
//   - Numeric: `\d+` (one or more digits)
//   - Alpha: `[a-zA-Z]+` (letters only)
//   - UUID: `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`
//
// Example:
//
//	r.GET("/users/:id", getUserHandler).Where("id", `\d+`)
//	r.GET("/files/:filename", getFileHandler).Where("filename", `[a-zA-Z0-9.-]+`)
//
// The panic on invalid regex is by design for early error detection during development.
func (route *Route) Where(param, pattern string) *Route {
	// Pre-compile the regex pattern (panics on invalid pattern)
	regex, err := regexp.Compile("^" + pattern + "$")
	if err != nil {
		panic(fmt.Sprintf("Invalid regex pattern for parameter '%s': %v", param, err))
	}

	// Lock route for modifications
	route.mu.Lock()
	route.constraints = append(route.constraints, RouteConstraint{
		Param:   param,
		Pattern: regex,
	})
	wasRegistered := route.registered
	route.registered = false // Allow re-registration with new constraints
	route.mu.Unlock()

	// Update RouteInfo with constraint for introspection
	route.router.routeTree.routesMutex.Lock()
	for i := range route.router.routeTree.routes {
		info := &route.router.routeTree.routes[i]
		if info.Method == route.method && info.Path == route.path && info.Version == route.version {
			if info.Constraints == nil {
				info.Constraints = make(map[string]string)
			}
			info.Constraints[param] = pattern
			break
		}
	}
	route.router.routeTree.routesMutex.Unlock()

	// If route was already registered (after warmup), re-register with new constraints
	if wasRegistered {
		route.registerRoute()
	}

	return route
}

// WhereUUID adds a typed constraint that ensures the parameter is a valid UUID.
// This maps to OpenAPI schema type "string" with format "uuid".
//
// Example:
//
//	r.GET("/entities/:uuid", handler).WhereUUID("uuid")
func (r *Route) WhereUUID(name string) *Route {
	r.mu.Lock()
	r.ensureTypedConstraints()
	r.typedConstraints[name] = ParamConstraint{Kind: ConstraintUUID}
	wasRegistered := r.registered
	r.registered = false
	r.mu.Unlock()

	if wasRegistered {
		r.registerRoute()
	}
	return r
}

// ensureTypedConstraints initializes the typed constraints map if needed.
func (r *Route) ensureTypedConstraints() {
	if r.typedConstraints == nil {
		r.typedConstraints = make(map[string]ParamConstraint)
	}
}

// WhereInt adds a typed constraint that ensures the parameter is an integer.
// This maps to OpenAPI schema type "integer" with format "int64".
//
// Example:
//
//	r.GET("/users/:id", handler).WhereInt("id")
func (r *Route) WhereInt(name string) *Route {
	r.mu.Lock()
	r.ensureTypedConstraints()
	r.typedConstraints[name] = ParamConstraint{Kind: ConstraintInt}
	wasRegistered := r.registered
	r.registered = false
	r.mu.Unlock()

	if wasRegistered {
		r.registerRoute()
	}
	return r
}

// WhereFloat adds a typed constraint that ensures the parameter is a floating-point number.
// This maps to OpenAPI schema type "number" with format "double".
//
// Example:
//
//	r.GET("/prices/:amount", handler).WhereFloat("amount")
func (r *Route) WhereFloat(name string) *Route {
	r.mu.Lock()
	r.ensureTypedConstraints()
	r.typedConstraints[name] = ParamConstraint{Kind: ConstraintFloat}
	wasRegistered := r.registered
	r.registered = false
	r.mu.Unlock()

	if wasRegistered {
		r.registerRoute()
	}
	return r
}

// WhereRegex adds a typed constraint with a custom regex pattern.
// This maps to OpenAPI schema type "string" with a pattern.
//
// Example:
//
//	r.GET("/files/:name", handler).WhereRegex("name", `[a-zA-Z0-9._-]+`)
func (r *Route) WhereRegex(name, pattern string) *Route {
	r.mu.Lock()
	r.ensureTypedConstraints()
	r.typedConstraints[name] = ParamConstraint{Kind: ConstraintRegex, Pattern: pattern}
	wasRegistered := r.registered
	r.registered = false
	r.mu.Unlock()

	if wasRegistered {
		r.registerRoute()
	}
	return r
}

// WhereEnum adds a typed constraint that ensures the parameter matches one of the provided values.
// This maps to OpenAPI schema type "string" with an enum.
//
// Example:
//
//	r.GET("/status/:state", handler).WhereEnum("state", "active", "pending", "deleted")
func (r *Route) WhereEnum(name string, values ...string) *Route {
	r.mu.Lock()
	r.ensureTypedConstraints()
	r.typedConstraints[name] = ParamConstraint{
		Kind: ConstraintEnum,
		Enum: append([]string(nil), values...),
	}
	wasRegistered := r.registered
	r.registered = false
	r.mu.Unlock()

	if wasRegistered {
		r.registerRoute()
	}
	return r
}

// WhereDate adds a typed constraint that ensures the parameter is an RFC3339 full-date.
// This maps to OpenAPI schema type "string" with format "date".
//
// Example:
//
//	r.GET("/orders/:date", handler).WhereDate("date")
func (r *Route) WhereDate(name string) *Route {
	r.mu.Lock()
	r.ensureTypedConstraints()
	r.typedConstraints[name] = ParamConstraint{Kind: ConstraintDate}
	wasRegistered := r.registered
	r.registered = false
	r.mu.Unlock()

	if wasRegistered {
		r.registerRoute()
	}
	return r
}

// WhereDateTime adds a typed constraint that ensures the parameter is an RFC3339 date-time.
// This maps to OpenAPI schema type "string" with format "date-time".
//
// Example:
//
//	r.GET("/events/:timestamp", handler).WhereDateTime("timestamp")
func (r *Route) WhereDateTime(name string) *Route {
	r.mu.Lock()
	r.ensureTypedConstraints()
	r.typedConstraints[name] = ParamConstraint{Kind: ConstraintDateTime}
	wasRegistered := r.registered
	r.registered = false
	r.mu.Unlock()

	if wasRegistered {
		r.registerRoute()
	}
	return r
}

// TypedConstraints returns a copy of the typed constraints map.
func (r *Route) TypedConstraints() map[string]ParamConstraint {
	if len(r.typedConstraints) == 0 {
		return nil
	}
	out := make(map[string]ParamConstraint, len(r.typedConstraints))
	maps.Copy(out, r.typedConstraints)
	return out
}

// compile compiles regex patterns in typed constraints (lazy compilation).
func (r *Route) compile() {
	if r.compiled {
		return
	}
	for k, pc := range r.typedConstraints {
		if pc.Kind == ConstraintRegex && pc.Pattern != "" && pc.re == nil {
			if rx, err := regexp.Compile("^" + pc.Pattern + "$"); err == nil {
				pc.re = rx
				r.typedConstraints[k] = pc
			}
		}
	}
	r.compiled = true
}

// convertTypedConstraintsToRegex converts typed constraints to regex-based RouteConstraint
// for use with the existing validation system. This allows typed constraints to work
// with the current router architecture while preserving semantic information for OpenAPI.
func (r *Route) convertTypedConstraintsToRegex() []RouteConstraint {
	if len(r.typedConstraints) == 0 {
		return nil
	}

	r.compile()

	var regexConstraints []RouteConstraint
	for name, pc := range r.typedConstraints {
		var pattern string
		switch pc.Kind {
		case ConstraintInt:
			pattern = `\d+`
		case ConstraintFloat:
			pattern = `-?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?`
		case ConstraintUUID:
			pattern = `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}`
		case ConstraintRegex:
			pattern = pc.Pattern
		case ConstraintEnum:
			// Convert enum to regex: (value1|value2|value3)
			escaped := make([]string, len(pc.Enum))
			for i, v := range pc.Enum {
				// Escape special regex characters in enum values
				escaped[i] = regexp.QuoteMeta(v)
			}
			pattern = "(" + strings.Join(escaped, "|") + ")"
		case ConstraintDate:
			pattern = `\d{4}-\d{2}-\d{2}`
		case ConstraintDateTime:
			pattern = `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})`
		default:
			continue // Skip unknown constraint types
		}

		// Compile regex pattern
		regex, err := regexp.Compile("^" + pattern + "$")
		if err != nil {
			// Should not happen for our predefined patterns, but handle gracefully
			continue
		}

		regexConstraints = append(regexConstraints, RouteConstraint{
			Param:   name,
			Pattern: regex,
		})
	}

	return regexConstraints
}

// routeTemplate represents a compiled route pattern for reverse routing.
// It stores the positions of parameters to avoid string replacements.
type routeTemplate struct {
	segments []routeSegment
}

type routeSegment struct {
	static bool   // true if static text, false if parameter
	value  string // static text or parameter name
}

// SetName assigns a human-readable name to the route for reverse routing and introspection.
// Names must be globally unique. Panics if the router is frozen or if the name is already taken.
// Returns the route for method chaining.
//
// Example:
//
//	r.GET("/users/:id", getUserHandler).SetName("users.get")
//	r.POST("/users", createUserHandler).SetName("users.create")
func (route *Route) SetName(name string) *Route {
	if route.router.frozen.Load() {
		panic("cannot name routes after router is frozen")
	}

	// Auto-prefix with group name if in a group
	if route.group != nil && route.group.namePrefix != "" {
		name = route.group.namePrefix + name
	} else if route.versionGroup != nil && route.versionGroup.namePrefix != "" {
		name = route.versionGroup.namePrefix + name
	}

	// Check for duplicate names
	route.router.routeTree.routesMutex.Lock()
	if existing, ok := route.router.namedRoutes[name]; ok {
		route.router.routeTree.routesMutex.Unlock()
		panic(fmt.Sprintf("duplicate route name: %s (existing: %s %s, new: %s %s)",
			name, existing.method, existing.path, route.method, route.path))
	}
	route.name = name
	route.router.namedRoutes[name] = route
	route.router.routeTree.routesMutex.Unlock()

	return route
}

// SetDescription sets an optional description for the route.
// Useful for API documentation generation.
// Returns the route for method chaining.
//
// Example:
//
//	r.GET("/users/:id", getUserHandler).
//	    SetName("users.get").
//	    SetDescription("Retrieve a user by ID")
func (route *Route) SetDescription(desc string) *Route {
	route.description = desc
	return route
}

// SetTags adds categorization tags to the route.
// Useful for grouping routes in documentation and filtering.
// Returns the route for method chaining.
//
// Example:
//
//	r.GET("/users/:id", getUserHandler).
//	    SetName("users.get").
//	    SetTags("users", "public", "read")
func (route *Route) SetTags(tags ...string) *Route {
	route.tags = append(route.tags, tags...)
	return route
}

// Method returns the HTTP method for this route.
func (route *Route) Method() string {
	return route.method
}

// Path returns the route path pattern.
func (route *Route) Path() string {
	return route.path
}

// Name returns the route name (empty if not named).
// This follows Go naming conventions: getters don't use a Get prefix.
func (route *Route) Name() string {
	return route.name
}

// Description returns the route description (empty if not set).
func (route *Route) Description() string {
	return route.description
}

// Tags returns the route tags.
func (route *Route) Tags() []string {
	return route.tags
}

// getHandlerName extracts the function name from a HandlerFunc using reflection.
// This is used for route introspection and has no impact on routing.
func getHandlerName(handler HandlerFunc) string {
	if handler == nil {
		return "nil"
	}

	funcPtr := runtime.FuncForPC(reflect.ValueOf(handler).Pointer())
	if funcPtr == nil {
		return "unknown"
	}

	fullName := funcPtr.Name()
	file, line := funcPtr.FileLine(funcPtr.Entry())

	// Extract meaningful name: package.function instead of full path
	// Example: "github.com/user/project/main.getUserHandler" -> "main.getUserHandler"
	//          "github.com/user/project/main.main.func1" -> "main.main.func1"
	lastSlash := strings.LastIndex(fullName, "/")
	if lastSlash >= 0 {
		fullName = fullName[lastSlash+1:]
	}

	// Extract just filename from full path
	fileName := filepath.Base(file)

	// Check if this is an anonymous function (pattern: *.func[number])
	// Example: "main.main.func1" -> "anonymous#1"
	if idx := strings.Index(fullName, ".func"); idx >= 0 {
		// Extract the function number (func1 -> #1, func2 -> #2)
		funcNum := fullName[idx+5:] // Skip ".func" to get the number
		return fmt.Sprintf("anonymous#%s (%s:%d)", funcNum, fileName, line)
	}

	// Add file location for better debugging
	return fmt.Sprintf("%s (%s:%d)", fullName, fileName, line)
}

// parseRouteTemplate parses a route path into segments for reverse routing.
// Example: "/users/:id/posts/:postId" -> [{static:"users"}, {param:"id"}, {static:"posts"}, {param:"postId"}]
func parseRouteTemplate(path string) *routeTemplate {
	segments := make([]routeSegment, 0)
	trimmed := strings.Trim(path, "/")

	for part := range strings.SplitSeq(trimmed, "/") {
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, ":") {
			// Parameter
			segments = append(segments, routeSegment{
				static: false,
				value:  part[1:], // Remove ":"
			})
		} else {
			// Static text
			segments = append(segments, routeSegment{
				static: true,
				value:  part,
			})
		}
	}

	return &routeTemplate{segments: segments}
}

// Frozen returns true if the router has been frozen (routes are immutable).
func (r *Router) Frozen() bool {
	return r.frozen.Load()
}

// Freeze freezes the router, making all routes immutable.
// After freezing, no new routes can be registered and route names cannot be changed.
// This enables route introspection and precompiles route templates.
func (r *Router) Freeze() {
	if r.frozen.CompareAndSwap(false, true) {
		// Compile all route templates
		r.routeTree.routesMutex.Lock()
		for _, route := range r.namedRoutes {
			if route.template == nil {
				route.template = parseRouteTemplate(route.path)
			}
		}

		// Build immutable snapshot (pointers to frozen routes)
		routes := make([]*Route, 0, len(r.namedRoutes))
		for _, route := range r.namedRoutes {
			routes = append(routes, route)
		}

		r.routeSnapshotMutex.Lock()
		r.routeSnapshot = routes
		r.routeSnapshotMutex.Unlock()

		r.routeTree.routesMutex.Unlock()

		// Compile routes
		r.CompileAllRoutes()
	}
}

// GetRoute retrieves a route by name. Returns the route pointer and true if found, or nil and false.
// The returned route is frozen and safe to read concurrently.
// Panics if the router is not frozen.
//
// Example:
//
//	route, ok := r.GetRoute("users.get")
//	if ok {
//	    fmt.Printf("Route: %s %s\n", route.Method(), route.Path())
//	}
func (r *Router) GetRoute(name string) (*Route, bool) {
	if !r.frozen.Load() {
		panic("routes not frozen yet; call Freeze() before accessing routes")
	}

	r.routeTree.routesMutex.RLock()
	route, ok := r.namedRoutes[name]
	r.routeTree.routesMutex.RUnlock()

	if !ok {
		return nil, false
	}

	return route, true
}

// GetRoutes returns an immutable snapshot of all named routes.
// The returned routes are frozen and safe to read concurrently.
// Panics if the router is not frozen.
//
// Example:
//
//	routes := r.GetRoutes()
//	for _, route := range routes {
//	    fmt.Printf("%s: %s %s\n", route.Name(), route.Method(), route.Path())
//	}
func (r *Router) GetRoutes() []*Route {
	if !r.frozen.Load() {
		panic("routes not frozen yet; call Freeze() before accessing routes")
	}

	r.routeSnapshotMutex.RLock()
	defer r.routeSnapshotMutex.RUnlock()

	// Return a copy of the snapshot slice (pointers are safe to share)
	result := make([]*Route, len(r.routeSnapshot))
	copy(result, r.routeSnapshot)
	return result
}

// URLFor generates a URL from a route name and parameters.
// Returns an error if the route is not found or if required parameters are missing.
//
// Example:
//
//	url, err := router.URLFor("users.get", map[string]string{"id": "123"}, nil)
//	// Returns: "/users/123", nil
//
//	url, err := router.URLFor("users.get", map[string]string{"id": "123"},
//	    url.Values{"include": []string{"posts"}})
//	// Returns: "/users/123?include=posts", nil
func (r *Router) URLFor(routeName string, params map[string]string, query url.Values) (string, error) {
	if !r.frozen.Load() {
		return "", ErrRoutesNotFrozen
	}

	r.routeTree.routesMutex.RLock()
	route, ok := r.namedRoutes[routeName]
	r.routeTree.routesMutex.RUnlock()

	if !ok {
		return "", fmt.Errorf("%w: %s", ErrRouteNotFound, routeName)
	}

	// Compile template if not already compiled
	if route.template == nil {
		route.template = parseRouteTemplate(route.path)
	}

	// Build URL using template
	var buf strings.Builder
	buf.WriteByte('/')

	for i, seg := range route.template.segments {
		if i > 0 {
			buf.WriteByte('/')
		}

		if seg.static {
			buf.WriteString(seg.value)
		} else {
			val, ok := params[seg.value]
			if !ok {
				return "", fmt.Errorf("%w: %s", ErrMissingRouteParameter, seg.value)
			}
			buf.WriteString(url.PathEscape(val))
		}
	}

	// Add query string if provided
	if len(query) > 0 {
		buf.WriteByte('?')
		buf.WriteString(query.Encode())
	}

	return buf.String(), nil
}

// MustURLFor generates a URL from a route name and parameters, panicking on error.
// Use this when you're certain the route exists and all parameters are provided.
//
// Example:
//
//	url := router.MustURLFor("users.get", map[string]string{"id": "123"}, nil)
//	// Returns: "/users/123"
func (r *Router) MustURLFor(routeName string, params map[string]string, query url.Values) string {
	url, err := r.URLFor(routeName, params, query)
	if err != nil {
		panic(fmt.Sprintf("MustURLFor failed: %v", err))
	}
	return url
}
