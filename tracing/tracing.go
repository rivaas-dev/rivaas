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
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Note: Option type and functional options are defined in options.go

// EventType represents the severity of an internal operational event.
type EventType int

const (
	// EventError indicates an error event (e.g., failed to export spans).
	EventError EventType = iota
	// EventWarning indicates a warning event (e.g., deprecated configuration).
	EventWarning
	// EventInfo indicates an informational event (e.g., tracing initialized).
	EventInfo
	// EventDebug indicates a debug event (e.g., detailed operation logs).
	EventDebug
)

// Event represents an internal operational event from the tracing package.
// Events are used to report errors, warnings, and informational messages
// about the tracing system's operation.
type Event struct {
	Type    EventType
	Message string
	Args    []any // slog-style key-value pairs
}

// EventHandler processes internal operational events from the tracing package.
// Implementations can log events, send them to monitoring systems, or take
// custom actions based on event type.
//
// Example custom handler:
//
//	tracing.WithEventHandler(func(e tracing.Event) {
//	    if e.Type == tracing.EventError {
//	        sentry.CaptureMessage(e.Message)
//	    }
//	    slog.Default().Info(e.Message, e.Args...)
//	})
type EventHandler func(Event)

// DefaultEventHandler returns an EventHandler that logs events to the provided slog.Logger.
// This is the default implementation used by WithLogger.
//
// If logger is nil, returns a no-op handler that discards all events.
func DefaultEventHandler(logger *slog.Logger) EventHandler {
	if logger == nil {
		return func(Event) {} // no-op
	}
	return func(e Event) {
		switch e.Type {
		case EventError:
			logger.Error(e.Message, e.Args...)
		case EventWarning:
			logger.Warn(e.Message, e.Args...)
		case EventInfo:
			logger.Info(e.Message, e.Args...)
		case EventDebug:
			logger.Debug(e.Message, e.Args...)
		}
	}
}

const (
	// DefaultServiceName is the default service name used for tracing when none is provided.
	DefaultServiceName = "rivaas-service"

	// DefaultServiceVersion is the default service version when none is provided.
	DefaultServiceVersion = "1.0.0"

	// DefaultSampleRate is the default sampling rate (100% of requests).
	DefaultSampleRate = 1.0
)

// Attribute key prefixes for string building
const (
	attrPrefixParam  = "http.request.param."
	attrPrefixHeader = "http.request.header."
)

// samplingMultiplier is used for sampling decisions.
//
// The value 2654435761 is 2^32/φ (where φ is the golden ratio ≈ 1.618),
// rounded to the nearest odd number. This constant is from Knuth's
// "The Art of Computer Programming, Vol. 3, Section 6.4" on multiplicative
// hashing. Being coprime to 2^64, it ensures the sequence (counter * multiplier)
// cycles through all values before repeating.
const samplingMultiplier = 2654435761

// Provider represents the available tracing providers.
type Provider string

const (
	// NoopProvider is a no-op provider that doesn't export anything (default).
	NoopProvider Provider = "noop"

	// StdoutProvider exports traces to stdout (development/testing).
	StdoutProvider Provider = "stdout"

	// OTLPProvider exports traces via OTLP gRPC protocol.
	OTLPProvider Provider = "otlp"

	// OTLPHTTPProvider exports traces via OTLP HTTP protocol.
	OTLPHTTPProvider Provider = "otlp-http"
)

// Tracer holds OpenTelemetry tracing configuration and runtime state.
// All operations on Tracer are thread-safe.
//
// It implements request tracing for integration with HTTP frameworks.
//
// Important: Tracer is immutable after creation via New(). All configuration
// must be done through functional options passed to New().
//
// Global State:
// By default, this package does NOT set the global OpenTelemetry tracer provider.
// Use WithGlobalTracerProvider() option if you want global registration.
// This allows multiple tracing configurations to coexist in the same process.
type Tracer struct {
	// Core tracing components
	tracer         trace.Tracer
	propagator     propagation.TextMapPropagator
	tracerProvider trace.TracerProvider
	sdkProvider    *sdktrace.TracerProvider // SDK provider for shutdown (nil if custom)
	eventHandler   EventHandler             // Handler for internal operational events
	serviceName    string
	serviceVersion string
	provider       Provider
	otlpEndpoint   string

	// Lifecycle hooks
	spanStartHook  SpanStartHook
	spanFinishHook SpanFinishHook

	// Tracing behavior settings
	sampleRate float64

	// Atomic types (must be 8-byte aligned)
	samplingCounter   atomic.Uint64 // Sampling counter
	samplingThreshold uint64        // Precomputed sampling threshold

	// Shutdown synchronization
	shutdownOnce sync.Once
	shutdownErr  error

	// Small types and booleans at end
	otlpInsecure         bool
	enabled              bool
	customTracerProvider bool
	registerGlobal       bool // If true, sets otel.SetTracerProvider()
	providerSet          bool // Tracks if a provider option was explicitly configured

	// Validation errors (collected during option application)
	validationErrors []error

	// String pool for reusable string builders
	spanNamePool sync.Pool
}

// New creates a new Tracer with the given options.
// Returns an error if the tracing provider fails to initialize.
// For a version that panics on error, use MustNew.
//
// By default, this function does NOT set the global OpenTelemetry tracer provider.
// Use WithGlobalTracerProvider() if you want to register the tracer provider as the global default.
//
// Default configuration:
//   - Service name: DefaultServiceName ("rivaas-service")
//   - Service version: DefaultServiceVersion ("1.0.0")
//   - Sample rate: DefaultSampleRate (1.0 = 100%)
//   - Provider: NoopProvider (no traces exported)
//
// Example:
//
//	tracer, err := tracing.New(
//	    tracing.WithServiceName("my-api"),
//	    tracing.WithOTLP("localhost:4317"),
//	    tracing.WithSampleRate(0.1),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer tracer.Shutdown(context.Background())
func New(opts ...Option) (*Tracer, error) {
	t := newDefaultTracer()

	// Apply options
	for _, opt := range opts {
		opt(t)
	}

	// Validate configuration
	if err := t.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Initialize the provider
	if err := t.initializeProvider(); err != nil {
		return nil, fmt.Errorf("failed to initialize tracing: %w", err)
	}

	return t, nil
}

// newDefaultTracer creates a new Tracer with default values.
func newDefaultTracer() *Tracer {
	t := &Tracer{
		enabled:        true,
		serviceName:    DefaultServiceName,
		serviceVersion: DefaultServiceVersion,
		propagator:     otel.GetTextMapPropagator(),
		sampleRate:     DefaultSampleRate,
		provider:       NoopProvider,
		otlpInsecure:   false,
	}

	// Initialize string pool for reusable string builders
	t.spanNamePool = sync.Pool{
		New: func() any {
			return &strings.Builder{}
		},
	}

	return t
}

// MustNew creates a new Tracer with the given options.
// It panics if the tracing provider fails to initialize.
// Use this for convenience when you want to panic on initialization errors.
//
// Example:
//
//	tracer := tracing.MustNew(
//	    tracing.WithServiceName("my-api"),
//	    tracing.WithStdout(),
//	)
//	defer tracer.Shutdown(context.Background())
func MustNew(opts ...Option) *Tracer {
	t, err := New(opts...)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize tracing: %v", err))
	}
	return t
}

// validate checks that the configuration is valid.
func (t *Tracer) validate() error {
	// Check for validation errors collected during option application
	if len(t.validationErrors) > 0 {
		var errMsgs []string
		for _, err := range t.validationErrors {
			errMsgs = append(errMsgs, err.Error())
		}
		return fmt.Errorf("validation errors: %s", strings.Join(errMsgs, "; "))
	}

	// Validate service name
	if t.serviceName == "" {
		return fmt.Errorf("serviceName: cannot be empty")
	}

	// Validate service version
	if t.serviceVersion == "" {
		return fmt.Errorf("serviceVersion: cannot be empty")
	}

	// Validate sample rate
	if t.sampleRate < 0.0 || t.sampleRate > 1.0 {
		return fmt.Errorf("sampleRate: must be between 0.0 and 1.0, got %f", t.sampleRate)
	}

	// Precompute sampling threshold for integer-based sampling
	if t.sampleRate > 0.0 && t.sampleRate < 1.0 {
		t.samplingThreshold = uint64(t.sampleRate * float64(^uint64(0)))
	} else if t.sampleRate == 1.0 {
		t.samplingThreshold = ^uint64(0)
	} else {
		t.samplingThreshold = 0
	}

	// Validate provider-specific settings
	switch t.provider {
	case NoopProvider:
		// No specific validation needed for noop
	case StdoutProvider:
		// No specific validation needed for stdout
	case OTLPProvider, OTLPHTTPProvider:
		if t.otlpEndpoint == "" {
			t.emitWarning("OTLP endpoint not specified, will use default", "default", "localhost:4317")
			t.otlpEndpoint = "localhost:4317"
		}
	default:
		return fmt.Errorf("provider: unsupported tracing provider %q", t.provider)
	}

	return nil
}

// IsEnabled returns true if tracing is enabled.
func (t *Tracer) IsEnabled() bool {
	return t.enabled
}

// ServiceName returns the service name.
func (t *Tracer) ServiceName() string {
	return t.serviceName
}

// ServiceVersion returns the service version.
func (t *Tracer) ServiceVersion() string {
	return t.serviceVersion
}

// GetTracer returns the OpenTelemetry tracer.
func (t *Tracer) GetTracer() trace.Tracer {
	return t.tracer
}

// GetPropagator returns the OpenTelemetry propagator.
func (t *Tracer) GetPropagator() propagation.TextMapPropagator {
	return t.propagator
}

// GetProvider returns the current tracing provider.
func (t *Tracer) GetProvider() Provider {
	if !t.enabled {
		return ""
	}
	return t.provider
}

// Shutdown gracefully shuts down the tracing system, flushing any pending spans.
// This should be called before the application exits to ensure all spans are exported.
// It shuts down the tracer provider if one was initialized.
// This method is idempotent - calling it multiple times is safe and will only perform shutdown once.
//
// Example:
//
//	tracer, _ := tracing.New(tracing.WithStdout())
//	defer func() {
//	    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	    defer cancel()
//	    if err := tracer.Shutdown(ctx); err != nil {
//	        log.Printf("Error shutting down tracer: %v", err)
//	    }
//	}()
func (t *Tracer) Shutdown(ctx context.Context) error {
	if !t.enabled {
		return nil
	}

	t.shutdownOnce.Do(func() {
		// Shutdown the SDK tracer provider if it exists and is NOT a custom provider
		if t.sdkProvider != nil && !t.customTracerProvider {
			t.emitDebug("Shutting down tracer provider")
			if err := t.sdkProvider.Shutdown(ctx); err != nil {
				t.emitError("Error shutting down tracer provider", "error", err)
				t.shutdownErr = fmt.Errorf("tracer provider shutdown: %w", err)
				return
			}
			t.emitDebug("Tracer provider shut down successfully")
		} else if t.customTracerProvider {
			t.emitDebug("Skipping shutdown of custom tracer provider (managed by user)")
		}
	})

	return t.shutdownErr
}

// StartSpan starts a new span with the given name and options.
// Returns a new context with the span attached and the span itself.
//
// If tracing is disabled, returns the original context and a non-recording span.
// The returned span should always be ended, even if tracing is disabled.
//
// Example:
//
//	ctx, span := tracer.StartSpan(ctx, "database-query")
//	defer tracer.FinishSpan(span, http.StatusOK)
func (t *Tracer) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if !t.enabled {
		return ctx, trace.SpanFromContext(ctx)
	}

	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return ctx, trace.SpanFromContext(ctx)
	default:
	}

	return t.tracer.Start(ctx, name, opts...) //nolint:spancheck // span is returned to caller who manages its lifecycle
}

// FinishSpan completes the span with the given status code.
// Sets the span status based on the HTTP status code:
//   - 2xx-3xx: Success (codes.Ok)
//   - 4xx-5xx: Error (codes.Error)
//
// This method is safe to call multiple times; subsequent calls are no-ops.
//
// Example:
//
//	defer tracer.FinishSpan(span, http.StatusOK)
func (t *Tracer) FinishSpan(span trace.Span, statusCode int) {
	if !t.enabled || span == nil || !span.IsRecording() {
		return
	}

	if statusCode >= 400 {
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.End()
}

// SetSpanAttribute adds an attribute to the span with type-safe handling.
//
// Supported types:
//   - string, int, int64, float64, bool: native OpenTelemetry handling
//   - Other types: converted to string using fmt.Sprintf
//
// This is a no-op if tracing is disabled, span is nil, or span is not recording.
//
// Example:
//
//	tracer.SetSpanAttribute(span, "user.id", 12345)
//	tracer.SetSpanAttribute(span, "user.premium", true)
func (t *Tracer) SetSpanAttribute(span trace.Span, key string, value any) {
	if !t.enabled || span == nil || !span.IsRecording() {
		return
	}
	span.SetAttributes(buildAttribute(key, value))
}

// AddSpanEvent adds an event to the span with optional attributes.
// Events represent important moments in a span's lifetime.
//
// This is a no-op if tracing is disabled, span is nil, or span is not recording.
//
// Example:
//
//	tracer.AddSpanEvent(span, "cache_hit", attribute.String("key", "user:123"))
func (t *Tracer) AddSpanEvent(span trace.Span, name string, attrs ...attribute.KeyValue) {
	if !t.enabled || span == nil || !span.IsRecording() {
		return
	}
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// ExtractTraceContext extracts trace context from HTTP request headers.
// Returns a new context with the extracted trace information.
//
// If no trace context is found in headers, returns the original context.
// Uses W3C Trace Context format by default.
//
// Example:
//
//	ctx := tracer.ExtractTraceContext(ctx, req.Header)
func (t *Tracer) ExtractTraceContext(ctx context.Context, headers http.Header) context.Context {
	if !t.enabled {
		return ctx
	}
	return t.propagator.Extract(ctx, propagation.HeaderCarrier(headers))
}

// InjectTraceContext injects trace context into HTTP headers.
// This allows trace context to propagate across service boundaries.
//
// Uses W3C Trace Context format by default.
// This is a no-op if tracing is disabled.
//
// Example:
//
//	tracer.InjectTraceContext(ctx, resp.Header)
func (t *Tracer) InjectTraceContext(ctx context.Context, headers http.Header) {
	if !t.enabled {
		return
	}
	t.propagator.Inject(ctx, propagation.HeaderCarrier(headers))
}

// StartRequestSpan starts a span for an HTTP request.
// This is used by the middleware to create request spans with standard attributes.
func (t *Tracer) StartRequestSpan(ctx context.Context, req *http.Request, path string, isStatic bool) (context.Context, trace.Span) {
	if !t.enabled {
		return ctx, trace.SpanFromContext(ctx)
	}

	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		t.emitDebug("Context cancelled before span creation", "path", path, "method", req.Method)
		return ctx, trace.SpanFromContext(ctx)
	default:
	}

	// Extract trace context from headers
	ctx = t.ExtractTraceContext(ctx, req.Header)

	// Sampling decision using integer arithmetic
	if t.sampleRate < 1.0 {
		if t.sampleRate == 0.0 {
			t.emitDebug("Request not sampled (0% sample rate)", "path", path, "method", req.Method)
			return ctx, trace.SpanFromContext(ctx)
		}
		counter := t.samplingCounter.Add(1)
		hash := counter * samplingMultiplier
		if hash > t.samplingThreshold {
			t.emitDebug("Request not sampled (probabilistic)", "path", path, "method", req.Method, "sample_rate", t.sampleRate)
			return ctx, trace.SpanFromContext(ctx)
		}
	}

	// Build span name from method and path
	var spanName string
	sb := t.spanNamePool.Get().(*strings.Builder)
	sb.Reset()
	sb.WriteString(req.Method)
	sb.WriteByte(' ')
	sb.WriteString(path)
	spanName = sb.String()
	t.spanNamePool.Put(sb)

	// Start span
	ctx, span := t.tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindServer))

	// Set standard attributes
	attrs := []attribute.KeyValue{
		attribute.String("http.method", req.Method),
		attribute.String("http.url", req.URL.String()),
		attribute.String("http.scheme", req.URL.Scheme),
		attribute.String("http.host", req.Host),
		attribute.String("http.route", path),
		attribute.String("http.user_agent", req.UserAgent()),
		attribute.String("service.name", t.serviceName),
		attribute.String("service.version", t.serviceVersion),
		attribute.Bool("rivaas.router.static_route", isStatic),
	}
	span.SetAttributes(attrs...)

	// Invoke span start hook if configured
	if t.spanStartHook != nil {
		t.spanStartHook(ctx, span, req)
	}

	return ctx, span
}

// FinishRequestSpan completes the span for an HTTP request.
func (t *Tracer) FinishRequestSpan(span trace.Span, statusCode int) {
	if !t.enabled || span == nil || !span.IsRecording() {
		return
	}

	// Set status code attribute
	span.SetAttributes(attribute.Int("http.status_code", statusCode))

	// Set status based on status code
	if statusCode >= 400 {
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
	} else {
		span.SetStatus(codes.Ok, "")
	}

	// Invoke span finish hook if configured
	if t.spanFinishHook != nil {
		t.spanFinishHook(span, statusCode)
	}

	span.End()
}

// buildAttribute creates an OpenTelemetry attribute from a key-value pair.
func buildAttribute(key string, value any) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	case bool:
		return attribute.Bool(key, v)
	default:
		return attribute.String(key, fmt.Sprintf("%v", v))
	}
}

// emitError emits an error event if an event handler is configured.
func (t *Tracer) emitError(msg string, args ...any) {
	if t.eventHandler != nil {
		t.eventHandler(Event{Type: EventError, Message: msg, Args: args})
	}
}

// emitWarning emits a warning event if an event handler is configured.
func (t *Tracer) emitWarning(msg string, args ...any) {
	if t.eventHandler != nil {
		t.eventHandler(Event{Type: EventWarning, Message: msg, Args: args})
	}
}

// emitInfo emits an info event if an event handler is configured.
func (t *Tracer) emitInfo(msg string, args ...any) {
	if t.eventHandler != nil {
		t.eventHandler(Event{Type: EventInfo, Message: msg, Args: args})
	}
}

// emitDebug emits a debug event if an event handler is configured.
func (t *Tracer) emitDebug(msg string, args ...any) {
	if t.eventHandler != nil {
		t.eventHandler(Event{Type: EventDebug, Message: msg, Args: args})
	}
}

// TraceID returns the current trace ID from the active span in the context.
// Returns an empty string if no active span or span context is invalid.
func TraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// SpanID returns the current span ID from the active span in the context.
// Returns an empty string if no active span or span context is invalid.
func SpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// SetSpanAttributeFromContext adds an attribute to the current span from context.
// This is a no-op if tracing is not active.
func SetSpanAttributeFromContext(ctx context.Context, key string, value any) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}
	span.SetAttributes(buildAttribute(key, value))
}

// AddSpanEventFromContext adds an event to the current span from context.
// This is a no-op if tracing is not active.
func AddSpanEventFromContext(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// TraceContext returns the context as-is (it should already contain trace information).
func TraceContext(ctx context.Context) context.Context {
	return ctx
}
