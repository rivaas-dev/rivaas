package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"rivaas.dev/router"
)

// APIError represents a structured API error response.
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
	ErrUserNotFound = APIError{
		Code:    "USER_NOT_FOUND",
		Message: "User not found",
	}
	ErrInvalidInput = APIError{
		Code:    "INVALID_INPUT",
		Message: "Invalid input provided",
	}
	ErrValidationFailed = APIError{
		Code:    "VALIDATION_FAILED",
		Message: "Request validation failed",
	}
)

// HandleError processes errors and sends appropriate HTTP responses.
func HandleError(c *router.Context, err error) {
	var apiErr APIError
	if errors.As(err, &apiErr) {
		status := getHTTPStatusForError(apiErr.Code)
		c.JSON(status, apiErr)
		return
	}

	// Unknown error - return generic error
	c.JSON(http.StatusInternalServerError, APIError{
		Code:    "INTERNAL_ERROR",
		Message: "An internal error occurred",
	})
}

// getHTTPStatusForError maps error codes to HTTP status codes.
func getHTTPStatusForError(code string) int {
	switch code {
	case "USER_NOT_FOUND":
		return http.StatusNotFound
	case "INVALID_INPUT", "VALIDATION_FAILED":
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// WrapError wraps an error with additional context.
func WrapError(baseErr APIError, format string, args ...any) error {
	return fmt.Errorf("%w: %s", baseErr, fmt.Sprintf(format, args...))
}
