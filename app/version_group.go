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
	"rivaas.dev/router"
)

// VersionGroup represents a version-specific route group that allows organizing
// related routes under a specific API version.
// VersionGroup supports app.HandlerFunc (with app.Context), providing access to binding, validation, and logging features.
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
}

// GET adds a GET route to the version group and returns a RouteWrapper for constraints and OpenAPI documentation.
// GET executes any middleware added via Use() before the handler.
//
// Example:
//
//	v1.GET("/users/:id", handler).WhereInt("id")
//	v1.GET("/files/:name", handler).WhereRegex("name", `[a-zA-Z0-9._-]+`)
func (vg *VersionGroup) GET(path string, handler HandlerFunc) *RouteWrapper {
	// Combine group middleware with handler
	allHandlers := make([]router.HandlerFunc, len(vg.middleware)+1)
	for i, m := range vg.middleware {
		allHandlers[i] = vg.app.wrapHandler(m)
	}
	allHandlers[len(vg.middleware)] = vg.app.wrapHandler(handler)

	route := vg.versionRouter.GET(path, allHandlers...)
	vg.app.fireRouteHook(*route)
	return vg.app.wrapRouteWithOpenAPI(route, "GET", path)
}

// POST adds a POST route to the version group and returns a RouteWrapper for constraints and OpenAPI documentation.
// POST executes any middleware added via Use() before the handler.
//
// Example:
//
//	v1.POST("/users", handler).Request(CreateUserRequest{})
func (vg *VersionGroup) POST(path string, handler HandlerFunc) *RouteWrapper {
	allHandlers := make([]router.HandlerFunc, len(vg.middleware)+1)
	for i, m := range vg.middleware {
		allHandlers[i] = vg.app.wrapHandler(m)
	}
	allHandlers[len(vg.middleware)] = vg.app.wrapHandler(handler)

	route := vg.versionRouter.POST(path, allHandlers...)
	vg.app.fireRouteHook(*route)
	return vg.app.wrapRouteWithOpenAPI(route, "POST", path)
}

// PUT adds a PUT route to the version group and returns a RouteWrapper for constraints and OpenAPI documentation.
// PUT executes any middleware added via Use() before the handler.
//
// Example:
//
//	v1.PUT("/users/:id", handler).WhereInt("id")
func (vg *VersionGroup) PUT(path string, handler HandlerFunc) *RouteWrapper {
	allHandlers := make([]router.HandlerFunc, len(vg.middleware)+1)
	for i, m := range vg.middleware {
		allHandlers[i] = vg.app.wrapHandler(m)
	}
	allHandlers[len(vg.middleware)] = vg.app.wrapHandler(handler)

	route := vg.versionRouter.PUT(path, allHandlers...)
	vg.app.fireRouteHook(*route)
	return vg.app.wrapRouteWithOpenAPI(route, "PUT", path)
}

// DELETE adds a DELETE route to the version group and returns a RouteWrapper for constraints and OpenAPI documentation.
// DELETE executes any middleware added via Use() before the handler.
//
// Example:
//
//	v1.DELETE("/users/:id", handler).WhereInt("id")
func (vg *VersionGroup) DELETE(path string, handler HandlerFunc) *RouteWrapper {
	allHandlers := make([]router.HandlerFunc, len(vg.middleware)+1)
	for i, m := range vg.middleware {
		allHandlers[i] = vg.app.wrapHandler(m)
	}
	allHandlers[len(vg.middleware)] = vg.app.wrapHandler(handler)

	route := vg.versionRouter.DELETE(path, allHandlers...)
	vg.app.fireRouteHook(*route)
	return vg.app.wrapRouteWithOpenAPI(route, "DELETE", path)
}

// PATCH adds a PATCH route to the version group and returns a RouteWrapper for constraints and OpenAPI documentation.
// PATCH executes any middleware added via Use() before the handler.
//
// Example:
//
//	v1.PATCH("/users/:id", handler).WhereInt("id")
func (vg *VersionGroup) PATCH(path string, handler HandlerFunc) *RouteWrapper {
	allHandlers := make([]router.HandlerFunc, len(vg.middleware)+1)
	for i, m := range vg.middleware {
		allHandlers[i] = vg.app.wrapHandler(m)
	}
	allHandlers[len(vg.middleware)] = vg.app.wrapHandler(handler)

	route := vg.versionRouter.PATCH(path, allHandlers...)
	vg.app.fireRouteHook(*route)
	return vg.app.wrapRouteWithOpenAPI(route, "PATCH", path)
}

// HEAD adds a HEAD route to the version group and returns a RouteWrapper for constraints and OpenAPI documentation.
// HEAD executes any middleware added via Use() before the handler.
//
// Example:
//
//	v1.HEAD("/users/:id", handler).WhereInt("id")
func (vg *VersionGroup) HEAD(path string, handler HandlerFunc) *RouteWrapper {
	allHandlers := make([]router.HandlerFunc, len(vg.middleware)+1)
	for i, m := range vg.middleware {
		allHandlers[i] = vg.app.wrapHandler(m)
	}
	allHandlers[len(vg.middleware)] = vg.app.wrapHandler(handler)

	route := vg.versionRouter.HEAD(path, allHandlers...)
	vg.app.fireRouteHook(*route)
	return vg.app.wrapRouteWithOpenAPI(route, "HEAD", path)
}

// OPTIONS adds an OPTIONS route to the version group and returns a RouteWrapper for constraints and OpenAPI documentation.
// OPTIONS executes any middleware added via Use() before the handler.
//
// Example:
//
//	v1.OPTIONS("/users", handler)
func (vg *VersionGroup) OPTIONS(path string, handler HandlerFunc) *RouteWrapper {
	allHandlers := make([]router.HandlerFunc, len(vg.middleware)+1)
	for i, m := range vg.middleware {
		allHandlers[i] = vg.app.wrapHandler(m)
	}
	allHandlers[len(vg.middleware)] = vg.app.wrapHandler(handler)

	route := vg.versionRouter.OPTIONS(path, allHandlers...)
	vg.app.fireRouteHook(*route)
	return vg.app.wrapRouteWithOpenAPI(route, "OPTIONS", path)
}

// Use adds middleware to the version group that will be executed for all routes in this group.
// Use middleware is executed after the router's global middleware but before
// the route-specific handlers.
//
// Use applies middleware to all subsequent routes registered in this version group.
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
// Any is useful for catch-all endpoints like health checks or proxies.
//
// Any registers 7 separate routes internally (GET, POST, PUT, DELETE,
// PATCH, HEAD, OPTIONS). For endpoints that only need specific methods,
// use individual method registrations (GET, POST, etc.).
//
// Example:
//
//	v1.Any("/health", healthCheckHandler)
func (vg *VersionGroup) Any(path string, handler HandlerFunc) {
	vg.GET(path, handler)
	vg.POST(path, handler)
	vg.PUT(path, handler)
	vg.DELETE(path, handler)
	vg.PATCH(path, handler)
	vg.HEAD(path, handler)
	vg.OPTIONS(path, handler)
}

// Group creates a nested version group under the current version group.
// Group combines the parent's prefix with the provided prefix.
// Group inherits middleware from the parent group.
//
// The returned router.VersionGroup does not support app.HandlerFunc.
// For full app.Context support in nested groups, consider using app.Group()
// with path-based versioning instead.
//
// Example:
//
//	v1 := app.Version("v1")
//	api := v1.Group("/api", AuthMiddleware())  // Creates v1/api prefix
//	api.GET("/users", handler)  // Matches /v1/api/users (depending on versioning config)
func (vg *VersionGroup) Group(prefix string, middleware ...HandlerFunc) *router.VersionGroup {
	routerMiddleware := make([]router.HandlerFunc, len(middleware))
	for i, m := range middleware {
		routerMiddleware[i] = vg.app.wrapHandler(m)
	}
	return vg.versionRouter.Group(prefix, routerMiddleware...)
}
