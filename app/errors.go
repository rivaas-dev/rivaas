package app

import (
	"fmt"
	"time"
)

// ConfigError represents a configuration validation error with structured information.
// It provides detailed context about which field failed validation and why.
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
// This allows errors.Is() and errors.As() to work correctly.
func (e *ConfigError) Unwrap() error {
	return nil
}

// ValidationErrors represents multiple configuration validation errors.
// This allows collecting all validation errors before returning them.
type ValidationErrors struct {
	Errors []*ConfigError
}

// Error implements the error interface and returns a formatted error message
// listing all validation errors.
func (ve *ValidationErrors) Error() string {
	if len(ve.Errors) == 0 {
		return "validation errors: (no errors)"
	}
	if len(ve.Errors) == 1 {
		return ve.Errors[0].Error()
	}

	msg := fmt.Sprintf("validation errors (%d):", len(ve.Errors))
	for i, err := range ve.Errors {
		msg += fmt.Sprintf("\n  %d. %s", i+1, err.Error())
	}
	return msg
}

// Add appends a new ConfigError to the ValidationErrors.
func (ve *ValidationErrors) Add(err *ConfigError) {
	ve.Errors = append(ve.Errors, err)
}

// HasErrors returns true if there are any validation errors.
func (ve *ValidationErrors) HasErrors() bool {
	return len(ve.Errors) > 0
}

// ToError returns nil if there are no errors, otherwise returns the ValidationErrors
// as an error. This is useful for returning from validation functions.
func (ve *ValidationErrors) ToError() error {
	if !ve.HasErrors() {
		return nil
	}
	return ve
}

// Helper functions for creating common validation errors

// newFieldError creates a ConfigError for a field validation failure.
func newFieldError(field string, value any, message, constraint string) *ConfigError {
	return &ConfigError{
		Field:      field,
		Value:      value,
		Message:    message,
		Constraint: constraint,
	}
}

// newEmptyFieldError creates a ConfigError for an empty required field.
func newEmptyFieldError(field string) *ConfigError {
	return newFieldError(field, nil, "cannot be empty", "required")
}

// newInvalidValueError creates a ConfigError for an invalid field value.
func newInvalidValueError(field string, value any, message string) *ConfigError {
	return newFieldError(field, value, message, "")
}

// newInvalidEnumError creates a ConfigError for an invalid enum value.
func newInvalidEnumError(field string, value any, validValues []string) *ConfigError {
	return newFieldError(field, value,
		fmt.Sprintf("must be one of: %v", validValues),
		fmt.Sprintf("enum: %v", validValues))
}

// newTimeoutError creates a ConfigError for an invalid timeout value.
func newTimeoutError(field string, value time.Duration, constraint string) *ConfigError {
	return newFieldError(field, value,
		fmt.Sprintf("timeout must be positive, got: %s", value),
		constraint)
}

// newComparisonError creates a ConfigError for a field comparison failure
// (e.g., readTimeout > writeTimeout).
func newComparisonError(field1, field2 string, value1, value2 any, message string) *ConfigError {
	return newFieldError(field1, value1,
		fmt.Sprintf("%s (compared with %s: %v)", message, field2, value2),
		fmt.Sprintf("%s vs %s", field1, field2))
}
