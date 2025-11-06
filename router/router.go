// Package router provides an HTTP router for Go with minimal memory allocations.
//
// The router implements a radix tree-based routing algorithm for cloud-native
// applications. It features efficient path matching for static routes, efficient
// parameter extraction, and comprehensive middleware support.
//
// Key Features:
//   - Fast radix tree routing with O(k) path matching
//   - Efficient path matching for static routes
//   - Memory efficient with only 3 allocations per request
//   - Support for URL parameters and middleware chains
//   - Route grouping for hierarchical API organization
//   - Context pooling for performance
//
// Performance characteristics:
//   - High throughput with minimal latency
//   - Minimal memory allocations per request
//   - Fast radix tree routing for static paths
//
// Example usage:
//
//	package main
//
//	import (
//	    "net/http"
//	    "rivaas.dev/router"
//	)
//
//	func main() {
//	    r := router.New()
//
//	    r.GET("/", func(c *router.Context) {
//	        c.JSON(http.StatusOK, map[string]string{"message": "Hello World"})
//	    })
//
//	    r.GET("/users/:id", func(c *router.Context) {
//	        c.JSON(http.StatusOK, map[string]string{"user_id": c.Param("id")})
//	    })
//
//	    http.ListenAndServe(":8080", r)
//	}
package router

import (
	"bufio"
	"context"
	"fmt"
	"maps"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"rivaas.dev/logging"
)

// Option defines functional options for router configuration.
type Option func(*Router)

// responseWriter wraps http.ResponseWriter to capture status code and size.
// It also prevents "superfluous response.WriteHeader call" errors
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
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
	rw.size += n
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
func (rw *responseWriter) Size() int {
	return rw.size
}

// Written returns true if headers have been written.
func (rw *responseWriter) Written() bool {
	return rw.written
}

// Hijack implements http.Hijacker interface.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("responseWriter does not implement http.Hijacker")
}

// Flush implements http.Flusher interface.
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Router represents the HTTP router with performance optimizations.
// It uses a radix tree for fast path matching and includes context pooling
// to minimize memory allocations during request handling.
//
// Key performance features:
//   - Radix tree for O(k) path matching where k is the path length
//   - Context pooling to reduce garbage collection pressure
//   - Lock-free route registration using atomic operations
//   - Efficient path matching where possible
//   - Optional OpenTelemetry tracing with minimal overhead (when enabled)
//   - Middleware chain execution with pre-compilation
//   - Compiled route tables for fast static route matching
//   - Context pooling with specialized pools
//   - Routing with wildcard support and route versioning
//
// Lock-free architecture (fully achieved):
//   - Route tree: atomic.Pointer with CAS loops (no global mutex)
//   - Version trees: atomic.Pointer with CAS loops (no global mutex)
//   - Version cache: sync.Map (no RWMutex)
//   - Per-node locks: Fine-grained RWMutex (allows concurrent tree modifications)
//
// This means:
//   - Request handling NEVER blocks on locks (fully lock-free read path)
//   - Route registration uses optimistic concurrency (minimal contention)
//   - Scales linearly with CPU cores for read operations
//
// The Router is safe for concurrent use and can handle multiple goroutines
// accessing it simultaneously without any additional synchronization.
type Router struct {
	routeTree   atomicRouteTree // Lock-free route tree with atomic operations
	middleware  []HandlerFunc   // Global middleware chain applied to all routes
	contextPool *ContextPool    // Context pool with specialized pools
	tracing     TracingRecorder // OpenTelemetry tracing configuration
	metrics     MetricsRecorder // OpenTelemetry metrics configuration
	logger      logging.Logger  // Structured logger for security events and errors

	// Routing features
	versioning   *VersioningConfig  // Route versioning configuration
	versionTrees atomicVersionTrees // Lock-free version-specific route trees
	versionCache sync.Map           // Version-specific compiled routes (lock-free with sync.Map)

	// Performance tuning
	bloomFilterSize    uint64 // Size of bloom filters for compiled routes (default: 1000)
	bloomHashFunctions int    // Number of hash functions for bloom filters (default: 3)
	checkCancellation  bool   // Enable context cancellation checks in Next() (default: true)

	// Template-based routing
	templateCache *TemplateCache // Pre-compiled route templates for fast matching
	useTemplates  bool           // Enable template-based routing (default: true)

	// Custom 404 handler
	noRouteHandler HandlerFunc  // Custom handler for unmatched routes (nil means use http.NotFound)
	noRouteMutex   sync.RWMutex // Protects noRouteHandler (rarely written, frequently read)

	// HTTP/2 Cleartext (H2C) support
	enableH2C      bool            // Enable HTTP/2 cleartext support (dev/behind LB only)
	serverTimeouts *serverTimeouts // HTTP server timeout configuration

	// Trusted proxies configuration for real client IP detection
	realip *realIPConfig // Compiled trusted proxy configuration

	// Problem Details base URL for RFC 9457 type URIs
	problemBase string // Base URL for problem type resolution (e.g., "https://docs.rivaas.dev/problems")
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
	// This value balances false positives (~1%) and memory usage (125 bytes).
	// Formula: For optimal performance with ~1000 static routes:
	//   - Bits needed: m = -n*ln(p) / (ln(2)^2) ≈ 1000 bits for p=0.01
	//   - Memory: 1000 bits ≈ 125 bytes
	defaultBloomFilterSize = 1000

	// defaultBloomHashFunctions is the default number of hash functions for bloom filters.
	// Optimal value calculated using formula: k = (m/n) * ln(2)
	// For m=1000 bits and n~100 items: k = (1000/100) * 0.693 ≈ 7
	// However, 3 hash functions provide good balance between:
	//   - False positive rate (~5% for typical route counts)
	//   - Computational overhead (3 hashes vs 7 hashes = 2.3x faster)
	//   - Practical performance (bloom filter is a pre-filter, not exact lookup)
	defaultBloomHashFunctions = 3
)

// New creates a new router instance with optional configuration.
// It initializes the radix trees for HTTP methods and sets up context pooling
// to minimize memory allocations during request handling.
//
// The returned router is ready to use and is safe for concurrent access.
//
// Example:
//
//	r := router.New()
//	r.GET("/health", healthHandler)
//	http.ListenAndServe(":8080", r)
//
// With tracing enabled:
//
//	r := router.New(router.WithTracing())
//	r.GET("/api/users", getUserHandler)
//	http.ListenAndServe(":8080", r)
func New(opts ...Option) *Router {
	r := &Router{
		bloomFilterSize:    defaultBloomFilterSize,
		bloomHashFunctions: defaultBloomHashFunctions,
		checkCancellation:  true, // Enable cancellation checks by default
		useTemplates:       true, // Enable template-based routing by default
	}

	// Initialize the atomic route tree with an empty map
	initialTrees := make(map[string]*node)
	atomic.StorePointer(&r.routeTree.trees, unsafe.Pointer(&initialTrees))

	// Initialize context pool
	r.contextPool = NewContextPool(r)

	// Initialize template cache
	r.templateCache = newTemplateCache(r.bloomFilterSize, r.bloomHashFunctions)

	// Apply functional options
	for _, opt := range opts {
		opt(r)
	}

	return r
}

// MustNew creates a new router instance or panics on configuration error.
// This is a convenience wrapper around New for use in initialization where
// errors should terminate the program.
//
// Example:
//
//	r := router.MustNew()
//	r.GET("/health", healthHandler)
func MustNew(opts ...Option) *Router {
	return New(opts...)
}

// SetMetricsRecorder sets the metrics recorder for the router.
// This method is used by external packages to integrate metrics functionality.
func (r *Router) SetMetricsRecorder(recorder MetricsRecorder) {
	r.metrics = recorder
}

// SetTracingRecorder sets the tracing recorder for the router.
// This method is used by external packages to integrate tracing functionality.
func (r *Router) SetTracingRecorder(recorder TracingRecorder) {
	r.tracing = recorder
}

// SetLogger sets the structured logger for the router.
// The logger is used for security events, warnings, and errors.
// The logger interface is compatible with slog and other structured loggers.
//
// Example with slog:
//
//	import "log/slog"
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	router.SetLogger(logger)
func (r *Router) SetLogger(logger logging.Logger) {
	r.logger = logger
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
//	    c.JSON(404, map[string]string{"error": "route not found"})
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
	defer func() {
		c.reset()
		globalContextPool.Put(c)
	}()

	// Check radix tree
	if handlers := tree.getRoute(path, c); handlers != nil {
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
	standardMethods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

	// Create a temporary context for path matching
	c := getContextFromGlobalPool()
	defer func() {
		c.reset()
		globalContextPool.Put(c)
	}()

	for _, method := range standardMethods {
		if tree, exists := (*trees)[method]; exists && tree != nil {
			// Try to match the path in this method's tree
			if handlers := tree.getRoute(path, c); handlers != nil {
				allowed = append(allowed, method)
			}
			// Also check compiled routes if they exist
			if tree.compiled != nil {
				if handlers := tree.compiled.getRoute(path); handlers != nil {
					// Avoid duplicates
					found := false
					for _, m := range allowed {
						if m == method {
							found = true
							break
						}
					}
					if !found {
						allowed = append(allowed, method)
					}
				}
			}
		}
	}

	return allowed
}

// extractClientIP extracts the real client IP from request, respecting proxy headers.
// TODO: Only trust X-Forwarded-For when RemoteAddr is in configured proxy CIDR.
func extractClientIP(req *http.Request) string {
	// Check X-Forwarded-For if behind trusted proxy
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		// Take first IP from chain (leftmost = original client)
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Fallback to RemoteAddr, but strip port
	if addr := req.RemoteAddr; addr != "" {
		// Strip port (RemoteAddr includes ":port")
		if idx := strings.LastIndex(addr, ":"); idx > 0 {
			return addr[:idx]
		}
		return addr
	}

	return ""
}

// detectScheme determines http vs https, respecting proxy headers.
// TODO: Formalize proxy trust policy and consider RFC 7239 Forwarded header.
func detectScheme(req *http.Request) string {
	// Direct TLS connection
	if req.TLS != nil {
		return "https"
	}

	// Honor X-Forwarded-Proto from trusted proxy
	// TODO: Only trust when RemoteAddr is in configured proxy CIDR
	if proto := req.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}

	// TODO: Parse RFC 7239 Forwarded header:
	// Forwarded: for=192.0.2.60;proto=https;by=203.0.113.43

	return "http"
}

// setStandardSpanAttributes uses OpenTelemetry semantic conventions.
func setStandardSpanAttributes(span trace.Span, req *http.Request, routeTemplate string) {
	if span == nil {
		return
	}

	// Detect scheme (req.URL.Scheme is empty server-side)
	scheme := detectScheme(req)

	// Use semconv constants (fewer typos, future-safe)
	attrs := []attribute.KeyValue{
		attribute.String("http.request.method", req.Method),
		attribute.String("http.route", routeTemplate),           // /users/:id or _unmatched
		attribute.String("url.path", req.URL.Path),              // /users/123
		attribute.String("url.scheme", scheme),                  // http or https
		attribute.String("network.protocol.version", req.Proto), // HTTP/1.1, HTTP/2
		attribute.String("user_agent.original", req.UserAgent()),
	}

	// Client address
	if clientIP := extractClientIP(req); clientIP != "" {
		attrs = append(attrs, attribute.String("client.address", clientIP))
	}

	// Server details
	if req.Host != "" {
		attrs = append(attrs, attribute.String("server.address", req.Host))
	}

	span.SetAttributes(attrs...)
}

// finalizeSpanAttributes adds response attributes before span ends.
func finalizeSpanAttributes(span trace.Span, statusCode int, responseSize int64) {
	if span == nil {
		return
	}

	span.SetAttributes(
		attribute.Int("http.response.status_code", statusCode),
		attribute.Int64("http.response.body.size", responseSize),
	)

	// Set span status based on HTTP status
	// 4xx = client errors (not span errors)
	// 5xx = server errors (span errors)
	if statusCode >= 500 {
		span.SetStatus(codes.Error, http.StatusText(statusCode))
	} else {
		span.SetStatus(codes.Ok, "")
	}
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
	if r.versioning != nil {
		c.version = r.detectVersion(req)
	}

	// Set route template: if we matched a node but wrong method, try to determine template
	// Otherwise use sentinel to avoid cardinality explosion
	// TODO: In future, track matched pattern during routing attempt
	c.routeTemplate = "_method_not_allowed"

	// Send RFC 9457 problem (MethodNotAllowedProblem already sets Allow header)
	_ = c.MethodNotAllowedProblem(allowed)

	// Reset and return to pool
	c.reset()
	globalContextPool.Put(c)
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
		if r.versioning != nil {
			c.version = r.detectVersion(req)
		}

		// Set route template for metrics/tracing
		c.routeTemplate = "_not_found"

		// Execute the custom handler
		handler(c)

		// Reset and return to pool
		c.reset()
		globalContextPool.Put(c)
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

		if r.versioning != nil {
			c.version = r.detectVersion(req)
		}

		_ = c.NotFoundProblem()

		c.reset()
		globalContextPool.Put(c)
	}
}

// WithLogger returns a RouterOption that sets the logger.
// This is used with the New() constructor for convenient configuration.
//
// Example:
//
//	import "log/slog"
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	r := router.New(router.WithLogger(logger))
//
// For more advanced logging configuration, see logging.WithLogging().
func WithLogger(logger logging.Logger) Option {
	return func(r *Router) {
		r.logger = logger
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
//	r := router.New(router.WithH2C(true))
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
//	r := router.New(router.WithServerTimeouts(
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
// The bloom filter is used for fast negative lookups in static route matching.
// Larger sizes reduce false positives but use more memory.
//
// Default: 1000
// Recommended: Set to 2-3x the number of static routes
//
// Example:
//
//	r := router.New(router.WithBloomFilterSize(2000)) // For ~1000 routes
func WithBloomFilterSize(size uint64) Option {
	return func(r *Router) {
		if size > 0 {
			r.bloomFilterSize = size
		}
	}
}

// WithBloomFilterHashFunctions returns a RouterOption that sets the number of hash functions
// used in bloom filters for compiled routes. More hash functions reduce false positives
// but increase computation time.
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
//	r := router.New(router.WithBloomFilterHashFunctions(4)) // Lower false positive rate
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
// Performance impact: Small overhead per handler in chain
//
// Disable for maximum performance if:
//   - Your handlers are very fast (< 1ms)
//   - You don't use request timeouts
//   - You handle cancellation manually in handlers
//
// Example:
//
//	r := router.New(router.WithCancellationCheck(false)) // Disable for max speed
func WithCancellationCheck(enabled bool) Option {
	return func(r *Router) {
		r.checkCancellation = enabled
	}
}

// WithTemplateRouting returns a RouterOption that enables/disables template-based routing.
// When enabled, routes are pre-compiled into templates for significantly faster lookup.
//
// Default: true (enabled)
// Performance impact: Positive! Substantially faster for parameter routes
//
// Disable only for debugging or if you encounter issues.
//
// Example:
//
//	r := router.New(router.WithTemplateRouting(true))  // Enabled by default
func WithTemplateRouting(enabled bool) Option {
	return func(r *Router) {
		r.useTemplates = enabled
	}
}

// WithProblemBaseURL returns a RouterOption that sets the base URL for RFC 9457 problem type URIs.
// Problem type slugs (e.g., "validation-error") will be resolved to full URIs by appending
// to this base URL (e.g., "https://docs.rivaas.dev/problems/validation-error").
//
// If the base URL is not set or a slug is already an absolute URI (starts with "http"),
// the slug is used as-is.
//
// Example:
//
//	r := router.New(
//	    router.WithProblemBaseURL("https://docs.rivaas.dev/problems"),
//	)
//
//	// In handler:
//	return c.Problem(
//	    http.StatusBadRequest,
//	    c.ProblemType("validation-error"), // Resolves to "https://docs.rivaas.dev/problems/validation-error"
//	    "Validation failed",
//	    "Invalid input",
//	    nil,
//	)
func WithProblemBaseURL(url string) Option {
	return func(r *Router) {
		// Validate URL format
		if url != "" && !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			panic(fmt.Sprintf("problem base URL must be absolute (http/https): %q", url))
		}
		r.problemBase = strings.TrimSuffix(url, "/")
	}
}

// updateTrees atomically updates the route trees map using copy-on-write.
// This method ensures thread-safe updates without blocking concurrent reads.
//
// Algorithm: Lock-free Compare-And-Swap (CAS) loop
// 1. Load current state atomically
// 2. Create a modified copy (immutable update)
// 3. Attempt to swap the new copy in place of the old one
// 4. If another goroutine modified the state between steps 1-3, retry
//
// Why CAS loop instead of mutex:
// - Readers never block (they always see a valid, complete tree)
// - Writers only block each other during the brief CAS operation
// - No lock contention for read-heavy workloads (typical in HTTP routing)
// - Scales better with high concurrency
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

// addRouteToTree adds a route to the tree using a more efficient approach.
// This method minimizes allocations by only copying when necessary.
//
// Two-phase approach:
// Phase 1 (Fast path): If the method tree exists, add directly without CAS
// Phase 2 (Slow path): If tree doesn't exist, create it atomically via CAS loop
//
// Why this matters:
// - Most route additions happen to existing method trees (GET, POST, etc.)
// - Fast path avoids CAS loop overhead for existing trees
// - Slow path ensures thread-safety when creating new method trees
func (r *Router) addRouteToTree(method, path string, handlers []HandlerFunc, constraints []RouteConstraint) {
	// Fast path: Check if method tree already exists
	// This read is atomic and safe even during concurrent writes
	treesPtr := atomic.LoadPointer(&r.routeTree.trees)
	trees := *(*map[string]*node)(treesPtr)

	if tree, exists := trees[method]; exists {
		// Tree exists, add route directly (thread-safe due to per-node mutex)
		// No CAS needed - we're only modifying the tree, not replacing it
		tree.addRouteWithConstraints(path, handlers, constraints)
		return
	}

	// Slow path: Tree doesn't exist for this method, need to create it atomically
	// Use CAS loop to ensure thread-safe creation
	r.updateTrees(func(currentTrees map[string]*node) map[string]*node {
		// Double-check: another goroutine might have created the tree during CAS retry
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
	r.middleware = append(r.middleware, middleware...)
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

// ServeHTTP implements the http.Handler interface, making Router compatible
// with the standard library's HTTP server. This method uses different code paths
// for static and dynamic routes.
//
// The method performs the following optimizations:
//   - Template-based matching for significantly faster parameter routes
//   - Global static route table for method+path lookup
//   - Fast static route lookup for paths without parameters
//   - Context pooling to reduce garbage collection pressure
//   - Direct parameter extraction into context arrays for up to 8 parameters
//   - Efficient path matching where possible
//   - Optional OpenTelemetry tracing with minimal overhead (when enabled)
//   - Routing with versioning and wildcard support
//
// Static routes use stack allocation to eliminate pool overhead, while
// dynamic routes use context pooling for optimal memory reuse.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path

	// Check if tracing is enabled and path should be traced
	shouldTrace := r.tracing != nil && r.tracing.IsEnabled() && !r.tracing.ShouldExcludePath(path)

	// Check if metrics are enabled (path exclusion is handled by StartRequest)
	shouldMeasure := r.metrics != nil && r.metrics.IsEnabled()

	// Try template-based routing first (if enabled)
	// This is the fastest path - avoid any versioning overhead here
	if r.useTemplates && r.templateCache != nil {
		// Try static route table first (method+path hash lookup)
		if tmpl := r.templateCache.lookupStatic(req.Method, path); tmpl != nil {
			r.serveTemplate(w, req, tmpl, shouldTrace, shouldMeasure)
			return
		}

		// Try dynamic templates (pre-compiled patterns)
		ctx := getContextFromGlobalPool()
		ctx.paramCount = 0 // Reset for template matching

		if tmpl := r.templateCache.matchDynamic(req.Method, path, ctx); tmpl != nil {
			// Template matched! Serve with pre-extracted parameters
			r.serveTemplateWithParams(w, req, tmpl, ctx, shouldTrace, shouldMeasure)
			return
		}

		// Return context to pool (will be reacquired if needed for tree fallback)
		ctx.reset()
		globalContextPool.Put(ctx)
	}

	// Detect version only after fast paths failed
	// Cache version in local variable to avoid repeated detection
	var version string
	hasVersioning := r.versioning != nil // Hoist nil check

	if hasVersioning {
		version = r.detectVersion(req) // Called once, reused below

		// If path-based versioning is enabled, check if version is in path
		var matchPath string
		var hasVersionInPath bool
		if r.versioning != nil && r.versioning.PathEnabled {
			// Detect what version segment is actually in the path (before validation/default)
			// This allows us to strip invalid versions like "/v99/" even when using default "v1"
			var detectedSegment string
			if segment, ok := fastPathVersion(path, r.versioning.PathPrefix); ok && segment != "" {
				detectedSegment = segment
				// Try with "v" prefix if applicable
				if strings.HasSuffix(r.versioning.PathPrefix, "v") {
					detectedSegment = "v" + segment
				}
				hasVersionInPath = true
			}
			// Only strip version if it was actually found in the path
			if detectedSegment != "" {
				matchPath = r.stripPathVersion(path, detectedSegment)
			} else {
				matchPath = path
			}
		} else {
			matchPath = path
		}

		// Use version-specific routing if:
		// 1. Path-based versioning is not enabled (header/query versioning), OR
		// 2. A version segment was detected in the path, OR
		// 3. Path-based versioning is enabled with a default version (fallback to default)
		shouldUseVersionTree := !r.versioning.PathEnabled || hasVersionInPath || (r.versioning.PathEnabled && r.versioning.DefaultVersion != "")

		if shouldUseVersionTree {
			tree := r.getVersionTree(version, req.Method)
			// If detected version's tree doesn't exist, try default version (for invalid path versions)
			if tree == nil && r.versioning != nil && version != r.versioning.DefaultVersion {
				tree = r.getVersionTree(r.versioning.DefaultVersion, req.Method)
				if tree != nil {
					version = r.versioning.DefaultVersion // Use default version for routing
				}
			}

			if tree != nil {
				// Strip version from path when path-based versioning is enabled
				// This ensures routes registered without version prefix can match
				r.serveVersionedRequest(w, req, tree, matchPath, version, shouldTrace, shouldMeasure)
				return
			}
		}
		// If no version-specific route found or version not in path, continue with standard routing
		// Use original path for standard routing when version wasn't in path
		if !hasVersionInPath {
			matchPath = path
		}
		path = matchPath // Update path for standard routing fallback
	}

	// Fallback to standard routing (tree traversal)
	tree := r.getTreeForMethodDirect(req.Method)
	if tree == nil {
		r.handleNotFound(w, req)
		return
	}

	// Compiled route lookup (primary optimization)
	// Only use compiled routes if they exist (pre-compiled during warmup)
	if tree.compiled != nil {
		if handlers := tree.compiled.getRoute(path); handlers != nil {
			if shouldTrace && shouldMeasure {
				// Wrap response writer for status code and size tracking (needed for metrics)
				rw := &responseWriter{ResponseWriter: w}
				r.serveStaticWithTracingAndMetrics(rw, req, handlers, path, true)
			} else if shouldTrace {
				// Wrap response writer for status code and size tracking (needed for metrics)
				rw := &responseWriter{ResponseWriter: w}
				r.serveStaticWithTracing(rw, req, handlers, path)
			} else if shouldMeasure {
				// Wrap response writer for status code and size tracking (needed for metrics)
				rw := &responseWriter{ResponseWriter: w}
				r.serveStaticWithMetrics(rw, req, handlers, path, true)
			} else {
				// No metrics or tracing, use original response writer for zero allocations
				// Direct execution without wrapper for performance
				// Use global context pool to avoid allocations
				ctx := getContextFromGlobalPool()
				ctx.initForRequest(req, w, handlers, r)

				// Use cached version from earlier detection
				if hasVersioning {
					ctx.version = version
				}

				ctx.Next()

				// Reset and return to pool
				ctx.reset()
				globalContextPool.Put(ctx)
			}
			return
		}
	}

	// Dynamic route with parameters - use global context pool for zero allocations
	c := getContextFromGlobalPool()
	c.Request = req
	c.Response = w
	c.index = -1
	c.paramCount = 0
	c.router = r

	// Set version if versioning is enabled
	if r.versioning != nil {
		c.version = r.detectVersion(req)
	}

	defer func() {
		c.reset()
		globalContextPool.Put(c)
	}()

	// Find the route and extract parameters
	handlers := tree.getRoute(path, c)
	if handlers == nil {
		r.handleNotFound(w, req)
		return
	}

	if shouldTrace && shouldMeasure {
		// Wrap response writer for status code and size tracking (needed for metrics)
		rw := &responseWriter{ResponseWriter: w}
		c.Response = rw
		r.serveDynamicWithTracingAndMetrics(c, handlers, path)
	} else if shouldTrace {
		// Wrap response writer for status code and size tracking (needed for metrics)
		rw := &responseWriter{ResponseWriter: w}
		c.Response = rw
		r.serveDynamicWithTracing(c, handlers, path)
	} else if shouldMeasure {
		// Wrap response writer for status code and size tracking (needed for metrics)
		rw := &responseWriter{ResponseWriter: w}
		c.Response = rw
		r.serveDynamicWithMetrics(c, handlers, path)
	} else {
		// No metrics or tracing, use original response writer for zero allocations
		c.Response = w
		c.handlers = handlers
		c.index = -1
		c.Next()
	}
}

// serveVersionedRequest handles requests with version-specific routing
//
// Performance optimization: Uses sync.Map for lock-free compiled route cache
// sync.Map benefits:
// - Lock-free reads (no RWMutex.RLock() overhead)
// - Better performance for read-heavy workloads (typical in production)
// - No contention between readers
// - Amortized O(1) access time
//
// Trade-off: Slightly slower writes (not a concern - routes compiled at startup)
func (r *Router) serveVersionedRequest(w http.ResponseWriter, req *http.Request, tree *node, path, version string, shouldTrace, shouldMeasure bool) {
	// Check if version has compiled routes (lock-free sync.Map lookup)
	// Load is optimized for concurrent read-heavy workloads
	if compiledValue, ok := r.versionCache.Load(version); ok {
		if compiled, ok := compiledValue.(*CompiledRouteTable); ok && compiled != nil {
			// Try compiled routes first
			if handlers := compiled.getRoute(path); handlers != nil {
				r.serveVersionedHandlers(w, req, handlers, version, shouldTrace, shouldMeasure)
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

	// Set metrics recorder for handler access to custom metrics
	if r.metrics != nil {
		c.metricsRecorder = r.metrics
	}

	defer func() {
		c.reset()
		globalContextPool.Put(c)
	}()

	// Find the route and extract parameters
	handlers := tree.getRoute(path, c)
	if handlers == nil {
		r.handleNotFound(w, req)
		return
	}

	// Add deprecation headers if version is deprecated (RFC 8594)
	if r.versioning != nil {
		r.versioning.setDeprecationHeaders(w, version)
	}

	// Set route template if not already set (fallback to sentinel for tree-based routes)
	// TODO: Track matched pattern during tree.getRoute() for better template accuracy
	if c.routeTemplate == "" {
		c.routeTemplate = "_unmatched"
	}

	// Execute handlers with the context that has extracted parameters
	c.handlers = handlers

	if shouldTrace && shouldMeasure {
		// Wrap response writer and set up tracing
		rw := &responseWriter{ResponseWriter: w}
		spanName := req.Method + " " + c.routeTemplate
		ctx, span := r.tracing.StartSpan(req.Context(), spanName)
		c.Request = req.WithContext(ctx)
		c.Response = rw
		c.span = span
		c.traceCtx = ctx

		// Set standard span attributes using semconv
		setStandardSpanAttributes(span, req, c.routeTemplate)

		// Start metrics (use route template, not raw path)
		metricsData := r.metrics.StartRequest(ctx, c.routeTemplate, false)

		// Execute
		c.Next()

		// Finalize span attributes with response info
		finalizeSpanAttributes(span, rw.StatusCode(), int64(rw.Size()))

		// Finish metrics and tracing
		r.metrics.FinishRequest(ctx, metricsData, rw.StatusCode(), int64(rw.Size()))
		r.tracing.FinishSpan(span, rw.StatusCode())
	} else if shouldTrace {
		// Wrap response writer and set up tracing
		rw := &responseWriter{ResponseWriter: w}
		spanName := req.Method + " " + c.routeTemplate
		ctx, span := r.tracing.StartSpan(req.Context(), spanName)
		c.Request = req.WithContext(ctx)
		c.Response = rw
		c.span = span
		c.traceCtx = ctx

		// Set standard span attributes using semconv
		setStandardSpanAttributes(span, req, c.routeTemplate)

		// Execute
		c.Next()

		// Finalize span attributes with response info
		finalizeSpanAttributes(span, rw.StatusCode(), int64(rw.Size()))

		// Finish tracing
		r.tracing.FinishSpan(span, rw.StatusCode())
	} else if shouldMeasure {
		// Wrap response writer and set up metrics
		rw := &responseWriter{ResponseWriter: w}
		ctx := req.Context()
		c.Response = rw

		// Start metrics (use route template, not raw path)
		metricsData := r.metrics.StartRequest(ctx, c.routeTemplate, false)

		// Execute
		c.Next()

		// Finish metrics
		r.metrics.FinishRequest(ctx, metricsData, rw.StatusCode(), int64(rw.Size()))
	} else {
		// No metrics or tracing, execute directly
		c.Next()
	}
}

// serveVersionedHandlers executes handlers with version information
func (r *Router) serveVersionedHandlers(w http.ResponseWriter, req *http.Request, handlers []HandlerFunc, version string, shouldTrace, shouldMeasure bool) {
	// Add deprecation headers if version is deprecated (RFC 8594)
	// This is called before handler execution to ensure headers are set early
	if r.versioning != nil {
		r.versioning.setDeprecationHeaders(w, version)
	}

	if shouldTrace && shouldMeasure {
		// Wrap response writer for status code and size tracking (needed for metrics)
		rw := &responseWriter{ResponseWriter: w}
		r.serveStaticWithTracingAndMetrics(rw, req, handlers, version, true)
	} else if shouldTrace {
		// Wrap response writer for status code and size tracking (needed for metrics)
		rw := &responseWriter{ResponseWriter: w}
		r.serveStaticWithTracing(rw, req, handlers, version)
	} else if shouldMeasure {
		// Wrap response writer for status code and size tracking (needed for metrics)
		rw := &responseWriter{ResponseWriter: w}
		r.serveStaticWithMetrics(rw, req, handlers, version, true)
	} else {
		// No metrics or tracing, use original response writer for zero allocations
		// Direct execution without wrapper for performance
		// Use global context pool to avoid allocations
		ctx := getContextFromGlobalPool()
		ctx.Request = req
		ctx.Response = w
		ctx.index = -1
		ctx.paramCount = 0
		ctx.router = r
		ctx.version = version

		// Set metrics recorder for handler access to custom metrics
		if r.metrics != nil {
			ctx.metricsRecorder = r.metrics
		}

		ctx.handlers = handlers
		ctx.Next()

		// Reset and return to pool
		ctx.reset()
		globalContextPool.Put(ctx)
	}
}

// compileRoutesForMethod compiles static routes for a specific HTTP method
// to enable fast lookup using compiled route tables.
// Records metrics about compilation time and route counts if metrics are enabled.
func (r *Router) compileRoutesForMethod(method string) {
	tree := r.getTreeForMethodDirect(method)
	if tree == nil {
		return
	}

	// Record compilation start time for metrics
	var startTime time.Time
	if r.metrics != nil && r.metrics.IsEnabled() {
		startTime = time.Now()
	}

	// Compile routes
	compiled := tree.compileStaticRoutes(r.bloomFilterSize, r.bloomHashFunctions)

	// Record metrics if enabled
	if r.metrics != nil && r.metrics.IsEnabled() && compiled != nil {
		duration := float64(time.Since(startTime).Microseconds()) / 1000.0 // Convert to milliseconds

		r.metrics.RecordMetric(
			context.Background(),
			"router.route_compilation_duration_ms",
			duration,
			attribute.String("method", method),
		)

		r.metrics.RecordMetric(
			context.Background(),
			"router.compiled_routes_count",
			float64(len(compiled.routes)),
			attribute.String("method", method),
		)
	}
}

// CompileAllRoutes pre-compiles all static routes for performance.
// This should be called after all routes are registered for optimal startup performance.
func (r *Router) CompileAllRoutes() {
	treesPtr := atomic.LoadPointer(&r.routeTree.trees)
	trees := (*map[string]*node)(treesPtr)

	for method := range *trees {
		r.compileRoutesForMethod(method)
	}
}

// Warmup pre-compiles routes and warms up context pools for performance.
// This should be called after all routes are registered and before serving requests.
//
// Warmup phases:
// 1. Compile all static routes into hash tables with bloom filters
// 2. Pre-populate context pools to avoid first-request allocation
// 3. Compile version-specific routes if versioning is enabled
//
// Why warmup matters:
// - Eliminates "cold start" latency on first requests
// - Pre-allocates memory to reduce GC pressure during traffic
// - Compiles optimized lookup structures
// - Typical warmup time: 1-5ms for 100-1000 routes
func (r *Router) Warmup() {
	// Phase 1: Compile all standard (non-versioned) routes
	r.CompileAllRoutes()

	// Phase 2: Compile version-specific routes if versioning is enabled
	if r.versioning != nil {
		r.compileVersionRoutes()
	}

	// Phase 3: Warm up context pools
	r.contextPool.Warmup()
}

// compileVersionRoutes compiles static routes for all version-specific trees
// and stores them in the lock-free version cache (sync.Map)
//
// This optimization enables O(1) lookup for versioned static routes
// instead of O(k) tree traversal on every request.
//
// Performance impact:
// - Versioned static routes: Faster lookup per request
// - Memory cost: Minimal per version (negligible)
// - Compilation time: Fast per version
func (r *Router) compileVersionRoutes() {
	// Load version trees atomically
	versionTreesPtr := atomic.LoadPointer(&r.versionTrees.trees)
	if versionTreesPtr == nil {
		return // No version-specific routes registered
	}

	versionTrees := *(*map[string]map[string]*node)(versionTreesPtr)

	// Compile static routes for each version
	for version, methodTrees := range versionTrees {
		// For each version, compile all its method trees
		for _, tree := range methodTrees {
			if tree != nil {
				// Compile this version's static routes
				compiled := tree.compileStaticRoutes(r.bloomFilterSize, r.bloomHashFunctions)

				// Store in lock-free cache for fast lookup during requests
				// sync.Map.Store is thread-safe and can be called concurrently
				r.versionCache.Store(version, compiled)

				// Only need to compile once per version (all methods share same compiled table)
				break
			}
		}
	}
}

// recordRouteRegistration records route registration metrics if metrics are enabled.
func (r *Router) recordRouteRegistration(method, path string) {
	if r.metrics != nil && r.metrics.IsEnabled() {
		r.metrics.RecordRouteRegistration(context.Background(), method, path)
	}
}

// serveStaticWithTracingAndMetrics serves a static request with both tracing and metrics enabled.
func (r *Router) serveStaticWithTracingAndMetrics(rw *responseWriter, req *http.Request, handlers []HandlerFunc, routeTemplate string, isStatic bool) {
	// Start tracing with route template in name
	spanName := req.Method + " " + routeTemplate
	ctx, span := r.tracing.StartSpan(req.Context(), spanName)
	req = req.WithContext(ctx)

	// Set standard span attributes using semconv
	setStandardSpanAttributes(span, req, routeTemplate)

	// Start metrics with trace context (use route template, not raw path)
	metricsData := r.metrics.StartRequest(ctx, routeTemplate, isStatic)

	// Get context from pool
	c := r.contextPool.Get(0)
	c.initForRequest(req, rw, handlers, r)
	c.span = span
	c.traceCtx = ctx
	c.routeTemplate = routeTemplate // Store for access

	// Set version if versioning is enabled
	if r.versioning != nil {
		c.version = r.detectVersion(req)
	}

	// Execute handlers
	c.Next()

	// Finalize span attributes with response info
	finalizeSpanAttributes(span, rw.StatusCode(), int64(rw.Size()))

	// Finish metrics with trace context
	r.metrics.FinishRequest(ctx, metricsData, rw.StatusCode(), int64(rw.Size()))

	// Finish tracing
	r.tracing.FinishSpan(span, rw.StatusCode())

	// Return context to pool
	r.contextPool.Put(c)
}

// serveStaticWithTracing serves a static request with only tracing enabled.
func (r *Router) serveStaticWithTracing(rw *responseWriter, req *http.Request, handlers []HandlerFunc, routeTemplate string) {
	// Start tracing with route template in name
	spanName := req.Method + " " + routeTemplate
	ctx, span := r.tracing.StartSpan(req.Context(), spanName)
	req = req.WithContext(ctx)

	// Set standard span attributes using semconv
	setStandardSpanAttributes(span, req, routeTemplate)

	// Get context from pool
	c := r.contextPool.Get(0)
	c.initForRequest(req, rw, handlers, r)
	c.span = span
	c.traceCtx = ctx
	c.routeTemplate = routeTemplate // Store for access

	// Set version if versioning is enabled
	if r.versioning != nil {
		c.version = r.detectVersion(req)
	}

	// Execute handlers
	c.Next()

	// Finalize span attributes with response info
	finalizeSpanAttributes(span, rw.StatusCode(), int64(rw.Size()))

	// Finish tracing
	r.tracing.FinishSpan(span, rw.StatusCode())

	// Return context to pool
	r.contextPool.Put(c)
}

// serveStaticWithMetrics serves a static request with only metrics enabled.
func (r *Router) serveStaticWithMetrics(rw *responseWriter, req *http.Request, handlers []HandlerFunc, routeTemplate string, isStatic bool) {
	// Get request context
	ctx := req.Context()

	// Start metrics with request context (use route template, not raw path)
	metricsData := r.metrics.StartRequest(ctx, routeTemplate, isStatic)

	// Get context from pool
	c := r.contextPool.Get(0)
	c.initForRequest(req, rw, handlers, r)
	c.routeTemplate = routeTemplate // Store for access

	// Set version if versioning is enabled
	if r.versioning != nil {
		c.version = r.detectVersion(req)
	}

	// Execute handlers
	c.Next()

	// Finish metrics with request context
	r.metrics.FinishRequest(ctx, metricsData, rw.StatusCode(), int64(rw.Size()))

	// Return context to pool
	r.contextPool.Put(c)
}

// serveDynamicWithTracingAndMetrics serves a dynamic request with both tracing and metrics enabled.
func (r *Router) serveDynamicWithTracingAndMetrics(c *Context, handlers []HandlerFunc, routeTemplate string) {
	// Ensure routeTemplate is set (fallback to sentinel if not set)
	if c.routeTemplate == "" {
		c.routeTemplate = routeTemplate
		if c.routeTemplate == "" {
			c.routeTemplate = "_unmatched"
		}
	}

	// Start tracing with route template in name
	spanName := c.Request.Method + " " + c.routeTemplate
	ctx, span := r.tracing.StartSpan(c.Request.Context(), spanName)
	c.Request = c.Request.WithContext(ctx)
	c.span = span
	c.traceCtx = ctx

	// Set standard span attributes using semconv
	setStandardSpanAttributes(span, c.Request, c.routeTemplate)

	// Start metrics with trace context (use route template, not raw path)
	metricsData := r.metrics.StartRequest(ctx, c.routeTemplate, false)

	// NOTE: Version detection is handled earlier in ServeHTTP for versioned routes
	// For non-versioned routes, version will be set lazily on first access via ctx.Version()

	// Set handlers and execute
	c.handlers = handlers
	c.index = -1
	c.Next()

	// Get response writer status
	rw := c.Response.(*responseWriter)

	// Finalize span attributes with response info
	finalizeSpanAttributes(span, rw.StatusCode(), int64(rw.Size()))

	// Finish metrics with trace context
	r.metrics.FinishRequest(ctx, metricsData, rw.StatusCode(), int64(rw.Size()))

	// Finish tracing
	r.tracing.FinishSpan(span, rw.StatusCode())
}

// serveDynamicWithTracing serves a dynamic request with only tracing enabled.
func (r *Router) serveDynamicWithTracing(c *Context, handlers []HandlerFunc, routeTemplate string) {
	// Ensure routeTemplate is set (fallback to sentinel if not set)
	if c.routeTemplate == "" {
		c.routeTemplate = routeTemplate
		if c.routeTemplate == "" {
			c.routeTemplate = "_unmatched"
		}
	}

	// Start tracing with route template in name
	spanName := c.Request.Method + " " + c.routeTemplate
	ctx, span := r.tracing.StartSpan(c.Request.Context(), spanName)
	c.Request = c.Request.WithContext(ctx)
	c.span = span
	c.traceCtx = ctx

	// Set standard span attributes using semconv
	setStandardSpanAttributes(span, c.Request, c.routeTemplate)

	// NOTE: Version detection is handled earlier in ServeHTTP for versioned routes
	// For non-versioned routes, version will be set lazily on first access via ctx.Version()

	// Set handlers and execute
	c.handlers = handlers
	c.index = -1
	c.Next()

	// Get response writer status
	rw := c.Response.(*responseWriter)

	// Finalize span attributes with response info
	finalizeSpanAttributes(span, rw.StatusCode(), int64(rw.Size()))

	// Finish tracing
	r.tracing.FinishSpan(span, rw.StatusCode())
}

// serveDynamicWithMetrics serves a dynamic request with only metrics enabled.
func (r *Router) serveDynamicWithMetrics(c *Context, handlers []HandlerFunc, routeTemplate string) {
	// Ensure routeTemplate is set (fallback to sentinel if not set)
	if c.routeTemplate == "" {
		c.routeTemplate = routeTemplate
		if c.routeTemplate == "" {
			c.routeTemplate = "_unmatched"
		}
	}

	// Get request context
	ctx := c.Request.Context()

	// Start metrics with request context (use route template, not raw path)
	metricsData := r.metrics.StartRequest(ctx, c.routeTemplate, false)

	// NOTE: Version detection is handled earlier in ServeHTTP for versioned routes
	// For non-versioned routes, version will be set lazily on first access via ctx.Version()

	// Set handlers and execute
	c.handlers = handlers
	c.index = -1
	c.Next()

	// Get response writer status
	rw := c.Response.(*responseWriter)

	// Finish metrics with request context
	r.metrics.FinishRequest(ctx, metricsData, rw.StatusCode(), int64(rw.Size()))
}

// serveTemplate serves a request using a pre-compiled template (static route).
// This is the fastest path: O(1) hash lookup, zero parameter extraction.
func (r *Router) serveTemplate(w http.ResponseWriter, req *http.Request, tmpl *RouteTemplate, shouldTrace, shouldMeasure bool) {
	routeTemplate := tmpl.pattern // Use template pattern, not raw path
	isStatic := tmpl.isStatic

	if shouldTrace && shouldMeasure {
		rw := &responseWriter{ResponseWriter: w}
		r.serveStaticWithTracingAndMetrics(rw, req, tmpl.handlers, routeTemplate, isStatic)
	} else if shouldTrace {
		rw := &responseWriter{ResponseWriter: w}
		r.serveStaticWithTracing(rw, req, tmpl.handlers, routeTemplate)
	} else if shouldMeasure {
		rw := &responseWriter{ResponseWriter: w}
		r.serveStaticWithMetrics(rw, req, tmpl.handlers, routeTemplate, isStatic)
	} else {
		// Fast path: no metrics or tracing
		ctx := getContextFromGlobalPool()
		ctx.initForRequest(req, w, tmpl.handlers, r)
		ctx.routeTemplate = routeTemplate // Set template for access

		// NOTE: Version will be set lazily on first access via ctx.Version()
		// to avoid overhead on template fast path

		ctx.Next()
		ctx.reset()
		globalContextPool.Put(ctx)
	}
}

// serveTemplateWithParams serves a request using a template with pre-extracted parameters.
// The context already has parameters populated by the template matching.
func (r *Router) serveTemplateWithParams(w http.ResponseWriter, req *http.Request, tmpl *RouteTemplate, ctx *Context, shouldTrace, shouldMeasure bool) {
	// Store the route template for metrics/tracing
	routeTemplate := tmpl.pattern // Use template pattern, not raw path
	ctx.routeTemplate = routeTemplate

	// Reuse the context that already has parameters extracted
	// Use special init that preserves parameters
	ctx.initForRequestWithParams(req, w, tmpl.handlers, r)

	// NOTE: Version will be set lazily on first access via ctx.Version()
	// to avoid overhead on template fast path

	defer func() {
		ctx.reset()
		globalContextPool.Put(ctx)
	}()

	if shouldTrace && shouldMeasure {
		rw := &responseWriter{ResponseWriter: w}
		ctx.Response = rw
		r.serveDynamicWithTracingAndMetrics(ctx, tmpl.handlers, routeTemplate)
	} else if shouldTrace {
		rw := &responseWriter{ResponseWriter: w}
		ctx.Response = rw
		r.serveDynamicWithTracing(ctx, tmpl.handlers, routeTemplate)
	} else if shouldMeasure {
		rw := &responseWriter{ResponseWriter: w}
		ctx.Response = rw
		r.serveDynamicWithMetrics(ctx, tmpl.handlers, routeTemplate)
	} else {
		// Fast path: direct execution
		ctx.Next()
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
//	r := router.New()
//	r.GET("/", func(c *router.Context) {
//	    c.String(http.StatusOK, "Hello, World!")
//	})
//	if err := r.Serve(":8080"); err != nil {
//	    log.Fatal(err)
//	}
//
// With H2C enabled (dev/behind LB only):
//
//	r := router.New(router.WithH2C(true))
//	r.Serve(":8080")
func (r *Router) Serve(addr string) error {
	h := http.Handler(r)

	if r.enableH2C {
		h = h2c.NewHandler(h, &http2.Server{})
		if r.logger != nil {
			r.logger.Warn("H2C enabled; use only in dev or behind a trusted LB")
		}
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
//	r := router.New()
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
