// Package tracing provides comprehensive OpenTelemetry-based distributed tracing
// for Go applications. It supports multiple exporters (Stdout, OTLP, Noop)
// and integrates seamlessly with the Rivaas router.
//
// # Basic Usage
//
//	config, err := tracing.New(
//	    tracing.WithServiceName("my-service"),
//	    tracing.WithServiceVersion("v1.0.0"),
//	    tracing.WithProvider(tracing.StdoutProvider),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer config.Shutdown(context.Background())
//
//	// Use with router
//	r := router.New()
//	r.SetTracingRecorder(config)
//
// # Thread Safety
//
// All methods are thread-safe. The Config struct is immutable after creation,
// with read-only maps and slices ensuring safe concurrent access without locks.
// Span operations use OpenTelemetry's thread-safe primitives.
//
// # Global State Warning
//
// This package sets the global OpenTelemetry tracer provider via otel.SetTracerProvider().
// Only one tracing configuration should be active per process. Creating multiple
// configurations will cause them to overwrite each other's global tracer provider.
//
// # Providers
//
// Three providers are supported:
//   - NoopProvider (default): No traces exported (safe default)
//   - StdoutProvider: Prints traces to stdout (for development/testing)
//   - OTLPProvider: Sends traces to OTLP collector (for production)
//
// # Custom Spans
//
// Create and manage spans using the provided methods:
//
//	ctx, span := config.StartSpan(ctx, "database-query")
//	defer config.FinishSpan(span, http.StatusOK)
//
//	config.SetSpanAttribute(span, "user.id", "123")
//	config.AddSpanEvent(span, "cache_hit", attribute.String("key", "user:123"))
//
// # Context Propagation
//
// Automatically propagate trace context across service boundaries:
//
//	ctx = config.ExtractTraceContext(ctx, req.Header)
//	config.InjectTraceContext(ctx, resp.Header)
//
// # Sampling
//
// Control which requests are traced using sampling:
//
//	config := tracing.New(
//	    tracing.WithServiceName("my-service"),
//	    tracing.WithSampleRate(0.1), // Sample 10% of requests
//	)
//
// # Environment Variables
//
// The package reads configuration from standard OpenTelemetry environment variables:
//   - OTEL_TRACES_EXPORTER: Provider (otlp, stdout, noop)
//   - OTEL_EXPORTER_OTLP_TRACES_ENDPOINT: OTLP endpoint
//   - OTEL_EXPORTER_OTLP_ENDPOINT: Fallback OTLP endpoint
//   - OTEL_SERVICE_NAME: Service name
//   - OTEL_SERVICE_VERSION: Service version
package tracing

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const (
	// DefaultServiceName is the default service name used for tracing when none is provided.
	DefaultServiceName = "rivaas-service"

	// DefaultServiceVersion is the default service version when none is provided.
	DefaultServiceVersion = "1.0.0"

	// DefaultSampleRate is the default sampling rate (100% of requests).
	DefaultSampleRate = 1.0
)

// TracingProvider represents the available tracing providers.
type TracingProvider string

const (
	// NoopProvider is a no-op provider that doesn't export anything (default).
	NoopProvider TracingProvider = "noop"

	// StdoutProvider exports traces to stdout (development/testing).
	StdoutProvider TracingProvider = "stdout"

	// OTLPProvider exports traces via OTLP gRPC protocol.
	OTLPProvider TracingProvider = "otlp"
)

// Logger is an interface for structured logging.
// Implement this interface to provide custom logging.
type Logger interface {
	Error(msg string, keysAndValues ...any)
	Warn(msg string, keysAndValues ...any)
	Info(msg string, keysAndValues ...any)
	Debug(msg string, keysAndValues ...any)
}

// Config holds OpenTelemetry tracing configuration.
// All operations on Config are thread-safe.
//
// Config implements the TracingRecorder interface for integration with
// the Rivaas router package.
//
// Important: Config is immutable after creation via New(). All configuration
// must be done through functional options passed to New(). The excludePaths map
// and recordHeaders slice are read-only after initialization, making concurrent
// access safe without additional synchronization.
//
// Global State Warning:
// This package sets the global OpenTelemetry tracer provider via otel.SetTracerProvider().
// Only one tracing configuration should be active per process. Creating multiple
// configurations will cause them to overwrite each other's global tracer provider.
type Config struct {
	// Core tracing components
	enabled        bool
	serviceName    string
	serviceVersion string
	tracer         trace.Tracer
	propagator     propagation.TextMapPropagator
	tracerProvider *sdktrace.TracerProvider
	logger         Logger

	// Configuration maps and slices
	excludePaths  map[string]bool
	recordHeaders []string

	// Tracing behavior settings
	sampleRate   float64
	recordParams bool

	// Provider configuration
	provider     TracingProvider
	otlpEndpoint string
	otlpInsecure bool

	// Shutdown coordination
	isShuttingDown       atomic.Bool
	customTracerProvider bool // If true, user provided their own tracer provider
}

// Option defines functional options for tracing configuration.
// Options are applied during Config creation via New().
type Option func(*Config)

// WithTracerProvider allows you to provide a custom OpenTelemetry TracerProvider.
// When using this option, the package will NOT set the global otel.SetTracerProvider(),
// giving you full control over the tracer provider lifecycle and avoiding global state.
//
// This is useful when:
//   - You want to manage the tracer provider lifecycle yourself
//   - You need multiple independent tracing configurations
//   - You want to avoid global state in your application
//
// Example:
//
//	tp := sdktrace.NewTracerProvider(...)
//	config, err := tracing.New(
//	    tracing.WithTracerProvider(tp),
//	    tracing.WithServiceName("my-service"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer tp.Shutdown(context.Background())
//
// Note: When using WithTracerProvider, provider options (StdoutProvider, OTLPProvider, etc.)
// are ignored since you're managing the provider yourself. You must also provide a tracer
// using WithCustomTracer, or the package will create one from your provider.
func WithTracerProvider(provider *sdktrace.TracerProvider) Option {
	return func(c *Config) {
		c.tracerProvider = provider
		c.customTracerProvider = true
	}
}

// WithServiceName sets the service name for tracing.
// This name appears in span attributes as 'service.name'.
//
// Example:
//
//	config := tracing.New(tracing.WithServiceName("my-api"))
func WithServiceName(name string) Option {
	return func(c *Config) {
		c.serviceName = name
	}
}

// WithServiceVersion sets the service version for tracing.
// This version appears in span attributes as 'service.version'.
//
// Example:
//
//	config := tracing.New(tracing.WithServiceVersion("v1.2.3"))
func WithServiceVersion(version string) Option {
	return func(c *Config) {
		c.serviceVersion = version
	}
}

// WithSampleRate sets the sampling rate (0.0 to 1.0).
// Values outside this range will be clamped to valid bounds.
//
// A rate of 1.0 samples all requests, 0.5 samples 50%, and 0.0 samples none.
// Sampling decisions are made per-request using a random number generator.
//
// Example:
//
//	config := tracing.New(tracing.WithSampleRate(0.1)) // Sample 10% of requests
func WithSampleRate(rate float64) Option {
	return func(c *Config) {
		if rate < 0.0 {
			rate = 0.0
		}
		if rate > 1.0 {
			rate = 1.0
		}
		c.sampleRate = rate
	}
}

// MaxExcludedPaths is the maximum number of paths that can be excluded from tracing.
// This limit prevents unbounded memory growth from dynamically added exclusions.
const MaxExcludedPaths = 1000

// WithExcludePaths excludes specific paths from tracing.
// Excluded paths will not create spans or record any tracing data.
// This is useful for health checks, metrics endpoints, etc.
//
// Maximum of 1000 paths can be excluded to prevent unbounded memory growth.
// If more paths are provided, only the first 1000 will be excluded.
//
// Note: If you need to exclude more than 1000 paths, consider using a
// pattern-based approach or implementing custom path filtering logic.
//
// Example:
//
//	config := tracing.New(tracing.WithExcludePaths("/health", "/metrics"))
func WithExcludePaths(paths ...string) Option {
	return func(c *Config) {
		for i, path := range paths {
			if i >= MaxExcludedPaths {
				// Limit reached - skip remaining paths
				break
			}
			c.excludePaths[path] = true
		}
	}
}

// sensitiveHeaders contains header names that should never be recorded in traces.
// These headers typically contain authentication credentials or other sensitive data.
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
//	config := tracing.New(tracing.WithHeaders("X-Request-ID", "User-Agent"))
func WithHeaders(headers ...string) Option {
	return func(c *Config) {
		// Filter out sensitive headers
		filtered := make([]string, 0, len(headers))
		for _, h := range headers {
			if !sensitiveHeaders[strings.ToLower(h)] {
				filtered = append(filtered, h)
			}
		}
		// Defensive copy to ensure immutability
		c.recordHeaders = make([]string, len(filtered))
		copy(c.recordHeaders, filtered)
	}
}

// WithDisableParams disables recording URL query parameters as span attributes.
// By default, all query parameters are recorded. Use this option if parameters
// may contain sensitive data (passwords, tokens, etc.).
//
// Example:
//
//	config := tracing.New(tracing.WithDisableParams())
func WithDisableParams() Option {
	return func(c *Config) {
		c.recordParams = false
	}
}

// WithCustomTracer allows using a custom OpenTelemetry tracer.
// This is useful when you need specific tracer configuration or
// want to use a tracer from an existing OpenTelemetry setup.
//
// Example:
//
//	tp := trace.NewTracerProvider(...)
//	tracer := tp.Tracer("my-tracer")
//	config := tracing.New(tracing.WithCustomTracer(tracer))
func WithCustomTracer(tracer trace.Tracer) Option {
	return func(c *Config) {
		c.tracer = tracer
	}
}

// WithCustomPropagator allows using a custom OpenTelemetry propagator.
// This is useful for custom trace context propagation formats.
// By default, uses the global propagator from otel.GetTextMapPropagator().
//
// Example:
//
//	prop := propagation.TraceContext{}
//	config := tracing.New(tracing.WithCustomPropagator(prop))
func WithCustomPropagator(propagator propagation.TextMapPropagator) Option {
	return func(c *Config) {
		c.propagator = propagator
	}
}

// WithProvider sets the tracing provider (exporter).
// Use with one of: NoopProvider, StdoutProvider, OTLPProvider
//
// Example:
//
//	config := tracing.New(tracing.WithProvider(tracing.StdoutProvider))
//	config := tracing.New(tracing.WithProvider(tracing.OTLPProvider))
func WithProvider(provider TracingProvider) Option {
	return func(c *Config) {
		c.provider = provider
	}
}

// WithOTLPEndpoint sets the OTLP endpoint (e.g., "localhost:4317").
// Only used when provider is OTLPProvider.
//
// Example:
//
//	config := tracing.New(
//	    tracing.WithProvider(tracing.OTLPProvider),
//	    tracing.WithOTLPEndpoint("jaeger:4317"),
//	)
func WithOTLPEndpoint(endpoint string) Option {
	return func(c *Config) {
		c.otlpEndpoint = endpoint
	}
}

// WithOTLPInsecure enables insecure gRPC for OTLP.
// Default is false (uses TLS). Set to true for local development.
//
// Example:
//
//	config := tracing.New(
//	    tracing.WithProvider(tracing.OTLPProvider),
//	    tracing.WithOTLPInsecure(true),
//	)
func WithOTLPInsecure(insecure bool) Option {
	return func(c *Config) {
		c.otlpInsecure = insecure
	}
}

// WithLogger sets a custom logger for tracing errors and warnings.
// This allows you to integrate tracing logs with your application's logging system.
//
// Example:
//
//	config := tracing.New(tracing.WithLogger(myLogger))
func WithLogger(logger Logger) Option {
	return func(c *Config) {
		c.logger = logger
	}
}

// New creates a new tracing configuration with the given options.
// Returns an error if the tracing provider fails to initialize.
// For a version that panics on error, use MustNew.
//
// IMPORTANT: This function sets the global OpenTelemetry tracer provider via otel.SetTracerProvider.
// Creating multiple tracing configurations in the same process will cause them to overwrite each other's
// global tracer provider. This is a limitation of the OpenTelemetry Go SDK. If you need multiple independent
// tracing configurations, consider running them in separate processes.
//
// For most applications, you should create a single tracing configuration at startup and reuse it
// throughout the application lifecycle.
//
// Default configuration:
//   - Service name: DefaultServiceName ("rivaas-service")
//   - Service version: DefaultServiceVersion ("1.0.0")
//   - Sample rate: DefaultSampleRate (1.0 = 100%)
//   - Parameter recording: enabled
//   - Provider: NoopProvider (no traces exported)
//
// Example:
//
//	config, err := tracing.New(
//	    tracing.WithServiceName("my-api"),
//	    tracing.WithProvider(tracing.StdoutProvider),
//	    tracing.WithSampleRate(0.1),
//	    tracing.WithExcludePaths("/health"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer config.Shutdown(context.Background())
func New(opts ...Option) (*Config, error) {
	config := newDefaultConfig()
	config.readFromEnv()

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	// Validate configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Initialize the provider
	if err := config.initializeProvider(); err != nil {
		return nil, fmt.Errorf("failed to initialize tracing: %w", err)
	}

	return config, nil
}

// newDefaultConfig creates a new tracing configuration with default values.
func newDefaultConfig() *Config {
	return &Config{
		enabled:        true,
		serviceName:    DefaultServiceName,
		serviceVersion: DefaultServiceVersion,
		propagator:     otel.GetTextMapPropagator(),
		excludePaths:   make(map[string]bool),
		sampleRate:     DefaultSampleRate,
		recordParams:   true,
		provider:       NoopProvider,
		otlpInsecure:   false,
	}
}

// MustNew creates a new tracing configuration with the given options.
// It panics if the tracing provider fails to initialize.
// Use this for convenience when you want to fail fast on initialization errors.
//
// Example:
//
//	config := tracing.MustNew(
//	    tracing.WithServiceName("my-api"),
//	    tracing.WithProvider(tracing.StdoutProvider),
//	)
//	defer config.Shutdown(context.Background())
func MustNew(opts ...Option) *Config {
	config, err := New(opts...)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize tracing: %v", err))
	}
	return config
}

// validate checks that the configuration is valid.
func (c *Config) validate() error {
	// Validate service name
	if c.serviceName == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	// Validate service version
	if c.serviceVersion == "" {
		return fmt.Errorf("service version cannot be empty")
	}

	// Validate sample rate
	if c.sampleRate < 0.0 || c.sampleRate > 1.0 {
		return fmt.Errorf("sample rate must be between 0.0 and 1.0, got %f", c.sampleRate)
	}

	// Validate provider-specific settings
	switch c.provider {
	case NoopProvider:
		// No specific validation needed for noop
	case StdoutProvider:
		// No specific validation needed for stdout
	case OTLPProvider:
		if c.otlpEndpoint == "" {
			c.logWarn("OTLP endpoint not specified, will use default", "default", "localhost:4317")
			c.otlpEndpoint = "localhost:4317"
		}
	default:
		return fmt.Errorf("unsupported tracing provider: %s", c.provider)
	}

	return nil
}

// readFromEnv reads configuration from environment variables.
func (c *Config) readFromEnv() {
	// OTEL_TRACES_EXPORTER
	if exporter := os.Getenv("OTEL_TRACES_EXPORTER"); exporter != "" {
		switch strings.ToLower(exporter) {
		case "otlp":
			c.provider = OTLPProvider
		case "stdout":
			c.provider = StdoutProvider
		case "none", "noop":
			c.provider = NoopProvider
		}
	}

	// OTEL_EXPORTER_OTLP_TRACES_ENDPOINT or OTEL_EXPORTER_OTLP_ENDPOINT
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"); endpoint != "" {
		c.otlpEndpoint = endpoint
	} else if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		c.otlpEndpoint = endpoint
	}

	// OTEL_SERVICE_NAME
	if serviceName := os.Getenv("OTEL_SERVICE_NAME"); serviceName != "" {
		c.serviceName = serviceName
	}

	// OTEL_SERVICE_VERSION
	if serviceVersion := os.Getenv("OTEL_SERVICE_VERSION"); serviceVersion != "" {
		c.serviceVersion = serviceVersion
	}
}

// IsEnabled returns true if tracing is enabled.
func (c *Config) IsEnabled() bool {
	return c.enabled
}

// GetServiceName returns the service name.
func (c *Config) GetServiceName() string {
	return c.serviceName
}

// GetServiceVersion returns the service version.
func (c *Config) GetServiceVersion() string {
	return c.serviceVersion
}

// GetTracer returns the OpenTelemetry tracer.
func (c *Config) GetTracer() trace.Tracer {
	return c.tracer
}

// GetPropagator returns the OpenTelemetry propagator.
func (c *Config) GetPropagator() propagation.TextMapPropagator {
	return c.propagator
}

// GetProvider returns the current tracing provider.
func (c *Config) GetProvider() TracingProvider {
	if !c.enabled {
		return ""
	}
	return c.provider
}

// ShouldExcludePath returns true if the given path should be excluded from tracing.
// Safe for concurrent access as excludePaths is read-only after Config creation.
func (c *Config) ShouldExcludePath(path string) bool {
	return c.excludePaths[path]
}

// Shutdown gracefully shuts down the tracing system, flushing any pending spans.
// This should be called before the application exits to ensure all spans are exported.
// It shuts down the tracer provider if one was initialized.
// This method is idempotent - calling it multiple times is safe and will only perform shutdown once.
//
// Example:
//
//	config, _ := tracing.New(tracing.WithProvider(tracing.StdoutProvider))
//	defer func() {
//	    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	    defer cancel()
//	    if err := config.Shutdown(ctx); err != nil {
//	        log.Printf("Error shutting down tracer: %v", err)
//	    }
//	}()
func (c *Config) Shutdown(ctx context.Context) error {
	if !c.enabled {
		return nil
	}

	// Use CompareAndSwap to ensure only one goroutine performs shutdown
	// If already shutting down or shut down, return immediately
	if !c.isShuttingDown.CompareAndSwap(false, true) {
		return nil // Already shutting down or shut down
	}

	// Shutdown the tracer provider if it exists and is NOT a custom provider
	// User-provided providers should be managed by the user
	if c.tracerProvider != nil && !c.customTracerProvider {
		c.logDebug("Shutting down tracer provider")
		if err := c.tracerProvider.Shutdown(ctx); err != nil {
			c.logError("Error shutting down tracer provider", "error", err)
			return fmt.Errorf("tracer provider shutdown: %w", err)
		}
		c.logDebug("Tracer provider shut down successfully")
	} else if c.customTracerProvider {
		c.logDebug("Skipping shutdown of custom tracer provider (managed by user)")
	}

	return nil
}

// logError logs an error message if a logger is configured.
func (c *Config) logError(msg string, keysAndValues ...interface{}) {
	if c.logger != nil {
		c.logger.Error(msg, keysAndValues...)
	}
}

// logWarn logs a warning message if a logger is configured.
func (c *Config) logWarn(msg string, keysAndValues ...interface{}) {
	if c.logger != nil {
		c.logger.Warn(msg, keysAndValues...)
	}
}

// logInfo logs an info message if a logger is configured.
func (c *Config) logInfo(msg string, keysAndValues ...interface{}) {
	if c.logger != nil {
		c.logger.Info(msg, keysAndValues...)
	}
}

// logDebug logs a debug message if a logger is configured.
func (c *Config) logDebug(msg string, keysAndValues ...interface{}) {
	if c.logger != nil {
		c.logger.Debug(msg, keysAndValues...)
	}
}

// initializeProvider initializes the tracing provider based on configuration.
func (c *Config) initializeProvider() error {
	switch c.provider {
	case NoopProvider:
		return c.initNoopProvider()
	case StdoutProvider:
		return c.initStdoutProvider()
	case OTLPProvider:
		return c.initOTLPProvider()
	default:
		return fmt.Errorf("unsupported tracing provider: %s", c.provider)
	}
}

// buildAttribute creates an OpenTelemetry attribute from a key-value pair.
// Supports string, int, int64, float64, and bool types natively.
// Other types are converted to string using fmt.Sprintf.
func buildAttribute(key string, value interface{}) attribute.KeyValue {
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

// StartSpan starts a new span with the given name and options.
// Returns a new context with the span attached and the span itself.
//
// If tracing is disabled, returns the original context and a non-recording span.
// The returned span should always be ended, even if tracing is disabled.
//
// If the context is already cancelled, returns immediately without creating a span.
//
// Example:
//
//	ctx, span := config.StartSpan(ctx, "database-query")
//	defer config.FinishSpan(span, http.StatusOK)
//	// ... perform operation
func (c *Config) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if !c.enabled {
		return ctx, trace.SpanFromContext(ctx)
	}

	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return ctx, trace.SpanFromContext(ctx)
	default:
	}

	return c.tracer.Start(ctx, name, opts...)
}

// FinishSpan completes the span with the given status code.
// Sets the span status based on the HTTP status code:
//   - 2xx-3xx: Success (codes.Ok)
//   - 4xx-5xx: Error (codes.Error)
//
// This method is safe to call multiple times; subsequent calls are no-ops.
// Always safe to call even if tracing is disabled, span is nil, or span is not recording.
//
// Example:
//
//	defer config.FinishSpan(span, http.StatusOK)
func (c *Config) FinishSpan(span trace.Span, statusCode int) {
	if !c.enabled || span == nil || !span.IsRecording() {
		return
	}

	// Set status based on status code
	if statusCode >= 400 {
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.End()
}

// SetSpanAttribute adds an attribute to the span with type-safe handling.
//
// Supported types with native OpenTelemetry handling:
//   - string: attribute.String
//   - int: attribute.Int
//   - int64: attribute.Int64
//   - float64: attribute.Float64
//   - bool: attribute.Bool
//
// All other types are converted to string using fmt.Sprintf("%v", value).
// This is a no-op if tracing is disabled, span is nil, or span is not recording.
//
// Example:
//
//	config.SetSpanAttribute(span, "user.id", 12345)
//	config.SetSpanAttribute(span, "user.premium", true)
func (c *Config) SetSpanAttribute(span trace.Span, key string, value interface{}) {
	if !c.enabled || span == nil || !span.IsRecording() {
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
//	config.AddSpanEvent(span, "cache_hit", attribute.String("key", "user:123"))
//	config.AddSpanEvent(span, "validation_failed")
func (c *Config) AddSpanEvent(span trace.Span, name string, attrs ...attribute.KeyValue) {
	if !c.enabled || span == nil || !span.IsRecording() {
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
//	ctx := config.ExtractTraceContext(ctx, req.Header)
func (c *Config) ExtractTraceContext(ctx context.Context, headers http.Header) context.Context {
	if !c.enabled {
		return ctx
	}
	return c.propagator.Extract(ctx, propagation.HeaderCarrier(headers))
}

// InjectTraceContext injects trace context into HTTP headers.
// This allows trace context to propagate across service boundaries.
//
// Uses W3C Trace Context format by default.
// This is a no-op if tracing is disabled.
//
// Example:
//
//	config.InjectTraceContext(ctx, resp.Header)
func (c *Config) InjectTraceContext(ctx context.Context, headers http.Header) {
	if !c.enabled {
		return
	}
	c.propagator.Inject(ctx, propagation.HeaderCarrier(headers))
}

// StartRequestSpan starts a span for an HTTP request.
func (c *Config) StartRequestSpan(ctx context.Context, req *http.Request, path string, isStatic bool) (context.Context, trace.Span) {
	if !c.enabled {
		return ctx, trace.SpanFromContext(ctx)
	}

	// Extract trace context from headers
	ctx = c.ExtractTraceContext(ctx, req.Header)

	// Apply sampling rate (rand.Float64() is thread-safe in math/rand/v2)
	if c.sampleRate < 1.0 && rand.Float64() > c.sampleRate {
		// Don't sample this request - return non-recording span
		return ctx, trace.SpanFromContext(ctx)
	}

	// Start span
	spanName := fmt.Sprintf("%s %s", req.Method, path)
	ctx, span := c.tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindServer))

	// Set standard attributes
	span.SetAttributes(
		attribute.String("http.method", req.Method),
		attribute.String("http.url", req.URL.String()),
		attribute.String("http.scheme", req.URL.Scheme),
		attribute.String("http.host", req.Host),
		attribute.String("http.route", path),
		attribute.String("http.user_agent", req.UserAgent()),
		attribute.String("service.name", c.serviceName),
		attribute.String("service.version", c.serviceVersion),
		attribute.Bool("rivaas.router.static_route", isStatic),
	)

	// Record URL parameters if enabled
	if c.recordParams && len(req.URL.Query()) > 0 {
		for key, values := range req.URL.Query() {
			if len(values) > 0 {
				span.SetAttributes(attribute.StringSlice(
					fmt.Sprintf("http.request.param.%s", key),
					values,
				))
			}
		}
	}

	// Record specific headers if configured
	for _, header := range c.recordHeaders {
		if value := req.Header.Get(header); value != "" {
			span.SetAttributes(attribute.String(
				fmt.Sprintf("http.request.header.%s", strings.ToLower(header)),
				value,
			))
		}
	}

	return ctx, span
}

// FinishRequestSpan completes the span for an HTTP request.
func (c *Config) FinishRequestSpan(span trace.Span, statusCode int) {
	if !c.enabled || span == nil || !span.IsRecording() {
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

	span.End()
}

// Context helpers for working with trace context

// TraceID returns the current trace ID from the active span in the context.
// Returns an empty string if no active span or span context is invalid.
//
// Example:
//
//	traceID := tracing.TraceID(ctx)
//	log.Printf("Processing request with trace ID: %s", traceID)
func TraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// SpanID returns the current span ID from the active span in the context.
// Returns an empty string if no active span or span context is invalid.
//
// Example:
//
//	spanID := tracing.SpanID(ctx)
//	log.Printf("Current span ID: %s", spanID)
func SpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// SetSpanAttributeFromContext adds an attribute to the current span from context.
// This is a no-op if tracing is not active.
// Supports string, int, int64, float64, and bool types natively.
func SetSpanAttributeFromContext(ctx context.Context, key string, value interface{}) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}
	span.SetAttributes(buildAttribute(key, value))
}

// AddSpanEventFromContext adds an event to the current span from context with optional attributes.
// This is a no-op if tracing is not active.
func AddSpanEventFromContext(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// NewProduction creates a production-optimized tracing configuration.
// This configuration uses conservative defaults suitable for production workloads:
//   - 10% sampling rate to reduce overhead
//   - Common health/metrics endpoints excluded
//   - Query parameter recording disabled to prevent sensitive data leakage
//   - OTLP provider (configure endpoint via WithOTLPEndpoint or OTEL_EXPORTER_OTLP_TRACES_ENDPOINT)
//
// Returns an error if the tracing provider fails to initialize.
//
// Example:
//
//	config, err := tracing.NewProduction("my-api", "v1.2.3")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer config.Shutdown(context.Background())
func NewProduction(serviceName, serviceVersion string) (*Config, error) {
	return New(
		WithServiceName(serviceName),
		WithServiceVersion(serviceVersion),
		WithProvider(OTLPProvider),
		WithSampleRate(0.1), // 10% sampling
		WithExcludePaths("/health", "/metrics", "/ready", "/live", "/healthz"),
		WithDisableParams(), // Don't record potentially sensitive query params
	)
}

// NewDevelopment creates a development-optimized tracing configuration.
// This configuration uses aggressive settings for maximum visibility:
//   - 100% sampling rate for full trace coverage
//   - Query parameter recording enabled
//   - Only basic health checks excluded
//   - Stdout provider for easy debugging
//
// Returns an error if the tracing provider fails to initialize.
//
// Example:
//
//	config, err := tracing.NewDevelopment("my-api", "dev")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer config.Shutdown(context.Background())
func NewDevelopment(serviceName, serviceVersion string) (*Config, error) {
	return New(
		WithServiceName(serviceName),
		WithServiceVersion(serviceVersion),
		WithProvider(StdoutProvider),
		WithSampleRate(1.0), // 100% sampling
		WithExcludePaths("/health", "/healthz"),
	)
}

// TraceContext returns the OpenTelemetry trace context.
// This can be used for manual span creation or context propagation.
// If tracing is not enabled, it returns the request context for proper cancellation support.
func TraceContext(ctx context.Context) context.Context {
	// Return the context as-is - it should already contain trace information
	return ctx
}
