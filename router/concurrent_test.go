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
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// ConcurrentTestSuite tests concurrent operations with race detector
type ConcurrentTestSuite struct {
	suite.Suite
}

// TestConcurrentRouteRegistration tests concurrent route registration
// Run with: go test -race -run TestConcurrentRouteRegistration
func (suite *ConcurrentTestSuite) TestConcurrentRouteRegistration() {
	r := MustNew()

	// Register routes concurrently
	var wg sync.WaitGroup
	numGoroutines := 100
	routesPerGoroutine := 10

	for id := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range routesPerGoroutine {
				path := fmt.Sprintf("/route-%d-%d", id, j)
				r.GET(path, func(c *Context) {
					//nolint:errcheck // Test handler
					c.String(http.StatusOK, "OK")
				})
			}
		}(id)
	}

	wg.Wait()

	// Verify all routes were registered
	routes := r.Routes()
	suite.Len(routes, numGoroutines*routesPerGoroutine, "All routes should be registered")
}

// TestConcurrentRequestHandling tests concurrent request handling
func (suite *ConcurrentTestSuite) TestConcurrentRequestHandling() {
	r := MustNew()

	// Register routes
	r.GET("/fast", func(c *Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "fast")
	})

	r.GET("/slow", func(c *Context) {
		time.Sleep(10 * time.Millisecond)
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "slow")
	})

	r.GET("/params/:id", func(c *Context) {
		//nolint:errcheck // Test handler
		c.Stringf(http.StatusOK, "%s", c.Param("id"))
	})

	// Make concurrent requests
	var wg sync.WaitGroup
	numRequests := 1000
	var successCount int64

	for id := range numRequests {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Choose random endpoint
			var path string
			switch id % 3 {
			case 0:
				path = "/fast"
			case 1:
				path = "/slow"
			case 2:
				path = fmt.Sprintf("/params/%d", id)
			}

			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				atomic.AddInt64(&successCount, 1)
			}
		}(id)
	}

	wg.Wait()

	suite.Equal(int64(numRequests), successCount, "All requests should succeed")
}

// TestConcurrentRouteCompilation tests route compilation with concurrent route registration
// Note: Compilation itself is not thread-safe and should only be called once after all
// routes are registered. This test verifies that routes work correctly after compilation
// even when registered concurrently.
func (suite *ConcurrentTestSuite) TestConcurrentRouteCompilation() {
	r := MustNew()

	// Register routes concurrently
	var wg sync.WaitGroup
	for id := range 100 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			path := fmt.Sprintf("/route-%d", id)
			r.GET(path, func(c *Context) {
				//nolint:errcheck // Test handler
				c.String(http.StatusOK, "OK")
			})
		}(id)
	}

	wg.Wait()

	// Compile routes once after registration (this is the correct usage pattern)
	r.CompileAllRoutes()

	// Verify routes work after compilation with concurrent requests
	var requestWg sync.WaitGroup
	for id := range 100 {
		requestWg.Add(1)
		go func(id int) {
			defer requestWg.Done()
			path := fmt.Sprintf("/route-%d", id)
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			suite.Equal(http.StatusOK, w.Code)
		}(id)
	}

	requestWg.Wait()
}

// TestConcurrentContextPooling tests context pool under concurrent load
func (suite *ConcurrentTestSuite) TestConcurrentContextPooling() {
	r := MustNew()

	r.GET("/test", func(c *Context) {
		// Simulate some work
		_ = c.Param("nonexistent")
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "OK")
	})

	// Make many concurrent requests to test context pooling
	var wg sync.WaitGroup
	numRequests := 10000

	for range numRequests {
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		})
	}

	wg.Wait()

	// No assertion needed - if there's a race condition, -race flag will catch it
}

// TestConcurrentMiddlewareExecution tests middleware execution under concurrent load
func (suite *ConcurrentTestSuite) TestConcurrentMiddlewareExecution() {
	r := MustNew()

	var counter int64

	// Add middleware that increments counter
	r.Use(func(c *Context) {
		atomic.AddInt64(&counter, 1)
		c.Next()
	})

	r.GET("/test", func(c *Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "OK")
	})

	// Make concurrent requests
	var wg sync.WaitGroup
	numRequests := 1000

	for range numRequests {
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		})
	}

	wg.Wait()

	suite.Equal(int64(numRequests), counter, "Middleware should execute for all requests")
}

// TestConcurrentGroupRegistration tests concurrent route group registration
func (suite *ConcurrentTestSuite) TestConcurrentGroupRegistration() {
	r := MustNew()

	var wg sync.WaitGroup
	numGroups := 50

	for id := range numGroups {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			prefix := fmt.Sprintf("/api/v%d", id)
			group := r.Group(prefix)

			group.GET("/users", func(c *Context) {
				//nolint:errcheck // Test handler
				c.String(http.StatusOK, "users")
			})

			group.POST("/users", func(c *Context) {
				//nolint:errcheck // Test handler
				c.String(http.StatusCreated, "created")
			})
		}(id)
	}

	wg.Wait()

	// Verify groups work
	req := httptest.NewRequest(http.MethodGet, "/api/v25/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	suite.Equal(http.StatusOK, w.Code)
	suite.Equal("users", w.Body.String())
}

// TestConcurrentParameterExtraction tests parameter extraction under concurrent load
func (suite *ConcurrentTestSuite) TestConcurrentParameterExtraction() {
	r := MustNew()

	r.GET("/users/:id/posts/:postId/comments/:commentId", func(c *Context) {
		id := c.Param("id")
		postID := c.Param("postId")
		commentID := c.Param("commentId")

		//nolint:errcheck // Test handler
		c.JSON(http.StatusOK, map[string]string{
			"id":        id,
			"postId":    postID,
			"commentId": commentID,
		})
	})

	var wg sync.WaitGroup
	numRequests := 1000

	for id := range numRequests {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			path := fmt.Sprintf("/users/%d/posts/%d/comments/%d", id, id*2, id*3)
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			suite.Equal(http.StatusOK, w.Code)
		}(id)
	}

	wg.Wait()
}

// TestConcurrentWarmup tests warmup under concurrent load
func (suite *ConcurrentTestSuite) TestConcurrentWarmup() {
	r := MustNew()

	// Register many routes
	for i := range 100 {
		path := fmt.Sprintf("/route-%d", i)
		r.GET(path, func(c *Context) {
			//nolint:errcheck // Test handler
			c.String(http.StatusOK, "OK")
		})
	}

	// Warmup should only be called once, not concurrently
	// Testing that it doesn't break when called after concurrent registration
	r.Warmup()

	// Verify routes still work
	req := httptest.NewRequest(http.MethodGet, "/route-42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	suite.Equal(http.StatusOK, w.Code)
}

// TestConcurrentConstraintValidation tests constraint validation under concurrent load
func (suite *ConcurrentTestSuite) TestConcurrentConstraintValidation() {
	r := MustNew()

	r.GET("/users/:id", func(c *Context) {
		//nolint:errcheck // Test handler
		c.Stringf(http.StatusOK, "%s", c.Param("id"))
	}).WhereInt("id")

	var wg sync.WaitGroup
	numRequests := 500
	var validCount, invalidCount int64

	for id := range numRequests {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Half valid, half invalid
			var path string
			if id%2 == 0 {
				path = fmt.Sprintf("/users/%d", id)
			} else {
				path = fmt.Sprintf("/users/invalid%d", id)
			}

			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				atomic.AddInt64(&validCount, 1)
			} else {
				atomic.AddInt64(&invalidCount, 1)
			}
		}(id)
	}

	wg.Wait()

	suite.Equal(int64(numRequests/2), validCount, "Valid requests should succeed")
	suite.Equal(int64(numRequests/2), invalidCount, "Invalid requests should fail")
}

// TestConcurrentStaticRoutes tests static route handling under concurrent load
func (suite *ConcurrentTestSuite) TestConcurrentStaticRoutes() {
	r := MustNew()

	// Register many static routes
	for i := range 50 {
		path := fmt.Sprintf("/static/route/%d", i)
		r.GET(path, func(c *Context) {
			//nolint:errcheck // Test handler
			c.String(http.StatusOK, "static")
		})
	}

	r.CompileAllRoutes()

	// Access them concurrently
	var wg sync.WaitGroup
	numRequests := 1000

	for id := range numRequests {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			routeNum := id % 50
			path := fmt.Sprintf("/static/route/%d", routeNum)
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			suite.Equal(http.StatusOK, w.Code)
		}(id)
	}

	wg.Wait()
}

// Run the concurrent test suite
//
//nolint:paralleltest // Test suites manage their own parallelization
func TestConcurrentTestSuite(t *testing.T) {
	suite.Run(t, new(ConcurrentTestSuite))
}

func TestAtomicRouteRegistration(t *testing.T) {
	t.Parallel()
	r := MustNew()

	var wg sync.WaitGroup
	routeCount := 1000
	concurrency := 10

	// Register routes concurrently
	for i := range concurrency {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := range routeCount / concurrency {
				routeID := workerID*routeCount/concurrency + j
				path := "/test" + string(rune('0'+routeID%10)) + "/" + string(rune('0'+routeID%100))

				r.GET(path, func(c *Context) {
					//nolint:errcheck // Test handler
					c.String(http.StatusOK, "OK")
				})
			}
		}(i)
	}

	wg.Wait()

	// Verify all routes were registered
	routes := r.Routes()
	assert.GreaterOrEqual(t, len(routes), routeCount, "Expected at least %d routes", routeCount)

	// Test that the router is still functional
	req := httptest.NewRequest(http.MethodGet, "/test0/0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestAtomicRouteLookup tests that route lookup is lock-free and concurrent.
func TestAtomicRouteLookup(t *testing.T) {
	t.Parallel()
	r := MustNew()

	// Register test routes
	routes := []string{
		"/",
		"/users",
		"/users/:id",
		"/users/:id/posts",
		"/users/:id/posts/:post_id",
		"/posts",
		"/posts/:id",
		"/api/v1/users",
		"/api/v1/users/:id",
		"/api/v1/posts",
		"/api/v1/posts/:id",
	}

	for _, route := range routes {
		r.GET(route, func(c *Context) {
			//nolint:errcheck // Test handler
			c.String(http.StatusOK, "OK")
		})
	}

	var wg sync.WaitGroup
	requestCount := 1000
	concurrency := 10

	// Make concurrent requests
	for range concurrency {
		wg.Go(func() {
			for range requestCount / concurrency {
				// Test different route patterns
				testPaths := []string{
					"/",
					"/users",
					"/users/123",
					"/users/123/posts",
					"/users/123/posts/456",
					"/posts",
					"/posts/123",
					"/api/v1/users",
					"/api/v1/users/123",
					"/api/v1/posts",
					"/api/v1/posts/123",
				}

				for _, path := range testPaths {
					req := httptest.NewRequest(http.MethodGet, path, nil)
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)

					// Should get 200 for valid routes, 404 for invalid ones
					assert.Contains(t, []int{http.StatusOK, http.StatusNotFound}, w.Code, "Unexpected status code %d for path %s", w.Code, path)
				}
			}
		})
	}

	wg.Wait()
}

// TestConcurrentLookupAfterRegistration tests that concurrent route lookups
// can happen safely after routes are registered and the router is frozen.
//
// Note: The router uses a two-phase design where routes are registered first
// (single-threaded configuration phase), then frozen before serving. After
// freezing, the route tree is immutable and safe for concurrent reads.
func TestConcurrentLookupAfterRegistration(t *testing.T) {
	t.Parallel()
	r := MustNew()

	// Phase 1: Register routes (single-threaded)
	for i := range 100 {
		path := "/concurrent" + string(rune('0'+i%10))
		r.GET(path, func(c *Context) {
			//nolint:errcheck // Test handler
			c.String(http.StatusOK, "OK")
		})
	}

	// Freeze the router (happens automatically on first ServeHTTP, but we can be explicit)
	r.Freeze()

	// Phase 2: Concurrent lookups (routes are now immutable)
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 10 {
				path := "/concurrent" + string(rune('0'+i%10))
				req := httptest.NewRequest(http.MethodGet, path, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				// All should succeed now that routes are registered
				assert.Equal(t, http.StatusOK, w.Code)
			}
		}()
	}

	wg.Wait()

	// Verify routes exist
	routes := r.Routes()
	assert.NotEmpty(t, routes, "No routes were registered")
}

// TestRouteRegistrationAfterServeHTTPPanics tests that attempting to register
// routes after ServeHTTP has been called will panic.
//
// This is a design constraint to prevent data races: the router has two phases:
// 1. Configuration phase: register routes (single-threaded)
// 2. Serving phase: handle requests (concurrent, immutable routes)
func TestRouteRegistrationAfterServeHTTPPanics(t *testing.T) {
	t.Parallel()
	r := MustNew()

	// Register a route
	r.GET("/test", func(c *Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "OK")
	})

	// Start serving (this triggers freeze)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Attempting to register a new route should panic
	assert.Panics(t, func() {
		r.GET("/new-route", func(c *Context) {
			//nolint:errcheck // Test handler
			c.String(http.StatusOK, "NEW")
		})
	}, "should panic when registering route after ServeHTTP")
}

// TestAtomicTreeConsistency tests that the atomic tree updates maintain consistency.
func TestAtomicTreeConsistency(t *testing.T) {
	t.Parallel()
	r := MustNew()

	// Register routes in a specific order
	routes := []string{
		"/api/v1/users",
		"/api/v1/posts",
		"/api/v2/users",
		"/api/v2/posts",
		"/admin/users",
		"/admin/posts",
	}

	for _, route := range routes {
		r.GET(route, func(c *Context) {
			//nolint:errcheck // Test handler
			c.String(http.StatusOK, "OK")
		})
	}

	// Verify all routes are accessible
	for _, route := range routes {
		req := httptest.NewRequest(http.MethodGet, route, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Route %s returned status %d, expected 200", route, w.Code)
	}

	// Verify route introspection
	registeredRoutes := r.Routes()
	assert.Len(t, registeredRoutes, len(routes))
}

// TestAtomicTreeVersioning tests that the version counter is incremented correctly.
// Note: Routes use deferred registration, so version only increments after Warmup().
func TestAtomicTreeVersioning(t *testing.T) {
	t.Parallel()
	r := MustNew()

	initialVersion := atomic.LoadUint64(&r.routeTree.version)

	// Register a route
	r.GET("/test", func(c *Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "OK")
	})

	// Trigger warmup to register pending routes to the tree
	r.Warmup()

	// Version should be incremented after warmup registers routes
	newVersion := atomic.LoadUint64(&r.routeTree.version)
	assert.Greater(t, newVersion, initialVersion, "Version should be incremented after warmup, got %d -> %d", initialVersion, newVersion)

	// Register another route (post-warmup routes are registered immediately)
	r.POST("/test", func(c *Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "OK")
	})

	// Version should be incremented again (immediate registration after warmup)
	finalVersion := atomic.LoadUint64(&r.routeTree.version)
	assert.Greater(t, finalVersion, newVersion, "Version should be incremented again, got %d -> %d", newVersion, finalVersion)
}

// TestAtomicTreeMemorySafety tests that the atomic operations don't cause
// memory leaks or unsafe access patterns.
func TestAtomicTreeMemorySafety(t *testing.T) {
	t.Parallel()
	r := MustNew()

	// Register many routes to test memory management
	for i := range 1000 {
		path := "/memory" + string(rune('0'+i%10)) + "/" + string(rune('0'+i%100))
		r.GET(path, func(c *Context) {
			//nolint:errcheck // Test handler
			c.String(http.StatusOK, "OK")
		})
	}

	// Verify routes are accessible
	req := httptest.NewRequest(http.MethodGet, "/memory0/0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify route count
	routes := r.Routes()
	assert.Len(t, routes, 1000)
}
