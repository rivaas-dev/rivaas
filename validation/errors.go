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

package validation

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrValidation is a sentinel error for validation failures.
// Use errors.Is(err, ErrValidation) to check if an error is a validation error.
var ErrValidation = errors.New("validation")

// Predefined validation errors.
var (
	// ErrCannotValidateNilValue is returned when attempting to validate a nil value.
	ErrCannotValidateNilValue = errors.New("cannot validate nil value")

	// ErrCannotValidateInvalidValue is returned when the value is not valid for reflection.
	ErrCannotValidateInvalidValue = errors.New("cannot validate invalid value")

	// ErrUnknownValidationStrategy is returned when an unknown validation strategy is specified.
	ErrUnknownValidationStrategy = errors.New("unknown validation strategy")

	// ErrValidationFailed is a generic validation failure error.
	ErrValidationFailed = errors.New("validation failed")

	// ErrInvalidType is returned when a value has an unexpected type.
	ErrInvalidType = errors.New("invalid type")
)

// FieldError represents a single validation error for a specific field.
// Multiple FieldError values are collected in an [Error].
//
// Example:
//
//	err := FieldError{
//	    Path:    "email",
//	    Code:    "tag.required",
//	    Message: "is required",
//	    Meta:    map[string]any{"tag": "required"},
//	}
type FieldError struct {
	Path    string         `json:"path"`           // JSON path (e.g., "items.2.price")
	Code    string         `json:"code"`           // Stable code (e.g., "tag.required", "schema.type")
	Message string         `json:"message"`        // Human-readable message
	Meta    map[string]any `json:"meta,omitempty"` // Additional metadata (tag, param, value, etc.)
}

// Error returns a formatted error message as "path: message" or just "message" if path is empty.
func (e FieldError) Error() string {
	if e.Path == "" {
		return e.Message
	}

	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// Unwrap returns [ErrValidation] for errors.Is/errors.As compatibility.
func (e FieldError) Unwrap() error {
	return ErrValidation
}

// HTTPStatus implements rivaas.dev/errors.ErrorType.
func (e FieldError) HTTPStatus() int {
	return 422 // Unprocessable Entity
}

// Error represents validation errors for one or more fields.
// Error implements error and can be used with errors.Is/errors.As.
// It contains a slice of [FieldError] values, one for each field that failed validation.
//
// Example:
//
//	var err *Error
//	if errors.As(validationErr, &err) {
//	    for _, fieldErr := range err.Fields {
//	        fmt.Printf("%s: %s\n", fieldErr.Path, fieldErr.Message)
//	    }
//	}
//
//nolint:recvcheck // Error must use value receiver for error interface compatibility, mutating methods use pointer
type Error struct {
	Fields    []FieldError `json:"errors"`              // List of field errors
	Truncated bool         `json:"truncated,omitempty"` // True if errors were truncated due to maxErrors limit
}

// Error returns a formatted error message.
func (v Error) Error() string {
	if len(v.Fields) == 0 {
		return ""
	}
	if len(v.Fields) == 1 {
		return v.Fields[0].Error()
	}

	suffix := ""
	if v.Truncated {
		suffix = " (truncated)"
	}

	var msgs []string
	for _, err := range v.Fields {
		msgs = append(msgs, err.Error())
	}

	return fmt.Sprintf("validation failed: %s%s", strings.Join(msgs, "; "), suffix)
}

// Unwrap returns [ErrValidation] for errors.Is/errors.As compatibility.
func (v Error) Unwrap() error {
	return ErrValidation
}

// HTTPStatus implements rivaas.dev/errors.ErrorType.
func (v Error) HTTPStatus() int {
	return 422 // Unprocessable Entity
}

// Details implements rivaas.dev/errors.ErrorDetails.
func (v Error) Details() any {
	return v.Fields
}

// Code implements rivaas.dev/errors.ErrorCode.
func (v Error) Code() string {
	return "validation_error"
}

// Add adds a new [FieldError] to the collection.
//
// Example:
//
//	var err Error
//	err.Add("email", "tag.required", "is required", map[string]any{"tag": "required"})
func (v *Error) Add(path, code, message string, meta map[string]any) {
	v.Fields = append(v.Fields, FieldError{
		Path:    path,
		Code:    code,
		Message: message,
		Meta:    meta,
	})
}

// AddError adds an error to the collection, handling different error types.
// AddError accepts [FieldError], [Error], or generic errors and converts them appropriately.
//
// Example:
//
//	var err Error
//	err.AddError(FieldError{Path: "email", Code: "tag.required", Message: "is required"})
func (v *Error) AddError(err error) {
	if err == nil {
		return
	}

	if fe, ok := err.(FieldError); ok {
		v.Fields = append(v.Fields, fe)
		return
	}

	if ve, ok := err.(Error); ok {
		v.Fields = append(v.Fields, ve.Fields...)
		if ve.Truncated {
			v.Truncated = true
		}

		return
	}

	if ve, ok := err.(*Error); ok {
		v.Fields = append(v.Fields, ve.Fields...)
		if ve.Truncated {
			v.Truncated = true
		}

		return
	}

	v.Fields = append(v.Fields, FieldError{
		Code:    "validation_error",
		Message: err.Error(),
	})
}

// HasErrors returns true if there are any errors.
func (v Error) HasErrors() bool {
	return len(v.Fields) > 0
}

// HasCode returns true if any error has the given code.
//
// Example:
//
//	if err.HasCode("tag.required") {
//	    // Handle required field errors
//	}
func (v Error) HasCode(code string) bool {
	for _, e := range v.Fields {
		if e.Code == code {
			return true
		}
	}

	return false
}

// Has checks if a specific field path has an error.
//
// Example:
//
//	if err.Has("email") {
//	    // Handle email field errors
//	}
func (v Error) Has(path string) bool {
	for _, f := range v.Fields {
		if f.Path == path {
			return true
		}
	}

	return false
}

// GetField returns the first [FieldError] for a given path, or nil if not found.
//
// Example:
//
//	fieldErr := err.GetField("email")
//	if fieldErr != nil {
//	    fmt.Println(fieldErr.Message)
//	}
func (v Error) GetField(path string) *FieldError {
	for _, f := range v.Fields {
		if f.Path == path {
			return &f
		}
	}

	return nil
}

// Sort sorts errors by path, then by code.
// Sort modifies the error in place and is useful for consistent error presentation.
func (v *Error) Sort() {
	sort.Slice(v.Fields, func(i, j int) bool {
		if v.Fields[i].Path != v.Fields[j].Path {
			return v.Fields[i].Path < v.Fields[j].Path
		}

		return v.Fields[i].Code < v.Fields[j].Code
	})
}
