package router

import (
	"net/http/httptest"
	"testing"
)

// BenchmarkAcceptsComparison provides a side-by-side comparison of the optimization impact.
// This benchmark helps visualize the dramatic improvement from manual scanning + arena allocation.
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
