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
	"sync/atomic"
)

// Global context pool for static routes
var globalContextPool = sync.Pool{
	New: func() any {
		return &Context{}
	},
}

// getContextFromGlobalPool safely retrieves a Context from the global pool.
// This function performs type assertion with safety checks to prevent panics
// from pool corruption.
//
// Why this matters:
//   - Pool corruption can occur if external code incorrectly uses the pool
//   - Type assertion panics are hard to debug in production
//   - Clear error message improves debugging
//
// Implementation:
// - Type assertion check
func getContextFromGlobalPool() *Context {
	ctx, ok := globalContextPool.Get().(*Context)
	if !ok {
		// This should never happen in normal operation. If it does, it indicates
		// either pool corruption or someone Put() an incorrect type into the pool.
		panic("router: pool corruption - globalContextPool returned non-Context type")
	}
	return ctx
}

// releaseGlobalContext safely cleans up and returns a context to the global pool.
// This helper ensures consistent cleanup across all context usage sites and provides
// a single point of truth for the cleanup logic.
//
// Usage:
//
//	c := getContextFromGlobalPool()
//	defer releaseGlobalContext(c)
//
// Why this matters:
// - Ensures consistent cleanup pattern throughout the codebase
// - Single point of change if cleanup logic needs to evolve
// - Easier to audit for context leaks
// - Reduces risk of forgetting cleanup steps
func releaseGlobalContext(c *Context) {
	c.reset()
	globalContextPool.Put(c)
}

// PoolStats holds statistics about context pool effectiveness.
// These metrics help tune pool behavior and diagnose issues.
type PoolStats struct {
	SmallHits  uint64  // Number of times small pool was used
	MediumHits uint64  // Number of times medium pool was used
	LargeHits  uint64  // Number of times large pool was used
	TotalGets  uint64  // Total Get() calls
	TotalPuts  uint64  // Total Put() calls
	HitRate    float64 // Calculated hit rate (Gets/Puts ratio)
	SmallPct   float64 // Percentage of small pool usage
	MediumPct  float64 // Percentage of medium pool usage
	LargePct   float64 // Percentage of large pool usage
}

// ContextPool provides context pooling for request handling.
//
// The pool uses separate pools for different context sizes:
//   - Small contexts: Few parameters
//   - Medium contexts: Moderate number of parameters
//   - Large contexts: Many parameters, requires map storage
type ContextPool struct {
	// Separate pools for different context sizes
	smallPool  sync.Pool // Few parameters
	mediumPool sync.Pool // Moderate number of parameters
	largePool  sync.Pool // Many parameters
	// Warm-up pool for high-traffic scenarios
	warmupPool sync.Pool
	router     *Router

	// Statistics (atomic counters for thread-safe updates)
	smallHits  uint64 // Atomic: small pool usage count
	mediumHits uint64 // Atomic: medium pool usage count
	largeHits  uint64 // Atomic: large pool usage count
	totalGets  uint64 // Atomic: total Get() calls
	totalPuts  uint64 // Atomic: total Put() calls
}

// NewContextPool creates a new context pool
func NewContextPool(router *Router) *ContextPool {
	cp := &ContextPool{router: router}

	// Small context pool (≤4 params) - most common case
	cp.smallPool = sync.Pool{
		New: func() any {
			ctx := &Context{
				router: router,
				// Pre-allocate small parameter arrays
				paramKeys:   [8]string{},
				paramValues: [8]string{},
			}
			ctx.reset()
			return ctx
		},
	}

	// Medium context pool (5-8 params)
	cp.mediumPool = sync.Pool{
		New: func() any {
			ctx := &Context{
				router:      router,
				paramKeys:   [8]string{},
				paramValues: [8]string{},
			}
			ctx.reset()
			return ctx
		},
	}

	// Large context pool (>8 params) - rare case
	cp.largePool = sync.Pool{
		New: func() any {
			ctx := &Context{
				router:      router,
				paramKeys:   [8]string{},
				paramValues: [8]string{},
				Params:      make(map[string]string, 16), // Pre-allocate map
			}
			ctx.reset()
			return ctx
		},
	}

	// Warm-up pool for high-traffic scenarios
	cp.warmupPool = sync.Pool{
		New: func() any {
			slice := make([]*Context, 0, 10) // Pool of contexts
			return &slice                    // Return pointer
		},
	}

	return cp
}

// Get gets a context from the appropriate pool based on parameter count.
// It routes to a pool based on the anticipated parameter count:
// - Small pool: Used for routes with few parameters
// - Medium pool: Used for routes with moderate parameter counts
// - Large pool: Handles edge cases with map-backed storage
func (cp *ContextPool) Get(paramCount int32) *Context {
	// Increment total gets counter
	atomic.AddUint64(&cp.totalGets, 1)

	// Choose pool based on parameter count
	if paramCount <= 4 {
		atomic.AddUint64(&cp.smallHits, 1)
		return cp.smallPool.Get().(*Context)
	}
	if paramCount <= 8 {
		atomic.AddUint64(&cp.mediumHits, 1)
		return cp.mediumPool.Get().(*Context)
	}
	atomic.AddUint64(&cp.largeHits, 1)
	return cp.largePool.Get().(*Context)
}

// Put returns a context to the appropriate pool
func (cp *ContextPool) Put(ctx *Context) {
	// Increment total puts counter
	atomic.AddUint64(&cp.totalPuts, 1)

	// CRITICAL: Save paramCount BEFORE reset() clears it to 0
	// This is needed for correct pool selection
	paramCount := ctx.paramCount

	// Reset context for reuse
	ctx.reset()

	// Return to appropriate pool based on ORIGINAL parameter count
	// Using saved paramCount because reset() cleared ctx.paramCount to 0
	if paramCount <= 4 {
		cp.smallPool.Put(ctx)
	} else if paramCount <= 8 {
		cp.mediumPool.Put(ctx)
	} else {
		cp.largePool.Put(ctx)
	}
}

// WarmupConfig configures pool warmup behavior for different traffic patterns.
// Use this to warm up the pool for your specific workload distribution.
//
// Default values are based on typical HTTP router traffic patterns where
// most routes have few parameters. Adjust these values based on your
// application's actual parameter distribution.
type WarmupConfig struct {
	SmallContexts  int // Number of small contexts to preallocate (default: 20)
	MediumContexts int // Number of medium contexts to preallocate (default: 10)
	LargeContexts  int // Number of large contexts to preallocate (default: 5)
}

// DefaultWarmupConfig returns the default warmup configuration.
// Defaults are chosen for typical REST API patterns where most routes
// have few parameters. Adjust based on your workload characteristics.
func DefaultWarmupConfig() *WarmupConfig {
	return &WarmupConfig{
		SmallContexts:  20, // Most common case
		MediumContexts: 10, // Occasional use
		LargeContexts:  5,  // Rare case
	}
}

// Warmup pre-allocates contexts in all pools.
// This ensures pools are populated before the first requests arrive.
//
// Configuration:
//   - No config: Uses defaults (20/10/5 for small/medium/large)
//   - Custom config: Configure based on your traffic patterns
//
// Note: sync.Pool may still GC items between warmup and usage, but warmup ensures
// pools are populated for initial burst traffic. Call periodically if needed.
//
// Example with custom config:
//
//	config := &router.WarmupConfig{
//	    SmallContexts:  50,  // Heavy small-param traffic
//	    MediumContexts: 5,   // Light medium-param traffic
//	    LargeContexts:  2,   // Minimal large-param traffic
//	}
//	router.contextPool.Warmup(config)
func (cp *ContextPool) Warmup(cfg ...*WarmupConfig) {
	// Use default or provided config
	config := DefaultWarmupConfig()
	if len(cfg) > 0 && cfg[0] != nil {
		config = cfg[0]
	}

	// Parallel warmup - each pool warms independently
	var wg sync.WaitGroup
	wg.Add(3)

	// Warm up small pool (most common case)
	go func() {
		defer wg.Done()
		for i := 0; i < config.SmallContexts; i++ {
			ctx := cp.smallPool.Get().(*Context)
			cp.smallPool.Put(ctx)
		}
	}()

	// Warm up medium pool (occasional use)
	go func() {
		defer wg.Done()
		for i := 0; i < config.MediumContexts; i++ {
			ctx := cp.mediumPool.Get().(*Context)
			cp.mediumPool.Put(ctx)
		}
	}()

	// Warm up large pool (rare case)
	go func() {
		defer wg.Done()
		for i := 0; i < config.LargeContexts; i++ {
			ctx := cp.largePool.Get().(*Context)
			cp.largePool.Put(ctx)
		}
	}()

	wg.Wait()
}

// Stats returns pool effectiveness statistics.
// This is useful for monitoring pool effectiveness and tuning pool sizes.
//
// Metrics:
//   - SmallHits/MediumHits/LargeHits: Usage count per pool
//   - TotalGets/TotalPuts: Overall pool activity
//   - HitRate: Ratio of Gets to Puts (should be ~1.0 for healthy pool)
//   - SmallPct/MediumPct/LargePct: Distribution of requests across pools
//
// Usage patterns:
//   - High HitRate (>0.95): Pool is being reused effectively
//   - Low HitRate (<0.80): Contexts may be leaking (not being Put() back)
//   - SmallPct typically highest: Most routes have ≤4 parameters
//   - LargePct typically lowest: Routes with >8 params are rare
//
// Example:
//
//	stats := router.contextPool.Stats()
//	log.Printf("Pool stats: HitRate=%.2f%%, Small=%.1f%%, Medium=%.1f%%, Large=%.1f%%",
//	    stats.HitRate*100, stats.SmallPct, stats.MediumPct, stats.LargePct)
func (cp *ContextPool) Stats() PoolStats {
	// Read counters atomically
	smallHits := atomic.LoadUint64(&cp.smallHits)
	mediumHits := atomic.LoadUint64(&cp.mediumHits)
	largeHits := atomic.LoadUint64(&cp.largeHits)
	totalGets := atomic.LoadUint64(&cp.totalGets)
	totalPuts := atomic.LoadUint64(&cp.totalPuts)

	// Calculate derived metrics
	var hitRate float64
	if totalGets > 0 {
		hitRate = float64(totalPuts) / float64(totalGets)
	}

	var smallPct, mediumPct, largePct float64
	if totalGets > 0 {
		smallPct = float64(smallHits) / float64(totalGets) * 100
		mediumPct = float64(mediumHits) / float64(totalGets) * 100
		largePct = float64(largeHits) / float64(totalGets) * 100
	}

	return PoolStats{
		SmallHits:  smallHits,
		MediumHits: mediumHits,
		LargeHits:  largeHits,
		TotalGets:  totalGets,
		TotalPuts:  totalPuts,
		HitRate:    hitRate,
		SmallPct:   smallPct,
		MediumPct:  mediumPct,
		LargePct:   largePct,
	}
}

// ResetStats resets all statistics counters to zero.
// This is useful for monitoring specific time periods.
func (cp *ContextPool) ResetStats() {
	atomic.StoreUint64(&cp.smallHits, 0)
	atomic.StoreUint64(&cp.mediumHits, 0)
	atomic.StoreUint64(&cp.largeHits, 0)
	atomic.StoreUint64(&cp.totalGets, 0)
	atomic.StoreUint64(&cp.totalPuts, 0)
}
