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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// RFC9457 formats errors as RFC 9457 Problem Details.
// It produces responses with Content-Type "application/problem+json".
//
// Example:
//
//	formatter := errors.NewRFC9457("https://api.example.com/problems")
//	response := formatter.Format(req, err)
//	w.Header().Set("Content-Type", response.ContentType)
//	w.WriteHeader(response.Status)
//	json.NewEncoder(w).Encode(response.Body)
type RFC9457 struct {
	// BaseURL is prepended to problem type slugs to create full URIs.
	// Example: "https://api.example.com/problems" + "/validation-error"
	BaseURL string

	// TypeResolver maps error types/codes to problem type URIs.
	// If nil, uses default mapping based on ErrorCode interface.
	TypeResolver func(err error) string

	// StatusResolver determines HTTP status from error.
	// If nil, uses default logic (ErrorType interface, then 500).
	StatusResolver func(err error) int

	// ErrorIDGenerator generates unique IDs for error tracking.
	// If nil, uses default UUID-based generation.
	ErrorIDGenerator func() string

	// DisableErrorID disables automatic error ID generation.
	DisableErrorID bool
}

// ProblemDetail represents an RFC 9457 problem detail.
// It contains the standard problem detail fields plus extensions.
//
// Example:
//
//	p := ProblemDetail{
//		Type:     "https://api.example.com/problems/validation-error",
//		Title:    "Validation Error",
//		Status:   400,
//		Detail:   "The request contains invalid data",
//		Instance: "/api/users",
//		Extensions: map[string]any{
//			"errors": []FieldError{...},
//		},
//	}
type ProblemDetail struct {
	Type       string         `json:"type"`
	Title      string         `json:"title"`
	Status     int            `json:"status"`
	Detail     string         `json:"detail,omitempty"`
	Instance   string         `json:"instance,omitempty"`
	Extensions map[string]any `json:"-"` // Marshaled inline
}

// MarshalJSON implements custom JSON marshaling to include extensions inline.
// It merges extension fields into the main JSON object while protecting reserved field names.
//
// Returns the JSON-encoded problem detail with extensions merged inline.
func (p ProblemDetail) MarshalJSON() ([]byte, error) {
	m := map[string]any{
		"type":   p.Type,
		"title":  p.Title,
		"status": p.Status,
	}
	if p.Detail != "" {
		m["detail"] = p.Detail
	}
	if p.Instance != "" {
		m["instance"] = p.Instance
	}
	// Merge extensions
	for k, v := range p.Extensions {
		// Protect reserved field names
		if k != "type" && k != "title" && k != "status" && k != "detail" && k != "instance" {
			m[k] = v
		}
	}

	return json.Marshal(m)
}

// Format converts an error into an RFC 9457 Problem Details response.
// It determines the status code, problem type, and builds the problem detail structure.
// If the error implements ErrorDetails or ErrorCode interfaces, those are included as extensions.
//
// Example:
//
//	formatter := errors.NewRFC9457("https://api.example.com/problems")
//	response := formatter.Format(req, err)
//	w.Header().Set("Content-Type", response.ContentType)
//	w.WriteHeader(response.Status)
//	json.NewEncoder(w).Encode(response.Body)
//
// Parameters:
//   - req: HTTP request (used for instance URI)
//   - err: Error to format
//
// Returns a Response with RFC 9457 formatted error.
func (f *RFC9457) Format(req *http.Request, err error) Response {
	status := f.determineStatus(err)
	problemType := f.determineType(err)

	p := ProblemDetail{
		Type:       problemType,
		Title:      http.StatusText(status),
		Status:     status,
		Detail:     err.Error(),
		Instance:   req.URL.Path,
		Extensions: make(map[string]any),
	}

	// Add error_id for tracing (unless disabled)
	if !f.DisableErrorID {
		var errorID string
		if f.ErrorIDGenerator != nil {
			errorID = f.ErrorIDGenerator()
		} else {
			errorID = generateErrorID()
		}
		p.Extensions["error_id"] = errorID
	}

	// Enrich with details if available
	var detailed ErrorDetails
	if errors.As(err, &detailed) {
		p.Extensions["errors"] = detailed.Details()
	}

	// Add code if available
	var coded ErrorCode
	if errors.As(err, &coded) {
		p.Extensions["code"] = coded.Code()
	}

	return Response{
		Status:      status,
		ContentType: "application/problem+json; charset=utf-8",
		Body:        p,
	}
}

// determineStatus determines the HTTP status code for an error.
// It checks StatusResolver first, then ErrorType interface, then defaults to 500.
//
// Parameters:
//   - err: Error to determine status for
//
// Returns the HTTP status code.
func (f *RFC9457) determineStatus(err error) int {
	// Custom resolver takes precedence
	if f.StatusResolver != nil {
		return f.StatusResolver(err)
	}

	// Check if error declares its own status
	var typed ErrorType
	if errors.As(err, &typed) {
		return typed.HTTPStatus()
	}

	// Default to 500
	return http.StatusInternalServerError
}

// determineType determines the problem type URI for an error.
// It checks TypeResolver first, then ErrorCode interface, then defaults to "about:blank".
//
// Parameters:
//   - err: Error to determine type for
//
// Returns the problem type URI.
func (f *RFC9457) determineType(err error) string {
	// Custom resolver takes precedence
	if f.TypeResolver != nil {
		return f.TypeResolver(err)
	}

	// Check if error has a code
	var coded ErrorCode
	if errors.As(err, &coded) {
		code := coded.Code()
		if f.BaseURL != "" {
			return f.BaseURL + "/" + code
		}

		return code
	}

	return "about:blank"
}

// generateErrorID generates a unique error ID for correlation.
// It uses cryptographically secure random bytes, falling back to timestamp-based ID if random generation fails.
//
// Returns a unique error identifier string.
func generateErrorID() string {
	bytes := make([]byte, 16) //nolint:makezero // crypto/rand.Read requires pre-allocated buffer
	if _, err := rand.Read(bytes); err != nil {
		// Fallback: use timestamp-based ID if random fails
		return fmt.Sprintf("err-%d", time.Now().UnixNano())
	}

	return "err-" + hex.EncodeToString(bytes)
}
