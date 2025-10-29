package router

import (
	"net/http/httptest"
	"testing"
)

// BenchmarkAcceptsOptimized benchmarks the optimized Accept header parsing
func BenchmarkAcceptsOptimized(b *testing.B) {
	tests := []struct {
		name         string
		acceptHeader string
		offers       []string
	}{
		{
			name:         "simple",
			acceptHeader: "application/json",
			offers:       []string{"json", "xml", "html"},
		},
		{
			name:         "with_quality",
			acceptHeader: "text/html, application/json;q=0.9, */*;q=0.8",
			offers:       []string{"json", "html", "xml"},
		},
		{
			name:         "complex_browser",
			acceptHeader: "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
			offers:       []string{"html", "json", "xml"},
		},
		{
			name:         "with_parameters",
			acceptHeader: "application/json;version=1;charset=utf-8, text/html;q=0.9",
			offers:       []string{"json", "html"},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Accept", tt.acceptHeader)
			w := httptest.NewRecorder()
			ctx := NewContext(w, req)

			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				ctx.Accepts(tt.offers...)
			}
		})
	}
}

// BenchmarkAcceptsCaching benchmarks the per-request caching benefit
func BenchmarkAcceptsCaching(b *testing.B) {
	acceptHeader := "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"
	offers := []string{"html", "json", "xml"}

	b.Run("with_cache", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept", acceptHeader)
		w := httptest.NewRecorder()
		ctx := NewContext(w, req)

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			// Multiple calls in same request (cache hit)
			ctx.Accepts(offers...)
			ctx.Accepts(offers...)
			ctx.Accepts(offers...)
		}
	})

	b.Run("without_cache_simulation", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept", acceptHeader)
		w := httptest.NewRecorder()

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			// Simulate no caching by creating new context each time
			ctx := NewContext(w, req)
			ctx.Accepts(offers...)
			ctx = NewContext(w, req)
			ctx.Accepts(offers...)
			ctx = NewContext(w, req)
			ctx.Accepts(offers...)
		}
	})
}

// BenchmarkParseAcceptFast benchmarks the core parsing function with arena
func BenchmarkParseAcceptFast(b *testing.B) {
	tests := []struct {
		name   string
		header string
	}{
		{"simple", "application/json"},
		{"with_quality", "text/html, application/json;q=0.9"},
		{"complex_browser", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"},
		{"with_params", "application/json;version=1;charset=utf-8, text/html;q=0.9, text/plain;q=0.8"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Get arena from pool
			arena := arenaPool.Get().(*headerArena)
			defer func() {
				arena.reset()
				arenaPool.Put(arena)
			}()

			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				_ = parseAcceptFast(tt.header, arena)
				arena.used = 0 // Reset for next iteration
			}
		})
	}
}

// BenchmarkAcceptsCharsets benchmarks charset negotiation
func BenchmarkAcceptsCharsets(b *testing.B) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Charset", "utf-8, iso-8859-1;q=0.5, *;q=0.1")
	w := httptest.NewRecorder()
	ctx := NewContext(w, req)

	offers := []string{"utf-8", "iso-8859-1", "ascii"}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		ctx.AcceptsCharsets(offers...)
	}
}

// BenchmarkAcceptsEncodingsOptimized benchmarks encoding negotiation
func BenchmarkAcceptsEncodingsOptimized(b *testing.B) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate, br;q=1.0, *;q=0.5")
	w := httptest.NewRecorder()
	ctx := NewContext(w, req)

	offers := []string{"gzip", "br", "deflate"}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		ctx.AcceptsEncodings(offers...)
	}
}

// BenchmarkAcceptsLanguages benchmarks language negotiation
func BenchmarkAcceptsLanguages(b *testing.B) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Language", "en-US, en;q=0.9, fr;q=0.8, de;q=0.7")
	w := httptest.NewRecorder()
	ctx := NewContext(w, req)

	offers := []string{"en", "fr", "de", "es"}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		ctx.AcceptsLanguages(offers...)
	}
}

// BenchmarkTrimWhitespace benchmarks the manual whitespace trimming
func BenchmarkTrimWhitespace(b *testing.B) {
	testStrings := []string{
		"  application/json  ",
		"text/html",
		"   ",
		"image/png; charset=utf-8",
	}

	b.ReportAllocs()
	for b.Loop() {
		for _, s := range testStrings {
			trimWhitespace(s)
		}
	}
}
