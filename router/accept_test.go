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
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccepts(t *testing.T) {
	t.Parallel()

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
		{
			name:         "empty specs returns first offer",
			acceptHeader: "   ",
			offers:       []string{"json", "xml"},
			expected:     "json",
			description:  "should return first offer when Accept header parses to empty specs",
		},
		{
			name:         "accept header with only commas and whitespace",
			acceptHeader: " , , ",
			offers:       []string{"html", "json"},
			expected:     "html",
			description:  "should return first offer when Accept header has only empty parts",
		},
		{
			name:         "accept header with only commas",
			acceptHeader: ",,,",
			offers:       []string{"xml", "json"},
			expected:     "xml",
			description:  "should return first offer when Accept header is only commas",
		},
		{
			name:         "accept header with only tabs",
			acceptHeader: "\t\t\t",
			offers:       []string{"json", "html"},
			expected:     "json",
			description:  "should return first offer when Accept header is only whitespace",
		},
		{
			name:         "accept header with mixed whitespace and commas",
			acceptHeader: " \t , \t , ",
			offers:       []string{"xml", "html", "json"},
			expected:     "xml",
			description:  "should return first offer when Accept header has invalid content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.acceptHeader != "" {
				req.Header.Set("Accept", tt.acceptHeader)
			}
			w := httptest.NewRecorder()

			c := NewContext(w, req)
			result := c.Accepts(tt.offers...)

			assert.Equal(t, tt.expected, result, "Accepts()\nDescription: %s\nAccept: %s\nOffers: %v", tt.description, tt.acceptHeader, tt.offers)
		})
	}
}

// TestAccepts_CacheHit tests the per-request caching in Accepts.
func TestAccepts_CacheHit(t *testing.T) {
	t.Parallel()

	t.Run("cached_specs_reused", func(t *testing.T) {
		t.Parallel()
		// Test that cached specs are reused when Accept header matches
		acceptHeader := "application/json, text/html;q=0.9"
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", acceptHeader)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// First call - should parse and cache
		result1 := c.Accepts("json", "html", "xml")
		assert.Equal(t, "json", result1, "First Accepts() call")

		// Verify cache was populated
		assert.Equal(t, acceptHeader, c.cachedAcceptHeader, "cachedAcceptHeader")
		require.NotNil(t, c.cachedAcceptSpecs, "cachedAcceptSpecs should be populated after first call")
		assert.NotEmpty(t, c.cachedAcceptSpecs, "cachedAcceptSpecs should contain parsed specs")

		// Second call with same Accept header - should use cached specs
		result2 := c.Accepts("json", "html", "xml")
		assert.Equal(t, "json", result2, "Second Accepts() call")

		// Verify cache is still valid
		assert.Equal(t, acceptHeader, c.cachedAcceptHeader, "cachedAcceptHeader should not change")
		assert.NotNil(t, c.cachedAcceptSpecs, "cachedAcceptSpecs should still be valid after second call")

		// Third call with different offers but same header - should still use cache
		result3 := c.Accepts("html", "json")
		assert.Equal(t, "json", result3, "Third Accepts() call")
	})

	t.Run("cache_invalidated_on_different_header", func(t *testing.T) {
		t.Parallel()

		// Test that cache is invalidated when Accept header changes
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// First call
		result1 := c.Accepts("json", "html")
		assert.Equal(t, "json", result1, "First Accepts()")

		originalCachedHeader := c.cachedAcceptHeader
		originalCachedSpecsLen := 0
		if c.cachedAcceptSpecs != nil {
			originalCachedSpecsLen = len(c.cachedAcceptSpecs)
		}
		if originalCachedSpecsLen == 0 {
			require.NotNil(t, c.cachedAcceptSpecs, "cachedAcceptSpecs should be populated")
		}

		// Change Accept header
		req.Header.Set("Accept", "text/html")

		// Second call with different header - should parse again (not use cache)
		result2 := c.Accepts("json", "html")
		assert.Equal(t, "html", result2, "Second Accepts() with different header")

		// Verify cache was updated
		assert.Equal(t, "text/html", c.cachedAcceptHeader, "cachedAcceptHeader")
		assert.NotEqual(t, originalCachedHeader, c.cachedAcceptHeader, "cachedAcceptHeader should be different after header change")
		assert.NotNil(t, c.cachedAcceptSpecs, "cachedAcceptSpecs should be populated after parsing new header")
		// Length may be different, which confirms it's a new parse
		_ = originalCachedSpecsLen
	})

	t.Run("multiple_cache_hits_same_request", func(t *testing.T) {
		t.Parallel()

		// Test multiple cache hits within the same request
		acceptHeader := "application/json, text/html;q=0.8, application/xml;q=0.6"
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", acceptHeader)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Multiple calls with same header
		for i := range 5 {
			result := c.Accepts("json", "html", "xml")
			assert.Equal(t, "json", result, "Accepts() call %d", i+1)
		}

		// Verify cache was used (header should remain the same)
		assert.Equal(t, acceptHeader, c.cachedAcceptHeader, "cachedAcceptHeader")
		assert.NotNil(t, c.cachedAcceptSpecs, "cachedAcceptSpecs should be populated")
	})

	t.Run("cache_nil_specs_handled", func(t *testing.T) {
		t.Parallel()

		// Test that nil cached specs don't cause issues
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", "")
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// First call with empty header
		result1 := c.Accepts("json", "html")
		assert.Equal(t, "json", result1, "Accepts() with empty header")

		// Verify empty header doesn't cache specs (early return, not parsing)
		assert.Empty(t, c.cachedAcceptHeader, "cachedAcceptHeader should be empty")
	})
}

func TestAcceptsCharsets(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Accept-Charset", tt.header)
			}
			w := httptest.NewRecorder()

			c := NewContext(w, req)
			result := c.AcceptsCharsets(tt.offers...)

			assert.Equal(t, tt.expected, result, "AcceptsCharsets()\nDescription: %s", tt.description)
		})
	}
}

func TestAcceptsEncodings(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Accept-Encoding", tt.header)
			}
			w := httptest.NewRecorder()

			c := NewContext(w, req)
			result := c.AcceptsEncodings(tt.offers...)

			assert.Equal(t, tt.expected, result, "AcceptsEncodings()\nDescription: %s", tt.description)
		})
	}
}

func TestAcceptsLanguages(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Accept-Language", tt.header)
			}
			w := httptest.NewRecorder()

			c := NewContext(w, req)
			result := c.AcceptsLanguages(tt.offers...)

			assert.Equal(t, tt.expected, result, "AcceptsLanguages()\nDescription: %s", tt.description)
		})
	}
}

func TestAcceptsRealWorldScenarios(t *testing.T) {
	t.Parallel()

	t.Run("browser accept header", func(t *testing.T) {
		t.Parallel()
		// Typical browser Accept header
		header := "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8"
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", header)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Should prefer HTML
		result := c.Accepts("json", "html", "xml")
		assert.Equal(t, "html", result, "Expected html for browser")
	})

	t.Run("api client accept header", func(t *testing.T) {
		t.Parallel()

		header := "application/json, */*; q=0.01"
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", header)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Should prefer JSON
		result := c.Accepts("html", "json", "xml")
		assert.Equal(t, "json", result, "Expected json for API client")
	})

	t.Run("compression negotiation", func(t *testing.T) {
		t.Parallel()

		header := "gzip, deflate, br"
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", header)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		// Should return first match
		result := c.AcceptsEncodings("br", "gzip", "deflate")
		assert.Equal(t, "br", result, "Expected br")
	})
}

func BenchmarkAccepts(b *testing.B) {
	b.Run("complex", func(b *testing.B) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", "text/html,application/json;q=0.9,*/*;q=0.8")
		w := httptest.NewRecorder()
		c := NewContext(w, req)
		offers := []string{"json", "html", "xml"}

		b.ResetTimer()
		for b.Loop() {
			_ = c.Accepts(offers...)
		}
	})

	b.Run("simple", func(b *testing.B) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		c := NewContext(w, req)
		offers := []string{"json", "html"}

		b.ResetTimer()
		for b.Loop() {
			_ = c.Accepts(offers...)
		}
	})
}

func BenchmarkAcceptsEncodings(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	w := httptest.NewRecorder()
	c := NewContext(w, req)
	offers := []string{"gzip", "br", "deflate"}

	b.ResetTimer()
	for b.Loop() {
		_ = c.AcceptsEncodings(offers...)
	}
}

// TestHeaderArena_Reset tests the reset method for arena recycling.
func TestHeaderArena_Reset(t *testing.T) {
	t.Parallel()

	arena, ok := arenaPool.Get().(*headerArena)
	require.True(t, ok, "arenaPool.Get() returned non-*headerArena type")

	// Simulate usage by setting used count and adding specs
	arena.used = 5
	for i := range 5 {
		arena.specs[i].value = "test"
		arena.specs[i].quality = 900
		arena.specs[i].params = make(map[string]string)
		arena.specs[i].params["key"] = "value"
	}

	// Reset should clear used and specs
	arena.reset()

	assert.Equal(t, 0, arena.used, "reset should clear used")

	// Verify specs are cleared
	for i := range 5 {
		assert.Empty(t, arena.specs[i].value, "spec value should be cleared")
		assert.Nil(t, arena.specs[i].params, "spec params should be nil")
		assert.InDelta(t, 0.0, arena.specs[i].quality, 0.001, "spec quality should be 0")
	}

	// Return to pool
	arenaPool.Put(arena)
}

// TestHeaderArena_GetSpecs tests the getSpecs method
func TestHeaderArena_GetSpecs(t *testing.T) {
	t.Parallel()

	t.Run("within_buffer_size_uses_arena", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			capacity int
		}{
			{"capacity 1", 1},
			{"capacity 4", 4},
			{"capacity 8", 8},
			{"capacity 16", 16}, // Max buffer size (boundary: should use arena)
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				// Each parallel subtest gets its own arena to avoid data races
				arena, ok := arenaPool.Get().(*headerArena)
				require.True(t, ok, "arenaPool.Get() returned non-*headerArena type")
				defer func() {
					arena.reset()
					arenaPool.Put(arena)
				}()

				result := arena.getSpecs(tt.capacity)

				require.NotNil(t, result, "getSpecs should not return nil")
				assert.GreaterOrEqual(t, cap(result), tt.capacity, "getSpecs(%d) returned slice with capacity %d, want at least %d", tt.capacity, cap(result), tt.capacity)
				assert.Empty(t, result, "getSpecs(%d) returned slice with length %d, want 0", tt.capacity, len(result))
				assert.LessOrEqual(t, cap(result), 16, "getSpecs(%d) returned slice with capacity %d > 16, should use arena buffer", tt.capacity, cap(result))
			})
		}
	})

	t.Run("exceeds_buffer_size_heap_alloc", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			capacity int
		}{
			{"capacity 17", 17},     // Boundary: just over buffer (should use heap)
			{"capacity 20", 20},     // Small overflow
			{"capacity 50", 50},     // Medium overflow
			{"capacity 100", 100},   // Large overflow
			{"capacity 1000", 1000}, // Very large overflow
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				// Each parallel subtest gets its own arena to avoid data races
				arena, ok := arenaPool.Get().(*headerArena)
				require.True(t, ok, "arenaPool.Get() returned non-*headerArena type")
				defer func() {
					arena.reset()
					arenaPool.Put(arena)
				}()

				result := arena.getSpecs(tt.capacity)

				require.NotNil(t, result, "getSpecs should not return nil even for large capacity")
				assert.GreaterOrEqual(t, cap(result), tt.capacity, "getSpecs(%d) returned slice with capacity %d, want at least %d", tt.capacity, cap(result), tt.capacity)
				assert.Empty(t, result, "getSpecs(%d) returned slice with length %d, want 0", tt.capacity, len(result))
				assert.Greater(t, cap(result), 16, "getSpecs(%d) returned slice with capacity %d <= 16, should use heap allocation", tt.capacity, cap(result))
				assert.Equal(t, 0, arena.used, "arena.used should be 0 after getSpecs")
			})
		}
	})

	t.Run("multiple_calls_reset_used", func(t *testing.T) {
		t.Parallel()

		// Test that getSpecs resets arena.used to 0
		arena, ok := arenaPool.Get().(*headerArena)
		require.True(t, ok, "arenaPool.Get() returned non-*headerArena type")
		defer func() {
			arena.reset()
			arenaPool.Put(arena)
		}()

		// Simulate previous usage
		arena.used = 10

		result := arena.getSpecs(4)

		if arena.used != 0 {
			assert.Equal(t, 0, arena.used, "getSpecs should reset arena.used to 0")
		}

		if result == nil {
			assert.NotNil(t, arena.getSpecs(10), "getSpecs should not return nil")
		}
	})

	t.Run("end_to_end_with_large_header", func(t *testing.T) {
		t.Parallel()

		// Test that parseAccept with large header triggers heap allocation
		arena, ok := arenaPool.Get().(*headerArena)
		require.True(t, ok, "arenaPool.Get() returned non-*headerArena type")
		defer func() {
			arena.reset()
			arenaPool.Put(arena)
		}()

		// Create a header with more than 16 parts to trigger heap allocation
		parts := make([]string, 0, 20)
		for range 20 {
			parts = append(parts, "application/json")
		}
		largeHeader := strings.Join(parts, ", ")

		specs := parseAccept(largeHeader, arena)

		assert.Len(t, specs, 20, "parseAccept with 20 parts returned %d specs, want 20", len(specs))

		// Verify all specs are parsed correctly
		for i, spec := range specs {
			assert.Equal(t, "application/json", spec.value, "spec[%d].value", i)
		}
	})
}

func TestParseQuality(t *testing.T) {
	t.Parallel()

	t.Run("valid_inputs", func(t *testing.T) {
		t.Parallel()
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

			// Basic invalid inputs (others covered in invalid_inputs subtest)
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
				t.Parallel()

				result := parseQuality(tt.input)
				assert.Equal(t, tt.expected, result, "parseQuality(%q)", tt.input)

				// For valid inputs, verify it matches strconv.ParseFloat
				if tt.expected >= 0 {
					if f, err := strconv.ParseFloat(tt.input, 64); err == nil {
						expectedFloat := float64(tt.expected) / 1000.0
						assert.InDelta(t, expectedFloat, f, 0.001, "parseQuality(%q) = %d (%.3f), but ParseFloat = %.3f", tt.input, result, expectedFloat, f)
					}
				}
			})
		}
	})

	t.Run("invalid_inputs", func(t *testing.T) {
		t.Parallel()

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
				t.Parallel()

				result := parseQuality(input)
				assert.Equal(t, -1, result, "parseQuality(%q) should return -1 for invalid input", input)
			})
		}
	})

	t.Run("vs_parseFloat_comprehensive", func(t *testing.T) {
		t.Parallel()

		// Test a range of valid q-values
		for i := range 1001 {
			// Generate q-values: 0, 0.001, 0.002, ..., 0.999, 1.000
			var input string
			if i == 1000 {
				input = "1"
			} else {
				input = "0." + strconv.Itoa(i + 1000)[1:] // Format as 0.XXX
			}

			fastResult := parseQuality(input)
			require.GreaterOrEqual(t, fastResult, 0, "parseQuality(%q) returned error %d", input, fastResult)

			floatResult, err := strconv.ParseFloat(input, 64)
			require.NoError(t, err, "ParseFloat(%q) failed", input)

			expectedInt := int(floatResult * 1000)
			assert.Equal(t, expectedInt, fastResult, "parseQuality(%q) = %d, but ParseFloat*1000 = %d", input, fastResult, expectedInt)
		}
	})
}

// TestParseAcceptPart tests parseAcceptPart with various inputs.
//
//nolint:paralleltest // Some subtests share arena pool state
func TestParseAcceptPart(t *testing.T) {
	t.Parallel()

	t.Run("empty_or_whitespace_single_part", func(t *testing.T) {
		t.Parallel()
		// This covers the early return path when an empty spec is returned
		// when start >= end after trimming whitespace.
		tests := []struct {
			name     string
			input    string
			expected acceptSpec
		}{
			{
				name:  "empty string",
				input: "",
				expected: acceptSpec{
					quality: 1.0,
					params:  nil,
					value:   "",
				},
			},
			{
				name:  "single space",
				input: " ",
				expected: acceptSpec{
					quality: 1.0,
					params:  nil,
					value:   "",
				},
			},
			{
				name:  "multiple spaces",
				input: "   ",
				expected: acceptSpec{
					quality: 1.0,
					params:  nil,
					value:   "",
				},
			},
			{
				name:  "single tab",
				input: "\t",
				expected: acceptSpec{
					quality: 1.0,
					params:  nil,
					value:   "",
				},
			},
			{
				name:  "multiple tabs",
				input: "\t\t\t",
				expected: acceptSpec{
					quality: 1.0,
					params:  nil,
					value:   "",
				},
			},
			{
				name:  "mixed spaces and tabs",
				input: " \t \t ",
				expected: acceptSpec{
					quality: 1.0,
					params:  nil,
					value:   "",
				},
			},
			{
				name:  "tabs and spaces",
				input: "\t \t",
				expected: acceptSpec{
					quality: 1.0,
					params:  nil,
					value:   "",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := parseAcceptPart(tt.input)

				assert.Equal(t, tt.expected.value, result.value, "parseAcceptPart(%q).value", tt.input)
				assert.InDelta(t, tt.expected.quality, result.quality, 0.001, "parseAcceptPart(%q).quality", tt.input)
				if tt.expected.params == nil {
					assert.Nil(t, result.params, "parseAcceptPart(%q).params should be nil", tt.input)
				}
				assert.Equal(t, tt.expected.rawQuality, result.rawQuality, "parseAcceptPart(%q).rawQuality", tt.input)
			})
		}
	})

	t.Run("empty_parts_in_full_header", func(t *testing.T) {
		// Tests that empty parts in Accept headers are properly filtered out
		// by parseAccept (which checks spec.value != "").
		tests := []struct {
			name           string
			header         string
			expectedCount  int
			expectedValues []string
		}{
			{
				name:           "empty parts between commas",
				header:         "application/json, , text/html",
				expectedCount:  2,
				expectedValues: []string{"application/json", "text/html"},
			},
			{
				name:           "empty parts at start",
				header:         " , application/json",
				expectedCount:  1,
				expectedValues: []string{"application/json"},
			},
			{
				name:           "empty parts at end",
				header:         "application/json, ",
				expectedCount:  1,
				expectedValues: []string{"application/json"},
			},
			{
				name:           "multiple empty parts",
				header:         " , , application/json, , text/html, ",
				expectedCount:  2,
				expectedValues: []string{"application/json", "text/html"},
			},
			{
				name:           "whitespace-only parts",
				header:         "application/json,  \t , text/html, \t\t",
				expectedCount:  2,
				expectedValues: []string{"application/json", "text/html"},
			},
			{
				name:           "all empty parts",
				header:         " , ,  \t ",
				expectedCount:  0,
				expectedValues: []string{},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				arena, ok := arenaPool.Get().(*headerArena)
				require.True(t, ok, "arenaPool.Get() returned non-*headerArena type")
				defer func() {
					arena.reset()
					arenaPool.Put(arena)
				}()

				specs := parseAccept(tt.header, arena)

				assert.Len(t, specs, tt.expectedCount, "parseAccept(%q) returned %d specs, want %d", tt.header, len(specs), tt.expectedCount)

				require.Len(t, specs, len(tt.expectedValues), "spec count mismatch: got %d, want %d", len(specs), len(tt.expectedValues))

				for i, spec := range specs {
					assert.Equal(t, tt.expectedValues[i], spec.value, "spec[%d].value", i)
				}
			})
		}
	})
}

// TestParseAccept_EmptyHeader tests parseAccept with empty header.
// This covers the early return path when header is empty.
//
//nolint:paralleltest // Some subtests share arena pool state
func TestParseAccept_EmptyHeader(t *testing.T) {
	t.Parallel()

	t.Run("empty_header_returns_nil", func(t *testing.T) {
		t.Parallel()
		// Test returning nil when header is empty string
		arena, ok := arenaPool.Get().(*headerArena)
		require.True(t, ok, "arenaPool.Get() returned non-*headerArena type")
		defer func() {
			arena.reset()
			arenaPool.Put(arena)
		}()

		result := parseAccept("", arena)

		assert.Nil(t, result, "parseAccept(\"\") should return nil")
		assert.Empty(t, result, "parseAccept(\"\") returned slice with length %d, want 0 or nil", len(result))
	})

	t.Run("empty_header_early_return_before_arena_usage", func(t *testing.T) {
		// Test that empty header returns nil immediately, even before accessing arena
		// This ensures early return before any arena operations
		result := parseAccept("", nil)
		assert.Nil(t, result, "parseAccept(\"\") with nil arena should return nil")
	})

	t.Run("empty_header_vs_whitespace_header", func(t *testing.T) {
		// Test distinction between empty string (returns nil) vs whitespace (returns empty slice)
		arena, ok := arenaPool.Get().(*headerArena)
		require.True(t, ok, "arenaPool.Get() returned non-*headerArena type")
		defer func() {
			arena.reset()
			arenaPool.Put(arena)
		}()

		// Empty string should return nil
		resultEmpty := parseAccept("", arena)
		assert.Nil(t, resultEmpty, "parseAccept(\"\") should return nil")

		// Whitespace should return empty slice (not nil)
		arena.reset()
		resultWhitespace := parseAccept("   ", arena)
		assert.NotNil(t, resultWhitespace, "parseAccept(\"   \") should return empty slice, not nil")
		assert.Empty(t, resultWhitespace)
	})
}

// TestParseAcceptParam tests parseAcceptParam function, specifically covering early return cases.
//
//nolint:paralleltest // Some subtests share arena pool state
func TestParseAcceptParam(t *testing.T) {
	t.Parallel()

	t.Run("empty_or_whitespace_only_returns_early", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name  string
			param string
		}{
			{"empty string", ""},
			{"single space", " "},
			{"multiple spaces", "   "},
			{"single tab", "\t"},
			{"multiple tabs", "\t\t\t"},
			{"mixed whitespace", " \t \t "},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				spec := acceptSpec{
					quality: 1.0,
					params:  nil,
				}
				originalSpec := spec

				parseAcceptParam(tt.param, &spec)

				// Spec should be unchanged (early return for empty/whitespace)
				assert.Equal(t, originalSpec.value, spec.value, "spec.value should be unchanged")
				assert.InDelta(t, originalSpec.quality, spec.quality, 0.001, "spec.quality should be unchanged")
				assert.Nil(t, spec.params, "spec.params should be nil")
			})
		}
	})

	t.Run("no_equals_sign_returns_early", func(t *testing.T) {
		tests := []struct {
			name  string
			param string
		}{
			{"no equals", "key"},
			{"no equals with spaces", "key value"},
			{"no equals with tabs", "key\tvalue"},
			{"no equals whitespace trimmed", "  key  "},
			{"invalid param format", "key;value"},
			{"just whitespace and text", "  some text  "},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				spec := acceptSpec{
					quality: 1.0,
					params:  nil,
				}
				originalSpec := spec

				parseAcceptParam(tt.param, &spec)

				// Spec should be unchanged (early return when no equals sign)
				assert.Equal(t, originalSpec.value, spec.value, "spec.value should be unchanged")
				assert.InDelta(t, originalSpec.quality, spec.quality, 0.001, "spec.quality should be unchanged")
				assert.Nil(t, spec.params, "spec.params should be nil")
			})
		}
	})

	t.Run("empty_key_after_trimming_returns_early", func(t *testing.T) {
		tests := []struct {
			name  string
			param string
		}{
			{"equals at start", "=value"},
			{"whitespace before equals", " =value"},
			{"multiple spaces before equals", "   =value"},
			{"tabs before equals", "\t=value"},
			{"mixed whitespace before equals", " \t =value"},
			{"equals only", "="},
			{"whitespace and equals", " ="},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				spec := acceptSpec{
					quality: 1.0,
					params:  nil,
				}
				originalSpec := spec

				parseAcceptParam(tt.param, &spec)

				// Spec should be unchanged (early return when key is empty after trimming)
				assert.Equal(t, originalSpec.value, spec.value, "spec.value should be unchanged")
				assert.InDelta(t, originalSpec.quality, spec.quality, 0.001, "spec.quality should be unchanged")
				assert.Nil(t, spec.params, "spec.params should be nil")
			})
		}
	})

	t.Run("empty_value_after_trimming_returns_early", func(t *testing.T) {
		tests := []struct {
			name  string
			param string
		}{
			{"equals at end", "key="},
			{"whitespace after equals", "key= "},
			{"multiple spaces after equals", "key=   "},
			{"tabs after equals", "key=\t"},
			{"mixed whitespace after equals", "key= \t "},
			{"key with whitespace before equals", "key ="},
			{"key with whitespace before and after equals", "key = "},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				spec := acceptSpec{
					quality: 1.0,
					params:  nil,
				}
				originalSpec := spec

				parseAcceptParam(tt.param, &spec)

				// Spec should be unchanged (early return when value is empty after trimming)
				assert.Equal(t, originalSpec.value, spec.value, "spec.value should be unchanged")
				assert.InDelta(t, originalSpec.quality, spec.quality, 0.001, "spec.quality should be unchanged")
				assert.Nil(t, spec.params, "spec.params should be nil")
			})
		}
	})

	t.Run("parseFloat_fallback_lines_376_379", func(t *testing.T) {
		// Test the fallback path when parseQuality fails but ParseFloat succeeds
		// Fallback to ParseFloat for edge cases
		tests := []struct {
			name           string
			param          string
			expectedResult string // "set" if quality should be set, "unchanged" if not
			expectedValue  float64
		}{
			{
				name:           "q value with 4 decimal places - valid fallback",
				param:          "q=0.1234",
				expectedResult: "set",
				expectedValue:  0.1234,
			},
			{
				name:           "q value with many decimal places - valid fallback",
				param:          "q=0.123456789",
				expectedResult: "set",
				expectedValue:  0.123456789,
			},
			{
				name:           "q value exactly 0 - valid fallback",
				param:          "q=0.0000001",
				expectedResult: "set",
				expectedValue:  0.0000001,
			},
			{
				name:           "q value very close to 1 - valid fallback",
				param:          "q=0.9999999",
				expectedResult: "set",
				expectedValue:  0.9999999,
			},
			{
				name:           "q value exactly 1.0",
				param:          "q=1.0",
				expectedResult: "set",
				expectedValue:  1.0,
			},
			{
				name:           "q value greater than 1 - fallback succeeds but out of range",
				param:          "q=1.5",
				expectedResult: "unchanged",
				expectedValue:  1.0, // Default quality
			},
			{
				name:           "q value negative - fallback succeeds but out of range",
				param:          "q=-0.5",
				expectedResult: "unchanged",
				expectedValue:  1.0, // Default quality
			},
			{
				name:           "q value exactly 1 but invalid format - fallback succeeds",
				param:          "q=1.000000",
				expectedResult: "set",
				expectedValue:  1.0,
			},
			{
				name:           "q value zero with many decimals - valid fallback",
				param:          "q=0.000000",
				expectedResult: "set",
				expectedValue:  0.0,
			},
			{
				name:           "q value very small - valid fallback",
				param:          "q=0.000001",
				expectedResult: "set",
				expectedValue:  0.000001,
			},
			{
				name:           "q value that parses but exceeds 1.0",
				param:          "q=1.001",
				expectedResult: "unchanged",
				expectedValue:  1.0, // Default quality, > 1 not accepted
			},
			{
				name:           "q value exactly 1.0 - edge case",
				param:          "q=1.0000",
				expectedResult: "set",
				expectedValue:  1.0,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				spec := acceptSpec{
					quality: 1.0, // Default quality
					params:  nil,
				}

				parseAcceptParam(tt.param, &spec)

				if tt.expectedResult == "set" {
					// Quality should be set via fallback ParseFloat
					assert.InDelta(t, tt.expectedValue, spec.quality, 0.001, "expected quality")
					assert.NotEmpty(t, spec.rawQuality, "rawQuality should be set")
				} else {
					// Quality should remain unchanged (fallback parsed but value out of range)
					assert.InDelta(t, tt.expectedValue, spec.quality, 0.001, "expected quality to remain unchanged")
				}
			})
		}
	})

	t.Run("valid_parameters_parsed_correctly", func(t *testing.T) {
		tests := []struct {
			name               string
			param              string
			expectQParam       bool
			expectedQValue     float64
			expectOtherKey     string
			expectedOtherValue string
		}{
			{
				name:               "simple key value",
				param:              "key=value",
				expectQParam:       false,
				expectOtherKey:     "key",
				expectedOtherValue: "value",
			},
			{
				name:               "key value with spaces",
				param:              " key = value ",
				expectQParam:       false,
				expectOtherKey:     "key",
				expectedOtherValue: "value",
			},
			{
				name:           "q parameter",
				param:          "q=0.9",
				expectQParam:   true,
				expectedQValue: 0.9,
			},
			{
				name:           "q parameter with spaces",
				param:          " q = 0.8 ",
				expectQParam:   true,
				expectedQValue: 0.8,
			},
			{
				name:               "quoted value",
				param:              "charset=\"utf-8\"",
				expectQParam:       false,
				expectOtherKey:     "charset",
				expectedOtherValue: "utf-8",
			},
			{
				name:               "version parameter",
				param:              "version=1",
				expectQParam:       false,
				expectOtherKey:     "version",
				expectedOtherValue: "1",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				spec := acceptSpec{
					quality: 1.0,
					params:  nil,
				}

				parseAcceptParam(tt.param, &spec)

				if tt.expectQParam {
					assert.Equal(t, tt.expectedQValue, spec.quality, "q parameter: expected quality") //nolint:testifylint // exact quality comparison
					assert.NotEmpty(t, spec.rawQuality, "q parameter: rawQuality should be set")
				} else {
					require.NotNil(t, spec.params, "params map should be initialized")
					val, ok := spec.params[tt.expectOtherKey]
					require.True(t, ok, "expected key %q not found in params", tt.expectOtherKey)
					assert.Equal(t, tt.expectedOtherValue, val, "expected value for key %q", tt.expectOtherKey)
				}
			})
		}
	})
}

// TestSplitMediaType tests splitMediaType function, covering semicolon handling and fallback cases.
//
//nolint:paralleltest // Some subtests share state
func TestSplitMediaType(t *testing.T) {
	t.Parallel()

	t.Run("with_semicolon_parameters", func(t *testing.T) {
		t.Parallel()
		// Test finding semicolon and breaking
		// Test trimming mediaType at semicolon
		tests := []struct {
			name            string
			mediaType       string
			expectedType    string
			expectedSubtype string
		}{
			{
				name:            "single parameter",
				mediaType:       "application/json;charset=utf-8",
				expectedType:    "application",
				expectedSubtype: "json",
			},
			{
				name:            "multiple parameters",
				mediaType:       "text/html;level=1;charset=utf-8",
				expectedType:    "text",
				expectedSubtype: "html",
			},
			{
				name:            "parameter with spaces",
				mediaType:       "application/json ; charset=utf-8",
				expectedType:    "application",
				expectedSubtype: "json",
			},
			{
				name:            "parameter at start",
				mediaType:       ";charset=utf-8",
				expectedType:    "",
				expectedSubtype: "*",
			},
			{
				name:            "semicolon with no value",
				mediaType:       "application/json;",
				expectedType:    "application",
				expectedSubtype: "json",
			},
			{
				name:            "multiple semicolons",
				mediaType:       "application/json;version=1;charset=utf-8;boundary=xyz",
				expectedType:    "application",
				expectedSubtype: "json",
			},
			{
				name:            "semicolon in subtype",
				mediaType:       "text/html;charset=utf-8",
				expectedType:    "text",
				expectedSubtype: "html",
			},
			{
				name:            "with whitespace around semicolon",
				mediaType:       "application/json ; charset=utf-8",
				expectedType:    "application",
				expectedSubtype: "json",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				typeStr, subtypeStr := splitMediaType(tt.mediaType)
				assert.Equal(t, tt.expectedType, typeStr, "splitMediaType(%q).type", tt.mediaType)
				assert.Equal(t, tt.expectedSubtype, subtypeStr, "splitMediaType(%q).subtype", tt.mediaType)
			})
		}
	})

	t.Run("no_slash_fallback", func(t *testing.T) {
		// Test returning when no slash is found (fallback to "*")
		tests := []struct {
			name            string
			mediaType       string
			expectedType    string
			expectedSubtype string
		}{
			{
				name:            "single word no slash",
				mediaType:       "json",
				expectedType:    "json",
				expectedSubtype: "*",
			},
			{
				name:            "single word with parameters",
				mediaType:       "json;version=1",
				expectedType:    "json",
				expectedSubtype: "*",
			},
			{
				name:            "empty string",
				mediaType:       "",
				expectedType:    "",
				expectedSubtype: "*",
			},
			{
				name:            "whitespace only",
				mediaType:       "   ",
				expectedType:    "",
				expectedSubtype: "*",
			},
			{
				name:            "type only with whitespace",
				mediaType:       "  application  ",
				expectedType:    "application",
				expectedSubtype: "*",
			},
			{
				name:            "type with parameters no slash",
				mediaType:       "json;charset=utf-8",
				expectedType:    "json",
				expectedSubtype: "*",
			},
			{
				name:            "uppercase type no slash",
				mediaType:       "APPLICATION",
				expectedType:    "application",
				expectedSubtype: "*",
			},
			{
				name:            "mixed case no slash",
				mediaType:       "ApPlIcAtIoN",
				expectedType:    "application",
				expectedSubtype: "*",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				typeStr, subtypeStr := splitMediaType(tt.mediaType)
				assert.Equal(t, tt.expectedType, typeStr, "splitMediaType(%q).type", tt.mediaType)
				assert.Equal(t, tt.expectedSubtype, subtypeStr, "splitMediaType(%q).subtype (should be \"*\" when no slash)", tt.mediaType)
			})
		}
	})

	t.Run("valid_media_types", func(t *testing.T) {
		// Test normal cases to ensure function works correctly overall
		tests := []struct {
			name            string
			mediaType       string
			expectedType    string
			expectedSubtype string
		}{
			{
				name:            "simple media type",
				mediaType:       "application/json",
				expectedType:    "application",
				expectedSubtype: "json",
			},
			{
				name:            "uppercase media type",
				mediaType:       "APPLICATION/JSON",
				expectedType:    "application",
				expectedSubtype: "json",
			},
			{
				name:            "mixed case media type",
				mediaType:       "Application/Json",
				expectedType:    "application",
				expectedSubtype: "json",
			},
			{
				name:            "media type with whitespace",
				mediaType:       "  application  /  json  ",
				expectedType:    "application  ", // Function only trims outer whitespace, not internal
				expectedSubtype: "  json",
			},
			{
				name:            "media type with plus in subtype",
				mediaType:       "application/vnd.api+json",
				expectedType:    "application",
				expectedSubtype: "vnd.api+json",
			},
			{
				name:            "wildcard type",
				mediaType:       "*/*",
				expectedType:    "*",
				expectedSubtype: "*",
			},
			{
				name:            "type wildcard",
				mediaType:       "text/*",
				expectedType:    "text",
				expectedSubtype: "*",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				typeStr, subtypeStr := splitMediaType(tt.mediaType)
				assert.Equal(t, tt.expectedType, typeStr, "splitMediaType(%q).type", tt.mediaType)
				assert.Equal(t, tt.expectedSubtype, subtypeStr, "splitMediaType(%q).subtype", tt.mediaType)
			})
		}
	})

	t.Run("media_type_with_parameters_and_slash", func(t *testing.T) {
		// Combined test: semicolon handling + normal split
		tests := []struct {
			name            string
			mediaType       string
			expectedType    string
			expectedSubtype string
		}{
			{
				name:            "media type with single parameter",
				mediaType:       "application/json;charset=utf-8",
				expectedType:    "application",
				expectedSubtype: "json",
			},
			{
				name:            "media type with multiple parameters",
				mediaType:       "text/html;level=1;charset=utf-8",
				expectedType:    "text",
				expectedSubtype: "html",
			},
			{
				name:            "wildcard with parameters",
				mediaType:       "*/*;q=0.8",
				expectedType:    "*",
				expectedSubtype: "*",
			},
			{
				name:            "type wildcard with parameters",
				mediaType:       "text/*;q=0.9",
				expectedType:    "text",
				expectedSubtype: "*",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				typeStr, subtypeStr := splitMediaType(tt.mediaType)
				assert.Equal(t, tt.expectedType, typeStr, "splitMediaType(%q).type", tt.mediaType)
				assert.Equal(t, tt.expectedSubtype, subtypeStr, "splitMediaType(%q).subtype", tt.mediaType)
			})
		}
	})
}

// TestNormalizeMediaType tests normalizeMediaType function, covering fallback cases.
//
//nolint:paralleltest // Some subtests share state
func TestNormalizeMediaType(t *testing.T) {
	t.Parallel()

	t.Run("unknown_short_name_fallback", func(t *testing.T) {
		t.Parallel()
		// Test returning unknown short names as-is when not in map and no slash
		tests := []struct {
			name        string
			mediaType   string
			expected    string
			description string
		}{
			{
				name:        "unknown single word",
				mediaType:   "unknown",
				expected:    "unknown",
				description: "should return unknown short name as-is",
			},
			{
				name:        "unknown word with uppercase",
				mediaType:   "UNKNOWN",
				expected:    "unknown", // ToLower converts it
				description: "should return lowercase unknown name",
			},
			{
				name:        "unknown word with mixed case",
				mediaType:   "UnKnOwN",
				expected:    "unknown",
				description: "should return lowercase unknown name",
			},
			{
				name:        "unknown word with whitespace",
				mediaType:   "  unknown  ",
				expected:    "unknown", // TrimSpace removes whitespace
				description: "should trim whitespace and return unknown name",
			},
			{
				name:        "empty string",
				mediaType:   "",
				expected:    "",
				description: "should return empty string",
			},
			{
				name:        "whitespace only",
				mediaType:   "   ",
				expected:    "",
				description: "should return empty after trimming",
			},
			{
				name:        "random text",
				mediaType:   "someRandomText",
				expected:    "somerandomtext",
				description: "should return lowercase unknown text",
			},
			{
				name:        "numeric only",
				mediaType:   "123",
				expected:    "123",
				description: "should return numeric as-is",
			},
			{
				name:        "special characters",
				mediaType:   "test-name_123",
				expected:    "test-name_123",
				description: "should return with special chars as-is",
			},
			{
				name:        "single character",
				mediaType:   "x",
				expected:    "x",
				description: "should return single char as-is",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := normalizeMediaType(tt.mediaType)
				assert.Equal(t, tt.expected, result, "normalizeMediaType(%q)\nDescription: %s", tt.mediaType, tt.description)
			})
		}
	})

	t.Run("known_short_names", func(t *testing.T) {
		// Test that known short names are converted
		tests := []struct {
			name      string
			mediaType string
			expected  string
		}{
			{"html", "html", "text/html"},
			{"json", "json", "application/json"},
			{"xml", "xml", "application/xml"},
			{"text", "text", "text/plain"},
			{"txt", "txt", "text/plain"},
			{"png", "png", "image/png"},
			{"jpg", "jpg", "image/jpeg"},
			{"jpeg", "jpeg", "image/jpeg"},
			{"gif", "gif", "image/gif"},
			{"uppercase html", "HTML", "text/html"},
			{"mixed case json", "JSON", "application/json"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := normalizeMediaType(tt.mediaType)
				assert.Equal(t, tt.expected, result, "normalizeMediaType(%q)", tt.mediaType)
			})
		}
	})

	t.Run("mime_types_with_slash", func(t *testing.T) {
		// Test returning MIME types with "/" as-is
		tests := []struct {
			name      string
			mediaType string
			expected  string
		}{
			{
				name:      "full mime type",
				mediaType: "application/json",
				expected:  "application/json",
			},
			{
				name:      "mime type with parameters",
				mediaType: "application/json;charset=utf-8",
				expected:  "application/json;charset=utf-8",
			},
			{
				name:      "wildcard",
				mediaType: "*/*",
				expected:  "*/*",
			},
			{
				name:      "type wildcard",
				mediaType: "text/*",
				expected:  "text/*",
			},
			{
				name:      "with whitespace",
				mediaType: "  application/json  ",
				expected:  "application/json",
			},
			{
				name:      "uppercase mime type",
				mediaType: "APPLICATION/JSON",
				expected:  "application/json",
			},
			{
				name:      "custom mime type",
				mediaType: "application/vnd.api+json",
				expected:  "application/vnd.api+json",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := normalizeMediaType(tt.mediaType)
				assert.Equal(t, tt.expected, result, "normalizeMediaType(%q)", tt.mediaType)
			})
		}
	})

	t.Run("edge_cases_covering_all_paths", func(t *testing.T) {
		// Test all code paths
		tests := []struct {
			name      string
			mediaType string
			expected  string
			path      string // Which code path this tests
		}{
			{
				name:      "known short name",
				mediaType: "html",
				expected:  "text/html",
				path:      "known in map",
			},
			{
				name:      "mime type with slash",
				mediaType: "application/custom",
				expected:  "application/custom",
				path:      "contains slash",
			},
			{
				name:      "unknown short name",
				mediaType: "custom",
				expected:  "custom",
				path:      "unknown short name fallback",
			},
			{
				name:      "unknown with whitespace",
				mediaType: "  custom  ",
				expected:  "custom",
				path:      "unknown with whitespace fallback",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := normalizeMediaType(tt.mediaType)
				assert.Equal(t, tt.expected, result, "normalizeMediaType(%q)\nPath: %s", tt.mediaType, tt.path)
			})
		}
	})
}

// TestAcceptHeaderMatch tests acceptHeaderMatch function, covering empty offers and specs cases.
//
//nolint:paralleltest // Some subtests share state
func TestAcceptHeaderMatch(t *testing.T) {
	t.Parallel()

	t.Run("empty_offers_returns_empty", func(t *testing.T) {
		t.Parallel()
		// Test returning empty string when offers slice is empty
		tests := []struct {
			name     string
			specs    []acceptSpec
			offers   []string
			expected string
		}{
			{
				name:     "empty offers with empty specs",
				specs:    nil,
				offers:   []string{},
				expected: "",
			},
			{
				name:     "empty offers with nil specs",
				specs:    nil,
				offers:   nil,
				expected: "",
			},
			{
				name: "empty offers with non-empty specs",
				specs: []acceptSpec{
					{value: "utf-8", quality: 1.0},
					{value: "iso-8859-1", quality: 0.8},
				},
				offers:   []string{},
				expected: "",
			},
			{
				name: "nil offers slice",
				specs: []acceptSpec{
					{value: "gzip", quality: 1.0},
				},
				offers:   nil,
				expected: "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := acceptHeaderMatch(tt.specs, tt.offers)
				assert.Equal(t, tt.expected, result, "acceptHeaderMatch(specs, offers)\nSpecs: %v\nOffers: %v", tt.specs, tt.offers)
			})
		}
	})

	t.Run("empty_specs_returns_first_offer", func(t *testing.T) {
		// Test returning first offer when specs are empty
		tests := []struct {
			name     string
			specs    []acceptSpec
			offers   []string
			expected string
		}{
			{
				name:     "nil specs",
				specs:    nil,
				offers:   []string{"utf-8", "iso-8859-1"},
				expected: "utf-8",
			},
			{
				name:     "empty specs slice",
				specs:    []acceptSpec{},
				offers:   []string{"gzip", "deflate"},
				expected: "gzip",
			},
			{
				name:     "single offer",
				specs:    nil,
				offers:   []string{"en"},
				expected: "en",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := acceptHeaderMatch(tt.specs, tt.offers)
				assert.Equal(t, tt.expected, result, "acceptHeaderMatch(specs, offers)")
			})
		}
	})

	t.Run("quality_based_matching", func(t *testing.T) {
		// Test normal matching behavior
		tests := []struct {
			name     string
			specs    []acceptSpec
			offers   []string
			expected string
		}{
			{
				name: "exact match",
				specs: []acceptSpec{
					{value: "utf-8", quality: 1.0},
				},
				offers:   []string{"utf-8", "iso-8859-1"},
				expected: "utf-8",
			},
			{
				name: "higher quality wins",
				specs: []acceptSpec{
					{value: "utf-8", quality: 0.8},
					{value: "iso-8859-1", quality: 1.0},
				},
				offers:   []string{"utf-8", "iso-8859-1"},
				expected: "iso-8859-1",
			},
			{
				name: "wildcard match",
				specs: []acceptSpec{
					{value: "*", quality: 0.8},
				},
				offers:   []string{"utf-8", "gzip"},
				expected: "utf-8",
			},
			{
				name: "language prefix match",
				specs: []acceptSpec{
					{value: "en-US", quality: 1.0},
				},
				offers:   []string{"en", "fr"},
				expected: "en",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := acceptHeaderMatch(tt.specs, tt.offers)
				assert.Equal(t, tt.expected, result, "acceptHeaderMatch(specs, offers)")
			})
		}
	})

	t.Run("no_match_returns_empty", func(t *testing.T) {
		// Test when no offers match any specs
		tests := []struct {
			name     string
			specs    []acceptSpec
			offers   []string
			expected string
		}{
			{
				name: "no matching offers",
				specs: []acceptSpec{
					{value: "utf-8", quality: 1.0},
				},
				offers:   []string{"iso-8859-1", "ascii"},
				expected: "",
			},
			{
				name: "different values",
				specs: []acceptSpec{
					{value: "gzip", quality: 1.0},
				},
				offers:   []string{"deflate", "br"},
				expected: "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := acceptHeaderMatch(tt.specs, tt.offers)
				assert.Equal(t, tt.expected, result, "acceptHeaderMatch(specs, offers)")
			})
		}
	})
}

// BenchmarkParseQuality benchmarks the quality value parser
func BenchmarkParseQuality(b *testing.B) {
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
			for b.Loop() {
				_ = parseQuality(tc.input)
			}
		})
	}
}

// BenchmarkParseQualityVsParseFloat compares parseQuality to strconv.ParseFloat
func BenchmarkParseQualityVsParseFloat(b *testing.B) {
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
			for b.Loop() {
				//nolint:errcheck // Benchmark measures performance; error checking would skew results
				parseQuality(tc.input)
			}
		})

		b.Run("stdlib_"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				//nolint:errcheck // Benchmark measures performance; error checking would skew results
				strconv.ParseFloat(tc.input, 64)
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
			arena, ok := arenaPool.Get().(*headerArena)
			if !ok {
				b.Fatal("arenaPool.Get() returned non-*headerArena type")
			}
			defer func() {
				arena.reset()
				arenaPool.Put(arena)
			}()

			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				_ = parseAccept(header, arena)
				arena.used = 0 // Reset arena for next iteration
			}
		})
	}
}
