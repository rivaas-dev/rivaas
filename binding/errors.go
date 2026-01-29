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
	"reflect"
	"strings"
)

// Source represents the binding source type.
type Source int

const (
	// SourceUnknown is an unspecified source.
	SourceUnknown Source = iota

	// SourceQuery represents URL query parameters.
	SourceQuery

	// SourcePath represents URL path parameters.
	SourcePath

	// SourceForm represents form data.
	SourceForm

	// SourceHeader represents HTTP headers.
	SourceHeader

	// SourceCookie represents HTTP cookies.
	SourceCookie

	// SourceJSON represents JSON body.
	SourceJSON

	// SourceXML represents XML body.
	SourceXML

	// SourceYAML represents YAML body.
	SourceYAML

	// SourceTOML represents TOML body.
	SourceTOML

	// SourceMsgPack represents MessagePack body.
	SourceMsgPack

	// SourceProto represents Protocol Buffers body.
	SourceProto
)

// String returns the string representation of the source.
func (s Source) String() string {
	switch s {
	case SourceQuery:
		return "query"
	case SourcePath:
		return "path"
	case SourceForm:
		return "form"
	case SourceHeader:
		return "header"
	case SourceCookie:
		return "cookie"
	case SourceJSON:
		return "json"
	case SourceXML:
		return "xml"
	case SourceYAML:
		return "yaml"
	case SourceTOML:
		return "toml"
	case SourceMsgPack:
		return "msgpack"
	case SourceProto:
		return "proto"
	default:
		return "unknown"
	}
}

// sourceFromTag converts a tag string to Source type.
func sourceFromTag(tag string) Source {
	switch tag {
	case TagQuery:
		return SourceQuery
	case TagPath:
		return SourcePath
	case TagForm:
		return SourceForm
	case TagHeader:
		return SourceHeader
	case TagCookie:
		return SourceCookie
	case TagJSON:
		return SourceJSON
	case TagXML:
		return SourceXML
	case TagYAML:
		return SourceYAML
	case TagTOML:
		return SourceTOML
	case TagMsgPack:
		return SourceMsgPack
	case TagProto:
		return SourceProto
	default:
		return SourceUnknown
	}
}

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
	ErrMaxDepthExceeded        = errors.New("exceeded maximum nesting depth")
	ErrSliceExceedsMaxLength   = errors.New("slice exceeds max length")
	ErrMapExceedsMaxSize       = errors.New("map exceeds max size")
	ErrInvalidStructTag        = errors.New("invalid struct tag")
	ErrInvalidUUIDFormat       = errors.New("invalid UUID format")
	ErrNoSourcesProvided       = errors.New("no binding sources provided")
)

// BindError represents a binding error with field-level context.
// It provides detailed information about which field failed, what value was
// provided, and what type was expected.
//
// Use [errors.As] to check for BindError:
//
//	var bindErr *BindError
//	if errors.As(err, &bindErr) {
//	    fmt.Printf("Field: %s, Source: %s\n", bindErr.Field, bindErr.Source)
//	}
type BindError struct {
	Field  string       // Field name that failed binding
	Source Source       // Binding source (typed)
	Value  string       // The value that failed conversion
	Type   reflect.Type // Expected Go type
	Reason string       // Human-readable reason for failure
	Err    error        // Underlying error
}

// Error returns a formatted error message with contextual hints.
func (e *BindError) Error() string {
	var base string
	if e.Reason != "" {
		base = fmt.Sprintf("binding field %q (%s): %s", e.Field, e.Source, e.Reason)
	} else {
		typeName := "unknown"
		if e.Type != nil {
			typeName = e.Type.String()
		}
		base = fmt.Sprintf("binding field %q (%s): failed to convert %q to %s: %v",
			e.Field, e.Source, e.Value, typeName, e.Err)
	}

	// Add contextual hints for common mistakes
	if hint := e.hint(); hint != "" {
		base += " (hint: " + hint + ")"
	}

	return base
}

// hint returns a contextual hint for common binding mistakes.
// It analyzes the error context and suggests fixes based on the field type
// and value that failed to bind.
func (e *BindError) hint() string {
	if e.Type == nil {
		return ""
	}

	// Decimal point in integer field
	if isIntType(e.Type) && strings.Contains(e.Value, ".") {
		return "use float type for decimal values"
	}

	// Time parsing failed
	if e.Type == timeType {
		return "use RFC3339 format (2006-01-02T15:04:05Z07:00) or configure custom layouts with TimeConverter"
	}

	// Duration parsing failed
	if e.Type == durationType {
		return "use Go duration format (e.g., '1h30m', '500ms') or configure aliases with DurationConverter"
	}

	// Boolean with unexpected value
	if e.Type.Kind() == reflect.Bool {
		return "accepted values: true/false, yes/no, 1/0, on/off, or configure custom values with BoolConverter"
	}

	// Slice parsing issues
	if e.Type.Kind() == reflect.Slice {
		if strings.Contains(e.Value, ",") {
			return "for CSV values, use comma-separated list; for repeated params, send multiple query parameters"
		}
		return "ensure value is properly formatted for slice type"
	}

	// Map parsing issues
	if e.Type.Kind() == reflect.Map {
		return "use dot notation (key.subkey=value) or bracket notation (key[subkey]=value)"
	}

	// Pointer type issues
	if e.Type.Kind() == reflect.Ptr {
		elemType := e.Type.Elem()
		if isIntType(elemType) && strings.Contains(e.Value, ".") {
			return "use float pointer type for decimal values"
		}
	}

	return ""
}

// isIntType returns true if the type is any integer type.
func isIntType(t reflect.Type) bool {
	if t == nil {
		return false
	}
	kind := t.Kind()
	return kind == reflect.Int || kind == reflect.Int8 || kind == reflect.Int16 ||
		kind == reflect.Int32 || kind == reflect.Int64 ||
		kind == reflect.Uint || kind == reflect.Uint8 || kind == reflect.Uint16 ||
		kind == reflect.Uint32 || kind == reflect.Uint64
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

// IsType returns true if the error is due to a type conversion failure.
func (e *BindError) IsType() bool {
	return e.Err != nil
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

// HTTPStatus implements rivaas.dev/errors.ErrorType.
func (e *UnknownFieldError) HTTPStatus() int {
	return 400 // Bad Request
}

// Code implements rivaas.dev/errors.ErrorCode.
func (e *UnknownFieldError) Code() string {
	return "unknown_field"
}

// MultiError aggregates multiple binding errors.
// It is returned when [WithAllErrors] is used and multiple fields fail binding.
//
// Use [errors.As] to check for MultiError:
//
//	var multi *MultiError
//	if errors.As(err, &multi) {
//	    for _, e := range multi.Errors {
//	        // Handle each error
//	    }
//	}
type MultiError struct {
	Errors []*BindError
}

// Error returns a formatted error message.
func (m *MultiError) Error() string {
	if len(m.Errors) == 0 {
		return "no errors"
	}
	if len(m.Errors) == 1 {
		return m.Errors[0].Error()
	}

	return fmt.Sprintf("%d binding errors occurred", len(m.Errors))
}

// Unwrap returns all errors for errors.Is/As compatibility.
func (m *MultiError) Unwrap() []error {
	errs := make([]error, 0, len(m.Errors))
	for _, e := range m.Errors {
		errs = append(errs, e)
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

// Add appends an error to the MultiError.
func (m *MultiError) Add(err *BindError) {
	m.Errors = append(m.Errors, err)
}

// HasErrors returns true if there are any errors.
func (m *MultiError) HasErrors() bool {
	return len(m.Errors) > 0
}

// ErrorOrNil returns nil if there are no errors, otherwise returns the MultiError.
func (m *MultiError) ErrorOrNil() error {
	if !m.HasErrors() {
		return nil
	}

	return m
}
