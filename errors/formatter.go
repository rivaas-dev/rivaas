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

package errors

import (
	"net/http"
)

// Formatter defines how errors are formatted in HTTP responses.
// Implementations are framework-agnostic and work with any HTTP handler.
type Formatter interface {
	// Format converts an error into HTTP response components.
	// Returns status code, content-type, and response body.
	Format(req *http.Request, err error) Response
}

// Response represents a formatted error response.
type Response struct {
	// Status is the HTTP status code.
	Status int

	// ContentType is the Content-Type header value.
	ContentType string

	// Body is the response body (will be marshaled to JSON/XML/etc).
	Body any

	// Headers contains additional headers to set (optional).
	Headers http.Header
}

// ErrorType allows errors to declare their own HTTP status.
// Domain errors can optionally implement this to control their status code.
type ErrorType interface {
	error
	HTTPStatus() int
}

// ErrorDetails allows errors to provide additional structured information.
// Domain errors can implement this to expose field-level details.
type ErrorDetails interface {
	error
	Details() any
}

// ErrorCode allows errors to provide a machine-readable code.
type ErrorCode interface {
	error
	Code() string
}

// NewRFC9457 creates a new RFC9457 formatter with sensible defaults.
func NewRFC9457(baseURL string) *RFC9457 {
	return &RFC9457{
		BaseURL: baseURL,
	}
}

// NewJSONAPI creates a new JSONAPI formatter with sensible defaults.
func NewJSONAPI() *JSONAPI {
	return &JSONAPI{}
}

// NewSimple creates a new Simple formatter with sensible defaults.
func NewSimple() *Simple {
	return &Simple{}
}
