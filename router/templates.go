package router

import (
	"regexp"
	"strings"
	"sync"
)

// RouteTemplate represents a pre-compiled route with metadata for fast matching.
// This eliminates the need for tree traversal on every request by pre-computing
// all route structure information during registration.
//
// Performance benefits:
//   - Single-pass path parsing instead of segment-by-segment traversal
//   - Direct position-based parameter extraction (no searching)
//   - Inline constraint validation (no separate validation pass)
//   - Stack-allocated segment buffer (zero heap allocations)
//
// Expected improvement: Significantly faster than tree traversal for parameter routes
type RouteTemplate struct {
	// Route identification
	method  string // HTTP method (GET, POST, etc.)
	pattern string // Original pattern (/users/:id/posts/:pid)
	hash    uint64 // Pre-computed hash for quick lookup

	// Route structure (pre-computed during registration)
	segmentCount   int32    // Total number of segments
	staticSegments []string // Static segments that must match exactly
	staticPos      []int32  // Positions of static segments
	paramNames     []string // Parameter names in extraction order
	paramPos       []int32  // Positions where parameters are extracted

	// Validation (pre-compiled)
	constraints []*regexp.Regexp // Constraints indexed by parameter position

	// Handler chain
	handlers []HandlerFunc

	// Optimization flags
	isStatic       bool // True if route has no parameters
	hasWildcard    bool // True if route has wildcard
	hasConstraints bool // True if route has parameter constraints
}

const (
	// minTemplatesForIndexing is the minimum number of dynamic templates required
	// before building the first-segment index for faster filtering.
	// Below this threshold, the overhead of maintaining the index outweighs the benefits.
	//
	// Rationale:
	//   - With ≤10 templates: Linear scan O(n) is acceptable (n is small)
	//   - Index build cost: O(n) + memory overhead for index structure
	//   - Index lookup benefit: O(1) vs O(n) when >10 templates
	//
	// This threshold ensures we only build the index when it provides a clear advantage.
	minTemplatesForIndexing = 10
)

// TemplateCache manages compiled route templates for fast lookup.
// Uses a three-tier strategy:
//  1. Static routes: Hash table with O(1) complexity
//  2. Dynamic routes: Template matching with O(k) complexity
//  3. Complex routes: Tree fallback with O(k*log(n)) complexity
//
// Enhancement: First-segment jump table for faster filtering
type TemplateCache struct {
	// Static route table: method+path → handlers (fastest path)
	staticRoutes map[uint64]*RouteTemplate
	staticBloom  *bloomFilter

	// Dynamic route templates: ordered by specificity
	dynamicTemplates []*RouteTemplate

	// First-segment index for fast filtering (ASCII-only optimization)
	// Maps first character after '/' to templates that start with that char
	// Example: 'u' → ["/users/:id", "/user/profile"]
	// This reduces search space by ~95% for typical APIs
	//
	// DESIGN DECISION: ASCII-only (0-127) for performance-first architecture.
	// - UTF-8 paths beyond ASCII fall back to linear scan (still correct, just slower)
	// - Extending to Latin-1 (256) or full UTF-8 would add complexity/memory without
	//   measurable benefit for typical HTTP APIs (99%+ use ASCII paths)
	// - This aligns with our performance-first philosophy: optimize for common case
	firstSegmentIndex    [128][]*RouteTemplate // ASCII quick lookup (0-127)
	hasFirstSegmentIndex bool                  // Whether index is built

	// Mutex protects template cache during updates
	mu sync.RWMutex
}

// newTemplateCache creates a new template cache
func newTemplateCache(bloomSize uint64, numHashFuncs int) *TemplateCache {
	return &TemplateCache{
		staticRoutes:     make(map[uint64]*RouteTemplate, 64),
		dynamicTemplates: make([]*RouteTemplate, 0, 32),
		staticBloom:      newBloomFilter(bloomSize, numHashFuncs),
	}
}

// compileRouteTemplate compiles a route pattern into a template for fast matching.
// This pre-computes all structure information during registration to avoid
// work during request handling.
func compileRouteTemplate(method, pattern string, handlers []HandlerFunc, constraints []RouteConstraint) *RouteTemplate {
	// Normalize pattern
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		pattern = "/"
	}

	tmpl := &RouteTemplate{
		method:   method,
		pattern:  pattern,
		handlers: handlers,
		hash:     fnv1aHash(method + pattern),
	}

	// Handle root path
	if pattern == "/" {
		tmpl.isStatic = true
		tmpl.segmentCount = 0
		return tmpl
	}

	// Parse pattern into segments
	segments := strings.Split(strings.Trim(pattern, "/"), "/")
	tmpl.segmentCount = int32(len(segments))

	// Check for wildcard
	if len(segments) > 0 && strings.HasSuffix(segments[len(segments)-1], "*") {
		tmpl.hasWildcard = true
		// Wildcard routes use tree fallback
		return tmpl
	}

	// Pre-allocate slices with known capacity
	staticSegs := make([]string, 0, len(segments))
	staticPositions := make([]int32, 0, len(segments))
	paramNames := make([]string, 0, len(segments)/2)
	paramPositions := make([]int32, 0, len(segments)/2)
	constraintsList := make([]*regexp.Regexp, 0, len(segments)/2)

	// Analyze each segment
	for i, seg := range segments {
		if strings.HasPrefix(seg, ":") {
			// Parameter segment
			paramName := seg[1:]
			paramNames = append(paramNames, paramName)
			paramPositions = append(paramPositions, int32(i))

			// Find matching constraint
			var constraint *regexp.Regexp
			for _, c := range constraints {
				if c.Param == paramName {
					constraint = c.Pattern
					tmpl.hasConstraints = true
					break
				}
			}
			constraintsList = append(constraintsList, constraint)
		} else {
			// Static segment
			staticSegs = append(staticSegs, seg)
			staticPositions = append(staticPositions, int32(i))
		}
	}

	// Store compiled metadata
	tmpl.staticSegments = staticSegs
	tmpl.staticPos = staticPositions
	tmpl.paramNames = paramNames
	tmpl.paramPos = paramPositions
	tmpl.constraints = constraintsList

	// Mark as static if no parameters
	tmpl.isStatic = len(paramNames) == 0

	return tmpl
}

// matchAndExtract attempts to match the path against this template.
// Returns true if matched and extracts parameters into context.
//
// Algorithm: Single-pass validation and extraction
// 1. Quick rejection by segment count
// 2. Parse path into stack-allocated array (zero heap allocation)
// 3. Validate static segments by direct position check
// 4. Extract parameters by position with inline constraint validation
//
// matchAndExtract attempts to match a path against this template and extract parameters.
// Uses early exit optimizations and fast paths for common patterns.
func (t *RouteTemplate) matchAndExtract(path string, ctx *Context) bool {
	// Handle root path specially (unlikely in most APIs)
	if t.segmentCount == 0 {
		return path == "/" || path == ""
	}

	// Fast path for common single-parameter routes
	// Pattern: /resource/:id (2 segments, 1 param at position 1)
	// This is the most common REST API pattern
	if t.segmentCount == 2 && len(t.paramPos) == 1 && t.paramPos[0] == 1 {
		// Fast path: /users/123, /posts/456, etc.
		// No need for full parsing - just extract after first /
		if len(path) < 3 || path[0] != '/' {
			return false // Too short or invalid
		}

		// Find first segment boundary
		firstSlash := -1
		for i := 1; i < len(path); i++ {
			if path[i] == '/' {
				firstSlash = i
				break
			}
		}

		if firstSlash == -1 {
			return false // No second segment
		}

		// Check if there's a third segment (shouldn't be)
		for i := firstSlash + 1; i < len(path); i++ {
			if path[i] == '/' {
				return false // Too many segments
			}
		}

		// Validate static segment
		firstSeg := path[1:firstSlash]
		if len(t.staticSegments) > 0 && firstSeg != t.staticSegments[0] {
			return false
		}

		// Extract parameter (second segment)
		paramValue := path[firstSlash+1:]

		// Inline constraint check
		if len(t.constraints) > 0 && t.constraints[0] != nil {
			if !t.constraints[0].MatchString(paramValue) {
				return false
			}
		}

		// Store in context
		ctx.paramKeys[0] = t.paramNames[0]
		ctx.paramValues[0] = paramValue
		ctx.paramCount = 1

		return true
	}

	pathLen := len(path)

	// Quick length check for early exit
	// If path is too short, it can't possibly match
	// Minimum length = segmentCount + (segmentCount - 1) slashes
	minLen := int(t.segmentCount) + int(t.segmentCount-1)
	if pathLen < minLen {
		return false
	}

	// Count slashes with early exit
	// Check segment count matches while counting
	slashCount := int32(0)
	for i := range pathLen {
		if path[i] == '/' {
			slashCount++
			// Early exit if too many slashes
			if slashCount > t.segmentCount {
				return false
			}
		}
	}

	// Exact slash count validation
	expectedSlashes := t.segmentCount
	if pathLen == 0 || path[0] != '/' {
		expectedSlashes-- // Pattern has leading / but path doesn't
	}

	if slashCount != expectedSlashes {
		return false
	}

	// Stack-allocated segment buffer (no heap allocation!)
	// 16 segments should be enough for any reasonable route
	var segments [16]string
	segCount := int32(0)

	// Single-pass path parsing
	start := 0
	if len(path) > 0 && path[0] == '/' {
		start = 1 // Skip leading slash
	}

	for start < pathLen && segCount < 16 {
		end := start
		for end < pathLen && path[end] != '/' {
			end++
		}
		segments[segCount] = path[start:end]
		segCount++
		start = end + 1
	}

	// Validate segment count matches
	if segCount != t.segmentCount {
		return false
	}

	// Validate static segments with early exit
	// Check most distinctive segments first for faster rejection
	staticCount := int32(len(t.staticPos))
	if staticCount > 0 {
		// Unroll first check (most common case)
		pos0 := t.staticPos[0]
		if pos0 >= segCount || segments[pos0] != t.staticSegments[0] {
			return false
		}

		// Check remaining static segments
		for i := int32(1); i < staticCount; i++ {
			pos := t.staticPos[i]
			if pos >= segCount || segments[pos] != t.staticSegments[i] {
				return false
			}
		}
	}

	// Extract and validate parameters (by position - no search!)
	paramCount := int32(len(t.paramPos))
	for i := range int(paramCount) {
		pos := t.paramPos[i]
		if pos >= segCount {
			return false
		}

		value := segments[pos]

		// Inline constraint validation (if constraint exists)
		if int32(i) < int32(len(t.constraints)) && t.constraints[int32(i)] != nil {
			if !t.constraints[int32(i)].MatchString(value) {
				return false
			}
		}

		// Store parameter in context (we already know the index!)
		if i < 8 {
			ctx.paramKeys[i] = t.paramNames[i]
			ctx.paramValues[i] = value
		} else {
			// Rare case: >8 parameters
			if ctx.Params == nil {
				ctx.Params = make(map[string]string, 2)
			}
			ctx.Params[t.paramNames[i]] = value
		}
	}

	ctx.paramCount = paramCount
	return true
}

// removeTemplate removes a template from the cache (used when updating constraints)
func (tc *TemplateCache) removeTemplate(method, pattern string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Calculate hash
	hash := fnv1aHash(method + pattern)

	// Remove from static routes
	delete(tc.staticRoutes, hash)

	// Remove from dynamic templates
	for i, tmpl := range tc.dynamicTemplates {
		if tmpl.method == method && tmpl.pattern == pattern {
			// Remove by swapping with last element and slicing
			tc.dynamicTemplates[i] = tc.dynamicTemplates[len(tc.dynamicTemplates)-1]
			tc.dynamicTemplates = tc.dynamicTemplates[:len(tc.dynamicTemplates)-1]
			tc.hasFirstSegmentIndex = false
			break
		}
	}
}

// addTemplate adds a template to the cache
func (tc *TemplateCache) addTemplate(tmpl *RouteTemplate) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tmpl.isStatic {
		// Add to static table
		tc.staticRoutes[tmpl.hash] = tmpl
		tc.staticBloom.Add([]byte(tmpl.method + tmpl.pattern))
	} else if !tmpl.hasWildcard {
		// Add to dynamic templates (sorted by specificity)
		tc.dynamicTemplates = append(tc.dynamicTemplates, tmpl)

		// Sort by specificity (more static segments = higher priority)
		// This ensures more specific routes match first
		tc.sortTemplatesBySpecificity()

		// Invalidate first-segment index (will be rebuilt on next lookup)
		tc.hasFirstSegmentIndex = false
	}
	// Wildcard routes fall back to tree
}

// sortTemplatesBySpecificity sorts templates by specificity (most specific first)
// Specificity = number of static segments (more static = more specific)
func (tc *TemplateCache) sortTemplatesBySpecificity() {
	// Sort in place for efficiency
	templates := tc.dynamicTemplates

	// Simple insertion sort (templates array is small, typically < 100)
	for i := 1; i < len(templates); i++ {
		key := templates[i]
		keySpecificity := len(key.staticSegments)

		j := i - 1
		for j >= 0 && len(templates[j].staticSegments) < keySpecificity {
			templates[j+1] = templates[j]
			j--
		}
		templates[j+1] = key
	}
}

// lookupStatic attempts to find a static route in the hash table
func (tc *TemplateCache) lookupStatic(method, path string) *RouteTemplate {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	// Bloom filter check first (fast negative lookup)
	key := method + path
	if !tc.staticBloom.Test([]byte(key)) {
		return nil // Definitely not present
	}

	// Hash lookup
	hash := fnv1aHash(key)
	return tc.staticRoutes[hash]
}

// matchDynamic attempts to match path against dynamic templates.
// Uses first-segment index for faster filtering.
func (tc *TemplateCache) matchDynamic(method, path string, ctx *Context) *RouteTemplate {
	tc.mu.RLock()

	// Build first-segment index if not exists (lazy initialization)
	if !tc.hasFirstSegmentIndex && len(tc.dynamicTemplates) > minTemplatesForIndexing {
		// Only build index if we have enough templates to benefit
		tc.mu.RUnlock()
		tc.buildFirstSegmentIndex()
		tc.mu.RLock()
	}

	// Try first-segment index for fast filtering
	if tc.hasFirstSegmentIndex && len(path) > 1 {
		// Extract first character after '/'
		firstChar := path[1]
		if firstChar < 128 {
			// ASCII fast path - check jump table
			// Note: Non-ASCII paths (UTF-8 beyond byte 127) skip this optimization
			// and fall back to linear scan. This is intentional - see struct comment.
			candidates := tc.firstSegmentIndex[firstChar]
			for _, tmpl := range candidates {
				// Check method before matching path
				if tmpl.method == method && tmpl.matchAndExtract(path, ctx) {
					tc.mu.RUnlock()
					return tmpl
				}
			}
			tc.mu.RUnlock()
			return nil
		}
	}

	// Fallback: Try each template in order (sorted by specificity)
	for _, tmpl := range tc.dynamicTemplates {
		// Check method before matching path
		if tmpl.method == method && tmpl.matchAndExtract(path, ctx) {
			tc.mu.RUnlock()
			return tmpl
		}
	}

	tc.mu.RUnlock()
	return nil
}

// buildFirstSegmentIndex builds the first-segment jump table for faster template filtering.
// This reduces the search space significantly for typical APIs.
//
// Instead of checking all templates, only check those that start with
// the same character as the path.
//
// Example: Path "/users/123" → Only check templates starting with 'u'
//
// DESIGN DECISION: Only indexes ASCII characters (0-127) for performance.
// UTF-8 paths beyond ASCII are still matched correctly via fallback linear scan.
// Extending to full UTF-8 would add complexity without measurable benefit for
// typical HTTP APIs where ASCII paths dominate.
func (tc *TemplateCache) buildFirstSegmentIndex() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Reset index
	for i := range tc.firstSegmentIndex {
		tc.firstSegmentIndex[i] = nil
	}

	// Build index from templates
	for _, tmpl := range tc.dynamicTemplates {
		// Get first character of pattern after '/'
		pattern := tmpl.pattern
		if len(pattern) > 1 && pattern[0] == '/' {
			firstChar := pattern[1]
			if firstChar < 128 {
				// Add to index
				tc.firstSegmentIndex[firstChar] = append(tc.firstSegmentIndex[firstChar], tmpl)
			}
		}
	}

	tc.hasFirstSegmentIndex = true
}
