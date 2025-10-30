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

// ============================================================================
// Comparison Benchmarks (merged from accept_comparison_bench_test.go)
// ============================================================================

func BenchmarkAcceptsComparison(b *testing.B) {
	// Real-world Accept header from Chrome browser
	chromeHeader := "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8"

	b.Run("optimized/simple_header", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		ctx := NewContext(w, req)
		offers := []string{"json", "html", "xml"}

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			ctx.Accepts(offers...)
		}
	})

	b.Run("optimized/chrome_browser", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept", chromeHeader)
		w := httptest.NewRecorder()
		ctx := NewContext(w, req)
		offers := []string{"html", "json", "xml"}

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			ctx.Accepts(offers...)
		}
	})

	b.Run("optimized/with_caching", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept", chromeHeader)
		w := httptest.NewRecorder()
		ctx := NewContext(w, req)
		offers := []string{"html", "json", "xml"}

		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			// Call multiple times to show caching benefit
			ctx.Accepts(offers...)
			ctx.Accepts(offers...)
			ctx.Accepts(offers...)
		}
	})
}

// BenchmarkArenaPooling demonstrates the arena pool's zero-allocation characteristics
func BenchmarkArenaPooling(b *testing.B) {
	b.Run("get_and_put", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			arena := arenaPool.Get().(*headerArena)
			specs := arena.getSpecs(4)
			_ = specs
			arena.reset()
			arenaPool.Put(arena)
		}
	})

	b.Run("parse_with_arena", func(b *testing.B) {
		header := "text/html,application/xml;q=0.9,*/*;q=0.8"

		b.ReportAllocs()

		for b.Loop() {
			arena := arenaPool.Get().(*headerArena)
			specs := parseAcceptFast(header, arena)
			_ = specs
			arena.reset()
			arenaPool.Put(arena)
		}
	})
}

// BenchmarkMemoryLocality tests the impact of arena buffer size on performance
func BenchmarkMemoryLocality(b *testing.B) {
	headers := []struct {
		name   string
		header string
		count  int // Expected number of specs
	}{
		{"small_1_spec", "application/json", 1},
		{"medium_3_specs", "text/html, application/json;q=0.9, */*;q=0.8", 3},
		{"large_6_specs", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8", 6},
	}

	for _, tt := range headers {
		b.Run(tt.name, func(b *testing.B) {
			arena := arenaPool.Get().(*headerArena)
			defer func() {
				arena.reset()
				arenaPool.Put(arena)
			}()

			b.ReportAllocs()

			for b.Loop() {
				specs := parseAcceptFast(tt.header, arena)
				if len(specs) != tt.count {
					b.Fatalf("expected %d specs, got %d", tt.count, len(specs))
				}
				arena.used = 0 // Reset for next iteration
			}
		})
	}
}

// BenchmarkZeroAllocationProof explicitly verifies zero allocations for common cases
func BenchmarkZeroAllocationProof(b *testing.B) {
	testCases := []string{
		"application/json",
		"text/html",
		"*/*",
		"application/json, text/html",
		"text/html;q=1.0, application/json;q=0.9",
	}

	for _, header := range testCases {
		b.Run(header, func(b *testing.B) {
			arena := arenaPool.Get().(*headerArena)
			defer func() {
				arena.reset()
				arenaPool.Put(arena)
			}()

			b.ReportAllocs()

			for b.Loop() {
				specs := parseAcceptFast(header, arena)
				if len(specs) == 0 && header != "" {
					b.Fatal("parsing failed")
				}
				arena.used = 0
			}

			// Verify zero allocations
			if testing.AllocsPerRun(100, func() {
				parseAcceptFast(header, arena)
				arena.used = 0
			}) > 0 {
				b.Errorf("Expected zero allocations for header: %s", header)
			}
		})
	}
}
