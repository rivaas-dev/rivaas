package logging

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// ContextLogger provides context-aware logging with trace correlation.
// It automatically extracts trace and span IDs from the OpenTelemetry context
// and adds them to all log entries.
type ContextLogger struct {
	logger  *slog.Logger
	ctx     context.Context
	traceID string
	spanID  string
}

// NewContextLogger creates a context-aware logger.
// If the context contains an active OpenTelemetry span, trace and span IDs
// will be automatically added to all log entries.
func NewContextLogger(cfg *Config, ctx context.Context) *ContextLogger {
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

// reset resets the ContextLogger for reuse from the pool.
// This is more efficient than creating a new ContextLogger for each request.
func (cl *ContextLogger) reset(cfg *Config, ctx context.Context) {
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
