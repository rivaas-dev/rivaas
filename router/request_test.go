package router

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test AllParams
func TestAllParams(t *testing.T) {
	t.Run("single parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/123", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.paramCount = 1
		c.paramKeys[0] = "id"
		c.paramValues[0] = "123"

		params := c.AllParams()
		if len(params) != 1 {
			t.Fatalf("Expected 1 param, got %d", len(params))
		}
		if params["id"] != "123" {
			t.Errorf("id = %v, want 123", params["id"])
		}
	})

	t.Run("multiple parameters", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/123/posts/456", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.paramCount = 2
		c.paramKeys[0] = "id"
		c.paramValues[0] = "123"
		c.paramKeys[1] = "post_id"
		c.paramValues[1] = "456"

		params := c.AllParams()
		if len(params) != 2 {
			t.Fatalf("Expected 2 params, got %d", len(params))
		}
		if params["id"] != "123" || params["post_id"] != "456" {
			t.Errorf("params = %v", params)
		}
	})

	t.Run("no parameters", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		params := c.AllParams()
		if len(params) != 0 {
			t.Errorf("Expected empty map, got %v", params)
		}
	})

	t.Run("more than 8 parameters (fallback to map)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Simulate >8 params using map
		c.Params = map[string]string{
			"p1": "v1",
			"p2": "v2",
			"p3": "v3",
		}

		params := c.AllParams()
		if len(params) != 3 {
			t.Fatalf("Expected 3 params, got %d", len(params))
		}
		if params["p2"] != "v2" {
			t.Errorf("p2 = %v", params["p2"])
		}
	})
}

// Test AllQueries
func TestAllQueries(t *testing.T) {
	t.Run("single query param", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?q=golang", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		queries := c.AllQueries()
		if len(queries) != 1 {
			t.Fatalf("Expected 1 query, got %d", len(queries))
		}
		if queries["q"] != "golang" {
			t.Errorf("q = %v, want golang", queries["q"])
		}
	})

	t.Run("multiple query params", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?q=golang&page=2&limit=10", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		queries := c.AllQueries()
		if len(queries) != 3 {
			t.Fatalf("Expected 3 queries, got %d", len(queries))
		}
		if queries["q"] != "golang" || queries["page"] != "2" || queries["limit"] != "10" {
			t.Errorf("queries = %v", queries)
		}
	})

	t.Run("multiple values - returns last", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?tag=first&tag=second&tag=third", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		queries := c.AllQueries()
		if queries["tag"] != "third" {
			t.Errorf("tag = %v, want third (last value)", queries["tag"])
		}
	})

	t.Run("no query params", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		queries := c.AllQueries()
		if len(queries) != 0 {
			t.Errorf("Expected empty map, got %v", queries)
		}
	})
}

// Test RequestHeaders and ResponseHeaders
func TestHeaders(t *testing.T) {
	t.Run("request headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("User-Agent", "Test/1.0")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer token123")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		headers := c.RequestHeaders()
		if len(headers) < 3 {
			t.Fatalf("Expected at least 3 headers, got %d", len(headers))
		}
		if headers["User-Agent"] != "Test/1.0" {
			t.Errorf("User-Agent = %v", headers["User-Agent"])
		}
		if headers["Accept"] != "application/json" {
			t.Errorf("Accept = %v", headers["Accept"])
		}
	})

	t.Run("response headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.Header("Content-Type", "application/json")
		c.Header("Cache-Control", "no-cache")
		c.Header("X-Custom", "value")

		headers := c.ResponseHeaders()
		if headers["Content-Type"] != "application/json" {
			t.Errorf("Content-Type = %v", headers["Content-Type"])
		}
		if headers["Cache-Control"] != "no-cache" {
			t.Errorf("Cache-Control = %v", headers["Cache-Control"])
		}
		if headers["X-Custom"] != "value" {
			t.Errorf("X-Custom = %v", headers["X-Custom"])
		}
	})
}

// Test Hostname and Port
func TestHostnameAndPort(t *testing.T) {
	t.Run("hostname without port", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com/path", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		if c.Hostname() != "example.com" {
			t.Errorf("Hostname() = %v, want example.com", c.Hostname())
		}
		if c.Port() != "" {
			t.Errorf("Port() = %v, want empty", c.Port())
		}
	})

	t.Run("hostname with port", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com:8080/path", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		if c.Hostname() != "example.com" {
			t.Errorf("Hostname() = %v, want example.com", c.Hostname())
		}
		if c.Port() != "8080" {
			t.Errorf("Port() = %v, want 8080", c.Port())
		}
	})

	t.Run("IPv6 hostname", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://[2001:db8::1]:8080/path", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		hostname := c.Hostname()
		if hostname != "[2001:db8::1]" {
			t.Errorf("Hostname() = %v, want [2001:db8::1]", hostname)
		}
		if c.Port() != "8080" {
			t.Errorf("Port() = %v, want 8080", c.Port())
		}
	})
}

// Test Scheme
func TestScheme(t *testing.T) {
	t.Run("http scheme", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		if c.Scheme() != "http" {
			t.Errorf("Scheme() = %v, want http", c.Scheme())
		}
	})

	t.Run("https scheme with TLS", func(t *testing.T) {
		req := httptest.NewRequest("GET", "https://example.com/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Note: httptest doesn't set TLS, so we test with header
		req.Header.Set("X-Forwarded-Proto", "https")
		if c.Scheme() != "https" {
			t.Errorf("Scheme() = %v, want https", c.Scheme())
		}
	})

	t.Run("X-Forwarded-Ssl header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com/", nil)
		req.Header.Set("X-Forwarded-Ssl", "on")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		if c.Scheme() != "https" {
			t.Errorf("Scheme() = %v, want https with X-Forwarded-Ssl", c.Scheme())
		}
	})
}

// Test BaseURL and FullURL
func TestURLs(t *testing.T) {
	t.Run("base URL", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com:8080/api/users?page=2", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		baseURL := c.BaseURL()
		if baseURL != "http://example.com:8080" {
			t.Errorf("BaseURL() = %v, want http://example.com:8080", baseURL)
		}
	})

	t.Run("full URL", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com/search?q=golang&page=2", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		fullURL := c.FullURL()
		expected := "http://example.com/search?q=golang&page=2"
		if fullURL != expected {
			t.Errorf("FullURL() = %v, want %v", fullURL, expected)
		}
	})

	t.Run("full URL without query", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com/path", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		fullURL := c.FullURL()
		if fullURL != "http://example.com/path" {
			t.Errorf("FullURL() = %v", fullURL)
		}
	})
}

// Test ClientIPs
func TestClientIPs(t *testing.T) {
	t.Run("single IP from RemoteAddr", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		ips := c.ClientIPs()
		if len(ips) != 1 {
			t.Fatalf("Expected 1 IP, got %d", len(ips))
		}
		if ips[0] != "192.168.1.1" {
			t.Errorf("IP = %v, want 192.168.1.1", ips[0])
		}
	})

	t.Run("IP chain from X-Forwarded-For", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.1, 198.51.100.1, 192.0.2.1")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		ips := c.ClientIPs()
		if len(ips) != 3 {
			t.Fatalf("Expected 3 IPs, got %d", len(ips))
		}
		if ips[0] != "203.0.113.1" {
			t.Errorf("First IP = %v, want 203.0.113.1", ips[0])
		}
		if ips[1] != "198.51.100.1" {
			t.Errorf("Second IP = %v", ips[1])
		}
	})

	t.Run("X-Forwarded-For with spaces", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Forwarded-For", "  203.0.113.1  ,  198.51.100.1  ")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		ips := c.ClientIPs()
		if len(ips) != 2 {
			t.Fatalf("Expected 2 IPs, got %d", len(ips))
		}
		if ips[0] != "203.0.113.1" || ips[1] != "198.51.100.1" {
			t.Errorf("IPs not trimmed correctly: %v", ips)
		}
	})

	t.Run("RemoteAddr without port format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		// Set RemoteAddr to format that SplitHostPort cannot parse (no port)
		// This tests the fallback behavior when RemoteAddr cannot be split
		req.RemoteAddr = "192.168.1.1"
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		ips := c.ClientIPs()
		if len(ips) != 1 {
			t.Fatalf("Expected 1 IP, got %d", len(ips))
		}
		if ips[0] != "192.168.1.1" {
			t.Errorf("IP = %v, want 192.168.1.1 (raw RemoteAddr)", ips[0])
		}
	})

	t.Run("RemoteAddr with invalid format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		// Set RemoteAddr to invalid format that SplitHostPort cannot parse
		req.RemoteAddr = "invalid-address-format"
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		ips := c.ClientIPs()
		if len(ips) != 1 {
			t.Fatalf("Expected 1 IP, got %d", len(ips))
		}
		if ips[0] != "invalid-address-format" {
			t.Errorf("IP = %v, want invalid-address-format (raw RemoteAddr)", ips[0])
		}
	})
}

// Test IsLocalhost
func TestIsLocalhost(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		expected   bool
	}{
		{"127.0.0.1", "127.0.0.1:12345", "", true},
		{"::1", "[::1]:12345", "", true},
		{"localhost", "localhost:12345", "", true},
		{"127.x.x.x range", "127.5.5.5:12345", "", true},
		{"external IP", "203.0.113.1:12345", "", false},
		{"IPv6 external", "[2001:db8::1]:12345", "", false},
		// Note: "localhost via XFF" test requires router with trusted proxies configured
		// Skipping this test case as it needs router setup
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			result := c.IsLocalhost()
			if result != tt.expected {
				t.Errorf("IsLocalhost() = %v, want %v (IP: %s)", result, tt.expected, c.ClientIP())
			}
		})
	}
}

// Test IsHTTPS
func TestIsHTTPS(t *testing.T) {
	t.Run("http request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		if c.IsHTTPS() {
			t.Error("IsHTTPS() should be false for http")
		}
	})

	t.Run("https via X-Forwarded-Proto", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com/", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		if !c.IsHTTPS() {
			t.Error("IsHTTPS() should be true with X-Forwarded-Proto: https")
		}
	})

	t.Run("https via X-Forwarded-Ssl", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com/", nil)
		req.Header.Set("X-Forwarded-Ssl", "on")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		if !c.IsHTTPS() {
			t.Error("IsHTTPS() should be true with X-Forwarded-Ssl: on")
		}
	})
}

// Test IsXHR
func TestIsXHR(t *testing.T) {
	t.Run("XHR request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		if !c.IsXHR() {
			t.Error("IsXHR() should be true with X-Requested-With header")
		}
	})

	t.Run("regular request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		if c.IsXHR() {
			t.Error("IsXHR() should be false without X-Requested-With header")
		}
	})
}

// Test Subdomains
func TestSubdomains(t *testing.T) {
	t.Run("simple subdomain", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://api.example.com/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		subdomains := c.Subdomains()
		if len(subdomains) != 1 {
			t.Fatalf("Expected 1 subdomain, got %d", len(subdomains))
		}
		if subdomains[0] != "api" {
			t.Errorf("Subdomain = %v, want api", subdomains[0])
		}
	})

	t.Run("multiple subdomains", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://api.v1.example.com/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		subdomains := c.Subdomains()
		if len(subdomains) != 2 {
			t.Fatalf("Expected 2 subdomains, got %d: %v", len(subdomains), subdomains)
		}
		if subdomains[0] != "v1" || subdomains[1] != "api" {
			t.Errorf("Subdomains = %v, want [v1, api]", subdomains)
		}
	})

	t.Run("no subdomain", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		subdomains := c.Subdomains()
		if len(subdomains) != 0 {
			t.Errorf("Expected no subdomains, got %v", subdomains)
		}
	})

	t.Run("custom offset for .co.uk", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://api.example.co.uk/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		subdomains := c.Subdomains(3) // offset 3 for .co.uk
		if len(subdomains) != 1 {
			t.Fatalf("Expected 1 subdomain, got %d: %v", len(subdomains), subdomains)
		}
		if subdomains[0] != "api" {
			t.Errorf("Subdomain = %v, want api", subdomains[0])
		}
	})
}

// Test IsFresh and IsStale
func TestCacheFreshness(t *testing.T) {
	t.Run("fresh with matching ETag", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("If-None-Match", `"abc123"`)
		w := httptest.NewRecorder()
		c := NewContext(w, req)
		c.Header("ETag", `"abc123"`)

		if !c.IsFresh() {
			t.Error("IsFresh() should be true with matching ETag")
		}
		if c.IsStale() {
			t.Error("IsStale() should be false with matching ETag")
		}
	})

	t.Run("stale with different ETag", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("If-None-Match", `"abc123"`)
		w := httptest.NewRecorder()
		c := NewContext(w, req)
		c.Header("ETag", `"xyz789"`)

		if c.IsFresh() {
			t.Error("IsFresh() should be false with different ETag")
		}
		if !c.IsStale() {
			t.Error("IsStale() should be true with different ETag")
		}
	})

	t.Run("fresh with If-Modified-Since", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("If-Modified-Since", "Wed, 21 Oct 2015 07:28:00 GMT")
		w := httptest.NewRecorder()
		c := NewContext(w, req)
		c.Header("Last-Modified", "Wed, 21 Oct 2015 07:28:00 GMT")

		if !c.IsFresh() {
			t.Error("IsFresh() should be true with matching Last-Modified")
		}
	})

	t.Run("stale with Cache-Control no-cache", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("If-None-Match", `"abc123"`)
		w := httptest.NewRecorder()
		c := NewContext(w, req)
		c.Header("ETag", `"abc123"`)

		if c.IsFresh() {
			t.Error("IsFresh() should be false with Cache-Control: no-cache")
		}
	})

	t.Run("no cache headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		if c.IsFresh() {
			t.Error("IsFresh() should be false without cache headers")
		}
	})
}

// Test FormFile
func TestFormFile(t *testing.T) {
	t.Run("single file upload", func(t *testing.T) {
		// Create multipart form
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Add a file
		fileWriter, err := writer.CreateFormFile("document", "test.txt")
		if err != nil {
			t.Fatal(err)
		}
		fileWriter.Write([]byte("test file content"))
		writer.Close()

		// Create request
		req := httptest.NewRequest("POST", "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Get file
		file, err := c.FormFile("document")
		if err != nil {
			t.Fatalf("FormFile failed: %v", err)
		}

		if file.Filename != "test.txt" {
			t.Errorf("Filename = %v, want test.txt", file.Filename)
		}
		if file.Size == 0 {
			t.Error("File size should not be zero")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.Close()

		req := httptest.NewRequest("POST", "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		_, err := c.FormFile("nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})

	t.Run("failed to parse multipart form", func(t *testing.T) {
		// Create request with multipart Content-Type but malformed body
		// This tests error handling when ParseMultipartForm fails
		req := httptest.NewRequest("POST", "/upload", bytes.NewBufferString("malformed multipart data"))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=----WebKitFormBoundary7MA4YWxkTrZu0gW")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		_, err := c.FormFile("document")
		if err == nil {
			t.Fatal("Expected error when parsing malformed multipart form")
		}
		if !strings.Contains(err.Error(), "failed to parse multipart form") {
			t.Errorf("Expected error about parsing multipart form, got: %v", err)
		}
	})

	t.Run("file not found after manipulation", func(t *testing.T) {
		// Test error handling when MultipartForm.File[key] is manipulated
		// after parsing. This tests defensive error handling.
		// Create multipart form with a file
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, err := writer.CreateFormFile("target", "data.txt")
		if err != nil {
			t.Fatal(err)
		}
		fileWriter.Write([]byte("content"))
		writer.Close()

		req := httptest.NewRequest("POST", "/upload", bytes.NewReader(body.Bytes()))
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Parse the form - this populates MultipartForm.File["target"]
		if err := req.ParseMultipartForm(32 << 20); err != nil {
			t.Fatalf("Failed to parse: %v", err)
		}

		// Verify setup
		if req.MultipartForm == nil || req.MultipartForm.File["target"] == nil {
			t.Fatal("Test setup failed: file should exist")
		}

		// Manipulate state: clear the file entry
		// This simulates an edge case where the file map is modified
		req.MultipartForm.File["target"] = []*multipart.FileHeader{} // empty, not nil

		// FormFile should return an error when the file entry is empty
		_, err = c.FormFile("target")
		if err == nil {
			t.Fatal("Expected error when file entry is cleared")
		}
		// Error should indicate the file is not found
		if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "no such file") {
			t.Errorf("Expected 'not found' or 'no such file' error, got: %v", err)
		}
	})
}

// Test FormFiles
func TestFormFiles(t *testing.T) {
	t.Run("multiple files", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Add multiple files
		for i, name := range []string{"file1.txt", "file2.txt", "file3.txt"} {
			fw, err := writer.CreateFormFile("documents", name)
			if err != nil {
				t.Fatal(err)
			}
			fw.Write([]byte("content " + string(rune('A'+i))))
		}
		writer.Close()

		req := httptest.NewRequest("POST", "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		files, err := c.FormFiles("documents")
		if err != nil {
			t.Fatalf("FormFiles failed: %v", err)
		}

		if len(files) != 3 {
			t.Fatalf("Expected 3 files, got %d", len(files))
		}

		if files[0].Filename != "file1.txt" {
			t.Errorf("First file = %v, want file1.txt", files[0].Filename)
		}
	})

	t.Run("no files found", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.Close()

		req := httptest.NewRequest("POST", "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		_, err := c.FormFiles("nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent files")
		}
	})
}

// Test SaveFile
func TestSaveFile(t *testing.T) {
	// Create temp directory for uploads
	tmpDir, err := os.MkdirTemp("", "router-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("save uploaded file", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		fileContent := []byte("test file content for saving")
		fw, err := writer.CreateFormFile("upload", "testfile.txt")
		if err != nil {
			t.Fatal(err)
		}
		fw.Write(fileContent)
		writer.Close()

		req := httptest.NewRequest("POST", "/", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Get file
		file, err := c.FormFile("upload")
		if err != nil {
			t.Fatalf("FormFile failed: %v", err)
		}

		// Save file
		dstPath := filepath.Join(tmpDir, "saved-file.txt")
		if err := c.SaveFile(file, dstPath); err != nil {
			t.Fatalf("SaveFile failed: %v", err)
		}

		// Verify file was saved
		savedContent, err := os.ReadFile(dstPath)
		if err != nil {
			t.Fatalf("Failed to read saved file: %v", err)
		}

		if string(savedContent) != string(fileContent) {
			t.Errorf("Saved content = %v, want %v", string(savedContent), string(fileContent))
		}
	})

	t.Run("create parent directories", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		fw, err := writer.CreateFormFile("upload", "test.txt")
		if err != nil {
			t.Fatal(err)
		}
		fw.Write([]byte("content"))
		writer.Close()

		req := httptest.NewRequest("POST", "/", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		file, _ := c.FormFile("upload")

		// Save to nested path
		dstPath := filepath.Join(tmpDir, "nested", "dir", "file.txt")
		if err := c.SaveFile(file, dstPath); err != nil {
			t.Fatalf("SaveFile should create parent dirs: %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(dstPath); os.IsNotExist(err) {
			t.Error("File should exist at nested path")
		}
	})
}

// Test MultipartForm
func TestMultipartForm(t *testing.T) {
	t.Run("access multipart form", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Add files
		fw1, _ := writer.CreateFormFile("docs", "file1.txt")
		fw1.Write([]byte("content1"))
		fw2, _ := writer.CreateFormFile("docs", "file2.txt")
		fw2.Write([]byte("content2"))

		// Add form values
		writer.WriteField("username", "testuser")
		writer.WriteField("email", "test@example.com")
		writer.Close()

		req := httptest.NewRequest("POST", "/", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		form, err := c.MultipartForm()
		if err != nil {
			t.Fatalf("MultipartForm failed: %v", err)
		}

		// Check files
		if len(form.File["docs"]) != 2 {
			t.Errorf("Expected 2 files, got %d", len(form.File["docs"]))
		}

		// Check values
		if len(form.Value["username"]) == 0 || form.Value["username"][0] != "testuser" {
			t.Error("Username not found in form values")
		}
	})
}
