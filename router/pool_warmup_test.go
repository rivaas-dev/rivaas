package router

import (
	"sync"
	"testing"
)

// TestWarmupDefault verifies default warmup behavior
func TestWarmupDefault(t *testing.T) {
	r := New()

	// Warmup with defaults
	r.contextPool.Warmup()

	// Verify pools can get contexts after warmup
	ctx := r.contextPool.Get(2) // Small pool
	if ctx == nil {
		t.Fatal("Failed to get context from small pool after warmup")
	}
	r.contextPool.Put(ctx)

	ctx = r.contextPool.Get(6) // Medium pool
	if ctx == nil {
		t.Fatal("Failed to get context from medium pool after warmup")
	}
	r.contextPool.Put(ctx)

	ctx = r.contextPool.Get(10) // Large pool
	if ctx == nil {
		t.Fatal("Failed to get context from large pool after warmup")
	}
	r.contextPool.Put(ctx)
}

// TestWarmupCustomConfig verifies custom warmup configuration
func TestWarmupCustomConfig(t *testing.T) {
	r := New()

	config := &WarmupConfig{
		SmallContexts:  50,
		MediumContexts: 25,
		LargeContexts:  10,
	}

	// Warmup with custom config should not panic
	r.contextPool.Warmup(config)

	// Verify pools still work
	ctx := r.contextPool.Get(2)
	if ctx == nil {
		t.Fatal("Failed to get context after custom warmup")
	}
	r.contextPool.Put(ctx)
}

// TestWarmupZeroConfig verifies warmup handles zero values gracefully
func TestWarmupZeroConfig(t *testing.T) {
	r := New()

	config := &WarmupConfig{
		SmallContexts:  0,
		MediumContexts: 0,
		LargeContexts:  0,
	}

	// Should not panic with zero config
	r.contextPool.Warmup(config)

	// Pools should still create contexts on demand
	ctx := r.contextPool.Get(2)
	if ctx == nil {
		t.Fatal("Failed to get context with zero warmup")
	}
	r.contextPool.Put(ctx)
}

// TestWarmupConcurrent verifies warmup is safe under concurrent access
func TestWarmupConcurrent(t *testing.T) {
	r := New()

	var wg sync.WaitGroup
	const goroutines = 10

	// Run warmup concurrently with Gets and Puts
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			r.contextPool.Warmup()

			// Also do some Gets/Puts
			ctx := r.contextPool.Get(2)
			r.contextPool.Put(ctx)
		}()
	}

	wg.Wait()

	// Pool should still work normally
	ctx := r.contextPool.Get(2)
	if ctx == nil {
		t.Fatal("Pool broken after concurrent warmup")
	}
	r.contextPool.Put(ctx)
}

// TestDefaultWarmupConfig verifies default config values
func TestDefaultWarmupConfig(t *testing.T) {
	config := DefaultWarmupConfig()

	if config.SmallContexts != 20 {
		t.Errorf("Expected SmallContexts=20, got %d", config.SmallContexts)
	}
	if config.MediumContexts != 10 {
		t.Errorf("Expected MediumContexts=10, got %d", config.MediumContexts)
	}
	if config.LargeContexts != 5 {
		t.Errorf("Expected LargeContexts=5, got %d", config.LargeContexts)
	}
}

// TestWarmupMultipleTimes verifies warmup can be called multiple times
func TestWarmupMultipleTimes(t *testing.T) {
	r := New()

	// Call warmup multiple times
	for i := 0; i < 5; i++ {
		r.contextPool.Warmup()
	}

	// Pool should still work
	ctx := r.contextPool.Get(2)
	if ctx == nil {
		t.Fatal("Pool broken after multiple warmups")
	}
	r.contextPool.Put(ctx)
}

// TestWarmupNilConfig verifies nil config uses defaults
func TestWarmupNilConfig(t *testing.T) {
	r := New()

	// Passing nil should use defaults
	r.contextPool.Warmup(nil)

	// Pool should work normally
	ctx := r.contextPool.Get(2)
	if ctx == nil {
		t.Fatal("Failed to get context with nil config")
	}
	r.contextPool.Put(ctx)
}

// BenchmarkWarmupDefault measures default warmup performance
func BenchmarkWarmupDefault(b *testing.B) {
	r := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.contextPool.Warmup()
	}
}

// BenchmarkWarmupLarge measures warmup with large config
func BenchmarkWarmupLarge(b *testing.B) {
	r := New()
	config := &WarmupConfig{
		SmallContexts:  100,
		MediumContexts: 50,
		LargeContexts:  25,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.contextPool.Warmup(config)
	}
}

// BenchmarkWarmupSequential compares parallel vs sequential warmup
// This benchmark shows the benefit of parallel warmup implementation
func BenchmarkWarmupSequential(b *testing.B) {
	r := New()
	config := DefaultWarmupConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
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
	r := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.contextPool.Warmup()
	}
}

// BenchmarkGetPutAfterWarmup measures Get/Put performance with warmed pool
func BenchmarkGetPutAfterWarmup(b *testing.B) {
	r := New()
	r.contextPool.Warmup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := r.contextPool.Get(2)
		r.contextPool.Put(ctx)
	}
}

// BenchmarkGetPutNoWarmup measures Get/Put performance without warmup (baseline)
func BenchmarkGetPutNoWarmup(b *testing.B) {
	r := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := r.contextPool.Get(2)
		r.contextPool.Put(ctx)
	}
}
