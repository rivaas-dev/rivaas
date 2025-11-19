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
	"strings"

	"rivaas.dev/router"
)

// Group represents a route group that allows organizing related routes
// under a common path prefix with shared middleware. Groups enable
// hierarchical organization of API endpoints and middleware application.
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
	app    *App
	router *router.Group
	prefix string // Track prefix for building full paths
}

// Use adds middleware to the group that will be executed for all routes in this group.
// Group middleware is executed after the router's global middleware but before
// the route-specific handlers.
//
// Example:
//
//	api := app.Group("/api")
//	api.Use(AuthMiddleware(), LoggingMiddleware())
//	api.GET("/users", getUsersHandler) // Will execute auth + logging + handler
func (g *Group) Use(middleware ...HandlerFunc) {
	routerMiddleware := make([]router.HandlerFunc, len(middleware))
	for i, m := range middleware {
		routerMiddleware[i] = g.app.wrapHandler(m)
	}
	g.router.Use(routerMiddleware...)
}

// Group creates a nested route group under the current group.
// The new group's prefix will be the parent's prefix + the provided prefix.
// Middleware from the parent group is inherited by the nested group.
//
// Example:
//
//	api := app.Group("/api")
//	v1 := api.Group("/v1")  // Creates /api/v1 prefix
//	v1.GET("/users", handler)  // Matches /api/v1/users
func (g *Group) Group(prefix string, middleware ...HandlerFunc) *Group {
	routerMiddleware := make([]router.HandlerFunc, len(middleware))
	for i, m := range middleware {
		routerMiddleware[i] = g.app.wrapHandler(m)
	}
	routerGroup := g.router.Group(prefix, routerMiddleware...)
	// Build full prefix for nested group
	fullPrefix := g.buildFullPath(prefix)
	return &Group{
		app:    g.app,
		router: routerGroup,
		prefix: fullPrefix,
	}
}

// GET adds a GET route to the group with the group's prefix.
// The final route path will be the group prefix + the provided path.
//
// Example:
//
//	api := app.Group("/api/v1")
//	api.GET("/users", handler) // Final path: /api/v1/users
func (g *Group) GET(path string, handler HandlerFunc) *RouteWrapper {
	route := g.router.GET(path, g.app.wrapHandler(handler))
	g.app.fireRouteHook(*route)
	fullPath := g.buildFullPath(path)
	return g.app.wrapRouteWithOpenAPI(route, "GET", fullPath)
}

// POST adds a POST route to the group with the group's prefix.
func (g *Group) POST(path string, handler HandlerFunc) *RouteWrapper {
	route := g.router.POST(path, g.app.wrapHandler(handler))
	g.app.fireRouteHook(*route)
	fullPath := g.buildFullPath(path)
	return g.app.wrapRouteWithOpenAPI(route, "POST", fullPath)
}

// PUT adds a PUT route to the group with the group's prefix.
func (g *Group) PUT(path string, handler HandlerFunc) *RouteWrapper {
	route := g.router.PUT(path, g.app.wrapHandler(handler))
	g.app.fireRouteHook(*route)
	fullPath := g.buildFullPath(path)
	return g.app.wrapRouteWithOpenAPI(route, "PUT", fullPath)
}

// DELETE adds a DELETE route to the group with the group's prefix.
func (g *Group) DELETE(path string, handler HandlerFunc) *RouteWrapper {
	route := g.router.DELETE(path, g.app.wrapHandler(handler))
	g.app.fireRouteHook(*route)
	fullPath := g.buildFullPath(path)
	return g.app.wrapRouteWithOpenAPI(route, "DELETE", fullPath)
}

// PATCH adds a PATCH route to the group with the group's prefix.
func (g *Group) PATCH(path string, handler HandlerFunc) *RouteWrapper {
	route := g.router.PATCH(path, g.app.wrapHandler(handler))
	g.app.fireRouteHook(*route)
	fullPath := g.buildFullPath(path)
	return g.app.wrapRouteWithOpenAPI(route, "PATCH", fullPath)
}

// HEAD adds a HEAD route to the group with the group's prefix.
func (g *Group) HEAD(path string, handler HandlerFunc) *RouteWrapper {
	route := g.router.HEAD(path, g.app.wrapHandler(handler))
	g.app.fireRouteHook(*route)
	fullPath := g.buildFullPath(path)
	return g.app.wrapRouteWithOpenAPI(route, "HEAD", fullPath)
}

// OPTIONS adds an OPTIONS route to the group with the group's prefix.
func (g *Group) OPTIONS(path string, handler HandlerFunc) *RouteWrapper {
	route := g.router.OPTIONS(path, g.app.wrapHandler(handler))
	g.app.fireRouteHook(*route)
	fullPath := g.buildFullPath(path)
	return g.app.wrapRouteWithOpenAPI(route, "OPTIONS", fullPath)
}

// Any registers a route that matches all HTTP methods.
// Useful for catch-all endpoints like health checks or proxies.
//
// Note: This registers 7 separate routes internally (GET, POST, PUT, DELETE,
// PATCH, HEAD, OPTIONS). For endpoints that only need specific methods,
// use individual method registrations (GET, POST, etc.) for better performance.
func (g *Group) Any(path string, handler HandlerFunc) {
	g.GET(path, handler)
	g.POST(path, handler)
	g.PUT(path, handler)
	g.DELETE(path, handler)
	g.PATCH(path, handler)
	g.HEAD(path, handler)
	g.OPTIONS(path, handler)
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
	sb.WriteString(g.prefix)
	sb.WriteString(path)
	return sb.String()
}
