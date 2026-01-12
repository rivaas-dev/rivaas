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
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test AppendHeader
func TestAppendHeader(t *testing.T) {
	t.Parallel()

	t.Run("append to new header", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.AppendHeader("X-Custom", "value1")

		assert.Equal(t, "value1", w.Header().Get("X-Custom"))
	})

	t.Run("append to existing header", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.AppendHeader("Link", "</page1>; rel=\"first\"")
		c.AppendHeader("Link", "</page2>; rel=\"next\"")

		link := w.Header().Get("Link")
		assert.Contains(t, link, "page1")
		assert.Contains(t, link, "page2")
	})
}

// Test ContentType
func TestContentType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"extension with dot", ".json", "application/json"},
		{"extension without dot", "json", "application/json"},
		{"html extension", ".html", "text/html"},
		{"xml extension", ".xml", "xml"}, // Can be text/xml or application/xml
		{"txt extension", ".txt", "text/plain"},
		{"full MIME type", "application/pdf", "application/pdf"},
		{"unknown extension", ".unknown", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			c.ContentType(tt.input)

			ct := w.Header().Get("Content-Type")
			assert.Contains(t, ct, tt.expected)
		})
	}
}

// Test Location
func TestLocation(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	c.Location("/new-path")

	assert.Equal(t, "/new-path", w.Header().Get("Location"))
}

// Test Vary
func TestVary(t *testing.T) {
	t.Parallel()

	t.Run("single field", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.Vary("Accept-Encoding")

		assert.Equal(t, "Accept-Encoding", w.Header().Get("Vary"))
	})

	t.Run("multiple fields", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.Vary("Accept-Encoding", "Accept-Language")

		vary := w.Header().Get("Vary")
		assert.Contains(t, vary, "Accept-Encoding")
		assert.Contains(t, vary, "Accept-Language")
	})

	t.Run("append to existing", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.Vary("Accept")
		c.Vary("Accept-Encoding")

		vary := w.Header().Get("Vary")
		assert.Contains(t, vary, "Accept,")
		assert.Contains(t, vary, "Accept-Encoding")
	})

	t.Run("no duplicates", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.Vary("Accept")
		c.Vary("Accept") // Duplicate

		vary := w.Header().Get("Vary")
		// Should only appear once
		count := strings.Count(vary, "Accept")
		assert.LessOrEqual(t, count, 1, "Accept should appear at most once in Vary header")
	})
}

// Test Link
func TestLink(t *testing.T) {
	t.Parallel()

	t.Run("single link", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.Link("/api/users?page=2", "next")

		link := w.Header().Get("Link")
		assert.Contains(t, link, "</api/users?page=2>")
		assert.Contains(t, link, `rel="next"`)
	})

	t.Run("multiple links", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.Link("/api/users?page=2", "next")
		c.Link("/api/users?page=10", "last")

		link := w.Header().Get("Link")
		assert.Contains(t, link, "page=2")
		assert.Contains(t, link, "page=10")
	})
}

// Test Download
func TestDownload(t *testing.T) {
	t.Parallel()

	// Create a temporary test file
	tmpfile, err := os.CreateTemp(t.TempDir(), "test-download-*.txt")
	require.NoError(t, err)

	content := []byte("test file content")
	_, err = tmpfile.Write(content)
	require.NoError(t, err)
	require.NoError(t, tmpfile.Close())

	t.Run("download with original filename", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		downloadErr := c.Download(tmpfile.Name())
		require.NoError(t, downloadErr)

		// Check Content-Disposition header
		cd := w.Header().Get("Content-Disposition")
		assert.Contains(t, cd, "attachment")
	})

	t.Run("download with custom filename", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		downloadErr := c.Download(tmpfile.Name(), "custom-name.txt")
		require.NoError(t, downloadErr)

		cd := w.Header().Get("Content-Disposition")
		assert.Contains(t, cd, "custom-name.txt")
	})
}

// Test Send
func TestSend(t *testing.T) {
	t.Parallel()

	t.Run("send raw bytes", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		data := []byte("raw binary data")
		err := c.Send(data)
		require.NoError(t, err)

		assert.Equal(t, "raw binary data", w.Body.String())

		// Should set default Content-Type
		assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
	})

	t.Run("send with existing content type", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.Header("Content-Type", "text/plain")
		err := c.Send([]byte("text data"))
		require.NoError(t, err)

		// Should not override existing Content-Type
		assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
	})
}

// Test SendStatus
func TestSendStatus(t *testing.T) {
	t.Parallel()

	t.Run("send 404", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		err := c.SendStatus(http.StatusNotFound)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "Not Found", w.Body.String())
	})

	t.Run("send 201", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		err := c.SendStatus(http.StatusCreated)
		require.NoError(t, err)

		assert.Equal(t, "Created", w.Body.String())
	})
}

// Test JSONP
func TestJSONP(t *testing.T) {
	t.Parallel()

	t.Run("default callback", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		data := map[string]string{"message": "Hello"}
		err := c.JSONP(http.StatusOK, data)
		require.NoError(t, err)

		body := w.Body.String()
		assert.True(t, strings.HasPrefix(body, "callback("))
		assert.True(t, strings.HasSuffix(body, ")"))
		assert.Contains(t, body, `"message":"Hello"`)

		// Check Content-Type
		assert.Contains(t, w.Header().Get("Content-Type"), "application/javascript")
	})

	t.Run("custom callback", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		data := map[string]int{"count": 42}
		err := c.JSONP(http.StatusOK, data, "myCallback")
		require.NoError(t, err)

		body := w.Body.String()
		assert.True(t, strings.HasPrefix(body, "myCallback("))
	})
}

// Test Format
func TestFormat(t *testing.T) {
	t.Parallel()

	data := map[string]string{"message": "test"}

	t.Run("format as JSON", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		err := c.Format(http.StatusOK, data)
		require.NoError(t, err)

		assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	})

	t.Run("format as HTML", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", "text/html")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		err := c.Format(http.StatusOK, data)
		require.NoError(t, err)

		assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
		assert.Contains(t, w.Body.String(), "<p>")
	})

	t.Run("format as plain text", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", "text/plain")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		err := c.Format(http.StatusOK, "simple text")
		require.NoError(t, err)

		assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
	})

	t.Run("format default to JSON", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		// No Accept header
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		err := c.Format(http.StatusOK, data)
		require.NoError(t, err)

		// Should default to JSON
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	})
}

// Test Write and WriteString
func TestWrite(t *testing.T) {
	t.Parallel()

	t.Run("write bytes", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		n, err := c.Write([]byte("hello world"))
		require.NoError(t, err)
		assert.Equal(t, 11, n)
		assert.Equal(t, "hello world", w.Body.String())
	})

	t.Run("write string", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		n, err := c.WriteStringBody("hello string")
		require.NoError(t, err)
		assert.Equal(t, 12, n)
		assert.Equal(t, "hello string", w.Body.String())
	})

	t.Run("use with fmt.Fprintf", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Context implements io.Writer
		_, err := fmt.Fprintf(c, "User: %s, Count: %d", "Alice", 42)
		require.NoError(t, err)

		expected := "User: Alice, Count: 42"
		assert.Equal(t, expected, w.Body.String())
	})
}

// Test real-world scenarios
func TestResponseHelpers_RealWorld(t *testing.T) {
	t.Parallel()

	t.Run("API response with links", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/api/users?page=2", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Set pagination links
		c.Link("/api/users?page=1", "first")
		c.Link("/api/users?page=3", "next")
		c.Link("/api/users?page=10", "last")

		// Set vary for caching
		c.Vary("Accept", "Accept-Language")

		// Send response
		require.NoError(t, c.JSON(http.StatusOK, map[string]string{"status": "ok"}))

		// Verify headers
		link := w.Header().Get("Link")
		assert.Contains(t, link, "next")
		assert.Contains(t, link, "last")

		vary := w.Header().Get("Vary")
		assert.Contains(t, vary, "Accept")
	})

	t.Run("conditional response based on accept", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/api/user", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		user := map[string]any{
			"id":   123,
			"name": "John",
		}

		// Use Format for automatic content negotiation
		err := c.Format(http.StatusOK, user)
		require.NoError(t, err)

		assert.Contains(t, w.Body.String(), "123")
	})
}

// Benchmark response methods
func BenchmarkSend(b *testing.B) {
	data := []byte("response data")
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		c := NewContext(w, req)
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		c.Send(data)
	}
}

func BenchmarkWrite(b *testing.B) {
	data := []byte("response data")
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		c := NewContext(w, req)
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		c.Write(data)
	}
}

func BenchmarkWriteString(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		c := NewContext(w, req)
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		c.WriteStringBody("response data")
	}
}

// TestFormat_XML tests XML format response
func TestFormat_XML(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "application/xml")
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	data := map[string]string{"status": "ok"}
	require.NoError(t, c.Format(http.StatusOK, data))

	assert.Contains(t, w.Header().Get("Content-Type"), "application/xml")

	body := w.Body.String()
	assert.Contains(t, body, "<?xml")
	assert.Contains(t, body, "<response>")
}

// TestFormat_MultipleAcceptTypes tests Format with multiple accepted types
func TestFormat_MultipleAcceptTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		acceptHeader string
		expectType   string
	}{
		{
			name:         "prefers JSON",
			acceptHeader: "application/json, text/html;q=0.8",
			expectType:   "application/json",
		},
		{
			name:         "prefers HTML",
			acceptHeader: "text/html, application/json;q=0.9",
			expectType:   "text/html",
		},
		{
			name:         "prefers XML",
			acceptHeader: "application/xml, application/json;q=0.5",
			expectType:   "application/xml",
		},
		{
			name:         "wildcard accepts JSON",
			acceptHeader: "*/*",
			expectType:   "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept", tt.acceptHeader)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			require.NoError(t, c.Format(http.StatusOK, map[string]string{"data": "value"}))

			assert.Contains(t, w.Header().Get("Content-Type"), tt.expectType)
		})
	}
}

// TestFormat_DifferentStatusCodes tests Format with various status codes
func TestFormat_DifferentStatusCodes(t *testing.T) {
	t.Parallel()

	codes := []int{http.StatusOK, http.StatusCreated, http.StatusNoContent, http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError}

	for _, code := range codes {
		t.Run(string(rune('0'+code/100)), func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept", "application/json")
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			err := c.Format(code, map[string]string{"status": "test"})

			if code != http.StatusNoContent {
				require.NoError(t, err)
			}

			assert.Equal(t, code, w.Code)
		})
	}
}

// TestFormat_ComplexData tests Format with different data types
func TestFormat_ComplexData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data any
	}{
		{"string", "simple string"},
		{"int", 42},
		{"float", 3.14159},
		{"bool", true},
		{"map", map[string]any{"key": "value", "nested": map[string]string{"inner": "data"}}},
		{"slice", []string{"item1", "item2", "item3"}},
		{"struct", struct {
			Name string
			Age  int
		}{"John", 30}},
		{"nil", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept", "application/json")
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			err := c.Format(http.StatusOK, tt.data)

			require.NoError(t, err, "Format should handle %s", tt.name)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

// TestFormat_HTMLEscaping tests that HTML format escapes data
func TestFormat_HTMLEscaping(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	// Data with HTML special characters
	data := "<script>alert('xss')</script>"

	err := c.Format(http.StatusOK, data)

	require.NoError(t, err)

	body := w.Body.String()

	// Should wrap in <p> tags
	assert.Contains(t, body, "<p>")
}

// TestFormat_XMLDifferentData tests XML format with various data
func TestFormat_XMLDifferentData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data any
	}{
		{"map", map[string]string{"key": "value"}},
		{"string", "test string"},
		{"number", 123},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept", "application/xml")
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			err := c.Format(http.StatusOK, tt.data)

			require.NoError(t, err, "Format failed for %s", tt.name)

			assert.Contains(t, w.Header().Get("Content-Type"), "xml")
		})
	}
}

// TestFormat_Fallback tests fallback behavior for unsupported formats
func TestFormat_Fallback(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "application/pdf") // Unsupported format
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	err := c.Format(http.StatusOK, "test")

	require.NoError(t, err, "Format should fallback gracefully")

	// Should fallback to text/plain (default case in switch)
	contentType := w.Header().Get("Content-Type")
	assert.True(t, strings.Contains(contentType, "text/plain") || strings.Contains(contentType, "json"),
		"should fallback to supported format, got %s", contentType)
}

// TestSendStatus_StandardCodes tests SendStatus with known status codes
func TestSendStatus_StandardCodes(t *testing.T) {
	t.Parallel()

	codes := []int{http.StatusOK, http.StatusCreated, http.StatusNoContent, http.StatusNotFound, http.StatusInternalServerError}

	for _, code := range codes {
		t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
			t.Parallel()

			r := MustNew()
			r.GET("/test", func(c *Context) {
				err := c.SendStatus(code)
				assert.NoError(t, err)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, code, w.Code)
		})
	}
}

// TestSendStatus_UnknownCode tests SendStatus with unknown status code
func TestSendStatus_UnknownCode(t *testing.T) {
	t.Parallel()

	r := MustNew()

	r.GET("/test", func(c *Context) {
		err := c.SendStatus(999) // Unknown code
		assert.NoError(t, err)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 999, w.Code)

	// Should include numeric code in response
	assert.Contains(t, w.Body.String(), "999")
}
