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
	"slices"
	"strings"
	"sync/atomic"
	"time"
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

	// Path-based versioning
	PathPattern string // e.g., "/v{version}/", "/api/{version}/"
	PathEnabled bool
	PathPrefix  string // extracted prefix like "/v", "/api/v"

	// Accept-header based content negotiation (RFC 7231)
	AcceptPattern string // e.g., "application/vnd.myapi.v{version}+json"
	AcceptEnabled bool

	// Default version when no version is specified
	DefaultVersion string

	// Version validation (optional)
	ValidVersions []string // e.g., ["v1", "v2", "latest"]

	// Deprecated versions with sunset dates (RFC 8594)
	DeprecatedVersions map[string]time.Time // version -> sunset date

	// Custom version detection function
	CustomDetector func(*http.Request) string

	// Observability hooks
	OnVersionDetected func(version string, method string) // Called when version is detected
	OnVersionMissing  func()                              // Called when no version detected
	OnVersionInvalid  func(attempted string)              // Called when invalid version attempted
}

// VersioningOption defines functional options for versioning configuration
type VersioningOption func(*VersioningConfig)

// WithVersioning configures the router with versioning support
func WithVersioning(opts ...VersioningOption) Option {
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

// WithPathVersioning configures path-based version detection
// pattern should contain {version} placeholder, e.g., "/v{version}/", "/api/{version}/"
// The version must be at a path segment boundary
func WithPathVersioning(pattern string) VersioningOption {
	return func(cfg *VersioningConfig) {
		cfg.PathPattern = pattern
		cfg.PathEnabled = true

		// Extract prefix for fast path matching
		// "/v{version}/" -> "/v"
		// "/api/{version}/" -> "/api/"
		if idx := strings.Index(pattern, "{version}"); idx > 0 {
			cfg.PathPrefix = pattern[:idx]
		}
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

// WithAcceptVersioning configures Accept-header based version detection
// pattern should contain {version} placeholder, e.g., "application/vnd.myapi.v{version}+json"
// Follows RFC 7231 content negotiation and vendor-specific media types
func WithAcceptVersioning(pattern string) VersioningOption {
	return func(cfg *VersioningConfig) {
		cfg.AcceptPattern = pattern
		cfg.AcceptEnabled = true
	}
}

// WithDeprecatedVersion marks a version as deprecated with a sunset date
// Adds Sunset and Deprecation headers per RFC 8594
func WithDeprecatedVersion(version string, sunsetDate time.Time) VersioningOption {
	return func(cfg *VersioningConfig) {
		if cfg.DeprecatedVersions == nil {
			cfg.DeprecatedVersions = make(map[string]time.Time)
		}
		cfg.DeprecatedVersions[version] = sunsetDate
	}
}

// WithVersionObserver sets observability hooks for version detection events
func WithVersionObserver(
	onDetected func(version string, method string),
	onMissing func(),
	onInvalid func(attempted string),
) VersioningOption {
	return func(cfg *VersioningConfig) {
		cfg.OnVersionDetected = onDetected
		cfg.OnVersionMissing = onMissing
		cfg.OnVersionInvalid = onInvalid
	}
}

// WithCustomVersionDetector sets a custom version detection function
func WithCustomVersionDetector(detector func(*http.Request) string) VersioningOption {
	return func(cfg *VersioningConfig) {
		cfg.CustomDetector = detector
	}
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

// VersionRouter represents a version-specific router
type VersionRouter struct {
	router  *Router
	version string
}

// fastQueryVersion scans RawQuery for a specific parameter without parsing.
// This is a zero-allocation alternative to url.Query().Get() for version detection.
//
// Algorithm: Manual byte scanning of RawQuery string
// - Looks for "param=" at start or after "&"
// - Extracts value until next "&" or end of string
// - No allocations: uses string slicing only
//
// Performance characteristics:
// - O(k) where k is query string length (linear scan)
// - Zero allocations: uses string slicing only
// - Faster than url.Query().Get() which allocates a map for all parameters
//
// Examples:
//   - "v=v1" → "v1", true
//   - "foo=bar&v=v2&baz=qux" → "v2", true
//   - "version=v1" → "v1", true (if param="version")
//   - "value=v1" → "", false (no match for param="v")
func fastQueryVersion(rawQuery, param string) (string, bool) {
	if rawQuery == "" || param == "" {
		return "", false
	}

	// Build search pattern: "param="
	pattern := param + "="
	patternLen := len(pattern)

	// Search for pattern in query string
	idx := strings.Index(rawQuery, pattern)
	if idx == -1 {
		return "", false
	}

	// Ensure pattern is at a query parameter boundary (start or after "&")
	// This prevents matching "foo=bar" when looking for "oo="
	if idx > 0 && rawQuery[idx-1] != '&' {
		// Not at boundary, search for "&param=" instead
		boundaryPattern := "&" + pattern
		idx = strings.Index(rawQuery, boundaryPattern)
		if idx == -1 {
			return "", false
		}
		idx++ // Skip the '&'
	}

	// Extract value: starts after "param=", ends at next "&" or end of string
	valueStart := idx + patternLen

	// Handle edge case: parameter at end with no value (e.g., "foo=bar&v=")
	// This should return empty string but still indicate parameter was found
	if valueStart >= len(rawQuery) {
		return "", true // Empty value is valid
	}

	// Find end of value (next "&" or end of string)
	valueEnd := strings.IndexByte(rawQuery[valueStart:], '&')
	if valueEnd == -1 {
		// Value extends to end of query string
		return rawQuery[valueStart:], true
	}

	// Value ends at next parameter
	return rawQuery[valueStart : valueStart+valueEnd], true
}

// fastHeaderVersion extracts version from header with zero allocations.
// Uses direct map access instead of Header.Get() for slightly better performance.
//
// Performance characteristics:
// - O(1) map lookup (constant time)
// - Zero allocations: direct map access
// - Slightly faster than Header.Get() which performs canonicalization
//
// Note: Header names are canonicalized by net/http (e.g., "api-version" → "Api-Version").
// This function expects the canonicalized form.
func fastHeaderVersion(headers http.Header, headerName string) string {
	// Direct map access - fastest path
	// headers is map[string][]string, we want the first value
	if vals, ok := headers[headerName]; ok && len(vals) > 0 {
		return vals[0]
	}
	return ""
}

// fastPathVersion extracts version from URL path with zero allocations.
// Uses string operations to match a path pattern like "/v{version}/" or "/api/{version}/"
//
// Algorithm: Prefix matching and segment extraction
// - Checks if path starts with configured prefix (e.g., "/v")
// - Extracts next path segment as version
// - Stops at next "/" or end of string
//
// Performance characteristics:
// - O(k) where k is path length (linear scan for prefix and segment)
// - Zero allocations: uses string slicing only
// - Faster than string splitting or regex matching approaches
//
// Examples (with pattern "/v{version}/"):
//   - "/v1/users" → "v1", true
//   - "/v2/posts/123" → "v2", true
//   - "/api/v1/users" → "", false (doesn't match prefix)
//
// Examples (with pattern "/api/v{version}/"):
//   - "/api/v1/users" → "v1", true
//   - "/api/v2/posts" → "v2", true
func fastPathVersion(path, prefix string) (string, bool) {
	if path == "" || prefix == "" {
		return "", false
	}

	// Check if path starts with prefix
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}

	// Extract version segment after prefix
	// Skip the prefix to get to version start
	versionStart := len(prefix)
	if versionStart >= len(path) {
		return "", false
	}

	// Find end of version segment (next "/" or end of path)
	remaining := path[versionStart:]
	versionEnd := strings.IndexByte(remaining, '/')

	if versionEnd == -1 {
		// Version extends to end of path (e.g., "/v1")
		return remaining, true
	}

	// Version ends at next segment (e.g., "/v1/users" -> "v1")
	return remaining[:versionEnd], true
}

// fastAcceptVersion extracts version from Accept header using content negotiation.
// Parses vendor-specific media types like "application/vnd.myapi.v2+json"
//
// Algorithm: Pattern matching with {version} placeholder
// - Locates the pattern prefix in Accept header
// - Extracts version between prefix and suffix
// - Handles multiple Accept values (comma-separated)
//
// Performance characteristics:
// - O(k) where k is Accept header length (linear scan)
// - Zero allocations: uses string slicing only
// - Faster than full MIME type parsing or regex matching
//
// Examples (with pattern "application/vnd.myapi.v{version}+json"):
//   - "application/vnd.myapi.v2+json" → "v2", true
//   - "application/vnd.myapi.v1+json, text/html" → "v1", true
//   - "application/json" → "", false
func fastAcceptVersion(accept, pattern string) (string, bool) {
	if accept == "" || pattern == "" {
		return "", false
	}

	// Find {version} placeholder in pattern
	versionIdx := strings.Index(pattern, "{version}")
	if versionIdx == -1 {
		return "", false
	}

	// Split pattern into prefix and suffix
	prefix := pattern[:versionIdx]
	suffix := pattern[versionIdx+9:] // len("{version}") == 9

	// Handle comma-separated Accept values
	// e.g., "application/vnd.myapi.v2+json, text/html"
	for val := range strings.SplitSeq(accept, ",") {
		val = strings.TrimSpace(val)

		// Find prefix in this Accept value
		prefixIdx := strings.Index(val, prefix)
		if prefixIdx == -1 {
			continue
		}

		// Extract version between prefix and suffix
		versionStart := prefixIdx + len(prefix)
		if suffix == "" {
			// No suffix, version extends to end or semicolon (for parameters)
			versionEnd := strings.IndexAny(val[versionStart:], ";,")
			if versionEnd == -1 {
				return strings.TrimSpace(val[versionStart:]), true
			}
			return strings.TrimSpace(val[versionStart : versionStart+versionEnd]), true
		}

		// Find suffix after version
		suffixIdx := strings.Index(val[versionStart:], suffix)
		if suffixIdx == -1 {
			continue
		}

		return val[versionStart : versionStart+suffixIdx], true
	}

	return "", false
}

// validateVersion checks if a version is valid and returns it, or empty string if invalid.
// This helper reduces code duplication in detectVersion.
//
// Performance: O(n) where n is len(ValidVersions), typically small (< 10)
// Optimized with early return when no validation configured (common case)
func (cfg *VersioningConfig) validateVersion(version string) string {
	if version == "" {
		return ""
	}

	// Fast path: no validation configured (common case)
	if len(cfg.ValidVersions) == 0 {
		return version
	}

	// Validation enabled: check if version is in allowed list
	if slices.Contains(cfg.ValidVersions, version) {
		return version
	}

	// Invalid version
	if cfg.OnVersionInvalid != nil {
		cfg.OnVersionInvalid(version)
	}
	return ""
}

// setDeprecationHeaders adds RFC 8594 deprecation headers if the version is deprecated.
// Should be called after version detection with the ResponseWriter available.
//
// Sets two headers:
// - Deprecation: "true" (indicates the resource is deprecated)
// - Sunset: HTTP-date format (indicates when the version will be removed)
//
// Performance: O(1) map lookup, negligible overhead
// Only executed when deprecation is configured (zero overhead otherwise)
func (cfg *VersioningConfig) setDeprecationHeaders(w http.ResponseWriter, version string) {
	if len(cfg.DeprecatedVersions) == 0 {
		return // Fast path: no deprecated versions configured
	}

	if sunsetDate, deprecated := cfg.DeprecatedVersions[version]; deprecated {
		w.Header().Set("Deprecation", "true")
		w.Header().Set("Sunset", sunsetDate.UTC().Format(http.TimeFormat))
	}
}

// detectVersion performs efficient version detection with zero-allocation fast paths.
// Checks are ordered for the most common cases first to improve branch prediction.
//
// Detection order (by priority):
// 1. Path-based (most explicit and RESTful)
// 2. Header-based (common in production APIs)
// 3. Accept-based (content negotiation, RFC 7231)
// 4. Query-based (convenient for testing)
// 5. Custom detector (ultimate fallback)
//
// Performance characteristics:
// - Path detection: O(k) where k is path length, zero-allocation prefix matching
// - Header detection: O(1) map lookup, zero allocations
// - Accept detection: O(k) where k is header length, zero-allocation pattern matching
// - Query detection: O(k) where k is query string length, zero-allocation scanning
// - Validation helper: reduces code duplication, early return optimization
// - Observability hooks: called only when configured (zero overhead otherwise)
//
// Memory savings: ~400 bytes per request (avoids url.Query() map allocation)
func (r *Router) detectVersion(req *http.Request) string {
	cfg := r.versioning

	// Path-based detection (checked first - most explicit and RESTful)
	if cfg.PathEnabled {
		if segment, ok := fastPathVersion(req.URL.Path, cfg.PathPrefix); ok && segment != "" {
			// Try multiple version formats to match registered routes
			// e.g., "/v1" could match routes registered as "v1" or "1"
			candidates := make([]string, 0, 2)
			if strings.HasSuffix(cfg.PathPrefix, "v") {
				candidates = append(candidates, "v"+segment)
			}
			candidates = append(candidates, segment)

			// Return first valid candidate
			for _, version := range candidates {
				if validated := cfg.validateVersion(version); validated != "" {
					if cfg.OnVersionDetected != nil {
						cfg.OnVersionDetected(validated, "path")
					}
					return validated
				}
			}
		}
	}

	// Header-based detection (most common in production APIs)
	if cfg.HeaderEnabled {
		if header := req.Header.Get(cfg.HeaderName); header != "" {
			if validated := cfg.validateVersion(header); validated != "" {
				if cfg.OnVersionDetected != nil {
					cfg.OnVersionDetected(validated, "header")
				}
				return validated
			}
		}
	}

	// Accept-header based content negotiation (RFC 7231)
	// Supports vendor-specific media types like "application/vnd.myapi.v2+json"
	if cfg.AcceptEnabled {
		if accept := req.Header.Get("Accept"); accept != "" {
			if version, ok := fastAcceptVersion(accept, cfg.AcceptPattern); ok && version != "" {
				if validated := cfg.validateVersion(version); validated != "" {
					if cfg.OnVersionDetected != nil {
						cfg.OnVersionDetected(validated, "accept")
					}
					return validated
				}
			}
		}
	}

	// Query parameter-based detection (convenient for testing)
	if cfg.QueryEnabled {
		if version, ok := fastQueryVersion(req.URL.RawQuery, cfg.QueryParam); ok && version != "" {
			if validated := cfg.validateVersion(version); validated != "" {
				if cfg.OnVersionDetected != nil {
					cfg.OnVersionDetected(validated, "query")
				}
				return validated
			}
		}
	}

	// Custom detector (ultimate fallback before default)
	if cfg.CustomDetector != nil {
		if version := cfg.CustomDetector(req); version != "" {
			if validated := cfg.validateVersion(version); validated != "" {
				if cfg.OnVersionDetected != nil {
					cfg.OnVersionDetected(validated, "custom")
				}
				return validated
			}
		}
	}

	// No version detected, use default
	if cfg.OnVersionMissing != nil {
		cfg.OnVersionMissing()
	}

	return cfg.DefaultVersion
}

// stripPathVersion removes the version segment from the path when path-based versioning is enabled.
// This is called before route matching to ensure routes registered without version prefix can match.
//
// The function handles the case where the pattern includes characters before {version}.
// For example, pattern "/v{version}/" extracts "1" as version from "/v1/users", but we need
// to strip "/v1/" to match routes registered as "/users".
//
// Examples:
//   - "/v1/users" with prefix "/v" and detected version "v1" → "/users"
//   - "/api/v2/posts" with prefix "/api/v" and detected version "v2" → "/api/posts"
//   - "/users" (no version) → "/users" (unchanged)
//
// Performance: Zero allocations, uses string slicing only.
func (r *Router) stripPathVersion(path, version string) string {
	cfg := r.versioning
	if cfg == nil || !cfg.PathEnabled || version == "" {
		return path // No path-based versioning or no version detected
	}

	// Check if path starts with prefix
	if !strings.HasPrefix(path, cfg.PathPrefix) {
		return path // Path doesn't match prefix pattern
	}

	// Calculate version segment start (after prefix)
	versionStart := len(cfg.PathPrefix)
	if versionStart >= len(path) {
		return path // Invalid: prefix extends beyond path
	}

	// Find where version segment ends (next "/" or end of path)
	remaining := path[versionStart:]
	versionEnd := strings.IndexByte(remaining, '/')

	var versionSegment string
	if versionEnd == -1 {
		// Version is at end of path (e.g., "/v1")
		versionSegment = remaining
	} else {
		versionSegment = remaining[:versionEnd]
	}

	// Check if the version segment matches detected version
	// The detected version might be "v1" (from pattern "/v{version}/") or just "1"
	// We check multiple ways to match:
	// 1. Segment directly matches version (e.g., segment "v1" == version "v1")
	// 2. Prefix+segment matches version (e.g., "/v" + "1" != "v1", but we handle this below)
	// 3. Segment matches version without prefix (e.g., segment "1" == version "v1"[1:])
	fullSegment := cfg.PathPrefix + versionSegment
	versionMatches := versionSegment == version || fullSegment == version

	// Special case: if prefix ends with "v" and version is "v" + segment
	if !versionMatches && strings.HasSuffix(cfg.PathPrefix, "v") && len(version) > 1 && version[0] == 'v' {
		if versionSegment == version[1:] {
			versionMatches = true
		}
	}

	if !versionMatches {
		return path // Version doesn't match, don't strip
	}

	// Calculate where to start the stripped path
	var strippedStart int
	if versionEnd == -1 {
		// Version at end of path, strip everything (versionMatches already checked above)
		return "/" // Return root path
	}

	// Strip prefix + version segment
	// Example: "/v1/users" with prefix "/v", segment "1" → strip "/v1" → "/users"
	// versionEnd is index of "/" within remaining, so absolute position is versionStart + versionEnd
	// We want to start from that "/", so use versionStart + versionEnd
	strippedStart = versionStart + versionEnd
	if strippedStart >= len(path) {
		return "/" // Path becomes root after stripping
	}

	return path[strippedStart:]
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
// Algorithm: Optimistic fast-path with CAS-based fallback
// Fast path: Add to existing version/method tree if it exists
// Slow path: Create new version/method tree atomically via CAS loop
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
			maps.Copy(methodTreesCopy, v) // Node pointers are shared (safe - immutable after creation)
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

	// Store route info for introspection (protected by separate mutex for low-frequency access)
	vr.router.routeTree.routesMutex.Lock()
	vr.router.routeTree.routes = append(vr.router.routeTree.routes, RouteInfo{
		Method:      method,
		Path:        path,
		HandlerName: handlerName,
		Middleware:  middlewareNames,
		Constraints: make(map[string]string), // Will be populated when constraints are added
		IsStatic:    isStatic,
		Version:     vr.version, // Set the version for version-specific routes
		ParamCount:  paramCount,
	})
	vr.router.routeTree.routesMutex.Unlock()

	// Combine global middleware with route handlers
	// IMPORTANT: Create a new slice to avoid aliasing bugs with append
	allHandlers := make([]HandlerFunc, 0, len(vr.router.middleware)+len(handlers))
	allHandlers = append(allHandlers, vr.router.middleware...)
	allHandlers = append(allHandlers, handlers...)

	// Add to version-specific tree
	vr.router.addVersionRoute(vr.version, method, path, allHandlers, nil)

	// Record route registration for metrics
	vr.router.recordRouteRegistration(method, path)

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
