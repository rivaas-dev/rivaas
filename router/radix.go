package router

import (
	"strings"
	"sync"
)

// node represents a node in the radix tree used for fast route matching.
// The radix tree implementation is optimized for performance with different
// strategies for static and dynamic routes.
//
// Performance optimizations:
//   - Static routes use direct map lookups for O(1) access
//   - Parameter routes use segment-based traversal
//   - Full path storage for faster exact matching
//   - Pre-allocated children maps to reduce allocations
//   - Thread-safe operations for concurrent route registration
type node struct {
	handlers    []HandlerFunc     // Handler chain for this route
	children    map[string]*node  // Static child routes
	param       *param            // Parameter child route (if any)
	wildcard    *wildcard         // Wildcard child route (if any)
	constraints []RouteConstraint // Parameter constraints for this route
	path        string            // Full path for this node (optimization)
	mu          sync.RWMutex      // Protects concurrent access to this node
}

// param represents a parameter node in the radix tree.
// Parameter nodes capture dynamic segments of the URL path like ":id" or ":name".
type param struct {
	key  string // Parameter name (without the ':' prefix)
	node *node  // Child node for continuing the route match
}

// wildcard represents a wildcard node that matches everything.
// Used for static file serving with /* patterns.
type wildcard struct {
	node *node // Node containing the handlers for wildcard routes
}

// addRoute adds a route to the radix tree with optimized insertion strategy.
// The method handles different route types with specific optimizations:
//
//   - Root routes ("/") are handled specially
//   - Static routes (no parameters) use fast path insertion
//   - Parameter routes (":param") use segment-based insertion
//
// Static routes are stored directly in the children map for O(1) lookup,
// while parameter routes are built using segment traversal for flexibility.
func (n *node) addRoute(path string, handlers []HandlerFunc) {
	n.addRouteWithConstraints(path, handlers, nil)
}

// addRouteWithConstraints adds a route with parameter constraints.
// This method is thread-safe and can be called concurrently.
func (n *node) addRouteWithConstraints(path string, handlers []HandlerFunc, constraints []RouteConstraint) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Handle root path specially
	if path == "/" {
		n.handlers = handlers
		n.constraints = constraints
		n.path = "/"
		return
	}

	if path == "" {
		n.handlers = handlers
		n.constraints = constraints
		n.path = ""
		return
	}

	// Check for wildcard route first
	if strings.HasSuffix(path, "/*") {
		// Handle wildcard route
		prefix := strings.TrimSuffix(path, "/*")
		if prefix == "" {
			// Root wildcard
			if n.wildcard == nil {
				n.wildcard = &wildcard{node: &node{}}
			}
			n.wildcard.node.handlers = handlers
			n.wildcard.node.constraints = constraints
			n.wildcard.node.path = path
			return
		}

		// Navigate to the prefix and add wildcard
		segments := strings.Split(strings.Trim(prefix, "/"), "/")
		current := n

		for _, segment := range segments {
			if segment == "" {
				continue
			}
			if current.children == nil {
				current.children = make(map[string]*node, 4)
			}
			if current.children[segment] == nil {
				current.children[segment] = &node{}
			}
			current = current.children[segment]
		}

		if current.wildcard == nil {
			current.wildcard = &wildcard{node: &node{}}
		}
		current.wildcard.node.handlers = handlers
		current.wildcard.node.constraints = constraints
		current.wildcard.node.path = path
		return
	}

	// Optimize: avoid string splitting for simple paths without parameters
	if !strings.Contains(path, ":") {
		// Fast path for static routes
		if n.children == nil {
			n.children = make(map[string]*node, 8) // Pre-allocate with reasonable capacity
		}
		if n.children[path] == nil {
			n.children[path] = &node{}
		}
		n.children[path].handlers = handlers
		n.children[path].constraints = constraints
		n.children[path].path = path
		return
	}

	// Split path into segments for parameter routes
	segments := strings.Split(strings.Trim(path, "/"), "/")
	current := n

	for i, segment := range segments {
		if segment == "" {
			continue
		}

		isLast := i == len(segments)-1

		// Check if this is a parameter
		if strings.HasPrefix(segment, ":") {
			paramName := segment[1:]
			if current.param == nil {
				current.param = &param{key: paramName, node: &node{}}
			}
			current = current.param.node
		} else {
			// Regular segment
			if current.children == nil {
				current.children = make(map[string]*node, 4)
			}
			if current.children[segment] == nil {
				current.children[segment] = &node{}
			}
			current = current.children[segment]
		}

		// If this is the last segment, set handlers and constraints
		if isLast {
			current.handlers = handlers
			current.constraints = constraints
			current.path = path
		}
	}
}

// getRoute finds a route and extracts parameters directly into context arrays.
// This method provides the highest performance by avoiding map allocations entirely
// for routes with up to 8 parameters.
//
// Performance features:
//   - Zero-allocation path parsing
//   - Direct parameter storage in context arrays
//   - Ultra-fast static route detection
//   - Fallback to map for >8 parameters (rare case)
//
// This is the primary route matching method used by the router for maximum performance.
//
//go:inline
func (n *node) getRoute(path string, ctx *Context) []HandlerFunc {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Handle root path specially - most common case
	if path == "/" {
		return n.handlers
	}

	if path == "" {
		return n.handlers
	}

	// Ultra-fast path for static routes (no parameters) - check full path first
	if n.children != nil {
		if child, exists := n.children[path]; exists && child.handlers != nil {
			return child.handlers
		}
	}

	// Zero-allocation path parsing - avoid strings.Split entirely
	current := n
	start := 0
	if path[0] == '/' {
		start = 1 // Skip leading slash
	}

	pathLen := len(path)

	for start < pathLen {
		// Find next slash or end of path
		end := start
		for end < pathLen && path[end] != '/' {
			end++
		}

		// Extract segment without allocation
		segment := path[start:end]
		isLast := end >= pathLen

		// Try to match exact segment first
		if current.children != nil && current.children[segment] != nil {
			current = current.children[segment]
		} else if current.param != nil {
			// Match parameter - store directly in context arrays
			if ctx.paramCount < 8 {
				ctx.paramKeys[ctx.paramCount] = current.param.key
				ctx.paramValues[ctx.paramCount] = segment
				ctx.paramCount++
			} else {
				// Fallback to map for >8 params (very rare)
				if ctx.Params == nil {
					ctx.Params = make(map[string]string, 1)
				}
				ctx.Params[current.param.key] = segment
			}
			current = current.param.node
		} else if current.wildcard != nil {
			// Match wildcard - captures rest of path
			if ctx.paramCount < 8 {
				ctx.paramKeys[ctx.paramCount] = "filepath"
				ctx.paramValues[ctx.paramCount] = path[start:]
				ctx.paramCount++
			} else {
				if ctx.Params == nil {
					ctx.Params = make(map[string]string, 1)
				}
				ctx.Params["filepath"] = path[start:]
			}
			return current.wildcard.node.handlers
		} else {
			// No match
			return nil
		}

		// If this is the last segment, validate constraints and return handlers
		if isLast {
			if current.handlers != nil && !validateConstraints(current.constraints, ctx) {
				return nil // Constraint validation failed
			}
			return current.handlers
		}

		start = end + 1 // Move past the slash
	}

	// If we reached here without returning, no match
	return nil
}

// getRouteStatic provides ultra-fast lookup for static routes only.
// This method is used as the first attempt in route matching to handle
// the common case of static routes with maximum performance.
//
// Static routes (no parameters) can be matched with a simple map lookup,
// which is significantly faster than parameter parsing.
//
// Returns handlers if found, nil if the route is not static or doesn't exist.
//
//go:inline
func (n *node) getRouteStatic(path string) []HandlerFunc {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Handle root path
	if path == "/" {
		return n.handlers
	}

	// Try direct static route lookup (no parameters)
	if n.children != nil {
		if child, exists := n.children[path]; exists {
			return child.handlers
		}
	}

	// Not a static route
	return nil
}

// validateConstraints checks if all parameter constraints are satisfied.
// This function is optimized for performance with early exits.
func validateConstraints(constraints []RouteConstraint, ctx *Context) bool {
	if len(constraints) == 0 {
		return true // No constraints to validate
	}

	for _, constraint := range constraints {
		var value string
		found := false

		// Check fast array lookup first (up to 8 params)
		for i := range ctx.paramCount {
			if ctx.paramKeys[i] == constraint.Param {
				value = ctx.paramValues[i]
				found = true
				break
			}
		}

		// Fallback to map for >8 parameters
		if !found && ctx.Params != nil {
			value, found = ctx.Params[constraint.Param]
		}

		// If parameter not found or doesn't match constraint, fail
		if !found || !constraint.Pattern.MatchString(value) {
			return false
		}
	}

	return true
}
