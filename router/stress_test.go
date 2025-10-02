package router

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// StressTestSuite tests router under high load
type StressTestSuite struct {
	suite.Suite
	router *Router
}

func (suite *StressTestSuite) SetupTest() {
	suite.router = New()
}

func (suite *StressTestSuite) TearDownTest() {
	if suite.router != nil {
		suite.router.StopMetricsServer()
	}
}

// TestRouterStress tests the router under high concurrent load
func (suite *StressTestSuite) TestRouterStress() {
	// Add many routes to test scalability
	for i := range 100 {
		route := "/api/v1/users/" + string(rune('a'+i%26))
		suite.router.GET(route, func(c *Context) {
			c.String(http.StatusOK, "User")
		})
	}

	// Add parameter routes
	for range 50 {
		route := "/api/v1/users/:id/posts/:post_id"
		suite.router.GET(route, func(c *Context) {
			c.String(http.StatusOK, "User: %s, Post: %s", c.Param("id"), c.Param("post_id"))
		})
	}

	// Test concurrent requests
	var wg sync.WaitGroup
	concurrency := 100
	requests := 1000

	start := time.Now()

	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range requests / concurrency {
				req := httptest.NewRequest("GET", "/api/v1/users/123/posts/456", nil)
				w := httptest.NewRecorder()
				suite.router.ServeHTTP(w, req)

				suite.Equal(http.StatusOK, w.Code)
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	suite.T().Logf("Processed %d requests with %d concurrent goroutines in %v",
		requests, concurrency, duration)
	suite.T().Logf("Average: %v per request", duration/time.Duration(requests))
	suite.T().Logf("Throughput: %.2f requests/second",
		float64(requests)/duration.Seconds())
}

// BenchmarkRouterConcurrent benchmarks concurrent requests
func BenchmarkRouterConcurrent(b *testing.B) {
	r := New()
	r.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "User: %s", c.Param("id"))
	})

	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest("GET", "/users/123", nil)
		w := httptest.NewRecorder()

		for pb.Next() {
			r.ServeHTTP(w, req)
		}
	})
}

// BenchmarkRouterMemoryAllocations tests memory efficiency
func BenchmarkRouterMemoryAllocations(b *testing.B) {
	r := New()
	r.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "User: %s", c.Param("id"))
	})

	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		r.ServeHTTP(w, req)
	}
}

// TestStressSuite runs the stress test suite
func TestStressSuite(t *testing.T) {
	suite.Run(t, new(StressTestSuite))
}
