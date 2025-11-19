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

package logging

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// ContextLogger provides context-aware logging with automatic trace correlation.
//
// Why this exists:
//   - Distributed tracing requires trace/span IDs in logs to correlate requests
//   - Manually passing trace IDs to every log call is error-prone and verbose
//   - This extracts them automatically from OpenTelemetry context
//
// When to use:
//
//	✓ Request handlers with OpenTelemetry tracing enabled
//	✓ Background jobs that propagate context
//	✗ Package-level loggers (no request context available)
//	✗ High-frequency logging (>1000/sec) where trace extraction overhead matters
//
// Performance: Trace extraction adds minimal overhead per log call (sub-microsecond).
// For most applications this is negligible compared to I/O cost of writing logs.
//
// Thread-safe: Safe to use concurrently. Each instance is typically
// created per-request and used by a single goroutine.
type ContextLogger struct {
	logger  *slog.Logger
	ctx     context.Context
	traceID string
	spanID  string
}

// NewContextLogger creates a context-aware logger.
// If the context contains an active OpenTelemetry span, trace and span IDs
// will be automatically added to all log entries.
func NewContextLogger(ctx context.Context, cfg *Config) *ContextLogger {
	l := cfg.Logger()

	// Extract trace information from context
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		sc := span.SpanContext()
		traceID := sc.TraceID().String()
		spanID := sc.SpanID().String()

		// Add trace IDs to logger
		l = l.With(
			slog.String("trace_id", traceID),
			slog.String("span_id", spanID),
		)

		return &ContextLogger{
			logger:  l,
			ctx:     ctx,
			traceID: traceID,
			spanID:  spanID,
		}
	}

	return &ContextLogger{
		logger: l,
		ctx:    ctx,
	}
}

// Logger returns the underlying slog.Logger.
func (cl *ContextLogger) Logger() *slog.Logger {
	return cl.logger
}

// TraceID returns the trace ID if available.
func (cl *ContextLogger) TraceID() string {
	return cl.traceID
}

// SpanID returns the span ID if available.
func (cl *ContextLogger) SpanID() string {
	return cl.spanID
}

// reset resets the ContextLogger for reuse from a pool.
//
// Why pooling: In high-throughput HTTP servers (>1000 req/sec), creating
// a new ContextLogger for every request causes GC pressure. Pooling amortizes
// the allocation cost.
//
// Performance impact: Reduces allocations from 1 per request to ~0.
// For 10,000 req/sec, this saves ~1-2MB/sec of garbage.
//
// Usage: Typically used by router middleware, not directly by application code.
//
// Thread-safety: NOT safe to call concurrently on the same instance.
// reset() is only called when acquiring from pool (single-threaded).
func (cl *ContextLogger) reset(ctx context.Context, cfg *Config) {
	l := cfg.Logger()
	cl.ctx = ctx
	cl.traceID = ""
	cl.spanID = ""

	// Extract trace information from context
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		sc := span.SpanContext()
		cl.traceID = sc.TraceID().String()
		cl.spanID = sc.SpanID().String()

		// Add trace IDs to logger
		l = l.With(
			slog.String("trace_id", cl.traceID),
			slog.String("span_id", cl.spanID),
		)
	}

	cl.logger = l
}

// With returns a logger with additional attributes.
func (cl *ContextLogger) With(args ...any) *slog.Logger {
	return cl.logger.With(args...)
}

// Debug logs a debug message with context.
func (cl *ContextLogger) Debug(msg string, args ...any) {
	cl.logger.DebugContext(cl.ctx, msg, args...)
}

// Info logs an info message with context.
func (cl *ContextLogger) Info(msg string, args ...any) {
	cl.logger.InfoContext(cl.ctx, msg, args...)
}

// Warn logs a warning message with context.
func (cl *ContextLogger) Warn(msg string, args ...any) {
	cl.logger.WarnContext(cl.ctx, msg, args...)
}

// Error logs an error message with context.
func (cl *ContextLogger) Error(msg string, args ...any) {
	cl.logger.ErrorContext(cl.ctx, msg, args...)
}
