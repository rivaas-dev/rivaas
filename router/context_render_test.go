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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestIndentedJSON tests pretty-printed JSON rendering
func TestIndentedJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		data           any
		expectedStatus int
		checkBody      func(*testing.T, string)
		shouldError    bool
	}{
		{
			name: "basic struct formatting",
			data: map[string]any{
				"id":   123,
				"name": "John",
			},
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				t.Helper()
				// Should be indented
				assert.Contains(t, body, "  ", "Expected indented JSON")
				// Should contain newlines
				assert.Contains(t, body, "\n", "Expected formatted JSON with newlines")
				// Should be valid JSON
				var result map[string]any
				err := json.Unmarshal([]byte(body), &result)
				assert.NoError(t, err, "Invalid JSON")
			},
		},
		{
			name: "nested objects",
			data: map[string]any{
				"user": map[string]any{
					"id":   1,
					"name": "Alice",
				},
				"settings": map[string]bool{
					"notifications": true,
				},
			},
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				t.Helper()
				// Should have proper indentation for nested objects
				assert.Contains(t, body, "    ", "Expected nested indentation")
			},
		},
		{
			name:           "empty object",
			data:           map[string]string{},
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "{}", "Expected empty object {}")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			c := NewContext(w, req)

			if tt.shouldError {
				err := c.IndentedJSON(tt.expectedStatus, tt.data)
				assert.Error(t, err, "Expected error but got none")
			} else {
				require.NoError(t, c.IndentedJSON(tt.expectedStatus, tt.data))
			}

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

			if tt.checkBody != nil {
				tt.checkBody(t, w.Body.String())
			}
		})
	}
}

// TestPureJSON tests JSON without HTML escaping
func TestPureJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		data           any
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "HTML tags unescaped",
			data: map[string]string{
				"html": "<h1>Title</h1>",
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"html":"<h1>Title</h1>"}`,
		},
		{
			name: "ampersand unescaped",
			data: map[string]string{
				"url": "https://example.com?foo=bar&baz=qux",
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"url":"https://example.com?foo=bar&baz=qux"}`,
		},
		{
			name: "greater than and less than unescaped",
			data: map[string]string{
				"comparison": "x < 10 && y > 5",
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"comparison":"x < 10 && y > 5"}`,
		},
		{
			name: "mixed HTML content",
			data: map[string]string{
				"content": "<script>alert('test')</script>",
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"content":"<script>alert('test')</script>"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			c := NewContext(w, req)

			c.PureJSON(tt.expectedStatus, tt.data)

			assert.Equal(t, tt.expectedStatus, w.Code)
			body := strings.TrimSpace(w.Body.String())
			assert.Equal(t, tt.expectedBody, body)
			assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
		})
	}
}

// TestPureJSON_vs_JSON_Escaping compares escaping behavior
func TestPureJSON_vs_JSON_Escaping(t *testing.T) {
	t.Parallel()

	data := map[string]string{
		"html": "<h1>Title</h1>",
		"url":  "https://example.com?a=1&b=2",
	}

	// Test standard JSON (should escape)
	wJSON := httptest.NewRecorder()
	reqJSON := httptest.NewRequest("GET", "/", nil)
	cJSON := NewContext(wJSON, reqJSON)
	cJSON.JSON(http.StatusOK, data)
	jsonBody := wJSON.Body.String()

	// Test PureJSON (should NOT escape)
	wPure := httptest.NewRecorder()
	reqPure := httptest.NewRequest("GET", "/", nil)
	cPure := NewContext(wPure, reqPure)
	cPure.PureJSON(http.StatusOK, data)
	pureBody := wPure.Body.String()

	// Verify JSON() escapes HTML
	assert.True(t, strings.Contains(jsonBody, "\\u003c") || strings.Contains(jsonBody, "\\u003e"), "Standard JSON should escape < and >")
	assert.Contains(t, jsonBody, "\\u0026", "Standard JSON should escape &")

	// Verify PureJSON() does NOT escape HTML
	assert.False(t, strings.Contains(pureBody, "\\u003c") || strings.Contains(pureBody, "\\u003e"), "PureJSON should NOT escape < and >")
	assert.NotContains(t, pureBody, "\\u0026", "PureJSON should NOT escape &")

	// Both should produce valid JSON
	var resultJSON, resultPure map[string]string
	err := json.Unmarshal([]byte(jsonBody), &resultJSON)
	assert.NoError(t, err, "JSON() output is not valid JSON")
	err = json.Unmarshal([]byte(pureBody), &resultPure)
	assert.NoError(t, err, "PureJSON() output is not valid JSON")

	// Both should unmarshal to same values
	assert.Equal(t, resultJSON["html"], resultPure["html"], "JSON and PureJSON should unmarshal to same values")
}

// TestSecureJSON tests JSON with security prefix
func TestSecureJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		data           any
		prefix         []string
		expectedStatus int
		expectedPrefix string
	}{
		{
			name:           "default prefix",
			data:           []string{"secret1", "secret2"},
			prefix:         nil,
			expectedStatus: http.StatusOK,
			expectedPrefix: "while(1);",
		},
		{
			name:           "custom prefix",
			data:           map[string]string{"key": "value"},
			prefix:         []string{"for(;;);"},
			expectedStatus: http.StatusOK,
			expectedPrefix: "for(;;);",
		},
		{
			name:           "empty custom prefix uses default",
			data:           map[string]int{"count": 42},
			prefix:         []string{""},
			expectedStatus: http.StatusOK,
			expectedPrefix: "while(1);",
		},
		{
			name:           "JSON array (hijacking target)",
			data:           []int{1, 2, 3, 4, 5},
			prefix:         nil,
			expectedStatus: http.StatusOK,
			expectedPrefix: "while(1);",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			c := NewContext(w, req)

			if len(tt.prefix) > 0 {
				c.SecureJSON(tt.expectedStatus, tt.data, tt.prefix[0])
			} else {
				c.SecureJSON(tt.expectedStatus, tt.data)
			}

			assert.Equal(t, tt.expectedStatus, w.Code)
			body := w.Body.String()
			assert.True(t, strings.HasPrefix(body, tt.expectedPrefix), "Expected prefix %q, got body: %q", tt.expectedPrefix, body)

			// Verify that removing prefix gives valid JSON
			jsonPart := strings.TrimPrefix(body, tt.expectedPrefix)
			var result any
			err := json.Unmarshal([]byte(jsonPart), &result)
			assert.NoError(t, err, "Content after prefix is not valid JSON")
		})
	}
}

// TestASCIIJSON tests ASCII-only JSON encoding
func TestASCIIJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		data           any
		expectedStatus int
		checkBody      func(*testing.T, string)
	}{
		{
			name: "Unicode characters",
			data: map[string]string{
				"message": "Hello 世界",
			},
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				t.Helper()
				// Should contain \u escape sequences
				assert.Contains(t, body, "\\u", "Expected Unicode escape sequences")
				// Should be pure ASCII (no bytes >= 128)
				for _, b := range []byte(body) {
					assert.Less(t, b, byte(128), "Found non-ASCII byte: %d (%c)", b, b)
				}
			},
		},
		{
			name: "emoji characters",
			data: map[string]string{
				"emoji": "🌍🎉",
			},
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				t.Helper()
				// Should escape emoji to \uXXXX
				assert.Contains(t, body, "\\u", "Expected emoji to be escaped")
				// Should be valid JSON that unmarshals correctly
				var result map[string]string
				err := json.Unmarshal([]byte(body), &result)
				assert.NoError(t, err, "Invalid JSON")
				assert.Equal(t, "🌍🎉", result["emoji"], "Emoji should unmarshal to original value")
			},
		},
		{
			name: "mixed ASCII and non-ASCII",
			data: map[string]string{
				"name":    "José",
				"city":    "São Paulo",
				"message": "Hello",
			},
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				t.Helper()
				// ASCII part should be unchanged
				assert.Contains(t, body, "Hello", "ASCII text should not be escaped")
				// Non-ASCII should be escaped
				assert.NotContains(t, body, "José", "Non-ASCII characters should be escaped")
			},
		},
		{
			name: "pure ASCII (no escaping needed)",
			data: map[string]string{
				"text": "Plain ASCII text 123",
			},
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				t.Helper()
				// Should not have escape sequences for pure ASCII
				assert.Equal(t, 0, strings.Count(body, "\\u"), "Pure ASCII should not have escape sequences")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			c := NewContext(w, req)

			c.ASCIIJSON(tt.expectedStatus, tt.data)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

			if tt.checkBody != nil {
				tt.checkBody(t, w.Body.String())
			}
		})
	}
}

// TestYAML tests YAML rendering
func TestYAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		data           any
		expectedStatus int
		checkBody      func(*testing.T, string)
	}{
		{
			name: "basic map",
			data: map[string]any{
				"database": map[string]string{
					"host": "localhost",
					"port": "5432",
				},
				"debug": true,
			},
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				t.Helper()
				// Should be valid YAML
				var result map[string]any
				err := yaml.Unmarshal([]byte(body), &result)
				require.NoError(t, err, "Invalid YAML")
				// Should contain YAML formatting
				assert.Contains(t, body, ":", "Expected YAML key:value format")
			},
		},
		{
			name:           "array",
			data:           []string{"item1", "item2", "item3"},
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				t.Helper()
				// Should be valid YAML
				var result []string
				err := yaml.Unmarshal([]byte(body), &result)
				assert.NoError(t, err, "Invalid YAML")
				assert.Len(t, result, 3, "Expected 3 items")
			},
		},
		{
			name: "nested structures",
			data: map[string]any{
				"server": map[string]any{
					"host":  "localhost",
					"ports": []int{8080, 8081},
				},
			},
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				t.Helper()
				var result map[string]any
				err := yaml.Unmarshal([]byte(body), &result)
				assert.NoError(t, err, "Invalid YAML")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			c := NewContext(w, req)

			c.YAML(tt.expectedStatus, tt.data)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/x-yaml; charset=utf-8", w.Header().Get("Content-Type"))

			if tt.checkBody != nil {
				tt.checkBody(t, w.Body.String())
			}
		})
	}
}

// TestDataFromReader tests streaming from io.Reader
func TestDataFromReader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		contentLength  int64
		contentType    string
		data           string
		extraHeaders   map[string]string
		expectedStatus int
	}{
		{
			name:           "basic streaming with known length",
			contentLength:  13,
			contentType:    "text/plain",
			data:           "Hello, World!",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "streaming without content length",
			contentLength:  -1,
			contentType:    "application/octet-stream",
			data:           "Binary data here",
			expectedStatus: http.StatusOK,
		},
		{
			name:          "streaming with extra headers",
			contentLength: 10,
			contentType:   "application/pdf",
			data:          "PDF data..",
			extraHeaders: map[string]string{
				"Content-Disposition": `attachment; filename="document.pdf"`,
				"Cache-Control":       "no-cache",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "large data stream",
			contentLength:  1024,
			contentType:    "application/octet-stream",
			data:           strings.Repeat("A", 1024),
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			c := NewContext(w, req)

			reader := strings.NewReader(tt.data)

			err := c.DataFromReader(tt.expectedStatus, tt.contentLength, tt.contentType, reader, tt.extraHeaders)
			require.NoError(t, err, "Unexpected error")

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.contentType != "" {
				assert.Equal(t, tt.contentType, w.Header().Get("Content-Type"))
			}

			if tt.contentLength >= 0 {
				expectedLength := strconv.FormatInt(tt.contentLength, 10)
				assert.Equal(t, expectedLength, w.Header().Get("Content-Length"))
			}

			// Verify extra headers
			for key, expectedValue := range tt.extraHeaders {
				assert.Equal(t, expectedValue, w.Header().Get(key), "Expected header %s", key)
			}

			// Verify streamed data
			assert.Equal(t, tt.data, w.Body.String())
		})
	}
}

// TestDataFromReader_Error tests error handling
func TestDataFromReader_Error(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	// Create a reader that returns an error
	errorReader := &errorReader{err: io.ErrUnexpectedEOF}

	err := c.DataFromReader(200, -1, "text/plain", errorReader, nil)
	assert.Error(t, err, "Expected error from failing reader")
	assert.Contains(t, err.Error(), "streaming from reader failed", "Expected streaming error")
}

// errorReader is a test helper that always returns an error
type errorReader struct {
	err error
}

func (er *errorReader) Read(_ []byte) (n int, err error) {
	return 0, er.err
}

// TestData tests custom content type data sending
func TestData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		contentType    string
		data           []byte
		expectedStatus int
		expectedCT     string
	}{
		{
			name:           "PNG image",
			contentType:    "image/png",
			data:           []byte{0x89, 0x50, 0x4E, 0x47}, // PNG header
			expectedStatus: http.StatusOK,
			expectedCT:     "image/png",
		},
		{
			name:           "PDF document",
			contentType:    "application/pdf",
			data:           []byte("%PDF-1.4"),
			expectedStatus: http.StatusOK,
			expectedCT:     "application/pdf",
		},
		{
			name:           "custom binary",
			contentType:    "application/octet-stream",
			data:           []byte{0x00, 0x01, 0x02, 0x03},
			expectedStatus: http.StatusOK,
			expectedCT:     "application/octet-stream",
		},
		{
			name:           "empty content type defaults to octet-stream",
			contentType:    "",
			data:           []byte("data"),
			expectedStatus: http.StatusOK,
			expectedCT:     "application/octet-stream",
		},
		{
			name:           "empty data",
			contentType:    "text/plain",
			data:           []byte{},
			expectedStatus: 204,
			expectedCT:     "text/plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			c := NewContext(w, req)

			c.Data(tt.expectedStatus, tt.contentType, tt.data)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedCT, w.Header().Get("Content-Type"))
			// Compare lengths and contents for empty vs nil slice compatibility
			if len(tt.data) == 0 && w.Body.Len() == 0 {
				// Both are empty, which is acceptable
			} else {
				assert.Equal(t, tt.data, w.Body.Bytes())
			}
		})
	}
}

// TestJSON_Variants_ContentType verifies all methods set correct content type
func TestJSON_Variants_ContentType(t *testing.T) {
	t.Parallel()

	data := map[string]string{"key": "value"}

	tests := []struct {
		name       string
		renderFunc func(*Context) error
		expectedCT string
	}{
		{
			name: "JSON",
			renderFunc: func(c *Context) error {
				c.JSON(http.StatusOK, data)
				return nil
			},
			expectedCT: "application/json; charset=utf-8",
		},
		{
			name: "IndentedJSON",
			renderFunc: func(c *Context) error {
				c.IndentedJSON(200, data)
				return nil
			},
			expectedCT: "application/json; charset=utf-8",
		},
		{
			name: "PureJSON",
			renderFunc: func(c *Context) error {
				c.PureJSON(200, data)
				return nil
			},
			expectedCT: "application/json; charset=utf-8",
		},
		{
			name: "SecureJSON",
			renderFunc: func(c *Context) error {
				c.SecureJSON(200, data)
				return nil
			},
			expectedCT: "application/json; charset=utf-8",
		},
		{
			name: "ASCIIJSON",
			renderFunc: func(c *Context) error {
				c.ASCIIJSON(200, data)
				return nil
			},
			expectedCT: "application/json; charset=utf-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			c := NewContext(w, req)

			_ = tt.renderFunc(c)

			assert.Equal(t, tt.expectedCT, w.Header().Get("Content-Type"))
		})
	}
}

// TestJSON_Variants_ErrorHandling tests error cases
func TestJSON_Variants_ErrorHandling(t *testing.T) {
	t.Parallel()

	// Type that fails JSON marshaling
	type badType struct {
		Channel chan int // Cannot be marshaled to JSON
	}

	bad := badType{Channel: make(chan int)}

	tests := []struct {
		name       string
		renderFunc func(*Context) error
	}{
		{
			name: "JSON encoding error",
			renderFunc: func(c *Context) error {
				return c.JSON(http.StatusOK, bad)
			},
		},
		{
			name: "IndentedJSON encoding error",
			renderFunc: func(c *Context) error {
				return c.IndentedJSON(200, bad)
			},
		},
		{
			name: "PureJSON encoding error",
			renderFunc: func(c *Context) error {
				return c.PureJSON(200, bad)
			},
		},
		{
			name: "SecureJSON encoding error",
			renderFunc: func(c *Context) error {
				return c.SecureJSON(200, bad)
			},
		},
		{
			name: "ASCIIJSON encoding error",
			renderFunc: func(c *Context) error {
				return c.ASCIIJSON(200, bad)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			c := NewContext(w, req)

			err := tt.renderFunc(c)
			assert.Error(t, err, "Expected encoding error but got none")
			assert.Contains(t, err.Error(), "encoding failed", "Expected encoding error message")
		})
	}
}

// TestYAML_Error tests YAML encoding error handling
func TestYAML_Error(t *testing.T) {
	t.Parallel()

	// YAML library panics for unencodable types, so we test with a recoverable panic
	defer func() {
		if r := recover(); r != nil {
			// Expected panic for function types
			t.Log("YAML correctly panics for unencodable types")
		}
	}()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	// Type that fails YAML marshaling
	type badType struct {
		Func func() // Functions cannot be marshaled
	}

	// This will panic - which is expected behavior for yaml.v3
	c.YAML(200, badType{Func: func() {}})
}

// TestDataFromReader_NilReader tests nil reader handling
func TestDataFromReader_NilReader(t *testing.T) {
	t.Parallel()

	// io.Copy will panic with nil reader, so we expect a panic
	defer func() {
		if r := recover(); r != nil {
			// Expected panic for nil reader
			t.Log("DataFromReader correctly panics for nil reader (io.Copy behavior)")
		}
	}()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	// This will panic - which is expected io.Copy behavior
	_ = c.DataFromReader(200, 0, "text/plain", nil, nil)
}

// TestData_EmptyData tests empty data handling
func TestData_EmptyData(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	c.Data(204, "text/plain", []byte{})

	assert.Equal(t, 204, w.Code)
	assert.Equal(t, 0, w.Body.Len(), "Expected empty body")
}

// TestData_LargeData tests handling of large byte slices
func TestData_LargeData(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	// Create 1MB of data
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	c.Data(200, "application/octet-stream", largeData)

	assert.Equal(t, len(largeData), w.Body.Len(), "Expected %d bytes", len(largeData))
	assert.Equal(t, largeData, w.Body.Bytes(), "Data mismatch")
}

// TestSecureJSON_StripPrefix tests that clients can strip the prefix
func TestSecureJSON_StripPrefix(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	originalData := map[string]string{
		"secret": "value",
		"token":  "abc123",
	}

	c.SecureJSON(200, originalData)

	body := w.Body.String()

	// Simulate client stripping prefix
	prefix := "while(1);"
	require.True(t, strings.HasPrefix(body, prefix), "Expected prefix %q", prefix)

	jsonPart := strings.TrimPrefix(body, prefix)

	// Should be valid JSON after stripping
	var decoded map[string]string
	err := json.Unmarshal([]byte(jsonPart), &decoded)
	require.NoError(t, err, "Failed to unmarshal after stripping prefix")

	// Should match original data
	assert.Equal(t, originalData["secret"], decoded["secret"], "Data mismatch after stripping prefix")
}
