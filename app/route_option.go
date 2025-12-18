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
	"rivaas.dev/openapi"
)

// RouteOption configures a route.
// Options can configure middleware (before/after), documentation, or combine multiple options.
// This follows the functional options pattern used throughout the framework.
type RouteOption func(*routeConfig)

// routeConfig accumulates all route configuration.
type routeConfig struct {
	before  []HandlerFunc
	after   []HandlerFunc
	docOpts []openapi.OperationOption
	skipDoc bool // Set to true to explicitly skip documentation
}

// WithBefore adds pre-handler middleware to the route.
// Middleware added with WithBefore executes before the main handler.
//
// Example:
//
//	app.GET("/users/:id", getUser,
//	    app.WithBefore(authMiddleware, rateLimitMiddleware),
//	)
func WithBefore(handlers ...HandlerFunc) RouteOption {
	return func(c *routeConfig) {
		c.before = append(c.before, handlers...)
	}
}

// WithAfter adds post-handler middleware to the route.
// Middleware added with WithAfter executes after the main handler.
//
// Example:
//
//	app.GET("/users/:id", getUser,
//	    app.WithAfter(auditLogMiddleware, metricsMiddleware),
//	)
func WithAfter(handlers ...HandlerFunc) RouteOption {
	return func(c *routeConfig) {
		c.after = append(c.after, handlers...)
	}
}

// WithDoc adds OpenAPI documentation to the route.
// Documentation options are provided by the openapi package.
//
// Example:
//
//	app.GET("/users/:id", getUser,
//	    app.WithDoc(
//	        openapi.Summary("Get user"),
//	        openapi.Description("Retrieves a user by ID"),
//	        openapi.Response(200, UserResponse{}),
//	        openapi.Response(404, ErrorResponse{}),
//	        openapi.Tags("users"),
//	    ),
//	)
func WithDoc(opts ...openapi.OperationOption) RouteOption {
	return func(c *routeConfig) {
		c.docOpts = append(c.docOpts, opts...)
	}
}

// WithoutDoc explicitly disables documentation for this route.
// This is useful when global documentation is enabled but specific routes should be excluded.
//
// Example:
//
//	app.GET("/health", healthCheck,
//	    app.WithoutDoc(),
//	)
func WithoutDoc() RouteOption {
	return func(c *routeConfig) {
		c.skipDoc = true
	}
}

// RouteOptions combines multiple options into a single option.
// This is useful for creating reusable option sets.
//
// Example:
//
//	var Authenticated = app.RouteOptions(
//	    app.WithBefore(authMiddleware),
//	    app.WithDoc(
//	        openapi.Security("bearerAuth"),
//	        openapi.Response(401, UnauthorizedError{}),
//	    ),
//	)
//
//	app.GET("/users/:id", getUser,
//	    Authenticated,
//	    app.WithDoc(
//	        openapi.Summary("Get user"),
//	        openapi.Response(200, UserResponse{}),
//	    ),
//	)
func RouteOptions(opts ...RouteOption) RouteOption {
	return func(c *routeConfig) {
		for _, opt := range opts {
			opt(c)
		}
	}
}
