package router

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestAtomicMetricsOperations tests atomic metrics operations for thread safety
func TestAtomicMetricsOperations(t *testing.T) {
	// Create router with metrics enabled
	r := New(WithMetrics())

	// Add a test route
	r.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "test")
	})

	// Test concurrent metrics recording
	const numGoroutines = 100
	const requestsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Start concurrent requests
	for range numGoroutines {
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				req := httptest.NewRequest("GET", "/test", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
			}
		}()
	}

	// Wait for all requests to complete
	wg.Wait()

	// Give metrics time to be recorded
	time.Sleep(100 * time.Millisecond)

	// Verify atomic counters are working
	if r.metrics == nil {
		t.Fatal("Metrics not initialized")
	}

	// Check that atomic counters have been incremented
	requestCount := r.metrics.getAtomicRequestCount()
	if requestCount == 0 {
		t.Error("Expected atomic request count to be > 0")
	}

	activeRequests := r.metrics.getAtomicActiveRequests()
	if activeRequests != 0 {
		t.Errorf("Expected active requests to be 0, got %d", activeRequests)
	}

	t.Logf("Atomic request count: %d", requestCount)
	t.Logf("Atomic active requests: %d", activeRequests)
}

// TestAtomicCustomMetrics tests atomic custom metrics operations
func TestAtomicCustomMetrics(t *testing.T) {
	// Create router with metrics enabled
	r := New(WithMetrics())

	// Add a test route that records custom metrics
	r.GET("/test", func(c *Context) {
		c.IncrementCounter("test_counter")
		c.RecordMetric("test_histogram", 1.5)
		c.SetGauge("test_gauge", 42.0)
		c.String(http.StatusOK, "test")
	})

	// Test concurrent custom metrics recording
	const numGoroutines = 50
	const requestsPerGoroutine = 5

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Start concurrent requests
	for range numGoroutines {
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				req := httptest.NewRequest("GET", "/test", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
			}
		}()
	}

	// Wait for all requests to complete
	wg.Wait()

	// Give metrics time to be recorded
	time.Sleep(100 * time.Millisecond)

	// Verify that custom metrics were created atomically
	if r.metrics == nil {
		t.Fatal("Metrics not initialized")
	}

	// Check that custom metrics maps exist
	counters := r.metrics.getAtomicCustomCounters()
	histograms := r.metrics.getAtomicCustomHistograms()
	gauges := r.metrics.getAtomicCustomGauges()

	if len(counters) == 0 {
		t.Error("Expected custom counters to be created")
	}
	if len(histograms) == 0 {
		t.Error("Expected custom histograms to be created")
	}
	if len(gauges) == 0 {
		t.Error("Expected custom gauges to be created")
	}

	t.Logf("Custom counters: %d", len(counters))
	t.Logf("Custom histograms: %d", len(histograms))
	t.Logf("Custom gauges: %d", len(gauges))
}

// TestAtomicMetricsConsistency tests that atomic metrics remain consistent under concurrent access
func TestAtomicMetricsConsistency(t *testing.T) {
	// Create router with metrics enabled
	r := New(WithMetrics())

	// Add a test route
	r.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "test")
	})

	// Test concurrent metrics access
	const numGoroutines = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Start concurrent requests
	for range numGoroutines {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}()
	}

	// Wait for all requests to complete
	wg.Wait()

	// Give metrics time to be recorded
	time.Sleep(100 * time.Millisecond)

	// Verify metrics consistency
	if r.metrics == nil {
		t.Fatal("Metrics not initialized")
	}

	// Check that atomic counters are consistent
	requestCount := r.metrics.getAtomicRequestCount()
	activeRequests := r.metrics.getAtomicActiveRequests()

	// Active requests should be 0 after all requests complete
	if activeRequests != 0 {
		t.Errorf("Expected active requests to be 0, got %d", activeRequests)
	}

	// Request count should be positive
	if requestCount <= 0 {
		t.Errorf("Expected request count to be > 0, got %d", requestCount)
	}

	t.Logf("Final request count: %d", requestCount)
	t.Logf("Final active requests: %d", activeRequests)
}

// TestAtomicMetricsMemorySafety tests that atomic metrics operations are memory safe
func TestAtomicMetricsMemorySafety(t *testing.T) {
	// Create router with metrics enabled
	r := New(WithMetrics())

	// Add a test route
	r.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "test")
	})

	// Test concurrent metrics access with race detection
	const numGoroutines = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Start concurrent requests
	for range numGoroutines {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}()
	}

	// Wait for all requests to complete
	wg.Wait()

	// Give metrics time to be recorded
	time.Sleep(100 * time.Millisecond)

	// Verify that no race conditions occurred
	// This test will fail if race conditions are detected by the race detector
	if r.metrics == nil {
		t.Fatal("Metrics not initialized")
	}

	// Access atomic counters to ensure they're safe
	_ = r.metrics.getAtomicRequestCount()
	_ = r.metrics.getAtomicActiveRequests()
	_ = r.metrics.getAtomicErrorCount()
	_ = r.metrics.getAtomicContextPoolHits()
	_ = r.metrics.getAtomicContextPoolMisses()

	t.Log("Atomic metrics memory safety test passed")
}
