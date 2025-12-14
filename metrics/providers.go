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
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"

	promclient "github.com/prometheus/client_golang/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// initializeProvider initializes the metrics provider based on configuration.
func (r *Recorder) initializeProvider() error {
	// If user provided a custom meter provider, skip built-in provider initialization
	if r.customMeterProvider {
		if r.meterProvider == nil {
			return errors.New("custom meter provider is nil")
		}
		r.emitDebug("Using custom user-provided meter provider")
		r.meter = r.meterProvider.Meter("rivaas.dev/metrics")

		return r.initializeMetrics()
	}

	// Otherwise, initialize built-in provider
	switch r.provider {
	case PrometheusProvider:
		return r.initPrometheusProvider()
	case OTLPProvider:
		// OTLP initialization is deferred to Start(ctx) to use the lifecycle context.
		// This allows proper context propagation for network connections to the collector
		// and enables graceful shutdown when the context is canceled.
		r.providerDeferred.Store(true)
		return nil
	case StdoutProvider:
		// Stdout doesn't need lifecycle context (no network I/O), so initialize immediately.
		// This allows simpler debugging use cases without requiring Start().
		return r.initStdoutProvider()
	default:
		return fmt.Errorf("unsupported metrics provider: %s", r.provider)
	}
}

// initPrometheusProvider initializes the Prometheus metrics provider.
func (r *Recorder) initPrometheusProvider() error {
	// Create a custom Prometheus registry to avoid conflicts with global registry
	r.prometheusRegistry = promclient.NewRegistry()

	// Create Prometheus exporter with custom registry
	exporter, err := prometheus.New(
		prometheus.WithRegisterer(r.prometheusRegistry),
	)
	if err != nil {
		return fmt.Errorf("failed to create Prometheus exporter: %w", err)
	}

	r.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)

	// Create handler for the custom registry
	r.prometheusHandler = promhttp.HandlerFor(
		r.prometheusRegistry,
		promhttp.HandlerOpts{},
	)

	// Set global meter provider only if requested
	if r.registerGlobal {
		r.emitDebug("Setting global OpenTelemetry meter provider", "provider", "prometheus")
		otel.SetMeterProvider(r.meterProvider)
	} else {
		r.emitDebug("Skipping global meter provider registration", "provider", "prometheus")
	}

	r.meter = r.meterProvider.Meter("rivaas.dev/metrics")

	// Initialize metrics instruments
	if initErr := r.initializeMetrics(); initErr != nil {
		return initErr
	}

	// Note: Server startup is deferred to Start(ctx) method
	// This allows the application to pass a proper lifecycle context

	return nil
}

// initOTLPProvider initializes the OTLP metrics provider.
// The context is used for the OTLP exporter's network connections and background goroutines.
// This is called from Start(ctx) to use the lifecycle context, enabling proper cancellation
// during graceful shutdown.
func (r *Recorder) initOTLPProvider(ctx context.Context) error {
	opts := []otlpmetrichttp.Option{}

	if r.otlpEndpoint != "" {
		// Parse endpoint to extract host:port and determine if HTTP or HTTPS
		endpoint := r.otlpEndpoint
		isHTTP := false

		// Remove protocol prefix if present
		if trimmed, trimmedOk := strings.CutPrefix(endpoint, "http://"); trimmedOk {
			endpoint = trimmed
			isHTTP = true
		} else if trimmedHTTPS, trimmedHTTPSOk := strings.CutPrefix(endpoint, "https://"); trimmedHTTPSOk {
			endpoint = trimmedHTTPS
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

	// Use the lifecycle context for proper shutdown propagation.
	// When ctx is canceled (e.g., during graceful shutdown), the exporter
	// will stop its background goroutines and flush pending metrics.
	exporter, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	reader := sdkmetric.NewPeriodicReader(
		exporter,
		sdkmetric.WithInterval(r.exportInterval),
	)

	r.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
	)

	// Set global meter provider only if requested
	if r.registerGlobal {
		r.emitDebug("Setting global OpenTelemetry meter provider", "provider", "otlp")
		otel.SetMeterProvider(r.meterProvider)
	} else {
		r.emitDebug("Skipping global meter provider registration", "provider", "otlp")
	}

	r.meter = r.meterProvider.Meter("rivaas.dev/metrics")

	return r.initializeMetrics()
}

// initStdoutProvider initializes the stdout metrics provider.
// Unlike OTLP, stdout doesn't need a lifecycle context since it doesn't
// perform network I/O - it just writes to stdout for debugging.
func (r *Recorder) initStdoutProvider() error {
	exporter, err := stdoutmetric.New()
	if err != nil {
		return fmt.Errorf("failed to create stdout exporter: %w", err)
	}

	reader := sdkmetric.NewPeriodicReader(
		exporter,
		sdkmetric.WithInterval(r.exportInterval),
	)

	r.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
	)

	// Set global meter provider only if requested
	if r.registerGlobal {
		r.emitDebug("Setting global OpenTelemetry meter provider", "provider", "stdout")
		otel.SetMeterProvider(r.meterProvider)
	} else {
		r.emitDebug("Skipping global meter provider registration", "provider", "stdout")
	}

	r.meter = r.meterProvider.Meter("rivaas.dev/metrics")

	return r.initializeMetrics()
}

// startMetricsServer starts a dedicated HTTP server for Prometheus metrics.
// The context is used for port binding operations.
func (r *Recorder) startMetricsServer(ctx context.Context) {
	if r.prometheusHandler == nil {
		return
	}

	// Check if shutting down
	if r.isShuttingDown.Load() {
		r.emitDebug("Not starting metrics server: shutdown in progress")
		return
	}

	var actualPort string
	var err error
	originalPort := r.metricsPort

	if r.strictPort {
		// Strict mode: use exact port only
		var lc net.ListenConfig
		listener, listenErr := lc.Listen(ctx, "tcp", r.metricsPort)
		if listenErr != nil {
			r.emitError("Failed to start metrics server on required port (strict mode)",
				"error", listenErr, "port", r.metricsPort)

			return
		}
		listener.Close() // Close immediately, we'll reopen in ListenAndServe
		actualPort = r.metricsPort
	} else {
		// Flexible mode: try to find an available port
		actualPort, err = findAvailablePort(ctx, r.metricsPort)
		if err != nil {
			r.emitError("Failed to find available port for metrics server", "error", err, "preferred_port", r.metricsPort)
			return
		}
	}

	// Update the metrics port to the actual port we're using
	r.metricsPort = actualPort

	mux := http.NewServeMux()
	mux.Handle(r.metricsPath, r.prometheusHandler)

	server := &http.Server{
		Addr:         actualPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Set the server reference with mutex protection
	r.serverMutex.Lock()
	r.metricsServer = server
	r.serverMutex.Unlock()

	// Capture metricsPath and originalPort before goroutine to avoid race
	metricsPath := r.metricsPath
	capturedOriginalPort := originalPort

	go func() {
		// Log which port we're actually using
		if actualPort != capturedOriginalPort {
			r.emitWarning("Metrics server using different port than requested",
				"actual_address", actualPort+metricsPath,
				"requested_port", capturedOriginalPort,
				"path", metricsPath,
				"reason", "requested port was unavailable",
				"recommendation", "use WithStrictPort() to fail instead of auto-discovering")
		} else {
			r.emitInfo("Metrics server starting",
				"address", actualPort+metricsPath,
				"path", metricsPath)
		}

		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			// Clear the server reference on error with mutex protection
			r.serverMutex.Lock()
			r.metricsServer = nil
			r.serverMutex.Unlock()
			r.emitError("Metrics server error", "error", serveErr)
		}
	}()
}

// stopMetricsServer stops the dedicated metrics server.
func (r *Recorder) stopMetricsServer(ctx context.Context) error {
	r.serverMutex.Lock()
	server := r.metricsServer
	r.metricsServer = nil // Clear first to avoid race conditions
	r.serverMutex.Unlock()

	if server != nil {
		r.emitDebug("Shutting down metrics server")
		if err := server.Shutdown(ctx); err != nil {
			r.emitError("Error shutting down metrics server", "error", err)
			return fmt.Errorf("metrics server shutdown: %w", err)
		}
		r.emitDebug("Metrics server shut down successfully")
	}

	return nil
}

// findAvailablePort attempts to find an available port starting from the given port.
// It tries the original port first, then increments until it finds an available one.
func findAvailablePort(ctx context.Context, preferredPort string) (string, error) {
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
	var lc net.ListenConfig
	for i := range 100 {
		testPort := portNum + i
		testAddr := fmt.Sprintf(":%d", testPort)

		// Try to listen on the port
		listener, listenErr := lc.Listen(ctx, "tcp", testAddr)
		if listenErr == nil {
			// Port is available
			listener.Close()
			return testAddr, nil
		}
	}

	return "", fmt.Errorf("no available port found starting from %s", preferredPort)
}
