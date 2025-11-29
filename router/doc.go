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

// Package router provides an HTTP router for Go.
//
// The router implements a routing system for cloud-native applications.
// It features path matching, parameter extraction, and comprehensive middleware support.
//
// # Key Features
//
//   - Path matching for static and parameterized routes
//   - Parameter extraction from URL paths
//   - Context pooling for request handling
//   - Route grouping for hierarchical API organization
//   - Template-based routing for route patterns
//   - API versioning support (path, header, query, Accept-based)
//   - OpenTelemetry tracing and metrics integration
//   - Comprehensive request binding (query, params, headers, cookies, body)
//   - Request validation with multiple strategies (interface, tag-based, JSON Schema)
//   - Content negotiation (Accept, Accept-Language, Accept-Encoding)
//   - Built-in middleware (CORS, compression, rate limiting, etc.)
//
// # Routing Details
//
//   - Static routes: Exact path matching
//   - Parameterized routes: Segment-based matching
//
// # Constructor Pattern
//
// The router follows a pragmatic constructor pattern:
//
//   - New() returns *Router (no error) because router initialization cannot fail.
//     The router is a simple data structure that allocates memory and applies options.
//     There is no network I/O, file system access, or external dependencies during construction.
//
//   - Options are validated at application time (e.g., invalid configuration panics on invalid input).
//     This approach is appropriate for configuration errors that should be caught during development.
//
//   - All configuration options use the "With" prefix for consistency (e.g., WithH2C, WithLogger).
//
//   - Grouping options (e.g., WithServerTimeouts) accept multiple related settings to reduce API surface.
//
// This pattern differs from packages that initialize external resources (metrics, tracing, logging),
// which return errors because they may fail to connect to backends or validate complex configurations.
//
// # Quick Start
//
//	package main
//
//	import (
//	    "net/http"
//	    "rivaas.dev/router"
//	)
//
//	func main() {
//	    r := router.MustNew()
//
//	    r.GET("/", func(c *router.Context) {
//	        c.JSON(http.StatusOK, map[string]string{"message": "Hello World"})
//	    })
//
//	    r.GET("/users/:id", func(c *router.Context) {
//	        c.JSON(http.StatusOK, map[string]string{"user_id": c.Param("id")})
//	    })
//
//	    http.ListenAndServe(":8080", r)
//	}
//
// # Request Binding
//
// The router provides comprehensive request binding with support for 15+ types:
//
//	type UserRequest struct {
//	    ID    int    `path:"id"`
//	    Name  string `query:"name"`
//	    Email string `json:"email"`
//	}
//
//	func handler(c *router.Context) {
//	    var req UserRequest
//	    if err := c.Bind(&req); err != nil {
//	        c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
//	        return
//	    }
//	    // Use req...
//	}
//
// # Middleware
//
// Middleware can be applied globally, to route groups, or to individual routes:
//
//	r.Use(middleware.Logger())
//	r.Use(middleware.Recovery())
//
//	api := r.Group("/api")
//	api.Use(middleware.Auth())
//	api.GET("/users", handler)
//
// # API Versioning
//
// The router supports multiple versioning strategies:
//
//	r := router.MustNew(
//	    router.WithVersioning(
//	        router.VersionByPath("/v{version}"),
//	        router.VersionByHeader("X-API-Version"),
//	    ),
//	)
//
// # Content Negotiation
//
// The router implements RFC 7231-compliant content negotiation.
// Content negotiation allows HTTP servers to serve different representations
// of resources based on client preferences expressed in Accept-* headers.
//
// Supported Headers:
//
//   - Accept: MIME type negotiation (application/json, text/html, etc.)
//   - Accept-Charset: Character encoding negotiation (utf-8, iso-8859-1, etc.)
//   - Accept-Encoding: Compression negotiation (gzip, br, deflate, etc.)
//   - Accept-Language: Language negotiation (en, fr, es, etc.)
//
// Usage Examples:
//
// Basic content type negotiation:
//
//	r.GET("/users/:id", func(c *router.Context) {
//	    switch c.Accepts("json", "xml", "html") {
//	    case "json":
//	        c.JSON(200, user)
//	    case "xml":
//	        c.XML(200, user)
//	    case "html":
//	        c.HTML(200, "user.html", user)
//	    default:
//	        c.Status(406) // Not Acceptable
//	    }
//	})
//
// Compression negotiation:
//
//	encoding := c.AcceptsEncodings("br", "gzip", "deflate")
//	if encoding != "" {
//	    c.Response.Header().Set("Content-Encoding", encoding)
//	    // ... compress response
//	}
//
// RFC 7231 Compliance:
//
//   - Quality values (q-values) respected: 0.0 to 1.0 range
//   - Specificity rules: exact > subtype wildcard > type wildcard
//   - Media type parameters ignored for matching (except 'q')
//   - Wildcards supported: */* and type/*
//   - Language prefix matching (e.g., "en" matches "en-US")
//
// # Observability
//
// OpenTelemetry integration for metrics and tracing:
//
//	r := router.MustNew(
//	    router.WithMetricsRecorder(metrics),
//	    router.WithTracer(tracer),
//	)
//
// # Examples
//
// See the examples directory for complete working examples:
//   - Basic routing and handlers
//   - Middleware usage
//   - Request binding and validation
//   - API versioning
//   - Content negotiation
//   - Advanced routing patterns
package router
