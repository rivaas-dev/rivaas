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
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

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

// Sampling multiplier (Knuth's multiplicative hash constant)
// Used for deterministic sampling with uniform distribution.
// The sampling counter wraps around at uint64 max (2^64-1), which ensures
// uniform distribution continues even after overflow (which would take ~584 billion
// years at 1 request per nanosecond, so overflow is not a practical concern).
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
)

// SpanStartHook is called when a request span is started.
// It receives the context, span, and HTTP request.
// This can be used for custom attribute injection, dynamic sampling, or integration with APM tools.
//
// Example:
//
//	hook := func(ctx context.Context, span trace.Span, req *http.Request) {
//	    // Add custom business logic attributes
//	    span.SetAttributes(attribute.String("tenant.id", extractTenantID(req)))
//	}
type SpanStartHook func(ctx context.Context, span trace.Span, req *http.Request)

// SpanFinishHook is called when a request span is finished.
// It receives the span and the HTTP status code.
// This can be used for custom metrics, logging, or post-processing.
//
// Example:
//
//	hook := func(span trace.Span, statusCode int) {
//	    // Record custom metrics
//	    if statusCode >= 500 {
//	        metrics.RecordServerError()
//	    }
//	}
type SpanFinishHook func(span trace.Span, statusCode int)

// Config holds OpenTelemetry tracing configuration.
// All operations on Config are thread-safe.
//
// Config implements the Recorder interface for integration with
// the Rivaas router package.
//
// Important: Config is immutable after creation via New(). All configuration
// must be done through functional options passed to New(). The excludePaths map
// and recordHeaders slice are read-only after initialization, making concurrent
// access safe without additional synchronization.
//
// Global State:
// By default, this package does NOT set the global OpenTelemetry tracer provider.
// Use WithGlobalTracerProvider() option if you want global registration.
// This allows multiple tracing configurations to coexist in the same process.
type Config struct {
	// Core tracing components
	tracer          trace.Tracer
	propagator      propagation.TextMapPropagator
	tracerProvider  *sdktrace.TracerProvider
	eventHandler    EventHandler // Handler for internal operational events
	excludePaths    map[string]bool
	excludePrefixes []string         // Path prefixes to exclude
	excludePatterns []*regexp.Regexp // Compiled regex patterns for path exclusion
	recordHeaders   []string
	serviceName     string
	serviceVersion  string
	provider        Provider
	otlpEndpoint    string

	// Parameter recording configuration
	recordParamsList []string        // Whitelist of params to record (nil = all)
	excludeParams    map[string]bool // Blacklist of params to exclude

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
	recordParams         bool
	otlpInsecure         bool
	enabled              bool
	customTracerProvider bool
	registerGlobal       bool // If true, sets otel.SetTracerProvider()

	// Validation errors (collected during option application)
	validationErrors []error

	// String pool for reusable string builders
	spanNamePool sync.Pool
}

// Option defines functional options for tracing configuration.
// Options are applied during Config creation via New().
type Option func(*Config)

// WithTracerProvider allows you to provide a custom OpenTelemetry TracerProvider.
// When using this option, the package will NOT set the global otel.SetTracerProvider()
// by default. Use WithGlobalTracerProvider() if you want global registration.
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
		// Note: registerGlobal stays false unless explicitly set
	}
}

// WithGlobalTracerProvider registers the tracer provider as the global
// OpenTelemetry tracer provider via otel.SetTracerProvider().
// By default, tracer providers are not registered globally to allow multiple
// tracing configurations to coexist in the same process.
//
// Example:
//
//	config := tracing.New(
//	    tracing.WithProvider(tracing.OTLPProvider),
//	    tracing.WithGlobalTracerProvider(), // Register as global default
//	)
func WithGlobalTracerProvider() Option {
	return func(c *Config) {
		c.registerGlobal = true
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
// Sampling decisions are made per-request based on the configured rate.
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
// This limit prevents unbounded growth of excluded paths.
const MaxExcludedPaths = 1000

// WithExcludePaths excludes specific paths from tracing.
// Excluded paths will not create spans or record any tracing data.
// This is useful for health checks, metrics endpoints, etc.
//
// Maximum of 1000 paths can be excluded to prevent unbounded growth.
// If more paths are provided, only the first 1000 will be excluded and a
// warning will be logged if a logger is configured.
//
// Note: If you need to exclude more than 1000 paths, consider using
// WithExcludePathPattern() for regex-based matching.
//
// Example:
//
//	config := tracing.New(tracing.WithExcludePaths("/health", "/metrics"))
func WithExcludePaths(paths ...string) Option {
	return func(c *Config) {
		for i, path := range paths {
			if i >= MaxExcludedPaths {
				// Limit reached - log warning and skip remaining paths
				c.emitWarning("Excluded paths limit reached",
					"limit", MaxExcludedPaths,
					"total_provided", len(paths),
					"dropped", len(paths)-MaxExcludedPaths,
					"hint", "consider using WithExcludePathPattern() for regex-based matching",
				)
				break
			}
			c.excludePaths[path] = true
		}
	}
}

// WithExcludePathPattern excludes paths matching the given regex pattern from tracing.
// This is useful when you need to exclude many paths that follow a pattern,
// or when the exact paths are not known at configuration time.
//
// The pattern is compiled once during configuration, so invalid regex patterns
// will cause New() to return an error.
//
// Example:
//
//	// Exclude all paths starting with /internal/
//	config := tracing.New(tracing.WithExcludePathPattern("^/internal/.*"))
//
//	// Exclude all health check endpoints
//	config := tracing.New(tracing.WithExcludePathPattern("^/(health|ready|live)"))
func WithExcludePathPattern(pattern string) Option {
	return func(c *Config) {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			// Store error to be returned during validation
			// We can't return error from Option, so we'll validate later
			if c.validationErrors == nil {
				c.validationErrors = make([]error, 0, 1)
			}
			c.validationErrors = append(c.validationErrors, fmt.Errorf("invalid regex pattern for path exclusion %q: %w", pattern, err))
			c.emitError("Invalid regex pattern for path exclusion",
				"pattern", pattern,
				"error", err,
			)
			return
		}
		if c.excludePatterns == nil {
			c.excludePatterns = make([]*regexp.Regexp, 0, 1)
		}
		c.excludePatterns = append(c.excludePatterns, compiled)
	}
}

// WithExcludePrefixes excludes paths with the given prefixes from tracing.
// This is useful for excluding entire path hierarchies like /debug/, /internal/, etc.
//
// Example:
//
//	config := tracing.MustNew(
//	    tracing.WithExcludePrefixes("/debug/", "/internal/"),
//	)
func WithExcludePrefixes(prefixes ...string) Option {
	return func(c *Config) {
		c.excludePrefixes = append(c.excludePrefixes, prefixes...)
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

// WithRecordParams specifies which URL query parameters to record as span attributes.
// Only parameters in this list will be recorded. This provides fine-grained control
// over which parameters are traced.
//
// If this option is not used, all query parameters are recorded by default
// (unless WithDisableParams is used).
//
// Example:
//
//	config := tracing.New(
//	    tracing.WithRecordParams("user_id", "request_id", "page"),
//	)
func WithRecordParams(params ...string) Option {
	return func(c *Config) {
		if len(params) > 0 {
			// Defensive copy to ensure immutability
			c.recordParamsList = make([]string, len(params))
			copy(c.recordParamsList, params)
			c.recordParams = true
		}
	}
}

// WithExcludeParams specifies which URL query parameters to exclude from tracing.
// This is useful for blacklisting sensitive parameters while recording all others.
//
// Parameters in this list will never be recorded, even if WithRecordParams includes them.
// This option works in combination with WithRecordParams for fine-grained control.
//
// Example:
//
//	config := tracing.New(
//	    tracing.WithExcludeParams("password", "token", "api_key", "secret"),
//	)
func WithExcludeParams(params ...string) Option {
	return func(c *Config) {
		if len(params) > 0 {
			if c.excludeParams == nil {
				c.excludeParams = make(map[string]bool, len(params))
			}
			for _, param := range params {
				c.excludeParams[param] = true
			}
		}
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
func WithProvider(provider Provider) Option {
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

// WithEventHandler sets a custom event handler for internal operational events.
// Use this for advanced use cases like sending errors to Sentry, custom alerting,
// or integrating with non-slog logging systems.
//
// Example:
//
//	tracing.New(tracing.WithEventHandler(func(e tracing.Event) {
//	    if e.Type == tracing.EventError {
//	        sentry.CaptureMessage(e.Message)
//	    }
//	    myLogger.Log(e.Type, e.Message, e.Args...)
//	}))
func WithEventHandler(handler EventHandler) Option {
	return func(c *Config) {
		c.eventHandler = handler
	}
}

// WithLogger sets the logger for internal operational events using the default event handler.
// This is a convenience wrapper around WithEventHandler that logs events to the provided slog.Logger.
//
// Example:
//
//	// Use stdlib slog
//	tracing.New(tracing.WithLogger(slog.Default()))
//
//	// Use custom slog logger
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	tracing.New(tracing.WithLogger(logger))
func WithLogger(logger *slog.Logger) Option {
	return WithEventHandler(DefaultEventHandler(logger))
}

// WithSpanStartHook sets a callback that is invoked when a request span is started.
// The hook receives the context, span, and HTTP request, allowing custom attribute
// injection, dynamic sampling decisions, or integration with APM tools.
//
// This is useful for:
//   - Adding custom business logic attributes
//   - Dynamic span configuration based on request
//   - Integration with external monitoring systems
//   - Request-specific tracing behavior
//
// Example:
//
//	hook := func(ctx context.Context, span trace.Span, req *http.Request) {
//	    // Add tenant ID from request header
//	    if tenantID := req.Header.Get("X-Tenant-ID"); tenantID != "" {
//	        span.SetAttributes(attribute.String("tenant.id", tenantID))
//	    }
//	}
//	config := tracing.New(tracing.WithSpanStartHook(hook))
func WithSpanStartHook(hook SpanStartHook) Option {
	return func(c *Config) {
		c.spanStartHook = hook
	}
}

// WithSpanFinishHook sets a callback that is invoked when a request span is finished.
// The hook receives the span and HTTP status code, allowing custom metrics recording,
// logging, or post-processing.
//
// This is useful for:
//   - Recording custom metrics based on span data
//   - Logging span information
//   - Post-processing trace data
//   - Integration with external systems
//
// Example:
//
//	hook := func(span trace.Span, statusCode int) {
//	    // Record custom metrics
//	    if statusCode >= 500 {
//	        metrics.IncrementServerErrors()
//	    }
//	}
//	config := tracing.New(tracing.WithSpanFinishHook(hook))
func WithSpanFinishHook(hook SpanFinishHook) Option {
	return func(c *Config) {
		c.spanFinishHook = hook
	}
}

// New creates a new tracing configuration with the given options.
// Returns an error if the tracing provider fails to initialize.
// For a version that panics on error, use MustNew.
//
// By default, this function does NOT set the global OpenTelemetry tracer provider.
// Use WithGlobalTracerProvider() if you want to register the tracer provider as the global default.
//
// This allows multiple tracing configurations to coexist in the same process,
// and makes it easier to integrate Rivaas into larger binaries that already
// manage their own global tracer provider.
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
	config := &Config{
		enabled:         true,
		serviceName:     DefaultServiceName,
		serviceVersion:  DefaultServiceVersion,
		propagator:      otel.GetTextMapPropagator(),
		excludePaths:    make(map[string]bool),
		excludePatterns: make([]*regexp.Regexp, 0),
		excludeParams:   make(map[string]bool),
		sampleRate:      DefaultSampleRate,
		recordParams:    true,
		provider:        NoopProvider,
		otlpInsecure:    false,
	}

	// Initialize string pool for reusable string builders
	config.spanNamePool = sync.Pool{
		New: func() interface{} {
			return &strings.Builder{}
		},
	}

	return config
}

// MustNew creates a new tracing configuration with the given options.
// It panics if the tracing provider fails to initialize.
// Use this for convenience when you want to panic on initialization errors.
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
	// Check for validation errors collected during option application
	if len(c.validationErrors) > 0 {
		var errMsgs []string
		for _, err := range c.validationErrors {
			errMsgs = append(errMsgs, err.Error())
		}
		return fmt.Errorf("validation errors: %s", strings.Join(errMsgs, "; "))
	}

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

	// Precompute sampling threshold for integer-based sampling
	if c.sampleRate > 0.0 && c.sampleRate < 1.0 {
		// Use max uint64 value to avoid overflow
		// Convert sample rate to threshold: 0.5 -> 0x7FFFFFFFFFFFFFFF
		c.samplingThreshold = uint64(c.sampleRate * float64(^uint64(0)))
	} else if c.sampleRate == 1.0 {
		// 100% sampling - set threshold to max so all samples pass
		c.samplingThreshold = ^uint64(0)
	} else {
		// 0% sampling - set threshold to 0 so no samples pass
		c.samplingThreshold = 0
	}

	// Validate provider-specific settings
	switch c.provider {
	case NoopProvider:
		// No specific validation needed for noop
	case StdoutProvider:
		// No specific validation needed for stdout
	case OTLPProvider:
		if c.otlpEndpoint == "" {
			c.emitWarning("OTLP endpoint not specified, will use default", "default", "localhost:4317")
			c.otlpEndpoint = "localhost:4317"
		}
	default:
		return fmt.Errorf("unsupported tracing provider: %s", c.provider)
	}

	return nil
}

// IsEnabled returns true if tracing is enabled.
func (c *Config) IsEnabled() bool {
	return c.enabled
}

// ServiceName returns the service name.
func (c *Config) ServiceName() string {
	return c.serviceName
}

// ServiceVersion returns the service version.
func (c *Config) ServiceVersion() string {
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
func (c *Config) GetProvider() Provider {
	if !c.enabled {
		return ""
	}
	return c.provider
}

// ShouldExcludePath returns true if the given path should be excluded from tracing.
// Checks exact path matches, prefixes, and regex patterns.
// Safe for concurrent access as excludePaths, excludePrefixes, and excludePatterns are read-only after Config creation.
func (c *Config) ShouldExcludePath(path string) bool {
	// Check exact path matches first (O(1) lookup)
	if c.excludePaths[path] {
		return true
	}

	// Check prefixes
	for _, prefix := range c.excludePrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	// Check regex patterns
	for _, pattern := range c.excludePatterns {
		if pattern.MatchString(path) {
			return true
		}
	}

	return false
}

// Shutdown gracefully shuts down the tracing system, flushing any pending spans.
// This should be called before the application exits to ensure all spans are exported.
// It shuts down the tracer provider if one was initialized.
// This method is idempotent - calling it multiple times is safe and will only perform shutdown once.
// All concurrent calls will wait for the same shutdown operation to complete.
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

	// Use sync.Once to ensure shutdown is performed exactly once
	// All concurrent calls will wait for the same shutdown operation
	c.shutdownOnce.Do(func() {
		// Shutdown the tracer provider if it exists and is NOT a custom provider
		// User-provided providers should be managed by the user
		if c.tracerProvider != nil && !c.customTracerProvider {
			c.emitDebug("Shutting down tracer provider")
			if err := c.tracerProvider.Shutdown(ctx); err != nil {
				c.emitError("Error shutting down tracer provider", "error", err)
				c.shutdownErr = fmt.Errorf("tracer provider shutdown: %w", err)
				return
			}
			c.emitDebug("Tracer provider shut down successfully")
		} else if c.customTracerProvider {
			c.emitDebug("Skipping shutdown of custom tracer provider (managed by user)")
		}
	})

	return c.shutdownErr
}

// emitError emits an error event if an event handler is configured.
func (c *Config) emitError(msg string, args ...any) {
	if c.eventHandler != nil {
		c.eventHandler(Event{Type: EventError, Message: msg, Args: args})
	}
}

// emitWarning emits a warning event if an event handler is configured.
func (c *Config) emitWarning(msg string, args ...any) {
	if c.eventHandler != nil {
		c.eventHandler(Event{Type: EventWarning, Message: msg, Args: args})
	}
}

// emitInfo emits an info event if an event handler is configured.
func (c *Config) emitInfo(msg string, args ...any) {
	if c.eventHandler != nil {
		c.eventHandler(Event{Type: EventInfo, Message: msg, Args: args})
	}
}

// emitDebug emits a debug event if an event handler is configured.
func (c *Config) emitDebug(msg string, args ...any) {
	if c.eventHandler != nil {
		c.eventHandler(Event{Type: EventDebug, Message: msg, Args: args})
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
//
// For cases where type is known at compile time, call OpenTelemetry functions
// directly (attribute.String(), attribute.Int(), etc.). This function is for
// convenience when the type is not known at compile time or when used in public APIs.
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

	// Check if context is already cancelled before proceeding
	select {
	case <-ctx.Done():
		c.emitDebug("Context cancelled before span creation", "path", path, "method", req.Method)
		return ctx, trace.SpanFromContext(ctx)
	default:
	}

	// Extract trace context from headers
	ctx = c.ExtractTraceContext(ctx, req.Header)

	// Sampling decision using integer arithmetic
	if c.sampleRate < 1.0 {
		if c.sampleRate == 0.0 {
			// Don't sample - return non-recording span
			c.emitDebug("Request not sampled (0% sample rate)", "path", path, "method", req.Method)
			return ctx, trace.SpanFromContext(ctx)
		}
		// Use atomic counter with multiplicative hash for deterministic sampling.
		// The counter wraps around at uint64 max, ensuring uniform distribution
		// continues even after overflow (not a practical concern).
		counter := c.samplingCounter.Add(1)
		hash := counter * samplingMultiplier
		if hash > c.samplingThreshold {
			// Don't sample this request - return non-recording span
			c.emitDebug("Request not sampled (probabilistic)",
				"path", path,
				"method", req.Method,
				"sample_rate", c.sampleRate,
				"counter", counter,
			)
			return ctx, trace.SpanFromContext(ctx)
		}
	}

	// Build span name from method and path
	var spanName string
	sb := c.spanNamePool.Get().(*strings.Builder)
	sb.Reset()
	sb.WriteString(req.Method)
	sb.WriteByte(' ')
	sb.WriteString(path)
	spanName = sb.String()
	c.spanNamePool.Put(sb)

	// Start span
	ctx, span := c.tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindServer))

	// Prepare attributes slice
	// Only parse query params if we'll actually record them
	estimatedCap := 9 + len(c.recordHeaders)
	if c.recordParams && req.URL.RawQuery != "" {
		// Estimate capacity assuming 1-2 params per query on average
		estimatedCap += 2
	}
	attrs := make([]attribute.KeyValue, 0, estimatedCap)

	// Set standard attributes
	attrs = append(attrs,
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

	// Record URL parameters if enabled - parse only when needed
	if c.recordParams && req.URL.RawQuery != "" {
		queryParams := req.URL.Query()
		for key, values := range queryParams {
			if len(values) > 0 {
				// Check if this parameter should be recorded
				if c.shouldRecordParam(key) {
					attrs = append(attrs, attribute.StringSlice(
						attrPrefixParam+key,
						values,
					))
				}
			}
		}
	}

	// Record specific headers if configured
	for _, header := range c.recordHeaders {
		if value := req.Header.Get(header); value != "" {
			attrs = append(attrs, attribute.String(
				attrPrefixHeader+strings.ToLower(header),
				value,
			))
		}
	}

	// Set all attributes on the span
	span.SetAttributes(attrs...)

	// Invoke span start hook if configured
	if c.spanStartHook != nil {
		c.spanStartHook(ctx, span, req)
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

	// Invoke span finish hook if configured (before ending span)
	if c.spanFinishHook != nil {
		c.spanFinishHook(span, statusCode)
	}

	span.End()
}

// shouldRecordParam determines if a query parameter should be recorded based on
// the whitelist (recordParamsList) and blacklist (excludeParams) configuration.
//
// Logic:
//   - If parameter is in excludeParams (blacklist), return false
//   - If recordParamsList is set (whitelist), return true only if param is in the list
//   - Otherwise, return true (default: record all params)
func (c *Config) shouldRecordParam(param string) bool {
	// Check blacklist first - highest priority
	if c.excludeParams[param] {
		return false
	}

	// If whitelist is configured, param must be in the list
	if c.recordParamsList != nil {
		for _, p := range c.recordParamsList {
			if p == param {
				return true
			}
		}
		return false
	}

	// No whitelist configured - record all params (except blacklisted)
	return true
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

// TraceContext returns the OpenTelemetry trace context.
// This can be used for manual span creation or context propagation.
// If tracing is not enabled, it returns the request context for proper cancellation support.
func TraceContext(ctx context.Context) context.Context {
	// Return the context as-is - it should already contain trace information
	return ctx
}
