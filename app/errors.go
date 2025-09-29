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

package app

import (
	"fmt"
	"strings"
	"time"
)

// ConfigError represents a configuration validation error with structured information.
//
// ConfigError provides structured error information that enables:
//   - Programmatic error inspection (field-level error detection)
//   - Rich formatting for CLI/web UI display
//   - Error aggregation for batch validation
//   - Internationalization support (field names remain constant)
//
// Validation happens once at startup, not during request handling.
type ConfigError struct {
	// Field is the name of the configuration field that failed validation
	Field string
	// Value is the actual value that was provided (may be nil for missing values)
	Value any
	// Message is a human-readable error message explaining the validation failure
	Message string
	// Constraint is an optional constraint that was violated (e.g., "must be positive", "must be between X and Y")
	Constraint string
}

// Error implements the error interface and returns a formatted error message.
// Error formats the ConfigError as a human-readable string.
func (e *ConfigError) Error() string {
	if e.Constraint != "" {
		return fmt.Sprintf("configuration error in %s: %s (constraint: %s, value: %v)",
			e.Field, e.Message, e.Constraint, e.Value)
	}
	if e.Value != nil {
		return fmt.Sprintf("configuration error in %s: %s (value: %v)",
			e.Field, e.Message, e.Value)
	}
	return fmt.Sprintf("configuration error in %s: %s", e.Field, e.Message)
}

// Unwrap returns nil as ConfigError is a leaf error type.
// Unwrap allows errors.Is() and errors.As() to work correctly.
func (e *ConfigError) Unwrap() error {
	return nil
}

// ValidationError represents multiple configuration validation errors.
// ValidationError allows collecting all validation errors before returning them.
type ValidationError struct {
	Errors []*ConfigError
}

// Error implements the error interface and returns a formatted error message
// listing all validation errors.
// Error formats all collected validation errors into a single message.
func (ve *ValidationError) Error() string {
	if len(ve.Errors) == 0 {
		return "validation errors: (no errors)"
	}
	if len(ve.Errors) == 1 {
		return ve.Errors[0].Error()
	}

	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("validation errors (%d):", len(ve.Errors)))
	for i, err := range ve.Errors {
		msg.WriteString(fmt.Sprintf("\n  %d. %s", i+1, err.Error()))
	}
	return msg.String()
}

// Add appends a new ConfigError to the ValidationError.
// Add collects validation errors for batch reporting.
func (ve *ValidationError) Add(err *ConfigError) {
	ve.Errors = append(ve.Errors, err)
}

// HasErrors returns true if there are any validation errors.
// HasErrors checks if the ValidationError contains any errors.
func (ve *ValidationError) HasErrors() bool {
	return len(ve.Errors) > 0
}

// ToError returns nil if there are no errors, otherwise returns the ValidationError
// as an error.
// ToError is useful for returning from validation functions.
func (ve *ValidationError) ToError() error {
	if !ve.HasErrors() {
		return nil
	}
	return ve
}

// Helper functions for creating common validation errors

// newFieldError creates a ConfigError for a field validation failure.
// newFieldError is a private helper for creating validation errors.
func newFieldError(field string, value any, message, constraint string) *ConfigError {
	return &ConfigError{
		Field:      field,
		Value:      value,
		Message:    message,
		Constraint: constraint,
	}
}

// newEmptyFieldError creates a ConfigError for an empty required field.
// newEmptyFieldError is a private helper for creating validation errors.
func newEmptyFieldError(field string) *ConfigError {
	return newFieldError(field, nil, "cannot be empty", "required")
}

// newInvalidValueError creates a ConfigError for an invalid field value.
// newInvalidValueError is a private helper for creating validation errors.
func newInvalidValueError(field string, value any, message string) *ConfigError {
	return newFieldError(field, value, message, "")
}

// newInvalidEnumError creates a ConfigError for an invalid enum value.
// newInvalidEnumError is a private helper for creating validation errors.
func newInvalidEnumError(field string, value any, validValues []string) *ConfigError {
	return newFieldError(field, value,
		fmt.Sprintf("must be one of: %v", validValues),
		fmt.Sprintf("enum: %v", validValues))
}

// newTimeoutError creates a ConfigError for an invalid timeout value.
// newTimeoutError is a private helper for creating validation errors.
func newTimeoutError(field string, value time.Duration, constraint string) *ConfigError {
	return newFieldError(field, value,
		fmt.Sprintf("timeout must be positive, got: %s", value),
		constraint)
}

// newComparisonError creates a ConfigError for a field comparison failure
// (e.g., readTimeout > writeTimeout).
// newComparisonError is a private helper for creating validation errors.
func newComparisonError(field1, field2 string, value1, value2 any, message string) *ConfigError {
	return newFieldError(field1, value1,
		fmt.Sprintf("%s (compared with %s: %v)", message, field2, value2),
		fmt.Sprintf("%s vs %s", field1, field2))
}
