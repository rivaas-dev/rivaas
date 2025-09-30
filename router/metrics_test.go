package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestMetricsDisabled(t *testing.T) {
	r := New()

	r.GET("/test", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if r.metrics != nil {
		t.Error("Expected no metrics configuration")
	}
}

func TestMetricsPrometheusDefault(t *testing.T) {
	r := New(
		WithMetrics(),
		WithMetricsPort(":9091"), // Use unique port to avoid conflicts
		WithMetricsServiceName("test-service"),
		WithMetricsServiceVersion("v1.0.0"),
	)
	defer r.StopMetricsServer()

	// Give the server a moment to start
	time.Sleep(10 * time.Millisecond)

	if r.metrics == nil {
		t.Fatal("Expected metrics to be configured")
	}

	if r.metrics.provider != PrometheusProvider {
		t.Errorf("Expected Prometheus provider by default, got %s", r.metrics.provider)
	}

	// Note: The actual port might be auto-discovered if 9091 is in use
	address := r.GetMetricsServerAddress()
	if address == "" {
		t.Error("Expected non-empty metrics server address")
	}

	// Should be able to get Prometheus handler
	handler := r.GetMetricsHandler()
	if handler == nil {
		t.Error("Expected non-nil Prometheus handler")
	}

	// Test the handler
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 from metrics handler, got %d", w.Code)
	}
}

func TestMetricsCustomPort(t *testing.T) {
	r := New(
		WithMetrics(),
		WithMetricsPort(":9092"),
		WithMetricsPath("/custom-metrics"),
		WithMetricsServiceName("test-service"),
	)
	defer r.StopMetricsServer()

	// Note: Port might be auto-discovered, so check the actual configured port
	actualPort := r.GetMetricsServerAddress()
	if actualPort == "" {
		t.Error("Expected non-empty server address")
	}

	if r.metrics.metricsPath != "/custom-metrics" {
		t.Errorf("Expected custom path /custom-metrics, got %s", r.metrics.metricsPath)
	}
}

func TestMetricsServerDisabled(t *testing.T) {
	r := New(
		WithMetrics(),
		WithMetricsServerDisabled(),
		WithMetricsServiceName("test-service"),
	)

	if r.metrics.autoStartServer {
		t.Error("Expected auto-start server to be disabled")
	}

	if r.GetMetricsServerAddress() != "" {
		t.Errorf("Expected empty server address when disabled, got %s", r.GetMetricsServerAddress())
	}

	// Should still be able to get the handler for manual serving
	handler := r.GetMetricsHandler()
	if handler == nil {
		t.Error("Expected non-nil Prometheus handler even when server disabled")
	}
}

func TestMetricsProviderOTLP(t *testing.T) {
	r := New(
		WithMetrics(),
		WithMetricsProviderOTLP("http://localhost:4318"),
		WithMetricsServiceName("test-service"),
	)

	if r.metrics == nil {
		t.Fatal("Expected metrics to be configured")
	}

	if r.metrics.provider != OTLPProvider {
		t.Errorf("Expected OTLP provider, got %s", r.metrics.provider)
	}

	if r.metrics.endpoint != "http://localhost:4318" {
		t.Errorf("Expected endpoint http://localhost:4318, got %s", r.metrics.endpoint)
	}

	// Should not have metrics server for OTLP
	if r.GetMetricsServerAddress() != "" {
		t.Errorf("Expected no metrics server address for OTLP, got %s", r.GetMetricsServerAddress())
	}
}

func TestMetricsProviderOTLPDefaultEndpoint(t *testing.T) {
	r := New(
		WithMetrics(),
		WithMetricsProviderOTLP(), // No endpoint specified
		WithMetricsServiceName("test-service"),
	)

	if r.metrics.endpoint != "http://localhost:4318" {
		t.Errorf("Expected default OTLP endpoint, got %s", r.metrics.endpoint)
	}
}

func TestMetricsProviderStdout(t *testing.T) {
	r := New(
		WithMetrics(),
		WithMetricsProviderStdout(),
		WithMetricsServiceName("test-service"),
		WithMetricsExportInterval(1*time.Second),
	)

	if r.metrics == nil {
		t.Fatal("Expected metrics to be configured")
	}

	if r.metrics.provider != StdoutProvider {
		t.Errorf("Expected stdout provider, got %s", r.metrics.provider)
	}

	if r.GetMetricsProvider() != StdoutProvider {
		t.Errorf("Expected stdout provider, got %s", r.GetMetricsProvider())
	}

	// Should not have metrics server for stdout
	if r.GetMetricsServerAddress() != "" {
		t.Errorf("Expected no metrics server address for stdout, got %s", r.GetMetricsServerAddress())
	}
}

func TestMetricsEnvironmentConfig(t *testing.T) {
	// Save original env vars
	oldService := os.Getenv("OTEL_SERVICE_NAME")
	oldVersion := os.Getenv("OTEL_SERVICE_VERSION")
	oldPort := os.Getenv("RIVAAS_METRICS_PORT")
	oldPath := os.Getenv("RIVAAS_METRICS_PATH")

	defer func() {
		os.Setenv("OTEL_SERVICE_NAME", oldService)
		os.Setenv("OTEL_SERVICE_VERSION", oldVersion)
		os.Setenv("RIVAAS_METRICS_PORT", oldPort)
		os.Setenv("RIVAAS_METRICS_PATH", oldPath)
	}()

	// Set test environment variables
	os.Setenv("OTEL_SERVICE_NAME", "env-test-service")
	os.Setenv("OTEL_SERVICE_VERSION", "v2.0.0")
	os.Setenv("RIVAAS_METRICS_PORT", "8090")
	os.Setenv("RIVAAS_METRICS_PATH", "/prometheus")

	r := New(WithMetrics())
	defer r.StopMetricsServer()

	// Check that environment config was applied
	if r.metrics.serviceName != "env-test-service" {
		t.Errorf("Expected service name from env, got %s", r.metrics.serviceName)
	}

	if r.metrics.serviceVersion != "v2.0.0" {
		t.Errorf("Expected service version from env, got %s", r.metrics.serviceVersion)
	}

	if r.metrics.metricsPort != ":8090" {
		t.Errorf("Expected metrics port from env :8090, got %s", r.metrics.metricsPort)
	}

	if r.metrics.metricsPath != "/prometheus" {
		t.Errorf("Expected metrics path from env /prometheus, got %s", r.metrics.metricsPath)
	}
}

func TestMetricsPortWithoutColon(t *testing.T) {
	// Test that port numbers without colon are handled correctly
	oldPort := os.Getenv("RIVAAS_METRICS_PORT")
	defer os.Setenv("RIVAAS_METRICS_PORT", oldPort)

	os.Setenv("RIVAAS_METRICS_PORT", "9091") // Without colon

	r := New(WithMetrics())
	defer r.StopMetricsServer()

	if r.metrics.metricsPort != ":9091" {
		t.Errorf("Expected :9091 (with colon), got %s", r.metrics.metricsPort)
	}
}

func TestGetMetricsHandlerPanicWithoutMetrics(t *testing.T) {
	r := New() // No metrics enabled

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when getting metrics handler without metrics enabled")
		}
	}()

	r.GetMetricsHandler()
}

func TestGetMetricsHandlerPanicWithOTLP(t *testing.T) {
	r := New(
		WithMetrics(),
		WithMetricsProviderOTLP(),
	)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when getting Prometheus handler with OTLP provider")
		}
	}()

	r.GetMetricsHandler()
}

func TestMetricsWithRequest(t *testing.T) {
	r := New(
		WithMetrics(),
		WithMetricsServiceName("test-service"),
	)
	defer r.StopMetricsServer()

	r.GET("/test/:id", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{
			"id": c.Param("id"),
		})
	})

	req := httptest.NewRequest("GET", "/test/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestMetricsConfiguration(t *testing.T) {
	r := New(
		WithMetrics(),
		WithMetricsServiceName("custom-service"),
		WithMetricsServiceVersion("v2.0.0"),
		WithMetricsPort(":8091"),
		WithMetricsPath("/custom-metrics"),
		WithMetricsExcludePaths("/health"),
		WithMetricsHeaders("Authorization"),
		WithMetricsDisableParams(),
		WithMetricsExportInterval(30*time.Second),
	)
	defer r.StopMetricsServer()

	if r.metrics == nil {
		t.Fatal("Expected metrics config to be set")
	}

	if r.metrics.serviceName != "custom-service" {
		t.Errorf("Expected service name 'custom-service', got '%s'", r.metrics.serviceName)
	}

	if r.metrics.serviceVersion != "v2.0.0" {
		t.Errorf("Expected service version 'v2.0.0', got '%s'", r.metrics.serviceVersion)
	}

	if r.metrics.metricsPort != ":8091" {
		t.Errorf("Expected metrics port ':8091', got '%s'", r.metrics.metricsPort)
	}

	if r.metrics.metricsPath != "/custom-metrics" {
		t.Errorf("Expected metrics path '/custom-metrics', got '%s'", r.metrics.metricsPath)
	}

	if !r.metrics.excludePaths["/health"] {
		t.Error("Expected /health to be in exclude paths")
	}

	if r.metrics.recordParams {
		t.Error("Expected params recording to be disabled")
	}

	if len(r.metrics.recordHeaders) != 1 || r.metrics.recordHeaders[0] != "Authorization" {
		t.Error("Expected Authorization header to be recorded")
	}

	if r.metrics.exportInterval != 30*time.Second {
		t.Errorf("Expected 30s export interval, got %v", r.metrics.exportInterval)
	}
}

func TestProviderSwitchingStopsServer(t *testing.T) {
	// Start with Prometheus (should start server)
	r := New(
		WithMetrics(),
		WithMetricsServiceName("test-service"),
	)

	if r.GetMetricsServerAddress() == "" {
		t.Error("Expected metrics server to be started with Prometheus")
	}

	// Switch to OTLP (should stop server)
	WithMetricsProviderOTLP()(r)

	if r.GetMetricsServerAddress() != "" {
		t.Error("Expected metrics server to be stopped when switching to OTLP")
	}

	if r.metrics.provider != OTLPProvider {
		t.Errorf("Expected OTLP provider after switch, got %s", r.metrics.provider)
	}
}

func TestMetricsWithTracing(t *testing.T) {
	// Test that both tracing and metrics work together
	r := New(
		WithTracing(),
		WithMetrics(),
		WithTracingServiceName("test-service"),
		WithMetricsServiceName("test-service"),
	)
	defer r.StopMetricsServer()

	r.GET("/test/:id", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{
			"id": c.Param("id"),
		})
	})

	req := httptest.NewRequest("GET", "/test/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check that both are configured
	if r.tracing == nil {
		t.Error("Expected tracing to be configured")
	}

	if r.metrics == nil {
		t.Error("Expected metrics to be configured")
	}
}

func TestMetricsWithGroups(t *testing.T) {
	r := New(WithMetrics(), WithMetricsPort(":9093"))
	defer r.StopMetricsServer()

	api := r.Group("/api/v1")
	api.GET("/users/:id", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{
			"id": c.Param("id"),
		})
	})

	req := httptest.NewRequest("GET", "/api/v1/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestMetricsExcludePaths(t *testing.T) {
	r := New(
		WithMetrics(),
		WithMetricsExcludePaths("/health", "/metrics"),
	)
	defer r.StopMetricsServer()

	r.GET("/health", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})

	r.GET("/api/users", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "users"})
	})

	// Test excluded path
	req1 := httptest.NewRequest("GET", "/health", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("Expected status 200 for /health, got %d", w1.Code)
	}

	// Test non-excluded path
	req2 := httptest.NewRequest("GET", "/api/users", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 for /api/users, got %d", w2.Code)
	}

	// Check exclude paths configuration
	if !r.metrics.excludePaths["/health"] {
		t.Error("Expected /health to be excluded")
	}

	if !r.metrics.excludePaths["/metrics"] {
		t.Error("Expected /metrics to be excluded")
	}
}

func TestMetricsErrorCounting(t *testing.T) {
	r := New(WithMetrics(), WithMetricsPort(":9094"))
	defer r.StopMetricsServer()

	r.GET("/error", func(c *Context) {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "test error"})
	})

	r.GET("/success", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Test error response
	req1 := httptest.NewRequest("GET", "/error", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for /error, got %d", w1.Code)
	}

	// Test success response
	req2 := httptest.NewRequest("GET", "/success", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 for /success, got %d", w2.Code)
	}
}

func TestMetricsEnvironmentProviderOverride(t *testing.T) {
	// Set environment to Prometheus
	oldExporter := os.Getenv("OTEL_METRICS_EXPORTER")
	os.Setenv("OTEL_METRICS_EXPORTER", "prometheus")
	defer os.Setenv("OTEL_METRICS_EXPORTER", oldExporter)

	r := New(
		WithMetrics(), // Would read env (prometheus)
		WithMetricsProviderOTLP("http://override:4318"), // But override to OTLP
	)

	// Should use the explicit override, not environment
	if r.metrics.provider != OTLPProvider {
		t.Errorf("Expected OTLP provider (override), got %s", r.metrics.provider)
	}
}

func TestGetMetricsProvider(t *testing.T) {
	r1 := New(WithMetrics(), WithMetricsPort(":9095"))
	defer r1.StopMetricsServer()

	if r1.GetMetricsProvider() != PrometheusProvider {
		t.Errorf("Expected Prometheus provider, got %s", r1.GetMetricsProvider())
	}

	r2 := New(
		WithMetrics(),
		WithMetricsProviderOTLP(),
	)

	if r2.GetMetricsProvider() != OTLPProvider {
		t.Errorf("Expected OTLP provider, got %s", r2.GetMetricsProvider())
	}

	r3 := New()

	if r3.GetMetricsProvider() != "" {
		t.Errorf("Expected empty provider when metrics disabled, got %s", r3.GetMetricsProvider())
	}
}
