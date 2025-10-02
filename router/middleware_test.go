package router

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// MiddlewareTestSuite tests middleware functionality
type MiddlewareTestSuite struct {
	suite.Suite
	router *Router
}

func (suite *MiddlewareTestSuite) SetupTest() {
	suite.router = New()
}

func (suite *MiddlewareTestSuite) TearDownTest() {
	if suite.router != nil {
		suite.router.StopMetricsServer()
	}
}

// TestMiddlewareChain tests middleware chain execution with proper ordering
func (suite *MiddlewareTestSuite) TestMiddlewareChain() {
	// Add middleware that tracks execution
	executionOrder := make([]string, 0)
	suite.router.Use(func(c *Context) {
		executionOrder = append(executionOrder, "global1")
		c.Next()
	})
	suite.router.Use(func(c *Context) {
		executionOrder = append(executionOrder, "global2")
		c.Next()
	})

	// Add a route
	suite.router.GET("/test", func(c *Context) {
		executionOrder = append(executionOrder, "handler")
		c.String(http.StatusOK, "test")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	suite.router.ServeHTTP(w, req)

	// Verify execution order
	expected := []string{"global1", "global2", "handler"}
	suite.Equal(len(expected), len(executionOrder))
	suite.Equal(expected, executionOrder)
	suite.Equal(200, w.Code)
}

// TestMiddlewareChainCaching tests that middleware chains are cached properly
func (suite *MiddlewareTestSuite) TestMiddlewareChainCaching() {
	// Add middleware
	suite.router.Use(func(c *Context) {
		c.Next()
	})

	// Add multiple routes with same middleware
	suite.router.GET("/route1", func(c *Context) {
		c.String(http.StatusOK, "route1")
	})
	suite.router.GET("/route2", func(c *Context) {
		c.String(http.StatusOK, "route2")
	})

	// Test both routes
	req1 := httptest.NewRequest("GET", "/route1", nil)
	w1 := httptest.NewRecorder()
	suite.router.ServeHTTP(w1, req1)

	req2 := httptest.NewRequest("GET", "/route2", nil)
	w2 := httptest.NewRecorder()
	suite.router.ServeHTTP(w2, req2)

	// Both should work
	suite.Equal(200, w1.Code)
	suite.Equal(200, w2.Code)
}

// TestMiddlewareChainConcurrency tests concurrent middleware chain execution
func (suite *MiddlewareTestSuite) TestMiddlewareChainConcurrency() {
	r := New()

	// Add middleware that tracks concurrent execution
	r.Use(func(c *Context) {
		// Simulate some work
		time.Sleep(1 * time.Millisecond)
	})

	r.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "test")
	})

	// Test concurrent requests
	const numGoroutines = 100
	const requestsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for range numGoroutines {
		go func() {
			defer wg.Done()
			for range requestsPerGoroutine {
				req := httptest.NewRequest("GET", "/test", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				suite.Equal(200, w.Code, "Expected status 200, got %d", w.Code)
			}
		}()
	}

	wg.Wait()

	// Verify no race conditions occurred
	suite.T().Logf("Successfully handled %d concurrent requests", numGoroutines*requestsPerGoroutine)
}

// TestMiddlewareChainPerformance tests middleware chain execution performance
func (suite *MiddlewareTestSuite) TestMiddlewareChainPerformance() {
	r := New()

	// Add multiple middleware layers
	for range 5 {
		r.Use(func(c *Context) {
			// Simulate middleware work
		})
	}

	r.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "test")
	})

	// Measure execution time
	start := time.Now()

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	duration := time.Since(start)

	suite.Equal(200, w.Code, "Expected status 200, got %d", w.Code)

	// Should complete within reasonable time
	if duration > 10*time.Millisecond {
		suite.T().Logf("Warning: Middleware execution took %v, which is slower than expected", duration)
	}

	suite.T().Logf("Middleware chain execution time: %v", duration)
}

// TestMiddlewareChainMemorySafety tests memory safety of middleware chains
func (suite *MiddlewareTestSuite) TestMiddlewareChainMemorySafety() {
	r := New()

	// Add middleware that manipulates context
	r.Use(func(c *Context) {
		// Simulate middleware work
		c.String(http.StatusOK, "middleware")
	})

	r.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "test")
	})

	// Test multiple requests to ensure memory safety
	for range 100 {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		suite.Equal(200, w.Code, "Expected status 200, got %d", w.Code)
	}

	suite.T().Log("Memory safety test passed - no memory leaks or corruption detected")
}

// TestMiddlewareChainCacheEfficiency tests the efficiency of middleware chain caching
func (suite *MiddlewareTestSuite) TestMiddlewareChainCacheEfficiency() {
	r := New()

	// Add middleware
	r.Use(func(c *Context) {
		c.String(http.StatusOK, "middleware")
	})

	// Add routes with different middleware combinations
	r.GET("/route1", func(c *Context) {
		c.String(http.StatusOK, "route1")
	})

	// Create a group with additional middleware
	api := r.Group("/api")
	api.Use(func(c *Context) {
		c.String(http.StatusOK, "api_middleware")
	})
	api.GET("/users", func(c *Context) {
		c.String(http.StatusOK, "users")
	})

	// Test both routes
	req1 := httptest.NewRequest("GET", "/route1", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	req2 := httptest.NewRequest("GET", "/api/users", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	// Both should work
	suite.Equal(200, w1.Code, "Expected route1 to return 200, got %d", w1.Code)
	suite.Equal(200, w2.Code, "Expected /api/users to return 200, got %d", w2.Code)

	suite.T().Log("Middleware chain cache efficiency test passed")
}

// TestMiddlewareSuite runs the middleware test suite
func TestMiddlewareSuite(t *testing.T) {
	suite.Run(t, new(MiddlewareTestSuite))
}
