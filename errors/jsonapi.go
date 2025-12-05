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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// JSONAPI formats errors per JSON:API specification.
// It produces responses with Content-Type "application/vnd.api+json".
// See: https://jsonapi.org/format/#errors
//
// Example:
//
//	formatter := errors.NewJSONAPI()
//	response := formatter.Format(req, err)
//	w.Header().Set("Content-Type", response.ContentType)
//	w.WriteHeader(response.Status)
//	json.NewEncoder(w).Encode(response.Body)
type JSONAPI struct {
	// StatusResolver determines HTTP status from error.
	// If nil, uses ErrorType interface or defaults to 500.
	StatusResolver func(err error) int
}

// jsonAPIError represents a single error in JSON:API format.
type jsonAPIError struct {
	ID     string         `json:"id,omitempty"`     // Unique identifier for this error
	Status string         `json:"status,omitempty"` // HTTP status code as string
	Code   string         `json:"code,omitempty"`   // Application-specific error code
	Title  string         `json:"title,omitempty"`  // Short, human-readable summary
	Detail string         `json:"detail,omitempty"` // Human-readable explanation
	Source *jsonAPISource `json:"source,omitempty"` // Source of the error
	Meta   map[string]any `json:"meta,omitempty"`   // Non-standard meta-information
}

// jsonAPISource points to the source of an error.
type jsonAPISource struct {
	Pointer   string `json:"pointer,omitempty"`   // JSON Pointer to field (e.g., "/data/attributes/email")
	Parameter string `json:"parameter,omitempty"` // Query parameter that caused error
	Header    string `json:"header,omitempty"`    // Header that caused error
}

// jsonAPIErrorResponse wraps errors in JSON:API format.
type jsonAPIErrorResponse struct {
	Errors []jsonAPIError `json:"errors"`
}

// Format converts an error into a JSON:API error response.
// If the error implements ErrorDetails, it converts field-level errors into multiple JSON:API error objects.
// If the error implements ErrorCode, it includes the code in the error object.
//
// Example:
//
//	formatter := errors.NewJSONAPI()
//	response := formatter.Format(req, err)
//	w.Header().Set("Content-Type", response.ContentType)
//	w.WriteHeader(response.Status)
//	json.NewEncoder(w).Encode(response.Body)
//
// Parameters:
//   - req: HTTP request (currently unused, reserved for future use)
//   - err: Error to format
//
// Returns a Response with JSON:API formatted error.
func (f *JSONAPI) Format(req *http.Request, err error) Response {
	status := f.determineStatus(err)

	var apiErrors []jsonAPIError

	// Handle validation errors - convert to multiple JSON:API errors
	if detailed, ok := err.(ErrorDetails); ok {
		details := detailed.Details()

		// Try to handle as a slice (validation.Error returns []FieldError)
		// Use JSON marshaling to convert structs to maps for generic handling
		detailsJSON, marshalErr := json.Marshal(details)
		if marshalErr == nil {
			var detailsData interface{}
			if unmarshalErr := json.Unmarshal(detailsJSON, &detailsData); unmarshalErr == nil {
				if fieldErrors, ok := detailsData.([]interface{}); ok {
					// It's a slice - convert each field error
					for _, field := range fieldErrors {
						apiErr := jsonAPIError{
							ID:     generateErrorID(),
							Status: strconv.Itoa(status),
							Title:  http.StatusText(status),
						}

						// Extract field information from map
						if fieldMap, ok := field.(map[string]interface{}); ok {
							if path, ok := fieldMap["path"].(string); ok && path != "" {
								// Convert path to JSON Pointer format
								// "email" -> "/data/attributes/email"
								// "items.0.price" -> "/data/attributes/items/0/price"
								pointer := convertPathToPointer(path)
								apiErr.Source = &jsonAPISource{
									Pointer: pointer,
								}
							}
							if code, ok := fieldMap["code"].(string); ok && code != "" {
								apiErr.Code = code
							}
							if message, ok := fieldMap["message"].(string); ok && message != "" {
								apiErr.Detail = message
							}
							if meta, ok := fieldMap["meta"].(map[string]interface{}); ok && len(meta) > 0 {
								apiErr.Meta = meta
							}
						}

						// Ensure we have at least a detail message
						if apiErr.Detail == "" {
							apiErr.Detail = err.Error()
						}

						apiErrors = append(apiErrors, apiErr)
					}
				}
			}
		}

		// If we didn't create any errors from the details, create a generic one
		if len(apiErrors) == 0 {
			apiErrors = []jsonAPIError{{
				ID:     generateErrorID(),
				Status: strconv.Itoa(status),
				Title:  http.StatusText(status),
				Detail: err.Error(),
				Meta:   map[string]any{"details": details},
			}}
		}
	} else {
		// Simple error without details
		apiErr := jsonAPIError{
			ID:     generateErrorID(),
			Status: strconv.Itoa(status),
			Title:  http.StatusText(status),
			Detail: err.Error(),
		}

		// Add code if available
		if coded, ok := err.(ErrorCode); ok {
			apiErr.Code = coded.Code()
		}

		apiErrors = []jsonAPIError{apiErr}
	}

	// If no errors were created (shouldn't happen, but be safe)
	if len(apiErrors) == 0 {
		apiErrors = []jsonAPIError{{
			ID:     generateErrorID(),
			Status: fmt.Sprintf("%d", status),
			Title:  http.StatusText(status),
			Detail: err.Error(),
		}}
	}

	return Response{
		Status:      status,
		ContentType: "application/vnd.api+json; charset=utf-8",
		Body:        jsonAPIErrorResponse{Errors: apiErrors},
	}
}

// determineStatus determines the HTTP status code for an error.
// It checks StatusResolver first, then ErrorType interface, then defaults to 500.
//
// Parameters:
//   - err: Error to determine status for
//
// Returns the HTTP status code.
func (f *JSONAPI) determineStatus(err error) int {
	if f.StatusResolver != nil {
		return f.StatusResolver(err)
	}

	var typed ErrorType
	if errors.As(err, &typed) {
		return typed.HTTPStatus()
	}

	return http.StatusInternalServerError
}

// convertPathToPointer converts a field path to JSON Pointer format.
// It replaces dots with slashes and prepends "/data/attributes/".
//
// Example conversions:
//   - "email" -> "/data/attributes/email"
//   - "items.0.price" -> "/data/attributes/items/0/price"
//   - "user.name" -> "/data/attributes/user/name"
//
// Parameters:
//   - path: Field path to convert (e.g., "email" or "items.0.price")
//
// Returns the JSON Pointer path.
func convertPathToPointer(path string) string {
	if path == "" {
		return ""
	}

	// Replace dots with slashes for nested paths
	pointer := strings.ReplaceAll(path, ".", "/")
	return "/data/attributes/" + pointer
}
