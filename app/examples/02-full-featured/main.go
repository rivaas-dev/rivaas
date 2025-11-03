package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"rivaas.dev/app"
	"rivaas.dev/metrics"
	"rivaas.dev/router"
	"rivaas.dev/router/middleware"
	"rivaas.dev/tracing"

	"example.com/full-featured/handlers"
)

func main() {
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
	default:
		tracingProvider = tracing.StdoutProvider
	}

	// Configure metrics provider
	var metricsProvider metrics.MetricsProvider
	metricsPort := getEnv("METRICS_PORT", ":9090")

	switch environment {
	case "production":
		metricsProvider = metrics.PrometheusProvider
	default:
		metricsProvider = metrics.PrometheusProvider
	}

	a, err := app.New(
		app.WithServiceName(serviceName),
		app.WithServiceVersion(serviceVersion),
		app.WithEnvironment(environment),
		// Configure router with path-based versioning
		app.WithRouterOptions(
			router.WithVersioning(
				router.WithPathVersioning("/v{version}/"), // Path-based versioning: /api/v1/, /api/v2/, etc.
				router.WithDefaultVersion("v1"),
				router.WithValidVersions("v1", "v2"), // Optional: validate versions
			),
		),
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
		app.WithServerConfig(
			app.WithReadTimeout(15*time.Second),
			app.WithWriteTimeout(15*time.Second),
			app.WithShutdownTimeout(30*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("Failed to create app: %v", err)
	}

	a.Use(middleware.RequestID())
	a.Use(middleware.CORS(middleware.WithAllowAllOrigins(true)))
	a.Use(middleware.Timeout(30 * time.Second))

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

	// User endpoints with route constraints and proper binding
	a.Router().GET("/users/:id", handlers.GetUserByID).WhereNumber("id")
	a.POST("/users", handlers.CreateUser)
	a.Router().GET("/users/:id/orders", handlers.GetUserOrders).WhereNumber("id")

	// Order endpoints
	a.POST("/orders", handlers.CreateOrder)
	a.Router().GET("/orders/:id", handlers.GetOrderByID).WhereNumber("id")

	// Error handling example - MOVED BEFORE VERSIONED ROUTES
	a.GET("/error", func(c *router.Context) {
		c.SetSpanAttribute("error.occurred", true)
		c.SetSpanAttribute("error.type", "demonstration")
		c.AddSpanEvent("error_triggered")

		c.IncrementCounter("errors_total",
			attribute.String("endpoint", "/error"),
			attribute.String("type", "simulated"),
		)

		handlers.HandleError(c, handlers.APIError{
			Code:    "DEMONSTRATION_ERROR",
			Message: "This is a simulated error for testing",
			Details: "This endpoint demonstrates error handling patterns",
		})
	})

	// API v1 routes using router versioning feature
	// Routes are registered without version prefix - router handles version detection from path
	// Pattern "/v{version}/" means paths like "/v1/status" will be routed to "/status" in v1 tree
	v1 := a.Router().Version("v1")

	// Simple test route first - IMPORTANT: Routes are registered WITHOUT /v1 prefix
	// The router automatically detects version from URL path like /v1/test
	v1.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"message": "v1 test route works",
			"version": c.Version(),
			"path":    c.Request.URL.Path,
		})
	})

	v1.GET("/status", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"status":      "operational",
			"environment": environment,
			"metrics":     a.GetMetricsServerAddress(),
			"version":     c.Version(), // Returns "v1" from router context
			"api_version": "v1",
		})
	})

	// Products with query binding
	v1.GET("/products", handlers.ListProducts)
	v1.GET("/products/:id", handlers.GetProductByID).Where("id", `[a-zA-Z0-9-]+`) // Alphanumeric + dashes

	// Search endpoint with advanced query binding
	v1.GET("/search", handlers.Search)

	// Example: You can easily add v2 when ready
	// v2 := a.Router().Version("v2")
	// v2.GET("/status", func(c *router.Context) {
	// 	c.JSON(http.StatusOK, map[string]any{
	// 		"status":      "operational",
	// 		"environment": environment,
	// 		"version":     c.Version(), // Returns "v2"
	// 		"api_version": "v2",
	// 		"enhanced":    true,
	// 	})
	// })

	// Start server with graceful shutdown
	if err := a.Run(":8181"); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

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
