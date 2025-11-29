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

package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	promclient "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// Default histogram buckets for different metric types.
// These follow OpenTelemetry semantic conventions and are suitable for most HTTP services.
var (
	// DefaultDurationBuckets are histogram boundaries for request duration in seconds.
	// Covers sub-millisecond to 10 second responses.
	DefaultDurationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

	// DefaultSizeBuckets are histogram boundaries for request/response size in bytes.
	// Covers 100B to 10MB.
	DefaultSizeBuckets = []float64{100, 1000, 10000, 100000, 1000000, 10000000}
)

// EventType represents the severity of an internal operational event.
type EventType int

const (
	// EventError indicates an error event (e.g., failed to export metrics).
	EventError EventType = iota
	// EventWarning indicates a warning event (e.g., deprecated configuration).
	EventWarning
	// EventInfo indicates an informational event (e.g., metrics server started).
	EventInfo
	// EventDebug indicates a debug event (e.g., detailed operation logs).
	EventDebug
)

// Event represents an internal operational event from the metrics package.
// Events are used to report errors, warnings, and informational messages
// about the metrics system's operation.
type Event struct {
	Type    EventType
	Message string
	Args    []any // slog-style key-value pairs
}

// EventHandler processes internal operational events from the metrics package.
// Implementations can log events, send them to monitoring systems, or take
// custom actions based on event type.
//
// Example custom handler:
//
//	metrics.WithEventHandler(func(e metrics.Event) {
//	    if e.Type == metrics.EventError {
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

// Provider represents the available metrics providers.
type Provider string

const (
	// PrometheusProvider uses Prometheus exporter for metrics (default).
	PrometheusProvider Provider = "prometheus"
	// OTLPProvider uses OTLP HTTP exporter for metrics.
	OTLPProvider Provider = "otlp"
	// StdoutProvider uses stdout exporter for metrics (development/testing).
	StdoutProvider Provider = "stdout"
)

// sensitiveHeaders contains header names that should never be recorded in metrics.
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

// Recorder holds OpenTelemetry metrics configuration and runtime state.
// All methods are safe for concurrent use.
//
// By default, this package does NOT set the global OpenTelemetry meter provider.
// Use WithGlobalMeterProvider() if you want global registration.
// This allows multiple Recorder instances to coexist in the same process.
type Recorder struct {
	meter              metric.Meter
	meterProvider      metric.MeterProvider
	prometheusHandler  http.Handler
	prometheusRegistry *promclient.Registry // Custom Prometheus registry to avoid conflicts
	metricsServer      *http.Server
	eventHandler       EventHandler // Handler for internal operational events

	// Built-in HTTP metrics
	requestDuration      metric.Float64Histogram
	requestCount         metric.Int64Counter
	activeRequests       metric.Int64UpDownCounter
	requestSize          metric.Int64Histogram
	responseSize         metric.Int64Histogram
	errorCount           metric.Int64Counter
	customMetricFailures metric.Int64Counter

	// Custom metrics storage (protected by RWMutex)
	customMu          sync.RWMutex
	customCounters    map[string]metric.Int64Counter
	customHistograms  map[string]metric.Float64Histogram
	customGauges      map[string]metric.Float64Gauge
	customMetricCount int

	// Histogram bucket configuration
	durationBuckets []float64 // Custom buckets for request duration histogram
	sizeBuckets     []float64 // Custom buckets for request/response size histograms

	validationErrors []error // Collected during option application

	exportInterval time.Duration

	// Atomic counter for tracking custom metric failures (used for testing/monitoring)
	atomicCustomMetricFailures int64

	serviceName    string
	serviceVersion string
	otlpEndpoint   string // OTLP collector endpoint
	metricsPort    string
	metricsPath    string

	// Pre-computed common attributes computed during initialization
	serviceNameAttr    attribute.KeyValue
	serviceVersionAttr attribute.KeyValue

	serverMutex sync.Mutex // Protects metricsServer access

	maxCustomMetrics int // Maximum number of custom metrics

	provider            Provider
	providerSetCount    int         // Tracks how many times a provider option was called
	isShuttingDown      atomic.Bool // Prevents server restart during shutdown
	enabled             bool
	autoStartServer     bool
	strictPort          bool // If true, fail instead of finding alternative port
	customMeterProvider bool // If true, user provided their own meter provider
	registerGlobal      bool // If true, sets otel.SetMeterProvider()
}

// Option defines functional options for Recorder configuration.
type Option func(*Recorder)

// WithMeterProvider allows you to provide a custom OpenTelemetry MeterProvider.
// When using this option, the package will NOT set the global otel.SetMeterProvider()
// by default. Use WithGlobalMeterProvider() if you want global registration.
//
// This is useful when:
//   - You want to manage the meter provider lifecycle yourself
//   - You need multiple independent metrics configurations
//   - You want to avoid global state in your application
//
// Example:
//
//	mp := sdkmetric.NewMeterProvider(...)
//	recorder := metrics.New(
//	    metrics.WithMeterProvider(mp),
//	    metrics.WithServiceName("my-service"),
//	)
//	defer mp.Shutdown(context.Background())
//
// Note: When using WithMeterProvider, provider options (PrometheusProvider, OTLPProvider, etc.)
// are ignored since you're managing the provider yourself.
func WithMeterProvider(provider metric.MeterProvider) Option {
	return func(r *Recorder) {
		r.meterProvider = provider
		r.customMeterProvider = true
		// Note: registerGlobal stays false unless explicitly set
	}
}

// WithGlobalMeterProvider registers the meter provider as the global
// OpenTelemetry meter provider via otel.SetMeterProvider().
// By default, meter providers are not registered globally to allow multiple
// metrics configurations to coexist in the same process.
//
// Example:
//
//	recorder := metrics.New(
//	    metrics.WithPrometheus(":9090", "/metrics"),
//	    metrics.WithGlobalMeterProvider(), // Register as global default
//	)
func WithGlobalMeterProvider() Option {
	return func(r *Recorder) {
		r.registerGlobal = true
	}
}

// WithServiceName sets the service name for metrics.
func WithServiceName(name string) Option {
	return func(r *Recorder) {
		r.serviceName = name
	}
}

// WithServiceVersion sets the service version for metrics.
func WithServiceVersion(version string) Option {
	return func(r *Recorder) {
		r.serviceVersion = version
	}
}

// WithExportInterval sets the export interval for OTLP and stdout metrics.
func WithExportInterval(interval time.Duration) Option {
	return func(r *Recorder) {
		r.exportInterval = interval
	}
}

// WithDurationBuckets sets custom histogram bucket boundaries for request duration metrics.
// Buckets are specified in seconds. If not set, DefaultDurationBuckets is used.
//
// Example:
//
//	recorder := metrics.MustNew(
//	    metrics.WithDurationBuckets(0.01, 0.05, 0.1, 0.5, 1, 5), // in seconds
//	)
func WithDurationBuckets(buckets ...float64) Option {
	return func(r *Recorder) {
		r.durationBuckets = buckets
	}
}

// WithSizeBuckets sets custom histogram bucket boundaries for request/response size metrics.
// Buckets are specified in bytes. If not set, DefaultSizeBuckets is used.
//
// Example:
//
//	recorder := metrics.MustNew(
//	    metrics.WithSizeBuckets(1000, 10000, 100000, 1000000), // in bytes
//	)
func WithSizeBuckets(buckets ...float64) Option {
	return func(r *Recorder) {
		r.sizeBuckets = buckets
	}
}

// WithServerDisabled disables the automatic metrics server for Prometheus.
// Use this if you want to manually serve metrics via Handler().
func WithServerDisabled() Option {
	return func(r *Recorder) {
		r.autoStartServer = false
	}
}

// WithStrictPort requires the metrics server to use the exact port specified.
// If the port is unavailable, initialization will fail instead of finding an alternative port.
// This is useful when you need metrics on a specific port for monitoring integrations.
func WithStrictPort() Option {
	return func(r *Recorder) {
		r.strictPort = true
	}
}

// WithMaxCustomMetrics sets the maximum number of custom metrics allowed.
func WithMaxCustomMetrics(maxLimit int) Option {
	return func(r *Recorder) {
		r.maxCustomMetrics = maxLimit
	}
}

// WithEventHandler sets a custom event handler for internal operational events.
// Use this for advanced use cases like sending errors to Sentry, custom alerting,
// or integrating with non-slog logging systems.
//
// Example:
//
//	metrics.New(metrics.WithEventHandler(func(e metrics.Event) {
//	    if e.Type == metrics.EventError {
//	        sentry.CaptureMessage(e.Message)
//	    }
//	    myLogger.Log(e.Type, e.Message, e.Args...)
//	}))
func WithEventHandler(handler EventHandler) Option {
	return func(r *Recorder) {
		r.eventHandler = handler
	}
}

// WithLogger sets the logger for internal operational events using the default event handler.
// This is a convenience wrapper around WithEventHandler that logs events to the provided slog.Logger.
//
// Example:
//
//	// Use stdlib slog
//	metrics.New(metrics.WithLogger(slog.Default()))
//
//	// Use custom slog logger
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	metrics.New(metrics.WithLogger(logger))
func WithLogger(logger *slog.Logger) Option {
	return WithEventHandler(DefaultEventHandler(logger))
}

// New creates a new Recorder with the given options.
// Returns an error if the metrics provider fails to initialize.
// For a version that panics on error, use MustNew.
//
// By default, this function does NOT set the global OpenTelemetry meter provider.
// Use WithGlobalMeterProvider() if you want to register the meter provider as the global default.
//
// This allows multiple metrics configurations to coexist in the same process,
// and makes it easier to integrate Rivaas into larger binaries that already
// manage their own global meter provider.
func New(opts ...Option) (*Recorder, error) {
	recorder := newDefaultRecorder()

	// Apply options
	for _, opt := range opts {
		opt(recorder)
	}

	// Validate configuration
	if err := recorder.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Initialize the provider
	if err := recorder.initializeProvider(); err != nil {
		return nil, fmt.Errorf("failed to initialize metrics: %w", err)
	}

	return recorder, nil
}

// newDefaultRecorder creates a new Recorder with default values.
func newDefaultRecorder() *Recorder {
	recorder := &Recorder{
		enabled:          true,
		serviceName:      "rivaas-service",
		serviceVersion:   "1.0.0",
		provider:         PrometheusProvider,
		exportInterval:   30 * time.Second,
		metricsPort:      ":9090",
		metricsPath:      "/metrics",
		autoStartServer:  true,
		maxCustomMetrics: 1000,  // Limit to prevent unbounded metric creation
		registerGlobal:   false, // Default: no global registration
		durationBuckets:  DefaultDurationBuckets,
		sizeBuckets:      DefaultSizeBuckets,
		customCounters:   make(map[string]metric.Int64Counter),
		customHistograms: make(map[string]metric.Float64Histogram),
		customGauges:     make(map[string]metric.Float64Gauge),
	}

	recorder.initCommonAttributes()
	return recorder
}

// initCommonAttributes pre-computes common attributes.
// These attributes are used frequently in request metrics.
func (r *Recorder) initCommonAttributes() {
	r.serviceNameAttr = attribute.String("service.name", r.serviceName)
	r.serviceVersionAttr = attribute.String("service.version", r.serviceVersion)
}

// validate checks that the configuration is valid.
func (r *Recorder) validate() error {
	// Check for errors collected during option application
	if len(r.validationErrors) > 0 {
		return fmt.Errorf("configuration errors: %v", r.validationErrors)
	}

	// Check for conflicting provider options
	if r.providerSetCount > 1 {
		return fmt.Errorf("conflicting provider options: only one of WithPrometheus, WithOTLP, or WithStdout can be used")
	}

	// Validate service name
	if r.serviceName == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	// Validate service version
	if r.serviceVersion == "" {
		return fmt.Errorf("service version cannot be empty")
	}

	// Validate max custom metrics
	if r.maxCustomMetrics < 1 {
		return fmt.Errorf("maxCustomMetrics must be at least 1, got %d", r.maxCustomMetrics)
	}

	// Validate export interval
	if r.exportInterval < time.Second {
		r.emitWarning("Export interval is very low, may cause high CPU usage", "interval", r.exportInterval)
	}

	// Validate provider-specific settings
	switch r.provider {
	case PrometheusProvider:
		if r.metricsPort == "" {
			return fmt.Errorf("metrics port cannot be empty for Prometheus provider")
		}
		if r.metricsPath == "" {
			return fmt.Errorf("metrics path cannot be empty for Prometheus provider")
		}
	case OTLPProvider:
		if r.otlpEndpoint == "" {
			r.emitWarning("OTLP endpoint not specified, will use default", "default", "http://localhost:4318")
			r.otlpEndpoint = "http://localhost:4318"
		}
	case StdoutProvider:
		// No specific validation needed for stdout
	default:
		return fmt.Errorf("unsupported metrics provider: %s", r.provider)
	}

	return nil
}

// MustNew creates a new Recorder with the given options.
// It panics if the metrics provider fails to initialize.
// Use this for convenience when you want to panic on initialization errors.
func MustNew(opts ...Option) *Recorder {
	recorder, err := New(opts...)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize metrics: %v", err))
	}
	return recorder
}

// Handler returns the Prometheus metrics HTTP handler.
// This is useful when you want to serve metrics manually or disable the auto-server.
// Returns an error if metrics are not enabled or if not using Prometheus provider.
//
// Example:
//
//	handler, err := recorder.Handler()
//	if err == nil {
//	    http.Handle("/metrics", handler)
//	}
func (r *Recorder) Handler() (http.Handler, error) {
	if !r.enabled {
		return nil, fmt.Errorf("metrics not enabled")
	}

	if r.provider != PrometheusProvider || r.prometheusHandler == nil {
		return nil, fmt.Errorf("handler only available with Prometheus provider, current provider: %s", r.provider)
	}

	return r.prometheusHandler, nil
}

// Provider returns the current metrics provider.
func (r *Recorder) Provider() Provider {
	if !r.enabled {
		return ""
	}
	return r.provider
}

// ServerAddress returns the address of the metrics server.
// Returns empty string if not using Prometheus or server is disabled.
func (r *Recorder) ServerAddress() string {
	if !r.enabled || r.provider != PrometheusProvider || !r.autoStartServer {
		return ""
	}
	return r.metricsPort
}

// Path returns the path for the Prometheus metrics endpoint.
// Returns empty string if not using Prometheus provider.
func (r *Recorder) Path() string {
	if !r.enabled || r.provider != PrometheusProvider {
		return ""
	}
	return r.metricsPath
}

// Shutdown gracefully shuts down the metrics system, flushing any pending metrics.
// This should be called before the application exits to ensure all metrics are exported.
// It stops the metrics server (if running) and shuts down the meter provider.
// This method is idempotent - calling it multiple times is safe and will only perform shutdown once.
func (r *Recorder) Shutdown(ctx context.Context) error {
	if !r.enabled {
		return nil
	}

	// Use CompareAndSwap to ensure only one goroutine performs shutdown
	// If already shutting down or shut down, return immediately
	if !r.isShuttingDown.CompareAndSwap(false, true) {
		return nil // Already shutting down or shut down
	}

	var errs []error

	// Stop the metrics server first with context
	if err := r.stopMetricsServer(ctx); err != nil {
		errs = append(errs, err)
	}

	// Shutdown the meter provider if it supports it and is NOT a custom provider
	// User-provided providers should be managed by the user
	if !r.customMeterProvider {
		if mp, ok := r.meterProvider.(*sdkmetric.MeterProvider); ok {
			r.emitDebug("Shutting down meter provider")
			if err := mp.Shutdown(ctx); err != nil {
				errs = append(errs, fmt.Errorf("meter provider shutdown: %w", err))
			}
		}
	} else {
		r.emitDebug("Skipping shutdown of custom meter provider (managed by user)")
	}

	// Return combined errors if any
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}

// IsEnabled returns true if metrics are enabled.
func (r *Recorder) IsEnabled() bool {
	return r.enabled
}

// ServiceName returns the service name.
func (r *Recorder) ServiceName() string {
	return r.serviceName
}

// ServiceVersion returns the service version.
func (r *Recorder) ServiceVersion() string {
	return r.serviceVersion
}

// WithPrometheus configures Prometheus provider with port and path.
// This is the recommended way to configure Prometheus metrics.
//
// Example:
//
//	recorder := metrics.MustNew(
//	    metrics.WithPrometheus(":9090", "/metrics"),
//	    metrics.WithServiceName("my-api"),
//	)
func WithPrometheus(port, path string) Option {
	return func(r *Recorder) {
		r.provider = PrometheusProvider
		r.providerSetCount++
		// Normalize and set port
		if port != "" && !strings.HasPrefix(port, ":") {
			port = ":" + port
		}
		r.metricsPort = port
		// Normalize and set path
		if path != "" && !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		r.metricsPath = path
	}
}

// WithOTLP configures OTLP HTTP provider with endpoint.
//
// Example:
//
//	recorder := metrics.MustNew(
//	    metrics.WithOTLP("http://localhost:4318"),
//	    metrics.WithServiceName("my-api"),
//	)
func WithOTLP(endpoint string) Option {
	return func(r *Recorder) {
		r.provider = OTLPProvider
		r.providerSetCount++
		r.otlpEndpoint = endpoint
	}
}

// WithStdout configures stdout provider for development/debugging.
//
// Example:
//
//	recorder := metrics.MustNew(
//	    metrics.WithStdout(),
//	    metrics.WithExportInterval(time.Second),
//	)
func WithStdout() Option {
	return func(r *Recorder) {
		r.provider = StdoutProvider
		r.providerSetCount++
	}
}

// emitError emits an error event if an event handler is configured.
func (r *Recorder) emitError(msg string, args ...any) {
	if r.eventHandler != nil {
		r.eventHandler(Event{Type: EventError, Message: msg, Args: args})
	}
}

// emitWarning emits a warning event if an event handler is configured.
func (r *Recorder) emitWarning(msg string, args ...any) {
	if r.eventHandler != nil {
		r.eventHandler(Event{Type: EventWarning, Message: msg, Args: args})
	}
}

// emitInfo emits an info event if an event handler is configured.
func (r *Recorder) emitInfo(msg string, args ...any) {
	if r.eventHandler != nil {
		r.eventHandler(Event{Type: EventInfo, Message: msg, Args: args})
	}
}

// emitDebug emits a debug event if an event handler is configured.
func (r *Recorder) emitDebug(msg string, args ...any) {
	if r.eventHandler != nil {
		r.eventHandler(Event{Type: EventDebug, Message: msg, Args: args})
	}
}
