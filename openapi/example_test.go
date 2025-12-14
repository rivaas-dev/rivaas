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

package openapi_test

import (
	"context"
	"fmt"
	"net/http"

	"rivaas.dev/openapi"
)

// ExampleNew demonstrates creating a new OpenAPI API definition.
func ExampleNew() {
	api, err := openapi.New(
		openapi.WithTitle("My API", "1.0.0"),
		openapi.WithInfoDescription("API for managing users"),
		openapi.WithServer("http://localhost:8080", "Local development"),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Title: %s, Version: %s\n", api.Info.Title, api.Info.Version)
	// Output: Title: My API, Version: 1.0.0
}

// ExampleMustNew demonstrates creating OpenAPI API definition that panics on error.
func ExampleMustNew() {
	api := openapi.MustNew(
		openapi.WithTitle("My API", "1.0.0"),
		openapi.WithSwaggerUI("/docs"),
	)

	fmt.Printf("UI enabled: %v\n", api.ServeUI)
	// Output: UI enabled: true
}

// ExampleAPI_Generate demonstrates generating an OpenAPI specification.
func ExampleAPI_Generate() {
	api := openapi.MustNew(
		openapi.WithTitle("User API", "1.0.0"),
	)

	// Generate the spec using HTTP method constructors
	result, err := api.Generate(context.Background(),
		openapi.GET("/users/:id",
			openapi.WithSummary("Get user"),
			openapi.WithDescription("Retrieves a user by ID"),
			openapi.WithTags("users"),
		),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Generated spec: %v\n", len(result.JSON) > 0)
	// Output: Generated spec: true
}

// ExampleGET demonstrates creating a GET operation.
func ExampleGET() {
	op := openapi.GET("/users/:id",
		openapi.WithSummary("Get user"),
		openapi.WithDescription("Retrieves a user by ID"),
		openapi.WithResponse(http.StatusOK, User{}),
	)

	fmt.Printf("Method: %s, Path: %s\n", op.Method, op.Path)
	// Output: Method: GET, Path: /users/:id
}

// ExamplePOST demonstrates creating a POST operation.
func ExamplePOST() {
	op := openapi.POST("/users",
		openapi.WithSummary("Create user"),
		openapi.WithRequest(CreateUserRequest{}),
		openapi.WithResponse(http.StatusCreated, User{}),
	)

	fmt.Printf("Method: %s, Path: %s\n", op.Method, op.Path)
	// Output: Method: POST, Path: /users
}

// ExampleDELETE demonstrates creating a DELETE operation.
func ExampleDELETE() {
	op := openapi.DELETE("/users/:id",
		openapi.WithSummary("Delete user"),
		openapi.WithResponse(http.StatusNoContent, nil),
	)

	fmt.Printf("Method: %s, Path: %s\n", op.Method, op.Path)
	// Output: Method: DELETE, Path: /users/:id
}

// ExampleWithSummary demonstrates setting an operation summary.
func ExampleWithSummary() {
	op := openapi.GET("/users",
		openapi.WithSummary("List all users"),
	)

	fmt.Printf("Method: %s, Path: %s\n", op.Method, op.Path)
	// Output: Method: GET, Path: /users
}

// ExampleWithTags demonstrates adding tags to an operation.
func ExampleWithTags() {
	op := openapi.GET("/users/:id",
		openapi.WithTags("users", "admin"),
	)

	fmt.Printf("Method: %s, Path: %s\n", op.Method, op.Path)
	// Output: Method: GET, Path: /users/:id
}

// ExampleWithSecurity demonstrates adding security requirements.
func ExampleWithSecurity() {
	op := openapi.GET("/users/:id",
		openapi.WithSecurity("bearerAuth"),
		openapi.WithSecurity("oauth2", "read:users", "write:users"),
	)

	fmt.Printf("Method: %s, Path: %s\n", op.Method, op.Path)
	// Output: Method: GET, Path: /users/:id
}

// ExampleWithDeprecated demonstrates marking an operation as deprecated.
func ExampleWithDeprecated() {
	op := openapi.GET("/old-endpoint",
		openapi.WithSummary("Old endpoint"),
		openapi.WithDeprecated(),
	)

	fmt.Printf("Method: %s, Path: %s\n", op.Method, op.Path)
	// Output: Method: GET, Path: /old-endpoint
}

// User is an example type for documentation.
type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ExampleWithResponse demonstrates setting response schemas.
func ExampleWithResponse() {
	op := openapi.GET("/users/:id",
		openapi.WithSummary("Get user"),
		openapi.WithResponse(200, User{}),
	)

	fmt.Printf("Method: %s, Path: %s\n", op.Method, op.Path)
	// Output: Method: GET, Path: /users/:id
}

// CreateUserRequest is an example request type.
type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// ExampleWithRequest demonstrates setting request schemas.
func ExampleWithRequest() {
	op := openapi.POST("/users",
		openapi.WithSummary("Create user"),
		openapi.WithRequest(CreateUserRequest{}),
	)

	fmt.Printf("Method: %s, Path: %s\n", op.Method, op.Path)
	// Output: Method: POST, Path: /users
}
