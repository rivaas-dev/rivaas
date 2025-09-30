package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/rivaas-dev/rivaas/router"
	"go.opentelemetry.io/otel/attribute"
)

func main() {
	// Read provider from environment or use default
	provider := os.Getenv("METRICS_PROVIDER") // "prometheus", "otlp", or "stdout"
	if provider == "" {
		provider = "prometheus"
	}

	var options []router.RouterOption

	// Configure metrics based on provider
	switch provider {
	case "prometheus":
		// Prometheus metrics (default)
		// Serves metrics on separate port :9090/metrics
		options = []router.RouterOption{
			router.WithMetrics(),
			router.WithMetricsServiceName("metrics-example"),
			router.WithMetricsServiceVersion("v1.0.0"),
			router.WithMetricsExcludePaths("/health"),
		}

	case "otlp":
		// OTLP metrics (push to collector)
		endpoint := os.Getenv("OTLP_ENDPOINT")
		if endpoint == "" {
			endpoint = "http://localhost:4318"
		}
		options = []router.RouterOption{
			router.WithMetrics(),
			router.WithMetricsProviderOTLP(endpoint),
			router.WithMetricsServiceName("metrics-example"),
			router.WithMetricsServiceVersion("v1.0.0"),
			router.WithMetricsExportInterval(10 * time.Second),
		}

	case "stdout":
		// Stdout metrics (development/debugging)
		options = []router.RouterOption{
			router.WithMetrics(),
			router.WithMetricsProviderStdout(),
			router.WithMetricsServiceName("metrics-example"),
			router.WithMetricsExportInterval(5 * time.Second),
		}

	default:
		log.Fatalf("Unknown metrics provider: %s", provider)
	}

	r := router.New(options...)

	// Basic routes with automatic metrics
	r.GET("/", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message":  "Metrics Example API",
			"provider": string(r.GetMetricsProvider()),
		})
	})

	r.GET("/users/:id", func(c *router.Context) {
		userID := c.Param("id")
		time.Sleep(10 * time.Millisecond) // Simulate work
		c.JSON(http.StatusOK, map[string]string{
			"user_id": userID,
			"name":    "John Doe",
		})
	})

	// Custom metrics examples
	r.POST("/orders", func(c *router.Context) {
		// Record custom histogram metric
		start := time.Now()
		time.Sleep(50 * time.Millisecond) // Simulate processing

		c.RecordMetric("order_processing_duration_seconds", time.Since(start).Seconds(),
			attribute.String("currency", "USD"),
			attribute.String("payment_method", "card"),
		)

		// Increment custom counter
		c.IncrementCounter("orders_total",
			attribute.String("status", "success"),
			attribute.String("type", "online"),
		)

		c.JSON(http.StatusCreated, map[string]string{
			"message":  "Order created",
			"order_id": "ord_123",
		})
	})

	// Custom gauge example
	r.GET("/status", func(c *router.Context) {
		// Set gauge metrics
		c.SetGauge("active_connections", 42,
			attribute.String("service", "api"),
		)

		c.SetGauge("memory_usage_percent", 65.5,
			attribute.String("service", "api"),
		)

		c.JSON(http.StatusOK, map[string]interface{}{
			"active_connections": 42,
			"memory_usage":       65.5,
		})
	})

	// Health check (excluded from metrics)
	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
		})
	})

	// Error endpoint to test error metrics
	r.GET("/error", func(c *router.Context) {
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Something went wrong",
		})
	})

	// Print provider-specific information
	log.Println("🚀 Server starting on http://localhost:8080")
	log.Printf("📊 Metrics Provider: %s\n", provider)

	switch provider {
	case "prometheus":
		log.Printf("📈 Metrics: %s/metrics", r.GetMetricsServerAddress())
		log.Println("\n📝 View metrics:")
		log.Println("   curl http://localhost:9090/metrics")
		log.Println("   curl http://localhost:9090/health")

	case "otlp":
		log.Printf("📤 Pushing to: %s", os.Getenv("OTLP_ENDPOINT"))
		log.Println("⏱️  Export interval: 10 seconds")
		log.Println("\n📋 Note: Ensure OTLP collector is running")

	case "stdout":
		log.Println("📄 Metrics will print to stdout every 5 seconds")
	}

	log.Println("\n🧪 Try these commands:")
	log.Println("   curl http://localhost:8080/")
	log.Println("   curl http://localhost:8080/users/123")
	log.Println("   curl -X POST http://localhost:8080/orders")
	log.Println("   curl http://localhost:8080/status")
	log.Println("   curl http://localhost:8080/error")

	log.Println("\n🔧 Change provider:")
	log.Println("   METRICS_PROVIDER=prometheus go run main.go")
	log.Println("   METRICS_PROVIDER=otlp OTLP_ENDPOINT=http://localhost:4318 go run main.go")
	log.Println("   METRICS_PROVIDER=stdout go run main.go")

	if provider == "prometheus" {
		defer r.StopMetricsServer()
	}

	log.Fatal(http.ListenAndServe(":8080", r))
}
