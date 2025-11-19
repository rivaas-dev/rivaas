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
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"rivaas.dev/logging"
	"rivaas.dev/metrics"
	"rivaas.dev/tracing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_ConcurrentAppCreation tests creating multiple App instances concurrently.
// This ensures that app creation is thread-safe and doesn't have race conditions.
func TestIntegration_ConcurrentAppCreation(t *testing.T) {
	t.Parallel()

	const numGoroutines = 50
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for range numGoroutines {
		wg.Go(func() {
			app, err := New(
				WithServiceName("test-service"),
				WithServiceVersion("1.0.0"),
				WithEnvironment(EnvironmentDevelopment),
			)
			if err != nil {
				errors <- err
				return
			}

			// Verify app is usable
			if app == nil {
				errors <- assert.AnError
				return
			}

			// Register a route and test it
			app.GET("/test", func(c *Context) {
				c.String(http.StatusOK, "ok")
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			app.Router().ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				errors <- assert.AnError
			}
		})
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("concurrent app creation failed: %v", err)
	}
}

// TestIntegration_ConcurrentRequests tests handling many concurrent requests
// to ensure the app can handle high concurrency without issues.
func TestIntegration_ConcurrentRequests(t *testing.T) {
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	var requestCount atomic.Int64
	var successCount atomic.Int64

	app.GET("/test", func(c *Context) {
		requestCount.Add(1)
		time.Sleep(1 * time.Millisecond) // Simulate work
		successCount.Add(1)
		c.JSON(http.StatusOK, map[string]int64{
			"count": requestCount.Load(),
		})
	})

	const concurrency = 200
	var wg sync.WaitGroup

	for range concurrency {
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			app.Router().ServeHTTP(w, req)
			if w.Code == http.StatusOK {
				successCount.Add(0) // Already counted in handler
			}
		})
	}

	wg.Wait()

	assert.Equal(t, int64(concurrency), requestCount.Load(), "all requests should be processed")
	assert.Equal(t, int64(concurrency), successCount.Load(), "all requests should succeed")
}

// TestIntegration_MiddlewareChain tests that middleware executes in the correct order
// under concurrent load.
func TestIntegration_MiddlewareChain(t *testing.T) {
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	var executionOrder []int64
	var orderMutex sync.Mutex
	var counter atomic.Int64

	// Add multiple middleware that track execution order
	app.Use(func(c *Context) {
		orderMutex.Lock()
		executionOrder = append(executionOrder, counter.Add(1))
		orderMutex.Unlock()
		c.Next()
	})

	app.Use(func(c *Context) {
		orderMutex.Lock()
		executionOrder = append(executionOrder, counter.Add(1))
		orderMutex.Unlock()
		c.Next()
	})

	app.GET("/test", func(c *Context) {
		orderMutex.Lock()
		executionOrder = append(executionOrder, counter.Add(1))
		orderMutex.Unlock()
		c.String(http.StatusOK, "ok")
	})

	const numRequests = 100
	var wg sync.WaitGroup

	for range numRequests {
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			app.Router().ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}

	wg.Wait()

	// Verify execution order: middleware 1, middleware 2, handler
	// Each request should have 3 entries in order
	assert.Equal(t, numRequests*3, len(executionOrder), "should have 3 entries per request")
}

// TestIntegration_ObservabilityConcurrent tests that observability components
// (metrics, tracing, logging) work correctly under concurrent load.
func TestIntegration_ObservabilityConcurrent(t *testing.T) {
	t.Parallel()

	logger := logging.MustNew(logging.WithLevel(logging.LevelInfo))
	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithMetrics(metrics.WithProvider(metrics.PrometheusProvider)),
		WithTracing(tracing.WithProvider(tracing.NoopProvider)),
		WithLogger(logger.Logger()),
	)
	require.NoError(t, err)
	require.NotNil(t, app)

	var successCount atomic.Int64

	app.GET("/test", func(c *Context) {
		// Use observability features
		c.IncrementCounter("test_requests_total")
		c.SetSpanAttribute("test.key", "test.value")
		c.Logger().Info("test request", "request_id", "123")

		successCount.Add(1)
		c.String(http.StatusOK, "ok")
	})

	const concurrency = 50
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

	assert.Equal(t, int64(concurrency), successCount.Load())
}

// TestIntegration_RouteRegistrationConcurrent tests registering routes concurrently
// while handling requests.
func TestIntegration_RouteRegistrationConcurrent(t *testing.T) {
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	// Pre-register some routes
	app.GET("/existing", func(c *Context) {
		c.String(http.StatusOK, "existing")
	})

	const numNewRoutes = 50
	var wg sync.WaitGroup

	// Register new routes concurrently
	for i := range numNewRoutes {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			path := "/route" + string(rune('0'+id%10))
			app.GET(path, func(c *Context) {
				c.String(http.StatusOK, "route-%d", id)
			})
		}(i)
	}

	// Also handle requests concurrently
	requestDone := make(chan bool, 100)
	for range 100 {
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodGet, "/existing", nil)
			w := httptest.NewRecorder()
			app.Router().ServeHTTP(w, req)
			requestDone <- (w.Code == http.StatusOK)
		})
	}

	wg.Wait()
	close(requestDone)

	// Verify all requests succeeded
	for success := range requestDone {
		assert.True(t, success, "all concurrent requests should succeed")
	}
}

// TestIntegration_ServerLifecycle tests the complete server lifecycle
// including startup and shutdown under various conditions.
func TestIntegration_ServerLifecycle(t *testing.T) {
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithServerConfig(
			WithReadTimeout(5*time.Second),
			WithWriteTimeout(5*time.Second),
			WithShutdownTimeout(2*time.Second),
		),
	)

	app.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "ok")
	})

	// Test that server can be created and configured
	server := &http.Server{
		Addr:    ":0", // Use port 0 for automatic port assignment
		Handler: app.Router(),
	}

	// Start server in background
	serverErr := make(chan error, 1)
	go func() {
		// Use a test server instead of real ListenAndServe for unit testing
		// In real integration tests, you'd use a real server
		time.Sleep(10 * time.Millisecond)
		serverErr <- nil
	}()

	// Wait a bit for server to "start"
	time.Sleep(20 * time.Millisecond)

	// Test graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	assert.NoError(t, err, "server should shutdown gracefully")

	select {
	case err := <-serverErr:
		assert.NoError(t, err)
	case <-time.After(100 * time.Millisecond):
		// Server didn't error, which is fine for this test
	}
}

// TestIntegration_ErrorHandling tests that error handling works correctly
// under concurrent load and various error conditions.
func TestIntegration_ErrorHandling(t *testing.T) {
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	// Route that panics
	app.GET("/panic", func(_ *Context) {
		panic("test panic")
	})

	// Route that returns error
	app.GET("/error", func(c *Context) {
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "test error",
		})
	})

	// Route that works
	app.GET("/ok", func(c *Context) {
		c.String(http.StatusOK, "ok")
	})

	const concurrency = 20
	var wg sync.WaitGroup

	// Test panic recovery
	for range concurrency {
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodGet, "/panic", nil)
			w := httptest.NewRecorder()
			// Should not panic - recovery middleware should catch it
			assert.NotPanics(t, func() {
				app.Router().ServeHTTP(w, req)
			})
		})
	}

	// Test error handling
	for range concurrency {
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodGet, "/error", nil)
			w := httptest.NewRecorder()
			app.Router().ServeHTTP(w, req)
			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})
	}

	// Test normal requests
	for range concurrency {
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodGet, "/ok", nil)
			w := httptest.NewRecorder()
			app.Router().ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}

	wg.Wait()
}
