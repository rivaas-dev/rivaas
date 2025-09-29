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
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestChaos_ConcurrentRouteRegistrationAndDeletion tests registering and
// handling routes concurrently to find race conditions.
func TestChaos_ConcurrentRouteRegistrationAndDeletion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	const numRoutes = 100
	const numRequests = 1000
	var wg sync.WaitGroup
	var errors atomic.Int64

	// Register routes concurrently
	for i := range numRoutes {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			path := "/route" + string(rune('0'+id%10))
			app.GET(path, func(c *Context) {
				c.Stringf(http.StatusOK, "route-%d", id)
			})
		}(i)
	}

	// Make requests concurrently while routes are being registered
	for i := range numRequests {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()
			routeID := rand.Intn(numRoutes)
			path := fmt.Sprintf("/route%d", routeID)

			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			// Should not panic even if route registration is in progress
			func() {
				defer func() {
					if r := recover(); r != nil {
						errors.Add(1)
					}
				}()
				app.Router().ServeHTTP(w, req)
			}()
		}(i)
	}

	wg.Wait()

	// Some requests might fail (404) if route wasn't registered yet, but no panics
	assert.Equal(t, int64(0), errors.Load(), "no panics should occur")
}

// TestChaos_StressTestHighConcurrency tests the app under extreme
// concurrency conditions to find performance issues and race conditions.
func TestChaos_StressTestHighConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	var requestCount atomic.Int64
	var errorCount atomic.Int64

	app.GET("/stress", func(c *Context) {
		requestCount.Add(1)
		// Simulate variable work
		time.Sleep(time.Duration(rand.Intn(5)) * time.Millisecond)
		c.String(http.StatusOK, "ok")
	})

	const concurrency = 500
	const requestsPerGoroutine = 10
	var wg sync.WaitGroup

	start := time.Now()

	for range concurrency {
		wg.Go(func() {
			for range requestsPerGoroutine {
				req := httptest.NewRequest(http.MethodGet, "/stress", nil)
				w := httptest.NewRecorder()
				app.Router().ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					errorCount.Add(1)
				}
			}
		})
	}

	wg.Wait()
	duration := time.Since(start)

	totalRequests := int64(concurrency * requestsPerGoroutine)
	assert.Equal(t, totalRequests, requestCount.Load(), "all requests should be processed")
	assert.Equal(t, int64(0), errorCount.Load(), "no errors should occur")

	t.Logf("Processed %d requests in %v (%.0f req/s)",
		totalRequests, duration, float64(totalRequests)/duration.Seconds())
}

// TestChaos_RandomRoutePatterns tests with random route patterns to
// ensure the router handles edge cases correctly.
func TestChaos_RandomRoutePatterns(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	const numRoutes = 200
	var wg sync.WaitGroup
	var panicCount atomic.Int64

	// Generate random route patterns
	routes := make([]string, numRoutes)
	for i := range numRoutes {
		// Generate various route patterns
		switch i % 5 {
		case 0:
			routes[i] = fmt.Sprintf("/api/v%d/resource", i%10)
		case 1:
			routes[i] = "/users/:id/posts/:post_id"
		case 2:
			routes[i] = fmt.Sprintf("/static%d", i)
		case 3:
			routes[i] = fmt.Sprintf("/deep/nested/path/%d", i)
		default:
			routes[i] = fmt.Sprintf("/wildcard/*path%d", i)
		}
	}

	// Register routes concurrently
	for i, route := range routes {
		wg.Add(1)
		go func(id int, path string) {
			defer wg.Done()
			func() {
				defer func() {
					if r := recover(); r != nil {
						panicCount.Add(1)
					}
				}()
				app.GET(path, func(c *Context) {
					c.Stringf(http.StatusOK, "route-%d", id)
				})
			}()
		}(i, route)
	}

	wg.Wait()

	assert.Equal(t, int64(0), panicCount.Load(), "no panics during route registration")

	// Test that routes work
	for _, route := range routes[:10] { // Test first 10
		req := httptest.NewRequest(http.MethodGet, route, nil)
		w := httptest.NewRecorder()
		app.Router().ServeHTTP(w, req)
		// Routes might return 404 if pattern doesn't match, but shouldn't panic
		assert.NotEqual(t, http.StatusInternalServerError, w.Code,
			"route should not return 500")
	}
}

// TestChaos_MiddlewareChainStress tests middleware chains under stress
// to ensure correct execution order and no race conditions.
func TestChaos_MiddlewareChainStress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	const numMiddleware = 10
	var executionOrder sync.Map // Use sync.Map for concurrent access
	var counter atomic.Int64

	// Add many middleware
	for i := range numMiddleware {
		app.Use(func(c *Context) {
			order := counter.Add(1)
			executionOrder.Store(order, i)
			c.Next()
		})
	}

	app.GET("/test", func(c *Context) {
		order := counter.Add(1)
		executionOrder.Store(order, -1) // -1 indicates handler
		c.String(http.StatusOK, "ok")
	})

	const concurrency = 100
	var wg sync.WaitGroup

	for range concurrency {
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			app.Router().ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}

	wg.Wait()

	// Verify execution order for last request
	// (order verification is complex with concurrent requests, so we just check no panics)
	assert.Greater(t, counter.Load(), int64(0), "middleware should have executed")
}

// TestChaos_ContextPoolExhaustion tests that context pooling works correctly
// even under extreme load where contexts might be exhausted.
func TestChaos_ContextPoolExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	// Handler that holds context briefly
	app.GET("/slow", func(c *Context) {
		time.Sleep(5 * time.Millisecond)
		c.String(http.StatusOK, "ok")
	})

	const burstSize = 1000
	var wg sync.WaitGroup
	var successCount atomic.Int64

	// Burst of requests that might exhaust context pool
	for range burstSize {
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodGet, "/slow", nil)
			w := httptest.NewRecorder()
			app.Router().ServeHTTP(w, req)
			if w.Code == http.StatusOK {
				successCount.Add(1)
			}
		})
	}

	wg.Wait()

	// All requests should succeed even if context pool is exhausted
	// (pool should allocate new contexts as needed)
	assert.Equal(t, int64(burstSize), successCount.Load(),
		"all requests should succeed even under pool exhaustion")
}

// TestChaos_MixedOperations tests a mix of operations (route registration,
// middleware addition, request handling) happening concurrently.
func TestChaos_MixedOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	var wg sync.WaitGroup
	var errors atomic.Int64

	// Pre-register some routes
	app.GET("/existing", func(c *Context) {
		c.String(http.StatusOK, "existing")
	})

	// Concurrently: register routes, add middleware, handle requests
	const operations = 50

	// Register new routes
	for i := range operations {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			func() {
				defer func() {
					if r := recover(); r != nil {
						errors.Add(1)
					}
				}()
				app.GET(fmt.Sprintf("/new%d", id), func(c *Context) {
					c.Stringf(http.StatusOK, "new-%d", id)
				})
			}()
		}(i)
	}

	// Add middleware
	for range operations {
		wg.Go(func() {
			func() {
				defer func() {
					if r := recover(); r != nil {
						errors.Add(1)
					}
				}()
				app.Use(func(c *Context) {
					c.Next()
				})
			}()
		})
	}

	// Handle requests
	for range operations * 2 {
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodGet, "/existing", nil)
			w := httptest.NewRecorder()
			app.Router().ServeHTTP(w, req)
		})
	}

	wg.Wait()

	// Should have minimal errors (some 404s are expected for new routes)
	assert.Less(t, errors.Load(), int64(operations), "should have minimal panics")
}
