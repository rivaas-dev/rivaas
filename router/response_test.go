package router

import (
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// Test AppendHeader
func TestAppendHeader(t *testing.T) {
	t.Run("append to new header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.AppendHeader("X-Custom", "value1")

		if w.Header().Get("X-Custom") != "value1" {
			t.Errorf("X-Custom = %v, want value1", w.Header().Get("X-Custom"))
		}
	})

	t.Run("append to existing header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.AppendHeader("Link", "</page1>; rel=\"first\"")
		c.AppendHeader("Link", "</page2>; rel=\"next\"")

		link := w.Header().Get("Link")
		if !strings.Contains(link, "page1") || !strings.Contains(link, "page2") {
			t.Errorf("Link header should contain both values: %v", link)
		}
	})
}

// Test ContentType
func TestContentType(t *testing.T) {
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
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			c.ContentType(tt.input)

			ct := w.Header().Get("Content-Type")
			if !strings.Contains(ct, tt.expected) {
				t.Errorf("Content-Type = %v, want %v", ct, tt.expected)
			}
		})
	}
}

// Test Location
func TestLocation(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	c.Location("/new-path")

	if w.Header().Get("Location") != "/new-path" {
		t.Errorf("Location = %v, want /new-path", w.Header().Get("Location"))
	}
}

// Test Vary
func TestVary(t *testing.T) {
	t.Run("single field", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.Vary("Accept-Encoding")

		if w.Header().Get("Vary") != "Accept-Encoding" {
			t.Errorf("Vary = %v", w.Header().Get("Vary"))
		}
	})

	t.Run("multiple fields", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.Vary("Accept-Encoding", "Accept-Language")

		vary := w.Header().Get("Vary")
		if !strings.Contains(vary, "Accept-Encoding") || !strings.Contains(vary, "Accept-Language") {
			t.Errorf("Vary = %v", vary)
		}
	})

	t.Run("append to existing", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.Vary("Accept")
		c.Vary("Accept-Encoding")

		vary := w.Header().Get("Vary")
		if !strings.Contains(vary, "Accept,") || !strings.Contains(vary, "Accept-Encoding") {
			t.Errorf("Vary = %v", vary)
		}
	})

	t.Run("no duplicates", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.Vary("Accept")
		c.Vary("Accept") // Duplicate

		vary := w.Header().Get("Vary")
		// Should only appear once
		count := strings.Count(vary, "Accept")
		if count > 1 {
			t.Errorf("Accept appears %d times in Vary header, should be 1", count)
		}
	})
}

// Test Link
func TestLink(t *testing.T) {
	t.Run("single link", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.Link("/api/users?page=2", "next")

		link := w.Header().Get("Link")
		if !strings.Contains(link, "</api/users?page=2>") || !strings.Contains(link, `rel="next"`) {
			t.Errorf("Link = %v", link)
		}
	})

	t.Run("multiple links", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.Link("/api/users?page=2", "next")
		c.Link("/api/users?page=10", "last")

		link := w.Header().Get("Link")
		if !strings.Contains(link, "page=2") || !strings.Contains(link, "page=10") {
			t.Errorf("Link header should contain both links: %v", link)
		}
	})
}

// Test Download
func TestDownload(t *testing.T) {
	// Create a temporary test file
	tmpfile, err := os.CreateTemp("", "test-download-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := []byte("test file content")
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	t.Run("download with original filename", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		err := c.Download(tmpfile.Name())
		if err != nil {
			t.Fatalf("Download failed: %v", err)
		}

		// Check Content-Disposition header
		cd := w.Header().Get("Content-Disposition")
		if !strings.Contains(cd, "attachment") {
			t.Errorf("Content-Disposition should contain 'attachment': %v", cd)
		}
	})

	t.Run("download with custom filename", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		err := c.Download(tmpfile.Name(), "custom-name.txt")
		if err != nil {
			t.Fatalf("Download failed: %v", err)
		}

		cd := w.Header().Get("Content-Disposition")
		if !strings.Contains(cd, "custom-name.txt") {
			t.Errorf("Content-Disposition should contain custom filename: %v", cd)
		}
	})
}

// Test Send
func TestSend(t *testing.T) {
	t.Run("send raw bytes", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		data := []byte("raw binary data")
		err := c.Send(data)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		if w.Body.String() != "raw binary data" {
			t.Errorf("Body = %v, want 'raw binary data'", w.Body.String())
		}

		// Should set default Content-Type
		if w.Header().Get("Content-Type") != "application/octet-stream" {
			t.Errorf("Content-Type = %v", w.Header().Get("Content-Type"))
		}
	})

	t.Run("send with existing content type", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.Header("Content-Type", "text/plain")
		err := c.Send([]byte("text data"))
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		// Should not override existing Content-Type
		if w.Header().Get("Content-Type") != "text/plain" {
			t.Errorf("Content-Type should remain text/plain")
		}
	})
}

// Test SendStatus
func TestSendStatus(t *testing.T) {
	t.Run("send 404", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		err := c.SendStatus(404)
		if err != nil {
			t.Fatalf("SendStatus failed: %v", err)
		}

		if w.Code != 404 {
			t.Errorf("Status code = %d, want 404", w.Code)
		}
		if w.Body.String() != "Not Found" {
			t.Errorf("Body = %v, want 'Not Found'", w.Body.String())
		}
	})

	t.Run("send 201", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		err := c.SendStatus(201)
		if err != nil {
			t.Fatalf("SendStatus failed: %v", err)
		}

		if w.Body.String() != "Created" {
			t.Errorf("Body = %v, want 'Created'", w.Body.String())
		}
	})
}

// Test JSONP
func TestJSONP(t *testing.T) {
	t.Run("default callback", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		data := map[string]string{"message": "Hello"}
		err := c.JSONP(200, data)
		if err != nil {
			t.Fatalf("JSONP failed: %v", err)
		}

		body := w.Body.String()
		if !strings.HasPrefix(body, "callback(") || !strings.HasSuffix(body, ")") {
			t.Errorf("JSONP should wrap with callback(): %v", body)
		}
		if !strings.Contains(body, `"message":"Hello"`) {
			t.Errorf("JSONP should contain JSON data: %v", body)
		}

		// Check Content-Type
		if !strings.Contains(w.Header().Get("Content-Type"), "application/javascript") {
			t.Errorf("Content-Type should be application/javascript")
		}
	})

	t.Run("custom callback", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		data := map[string]int{"count": 42}
		err := c.JSONP(200, data, "myCallback")
		if err != nil {
			t.Fatalf("JSONP failed: %v", err)
		}

		body := w.Body.String()
		if !strings.HasPrefix(body, "myCallback(") {
			t.Errorf("JSONP should use custom callback: %v", body)
		}
	})
}

// Test Format
func TestFormat(t *testing.T) {
	data := map[string]string{"message": "test"}

	t.Run("format as JSON", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		err := c.Format(200, data)
		if err != nil {
			t.Fatalf("Format failed: %v", err)
		}

		if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
			t.Error("Should send as JSON")
		}
	})

	t.Run("format as HTML", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept", "text/html")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		err := c.Format(200, data)
		if err != nil {
			t.Fatalf("Format failed: %v", err)
		}

		if !strings.Contains(w.Header().Get("Content-Type"), "text/html") {
			t.Error("Should send as HTML")
		}
		if !strings.Contains(w.Body.String(), "<p>") {
			t.Error("HTML should be wrapped in tags")
		}
	})

	t.Run("format as plain text", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept", "text/plain")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		err := c.Format(200, "simple text")
		if err != nil {
			t.Fatalf("Format failed: %v", err)
		}

		if !strings.Contains(w.Header().Get("Content-Type"), "text/plain") {
			t.Error("Should send as plain text")
		}
	})

	t.Run("format default to JSON", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		// No Accept header
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		err := c.Format(200, data)
		if err != nil {
			t.Fatalf("Format failed: %v", err)
		}

		// Should default to JSON
		if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
			t.Error("Should default to JSON when no Accept header")
		}
	})
}

// Test Write and WriteString
func TestWrite(t *testing.T) {
	t.Run("write bytes", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		n, err := c.Write([]byte("hello world"))
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if n != 11 {
			t.Errorf("Write returned %d bytes, want 11", n)
		}
		if w.Body.String() != "hello world" {
			t.Errorf("Body = %v", w.Body.String())
		}
	})

	t.Run("write string", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		n, err := c.WriteString("hello string")
		if err != nil {
			t.Fatalf("WriteString failed: %v", err)
		}
		if n != 12 {
			t.Errorf("WriteString returned %d bytes, want 12", n)
		}
		if w.Body.String() != "hello string" {
			t.Errorf("Body = %v", w.Body.String())
		}
	})

	t.Run("use with fmt.Fprintf", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Context implements io.Writer
		fmt.Fprintf(c, "User: %s, Count: %d", "Alice", 42)

		expected := "User: Alice, Count: 42"
		if w.Body.String() != expected {
			t.Errorf("Body = %v, want %v", w.Body.String(), expected)
		}
	})
}

// Test real-world scenarios
func TestResponseHelpers_RealWorld(t *testing.T) {
	t.Run("API response with links", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/users?page=2", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Set pagination links
		c.Link("/api/users?page=1", "first")
		c.Link("/api/users?page=3", "next")
		c.Link("/api/users?page=10", "last")

		// Set vary for caching
		c.Vary("Accept", "Accept-Language")

		// Send response
		c.JSON(200, map[string]string{"status": "ok"})

		// Verify headers
		link := w.Header().Get("Link")
		if !strings.Contains(link, "next") || !strings.Contains(link, "last") {
			t.Errorf("Link header incomplete: %v", link)
		}

		vary := w.Header().Get("Vary")
		if !strings.Contains(vary, "Accept") {
			t.Errorf("Vary header incomplete: %v", vary)
		}
	})

	t.Run("conditional response based on accept", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/user", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		user := map[string]any{
			"id":   123,
			"name": "John",
		}

		// Use Format for automatic content negotiation
		c.Format(200, user)

		if !strings.Contains(w.Body.String(), "123") {
			t.Error("Should contain user data")
		}
	})
}

// Benchmark response methods
func BenchmarkSend(b *testing.B) {
	data := []byte("response data")
	req := httptest.NewRequest("GET", "/", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c := NewContext(w, req)
		c.Send(data)
	}
}

func BenchmarkWrite(b *testing.B) {
	data := []byte("response data")
	req := httptest.NewRequest("GET", "/", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c := NewContext(w, req)
		c.Write(data)
	}
}

func BenchmarkWriteString(b *testing.B) {
	req := httptest.NewRequest("GET", "/", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c := NewContext(w, req)
		c.WriteString("response data")
	}
}
