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

package route

import (
	"strings"
)

// Group represents a route group that allows organizing related routes
// under a common path prefix with shared middleware. Groups enable
// hierarchical organization of API endpoints and middleware application.
//
// Groups inherit the parent router's global middleware and can add their own
// group-specific middleware. The final handler chain for a grouped route will be:
// [global middleware...] + [group middleware...] + [route handlers...]
//
// Example:
//
//	api := r.Group("/api/v1", AuthMiddleware())
//	users := api.Group("/users", RateLimitMiddleware())
//	users.GET("/:id", getUserHandler) // Final path: /api/v1/users/:id
type Group struct {
	registrar  Registrar // Reference to the parent router (implements Registrar)
	prefix     string    // Path prefix for all routes in this group
	middleware []Handler // Group-specific middleware
	namePrefix string    // Name prefix for all routes in this group (e.g., "api.v1.")
}

// NewGroup creates a new Group with the given registrar, prefix, and middleware.
func NewGroup(registrar Registrar, prefix string, middleware []Handler) *Group {
	return &Group{
		registrar:  registrar,
		prefix:     prefix,
		middleware: middleware,
	}
}

// Use adds middleware to the group that will be executed for all routes in this group.
// Group middleware is executed after the router's global middleware but before
// the route-specific handlers.
//
// Example:
//
//	api := r.Group("/api")
//	api.Use(AuthMiddleware(), LoggingMiddleware())
//	api.GET("/users", getUsersHandler) // Will execute auth + logging + handler
func (g *Group) Use(middleware ...Handler) {
	g.middleware = append(g.middleware, middleware...)
}

// SetNamePrefix sets a prefix for all route names in this group.
// The prefix is appended to any existing name prefix from parent groups,
// enabling hierarchical route naming.
// Returns the group for method chaining.
//
// Example:
//
//	api := r.Group("/api").SetNamePrefix("api.")
//	v1 := api.Group("/v1").SetNamePrefix("v1.")  // Inherits "api." prefix
//	v1.GET("/users", handler).SetName("users.list")
//	// Full route name becomes: "api.v1.users.list"
func (g *Group) SetNamePrefix(prefix string) *Group {
	g.namePrefix = g.namePrefix + prefix
	return g
}

// NamePrefix returns the current name prefix for this group.
func (g *Group) NamePrefix() string {
	return g.namePrefix
}

// Group creates a nested route group under the current group.
// The new group's prefix will be the parent's prefix + the provided prefix.
// Middleware and name prefix from the parent group are inherited by the nested group.
//
// Example:
//
//	api := r.Group("/api")
//	v1 := api.Group("/v1")  // Creates /api/v1 prefix
//	v1.GET("/users", handler)  // Matches /api/v1/users
func (g *Group) Group(prefix string, middleware ...Handler) *Group {
	var fullPrefix string
	if len(g.prefix) == 0 {
		fullPrefix = prefix
	} else if len(prefix) == 0 {
		fullPrefix = g.prefix
	} else {
		var sb strings.Builder
		sb.Grow(len(g.prefix) + len(prefix))
		sb.WriteString(g.prefix)
		sb.WriteString(prefix)
		fullPrefix = sb.String()
	}

	// Combine parent middleware with new middleware
	allMiddleware := make([]Handler, 0, len(g.middleware)+len(middleware))
	allMiddleware = append(allMiddleware, g.middleware...)
	allMiddleware = append(allMiddleware, middleware...)

	return &Group{
		registrar:  g.registrar,
		prefix:     fullPrefix,
		middleware: allMiddleware,
		namePrefix: g.namePrefix, // Inherit parent's name prefix
	}
}

// GET adds a GET route to the group with the group's prefix.
// The final route path will be the group prefix + the provided path.
//
// Example:
//
//	api := r.Group("/api/v1")
//	api.GET("/users", handler) // Final path: /api/v1/users
func (g *Group) GET(path string, handlers ...Handler) *Route {
	return g.addRoute("GET", path, handlers)
}

// POST adds a POST route to the group with the group's prefix.
// The final route path will be the group prefix + the provided path.
//
// Example:
//
//	api := r.Group("/api/v1")
//	api.POST("/users", handler) // Final path: /api/v1/users
func (g *Group) POST(path string, handlers ...Handler) *Route {
	return g.addRoute("POST", path, handlers)
}

// PUT adds a PUT route to the group with the group's prefix.
// The final route path will be the group prefix + the provided path.
//
// Example:
//
//	api := r.Group("/api/v1")
//	api.PUT("/users/:id", handler) // Final path: /api/v1/users/:id
func (g *Group) PUT(path string, handlers ...Handler) *Route {
	return g.addRoute("PUT", path, handlers)
}

// DELETE adds a DELETE route to the group with the group's prefix.
// The final route path will be the group prefix + the provided path.
//
// Example:
//
//	api := r.Group("/api/v1")
//	api.DELETE("/users/:id", handler) // Final path: /api/v1/users/:id
func (g *Group) DELETE(path string, handlers ...Handler) *Route {
	return g.addRoute("DELETE", path, handlers)
}

// PATCH adds a PATCH route to the group with the group's prefix.
// The final route path will be the group prefix + the provided path.
//
// Example:
//
//	api := r.Group("/api/v1")
//	api.PATCH("/users/:id", handler) // Final path: /api/v1/users/:id
func (g *Group) PATCH(path string, handlers ...Handler) *Route {
	return g.addRoute("PATCH", path, handlers)
}

// OPTIONS adds an OPTIONS route to the group with the group's prefix.
// The final route path will be the group prefix + the provided path.
//
// Example:
//
//	api := r.Group("/api/v1")
//	api.OPTIONS("/users", handler) // Final path: /api/v1/users
func (g *Group) OPTIONS(path string, handlers ...Handler) *Route {
	return g.addRoute("OPTIONS", path, handlers)
}

// HEAD adds a HEAD route to the group with the group's prefix.
// The final route path will be the group prefix + the provided path.
//
// Example:
//
//	api := r.Group("/api/v1")
//	api.HEAD("/users/:id", handler) // Final path: /api/v1/users/:id
func (g *Group) HEAD(path string, handlers ...Handler) *Route {
	return g.addRoute("HEAD", path, handlers)
}

// addRoute adds a route to the group by combining the group's prefix with the path
// and merging group middleware with the route handlers. This is an internal method
// used by the HTTP method functions on groups.
func (g *Group) addRoute(method, path string, handlers []Handler) *Route {
	var fullPath string

	if len(g.prefix) == 0 {
		fullPath = path
	} else if len(path) == 0 {
		fullPath = g.prefix
	} else {
		var sb strings.Builder
		sb.Grow(len(g.prefix) + len(path))
		sb.WriteString(g.prefix)
		sb.WriteString(path)
		fullPath = sb.String()
	}

	allHandlers := make([]Handler, 0, len(g.middleware)+len(handlers))
	allHandlers = append(allHandlers, g.middleware...)
	allHandlers = append(allHandlers, handlers...)

	route := g.registrar.AddRouteWithConstraints(method, fullPath, allHandlers)
	// Set group reference for name prefixing
	route.SetGroup(g)

	return route
}
