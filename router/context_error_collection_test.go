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

package router

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContext_Error_Collection tests the basic error collection mechanism
func TestContext_Error_Collection(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(w, req)

	// Initially no errors
	assert.False(t, c.HasErrors(), "Expected no errors initially")
	assert.Nil(t, c.Errors(), "Expected nil errors slice initially")

	// Collect first error
	err1 := errors.New("first error")
	c.Error(err1)

	assert.True(t, c.HasErrors(), "Expected errors after collecting one")
	collectedErrors := c.Errors()
	require.Len(t, collectedErrors, 1, "Expected 1 error")
	assert.Equal(t, err1, collectedErrors[0], "Expected first error to match")

	// Collect second error
	err2 := errors.New("second error")
	c.Error(err2)

	assert.Len(t, c.Errors(), 2, "Expected 2 errors")
	assert.Equal(t, err2, c.Errors()[1], "Expected second error to match")
}

// TestContext_Error_NilIgnored tests that nil errors are ignored
func TestContext_Error_NilIgnored(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(w, req)

	// Collecting nil should not add to errors
	c.Error(nil)

	assert.False(t, c.HasErrors(), "Expected no errors after collecting nil")
	assert.Nil(t, c.Errors(), "Expected nil errors slice after collecting nil")

	// Collect real error
	err := errors.New("real error")
	c.Error(err)

	// Collect nil again - should still only have one error
	c.Error(nil)

	assert.Len(t, c.Errors(), 1, "Expected 1 error after collecting nil")
}

// TestContext_Error_MultipleErrors tests collecting multiple errors
func TestContext_Error_MultipleErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		errors    []error
		wantCount int
	}{
		{
			name:      "single error",
			errors:    []error{errors.New("error 1")},
			wantCount: 1,
		},
		{
			name: "multiple errors",
			errors: []error{
				errors.New("error 1"),
				errors.New("error 2"),
				errors.New("error 3"),
			},
			wantCount: 3,
		},
		{
			name: "many errors",
			errors: []error{
				errors.New("error 1"),
				errors.New("error 2"),
				errors.New("error 3"),
				errors.New("error 4"),
				errors.New("error 5"),
			},
			wantCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			c := NewContext(w, req)

			for _, err := range tt.errors {
				c.Error(err)
			}

			assert.True(t, c.HasErrors(), "Expected errors after collecting multiple")

			collected := c.Errors()
			assert.Len(t, collected, tt.wantCount, "Expected %d errors", tt.wantCount)

			for i, expectedErr := range tt.errors {
				assert.Equal(t, expectedErr, collected[i], "Error at index %d does not match", i)
			}
		})
	}
}

// TestContext_Error_WithErrorsJoin tests integration with errors.Join
func TestContext_Error_WithErrorsJoin(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(w, req)

	err1 := errors.New("validation error")
	err2 := errors.New("database error")
	err3 := errors.New("network error")

	c.Error(err1)
	c.Error(err2)
	c.Error(err3)

	// Join all errors
	joinedErr := errors.Join(c.Errors()...)
	require.Error(t, joinedErr, "Expected joined error to be non-nil")

	// Verify joined error contains all errors
	errStr := joinedErr.Error()
	assert.NotEmpty(t, errStr, "Expected joined error to have error message")
}

// TestContext_Error_WithErrorsIs tests integration with errors.Is
func TestContext_Error_WithErrorsIs(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(w, req)

	// Collect sentinel errors
	c.Error(ErrContextResponseNil)
	c.Error(ErrContentTypeNotAllowed)
	c.Error(errors.New("generic error"))

	// Check if specific errors exist
	require.ErrorIs(t, c.Errors()[0], ErrContextResponseNil, "Expected first error to be ErrContextResponseNil")
	require.ErrorIs(t, c.Errors()[1], ErrContentTypeNotAllowed, "Expected second error to be ErrContentTypeNotAllowed")
	assert.NotErrorIs(t, c.Errors()[2], ErrContextResponseNil, "Expected third error not to be ErrContextResponseNil")
}

// CustomError is a custom error type for testing errors.As
type CustomError struct {
	Code    int
	Message string
}

func (e *CustomError) Error() string {
	return fmt.Sprintf("code %d: %s", e.Code, e.Message)
}

// TestContext_Error_WithErrorsAs tests integration with errors.As
func TestContext_Error_WithErrorsAs(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(w, req)

	ce := &CustomError{Code: http.StatusBadRequest, Message: "bad request"}
	c.Error(fmt.Errorf("wrapped: %w", ce))

	// Try to extract custom error
	var extracted *CustomError
	require.ErrorAs(t, c.Errors()[0], &extracted, "Expected to extract CustomError using errors.As")
	require.NotNil(t, extracted, "Expected extracted to be non-nil")
	assert.Equal(t, http.StatusBadRequest, extracted.Code)
	assert.Equal(t, "bad request", extracted.Message)
}

// TestContext_Error_ResetClearsErrors tests that reset clears errors
func TestContext_Error_ResetClearsErrors(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(w, req)

	// Collect errors
	c.Error(errors.New("error 1"))
	c.Error(errors.New("error 2"))

	assert.True(t, c.HasErrors(), "Expected errors before reset")

	// Reset context
	c.reset()

	assert.False(t, c.HasErrors(), "Expected no errors after reset")
	assert.Nil(t, c.Errors(), "Expected nil errors slice after reset")
}

// TestContext_JSON_CollectsErrors tests that JSON returns errors (not automatically collected)
func TestContext_JSON_CollectsErrors(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(w, req)

	// Type that cannot be marshaled to JSON
	type BadType struct {
		Channel chan int // Cannot be marshaled
	}

	badData := BadType{Channel: make(chan int)}

	// JSON should return error, not collect it automatically
	err := c.JSON(http.StatusOK, badData)
	require.Error(t, err, "Expected error to be returned from JSON encoding failure")

	// Error should NOT be automatically collected
	assert.False(t, c.HasErrors(), "Expected error NOT to be automatically collected")

	// If caller wants to collect, they must do so explicitly
	if err != nil {
		c.Error(err)
		assert.True(t, c.HasErrors(), "Expected error to be collected after explicit c.Error() call")

		collectedErrors := c.Errors()
		require.Len(t, collectedErrors, 1, "Expected 1 error after explicit collection")

		// Verify error message contains encoding failure info
		errMsg := collectedErrors[0].Error()
		assert.NotEmpty(t, errMsg, "Expected error message to be non-empty")
	}
}

// TestContext_JSON_ReturnsError tests that JSON returns error
func TestContext_JSON_ReturnsError(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(w, req)

	// Type that cannot be marshaled to JSON
	type BadType struct {
		Channel chan int
	}

	badData := BadType{Channel: make(chan int)}

	// JSON should return error
	err := c.JSON(http.StatusOK, badData)
	require.Error(t, err, "Expected JSON to return error for unencodable data")

	// Error should NOT be automatically collected
	assert.False(t, c.HasErrors(), "Expected JSON not to automatically collect errors")

	// But we can manually collect it
	c.Error(err)
	assert.True(t, c.HasErrors(), "Expected error after manually collecting")
}

// TestContext_AllResponseMethods_ReturnErrors tests all response methods return errors
func TestContext_AllResponseMethods_ReturnErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		callFunc func(*Context) error
	}{
		{
			name: "JSON",
			callFunc: func(c *Context) error {
				return c.JSON(http.StatusOK, make(chan int)) // Unencodable
			},
		},
		{
			name: "IndentedJSON",
			callFunc: func(c *Context) error {
				return c.IndentedJSON(http.StatusOK, make(chan int))
			},
		},
		{
			name: "PureJSON",
			callFunc: func(c *Context) error {
				return c.PureJSON(http.StatusOK, make(chan int))
			},
		},
		{
			name: "SecureJSON",
			callFunc: func(c *Context) error {
				return c.SecureJSON(http.StatusOK, make(chan int))
			},
		},
		{
			name: "ASCIIJSON",
			callFunc: func(c *Context) error {
				return c.ASCIIJSON(http.StatusOK, make(chan int))
			},
		},
		{
			name: "YAML",
			callFunc: func(c *Context) (err error) {
				// YAML panics for unencodable types, so we'll test with a recover
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("YAML encoding panicked: %v", r)
					}
				}()

				return c.YAML(http.StatusOK, struct{ Func func() }{Func: func() {}})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			c := NewContext(w, req)

			// Call the method - should return error if encoding fails
			err := tt.callFunc(c)

			// All methods should return errors, not collect them
			require.Error(t, err, "Expected %s to return error on encoding failure", tt.name)
			assert.False(t, c.HasErrors(), "Expected %s not to automatically collect errors", tt.name)
		})
	}
}

// TestContext_ErrorCollection_WithSuccessfulWrites tests that successful writes don't collect errors
func TestContext_ErrorCollection_WithSuccessfulWrites(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(w, req)

	// Successful JSON write
	err := c.JSON(http.StatusOK, map[string]string{"message": "success"})
	require.NoError(t, err)

	// Should not have errors
	assert.False(t, c.HasErrors(), "Expected no errors after successful JSON write")

	// Successful String write
	err = c.String(http.StatusOK, "Hello World")
	require.NoError(t, err)

	// Should still not have errors
	assert.False(t, c.HasErrors(), "Expected no errors after successful String write")
}

// TestContext_ErrorCollection_MixedSuccessAndFailure tests mixed success and failure
func TestContext_ErrorCollection_MixedSuccessAndFailure(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(w, req)

	// Successful write - should return no error
	err := c.JSON(http.StatusOK, map[string]string{"ok": "yes"})
	require.NoError(t, err, "Expected no error after successful write")
	assert.False(t, c.HasErrors(), "Expected no errors after successful write")

	// Failed write - should return error, not collect automatically
	err = c.JSON(http.StatusOK, make(chan int))
	require.Error(t, err, "Expected error to be returned from failed write")
	assert.False(t, c.HasErrors(), "Expected error NOT to be automatically collected")

	// Explicitly collect the error
	if err != nil {
		c.Error(err)
		assert.True(t, c.HasErrors(), "Expected error after explicit collection")
	}

	// Another successful write - error should still be there (manually collected)
	err = c.String(http.StatusOK, "text")
	require.NoError(t, err, "Expected no error from successful String call")
	assert.True(t, c.HasErrors(), "Expected manually collected error to persist after subsequent successful write")

	// Should have exactly one error
	assert.Len(t, c.Errors(), 1, "Expected 1 error")
}

// TestContext_ErrorCollection_RealWorldScenario tests a realistic error collection scenario
func TestContext_ErrorCollection_RealWorldScenario(t *testing.T) {
	t.Parallel()

	r := MustNew()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	r.GET("/test", func(c *Context) {
		// Simulate validation errors
		if err := validateUserID(""); err != nil {
			c.Error(err)
		}
		if err := validateEmail("invalid-email"); err != nil {
			c.Error(err)
		}

		// Check if any errors were collected
		if c.HasErrors() {
			// Combine errors
			joinedErr := errors.Join(c.Errors()...)
			c.JSON(http.StatusBadRequest, map[string]any{
				"error":  "validation failed",
				"errors": c.Errors(),
				"joined": joinedErr.Error(),
			})

			return
		}

		// Success case
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	r.ServeHTTP(w, req)

	// Should have validation errors
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Helper functions for realistic scenario test
func validateUserID(userID string) error {
	if userID == "" {
		return errors.New("user ID is required")
	}

	return nil
}

func validateEmail(email string) error {
	if email == "" || !contains(email, "@") {
		return errors.New("invalid email format")
	}

	return nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0)
}

// NOTE: Context is NOT thread-safe and is designed to be used by a single goroutine
// (the one handling the HTTP request). Concurrent access to a Context is not supported
// and will result in data races. Each HTTP request gets its own Context instance.
