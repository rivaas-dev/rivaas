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

package app

import (
	"fmt"
	"net/http"
	"strings"

	"rivaas.dev/openapi"
	"rivaas.dev/router/route"
)

// Group represents a route group that allows organizing related routes
// under a common path prefix with shared middleware.
// It enables hierarchical organization of API endpoints and middleware application.
//
// Groups created from App support app.HandlerFunc (with app.Context),
// providing access to binding and validation features.
//
// Example:
//
//	api := app.Group("/api/v1", AuthMiddleware())
//	api.GET("/users", handler)    // handler receives *app.Context
//	api.POST("/users", handler)   // handler receives *app.Context
type Group struct {
	app        *App
	router     *route.Group
	prefix     string        // Track prefix for building full paths
	middleware []HandlerFunc // Group-specific middleware
}

// Use adds middleware to the group that will be executed for all routes in this group.
// Use middleware is executed after the router's global middleware but before
// the route-specific handlers.
//
// Example:
//
//	api := app.Group("/api")
//	api.Use(AuthMiddleware(), LoggingMiddleware())
//	api.GET("/users", getUsersHandler) // Will execute auth + logging + handler
func (g *Group) Use(middleware ...HandlerFunc) {
	g.middleware = append(g.middleware, middleware...)
}

// Group creates a nested route group under the current group.
// It combines the parent's prefix with the provided prefix.
// It inherits middleware from the parent group.
//
// Example:
//
//	api := app.Group("/api")
//	v1 := api.Group("/v1")  // Creates /api/v1 prefix
//	v1.GET("/users", handler)  // Matches /api/v1/users
func (g *Group) Group(prefix string, middleware ...HandlerFunc) *Group {
	// Build full prefix for nested group
	fullPrefix := g.buildFullPath(prefix)

	// Combine parent middleware with new middleware
	allMiddleware := make([]HandlerFunc, 0, len(g.middleware)+len(middleware))
	allMiddleware = append(allMiddleware, g.middleware...)
	allMiddleware = append(allMiddleware, middleware...)

	// Create router group without middleware (we handle it at route registration)
	routerGroup := g.router.Group(prefix)

	return &Group{
		app:        g.app,
		router:     routerGroup,
		prefix:     fullPrefix,
		middleware: allMiddleware,
	}
}

// addRoute adds a route to the group by combining the group's middleware with handlers.
// It returns the underlying route.Route for constraint configuration.
func (g *Group) addRoute(method, path string, handler HandlerFunc, opts ...RouteOption) *route.Route {
	// Capture handler name and caller location before any other operations
	handlerName := getHandlerFuncName(handler)
	// Skip: getCallerLocation(1) → addRoute(2) → GET/POST/etc(3) → user code(4)
	callerLoc := getCallerLocation(3)

	// Apply route options
	cfg := &routeConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Build handler chain: group middleware → before options → handler → after options
	allHandlers := make([]route.Handler, 0, len(g.middleware)+len(cfg.before)+1+len(cfg.after))
	for _, m := range g.middleware {
		allHandlers = append(allHandlers, g.app.wrapHandler(m))
	}
	for _, h := range cfg.before {
		allHandlers = append(allHandlers, g.app.wrapHandler(h))
	}
	allHandlers = append(allHandlers, g.app.wrapHandler(handler))
	for _, h := range cfg.after {
		allHandlers = append(allHandlers, g.app.wrapHandler(h))
	}

	// Register route with combined handlers
	var rt *route.Route
	switch method {
	case http.MethodGet:
		rt = g.router.GET(path, allHandlers...)
	case http.MethodPost:
		rt = g.router.POST(path, allHandlers...)
	case http.MethodPut:
		rt = g.router.PUT(path, allHandlers...)
	case http.MethodDelete:
		rt = g.router.DELETE(path, allHandlers...)
	case http.MethodPatch:
		rt = g.router.PATCH(path, allHandlers...)
	case http.MethodHead:
		rt = g.router.HEAD(path, allHandlers...)
	case http.MethodOptions:
		rt = g.router.OPTIONS(path, allHandlers...)
	}

	// Update route info with actual handler name and caller location
	fullPath := g.buildFullPath(path)
	g.app.router.UpdateRouteInfo(method, fullPath, "", func(info *route.Info) {
		info.HandlerName = fmt.Sprintf("%s (%s)", handlerName, callerLoc)
	})

	// Fire route hook
	g.app.fireRouteHook(rt)

	// Register OpenAPI documentation if enabled
	if g.app.openapi != nil && !cfg.skipDoc && len(cfg.docOpts) > 0 {
		g.app.openapi.AddOperation(openapi.Op(method, fullPath, cfg.docOpts...))
	}

	return rt
}

// GET adds a GET route to the group with the group's prefix.
//
// Example:
//
//	api := app.Group("/api/v1")
//	api.GET("/users", handler)
//	api.GET("/users/:id", getUser,
//	    app.WithDoc(openapi.WithSummary("Get user")),
//	)
func (g *Group) GET(path string, handler HandlerFunc, opts ...RouteOption) *route.Route {
	return g.addRoute(http.MethodGet, path, handler, opts...)
}

// POST adds a POST route to the group with the group's prefix.
//
// Example:
//
//	api := app.Group("/api/v1")
//	api.POST("/users", createUser,
//	    app.WithDoc(
//	        openapi.WithSummary("Create user"),
//	        openapi.WithRequest(CreateUserRequest{}),
//	    ),
//	)
func (g *Group) POST(path string, handler HandlerFunc, opts ...RouteOption) *route.Route {
	return g.addRoute(http.MethodPost, path, handler, opts...)
}

// PUT adds a PUT route to the group with the group's prefix.
//
// Example:
//
//	api := app.Group("/api/v1")
//	api.PUT("/users/:id", updateUser)
func (g *Group) PUT(path string, handler HandlerFunc, opts ...RouteOption) *route.Route {
	return g.addRoute(http.MethodPut, path, handler, opts...)
}

// DELETE adds a DELETE route to the group with the group's prefix.
//
// Example:
//
//	api := app.Group("/api/v1")
//	api.DELETE("/users/:id", deleteUser)
func (g *Group) DELETE(path string, handler HandlerFunc, opts ...RouteOption) *route.Route {
	return g.addRoute(http.MethodDelete, path, handler, opts...)
}

// PATCH adds a PATCH route to the group with the group's prefix.
//
// Example:
//
//	api := app.Group("/api/v1")
//	api.PATCH("/users/:id", patchUser)
func (g *Group) PATCH(path string, handler HandlerFunc, opts ...RouteOption) *route.Route {
	return g.addRoute(http.MethodPatch, path, handler, opts...)
}

// HEAD adds a HEAD route to the group with the group's prefix.
//
// Example:
//
//	api := app.Group("/api/v1")
//	api.HEAD("/users/:id", handler)
func (g *Group) HEAD(path string, handler HandlerFunc, opts ...RouteOption) *route.Route {
	return g.addRoute(http.MethodHead, path, handler, opts...)
}

// OPTIONS adds an OPTIONS route to the group with the group's prefix.
//
// Example:
//
//	api := app.Group("/api/v1")
//	api.OPTIONS("/users", handler)
func (g *Group) OPTIONS(path string, handler HandlerFunc, opts ...RouteOption) *route.Route {
	return g.addRoute(http.MethodOptions, path, handler, opts...)
}

// Any registers a route that matches all HTTP methods.
// It is useful for catch-all endpoints like health checks or proxies.
//
// It registers 7 separate routes internally (GET, POST, PUT, DELETE,
// PATCH, HEAD, OPTIONS). For endpoints that only need specific methods,
// use individual method registrations (GET, POST, etc.).
//
// Returns the GET route (most common for docs/constraints).
//
// Example:
//
//	api := app.Group("/api/v1")
//	api.Any("/health", healthCheckHandler)
func (g *Group) Any(path string, handler HandlerFunc, opts ...RouteOption) *route.Route {
	rt := g.GET(path, handler, opts...)
	g.POST(path, handler, opts...)
	g.PUT(path, handler, opts...)
	g.DELETE(path, handler, opts...)
	g.PATCH(path, handler, opts...)
	g.HEAD(path, handler, opts...)
	g.OPTIONS(path, handler, opts...)

	return rt
}

// buildFullPath builds the full path by combining group prefix with the route path.
func (g *Group) buildFullPath(path string) string {
	if len(g.prefix) == 0 {
		return path
	}
	if len(path) == 0 {
		return g.prefix
	}
	// Both non-empty: concatenate
	var sb strings.Builder
	sb.Grow(len(g.prefix) + len(path))
	_, _ = sb.WriteString(g.prefix)
	_, _ = sb.WriteString(path)

	return sb.String()
}

// Ensure Group uses openapi package (used in addRoute)
var _ = openapi.Op
