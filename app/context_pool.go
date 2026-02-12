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

package app

import (
	"sync"
)

// contextPool provides pooling for [Context] instances, reusing them across requests.
type contextPool struct {
	pool sync.Pool
}

// newContextPool creates a new context pool.
func newContextPool() *contextPool {
	return &contextPool{
		pool: sync.Pool{
			New: func() any {
				return &Context{}
			},
		},
	}
}

// Get retrieves a [Context] from the pool, creating a new one if the pool is empty.
func (cp *contextPool) Get() *Context {
	//nolint:errcheck,forcetypeassert // Type assertion is safe: pool.New always returns *Context
	return cp.pool.Get().(*Context)
}

// Put returns a Context to the pool after resetting it.
//
// Cleanup is also performed in wrapHandler's defer to ensure
// contexts are reset even if handlers panic. This method's cleanup
// is idempotent and provides an additional safety layer.
func (cp *contextPool) Put(c *Context) {
	// Reset context state
	c.Context = nil
	c.app = nil
	c.bindingMeta = nil
	cp.pool.Put(c)
}
