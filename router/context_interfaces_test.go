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
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockParameterReader is a mock implementation of ParameterReader for testing.
type mockParameterReader struct {
	params        map[string]string
	queries       map[string]string
	formValues    map[string]string
	cookies       map[string]string
	queryDefaults map[string]string
	formDefaults  map[string]string
}

func (m *mockParameterReader) Param(key string) string {
	return m.params[key]
}

func (m *mockParameterReader) Query(key string) string {
	return m.queries[key]
}

func (m *mockParameterReader) QueryDefault(key, defaultValue string) string {
	if val, ok := m.queryDefaults[key]; ok {
		return val
	}
	return defaultValue
}

func (m *mockParameterReader) FormValue(key string) string {
	return m.formValues[key]
}

func (m *mockParameterReader) FormValueDefault(key, defaultValue string) string {
	if val, ok := m.formDefaults[key]; ok {
		return val
	}
	return defaultValue
}

func (m *mockParameterReader) AllParams() map[string]string {
	result := make(map[string]string, len(m.params))
	maps.Copy(result, m.params)
	return result
}

func (m *mockParameterReader) AllQueries() map[string]string {
	result := make(map[string]string, len(m.queries))
	maps.Copy(result, m.queries)
	return result
}

func (m *mockParameterReader) GetCookie(name string) (string, error) {
	if val, ok := m.cookies[name]; ok {
		return val, nil
	}
	return "", ErrCookieNotFound
}

// mockResponseWriter is a mock implementation of ResponseWriter for testing.
type mockResponseWriter struct {
	statusCode int
	headers    map[string]string
	body       []byte
	cookies    []http.Cookie
}

func (m *mockResponseWriter) JSON(code int, _ any) {
	m.statusCode = code
	m.headers["Content-Type"] = "application/json; charset=utf-8"
	// In real implementation, would marshal obj to JSON
	m.body = []byte(`{"test":"data"}`)
}

func (m *mockResponseWriter) IndentedJSON(code int, obj any) {
	m.JSON(code, obj)
}

func (m *mockResponseWriter) PureJSON(code int, obj any) {
	m.JSON(code, obj)
}

func (m *mockResponseWriter) SecureJSON(code int, obj any, _ ...string) {
	m.JSON(code, obj)
}

func (m *mockResponseWriter) ASCIIJSON(code int, obj any) {
	m.JSON(code, obj)
}

func (m *mockResponseWriter) String(code int, value string) {
	m.statusCode = code
	m.headers["Content-Type"] = "text/plain"
	m.body = []byte(value)
}

func (m *mockResponseWriter) Stringf(code int, format string, _ ...any) {
	m.statusCode = code
	m.headers["Content-Type"] = "text/plain"
	m.body = []byte(format)
}

func (m *mockResponseWriter) HTML(code int, html string) {
	m.statusCode = code
	m.headers["Content-Type"] = "text/html"
	m.body = []byte(html)
}

func (m *mockResponseWriter) YAML(code int, _ any) {
	m.statusCode = code
	m.headers["Content-Type"] = "application/x-yaml"
}

func (m *mockResponseWriter) Data(code int, contentType string, data []byte) {
	m.statusCode = code
	m.headers["Content-Type"] = contentType
	m.body = data
}

// WriteXxx methods for mock (not used in tests but required by interface)
func (m *mockResponseWriter) WriteJSON(code int, _ any) error {
	m.JSON(code, nil)
	return nil
}

func (m *mockResponseWriter) WriteIndentedJSON(code int, obj any) error {
	m.IndentedJSON(code, obj)
	return nil
}

func (m *mockResponseWriter) WritePureJSON(code int, obj any) error {
	m.PureJSON(code, obj)
	return nil
}

func (m *mockResponseWriter) WriteSecureJSON(code int, obj any, prefix ...string) error {
	m.SecureJSON(code, obj, prefix...)
	return nil
}

func (m *mockResponseWriter) WriteASCIIJSON(code int, obj any) error {
	m.ASCIIJSON(code, obj)
	return nil
}

func (m *mockResponseWriter) WriteString(code int, value string) error {
	m.String(code, value)
	return nil
}

func (m *mockResponseWriter) WriteStringf(code int, format string, values ...any) error {
	m.Stringf(code, format, values...)
	return nil
}

func (m *mockResponseWriter) WriteHTML(code int, html string) error {
	m.HTML(code, html)
	return nil
}

func (m *mockResponseWriter) WriteYAML(code int, obj any) error {
	m.YAML(code, obj)
	return nil
}

func (m *mockResponseWriter) WriteData(code int, contentType string, data []byte) error {
	m.Data(code, contentType, data)
	return nil
}

func (m *mockResponseWriter) Status(code int) {
	m.statusCode = code
}

func (m *mockResponseWriter) Header(key, value string) {
	m.headers[key] = value
}

func (m *mockResponseWriter) Redirect(code int, location string) {
	m.statusCode = code
	m.headers["Location"] = location
}

func (m *mockResponseWriter) NoContent() {
	m.statusCode = http.StatusNoContent
}

func (m *mockResponseWriter) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	m.cookies = append(m.cookies, http.Cookie{
		Name:     name,
		Value:    value,
		MaxAge:   maxAge,
		Path:     path,
		Domain:   domain,
		Secure:   secure,
		HttpOnly: httpOnly,
	})
}

// Example business logic function that uses ParameterReader interface.
// This demonstrates how interfaces enable easier testing.
func processUserRequest(reader ParameterReader) (string, error) {
	userID := reader.Param("id")
	if userID == "" {
		return "", ErrUserIDRequired
	}

	page := reader.QueryDefault("page", "1")
	if page == "" {
		return "", ErrPageParameterInvalid
	}

	return userID + ":" + page, nil
}

// Example business logic function that uses ResponseWriter interface.
// This demonstrates how interfaces enable easier testing.
func sendUserResponse(writer ResponseWriter, userID string) {
	writer.JSON(http.StatusOK, map[string]string{
		"user_id": userID,
		"status":  "active",
	})
}

// TestParameterReaderInterface demonstrates testing with ParameterReader interface.
func TestParameterReaderInterface(t *testing.T) {
	t.Run("with real Context", func(t *testing.T) {
		r := MustNew()
		r.GET("/users/:id", func(c *Context) {
			result, err := processUserRequest(c)
			require.NoError(t, err)
			assert.Equal(t, "123:1", result)
		})

		req := httptest.NewRequest("GET", "/users/123?page=1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("with mock ParameterReader", func(t *testing.T) {
		mock := &mockParameterReader{
			params: map[string]string{
				"id": "456",
			},
			queryDefaults: map[string]string{
				"page": "2",
			},
		}

		result, err := processUserRequest(mock)
		require.NoError(t, err)
		assert.Equal(t, "456:2", result)
	})

	t.Run("missing user ID", func(t *testing.T) {
		mock := &mockParameterReader{
			params: map[string]string{},
		}

		_, err := processUserRequest(mock)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user ID is required")
	})
}

// TestResponseWriterInterface demonstrates testing with ResponseWriter interface.
func TestResponseWriterInterface(t *testing.T) {
	t.Run("with real Context", func(t *testing.T) {
		r := MustNew()
		r.GET("/users/:id", func(c *Context) {
			sendUserResponse(c, "789")
		})

		req := httptest.NewRequest("GET", "/users/789", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "user_id")
		assert.Contains(t, w.Body.String(), "789")
	})

	t.Run("with mock ResponseWriter", func(t *testing.T) {
		mock := &mockResponseWriter{
			headers: make(map[string]string),
		}

		sendUserResponse(mock, "999")
		assert.Equal(t, http.StatusOK, mock.statusCode)
		assert.Equal(t, "application/json; charset=utf-8", mock.headers["Content-Type"])
	})
}

// TestContextImplementsInterfaces verifies that Context implements all interfaces.
func TestContextImplementsInterfaces(t *testing.T) {
	r := MustNew()
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Use router to create properly initialized context
	r.GET("/test", func(c *Context) {
		// Test ParameterReader interface
		var reader ParameterReader = c
		assert.NotNil(t, reader)
		assert.Equal(t, "", reader.Param("nonexistent"))
		assert.Equal(t, "", reader.Query("nonexistent"))

		// Test ResponseWriter interface
		var writer ResponseWriter = c
		assert.NotNil(t, writer)
		writer.String(http.StatusOK, "test")

		// Test ContextReader interface
		var contextReader ContextReader = c
		assert.NotNil(t, contextReader)
		// Version may be empty if versioning is not configured
		_ = contextReader.Version()

		// Test ContextWriter interface
		var contextWriter ContextWriter = c
		assert.NotNil(t, contextWriter)
		// ContextWriter now only extends ResponseWriter - error formatting is handled by app.Context
	})

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// Example helper functions that demonstrate interface composition.
// These functions show how interfaces enable clear separation of concerns.

// readParamsOnly demonstrates a function that only needs to read parameters.
func readParamsOnly(reader ParameterReader) {
	userID := reader.Param("id")
	page := reader.Query("page")
	_ = userID
	_ = page
}

// writeResponseOnly demonstrates a function that only needs to write responses.
func writeResponseOnly(writer ResponseWriter) {
	writer.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// processRequestBoth demonstrates a function that needs both reading and writing.
func processRequestBoth(reader ParameterReader, writer ResponseWriter) {
	userID := reader.Param("id")
	writer.JSON(http.StatusOK, map[string]string{"user_id": userID})
}

// TestComposition demonstrates how interfaces enable composition.
func TestComposition(t *testing.T) {
	// All functions can work with Context
	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	// Or with mocks for testing
	mockReader := &mockParameterReader{
		params: map[string]string{"id": "456"},
	}
	mockWriter := &mockResponseWriter{
		headers: make(map[string]string),
	}

	// Test with real Context
	readParamsOnly(c)
	writeResponseOnly(c)
	processRequestBoth(c, c)

	// Test with mocks
	readParamsOnly(mockReader)
	writeResponseOnly(mockWriter)
	processRequestBoth(mockReader, mockWriter)
}
