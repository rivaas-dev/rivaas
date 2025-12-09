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

package metrics

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/attribute"

	"rivaas.dev/router"
)

// MiddlewareOption configures the metrics middleware.
// These options are separate from Recorder options and only affect HTTP middleware behavior.
type MiddlewareOption func(*middlewareConfig)

// middlewareConfig holds configuration for the middleware.
type middlewareConfig struct {
	pathFilter       *pathFilter
	recordHeaders    []string
	recordHeadersLow []string // Pre-lowercased for efficient lookup
}

// newMiddlewareConfig creates a default middleware configuration.
func newMiddlewareConfig() *middlewareConfig {
	return &middlewareConfig{
		pathFilter: newPathFilter(),
	}
}

// WithExcludePaths excludes specific paths from metrics collection.
// This is useful for health checks, metrics endpoints, etc.
//
// Example:
//
//	handler := metrics.Middleware(recorder,
//	    metrics.WithExcludePaths("/health", "/metrics"),
//	)(mux)
func WithExcludePaths(paths ...string) MiddlewareOption {
	return func(c *middlewareConfig) {
		if c.pathFilter == nil {
			c.pathFilter = newPathFilter()
		}
		c.pathFilter.addPaths(paths...)
	}
}

// WithExcludePrefixes excludes paths with the given prefixes from metrics collection.
// This is useful for excluding entire path hierarchies like /debug/, /internal/, etc.
//
// Example:
//
//	handler := metrics.Middleware(recorder,
//	    metrics.WithExcludePrefixes("/debug/", "/internal/"),
//	)(mux)
func WithExcludePrefixes(prefixes ...string) MiddlewareOption {
	return func(c *middlewareConfig) {
		if c.pathFilter == nil {
			c.pathFilter = newPathFilter()
		}
		c.pathFilter.addPrefixes(prefixes...)
	}
}

// WithExcludePatterns excludes paths matching the given regex patterns from metrics collection.
// The patterns are compiled once during configuration.
// Invalid regex patterns are silently ignored.
//
// Example:
//
//	handler := metrics.Middleware(recorder,
//	    metrics.WithExcludePatterns(`^/v[0-9]+/internal/.*`, `^/debug/.*`),
//	)(mux)
func WithExcludePatterns(patterns ...string) MiddlewareOption {
	return func(c *middlewareConfig) {
		if c.pathFilter == nil {
			c.pathFilter = newPathFilter()
		}
		for _, pattern := range patterns {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				continue // Skip invalid patterns silently
			}
			c.pathFilter.addPatterns(compiled)
		}
	}
}

// WithHeaders records specific headers as metric attributes.
// Headers are normalized to lowercase for consistent lookup.
//
// Security: Sensitive headers (Authorization, Cookie, X-API-Key, etc.) are
// automatically filtered out to prevent accidental exposure in metrics.
//
// Example:
//
//	handler := metrics.Middleware(recorder,
//	    metrics.WithHeaders("X-Request-ID", "X-Correlation-ID"),
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
		c.recordHeadersLow = make([]string, len(filtered))
		for i, h := range filtered {
			c.recordHeadersLow[i] = strings.ToLower(h)
		}
	}
}

// Middleware creates a middleware function for standalone HTTP integration.
// This is useful when you want to add metrics to an existing router
// without using the app package.
//
// Path filtering and header recording are configured via [MiddlewareOption].
// Use [WithExcludePaths], [WithExcludePrefixes], [WithExcludePatterns], or [WithHeaders]
// to customize behavior.
//
// Example:
//
//	recorder := metrics.MustNew(
//	    metrics.WithPrometheus(":9090", "/metrics"),
//	    metrics.WithServiceName("my-api"),
//	)
//
//	handler := metrics.Middleware(recorder,
//	    metrics.WithExcludePaths("/health", "/metrics"),
//	    metrics.WithHeaders("X-Request-ID"),
//	)(mux)
//
//	http.ListenAndServe(":8080", handler)
func Middleware(recorder *Recorder, opts ...MiddlewareOption) func(http.Handler) http.Handler {
	cfg := newMiddlewareConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !recorder.IsEnabled() {
				next.ServeHTTP(w, r)
				return
			}

			// Check if path should be excluded
			if cfg.pathFilter != nil && cfg.pathFilter.shouldExclude(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()

			// Start metrics collection
			m := recorder.BeginRequest(ctx)
			if m == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Add HTTP-specific attributes
			m.AddAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.scheme", r.URL.Scheme),
				attribute.String("http.host", r.Host),
				attribute.String("http.user_agent", r.UserAgent()),
			)

			// Record request size if available
			if r.ContentLength > 0 {
				recorder.RecordRequestSize(ctx, m, r.ContentLength)
			}

			// Record specific headers if configured
			for i, header := range cfg.recordHeaders {
				if value := r.Header.Get(header); value != "" {
					attrKey := "http.request.header." + cfg.recordHeadersLow[i]
					m.AddAttributes(attribute.String(attrKey, value))
				}
			}

			// Wrap response writer to capture status code and size
			// Check if already wrapped to prevent double-wrapping
			if _, ok := w.(router.ObservabilityWrappedWriter); ok {
				// Already wrapped, use as-is
				next.ServeHTTP(w, r)
				// Can't extract metrics reliably from outer wrapper
				return
			}

			rw := newResponseWriter(w)

			// Execute the next handler
			next.ServeHTTP(rw, r)

			// Finish metrics collection
			// Use raw path as route pattern since middleware cannot determine actual route template
			recorder.Finish(ctx, m, rw.StatusCode(), int64(rw.Size()), r.URL.Path)
		})
	}
}

// responseWriter wraps [http.ResponseWriter] to capture status code and size.
// It also implements optional interfaces ([http.Flusher], [http.Hijacker], [http.Pusher]) if the
// underlying ResponseWriter supports them.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
	written    bool
}

// newResponseWriter creates a new responseWriter wrapping the given http.ResponseWriter.
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w}
}

// Ensure responseWriter implements required interfaces
var (
	_ router.ObservabilityWrappedWriter = (*responseWriter)(nil)
)

// WriteHeader captures the status code and prevents duplicate calls.
func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
		rw.written = true
	}
}

// Write captures the response size and marks as written.
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

// Flush implements [http.Flusher] if the underlying ResponseWriter supports it.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements [http.Hijacker] for WebSocket support.
// Returns an error if the underlying ResponseWriter doesn't support hijacking.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}

	return nil, nil, fmt.Errorf("underlying ResponseWriter doesn't support Hijack")
}

// Push implements [http.Pusher] for HTTP/2 server push.
// Returns [http.ErrNotSupported] if the underlying ResponseWriter doesn't support it.
func (rw *responseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := rw.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}

	return http.ErrNotSupported
}

// Unwrap returns the underlying ResponseWriter for [http.ResponseController] support (Go 1.20+).
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// IsObservabilityWrapped implements [router.ObservabilityWrappedWriter] marker interface.
// This signals that the writer has been wrapped by observability middleware,
// preventing double-wrapping when combining standalone metrics with app observability.
func (rw *responseWriter) IsObservabilityWrapped() bool {
	return true
}
