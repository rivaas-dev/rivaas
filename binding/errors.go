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

package binding

import (
	"errors"
	"fmt"
	"strings"
)

// Static errors for binding operations.
var (
	ErrUnsupportedContentType  = errors.New("unsupported content type")
	ErrRequestBodyNil          = errors.New("request body is nil")
	ErrOutMustBePointer        = errors.New("out must be a pointer to struct")
	ErrOutPointerNil           = errors.New("out pointer is nil")
	ErrInvalidIPAddress        = errors.New("invalid IP address")
	ErrUnsupportedType         = errors.New("unsupported type")
	ErrInvalidBooleanValue     = errors.New("invalid boolean value")
	ErrEmptyTimeValue          = errors.New("empty time value")
	ErrUnableToParseTime       = errors.New("unable to parse time")
	ErrOnlyMapStringTSupported = errors.New("only map[string]T is supported")
	ErrInvalidBracketNotation  = errors.New("invalid bracket notation in key")
	ErrValueNotInAllowedValues = errors.New("value not in allowed values")
	ErrMaxDepthExceeded        = errors.New("exceeded maximum nesting depth")
	ErrSliceExceedsMaxLength   = errors.New("slice exceeds max length")
	ErrMapExceedsMaxSize       = errors.New("map exceeds max size")
	ErrInvalidStructTag        = errors.New("invalid struct tag")
	ErrInvalidUUIDFormat       = errors.New("invalid UUID format")
)

// BindError represents a binding error with field-level context.
// It provides detailed information about which field failed, what value was
// provided, and what type was expected.
type BindError struct {
	Field  string // Field name that failed binding
	Tag    string // Source tag (query, params, form, etc.)
	Value  string // The value that failed conversion
	Type   string // Expected Go type name
	Reason string // Human-readable reason for failure
	Err    error  // Underlying error
}

// Error returns a formatted error message.
func (e *BindError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("binding field %q (%s): %s", e.Field, e.Tag, e.Reason)
	}
	return fmt.Sprintf("binding field %q (%s): failed to convert %q to %s: %v",
		e.Field, e.Tag, e.Value, e.Type, e.Err)
}

// Unwrap returns the underlying error for errors.Is/As compatibility.
func (e *BindError) Unwrap() error {
	return e.Err
}

// HTTPStatus implements rivaas.dev/errors.ErrorType.
func (e *BindError) HTTPStatus() int {
	return 400 // Bad Request
}

// Code implements rivaas.dev/errors.ErrorCode.
func (e *BindError) Code() string {
	return "binding_error"
}

// UnknownFieldError is returned when strict JSON decoding encounters unknown fields.
// It contains the list of field names that were present in the JSON but not
// defined in the target struct.
type UnknownFieldError struct {
	Fields []string // List of unknown field names
}

// Error returns a formatted error message.
func (e *UnknownFieldError) Error() string {
	if len(e.Fields) == 1 {
		return "unknown field: " + e.Fields[0]
	}
	return "unknown fields: " + strings.Join(e.Fields, ", ")
}

// MultiError aggregates multiple binding errors.
// It is returned when binding from multiple sources fails for multiple fields.
type MultiError struct {
	Errors []*BindError
}

// Error returns a formatted error message.
func (m *MultiError) Error() string {
	if len(m.Errors) == 1 {
		return m.Errors[0].Error()
	}
	return fmt.Sprintf("%d binding errors occurred", len(m.Errors))
}

// Unwrap returns all errors for errors.Is/As compatibility.
func (m *MultiError) Unwrap() []error {
	errs := make([]error, len(m.Errors))
	for i, e := range m.Errors {
		errs[i] = e
	}
	return errs
}

// HTTPStatus implements rivaas.dev/errors.ErrorType.
func (m *MultiError) HTTPStatus() int {
	return 400 // Bad Request
}

// Details implements rivaas.dev/errors.ErrorDetails.
func (m *MultiError) Details() any {
	return m.Errors
}

// Code implements rivaas.dev/errors.ErrorCode.
func (m *MultiError) Code() string {
	return "multiple_binding_errors"
}
