package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/rivaas-dev/rivaas/router"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	// Initialize OpenTelemetry
	initTracer()

	// Create router with tracing enabled
	r := router.New(
		router.WithTracing(),
		router.WithTracingServiceName("example-api"),
		router.WithTracingServiceVersion("v1.0.0"),
		router.WithTracingExcludePaths("/health", "/metrics"),
		router.WithTracingHeaders("Authorization", "X-Request-ID"),
		router.WithTracingSampleRate(1.0), // Sample all requests for demo
	)

	// Routes
	r.GET("/", func(c *router.Context) {
		// Add custom span attributes
		c.SetSpanAttribute("custom.field", "home-page")
		c.AddSpanEvent("processing_request")

		c.JSON(http.StatusOK, map[string]interface{}{
			"message":  "Welcome to Rivaas with Tracing!",
			"trace_id": c.TraceID(),
			"span_id":  c.SpanID(),
		})
	})

	r.GET("/users/:id", func(c *router.Context) {
		userID := c.Param("id")

		// Add span attributes
		c.SetSpanAttribute("user.id", userID)
		c.AddSpanEvent("user_lookup_started")

		// Simulate some work
		time.Sleep(10 * time.Millisecond)

		c.AddSpanEvent("user_lookup_completed")

		c.JSON(http.StatusOK, map[string]interface{}{
			"user_id":  userID,
			"trace_id": c.TraceID(),
			"span_id":  c.SpanID(),
		})
	})

	// Health endpoint (excluded from tracing)
	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
		})
	})

	// API group with middleware
	api := r.Group("/api/v1")
	api.Use(TracingMiddleware())

	api.GET("/orders/:order_id", func(c *router.Context) {
		orderID := c.Param("order_id")

		c.SetSpanAttribute("order.id", orderID)
		c.AddSpanEvent("order_processing")

		// Simulate some processing
		time.Sleep(5 * time.Millisecond)

		c.JSON(http.StatusOK, map[string]interface{}{
			"order_id": orderID,
			"status":   "processing",
			"trace_id": c.TraceID(),
		})
	})

	log.Println("Server starting on :8080")
	log.Println("Try these endpoints:")
	log.Println("  GET http://localhost:8080/")
	log.Println("  GET http://localhost:8080/users/123")
	log.Println("  GET http://localhost:8080/api/v1/orders/456")
	log.Println("  GET http://localhost:8080/health (no tracing)")

	log.Fatal(http.ListenAndServe(":8080", r))
}

// TracingMiddleware demonstrates adding custom tracing in middleware
func TracingMiddleware() router.HandlerFunc {
	return func(c *router.Context) {
		start := time.Now()

		// Add middleware span event
		c.AddSpanEvent("middleware_start")
		c.SetSpanAttribute("middleware.name", "TracingMiddleware")

		c.Next()

		// Add timing information
		duration := time.Since(start)
		c.SetSpanAttribute("middleware.duration_ms", duration.Milliseconds())
		c.AddSpanEvent("middleware_end")
	}
}

// initTracer initializes the OpenTelemetry tracer
func initTracer() {
	// Create a stdout exporter for demo purposes
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Fatal(err)
	}

	// Create a trace provider
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithSampler(trace.AlwaysSample()),
	)

	// Set the global trace provider
	otel.SetTracerProvider(tp)

	// Graceful shutdown
	go func() {
		time.Sleep(30 * time.Second) // For demo, shut down after 30 seconds
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
		os.Exit(0)
	}()
}
