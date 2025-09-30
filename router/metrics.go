package router

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
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

	// Custom metrics cache
	customCounters   map[string]metric.Int64Counter
	customHistograms map[string]metric.Float64Histogram
	customGauges     map[string]metric.Float64Gauge
	metricsMutex     sync.RWMutex // Protects custom metrics maps
	maxCustomMetrics int          // Maximum number of custom metrics
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
			customCounters:   make(map[string]metric.Int64Counter),
			customHistograms: make(map[string]metric.Float64Histogram),
			customGauges:     make(map[string]metric.Float64Gauge),
			maxCustomMetrics: 1000, // Limit to prevent memory leaks
		}

		// Read from environment variables if available
		config.readFromEnv()

		// Initialize the provider
		if err := config.initializeProvider(); err != nil {
			panic(fmt.Sprintf("Failed to initialize metrics: %v", err))
		}

		r.metrics = config
	}
}

// WithMetricsProviderOTLP enables OTLP metrics provider with optional endpoint.
func WithMetricsProviderOTLP(endpoint ...string) RouterOption {
	return func(r *Router) {
		if r.metrics == nil {
			WithMetrics()(r)
		}

		// Stop Prometheus server if it was started (stopMetricsServer waits for graceful shutdown)
		r.metrics.stopMetricsServer()

		r.metrics.provider = OTLPProvider
		if len(endpoint) > 0 && endpoint[0] != "" {
			r.metrics.endpoint = endpoint[0]
		} else if r.metrics.endpoint == "" {
			r.metrics.endpoint = "http://localhost:4318"
		}

		if err := r.metrics.initializeProvider(); err != nil {
			panic(fmt.Sprintf("Failed to initialize OTLP metrics: %v", err))
		}
	}
}

// WithMetricsProviderStdout enables stdout metrics provider (for development/testing).
func WithMetricsProviderStdout() RouterOption {
	return func(r *Router) {
		if r.metrics == nil {
			WithMetrics()(r)
		}

		// Stop Prometheus server if it was started (stopMetricsServer waits for graceful shutdown)
		r.metrics.stopMetricsServer()

		r.metrics.provider = StdoutProvider
		if err := r.metrics.initializeProvider(); err != nil {
			panic(fmt.Sprintf("Failed to initialize stdout metrics: %v", err))
		}
	}
}

// WithMetricsPort sets the port for the Prometheus metrics server.
// Default is ":9090". Only affects Prometheus provider.
func WithMetricsPort(port string) RouterOption {
	return func(r *Router) {
		if r.metrics != nil {
			r.metrics.metricsPort = port
		}
	}
}

// WithMetricsPath sets the path for the Prometheus metrics endpoint.
// Default is "/metrics". Only affects Prometheus provider.
func WithMetricsPath(path string) RouterOption {
	return func(r *Router) {
		if r.metrics != nil {
			r.metrics.metricsPath = path
		}
	}
}

// WithMetricsServerDisabled disables the automatic metrics server for Prometheus.
// Use this if you want to manually serve metrics via GetMetricsHandler().
func WithMetricsServerDisabled() RouterOption {
	return func(r *Router) {
		if r.metrics != nil {
			r.metrics.autoStartServer = false
			if r.metrics.metricsServer != nil {
				r.metrics.stopMetricsServer()
			}
		}
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

// initPrometheusProvider initializes the Prometheus metrics provider.
func (m *MetricsConfig) initPrometheusProvider() error {
	// Create a custom Prometheus registry to avoid conflicts with global registry
	m.prometheusRegistry = promclient.NewRegistry()

	// Create Prometheus exporter with custom registry
	exporter, err := prometheus.New(
		prometheus.WithRegisterer(m.prometheusRegistry),
	)
	if err != nil {
		return fmt.Errorf("failed to create Prometheus exporter: %w", err)
	}

	m.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)

	// Create handler for the custom registry
	m.prometheusHandler = promhttp.HandlerFor(
		m.prometheusRegistry,
		promhttp.HandlerOpts{},
	)

	// Set global meter provider
	otel.SetMeterProvider(m.meterProvider)

	m.meter = m.meterProvider.Meter("github.com/rivaas-dev/rivaas/router")

	// Initialize metrics instruments
	if err := m.initializeMetrics(); err != nil {
		return err
	}

	// Start the metrics server if auto-start is enabled
	if m.autoStartServer {
		m.startMetricsServer()
	}

	return nil
}

// initOTLPProvider initializes the OTLP metrics provider.
func (m *MetricsConfig) initOTLPProvider() error {
	opts := []otlpmetrichttp.Option{}

	if m.endpoint != "" {
		// Parse endpoint to extract host:port and determine if HTTP or HTTPS
		endpoint := m.endpoint
		isHTTP := false

		// Remove protocol prefix if present
		if strings.HasPrefix(endpoint, "http://") {
			endpoint = strings.TrimPrefix(endpoint, "http://")
			isHTTP = true
		} else if strings.HasPrefix(endpoint, "https://") {
			endpoint = strings.TrimPrefix(endpoint, "https://")
		}

		// Remove trailing path if present
		if idx := strings.Index(endpoint, "/"); idx != -1 {
			endpoint = endpoint[:idx]
		}

		opts = append(opts, otlpmetrichttp.WithEndpoint(endpoint))
		if isHTTP {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}
	}

	exporter, err := otlpmetrichttp.New(context.Background(), opts...)
	if err != nil {
		return fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	reader := sdkmetric.NewPeriodicReader(
		exporter,
		sdkmetric.WithInterval(m.exportInterval),
	)

	m.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
	)

	// Set global meter provider
	otel.SetMeterProvider(m.meterProvider)

	m.meter = m.meterProvider.Meter("github.com/rivaas-dev/rivaas/router")
	return m.initializeMetrics()
}

// initStdoutProvider initializes the stdout metrics provider.
func (m *MetricsConfig) initStdoutProvider() error {
	exporter, err := stdoutmetric.New()
	if err != nil {
		return fmt.Errorf("failed to create stdout exporter: %w", err)
	}

	reader := sdkmetric.NewPeriodicReader(
		exporter,
		sdkmetric.WithInterval(m.exportInterval),
	)

	m.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
	)

	// Set global meter provider
	otel.SetMeterProvider(m.meterProvider)

	m.meter = m.meterProvider.Meter("github.com/rivaas-dev/rivaas/router")
	return m.initializeMetrics()
}

// startMetricsServer starts a dedicated HTTP server for Prometheus metrics.
func (m *MetricsConfig) startMetricsServer() {
	if m.prometheusHandler == nil {
		return
	}

	// Try to find an available port, starting with the preferred port
	actualPort, err := findAvailablePort(m.metricsPort)
	if err != nil {
		log.Printf("❌ Failed to find available port for metrics server: %v", err)
		return
	}

	// Update the metrics port to the actual port we're using
	originalPort := m.metricsPort
	m.metricsPort = actualPort

	mux := http.NewServeMux()
	mux.Handle(m.metricsPath, m.prometheusHandler)

	// Add a health endpoint for the metrics server
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"metrics-server"}`))
	})

	server := &http.Server{
		Addr:         actualPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Set the server reference with mutex protection
	m.serverMutex.Lock()
	m.metricsServer = server
	m.serverMutex.Unlock()

	// Capture metricsPath before goroutine to avoid race
	metricsPath := m.metricsPath

	go func() {
		// Log which port we're actually using
		if actualPort != originalPort {
			log.Printf("📊 Metrics server starting on %s%s (auto-discovered from %s)", actualPort, metricsPath, originalPort)
		} else {
			log.Printf("📊 Metrics server starting on %s%s", actualPort, metricsPath)
		}

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Clear the server reference on error with mutex protection
			m.serverMutex.Lock()
			m.metricsServer = nil
			m.serverMutex.Unlock()
			log.Printf("Metrics server error: %v", err)
		}
	}()
}

// stopMetricsServer stops the dedicated metrics server.
func (m *MetricsConfig) stopMetricsServer() {
	m.serverMutex.Lock()
	server := m.metricsServer
	m.metricsServer = nil // Clear first to avoid race conditions
	m.serverMutex.Unlock()

	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down metrics server: %v", err)
		}
	}
}

// findAvailablePort attempts to find an available port starting from the given port.
// It tries the original port first, then increments until it finds an available one.
func findAvailablePort(preferredPort string) (string, error) {
	// Handle port format (:port or just port number)
	port := preferredPort
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	// Extract the numeric part
	portStr := strings.TrimPrefix(port, ":")
	portNum, err := strconv.Atoi(portStr)
	if err != nil {
		return "", fmt.Errorf("invalid port format: %s", preferredPort)
	}

	// Try up to 100 ports starting from the preferred port
	for i := 0; i < 100; i++ {
		testPort := portNum + i
		testAddr := fmt.Sprintf(":%d", testPort)

		// Try to listen on the port
		listener, err := net.Listen("tcp", testAddr)
		if err == nil {
			// Port is available
			listener.Close()
			return testAddr, nil
		}
	}

	return "", fmt.Errorf("no available port found starting from %s", preferredPort)
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

// requestMetrics holds metrics data for a single request.
type requestMetrics struct {
	startTime   time.Time
	requestSize int64
	attributes  []attribute.KeyValue
}

// startMetrics initializes metrics collection for a request.
func (r *Router) startMetrics(c *Context, path string, isStatic bool) *requestMetrics {
	if r.metrics == nil || !r.metrics.enabled {
		return nil
	}

	// Check if path should be excluded
	if r.metrics.excludePaths[path] {
		return nil
	}

	metrics := &requestMetrics{
		startTime: time.Now(),
	}

	// Calculate request size
	if c.Request.ContentLength > 0 {
		metrics.requestSize = c.Request.ContentLength
	}

	// Build base attributes
	metrics.attributes = []attribute.KeyValue{
		attribute.String("http.method", c.Request.Method),
		attribute.String("http.route", path),
		attribute.String("http.host", c.Request.Host),
		attribute.String("service.name", r.metrics.serviceName),
		attribute.String("service.version", r.metrics.serviceVersion),
		attribute.Bool("rivaas.router.static_route", isStatic),
	}

	// Record parameters if enabled
	if r.metrics.recordParams && c.paramCount > 0 {
		for i := 0; i < c.paramCount; i++ {
			metrics.attributes = append(metrics.attributes, attribute.String(
				fmt.Sprintf("http.route.param.%s", c.paramKeys[i]),
				c.paramValues[i],
			))
		}
	}

	// Record specific headers if configured
	for _, header := range r.metrics.recordHeaders {
		if value := c.Request.Header.Get(header); value != "" {
			metrics.attributes = append(metrics.attributes, attribute.String(
				fmt.Sprintf("http.request.header.%s", strings.ToLower(header)),
				value,
			))
		}
	}

	// Increment active requests
	r.metrics.activeRequests.Add(context.Background(), 1, metric.WithAttributes(metrics.attributes...))

	// Record request size
	if metrics.requestSize > 0 {
		r.metrics.requestSize.Record(context.Background(), metrics.requestSize, metric.WithAttributes(metrics.attributes...))
	}

	return metrics
}

// finishMetrics completes metrics collection for a request.
func (r *Router) finishMetrics(c *Context, requestMetrics *requestMetrics) {
	if requestMetrics == nil {
		return
	}

	// Calculate duration
	duration := time.Since(requestMetrics.startTime).Seconds()

	// Capture response status if available
	statusCode := 200 // Default to 200 if not set
	if rw, ok := c.Response.(interface{ StatusCode() int }); ok {
		statusCode = rw.StatusCode()
	}

	// Add status code to attributes
	finalAttributes := append(requestMetrics.attributes,
		attribute.Int("http.status_code", statusCode),
		attribute.String("http.status_class", getStatusClass(statusCode)),
	)

	// Record duration
	r.metrics.requestDuration.Record(context.Background(), duration, metric.WithAttributes(finalAttributes...))

	// Increment request count
	r.metrics.requestCount.Add(context.Background(), 1, metric.WithAttributes(finalAttributes...))

	// Decrement active requests
	r.metrics.activeRequests.Add(context.Background(), -1, metric.WithAttributes(finalAttributes...))

	// Record error if status indicates error
	if statusCode >= 400 {
		r.metrics.errorCount.Add(context.Background(), 1, metric.WithAttributes(finalAttributes...))
	}

	// Record response size if available
	if rw, ok := c.Response.(interface{ Size() int }); ok {
		if size := rw.Size(); size > 0 {
			r.metrics.responseSize.Record(context.Background(), int64(size), metric.WithAttributes(finalAttributes...))
		}
	}
}

// getStatusClass returns the HTTP status class (1xx, 2xx, 3xx, 4xx, 5xx).
func getStatusClass(statusCode int) string {
	switch {
	case statusCode >= 100 && statusCode < 200:
		return "1xx"
	case statusCode >= 200 && statusCode < 300:
		return "2xx"
	case statusCode >= 300 && statusCode < 400:
		return "3xx"
	case statusCode >= 400 && statusCode < 500:
		return "4xx"
	case statusCode >= 500:
		return "5xx"
	default:
		return "unknown"
	}
}

// Custom metrics management methods

// getOrCreateCounter gets an existing counter or creates a new one.
// Returns error if max custom metrics limit is reached to prevent memory leaks.
func (m *MetricsConfig) getOrCreateCounter(name string) (metric.Int64Counter, error) {
	m.metricsMutex.RLock()
	if counter, exists := m.customCounters[name]; exists {
		m.metricsMutex.RUnlock()
		return counter, nil
	}
	m.metricsMutex.RUnlock()

	// Check if we've reached the limit before creating
	m.metricsMutex.Lock()
	defer m.metricsMutex.Unlock()

	// Double-check after acquiring write lock
	if counter, exists := m.customCounters[name]; exists {
		return counter, nil
	}

	// Check total custom metrics count
	totalMetrics := len(m.customCounters) + len(m.customHistograms) + len(m.customGauges)
	if totalMetrics >= m.maxCustomMetrics {
		// Record failure metric
		if m.customMetricFailures != nil {
			m.customMetricFailures.Add(context.Background(), 1,
				metric.WithAttributes(
					attribute.String("metric_type", "counter"),
					attribute.String("metric_name", name),
					attribute.String("reason", "limit_reached"),
				))
		}
		return nil, fmt.Errorf("max custom metrics limit (%d) reached, cannot create counter %s", m.maxCustomMetrics, name)
	}

	counter, err := m.meter.Int64Counter(
		name,
		metric.WithDescription(fmt.Sprintf("Custom counter metric: %s", name)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create counter %s: %w", name, err)
	}

	m.customCounters[name] = counter
	return counter, nil
}

// getOrCreateHistogram gets an existing histogram or creates a new one.
// Returns error if max custom metrics limit is reached to prevent memory leaks.
func (m *MetricsConfig) getOrCreateHistogram(name string) (metric.Float64Histogram, error) {
	m.metricsMutex.RLock()
	if histogram, exists := m.customHistograms[name]; exists {
		m.metricsMutex.RUnlock()
		return histogram, nil
	}
	m.metricsMutex.RUnlock()

	// Check if we've reached the limit before creating
	m.metricsMutex.Lock()
	defer m.metricsMutex.Unlock()

	// Double-check after acquiring write lock
	if histogram, exists := m.customHistograms[name]; exists {
		return histogram, nil
	}

	// Check total custom metrics count
	totalMetrics := len(m.customCounters) + len(m.customHistograms) + len(m.customGauges)
	if totalMetrics >= m.maxCustomMetrics {
		// Record failure metric
		if m.customMetricFailures != nil {
			m.customMetricFailures.Add(context.Background(), 1,
				metric.WithAttributes(
					attribute.String("metric_type", "histogram"),
					attribute.String("metric_name", name),
					attribute.String("reason", "limit_reached"),
				))
		}
		return nil, fmt.Errorf("max custom metrics limit (%d) reached, cannot create histogram %s", m.maxCustomMetrics, name)
	}

	histogram, err := m.meter.Float64Histogram(
		name,
		metric.WithDescription(fmt.Sprintf("Custom histogram metric: %s", name)),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create histogram %s: %w", name, err)
	}

	m.customHistograms[name] = histogram
	return histogram, nil
}

// getOrCreateGauge gets an existing gauge or creates a new one.
// Returns error if max custom metrics limit is reached to prevent memory leaks.
func (m *MetricsConfig) getOrCreateGauge(name string) (metric.Float64Gauge, error) {
	m.metricsMutex.RLock()
	if gauge, exists := m.customGauges[name]; exists {
		m.metricsMutex.RUnlock()
		return gauge, nil
	}
	m.metricsMutex.RUnlock()

	// Check if we've reached the limit before creating
	m.metricsMutex.Lock()
	defer m.metricsMutex.Unlock()

	// Double-check after acquiring write lock
	if gauge, exists := m.customGauges[name]; exists {
		return gauge, nil
	}

	// Check total custom metrics count
	totalMetrics := len(m.customCounters) + len(m.customHistograms) + len(m.customGauges)
	if totalMetrics >= m.maxCustomMetrics {
		// Record failure metric
		if m.customMetricFailures != nil {
			m.customMetricFailures.Add(context.Background(), 1,
				metric.WithAttributes(
					attribute.String("metric_type", "gauge"),
					attribute.String("metric_name", name),
					attribute.String("reason", "limit_reached"),
				))
		}
		return nil, fmt.Errorf("max custom metrics limit (%d) reached, cannot create gauge %s", m.maxCustomMetrics, name)
	}

	gauge, err := m.meter.Float64Gauge(
		name,
		metric.WithDescription(fmt.Sprintf("Custom gauge metric: %s", name)),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gauge %s: %w", name, err)
	}

	m.customGauges[name] = gauge
	return gauge, nil
}

// Context methods for custom metrics

// RecordMetric records a custom histogram metric with the given value and attributes.
// This method allows you to record custom business metrics in your handlers.
//
// Example usage:
//
//	func OrderHandler(c *router.Context) {
//	    orderValue := 99.95
//	    c.RecordMetric("order_value", orderValue,
//	        attribute.String("currency", "USD"),
//	        attribute.String("category", "electronics"),
//	    )
//	    c.JSON(200, map[string]string{"status": "success"})
//	}
func (c *Context) RecordMetric(name string, value float64, attributes ...attribute.KeyValue) {
	// Check if metrics are enabled and router reference exists
	if c.router == nil || c.router.metrics == nil || !c.router.metrics.enabled {
		return
	}

	// Get or create the histogram metric
	histogram, err := c.router.metrics.getOrCreateHistogram(name)
	if err != nil {
		// Log error but don't fail the request
		log.Printf("Failed to create/get histogram metric %s: %v", name, err)
		return
	}

	// Add service information to attributes
	allAttributes := append(attributes,
		attribute.String("service.name", c.router.metrics.serviceName),
		attribute.String("service.version", c.router.metrics.serviceVersion),
	)

	// Record the metric
	histogram.Record(context.Background(), value, metric.WithAttributes(allAttributes...))
}

// IncrementCounter increments a custom counter metric by 1.
// Use this method to count events, operations, or occurrences in your handlers.
//
// Example usage:
//
//	func LoginHandler(c *router.Context) {
//	    // ... login logic ...
//	    if loginSuccessful {
//	        c.IncrementCounter("user_logins_total",
//	            attribute.String("method", "password"),
//	            attribute.String("user_type", "premium"),
//	        )
//	    } else {
//	        c.IncrementCounter("failed_logins_total",
//	            attribute.String("reason", "invalid_password"),
//	        )
//	    }
//	}
func (c *Context) IncrementCounter(name string, attributes ...attribute.KeyValue) {
	// Check if metrics are enabled and router reference exists
	if c.router == nil || c.router.metrics == nil || !c.router.metrics.enabled {
		return
	}

	// Get or create the counter metric
	counter, err := c.router.metrics.getOrCreateCounter(name)
	if err != nil {
		// Log error but don't fail the request
		log.Printf("Failed to create/get counter metric %s: %v", name, err)
		return
	}

	// Add service information to attributes
	allAttributes := append(attributes,
		attribute.String("service.name", c.router.metrics.serviceName),
		attribute.String("service.version", c.router.metrics.serviceVersion),
	)

	// Increment the counter by 1
	counter.Add(context.Background(), 1, metric.WithAttributes(allAttributes...))
}

// SetGauge sets a custom gauge metric value.
// Use this method to record current values like queue length, memory usage, or connection counts.
//
// Example usage:
//
//	func StatusHandler(c *router.Context) {
//	    queueLength := getQueueLength()
//	    c.SetGauge("queue_length", float64(queueLength),
//	        attribute.String("queue_type", "processing"),
//	        attribute.String("priority", "high"),
//	    )
//
//	    memoryUsage := getMemoryUsagePercent()
//	    c.SetGauge("memory_usage_percent", memoryUsage,
//	        attribute.String("process", "api_server"),
//	    )
//
//	    c.JSON(200, map[string]interface{}{
//	        "queue_length": queueLength,
//	        "memory_usage": memoryUsage,
//	    })
//	}
func (c *Context) SetGauge(name string, value float64, attributes ...attribute.KeyValue) {
	// Check if metrics are enabled and router reference exists
	if c.router == nil || c.router.metrics == nil || !c.router.metrics.enabled {
		return
	}

	// Get or create the gauge metric
	gauge, err := c.router.metrics.getOrCreateGauge(name)
	if err != nil {
		// Log error but don't fail the request
		log.Printf("Failed to create/get gauge metric %s: %v", name, err)
		return
	}

	// Add service information to attributes
	allAttributes := append(attributes,
		attribute.String("service.name", c.router.metrics.serviceName),
		attribute.String("service.version", c.router.metrics.serviceVersion),
	)

	// Set the gauge value
	gauge.Record(context.Background(), value, metric.WithAttributes(allAttributes...))
}

// recordRouteRegistration records when a new route is registered.
func (r *Router) recordRouteRegistration(method, path string) {
	if r.metrics == nil || !r.metrics.enabled {
		return
	}

	attributes := []attribute.KeyValue{
		attribute.String("http.method", method),
		attribute.String("http.route", path),
		attribute.String("service.name", r.metrics.serviceName),
		attribute.String("service.version", r.metrics.serviceVersion),
	}

	r.metrics.routeCount.Add(context.Background(), 1, metric.WithAttributes(attributes...))
}

// serveWithMetrics handles static routes with metrics.
func (r *Router) serveWithMetrics(w http.ResponseWriter, req *http.Request, handlers []HandlerFunc, path string, isStatic bool) {
	ctx := &Context{
		Request:    req,
		Response:   w,
		index:      -1,
		paramCount: 0,
		router:     r,
	}

	requestMetrics := r.startMetrics(ctx, path, isStatic)
	defer r.finishMetrics(ctx, requestMetrics)

	for i := 0; i < len(handlers); i++ {
		handlers[i](ctx)
	}
}

// serveDynamicWithMetrics handles dynamic routes with metrics.
func (r *Router) serveDynamicWithMetrics(c *Context, handlers []HandlerFunc, path string) {
	requestMetrics := r.startMetrics(c, path, false)
	defer r.finishMetrics(c, requestMetrics)

	for i := 0; i < len(handlers); i++ {
		handlers[i](c)
	}
}

// serveWithTracingAndMetrics handles static routes with both tracing and metrics.
func (r *Router) serveWithTracingAndMetrics(w http.ResponseWriter, req *http.Request, handlers []HandlerFunc, path string, isStatic bool) {
	ctx := &Context{
		Request:    req,
		Response:   w,
		index:      -1,
		paramCount: 0,
		router:     r,
	}

	r.startTracing(ctx, path, isStatic)
	requestMetrics := r.startMetrics(ctx, path, isStatic)

	defer func() {
		r.finishTracing(ctx)
		r.finishMetrics(ctx, requestMetrics)
	}()

	for i := 0; i < len(handlers); i++ {
		handlers[i](ctx)
	}
}

// serveDynamicWithTracingAndMetrics handles dynamic routes with both tracing and metrics.
func (r *Router) serveDynamicWithTracingAndMetrics(c *Context, handlers []HandlerFunc, path string) {
	r.startTracing(c, path, false)
	requestMetrics := r.startMetrics(c, path, false)

	defer func() {
		r.finishTracing(c)
		r.finishMetrics(c, requestMetrics)
	}()

	for i := 0; i < len(handlers); i++ {
		handlers[i](c)
	}
}
