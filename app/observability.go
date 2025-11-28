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
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/trace"
	"rivaas.dev/metrics"
	"rivaas.dev/router"
	"rivaas.dev/telemetry/semconv"
	"rivaas.dev/tracing"
)

// observabilityRecorder implements router.ObservabilityRecorder by unifying
// metrics, tracing, and logging into a single lifecycle.
// observabilityRecorder coordinates observability data collection across all three pillars.
type observabilityRecorder struct {
	metrics    *metrics.Config
	tracing    *tracing.Config
	logger     *slog.Logger
	pathFilter *pathFilter

	// Access log configuration
	logAccessRequests bool
	logErrorsOnly     bool
	slowThreshold     time.Duration
}

// observabilityConfig configures the unified observability recorder.
type observabilityConfig struct {
	Metrics           *metrics.Config
	Tracing           *tracing.Config
	Logger            *slog.Logger
	PathFilter        *pathFilter
	LogAccessRequests bool
	LogErrorsOnly     bool
	SlowThreshold     time.Duration
}

// newObservabilityRecorder creates an observabilityRecorder from configuration.
// newObservabilityRecorder initializes path filtering and all observability components.
func newObservabilityRecorder(cfg *observabilityConfig) router.ObservabilityRecorder {
	pf := cfg.PathFilter
	if pf == nil {
		pf = newPathFilterWithDefaults()
	}

	return &observabilityRecorder{
		metrics:           cfg.Metrics,
		tracing:           cfg.Tracing,
		logger:            cfg.Logger,
		pathFilter:        pf,
		logAccessRequests: cfg.LogAccessRequests,
		logErrorsOnly:     cfg.LogErrorsOnly,
		slowThreshold:     cfg.SlowThreshold,
	}
}

// observabilityState holds per-request observability state.
// observabilityState is the opaque token passed between lifecycle methods.
type observabilityState struct {
	metricsData any           // Opaque metrics state from metrics.StartRequest
	span        trace.Span    // Active span from tracing
	startTime   time.Time     // Request start time for duration calculation
	req         *http.Request // Original request for access logging
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
	// Note: We start with raw path; will rename span to route pattern in OnRequestEnd
	if o.tracing != nil && o.tracing.IsEnabled() {
		spanName := req.Method + " " + req.URL.Path
		ctx, state.span = o.tracing.StartSpan(ctx, spanName)
	}

	// Start metrics (if enabled)
	// Note: We'll update with route pattern in OnRequestEnd for cardinality control
	if o.metrics != nil && o.metrics.IsEnabled() {
		// Pass empty string for now; will use routePattern in FinishRequest
		state.metricsData = o.metrics.StartRequest(ctx, "", false)
	}

	return ctx, state
}

func (o *observabilityRecorder) WrapResponseWriter(w http.ResponseWriter, state any) http.ResponseWriter {
	if state == nil {
		return w // Excluded: don't wrap
	}
	return &observabilityResponseWriter{ResponseWriter: w}
}

func (o *observabilityRecorder) OnRequestEnd(ctx context.Context, state any, writer http.ResponseWriter, routePattern string) {
	s, ok := state.(*observabilityState)
	if !ok || s == nil {
		return // Excluded or invalid state
	}

	duration := time.Since(s.startTime)

	// Extract response metadata from wrapped writer
	var statusCode = http.StatusOK
	var responseSize int64 = 0
	if ri, ok := writer.(router.ResponseInfo); ok {
		statusCode = ri.StatusCode()
		responseSize = ri.Size()
	}

	// Update span name to use route pattern (better cardinality)
	if s.span != nil && s.span.IsRecording() && routePattern != "" {
		spanName := s.req.Method + " " + routePattern
		s.span.SetName(spanName)
	}

	// Finish tracing
	if s.span != nil {
		o.tracing.FinishSpan(s.span, statusCode)
	}

	// Finish metrics with route pattern (prevents cardinality explosion)
	if s.metricsData != nil {
		// Use routePattern for metrics to avoid high cardinality
		// If no route matched, use sentinel value
		metricsPath := routePattern
		if metricsPath == "" {
			metricsPath = "_unmatched"
		}
		o.metrics.FinishRequest(ctx, s.metricsData, statusCode, responseSize, metricsPath)
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

	// Add trace ID (for correlation with traces)
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		fields = append(fields, "trace_id", span.SpanContext().TraceID().String())
	}

	// Mark slow requests explicitly
	if isSlow {
		fields = append(fields, "slow", true)
	}

	// Log at appropriate level
	// Note: Slow 200s appear as warnings (intentional)
	switch {
	case statusCode >= 500:
		o.logger.ErrorContext(ctx, "access", fields...)
	case statusCode >= 400:
		o.logger.WarnContext(ctx, "access", fields...)
	case isSlow:
		o.logger.WarnContext(ctx, "access", fields...) // Slow success still notable
	default:
		o.logger.InfoContext(ctx, "access", fields...)
	}
}

func (o *observabilityRecorder) BuildRequestLogger(ctx context.Context, req *http.Request, routePattern string) *slog.Logger {
	// Always return a non-nil logger
	// If logging disabled, return no-op logger
	if o.logger == nil {
		return router.NoopLogger()
	}

	// Build request-scoped logger with HTTP metadata (semantic conventions)
	attrs := []any{
		semconv.HTTPMethod, req.Method,
		semconv.HTTPTarget, req.URL.Path,
	}

	// Add route template (available after routing)
	if routePattern != "" {
		attrs = append(attrs, semconv.HTTPRoute, routePattern)
	}

	// Add request ID (for correlation)
	if reqID := req.Header.Get("X-Request-ID"); reqID != "" {
		attrs = append(attrs, semconv.RequestID, reqID)
	}

	// Add client IP (proxy-aware if configured)
	// Note: This should use router's ClientIP() helper if available
	// For now, using raw RemoteAddr
	attrs = append(attrs, semconv.NetworkClientIP, req.RemoteAddr)

	logger := o.logger.With(attrs...)

	// Add trace correlation (if span is active)
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		sc := span.SpanContext()
		logger = logger.With(
			semconv.TraceID, sc.TraceID().String(),
			semconv.SpanID, sc.SpanID().String(),
		)
	}

	return logger
}

// observabilityResponseWriter wraps http.ResponseWriter to capture metadata.
// observabilityResponseWriter implements router.ResponseInfo plus common optional interfaces.
type observabilityResponseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int64
	written    bool
}

// Ensure we implement required interface
var _ router.ResponseInfo = (*observabilityResponseWriter)(nil)

func (rw *observabilityResponseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
		rw.written = true
	}
}

func (rw *observabilityResponseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
		rw.statusCode = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += int64(n)
	return n, err
}

func (rw *observabilityResponseWriter) StatusCode() int {
	if rw.statusCode == 0 {
		return http.StatusOK
	}
	return rw.statusCode
}

func (rw *observabilityResponseWriter) Size() int64 {
	return rw.size
}

// Preserve http.Hijacker (for WebSockets, HTTP/2, etc.)
func (rw *observabilityResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("response writer does not support hijacking")
}

// Preserve http.Flusher (for streaming responses)
func (rw *observabilityResponseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Preserve http.Pusher (for HTTP/2 server push)
func (rw *observabilityResponseWriter) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := rw.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return fmt.Errorf("response writer does not support push")
}

// Preserve io.ReaderFrom (for io.Copy)
func (rw *observabilityResponseWriter) ReadFrom(r io.Reader) (int64, error) {
	// Try to use underlying ReaderFrom implementation if available
	if rf, ok := rw.ResponseWriter.(io.ReaderFrom); ok {
		n, err := rf.ReadFrom(r)
		rw.size += n
		if !rw.written {
			rw.written = true
			if rw.statusCode == 0 {
				rw.statusCode = http.StatusOK
			}
		}
		return n, err
	}

	// Fallback: use io.Copy but still track size & status
	// This ensures StatusCode() and Size() remain accurate
	n, err := io.Copy(rw.ResponseWriter, r)
	rw.size += n
	if !rw.written {
		rw.written = true
		if rw.statusCode == 0 {
			rw.statusCode = http.StatusOK
		}
	}
	return n, err
}
