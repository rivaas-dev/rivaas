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
	"log/slog"
	"sync"

	"go.opentelemetry.io/otel/trace"
	"rivaas.dev/router"
)

// contextPool provides pooling for app.Context instances.
// contextPool reuses Context instances across requests.
type contextPool struct {
	pool sync.Pool
}

// newContextPool creates a new context pool.
// newContextPool is a private helper for creating context pools.
func newContextPool() *contextPool {
	return &contextPool{
		pool: sync.Pool{
			New: func() any {
				return &Context{}
			},
		},
	}
}

// Get retrieves a Context from the pool.
// Get returns a Context instance, creating a new one if the pool is empty.
func (cp *contextPool) Get() *Context {
	return cp.pool.Get().(*Context)
}

// Put returns a Context to the pool after resetting it.
//
// Put cleanup is also performed in wrapHandler's defer to ensure
// contexts are reset even if handlers panic. This method's cleanup
// is idempotent and provides an additional safety layer.
func (cp *contextPool) Put(c *Context) {
	// Reset context state
	c.Context = nil
	c.app = nil
	c.bindingMeta = nil
	c.logger = nil
	cp.pool.Put(c)
}

// buildRequestLogger creates a request-scoped logger with automatic context.
// buildRequestLogger adds HTTP metadata and OpenTelemetry trace correlation using semantic conventions.
//
// Security considerations:
//   - Uses router's ClientIP() which respects trusted proxy configuration
//   - Does NOT log query strings by default (may contain PII)
//   - Only logs network.client.ip (proxy-aware), not network.peer.ip
func buildRequestLogger(base *slog.Logger, rc *router.Context) *slog.Logger {
	// Use no-op logger if none configured (singleton)
	if base == nil {
		base = noopLogger
	}

	req := rc.Request

	// Add HTTP metadata (OpenTelemetry semantic conventions)
	attrs := []slog.Attr{
		slog.String(fieldHTTPMethod, req.Method),
		slog.String(fieldHTTPTarget, req.URL.Path), // Actual path: /orders/42
	}

	// Add route template if available (after routing completes)
	if route := rc.RouteTemplate(); route != "" {
		attrs = append(attrs, slog.String(fieldHTTPRoute, route)) // Template: /orders/:id
	}

	// Add client IP (proxy-aware if configured, otherwise socket peer)
	// This respects router's trusted proxy configuration
	if clientIP := rc.ClientIP(); clientIP != "" {
		attrs = append(attrs, slog.String(fieldNetworkClientIP, clientIP))
	}

	// Add request ID if present
	if reqID := req.Header.Get("X-Request-ID"); reqID != "" {
		attrs = append(attrs, slog.String(fieldRequestID, reqID))
	}

	// Create logger with HTTP context
	// Convert []slog.Attr to []any for With()
	anyAttrs := make([]any, 0, len(attrs)*2)
	for _, attr := range attrs {
		anyAttrs = append(anyAttrs, attr.Key, attr.Value.Any())
	}
	logger := base.With(anyAttrs...)

	// Add OpenTelemetry trace correlation (if span is active)
	ctx := req.Context()
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		sc := span.SpanContext()
		logger = logger.With(
			slog.String(fieldTraceID, sc.TraceID().String()),
			slog.String(fieldSpanID, sc.SpanID().String()),
		)
	}

	return logger
}
