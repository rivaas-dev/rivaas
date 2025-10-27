package metrics

import (
	"context"
	"fmt"
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

// initializeProvider initializes the metrics provider based on configuration.
func (c *Config) initializeProvider() error {
	// If user provided a custom meter provider, skip built-in provider initialization
	if c.customMeterProvider {
		if c.meterProvider == nil {
			return fmt.Errorf("custom meter provider is nil")
		}
		c.logDebug("Using custom user-provided meter provider")
		c.meter = c.meterProvider.Meter("github.com/rivaas-dev/rivaas/metrics")
		return c.initializeMetrics()
	}

	// Otherwise, initialize built-in provider
	switch c.provider {
	case PrometheusProvider:
		return c.initPrometheusProvider()
	case OTLPProvider:
		return c.initOTLPProvider()
	case StdoutProvider:
		return c.initStdoutProvider()
	default:
		return fmt.Errorf("unsupported metrics provider: %s", c.provider)
	}
}

// initPrometheusProvider initializes the Prometheus metrics provider.
func (c *Config) initPrometheusProvider() error {
	// Create a custom Prometheus registry to avoid conflicts with global registry
	c.prometheusRegistry = promclient.NewRegistry()

	// Create Prometheus exporter with custom registry
	exporter, err := prometheus.New(
		prometheus.WithRegisterer(c.prometheusRegistry),
	)
	if err != nil {
		return fmt.Errorf("failed to create Prometheus exporter: %w", err)
	}

	c.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)

	// Create handler for the custom registry
	c.prometheusHandler = promhttp.HandlerFor(
		c.prometheusRegistry,
		promhttp.HandlerOpts{},
	)

	// Set global meter provider
	// WARNING: This overwrites any previously set global meter provider
	c.logDebug("Setting global OpenTelemetry meter provider", "provider", "prometheus")
	otel.SetMeterProvider(c.meterProvider)

	c.meter = c.meterProvider.Meter("github.com/rivaas-dev/rivaas/metrics")

	// Initialize metrics instruments
	if err := c.initializeMetrics(); err != nil {
		return err
	}

	// Start the metrics server if auto-start is enabled
	if c.autoStartServer {
		c.startMetricsServer()
	}

	return nil
}

// initOTLPProvider initializes the OTLP metrics provider.
func (c *Config) initOTLPProvider() error {
	opts := []otlpmetrichttp.Option{}

	if c.endpoint != "" {
		// Parse endpoint to extract host:port and determine if HTTP or HTTPS
		endpoint := c.endpoint
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
		sdkmetric.WithInterval(c.exportInterval),
	)

	c.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
	)

	// Set global meter provider
	// WARNING: This overwrites any previously set global meter provider
	c.logDebug("Setting global OpenTelemetry meter provider", "provider", "otlp")
	otel.SetMeterProvider(c.meterProvider)

	c.meter = c.meterProvider.Meter("github.com/rivaas-dev/rivaas/metrics")
	return c.initializeMetrics()
}

// initStdoutProvider initializes the stdout metrics provider.
func (c *Config) initStdoutProvider() error {
	exporter, err := stdoutmetric.New()
	if err != nil {
		return fmt.Errorf("failed to create stdout exporter: %w", err)
	}

	reader := sdkmetric.NewPeriodicReader(
		exporter,
		sdkmetric.WithInterval(c.exportInterval),
	)

	c.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
	)

	// Set global meter provider
	// WARNING: This overwrites any previously set global meter provider
	c.logDebug("Setting global OpenTelemetry meter provider", "provider", "stdout")
	otel.SetMeterProvider(c.meterProvider)

	c.meter = c.meterProvider.Meter("github.com/rivaas-dev/rivaas/metrics")
	return c.initializeMetrics()
}

// startMetricsServer starts a dedicated HTTP server for Prometheus metrics.
func (c *Config) startMetricsServer() {
	if c.prometheusHandler == nil {
		return
	}

	// Check if shutting down
	if c.isShuttingDown.Load() {
		c.logDebug("Not starting metrics server: shutdown in progress")
		return
	}

	var actualPort string
	var err error
	originalPort := c.metricsPort

	if c.strictPort {
		// Strict mode: use exact port only
		listener, err := net.Listen("tcp", c.metricsPort)
		if err != nil {
			c.logError("Failed to start metrics server on required port (strict mode)",
				"error", err, "port", c.metricsPort)
			return
		}
		listener.Close() // Close immediately, we'll reopen in ListenAndServe
		actualPort = c.metricsPort
	} else {
		// Flexible mode: try to find an available port
		actualPort, err = findAvailablePort(c.metricsPort)
		if err != nil {
			c.logError("Failed to find available port for metrics server", "error", err, "preferred_port", c.metricsPort)
			return
		}
	}

	// Update the metrics port to the actual port we're using
	c.metricsPort = actualPort

	mux := http.NewServeMux()
	mux.Handle(c.metricsPath, c.prometheusHandler)

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
	c.serverMutex.Lock()
	c.metricsServer = server
	c.serverMutex.Unlock()

	// Capture metricsPath and originalPort before goroutine to avoid race
	metricsPath := c.metricsPath
	capturedOriginalPort := originalPort

	go func() {
		// Log which port we're actually using
		if actualPort != capturedOriginalPort {
			c.logWarn("Metrics server using different port than requested",
				"actual_address", actualPort+metricsPath,
				"requested_port", capturedOriginalPort,
				"path", metricsPath,
				"reason", "requested port was unavailable",
				"recommendation", "use WithStrictPort() to fail instead of auto-discovering")
		} else {
			c.logInfo("Metrics server starting",
				"address", actualPort+metricsPath,
				"path", metricsPath)
		}

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Clear the server reference on error with mutex protection
			c.serverMutex.Lock()
			c.metricsServer = nil
			c.serverMutex.Unlock()
			c.logError("Metrics server error", "error", err)
		}
	}()
}

// stopMetricsServer stops the dedicated metrics server.
func (c *Config) stopMetricsServer(ctx context.Context) error {
	c.serverMutex.Lock()
	server := c.metricsServer
	c.metricsServer = nil // Clear first to avoid race conditions
	c.serverMutex.Unlock()

	if server != nil {
		c.logDebug("Shutting down metrics server")
		if err := server.Shutdown(ctx); err != nil {
			c.logError("Error shutting down metrics server", "error", err)
			return fmt.Errorf("metrics server shutdown: %w", err)
		}
		c.logDebug("Metrics server shut down successfully")
	}
	return nil
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
