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

// TestContext_WriteJSON_Success tests successful WriteJSON call
func TestContext_WriteJSON_Success(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	data := map[string]string{"message": "success"}
	err := c.WriteJSON(http.StatusOK, data)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
	assert.False(t, c.HasErrors(), "Expected no errors after successful WriteJSON")
}

// TestContext_WriteJSON_EncodingError tests WriteJSON with encoding error
func TestContext_WriteJSON_EncodingError(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	// Type that cannot be marshaled
	type BadType struct {
		Channel chan int
	}

	err := c.WriteJSON(http.StatusOK, BadType{Channel: make(chan int)})

	assert.Error(t, err, "Expected error for unencodable data")
	assert.False(t, c.HasErrors(), "Expected WriteJSON not to automatically collect errors")
	assert.NotEmpty(t, err.Error(), "Expected non-empty error message")
}

// TestContext_WriteJSON_NilResponse tests WriteJSON when Response is nil
func TestContext_WriteJSON_NilResponse(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(nil, req) // nil Response

	err := c.WriteJSON(http.StatusOK, map[string]string{"test": "value"})

	require.Error(t, err, "Expected error when Response is nil")
	assert.ErrorIs(t, err, ErrContextResponseNil, "Expected ErrContextResponseNil")
}

// TestContext_WriteString_Success tests successful WriteStringf call
func TestContext_WriteString_Success(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	err := c.WriteStringf(http.StatusOK, "Hello %s", "World")

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Hello World", w.Body.String())
}

// TestContext_WriteString_PlainText tests WriteString without formatting
func TestContext_WriteString_PlainText(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	err := c.WriteString(http.StatusOK, "Plain text")

	assert.NoError(t, err)
	assert.Equal(t, "Plain text", w.Body.String())
}

// TestContext_WriteHTML_Success tests successful WriteHTML call
func TestContext_WriteHTML_Success(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	html := "<h1>Title</h1><p>Content</p>"
	err := c.WriteHTML(http.StatusOK, html)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/html", w.Header().Get("Content-Type"))
	assert.Equal(t, html, w.Body.String())
}

// TestContext_WriteData_Success tests successful WriteData call
func TestContext_WriteData_Success(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	data := []byte("test data")
	err := c.WriteData(http.StatusOK, "application/octet-stream", data)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, string(data), w.Body.String())
}

// TestContext_WriteData_EmptyContentType tests WriteData with empty content type
func TestContext_WriteData_EmptyContentType(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	data := []byte("test")
	err := c.WriteData(http.StatusOK, "", data)

	assert.NoError(t, err)
	// Should default to application/octet-stream
	assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
}

// TestContext_HighLevelVsLowLevel tests the difference between high-level and low-level methods
func TestContext_HighLevelVsLowLevel(t *testing.T) {
	t.Parallel()

	t.Run("high-level collects errors", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		c := NewContext(w, req)

		// High-level JSON should collect error automatically
		c.JSON(http.StatusOK, make(chan int))

		assert.True(t, c.HasErrors(), "Expected high-level JSON to collect errors")
	})

	t.Run("low-level returns errors", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		c := NewContext(w, req)

		// Low-level WriteJSON should return error
		err := c.WriteJSON(http.StatusOK, make(chan int))

		assert.Error(t, err, "Expected low-level WriteJSON to return error")
		assert.False(t, c.HasErrors(), "Expected low-level WriteJSON not to collect errors automatically")
	})

	t.Run("manual error collection with low-level", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		c := NewContext(w, req)

		// Use low-level and manually collect error
		err := c.WriteJSON(http.StatusOK, make(chan int))
		if err != nil {
			c.Error(err)
		}

		assert.True(t, c.HasErrors(), "Expected error after manual collection")
	})
}

// TestContext_WriteMethods_AllVariants tests all WriteXxx method variants
func TestContext_WriteMethods_AllVariants(t *testing.T) {
	t.Parallel()

	validData := map[string]string{"key": "value"}
	invalidData := make(chan int)

	tests := []struct {
		name        string
		callValid   func(*Context) error
		callInvalid func(*Context) error
	}{
		{
			name: "WriteJSON",
			callValid: func(c *Context) error {
				return c.WriteJSON(200, validData)
			},
			callInvalid: func(c *Context) error {
				return c.WriteJSON(200, invalidData)
			},
		},
		{
			name: "WriteIndentedJSON",
			callValid: func(c *Context) error {
				return c.WriteIndentedJSON(200, validData)
			},
			callInvalid: func(c *Context) error {
				return c.WriteIndentedJSON(200, invalidData)
			},
		},
		{
			name: "WritePureJSON",
			callValid: func(c *Context) error {
				return c.WritePureJSON(200, validData)
			},
			callInvalid: func(c *Context) error {
				return c.WritePureJSON(200, invalidData)
			},
		},
		{
			name: "WriteSecureJSON",
			callValid: func(c *Context) error {
				return c.WriteSecureJSON(200, validData)
			},
			callInvalid: func(c *Context) error {
				return c.WriteSecureJSON(200, invalidData)
			},
		},
		{
			name: "WriteASCIIJSON",
			callValid: func(c *Context) error {
				return c.WriteASCIIJSON(200, validData)
			},
			callInvalid: func(c *Context) error {
				return c.WriteASCIIJSON(200, invalidData)
			},
		},
		{
			name: "WriteString",
			callValid: func(c *Context) error {
				return c.WriteString(200, "test")
			},
			callInvalid: nil, // String doesn't have encoding errors
		},
		{
			name: "WriteHTML",
			callValid: func(c *Context) error {
				return c.WriteHTML(200, "<h1>test</h1>")
			},
			callInvalid: nil, // HTML doesn't have encoding errors
		},
		{
			name: "WriteData",
			callValid: func(c *Context) error {
				return c.WriteData(200, "text/plain", []byte("test"))
			},
			callInvalid: nil, // Data doesn't have encoding errors
		},
		{
			name: "WriteYAML",
			callValid: func(c *Context) error {
				return c.WriteYAML(200, validData)
			},
			callInvalid: func(c *Context) (err error) {
				// YAML encoding panics on channels, so we need to recover
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("YAML encoding panicked: %v", r)
					}
				}()
				return c.WriteYAML(200, invalidData)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_success", func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			c := NewContext(w, req)

			err := tt.callValid(c)
			assert.NoError(t, err, "Expected no error for valid data")
			assert.False(t, c.HasErrors(), "Expected no errors after successful write")
		})

		if tt.callInvalid != nil {
			t.Run(tt.name+"_error", func(t *testing.T) {
				t.Parallel()

				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/", nil)
				c := NewContext(w, req)

				err := tt.callInvalid(c)
				assert.Error(t, err, "Expected error for invalid data")
				assert.False(t, c.HasErrors(), "Expected WriteXxx methods not to automatically collect errors")
			})
		}
	}
}

// TestContext_WriteMethods_HeadersAlreadyWritten tests WriteXxx with headers already written
func TestContext_WriteMethods_HeadersAlreadyWritten(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	// Write headers first
	c.Response.WriteHeader(http.StatusOK)

	// Then write JSON - should not error
	err := c.WriteJSON(http.StatusOK, map[string]string{"test": "value"})
	assert.NoError(t, err, "Expected no error when headers already written")
}

// TestContext_WriteMethods_ResponseNil tests WriteXxx with nil response
func TestContext_WriteMethods_ResponseNil(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(nil, req)

	err := c.WriteJSON(http.StatusOK, map[string]string{"test": "value"})
	require.Error(t, err, "Expected error when Response is nil")
	assert.True(t, errors.Is(err, ErrContextResponseNil), "Expected ErrContextResponseNil, got: %v", err)
}
