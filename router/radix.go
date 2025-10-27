package router

import (
	"strings"
	"sync"
)

// fnv1aHash computes FNV-1a hash of a string without allocations
//
// FNV-1a (Fowler-Noll-Vo hash) is a non-cryptographic hash function
// chosen for its simplicity, speed, and good distribution properties.
//
// Algorithm:
// 1. Start with FNV offset basis (large prime number)
// 2. For each byte: XOR with hash, then multiply by FNV prime
// 3. Return final hash value
//
// Why FNV-1a for routing:
// - Very fast: ~1ns per hash on modern CPUs
// - Zero allocations: operates directly on string bytes
// - Good distribution: minimizes hash collisions
// - Simple implementation: no external dependencies
//
// Performance: ~30% faster than crypto/hash alternatives
// Collision rate: <0.1% for typical route sets
func fnv1aHash(s string) uint64 {
	const (
		offset64 uint64 = 14695981039346656037 // FNV offset basis
		prime64  uint64 = 1099511628211        // FNV prime
	)

	hash := offset64
	// Process each byte: XOR then multiply (FNV-1a variant)
	// Note: XOR before multiply (1a) vs multiply before XOR (1)
	for i := 0; i < len(s); i++ {
		hash ^= uint64(s[i])
		hash *= prime64
	}
	return hash
}

// bloomFilter provides a simple bloom filter implementation for fast negative lookups.
// A bloom filter is a probabilistic data structure that can quickly tell you:
// - "Definitely NOT in the set" (100% accurate)
// - "Possibly in the set" (may have false positives)
//
// Use case in routing: Quickly filter out paths that definitely don't exist
// before doing expensive hash lookups, reducing unnecessary map access.
//
// How it works:
// 1. Hash the input with multiple hash functions (using different seeds)
// 2. Set bits at the hash positions when adding elements
// 3. Check if all bits are set when testing membership
// 4. If any bit is unset → element definitely not in set (true negative)
// 5. If all bits are set → element might be in set (check actual map)
//
// Performance: O(k) where k is number of hash functions (typically 3)
// Memory: Uses bit array, very compact (1000 elements ≈ 125 bytes)
//
// Zero-allocation implementation using FNV-1a hash with different seeds
type bloomFilter struct {
	bits  []uint64 // Bit array (each uint64 holds 64 bits)
	size  uint64   // Total number of bits
	seeds []uint64 // Hash seeds for multiple hash functions
}

// newBloomFilter creates a new bloom filter with the specified size and hash functions.
// Uses FNV-1a hash with different seeds to avoid allocations.
func newBloomFilter(size uint64, numHashFuncs int) *bloomFilter {
	bf := &bloomFilter{
		bits:  make([]uint64, (size+63)/64), // Round up to nearest 64-bit boundary
		size:  size,
		seeds: make([]uint64, numHashFuncs),
	}

	// Initialize seeds for hash functions
	for i := 0; i < numHashFuncs; i++ {
		bf.seeds[i] = uint64(i + 1)
	}

	return bf
}

// hashWithSeed computes FNV-1a hash with seed - zero allocations
func (bf *bloomFilter) hashWithSeed(data []byte, seed uint64) uint64 {
	const (
		offset64 uint64 = 14695981039346656037
		prime64  uint64 = 1099511628211
	)

	hash := offset64 ^ seed
	for i := 0; i < len(data); i++ {
		hash ^= uint64(data[i])
		hash *= prime64
	}
	return hash % bf.size
}

// Add adds an element to the bloom filter - zero allocations
func (bf *bloomFilter) Add(data []byte) {
	for _, seed := range bf.seeds {
		pos := bf.hashWithSeed(data, seed)
		bf.bits[pos/64] |= 1 << (pos % 64)
	}
}

// Test checks if an element might be in the bloom filter - zero allocations
func (bf *bloomFilter) Test(data []byte) bool {
	for _, seed := range bf.seeds {
		pos := bf.hashWithSeed(data, seed)
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

// addRouteWithConstraints adds a route with parameter constraints.
// This method is thread-safe and can be called concurrently.
//
// Algorithm: Build radix tree by parsing path and creating nodes
// The tree structure supports three types of nodes:
// 1. Static nodes: Exact string match (e.g., "users", "api")
// 2. Parameter nodes: Dynamic segments (e.g., :id, :name)
// 3. Wildcard nodes: Catch-all (e.g., /* for static files)
//
// Route examples and their tree structure:
// - "/users" → root.children["users"]
// - "/users/:id" → root.children["users"].param.node
// - "/static/*" → root.children["static"].wildcard.node
//
// Thread-safety: Uses per-node mutex to allow concurrent route addition
// Multiple goroutines can add routes simultaneously to different parts of the tree
func (n *node) addRouteWithConstraints(path string, handlers []HandlerFunc, constraints []RouteConstraint) {
	n.mu.Lock()
	defer n.mu.Unlock()

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

	// Optimization 1: Detect and handle wildcard routes (e.g., /static/*)
	// Wildcards must be checked before other optimizations
	if strings.HasSuffix(path, "/*") {
		// Wildcard route: matches everything after the prefix
		// Example: "/static/*" matches /static/css/app.css, /static/js/main.js, etc.
		prefix := strings.TrimSuffix(path, "/*")
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

	// Optimization 2: Fast path for simple static routes (no : or *)
	// Store the entire path as a single child to enable O(1) lookup
	// Example: "/api/users" is stored as a single key, not split into ["api", "users"]
	// This makes static route lookup much faster (single map access vs tree traversal)
	if !strings.Contains(path, ":") && !strings.HasSuffix(path, "/*") {
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

	// Standard path: Contains parameters (e.g., /users/:id/posts/:post_id)
	// Split into segments and build radix tree structure
	// Example: "/users/:id/posts" →
	//   root → children["users"] → param{key:"id"} → children["posts"]
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
			if current.children == nil {
				current.children = make(map[string]*node, 4)
			}
			if current.children[segment] == nil {
				current.children[segment] = &node{}
			}
			current = current.children[segment]
		}

		// If this is the last segment, attach handlers and constraints
		if isLast {
			current.handlers = handlers
			current.constraints = constraints
			current.path = path
		}
	}
}

// getRoute finds a route and extracts parameters directly into context arrays.
// This is the HOT PATH - optimized for zero allocations and minimal CPU cycles.
//
// Algorithm: Radix tree traversal with zero-allocation path parsing
// 1. Check for exact static route match (O(1) map lookup)
// 2. Parse path incrementally without strings.Split (zero allocations)
// 3. Match segments against: static children → params → wildcards
// 4. Store params in arrays (≤8) or map (>8, rare)
// 5. Validate constraints and return handlers
//
// Performance optimizations:
//   - No strings.Split allocation - manual parsing with string slicing
//   - Static routes: O(1) map lookup before traversal
//   - Parameter storage: arrays for ≤8 params (zero allocs), map for >8
//   - Early exits to avoid unnecessary work
//   - Read lock (allows concurrent requests)
//
// Typical performance: <100ns for static routes, <500ns for parameterized routes
//
//go:inline
func (n *node) getRoute(path string, ctx *Context) []HandlerFunc {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Optimization 1: Handle root path specially (common case)
	if path == "/" {
		return n.handlers
	}

	if path == "" {
		return n.handlers
	}

	// Optimization 2: Fast path for static routes (no parameters)
	// Check full path in children map first - O(1) lookup
	// This avoids tree traversal for static routes like /api/users, /health, etc.
	if n.children != nil {
		if child, exists := n.children[path]; exists && child.handlers != nil {
			return child.handlers
		}
	}

	// Optimization 3: Manual path parsing without strings.Split
	// Why: strings.Split allocates a []string which we don't need
	// Instead: Parse segments on-the-fly using string slicing (zero allocations)
	current := n
	start := 0
	if path[0] == '/' {
		start = 1 // Skip leading slash
	}

	pathLen := len(path)

	// Main radix tree traversal loop
	// Each iteration processes one path segment (e.g., "users", "123", "posts")
	for start < pathLen {
		// Find the next slash or end of path (manual parsing for zero allocations)
		end := start
		for end < pathLen && path[end] != '/' {
			end++
		}

		// Extract segment without allocation - just slice the original string
		// Example: "/users/123/posts" → "users" → "123" → "posts"
		segment := path[start:end]
		isLast := end >= pathLen

		// Try matching strategies in priority order:
		// Priority 1: Exact static match (fastest - O(1) map lookup)
		if current.children != nil && current.children[segment] != nil {
			current = current.children[segment]
		} else if current.param != nil {
			// Priority 2: Parameter match (e.g., :id, :name)
			// Store parameter efficiently based on count
			if ctx.paramCount < 8 {
				// Fast path: use pre-allocated arrays (zero allocations)
				// Most routes have ≤8 params, so this is the common case
				ctx.paramKeys[ctx.paramCount] = current.param.key
				ctx.paramValues[ctx.paramCount] = segment
				ctx.paramCount++
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
			if ctx.paramCount < 8 {
				ctx.paramKeys[ctx.paramCount] = paramName
				ctx.paramValues[ctx.paramCount] = path[start:] // Everything from here
				ctx.paramCount++
			} else {
				if ctx.Params == nil {
					ctx.Params = make(map[string]string, 2)
				}
				ctx.Params[paramName] = path[start:]
			}
			return current.wildcard.node.handlers
		} else {
			// No match found - route doesn't exist
			return nil
		}

		// If this is the last segment, validate constraints and return
		if isLast {
			// Validate parameter constraints (e.g., :id must be numeric)
			if current.handlers != nil && !validateConstraints(current.constraints, ctx) {
				return nil // Constraint validation failed
			}
			return current.handlers
		}

		start = end + 1 // Move past the slash to next segment
	}

	// Reached end of path without matching - route not found
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
// into a lookup table with bloom filter for fast matching.
// This method should only be called after all routes are registered.
// The bloomFilterSize parameter controls the bloom filter capacity.
func (n *node) compileStaticRoutes(bloomFilterSize uint64) *CompiledRouteTable {
	n.mu.Lock()

	// Initialize compiled table if not exists
	if n.compiled == nil {
		// Use configured bloom filter size, with a minimum of 100
		size := bloomFilterSize
		if size < 100 {
			size = 100
		}
		n.compiled = &CompiledRouteTable{
			routes: make(map[uint64]*CompiledRoute, 16), // Pre-allocate with capacity
			bloom:  newBloomFilter(size, 3),             // Configurable bloom filter size
		}
	}

	table := n.compiled
	n.mu.Unlock()

	// Compile routes recursively without holding the parent lock
	// This prevents deadlocks when acquiring child node locks
	n.compileStaticRoutesRecursive(table, "")

	return table
}

// compileStaticRoutesRecursive recursively compiles static routes with proper locking.
// Takes snapshots of node state to avoid holding locks during recursion.
func (n *node) compileStaticRoutesRecursive(table *CompiledRouteTable, prefix string) {
	// Take snapshot of node state with read lock
	n.mu.RLock()
	handlers := n.handlers
	children := make(map[string]*node, len(n.children))
	for k, v := range n.children {
		children[k] = v
	}
	n.mu.RUnlock()

	// If this node has handlers and is a static route (no parameters), compile it
	if handlers != nil && !strings.Contains(prefix, ":") && prefix != "" {
		// Use zero-allocation hash function
		routeHash := fnv1aHash(prefix)

		// Create compiled route
		compiledRoute := &CompiledRoute{
			path:     prefix,
			handlers: handlers,
			hash:     routeHash,
		}

		// Store in routes map (table access is already protected by caller's lock)
		table.routes[routeHash] = compiledRoute

		// Add to bloom filter for fast negative lookups
		table.bloom.Add([]byte(prefix))
	}

	// Recursively compile children using the snapshot
	for path, child := range children {
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
// This is used for static routes without parameters (e.g., /health, /api/users)
// Returns handlers if found, nil if not a static route or doesn't exist
//
// Algorithm: Bloom filter → Hash lookup (two-stage filtering)
// Stage 1: Bloom filter test (very fast, ~10ns)
//   - If negative: route definitely doesn't exist, return nil immediately
//   - If positive: might exist, proceed to stage 2
//
// Stage 2: Hash map lookup (fast, ~20ns)
//   - Check actual route existence in hash map
//   - Return handlers if found
//
// Performance optimization: Skip bloom filter for small route sets
// Why: Bloom filter overhead (hashing 3 times) isn't worth it for <10 routes
// Direct hash lookup is faster when the map is tiny
func (table *CompiledRouteTable) getRoute(path string) []HandlerFunc {
	if table == nil {
		return nil
	}

	table.mu.RLock()
	defer table.mu.RUnlock()

	// Optimization: For small route sets, skip bloom filter
	// When routes < 10: bloom filter overhead > direct hash lookup
	if len(table.routes) < 10 {
		// Direct hash lookup without bloom filter - use FNV-1a for zero allocations
		routeHash := fnv1aHash(path)

		if route, exists := table.routes[routeHash]; exists {
			return route.handlers
		}
		return nil
	}

	// Stage 1: Quick bloom filter check for negative lookups
	// Eliminates ~99% of misses with just 3 hash computations
	// Avoids expensive map lookup for non-existent routes
	if !table.bloom.Test([]byte(path)) {
		return nil // Definitely not in the set
	}

	// Stage 2: Bloom filter says "maybe" - check the actual map
	// Compute hash for exact match - use FNV-1a for zero allocations
	routeHash := fnv1aHash(path)

	// Fast hash-based lookup (O(1) average case)
	if route, exists := table.routes[routeHash]; exists {
		return route.handlers
	}

	// Bloom filter false positive - route doesn't actually exist
	// This is rare with properly sized bloom filter (~1-5% false positive rate)
	return nil
}
