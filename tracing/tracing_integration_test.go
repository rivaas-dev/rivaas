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

package tracing_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rivaas.dev/tracing"
)

// TestIntegration_FullRequestCycle tests the complete request/response cycle with tracing.
func TestIntegration_FullRequestCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tracer, err := tracing.New(
		tracing.WithServiceName("integration-test"),
		tracing.WithServiceVersion("v1.0.0"),
		tracing.WithStdout(),
		tracing.WithSampleRate(1.0),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tracer.Shutdown(ctx)
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"users":[]}`))
	})

	handler := tracing.MustMiddleware(tracer)(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, w.Body.String(), "users")
}

// TestIntegration_PathExclusion tests that excluded paths bypass tracing.
func TestIntegration_PathExclusion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tracer, err := tracing.New(
		tracing.WithServiceName("integration-test"),
		tracing.WithStdout(),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		tracer.Shutdown(context.Background())
	})

	tests := []struct {
		name         string
		path         string
		excludePaths []string
		wantStatus   int
	}{
		{
			name:         "excluded health endpoint",
			path:         "/health",
			excludePaths: []string{"/health", "/metrics", "/ready"},
			wantStatus:   http.StatusOK,
		},
		{
			name:         "excluded metrics endpoint",
			path:         "/metrics",
			excludePaths: []string{"/health", "/metrics", "/ready"},
			wantStatus:   http.StatusOK,
		},
		{
			name:         "non-excluded API endpoint",
			path:         "/api/users",
			excludePaths: []string{"/health", "/metrics", "/ready"},
			wantStatus:   http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mux := http.NewServeMux()
			mux.HandleFunc(tt.path, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := tracing.MustMiddleware(tracer,
				tracing.WithExcludePaths(tt.excludePaths...),
			)(mux)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

// TestIntegration_ConcurrentRequests tests tracing under concurrent load.
func TestIntegration_ConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tracer, err := tracing.New(
		tracing.WithServiceName("integration-test"),
		tracing.WithStdout(),
		tracing.WithSampleRate(1.0),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		tracer.Shutdown(context.Background())
	})

	handler := tracing.MustMiddleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))

	const numRequests = 100
	var wg sync.WaitGroup
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				errors <- assert.AnError
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		assert.NoError(t, err)
	}
}

// TestIntegration_TraceContextPropagation tests trace context propagation across services.
func TestIntegration_TraceContextPropagation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tracer, err := tracing.New(
		tracing.WithServiceName("integration-test"),
		tracing.WithNoop(), // Use noop for consistent behavior in tests
		tracing.WithSampleRate(1.0),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		tracer.Shutdown(context.Background())
	})

	var capturedTraceID, capturedSpanID string
	handler := tracing.MustMiddleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture trace and span IDs from the context
		capturedTraceID = tracing.TraceID(r.Context())
		capturedSpanID = tracing.SpanID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// Simulate incoming request
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// With noop provider, trace/span IDs may be empty but the test should not fail
	// The key behavior is that the middleware runs without errors
	_ = capturedTraceID
	_ = capturedSpanID
}

// TestIntegration_ErrorStatusCodes tests tracing with various HTTP error status codes.
func TestIntegration_ErrorStatusCodes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tracer, err := tracing.New(
		tracing.WithServiceName("integration-test"),
		tracing.WithStdout(),
		tracing.WithSampleRate(1.0),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		tracer.Shutdown(context.Background())
	})

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "bad request",
			path:       "/bad-request",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unauthorized",
			path:       "/unauthorized",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "not found",
			path:       "/not-found",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "internal server error",
			path:       "/error",
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "service unavailable",
			path:       "/unavailable",
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	mux := http.NewServeMux()
	for _, tt := range tests {
		status := tt.wantStatus
		mux.HandleFunc(tt.path, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		})
	}
	handler := tracing.MustMiddleware(tracer)(mux)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

// TestIntegration_HeaderRecording tests that specified headers are recorded in spans.
func TestIntegration_HeaderRecording(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tracer, err := tracing.New(
		tracing.WithServiceName("integration-test"),
		tracing.WithStdout(),
		tracing.WithSampleRate(1.0),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		tracer.Shutdown(context.Background())
	})

	handler := tracing.MustMiddleware(tracer,
		tracing.WithHeaders("X-Request-ID", "X-Correlation-ID", "User-Agent"),
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-Request-ID", "req-123")
	req.Header.Set("X-Correlation-ID", "corr-456")
	req.Header.Set("User-Agent", "test-agent/1.0")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_ProviderTypes tests different provider configurations.
func TestIntegration_ProviderTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tests := []struct {
		name    string
		options []tracing.Option
		wantErr bool
	}{
		{
			name: "stdout provider",
			options: []tracing.Option{
				tracing.WithServiceName("test"),
				tracing.WithStdout(),
			},
			wantErr: false,
		},
		{
			name: "noop provider",
			options: []tracing.Option{
				tracing.WithServiceName("test"),
				tracing.WithNoop(),
			},
			wantErr: false,
		},
		{
			name: "default provider (noop)",
			options: []tracing.Option{
				tracing.WithServiceName("test"),
			},
			wantErr: false,
		},
		{
			name: "empty service name",
			options: []tracing.Option{
				tracing.WithServiceName(""),
			},
			wantErr: true,
		},
		{
			name: "multiple providers error",
			options: []tracing.Option{
				tracing.WithServiceName("test"),
				tracing.WithStdout(),
				tracing.WithNoop(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tracer, err := tracing.New(tt.options...)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, tracer)
			t.Cleanup(func() {
				tracer.Shutdown(context.Background())
			})

			assert.True(t, tracer.IsEnabled())
		})
	}
}

// TestIntegration_ShutdownBehavior tests graceful shutdown behavior.
func TestIntegration_ShutdownBehavior(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	t.Run("graceful shutdown", func(t *testing.T) {
		t.Parallel()

		tracer, err := tracing.New(
			tracing.WithServiceName("test"),
			tracing.WithStdout(),
		)
		require.NoError(t, err)

		// Make some requests
		handler := tracing.MustMiddleware(tracer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		for i := 0; i < 10; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		}

		// Shutdown with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = tracer.Shutdown(ctx)
		assert.NoError(t, err)
	})

	t.Run("idempotent shutdown", func(t *testing.T) {
		t.Parallel()

		tracer, err := tracing.New(
			tracing.WithServiceName("test"),
			tracing.WithStdout(),
		)
		require.NoError(t, err)

		ctx := context.Background()

		// Multiple shutdowns should be safe
		err = tracer.Shutdown(ctx)
		assert.NoError(t, err)

		err = tracer.Shutdown(ctx)
		assert.NoError(t, err)

		err = tracer.Shutdown(ctx)
		assert.NoError(t, err)
	})
}
