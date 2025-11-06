package tracing

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Recorder interface for integration with router.
// This is defined locally to avoid circular dependencies.
type Recorder interface {
	StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span)
	FinishSpan(span trace.Span, statusCode int)
	SetSpanAttribute(span trace.Span, key string, value interface{})
	AddSpanEvent(span trace.Span, name string, attrs ...attribute.KeyValue)
	ExtractTraceContext(ctx context.Context, headers http.Header) context.Context
	InjectTraceContext(ctx context.Context, headers http.Header)
	IsEnabled() bool
	ShouldExcludePath(path string) bool
}

// RouterOption defines functional options for router configuration.
type RouterOption func(interface{})

// WithTracing creates a router option that enables tracing.
// This is the main entry point for integrating tracing with the router.
// Panics if tracing initialization fails. Use WithTracingOrError for error handling.
func WithTracing(opts ...Option) RouterOption {
	return func(router interface{}) {
		// Create tracing configuration
		config := MustNew(opts...)

		// Try to set the tracing configuration on the router
		// This uses interface{} to avoid circular dependencies
		if setter, ok := router.(interface{ SetTracingRecorder(Recorder) }); ok {
			setter.SetTracingRecorder(config)
		}
	}
}

// WithTracingFromConfig creates a router option from an existing tracing config.
func WithTracingFromConfig(config *Config) RouterOption {
	return func(router interface{}) {
		if setter, ok := router.(interface{ SetTracingRecorder(Recorder) }); ok {
			setter.SetTracingRecorder(config)
		}
	}
}

// Middleware creates a middleware function for manual integration.
// This is useful when you want to add tracing to an existing router
// without using the options pattern.
func Middleware(config *Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !config.IsEnabled() {
				next.ServeHTTP(w, r)
				return
			}

			// Check if path should be excluded
			if config.ShouldExcludePath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Start tracing
			ctx, span := config.StartRequestSpan(r.Context(), r, r.URL.Path, true)

			// Wrap response writer to capture status code
			rw := &responseWriter{ResponseWriter: w}

			// Execute the next handler with trace context
			next.ServeHTTP(rw, r.WithContext(ctx))

			// Finish tracing
			config.FinishRequestSpan(span, rw.StatusCode())
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code and size.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
	written    bool
}

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

// ContextTracing provides context integration helpers for router context.
type ContextTracing struct {
	config *Config
	span   trace.Span
	ctx    context.Context
}

// NewContextTracing creates a new context tracing helper.
// The context parameter must not be nil; if nil, context.Background() will be used.
func NewContextTracing(ctx context.Context, config *Config, span trace.Span) *ContextTracing {
	if ctx == nil {
		ctx = context.Background()
	}
	return &ContextTracing{
		config: config,
		span:   span,
		ctx:    ctx,
	}
}

// TraceID returns the current trace ID from the active span.
// Returns an empty string if tracing is not active.
func (ct *ContextTracing) TraceID() string {
	if ct.span != nil && ct.span.SpanContext().IsValid() {
		return ct.span.SpanContext().TraceID().String()
	}
	return ""
}

// SpanID returns the current span ID from the active span.
// Returns an empty string if tracing is not active.
func (ct *ContextTracing) SpanID() string {
	if ct.span != nil && ct.span.SpanContext().IsValid() {
		return ct.span.SpanContext().SpanID().String()
	}
	return ""
}

// SetSpanAttribute adds an attribute to the current span.
// This is a no-op if tracing is not active.
// Supports string, int, int64, float64, and bool types natively.
func (ct *ContextTracing) SetSpanAttribute(key string, value interface{}) {
	if ct.span == nil || !ct.span.IsRecording() {
		return
	}
	ct.span.SetAttributes(buildAttribute(key, value))
}

// AddSpanEvent adds an event to the current span with optional attributes.
// This is a no-op if tracing is not active.
func (ct *ContextTracing) AddSpanEvent(name string, attrs ...attribute.KeyValue) {
	if ct.span != nil && ct.span.IsRecording() {
		ct.span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// TraceContext returns the OpenTelemetry trace context.
// This can be used for manual span creation or context propagation.
//
// The returned context preserves the request context's cancellation signal and
// includes trace propagation information. This is safe to use for downstream
// operations that need both tracing and request lifecycle management.
func (ct *ContextTracing) TraceContext() context.Context {
	return ct.ctx
}

// GetSpan returns the current span.
func (ct *ContextTracing) GetSpan() trace.Span {
	return ct.span
}

// GetConfig returns the underlying tracing configuration.
func (ct *ContextTracing) GetConfig() *Config {
	return ct.config
}
