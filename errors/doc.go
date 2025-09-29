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

// Package errors provides framework-agnostic error formatting for HTTP responses.
//
// The package defines a Formatter interface and provides concrete implementations
// for different error response formats:
//   - RFC9457: RFC 9457 Problem Details (application/problem+json)
//   - JSONAPI: JSON:API error responses (application/vnd.api+json)
//   - Simple: Simple JSON error responses (application/json)
//
// The package is independent of any HTTP framework and can be used with any
// HTTP handler. Domain errors can implement optional interfaces (ErrorType,
// ErrorDetails, ErrorCode) to control status codes and provide structured details.
//
// # Quick Start
//
// Basic usage with RFC 9457 format:
//
//	package main
//
//	import (
//		"encoding/json"
//		"net/http"
//		"rivaas.dev/errors"
//	)
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//		err := someOperation()
//		if err != nil {
//			formatter := errors.NewRFC9457("https://api.example.com/problems")
//			response := formatter.Format(r, err)
//			w.Header().Set("Content-Type", response.ContentType)
//			w.WriteHeader(response.Status)
//			json.NewEncoder(w).Encode(response.Body)
//			return
//		}
//	}
//
// JSON:API format:
//
//	formatter := errors.NewJSONAPI()
//	response := formatter.Format(r, err)
//	w.Header().Set("Content-Type", response.ContentType)
//	w.WriteHeader(response.Status)
//	json.NewEncoder(w).Encode(response.Body)
//
// Simple JSON format:
//
//	formatter := errors.NewSimple()
//	response := formatter.Format(r, err)
//	w.Header().Set("Content-Type", response.ContentType)
//	w.WriteHeader(response.Status)
//	json.NewEncoder(w).Encode(response.Body)
//
// # Error Interfaces
//
// Domain errors can implement optional interfaces to provide additional information:
//
//   - ErrorType: Declare HTTP status code
//   - ErrorDetails: Provide structured details (e.g., field-level validation errors)
//   - ErrorCode: Provide machine-readable error codes
//
// Example error with all interfaces:
//
//	type ValidationError struct {
//		Message string
//		Fields  []FieldError
//		Code    string
//	}
//
//	func (e ValidationError) Error() string {
//		return e.Message
//	}
//
//	func (e ValidationError) HTTPStatus() int {
//		return http.StatusBadRequest
//	}
//
//	func (e ValidationError) Details() any {
//		return e.Fields
//	}
//
//	func (e ValidationError) Code() string {
//		return e.Code
//	}
//
// # Examples
//
// See the example_test.go file for complete working examples.
package errors
