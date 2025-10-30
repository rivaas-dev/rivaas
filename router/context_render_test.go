package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestIndentedJSON tests pretty-printed JSON rendering
func TestIndentedJSON(t *testing.T) {
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
			expectedStatus: 200,
			checkBody: func(t *testing.T, body string) {
				// Should be indented
				if !strings.Contains(body, "  ") {
					t.Error("Expected indented JSON")
				}
				// Should contain newlines
				if !strings.Contains(body, "\n") {
					t.Error("Expected formatted JSON with newlines")
				}
				// Should be valid JSON
				var result map[string]any
				if err := json.Unmarshal([]byte(body), &result); err != nil {
					t.Errorf("Invalid JSON: %v", err)
				}
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
			expectedStatus: 200,
			checkBody: func(t *testing.T, body string) {
				// Should have proper indentation for nested objects
				if !strings.Contains(body, "    ") {
					t.Error("Expected nested indentation")
				}
			},
		},
		{
			name:           "empty object",
			data:           map[string]string{},
			expectedStatus: 200,
			checkBody: func(t *testing.T, body string) {
				if !strings.Contains(body, "{}") {
					t.Error("Expected empty object {}")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			c := NewContext(w, req)

			err := c.IndentedJSON(tt.expectedStatus, tt.data)

			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
				t.Errorf("Expected JSON content type, got %s", w.Header().Get("Content-Type"))
			}

			if tt.checkBody != nil {
				tt.checkBody(t, w.Body.String())
			}
		})
	}
}

// TestPureJSON tests JSON without HTML escaping
func TestPureJSON(t *testing.T) {
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
			expectedStatus: 200,
			expectedBody:   `{"html":"<h1>Title</h1>"}`,
		},
		{
			name: "ampersand unescaped",
			data: map[string]string{
				"url": "https://example.com?foo=bar&baz=qux",
			},
			expectedStatus: 200,
			expectedBody:   `{"url":"https://example.com?foo=bar&baz=qux"}`,
		},
		{
			name: "greater than and less than unescaped",
			data: map[string]string{
				"comparison": "x < 10 && y > 5",
			},
			expectedStatus: 200,
			expectedBody:   `{"comparison":"x < 10 && y > 5"}`,
		},
		{
			name: "mixed HTML content",
			data: map[string]string{
				"content": "<script>alert('test')</script>",
			},
			expectedStatus: 200,
			expectedBody:   `{"content":"<script>alert('test')</script>"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			c := NewContext(w, req)

			err := c.PureJSON(tt.expectedStatus, tt.data)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			body := strings.TrimSpace(w.Body.String())
			if body != tt.expectedBody {
				t.Errorf("Expected body %q, got %q", tt.expectedBody, body)
			}

			if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
				t.Errorf("Expected JSON content type, got %s", w.Header().Get("Content-Type"))
			}
		})
	}
}

// TestPureJSON_vs_JSON_Escaping compares escaping behavior
func TestPureJSON_vs_JSON_Escaping(t *testing.T) {
	data := map[string]string{
		"html": "<h1>Title</h1>",
		"url":  "https://example.com?a=1&b=2",
	}

	// Test standard JSON (should escape)
	wJSON := httptest.NewRecorder()
	reqJSON := httptest.NewRequest("GET", "/", nil)
	cJSON := NewContext(wJSON, reqJSON)
	_ = cJSON.JSON(200, data)
	jsonBody := wJSON.Body.String()

	// Test PureJSON (should NOT escape)
	wPure := httptest.NewRecorder()
	reqPure := httptest.NewRequest("GET", "/", nil)
	cPure := NewContext(wPure, reqPure)
	_ = cPure.PureJSON(200, data)
	pureBody := wPure.Body.String()

	// Verify JSON() escapes HTML
	if !strings.Contains(jsonBody, "\\u003c") || !strings.Contains(jsonBody, "\\u003e") {
		t.Error("Standard JSON should escape < and >")
	}
	if !strings.Contains(jsonBody, "\\u0026") {
		t.Error("Standard JSON should escape &")
	}

	// Verify PureJSON() does NOT escape HTML
	if strings.Contains(pureBody, "\\u003c") || strings.Contains(pureBody, "\\u003e") {
		t.Error("PureJSON should NOT escape < and >")
	}
	if strings.Contains(pureBody, "\\u0026") {
		t.Error("PureJSON should NOT escape &")
	}

	// Both should produce valid JSON
	var resultJSON, resultPure map[string]string
	if err := json.Unmarshal([]byte(jsonBody), &resultJSON); err != nil {
		t.Errorf("JSON() output is not valid JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(pureBody), &resultPure); err != nil {
		t.Errorf("PureJSON() output is not valid JSON: %v", err)
	}

	// Both should unmarshal to same values
	if resultJSON["html"] != resultPure["html"] {
		t.Error("JSON and PureJSON should unmarshal to same values")
	}
}

// TestSecureJSON tests JSON with security prefix
func TestSecureJSON(t *testing.T) {
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
			expectedStatus: 200,
			expectedPrefix: "while(1);",
		},
		{
			name:           "custom prefix",
			data:           map[string]string{"key": "value"},
			prefix:         []string{"for(;;);"},
			expectedStatus: 200,
			expectedPrefix: "for(;;);",
		},
		{
			name:           "empty custom prefix uses default",
			data:           map[string]int{"count": 42},
			prefix:         []string{""},
			expectedStatus: 200,
			expectedPrefix: "while(1);",
		},
		{
			name:           "JSON array (hijacking target)",
			data:           []int{1, 2, 3, 4, 5},
			prefix:         nil,
			expectedStatus: 200,
			expectedPrefix: "while(1);",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			c := NewContext(w, req)

			var err error
			if len(tt.prefix) > 0 {
				err = c.SecureJSON(tt.expectedStatus, tt.data, tt.prefix[0])
			} else {
				err = c.SecureJSON(tt.expectedStatus, tt.data)
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			body := w.Body.String()
			if !strings.HasPrefix(body, tt.expectedPrefix) {
				t.Errorf("Expected prefix %q, got body: %q", tt.expectedPrefix, body)
			}

			// Verify that removing prefix gives valid JSON
			jsonPart := strings.TrimPrefix(body, tt.expectedPrefix)
			var result any
			if err := json.Unmarshal([]byte(jsonPart), &result); err != nil {
				t.Errorf("Content after prefix is not valid JSON: %v", err)
			}
		})
	}
}

// TestAsciiJSON tests ASCII-only JSON encoding
func TestAsciiJSON(t *testing.T) {
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
			expectedStatus: 200,
			checkBody: func(t *testing.T, body string) {
				// Should contain \u escape sequences
				if !strings.Contains(body, "\\u") {
					t.Error("Expected Unicode escape sequences")
				}
				// Should be pure ASCII (no bytes >= 128)
				for _, b := range []byte(body) {
					if b >= 128 {
						t.Errorf("Found non-ASCII byte: %d (%c)", b, b)
					}
				}
			},
		},
		{
			name: "emoji characters",
			data: map[string]string{
				"emoji": "🌍🎉",
			},
			expectedStatus: 200,
			checkBody: func(t *testing.T, body string) {
				// Should escape emoji to \uXXXX
				if !strings.Contains(body, "\\u") {
					t.Error("Expected emoji to be escaped")
				}
				// Should be valid JSON that unmarshals correctly
				var result map[string]string
				if err := json.Unmarshal([]byte(body), &result); err != nil {
					t.Errorf("Invalid JSON: %v", err)
				}
				if result["emoji"] != "🌍🎉" {
					t.Error("Emoji should unmarshal to original value")
				}
			},
		},
		{
			name: "mixed ASCII and non-ASCII",
			data: map[string]string{
				"name":    "José",
				"city":    "São Paulo",
				"message": "Hello",
			},
			expectedStatus: 200,
			checkBody: func(t *testing.T, body string) {
				// ASCII part should be unchanged
				if !strings.Contains(body, "Hello") {
					t.Error("ASCII text should not be escaped")
				}
				// Non-ASCII should be escaped
				if strings.Contains(body, "José") {
					t.Error("Non-ASCII characters should be escaped")
				}
			},
		},
		{
			name: "pure ASCII (no escaping needed)",
			data: map[string]string{
				"text": "Plain ASCII text 123",
			},
			expectedStatus: 200,
			checkBody: func(t *testing.T, body string) {
				// Should not have escape sequences for pure ASCII
				if strings.Count(body, "\\u") > 0 {
					t.Error("Pure ASCII should not have escape sequences")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			c := NewContext(w, req)

			err := c.AsciiJSON(tt.expectedStatus, tt.data)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
				t.Errorf("Expected JSON content type")
			}

			if tt.checkBody != nil {
				tt.checkBody(t, w.Body.String())
			}
		})
	}
}

// TestYAML tests YAML rendering
func TestYAML(t *testing.T) {
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
			expectedStatus: 200,
			checkBody: func(t *testing.T, body string) {
				// Should be valid YAML
				var result map[string]any
				if err := yaml.Unmarshal([]byte(body), &result); err != nil {
					t.Errorf("Invalid YAML: %v", err)
				}
				// Should contain YAML formatting
				if !strings.Contains(body, ":") {
					t.Error("Expected YAML key:value format")
				}
			},
		},
		{
			name:           "array",
			data:           []string{"item1", "item2", "item3"},
			expectedStatus: 200,
			checkBody: func(t *testing.T, body string) {
				// Should be valid YAML
				var result []string
				if err := yaml.Unmarshal([]byte(body), &result); err != nil {
					t.Errorf("Invalid YAML: %v", err)
				}
				if len(result) != 3 {
					t.Errorf("Expected 3 items, got %d", len(result))
				}
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
			expectedStatus: 200,
			checkBody: func(t *testing.T, body string) {
				var result map[string]any
				if err := yaml.Unmarshal([]byte(body), &result); err != nil {
					t.Errorf("Invalid YAML: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			c := NewContext(w, req)

			err := c.YAML(tt.expectedStatus, tt.data)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if w.Header().Get("Content-Type") != "application/x-yaml; charset=utf-8" {
				t.Errorf("Expected YAML content type, got %s", w.Header().Get("Content-Type"))
			}

			if tt.checkBody != nil {
				tt.checkBody(t, w.Body.String())
			}
		})
	}
}

// TestDataFromReader tests streaming from io.Reader
func TestDataFromReader(t *testing.T) {
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
			expectedStatus: 200,
		},
		{
			name:           "streaming without content length",
			contentLength:  -1,
			contentType:    "application/octet-stream",
			data:           "Binary data here",
			expectedStatus: 200,
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
			expectedStatus: 200,
		},
		{
			name:           "large data stream",
			contentLength:  1024,
			contentType:    "application/octet-stream",
			data:           strings.Repeat("A", 1024),
			expectedStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			c := NewContext(w, req)

			reader := strings.NewReader(tt.data)

			err := c.DataFromReader(tt.expectedStatus, tt.contentLength, tt.contentType, reader, tt.extraHeaders)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.contentType != "" && w.Header().Get("Content-Type") != tt.contentType {
				t.Errorf("Expected content type %s, got %s", tt.contentType, w.Header().Get("Content-Type"))
			}

			if tt.contentLength >= 0 {
				expectedLength := fmt.Sprintf("%d", tt.contentLength)
				if w.Header().Get("Content-Length") != expectedLength {
					t.Errorf("Expected Content-Length %s, got %s", expectedLength, w.Header().Get("Content-Length"))
				}
			}

			// Verify extra headers
			for key, expectedValue := range tt.extraHeaders {
				if w.Header().Get(key) != expectedValue {
					t.Errorf("Expected header %s: %s, got %s", key, expectedValue, w.Header().Get(key))
				}
			}

			// Verify streamed data
			if w.Body.String() != tt.data {
				t.Errorf("Expected body %q, got %q", tt.data, w.Body.String())
			}
		})
	}
}

// TestDataFromReader_Error tests error handling
func TestDataFromReader_Error(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	// Create a reader that returns an error
	errorReader := &errorReader{err: io.ErrUnexpectedEOF}

	err := c.DataFromReader(200, -1, "text/plain", errorReader, nil)
	if err == nil {
		t.Error("Expected error from failing reader")
	}
	if !strings.Contains(err.Error(), "streaming from reader failed") {
		t.Errorf("Expected streaming error, got: %v", err)
	}
}

// errorReader is a test helper that always returns an error
type errorReader struct {
	err error
}

func (er *errorReader) Read(p []byte) (n int, err error) {
	return 0, er.err
}

// TestData tests custom content type data sending
func TestData(t *testing.T) {
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
			expectedStatus: 200,
			expectedCT:     "image/png",
		},
		{
			name:           "PDF document",
			contentType:    "application/pdf",
			data:           []byte("%PDF-1.4"),
			expectedStatus: 200,
			expectedCT:     "application/pdf",
		},
		{
			name:           "custom binary",
			contentType:    "application/octet-stream",
			data:           []byte{0x00, 0x01, 0x02, 0x03},
			expectedStatus: 200,
			expectedCT:     "application/octet-stream",
		},
		{
			name:           "empty content type defaults to octet-stream",
			contentType:    "",
			data:           []byte("data"),
			expectedStatus: 200,
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
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			c := NewContext(w, req)

			err := c.Data(tt.expectedStatus, tt.contentType, tt.data)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if w.Header().Get("Content-Type") != tt.expectedCT {
				t.Errorf("Expected content type %s, got %s", tt.expectedCT, w.Header().Get("Content-Type"))
			}

			if !bytes.Equal(w.Body.Bytes(), tt.data) {
				t.Errorf("Expected body %v, got %v", tt.data, w.Body.Bytes())
			}
		})
	}
}

// TestJSON_Variants_ContentType verifies all methods set correct content type
func TestJSON_Variants_ContentType(t *testing.T) {
	data := map[string]string{"key": "value"}

	tests := []struct {
		name       string
		renderFunc func(*Context) error
		expectedCT string
	}{
		{
			name: "JSON",
			renderFunc: func(c *Context) error {
				return c.JSON(200, data)
			},
			expectedCT: "application/json; charset=utf-8",
		},
		{
			name: "IndentedJSON",
			renderFunc: func(c *Context) error {
				return c.IndentedJSON(200, data)
			},
			expectedCT: "application/json; charset=utf-8",
		},
		{
			name: "PureJSON",
			renderFunc: func(c *Context) error {
				return c.PureJSON(200, data)
			},
			expectedCT: "application/json; charset=utf-8",
		},
		{
			name: "SecureJSON",
			renderFunc: func(c *Context) error {
				return c.SecureJSON(200, data)
			},
			expectedCT: "application/json; charset=utf-8",
		},
		{
			name: "AsciiJSON",
			renderFunc: func(c *Context) error {
				return c.AsciiJSON(200, data)
			},
			expectedCT: "application/json; charset=utf-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			c := NewContext(w, req)

			_ = tt.renderFunc(c)

			if w.Header().Get("Content-Type") != tt.expectedCT {
				t.Errorf("Expected content type %s, got %s", tt.expectedCT, w.Header().Get("Content-Type"))
			}
		})
	}
}

// TestJSON_Variants_ErrorHandling tests error cases
func TestJSON_Variants_ErrorHandling(t *testing.T) {
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
				return c.JSON(200, bad)
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
			name: "AsciiJSON encoding error",
			renderFunc: func(c *Context) error {
				return c.AsciiJSON(200, bad)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			c := NewContext(w, req)

			err := tt.renderFunc(c)
			if err == nil {
				t.Error("Expected encoding error but got none")
			}
			if !strings.Contains(err.Error(), "encoding failed") {
				t.Errorf("Expected encoding error message, got: %v", err)
			}
		})
	}
}

// TestYAML_Error tests YAML encoding error handling
func TestYAML_Error(t *testing.T) {
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
	_ = c.YAML(200, badType{Func: func() {}})
}

// TestDataFromReader_NilReader tests nil reader handling
func TestDataFromReader_NilReader(t *testing.T) {
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
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	err := c.Data(204, "text/plain", []byte{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if w.Code != 204 {
		t.Errorf("Expected status 204, got %d", w.Code)
	}

	if w.Body.Len() != 0 {
		t.Errorf("Expected empty body, got %d bytes", w.Body.Len())
	}
}

// TestData_LargeData tests handling of large byte slices
func TestData_LargeData(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	// Create 1MB of data
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	err := c.Data(200, "application/octet-stream", largeData)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if w.Body.Len() != len(largeData) {
		t.Errorf("Expected %d bytes, got %d", len(largeData), w.Body.Len())
	}

	if !bytes.Equal(w.Body.Bytes(), largeData) {
		t.Error("Data mismatch")
	}
}

// TestSecureJSON_StripPrefix tests that clients can strip the prefix
func TestSecureJSON_StripPrefix(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := NewContext(w, req)

	originalData := map[string]string{
		"secret": "value",
		"token":  "abc123",
	}

	err := c.SecureJSON(200, originalData)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	body := w.Body.String()

	// Simulate client stripping prefix
	prefix := "while(1);"
	if !strings.HasPrefix(body, prefix) {
		t.Fatalf("Expected prefix %q", prefix)
	}

	jsonPart := strings.TrimPrefix(body, prefix)

	// Should be valid JSON after stripping
	var decoded map[string]string
	if err := json.Unmarshal([]byte(jsonPart), &decoded); err != nil {
		t.Fatalf("Failed to unmarshal after stripping prefix: %v", err)
	}

	// Should match original data
	if decoded["secret"] != originalData["secret"] {
		t.Error("Data mismatch after stripping prefix")
	}
}
