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

// Package router provides an HTTP router for Go with minimal memory allocations.
//
// The router implements a radix tree-based routing algorithm optimized for
// cloud-native applications. It features efficient path matching, zero-allocation
// parameter extraction, and comprehensive middleware support.
//
// # Architecture
//
// The router uses a lock-free, copy-on-write architecture for concurrent
// route registration and request handling:
//   - Route tree: Atomic pointer with CAS loops (no global mutex)
//   - Version trees: Atomic pointer with CAS loops (no global mutex)
//   - Version cache: sync.Map for lock-free compiled route lookups
//   - Per-node locks: Fine-grained RWMutex for concurrent tree modifications
//
// This design ensures:
//   - Request handling never blocks on locks (fully lock-free read path)
//   - Route registration uses optimistic concurrency (minimal contention)
//   - Linear scalability with CPU cores for read operations
//
// # Key Features
//
//   - Fast radix tree routing with O(k) path matching where k is path length
//   - O(1) hash-based lookup for static routes (after compilation)
//   - Zero-allocation parameter extraction for routes with ≤8 parameters
//   - Context pooling with specialized pools for different parameter counts
//   - Route grouping for hierarchical API organization
//   - Template-based routing for pre-compiled route patterns
//   - API versioning support (path, header, query, Accept-based)
//   - OpenTelemetry tracing and metrics integration
//   - Comprehensive request binding (query, params, headers, cookies, body)
//   - Request validation with multiple strategies (interface, tag-based, JSON Schema)
//   - Content negotiation (Accept, Accept-Language, Accept-Encoding)
//   - Built-in middleware (CORS, compression, rate limiting, etc.)
//
// # Performance Characteristics
//
//   - Static routes: O(1) lookup after compilation (hash table)
//   - Parameterized routes: O(k) where k is number of path segments
//   - Memory: Zero allocations for routes with ≤8 parameters
//   - Scalability: Lookup time remains constant regardless of route count
//   - Concurrency: Lock-free read path scales linearly with CPU cores
//
// For current performance benchmarks, see router_bench_test.go.
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
//     This fail-fast approach is appropriate for configuration errors that should be caught during development.
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
//	    ID    int    `params:"id"`
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
