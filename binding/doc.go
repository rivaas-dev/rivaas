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

// Package binding provides request data binding for HTTP handlers.
//
// The binding package maps values from various sources (query parameters,
// form data, JSON bodies, headers, cookies, path parameters) into Go structs
// using struct tags. It supports nested structs, slices, maps, pointers,
// custom types, default values, enum validation, and type conversion.
//
// # Key Features
//
//   - Multiple data sources: query parameters, form data, JSON, headers, cookies, path parameters
//   - Type conversion: automatic conversion from strings to Go types
//   - Nested structures: support for nested structs and maps
//   - Default values: struct tag-based default values
//   - Enum validation: validate values against allowed sets
//   - Custom converters: register custom type converters
//   - Unknown field handling: configurable handling of unknown JSON fields
//   - Observability: event hooks for monitoring binding operations
//
// # Quick Start
//
// Basic usage with query parameters:
//
//	package main
//
//	import (
//		"net/url"
//		"rivaas.dev/binding"
//	)
//
//	type UserRequest struct {
//		Name  string `query:"name"`
//		Age   int    `query:"age"`
//		Email string `query:"email"`
//	}
//
//	func main() {
//		query := url.Values{"name": {"John"}, "age": {"30"}}
//		var req UserRequest
//		err := binding.Bind(&req, binding.NewQueryGetter(query), "query")
//		if err != nil {
//			// Handle error
//		}
//	}
//
// JSON binding:
//
//	package main
//
//	import (
//		"bytes"
//		"rivaas.dev/binding"
//	)
//
//	type CreateUserRequest struct {
//		Name  string `json:"name"`
//		Email string `json:"email"`
//	}
//
//	func main() {
//		body := []byte(`{"name": "John", "email": "john@example.com"}`)
//		var req CreateUserRequest
//		err := binding.BindJSONBytes(&req, body)
//		if err != nil {
//			// Handle error
//		}
//	}
//
// # Struct Tags
//
// The package supports multiple struct tag types:
//
//   - query: Query parameters (?name=value)
//   - form: Form data (application/x-www-form-urlencoded)
//   - json: JSON body fields
//   - params: URL path parameters (/users/:id)
//   - header: HTTP headers
//   - cookie: HTTP cookies
//
// Tag syntax supports aliases and options:
//
//	type Request struct {
//		UserID string `query:"user_id,id"`  // Primary name "user_id", alias "id"
//		Name   string `json:"name,omitempty"` // JSON name with omitempty option
//	}
//
// # Advanced Features
//
// Default values:
//
//	type Request struct {
//		Page int `query:"page" default:"1"`
//	}
//
// Enum validation:
//
//	type Request struct {
//		Status string `query:"status" enum:"pending,active,inactive"`
//	}
//
// Nested structures:
//
//	type Request struct {
//		User struct {
//			Name  string `query:"user.name"`
//			Email string `query:"user.email"`
//		}
//	}
//
// Custom type converters:
//
//	binding.Bind(&result, getter, "query",
//		binding.WithTypedConverter(func(s string) (uuid.UUID, error) {
//			return uuid.Parse(s)
//		}),
//	)
//
// # Examples
//
// See the example_test.go file for complete working examples.
package binding
