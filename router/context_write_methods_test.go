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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContext_JSON_Success tests successful JSON call
func TestContext_JSON_Success(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	data := map[string]string{"message": "success"}
	err := c.JSON(http.StatusOK, data)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
	assert.False(t, c.HasErrors(), "Expected no errors after successful JSON")
}

// TestContext_JSON_EncodingError_ReturnsError tests that JSON returns error on encoding failure
func TestContext_JSON_EncodingError_ReturnsError(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	// Type that cannot be marshaled
	type BadType struct {
		Channel chan int
	}

	err := c.JSON(http.StatusOK, BadType{Channel: make(chan int)})

	assert.Error(t, err, "Expected error for unencodable data")
	assert.False(t, c.HasErrors(), "Expected JSON not to automatically collect errors")
	assert.NotEmpty(t, err.Error(), "Expected non-empty error message")
}

// TestContext_JSON_NilResponse tests JSON when Response is nil
func TestContext_JSON_NilResponse(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(nil, req) // nil Response

	err := c.JSON(http.StatusOK, map[string]string{"test": "value"})

	require.Error(t, err, "Expected error when Response is nil")
	assert.ErrorIs(t, err, ErrContextResponseNil, "Expected ErrContextResponseNil")
}

// TestContext_Stringf_Success tests successful Stringf call
func TestContext_Stringf_Success(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	err := c.Stringf(http.StatusOK, "Hello %s", "World")

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Hello World", w.Body.String())
}

// TestContext_String_PlainText tests String without formatting
func TestContext_String_PlainText(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	err := c.String(http.StatusOK, "Plain text")

	assert.NoError(t, err)
	assert.Equal(t, "Plain text", w.Body.String())
}

// TestContext_HTML_Success tests successful HTML call
func TestContext_HTML_Success(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	html := "<h1>Title</h1><p>Content</p>"
	err := c.HTML(http.StatusOK, html)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/html", w.Header().Get("Content-Type"))
	assert.Equal(t, html, w.Body.String())
}

// TestContext_Data_Success tests successful Data call
func TestContext_Data_Success(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	data := []byte("test data")
	err := c.Data(http.StatusOK, "application/octet-stream", data)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, string(data), w.Body.String())
}

// TestContext_Data_EmptyContentType tests Data with empty content type
func TestContext_Data_EmptyContentType(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	data := []byte("test")
	err := c.Data(http.StatusOK, "", data)

	assert.NoError(t, err)
	// Should default to application/octet-stream
	assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
}

// TestContext_JSON_ReturnsErrors tests that JSON returns errors explicitly
func TestContext_JSON_ReturnsErrors(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	// JSON should return error explicitly
	err := c.JSON(http.StatusOK, make(chan int))

	require.Error(t, err, "Expected JSON to return error")
	assert.False(t, c.HasErrors(), "Expected JSON not to automatically collect errors")
}

// TestContext_ManualErrorCollection tests manual error collection
func TestContext_ManualErrorCollection(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	// Use JSON and manually collect error if needed
	err := c.JSON(http.StatusOK, make(chan int))
	if err != nil {
		c.Error(err)
	}

	assert.True(t, c.HasErrors(), "Expected error after manual collection")
}

// TestContext_ResponseMethods_AllVariants tests all response method variants
func TestContext_ResponseMethods_AllVariants(t *testing.T) {
	t.Parallel()

	validData := map[string]string{"key": "value"}
	invalidData := make(chan int)

	tests := []struct {
		name        string
		callValid   func(*Context) error
		callInvalid func(*Context) error
	}{
		{
			name: "JSON",
			callValid: func(c *Context) error {
				return c.JSON(200, validData)
			},
			callInvalid: func(c *Context) error {
				return c.JSON(200, invalidData)
			},
		},
		{
			name: "IndentedJSON",
			callValid: func(c *Context) error {
				return c.IndentedJSON(200, validData)
			},
			callInvalid: func(c *Context) error {
				return c.IndentedJSON(200, invalidData)
			},
		},
		{
			name: "PureJSON",
			callValid: func(c *Context) error {
				return c.PureJSON(200, validData)
			},
			callInvalid: func(c *Context) error {
				return c.PureJSON(200, invalidData)
			},
		},
		{
			name: "SecureJSON",
			callValid: func(c *Context) error {
				return c.SecureJSON(200, validData)
			},
			callInvalid: func(c *Context) error {
				return c.SecureJSON(200, invalidData)
			},
		},
		{
			name: "ASCIIJSON",
			callValid: func(c *Context) error {
				return c.ASCIIJSON(200, validData)
			},
			callInvalid: func(c *Context) error {
				return c.ASCIIJSON(200, invalidData)
			},
		},
		{
			name: "String",
			callValid: func(c *Context) error {
				return c.String(200, "test")
			},
			callInvalid: nil, // String doesn't have encoding errors
		},
		{
			name: "HTML",
			callValid: func(c *Context) error {
				return c.HTML(200, "<h1>test</h1>")
			},
			callInvalid: nil, // HTML doesn't have encoding errors
		},
		{
			name: "Data",
			callValid: func(c *Context) error {
				return c.Data(200, "text/plain", []byte("test"))
			},
			callInvalid: nil, // Data doesn't have encoding errors
		},
		{
			name: "YAML",
			callValid: func(c *Context) error {
				return c.YAML(200, validData)
			},
			callInvalid: func(c *Context) (err error) {
				// YAML encoding panics on channels, so we need to recover
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("YAML encoding panicked: %v", r)
					}
				}()
				return c.YAML(200, invalidData)
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
			require.NoError(t, err, "Expected no error for valid data")
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
				assert.False(t, c.HasErrors(), "Expected response methods not to automatically collect errors")
			})
		}
	}
}

// TestContext_ResponseMethods_HeadersAlreadyWritten tests response methods with headers already written
func TestContext_ResponseMethods_HeadersAlreadyWritten(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	// Write headers first
	c.Response.WriteHeader(http.StatusOK)

	// Then write JSON - should not error
	err := c.JSON(http.StatusOK, map[string]string{"test": "value"})
	assert.NoError(t, err, "Expected no error when headers already written")
}

// TestContext_ResponseMethods_ResponseNil tests response methods with nil response
func TestContext_ResponseMethods_ResponseNil(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(nil, req)

	err := c.JSON(http.StatusOK, map[string]string{"test": "value"})
	require.Error(t, err, "Expected error when Response is nil")
	assert.ErrorIs(t, err, ErrContextResponseNil, "Expected ErrContextResponseNil")
}
