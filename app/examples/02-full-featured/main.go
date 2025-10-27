package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rivaas-dev/rivaas/app"
	"github.com/rivaas-dev/rivaas/metrics"
	"github.com/rivaas-dev/rivaas/router"
	"github.com/rivaas-dev/rivaas/router/middleware"
	"github.com/rivaas-dev/rivaas/tracing"
	"go.opentelemetry.io/otel/attribute"
)

func main() {
	// ========================================
	// Step 1: Configure Observability
	// ========================================

	// Get environment configuration
	environment := getEnv("ENVIRONMENT", "development")
	serviceName := getEnv("SERVICE_NAME", "full-featured-api")
	serviceVersion := getEnv("SERVICE_VERSION", "v1.0.0")

	// Set up tracing provider based on environment
	var tracingProvider tracing.TracingProvider
	var otlpEndpoint string

	switch environment {
	case "production":
		tracingProvider = tracing.OTLPProvider
		otlpEndpoint = getEnv("OTLP_ENDPOINT", "localhost:4317")
		log.Printf("📤 Using OTLP provider: %s", otlpEndpoint)
	case "development":
		tracingProvider = tracing.StdoutProvider
		log.Println("📝 Using stdout provider (traces in terminal)")
	default:
		tracingProvider = tracing.NoopProvider
		log.Println("🚫 Tracing disabled (noop provider)")
	}

	// ========================================
	// Step 2: Create App with Full Observability
	// ========================================

	// Configure metrics provider
	var metricsProvider metrics.MetricsProvider
	metricsPort := getEnv("METRICS_PORT", ":9090")

	switch environment {
	case "production":
		// In production, you might use OTLP for both metrics and tracing
		metricsProvider = metrics.PrometheusProvider
	default:
		metricsProvider = metrics.PrometheusProvider
	}

	a, err := app.New(
		app.WithServiceName(serviceName),
		app.WithVersion(serviceVersion),
		app.WithEnvironment(environment),
		// Configure metrics
		app.WithMetrics(
			metrics.WithProvider(metricsProvider),
			metrics.WithPort(metricsPort),
		),
		// Configure tracing
		app.WithTracing(
			tracing.WithServiceName(serviceName),
			tracing.WithServiceVersion(serviceVersion),
			tracing.WithProvider(tracingProvider),
			tracing.WithOTLPEndpoint(otlpEndpoint),
			tracing.WithOTLPInsecure(environment != "production"),
			tracing.WithSampleRate(getSampleRate(environment)),
			tracing.WithExcludePaths("/health", "/metrics"),
		),
		// Server config
		app.WithServerConfig(&app.ServerConfig{
			ReadTimeout:     15 * time.Second,
			WriteTimeout:    15 * time.Second,
			ShutdownTimeout: 30 * time.Second,
		}),
		// Middleware config
		app.WithMiddleware(
			environment == "development", // includeLogger
			true,                         // includeRecovery
		),
	)
	if err != nil {
		log.Fatalf("Failed to create app: %v", err)
	}

	// ========================================
	// Step 3: Add Custom Middleware
	// ========================================

	a.Use(middleware.RequestID())
	a.Use(middleware.CORS(middleware.WithAllowAllOrigins(true)))
	a.Use(middleware.Timeout(30 * time.Second))

	// ========================================
	// Step 4: Register Routes
	// ========================================

	// Root endpoint
	a.GET("/", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"message":     "Full Featured API",
			"service":     serviceName,
			"version":     serviceVersion,
			"environment": environment,
			"trace_id":    c.TraceID(),
			"span_id":     c.SpanID(),
			"request_id":  c.Response.Header().Get("X-Request-ID"),
		})
	})

	// Health check
	a.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"service":   serviceName,
		})
	})

	// User endpoints with full observability
	a.GET("/users/:id", func(c *router.Context) {
		userID := c.Param("id")

		// Add tracing attributes
		c.SetSpanAttribute("user.id", userID)
		c.SetSpanAttribute("operation.type", "read")
		c.AddSpanEvent("user_lookup_started")

		// Simulate database lookup
		time.Sleep(10 * time.Millisecond)

		// Record custom metrics
		c.IncrementCounter("user_lookups_total",
			attribute.String("user_id", userID),
			attribute.String("result", "success"),
		)

		c.AddSpanEvent("user_found")
		c.JSON(http.StatusOK, map[string]any{
			"user_id":    userID,
			"name":       "John Doe",
			"email":      "john@example.com",
			"trace_id":   c.TraceID(),
			"span_id":    c.SpanID(),
			"request_id": c.Response.Header().Get("X-Request-ID"),
		})
	})

	// Create order endpoint
	a.POST("/orders", func(c *router.Context) {
		// Add tracing
		c.SetSpanAttribute("operation", "create_order")
		c.AddSpanEvent("order_creation_started")

		// Simulate order processing with timing metrics
		start := time.Now()
		time.Sleep(50 * time.Millisecond)
		processingTime := time.Since(start).Seconds()

		// Record custom metrics
		c.RecordMetric("order_processing_duration_seconds", processingTime,
			attribute.String("currency", "USD"),
			attribute.String("payment_method", "card"),
		)

		c.IncrementCounter("orders_total",
			attribute.String("status", "success"),
			attribute.String("type", "online"),
		)

		c.AddSpanEvent("order_created")
		c.JSON(http.StatusCreated, map[string]any{
			"order_id":        "ord_" + generateID(),
			"status":          "created",
			"processing_time": processingTime,
			"trace_id":        c.TraceID(),
			"span_id":         c.SpanID(),
			"request_id":      c.Response.Header().Get("X-Request-ID"),
		})
	})

	// API v1 group
	apiV1 := a.Group("/api/v1")

	apiV1.GET("/status", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"status":      "operational",
			"environment": environment,
			"metrics":     a.GetMetricsServerAddress(),
			"version":     serviceVersion,
		})
	})

	apiV1.GET("/products", func(c *router.Context) {
		c.SetSpanAttribute("resource", "products")
		c.AddSpanEvent("fetching_products")

		// Simulate work
		time.Sleep(20 * time.Millisecond)

		c.JSON(http.StatusOK, map[string]any{
			"products": []map[string]any{
				{"id": "1", "name": "Product A", "price": 29.99},
				{"id": "2", "name": "Product B", "price": 49.99},
			},
			"trace_id": c.TraceID(),
		})
	})

	// Error handling example
	a.GET("/error", func(c *router.Context) {
		c.SetSpanAttribute("error.occurred", true)
		c.SetSpanAttribute("error.type", "demonstration")
		c.AddSpanEvent("error_triggered")

		c.IncrementCounter("errors_total",
			attribute.String("endpoint", "/error"),
			attribute.String("type", "simulated"),
		)

		c.JSON(http.StatusInternalServerError, map[string]any{
			"error":      "This is a simulated error for testing",
			"trace_id":   c.TraceID(),
			"span_id":    c.SpanID(),
			"request_id": c.Response.Header().Get("X-Request-ID"),
		})
	})

	// ========================================
	// Step 5: Start Server
	// ========================================

	port := getEnv("PORT", ":8080")

	log.Println("=" + strings.Repeat("=", 60))
	log.Printf("🚀 %s starting", serviceName)
	log.Println("=" + strings.Repeat("=", 60))
	log.Printf("📍 Environment: %s", environment)
	log.Printf("📍 Version: %s", serviceVersion)
	log.Printf("📍 Server: http://localhost%s", port)
	log.Printf("📊 Metrics: http://localhost%s/metrics", metricsPort)
	log.Printf("🔍 Tracing: %s provider", tracingProvider)
	if tracingProvider == tracing.OTLPProvider {
		log.Printf("   → Endpoint: %s", otlpEndpoint)
	}
	log.Println("=" + strings.Repeat("=", 60))

	// Start server with graceful shutdown
	if err := a.Run(port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getSampleRate(environment string) float64 {
	if environment == "production" {
		return 0.1 // 10% sampling in production
	}
	return 1.0 // 100% sampling in development
}

func generateID() string {
	return time.Now().Format("20060102150405")
}
