package router

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	promclient "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/metric"
)

// MetricsProvider represents the available metrics providers.
type MetricsProvider string

const (
	// PrometheusProvider uses Prometheus exporter for metrics (default).
	PrometheusProvider MetricsProvider = "prometheus"
	// OTLPProvider uses OTLP HTTP exporter for metrics.
	OTLPProvider MetricsProvider = "otlp"
	// StdoutProvider uses stdout exporter for metrics (development/testing).
	StdoutProvider MetricsProvider = "stdout"
)

// MetricsConfig holds OpenTelemetry metrics configuration.
type MetricsConfig struct {
	enabled            bool
	serviceName        string
	serviceVersion     string
	meter              metric.Meter
	meterProvider      metric.MeterProvider
	prometheusHandler  http.Handler
	prometheusRegistry *promclient.Registry // Custom Prometheus registry to avoid conflicts
	excludePaths       map[string]bool
	recordParams       bool
	recordHeaders      []string
	provider           MetricsProvider
	endpoint           string
	exportInterval     time.Duration

	// Prometheus-specific configuration
	metricsPort     string
	metricsPath     string
	metricsServer   *http.Server
	serverMutex     sync.Mutex // Protects metricsServer access
	autoStartServer bool

	// Built-in HTTP metrics
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

	// Atomic custom metrics cache - lock-free operations
	atomicCustomCounters   unsafe.Pointer // *map[string]metric.Int64Counter
	atomicCustomHistograms unsafe.Pointer // *map[string]metric.Float64Histogram
	atomicCustomGauges     unsafe.Pointer // *map[string]metric.Float64Gauge
	maxCustomMetrics       int            // Maximum number of custom metrics

	// Atomic counters for built-in metrics
	atomicRequestCount      int64
	atomicActiveRequests    int64
	atomicErrorCount        int64
	atomicContextPoolHits   int64
	atomicContextPoolMisses int64
}

// WithMetrics enables OpenTelemetry metrics with auto-configured Prometheus (default).
// By default, Prometheus metrics will be served on :9090/metrics
func WithMetrics() RouterOption {
	return func(r *Router) {
		config := &MetricsConfig{
			enabled:          true,
			serviceName:      "rivaas-router",
			serviceVersion:   "1.0.0",
			excludePaths:     make(map[string]bool),
			recordParams:     true,
			provider:         PrometheusProvider,
			exportInterval:   30 * time.Second,
			metricsPort:      ":9090",
			metricsPath:      "/metrics",
			autoStartServer:  true,
			maxCustomMetrics: 1000, // Limit to prevent memory leaks
		}

		// Initialize atomic custom metrics maps
		initialCounters := make(map[string]metric.Int64Counter)
		initialHistograms := make(map[string]metric.Float64Histogram)
		initialGauges := make(map[string]metric.Float64Gauge)

		atomic.StorePointer(&config.atomicCustomCounters, unsafe.Pointer(&initialCounters))
		atomic.StorePointer(&config.atomicCustomHistograms, unsafe.Pointer(&initialHistograms))
		atomic.StorePointer(&config.atomicCustomGauges, unsafe.Pointer(&initialGauges))

		// Read from environment variables if available
		config.readFromEnv()

		// Initialize the provider
		if err := config.initializeProvider(); err != nil {
			panic(fmt.Sprintf("Failed to initialize metrics: %v", err))
		}

		r.metrics = config
	}
}

// WithMetricsServiceName sets the service name for metrics.
func WithMetricsServiceName(name string) RouterOption {
	return func(r *Router) {
		if r.metrics != nil {
			r.metrics.serviceName = name
		}
	}
}

// WithMetricsServiceVersion sets the service version for metrics.
func WithMetricsServiceVersion(version string) RouterOption {
	return func(r *Router) {
		if r.metrics != nil {
			r.metrics.serviceVersion = version
		}
	}
}

// WithMetricsEndpoint sets the endpoint for OTLP metrics.
func WithMetricsEndpoint(endpoint string) RouterOption {
	return func(r *Router) {
		if r.metrics != nil {
			r.metrics.endpoint = endpoint
		}
	}
}

// WithMetricsExportInterval sets the export interval for OTLP and stdout metrics.
func WithMetricsExportInterval(interval time.Duration) RouterOption {
	return func(r *Router) {
		if r.metrics != nil {
			r.metrics.exportInterval = interval
		}
	}
}

// WithMetricsExcludePaths excludes specific paths from metrics collection.
func WithMetricsExcludePaths(paths ...string) RouterOption {
	return func(r *Router) {
		if r.metrics != nil {
			for _, path := range paths {
				r.metrics.excludePaths[path] = true
			}
		}
	}
}

// WithMetricsHeaders records specific headers as metric attributes.
func WithMetricsHeaders(headers ...string) RouterOption {
	return func(r *Router) {
		if r.metrics != nil {
			r.metrics.recordHeaders = headers
		}
	}
}

// WithMetricsDisableParams disables recording URL parameters in metrics.
func WithMetricsDisableParams() RouterOption {
	return func(r *Router) {
		if r.metrics != nil {
			r.metrics.recordParams = false
		}
	}
}

// readFromEnv reads configuration from environment variables.
func (m *MetricsConfig) readFromEnv() {
	// OTEL_METRICS_EXPORTER
	if exporter := os.Getenv("OTEL_METRICS_EXPORTER"); exporter != "" {
		switch strings.ToLower(exporter) {
		case "prometheus":
			m.provider = PrometheusProvider
		case "otlp":
			m.provider = OTLPProvider
		case "stdout":
			m.provider = StdoutProvider
		}
	}

	// OTEL_EXPORTER_OTLP_METRICS_ENDPOINT or OTEL_EXPORTER_OTLP_ENDPOINT
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"); endpoint != "" {
		m.endpoint = endpoint
	} else if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		m.endpoint = endpoint
	}

	// OTEL_SERVICE_NAME
	if serviceName := os.Getenv("OTEL_SERVICE_NAME"); serviceName != "" {
		m.serviceName = serviceName
	}

	// OTEL_SERVICE_VERSION
	if serviceVersion := os.Getenv("OTEL_SERVICE_VERSION"); serviceVersion != "" {
		m.serviceVersion = serviceVersion
	}

	// RIVAAS_METRICS_PORT (custom env var for metrics port)
	if port := os.Getenv("RIVAAS_METRICS_PORT"); port != "" {
		if !strings.HasPrefix(port, ":") {
			port = ":" + port
		}
		m.metricsPort = port
	}

	// RIVAAS_METRICS_PATH (custom env var for metrics path)
	if path := os.Getenv("RIVAAS_METRICS_PATH"); path != "" {
		m.metricsPath = path
	}
}

// initializeProvider initializes the metrics provider based on configuration.
func (m *MetricsConfig) initializeProvider() error {
	switch m.provider {
	case PrometheusProvider:
		return m.initPrometheusProvider()
	case OTLPProvider:
		return m.initOTLPProvider()
	case StdoutProvider:
		return m.initStdoutProvider()
	default:
		return fmt.Errorf("unsupported metrics provider: %s", m.provider)
	}
}

// initializeMetrics creates all the metric instruments.
func (m *MetricsConfig) initializeMetrics() error {
	var err error

	// Request duration histogram
	m.requestDuration, err = m.meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("Duration of HTTP requests in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create request duration histogram: %w", err)
	}

	// Request count counter
	m.requestCount, err = m.meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return fmt.Errorf("failed to create request count counter: %w", err)
	}

	// Active requests gauge
	m.activeRequests, err = m.meter.Int64UpDownCounter(
		"http_requests_active",
		metric.WithDescription("Number of active HTTP requests"),
	)
	if err != nil {
		return fmt.Errorf("failed to create active requests gauge: %w", err)
	}

	// Request size histogram
	m.requestSize, err = m.meter.Int64Histogram(
		"http_request_size_bytes",
		metric.WithDescription("Size of HTTP request bodies in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("failed to create request size histogram: %w", err)
	}

	// Response size histogram
	m.responseSize, err = m.meter.Int64Histogram(
		"http_response_size_bytes",
		metric.WithDescription("Size of HTTP response bodies in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("failed to create response size histogram: %w", err)
	}

	// Route count counter
	m.routeCount, err = m.meter.Int64Counter(
		"http_routes_total",
		metric.WithDescription("Total number of registered routes"),
	)
	if err != nil {
		return fmt.Errorf("failed to create route count counter: %w", err)
	}

	// Error count counter
	m.errorCount, err = m.meter.Int64Counter(
		"http_errors_total",
		metric.WithDescription("Total number of HTTP errors"),
	)
	if err != nil {
		return fmt.Errorf("failed to create error count counter: %w", err)
	}

	// Constraint failures counter
	m.constraintFailures, err = m.meter.Int64Counter(
		"http_constraint_failures_total",
		metric.WithDescription("Total number of route constraint validation failures"),
	)
	if err != nil {
		return fmt.Errorf("failed to create constraint failures counter: %w", err)
	}

	// Context pool hits counter
	m.contextPoolHits, err = m.meter.Int64Counter(
		"router_context_pool_hits_total",
		metric.WithDescription("Total number of context pool hits"),
	)
	if err != nil {
		return fmt.Errorf("failed to create context pool hits counter: %w", err)
	}

	// Context pool misses counter
	m.contextPoolMisses, err = m.meter.Int64Counter(
		"router_context_pool_misses_total",
		metric.WithDescription("Total number of context pool misses (new allocations)"),
	)
	if err != nil {
		return fmt.Errorf("failed to create context pool misses counter: %w", err)
	}

	// Custom metric failures counter
	m.customMetricFailures, err = m.meter.Int64Counter(
		"router_custom_metric_failures_total",
		metric.WithDescription("Total number of custom metric creation failures"),
	)
	if err != nil {
		return fmt.Errorf("failed to create custom metric failures counter: %w", err)
	}

	return nil
}

// GetMetricsHandler returns the Prometheus metrics HTTP handler.
// This is useful when you want to serve metrics manually or disable the auto-server.
func (r *Router) GetMetricsHandler() http.Handler {
	if r.metrics == nil {
		panic("Metrics not enabled. Use WithMetrics() to enable metrics.")
	}

	if r.metrics.provider != PrometheusProvider || r.metrics.prometheusHandler == nil {
		panic("Prometheus handler is only available when using Prometheus provider. Use WithMetrics() for Prometheus (default) or switch providers.")
	}

	return r.metrics.prometheusHandler
}

// GetMetricsProvider returns the current metrics provider.
func (r *Router) GetMetricsProvider() MetricsProvider {
	if r.metrics == nil {
		return ""
	}
	return r.metrics.provider
}

// GetMetricsServerAddress returns the address of the metrics server.
// Returns empty string if not using Prometheus or server is disabled.
func (r *Router) GetMetricsServerAddress() string {
	if r.metrics == nil || r.metrics.provider != PrometheusProvider || !r.metrics.autoStartServer {
		return ""
	}
	return r.metrics.metricsPort
}

// StopMetricsServer stops the dedicated metrics server.
// This is automatically called when the router is garbage collected.
func (r *Router) StopMetricsServer() {
	if r.metrics != nil {
		r.metrics.stopMetricsServer()
	}
}
