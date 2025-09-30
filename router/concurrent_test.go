package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// TestConcurrentRouteRegistration tests concurrent route registration
func TestConcurrentRouteRegistration(t *testing.T) {
	r := New()

	var wg sync.WaitGroup
	routeCount := 100

	// Register routes concurrently
	for i := range routeCount {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			r.GET("/test", func(c *Context) {
				c.String(200, "OK")
			})
		}(i)
	}

	wg.Wait()

	// Verify router is still functional
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestConcurrentConstraintAddition tests adding constraints concurrently
func TestConcurrentConstraintAddition(t *testing.T) {
	r := New()

	route := r.GET("/users/:id", func(c *Context) {
		c.String(200, "User: %s", c.Param("id"))
	})

	var wg sync.WaitGroup

	// Add constraint (should be safe due to finalized flag)
	wg.Add(1)
	go func() {
		defer wg.Done()
		route.WhereNumber("id")
	}()

	wg.Wait()

	// Test valid numeric ID
	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200 for valid ID, got %d", w.Code)
	}

	// Test invalid non-numeric ID
	req = httptest.NewRequest("GET", "/users/abc", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("Expected status 404 for invalid ID, got %d", w.Code)
	}
}

// TestConcurrentRequests tests handling concurrent requests
func TestConcurrentRequests(t *testing.T) {
	r := New()

	r.GET("/", func(c *Context) {
		c.JSON(200, map[string]string{"status": "ok"})
	})

	r.GET("/users/:id", func(c *Context) {
		c.JSON(200, map[string]string{"user_id": c.Param("id")})
	})

	var wg sync.WaitGroup
	requestCount := 1000
	errors := make(chan error, requestCount)

	for i := range requestCount {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Alternate between static and dynamic routes
			var req *http.Request
			if id%2 == 0 {
				req = httptest.NewRequest("GET", "/", nil)
			} else {
				req = httptest.NewRequest("GET", "/users/123", nil)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != 200 {
				errors <- http.ErrNotSupported
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	errorCount := 0
	for range errors {
		errorCount++
	}

	if errorCount > 0 {
		t.Errorf("Got %d errors out of %d requests", errorCount, requestCount)
	}
}

// TestConcurrentMetricsCreation tests custom metrics creation under concurrent load
func TestConcurrentMetricsCreation(t *testing.T) {
	r := New(WithMetrics())
	defer r.StopMetricsServer()

	r.GET("/test", func(c *Context) {
		// Try to create metrics concurrently
		c.IncrementCounter("test_counter")
		c.RecordMetric("test_histogram", 1.0)
		c.SetGauge("test_gauge", 100.0)
		c.JSON(200, map[string]string{"status": "ok"})
	})

	var wg sync.WaitGroup
	requestCount := 100

	for range requestCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}()
	}

	wg.Wait()
}

// TestMetricsLimitEnforcement tests that custom metrics limit is enforced
func TestMetricsLimitEnforcement(t *testing.T) {
	r := New(WithMetrics())
	defer r.StopMetricsServer()

	// Get the max limit
	maxMetrics := r.metrics.maxCustomMetrics

	// Create metrics up to the limit using valid metric names
	r.GET("/test", func(c *Context) {
		for i := range maxMetrics+10 {
			// These should succeed up to limit, then fail silently (errors logged)
			// Use valid metric names with only alphanumeric and underscore
			c.IncrementCounter("test_counter_" + fmt.Sprint(i))
		}
		c.JSON(200, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should still respond successfully even if metric creation fails
	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify we didn't exceed the limit
	counters := r.metrics.getAtomicCustomCounters()
	histograms := r.metrics.getAtomicCustomHistograms()
	gauges := r.metrics.getAtomicCustomGauges()
	totalMetrics := len(counters) + len(histograms) + len(gauges)
	if totalMetrics > maxMetrics {
		t.Errorf("Metrics limit exceeded: %d > %d", totalMetrics, maxMetrics)
	}
}

// TestResponseWriterStatusCode tests that status code is properly captured
func TestResponseWriterStatusCode(t *testing.T) {
	r := New(WithMetrics())
	defer r.StopMetricsServer()

	r.GET("/ok", func(c *Context) {
		c.JSON(200, map[string]string{"status": "ok"})
	})

	r.GET("/error", func(c *Context) {
		c.JSON(500, map[string]string{"error": "Internal error"})
	})

	// Test 200 response
	req := httptest.NewRequest("GET", "/ok", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test 500 response
	req = httptest.NewRequest("GET", "/error", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

// TestContextPoolingUnderLoad tests context pooling with many concurrent requests
func TestContextPoolingUnderLoad(t *testing.T) {
	r := New()

	r.GET("/users/:id/posts/:post_id", func(c *Context) {
		userID := c.Param("id")
		postID := c.Param("post_id")
		c.JSON(200, map[string]string{
			"user_id": userID,
			"post_id": postID,
		})
	})

	var wg sync.WaitGroup
	requestCount := 1000

	for i := range requestCount {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/users/123/posts/456", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != 200 {
				t.Errorf("Request %d: expected status 200, got %d", id, w.Code)
			}
		}(i)
	}

	wg.Wait()
}
