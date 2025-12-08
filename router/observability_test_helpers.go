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

package router

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
)

// mockObservabilityRecorder is a test double for ObservabilityRecorder
type mockObservabilityRecorder struct {
	enabled       bool
	shouldExclude func(path string) bool

	// Counters for verification
	startCalls  atomic.Int32
	endCalls    atomic.Int32
	wrapCalls   atomic.Int32
	loggerCalls atomic.Int32

	// Captured data
	lastStatusCode   int
	lastResponseSize int64
	lastRoutePattern string
}

func newMockObservabilityRecorder(_ bool) *mockObservabilityRecorder {
	return &mockObservabilityRecorder{
		enabled: true,
		shouldExclude: func(path string) bool {
			// Default: don't exclude anything
			return false
		},
	}
}

func (m *mockObservabilityRecorder) OnRequestStart(ctx context.Context, req *http.Request) (context.Context, any) {
	if !m.enabled {
		return ctx, nil
	}

	m.startCalls.Add(1)

	// Check if path should be excluded
	if m.shouldExclude(req.URL.Path) {
		return ctx, nil
	}

	// Return a non-nil state to indicate observability is active
	return ctx, &struct{}{}
}

func (m *mockObservabilityRecorder) WrapResponseWriter(w http.ResponseWriter, state any) http.ResponseWriter {
	m.wrapCalls.Add(1)

	if state == nil {
		return w // Don't wrap if excluded
	}

	return &mockHTTPResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (m *mockObservabilityRecorder) OnRequestEnd(ctx context.Context, state any, writer http.ResponseWriter, routePattern string) {
	if state == nil {
		return // Excluded request
	}

	m.endCalls.Add(1)

	// Extract status and size from wrapped writer
	if rw, ok := writer.(ResponseInfo); ok {
		m.lastStatusCode = rw.StatusCode()
		m.lastResponseSize = rw.Size()
	} else {
		m.lastStatusCode = http.StatusOK
		m.lastResponseSize = 0
	}
	m.lastRoutePattern = routePattern
}

func (m *mockObservabilityRecorder) BuildRequestLogger(ctx context.Context, req *http.Request, routePattern string) *slog.Logger {
	m.loggerCalls.Add(1)
	return noopLogger
}

// mockHTTPResponseWriter implements http.ResponseWriter and ResponseInfo for testing
// It also implements optional interfaces like Flusher, Hijacker for testing
type mockHTTPResponseWriter struct {
	http.ResponseWriter
	statusCode        int
	responseSize      int64
	writeHeaderCalled bool
}

func (m *mockHTTPResponseWriter) WriteHeader(code int) {
	if !m.writeHeaderCalled {
		m.statusCode = code
		m.writeHeaderCalled = true
	}
	if m.ResponseWriter != nil {
		m.ResponseWriter.WriteHeader(code)
	}
}

func (m *mockHTTPResponseWriter) Write(b []byte) (int, error) {
	if !m.writeHeaderCalled {
		m.WriteHeader(http.StatusOK)
	}
	var n int
	var err error
	if m.ResponseWriter != nil {
		n, err = m.ResponseWriter.Write(b)
	} else {
		n = len(b)
	}
	m.responseSize += int64(n)

	return n, err
}

func (m *mockHTTPResponseWriter) StatusCode() int {
	return m.statusCode
}

func (m *mockHTTPResponseWriter) Size() int64 {
	return m.responseSize
}

// Implement optional http.ResponseWriter interfaces for testing

// Flush implements http.Flusher
func (m *mockHTTPResponseWriter) Flush() {
	if f, ok := m.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements http.Hijacker
func (m *mockHTTPResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := m.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}

	return nil, nil, http.ErrNotSupported
}

// Helper to create exclusion-based mock
func newMockObservabilityWithExclusion(excludePath string) *mockObservabilityRecorder {
	mock := newMockObservabilityRecorder(true)
	mock.shouldExclude = func(path string) bool {
		return path == excludePath
	}

	return mock
}
