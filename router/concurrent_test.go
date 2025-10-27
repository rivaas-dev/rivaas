package router

import (
	"fmt"
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
