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
	"testing"
)

func TestContextPool_Stats(t *testing.T) {
r := MustNew()
	pool := r.contextPool

	// Reset stats to start fresh
	pool.ResetStats()

	// Verify initial state
	stats := pool.Stats()
	if stats.TotalGets != 0 {
		t.Errorf("Expected TotalGets=0, got %d", stats.TotalGets)
	}
	if stats.TotalPuts != 0 {
		t.Errorf("Expected TotalPuts=0, got %d", stats.TotalPuts)
	}

	// Get and put some contexts
	for range 10 {
		ctx := pool.Get(2) // Small pool (≤4 params)
		pool.Put(ctx)
	}

	stats = pool.Stats()

	// Check totals
	if stats.TotalGets != 10 {
		t.Errorf("Expected TotalGets=10, got %d", stats.TotalGets)
	}
	if stats.TotalPuts != 10 {
		t.Errorf("Expected TotalPuts=10, got %d", stats.TotalPuts)
	}

	// Check hit rate (should be ~1.0 when Gets=Puts)
	if stats.HitRate < 0.99 || stats.HitRate > 1.01 {
		t.Errorf("Expected HitRate~1.0, got %.2f", stats.HitRate)
	}

	// Check small pool usage (all 10 should be in small pool)
	if stats.SmallHits != 10 {
		t.Errorf("Expected SmallHits=10, got %d", stats.SmallHits)
	}

	// Check percentage
	if stats.SmallPct < 99.0 || stats.SmallPct > 101.0 {
		t.Errorf("Expected SmallPct~100%%, got %.1f%%", stats.SmallPct)
	}
}

func TestContextPool_Stats_Distribution(t *testing.T) {
r := MustNew()
	pool := r.contextPool

	pool.ResetStats()

	// Get contexts from different pools
	// Small pool: 10 contexts (≤4 params)
	for range 10 {
		ctx := pool.Get(2)
		pool.Put(ctx)
	}

	// Medium pool: 5 contexts (5-8 params)
	for range 5 {
		ctx := pool.Get(6)
		pool.Put(ctx)
	}

	// Large pool: 2 contexts (>8 params)
	for range 2 {
		ctx := pool.Get(10)
		pool.Put(ctx)
	}

	stats := pool.Stats()

	// Check distribution
	if stats.SmallHits != 10 {
		t.Errorf("Expected SmallHits=10, got %d", stats.SmallHits)
	}
	if stats.MediumHits != 5 {
		t.Errorf("Expected MediumHits=5, got %d", stats.MediumHits)
	}
	if stats.LargeHits != 2 {
		t.Errorf("Expected LargeHits=2, got %d", stats.LargeHits)
	}

	// Check totals
	if stats.TotalGets != 17 {
		t.Errorf("Expected TotalGets=17, got %d", stats.TotalGets)
	}
	if stats.TotalPuts != 17 {
		t.Errorf("Expected TotalPuts=17, got %d", stats.TotalPuts)
	}

	// Check percentages
	expectedSmallPct := (10.0 / 17.0) * 100
	if stats.SmallPct < expectedSmallPct-1 || stats.SmallPct > expectedSmallPct+1 {
		t.Errorf("Expected SmallPct~%.1f%%, got %.1f%%", expectedSmallPct, stats.SmallPct)
	}

	expectedMediumPct := (5.0 / 17.0) * 100
	if stats.MediumPct < expectedMediumPct-1 || stats.MediumPct > expectedMediumPct+1 {
		t.Errorf("Expected MediumPct~%.1f%%, got %.1f%%", expectedMediumPct, stats.MediumPct)
	}

	expectedLargePct := (2.0 / 17.0) * 100
	if stats.LargePct < expectedLargePct-1 || stats.LargePct > expectedLargePct+1 {
		t.Errorf("Expected LargePct~%.1f%%, got %.1f%%", expectedLargePct, stats.LargePct)
	}

	t.Logf("Pool distribution: Small=%.1f%%, Medium=%.1f%%, Large=%.1f%%",
		stats.SmallPct, stats.MediumPct, stats.LargePct)
}

func TestContextPool_ResetStats(t *testing.T) {
r := MustNew()
	pool := r.contextPool

	// Generate some activity
	for range 5 {
		ctx := pool.Get(2)
		pool.Put(ctx)
	}

	// Verify stats were recorded
	stats := pool.Stats()
	if stats.TotalGets == 0 {
		t.Error("Expected some activity before reset")
	}

	// Reset stats
	pool.ResetStats()

	// Verify all counters are zero
	stats = pool.Stats()
	if stats.SmallHits != 0 {
		t.Errorf("After reset: expected SmallHits=0, got %d", stats.SmallHits)
	}
	if stats.MediumHits != 0 {
		t.Errorf("After reset: expected MediumHits=0, got %d", stats.MediumHits)
	}
	if stats.LargeHits != 0 {
		t.Errorf("After reset: expected LargeHits=0, got %d", stats.LargeHits)
	}
	if stats.TotalGets != 0 {
		t.Errorf("After reset: expected TotalGets=0, got %d", stats.TotalGets)
	}
	if stats.TotalPuts != 0 {
		t.Errorf("After reset: expected TotalPuts=0, got %d", stats.TotalPuts)
	}
	if stats.HitRate != 0 {
		t.Errorf("After reset: expected HitRate=0, got %.2f", stats.HitRate)
	}
}

func TestContextPool_Stats_HitRate(t *testing.T) {
r := MustNew()
	pool := r.contextPool

	pool.ResetStats()

	// Get 10, Put 8 (simulate 2 contexts not returned - potential leak)
	for range 10 {
		pool.Get(2)
	}
	for range 8 {
		ctx := &Context{paramCount: 2}
		pool.Put(ctx)
	}

	stats := pool.Stats()

	// Hit rate should be 8/10 = 0.8
	expectedHitRate := 8.0 / 10.0
	if stats.HitRate < expectedHitRate-0.01 || stats.HitRate > expectedHitRate+0.01 {
		t.Errorf("Expected HitRate~%.2f, got %.2f", expectedHitRate, stats.HitRate)
	}

	if stats.HitRate >= 0.95 {
		t.Error("Expected low hit rate (<0.95) to indicate potential context leak")
	}

	t.Logf("Hit rate with simulated leak: %.2f (%.0f%%)", stats.HitRate, stats.HitRate*100)
}

func TestContextPool_Stats_ZeroGets(t *testing.T) {
r := MustNew()
	pool := r.contextPool

	pool.ResetStats()

	// Get stats without any activity
	stats := pool.Stats()

	// All metrics should be zero
	if stats.HitRate != 0 {
		t.Errorf("Expected HitRate=0 with no activity, got %.2f", stats.HitRate)
	}
	if stats.SmallPct != 0 {
		t.Errorf("Expected SmallPct=0 with no activity, got %.1f", stats.SmallPct)
	}
	if stats.MediumPct != 0 {
		t.Errorf("Expected MediumPct=0 with no activity, got %.1f", stats.MediumPct)
	}
	if stats.LargePct != 0 {
		t.Errorf("Expected LargePct=0 with no activity, got %.1f", stats.LargePct)
	}
}

func TestContextPool_Stats_Concurrent(t *testing.T) {
r := MustNew()
	pool := r.contextPool

	pool.ResetStats()

	// Concurrent Get/Put operations
	done := make(chan struct{})
	for range 10 {
		go func() {
			for range 100 {
				ctx := pool.Get(2)
				pool.Put(ctx)
			}
			done <- struct{}{}
		}()
	}

	// Wait for all goroutines
	for range 10 {
		<-done
	}

	stats := pool.Stats()

	// Should have 1000 total operations (10 goroutines * 100 iterations)
	if stats.TotalGets != 1000 {
		t.Errorf("Expected TotalGets=1000, got %d", stats.TotalGets)
	}
	if stats.TotalPuts != 1000 {
		t.Errorf("Expected TotalPuts=1000, got %d", stats.TotalPuts)
	}

	// Hit rate should be 1.0
	if stats.HitRate < 0.99 || stats.HitRate > 1.01 {
		t.Errorf("Expected HitRate~1.0, got %.2f", stats.HitRate)
	}

	t.Logf("Concurrent stats: Gets=%d, Puts=%d, HitRate=%.2f",
		stats.TotalGets, stats.TotalPuts, stats.HitRate)
}

func BenchmarkContextPool_Stats(b *testing.B) {
r := MustNew()
	pool := r.contextPool

	pool.ResetStats()

	// Pre-warm the pool
	for range 100 {
		ctx := pool.Get(2)
		pool.Put(ctx)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = pool.Stats()
	}
}

func BenchmarkContextPool_GetPut_WithStats(b *testing.B) {
r := MustNew()
	pool := r.contextPool

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		ctx := pool.Get(2)
		pool.Put(ctx)
	}
}
