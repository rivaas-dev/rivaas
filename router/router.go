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
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"rivaas.dev/router/compiler"
	"rivaas.dev/router/route"
	"rivaas.dev/router/version"
)

// noopLogger is a singleton no-op logger used when no observability is configured.
var noopLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// NoopLogger returns the singleton no-op logger.
// This is used by implementations of ObservabilityRecorder when logging is disabled.
func NoopLogger() *slog.Logger {
	return noopLogger
}

// Option defines functional options for router configuration.
type Option func(*Router)

// responseWriter wraps http.ResponseWriter to capture status code and size.
// It also prevents "superfluous response.WriteHeader call" errors
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int64
	written    bool
}

// WriteHeader captures the status code and prevents duplicate calls.
func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
		rw.written = true
	}
}

// Write captures the response size and marks as written.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
	}
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += int64(n)

	return n, err
}

// StatusCode returns the HTTP status code.
func (rw *responseWriter) StatusCode() int {
	if rw.statusCode == 0 {
		return http.StatusOK
	}

	return rw.statusCode
}

// Size returns the response size in bytes.
func (rw *responseWriter) Size() int64 {
	return rw.size
}

// Written returns true if headers have been written.
func (rw *responseWriter) Written() bool {
	return rw.written
}

// Compile-time check that responseWriter implements ResponseInfo.
var _ ResponseInfo = (*responseWriter)(nil)

// Hijack implements http.Hijacker interface.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}

	return nil, nil, ErrResponseWriterNotHijacker
}

// Flush implements http.Flusher interface.
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Router represents the HTTP router.
// It matches HTTP requests to registered routes and executes handler chains.
//
// Key features:
//   - Path matching for static and parameterized routes
//   - Context pooling to reuse contexts across requests
//   - Optional OpenTelemetry tracing (when enabled)
//   - Middleware chain execution
//   - Route tables for static route matching
//   - Routing with wildcard support and route versioning
//
// The Router is safe for concurrent use and can handle multiple goroutines
// accessing it simultaneously without any additional synchronization.
//
// Example:
//
//	r := router.MustNew()
//	r.GET("/users/:id", func(c *router.Context) {
//	    userID := c.Param("id")
//	    c.JSON(http.StatusOK, map[string]string{"id": userID})
//	})
//	http.ListenAndServe(":8080", r)
type Router struct {
	routeTree     atomicRouteTree       // Route tree with atomic operations
	middleware    []HandlerFunc         // Global middleware chain applied to all routes
	middlewareMu  sync.RWMutex          // Protects middleware slice
	observability ObservabilityRecorder // Unified observability (metrics, tracing, logging)
	diagnostics   DiagnosticHandler     // Optional diagnostic event handler

	// Deferred route registration
	pendingRoutes   []*route.Route // Routes waiting to be registered during Warmup
	pendingRoutesMu sync.Mutex     // Protects pendingRoutes slice and warmedUp flag
	warmupOnce      sync.Once      // Ensures warmup runs exactly once
	warmedUp        bool           // True after Warmup has completed

	// Routing features
	versionEngine *version.Engine    // API versioning engine for version detection
	versionOpts   []version.Option   // Pending versioning options (validated during validate())
	versionTrees  atomicVersionTrees // Version-specific route trees
	versionCache  sync.Map           // Version-specific compiled routes

	// Configuration
	bloomFilterSize    uint64 // Size of bloom filters for compiled routes (default: 1000)
	bloomHashFunctions int    // Number of hash functions for bloom filters (default: 3)
	checkCancellation  bool   // Enable context cancellation checks in Next() (default: true)

	// Route compilation
	routeCompiler     *compiler.RouteCompiler // Pre-compiled routes for matching
	useCompiledRoutes bool                    // Enable compiled route matching (default: true)

	// Custom 404 handler
	noRouteHandler HandlerFunc  // Custom handler for unmatched routes (nil means use http.NotFound)
	noRouteMutex   sync.RWMutex // Protects noRouteHandler (rarely written, frequently read)

	// HTTP/2 Cleartext (H2C) support
	enableH2C      bool            // Enable HTTP/2 cleartext support (dev/behind LB only)
	serverTimeouts *serverTimeouts // HTTP server timeout configuration

	// Trusted proxies configuration for real client IP detection
	realip *realIPConfig // Compiled trusted proxy configuration

	// Route freezing and naming
	frozen             atomic.Bool             // Routes are frozen (immutable) after freeze
	namedRoutes        map[string]*route.Route // name -> route mapping
	routeSnapshot      []*route.Route          // Immutable snapshot built at freeze time
	routeSnapshotMutex sync.RWMutex            // Protects routeSnapshot
}

// serverTimeouts holds HTTP server timeout configuration.
type serverTimeouts struct {
	readHeader time.Duration
	read       time.Duration
	write      time.Duration
	idle       time.Duration
}

const (
	// defaultBloomFilterSize is the default size of bloom filters for compiled routes.
	// The bloom filter is used to determine if a route might exist before
	// performing a full lookup. Default of 1000 bits provides acceptable false
	// positive rate for typical route sets.
	defaultBloomFilterSize = 1000

	// defaultBloomHashFunctions is the default number of hash functions for bloom filters.
	// Three hash functions are used to set bits in the bloom filter.
	defaultBloomHashFunctions = 3
)

// New creates a new router instance with optional configuration.
// It initializes the route trees for HTTP methods and sets up context pooling
// to reuse contexts during request handling.
//
// The returned router is ready to use and is safe for concurrent access.
//
// Returns an error if the router configuration is invalid. Configuration
// is validated immediately at startup rather than at runtime.
//
// For a version that panics instead of returning an error, use MustNew.
//
// Example:
//
//	r, err := router.New()
//	if err != nil {
//	    log.Fatalf("Failed to create router: %v", err)
//	}
//	r.GET("/health", healthHandler)
//	http.ListenAndServe(":8080", r)
//
// With options:
//
//	r, err := router.New(
//	    router.WithH2C(true),
//	    router.WithServerTimeouts(10*time.Second, 30*time.Second, 60*time.Second, 120*time.Second),
//	)
//	if err != nil {
//	    log.Fatalf("Invalid router configuration: %v", err)
//	}
//	r.GET("/api/users", getUserHandler)
//	http.ListenAndServe(":8080", r)
func New(opts ...Option) (*Router, error) {
	r := &Router{
		bloomFilterSize:    defaultBloomFilterSize,
		bloomHashFunctions: defaultBloomHashFunctions,
		checkCancellation:  true, // Enable cancellation checks by default
		useCompiledRoutes:  true, // Enable compiled route matching by default
		namedRoutes:        make(map[string]*route.Route),
	}

	// Initialize the atomic route tree with an empty map
	initialTrees := make(map[string]*node)
	atomic.StorePointer(&r.routeTree.trees, unsafe.Pointer(&initialTrees))

	// Initialize route compiler
	r.routeCompiler = compiler.NewRouteCompiler(r.bloomFilterSize, r.bloomHashFunctions)

	// Apply functional options
	for _, opt := range opts {
		opt(r)
	}

	// Validate configuration
	if err := r.validate(); err != nil {
		return nil, fmt.Errorf("router configuration validation failed: %w", err)
	}

	return r, nil
}

// MustNew creates a new Router instance and panics if configuration is invalid.
// This is a convenience wrapper around New for cases where configuration errors
// should cause the application to fail immediately at startup.
//
// Usage:
//
//	r := router.MustNew(
//	    router.WithH2C(true),
//	    router.WithMaxParams(16),
//	)
//	// Panics if configuration is invalid
func MustNew(opts ...Option) *Router {
	r, err := New(opts...)
	if err != nil {
		panic(fmt.Sprintf("router.MustNew: %v", err))
	}

	return r
}

// validate checks the router configuration for common errors.
// This method is called automatically by New() to validate configuration.
//
// Validation checks:
//   - Bloom filter parameters are positive
//   - Versioning configuration is valid (if present)
//   - Server timeouts are positive (if configured)
//
// Note: Routes are validated at registration time, not at router creation time,
// because routes are registered after New() returns.
func (r *Router) validate() error {
	// Validate bloom filter configuration
	// Note: bloomFilterSize is uint64, so it can only be 0 or positive
	if r.bloomFilterSize == 0 {
		return ErrBloomFilterSizeZero
	}
	if r.bloomHashFunctions <= 0 {
		return fmt.Errorf("%w: got %d", ErrBloomHashFunctionsInvalid, r.bloomHashFunctions)
	}

	// Validate and create versioning engine from pending options
	// This deferred creation ensures errors are returned through New() instead of panicking
	if len(r.versionOpts) > 0 {
		engine, err := version.New(r.versionOpts...)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrVersioningConfigInvalid, err)
		}
		r.versionEngine = engine
		r.versionOpts = nil // Clear pending options after successful creation
	}

	// Validate server timeouts if configured
	if r.serverTimeouts != nil {
		if r.serverTimeouts.readHeader <= 0 {
			return fmt.Errorf("%w: readHeaderTimeout must be positive", ErrServerTimeoutInvalid)
		}
		if r.serverTimeouts.read <= 0 {
			return fmt.Errorf("%w: readTimeout must be positive", ErrServerTimeoutInvalid)
		}
		if r.serverTimeouts.write <= 0 {
			return fmt.Errorf("%w: writeTimeout must be positive", ErrServerTimeoutInvalid)
		}
		if r.serverTimeouts.idle <= 0 {
			return fmt.Errorf("%w: idleTimeout must be positive", ErrServerTimeoutInvalid)
		}
	}

	return nil
}

// SetObservabilityRecorder sets the unified observability recorder for the router.
// This integrates metrics, tracing, and logging into a single lifecycle.
// Pass nil to disable all observability.
//
// This method is typically called by the app package during initialization,
// but can also be used with standalone routers for custom observability implementations.
// It allows you to configure observability after router creation or change it at runtime.
//
// Example:
//
//	r := router.MustNew()
//	r.SetObservabilityRecorder(myObservabilityRecorder)
func (r *Router) SetObservabilityRecorder(recorder ObservabilityRecorder) {
	r.observability = recorder
}

// emit sends a diagnostic event if a handler is configured.
func (r *Router) emit(kind DiagnosticKind, message string, fields map[string]any) {
	if r.diagnostics != nil {
		r.diagnostics.OnDiagnostic(DiagnosticEvent{
			Kind:    kind,
			Message: message,
			Fields:  fields,
		})
	}
}

// NoRoute sets a custom handler for requests that don't match any registered routes.
// This allows you to customize 404 error responses instead of using the default http.NotFound.
//
// The handler receives a Context that can be used to send custom JSON responses,
// redirect to another page, or perform any other action.
//
// Example:
//
//	r.NoRoute(func(c *Context) {
//	    c.JSON(http.StatusNotFound, map[string]string{"error": "route not found"})
//	})
//
// Setting handler to nil will restore the default http.NotFound behavior.
func (r *Router) NoRoute(handler HandlerFunc) {
	r.noRouteMutex.Lock()
	defer r.noRouteMutex.Unlock()
	r.noRouteHandler = handler
}

// RouteExists checks if a route exists for the given method and path.
// Returns true if the route is registered, false otherwise.
// This is useful for collision detection when registering routes.
//
// Example:
//
//	if r.RouteExists("GET", "/healthz") {
//	    return fmt.Errorf("route already registered: GET /healthz")
//	}
func (r *Router) RouteExists(method, path string) bool {
	treesPtr := atomic.LoadPointer(&r.routeTree.trees)
	if treesPtr == nil {
		return false
	}
	trees := (*map[string]*node)(treesPtr)
	if trees == nil {
		return false
	}

	tree, exists := (*trees)[method]
	if !exists || tree == nil {
		return false
	}

	// Create a temporary context for path matching
	c := getContextFromGlobalPool()
	defer releaseGlobalContext(c)

	// Check radix tree
	if handlers, _ := tree.getRoute(path, c); handlers != nil {
		return true
	}

	// Also check compiled routes if they exist
	if tree.compiled != nil {
		if handlers := tree.compiled.getRoute(path); handlers != nil {
			return true
		}
	}

	return false
}

// getAllowedMethodsForPath checks all method trees to find which methods have routes for the given path.
// Returns a list of allowed HTTP methods, or empty slice if path doesn't match any route.
func (r *Router) getAllowedMethodsForPath(path string) []string {
	treesPtr := atomic.LoadPointer(&r.routeTree.trees)
	if treesPtr == nil {
		return nil
	}
	trees := (*map[string]*node)(treesPtr)
	if trees == nil {
		return nil
	}

	var allowed []string
	// Standard HTTP methods to check
	standardMethods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions}

	// Create a temporary context for path matching
	c := getContextFromGlobalPool()
	defer releaseGlobalContext(c)

	for _, method := range standardMethods {
		// CRITICAL: Reset context state between method checks to prevent parameter pollution
		// If one tree populates parameters, they could leak into subsequent checks
		c.reset()

		if tree, exists := (*trees)[method]; exists && tree != nil {
			// Try to match the path in this method's tree
			if handlers, _ := tree.getRoute(path, c); handlers != nil {
				allowed = append(allowed, method)
			}
			// Also check compiled routes if they exist
			if tree.compiled != nil {
				if handlers := tree.compiled.getRoute(path); handlers != nil {
					// Avoid duplicates
					if !slices.Contains(allowed, method) {
						allowed = append(allowed, method)
					}
				}
			}
		}
	}

	return allowed
}

// handleMethodNotAllowed handles requests where the path matches but the method doesn't.
// Sends an RFC 9457 405 Method Not Allowed problem response with Allow header.
func (r *Router) handleMethodNotAllowed(w http.ResponseWriter, req *http.Request, allowed []string) {
	c := getContextFromGlobalPool()
	c.Request = req
	c.Response = w
	c.index = -1
	c.paramCount = 0
	c.router = r

	// Set version if versioning is enabled
	if r.versionEngine != nil {
		c.version = r.versionEngine.DetectVersion(req)
	}

	// Set route pattern: if we matched a node but wrong method, try to determine pattern
	// Otherwise use sentinel to avoid cardinality explosion
	c.routePattern = "_method_not_allowed"

	// Send 405 response (MethodNotAllowed already sets Allow header)
	c.MethodNotAllowed(allowed)

	// Reset and return to pool
	releaseGlobalContext(c)
}

// handleNotFound handles unmatched routes by either calling the custom NoRoute handler
// or using RFC 9457 problem details by default.
// It also checks if the path exists for other methods (405) vs doesn't exist at all (404).
func (r *Router) handleNotFound(w http.ResponseWriter, req *http.Request) {
	// First check if this path exists for any other method (405)
	allowed := r.getAllowedMethodsForPath(req.URL.Path)
	if len(allowed) > 0 {
		// Path exists but method doesn't - return 405
		r.handleMethodNotAllowed(w, req, allowed)
		return
	}

	// Path doesn't exist for any method - check for custom handler
	r.noRouteMutex.RLock()
	handler := r.noRouteHandler
	r.noRouteMutex.RUnlock()

	if handler != nil {
		// Create a context for the custom handler
		c := getContextFromGlobalPool()
		c.Request = req
		c.Response = w
		c.index = -1
		c.paramCount = 0
		c.router = r

		// Set version if versioning is enabled
		if r.versionEngine != nil {
			c.version = r.versionEngine.DetectVersion(req)
		}

		// Set route pattern for metrics/tracing
		c.routePattern = "_not_found"

		// Execute the custom handler
		handler(c)

		// Reset and return to pool
		releaseGlobalContext(c)
	} else {
		// Default: Use RFC 9457 problem details
		c := getContextFromGlobalPool()
		c.Request = req
		c.Response = w
		c.index = -1
		c.paramCount = 0
		c.router = r

		// Set route pattern for metrics/tracing (sentinel, not raw path)
		c.routePattern = "_not_found"

		if r.versionEngine != nil {
			c.version = r.versionEngine.DetectVersion(req)
		}

		c.NotFound()

		releaseGlobalContext(c)
	}
}

// updateTrees updates the route trees map using copy-on-write semantics.
// This method ensures thread-safe updates without blocking concurrent reads.
func (r *Router) updateTrees(updater func(map[string]*node) map[string]*node) {
	for {
		// Step 1: Atomically load the current tree pointer
		// Multiple goroutines can read this simultaneously without blocking
		currentPtr := atomic.LoadPointer(&r.routeTree.trees)
		currentTrees := *(*map[string]*node)(currentPtr)

		// Step 2: Create a modified copy using the updater function
		// This is copy-on-write: we never modify the existing tree
		// Other goroutines still see the old tree during this operation
		newTrees := updater(currentTrees)

		// Step 3: Attempt atomic compare-and-swap
		// This succeeds only if no other goroutine modified the pointer since step 1
		// If successful, all future readers will see the new tree
		if atomic.CompareAndSwapPointer(&r.routeTree.trees, currentPtr, unsafe.Pointer(&newTrees)) {
			// Successfully updated, increment version for optimistic concurrency control
			atomic.AddUint64(&r.routeTree.version, 1)
			return
		}
		// Step 4: CAS failed - another goroutine won the race
		// Retry the entire operation with a fresh snapshot of the current state
		// This is rare in practice since route registration typically happens at startup
	}
}

// addRouteToTree adds a route to the tree.
// This method uses copy-on-write semantics, only copying when necessary.
func (r *Router) addRouteToTree(method, path string, handlers []HandlerFunc, constraints []route.Constraint) {
	// Phase 1: Check if method tree already exists
	// This read is atomic and safe even during concurrent writes
	treesPtr := atomic.LoadPointer(&r.routeTree.trees)
	trees := *(*map[string]*node)(treesPtr)

	if tree, exists := trees[method]; exists {
		// Tree exists, add route directly (thread-safe due to per-node mutex)
		// Direct modification - we're only modifying the tree, not replacing it
		tree.addRouteWithConstraints(path, handlers, constraints)
		return
	}

	// Phase 2: Tree doesn't exist for this method, need to create it atomically
	r.updateTrees(func(currentTrees map[string]*node) map[string]*node {
		// Double-check: another goroutine might have created the tree during retry
		if tree, exists := currentTrees[method]; exists {
			// Another goroutine won the race and created it, add route directly
			tree.addRouteWithConstraints(path, handlers, constraints)
			return currentTrees // No copy needed
		}

		// Create new trees map with the new method tree
		// Copy-on-write: clone the map and add the new tree
		newTrees := make(map[string]*node, len(currentTrees)+1)
		maps.Copy(newTrees, currentTrees)
		newTrees[method] = &node{}
		newTrees[method].addRouteWithConstraints(path, handlers, constraints)

		return newTrees
	})
}

// Use adds global middleware to the router that will be executed for all routes.
// Middleware functions are executed in the order they are added.
//
// The middleware will be combined with route-specific handlers and executed
// as part of the handler chain for every matching request.
//
// Example:
//
//	r.Use(Logger(), Recovery(), CORS())
//	r.GET("/api/users", getUsersHandler) // Will execute all 3 middleware + handler
func (r *Router) Use(middleware ...HandlerFunc) {
	r.middlewareMu.Lock()
	r.middleware = append(r.middleware, middleware...)
	r.middlewareMu.Unlock()
}
