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
	"context"
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

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"rivaas.dev/router/compiler"
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
	pendingRoutes   []*Route   // Routes waiting to be registered during Warmup
	pendingRoutesMu sync.Mutex // Protects pendingRoutes slice and warmedUp flag
	warmupOnce      sync.Once  // Ensures warmup runs exactly once
	warmedUp        bool       // True after Warmup has completed

	// Routing features
	versionEngine *version.Engine    // API versioning engine for version detection
	versionTrees  atomicVersionTrees // Version-specific route trees
	versionCache  sync.Map           // Version-specific compiled routes

	// Configuration
	bloomFilterSize    uint64 // Size of bloom filters for compiled routes (default: 1000)
	bloomHashFunctions int    // Number of hash functions for bloom filters (default: 3)
	checkCancellation  bool   // Enable context cancellation checks in Next() (default: true)

	// Route compilation
	routeCompiler *compiler.RouteCompiler // Pre-compiled routes for matching
	useTemplates  bool                    // Enable template-based routing (default: true)

	// Custom 404 handler
	noRouteHandler HandlerFunc  // Custom handler for unmatched routes (nil means use http.NotFound)
	noRouteMutex   sync.RWMutex // Protects noRouteHandler (rarely written, frequently read)

	// HTTP/2 Cleartext (H2C) support
	enableH2C      bool            // Enable HTTP/2 cleartext support (dev/behind LB only)
	serverTimeouts *serverTimeouts // HTTP server timeout configuration

	// Trusted proxies configuration for real client IP detection
	realip *realIPConfig // Compiled trusted proxy configuration

	// Route freezing and naming
	frozen             atomic.Bool       // Routes are frozen (immutable) after freeze
	namedRoutes        map[string]*Route // name -> route mapping
	routeSnapshot      []*Route          // Immutable snapshot built at freeze time
	routeSnapshotMutex sync.RWMutex      // Protects routeSnapshot
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
		useTemplates:       true, // Enable template-based routing by default
		namedRoutes:        make(map[string]*Route),
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

	// Validate versioning configuration if present
	// Note: Versioning validation is handled internally by the versioning system
	// We just verify it exists and was properly initialized
	// Engine validates configuration during construction
	if r.versionEngine != nil {
		_ = r.versionEngine // Verify it exists
	}

	return nil
}

// SetObservabilityRecorder sets the unified observability recorder for the router.
// This integrates metrics, tracing, and logging into a single lifecycle.
// Pass nil to disable all observability.
//
// This method is typically called by the app package during initialization,
// but can also be used with standalone routers for custom observability implementations.
// SetObservabilityRecorder sets the observability recorder for metrics, tracing, and logging.
// This allows you to configure observability after router creation or change it at runtime.
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

	// Set route template: if we matched a node but wrong method, try to determine template
	// Otherwise use sentinel to avoid cardinality explosion
	c.routeTemplate = "_method_not_allowed"

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

		// Set route template for metrics/tracing
		c.routeTemplate = "_not_found"

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

		// Set route template for metrics/tracing (sentinel, not raw path)
		c.routeTemplate = "_not_found"

		if r.versionEngine != nil {
			c.version = r.versionEngine.DetectVersion(req)
		}

		c.NotFound()

		releaseGlobalContext(c)
	}
}

// WithDiagnostics sets a diagnostic handler for the router.
//
// Diagnostic events are optional informational events that may indicate
// configuration issues or security concerns.
// The router functions correctly whether diagnostics are collected or not.
//
// Example with logging:
//
//	import "log/slog"
//
//	handler := router.DiagnosticHandlerFunc(func(e router.DiagnosticEvent) {
//	    slog.Warn(e.Message, "kind", e.Kind, "fields", e.Fields)
//	})
//	r := router.MustNew(router.WithDiagnostics(handler))
//
// Example with metrics:
//
//	handler := router.DiagnosticHandlerFunc(func(e router.DiagnosticEvent) {
//	    metrics.Increment("router.diagnostics", "kind", string(e.Kind))
//	})
//
// Example with OpenTelemetry:
//
//	import "go.opentelemetry.io/otel/attribute"
//	import "go.opentelemetry.io/otel/trace"
//
//	handler := router.DiagnosticHandlerFunc(func(e router.DiagnosticEvent) {
//	    span := trace.SpanFromContext(ctx)
//	    if span.IsRecording() {
//	        attrs := []attribute.KeyValue{
//	            attribute.String("diagnostic.kind", string(e.Kind)),
//	        }
//	        for k, v := range e.Fields {
//	            attrs = append(attrs, attribute.String(k, fmt.Sprint(v)))
//	        }
//	        span.AddEvent(e.Message, trace.WithAttributes(attrs...))
//	    }
//	})
func WithDiagnostics(handler DiagnosticHandler) Option {
	return func(r *Router) {
		r.diagnostics = handler
	}
}

// WithH2C enables HTTP/2 Cleartext support.
//
// ⚠️ SECURITY WARNING: Only use in development or behind a trusted load balancer.
// DO NOT enable on public-facing servers without TLS.
//
// Common deployment patterns:
//   - Dev/local testing: Enable h2c for direct HTTP/2 testing
//   - Behind Envoy/Caddy: LB speaks h2c to app (configure LB accordingly)
//   - Behind Nginx: Typically uses HTTP/1.1 upstream (h2c not needed)
//
// Requires: golang.org/x/net/http2/h2c
//
// Example:
//
//	r := router.MustNew(router.WithH2C(true))
//	r.Serve(":8080")
func WithH2C(enable bool) Option {
	return func(r *Router) {
		r.enableH2C = enable
	}
}

// WithServerTimeouts configures HTTP server timeouts.
// These are critical for preventing slowloris attacks and resource exhaustion.
//
// Defaults (if not set):
//
//	ReadHeaderTimeout: 5s  - Time to read request headers
//	ReadTimeout:       15s - Time to read entire request
//	WriteTimeout:      30s - Time to write response
//	IdleTimeout:       60s - Keep-alive idle time
//
// Example:
//
//	r := router.MustNew(router.WithServerTimeouts(
//	    10*time.Second,  // ReadHeaderTimeout
//	    30*time.Second,  // ReadTimeout
//	    60*time.Second,  // WriteTimeout
//	    120*time.Second, // IdleTimeout
//	))
func WithServerTimeouts(readHeader, read, write, idle time.Duration) Option {
	return func(r *Router) {
		r.serverTimeouts = &serverTimeouts{
			readHeader: readHeader,
			read:       read,
			write:      write,
			idle:       idle,
		}
	}
}

// defaultServerTimeouts returns default timeout configuration.
func defaultServerTimeouts() *serverTimeouts {
	return &serverTimeouts{
		readHeader: 5 * time.Second,
		read:       15 * time.Second,
		write:      30 * time.Second,
		idle:       60 * time.Second,
	}
}

// WithBloomFilterSize returns a RouterOption that sets the bloom filter size for compiled routes.
// The bloom filter is used for negative lookups in static route matching.
// Larger sizes reduce false positives.
//
// Default: 1000
// Recommended: Set to 2-3x the number of static routes
// Must be > 0 or validation will fail.
//
// Example:
//
//	r := router.MustNew(router.WithBloomFilterSize(2000)) // For ~1000 routes
func WithBloomFilterSize(size uint64) Option {
	return func(r *Router) {
		r.bloomFilterSize = size
	}
}

// WithBloomFilterHashFunctions returns a RouterOption that sets the number of hash functions
// used in bloom filters for compiled routes. More hash functions reduce false positives.
//
// Default: 3
// Range: 1-10 (values outside this range are clamped)
// Recommended: 3-5 for most use cases
//
// False positive rate formula: (1 - e^(-kn/m))^k
// where k = hash functions, n = items, m = bits
//
// Example:
//
//	r := router.MustNew(router.WithBloomFilterHashFunctions(4))
func WithBloomFilterHashFunctions(numFuncs int) Option {
	return func(r *Router) {
		// Clamp to reasonable range
		if numFuncs < 1 {
			numFuncs = 1
		} else if numFuncs > 10 {
			numFuncs = 10
		}
		r.bloomHashFunctions = numFuncs
	}
}

// WithCancellationCheck returns a RouterOption that enables/disables context cancellation
// checking in the middleware chain. When enabled, the router checks for cancelled contexts
// between each handler, preventing wasted work on timed-out requests.
//
// Default: true (enabled)
//
// Disable if:
//   - You don't use request timeouts
//   - You handle cancellation manually in handlers
//
// Example:
//
//	r := router.MustNew(router.WithCancellationCheck(false))
func WithCancellationCheck(enabled bool) Option {
	return func(r *Router) {
		r.checkCancellation = enabled
	}
}

// WithTemplateRouting returns a RouterOption that enables/disables template-based routing.
// When enabled, routes are compiled into templates for lookup.
//
// Default: true (enabled)
//
// Disable only for debugging or if you encounter issues.
//
// Example:
//
//	r := router.MustNew(router.WithTemplateRouting(true))  // Enabled by default
func WithTemplateRouting(enabled bool) Option {
	return func(r *Router) {
		r.useTemplates = enabled
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
func (r *Router) addRouteToTree(method, path string, handlers []HandlerFunc, constraints []RouteConstraint) {
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

// Group creates a new route group with the specified prefix and optional middleware.
// Route groups allow you to organize related routes under a common path prefix
// and apply middleware that is specific to that group.
//
// The prefix will be prepended to all routes registered with the group.
// Group middleware is executed after global middleware but before route handlers.
//
// Example:
//
//	api := r.Group("/api/v1", AuthMiddleware())
//	api.GET("/users", getUsersHandler)    // Matches: GET /api/v1/users
//	api.POST("/users", createUserHandler) // Matches: POST /api/v1/users
func (r *Router) Group(prefix string, middleware ...HandlerFunc) *Group {
	return &Group{
		router:     r,
		prefix:     prefix,
		middleware: middleware,
	}
}

// ServeHTTP implements the http.Handler interface for Router.
// It matches the incoming HTTP request to a registered route and executes
// the associated handler chain.
//
// The routing algorithm uses explicit versioning - routes are only versioned
// if registered via r.Version(). The precedence is:
//
//  1. Main tree (non-versioned routes registered via r.GET, r.POST, etc.)
//     - These routes bypass version detection entirely
//     - Common for: /health, /metrics, /docs, static assets
//  2. Version-specific trees (routes registered via r.Version().GET, etc.)
//     - Only checked if main tree has no match
//     - Subject to version detection (header/path/query/accept)
//
// Within each tree, the lookup order is:
//  1. Compiled static routes (hash table O(1) lookup)
//  2. Compiled dynamic routes (pre-compiled patterns with bloom filter)
//  3. Dynamic tree traversal (fallback for uncached routes)
//
// For each request:
//  1. Resets a pooled context for the request
//  2. Matches the path to a route (main tree first, then version trees)
//  3. Extracts URL parameters (for dynamic routes)
//  4. Executes the handler chain with middleware
//  5. Returns the context to the pool
//
// Static routes and dynamic routes both use context pooling.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Lazy warmup: ensure routes are registered on first request
	// This is safe to call multiple times due to sync.Once
	r.Warmup()

	path := req.URL.Path
	ctx := req.Context()
	var obsState any

	// Observability lifecycle - start
	if r.observability != nil {
		var enrichedCtx context.Context
		enrichedCtx, obsState = r.observability.OnRequestStart(ctx, req)

		// Only attach enriched context if it actually changed
		// This avoids unnecessary creation when observability doesn't enrich the context
		if enrichedCtx != ctx {
			ctx = enrichedCtx
			req = req.WithContext(ctx)
		}
	}

	// Only wrap ResponseWriter if not excluded
	if r.observability != nil && obsState != nil {
		w = r.observability.WrapResponseWriter(w, obsState)
	}

	// ══════════════════════════════════════════════════════════════════════════
	// STEP 1: Try main tree FIRST (non-versioned routes)
	// Routes registered via r.GET(), r.POST() etc. bypass version detection.
	// This is the fast path for infrastructure endpoints like /health, /metrics.
	// ══════════════════════════════════════════════════════════════════════════

	// Try compiled routes from main tree (if enabled)
	if r.useTemplates && r.routeCompiler != nil {
		// Try static route table first (O(1) hash lookup)
		if route := r.routeCompiler.LookupStatic(req.Method, path); route != nil {
			r.serveCompiledRoute(w, req, route, obsState)
			return
		}

		// Try dynamic routes (pre-compiled patterns with bloom filter)
		poolCtx := getContextFromGlobalPool()
		poolCtx.SetParamCount(0)

		if route := r.routeCompiler.MatchDynamic(req.Method, path, poolCtx); route != nil {
			r.serveCompiledRouteWithParams(w, req, route, poolCtx, obsState)
			return
		}

		releaseGlobalContext(poolCtx)
	}

	// Try main tree's dynamic routing (tree traversal fallback)
	tree := r.getTreeForMethodDirect(req.Method)
	if tree != nil {
		// Try per-tree compiled routes (legacy path)
		if tree.compiled != nil {
			if handlers := tree.compiled.getRoute(path); handlers != nil {
				r.serveStaticRoute(w, req, handlers, path, "", false, obsState)
				return
			}
		}

		// Try dynamic tree traversal
		c := getContextFromGlobalPool()
		c.Request = req
		c.Response = w
		c.index = -1
		c.paramCount = 0
		c.router = r
		c.version = "" // Non-versioned route

		handlers, routePattern := tree.getRoute(path, c)
		if handlers != nil {
			// Found in main tree - serve without version context
			c.routeTemplate = routePattern
			if c.routeTemplate == "" {
				c.routeTemplate = "_unmatched"
			}

			var logger *slog.Logger
			if r.observability != nil {
				logger = r.observability.BuildRequestLogger(ctx, req, routePattern)
			} else {
				logger = noopLogger
			}
			c.logger = logger

			c.handlers = handlers
			c.index = -1
			c.Next()

			releaseGlobalContext(c)

			if obsState != nil {
				r.observability.OnRequestEnd(ctx, obsState, w, routePattern)
			}
			return
		}

		releaseGlobalContext(c)
	}

	// ══════════════════════════════════════════════════════════════════════════
	// STEP 2: No match in main tree → try version-specific trees
	// Routes registered via r.Version().GET() are subject to version detection.
	// ══════════════════════════════════════════════════════════════════════════

	// Only do version detection if versioning is enabled
	if r.versionEngine != nil {
		vc := r.processVersioning(req, path)

		if vc.tree != nil {
			r.serveVersionedRequest(w, req, vc.tree, vc.routingPath, vc.version, obsState)
			return
		}
	}

	// ══════════════════════════════════════════════════════════════════════════
	// STEP 3: No match anywhere → 404
	// ══════════════════════════════════════════════════════════════════════════
	r.handleNotFoundWithObs(w, req, obsState)
}

// handleNotFoundWithObs handles 404 responses with observability support.
func (r *Router) handleNotFoundWithObs(w http.ResponseWriter, req *http.Request, obsState any) {
	// Build request-scoped logger for 404 handler (for future use)
	// Currently handleNotFound doesn't use the logger, but it could in the future
	if r.observability != nil {
		_ = r.observability.BuildRequestLogger(req.Context(), req, "_not_found")
	}

	// Call the 404 handler
	r.handleNotFound(w, req)

	// Finish observability with "_not_found" sentinel
	if obsState != nil {
		r.observability.OnRequestEnd(req.Context(), obsState, w, "_not_found")
	}
}

// serveStaticRoute serves a static (compiled) route with observability support.
func (r *Router) serveStaticRoute(w http.ResponseWriter, req *http.Request, handlers []HandlerFunc, routePattern, version string, hasVersioning bool, obsState any) {
	ctx := req.Context()

	// Build request-scoped logger (after routing)
	var logger *slog.Logger
	if r.observability != nil {
		logger = r.observability.BuildRequestLogger(ctx, req, routePattern)
	} else {
		logger = noopLogger
	}

	// Get context from pool
	c := getContextFromGlobalPool()
	c.Request = req
	c.Response = w
	c.handlers = handlers
	c.router = r
	c.logger = logger
	c.routeTemplate = routePattern
	c.index = -1
	c.paramCount = 0

	// Set version: explicitly set to provided value or empty for non-versioned routes
	// Important: Must always set to avoid leftover values from pooled context
	if hasVersioning {
		c.version = version
	} else {
		c.version = ""
	}

	// Execute handlers
	c.Next()

	// Reset and return to pool
	releaseGlobalContext(c)

	// Finish observability
	if obsState != nil {
		r.observability.OnRequestEnd(ctx, obsState, w, routePattern)
	}
}

// serveVersionedRequest handles requests with version-specific routing.
func (r *Router) serveVersionedRequest(w http.ResponseWriter, req *http.Request, tree *node, path, version string, obsState any) {
	ctx := req.Context()

	// Check if version has compiled routes
	// NOTE: Version cache lookup uses version+method as key because different HTTP methods
	// have different handlers even for the same path
	cacheKey := version + ":" + req.Method
	if compiledValue, ok := r.versionCache.Load(cacheKey); ok {
		if compiled, ok := compiledValue.(*CompiledRouteTable); ok && compiled != nil {
			// Try compiled routes first - get both handlers and route pattern
			if handlers, routePath := compiled.getRouteWithPath(path); handlers != nil {
				// Use the actual route path (pattern) for route template, version for deprecation headers
				r.serveVersionedHandlers(w, req, handlers, routePath, version, obsState)
				return
			}
		}
	}

	// Fallback to dynamic routing
	c := getContextFromGlobalPool()
	c.Request = req
	c.Response = w
	c.index = -1
	c.paramCount = 0
	c.router = r
	c.version = version

	defer releaseGlobalContext(c)

	// Find the route and extract parameters
	handlers, routePattern := tree.getRoute(path, c)
	if handlers == nil {
		r.handleNotFound(w, req)
		return
	}

	// Set lifecycle headers (deprecation, sunset, etc.) and check if version is past sunset
	if r.versionEngine != nil {
		if isSunset := r.versionEngine.SetLifecycleHeaders(w, version, routePattern); isSunset {
			// Version is past sunset date - return 410 Gone
			w.WriteHeader(http.StatusGone)
			w.Write([]byte(fmt.Sprintf("API %s was removed. Please upgrade to a supported version.", version)))
			return
		}
	}

	// Set route template from the matched pattern
	c.routeTemplate = routePattern
	if c.routeTemplate == "" {
		c.routeTemplate = "_unmatched" // Fallback (should rarely happen)
	}

	// Execute handlers with the context that has extracted parameters
	c.handlers = handlers

	// Build request-scoped logger (after routing)
	var logger *slog.Logger
	if r.observability != nil {
		logger = r.observability.BuildRequestLogger(ctx, req, routePattern)
	} else {
		logger = noopLogger
	}
	c.logger = logger

	// Execute
	c.Next()

	// Finish observability
	if obsState != nil {
		r.observability.OnRequestEnd(ctx, obsState, w, routePattern)
	}
}

// serveVersionedHandlers executes handlers with version information
// routeTemplate is the actual route pattern (e.g., "/test") used for logging/metrics
// version is the API version (e.g., "v1") used for deprecation headers and context
func (r *Router) serveVersionedHandlers(w http.ResponseWriter, req *http.Request, handlers []HandlerFunc, routeTemplate, version string, obsState any) {
	ctx := req.Context()

	// Set lifecycle headers (deprecation, sunset, etc.) and check if version is past sunset
	// This is called before handler execution to ensure headers are set early
	if r.versionEngine != nil {
		if isSunset := r.versionEngine.SetLifecycleHeaders(w, version, routeTemplate); isSunset {
			// Version is past sunset date - return 410 Gone
			w.WriteHeader(http.StatusGone)
			w.Write([]byte(fmt.Sprintf("API %s was removed. Please upgrade to a supported version.", version)))
			return
		}
	}

	// Build request-scoped logger (after routing)
	var logger *slog.Logger
	if r.observability != nil {
		logger = r.observability.BuildRequestLogger(ctx, req, routeTemplate)
	} else {
		logger = noopLogger
	}

	// Use global context pool
	c := getContextFromGlobalPool()
	c.Request = req
	c.Response = w
	c.index = -1
	c.paramCount = 0
	c.router = r
	c.version = version
	c.routeTemplate = routeTemplate // Set route template for access log
	c.logger = logger
	c.handlers = handlers

	// Execute
	c.Next()

	// Reset and return to pool
	releaseGlobalContext(c)

	// Finish observability
	if obsState != nil {
		r.observability.OnRequestEnd(ctx, obsState, w, routeTemplate)
	}
}

// countStaticRoutesForMethod counts the number of static routes (no parameters) in a method tree.
// This is used to determine optimal bloom filter size.
func (r *Router) countStaticRoutesForMethod(method string) int {
	tree := r.getTreeForMethodDirect(method)
	if tree == nil {
		return 0
	}
	return tree.countStaticRoutes()
}

// optimalBloomFilterSize calculates the bloom filter size based on route count.
// Uses the formula: m = -n*ln(p) / (ln(2)^2) where:
//   - n = number of routes
//   - p = desired false positive rate (0.01 = 1%)
//   - m = bits needed
//
// Uses 10 bits per route for approximately 1% false positive rate.
func optimalBloomFilterSize(routeCount int) uint64 {
	if routeCount <= 0 {
		return defaultBloomFilterSize
	}
	// Calculate size based on route count
	// Minimum size of 100 to avoid degenerate cases
	size := uint64(routeCount * 10)
	if size < 100 {
		return 100
	}
	// Cap at maximum size
	if size > 1000000 {
		return 1000000
	}
	return size
}

func (r *Router) compileRoutesForMethod(method string) {
	tree := r.getTreeForMethodDirect(method)
	if tree == nil {
		return
	}

	// Calculate optimal bloom filter size based on route count
	// If user hasn't explicitly set a size, auto-size based on routes
	bloomSize := r.bloomFilterSize
	if bloomSize == defaultBloomFilterSize {
		// Count static routes in this tree to determine optimal size
		routeCount := r.countStaticRoutesForMethod(method)
		bloomSize = optimalBloomFilterSize(routeCount)
	}

	// Compile routes
	_ = tree.compileStaticRoutes(bloomSize, r.bloomHashFunctions)
}

// CompileAllRoutes pre-compiles all static routes.
// This should be called after all routes are registered.
func (r *Router) CompileAllRoutes() {
	treesPtr := atomic.LoadPointer(&r.routeTree.trees)
	trees := (*map[string]*node)(treesPtr)

	for method := range *trees {
		r.compileRoutesForMethod(method)
	}
}

// Warmup registers all pending routes and pre-compiles them for optimal request handling.
// This should be called after all routes are registered and before serving requests.
//
// Warmup phases:
// 1. Register all pending routes to their appropriate trees (standard or version-specific)
// 2. Compile all static routes into hash tables with bloom filters
// 3. Compile version-specific routes if versioning is enabled
//
// Warmup prepares the router for handling requests by registering routes,
// compiling data structures, and initializing caches before traffic arrives.
//
// Calling Warmup() multiple times is safe - routes are only registered once.
func (r *Router) Warmup() {
	r.warmupOnce.Do(r.doWarmup)
}

// doWarmup performs the actual warmup work (called via sync.Once).
func (r *Router) doWarmup() {
	// CRITICAL: Set warmedUp=true BEFORE clearing pendingRoutes to avoid race condition
	// Without this, routes added between clearing pendingRoutes and setting warmedUp=true
	// would be lost (added to empty pendingRoutes, but warmedUp still false, warmup done)
	r.pendingRoutesMu.Lock()
	r.warmedUp = true
	routes := r.pendingRoutes
	r.pendingRoutes = nil // Clear pending routes
	r.pendingRoutesMu.Unlock()

	// Phase 1: Register all pending routes to their appropriate trees
	for _, route := range routes {
		route.registerRoute()
	}

	// Phase 2: Compile all standard (non-versioned) routes
	r.CompileAllRoutes()

	// Phase 3: Compile version-specific routes if versioning is enabled
	if r.versionEngine != nil {
		r.compileVersionRoutes()
	}
}

// compileVersionRoutes compiles static routes for all version-specific trees
// and stores them in the version cache (sync.Map).
// This enables lookup for versioned static routes.
// Cache key format: "version:method" (e.g., "v1:GET")
func (r *Router) compileVersionRoutes() {
	// Load version trees atomically
	versionTreesPtr := atomic.LoadPointer(&r.versionTrees.trees)
	if versionTreesPtr == nil {
		return // No version-specific routes registered
	}

	versionTrees := *(*map[string]map[string]*node)(versionTreesPtr)

	// Compile static routes for each version AND method
	// Each method gets its own compiled table to avoid handler conflicts
	for version, methodTrees := range versionTrees {
		for method, tree := range methodTrees {
			if tree == nil {
				continue
			}

			// Count static routes for this method tree
			staticRoutes := tree.countStaticRoutes()
			if staticRoutes == 0 {
				continue
			}

			// Calculate optimal bloom filter size
			bloomSize := r.bloomFilterSize
			if bloomSize == defaultBloomFilterSize {
				bloomSize = optimalBloomFilterSize(staticRoutes)
			}

			// Create compiled table for this version+method combination
			compiled := &CompiledRouteTable{
				routes: make(map[uint64]*CompiledRoute),
				bloom:  compiler.NewBloomFilter(bloomSize, r.bloomHashFunctions),
			}

			// Compile routes from this method's tree
			tree.compileStaticRoutesRecursive(compiled, "")

			// Store with version:method key
			if len(compiled.routes) > 0 {
				cacheKey := version + ":" + method
				r.versionCache.Store(cacheKey, compiled)
			}
		}
	}
}

// recordRouteRegistration is a hook for route registration tracking.
// Currently a no-op; route registration is tracked via RouteInfo in the route tree.
// Diagnostic events are reserved for runtime anomalies (security, performance),
// not routine setup events which would be too noisy.
func (r *Router) recordRouteRegistration(method, path string) {
	// Intentionally empty - route registration is tracked via r.routeTree.routes
	// Diagnostic events are for runtime anomalies, not routine setup
	_ = method
	_ = path
}

// serveCompiledRoute serves a request using a compiled route (static route).
// This path handles routes without parameters.
func (r *Router) serveCompiledRoute(w http.ResponseWriter, req *http.Request, route *compiler.CompiledRoute, obsState any) {
	routeTemplate := route.Pattern() // Use route pattern, not raw path
	ctx := req.Context()

	// Build request-scoped logger (after routing)
	var logger *slog.Logger
	if r.observability != nil {
		logger = r.observability.BuildRequestLogger(ctx, req, routeTemplate)
	} else {
		logger = noopLogger
	}

	// Execute handlers
	c := getContextFromGlobalPool()
	defer releaseGlobalContext(c)

	// Use cached handlers
	cachedPtr := route.CachedHandlers()
	if cachedPtr != nil {
		handlers := *(*[]HandlerFunc)(cachedPtr)
		c.initForRequest(req, w, handlers, r)
	} else {
		// Fallback: Convert compiler.HandlerFunc to router.HandlerFunc
		handlers := make([]HandlerFunc, len(route.Handlers()))
		for i, h := range route.Handlers() {
			handlers[i] = h.(HandlerFunc)
		}
		c.initForRequest(req, w, handlers, r)
	}

	c.routeTemplate = routeTemplate // Set template for access
	c.logger = logger

	c.Next()

	// Finish observability
	if obsState != nil {
		r.observability.OnRequestEnd(ctx, obsState, w, routeTemplate)
	}
}

// serveCompiledRouteWithParams serves a request using a compiled route with pre-extracted parameters.
// The context already has parameters populated by the route matching.
func (r *Router) serveCompiledRouteWithParams(w http.ResponseWriter, req *http.Request, route *compiler.CompiledRoute, c *Context, obsState any) {
	// Store the route template for metrics/tracing
	routeTemplate := route.Pattern() // Use route pattern, not raw path
	c.routeTemplate = routeTemplate
	ctx := req.Context()

	// Reuse the context that already has parameters extracted
	// Use special init that preserves parameters

	// Use cached handlers
	cachedPtr := route.CachedHandlers()
	if cachedPtr != nil {
		handlers := *(*[]HandlerFunc)(cachedPtr)
		c.initForRequestWithParams(req, w, handlers, r)
	} else {
		// Fallback: Convert compiler.HandlerFunc to router.HandlerFunc
		handlers := make([]HandlerFunc, len(route.Handlers()))
		for i, h := range route.Handlers() {
			handlers[i] = h.(HandlerFunc)
		}
		c.initForRequestWithParams(req, w, handlers, r)
	}

	// Build request-scoped logger (after routing)
	var logger *slog.Logger
	if r.observability != nil {
		logger = r.observability.BuildRequestLogger(ctx, req, routeTemplate)
	} else {
		logger = noopLogger
	}
	c.logger = logger

	defer releaseGlobalContext(c)

	// Execute handler chain
	c.Next()

	// Finish observability
	if obsState != nil {
		r.observability.OnRequestEnd(ctx, obsState, w, routeTemplate)
	}
}

// Serve starts the HTTP server on the specified address.
// Automatically enables h2c if configured via WithH2C().
//
// The server is configured with production-safe timeouts to prevent
// slowloris attacks and resource exhaustion. These timeouts are critical
// for production deployments.
//
// Example:
//
//	r := router.MustNew()
//	r.GET("/", func(c *router.Context) {
//	    c.String(http.StatusOK, "Hello, World!")
//	})
//	if err := r.Serve(":8080"); err != nil {
//	    log.Fatal(err)
//	}
//
// With H2C enabled (dev/behind LB only):
//
//	r := router.MustNew(router.WithH2C(true))
//	r.Serve(":8080")
func (r *Router) Serve(addr string) error {
	h := http.Handler(r)

	if r.enableH2C {
		h = h2c.NewHandler(h, &http2.Server{})
		r.emit(DiagH2CEnabled, "H2C enabled; use only in dev or behind a trusted LB", nil)
	}

	timeouts := r.serverTimeouts
	if timeouts == nil {
		timeouts = defaultServerTimeouts()
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: timeouts.readHeader,
		ReadTimeout:       timeouts.read,
		WriteTimeout:      timeouts.write,
		IdleTimeout:       timeouts.idle,
	}

	return srv.ListenAndServe()
}

// ServeTLS starts the HTTPS server with TLS configuration.
// For TLS servers, HTTP/2 is automatically enabled via ALPN.
//
// The server is configured with production-safe timeouts to prevent
// slowloris attacks and resource exhaustion.
//
// Optional: Configure HTTP/2 settings for TLS:
//
//	import "golang.org/x/net/http2"
//	srv := &http.Server{...}
//	http2.ConfigureServer(srv, &http2.Server{
//	    MaxConcurrentStreams: 256,
//	})
//
// Example:
//
//	r := router.MustNew()
//	r.GET("/", func(c *router.Context) {
//	    c.String(http.StatusOK, "Hello, World!")
//	})
//	if err := r.ServeTLS(":8443", "cert.pem", "key.pem"); err != nil {
//	    log.Fatal(err)
//	}
func (r *Router) ServeTLS(addr, certFile, keyFile string) error {
	timeouts := r.serverTimeouts
	if timeouts == nil {
		timeouts = defaultServerTimeouts()
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: timeouts.readHeader,
		ReadTimeout:       timeouts.read,
		WriteTimeout:      timeouts.write,
		IdleTimeout:       timeouts.idle,
	}

	// HTTP/2 is automatically enabled over TLS via ALPN
	// Optional: tune HTTP/2 settings
	// http2.ConfigureServer(srv, &http2.Server{MaxConcurrentStreams: 256})

	return srv.ListenAndServeTLS(certFile, keyFile)
}
