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
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"rivaas.dev/router/compiler"
)

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
//  1. Compiled static routes (hash table lookup)
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
	// Auto-freeze on first request: ensures routes are immutable during serving.
	// This is a one-time operation that:
	// 1. Registers all pending routes
	// 2. Compiles routes for optimal lookups
	// 3. Freezes routes to prevent further modifications
	//
	// After this point, any attempt to register new routes will panic immediately.
	// This design eliminates data races by making configuration and serving
	// mutually exclusive phases.
	//
	// Note: We use Freeze() which calls Warmup() internally via sync.Once,
	// ensuring route compilation happens exactly once even with concurrent requests.
	r.Freeze()

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

	// Try main tree first (non-versioned routes)
	// Routes registered via r.GET(), r.POST() etc. bypass version detection.
	// Common for infrastructure endpoints like /health, /metrics.

	// Try compiled routes from main tree (if enabled)
	if r.useCompiledRoutes && r.routeCompiler != nil {
		// Try static route table first (only if static routes exist)
		// Checking hasStatic avoids the function call overhead entirely
		if r.routeCompiler.HasStatic() {
			if route := r.routeCompiler.LookupStatic(req.Method, path); route != nil {
				r.serveCompiledRoute(w, req, route, obsState)
				return
			}
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
			c.routePattern = routePattern
			if c.routePattern == "" {
				c.routePattern = "_unmatched"
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

	// No match in main tree - try version-specific trees
	// Routes registered via r.Version().GET() are subject to version detection.

	// Only do version detection if versioning is enabled
	if r.versionEngine != nil {
		vc := r.processVersioning(req, path)

		if vc.tree != nil {
			r.serveVersionedRequest(w, req, vc.tree, vc.routingPath, vc.version, obsState)
			return
		}
	}

	// No match anywhere - return 404
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
	c.routePattern = routePattern
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
		if compiled, compiledOk := compiledValue.(*CompiledRouteTable); compiledOk && compiled != nil {
			// Try compiled routes first - get both handlers and route pattern
			if handlers, routePath := compiled.getRouteWithPath(path); handlers != nil {
				// Use the actual route path (pattern) for observability, version for deprecation headers
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
			w.Write(fmt.Appendf(nil, "API %s was removed. Please upgrade to a supported version.", version))

			return
		}
	}

	// Set route template from the matched pattern
	c.routePattern = routePattern
	if c.routePattern == "" {
		c.routePattern = "_unmatched" // Fallback (should rarely happen)
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
// routePattern is the actual route pattern (e.g., "/test") used for logging/metrics
// version is the API version (e.g., "v1") used for deprecation headers and context
func (r *Router) serveVersionedHandlers(w http.ResponseWriter, req *http.Request, handlers []HandlerFunc, routePattern, version string, obsState any) {
	ctx := req.Context()

	// Set lifecycle headers (deprecation, sunset, etc.) and check if version is past sunset
	// This is called before handler execution to ensure headers are set early
	if r.versionEngine != nil {
		if isSunset := r.versionEngine.SetLifecycleHeaders(w, version, routePattern); isSunset {
			// Version is past sunset date - return 410 Gone
			w.WriteHeader(http.StatusGone)
			w.Write(fmt.Appendf(nil, "API %s was removed. Please upgrade to a supported version.", version))

			return
		}
	}

	// Build request-scoped logger (after routing)
	var logger *slog.Logger
	if r.observability != nil {
		logger = r.observability.BuildRequestLogger(ctx, req, routePattern)
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
	c.routePattern = routePattern // Set route pattern for access log
	c.logger = logger
	c.handlers = handlers

	// Execute
	c.Next()

	// Reset and return to pool
	releaseGlobalContext(c)

	// Finish observability
	if obsState != nil {
		r.observability.OnRequestEnd(ctx, obsState, w, routePattern)
	}
}

// serveCompiledRoute serves a request using a compiled route (static route).
// This path handles routes without parameters.
func (r *Router) serveCompiledRoute(w http.ResponseWriter, req *http.Request, route *compiler.CompiledRoute, obsState any) {
	routePattern := route.Pattern() // Use route pattern, not raw path
	ctx := req.Context()

	// Build request-scoped logger (after routing)
	var logger *slog.Logger
	if r.observability != nil {
		logger = r.observability.BuildRequestLogger(ctx, req, routePattern)
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
		handlers := make([]HandlerFunc, 0, len(route.Handlers()))
		for _, h := range route.Handlers() {
			handlers = append(handlers, h.(HandlerFunc))
		}
		c.initForRequest(req, w, handlers, r)
	}

	c.routePattern = routePattern // Set template for access
	c.logger = logger

	c.Next()

	// Finish observability
	if obsState != nil {
		r.observability.OnRequestEnd(ctx, obsState, w, routePattern)
	}
}

// serveCompiledRouteWithParams serves a request using a compiled route with pre-extracted parameters.
// The context already has parameters populated by the route matching.
func (r *Router) serveCompiledRouteWithParams(w http.ResponseWriter, req *http.Request, route *compiler.CompiledRoute, c *Context, obsState any) {
	// Store the route pattern for metrics/tracing
	routePattern := route.Pattern() // Use route pattern, not raw path
	c.routePattern = routePattern
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
		handlers := make([]HandlerFunc, 0, len(route.Handlers()))
		for _, h := range route.Handlers() {
			handlers = append(handlers, h.(HandlerFunc))
		}
		c.initForRequestWithParams(req, w, handlers, r)
	}

	// Build request-scoped logger (after routing)
	var logger *slog.Logger
	if r.observability != nil {
		logger = r.observability.BuildRequestLogger(ctx, req, routePattern)
	} else {
		logger = noopLogger
	}
	c.logger = logger

	defer releaseGlobalContext(c)

	// Execute handler chain
	c.Next()

	// Finish observability
	if obsState != nil {
		r.observability.OnRequestEnd(ctx, obsState, w, routePattern)
	}
}

// Serve starts the HTTP server on the specified address.
// Automatically enables h2c if configured via WithH2C().
//
// This method follows the stdlib pattern: it blocks until the server exits.
// For graceful shutdown, use the Shutdown method from another goroutine.
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
//
//	// Start server in goroutine
//	go func() {
//	    if err := r.Serve(":8080"); err != nil && err != http.ErrServerClosed {
//	        log.Fatal(err)
//	    }
//	}()
//
//	// Wait for signal
//	quit := make(chan os.Signal, 1)
//	signal.Notify(quit, os.Interrupt)
//	<-quit
//
//	// Graceful shutdown
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//	r.Shutdown(ctx)
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

	// Store server reference for Shutdown
	r.serverMu.Lock()
	r.server = srv
	r.serverMu.Unlock()

	return srv.ListenAndServe()
}

// ServeTLS starts the HTTPS server with TLS configuration.
// For TLS servers, HTTP/2 is automatically enabled via ALPN.
//
// This method follows the stdlib pattern: it blocks until the server exits.
// For graceful shutdown, use the Shutdown method from another goroutine.
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

	// Store server reference for Shutdown
	r.serverMu.Lock()
	r.server = srv
	r.serverMu.Unlock()

	// HTTP/2 is automatically enabled over TLS via ALPN
	// Optional: tune HTTP/2 settings
	// http2.ConfigureServer(srv, &http2.Server{MaxConcurrentStreams: 256})

	return srv.ListenAndServeTLS(certFile, keyFile)
}

// Shutdown gracefully shuts down the server without interrupting active connections.
// This follows the stdlib http.Server.Shutdown pattern.
//
// The provided context controls the timeout for the graceful shutdown.
// When the context is canceled, active connections are forcefully closed.
//
// Shutdown returns nil if no server is running, or the error from http.Server.Shutdown.
//
// Example:
//
//	// In main goroutine
//	go func() {
//	    if err := r.Serve(":8080"); err != nil && err != http.ErrServerClosed {
//	        log.Fatal(err)
//	    }
//	}()
//
//	// Wait for signal
//	quit := make(chan os.Signal, 1)
//	signal.Notify(quit, os.Interrupt)
//	<-quit
//
//	// Graceful shutdown with 30s timeout
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//	if err := r.Shutdown(ctx); err != nil {
//	    log.Printf("Server shutdown error: %v", err)
//	}
func (r *Router) Shutdown(ctx context.Context) error {
	r.serverMu.Lock()
	srv := r.server
	r.server = nil
	r.serverMu.Unlock()

	if srv == nil {
		return nil // No server running
	}

	return srv.Shutdown(ctx)
}
