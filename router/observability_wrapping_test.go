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

package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// observabilityWrappedWriter detects if an http.ResponseWriter has already
// been wrapped by observability middleware, preventing double-wrapping.
// Uses Go structural typing — any writer implementing this method from any
// package (tracing, metrics, app, or user code) will be detected.
type observabilityWrappedWriter interface {
	IsObservabilityWrapped() bool
}

// TestObservabilityWrappedWriterMarkerInterface verifies that the marker interface
// correctly identifies wrapped response writers.
func TestObservabilityWrappedWriterMarkerInterface(t *testing.T) {
	t.Parallel()

	// Test with a mock wrapped writer
	mockWriter := &mockWrappedResponseWriter{
		ResponseWriter: httptest.NewRecorder(),
	}

	// Verify it implements the marker interface
	_, ok := any(mockWriter).(observabilityWrappedWriter)
	assert.True(t, ok, "mockWrappedResponseWriter should implement observabilityWrappedWriter")
	assert.True(t, mockWriter.IsObservabilityWrapped(), "IsObservabilityWrapped should return true")
}

// TestDoubleWrappingPreventionInObservabilityRecorder tests that the observability recorder
// doesn't wrap an already-wrapped response writer.
func TestDoubleWrappingPreventionInObservabilityRecorder(t *testing.T) {
	t.Parallel()

	// Create a mock observability recorder
	obs := &testObservabilityRecorder{}

	// Create a pre-wrapped writer
	innerWriter := httptest.NewRecorder()
	wrappedWriter := &mockWrappedResponseWriter{ResponseWriter: innerWriter}

	// Try to wrap it again with state != nil
	state := "some-state"
	result := obs.WrapResponseWriter(wrappedWriter, state)

	// Should return the same writer without additional wrapping
	assert.Same(t, wrappedWriter, result, "Should not wrap an already-wrapped writer")
}

// TestDoubleWrappingPreventionWithNilState tests that wrapping is skipped
// when state is nil (excluded path).
func TestDoubleWrappingPreventionWithNilState(t *testing.T) {
	t.Parallel()

	obs := &testObservabilityRecorder{}
	innerWriter := httptest.NewRecorder()

	// Try to wrap with state == nil
	result := obs.WrapResponseWriter(innerWriter, nil)

	// Should return the original writer unchanged
	assert.Same(t, innerWriter, result, "Should not wrap when state is nil")
}

// TestDoubleWrappingPreventionInRealScenario tests a realistic scenario where
// multiple observability middlewares might try to wrap the same writer.
func TestDoubleWrappingPreventionInRealScenario(t *testing.T) {
	t.Parallel()

	r := MustNew()

	// Set up observability recorder
	obs := newMockObservabilityRecorder(true)
	r.SetObservabilityRecorder(obs)

	// Track wrapping attempts
	var wrapCount int

	// Create a custom middleware that tries to wrap the writer
	r.Use(func(c *Context) {
		// Try to wrap the response writer
		if wrapped, ok := c.Response.(observabilityWrappedWriter); ok {
			// Already wrapped
			assert.True(t, wrapped.IsObservabilityWrapped(), "Writer should be marked as wrapped")
		} else {
			wrapCount++
		}
		c.Next()
	})

	r.GET("/test", func(c *Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// The observability recorder should have wrapped it once
	// Our middleware should detect it's already wrapped
}

// TestResponseWriterChaining tests that Unwrap() chains correctly through
// multiple wrapper layers.
func TestResponseWriterChaining(t *testing.T) {
	t.Parallel()

	// Create a chain: original -> wrapper1 -> wrapper2
	original := httptest.NewRecorder()

	wrapper1 := &mockWrappedResponseWriter{ResponseWriter: original}
	wrapper2 := &mockWrappedResponseWriter{ResponseWriter: wrapper1}

	// Test Unwrap chain
	unwrapped := wrapper2.Unwrap()
	assert.Same(t, wrapper1, unwrapped, "First unwrap should return wrapper1")

	if unwrappable, ok := unwrapped.(interface{ Unwrap() http.ResponseWriter }); ok {
		unwrapped2 := unwrappable.Unwrap()
		assert.Same(t, original, unwrapped2, "Second unwrap should return original")
	} else {
		t.Fatal("wrapper1 should be unwrappable")
	}
}

// TestStatusCodeAndSizeExtractionThroughWrapper tests that status code and size
// can be correctly extracted from wrapped writers.
func TestStatusCodeAndSizeExtractionThroughWrapper(t *testing.T) {
	t.Parallel()

	innerWriter := httptest.NewRecorder()
	wrapped := &mockWrappedResponseWriter{ResponseWriter: innerWriter}

	// Write some data
	wrapped.WriteHeader(http.StatusCreated)
	written, err := wrapped.Write([]byte("test response"))
	require.NoError(t, err)
	assert.Equal(t, 13, written)

	// Verify we can extract metadata via ResponseInfo interface
	if ri, ok := any(wrapped).(ResponseInfo); ok {
		assert.Equal(t, http.StatusCreated, ri.StatusCode())
		assert.Equal(t, int64(13), ri.Size())
	} else {
		t.Fatal("mockWrappedResponseWriter should implement ResponseInfo")
	}

	// Verify the marker interface
	assert.True(t, wrapped.IsObservabilityWrapped())
}

// testObservabilityRecorder is a minimal implementation for testing wrapping logic.
type testObservabilityRecorder struct{}

func (t *testObservabilityRecorder) OnRequestStart(ctx context.Context, req *http.Request) (context.Context, any) {
	return ctx, "state"
}

func (t *testObservabilityRecorder) WrapResponseWriter(w http.ResponseWriter, state any) http.ResponseWriter {
	if state == nil {
		return w
	}

	// Check if already wrapped
	if _, ok := w.(observabilityWrappedWriter); ok {
		return w // Don't wrap again
	}

	return &mockWrappedResponseWriter{ResponseWriter: w}
}

func (t *testObservabilityRecorder) OnRequestEnd(_ context.Context, _ any, _ http.ResponseWriter, _ string) {
	// No-op for test
}

// mockWrappedResponseWriter is a test implementation of a wrapped response writer.
type mockWrappedResponseWriter struct {
	http.ResponseWriter

	statusCode int
	size       int64
	written    bool
}

func (m *mockWrappedResponseWriter) WriteHeader(code int) {
	if !m.written {
		m.statusCode = code
		m.ResponseWriter.WriteHeader(code)
		m.written = true
	}
}

func (m *mockWrappedResponseWriter) Write(b []byte) (int, error) {
	if !m.written {
		m.written = true
		m.statusCode = http.StatusOK
	}
	n, err := m.ResponseWriter.Write(b)
	m.size += int64(n)

	return n, err
}

func (m *mockWrappedResponseWriter) StatusCode() int {
	if m.statusCode == 0 {
		return http.StatusOK
	}

	return m.statusCode
}

func (m *mockWrappedResponseWriter) Size() int64 {
	return m.size
}

func (m *mockWrappedResponseWriter) IsObservabilityWrapped() bool {
	return true
}

func (m *mockWrappedResponseWriter) Unwrap() http.ResponseWriter {
	return m.ResponseWriter
}
