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

import "time"

// WithDiagnostics sets a diagnostic handler for the router.
//
// Diagnostic events are optional informational events that may indicate
// configuration issues or security concerns.
// The router functions correctly whether diagnostics are collected or not.
//
// Example with logging:
//
//	import "log/slog"
//
//	handler := router.DiagnosticHandlerFunc(func(e router.DiagnosticEvent) {
//	    slog.Warn(e.Message, "kind", e.Kind, "fields", e.Fields)
//	})
//	r := router.MustNew(router.WithDiagnostics(handler))
//
// Example with metrics:
//
//	handler := router.DiagnosticHandlerFunc(func(e router.DiagnosticEvent) {
//	    metrics.Increment("router.diagnostics", "kind", string(e.Kind))
//	})
//
// Example with OpenTelemetry:
//
//	import "go.opentelemetry.io/otel/attribute"
//	import "go.opentelemetry.io/otel/trace"
//
//	handler := router.DiagnosticHandlerFunc(func(e router.DiagnosticEvent) {
//	    span := trace.SpanFromContext(ctx)
//	    if span.IsRecording() {
//	        attrs := []attribute.KeyValue{
//	            attribute.String("diagnostic.kind", string(e.Kind)),
//	        }
//	        for k, v := range e.Fields {
//	            attrs = append(attrs, attribute.String(k, fmt.Sprint(v)))
//	        }
//	        span.AddEvent(e.Message, trace.WithAttributes(attrs...))
//	    }
//	})
func WithDiagnostics(handler DiagnosticHandler) Option {
	return func(c *config) {
		c.diagnostics = handler
	}
}

// WithH2C enables HTTP/2 Cleartext support.
//
// ⚠️ SECURITY WARNING: Only use in development or behind a trusted load balancer.
// DO NOT enable on public-facing servers without TLS.
//
// Common deployment patterns:
//   - Dev/local testing: Enable h2c for direct HTTP/2 testing
//   - Behind Envoy/Caddy: LB speaks h2c to app (configure LB accordingly)
//   - Behind Nginx: Typically uses HTTP/1.1 upstream (h2c not needed)
//
// Requires: golang.org/x/net/http2/h2c
//
// Example:
//
//	r := router.MustNew(router.WithH2C(true))
//	r.Serve(":8080")
func WithH2C(enable bool) Option {
	return func(c *config) {
		c.enableH2C = enable
	}
}

// ServerTimeoutOption configures HTTP server timeouts when passed to [WithServerTimeouts].
type ServerTimeoutOption func(*serverTimeouts)

// WithReadHeaderTimeout sets the server read header timeout.
// It configures how long the server waits to read request headers.
//
// Example:
//
//	r := router.MustNew(router.WithServerTimeouts(
//	    router.WithReadHeaderTimeout(5 * time.Second),
//	))
func WithReadHeaderTimeout(d time.Duration) ServerTimeoutOption {
	return func(s *serverTimeouts) {
		s.readHeader = d
	}
}

// WithReadTimeout sets the server read timeout.
// It configures how long the server waits to read the entire request.
//
// Example:
//
//	r := router.MustNew(router.WithServerTimeouts(
//	    router.WithReadTimeout(30 * time.Second),
//	))
func WithReadTimeout(d time.Duration) ServerTimeoutOption {
	return func(s *serverTimeouts) {
		s.read = d
	}
}

// WithWriteTimeout sets the server write timeout.
// It configures how long the server waits to write the response.
//
// Example:
//
//	r := router.MustNew(router.WithServerTimeouts(
//	    router.WithWriteTimeout(60 * time.Second),
//	))
func WithWriteTimeout(d time.Duration) ServerTimeoutOption {
	return func(s *serverTimeouts) {
		s.write = d
	}
}

// WithIdleTimeout sets the server idle timeout.
// It configures how long the server waits for the next request on a keep-alive connection.
//
// Example:
//
//	r := router.MustNew(router.WithServerTimeouts(
//	    router.WithIdleTimeout(120 * time.Second),
//	))
func WithIdleTimeout(d time.Duration) ServerTimeoutOption {
	return func(s *serverTimeouts) {
		s.idle = d
	}
}

// WithServerTimeouts configures HTTP server timeouts using functional options.
// These are critical for preventing slowloris attacks and resource exhaustion.
// When no options are passed, server timeouts remain at their defaults (applied at serve time).
//
// Defaults (if not set):
//
//	ReadHeaderTimeout: 5s  - Time to read request headers
//	ReadTimeout:       15s - Time to read entire request
//	WriteTimeout:      30s - Time to write response
//	IdleTimeout:       60s - Keep-alive idle time
//
// Example:
//
//	r := router.MustNew(router.WithServerTimeouts(
//	    router.WithReadHeaderTimeout(5 * time.Second),
//	    router.WithReadTimeout(20 * time.Second),
//	    router.WithWriteTimeout(60 * time.Second),
//	    router.WithIdleTimeout(120 * time.Second),
//	))
func WithServerTimeouts(opts ...ServerTimeoutOption) Option {
	return func(c *config) {
		if len(opts) == 0 {
			return
		}
		s := defaultServerTimeouts()
		for _, opt := range opts {
			opt(s)
		}
		c.serverTimeouts = s
	}
}

// defaultServerTimeouts returns default timeout configuration.
func defaultServerTimeouts() *serverTimeouts {
	return &serverTimeouts{
		readHeader: 5 * time.Second,
		read:       15 * time.Second,
		write:      30 * time.Second,
		idle:       60 * time.Second,
	}
}

// WithBloomFilterSize returns a RouterOption that sets the bloom filter size for compiled routes.
// The bloom filter is used for negative lookups in static route matching.
// Larger sizes reduce false positives.
//
// Default: 1000
// Recommended: Set to 2-3x the number of static routes
// Must be > 0 or validation will fail.
//
// Example:
//
//	r := router.MustNew(router.WithBloomFilterSize(2000)) // For ~1000 routes
func WithBloomFilterSize(size uint64) Option {
	return func(c *config) {
		c.bloomFilterSize = size
	}
}

// WithBloomFilterHashFunctions returns a RouterOption that sets the number of hash functions
// used in bloom filters for compiled routes. More hash functions reduce false positives.
//
// Default: 3
// Range: 1-10 (values outside this range are clamped)
// Recommended: 3-5 for most use cases
//
// False positive rate formula: (1 - e^(-kn/m))^k
// where k = hash functions, n = items, m = bits
//
// Example:
//
//	r := router.MustNew(router.WithBloomFilterHashFunctions(4))
func WithBloomFilterHashFunctions(numFuncs int) Option {
	return func(c *config) {
		// Clamp to reasonable range [1, 10]
		c.bloomHashFunctions = max(1, min(numFuncs, 10))
	}
}

// WithoutCancellationCheck disables context cancellation checking in the middleware chain.
// Cancellation checking is enabled by default: the router checks for canceled contexts
// between each handler, preventing wasted work on timed-out requests.
//
// Use when:
//   - You don't use request timeouts
//   - You handle cancellation manually in handlers
//   - You want to avoid the small overhead of cancellation checks
//
// Example:
//
//	r := router.MustNew(router.WithoutCancellationCheck())
func WithoutCancellationCheck() Option {
	return func(c *config) {
		c.checkCancellation = false
	}
}

// WithRouteCompilation enables or disables compiled route matching.
// When enabled, routes are pre-compiled into data structures for lookup:
//   - Static routes use hash table lookup
//   - Dynamic routes use first-segment indexing and bloom filters
//
// Default: false (tree traversal). Enable for large route sets (hundreds of
// routes) where bloom filter and hash lookups reduce lookup cost.
//
// Example:
//
//	r := router.MustNew(router.WithRouteCompilation(true))  // Opt-in for large APIs
func WithRouteCompilation(enabled bool) Option {
	return func(c *config) {
		c.useCompiledRoutes = enabled
	}
}
