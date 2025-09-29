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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWarmupDefault verifies default warmup behavior
func TestWarmupDefault(t *testing.T) {
	t.Parallel()

	r := MustNew()

	// Warmup with defaults
	r.contextPool.Warmup()

	// Verify pools can get contexts after warmup
	ctx := r.contextPool.Get(int32(2)) // Small pool
	require.NotNil(t, ctx, "Failed to get context from small pool after warmup")
	r.contextPool.Put(ctx)

	ctx = r.contextPool.Get(int32(6)) // Medium pool
	require.NotNil(t, ctx, "Failed to get context from medium pool after warmup")
	r.contextPool.Put(ctx)

	ctx = r.contextPool.Get(int32(10)) // Large pool
	require.NotNil(t, ctx, "Failed to get context from large pool after warmup")
	r.contextPool.Put(ctx)
}

// TestWarmupCustomConfig verifies custom warmup configuration
func TestWarmupCustomConfig(t *testing.T) {
	t.Parallel()

	r := MustNew()

	config := &WarmupConfig{
		SmallContexts:  50,
		MediumContexts: 25,
		LargeContexts:  10,
	}

	// Warmup with custom config should not panic
	r.contextPool.Warmup(config)

	// Verify pools still work
	ctx := r.contextPool.Get(int32(2))
	require.NotNil(t, ctx, "Failed to get context after custom warmup")
	r.contextPool.Put(ctx)
}

// TestWarmupZeroConfig verifies warmup handles zero values gracefully
func TestWarmupZeroConfig(t *testing.T) {
	t.Parallel()

	r := MustNew()

	config := &WarmupConfig{
		SmallContexts:  0,
		MediumContexts: 0,
		LargeContexts:  0,
	}

	// Should not panic with zero config
	r.contextPool.Warmup(config)

	// Pools should still create contexts on demand
	ctx := r.contextPool.Get(int32(2))
	require.NotNil(t, ctx, "Failed to get context with zero warmup")
	r.contextPool.Put(ctx)
}

// TestWarmupConcurrent verifies warmup is safe under concurrent access
func TestWarmupConcurrent(t *testing.T) {
	t.Parallel()

	r := MustNew()

	var wg sync.WaitGroup
	const goroutines = 10

	// Run warmup concurrently with Gets and Puts
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			r.contextPool.Warmup()

			// Also do some Gets/Puts
			ctx := r.contextPool.Get(int32(2))
			r.contextPool.Put(ctx)
		}()
	}

	wg.Wait()

	// Pool should still work normally
	ctx := r.contextPool.Get(int32(2))
	require.NotNil(t, ctx, "Pool broken after concurrent warmup")
	r.contextPool.Put(ctx)
}

// TestDefaultWarmupConfig verifies default config values
func TestDefaultWarmupConfig(t *testing.T) {
	t.Parallel()

	config := DefaultWarmupConfig()

	assert.Equal(t, 20, config.SmallContexts)
	assert.Equal(t, 10, config.MediumContexts)
	assert.Equal(t, 5, config.LargeContexts)
}

// TestWarmupMultipleTimes verifies warmup can be called multiple times
func TestWarmupMultipleTimes(t *testing.T) {
	t.Parallel()

	r := MustNew()

	// Call warmup multiple times
	for range 5 {
		r.contextPool.Warmup()
	}

	// Pool should still work
	ctx := r.contextPool.Get(int32(2))
	require.NotNil(t, ctx, "Pool broken after multiple warmups")
	r.contextPool.Put(ctx)
}

// TestWarmupNilConfig verifies nil config uses defaults
func TestWarmupNilConfig(t *testing.T) {
	t.Parallel()

	r := MustNew()

	// Passing nil should use defaults
	r.contextPool.Warmup(nil)

	// Pool should work normally
	ctx := r.contextPool.Get(int32(2))
	require.NotNil(t, ctx, "Failed to get context with nil config")
	r.contextPool.Put(ctx)
}

// BenchmarkWarmupDefault measures default warmup.
func BenchmarkWarmupDefault(b *testing.B) {
	r := MustNew()

	b.ResetTimer()
	for b.Loop() {
		r.contextPool.Warmup()
	}
}

// BenchmarkWarmupLarge measures warmup with large config
func BenchmarkWarmupLarge(b *testing.B) {
	r := MustNew()
	config := &WarmupConfig{
		SmallContexts:  100,
		MediumContexts: 50,
		LargeContexts:  25,
	}

	b.ResetTimer()
	for b.Loop() {
		r.contextPool.Warmup(config)
	}
}

// BenchmarkWarmupSequential compares parallel vs sequential warmup
// This benchmark shows the benefit of parallel warmup implementation
func BenchmarkWarmupSequential(b *testing.B) {
	r := MustNew()
	config := DefaultWarmupConfig()

	b.ResetTimer()
	for b.Loop() {
		// Sequential warmup (old approach)
		for j := 0; j < config.SmallContexts; j++ {
			ctx := r.contextPool.smallPool.Get().(*Context)
			r.contextPool.smallPool.Put(ctx)
		}
		for j := 0; j < config.MediumContexts; j++ {
			ctx := r.contextPool.mediumPool.Get().(*Context)
			r.contextPool.mediumPool.Put(ctx)
		}
		for j := 0; j < config.LargeContexts; j++ {
			ctx := r.contextPool.largePool.Get().(*Context)
			r.contextPool.largePool.Put(ctx)
		}
	}
}

// BenchmarkWarmupParallel measures parallel warmup (current implementation)
func BenchmarkWarmupParallel(b *testing.B) {
	r := MustNew()

	b.ResetTimer()
	for b.Loop() {
		r.contextPool.Warmup()
	}
}

// BenchmarkGetPutAfterWarmup measures Get/Put with warmed pool.
func BenchmarkGetPutAfterWarmup(b *testing.B) {
	r := MustNew()
	r.contextPool.Warmup()

	b.ResetTimer()
	for b.Loop() {
		ctx := r.contextPool.Get(int32(2))
		r.contextPool.Put(ctx)
	}
}

// BenchmarkGetPutNoWarmup measures Get/Put without warmup (baseline).
func BenchmarkGetPutNoWarmup(b *testing.B) {
	r := MustNew()

	b.ResetTimer()
	for b.Loop() {
		ctx := r.contextPool.Get(int32(2))
		r.contextPool.Put(ctx)
	}
}
