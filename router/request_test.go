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

//go:build !integration

package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test AllParams
func TestAllParams(t *testing.T) {
	t.Parallel()

	t.Run("single parameter", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.paramCount = 1
		c.paramKeys[0] = "id"
		c.paramValues[0] = "123"

		params := c.AllParams()
		require.Len(t, params, 1, "Expected 1 param")
		assert.Equal(t, "123", params["id"])
	})

	t.Run("multiple parameters", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/users/123/posts/456", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.paramCount = 2
		c.paramKeys[0] = "id"
		c.paramValues[0] = "123"
		c.paramKeys[1] = "post_id"
		c.paramValues[1] = "456"

		params := c.AllParams()
		require.Len(t, params, 2, "Expected 2 params")
		assert.Equal(t, "123", params["id"])
		assert.Equal(t, "456", params["post_id"])
	})

	t.Run("no parameters", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		params := c.AllParams()
		assert.Empty(t, params, "Expected empty map")
	})

	t.Run("more than 8 parameters (fallback to map)", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Simulate >8 params using map
		c.Params = map[string]string{
			"p1": "v1",
			"p2": "v2",
			"p3": "v3",
		}

		params := c.AllParams()
		require.Len(t, params, 3, "Expected 3 params")
		assert.Equal(t, "v2", params["p2"])
	})
}

// Test AllQueries
func TestAllQueries(t *testing.T) {
	t.Parallel()

	t.Run("single query param", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/?q=golang", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		queries := c.AllQueries()
		require.Len(t, queries, 1, "Expected 1 query")
		assert.Equal(t, "golang", queries["q"])
	})

	t.Run("multiple query params", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/?q=golang&page=2&limit=10", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		queries := c.AllQueries()
		require.Len(t, queries, 3, "Expected 3 queries")
		assert.Equal(t, "golang", queries["q"])
		assert.Equal(t, "2", queries["page"])
		assert.Equal(t, "10", queries["limit"])
	})

	t.Run("multiple values - returns last", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/?tag=first&tag=second&tag=third", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		queries := c.AllQueries()
		assert.Equal(t, "third", queries["tag"], "Should return last value")
	})

	t.Run("no query params", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		queries := c.AllQueries()
		assert.Empty(t, queries, "Expected empty map")
	})
}

// Test RequestHeaders and ResponseHeaders
func TestHeaders(t *testing.T) {
	t.Parallel()

	t.Run("request headers", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("User-Agent", "Test/1.0")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer token123")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		headers := c.RequestHeaders()
		assert.GreaterOrEqual(t, len(headers), 3, "Expected at least 3 headers")
		assert.Equal(t, "Test/1.0", headers["User-Agent"])
		assert.Equal(t, "application/json", headers["Accept"])
	})

	t.Run("response headers", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		c.Header("Content-Type", "application/json")
		c.Header("Cache-Control", "no-cache")
		c.Header("X-Custom", "value")

		headers := c.ResponseHeaders()
		assert.Equal(t, "application/json", headers["Content-Type"])
		assert.Equal(t, "no-cache", headers["Cache-Control"])
		assert.Equal(t, "value", headers["X-Custom"])
	})
}

// Test Hostname and Port
func TestHostnameAndPort(t *testing.T) {
	t.Parallel()

	t.Run("hostname without port", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		assert.Equal(t, "example.com", c.Hostname())
		assert.Empty(t, c.Port())
	})

	t.Run("hostname with port", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "http://example.com:8080/path", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		assert.Equal(t, "example.com", c.Hostname())
		assert.Equal(t, "8080", c.Port())
	})

	t.Run("IPv6 hostname", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "http://[2001:db8::1]:8080/path", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		hostname := c.Hostname()
		assert.Equal(t, "[2001:db8::1]", hostname)
		assert.Equal(t, "8080", c.Port())
	})
}

// Test Scheme
func TestScheme(t *testing.T) {
	t.Parallel()

	t.Run("http scheme", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		assert.Equal(t, "http", c.Scheme())
	})

	t.Run("https scheme with TLS", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Note: httptest doesn't set TLS, so we test with header
		req.Header.Set("X-Forwarded-Proto", "https")
		assert.Equal(t, "https", c.Scheme())
	})

	t.Run("X-Forwarded-Ssl header", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		req.Header.Set("X-Forwarded-Ssl", "on")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		assert.Equal(t, "https", c.Scheme(), "Should detect https with X-Forwarded-Ssl")
	})
}

// Test BaseURL and FullURL
func TestURLs(t *testing.T) {
	t.Parallel()

	t.Run("base URL", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "http://example.com:8080/api/users?page=2", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		baseURL := c.BaseURL()
		assert.Equal(t, "http://example.com:8080", baseURL)
	})

	t.Run("full URL", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "http://example.com/search?q=golang&page=2", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		fullURL := c.FullURL()
		expected := "http://example.com/search?q=golang&page=2"
		assert.Equal(t, expected, fullURL)
	})

	t.Run("full URL without query", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		fullURL := c.FullURL()
		assert.Equal(t, "http://example.com/path", fullURL)
	})
}

// Test ClientIPs
func TestClientIPs(t *testing.T) {
	t.Parallel()

	t.Run("single IP from RemoteAddr", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		ips := c.ClientIPs()
		require.Len(t, ips, 1, "Expected 1 IP")
		assert.Equal(t, "192.168.1.1", ips[0])
	})

	t.Run("IP chain from X-Forwarded-For", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.1, 198.51.100.1, 192.0.2.1")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		ips := c.ClientIPs()
		require.Len(t, ips, 3, "Expected 3 IPs")
		assert.Equal(t, "203.0.113.1", ips[0])
		assert.Equal(t, "198.51.100.1", ips[1])
	})

	t.Run("X-Forwarded-For with spaces", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "  203.0.113.1  ,  198.51.100.1  ")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		ips := c.ClientIPs()
		require.Len(t, ips, 2, "Expected 2 IPs")
		assert.Equal(t, "203.0.113.1", ips[0])
		assert.Equal(t, "198.51.100.1", ips[1])
	})

	t.Run("RemoteAddr without port format", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		// Set RemoteAddr to format that SplitHostPort cannot parse (no port)
		// This tests the fallback behavior when RemoteAddr cannot be split
		req.RemoteAddr = "192.168.1.1"
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		ips := c.ClientIPs()
		require.Len(t, ips, 1, "Expected 1 IP")
		assert.Equal(t, "192.168.1.1", ips[0], "Should use raw RemoteAddr")
	})

	t.Run("RemoteAddr with invalid format", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		// Set RemoteAddr to invalid format that SplitHostPort cannot parse
		req.RemoteAddr = "invalid-address-format"
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		ips := c.ClientIPs()
		require.Len(t, ips, 1, "Expected 1 IP")
		assert.Equal(t, "invalid-address-format", ips[0], "Should use raw RemoteAddr")
	})
}

// Test IsLocalhost
func TestIsLocalhost(t *testing.T) {
	t.Parallel()

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
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			result := c.IsLocalhost()
			assert.Equal(t, tt.expected, result, "IsLocalhost() for IP: %s", c.ClientIP())
		})
	}
}

// Test IsHTTPS
func TestIsHTTPS(t *testing.T) {
	t.Parallel()

	t.Run("http request", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		assert.False(t, c.IsHTTPS(), "IsHTTPS() should be false for http")
	})

	t.Run("https via X-Forwarded-Proto", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		assert.True(t, c.IsHTTPS(), "IsHTTPS() should be true with X-Forwarded-Proto: https")
	})

	t.Run("https via X-Forwarded-Ssl", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		req.Header.Set("X-Forwarded-Ssl", "on")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		assert.True(t, c.IsHTTPS(), "IsHTTPS() should be true with X-Forwarded-Ssl: on")
	})
}

// Test IsXHR
func TestIsXHR(t *testing.T) {
	t.Parallel()

	t.Run("XHR request", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		assert.True(t, c.IsXHR(), "IsXHR() should be true with X-Requested-With header")
	})

	t.Run("regular request", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		assert.False(t, c.IsXHR(), "IsXHR() should be false without X-Requested-With header")
	})
}

// Test Subdomains
func TestSubdomains(t *testing.T) {
	t.Parallel()

	t.Run("simple subdomain", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "http://api.example.com/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		subdomains := c.Subdomains()
		require.Len(t, subdomains, 1, "Expected 1 subdomain")
		assert.Equal(t, "api", subdomains[0])
	})

	t.Run("multiple subdomains", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "http://api.v1.example.com/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		subdomains := c.Subdomains()
		require.Len(t, subdomains, 2, "Expected 2 subdomains")
		assert.Equal(t, "v1", subdomains[0])
		assert.Equal(t, "api", subdomains[1])
	})

	t.Run("no subdomain", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		subdomains := c.Subdomains()
		assert.Empty(t, subdomains, "Expected no subdomains")
	})

	t.Run("custom offset for .co.uk", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "http://api.example.co.uk/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		subdomains := c.Subdomains(3) // offset 3 for .co.uk
		require.Len(t, subdomains, 1, "Expected 1 subdomain")
		assert.Equal(t, "api", subdomains[0])
	})
}

// Test IsFresh and IsStale
func TestCacheFreshness(t *testing.T) {
	t.Parallel()

	t.Run("fresh with matching ETag", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("If-None-Match", `"abc123"`)
		w := httptest.NewRecorder()
		c := NewContext(w, req)
		c.Header("ETag", `"abc123"`)

		assert.True(t, c.IsFresh(), "IsFresh() should be true with matching ETag")
		assert.False(t, c.IsStale(), "IsStale() should be false with matching ETag")
	})

	t.Run("stale with different ETag", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("If-None-Match", `"abc123"`)
		w := httptest.NewRecorder()
		c := NewContext(w, req)
		c.Header("ETag", `"xyz789"`)

		assert.False(t, c.IsFresh(), "IsFresh() should be false with different ETag")
		assert.True(t, c.IsStale(), "IsStale() should be true with different ETag")
	})

	t.Run("fresh with If-Modified-Since", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("If-Modified-Since", "Wed, 21 Oct 2015 07:28:00 GMT")
		w := httptest.NewRecorder()
		c := NewContext(w, req)
		c.Header("Last-Modified", "Wed, 21 Oct 2015 07:28:00 GMT")

		assert.True(t, c.IsFresh(), "IsFresh() should be true with matching Last-Modified")
	})

	t.Run("stale with Cache-Control no-cache", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("If-None-Match", `"abc123"`)
		w := httptest.NewRecorder()
		c := NewContext(w, req)
		c.Header("ETag", `"abc123"`)

		assert.False(t, c.IsFresh(), "IsFresh() should be false with Cache-Control: no-cache")
	})

	t.Run("no cache headers", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		assert.False(t, c.IsFresh(), "IsFresh() should be false without cache headers")
	})
}
