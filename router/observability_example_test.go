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

package router_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"rivaas.dev/router"
)

// Example_simpleObservabilityRecorder demonstrates a minimal implementation of router.ObservabilityRecorder.
// This example shows the basic structure and lifecycle of observability integration.
//
// Note: This is a reference implementation showing the ObservabilityRecorder interface.
// Integration with the router is done at the app level - see app/observability.go for usage.
func Example_simpleObservabilityRecorder() {
	// Create a simple observability recorder (with discarded output for example)
	obs := &SimpleObservabilityRecorder{
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
		excludePaths: map[string]bool{
			"/health": true,
			"/ready":  true,
		},
	}

	// Example request simulation
	req := httptest.NewRequest("GET", "/users/123", nil)
	ctx := req.Context()

	// 1. OnRequestStart - called before routing
	enrichedCtx, state := obs.OnRequestStart(ctx, req)
	if state != nil {
		// 2. WrapResponseWriter - capture response metadata
		w := httptest.NewRecorder()
		wrapped := obs.WrapResponseWriter(w, state)

		// 3. BuildRequestLogger - create request-scoped logger
		logger := obs.BuildRequestLogger(enrichedCtx, req, "/users/:id")
		logger.Info("processing request")

		// Simulate handler writing response
		if rw, ok := wrapped.(*observableResponseWriter); ok {
			rw.WriteHeader(200)
			_, _ = rw.Write([]byte(`{"status":"ok"}`))
		}

		// 4. OnRequestEnd - called after request completes
		obs.OnRequestEnd(enrichedCtx, state, wrapped, "/users/:id")
	}

	fmt.Println("Observability lifecycle complete")

	// Output:
	// Observability lifecycle complete
}

// SimpleObservabilityRecorder is a minimal reference implementation of router.ObservabilityRecorder.
// It demonstrates the basic lifecycle and how to implement the three observability pillars:
//   - Metrics: Track request counts, durations, status codes
//   - Tracing: Add request IDs and track request flow
//   - Logging: Provide request-scoped structured logging
type SimpleObservabilityRecorder struct {
	logger       *slog.Logger
	excludePaths map[string]bool
}

// requestState holds per-request observability state.
// This is the opaque state token passed between lifecycle methods.
type requestState struct {
	startTime time.Time
	requestID string
	path      string
}

// OnRequestStart is called before routing begins.
// It enriches the context with a request ID and decides whether to track this request.
func (s *SimpleObservabilityRecorder) OnRequestStart(ctx context.Context, req *http.Request) (context.Context, any) {
	// Check if this path should be excluded from observability
	if s.excludePaths[req.URL.Path] {
		// Return enriched context but nil state to exclude from tracking
		// This still allows trace propagation but skips metrics/logging
		return ctx, nil
	}

	// Create per-request state
	state := &requestState{
		startTime: time.Now(),
		requestID: generateRequestID(),
		path:      req.URL.Path,
	}

	// Enrich context with request ID for distributed tracing
	ctx = context.WithValue(ctx, "request_id", state.requestID)

	// Return both enriched context and state for tracking
	return ctx, state
}

// WrapResponseWriter wraps the http.ResponseWriter to capture response metadata.
// This is essential for collecting metrics like status codes and response sizes.
func (s *SimpleObservabilityRecorder) WrapResponseWriter(w http.ResponseWriter, state any) http.ResponseWriter {
	if state == nil {
		// Excluded request - return original writer unchanged
		return w
	}

	// Wrap the writer to capture status and size
	return &observableResponseWriter{
		ResponseWriter: w,
		status:         200, // Default status
	}
}

// OnRequestEnd is called after request handling completes.
// This is where we record metrics, log access entries, and finish traces.
func (s *SimpleObservabilityRecorder) OnRequestEnd(ctx context.Context, state any, writer http.ResponseWriter, routePattern string) {
	if state == nil {
		// This should never happen (excluded requests don't reach here)
		return
	}

	rs := state.(*requestState)
	duration := time.Since(rs.startTime)

	// Extract response metadata
	var status int
	var size int64
	if rw, ok := writer.(router.ResponseInfo); ok {
		status = rw.StatusCode()
		size = rw.Size()
	}

	// Log access entry (structured logging)
	s.logger.Info("request completed",
		"request_id", rs.requestID,
		"route", routePattern,
		"path", rs.path,
		"status", status,
		"size_bytes", size,
		"duration_ms", duration.Milliseconds(),
	)

	// In a real implementation, you would also:
	// - Record metrics (request count, duration histogram, status code counter)
	// - Finish distributed tracing span
	// - Update dashboards/monitoring systems
}

// BuildRequestLogger creates a request-scoped logger with HTTP metadata.
// This logger is available to handlers for business logic logging.
func (s *SimpleObservabilityRecorder) BuildRequestLogger(ctx context.Context, req *http.Request, routePattern string) *slog.Logger {
	// Extract request ID from context (added in OnRequestStart)
	requestID, _ := ctx.Value("request_id").(string)

	// Create logger with request context
	return s.logger.With(
		"request_id", requestID,
		"route", routePattern,
		"method", req.Method,
		"path", req.URL.Path,
		"remote_addr", req.RemoteAddr,
	)
}

// observableResponseWriter wraps http.ResponseWriter to capture response metadata.
type observableResponseWriter struct {
	http.ResponseWriter
	status      int
	size        int64
	wroteHeader bool
}

func (w *observableResponseWriter) WriteHeader(status int) {
	if !w.wroteHeader {
		w.status = status
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *observableResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.size += int64(n)
	return n, err
}

func (w *observableResponseWriter) StatusCode() int {
	return w.status
}

func (w *observableResponseWriter) Size() int64 {
	return w.size
}

// Helper to generate request IDs (simplified for example)
func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}
