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

// Package openapi provides OpenAPI 3.0.4 and 3.1.2 specification generation for Go applications.
//
// This package enables automatic generation of OpenAPI specifications from Go code using struct tags
// and reflection. It provides a pure, stateless API for building specifications with minimal boilerplate.
//
// # Features
//
//   - Operation builders (WithGET, WithPOST, WithPUT, etc.) for clean operation definitions
//   - Automatic parameter discovery from struct tags (query, path, header, cookie)
//   - Request/response body schema generation from Go types
//   - Swagger UI integration with customizable appearance
//   - Semantic operation ID generation based on HTTP method and path
//   - Support for security schemes (Bearer, API Key, OAuth2, OpenID Connect)
//   - Collision-resistant schema naming (pkgname.TypeName format)
//   - Built-in validation against official OpenAPI meta-schemas
//   - Standalone validator for external OpenAPI specifications
//
// # Quick Start
//
// Declarative: pass operations at construction.
//
//	api := openapi.MustNew(
//	    openapi.WithTitle("My API", "1.0.0"),
//	    openapi.WithDescription("API description"),
//	    openapi.WithBearerAuth("bearerAuth", "JWT"),
//	    openapi.WithServer("http://localhost:8080", "Local development"),
//	    openapi.WithOperations(
//	        openapi.WithGET("/users/:id", openapi.WithSummary("Get user"), openapi.WithResponse(200, UserResponse{}), openapi.WithTags("users"), openapi.WithSecurity("bearerAuth")),
//	        openapi.WithPOST("/users", openapi.WithSummary("Create user"), openapi.WithRequest(CreateUserRequest{}), openapi.WithResponse(201, UserResponse{}), openapi.WithTags("users")),
//	    ),
//	)
//	result, err := api.Spec(context.Background())
//
// Incremental: add operations after construction.
//
//	api := openapi.MustNew(openapi.WithTitle("My API", "1.0.0"))
//	op, _ := openapi.WithGET("/users/:id", openapi.WithSummary("Get user"), openapi.WithResponse(200, UserResponse{}))
//	api.AddOperation(op)
//	result, err := api.Spec(context.Background())
//
// # Configuration vs Operations
//
// The package uses two distinct types of options, both with the With* prefix:
//
//   - API options configure the spec: WithTitle, WithServer, WithBearerAuth
//   - Operation options configure routes: WithSummary, WithDescription, WithResponse, WithTags
//
// API configuration is read-only after creation. Use getters to read values (e.g. Info(), SpecPath(), ServeUI(), UI()).
//
// Example:
//
//	api := openapi.MustNew(
//	    openapi.WithTitle("My API", "1.0.0"),  // API option
//	)
//
//	openapi.WithGET("/users/:id",
//	    openapi.WithSummary("Get user"),       // Operation option
//	    openapi.WithResponse(200, User{}),     // Operation option
//	)
//
// # Auto-Discovery
//
// The package automatically discovers API parameters from struct tags compatible with
// the binding package:
//
//   - query: Query parameters
//   - path: Path parameters
//   - header: Header parameters
//   - cookie: Cookie parameters
//   - json: Request body fields
//
// Example:
//
//	type GetUserRequest struct {
//	    ID     int    `path:"id" doc:"User ID" example:"123"`
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
// Custom operation IDs can be set using the WithOperationID option.
//
// # Validation
//
// Generated specifications can be validated against the official OpenAPI meta-schemas.
// Validation is opt-in to avoid performance overhead:
//
//	api := openapi.MustNew(
//	    openapi.WithTitle("My API", "1.0.0"),
//	    openapi.WithValidateSpec(true), // Enable validation
//	)
//
//	result, err := api.Spec(context.Background())
//	if err != nil {
//	    log.Fatal(err) // Will fail if spec is invalid
//	}
//
// The validate subpackage provides standalone validation for external OpenAPI specs:
//
//	import "rivaas.dev/openapi/validate"
//
//	// Validate any OpenAPI spec
//	specJSON, _ := os.ReadFile("openapi.json")
//	if err := validate.ValidateSpecJSON(specJSON); err != nil {
//	    log.Fatal(err)
//	}
package openapi
