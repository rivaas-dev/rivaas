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
	"hash/fnv"
	"maps"
	"strings"
	"sync"

	"rivaas.dev/router/compiler"
	"rivaas.dev/router/route"
)

// CompiledRoute represents a compiled static route for lookup
type CompiledRoute struct {
	path     string        // Route path
	handlers []HandlerFunc // Handler chain
	hash     uint64        // Hash for route matching
}

// CompiledRouteTable provides static route lookup
type CompiledRouteTable struct {
	routes map[uint64]*CompiledRoute // Route lookup map
	bloom  *compiler.BloomFilter     // Bloom filter for negative lookups
	mu     sync.RWMutex              // Protects concurrent access
}

// edge represents a per-segment child in the radix tree (avoids map hashing in hot path).
type edge struct {
	label string
	node  *node
}

// node represents a node in the route tree used for route matching.
// The tree implementation uses different strategies for static and dynamic routes.
//
// Features:
//   - edges: per-segment children (linear scan for traversal, no map hash)
//   - staticPaths: full-path static routes at root only (e.g. /api/users)
//   - Parameter routes use segment-based traversal via edges
//
// Thread safety:
// Routes are registered during a single-threaded configuration phase (before
// ServeHTTP/Start is called). After Freeze(), the tree is immutable and safe
// for concurrent reads without locking.
type node struct {
	handlers    []HandlerFunc       // Handler chain for this route
	edges       []edge              // Per-segment children (linear scan for traversal)
	staticPaths map[string]*node    // Full-path static routes (root node only, nil otherwise)
	param       *param              // Parameter child route (if any)
	wildcard    *wildcard           // Wildcard child route (if any)
	constraints []route.Constraint  // Parameter constraints for this route
	path        string              // Full path for this node
	compiled    *CompiledRouteTable // Route table for static route matching
}

// findChild returns the child node for the given segment, or nil.
func (n *node) findChild(segment string) *node {
	for i := range n.edges {
		if n.edges[i].label == segment {
			return n.edges[i].node
		}
	}
	return nil
}

// findOrCreateChild returns the child node for the given segment, creating it if needed.
func (n *node) findOrCreateChild(segment string) *node {
	for i := range n.edges {
		if n.edges[i].label == segment {
			return n.edges[i].node
		}
	}
	child := &node{}
	n.edges = append(n.edges, edge{label: segment, node: child})
	return child
}

// param represents a parameter node in the route tree.
// Parameter nodes capture dynamic segments of the URL path like ":id" or ":name".
type param struct {
	key  string // Parameter name (without the ':' prefix)
	node *node  // Child node for continuing the route match
}

// wildcard represents a wildcard node that matches everything.
// Used for static file serving with /* patterns.
type wildcard struct {
	node      *node  // Node containing the handlers for wildcard routes
	paramName string // Custom parameter name instead of default "filepath"
}

// addRouteWithConstraints adds a route with parameter constraints.
//
// Thread safety: This method must only be called during the configuration phase,
// before any calls to ServeHTTP or Freeze. After the router starts serving,
// routes are immutable. This design eliminates the need for per-node locks
// and prevents data races between route registration and request serving.
//
// The tree structure supports three types of nodes:
// 1. Static nodes: Exact string match (e.g., "users", "api")
// 2. Parameter nodes: Dynamic segments (e.g., :id, :name)
// 3. Wildcard nodes: Catch-all (e.g., /* for static files)
//
// Route examples and their tree structure:
// - "/users" → root.staticPaths["/users"] or root.edges (per-segment)
// - "/users/:id" → root.findChild("users").param.node
// - "/static/*" → root.findChild("static").wildcard.node
func (n *node) addRouteWithConstraints(path string, handlers []HandlerFunc, constraints []route.Constraint) {
	// NOTE: We store handlers directly without copying.
	// Callers MUST NOT modify the handler slice after registration.
	// This is safe because:
	// 1. Router.addRoute creates a new slice when combining middleware + handlers
	// 2. User code typically passes fresh slices: []HandlerFunc{handler1, handler2}
	// 3. Defensive copying on every route registration is avoided
	//
	// If slice aliasing becomes an issue in practice, only copy when cap > len
	// (indicating potential for append mutations):
	//   if cap(handlers) > len(handlers) {
	//       handlersCopy := make([]HandlerFunc, len(handlers))
	//       copy(handlersCopy, handlers)
	//       handlers = handlersCopy
	//   }

	// Special case: Root path
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

	// Detect and handle wildcard routes (e.g., /static/*)
	// Wildcards must be checked first
	if prefix, ok := strings.CutSuffix(path, "/*"); ok {
		// Wildcard route: matches everything after the prefix
		// Example: "/static/*" matches /static/css/app.css, /static/js/main.js, etc.
		paramName := "filepath" // Default parameter name for wildcard captures

		if prefix == "" {
			// Root wildcard: /* (matches everything)
			if n.wildcard == nil {
				n.wildcard = &wildcard{node: &node{}, paramName: paramName}
			}
			n.wildcard.node.handlers = handlers
			n.wildcard.node.constraints = constraints
			n.wildcard.node.path = path

			return
		}

		// Navigate to the prefix node and add wildcard
		// Example: "/static/*" → navigate to "static" node, add wildcard child
		segments := strings.Split(strings.Trim(prefix, "/"), "/")
		current := n

		for _, segment := range segments {
			if segment == "" {
				continue
			}
			current = current.findOrCreateChild(segment)
		}

		if current.wildcard == nil {
			current.wildcard = &wildcard{node: &node{}, paramName: paramName}
		}
		current.wildcard.node.handlers = handlers
		current.wildcard.node.constraints = constraints
		current.wildcard.node.path = path

		return
	}

	// Path for simple static routes (no : or *)
	// Store the entire path in staticPaths (root only) for full-path lookup
	// Example: "/api/users" is stored as staticPaths["/api/users"]
	if !strings.Contains(path, ":") && !strings.HasSuffix(path, "/*") {
		if n.staticPaths == nil {
			n.staticPaths = make(map[string]*node, 8)
		}
		if n.staticPaths[path] == nil {
			n.staticPaths[path] = &node{}
		}
		n.staticPaths[path].handlers = handlers
		n.staticPaths[path].constraints = constraints
		n.staticPaths[path].path = path

		return
	}

	// Standard path: Contains parameters (e.g., /users/:id/posts/:post_id)
	// Split into segments and build radix tree structure
	// Example: "/users/:id/posts" →
	//
	//	root → children["users"] → param{key:"id"} → children["posts"]
	segments := strings.Split(strings.Trim(path, "/"), "/")
	current := n

	for i, segment := range segments {
		if segment == "" {
			continue
		}

		isLast := i == len(segments)-1

		// Determine segment type and create appropriate node
		if strings.HasPrefix(segment, ":") {
			// Parameter segment: :id, :name, :post_id, etc.
			paramName := segment[1:] // Remove ':' prefix
			if current.param == nil {
				// Create param node if it doesn't exist
				// Each node can have at most one param child (radix tree property)
				current.param = &param{key: paramName, node: &node{}}
			}
			current = current.param.node
		} else {
			// Static segment: "users", "api", "posts", etc.
			current = current.findOrCreateChild(segment)
		}

		// If this is the last segment, attach handlers and constraints
		if isLast {
			current.handlers = handlers
			current.constraints = constraints
			current.path = path
		}
	}
}

// getRoute finds a route and extracts parameters into context arrays.
// Returns both the handlers and the route pattern for observability.
//
// Thread safety: This method is safe for concurrent use after Freeze().
// The route tree is immutable after Freeze, so no locking is required.
//
// Returns:
//   - handlers: The handler chain for the matched route (nil if no match)
//   - pattern: The original route pattern (e.g., "/users/:id", "/posts/:pid/comments", "")
//     Empty string if no route matches or pattern not available
//
// For implementation details, see radix_test.go.
func (n *node) getRoute(path string, ctx *Context) ([]HandlerFunc, string) {
	// Handle root path specially (common case)
	if path == "/" {
		return n.handlers, n.path
	}

	if path == "" {
		return n.handlers, n.path
	}

	// Path for static routes (no parameters)
	// During addRoute(), static routes are stored in staticPaths at root (e.g., "/api/users")
	if n.staticPaths != nil {
		if child := n.staticPaths[path]; child != nil && child.handlers != nil {
			return child.handlers, child.path
		}
	}

	// Manual path parsing without strings.Split
	// Why: strings.Split allocates a []string which we don't need
	// Instead: Parse segments on-the-fly using string slicing
	current := n
	start := 0
	if path[0] == '/' {
		start = 1 // Skip leading slash
	}

	pathLen := len(path)

	// Main radix tree traversal loop
	// Each iteration processes one path segment (e.g., "users", "123", "posts")
	for start < pathLen {
		// Find the next slash or end of path (manual parsing)
		end := start
		for end < pathLen && path[end] != '/' {
			end++
		}

		// Extract segment - just slice the original string
		// Example: "/users/123/posts" → "users" → "123" → "posts"
		segment := path[start:end]
		isLast := end >= pathLen

		// Try matching strategies in priority order:
		// Priority 1: Exact static match (linear scan over edges)
		if next := current.findChild(segment); next != nil {
			current = next
		} else if current.param != nil {
			// Priority 2: Parameter match (e.g., :id, :name)
			// Store parameter based on count
			// Avoid bounds check by using constant
			paramIdx := ctx.paramCount
			if paramIdx < 8 {
				// Use pre-allocated arrays
				// Most routes have ≤8 params, so this is the common case
				ctx.paramKeys[paramIdx] = current.param.key
				ctx.paramValues[paramIdx] = segment
				ctx.paramCount = paramIdx + 1
			} else {
				// Fallback: use map for >8 params (extremely rare)
				// Example: /a/:p1/b/:p2/c/:p3/.../i/:p9 (unusual route design)
				if ctx.Params == nil {
					ctx.Params = make(map[string]string, 2) // Pre-allocate with capacity
				}
				ctx.Params[current.param.key] = segment
			}
			current = current.param.node
		} else if current.wildcard != nil {
			// Priority 3: Wildcard match (e.g., /static/*)
			// Captures everything from this point onwards
			paramName := current.wildcard.paramName
			if paramName == "" {
				paramName = "filepath" // Default parameter name
			}

			// Store the remainder of the path as the wildcard value
			paramIdx := ctx.paramCount
			if paramIdx < 8 {
				ctx.paramKeys[paramIdx] = paramName
				ctx.paramValues[paramIdx] = path[start:] // Everything from here
				ctx.paramCount = paramIdx + 1
			} else {
				if ctx.Params == nil {
					ctx.Params = make(map[string]string, 2)
				}
				ctx.Params[paramName] = path[start:]
			}

			return current.wildcard.node.handlers, current.wildcard.node.path
		} else {
			// No match found - route doesn't exist
			return nil, ""
		}

		// If this is the last segment, validate constraints and return
		if isLast {
			// Validate parameter constraints (e.g., :id must be numeric)
			if current.handlers != nil && !validateConstraints(current.constraints, ctx) {
				return nil, "" // Constraint validation failed
			}

			return current.handlers, current.path
		}

		start = end + 1 // Move past the slash to next segment
	}

	// Reached end of path without matching - route not found
	return nil, ""
}

// validateConstraints checks if all parameter constraints are satisfied.
// This function uses early exits.
//
// For routes with many constraints and many parameters,
// build a temporary map to avoid nested loops. Otherwise use array lookup.
func validateConstraints(constraints []route.Constraint, ctx *Context) bool {
	if len(constraints) == 0 {
		return true // No constraints to validate
	}

	// Path: Few constraints - use array lookup
	// Threshold: <=3 constraints OR <=4 parameters
	// For small sizes, nested loop overhead is less than map creation overhead
	if len(constraints) <= 3 || ctx.paramCount <= 4 {
		for _, constraint := range constraints {
			var value string
			found := false

			// Check array lookup first (up to 8 params)
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

	// Slow path: Many constraints AND many parameters - build lookup map once
	// This reduces nested loops
	params := make(map[string]string, ctx.paramCount)
	for i := range ctx.paramCount {
		params[ctx.paramKeys[i]] = ctx.paramValues[i]
	}
	// Merge overflow map if exists
	if ctx.Params != nil {
		maps.Copy(params, ctx.Params)
	}

	// Validate all constraints with map lookup
	for _, constraint := range constraints {
		value, found := params[constraint.Param]
		if !found || !constraint.Pattern.MatchString(value) {
			return false
		}
	}

	return true
}

// countStaticRoutes recursively counts the number of static routes (no parameters) in this node and its children.
// Static routes are those without ':' parameters or '*' wildcards.
//
// Thread safety: Called during Freeze(), before serving begins.
func (n *node) countStaticRoutes() int {
	count := 0
	// Count this node if it has handlers and is static (no parameters in path)
	if n.handlers != nil && n.path != "" && !strings.Contains(n.path, ":") && !strings.HasSuffix(n.path, "*") {
		count++
	}

	// Count static routes in edges and staticPaths
	for i := range n.edges {
		count += n.edges[i].node.countStaticRoutes()
	}
	if n.staticPaths != nil {
		for _, child := range n.staticPaths {
			count += child.countStaticRoutes()
		}
	}

	// Don't count param or wildcard nodes (they're not static)
	return count
}

// compileStaticRoutes compiles all static routes in this node and its children
// into a lookup table with bloom filter for matching.
// This method should only be called during Freeze(), after all routes are registered.
// The bloomFilterSize and numHashFuncs parameters control the bloom filter configuration.
//
// Thread safety: Called during Freeze(), before serving begins.
func (n *node) compileStaticRoutes(bloomFilterSize uint64, numHashFuncs int) *CompiledRouteTable {
	// Initialize compiled table if not exists
	if n.compiled == nil {
		// Use configured bloom filter size, with a minimum of 100
		size := max(bloomFilterSize, 100)
		n.compiled = &CompiledRouteTable{
			routes: make(map[uint64]*CompiledRoute, 16),         // Pre-allocate with capacity
			bloom:  compiler.NewBloomFilter(size, numHashFuncs), // Configurable bloom filter
		}
	}

	table := n.compiled

	// Compile routes recursively
	n.compileStaticRoutesRecursive(table, "")

	return table
}

// compileStaticRoutesRecursive recursively compiles static routes.
//
// Thread safety: Called during Freeze(), before serving begins.
// No locking needed as route tree is not accessed concurrently during compilation.
func (n *node) compileStaticRoutesRecursive(table *CompiledRouteTable, prefix string) {
	handlers := n.handlers

	// If this node has handlers and is a static route (no parameters), compile it
	if handlers != nil && !strings.Contains(prefix, ":") && prefix != "" {
		// Compute hash
		h := fnv.New64a()
		h.Write([]byte(prefix))
		routeHash := h.Sum64()

		// Create compiled route
		compiledRoute := &CompiledRoute{
			path:     prefix,
			handlers: handlers,
			hash:     routeHash,
		}

		// Store in routes map (table access is already protected by caller's lock)
		table.routes[routeHash] = compiledRoute

		// Add to bloom filter for negative lookups
		table.bloom.Add([]byte(prefix))
	}

	// Recursively compile edges (per-segment children)
	for i := range n.edges {
		e := &n.edges[i]
		childPath := prefix
		if childPath == "" {
			childPath = "/" + e.label
		} else {
			childPath = prefix + "/" + e.label
		}
		e.node.compileStaticRoutesRecursive(table, childPath)
	}
	// Recursively compile staticPaths (full-path children at root; key is full path)
	for fullPath, child := range n.staticPaths {
		child.compileStaticRoutesRecursive(table, fullPath)
	}
}

// getRouteCompiled provides lookup for compiled static routes
// This is used for static routes without parameters (e.g., /health, /api/users)
// Returns handlers if found, nil if not a static route or doesn't exist
func (table *CompiledRouteTable) getRoute(path string) []HandlerFunc {
	table.mu.RLock()
	defer table.mu.RUnlock()

	// For small route sets, use direct lookup
	if len(table.routes) < 10 {
		// Direct lookup without bloom filter
		h := fnv.New64a()
		h.Write([]byte(path))
		routeHash := h.Sum64()

		if route, exists := table.routes[routeHash]; exists {
			return route.handlers
		}

		return nil
	}

	// Stage 1: Bloom filter check for negative lookups
	if !table.bloom.Test([]byte(path)) {
		return nil // Definitely not in the set
	}

	// Stage 2: Check the actual map
	h := fnv.New64a()
	h.Write([]byte(path))
	routeHash := h.Sum64()
	if route, exists := table.routes[routeHash]; exists {
		return route.handlers
	}

	// Bloom filter false positive - route doesn't actually exist
	// False positive rate depends on bloom filter size and number of routes.
	// With default configuration (1000 bits, 3 hash functions), false positive
	// rate is typically <5% for typical route counts (<1000 routes).
	return nil
}

// getRouteWithPath provides lookup for compiled static routes and returns both handlers and route path.
// This is used when you need the actual route pattern (e.g., for logging/metrics).
// Returns (handlers, routePath) if found, (nil, "") if not a static route or doesn't exist.
func (table *CompiledRouteTable) getRouteWithPath(path string) ([]HandlerFunc, string) {
	table.mu.RLock()
	defer table.mu.RUnlock()

	// For small route sets, use direct lookup
	if len(table.routes) < 10 {
		// Direct lookup without bloom filter
		h := fnv.New64a()
		h.Write([]byte(path))
		routeHash := h.Sum64()

		if route, exists := table.routes[routeHash]; exists {
			return route.handlers, route.path
		}

		return nil, ""
	}

	// Stage 1: Bloom filter check for negative lookups
	if !table.bloom.Test([]byte(path)) {
		return nil, "" // Definitely not in the set
	}

	// Stage 2: Check the actual map
	h := fnv.New64a()
	h.Write([]byte(path))
	routeHash := h.Sum64()
	if route, exists := table.routes[routeHash]; exists {
		return route.handlers, route.path
	}

	// Bloom filter false positive - route doesn't actually exist
	return nil, ""
}
