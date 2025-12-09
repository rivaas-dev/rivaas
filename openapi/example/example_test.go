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

package example_test

import (
	"fmt"
	"net/http"

	"rivaas.dev/openapi"
	"rivaas.dev/openapi/example"
)

// ExampleNew demonstrates creating a named example.
func ExampleNew() {
	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	ex := example.New("success", User{ID: 123, Name: "John"})
	fmt.Printf("Name: %s, Value: %+v\n", ex.Name(), ex.Value())
	// Output: Name: success, Value: {ID:123 Name:John}
}

// ExampleNew_withOptions demonstrates creating an example with options.
func ExampleNew_withOptions() {
	ex := example.New("admin", map[string]any{"id": 1, "role": "admin"},
		example.WithSummary("Admin user response"),
		example.WithDescription("Users with admin role have elevated permissions"),
	)

	fmt.Printf("Summary: %s\n", ex.Summary())
	// Output: Summary: Admin user response
}

// ExampleNewExternal demonstrates creating an external example.
func ExampleNewExternal() {
	ex := example.NewExternal("large-dataset", "https://api.example.com/examples/large.json",
		example.WithSummary("Large response dataset"),
	)

	fmt.Printf("External: %v, URL: %s\n", ex.IsExternal(), ex.ExternalValue())
	// Output: External: true, URL: https://api.example.com/examples/large.json
}

// ExampleRouteWrapper_Response_namedExamples demonstrates response with named examples.
func ExampleResponse_namedExamples() {
	type UserResponse struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	cfg := openapi.MustNew(
		openapi.WithTitle("My API", "1.0.0"),
	)

	manager := openapi.NewManager(cfg)
	route := manager.Register(http.MethodGet, "/users/:id")
	route.Doc("Get user", "Retrieves a user by ID").
		Response(200, UserResponse{},
			example.New("regular", UserResponse{ID: 123, Name: "John"},
				example.WithSummary("Regular user")),
			example.New("admin", UserResponse{ID: 1, Name: "Admin"},
				example.WithSummary("Admin user")),
		)

	fmt.Println("Response with named examples registered")
	// Output: Response with named examples registered
}
