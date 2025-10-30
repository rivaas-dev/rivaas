package router

import (
	"net/http/httptest"
	"strconv"
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

// ============================================================================
// Edge Cases Tests (merged from accept_edge_test.go)
// ============================================================================

// TestHeaderArena_Reset tests the reset method for arena recycling
func TestHeaderArena_Reset(t *testing.T) {
	arena := arenaPool.Get().(*headerArena)

	// Simulate usage by setting used count and adding specs
	arena.used = 5
	for i := 0; i < 5; i++ {
		arena.specs[i].value = "test"
		arena.specs[i].quality = 900
		arena.specs[i].params = make(map[string]string)
		arena.specs[i].params["key"] = "value"
	}

	// Reset should clear used and specs
	arena.reset()

	if arena.used != 0 {
		t.Errorf("reset should clear used, got %d", arena.used)
	}

	// Verify specs are cleared
	for i := 0; i < 5; i++ {
		if arena.specs[i].value != "" {
			t.Error("spec value should be cleared")
		}
		if arena.specs[i].params != nil {
			t.Error("spec params should be nil")
		}
		if arena.specs[i].quality != 0 {
			t.Error("spec quality should be 0")
		}
	}

	// Return to pool
	arenaPool.Put(arena)
}

// TestHeaderArena_GetSpecs tests the getSpecs method
func TestHeaderArena_GetSpecs(t *testing.T) {
	r := New()

	r.GET("/test", func(c *Context) {
		// First call to Accepts triggers spec parsing
		accepts := c.Accepts("application/json", "text/html")

		if accepts == "" {
			t.Error("should accept something")
		}

		// Second call should reuse cached specs via getSpecs
		accepts2 := c.Accepts("text/html", "application/json")

		if accepts2 == "" {
			t.Error("second call should also work")
		}

		c.Status(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept", "application/json, text/html;q=0.9")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// ============================================================================
// Q-Value Tests (merged from accept_qvalue_test.go)
// ============================================================================

func TestParseQFast(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		// Valid q=1 variants
		{"q=1", "1", 1000},
		{"q=1.0", "1.0", 1000},
		{"q=1.00", "1.00", 1000},
		{"q=1.000", "1.000", 1000},

		// Valid q=0 variants
		{"q=0", "0", 0},
		{"q=0.0", "0.0", 0},
		{"q=0.00", "0.00", 0},
		{"q=0.000", "0.000", 0},

		// Common q-values
		{"q=0.9", "0.9", 900},
		{"q=0.8", "0.8", 800},
		{"q=0.5", "0.5", 500},
		{"q=0.1", "0.1", 100},

		// Two decimal places
		{"q=0.95", "0.95", 950},
		{"q=0.85", "0.85", 850},
		{"q=0.75", "0.75", 750},
		{"q=0.50", "0.50", 500},
		{"q=0.25", "0.25", 250},
		{"q=0.10", "0.10", 100},
		{"q=0.05", "0.05", 50},
		{"q=0.01", "0.01", 10},

		// Three decimal places
		{"q=0.999", "0.999", 999},
		{"q=0.500", "0.500", 500},
		{"q=0.123", "0.123", 123},
		{"q=0.001", "0.001", 1},

		// Edge cases - invalid inputs
		{"empty", "", -1},
		{"too_long", "1.0000", -1},
		{"invalid_start", "2", -1},
		{"invalid_start_letter", "a", -1},
		{"no_decimal", "10", -1},
		{"invalid_after_decimal", "1.5", -1},
		{"invalid_char", "0.a", -1},
		{"missing_decimal", "01", -1},
		{"greater_than_1", "1.1", -1},
		{"negative", "-0.5", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseQFast(tt.input)
			if result != tt.expected {
				t.Errorf("parseQFast(%q) = %d, want %d", tt.input, result, tt.expected)
			}

			// For valid inputs, verify it matches strconv.ParseFloat
			if tt.expected >= 0 {
				if f, err := strconv.ParseFloat(tt.input, 64); err == nil {
					expectedFloat := float64(tt.expected) / 1000.0
					if f != expectedFloat {
						t.Errorf("parseQFast(%q) = %d (%.3f), but ParseFloat = %.3f",
							tt.input, result, expectedFloat, f)
					}
				}
			}
		})
	}
}

// TestParseQFastVsParseFloat verifies parseQFast matches ParseFloat for valid inputs
func TestParseQFastVsParseFloat(t *testing.T) {
	// Test a range of valid q-values
	for i := 0; i <= 1000; i++ {
		// Generate q-values: 0, 0.001, 0.002, ..., 0.999, 1.000
		var input string
		if i == 1000 {
			input = "1"
		} else {
			input = "0." + strconv.Itoa(i + 1000)[1:] // Format as 0.XXX
		}

		fastResult := parseQFast(input)
		if fastResult < 0 {
			t.Errorf("parseQFast(%q) returned error %d", input, fastResult)
			continue
		}

		floatResult, err := strconv.ParseFloat(input, 64)
		if err != nil {
			t.Errorf("ParseFloat(%q) failed: %v", input, err)
			continue
		}

		expectedInt := int(floatResult * 1000)
		if fastResult != expectedInt {
			t.Errorf("parseQFast(%q) = %d, but ParseFloat*1000 = %d",
				input, fastResult, expectedInt)
		}
	}
}

// TestParseQFastEdgeCases tests edge cases and malformed inputs
func TestParseQFastEdgeCases(t *testing.T) {
	invalidInputs := []string{
		"",       // Empty
		" ",      // Whitespace
		"1.0.0",  // Double decimal
		"0.9.8",  // Double decimal
		".5",     // Missing leading zero
		"0.",     // Trailing decimal
		"1.",     // Trailing decimal
		"00.5",   // Leading zero
		"1.0000", // Too many decimals
		"2.0",    // Greater than 1
		"1.1",    // Greater than 1
		"-1",     // Negative
		"-0.5",   // Negative
		"abc",    // Non-numeric
		"0.abc",  // Non-numeric after decimal
		"1.abc",  // Non-numeric after decimal
		"0x1",    // Hex notation
	}

	for _, input := range invalidInputs {
		t.Run("invalid_"+input, func(t *testing.T) {
			result := parseQFast(input)
			if result != -1 {
				t.Errorf("parseQFast(%q) = %d, want -1 (invalid)", input, result)
			}
		})
	}
}

// BenchmarkParseQFast benchmarks the fast q-value parser
func BenchmarkParseQFast(b *testing.B) {
	testCases := []struct {
		name  string
		input string
	}{
		{"q=1", "1"},
		{"q=1.0", "1.0"},
		{"q=0.9", "0.9"},
		{"q=0.85", "0.85"},
		{"q=0.001", "0.001"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = parseQFast(tc.input)
			}
		})
	}
}

// BenchmarkParseQFastVsParseFloat compares parseQFast to strconv.ParseFloat
func BenchmarkParseQFastVsParseFloat(b *testing.B) {
	testCases := []struct {
		name  string
		input string
	}{
		{"q=1", "1"},
		{"q=0.9", "0.9"},
		{"q=0.85", "0.85"},
	}

	for _, tc := range testCases {
		b.Run("fast_"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = parseQFast(tc.input)
			}
		})

		b.Run("stdlib_"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = strconv.ParseFloat(tc.input, 64)
			}
		})
	}
}

// BenchmarkAcceptParsingWithQValues benchmarks full Accept header parsing with q-values
func BenchmarkAcceptParsingWithQValues(b *testing.B) {
	headers := []string{
		"text/html, application/json;q=0.9, */*;q=0.8",
		"application/json;q=1.0, text/html;q=0.9, text/plain;q=0.8",
		"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	}

	for _, header := range headers {
		b.Run(header[:20], func(b *testing.B) {
			arena := arenaPool.Get().(*headerArena)
			defer func() {
				arena.reset()
				arenaPool.Put(arena)
			}()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = parseAcceptFast(header, arena)
				arena.used = 0 // Reset arena for next iteration
			}
		})
	}
}
