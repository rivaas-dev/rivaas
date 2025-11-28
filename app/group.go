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
	"net/http"
	"strings"

	"rivaas.dev/router"
)

// Group represents a route group that allows organizing related routes
// under a common path prefix with shared middleware.
// Group enables hierarchical organization of API endpoints and middleware application.
//
// Groups created from App support app.HandlerFunc (with app.Context),
// providing access to binding and validation features.
//
// Example:
//
//	api := app.Group("/api/v1", AuthMiddleware())
//	api.GET("/users", handler)    // handler receives *app.Context
//	api.POST("/users", handler)    // handler receives *app.Context
type Group struct {
	app        *App
	router     *router.Group
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
// Group combines the parent's prefix with the provided prefix.
// Group inherits middleware from the parent group.
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
// addRoute returns a [RouteWrapper] for route configuration and OpenAPI documentation.
// addRoute is an internal method used by the HTTP method functions.
func (g *Group) addRoute(method, path string, handlers []HandlerFunc) *RouteWrapper {
	// Combine group middleware with route handlers
	allHandlers := make([]router.HandlerFunc, 0, len(g.middleware)+len(handlers))
	for _, m := range g.middleware {
		allHandlers = append(allHandlers, g.app.wrapHandler(m))
	}
	for _, h := range handlers {
		allHandlers = append(allHandlers, g.app.wrapHandler(h))
	}

	// Register route with combined handlers
	var route *router.Route
	switch method {
	case http.MethodGet:
		route = g.router.GET(path, allHandlers...)
	case http.MethodPost:
		route = g.router.POST(path, allHandlers...)
	case http.MethodPut:
		route = g.router.PUT(path, allHandlers...)
	case http.MethodDelete:
		route = g.router.DELETE(path, allHandlers...)
	case http.MethodPatch:
		route = g.router.PATCH(path, allHandlers...)
	case http.MethodHead:
		route = g.router.HEAD(path, allHandlers...)
	case http.MethodOptions:
		route = g.router.OPTIONS(path, allHandlers...)
	}

	g.app.fireRouteHook(route)
	fullPath := g.buildFullPath(path)
	return g.app.wrapRouteWithOpenAPI(route, method, fullPath)
}

// GET adds a GET route to the group with the group's prefix.
// GET combines the group prefix with the provided path.
//
// Example:
//
//	api := app.Group("/api/v1")
//	api.GET("/users", handler)                    // Single handler
//	api.GET("/users/:id", Auth(), GetUser)        // With inline middleware
func (g *Group) GET(path string, handlers ...HandlerFunc) *RouteWrapper {
	return g.addRoute(http.MethodGet, path, handlers)
}

// POST adds a POST route to the group with the group's prefix.
// POST combines the group prefix with the provided path.
//
// Example:
//
//	api := app.Group("/api/v1")
//	api.POST("/users", handler)                   // Single handler
//	api.POST("/users", Validate(), CreateUser)    // With inline middleware
func (g *Group) POST(path string, handlers ...HandlerFunc) *RouteWrapper {
	return g.addRoute(http.MethodPost, path, handlers)
}

// PUT adds a PUT route to the group with the group's prefix.
// PUT combines the group prefix with the provided path.
//
// Example:
//
//	api := app.Group("/api/v1")
//	api.PUT("/users/:id", handler)                // Single handler
func (g *Group) PUT(path string, handlers ...HandlerFunc) *RouteWrapper {
	return g.addRoute(http.MethodPut, path, handlers)
}

// DELETE adds a DELETE route to the group with the group's prefix.
// DELETE combines the group prefix with the provided path.
//
// Example:
//
//	api := app.Group("/api/v1")
//	api.DELETE("/users/:id", handler)             // Single handler
func (g *Group) DELETE(path string, handlers ...HandlerFunc) *RouteWrapper {
	return g.addRoute(http.MethodDelete, path, handlers)
}

// PATCH adds a PATCH route to the group with the group's prefix.
// PATCH combines the group prefix with the provided path.
//
// Example:
//
//	api := app.Group("/api/v1")
//	api.PATCH("/users/:id", handler)              // Single handler
func (g *Group) PATCH(path string, handlers ...HandlerFunc) *RouteWrapper {
	return g.addRoute(http.MethodPatch, path, handlers)
}

// HEAD adds a HEAD route to the group with the group's prefix.
// HEAD combines the group prefix with the provided path.
//
// Example:
//
//	api := app.Group("/api/v1")
//	api.HEAD("/users/:id", handler)               // Single handler
func (g *Group) HEAD(path string, handlers ...HandlerFunc) *RouteWrapper {
	return g.addRoute(http.MethodHead, path, handlers)
}

// OPTIONS adds an OPTIONS route to the group with the group's prefix.
// OPTIONS combines the group prefix with the provided path.
//
// Example:
//
//	api := app.Group("/api/v1")
//	api.OPTIONS("/users", handler)                // Single handler
func (g *Group) OPTIONS(path string, handlers ...HandlerFunc) *RouteWrapper {
	return g.addRoute(http.MethodOptions, path, handlers)
}

// Any registers a route that matches all HTTP methods.
// Any is useful for catch-all endpoints like health checks or proxies.
//
// Any registers 7 separate routes internally (GET, POST, PUT, DELETE,
// PATCH, HEAD, OPTIONS). For endpoints that only need specific methods,
// use individual method registrations (GET, POST, etc.).
//
// Returns the RouteWrapper for the GET route (most common for docs/constraints).
//
// Example:
//
//	api := app.Group("/api/v1")
//	api.Any("/health", healthCheckHandler)
func (g *Group) Any(path string, handlers ...HandlerFunc) *RouteWrapper {
	rw := g.GET(path, handlers...)
	g.POST(path, handlers...)
	g.PUT(path, handlers...)
	g.DELETE(path, handlers...)
	g.PATCH(path, handlers...)
	g.HEAD(path, handlers...)
	g.OPTIONS(path, handlers...)
	return rw
}

// buildFullPath builds the full path by combining group prefix with the route path.
// buildFullPath is a private helper used internally.
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
	sb.WriteString(g.prefix)
	sb.WriteString(path)
	return sb.String()
}
