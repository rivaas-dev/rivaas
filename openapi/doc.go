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

// Package openapi provides OpenAPI 3.0.4 and 3.1.2 specification generation and Swagger UI integration for Rivaas.
//
// This package enables automatic generation of OpenAPI specifications from Go code using struct tags
// and reflection. It integrates seamlessly with the Rivaas router to provide comprehensive API
// documentation with minimal boilerplate.
//
// # Features
//
//   - Automatic parameter discovery from struct tags (query, path, header, cookie)
//   - Request/response body schema generation from Go types
//   - Swagger UI integration with customizable appearance
//   - Semantic operation ID generation based on HTTP method and path
//   - Support for security schemes (Bearer, API Key, OAuth)
//   - ETag-based caching for spec serving
//   - Collision-resistant schema naming (pkgname.TypeName format)
//
// # Quick Start
//
//	import (
//	    "net/http"
//	    "rivaas.dev/openapi"
//	)
//
//	// Create OpenAPI configuration
//	cfg := openapi.MustNew(
//	    openapi.WithTitle("My API", "1.0.0"),
//	    openapi.WithDescription("API description"),
//	    openapi.WithBearerAuth("bearerAuth", "JWT authentication"),
//	    openapi.WithServer("http://localhost:8080", "Local development"),
//	    openapi.WithSwaggerUI(true, "/docs"),
//	)
//
//	// Create manager and register routes
//	manager := openapi.NewManager(cfg)
//	route := manager.Register("GET", "/users/:id")
//	route.Doc("Get user", "Retrieves a user by ID").
//	    Request(GetUserRequest{}).
//	    Response(200, UserResponse{}).
//	    Tags("users")
//
//	// Generate OpenAPI specification
//	specJSON, etag, err := manager.GenerateSpec()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Serve the specification via HTTP (example)
//	http.HandleFunc(cfg.SpecPath, func(w http.ResponseWriter, r *http.Request) {
//	    w.Header().Set("Content-Type", "application/json")
//	    w.Header().Set("ETag", etag)
//	    w.Write(specJSON)
//	})
//
// # Configuration
//
// Configuration is done exclusively through functional options using [New] or [MustNew].
// All UI configuration types are private to enforce this pattern and prevent direct struct initialization.
//
// # Auto-Discovery
//
// The package automatically discovers API parameters from struct tags compatible with
// the binding package:
//
//   - query: Query parameters
//   - params: Path parameters
//   - header: Header parameters
//   - cookie: Cookie parameters
//   - json: Request body fields
//
// Example:
//
//	type GetUserRequest struct {
//	    ID    int    `params:"id" doc:"User ID" example:"123"`
//	    Expand string `query:"expand" doc:"Fields to expand" enum:"profile,settings"`
//	}
//
// This automatically generates OpenAPI parameters without manual specification.
//
// # Schema Naming
//
// Component schema names use the format "pkgname.TypeName" to prevent
// cross-package type name collisions. For example, types from different
// packages with the same name (e.g., "api.User" and "models.User") will
// generate distinct schema names in the OpenAPI specification.
//
// The package name is extracted from the last component of the package path
// (e.g., "github.com/user/api" -> "api"). This ensures that types with the
// same name from different packages don't collide in the components/schemas
// section of the generated OpenAPI specification.
//
// # Operation IDs
//
// Operation IDs are automatically generated from HTTP method and path using semantic naming:
//
//   - GET /users -> getUsers
//   - GET /users/:id -> getUserById
//   - POST /users -> createUser
//   - PATCH /users/:id -> updateUserById
//   - PUT /users/:id -> replaceUserById
//
// Custom operation IDs can be set using [RouteWrapper.OperationID].
package openapi
