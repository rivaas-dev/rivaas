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
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	promclient "github.com/prometheus/client_golang/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// Note: Option type and functional options are defined in options.go

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
	isStarted           atomic.Bool // Tracks if Start() has been called
	providerDeferred    atomic.Bool // If true, provider initialization is deferred to Start()
	warnNotStarted      sync.Once   // Warn once if BeginRequest called before Start()
	enabled             bool
	autoStartServer     bool
	strictPort          bool // If true, fail instead of finding alternative port
	customMeterProvider bool // If true, user provided their own meter provider
	registerGlobal      bool // If true, sets otel.SetMeterProvider()
}

// New creates a new [Recorder] with the given options.
// Returns an error if the metrics provider fails to initialize.
// For a version that panics on error, use [MustNew].
//
// By default, this function does NOT set the global OpenTelemetry meter provider.
// Use [WithGlobalMeterProvider] if you want to register the meter provider as the global default.
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

// MustNew creates a new [Recorder] with the given options.
// It panics if the metrics provider fails to initialize.
// Use this for convenience when you want to panic on initialization errors.
// For error handling, use [New] instead.
func MustNew(opts ...Option) *Recorder {
	recorder, err := New(opts...)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize metrics: %v", err))
	}

	return recorder
}

// Handler returns the Prometheus metrics [http.Handler].
// This is useful when you want to serve metrics manually or disable the auto-server
// using [WithServerDisabled].
// Returns an error if metrics are not enabled or if not using [PrometheusProvider].
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
// Returns empty string if not using [PrometheusProvider] or server is disabled.
func (r *Recorder) ServerAddress() string {
	if !r.enabled || r.provider != PrometheusProvider || !r.autoStartServer {
		return ""
	}

	return r.metricsPort
}

// Path returns the path for the Prometheus metrics endpoint.
// Returns empty string if not using [PrometheusProvider].
func (r *Recorder) Path() string {
	if !r.enabled || r.provider != PrometheusProvider {
		return ""
	}

	return r.metricsPath
}

// Start starts the metrics server if auto-start is enabled.
// The context is used for the server's lifecycle - when cancelled, it signals shutdown.
// This method is idempotent; calling it multiple times is safe.
//
// For Prometheus provider with auto-start enabled, this starts the HTTP server
// that exposes the /metrics endpoint.
//
// Example:
//
//	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
//	defer cancel()
//
//	recorder, _ := metrics.New(metrics.WithPrometheus(":9090", "/metrics"))
//	if err := recorder.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
func (r *Recorder) Start(ctx context.Context) error {
	if !r.enabled {
		return nil
	}

	// Idempotent: only start once
	if !r.isStarted.CompareAndSwap(false, true) {
		return nil // Already started
	}

	// Initialize deferred providers (OTLP) with lifecycle context for proper shutdown
	if r.providerDeferred.Load() {
		if err := r.initOTLPProvider(ctx); err != nil {
			r.isStarted.Store(false) // Reset on failure to allow retry
			return fmt.Errorf("failed to initialize OTLP provider: %w", err)
		}
		r.providerDeferred.Store(false) // Initialization complete
	}

	// Start the metrics server if auto-start is enabled and using Prometheus
	if r.autoStartServer && r.provider == PrometheusProvider {
		r.startMetricsServer(ctx)
	}

	return nil
}

// Shutdown gracefully shuts down the metrics system, flushing any pending metrics.
// This should be called before the application exits to ensure all metrics are exported.
// It stops the metrics server (if running) and shuts down the meter provider.
// This method is idempotent; calling it multiple times is safe and will only perform shutdown once.
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

	// Flush and shutdown the meter provider if it supports it and is NOT a custom provider
	// User-provided providers should be managed by the user
	if r.customMeterProvider {
		r.emitDebug("Skipping flush and shutdown of custom meter provider (managed by user)")
	} else if err := r.shutdownSDKMeterProvider(ctx); err != nil {
		errs = append(errs, err)
	}

	// Return combined errors if any
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}

// shutdownSDKMeterProvider flushes and shuts down the SDK meter provider.
// Returns an error only if shutdown fails; flush failures are logged as warnings.
func (r *Recorder) shutdownSDKMeterProvider(ctx context.Context) error {
	mp, ok := r.meterProvider.(*sdkmetric.MeterProvider)
	if !ok {
		return nil
	}

	// Explicitly flush pending metrics before shutdown
	// This is especially important for push-based providers (OTLP, stdout)
	// to ensure all buffered data is exported before the provider is closed
	r.emitDebug("Flushing pending metrics")
	if err := mp.ForceFlush(ctx); err != nil {
		// Log warning but continue with shutdown - flush failure shouldn't block shutdown
		r.emitWarning("metrics flush warning", "error", err)
	} else {
		r.emitDebug("Metrics flushed successfully")
	}

	r.emitDebug("Shutting down meter provider")
	if err := mp.Shutdown(ctx); err != nil {
		return fmt.Errorf("meter provider shutdown: %w", err)
	}

	r.emitDebug("Meter provider shut down successfully")

	return nil
}

// ForceFlush immediately exports any pending metric data.
// This is useful for push-based providers (OTLP, stdout) when you want to ensure
// metrics are exported without shutting down the recorder (e.g., before a deployment,
// at checkpoints, or during long-running operations).
// For pull-based providers (Prometheus), this is typically a no-op as metrics are
// collected on-demand when scraped.
// Returns an error if the flush fails or if the recorder is disabled.
func (r *Recorder) ForceFlush(ctx context.Context) error {
	if !r.enabled {
		return nil
	}

	// Don't flush if already shutting down
	if r.isShuttingDown.Load() {
		return nil
	}

	if mp, ok := r.meterProvider.(*sdkmetric.MeterProvider); ok {
		r.emitDebug("Force flushing metrics")
		if err := mp.ForceFlush(ctx); err != nil {
			return fmt.Errorf("metrics force flush: %w", err)
		}
		r.emitDebug("Metrics force flushed successfully")
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
