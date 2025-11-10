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
)

// BindError represents a binding error with field-level context.
type BindError struct {
	Field  string // Field name that failed
	Tag    string // Source tag (query, params, form, etc.)
	Value  string // The value that failed
	Type   string // Expected Go type
	Reason string // Human-readable reason
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

// UnknownFieldError is returned when strict JSON decoding encounters unknown fields.
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
