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
	"fmt"

	"rivaas.dev/openapi"
)

// ExampleNew demonstrates creating a new OpenAPI configuration.
func ExampleNew() {
	cfg, err := openapi.New(
		openapi.WithTitle("My API", "1.0.0"),
		openapi.WithDescription("API for managing users"),
		openapi.WithServer("http://localhost:8080", "Local development"),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Title: %s, Version: %s\n", cfg.Info.Title, cfg.Info.Version)
	// Output: Title: My API, Version: 1.0.0
}

// ExampleMustNew demonstrates creating OpenAPI configuration that panics on error.
func ExampleMustNew() {
	cfg := openapi.MustNew(
		openapi.WithTitle("My API", "1.0.0"),
		openapi.WithSwaggerUI(true, "/docs"),
	)

	fmt.Printf("UI enabled: %v\n", cfg.ServeUI)
	// Output: UI enabled: true
}

// ExampleNewManager demonstrates creating an OpenAPI manager.
func ExampleNewManager() {
	cfg := openapi.MustNew(
		openapi.WithTitle("My API", "1.0.0"),
		openapi.WithDescription("API documentation"),
	)

	manager := openapi.NewManager(cfg)
	if manager == nil {
		fmt.Println("Manager is nil")
		return
	}

	fmt.Println("Manager created successfully")
	// Output: Manager created successfully
}

// ExampleManager_Register demonstrates registering routes for OpenAPI documentation.
func ExampleManager_Register() {
	cfg := openapi.MustNew(
		openapi.WithTitle("User API", "1.0.0"),
	)

	manager := openapi.NewManager(cfg)

	// Register a route
	route := manager.Register("GET", "/users/:id")
	route.Doc("Get user", "Retrieves a user by ID").
		Tags("users")

	fmt.Println("Route registered")
	// Output: Route registered
}

// ExampleWithBearerAuth demonstrates configuring Bearer authentication.
func ExampleWithBearerAuth() {
	cfg := openapi.MustNew(
		openapi.WithTitle("My API", "1.0.0"),
		openapi.WithBearerAuth("bearerAuth", "JWT authentication"),
	)

	fmt.Printf("Security schemes: %d\n", len(cfg.SecuritySchemes))
	// Output: Security schemes: 1
}

// ExampleWithSwaggerUI demonstrates enabling Swagger UI.
func ExampleWithSwaggerUI() {
	cfg := openapi.MustNew(
		openapi.WithTitle("My API", "1.0.0"),
		openapi.WithSwaggerUI(true, "/docs"),
	)

	fmt.Printf("UI enabled: %v, UI path: %s\n", cfg.ServeUI, cfg.UIPath)
	// Output: UI enabled: true, UI path: /docs
}

// ExampleWithServer demonstrates adding server URLs.
func ExampleWithServer() {
	cfg := openapi.MustNew(
		openapi.WithTitle("My API", "1.0.0"),
		openapi.WithServer("http://localhost:8080", "Local development"),
		openapi.WithServer("https://api.example.com", "Production"),
	)

	fmt.Printf("Servers: %d\n", len(cfg.Servers))
	// Output: Servers: 2
}

// ExampleWithTag demonstrates adding API tags.
func ExampleWithTag() {
	cfg := openapi.MustNew(
		openapi.WithTitle("My API", "1.0.0"),
		openapi.WithTag("users", "User management operations"),
		openapi.WithTag("orders", "Order management operations"),
	)

	fmt.Printf("Tags: %d\n", len(cfg.Tags))
	// Output: Tags: 2
}

// ExampleRouteWrapper_Doc demonstrates documenting a route.
func ExampleRouteWrapper_Doc() {
	cfg := openapi.MustNew(
		openapi.WithTitle("My API", "1.0.0"),
	)

	manager := openapi.NewManager(cfg)
	route := manager.Register("POST", "/users")
	route.Doc("Create user", "Creates a new user account").
		Tags("users")

	fmt.Println("Route documented")
	// Output: Route documented
}

// ExampleRouteWrapper_Request demonstrates documenting request body.
func ExampleRouteWrapper_Request() {
	type CreateUserRequest struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	cfg := openapi.MustNew(
		openapi.WithTitle("My API", "1.0.0"),
	)

	manager := openapi.NewManager(cfg)
	route := manager.Register("POST", "/users")
	route.Doc("Create user", "Creates a new user").
		Request(CreateUserRequest{})

	fmt.Println("Request body documented")
	// Output: Request body documented
}

// ExampleRouteWrapper_Response demonstrates documenting responses.
func ExampleRouteWrapper_Response() {
	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	cfg := openapi.MustNew(
		openapi.WithTitle("My API", "1.0.0"),
	)

	manager := openapi.NewManager(cfg)
	route := manager.Register("GET", "/users/:id")
	route.Doc("Get user", "Retrieves a user by ID").
		Response(200, User{}).
		Response(404, map[string]string{"error": "User not found"})

	fmt.Println("Responses documented")
	// Output: Responses documented
}
