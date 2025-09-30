package router

import (
	"sync"
)

// Global context pool for zero-allocation static routes
var globalContextPool = sync.Pool{
	New: func() interface{} {
		return &Context{}
	},
}

// ContextPool provides enhanced context pooling with specialized pools
// for different parameter counts to optimize memory usage and GC pressure
type ContextPool struct {
	// Separate pools for different context sizes
	smallPool  sync.Pool // ≤4 parameters (most common case)
	mediumPool sync.Pool // 5-8 parameters
	largePool  sync.Pool // >8 parameters (rare case)
	// Warm-up pool for high-traffic scenarios
	warmupPool sync.Pool
	router     *Router
}

// NewContextPool creates a new enhanced context pool
func NewContextPool(router *Router) *ContextPool {
	cp := &ContextPool{router: router}

	// Small context pool (≤4 params) - most common case
	cp.smallPool = sync.Pool{
		New: func() interface{} {
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
		New: func() interface{} {
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
		New: func() interface{} {
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
		New: func() interface{} {
			return make([]*Context, 0, 10) // Pool of contexts
		},
	}

	return cp
}

// GetContext gets a context from the appropriate pool based on parameter count
func (cp *ContextPool) GetContext(paramCount int) *Context {
	// Choose pool based on parameter count - optimized for common cases
	if paramCount <= 4 {
		return cp.smallPool.Get().(*Context)
	} else if paramCount <= 8 {
		return cp.mediumPool.Get().(*Context)
	} else {
		return cp.largePool.Get().(*Context)
	}
}

// PutContext returns a context to the appropriate pool
func (cp *ContextPool) PutContext(ctx *Context) {
	// Reset context for reuse
	ctx.reset()

	// Return to appropriate pool based on parameter count - optimized
	if ctx.paramCount <= 4 {
		cp.smallPool.Put(ctx)
	} else if ctx.paramCount <= 8 {
		cp.mediumPool.Put(ctx)
	} else {
		cp.largePool.Put(ctx)
	}
}

// WarmupPools pre-allocates contexts in all pools for high-traffic scenarios.
// This reduces allocation pressure during peak load.
func (cp *ContextPool) WarmupPools() {
	// Warm up small pool (most common case)
	for i := 0; i < 10; i++ {
		ctx := cp.smallPool.Get().(*Context)
		cp.smallPool.Put(ctx)
	}

	// Warm up medium pool
	for i := 0; i < 5; i++ {
		ctx := cp.mediumPool.Get().(*Context)
		cp.mediumPool.Put(ctx)
	}

	// Warm up large pool
	for i := 0; i < 2; i++ {
		ctx := cp.largePool.Get().(*Context)
		cp.largePool.Put(ctx)
	}
}
