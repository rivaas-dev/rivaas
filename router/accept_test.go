package router

import (
	"net/http/httptest"
	"testing"
)

func TestAccepts(t *testing.T) {
	tests := []struct {
		name         string
		acceptHeader string
		offers       []string
		expected     string
		description  string
	}{
		{
			name:         "simple json preference",
			acceptHeader: "application/json",
			offers:       []string{"json", "xml", "html"},
			expected:     "json",
			description:  "should match json when Accept is application/json",
		},
		{
			name:         "quality values - prefer higher quality",
			acceptHeader: "text/html, application/json;q=0.8",
			offers:       []string{"json", "html"},
			expected:     "html",
			description:  "should prefer html with implicit q=1.0 over json with q=0.8",
		},
		{
			name:         "quality values - prefer explicit higher quality",
			acceptHeader: "application/json;q=0.9, text/html;q=0.7",
			offers:       []string{"html", "json"},
			expected:     "json",
			description:  "should prefer json with q=0.9 over html with q=0.7",
		},
		{
			name:         "wildcard match",
			acceptHeader: "*/*",
			offers:       []string{"json", "xml"},
			expected:     "json",
			description:  "should return first offer when Accept is */*",
		},
		{
			name:         "type wildcard",
			acceptHeader: "text/*",
			offers:       []string{"json", "html", "txt"},
			expected:     "html",
			description:  "should match text/html when Accept is text/*",
		},
		{
			name:         "no accept header",
			acceptHeader: "",
			offers:       []string{"json", "xml"},
			expected:     "json",
			description:  "should return first offer when no Accept header",
		},
		{
			name:         "no match",
			acceptHeader: "application/xml",
			offers:       []string{"json", "html"},
			expected:     "",
			description:  "should return empty string when no match",
		},
		{
			name:         "full mime type offers",
			acceptHeader: "application/json, text/html",
			offers:       []string{"application/json", "text/html"},
			expected:     "application/json",
			description:  "should work with full MIME type offers",
		},
		{
			name:         "mixed short and full names",
			acceptHeader: "application/json",
			offers:       []string{"html", "json", "xml"},
			expected:     "json",
			description:  "should match short name to full MIME type",
		},
		{
			name:         "specificity - exact over wildcard",
			acceptHeader: "text/*, text/html",
			offers:       []string{"html", "txt"},
			expected:     "html",
			description:  "should prefer exact match over wildcard",
		},
		{
			name:         "empty offers",
			acceptHeader: "application/json",
			offers:       []string{},
			expected:     "",
			description:  "should return empty string with no offers",
		},
		{
			name:         "media type parameters",
			acceptHeader: "application/json;version=1",
			offers:       []string{"json", "xml"},
			expected:     "json",
			description:  "should match media type with parameters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.acceptHeader != "" {
				req.Header.Set("Accept", tt.acceptHeader)
			}
			w := httptest.NewRecorder()

			c := NewContext(w, req)
			result := c.Accepts(tt.offers...)

			if result != tt.expected {
				t.Errorf("Accepts() = %q, want %q\nDescription: %s\nAccept: %s\nOffers: %v",
					result, tt.expected, tt.description, tt.acceptHeader, tt.offers)
			}
		})
	}
}

func TestAcceptsCharsets(t *testing.T) {
	tests := []struct {
		name        string
		header      string
		offers      []string
		expected    string
		description string
	}{
		{
			name:        "simple utf-8 preference",
			header:      "utf-8",
			offers:      []string{"utf-8", "iso-8859-1"},
			expected:    "utf-8",
			description: "should match utf-8",
		},
		{
			name:        "quality values",
			header:      "utf-8, iso-8859-1;q=0.5",
			offers:      []string{"iso-8859-1", "utf-8"},
			expected:    "utf-8",
			description: "should prefer utf-8 with higher quality",
		},
		{
			name:        "wildcard",
			header:      "*",
			offers:      []string{"utf-8", "ascii"},
			expected:    "utf-8",
			description: "should return first offer with wildcard",
		},
		{
			name:        "no match",
			header:      "utf-16",
			offers:      []string{"utf-8", "iso-8859-1"},
			expected:    "",
			description: "should return empty string when no match",
		},
		{
			name:        "empty header",
			header:      "",
			offers:      []string{"utf-8"},
			expected:    "utf-8",
			description: "should return first offer when header empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("Accept-Charset", tt.header)
			}
			w := httptest.NewRecorder()

			c := NewContext(w, req)
			result := c.AcceptsCharsets(tt.offers...)

			if result != tt.expected {
				t.Errorf("AcceptsCharsets() = %q, want %q\nDescription: %s",
					result, tt.expected, tt.description)
			}
		})
	}
}

func TestAcceptsEncodings(t *testing.T) {
	tests := []struct {
		name        string
		header      string
		offers      []string
		expected    string
		description string
	}{
		{
			name:        "simple gzip",
			header:      "gzip",
			offers:      []string{"gzip", "deflate"},
			expected:    "gzip",
			description: "should match gzip",
		},
		{
			name:        "quality values - equal quality prefers first in header",
			header:      "gzip, br;q=1.0, deflate;q=0.8",
			offers:      []string{"gzip", "br", "deflate"},
			expected:    "gzip",
			description: "should prefer gzip when quality is equal (both 1.0) per RFC 7231",
		},
		{
			name:        "quality values - prefer br with higher quality",
			header:      "gzip;q=0.8, br;q=1.0, deflate;q=0.6",
			offers:      []string{"gzip", "br", "deflate"},
			expected:    "br",
			description: "should prefer br with q=1.0 over gzip with q=0.8",
		},
		{
			name:        "quality values - prefer gzip",
			header:      "gzip;q=1.0, br;q=0.9, deflate;q=0.8",
			offers:      []string{"deflate", "br", "gzip"},
			expected:    "gzip",
			description: "should prefer gzip with q=1.0",
		},
		{
			name:        "wildcard",
			header:      "*",
			offers:      []string{"gzip", "br"},
			expected:    "gzip",
			description: "should return first offer with wildcard",
		},
		{
			name:        "no match",
			header:      "compress",
			offers:      []string{"gzip", "deflate"},
			expected:    "",
			description: "should return empty when no match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("Accept-Encoding", tt.header)
			}
			w := httptest.NewRecorder()

			c := NewContext(w, req)
			result := c.AcceptsEncodings(tt.offers...)

			if result != tt.expected {
				t.Errorf("AcceptsEncodings() = %q, want %q\nDescription: %s",
					result, tt.expected, tt.description)
			}
		})
	}
}

func TestAcceptsLanguages(t *testing.T) {
	tests := []struct {
		name        string
		header      string
		offers      []string
		expected    string
		description string
	}{
		{
			name:        "simple english",
			header:      "en",
			offers:      []string{"en", "fr", "de"},
			expected:    "en",
			description: "should match en",
		},
		{
			name:        "quality values",
			header:      "en-US, en;q=0.9, fr;q=0.8",
			offers:      []string{"en", "fr", "de"},
			expected:    "en",
			description: "should prefer en with higher quality",
		},
		{
			name:        "language prefix match",
			header:      "en-US",
			offers:      []string{"en", "fr"},
			expected:    "en",
			description: "should match en-US to en",
		},
		{
			name:        "wildcard",
			header:      "*",
			offers:      []string{"en", "fr"},
			expected:    "en",
			description: "should return first offer with wildcard",
		},
		{
			name:        "no match",
			header:      "de",
			offers:      []string{"en", "fr"},
			expected:    "",
			description: "should return empty when no match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("Accept-Language", tt.header)
			}
			w := httptest.NewRecorder()

			c := NewContext(w, req)
			result := c.AcceptsLanguages(tt.offers...)

			if result != tt.expected {
				t.Errorf("AcceptsLanguages() = %q, want %q\nDescription: %s",
					result, tt.expected, tt.description)
			}
		})
	}
}

func TestAcceptsRealWorldScenarios(t *testing.T) {
	t.Run("browser accept header", func(t *testing.T) {
		// Typical browser Accept header
		header := "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8"
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept", header)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Should prefer HTML
		result := c.Accepts("json", "html", "xml")
		if result != "html" {
			t.Errorf("Expected html for browser, got %q", result)
		}
	})

	t.Run("api client accept header", func(t *testing.T) {
		header := "application/json, */*; q=0.01"
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept", header)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Should prefer JSON
		result := c.Accepts("html", "json", "xml")
		if result != "json" {
			t.Errorf("Expected json for API client, got %q", result)
		}
	})

	t.Run("compression negotiation", func(t *testing.T) {
		header := "gzip, deflate, br"
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", header)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Should return first match
		result := c.AcceptsEncodings("br", "gzip", "deflate")
		if result != "br" {
			t.Errorf("Expected br, got %q", result)
		}
	})
}

// Benchmark to ensure performance
func BenchmarkAccepts(b *testing.B) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept", "text/html,application/json;q=0.9,*/*;q=0.8")
	w := httptest.NewRecorder()
	c := NewContext(w, req)
	offers := []string{"json", "html", "xml"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Accepts(offers...)
	}
}

func BenchmarkAcceptsSimple(b *testing.B) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	c := NewContext(w, req)
	offers := []string{"json", "html"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Accepts(offers...)
	}
}

func BenchmarkAcceptsEncodings(b *testing.B) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	w := httptest.NewRecorder()
	c := NewContext(w, req)
	offers := []string{"gzip", "br", "deflate"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.AcceptsEncodings(offers...)
	}
}
