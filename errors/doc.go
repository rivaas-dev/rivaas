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
// This package defines a Formatter interface and provides concrete implementations
// for different error response formats:
//   - RFC9457: RFC 9457 Problem Details (application/problem+json)
//   - JSONAPI: JSON:API error responses (application/vnd.api+json)
//   - Simple: Simple JSON error responses (application/json)
//
// The package is independent of any HTTP framework and can be used with any
// HTTP handler. Domain errors can implement optional interfaces (ErrorType,
// ErrorDetails, ErrorCode) to control status codes and provide structured details.
//
// Example usage:
//
//	formatter := errors.NewRFC9457("https://api.example.com/problems")
//	response := formatter.Format(req, err)
//	// Write response.Body as JSON with response.Status and response.ContentType
package errors
