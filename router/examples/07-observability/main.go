package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/rivaas-dev/rivaas/router"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	// Initialize tracing
	shutdown := initTracer()
	defer shutdown()

	// Create router with both tracing and metrics
	r := router.New(
		// Enable tracing
		router.WithTracing(),
		router.WithTracingServiceName("observability-demo"),
		router.WithTracingServiceVersion("v1.0.0"),

		// Enable metrics
		router.WithMetrics(),
		router.WithMetricsServiceName("observability-demo"),
		router.WithMetricsServiceVersion("v1.0.0"),

		// Shared configuration
		router.WithTracingExcludePaths("/health"),
		router.WithMetricsExcludePaths("/health"),
	)

	// Home endpoint with full observability
	r.GET("/", func(c *router.Context) {
		c.SetSpanAttribute("page", "home")
		c.AddSpanEvent("home_request_started")

		c.IncrementCounter("page_views_total",
			attribute.String("page", "home"),
		)

		c.JSON(http.StatusOK, map[string]interface{}{
			"message":        "Full Observability Demo",
			"trace_id":       c.TraceID(),
			"span_id":        c.SpanID(),
			"metrics_server": r.GetMetricsServerAddress(),
		})
	})

	// User lookup with correlated tracing and metrics
	r.GET("/users/:id", func(c *router.Context) {
		userID := c.Param("id")

		// Tracing
		c.SetSpanAttribute("user.id", userID)
		c.AddSpanEvent("user_lookup_started")

		// Metrics
		c.IncrementCounter("user_lookups_total",
			attribute.String("operation", "get_user"),
		)

		// Simulate database query with timing
		start := time.Now()
		time.Sleep(25 * time.Millisecond)
		queryDuration := time.Since(start).Seconds()

		c.AddSpanEvent("database_query_completed")
		c.RecordMetric("database_query_duration_seconds", queryDuration,
			attribute.String("table", "users"),
			attribute.String("operation", "select"),
		)

		c.JSON(http.StatusOK, map[string]interface{}{
			"user_id":  userID,
			"name":     "John Doe",
			"trace_id": c.TraceID(), // Use this to correlate metrics with traces
		})
	})

	// Order processing with business metrics and tracing
	r.POST("/orders", func(c *router.Context) {
		c.SetSpanAttribute("operation", "create_order")
		c.AddSpanEvent("order_processing_started")

		// Simulate order processing stages
		stages := []struct {
			name     string
			duration time.Duration
		}{
			{"validation", 10 * time.Millisecond},
			{"payment", 50 * time.Millisecond},
			{"inventory", 30 * time.Millisecond},
		}

		for _, stage := range stages {
			start := time.Now()
			time.Sleep(stage.duration)

			// Record both trace event and metric
			c.AddSpanEvent(stage.name + "_completed")
			c.RecordMetric("order_stage_duration_seconds", time.Since(start).Seconds(),
				attribute.String("stage", stage.name),
			)
		}

		// Business metrics
		c.IncrementCounter("orders_total",
			attribute.String("status", "success"),
		)

		orderValue := 99.99
		c.RecordMetric("order_value_dollars", orderValue,
			attribute.String("currency", "USD"),
		)

		orderID := "ord_" + time.Now().Format("20060102150405")
		c.SetSpanAttribute("order.id", orderID)
		c.SetSpanAttribute("order.value", orderValue)

		c.JSON(http.StatusCreated, map[string]interface{}{
			"order_id": orderID,
			"value":    orderValue,
			"trace_id": c.TraceID(),
		})
	})

	// Error handling with observability
	r.GET("/error", func(c *router.Context) {
		c.SetSpanAttribute("error.occurred", true)
		c.AddSpanEvent("error_triggered")

		// Error metrics are automatically recorded by the router
		c.IncrementCounter("application_errors_total",
			attribute.String("type", "simulated"),
		)

		c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":    "Simulated error for observability testing",
			"trace_id": c.TraceID(), // Include trace ID in error response
		})
	})

	// Performance monitoring endpoint
	r.GET("/slow", func(c *router.Context) {
		c.SetSpanAttribute("operation", "slow_query")
		c.AddSpanEvent("slow_operation_started")

		start := time.Now()
		time.Sleep(200 * time.Millisecond)
		duration := time.Since(start)

		c.SetSpanAttribute("duration_ms", duration.Milliseconds())
		c.RecordMetric("slow_operations_duration_seconds", duration.Seconds(),
			attribute.String("operation", "slow_query"),
		)

		c.JSON(http.StatusOK, map[string]interface{}{
			"duration_ms": duration.Milliseconds(),
			"trace_id":    c.TraceID(),
		})
	})

	// Health check (excluded from both tracing and metrics)
	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
		})
	})

	log.Println("🚀 Server with full observability on http://localhost:8080")
	log.Println("🔍 Tracing: Enabled (stdout)")
	log.Printf("📊 Metrics: %s/metrics\n", r.GetMetricsServerAddress())
	log.Println("\n📝 Try these commands:")
	log.Println("   curl http://localhost:8080/")
	log.Println("   curl http://localhost:8080/users/123")
	log.Println("   curl -X POST http://localhost:8080/orders")
	log.Println("   curl http://localhost:8080/error")
	log.Println("   curl http://localhost:8080/slow")
	log.Println("\n📈 View metrics:")
	log.Println("   curl http://localhost:9090/metrics")
	log.Println("\n💡 Tip: Use trace_id from responses to correlate metrics with traces")

	defer r.StopMetricsServer()

	log.Fatal(http.ListenAndServe(":8080", r))
}

func initTracer() func() {
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Fatalf("Failed to create stdout exporter: %v", err)
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithSampler(trace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
		os.Exit(0)
	}
}
