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

	"rivaas.dev/router"
	"rivaas.dev/router/route"
)

// VersionGroup represents a version-specific route group that allows organizing
// related routes under a specific API version.
// VersionGroup supports app.HandlerFunc (with app.Context), providing access to
// binding, validation, and logging features.
//
// Routes registered in a VersionGroup are automatically scoped to that version.
// The version is detected from the request path, headers, query parameters, or other
// configured versioning strategies.
//
// Example:
//
//	v1 := app.Version("v1")
//	v1.GET("/status", handlers.Status)      // handler receives *app.Context
//	v1.POST("/users", handlers.CreateUser)  // handler receives *app.Context
type VersionGroup struct {
	app           *App
	versionRouter *router.VersionRouter
	middleware    []HandlerFunc // Middleware for this version group
	prefix        string        // Path prefix for nested groups
}

// addRoute adds a route to the version group by combining the group's middleware with handlers.
// addRoute is an internal method used by the HTTP method functions.
func (vg *VersionGroup) addRoute(method, path string, handlers []HandlerFunc) *RouteWrapper {
	// Combine group middleware with route handlers
	allHandlers := make([]router.HandlerFunc, 0, len(vg.middleware)+len(handlers))
	for _, m := range vg.middleware {
		allHandlers = append(allHandlers, vg.app.wrapHandler(m))
	}
	for _, h := range handlers {
		allHandlers = append(allHandlers, vg.app.wrapHandler(h))
	}

	// Build full path with prefix
	fullPath := vg.prefix + path

	// Register route with combined handlers
	var rt *route.Route
	switch method {
	case http.MethodGet:
		rt = vg.versionRouter.GET(fullPath, allHandlers...)
	case http.MethodPost:
		rt = vg.versionRouter.POST(fullPath, allHandlers...)
	case http.MethodPut:
		rt = vg.versionRouter.PUT(fullPath, allHandlers...)
	case http.MethodDelete:
		rt = vg.versionRouter.DELETE(fullPath, allHandlers...)
	case http.MethodPatch:
		rt = vg.versionRouter.PATCH(fullPath, allHandlers...)
	case http.MethodHead:
		rt = vg.versionRouter.HEAD(fullPath, allHandlers...)
	case http.MethodOptions:
		rt = vg.versionRouter.OPTIONS(fullPath, allHandlers...)
	}

	vg.app.fireRouteHook(rt)

	return vg.app.wrapRouteWithOpenAPI(rt, method, fullPath)
}

// GET adds a GET route to the version group and returns a RouteWrapper for constraints and OpenAPI documentation.
// It executes any middleware added via Use() before the handler.
//
// Example:
//
//	v1.GET("/users/:id", handler).WhereInt("id")
//	v1.GET("/users/:id", Auth(), GetUser)  // With inline middleware
func (vg *VersionGroup) GET(path string, handlers ...HandlerFunc) *RouteWrapper {
	return vg.addRoute(http.MethodGet, path, handlers)
}

// POST adds a POST route to the version group and returns a RouteWrapper for constraints and OpenAPI documentation.
// It executes any middleware added via Use() before the handler.
//
// Example:
//
//	v1.POST("/users", handler).Request(CreateUserRequest{})
//	v1.POST("/users", Validate(), CreateUser)  // With inline middleware
func (vg *VersionGroup) POST(path string, handlers ...HandlerFunc) *RouteWrapper {
	return vg.addRoute(http.MethodPost, path, handlers)
}

// PUT adds a PUT route to the version group and returns a RouteWrapper for constraints and OpenAPI documentation.
// It executes any middleware added via Use() before the handler.
//
// Example:
//
//	v1.PUT("/users/:id", handler).WhereInt("id")
func (vg *VersionGroup) PUT(path string, handlers ...HandlerFunc) *RouteWrapper {
	return vg.addRoute(http.MethodPut, path, handlers)
}

// DELETE adds a DELETE route to the version group and returns a RouteWrapper for constraints and OpenAPI documentation.
// It executes any middleware added via Use() before the handler.
//
// Example:
//
//	v1.DELETE("/users/:id", handler).WhereInt("id")
func (vg *VersionGroup) DELETE(path string, handlers ...HandlerFunc) *RouteWrapper {
	return vg.addRoute(http.MethodDelete, path, handlers)
}

// PATCH adds a PATCH route to the version group and returns a RouteWrapper for constraints and OpenAPI documentation.
// It executes any middleware added via Use() before the handler.
//
// Example:
//
//	v1.PATCH("/users/:id", handler).WhereInt("id")
func (vg *VersionGroup) PATCH(path string, handlers ...HandlerFunc) *RouteWrapper {
	return vg.addRoute(http.MethodPatch, path, handlers)
}

// HEAD adds a HEAD route to the version group and returns a RouteWrapper for constraints and OpenAPI documentation.
// It executes any middleware added via Use() before the handler.
//
// Example:
//
//	v1.HEAD("/users/:id", handler).WhereInt("id")
func (vg *VersionGroup) HEAD(path string, handlers ...HandlerFunc) *RouteWrapper {
	return vg.addRoute(http.MethodHead, path, handlers)
}

// OPTIONS adds an OPTIONS route to the version group and returns a RouteWrapper
// for constraints and OpenAPI documentation.
// It executes any middleware added via Use() before the handler.
//
// Example:
//
//	v1.OPTIONS("/users", handler)
func (vg *VersionGroup) OPTIONS(path string, handlers ...HandlerFunc) *RouteWrapper {
	return vg.addRoute(http.MethodOptions, path, handlers)
}

// Use adds middleware to the version group that will be executed for all routes in this group.
// Middleware is executed after the router's global middleware but before
// the route-specific handlers.
//
// It applies middleware to all subsequent routes registered in this version group.
//
// Example:
//
//	v1 := app.Version("v1")
//	v1.Use(AuthMiddleware(), LoggingMiddleware())
//	v1.GET("/users", getUsersHandler) // Will execute auth + logging + handler
func (vg *VersionGroup) Use(middleware ...HandlerFunc) {
	vg.middleware = append(vg.middleware, middleware...)
}

// Any registers a route that matches all HTTP methods.
// It is useful for catch-all endpoints like health checks or proxies.
//
// It registers 7 separate routes internally (GET, POST, PUT, DELETE,
// PATCH, HEAD, OPTIONS). For endpoints that only need specific methods,
// use individual method registrations (GET, POST, etc.).
//
// Returns the RouteWrapper for the GET route (most common for docs/constraints).
//
// Example:
//
//	v1.Any("/health", healthCheckHandler)
func (vg *VersionGroup) Any(path string, handlers ...HandlerFunc) *RouteWrapper {
	rw := vg.GET(path, handlers...)
	vg.POST(path, handlers...)
	vg.PUT(path, handlers...)
	vg.DELETE(path, handlers...)
	vg.PATCH(path, handlers...)
	vg.HEAD(path, handlers...)
	vg.OPTIONS(path, handlers...)

	return rw
}

// Group creates a nested version group under the current version group.
// It combines the parent's prefix with the provided prefix.
// It inherits middleware from the parent group.
//
// Example:
//
//	v1 := app.Version("v1")
//	api := v1.Group("/api", AuthMiddleware())  // Creates /api prefix within v1
//	api.GET("/users", handler)                 // Matches /api/users in v1
func (vg *VersionGroup) Group(prefix string, middleware ...HandlerFunc) *VersionGroup {
	// Combine parent middleware with new middleware
	allMiddleware := make([]HandlerFunc, 0, len(vg.middleware)+len(middleware))
	allMiddleware = append(allMiddleware, vg.middleware...)
	allMiddleware = append(allMiddleware, middleware...)

	return &VersionGroup{
		app:           vg.app,
		versionRouter: vg.versionRouter,
		middleware:    allMiddleware,
		prefix:        vg.prefix + prefix,
	}
}
