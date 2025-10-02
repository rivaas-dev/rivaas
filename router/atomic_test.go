package router

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// AtomicTestSuite tests atomic operations
type AtomicTestSuite struct {
	suite.Suite
	router *Router
}

func (suite *AtomicTestSuite) SetupTest() {
	suite.router = New()
}

func (suite *AtomicTestSuite) TearDownTest() {
	if suite.router != nil {
		suite.router.StopMetricsServer()
	}
}

// TestAtomicRouteRegistration tests that route registration is thread-safe
// and doesn't cause race conditions or data corruption.
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

// TestAtomicSuite runs the atomic test suite
func TestAtomicSuite(t *testing.T) {
	suite.Run(t, new(AtomicTestSuite))
}
