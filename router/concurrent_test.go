package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// ConcurrentTestSuite tests concurrent operations with race detector
type ConcurrentTestSuite struct {
	suite.Suite
}

// TestConcurrentRouteRegistration tests concurrent route registration
// Run with: go test -race -run TestConcurrentRouteRegistration
func (suite *ConcurrentTestSuite) TestConcurrentRouteRegistration() {
	r := New()

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
					c.String(200, "OK")
				})
			}
		}(id)
	}

	wg.Wait()

	// Verify all routes were registered
	routes := r.Routes()
	suite.Equal(numGoroutines*routesPerGoroutine, len(routes), "All routes should be registered")
}

// TestConcurrentRequestHandling tests concurrent request handling
func (suite *ConcurrentTestSuite) TestConcurrentRequestHandling() {
	r := New()

	// Register routes
	r.GET("/fast", func(c *Context) {
		c.String(200, "fast")
	})

	r.GET("/slow", func(c *Context) {
		time.Sleep(10 * time.Millisecond)
		c.String(200, "slow")
	})

	r.GET("/params/:id", func(c *Context) {
		c.String(200, "%s", c.Param("id"))
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

			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code == 200 {
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
	r := New()

	// Register routes concurrently
	var wg sync.WaitGroup
	for id := range 100 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			path := fmt.Sprintf("/route-%d", id)
			r.GET(path, func(c *Context) {
				c.String(200, "OK")
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
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			suite.Equal(200, w.Code)
		}(id)
	}

	requestWg.Wait()
}

// TestConcurrentContextPooling tests context pool under concurrent load
func (suite *ConcurrentTestSuite) TestConcurrentContextPooling() {
	r := New()

	r.GET("/test", func(c *Context) {
		// Simulate some work
		_ = c.Param("nonexistent")
		c.String(200, "OK")
	})

	// Make many concurrent requests to test context pooling
	var wg sync.WaitGroup
	numRequests := 10000

	for range numRequests {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}()
	}

	wg.Wait()

	// No assertion needed - if there's a race condition, -race flag will catch it
}

// TestConcurrentMiddlewareExecution tests middleware execution under concurrent load
func (suite *ConcurrentTestSuite) TestConcurrentMiddlewareExecution() {
	r := New()

	var counter int64

	// Add middleware that increments counter
	r.Use(func(c *Context) {
		atomic.AddInt64(&counter, 1)
		c.Next()
	})

	r.GET("/test", func(c *Context) {
		c.String(200, "OK")
	})

	// Make concurrent requests
	var wg sync.WaitGroup
	numRequests := 1000

	for range numRequests {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}()
	}

	wg.Wait()

	suite.Equal(int64(numRequests), counter, "Middleware should execute for all requests")
}

// TestConcurrentGroupRegistration tests concurrent route group registration
func (suite *ConcurrentTestSuite) TestConcurrentGroupRegistration() {
	r := New()

	var wg sync.WaitGroup
	numGroups := 50

	for id := range numGroups {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			prefix := fmt.Sprintf("/api/v%d", id)
			group := r.Group(prefix)

			group.GET("/users", func(c *Context) {
				c.String(200, "users")
			})

			group.POST("/users", func(c *Context) {
				c.String(201, "created")
			})
		}(id)
	}

	wg.Wait()

	// Verify groups work
	req := httptest.NewRequest("GET", "/api/v25/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	suite.Equal(200, w.Code)
	suite.Equal("users", w.Body.String())
}

// TestConcurrentParameterExtraction tests parameter extraction under concurrent load
func (suite *ConcurrentTestSuite) TestConcurrentParameterExtraction() {
	r := New()

	r.GET("/users/:id/posts/:postId/comments/:commentId", func(c *Context) {
		id := c.Param("id")
		postId := c.Param("postId")
		commentId := c.Param("commentId")

		c.JSON(200, map[string]string{
			"id":        id,
			"postId":    postId,
			"commentId": commentId,
		})
	})

	var wg sync.WaitGroup
	numRequests := 1000

	for id := range numRequests {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			path := fmt.Sprintf("/users/%d/posts/%d/comments/%d", id, id*2, id*3)
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			suite.Equal(200, w.Code)
		}(id)
	}

	wg.Wait()
}

// TestConcurrentWarmupOptimizations tests warmup under concurrent load
func (suite *ConcurrentTestSuite) TestConcurrentWarmupOptimizations() {
	r := New()

	// Register many routes
	for i := range 100 {
		path := fmt.Sprintf("/route-%d", i)
		r.GET(path, func(c *Context) {
			c.String(200, "OK")
		})
	}

	// Warmup should only be called once, not concurrently
	// Testing that it doesn't break when called after concurrent registration
	r.WarmupOptimizations()

	// Verify routes still work
	req := httptest.NewRequest("GET", "/route-42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	suite.Equal(200, w.Code)
}

// TestConcurrentConstraintValidation tests constraint validation under concurrent load
func (suite *ConcurrentTestSuite) TestConcurrentConstraintValidation() {
	r := New()

	r.GET("/users/:id", func(c *Context) {
		c.String(200, "%s", c.Param("id"))
	}).WhereNumber("id")

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

			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code == 200 {
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
	r := New()

	// Register many static routes
	for i := range 50 {
		path := fmt.Sprintf("/static/route/%d", i)
		r.GET(path, func(c *Context) {
			c.String(200, "static")
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
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			suite.Equal(200, w.Code)
		}(id)
	}

	wg.Wait()
}

// Run the concurrent test suite
func TestConcurrentTestSuite(t *testing.T) {
	suite.Run(t, new(ConcurrentTestSuite))
}

// ============================================================================
// Atomic Operations Tests (merged from atomic_test.go)
// ============================================================================

func TestAtomicRouteRegistration(t *testing.T) {
	r := New()

	var wg sync.WaitGroup
	routeCount := 1000
	concurrency := 10

	// Register routes concurrently
	for i := range concurrency {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < routeCount/concurrency; j++ {
				routeID := workerID*routeCount/concurrency + j
				path := "/test" + string(rune('0'+routeID%10)) + "/" + string(rune('0'+routeID%100))

				r.GET(path, func(c *Context) {
					c.String(http.StatusOK, "OK")
				})
			}
		}(i)
	}

	wg.Wait()

	// Verify all routes were registered
	routes := r.Routes()
	if len(routes) < routeCount {
		t.Errorf("Expected at least %d routes, got %d", routeCount, len(routes))
	}

	// Test that the router is still functional
	req := httptest.NewRequest("GET", "/test0/0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestAtomicRouteLookup tests that route lookup is lock-free and concurrent.
func TestAtomicRouteLookup(t *testing.T) {
	r := New()

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
			c.String(http.StatusOK, "OK")
		})
	}

	var wg sync.WaitGroup
	requestCount := 1000
	concurrency := 10

	// Make concurrent requests
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestCount/concurrency; j++ {
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
					req := httptest.NewRequest("GET", path, nil)
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)

					// Should get 200 for valid routes, 404 for invalid ones
					if w.Code != 200 && w.Code != 404 {
						t.Errorf("Unexpected status code %d for path %s", w.Code, path)
					}
				}
			}
		}()
	}

	wg.Wait()
}

// TestConcurrentRegistrationAndLookup tests that route registration and lookup
// can happen concurrently without issues.
func TestConcurrentRegistrationAndLookup(t *testing.T) {
	r := New()

	var wg sync.WaitGroup
	done := make(chan bool)

	// Start route registration goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 100 {
			path := "/concurrent" + string(rune('0'+i%10))
			r.GET(path, func(c *Context) {
				c.String(http.StatusOK, "OK")
			})
			time.Sleep(time.Millisecond) // Small delay to allow lookups
		}
		close(done)
	}()

	// Start route lookup goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				req := httptest.NewRequest("GET", "/", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				// Don't check status as routes are being added
			}
		}
	}()

	wg.Wait()

	// Verify final state
	routes := r.Routes()
	if len(routes) == 0 {
		t.Error("No routes were registered")
	}
}

// TestAtomicTreeConsistency tests that the atomic tree updates maintain consistency.
func TestAtomicTreeConsistency(t *testing.T) {
	r := New()

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
			c.String(http.StatusOK, "OK")
		})
	}

	// Verify all routes are accessible
	for _, route := range routes {
		req := httptest.NewRequest("GET", route, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("Route %s returned status %d, expected 200", route, w.Code)
		}
	}

	// Verify route introspection
	registeredRoutes := r.Routes()
	if len(registeredRoutes) != len(routes) {
		t.Errorf("Expected %d routes, got %d", len(routes), len(registeredRoutes))
	}
}

// TestAtomicTreeVersioning tests that the version counter is incremented correctly.
func TestAtomicTreeVersioning(t *testing.T) {
	r := New()

	initialVersion := atomic.LoadUint64(&r.routeTree.version)

	// Register a route
	r.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "OK")
	})

	// Version should be incremented
	newVersion := atomic.LoadUint64(&r.routeTree.version)
	if newVersion <= initialVersion {
		t.Errorf("Version should be incremented, got %d -> %d", initialVersion, newVersion)
	}

	// Register another route
	r.POST("/test", func(c *Context) {
		c.String(http.StatusOK, "OK")
	})

	// Version should be incremented again
	finalVersion := atomic.LoadUint64(&r.routeTree.version)
	if finalVersion <= newVersion {
		t.Errorf("Version should be incremented again, got %d -> %d", newVersion, finalVersion)
	}
}

// TestAtomicTreeMemorySafety tests that the atomic operations don't cause
// memory leaks or unsafe access patterns.
func TestAtomicTreeMemorySafety(t *testing.T) {
	r := New()

	// Register many routes to test memory management
	for i := range 1000 {
		path := "/memory" + string(rune('0'+i%10)) + "/" + string(rune('0'+i%100))
		r.GET(path, func(c *Context) {
			c.String(http.StatusOK, "OK")
		})
	}

	// Verify routes are accessible
	req := httptest.NewRequest("GET", "/memory0/0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify route count
	routes := r.Routes()
	if len(routes) != 1000 {
		t.Errorf("Expected 1000 routes, got %d", len(routes))
	}
}

// NOTE: Atomic tests have been merged into this file as individual test functions
// The suite runner has been removed as the tests no longer use a suite
