// Copyright 2025 The Rivaas Authors
// Copyright 2025 Company.info B.V.
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

package config

import "fmt"

// ConfigError represents a configuration error with detailed context.
// It provides information about where the error occurred (source, field),
// what operation was being performed, and the underlying error.
type ConfigError struct {
	Source    string // The source where the error occurred (e.g., "source[0]", "json-schema", "binding")
	Field     string // The specific field where the error occurred (optional)
	Operation string // The operation being performed (e.g., "load", "validate", "bind", "merge")
	Err       error  // The underlying error
}

// Error returns a formatted error message with context information.
// If Field is provided, it includes the field in the error message.
func (e *ConfigError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("config error in %s.%s during %s: %v",
			e.Source, e.Field, e.Operation, e.Err)
	}
	return fmt.Sprintf("config error in %s during %s: %v",
		e.Source, e.Operation, e.Err)
}

// Unwrap returns the underlying error, allowing for error chain inspection.
// This enables the use of errors.Is() and errors.As() with ConfigError.
func (e *ConfigError) Unwrap() error {
	return e.Err
}

// NewConfigError creates a new ConfigError with the provided context.
// This is a convenience function for creating ConfigError instances.
func NewConfigError(source, operation string, err error) *ConfigError {
	return &ConfigError{
		Source:    source,
		Operation: operation,
		Err:       err,
	}
}

// NewConfigFieldError creates a new ConfigError with field information.
// This is useful when the error is specific to a particular configuration field.
func NewConfigFieldError(source, field, operation string, err error) *ConfigError {
	return &ConfigError{
		Source:    source,
		Field:     field,
		Operation: operation,
		Err:       err,
	}
}
