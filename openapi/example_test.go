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

//go:build !integration

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
		openapi.WithDescription("API for managing users"),
		openapi.WithServer("http://localhost:8080", "Local development"),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Title: %s, Version: %s\n", api.Info().Title, api.Info().Version)
	// Output: Title: My API, Version: 1.0.0
}

// ExampleMustNew demonstrates creating OpenAPI API definition that panics on error.
func ExampleMustNew() {
	api := openapi.MustNew(
		openapi.WithTitle("My API", "1.0.0"),
		openapi.WithSwaggerUI("/docs"),
	)

	fmt.Printf("UI enabled: %v\n", api.ServeUI())
	// Output: UI enabled: true
}

// ExampleAPI_Generate demonstrates generating an OpenAPI specification.
func ExampleAPI_Spec() {
	api := openapi.MustNew(
		openapi.WithTitle("User API", "1.0.0"),
	)

	op, err := openapi.WithGET("/users/:id",
		openapi.WithSummary("Get user"),
		openapi.WithOperationDescription("Retrieves a user by ID"),
		openapi.WithTags("users"),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	api.AddOperation(op)
	result, err := api.Spec(context.Background())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Generated spec: %v\n", len(result.JSON) > 0)
	// Output: Generated spec: true
}

// ExampleWithGET demonstrates creating a GET operation.
func ExampleWithGET() {
	op, err := openapi.WithGET("/users/:id",
		openapi.WithSummary("Get user"),
		openapi.WithOperationDescription("Retrieves a user by ID"),
		openapi.WithResponse(http.StatusOK, User{}),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Method: %s, Path: %s\n", op.Method, op.Path)
	// Output: Method: GET, Path: /users/:id
}

// ExampleWithPOST demonstrates creating a POST operation.
func ExampleWithPOST() {
	op, err := openapi.WithPOST("/users",
		openapi.WithSummary("Create user"),
		openapi.WithRequest(CreateUserRequest{}),
		openapi.WithResponse(http.StatusCreated, User{}),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Method: %s, Path: %s\n", op.Method, op.Path)
	// Output: Method: POST, Path: /users
}

// ExampleWithDELETE demonstrates creating a DELETE operation.
func ExampleWithDELETE() {
	op, err := openapi.WithDELETE("/users/:id",
		openapi.WithSummary("Delete user"),
		openapi.WithResponse(http.StatusNoContent, nil),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Method: %s, Path: %s\n", op.Method, op.Path)
	// Output: Method: DELETE, Path: /users/:id
}

// ExampleWithSummary demonstrates setting an operation summary.
func ExampleWithSummary() {
	op, err := openapi.WithGET("/users", openapi.WithSummary("List all users"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Method: %s, Path: %s\n", op.Method, op.Path)
	// Output: Method: GET, Path: /users
}

// ExampleWithTags demonstrates adding tags to an operation.
func ExampleWithTags() {
	op, err := openapi.WithGET("/users/:id", openapi.WithTags("users", "admin"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Method: %s, Path: %s\n", op.Method, op.Path)
	// Output: Method: GET, Path: /users/:id
}

// ExampleWithSecurity demonstrates adding security requirements.
func ExampleWithSecurity() {
	op, err := openapi.WithGET("/users/:id",
		openapi.WithSecurity("bearerAuth"),
		openapi.WithSecurity("oauth2", "read:users", "write:users"),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Method: %s, Path: %s\n", op.Method, op.Path)
	// Output: Method: GET, Path: /users/:id
}

// ExampleWithDeprecated demonstrates marking an operation as deprecated.
func ExampleWithDeprecated() {
	op, err := openapi.WithGET("/old-endpoint",
		openapi.WithSummary("Old endpoint"),
		openapi.WithDeprecated(),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

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
	op, err := openapi.WithGET("/users/:id",
		openapi.WithSummary("Get user"),
		openapi.WithResponse(200, User{}),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

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
	op, err := openapi.WithPOST("/users",
		openapi.WithSummary("Create user"),
		openapi.WithRequest(CreateUserRequest{}),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Method: %s, Path: %s\n", op.Method, op.Path)
	// Output: Method: POST, Path: /users
}
