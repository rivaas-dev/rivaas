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

// Package handlers provides error handling utilities and structured error types
// for the blog API.
package handlers

import (
	"errors"
	"fmt"

	"rivaas.dev/app"
)

// APIError represents a structured API error response.
// It provides a consistent format for error responses across the API.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func (e APIError) Error() string {
	return e.Message
}

// Predefined error constants for consistent error handling.
var (
	ErrPostNotFound = APIError{
		Code:    "POST_NOT_FOUND",
		Message: "Post not found",
	}
	ErrAuthorNotFound = APIError{
		Code:    "AUTHOR_NOT_FOUND",
		Message: "Author not found",
	}
	ErrSlugTaken = APIError{
		Code:    "SLUG_TAKEN",
		Message: "A post with this slug already exists",
	}
	ErrInvalidInput = APIError{
		Code:    "INVALID_INPUT",
		Message: "Invalid input provided",
	}
	ErrValidationFailed = APIError{
		Code:    "VALIDATION_FAILED",
		Message: "Request validation failed",
	}
	ErrCannotPublish = APIError{
		Code:    "CANNOT_PUBLISH",
		Message: "Cannot publish post",
	}
)

// HandleError processes errors and sends appropriate HTTP responses using the new Fail API.
// It checks if the error is an APIError and maps it to the corresponding
// HTTP status code using convenience methods. Unknown errors are returned as internal server errors.
//
// Example:
//
//	HandleError(c, ErrPostNotFound)
//	HandleError(c, WrapError(ErrValidationFailed, "title is required"))
func HandleError(c *app.Context, err error) {
	var apiErr APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case "POST_NOT_FOUND", "AUTHOR_NOT_FOUND":
			c.NotFound(err)
		case "SLUG_TAKEN":
			c.Conflict(err)
		case "INVALID_INPUT", "VALIDATION_FAILED", "CANNOT_PUBLISH":
			c.BadRequest(err)
		default:
			c.InternalError(err)
		}
		return
	}

	// Unknown error - return generic internal error
	c.InternalError(err)
}

// WrapError wraps an APIError with additional context using fmt.Errorf.
// It returns an error that wraps the base error and includes formatted details.
//
// Example:
//
//	err := WrapError(ErrValidationFailed, "title must be between 2 and 200 characters")
//	return err
func WrapError(baseErr APIError, format string, args ...any) error {
	wrapped := baseErr
	wrapped.Details = fmt.Sprintf(format, args...)
	return wrapped
}
