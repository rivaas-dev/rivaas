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
//
// Example:
//
//	formatter := errors.NewRFC9457("https://api.example.com/problems")
//	response := formatter.Format(req, err)
//	w.Header().Set("Content-Type", response.ContentType)
//	w.WriteHeader(response.Status)
//	json.NewEncoder(w).Encode(response.Body)
type Formatter interface {
	// Format converts an error into HTTP response components.
	// It returns status code, content-type, and response body.
	//
	// Example:
	//
	//	response := formatter.Format(req, err)
	//	w.Header().Set("Content-Type", response.ContentType)
	//	w.WriteHeader(response.Status)
	//	json.NewEncoder(w).Encode(response.Body)
	//
	// Parameters:
	//   - req: HTTP request context (used for instance URI in RFC9457)
	//   - err: Error to format
	//
	// Returns a Response containing status code, content type, and body.
	Format(req *http.Request, err error) Response
}

// Response represents a formatted error response.
// It contains all components needed to write an HTTP error response.
//
// Example:
//
//	response := formatter.Format(req, err)
//	w.Header().Set("Content-Type", response.ContentType)
//	if response.Headers != nil {
//		for k, v := range response.Headers {
//			for _, val := range v {
//				w.Header().Add(k, val)
//			}
//		}
//	}
//	w.WriteHeader(response.Status)
//	json.NewEncoder(w).Encode(response.Body)
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

// ErrorType allows errors to declare their own HTTP status code.
// Domain errors can optionally implement this interface to control their status code.
//
// Example:
//
//	type ValidationError struct {
//		Message string
//	}
//
//	func (e ValidationError) Error() string {
//		return e.Message
//	}
//
//	func (e ValidationError) HTTPStatus() int {
//		return http.StatusBadRequest
//	}
type ErrorType interface {
	error
	// HTTPStatus returns the HTTP status code for this error.
	HTTPStatus() int
}

// ErrorDetails allows errors to provide additional structured information.
// Domain errors can implement this interface to expose field-level details.
//
// Example:
//
//	type ValidationError struct {
//		Message string
//		Fields  []FieldError
//	}
//
//	func (e ValidationError) Error() string {
//		return e.Message
//	}
//
//	func (e ValidationError) Details() any {
//		return e.Fields
//	}
type ErrorDetails interface {
	error
	// Details returns structured information about the error.
	Details() any
}

// ErrorCode allows errors to provide a machine-readable code.
// Domain errors can implement this interface to expose application-specific error codes.
//
// Example:
//
//	type NotFoundError struct {
//		Resource string
//	}
//
//	func (e NotFoundError) Error() string {
//		return fmt.Sprintf("%s not found", e.Resource)
//	}
//
//	func (e NotFoundError) Code() string {
//		return "RESOURCE_NOT_FOUND"
//	}
type ErrorCode interface {
	error
	// Code returns a machine-readable error code.
	Code() string
}

// NewRFC9457 creates a new RFC9457 formatter.
// The baseURL parameter is prepended to problem type slugs to create full URIs.
//
// Example:
//
//	formatter := errors.NewRFC9457("https://api.example.com/problems")
//	response := formatter.Format(req, err)
//
// Parameters:
//   - baseURL: Base URL for problem type URIs (e.g., "https://api.example.com/problems")
//
// Returns a new RFC9457 formatter instance.
func NewRFC9457(baseURL string) *RFC9457 {
	return &RFC9457{
		BaseURL: baseURL,
	}
}

// NewJSONAPI creates a new JSONAPI formatter.
//
// Example:
//
//	formatter := errors.NewJSONAPI()
//	response := formatter.Format(req, err)
//	w.Header().Set("Content-Type", response.ContentType)
//	w.WriteHeader(response.Status)
//	json.NewEncoder(w).Encode(response.Body)
//
// Returns a new JSONAPI formatter instance.
func NewJSONAPI() *JSONAPI {
	return &JSONAPI{}
}

// NewSimple creates a new Simple formatter.
//
// Example:
//
//	formatter := errors.NewSimple()
//	response := formatter.Format(req, err)
//	w.Header().Set("Content-Type", response.ContentType)
//	w.WriteHeader(response.Status)
//	json.NewEncoder(w).Encode(response.Body)
//
// Returns a new Simple formatter instance.
func NewSimple() *Simple {
	return &Simple{}
}

// WithStatus wraps an error with an explicit HTTP status code.
// The wrapped error implements ErrorType interface.
//
// This is useful when you want to override the default status code for an error.
// If err is nil, the status text for the given status code is used as the error message.
//
// Example:
//
//	return errors.WithStatus(err, http.StatusNotFound)
//	return errors.WithStatus(nil, http.StatusNoContent) // nil allowed
func WithStatus(err error, status int) error {
	return &statusError{err: err, status: status}
}

// statusError wraps an error with an explicit status code.
type statusError struct {
	err    error
	status int
}

func (e *statusError) Error() string {
	if e.err == nil {
		return http.StatusText(e.status)
	}
	return e.err.Error()
}

func (e *statusError) Unwrap() error {
	return e.err
}

func (e *statusError) HTTPStatus() int {
	return e.status
}
