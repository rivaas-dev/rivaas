package router

import (
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

// RouteConstraint represents a compiled constraint for route parameters.
// Constraints are pre-compiled for efficient validation during routing.
type RouteConstraint struct {
	Param   string         // Parameter name
	Pattern *regexp.Regexp // Pre-compiled regex pattern
}

// Route represents a registered route with optional constraints.
// This provides a fluent interface for adding constraints and metadata.
type Route struct {
	router      *Router
	method      string
	path        string
	handlers    []HandlerFunc
	constraints []RouteConstraint
	finalized   bool // Prevents duplicate route registration
}

// RouteInfo contains information about a registered route for introspection.
// This is used for debugging, documentation generation, and monitoring.
type RouteInfo struct {
	Method      string // HTTP method (GET, POST, etc.)
	Path        string // Route path pattern (/users/:id)
	HandlerName string // Name of the handler function
}

// atomicRouteTree represents a lock-free route tree with atomic operations.
// This structure enables concurrent reads and writes without mutex contention.
type atomicRouteTree struct {
	// trees is an atomic pointer to the current route tree map
	// This allows lock-free reads and atomic updates during route registration
	trees unsafe.Pointer // *map[string]*node

	// version is incremented on each tree update for optimistic concurrency control
	version uint64

	// routes is protected by a separate mutex for introspection (low-frequency access)
	routes      []RouteInfo
	routesMutex sync.RWMutex
}

// getTreeForMethodDirect atomically gets the tree for a specific HTTP method without copying.
// This method uses direct pointer access to avoid allocations.
func (r *Router) getTreeForMethodDirect(method string) *node {
	treesPtr := atomic.LoadPointer(&r.routeTree.trees)
	trees := (*map[string]*node)(treesPtr)
	return (*trees)[method]
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
	return r.addRouteWithConstraints("GET", path, handlers)
}

// POST adds a route that matches POST requests to the specified path.
// Commonly used for creating resources and handling form submissions.
//
// Example:
//
//	r.POST("/users", createUserHandler)
//	r.POST("/login", loginHandler)
func (r *Router) POST(path string, handlers ...HandlerFunc) *Route {
	return r.addRouteWithConstraints("POST", path, handlers)
}

// PUT adds a route that matches PUT requests to the specified path.
// Typically used for updating or replacing entire resources.
//
// Example:
//
//	r.PUT("/users/:id", updateUserHandler)
func (r *Router) PUT(path string, handlers ...HandlerFunc) *Route {
	return r.addRouteWithConstraints("PUT", path, handlers)
}

// DELETE adds a route that matches DELETE requests to the specified path.
// Used for removing resources from the server.
//
// Example:
//
//	r.DELETE("/users/:id", deleteUserHandler)
func (r *Router) DELETE(path string, handlers ...HandlerFunc) *Route {
	return r.addRouteWithConstraints("DELETE", path, handlers)
}

// PATCH adds a route that matches PATCH requests to the specified path.
// Used for partial updates to existing resources.
//
// Example:
//
//	r.PATCH("/users/:id", patchUserHandler)
func (r *Router) PATCH(path string, handlers ...HandlerFunc) *Route {
	return r.addRouteWithConstraints("PATCH", path, handlers)
}

// OPTIONS adds a route that matches OPTIONS requests to the specified path.
// Commonly used for CORS preflight requests and API discovery.
//
// Example:
//
//	r.OPTIONS("/api/*", corsHandler)
func (r *Router) OPTIONS(path string, handlers ...HandlerFunc) *Route {
	return r.addRouteWithConstraints("OPTIONS", path, handlers)
}

// HEAD adds a route that matches HEAD requests to the specified path.
// HEAD requests are like GET requests but return only headers without the response body.
//
// Example:
//
//	r.HEAD("/users/:id", checkUserExistsHandler)
func (r *Router) HEAD(path string, handlers ...HandlerFunc) *Route {
	return r.addRouteWithConstraints("HEAD", path, handlers)
}

// addRouteWithConstraints adds a route with support for parameter constraints.
// Returns a Route object that can be used to add constraints and metadata.
// This method is now lock-free and uses atomic operations for thread safety.
func (r *Router) addRouteWithConstraints(method, path string, handlers []HandlerFunc) *Route {
	// Store route info for introspection (protected by separate mutex for low-frequency access)
	handlerName := "anonymous"
	if len(handlers) > 0 {
		handlerName = getHandlerName(handlers[len(handlers)-1])
	}

	r.routeTree.routesMutex.Lock()
	r.routeTree.routes = append(r.routeTree.routes, RouteInfo{
		Method:      method,
		Path:        path,
		HandlerName: handlerName,
	})
	r.routeTree.routesMutex.Unlock()

	// Create route object for constraint support
	route := &Route{
		router:   r,
		method:   method,
		path:     path,
		handlers: handlers,
	}

	// Record route registration for metrics
	r.recordRouteRegistration(method, path)

	// Note: The actual route is added to the tree when constraints are finalized
	// This is handled by finalizeRoute() which is called automatically
	route.finalizeRoute()

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

// PrintRoutes prints all registered routes to stdout in a formatted table.
// This is useful for development and debugging to see all available routes.
//
// Example output:
//
//	Method  Path              Handler
//	------  ----              -------
//	GET     /                 homeHandler
//	GET     /users/:id        getUserHandler
//	POST    /users            createUserHandler
func (r *Router) PrintRoutes() {
	routes := r.Routes()
	if len(routes) == 0 {
		fmt.Println("No routes registered")
		return
	}

	// Calculate column widths
	maxMethod := 6  // "Method"
	maxPath := 4    // "Path"
	maxHandler := 7 // "Handler"

	for _, route := range routes {
		if len(route.Method) > maxMethod {
			maxMethod = len(route.Method)
		}
		if len(route.Path) > maxPath {
			maxPath = len(route.Path)
		}
		if len(route.HandlerName) > maxHandler {
			maxHandler = len(route.HandlerName)
		}
	}

	// Print header
	fmt.Printf("%-*s  %-*s  %s\n", maxMethod, "Method", maxPath, "Path", "Handler")
	fmt.Printf("%s  %s  %s\n",
		strings.Repeat("-", maxMethod),
		strings.Repeat("-", maxPath),
		strings.Repeat("-", maxHandler))

	// Print routes
	for _, route := range routes {
		fmt.Printf("%-*s  %-*s  %s\n", maxMethod, route.Method, maxPath, route.Path, route.HandlerName)
	}
}

// finalizeRoute adds the route to the radix tree with its current constraints.
// This is called automatically when the route is created or when constraints are added.
// It uses the finalized flag to prevent duplicate route registration.
// This method is now lock-free and uses atomic operations for thread safety.
func (route *Route) finalizeRoute() {
	if route.finalized {
		return // Already added to tree, skip re-registration
	}
	route.finalized = true

	// Combine global middleware with route handlers
	allHandlers := append(route.router.middleware, route.handlers...)

	// Use efficient route addition that minimizes allocations
	route.router.addRouteToTree(route.method, route.path, allHandlers, route.constraints)

	// Routes will be compiled during WarmupOptimizations() call
	// No automatic compilation to avoid deadlocks
}

// Where adds a constraint to a route parameter using a regular expression.
// The constraint is pre-compiled for efficient validation during routing.
// This method provides a fluent interface for building routes with validation.
//
// IMPORTANT: This method panics if the regex pattern is invalid. This is intentional
// for fail-fast behavior during application startup. Ensure patterns are tested.
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
	// Pre-compile the regex pattern (panics on invalid pattern for fail-fast)
	regex, err := regexp.Compile("^" + pattern + "$")
	if err != nil {
		panic(fmt.Sprintf("Invalid regex pattern for parameter '%s': %v", param, err))
	}

	// Add constraint to the route
	route.constraints = append(route.constraints, RouteConstraint{
		Param:   param,
		Pattern: regex,
	})

	// Reset finalized flag and re-add the route to the tree with updated constraints
	route.finalized = false
	route.finalizeRoute()

	return route
}

// WhereNumber adds a constraint that ensures the parameter is a positive integer.
// This is a convenience method equivalent to Where(param, `\d+`).
//
// Example:
//
//	r.GET("/users/:id", getUserHandler).WhereNumber("id")
func (route *Route) WhereNumber(param string) *Route {
	return route.Where(param, `\d+`)
}

// WhereAlpha adds a constraint that ensures the parameter contains only letters.
// This is a convenience method equivalent to Where(param, `[a-zA-Z]+`).
//
// Example:
//
//	r.GET("/categories/:name", getCategoryHandler).WhereAlpha("name")
func (route *Route) WhereAlpha(param string) *Route {
	return route.Where(param, `[a-zA-Z]+`)
}

// WhereAlphaNumeric adds a constraint that ensures the parameter contains only letters and numbers.
// This is a convenience method equivalent to Where(param, `[a-zA-Z0-9]+`).
//
// Example:
//
//	r.GET("/slugs/:slug", getSlugHandler).WhereAlphaNumeric("slug")
func (route *Route) WhereAlphaNumeric(param string) *Route {
	return route.Where(param, `[a-zA-Z0-9]+`)
}

// WhereUUID adds a constraint that ensures the parameter is a valid UUID format.
// This is a convenience method for UUID validation.
//
// Example:
//
//	r.GET("/entities/:uuid", getEntityHandler).WhereUUID("uuid")
func (route *Route) WhereUUID(param string) *Route {
	return route.Where(param, `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
}

// getHandlerName extracts the function name from a HandlerFunc using reflection.
// This is used for route introspection and has zero performance impact on routing.
func getHandlerName(handler HandlerFunc) string {
	if handler == nil {
		return "nil"
	}

	funcPtr := runtime.FuncForPC(reflect.ValueOf(handler).Pointer())
	if funcPtr == nil {
		return "unknown"
	}

	fullName := funcPtr.Name()

	// Extract just the function name from the full path
	parts := strings.Split(fullName, ".")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		// Remove closure suffixes like .func1
		if strings.Contains(name, ".func") {
			return "anonymous"
		}
		return name
	}

	return "unknown"
}
