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

package tracing

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"rivaas.dev/router"
)

// MiddlewareOption configures the tracing middleware.
// These options are separate from Tracer options and only affect HTTP middleware behavior.
type MiddlewareOption func(*middlewareConfig)

// middlewareConfig holds configuration for the middleware.
type middlewareConfig struct {
	pathFilter       *pathFilter
	recordHeaders    []string
	recordHeadersLow []string        // Pre-lowercased for efficient lookup
	recordParams     bool            // Whether to record URL params
	recordParamsList []string        // Whitelist of params to record (nil = all)
	excludeParams    map[string]bool // Blacklist of params to exclude
	validationErrors []error         // Errors collected during option application
}

// newMiddlewareConfig creates a default middleware configuration.
func newMiddlewareConfig() *middlewareConfig {
	return &middlewareConfig{
		pathFilter:    newPathFilter(),
		recordParams:  true, // Default: record all params
		excludeParams: make(map[string]bool),
	}
}

// validate checks the middleware configuration and returns any collected errors.
func (c *middlewareConfig) validate() error {
	if len(c.validationErrors) == 0 {
		return nil
	}

	var errMsgs []string
	for _, err := range c.validationErrors {
		errMsgs = append(errMsgs, err.Error())
	}

	return fmt.Errorf("middleware validation errors: %s", strings.Join(errMsgs, "; "))
}

// MaxExcludedPaths is the maximum number of paths that can be excluded from tracing.
const MaxExcludedPaths = 1000

// WithExcludePaths excludes specific paths from tracing.
// Excluded paths will not create spans or record any tracing data.
// This is useful for health checks, metrics endpoints, etc.
//
// Maximum of 1000 paths can be excluded to prevent unbounded growth.
//
// Example:
//
//	handler := tracing.Middleware(tracer,
//	    tracing.WithExcludePaths("/health", "/metrics"),
//	)(mux)
func WithExcludePaths(paths ...string) MiddlewareOption {
	return func(c *middlewareConfig) {
		if c.pathFilter == nil {
			c.pathFilter = newPathFilter()
		}
		for i, path := range paths {
			if i >= MaxExcludedPaths {
				break
			}
			c.pathFilter.addPaths(path)
		}
	}
}

// WithExcludePrefixes excludes paths with the given prefixes from tracing.
// This is useful for excluding entire path hierarchies like /debug/, /internal/, etc.
//
// Example:
//
//	handler := tracing.Middleware(tracer,
//	    tracing.WithExcludePrefixes("/debug/", "/internal/"),
//	)(mux)
func WithExcludePrefixes(prefixes ...string) MiddlewareOption {
	return func(c *middlewareConfig) {
		if c.pathFilter == nil {
			c.pathFilter = newPathFilter()
		}
		c.pathFilter.addPrefixes(prefixes...)
	}
}

// WithExcludePatterns excludes paths matching the given regex patterns from tracing.
// The patterns are compiled once during configuration.
// Returns a validation error if any pattern fails to compile.
//
// Example:
//
//	handler, err := tracing.Middleware(tracer,
//	    tracing.WithExcludePatterns(`^/v[0-9]+/internal/.*`, `^/debug/.*`),
//	)(mux)
func WithExcludePatterns(patterns ...string) MiddlewareOption {
	return func(c *middlewareConfig) {
		if c.pathFilter == nil {
			c.pathFilter = newPathFilter()
		}
		for _, pattern := range patterns {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				c.validationErrors = append(c.validationErrors,
					fmt.Errorf("excludePatterns: invalid regex %q: %w", pattern, err))

				continue
			}
			c.pathFilter.addPatterns(compiled)
		}
	}
}

// sensitiveHeaders contains header names that should never be recorded in traces.
var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"cookie":              true,
	"set-cookie":          true,
	"x-api-key":           true,
	"x-auth-token":        true,
	"proxy-authorization": true,
	"www-authenticate":    true,
}

// WithHeaders records specific request headers as span attributes.
// Header names are case-insensitive. Recorded as 'http.request.header.{name}'.
//
// Security: Sensitive headers (Authorization, Cookie, etc.) are automatically
// filtered out to prevent accidental exposure of credentials in traces.
//
// Example:
//
//	handler := tracing.Middleware(tracer,
//	    tracing.WithHeaders("X-Request-ID", "X-Correlation-ID"),
//	)(mux)
func WithHeaders(headers ...string) MiddlewareOption {
	return func(c *middlewareConfig) {
		// Filter out sensitive headers
		filtered := make([]string, 0, len(headers))
		for _, h := range headers {
			if !sensitiveHeaders[strings.ToLower(h)] {
				filtered = append(filtered, h)
			}
		}
		c.recordHeaders = filtered
		// Pre-compute lowercased header names
		c.recordHeadersLow = make([]string, 0, len(filtered))
		for _, h := range filtered {
			c.recordHeadersLow = append(c.recordHeadersLow, strings.ToLower(h))
		}
	}
}

// WithRecordParams specifies which URL query parameters to record as span attributes.
// Only parameters in this list will be recorded. This provides fine-grained control
// over which parameters are traced.
//
// If this option is not used, all query parameters are recorded by default
// (unless WithoutParams is used).
//
// Example:
//
//	handler := tracing.Middleware(tracer,
//	    tracing.WithRecordParams("user_id", "request_id", "page"),
//	)(mux)
func WithRecordParams(params ...string) MiddlewareOption {
	return func(c *middlewareConfig) {
		if len(params) > 0 {
			c.recordParamsList = make([]string, 0, len(params))
			c.recordParamsList = append(c.recordParamsList, params...)
			c.recordParams = true
		}
	}
}

// WithExcludeParams specifies which URL query parameters to exclude from tracing.
// This is useful for blacklisting sensitive parameters while recording all others.
//
// Parameters in this list will never be recorded, even if WithRecordParams includes them.
//
// Example:
//
//	handler := tracing.Middleware(tracer,
//	    tracing.WithExcludeParams("password", "token", "api_key", "secret"),
//	)(mux)
func WithExcludeParams(params ...string) MiddlewareOption {
	return func(c *middlewareConfig) {
		if len(params) > 0 {
			if c.excludeParams == nil {
				c.excludeParams = make(map[string]bool, len(params))
			}
			for _, param := range params {
				c.excludeParams[param] = true
			}
		}
	}
}

// WithoutParams disables recording URL query parameters as span attributes.
// By default, all query parameters are recorded. Use this option if parameters
// may contain sensitive data.
//
// Example:
//
//	handler := tracing.Middleware(tracer,
//	    tracing.WithoutParams(),
//	)(mux)
func WithoutParams() MiddlewareOption {
	return func(c *middlewareConfig) {
		c.recordParams = false
	}
}

// Middleware creates a middleware function for standalone HTTP integration.
// This is useful when you want to add tracing to an existing router
// without using the app package.
//
// Path filtering, header recording, and param recording are configured via MiddlewareOption.
// Panics if any middleware option is invalid (e.g., invalid regex pattern).
//
// Example:
//
//	tracer := tracing.MustNew(
//	    tracing.WithOTLP("localhost:4317"),
//	    tracing.WithServiceName("my-api"),
//	)
//
//	handler := tracing.Middleware(tracer,
//	    tracing.WithExcludePaths("/health", "/metrics"),
//	    tracing.WithHeaders("X-Request-ID"),
//	)(mux)
//
//	http.ListenAndServe(":8080", handler)
func Middleware(tracer *Tracer, opts ...MiddlewareOption) func(http.Handler) http.Handler {
	cfg := newMiddlewareConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Validate configuration - panic on error for consistent API
	if err := cfg.validate(); err != nil {
		panic(fmt.Sprintf("tracing.Middleware: %v", err))
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !tracer.IsEnabled() {
				next.ServeHTTP(w, r)
				return
			}

			// Check if path should be excluded
			if cfg.pathFilter != nil && cfg.pathFilter.shouldExclude(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Start tracing with middleware-specific attribute recording
			ctx, span := startMiddlewareSpan(tracer, cfg, r)

			// Wrap response writer to capture status code
			// Check if already wrapped to prevent double-wrapping
			if _, ok := w.(router.ObservabilityWrappedWriter); ok {
				// Already wrapped, use as-is
				next.ServeHTTP(w, r.WithContext(ctx))
				// Finish with default status (can't extract from outer wrapper)
				tracer.FinishRequestSpan(span, http.StatusOK)
				return
			}

			rw := newResponseWriter(w)

			// Execute the next handler with trace context
			next.ServeHTTP(rw, r.WithContext(ctx))

			// Finish tracing
			tracer.FinishRequestSpan(span, rw.StatusCode())
		})
	}
}

// startMiddlewareSpan starts a span for HTTP request with middleware configuration.
func startMiddlewareSpan(t *Tracer, cfg *middlewareConfig, req *http.Request) (context.Context, trace.Span) {
	ctx := req.Context()

	// Extract trace context from headers
	ctx = t.ExtractTraceContext(ctx, req.Header)

	// Sampling decision
	if t.sampleRate < 1.0 {
		if t.sampleRate == 0.0 {
			return ctx, trace.SpanFromContext(ctx)
		}
		counter := t.samplingCounter.Add(1)
		hash := counter * samplingMultiplier
		if hash > t.samplingThreshold {
			return ctx, trace.SpanFromContext(ctx)
		}
	}

	// Build span name
	var spanName string
	sb := t.spanNamePool.Get().(*strings.Builder)
	sb.Reset()
	sb.WriteString(req.Method)
	sb.WriteByte(' ')
	sb.WriteString(req.URL.Path)
	spanName = sb.String()
	t.spanNamePool.Put(sb)

	// Start span
	ctx, span := t.tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindServer))

	// Prepare attributes
	attrs := make([]attribute.KeyValue, 0, 9+len(cfg.recordHeaders))

	// Set standard attributes
	attrs = append(attrs,
		attribute.String("http.method", req.Method),
		attribute.String("http.url", req.URL.String()),
		attribute.String("http.scheme", req.URL.Scheme),
		attribute.String("http.host", req.Host),
		attribute.String("http.route", req.URL.Path),
		attribute.String("http.user_agent", req.UserAgent()),
		attribute.String("service.name", t.serviceName),
		attribute.String("service.version", t.serviceVersion),
		attribute.Bool("rivaas.router.static_route", true),
	)

	// Record URL parameters if enabled
	if cfg.recordParams && req.URL.RawQuery != "" {
		queryParams := req.URL.Query()
		for key, values := range queryParams {
			if len(values) > 0 && shouldRecordParam(cfg, key) {
				attrs = append(attrs, attribute.StringSlice(attrPrefixParam+key, values))
			}
		}
	}

	// Record specific headers if configured
	for i, header := range cfg.recordHeaders {
		if value := req.Header.Get(header); value != "" {
			attrKey := attrPrefixHeader + cfg.recordHeadersLow[i]
			attrs = append(attrs, attribute.String(attrKey, value))
		}
	}

	span.SetAttributes(attrs...)

	// Invoke span start hook if configured
	if t.spanStartHook != nil {
		t.spanStartHook(ctx, span, req)
	}

	return ctx, span
}

// shouldRecordParam determines if a query parameter should be recorded.
func shouldRecordParam(cfg *middlewareConfig, param string) bool {
	// Check blacklist first
	if cfg.excludeParams[param] {
		return false
	}

	// If whitelist is configured, param must be in the list
	if cfg.recordParamsList != nil {
		return slices.Contains(cfg.recordParamsList, param)
	}

	// No whitelist - record all params
	return true
}

// responseWriter wraps http.ResponseWriter to capture status code and size.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
	written    bool
}

// newResponseWriter creates a new responseWriter.
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w}
}

// Ensure responseWriter implements required interfaces
var (
	_ router.ObservabilityWrappedWriter = (*responseWriter)(nil)
)

// WriteHeader captures the status code.
func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
		rw.written = true
	}
}

// Write captures the response size.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
	}
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n

	return n, err
}

// StatusCode returns the HTTP status code.
func (rw *responseWriter) StatusCode() int {
	if rw.statusCode == 0 {
		return http.StatusOK
	}

	return rw.statusCode
}

// Size returns the response size in bytes.
func (rw *responseWriter) Size() int {
	return rw.size
}

// Flush implements http.Flusher.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements http.Hijacker for WebSocket support.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}

	return nil, nil, fmt.Errorf("underlying ResponseWriter doesn't support Hijack")
}

// Push implements http.Pusher for HTTP/2 server push.
func (rw *responseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := rw.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}

	return http.ErrNotSupported
}

// Unwrap returns the underlying ResponseWriter for http.ResponseController support.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// IsObservabilityWrapped implements [router.ObservabilityWrappedWriter] marker interface.
// This signals that the writer has been wrapped by observability middleware,
// preventing double-wrapping when combining standalone tracing with app observability.
func (rw *responseWriter) IsObservabilityWrapped() bool {
	return true
}

// ContextTracing provides context integration helpers for router context.
type ContextTracing struct {
	tracer *Tracer
	span   trace.Span
	ctx    context.Context
}

// NewContextTracing creates a new context tracing helper.
func NewContextTracing(ctx context.Context, tracer *Tracer, span trace.Span) *ContextTracing {
	if ctx == nil {
		panic("tracing: nil context passed to NewContextTracing")
	}

	return &ContextTracing{
		tracer: tracer,
		span:   span,
		ctx:    ctx,
	}
}

// TraceID returns the current trace ID.
func (ct *ContextTracing) TraceID() string {
	if ct.span != nil && ct.span.SpanContext().IsValid() {
		return ct.span.SpanContext().TraceID().String()
	}

	return ""
}

// SpanID returns the current span ID.
func (ct *ContextTracing) SpanID() string {
	if ct.span != nil && ct.span.SpanContext().IsValid() {
		return ct.span.SpanContext().SpanID().String()
	}

	return ""
}

// SetSpanAttribute adds an attribute to the current span.
func (ct *ContextTracing) SetSpanAttribute(key string, value any) {
	if ct.span == nil || !ct.span.IsRecording() {
		return
	}
	ct.span.SetAttributes(buildAttribute(key, value))
}

// AddSpanEvent adds an event to the current span.
func (ct *ContextTracing) AddSpanEvent(name string, attrs ...attribute.KeyValue) {
	if ct.span != nil && ct.span.IsRecording() {
		ct.span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// TraceContext returns the trace context.
func (ct *ContextTracing) TraceContext() context.Context {
	return ct.ctx
}

// GetSpan returns the current span.
func (ct *ContextTracing) GetSpan() trace.Span {
	return ct.span
}

// GetTracer returns the underlying Tracer.
func (ct *ContextTracing) GetTracer() *Tracer {
	return ct.tracer
}
