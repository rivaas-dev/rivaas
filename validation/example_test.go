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

package validation_test

import (
	"context"
	"errors"
	"fmt"

	"rivaas.dev/validation"
)

// ExampleValidate demonstrates basic validation with struct tags.
func ExampleValidate() {
	type User struct {
		Email string `json:"email" validate:"required,email"`
		Age   int    `json:"age" validate:"min=18"`
	}

	user := User{Email: "invalid", Age: 15}
	err := validation.Validate(context.Background(), &user)
	if err != nil {
		var verr *validation.Error
		if errors.As(err, &verr) {
			for _, fieldErr := range verr.Fields {
				fmt.Printf("%s: %s\n", fieldErr.Path, fieldErr.Message)
			}
		}
	}
	// Output:
	// age: must be at least 18
	// email: must be a valid email address
}

// ExampleNew demonstrates creating a configured Validator instance.
func ExampleNew() {
	// Create a validator with custom configuration
	validator, err := validation.New(
		validation.WithMaxErrors(5),
		validation.WithRedactor(func(path string) bool {
			return path == "password" || path == "token"
		}),
	)
	if err != nil {
		fmt.Printf("Failed to create validator: %v\n", err)
		return
	}

	type User struct {
		Email string `json:"email" validate:"required,email"`
	}

	user := User{Email: "john@example.com"}
	if validateErr := validator.Validate(context.Background(), &user); validateErr != nil {
		fmt.Printf("Validation failed: %v\n", validateErr)
	} else {
		fmt.Println("Validation passed")
	}
	// Output: Validation passed
}

// ExampleMustNew demonstrates creating a Validator with MustNew (panics on error).
func ExampleMustNew() {
	// MustNew panics if configuration is invalid - suitable for use in main() or init()
	validator := validation.MustNew(
		validation.WithMaxErrors(10),
	)

	type User struct {
		Name string `json:"name" validate:"required"`
	}

	user := User{Name: "Alice"}
	if err := validator.Validate(context.Background(), &user); err != nil {
		fmt.Printf("Validation failed: %v\n", err)
	} else {
		fmt.Println("User is valid")
	}
	// Output: User is valid
}

// ExampleValidatePartial demonstrates partial validation for PATCH requests.
func ExampleValidatePartial() {
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
		Age   int    `json:"age" validate:"min=18"`
	}

	// Simulate a PATCH request that only updates email
	rawJSON := []byte(`{"email": "new@example.com"}`)
	presence, _ := validation.ComputePresence(rawJSON)

	user := User{Name: "Existing Name", Email: "new@example.com", Age: 25}
	err := validation.ValidatePartial(context.Background(), &user, presence)

	if err != nil {
		fmt.Printf("Validation failed: %v\n", err)
	} else {
		fmt.Println("Validation passed")
	}
	// Output: Validation passed
}

// ExampleValidator_Validate demonstrates using a Validator instance.
func ExampleValidator_Validate() {
	validator := validation.MustNew()

	type User struct {
		Email string `json:"email" validate:"required,email"`
	}

	user := User{Email: "invalid-email"}
	err := validator.Validate(context.Background(), &user)
	if err != nil {
		var verr *validation.Error
		if errors.As(err, &verr) {
			fmt.Printf("Found %d error(s)\n", len(verr.Fields))
			fmt.Printf("First error: %s\n", verr.Fields[0].Message)
		}
	}
	// Output:
	// Found 1 error(s)
	// First error: must be a valid email address
}

// ExampleValidate_withOptions demonstrates validation with various options.
func ExampleValidate_withOptions() {
	type User struct {
		Password string `json:"password" validate:"required,strong_password"`
		Token    string `json:"token" validate:"required"`
	}

	user := User{
		Password: "short",
		Token:    "secret-token-12345",
	}

	// Use redactor to hide sensitive fields
	redactor := func(path string) bool {
		return path == "password" || path == "token"
	}

	err := validation.Validate(context.Background(), &user,
		validation.WithRedactor(redactor),
		validation.WithMaxErrors(5),
	)
	if err != nil {
		var verr *validation.Error
		if errors.As(err, &verr) {
			for _, fieldErr := range verr.Fields {
				fmt.Printf("%s: %s (value: %v)\n",
					fieldErr.Path,
					fieldErr.Message,
					fieldErr.Meta["value"],
				)
			}
		}
	}
	// Output:
	// password: must be at least 8 characters (value: ***REDACTED***)
}

// ExampleComputePresence demonstrates how to compute presence map from JSON.
func ExampleComputePresence() {
	rawJSON := []byte(`{
		"user": {
			"name": "Alice",
			"age": 30
		},
		"items": [
			{"id": 1, "name": "Item 1"},
			{"id": 2}
		]
	}`)

	presence, err := validation.ComputePresence(rawJSON)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Check if specific paths are present
	fmt.Printf("user.name present: %v\n", presence.Has("user.name"))
	fmt.Printf("user.email present: %v\n", presence.Has("user.email"))
	fmt.Printf("items.0.name present: %v\n", presence.Has("items.0.name"))
	fmt.Printf("items.1.name present: %v\n", presence.Has("items.1.name"))

	// Get leaf paths (fields that aren't prefixes of others)
	leaves := presence.LeafPaths()
	// Sort for consistent output in example
	fmt.Printf("Leaf paths count: %d\n", len(leaves))
	fmt.Printf("Sample leaf: %s\n", leaves[0])

	// Output:
	// user.name present: true
	// user.email present: false
	// items.0.name present: true
	// items.1.name present: false
	// Leaf paths count: 5
	// Sample leaf: items.0.id
}
