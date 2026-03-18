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
	"io"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/trace"

	"rivaas.dev/metrics"
	"rivaas.dev/router"
	"rivaas.dev/tracing"

	stderrors "errors"
)

// observabilityWrappedWriter detects if an http.ResponseWriter has already
// been wrapped by observability middleware, preventing double-wrapping.
// Uses Go structural typing — any writer implementing this method from any
// package (tracing, metrics, app, or user code) will be detected.
type observabilityWrappedWriter interface {
	IsObservabilityWrapped() bool
}

// observabilityRecorder implements [router.ObservabilityRecorder] by unifying
// metrics, tracing, and logging into a single lifecycle.
// It coordinates observability data collection across all three pillars.
type observabilityRecorder struct {
	metrics    *metrics.Recorder
	tracing    *tracing.Tracer
	logger     *slog.Logger
	pathFilter *pathFilter

	// Access log configuration
	logAccessRequests bool
	logErrorsOnly     bool
	slowThreshold     time.Duration
}

// observabilityConfig configures the unified observability recorder.
type observabilityConfig struct {
	metrics           *metrics.Recorder
	tracing           *tracing.Tracer
	logger            *slog.Logger
	pathFilter        *pathFilter
	logAccessRequests bool
	logErrorsOnly     bool
	slowThreshold     time.Duration
}

// newObservabilityRecorder creates an [observabilityRecorder] from configuration.
func newObservabilityRecorder(cfg *observabilityConfig) router.ObservabilityRecorder {
	pf := cfg.pathFilter
	if pf == nil {
		pf = newPathFilterWithDefaults()
	}

	return &observabilityRecorder{
		metrics:           cfg.metrics,
		tracing:           cfg.tracing,
		logger:            cfg.logger,
		pathFilter:        pf,
		logAccessRequests: cfg.logAccessRequests,
		logErrorsOnly:     cfg.logErrorsOnly,
		slowThreshold:     cfg.slowThreshold,
	}
}

// observabilityState is the opaque token holding per-request observability state
// passed between lifecycle methods.
type observabilityState struct {
	metricsData *metrics.RequestMetrics // Metrics state from metrics.BeginRequest
	span        trace.Span              // Active span from tracing
	startTime   time.Time               // Request start time for duration calculation
	req         *http.Request           // Original request for access logging
}

func (o *observabilityRecorder) OnRequestStart(ctx context.Context, req *http.Request) (context.Context, any) {
	// Single source of truth for exclusions
	if o.pathFilter != nil && o.pathFilter.shouldExclude(req.URL.Path) {
		return ctx, nil // Excluded: skip all observability
	}

	state := &observabilityState{
		startTime: time.Now(),
		req:       req, // Store for later use
	}

	// Start tracing (if enabled)
	// Use StartRequestSpan for W3C propagation, sampling, and standard HTTP attributes.
	// Note: We start with raw path; will rename span to route pattern in OnRequestEnd
	if o.tracing != nil && o.tracing.IsEnabled() {
		ctx, state.span = o.tracing.StartRequestSpan(ctx, req, req.URL.Path, false)
	}

	// Start metrics (if enabled)
	// Note: We'll update with route pattern in OnRequestEnd for cardinality control
	if o.metrics != nil && o.metrics.IsEnabled() {
		state.metricsData = o.metrics.BeginRequest(ctx)
	}

	return ctx, state
}

func (o *observabilityRecorder) WrapResponseWriter(w http.ResponseWriter, state any) http.ResponseWriter {
	if state == nil {
		return w // Excluded: don't wrap
	}

	// Check if already wrapped by observability middleware
	// This prevents double-wrapping when combining app observability with standalone middleware
	if _, ok := w.(observabilityWrappedWriter); ok {
		return w // Already wrapped, don't wrap again
	}

	return &observabilityResponseWriter{ResponseWriterWrapper: router.NewResponseWriterWrapper(w)}
}

func (o *observabilityRecorder) OnRequestEnd(ctx context.Context, state any, writer http.ResponseWriter, routePattern string) {
	s, ok := state.(*observabilityState)
	if !ok || s == nil {
		return // Excluded or invalid state
	}

	duration := time.Since(s.startTime)

	// Extract response metadata from wrapped writer
	statusCode := http.StatusOK
	var responseSize int64 = 0
	if ri, riOk := writer.(router.ResponseInfo); riOk {
		statusCode = ri.StatusCode()
		responseSize = ri.Size()
	}

	// Update span name to use route pattern (better cardinality)
	if s.span != nil && s.span.IsRecording() && routePattern != "" {
		spanName := s.req.Method + " " + routePattern
		s.span.SetName(spanName)
	}

	// Finish tracing (sets http.status_code and invokes span finish hook if configured)
	if s.span != nil {
		o.tracing.FinishRequestSpan(s.span, statusCode)
	}

	// Finish metrics with route pattern (prevents cardinality explosion)
	if s.metricsData != nil {
		// Use routePattern for metrics to avoid high cardinality
		// If no route matched, use sentinel value
		route := routePattern
		if route == "" {
			route = "_unmatched"
		}
		o.metrics.Finish(ctx, s.metricsData, statusCode, responseSize, route)
	}

	// Access logging (if enabled)
	if o.logAccessRequests && o.logger != nil {
		o.logAccessRequest(ctx, s.req, statusCode, responseSize, duration, routePattern)
	}
}

func (o *observabilityRecorder) logAccessRequest(
	ctx context.Context,
	req *http.Request,
	statusCode int,
	responseSize int64,
	duration time.Duration,
	routePattern string,
) {
	isError := statusCode >= 400
	isSlow := o.slowThreshold > 0 && duration >= o.slowThreshold

	// Skip non-errors if error-only mode (unless slow)
	if o.logErrorsOnly && !isError && !isSlow {
		return
	}

	// Build structured log fields
	fields := []any{
		"method", req.Method,
		"path", req.URL.Path,
		"status", statusCode,
		"duration_ms", duration.Milliseconds(),
		"bytes_sent", responseSize,
		"user_agent", req.UserAgent(),
		"remote_addr", req.RemoteAddr, // Added: raw remote address
		"host", req.Host,
		"proto", req.Proto,
	}

	// Add route template (key for aggregation)
	if routePattern != "" {
		fields = append(fields, "route", routePattern)
	}

	// Add request ID (for correlation)
	if reqID := req.Header.Get("X-Request-ID"); reqID != "" {
		fields = append(fields, "request_id", reqID)
	}

	// Mark slow requests explicitly
	if isSlow {
		fields = append(fields, "slow", true)
	}

	// Log at appropriate level
	// Note: Slow 200s appear as warnings (intentional)
	switch {
	case statusCode >= 500:
		o.logger.ErrorContext(ctx, "http request", fields...)
	case statusCode >= 400:
		o.logger.WarnContext(ctx, "http request", fields...)
	case isSlow:
		o.logger.WarnContext(ctx, "http request", fields...) // Slow success still notable
	default:
		o.logger.InfoContext(ctx, "http request", fields...)
	}
}

// observabilityResponseWriter wraps [http.ResponseWriter] to capture metadata.
// It embeds [router.ResponseWriterWrapper] and adds Push, ReadFrom, and the observability marker.
type observabilityResponseWriter struct {
	*router.ResponseWriterWrapper
}

// Ensure we implement required interfaces
var (
	_ router.ResponseInfo        = (*observabilityResponseWriter)(nil)
	_ router.WrittenChecker      = (*observabilityResponseWriter)(nil)
	_ observabilityWrappedWriter = (*observabilityResponseWriter)(nil)
)

// IsObservabilityWrapped implements observabilityWrappedWriter marker interface.
// This signals that the writer has been wrapped by observability middleware,
// preventing double-wrapping when combining app observability with standalone middleware.
func (rw *observabilityResponseWriter) IsObservabilityWrapped() bool {
	return true
}

// Push preserves http.Pusher (for HTTP/2 server push).
func (rw *observabilityResponseWriter) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := rw.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}

	return stderrors.New("response writer does not support push")
}

// ReadFrom preserves io.ReaderFrom (for io.Copy).
func (rw *observabilityResponseWriter) ReadFrom(r io.Reader) (int64, error) {
	underlying := rw.ResponseWriter
	if rf, ok := underlying.(io.ReaderFrom); ok {
		n, err := rf.ReadFrom(r)
		rw.AddSize(n)
		rw.MarkWritten()
		return n, err
	}

	n, err := io.Copy(underlying, r)
	rw.AddSize(n)
	rw.MarkWritten()
	return n, err
}
