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
	"net/http"
	"strings"
)

// mountCfg holds configuration for a mounted subrouter.
type mountCfg struct {
	inheritMiddleware bool
	extraMiddleware   []HandlerFunc
	namePrefix        string
	notFoundHandler   HandlerFunc
}

// MountOption configures how a subrouter is mounted.
type MountOption func(*mountCfg)

// InheritMiddleware makes the subrouter inherit parent router's global middleware.
// Parent middleware runs before subrouter middleware.
func InheritMiddleware() MountOption {
	return func(cfg *mountCfg) {
		cfg.inheritMiddleware = true
	}
}

// WithMiddleware adds additional middleware to the subrouter.
// These middleware run after inherited middleware but before route handlers.
func WithMiddleware(m ...HandlerFunc) MountOption {
	return func(cfg *mountCfg) {
		cfg.extraMiddleware = append(cfg.extraMiddleware, m...)
	}
}

// NamePrefix adds a prefix to all route names in the subrouter.
// Useful for metrics and logging scoping.
//
// Example:
//
//	r.Mount("/admin", sub, router.NamePrefix("admin."))
//	// Route named "users" becomes "admin.users"
func NamePrefix(prefix string) MountOption {
	return func(cfg *mountCfg) {
		cfg.namePrefix = prefix
	}
}

// WithNotFound sets a custom 404 handler for the subrouter.
// This handler is only used when no route matches within the subrouter's prefix.
func WithNotFound(h HandlerFunc) MountOption {
	return func(cfg *mountCfg) {
		cfg.notFoundHandler = h
	}
}

// Mount mounts a subrouter at the given prefix by merging routes into the parent router.
//
// Routes from the subrouter are copied with the prefix prepended, preserving the full
// route pattern for observability (metrics, tracing, logging). This ensures route
// templates like "/admin/users/:id" are correctly recorded instead of catch-all patterns.
//
// Middleware execution order: parent global (if InheritMiddleware) → subrouter middleware → handlers.
//
// Example:
//
//	admin := router.MustNew()
//	admin.GET("/users/:id", getUser)
//	admin.POST("/users", createUser)
//
//	r.Mount("/admin", admin,
//	    router.InheritMiddleware(),      // Parent auth applies
//	    router.WithMiddleware(adminLog), // Plus admin-only middleware
//	    router.NamePrefix("admin."),     // Route names prefixed
//	    router.WithNotFound(adminNotFound),
//	)
//	// Results in routes: GET /admin/users/:id, POST /admin/users
//	// Observability will see "/admin/users/:id" not "/admin/*"
func (r *Router) Mount(prefix string, sub *Router, opts ...MountOption) {
	if sub == nil {
		return
	}

	// Normalize prefix: ensure it starts with / and doesn't end with /
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" || prefix[0] != '/' {
		prefix = "/" + prefix
	}

	// Build mount configuration
	cfg := &mountCfg{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Build middleware chain for mounted routes
	var middlewareChain []HandlerFunc
	if cfg.inheritMiddleware {
		// Copy parent's global middleware
		r.middlewareMu.RLock()
		middlewareChain = make([]HandlerFunc, len(r.middleware))
		copy(middlewareChain, r.middleware)
		r.middlewareMu.RUnlock()
	}
	// Add subrouter's global middleware
	sub.middlewareMu.RLock()
	middlewareChain = append(middlewareChain, sub.middleware...)
	sub.middlewareMu.RUnlock()
	// Add extra middleware from mount options
	middlewareChain = append(middlewareChain, cfg.extraMiddleware...)

	// Merge routes from subrouter into parent router
	// This preserves the full route pattern for observability
	r.mergeSubrouterRoutes(prefix, sub, middlewareChain, cfg.namePrefix)

	// Handle custom 404 for subrouter prefix
	if cfg.notFoundHandler != nil {
		originalNoRoute := r.noRouteHandler
		r.NoRoute(func(c *Context) {
			path := c.Request.URL.Path
			if strings.HasPrefix(path, prefix) {
				// Request is within subrouter prefix, use subrouter's 404
				cfg.notFoundHandler(c)
			} else if originalNoRoute != nil {
				// Use parent's 404
				originalNoRoute(c)
			} else {
				// Default 404
				c.Status(http.StatusNotFound)
			}
		})
	}
}

// mergeSubrouterRoutes copies routes from the subrouter into the parent router
// with the mount prefix prepended. This preserves full route patterns for observability.
func (r *Router) mergeSubrouterRoutes(prefix string, sub *Router, middlewareChain []HandlerFunc, namePrefix string) {
	// Collect routes from subrouter's pending routes (not yet registered)
	sub.pendingRoutesMu.Lock()
	pendingRoutes := make([]*Route, len(sub.pendingRoutes))
	copy(pendingRoutes, sub.pendingRoutes)
	sub.pendingRoutesMu.Unlock()

	// Register each pending route with prefix
	for _, route := range pendingRoutes {
		r.mountRoute(prefix, route, middlewareChain, namePrefix)
	}

	// Also check routeTree.routes for already-registered routes (after warmup)
	sub.routeTree.routesMutex.RLock()
	routeInfos := make([]RouteInfo, len(sub.routeTree.routes))
	copy(routeInfos, sub.routeTree.routes)
	sub.routeTree.routesMutex.RUnlock()

	// For routes that were registered directly to tree (post-warmup),
	// we need to traverse the tree to get handlers
	if len(routeInfos) > 0 && len(pendingRoutes) == 0 {
		// Subrouter was already warmed up, need to copy from trees
		r.mergeFromSubrouterTrees(prefix, sub, middlewareChain, namePrefix)
	}
}

// mountRoute registers a single route from the subrouter with the mount prefix.
func (r *Router) mountRoute(prefix string, route *Route, middlewareChain []HandlerFunc, namePrefix string) {
	// Build full path: prefix + route path
	var fullPath string
	if route.path == "/" {
		fullPath = prefix
	} else {
		fullPath = prefix + route.path
	}

	// Combine middleware chain with route handlers
	allHandlers := make([]HandlerFunc, 0, len(middlewareChain)+len(route.handlers))
	allHandlers = append(allHandlers, middlewareChain...)
	allHandlers = append(allHandlers, route.handlers...)

	// Register the route in parent router
	newRoute := r.addRouteWithConstraints(route.method, fullPath, allHandlers)

	// Copy regex constraints - extract pattern string from compiled regex
	if len(route.constraints) > 0 {
		for _, constraint := range route.constraints {
			// Extract pattern from compiled regex (remove ^...$ anchors if present)
			pattern := constraint.Pattern.String()
			if len(pattern) >= 2 && pattern[0] == '^' && pattern[len(pattern)-1] == '$' {
				pattern = pattern[1 : len(pattern)-1]
			}
			newRoute.Where(constraint.Param, pattern)
		}
	}

	// Copy typed constraints directly
	if len(route.typedConstraints) > 0 {
		newRoute.mu.Lock()
		newRoute.ensureTypedConstraints()
		for param, constraint := range route.typedConstraints {
			newRoute.typedConstraints[param] = constraint
		}
		newRoute.mu.Unlock()
	}

	// Set route name with prefix
	if route.name != "" {
		newRoute.SetName(namePrefix + route.name)
	}

	// Copy metadata
	if route.description != "" {
		newRoute.SetDescription(route.description)
	}
	if len(route.tags) > 0 {
		newRoute.SetTags(route.tags...)
	}
}

// mergeFromSubrouterTrees extracts routes from an already-warmed-up subrouter's trees.
func (r *Router) mergeFromSubrouterTrees(prefix string, sub *Router, middlewareChain []HandlerFunc, namePrefix string) {
	// Get all method trees from subrouter
	treesPtr := sub.routeTree.loadTrees()
	if treesPtr == nil {
		return
	}
	trees := *treesPtr

	for method, tree := range trees {
		// Traverse the tree and extract routes
		r.extractAndMountFromNode(prefix, method, "", tree, middlewareChain, namePrefix)
	}
}

// extractAndMountFromNode recursively extracts routes from a radix tree node.
func (r *Router) extractAndMountFromNode(prefix, method, currentPath string, n *node, middlewareChain []HandlerFunc, namePrefix string) {
	if n == nil {
		return
	}

	n.mu.RLock()
	handlers := n.handlers
	nodePath := n.path
	constraints := n.constraints
	children := make(map[string]*node, len(n.children))
	for k, v := range n.children {
		children[k] = v
	}
	paramNode := n.param
	wildcardNode := n.wildcard
	n.mu.RUnlock()

	// If this node has handlers, register it
	if len(handlers) > 0 && nodePath != "" {
		fullPath := prefix + nodePath

		// Combine middleware with handlers
		allHandlers := make([]HandlerFunc, 0, len(middlewareChain)+len(handlers))
		allHandlers = append(allHandlers, middlewareChain...)
		allHandlers = append(allHandlers, handlers...)

		// Register route
		newRoute := r.addRouteWithConstraints(method, fullPath, allHandlers)

		// Copy regex constraints - extract pattern string from compiled regex
		if len(constraints) > 0 {
			for _, constraint := range constraints {
				pattern := constraint.Pattern.String()
				if len(pattern) >= 2 && pattern[0] == '^' && pattern[len(pattern)-1] == '$' {
					pattern = pattern[1 : len(pattern)-1]
				}
				newRoute.Where(constraint.Param, pattern)
			}
		}
	}

	// Recursively process children
	for _, child := range children {
		r.extractAndMountFromNode(prefix, method, currentPath, child, middlewareChain, namePrefix)
	}

	// Process param node
	if paramNode != nil && paramNode.node != nil {
		r.extractAndMountFromNode(prefix, method, currentPath, paramNode.node, middlewareChain, namePrefix)
	}

	// Process wildcard node
	if wildcardNode != nil && wildcardNode.node != nil {
		r.extractAndMountFromNode(prefix, method, currentPath, wildcardNode.node, middlewareChain, namePrefix)
	}
}
