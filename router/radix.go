package router

import (
	"hash/fnv"
	"strings"
	"sync"
)

// fnv1aHash computes FNV-1a hash of a string without allocations
func fnv1aHash(s string) uint64 {
	const (
		offset64 uint64 = 14695981039346656037
		prime64  uint64 = 1099511628211
	)

	hash := offset64
	for i := 0; i < len(s); i++ {
		hash ^= uint64(s[i])
		hash *= prime64
	}
	return hash
}

// bloomFilter provides a simple bloom filter implementation for fast negative lookups
type bloomFilter struct {
	bits      []uint64
	size      uint64
	hashFuncs []func([]byte) uint64
}

// newBloomFilter creates a new bloom filter with the specified size and hash functions
func newBloomFilter(size uint64, numHashFuncs int) *bloomFilter {
	bf := &bloomFilter{
		bits:      make([]uint64, (size+63)/64), // Round up to nearest 64-bit boundary
		size:      size,
		hashFuncs: make([]func([]byte) uint64, numHashFuncs),
	}

	// Initialize hash functions (simple hash functions for performance)
	for i := 0; i < numHashFuncs; i++ {
		seed := uint64(i + 1)
		bf.hashFuncs[i] = func(data []byte) uint64 {
			h := fnv.New64a()
			h.Write([]byte{byte(seed)})
			h.Write(data)
			return h.Sum64() % size
		}
	}

	return bf
}

// Add adds an element to the bloom filter
func (bf *bloomFilter) Add(data []byte) {
	for _, hashFunc := range bf.hashFuncs {
		pos := hashFunc(data)
		bf.bits[pos/64] |= 1 << (pos % 64)
	}
}

// Test checks if an element might be in the bloom filter
func (bf *bloomFilter) Test(data []byte) bool {
	for _, hashFunc := range bf.hashFuncs {
		pos := hashFunc(data)
		if bf.bits[pos/64]&(1<<(pos%64)) == 0 {
			return false
		}
	}
	return true
}

// CompiledRoute represents a pre-compiled static route for fast lookup
type CompiledRoute struct {
	path     string        // Route path
	handlers []HandlerFunc // Handler chain
	hash     uint64        // Pre-computed hash for instant matching
}

// CompiledRouteTable provides fast static route lookup
type CompiledRouteTable struct {
	routes map[uint64]*CompiledRoute // Hash-based route lookup
	bloom  *bloomFilter              // Bloom filter for fast negative lookups
	mu     sync.RWMutex              // Protects concurrent access
}

// node represents a node in the radix tree used for fast route matching.
// The radix tree implementation uses different strategies for static and dynamic routes.
//
// Performance features:
//   - Static routes use direct map lookups for O(1) access
//   - Parameter routes use segment-based traversal
//   - Full path storage for faster exact matching
//   - Pre-allocated children maps to reduce allocations
//   - Thread-safe operations for concurrent route registration
//   - Compiled route table for fast static route matching
type node struct {
	handlers    []HandlerFunc       // Handler chain for this route
	children    map[string]*node    // Static child routes
	param       *param              // Parameter child route (if any)
	wildcard    *wildcard           // Wildcard child route (if any)
	constraints []RouteConstraint   // Parameter constraints for this route
	path        string              // Full path for this node (optimization)
	compiled    *CompiledRouteTable // Compiled static routes for fast lookup
	mu          sync.RWMutex        // Protects concurrent access to this node
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
	node      *node  // Node containing the handlers for wildcard routes
	paramName string // Custom parameter name instead of default "filepath"
}

// addRoute adds a route to the radix tree with insertion strategy.
// The method handles different route types with specific approaches:
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
		paramName := "filepath" // Default parameter name

		// For now, use simple wildcard parsing
		// Custom parameter names can be added later if needed

		if prefix == "" {
			// Root wildcard
			if n.wildcard == nil {
				n.wildcard = &wildcard{node: &node{}, paramName: paramName}
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
			current.wildcard = &wildcard{node: &node{}, paramName: paramName}
		}
		current.wildcard.node.handlers = handlers
		current.wildcard.node.constraints = constraints
		current.wildcard.node.path = path
		return
	}

	// Optimize: avoid string splitting for simple paths without parameters
	if !strings.Contains(path, ":") && !strings.HasSuffix(path, "/*") {
		// Fast path for static routes (but not wildcard routes)
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
// This method provides good performance by avoiding map allocations entirely
// for routes with up to 8 parameters.
//
// Performance features:
//   - Efficient path parsing
//   - Direct parameter storage in context arrays
//   - Fast static route detection
//   - Fallback to map for >8 parameters (rare case)
//
// This is the primary route matching method used by the router.
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

	// Fast path for static routes (no parameters) - check full path first
	if n.children != nil {
		if child, exists := n.children[path]; exists && child.handlers != nil {
			return child.handlers
		}
	}

	// Efficient path parsing - avoid strings.Split entirely
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
			// Match parameter - efficient storage
			if ctx.paramCount < 8 {
				// Fast path: use pre-allocated arrays
				ctx.paramKeys[ctx.paramCount] = current.param.key
				ctx.paramValues[ctx.paramCount] = segment
				ctx.paramCount++
			} else {
				// Fallback to map for >8 params (very rare) - reuse existing map
				if ctx.Params == nil {
					ctx.Params = make(map[string]string, 2) // Pre-allocate with capacity
				}
				ctx.Params[current.param.key] = segment
			}
			current = current.param.node
		} else if current.wildcard != nil {
			// Match wildcard - captures rest of path
			paramName := current.wildcard.paramName
			if paramName == "" {
				paramName = "filepath" // Default fallback
			}

			if ctx.paramCount < 8 {
				ctx.paramKeys[ctx.paramCount] = paramName
				ctx.paramValues[ctx.paramCount] = path[start:]
				ctx.paramCount++
			} else {
				if ctx.Params == nil {
					ctx.Params = make(map[string]string, 2) // Pre-allocate with capacity
				}
				ctx.Params[paramName] = path[start:]
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

// validateConstraints checks if all parameter constraints are satisfied.
// This function uses early exits for performance.
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

// compileStaticRoutes compiles all static routes in this node and its children
// into a lookup table with bloom filter for fast matching
func (n *node) compileStaticRoutes() *CompiledRouteTable {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Initialize compiled table if not exists
	if n.compiled == nil {
		n.compiled = &CompiledRouteTable{
			routes: make(map[uint64]*CompiledRoute, 16), // Pre-allocate with capacity
			bloom:  newBloomFilter(1000, 3),             // Smaller bloom filter for better performance
		}
	}

	// Compile routes recursively
	n.compileStaticRoutesRecursive(n.compiled, "")

	return n.compiled
}

// compileStaticRoutesRecursive recursively compiles static routes
func (n *node) compileStaticRoutesRecursive(table *CompiledRouteTable, prefix string) {
	// If this node has handlers and is a static route (no parameters), compile it
	if n.handlers != nil && !strings.Contains(prefix, ":") && prefix != "" {
		// Compute hash for fast lookup
		hash := fnv.New64a()
		hash.Write([]byte(prefix))
		routeHash := hash.Sum64()

		// Create compiled route
		compiledRoute := &CompiledRoute{
			path:     prefix,
			handlers: n.handlers,
			hash:     routeHash,
		}

		// Store in routes map
		table.routes[routeHash] = compiledRoute

		// Add to bloom filter for fast negative lookups
		table.bloom.Add([]byte(prefix))
	}

	// Recursively compile children
	for path, child := range n.children {
		childPath := prefix
		if prefix == "" {
			childPath = "/" + path
		} else {
			childPath = prefix + "/" + path
		}
		child.compileStaticRoutesRecursive(table, childPath)
	}
}

// getRouteCompiled provides fast lookup for compiled static routes
// Returns handlers if found, nil if not a static route or doesn't exist
func (table *CompiledRouteTable) getRoute(path string) []HandlerFunc {
	if table == nil {
		return nil
	}

	table.mu.RLock()
	defer table.mu.RUnlock()

	// For small route sets, skip bloom filter to avoid allocations
	if len(table.routes) < 10 {
		// Direct hash lookup without bloom filter - use FNV-1a for zero allocations
		routeHash := fnv1aHash(path)

		if route, exists := table.routes[routeHash]; exists {
			return route.handlers
		}
		return nil
	}

	// Quick bloom filter check for negative lookups (eliminates most misses)
	if !table.bloom.Test([]byte(path)) {
		return nil
	}

	// Compute hash for exact match - use FNV-1a for zero allocations
	routeHash := fnv1aHash(path)

	// Fast hash-based lookup
	if route, exists := table.routes[routeHash]; exists {
		return route.handlers
	}

	return nil
}
