package router

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// MetricsRecorder interface for recording metrics from external packages.
// This interface allows the router to work with any metrics implementation
// without tight coupling to specific packages.
type MetricsRecorder interface {
	// RecordMetric records a custom histogram metric with the given name and value.
	RecordMetric(ctx context.Context, name string, value float64, attributes ...attribute.KeyValue)

	// IncrementCounter increments a custom counter metric with the given name.
	IncrementCounter(ctx context.Context, name string, attributes ...attribute.KeyValue)

	// SetGauge sets a custom gauge metric with the given name and value.
	SetGauge(ctx context.Context, name string, value float64, attributes ...attribute.KeyValue)

	// RecordRouteRegistration records route registration metrics.
	RecordRouteRegistration(ctx context.Context, method, path string)

	// StartRequest initializes metrics collection for a request.
	// Returns a request metrics object that should be passed to FinishRequest.
	StartRequest(ctx context.Context, path string, isStatic bool, attributes ...attribute.KeyValue) interface{}

	// FinishRequest completes metrics collection for a request.
	// Takes the request metrics object returned by StartRequest.
	FinishRequest(ctx context.Context, metrics interface{}, statusCode int, responseSize int64)

	// IsEnabled returns true if metrics are enabled.
	IsEnabled() bool
}

// TracingRecorder interface for recording traces from external packages.
// This interface allows the router to work with any tracing implementation
// without tight coupling to specific packages.
type TracingRecorder interface {
	// StartSpan starts a new span with the given name and options.
	StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span)

	// FinishSpan completes the span with the given status code.
	FinishSpan(span trace.Span, statusCode int)

	// SetSpanAttribute adds an attribute to the span.
	SetSpanAttribute(span trace.Span, key string, value interface{})

	// AddSpanEvent adds an event to the span with optional attributes.
	AddSpanEvent(span trace.Span, name string, attrs ...attribute.KeyValue)

	// ExtractTraceContext extracts trace context from HTTP headers.
	ExtractTraceContext(ctx context.Context, headers http.Header) context.Context

	// InjectTraceContext injects trace context into HTTP headers.
	InjectTraceContext(ctx context.Context, headers http.Header)

	// IsEnabled returns true if tracing is enabled.
	IsEnabled() bool

	// ShouldExcludePath returns true if the given path should be excluded from tracing.
	ShouldExcludePath(path string) bool
}

// ContextMetricsRecorder interface for context-level metrics recording.
// This interface provides methods that can be called from router.Context
// to record custom metrics.
type ContextMetricsRecorder interface {
	// RecordMetric records a custom histogram metric with the given name and value.
	RecordMetric(ctx context.Context, name string, value float64, attributes ...attribute.KeyValue)

	// IncrementCounter increments a custom counter metric with the given name.
	IncrementCounter(ctx context.Context, name string, attributes ...attribute.KeyValue)

	// SetGauge sets a custom gauge metric with the given name and value.
	SetGauge(ctx context.Context, name string, value float64, attributes ...attribute.KeyValue)
}

// ContextTracingRecorder interface for context-level tracing recording.
// This interface provides methods that can be called from router.Context
// to interact with tracing.
type ContextTracingRecorder interface {
	// TraceID returns the current trace ID from the active span.
	// Returns an empty string if tracing is not active.
	TraceID() string

	// SpanID returns the current span ID from the active span.
	// Returns an empty string if tracing is not active.
	SpanID() string

	// SetSpanAttribute adds an attribute to the current span.
	// This is a no-op if tracing is not active.
	SetSpanAttribute(key string, value interface{})

	// AddSpanEvent adds an event to the current span with optional attributes.
	// This is a no-op if tracing is not active.
	AddSpanEvent(name string, attrs ...attribute.KeyValue)

	// TraceContext returns the OpenTelemetry trace context.
	// This can be used for manual span creation or context propagation.
	// If tracing is not enabled, it returns the request context for proper cancellation support.
	TraceContext() context.Context
}

// RequestMetrics interface for request-level metrics tracking.
// This interface abstracts the metrics data structure used during request processing.
// It is implemented by the metrics package and returned by StartRequest().
// The interface is intentionally minimal to allow for flexible implementations.
type RequestMetrics interface {
	// GetStartTime returns the request start time.
	GetStartTime() interface{}

	// GetRequestSize returns the request size in bytes.
	GetRequestSize() int64

	// GetAttributes returns the request attributes.
	GetAttributes() []attribute.KeyValue
}
