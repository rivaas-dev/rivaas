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

// Package route provides route definition, grouping, and mounting functionality
// for the Rivaas router.
//
// This package contains:
//   - Route: Represents a registered route with constraints and metadata
//   - Group: Route grouping with shared prefix and middleware
//   - Mount: Subrouter mounting functionality
//   - Constraints: Parameter validation (int, UUID, regex, enum, etc.)
//
// The types in this package are used at application startup during route
// registration and do not affect runtime request handling performance.
//
// # Route Definition
//
// Routes are created through the Router's HTTP method functions:
//
//	r.GET("/users/:id", handler).WhereInt("id")
//	r.POST("/users", handler).SetName("users.create")
//
// # Route Groups
//
// Groups allow organizing routes under a common prefix with shared middleware:
//
//	api := r.Group("/api/v1", authMiddleware)
//	api.GET("/users", listUsers)
//	api.GET("/users/:id", getUser)
//
// # Mounting Subrouters
//
// Subrouters can be mounted at a prefix, preserving route patterns for observability:
//
//	admin := router.MustNew()
//	admin.GET("/users", adminListUsers)
//
//	r.Mount("/admin", admin, route.InheritMiddleware())
//
// # Startup Operations
//
// All operations in this package occur at startup time during route registration.
// The use of interfaces allows the router package to call these functions without
// creating import cycles.
package route
