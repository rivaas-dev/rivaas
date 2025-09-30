package router

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

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
	for i := range 100 {
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
