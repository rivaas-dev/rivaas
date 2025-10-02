package router

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// AtomicMetricsTestSuite tests atomic metrics operations
type AtomicMetricsTestSuite struct {
	suite.Suite
	router *Router
}

func (suite *AtomicMetricsTestSuite) SetupTest() {
	suite.router = New(WithMetrics())
}

func (suite *AtomicMetricsTestSuite) TearDownTest() {
	if suite.router != nil {
		suite.router.StopMetricsServer()
	}
}

// TestAtomicMetricsOperations tests atomic metrics operations for thread safety
func (suite *AtomicMetricsTestSuite) TestAtomicMetricsOperations() {
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
	suite.NotNil(r.metrics, "Metrics not initialized")

	// Check that atomic counters have been incremented
	requestCount := r.metrics.getAtomicRequestCount()
	suite.Greater(requestCount, int64(0), "Expected atomic request count to be > 0")

	activeRequests := r.metrics.getAtomicActiveRequests()
	suite.Equal(int64(0), activeRequests, "Expected active requests to be 0, got %d", activeRequests)

	suite.T().Logf("Atomic request count: %d", requestCount)
	suite.T().Logf("Atomic active requests: %d", activeRequests)
}

// TestAtomicCustomMetrics tests atomic custom metrics operations
func (suite *AtomicMetricsTestSuite) TestAtomicCustomMetrics() {
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
	suite.NotNil(r.metrics, "Metrics not initialized")

	// Check that custom metrics maps exist
	counters := r.metrics.getAtomicCustomCounters()
	histograms := r.metrics.getAtomicCustomHistograms()
	gauges := r.metrics.getAtomicCustomGauges()

	suite.Greater(len(counters), 0, "Expected custom counters to be created")
	suite.Greater(len(histograms), 0, "Expected custom histograms to be created")
	suite.Greater(len(gauges), 0, "Expected custom gauges to be created")

	suite.T().Logf("Custom counters: %d", len(counters))
	suite.T().Logf("Custom histograms: %d", len(histograms))
	suite.T().Logf("Custom gauges: %d", len(gauges))
}

// TestAtomicMetricsConsistency tests that atomic metrics remain consistent under concurrent access
func (suite *AtomicMetricsTestSuite) TestAtomicMetricsConsistency() {
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
	suite.NotNil(r.metrics, "Metrics not initialized")

	// Check that atomic counters are consistent
	requestCount := r.metrics.getAtomicRequestCount()
	activeRequests := r.metrics.getAtomicActiveRequests()

	// Active requests should be 0 after all requests complete
	suite.Equal(int64(0), activeRequests, "Expected active requests to be 0, got %d", activeRequests)

	// Request count should be positive
	suite.Greater(requestCount, int64(0), "Expected request count to be > 0, got %d", requestCount)

	suite.T().Logf("Final request count: %d", requestCount)
	suite.T().Logf("Final active requests: %d", activeRequests)
}

// TestAtomicMetricsMemorySafety tests that atomic metrics operations are memory safe
func (suite *AtomicMetricsTestSuite) TestAtomicMetricsMemorySafety() {
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
	suite.NotNil(r.metrics, "Metrics not initialized")

	// Access atomic counters to ensure they're safe
	_ = r.metrics.getAtomicRequestCount()
	_ = r.metrics.getAtomicActiveRequests()
	_ = r.metrics.getAtomicErrorCount()
	_ = r.metrics.getAtomicContextPoolHits()
	_ = r.metrics.getAtomicContextPoolMisses()

	suite.T().Log("Atomic metrics memory safety test passed")
}

// TestAtomicMetricsSuite runs the atomic metrics test suite
func TestAtomicMetricsSuite(t *testing.T) {
	suite.Run(t, new(AtomicMetricsTestSuite))
}
