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

	"github.com/stretchr/testify/assert"
)

func TestContextPool_Stats(t *testing.T) {
	t.Parallel()

	r := MustNew()
	pool := r.contextPool

	// Reset stats to start fresh
	pool.ResetStats()

	// Verify initial state
	stats := pool.Stats()
	assert.Equal(t, uint64(0), stats.TotalGets, "Expected TotalGets=0")
	assert.Equal(t, uint64(0), stats.TotalPuts, "Expected TotalPuts=0")

	// Get and put some contexts
	for range 10 {
		ctx := pool.Get(int32(2)) // Small pool (≤4 params)
		pool.Put(ctx)
	}

	stats = pool.Stats()

	// Check totals
	assert.Equal(t, uint64(10), stats.TotalGets, "Expected TotalGets=10")
	assert.Equal(t, uint64(10), stats.TotalPuts, "Expected TotalPuts=10")

	// Check hit rate (should be ~1.0 when Gets=Puts)
	assert.GreaterOrEqual(t, stats.HitRate, 0.99, "Expected HitRate >= 0.99")
	assert.LessOrEqual(t, stats.HitRate, 1.01, "Expected HitRate <= 1.01")

	// Check small pool usage (all 10 should be in small pool)
	assert.Equal(t, uint64(10), stats.SmallHits, "Expected SmallHits=10")

	// Check percentage
	assert.GreaterOrEqual(t, stats.SmallPct, 99.0, "Expected SmallPct >= 99.0%%")
	assert.LessOrEqual(t, stats.SmallPct, 101.0, "Expected SmallPct <= 101.0%%")
}

func TestContextPool_Stats_Distribution(t *testing.T) {
	t.Parallel()

	r := MustNew()
	pool := r.contextPool

	pool.ResetStats()

	// Get contexts from different pools
	// Small pool: 10 contexts (≤4 params)
	for range 10 {
		ctx := pool.Get(int32(2))
		pool.Put(ctx)
	}

	// Medium pool: 5 contexts (5-8 params)
	for range 5 {
		ctx := pool.Get(int32(6))
		pool.Put(ctx)
	}

	// Large pool: 2 contexts (>8 params)
	for range 2 {
		ctx := pool.Get(int32(10))
		pool.Put(ctx)
	}

	stats := pool.Stats()

	// Check distribution
	assert.Equal(t, uint64(10), stats.SmallHits, "Expected SmallHits=10")
	assert.Equal(t, uint64(5), stats.MediumHits, "Expected MediumHits=5")
	assert.Equal(t, uint64(2), stats.LargeHits, "Expected LargeHits=2")

	// Check totals
	assert.Equal(t, uint64(17), stats.TotalGets, "Expected TotalGets=17")
	assert.Equal(t, uint64(17), stats.TotalPuts, "Expected TotalPuts=17")

	// Check percentages
	expectedSmallPct := (10.0 / 17.0) * 100
	assert.GreaterOrEqual(t, stats.SmallPct, expectedSmallPct-1, "Expected SmallPct >= %.1f%%", expectedSmallPct-1)
	assert.LessOrEqual(t, stats.SmallPct, expectedSmallPct+1, "Expected SmallPct <= %.1f%%", expectedSmallPct+1)

	expectedMediumPct := (5.0 / 17.0) * 100
	assert.GreaterOrEqual(t, stats.MediumPct, expectedMediumPct-1, "Expected MediumPct >= %.1f%%", expectedMediumPct-1)
	assert.LessOrEqual(t, stats.MediumPct, expectedMediumPct+1, "Expected MediumPct <= %.1f%%", expectedMediumPct+1)

	expectedLargePct := (2.0 / 17.0) * 100
	assert.GreaterOrEqual(t, stats.LargePct, expectedLargePct-1, "Expected LargePct >= %.1f%%", expectedLargePct-1)
	assert.LessOrEqual(t, stats.LargePct, expectedLargePct+1, "Expected LargePct <= %.1f%%", expectedLargePct+1)

	t.Logf("Pool distribution: Small=%.1f%%, Medium=%.1f%%, Large=%.1f%%",
		stats.SmallPct, stats.MediumPct, stats.LargePct)
}

func TestContextPool_ResetStats(t *testing.T) {
	t.Parallel()

	r := MustNew()
	pool := r.contextPool

	// Generate some activity
	for range 5 {
		ctx := pool.Get(int32(2))
		pool.Put(ctx)
	}

	// Verify stats were recorded
	stats := pool.Stats()
	assert.NotEqual(t, int64(0), stats.TotalGets, "Expected some activity before reset")

	// Reset stats
	pool.ResetStats()

	// Verify all counters are zero
	stats = pool.Stats()
	assert.Equal(t, uint64(0), stats.SmallHits, "After reset: expected SmallHits=0")
	assert.Equal(t, uint64(0), stats.MediumHits, "After reset: expected MediumHits=0")
	assert.Equal(t, uint64(0), stats.LargeHits, "After reset: expected LargeHits=0")
	assert.Equal(t, uint64(0), stats.TotalGets, "After reset: expected TotalGets=0")
	assert.Equal(t, uint64(0), stats.TotalPuts, "After reset: expected TotalPuts=0")
	assert.Equal(t, 0.0, stats.HitRate, "After reset: expected HitRate=0")
}

func TestContextPool_Stats_HitRate(t *testing.T) {
	t.Parallel()

	r := MustNew()
	pool := r.contextPool

	pool.ResetStats()

	// Get 10, Put 8 (simulate 2 contexts not returned - potential leak)
	for range 10 {
		pool.Get(int32(2))
	}
	for range 8 {
		ctx := &Context{paramCount: 2}
		pool.Put(ctx)
	}

	stats := pool.Stats()

	// Hit rate should be 8/10 = 0.8
	expectedHitRate := 8.0 / 10.0
	assert.GreaterOrEqual(t, stats.HitRate, expectedHitRate-0.01, "Expected HitRate >= %.2f", expectedHitRate-0.01)
	assert.LessOrEqual(t, stats.HitRate, expectedHitRate+0.01, "Expected HitRate <= %.2f", expectedHitRate+0.01)

	assert.Less(t, stats.HitRate, 0.95, "Expected low hit rate (<0.95) to indicate potential context leak")

	t.Logf("Hit rate with simulated leak: %.2f (%.0f%%)", stats.HitRate, stats.HitRate*100)
}

func TestContextPool_Stats_ZeroGets(t *testing.T) {
	t.Parallel()

	r := MustNew()
	pool := r.contextPool

	pool.ResetStats()

	// Get stats without any activity
	stats := pool.Stats()

	// All metrics should be zero
	assert.Equal(t, 0.0, stats.HitRate, "Expected HitRate=0 with no activity")
	assert.Equal(t, 0.0, stats.SmallPct, "Expected SmallPct=0 with no activity")
	assert.Equal(t, 0.0, stats.MediumPct, "Expected MediumPct=0 with no activity")
	assert.Equal(t, 0.0, stats.LargePct, "Expected LargePct=0 with no activity")
}

func TestContextPool_Stats_Concurrent(t *testing.T) {
	t.Parallel()

	r := MustNew()
	pool := r.contextPool

	pool.ResetStats()

	// Concurrent Get/Put operations
	done := make(chan struct{})
	for range 10 {
		go func() {
			for range 100 {
				ctx := pool.Get(int32(2))
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
	assert.Equal(t, uint64(1000), stats.TotalGets, "Expected TotalGets=1000")
	assert.Equal(t, uint64(1000), stats.TotalPuts, "Expected TotalPuts=1000")

	// Hit rate should be 1.0
	assert.GreaterOrEqual(t, stats.HitRate, 0.99, "Expected HitRate >= 0.99")
	assert.LessOrEqual(t, stats.HitRate, 1.01, "Expected HitRate <= 1.01")

	t.Logf("Concurrent stats: Gets=%d, Puts=%d, HitRate=%.2f",
		stats.TotalGets, stats.TotalPuts, stats.HitRate)
}

func BenchmarkContextPool_Stats(b *testing.B) {
	r := MustNew()
	pool := r.contextPool

	pool.ResetStats()

	// Pre-warm the pool
	for range 100 {
		ctx := pool.Get(int32(2))
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
		ctx := pool.Get(int32(2))
		pool.Put(ctx)
	}
}
