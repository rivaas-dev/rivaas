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
)

// globalContextPool is the context pool used for all request handling.
// Go's sync.Pool is already highly optimized with per-P caches and victim caches,
// making additional tiering unnecessary.
var globalContextPool = sync.Pool{
	New: func() any {
		return &Context{
			paramKeys:   [8]string{},
			paramValues: [8]string{},
		}
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
