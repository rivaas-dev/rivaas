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
	"errors"
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
	logger         *slog.Logger             // Logger for internal operational events; never nil (uses DiscardHandler when not set)
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
	registerGlobal       bool        // If true, sets otel.SetTracerProvider()
	providerSet          bool        // Tracks if a provider option was explicitly configured
	isStarted            atomic.Bool // Tracks if Start() has been called

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
	cfg := defaultConfig()
	for i, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("tracing: option at index %d cannot be nil", i)
		}
		opt(cfg)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	t, err := newTracerFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	if !t.requiresNetworkInit() {
		if initErr := t.initializeProvider(); initErr != nil {
			return nil, fmt.Errorf("failed to initialize tracing: %w", initErr)
		}
	}
	return t, nil
}

// defaultConfig returns a config with default values.
func defaultConfig() *config {
	return &config{
		serviceName:    DefaultServiceName,
		serviceVersion: DefaultServiceVersion,
		sampleRate:     DefaultSampleRate,
		provider:       NoopProvider,
		propagator:     otel.GetTextMapPropagator(),
	}
}

// validate checks the config and sets samplingThreshold and OTLP default endpoint if needed.
func (c *config) validate() error {
	if len(c.validationErrors) > 0 {
		var errMsgs []string
		for _, err := range c.validationErrors {
			errMsgs = append(errMsgs, err.Error())
		}
		return fmt.Errorf("validation errors: %s", strings.Join(errMsgs, "; "))
	}
	if c.serviceName == "" {
		return errors.New("serviceName: cannot be empty")
	}
	if c.serviceVersion == "" {
		return errors.New("serviceVersion: cannot be empty")
	}
	if c.sampleRate < 0.0 || c.sampleRate > 1.0 {
		return fmt.Errorf("sampleRate: must be between 0.0 and 1.0, got %f", c.sampleRate)
	}
	if c.sampleRate > 0.0 && c.sampleRate < 1.0 {
		c.samplingThreshold = uint64(c.sampleRate * float64(^uint64(0)))
	} else if c.sampleRate == 1.0 {
		c.samplingThreshold = ^uint64(0)
	} else {
		c.samplingThreshold = 0
	}
	switch c.provider {
	case NoopProvider, StdoutProvider:
		// no-op
	case OTLPProvider, OTLPHTTPProvider:
		if c.otlpEndpoint == "" {
			c.otlpEndpointDefaulted = true
			c.otlpEndpoint = "localhost:4317"
		}
	default:
		return fmt.Errorf("provider: unsupported tracing provider %q", c.provider)
	}
	return nil
}

// newTracerFromConfig builds a Tracer from a validated config.
func newTracerFromConfig(cfg *config) (*Tracer, error) {
	logger := cfg.logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	t := &Tracer{
		tracerProvider:       cfg.tracerProvider,
		customTracerProvider: cfg.customTracerProvider,
		registerGlobal:       cfg.registerGlobal,
		serviceName:          cfg.serviceName,
		serviceVersion:       cfg.serviceVersion,
		sampleRate:           cfg.sampleRate,
		samplingThreshold:    cfg.samplingThreshold,
		tracer:               cfg.tracer,
		propagator:           cfg.propagator,
		logger:               logger,
		spanStartHook:        cfg.spanStartHook,
		spanFinishHook:       cfg.spanFinishHook,
		provider:             cfg.provider,
		otlpEndpoint:         cfg.otlpEndpoint,
		otlpInsecure:         cfg.otlpInsecure,
		providerSet:          cfg.providerSet,
		enabled:              true,
		spanNamePool: sync.Pool{
			New: func() any {
				return &strings.Builder{}
			},
		},
	}
	if cfg.otlpEndpointDefaulted {
		t.logger.Warn("OTLP endpoint not specified, will use default", "default", "localhost:4317")
	}
	return t, nil
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

// requiresNetworkInit returns true if the provider requires network initialization.
// OTLP providers need network connections and should be initialized in Start(ctx).
func (t *Tracer) requiresNetworkInit() bool {
	return t.provider == OTLPProvider || t.provider == OTLPHTTPProvider
}

// Start initializes OTLP providers that require network connections.
// The context is used for the OTLP connection establishment.
// This method is idempotent; calling it multiple times is safe.
//
// For non-OTLP providers (Noop, Stdout), this is a no-op since they
// are initialized in New().
//
// Example:
//
//	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
//	defer cancel()
//
//	tracer, _ := tracing.New(tracing.WithOTLP("localhost:4317"))
//	if err := tracer.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
func (t *Tracer) Start(ctx context.Context) error {
	if !t.enabled {
		return nil
	}

	// Idempotent: only start once
	if !t.isStarted.CompareAndSwap(false, true) {
		return nil // Already started
	}

	// Initialize OTLP providers with the provided context
	if t.requiresNetworkInit() {
		if err := t.initializeProviderWithContext(ctx); err != nil {
			return fmt.Errorf("failed to initialize tracing: %w", err)
		}
	}

	return nil
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
			t.logger.Debug("Shutting down tracer provider")
			if err := t.sdkProvider.Shutdown(ctx); err != nil {
				t.logger.Error("Error shutting down tracer provider", "error", err)
				t.shutdownErr = fmt.Errorf("tracer provider shutdown: %w", err)

				return
			}
			t.logger.Debug("Tracer provider shut down successfully")
		} else if t.customTracerProvider {
			t.logger.Debug("Skipping shutdown of custom tracer provider (managed by user)")
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

	// Check if context is already canceled
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

	// Check if context is already canceled
	select {
	case <-ctx.Done():
		t.logger.Debug("Context canceled before span creation", "path", path, "method", req.Method)
		return ctx, trace.SpanFromContext(ctx)
	default:
	}

	// Extract trace context from headers
	ctx = t.ExtractTraceContext(ctx, req.Header)

	// Sampling decision using integer arithmetic
	if t.sampleRate < 1.0 {
		if t.sampleRate == 0.0 {
			t.logger.Debug("Request not sampled (0% sample rate)", "path", path, "method", req.Method)
			return ctx, trace.SpanFromContext(ctx)
		}
		counter := t.samplingCounter.Add(1)
		hash := counter * samplingMultiplier
		if hash > t.samplingThreshold {
			t.logger.Debug("Request not sampled (probabilistic)", "path", path, "method", req.Method, "sample_rate", t.sampleRate)
			return ctx, trace.SpanFromContext(ctx)
		}
	}

	// Build span name from method and path
	var spanName string
	sb, ok := t.spanNamePool.Get().(*strings.Builder)
	if !ok {
		sb = &strings.Builder{}
	}
	sb.Reset()
	_, _ = sb.WriteString(req.Method)
	_ = sb.WriteByte(' ')
	_, _ = sb.WriteString(path)
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
