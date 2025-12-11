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

package errors_test

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"

	"rivaas.dev/errors"
)

// ExampleRFC9457 demonstrates how to use the RFC9457 formatter.
func ExampleRFC9457() {
	// Create a formatter with a base URL for problem types
	formatter := errors.NewRFC9457("https://api.example.com/problems")

	// Create a test error
	err := stderrors.New("validation failed")

	// Create a request
	req := httptest.NewRequest(http.MethodPost, "/api/users", nil)

	// Format the error
	response := formatter.Format(req, err)

	// Write the response
	w := httptest.NewRecorder()
	w.WriteHeader(response.Status)
	w.Header().Set("Content-Type", response.ContentType)
	_ = json.NewEncoder(w).Encode(response.Body)

	_, _ = fmt.Printf("Status: %d\n", response.Status)
	_, _ = fmt.Printf("Content-Type: %s\n", response.ContentType)
	// Output:
	// Status: 500
	// Content-Type: application/problem+json; charset=utf-8
}

// ExampleJSONAPI demonstrates how to use the JSONAPI formatter.
func ExampleJSONAPI() {
	// Create a formatter
	formatter := errors.NewJSONAPI()

	// Create a test error
	err := stderrors.New("resource not found")

	// Create a request
	req := httptest.NewRequest(http.MethodGet, "/api/users/123", nil)

	// Format the error
	response := formatter.Format(req, err)

	// Write the response
	w := httptest.NewRecorder()
	w.WriteHeader(response.Status)
	w.Header().Set("Content-Type", response.ContentType)
	_ = json.NewEncoder(w).Encode(response.Body)

	_, _ = fmt.Printf("Status: %d\n", response.Status)
	_, _ = fmt.Printf("Content-Type: %s\n", response.ContentType)
	// Output:
	// Status: 500
	// Content-Type: application/vnd.api+json; charset=utf-8
}

// ExampleSimple demonstrates how to use the Simple formatter.
func ExampleSimple() {
	// Create a formatter
	formatter := errors.NewSimple()

	// Create a test error
	err := stderrors.New("internal server error")

	// Create a request
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)

	// Format the error
	response := formatter.Format(req, err)

	// Write the response
	w := httptest.NewRecorder()
	w.WriteHeader(response.Status)
	w.Header().Set("Content-Type", response.ContentType)
	_ = json.NewEncoder(w).Encode(response.Body)

	_, _ = fmt.Printf("Status: %d\n", response.Status)
	_, _ = fmt.Printf("Content-Type: %s\n", response.ContentType)
	// Output:
	// Status: 500
	// Content-Type: application/json; charset=utf-8
}

// ExampleRFC9457_customErrorID demonstrates custom error ID generation.
func ExampleRFC9457_customErrorID() {
	// Create a formatter with custom error ID generator
	formatter := &errors.RFC9457{
		BaseURL: "https://api.example.com/problems",
		ErrorIDGenerator: func() string {
			return "custom-id-12345"
		},
	}

	err := stderrors.New("test error")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	response := formatter.Format(req, err)

	body := response.Body.(errors.ProblemDetail)
	_, _ = fmt.Printf("Error ID: %v\n", body.Extensions["error_id"])
	// Output:
	// Error ID: custom-id-12345
}

// ExampleRFC9457_disableErrorID demonstrates disabling error ID generation.
func ExampleRFC9457_disableErrorID() {
	// Create a formatter with error ID disabled
	formatter := &errors.RFC9457{
		BaseURL:        "https://api.example.com/problems",
		DisableErrorID: true,
	}

	err := stderrors.New("test error")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	response := formatter.Format(req, err)

	body := response.Body.(errors.ProblemDetail)
	if _, ok := body.Extensions["error_id"]; !ok {
		_, _ = fmt.Println("Error ID is disabled")
	}
	// Output:
	// Error ID is disabled
}
