// Package metrics provides comprehensive OpenTelemetry-based metrics collection
// for Go applications. It supports multiple exporters (Prometheus, OTLP, stdout)
// and integrates seamlessly with the Rivaas router.
//
// # Basic Usage
//
//	config := metrics.MustNew(
//	    metrics.WithServiceName("my-service"),
//	    metrics.WithProvider(metrics.PrometheusProvider),
//	)
//	defer config.Shutdown(context.Background())
//
//	// Use with router
//	r := router.New()
//	r.SetMetricsRecorder(config)
//
// # Thread Safety
//
// All methods are thread-safe. Custom metric creation uses lock-free atomic operations
// for performance. Custom metrics are limited (default 1000) to prevent memory leaks
// from unbounded metric creation.
//
// # Global State Warning
//
// This package sets the global OpenTelemetry meter provider via otel.SetMeterProvider().
// Only one metrics configuration should be active per process. Creating multiple
// configurations will cause them to overwrite each other's global meter provider.
//
// # Providers
//
// Three providers are supported:
//   - PrometheusProvider (default): Exposes metrics via HTTP endpoint
//   - OTLPProvider: Sends metrics to OTLP collector
//   - StdoutProvider: Prints metrics to stdout (for development/testing)
//
// # Custom Metrics
//
// Record custom metrics using the provided methods:
//
//	config.RecordMetric(ctx, "processing_duration", 1.5,
//	    attribute.String("operation", "create_user"))
//	config.IncrementCounter(ctx, "requests_total",
//	    attribute.String("status", "success"))
//	config.SetGauge(ctx, "active_connections", 42)
//
// # Environment Variables
//
// The package reads configuration from standard OpenTelemetry environment variables:
//   - OTEL_METRICS_EXPORTER: Provider (prometheus, otlp, stdout)
//   - OTEL_EXPORTER_OTLP_METRICS_ENDPOINT: OTLP endpoint
//   - OTEL_SERVICE_NAME: Service name
//   - OTEL_SERVICE_VERSION: Service version
//   - RIVAAS_METRICS_PORT: Prometheus port (default :9090)
//   - RIVAAS_METRICS_PATH: Prometheus path (default /metrics)
package metrics

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	promclient "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"rivaas.dev/logging"
)

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

// Config holds OpenTelemetry metrics configuration.
// Fields are ordered by size (largest to smallest) for optimal memory alignment.
type Config struct {
	// Pointers and interfaces (8 bytes on 64-bit)
	meter              metric.Meter
	meterProvider      metric.MeterProvider
	prometheusHandler  http.Handler
	prometheusRegistry *promclient.Registry // Custom Prometheus registry to avoid conflicts
	metricsServer      *http.Server
	logger             logging.Logger // Structured logger for errors and warnings

	// Built-in HTTP metrics (interfaces, 8 bytes each)
	requestDuration      metric.Float64Histogram
	requestCount         metric.Int64Counter
	activeRequests       metric.Int64UpDownCounter
	requestSize          metric.Int64Histogram
	responseSize         metric.Int64Histogram
	routeCount           metric.Int64Counter
	errorCount           metric.Int64Counter
	constraintFailures   metric.Int64Counter
	contextPoolHits      metric.Int64Counter
	contextPoolMisses    metric.Int64Counter
	customMetricFailures metric.Int64Counter
	casRetriesCounter    metric.Int64Counter

	// Maps and slices
	excludePaths       map[string]bool
	recordHeaders      []string
	recordHeadersLower []string // Pre-lowercased header names for performance

	// Atomic custom metrics cache - lock-free operations (unsafe.Pointer = 8 bytes)
	atomicCustomCounters   unsafe.Pointer // *map[string]metric.Int64Counter
	atomicCustomHistograms unsafe.Pointer // *map[string]metric.Float64Histogram
	atomicCustomGauges     unsafe.Pointer // *map[string]metric.Float64Gauge

	// int64 and time.Duration (8 bytes)
	exportInterval             time.Duration
	atomicCustomMetricsCount   int64 // Atomic counter for total custom metrics
	atomicRequestCount         int64
	atomicActiveRequests       int64
	atomicErrorCount           int64
	atomicContextPoolHits      int64
	atomicContextPoolMisses    int64
	atomicCustomMetricFailures int64
	atomicCASRetries           int64 // Tracks CAS retry attempts for contention monitoring

	// Strings (16 bytes: pointer + length)
	serviceName    string
	serviceVersion string
	endpoint       string
	metricsPort    string
	metricsPath    string

	// Pre-computed common attributes (performance optimization)
	// These are computed once during initialization to avoid repeated allocations
	serviceNameAttr    attribute.KeyValue
	serviceVersionAttr attribute.KeyValue
	staticRouteAttr    attribute.KeyValue
	dynamicRouteAttr   attribute.KeyValue

	// Mutex (typically 8-16 bytes depending on implementation)
	serverMutex sync.Mutex // Protects metricsServer access

	// int (4 or 8 bytes depending on platform)
	maxCustomMetrics int // Maximum number of custom metrics

	// Small types (1-2 bytes, grouped together to minimize padding)
	provider            Provider
	isShuttingDown      atomic.Bool // Prevents server restart during shutdown
	enabled             bool
	recordParams        bool
	autoStartServer     bool
	strictPort          bool // If true, fail instead of finding alternative port
	customMeterProvider bool // If true, user provided their own meter provider
	registerGlobal      bool // If true, sets otel.SetMeterProvider()
}

// Option defines functional options for metrics configuration.
type Option func(*Config)

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
//	config := metrics.New(
//	    metrics.WithMeterProvider(mp),
//	    metrics.WithServiceName("my-service"),
//	)
//	defer mp.Shutdown(context.Background())
//
// Note: When using WithMeterProvider, provider options (PrometheusProvider, OTLPProvider, etc.)
// are ignored since you're managing the provider yourself.
func WithMeterProvider(provider metric.MeterProvider) Option {
	return func(c *Config) {
		c.meterProvider = provider
		c.customMeterProvider = true
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
//	config := metrics.New(
//	    metrics.WithProvider(metrics.PrometheusProvider),
//	    metrics.WithGlobalMeterProvider(), // Register as global default
//	)
func WithGlobalMeterProvider() Option {
	return func(c *Config) {
		c.registerGlobal = true
	}
}

// WithServiceName sets the service name for metrics.
func WithServiceName(name string) Option {
	return func(c *Config) {
		c.serviceName = name
	}
}

// WithServiceVersion sets the service version for metrics.
func WithServiceVersion(version string) Option {
	return func(c *Config) {
		c.serviceVersion = version
	}
}

// WithProvider sets the metrics provider.
func WithProvider(provider Provider) Option {
	return func(c *Config) {
		c.provider = provider
	}
}

// WithOTLPEndpoint sets the endpoint for OTLP metrics.
// Only used when provider is OTLPProvider.
//
// Example:
//
//	config := metrics.New(
//	    metrics.WithProvider(metrics.OTLPProvider),
//	    metrics.WithOTLPEndpoint("localhost:4318"),
//	)
func WithOTLPEndpoint(endpoint string) Option {
	return func(c *Config) {
		c.endpoint = endpoint
	}
}

// WithExportInterval sets the export interval for OTLP and stdout metrics.
func WithExportInterval(interval time.Duration) Option {
	return func(c *Config) {
		c.exportInterval = interval
	}
}

// WithExcludePaths excludes specific paths from metrics collection.
func WithExcludePaths(paths ...string) Option {
	return func(c *Config) {
		for _, path := range paths {
			c.excludePaths[path] = true
		}
	}
}

// WithHeaders records specific headers as metric attributes.
// Headers are normalized to lowercase for efficient lookup.
func WithHeaders(headers ...string) Option {
	return func(c *Config) {
		c.recordHeaders = headers
		// Pre-compute lowercased header names for performance
		c.recordHeadersLower = make([]string, len(headers))
		for i, h := range headers {
			c.recordHeadersLower[i] = strings.ToLower(h)
		}
	}
}

// WithDisableParams disables recording URL parameters in metrics.
func WithDisableParams() Option {
	return func(c *Config) {
		c.recordParams = false
	}
}

// WithPort sets the port for the Prometheus metrics server.
// Default is ":9090". Only affects Prometheus provider.
func WithPort(port string) Option {
	return func(c *Config) {
		c.metricsPort = port
	}
}

// WithPath sets the path for the Prometheus metrics endpoint.
// Default is "/metrics". Only affects Prometheus provider.
func WithPath(path string) Option {
	return func(c *Config) {
		c.metricsPath = path
	}
}

// WithServerDisabled disables the automatic metrics server for Prometheus.
// Use this if you want to manually serve metrics via GetHandler().
func WithServerDisabled() Option {
	return func(c *Config) {
		c.autoStartServer = false
	}
}

// WithStrictPort requires the metrics server to use the exact port specified.
// If the port is unavailable, initialization will fail instead of finding an alternative port.
// This is useful when you need metrics on a specific port for monitoring integrations.
func WithStrictPort() Option {
	return func(c *Config) {
		c.strictPort = true
	}
}

// WithMaxCustomMetrics sets the maximum number of custom metrics allowed.
func WithMaxCustomMetrics(maxLimit int) Option {
	return func(c *Config) {
		c.maxCustomMetrics = maxLimit
	}
}

// WithLogger sets a custom logger for metrics errors and warnings.
func WithLogger(logger logging.Logger) Option {
	return func(c *Config) {
		c.logger = logger
	}
}

// New creates a new metrics configuration with the given options.
// Returns an error if the metrics provider fails to initialize.
// For a version that panics on error, use MustNew.
//
// By default, this function does NOT set the global OpenTelemetry meter provider.
// Use WithGlobalMeterProvider() if you want to register the meter provider as the global default.
//
// This allows multiple metrics configurations to coexist in the same process,
// and makes it easier to integrate Rivaas into larger binaries that already
// manage their own global meter provider.
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
		return nil, fmt.Errorf("failed to initialize metrics: %w", err)
	}

	return config, nil
}

// newDefaultConfig creates a new metrics configuration with default values.
func newDefaultConfig() *Config {
	config := &Config{
		enabled:          true,
		serviceName:      "rivaas-service",
		serviceVersion:   "1.0.0",
		excludePaths:     make(map[string]bool),
		recordParams:     true,
		provider:         PrometheusProvider,
		exportInterval:   30 * time.Second,
		metricsPort:      ":9090",
		metricsPath:      "/metrics",
		autoStartServer:  true,
		maxCustomMetrics: 1000,  // Limit to prevent memory leaks
		registerGlobal:   false, // Default: no global registration
	}

	config.initAtomicMaps()
	config.initCommonAttributes()
	return config
}

// initAtomicMaps initializes the atomic custom metrics maps.
func (c *Config) initAtomicMaps() {
	initialCounters := make(map[string]metric.Int64Counter)
	initialHistograms := make(map[string]metric.Float64Histogram)
	initialGauges := make(map[string]metric.Float64Gauge)

	atomic.StorePointer(&c.atomicCustomCounters, unsafe.Pointer(&initialCounters))
	atomic.StorePointer(&c.atomicCustomHistograms, unsafe.Pointer(&initialHistograms))
	atomic.StorePointer(&c.atomicCustomGauges, unsafe.Pointer(&initialGauges))
}

// initCommonAttributes pre-computes common attributes to avoid repeated allocations.
// These attributes are used frequently in request metrics and should be created once.
func (c *Config) initCommonAttributes() {
	c.serviceNameAttr = attribute.String("service.name", c.serviceName)
	c.serviceVersionAttr = attribute.String("service.version", c.serviceVersion)
	c.staticRouteAttr = attribute.Bool("rivaas.router.static_route", true)
	c.dynamicRouteAttr = attribute.Bool("rivaas.router.static_route", false)
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

	// Validate max custom metrics
	if c.maxCustomMetrics < 1 {
		return fmt.Errorf("maxCustomMetrics must be at least 1, got %d", c.maxCustomMetrics)
	}

	// Validate export interval
	if c.exportInterval < time.Second {
		c.logWarn("Export interval is very low, may cause high CPU usage", "interval", c.exportInterval)
	}

	// Validate provider-specific settings
	switch c.provider {
	case PrometheusProvider:
		if c.metricsPort == "" {
			return fmt.Errorf("metrics port cannot be empty for Prometheus provider")
		}
		if c.metricsPath == "" {
			return fmt.Errorf("metrics path cannot be empty for Prometheus provider")
		}
	case OTLPProvider:
		if c.endpoint == "" {
			c.logWarn("OTLP endpoint not specified, will use default", "default", "http://localhost:4318")
			c.endpoint = "http://localhost:4318"
		}
	case StdoutProvider:
		// No specific validation needed for stdout
	default:
		return fmt.Errorf("unsupported metrics provider: %s", c.provider)
	}

	return nil
}

// MustNew creates a new metrics configuration with the given options.
// It panics if the metrics provider fails to initialize.
// Use this for convenience when you want to fail fast on initialization errors.
func MustNew(opts ...Option) *Config {
	config, err := New(opts...)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize metrics: %v", err))
	}
	return config
}

// readFromEnv reads configuration from environment variables.
func (c *Config) readFromEnv() {
	// OTEL_METRICS_EXPORTER
	if exporter := os.Getenv("OTEL_METRICS_EXPORTER"); exporter != "" {
		switch strings.ToLower(exporter) {
		case "prometheus":
			c.provider = PrometheusProvider
		case "otlp":
			c.provider = OTLPProvider
		case "stdout":
			c.provider = StdoutProvider
		}
	}

	// OTEL_EXPORTER_OTLP_METRICS_ENDPOINT or OTEL_EXPORTER_OTLP_ENDPOINT
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"); endpoint != "" {
		c.endpoint = endpoint
	} else if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		c.endpoint = endpoint
	}

	// OTEL_SERVICE_NAME
	if serviceName := os.Getenv("OTEL_SERVICE_NAME"); serviceName != "" {
		c.serviceName = serviceName
	}

	// OTEL_SERVICE_VERSION
	if serviceVersion := os.Getenv("OTEL_SERVICE_VERSION"); serviceVersion != "" {
		c.serviceVersion = serviceVersion
	}

	// RIVAAS_METRICS_PORT (custom env var for metrics port)
	if port := os.Getenv("RIVAAS_METRICS_PORT"); port != "" {
		if !strings.HasPrefix(port, ":") {
			port = ":" + port
		}
		c.metricsPort = port
	}

	// RIVAAS_METRICS_PATH (custom env var for metrics path)
	if path := os.Getenv("RIVAAS_METRICS_PATH"); path != "" {
		c.metricsPath = path
	}
}

// GetHandler returns the Prometheus metrics HTTP handler.
// This is useful when you want to serve metrics manually or disable the auto-server.
// Returns an error if metrics are not enabled or if not using Prometheus provider.
func (c *Config) GetHandler() (http.Handler, error) {
	if !c.enabled {
		return nil, fmt.Errorf("metrics not enabled")
	}

	if c.provider != PrometheusProvider || c.prometheusHandler == nil {
		return nil, fmt.Errorf("handler only available with Prometheus provider, current provider: %s", c.provider)
	}

	return c.prometheusHandler, nil
}

// GetProvider returns the current metrics provider.
func (c *Config) GetProvider() Provider {
	if !c.enabled {
		return ""
	}
	return c.provider
}

// GetServerAddress returns the address of the metrics server.
// Returns empty string if not using Prometheus or server is disabled.
func (c *Config) GetServerAddress() string {
	if !c.enabled || c.provider != PrometheusProvider || !c.autoStartServer {
		return ""
	}
	return c.metricsPort
}

// Path returns the path for the Prometheus metrics endpoint.
// Returns empty string if not using Prometheus provider.
func (c *Config) Path() string {
	if !c.enabled || c.provider != PrometheusProvider {
		return ""
	}
	return c.metricsPath
}

// Shutdown gracefully shuts down the metrics system, flushing any pending metrics.
// This should be called before the application exits to ensure all metrics are exported.
// It stops the metrics server (if running) and shuts down the meter provider.
// This method is idempotent - calling it multiple times is safe and will only perform shutdown once.
func (c *Config) Shutdown(ctx context.Context) error {
	if !c.enabled {
		return nil
	}

	// Use CompareAndSwap to ensure only one goroutine performs shutdown
	// If already shutting down or shut down, return immediately
	if !c.isShuttingDown.CompareAndSwap(false, true) {
		return nil // Already shutting down or shut down
	}

	var errs []error

	// Stop the metrics server first with context
	if err := c.stopMetricsServer(ctx); err != nil {
		errs = append(errs, err)
	}

	// Shutdown the meter provider if it supports it and is NOT a custom provider
	// User-provided providers should be managed by the user
	if !c.customMeterProvider {
		if mp, ok := c.meterProvider.(*sdkmetric.MeterProvider); ok {
			c.logDebug("Shutting down meter provider")
			if err := mp.Shutdown(ctx); err != nil {
				errs = append(errs, fmt.Errorf("meter provider shutdown: %w", err))
			}
		}
	} else {
		c.logDebug("Skipping shutdown of custom meter provider (managed by user)")
	}

	// Return combined errors if any
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}

// IsEnabled returns true if metrics are enabled.
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
