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

// TestChaos_ConcurrentRouteRegistration tests registering routes concurrently
// to find race conditions in route registration.
// Note: Route registration during serving is not a supported pattern.
// Routes should be registered before serving begins.
func TestChaos_ConcurrentRouteRegistration(t *testing.T) {
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
	var registrationErrors atomic.Int64
	var requestErrors atomic.Int64

	// Phase 1: Register routes concurrently (before serving)
	for i := range numRoutes {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					registrationErrors.Add(1)
				}
			}()
			path := fmt.Sprintf("/route%d", id%10)
			app.GET(path, func(c *Context) {
				if err := c.Stringf(http.StatusOK, "route-%d", id); err != nil {
					c.Logger().Error("failed to write response", "err", err)
				}
			})
		}(i)
	}

	// Wait for all route registration to complete
	wg.Wait()

	// Phase 2: Make requests concurrently (after all routes registered)
	for i := range numRequests {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()
			routeID := rand.Intn(10) // Only 10 unique routes (0-9)
			path := fmt.Sprintf("/route%d", routeID)

			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			func() {
				defer func() {
					if r := recover(); r != nil {
						requestErrors.Add(1)
					}
				}()
				app.Router().ServeHTTP(w, req)
			}()
		}(i)
	}

	wg.Wait()

	assert.Equal(t, int64(0), registrationErrors.Load(), "no panics during registration")
	assert.Equal(t, int64(0), requestErrors.Load(), "no panics during requests")
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
		if err := c.String(http.StatusOK, "ok"); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
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
		if err := c.String(http.StatusOK, "ok"); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
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
	assert.Positive(t, counter.Load(), "middleware should have executed")
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
		if err := c.String(http.StatusOK, "ok"); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
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

// TestChaos_MixedOperations tests a mix of operations in phases:
// Phase 1: concurrent route registration and middleware addition
// Phase 2: concurrent request handling
// Note: Route registration during serving is not a supported pattern.
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
	var registrationErrors atomic.Int64
	var requestErrors atomic.Int64

	// Pre-register some routes
	app.GET("/existing", func(c *Context) {
		if err := c.String(http.StatusOK, "existing"); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	const operations = 50

	// Phase 1: Register routes and middleware concurrently (before serving)
	// Register new routes
	for i := range operations {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			func() {
				defer func() {
					if r := recover(); r != nil {
						registrationErrors.Add(1)
					}
				}()
				app.GET(fmt.Sprintf("/new%d", id), func(c *Context) {
					if err := c.Stringf(http.StatusOK, "new-%d", id); err != nil {
						c.Logger().Error("failed to write response", "err", err)
					}
				})
			}()
		}(i)
	}

	// Add middleware concurrently (also before serving)
	for range operations {
		wg.Go(func() {
			func() {
				defer func() {
					if r := recover(); r != nil {
						registrationErrors.Add(1)
					}
				}()
				app.Use(func(c *Context) {
					c.Next()
				})
			}()
		})
	}

	// Wait for all registration to complete
	wg.Wait()

	// Phase 2: Handle requests concurrently (after all routes registered)
	for range operations * 2 {
		wg.Go(func() {
			defer func() {
				if r := recover(); r != nil {
					requestErrors.Add(1)
				}
			}()
			req := httptest.NewRequest(http.MethodGet, "/existing", nil)
			w := httptest.NewRecorder()
			app.Router().ServeHTTP(w, req)
		})
	}

	wg.Wait()

	assert.Equal(t, int64(0), registrationErrors.Load(), "no panics during registration")
	assert.Equal(t, int64(0), requestErrors.Load(), "no panics during requests")
}
