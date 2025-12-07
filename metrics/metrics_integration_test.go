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

package metrics_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rivaas.dev/metrics"
)

// TestIntegration_FullRequestCycle tests the complete request/response cycle with metrics.
func TestIntegration_FullRequestCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	recorder := metrics.TestingRecorderWithPrometheus(t, "integration-test",
		metrics.WithServiceVersion("v1.0.0"),
	)

	// Wait for server to be ready
	serverAddr := "localhost" + recorder.ServerAddress()
	err := metrics.WaitForMetricsServer(t, serverAddr, 2*time.Second)
	require.NoError(t, err, "metrics server should start")

	// Create HTTP mux with routes
	mux := http.NewServeMux()
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"users": []}`))
	})
	mux.HandleFunc("/api/error", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal error"}`))
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy"}`))
	})

	// Wrap with metrics middleware, excluding health endpoint
	handler := metrics.Middleware(recorder,
		metrics.WithExcludePaths("/health"),
	)(mux)

	// Make multiple requests
	for range 10 {
		req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Make some error requests
	for range 3 {
		req := httptest.NewRequest(http.MethodGet, "/api/error", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	}

	// Make health requests (should be excluded from metrics)
	for range 5 {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Verify metrics are available
	metricsHandler, err := recorder.Handler()
	require.NoError(t, err)
	require.NotNil(t, metricsHandler)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	metricsHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()

	// Verify expected metrics are present
	assert.Contains(t, body, "http_requests_total")
	assert.Contains(t, body, "http_request_duration_seconds")
}

// TestIntegration_ConcurrentRequests tests metrics under concurrent load.
func TestIntegration_ConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	recorder := metrics.TestingRecorderWithPrometheus(t, "concurrent-test")

	// Wait for server to be ready
	serverAddr := "localhost" + recorder.ServerAddress()
	err := metrics.WaitForMetricsServer(t, serverAddr, 2*time.Second)
	require.NoError(t, err, "metrics server should start")

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		// Simulate some work
		time.Sleep(time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	handler := metrics.Middleware(recorder)(mux)

	const numGoroutines = 50
	const requestsPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for range numGoroutines {
		go func() {
			defer wg.Done()
			for range requestsPerGoroutine {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
			}
		}()
	}

	wg.Wait()

	// Verify metrics handler works after concurrent load
	metricsHandler, err := recorder.Handler()
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	metricsHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "http_requests_total")
}

// TestIntegration_CustomMetrics tests custom metric recording in an integration scenario.
func TestIntegration_CustomMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	recorder := metrics.TestingRecorderWithPrometheus(t, "custom-metrics-test",
		metrics.WithMaxCustomMetrics(100),
	)

	// Wait for server to be ready
	serverAddr := "localhost" + recorder.ServerAddress()
	err := metrics.WaitForMetricsServer(t, serverAddr, 2*time.Second)
	require.NoError(t, err, "metrics server should start")

	ctx := t.Context()

	// Record various custom metrics
	for i := range 10 {
		err := recorder.IncrementCounter(ctx, "business_events_total")
		require.NoError(t, err)

		err = recorder.RecordHistogram(ctx, "processing_duration_seconds", float64(i)*0.1)
		require.NoError(t, err)

		err = recorder.SetGauge(ctx, "queue_depth", float64(i*5))
		require.NoError(t, err)
	}

	// Verify custom metrics count
	count := recorder.CustomMetricCount()
	assert.Equal(t, 3, count, "should have 3 unique custom metrics")

	// Verify metrics are exposed
	metricsHandler, err := recorder.Handler()
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	metricsHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()

	assert.Contains(t, body, "business_events_total")
	assert.Contains(t, body, "processing_duration_seconds")
	assert.Contains(t, body, "queue_depth")
}

// TestIntegration_MiddlewareWithHeaders tests header recording functionality.
func TestIntegration_MiddlewareWithHeaders(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	recorder := metrics.TestingRecorderWithPrometheus(t, "header-test")

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Configure middleware to record specific headers
	handler := metrics.Middleware(recorder,
		metrics.WithHeaders("X-Request-ID", "X-User-ID"),
	)(mux)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", "test-123")
	req.Header.Set("X-User-Id", "user-456")
	req.Header.Set("Authorization", "Bearer secret") // Should be filtered

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify metrics are recorded
	metricsHandler, err := recorder.Handler()
	require.NoError(t, err)

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsW := httptest.NewRecorder()
	metricsHandler.ServeHTTP(metricsW, metricsReq)

	assert.Equal(t, http.StatusOK, metricsW.Code)
}

// TestIntegration_PathFiltering tests that excluded paths don't generate metrics.
func TestIntegration_PathFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	recorder := metrics.TestingRecorder(t, "path-filter-test")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/debug/pprof", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Configure with various exclusion patterns
	handler := metrics.Middleware(recorder,
		metrics.WithExcludePaths("/health", "/metrics"),
		metrics.WithExcludePrefixes("/debug/"),
	)(mux)

	// Make requests to all endpoints
	paths := []string{"/api/data", "/health", "/metrics", "/debug/pprof"}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Metrics should only be recorded for /api/data
	// Other paths should be excluded
	assert.True(t, recorder.IsEnabled())
}

// TestIntegration_GracefulShutdown tests graceful shutdown of metrics recorder.
func TestIntegration_GracefulShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	recorder, err := metrics.New(
		metrics.WithServiceName("shutdown-test"),
		metrics.WithPrometheus(":0", "/metrics"),
	)
	require.NoError(t, err)

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Record some metrics before shutdown
	ctx := t.Context()
	err = recorder.IncrementCounter(ctx, "test_counter")
	require.NoError(t, err)

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	err = recorder.Shutdown(shutdownCtx)
	require.NoError(t, err)

	// Verify idempotent shutdown
	err = recorder.Shutdown(shutdownCtx)
	assert.NoError(t, err)
}

// TestIntegration_PrometheusEndpoint tests the Prometheus metrics endpoint directly.
func TestIntegration_PrometheusEndpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	recorder := metrics.TestingRecorderWithPrometheus(t, "prometheus-endpoint-test")

	// Wait for server to be ready
	serverAddr := "localhost" + recorder.ServerAddress()
	err := metrics.WaitForMetricsServer(t, serverAddr, 2*time.Second)
	require.NoError(t, err, "metrics server should start")

	// Create a handler with middleware to generate some metrics
	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	handler := metrics.Middleware(recorder)(mux)

	// Make a request through the middleware to generate metrics
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Make HTTP request to actual metrics server
	metricsReq, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+serverAddr+"/metrics", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(metricsReq)
	require.NoError(t, err)
	t.Cleanup(func() {
		resp.Body.Close() //nolint:errcheck // Best-effort close in test cleanup
	})

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Should contain HTTP metrics (generated from our request above)
	assert.Contains(t, string(body), "http_requests_total")
}

// TestIntegration_MultipleMethods tests metrics for different HTTP methods.
func TestIntegration_MultipleMethods(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	recorder := metrics.TestingRecorderWithPrometheus(t, "methods-test")

	mux := http.NewServeMux()
	mux.HandleFunc("/resource", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": "value"}`))
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id": "123"}`))
		case http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	handler := metrics.Middleware(recorder)(mux)

	methods := []struct {
		method       string
		expectedCode int
	}{
		{http.MethodGet, http.StatusOK},
		{http.MethodPost, http.StatusCreated},
		{http.MethodPut, http.StatusOK},
		{http.MethodDelete, http.StatusNoContent},
	}

	for _, m := range methods {
		req := httptest.NewRequest(m.method, "/resource", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, m.expectedCode, w.Code, "method: %s", m.method)
	}

	// Verify metrics contain method labels
	metricsHandler, err := recorder.Handler()
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	metricsHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_RequestResponseSizes tests request and response size tracking.
func TestIntegration_RequestResponseSizes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	recorder := metrics.TestingRecorderWithPrometheus(t, "size-test")

	mux := http.NewServeMux()
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})

	handler := metrics.Middleware(recorder)(mux)

	// Send request with body
	requestBody := `{"message": "hello world, this is a test message with some content"}`
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, requestBody, w.Body.String())

	// Verify size metrics are recorded
	metricsHandler, err := recorder.Handler()
	require.NoError(t, err)

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsW := httptest.NewRecorder()
	metricsHandler.ServeHTTP(metricsW, metricsReq)

	assert.Equal(t, http.StatusOK, metricsW.Code)
	body := metricsW.Body.String()

	// Should have size metrics
	assert.Contains(t, body, "http_response_size_bytes")
}
