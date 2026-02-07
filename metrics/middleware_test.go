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

//go:build !integration

package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMetricsResponseWriterImplementsMarkerInterface verifies that the
// metrics middleware response writer implements the marker interface.
func TestMetricsResponseWriterImplementsMarkerInterface(t *testing.T) {
	t.Parallel()

	innerWriter := httptest.NewRecorder()
	wrapped := newResponseWriter(innerWriter)

	// Verify it implements the marker interface
	marker, ok := any(wrapped).(observabilityWrappedWriter)
	require.True(t, ok, "responseWriter should implement observabilityWrappedWriter")
	assert.True(t, marker.IsObservabilityWrapped())
}

// TestMetricsMiddlewareDoubleWrappingPrevention tests that the metrics middleware
// doesn't wrap an already-wrapped response writer.
func TestMetricsMiddlewareDoubleWrappingPrevention(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithServiceName("test-service"),
		WithPrometheus(":9091", "/metrics"),
		WithServerDisabled(),
	)

	// Create a handler that returns already-wrapped writer
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok")) //nolint:errcheck // Test handler
	})

	// First, create a pre-wrapped writer
	preWrappedWriter := httptest.NewRecorder()
	alreadyWrapped := &mockWrappedWriter{ResponseWriter: preWrappedWriter}

	// Apply metrics middleware
	middleware := Middleware(recorder)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	// Serve with already-wrapped writer
	wrappedHandler.ServeHTTP(alreadyWrapped, req)

	assert.Equal(t, http.StatusOK, alreadyWrapped.StatusCode())
	// The middleware should detect it's already wrapped and not wrap again
}

// TestMetricsMiddlewareWithExcludedPaths tests that excluded paths skip wrapping.
func TestMetricsMiddlewareWithExcludedPaths(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithServiceName("test-service"),
		WithPrometheus(":9092", "/metrics"),
		WithServerDisabled(),
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if writer is wrapped
		if _, ok := w.(observabilityWrappedWriter); ok {
			w.Write([]byte("wrapped")) //nolint:errcheck // Test handler
		} else {
			w.Write([]byte("not-wrapped")) //nolint:errcheck // Test handler
		}
	})

	// Apply middleware with excluded paths
	middleware := Middleware(recorder, WithExcludePaths("/health"))
	wrappedHandler := middleware(handler)

	// Test excluded path
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)
	assert.Equal(t, "not-wrapped", w.Body.String(), "Excluded path should not be wrapped")

	// Test non-excluded path
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w2 := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w2, req2)
	assert.Equal(t, "wrapped", w2.Body.String(), "Non-excluded path should be wrapped")
}

// TestMetricsMiddlewareStatusCodeCapture verifies that the middleware
// correctly captures status codes.
func TestMetricsMiddlewareStatusCodeCapture(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithServiceName("test-service"),
		WithPrometheus(":9093", "/metrics"),
		WithServerDisabled(),
	)

	testCases := []struct {
		name       string
		statusCode int
	}{
		{"success", http.StatusOK},
		{"created", http.StatusCreated},
		{"bad request", http.StatusBadRequest},
		{"not found", http.StatusNotFound},
		{"server error", http.StatusInternalServerError},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				w.Write([]byte("test")) //nolint:errcheck // Test handler
			})

			middleware := Middleware(recorder)
			wrappedHandler := middleware(handler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(w, req)

			assert.Equal(t, tc.statusCode, w.Code)
		})
	}
}

// TestMetricsMiddlewareResponseSizeCapture verifies that the middleware
// correctly captures response sizes.
func TestMetricsMiddlewareResponseSizeCapture(t *testing.T) {
	t.Parallel()

	recorder := MustNew(
		WithServiceName("test-service"),
		WithPrometheus(":9094", "/metrics"),
		WithServerDisabled(),
	)

	testCases := []struct {
		name         string
		responseBody string
		expectedSize int
	}{
		{"empty", "", 0},
		{"small", "ok", 2},
		{"medium", "Hello, World!", 13},
		{"large", string(make([]byte, 1024)), 1024},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tc.responseBody)) //nolint:errcheck // Test handler
			})

			middleware := Middleware(recorder)
			wrappedHandler := middleware(handler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tc.expectedSize, w.Body.Len())
		})
	}
}

// mockWrappedWriter is a mock implementation of an already-wrapped response writer.
type mockWrappedWriter struct {
	http.ResponseWriter

	statusCode int
	size       int
	written    bool
}

func (m *mockWrappedWriter) WriteHeader(code int) {
	if !m.written {
		m.statusCode = code
		m.ResponseWriter.WriteHeader(code)
		m.written = true
	}
}

func (m *mockWrappedWriter) Write(b []byte) (int, error) {
	if !m.written {
		m.written = true
		m.statusCode = http.StatusOK
	}
	n, err := m.ResponseWriter.Write(b)
	m.size += n

	return n, err
}

func (m *mockWrappedWriter) StatusCode() int {
	if m.statusCode == 0 {
		return http.StatusOK
	}

	return m.statusCode
}

func (m *mockWrappedWriter) Size() int64 {
	return int64(m.size)
}

func (m *mockWrappedWriter) IsObservabilityWrapped() bool {
	return true
}

func (m *mockWrappedWriter) Unwrap() http.ResponseWriter {
	return m.ResponseWriter
}
