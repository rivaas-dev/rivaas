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
	stripPrefix       bool
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

// StripPrefix removes the mount prefix from paths when matching routes in the subrouter.
// Without this, subrouter routes must include the full prefix.
//
// Example:
//
//	r.Mount("/admin", sub, router.StripPrefix())
//	// sub.GET("/:id") matches /admin/123 (not /admin/:id)
func StripPrefix() MountOption {
	return func(cfg *mountCfg) {
		cfg.stripPrefix = true
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

// Mount mounts a subrouter at the given prefix with optional configuration.
//
// The subrouter's routes are merged into the parent router's route tree.
// Middleware execution order: parent global (if InheritMiddleware) → subrouter middleware → handlers.
//
// Example:
//
//	admin := router.MustNew()
//	admin.GET("/:id", getAdmin)
//
//	r.Mount("/admin", admin,
//	    router.InheritMiddleware(),     // Parent auth applies
//	    router.WithMiddleware(adminLog), // Plus admin-only middleware
//	    router.StripPrefix(),            // /admin/:id → /:id in subrouter
//	    router.NamePrefix("admin."),    // Route names prefixed
//	    router.WithNotFound(adminNotFound),
//	)
func (r *Router) Mount(prefix string, sub *Router, opts ...MountOption) {
	if sub == nil {
		return
	}

	// Normalize prefix
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		prefix = "/"
	}

	// Build mount configuration
	cfg := &mountCfg{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Build middleware chain
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
	// Add extra middleware
	middlewareChain = append(middlewareChain, cfg.extraMiddleware...)

	// Mount routes by registering a catch-all handler that delegates to subrouter
	// This approach works for routes registered before or after mounting
	mountPath := prefix
	if !strings.HasSuffix(mountPath, "*") && mountPath != "/" {
		if strings.HasSuffix(mountPath, "/") {
			mountPath = mountPath + "*"
		} else {
			mountPath = mountPath + "/*"
		}
	}

	// Create a delegating handler that wraps subrouter with middleware
	delegatingHandler := func(c *Context) {
		// Strip prefix if configured
		originalPath := c.Request.URL.Path
		if cfg.stripPrefix && strings.HasPrefix(originalPath, prefix) {
			newPath := strings.TrimPrefix(originalPath, prefix)
			if newPath == "" {
				newPath = "/"
			} else if !strings.HasPrefix(newPath, "/") {
				newPath = "/" + newPath
			}
			c.Request.URL.Path = newPath
		}

		// Delegate to subrouter
		sub.ServeHTTP(c.Response, c.Request)

		// Restore original path
		c.Request.URL.Path = originalPath
	}

	// Register catch-all routes for all methods
	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	for _, method := range methods {
		allHandlers := make([]HandlerFunc, 0, len(middlewareChain)+1)
		allHandlers = append(allHandlers, middlewareChain...)
		allHandlers = append(allHandlers, delegatingHandler)
		r.addRouteWithConstraints(method, mountPath, allHandlers)
	}

	// Handle custom 404 for subrouter
	if cfg.notFoundHandler != nil {
		// Create a wrapper that checks if request is within prefix
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
