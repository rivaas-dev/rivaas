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
	// Initialize OpenTelemetry tracer
	shutdown := initTracer()
	defer shutdown()

	// Create router with tracing enabled
	r := router.New(
		router.WithTracing(),
		router.WithTracingServiceName("tracing-example"),
		router.WithTracingServiceVersion("v1.0.0"),
		router.WithTracingExcludePaths("/health"),
		router.WithTracingHeaders("Authorization", "X-Request-ID"),
		router.WithTracingSampleRate(1.0), // Sample 100% of requests for demo
	)

	// Basic route with tracing
	r.GET("/", func(c *router.Context) {
		c.SetSpanAttribute("page", "home")
		c.AddSpanEvent("processing_home_request")

		c.JSON(http.StatusOK, map[string]interface{}{
			"message":  "Welcome to Tracing Example!",
			"trace_id": c.TraceID(),
			"span_id":  c.SpanID(),
		})
	})

	// Route with custom span attributes
	r.GET("/users/:id", func(c *router.Context) {
		userID := c.Param("id")

		// Add span attributes
		c.SetSpanAttribute("user.id", userID)
		c.SetSpanAttribute("operation", "get_user")
		c.AddSpanEvent("user_lookup_started")

		// Simulate database query
		time.Sleep(20 * time.Millisecond)
		c.AddSpanEvent("database_query_completed")

		// Simulate cache check
		time.Sleep(5 * time.Millisecond)
		c.AddSpanEvent("cache_check_completed")

		c.JSON(http.StatusOK, map[string]interface{}{
			"user_id":  userID,
			"name":     "John Doe",
			"trace_id": c.TraceID(),
			"span_id":  c.SpanID(),
		})
	})

	// Order processing with detailed tracing
	r.POST("/orders", func(c *router.Context) {
		c.SetSpanAttribute("operation", "create_order")
		c.AddSpanEvent("order_creation_started")

		// Validate order
		time.Sleep(10 * time.Millisecond)
		c.AddSpanEvent("order_validated")

		// Process payment
		time.Sleep(50 * time.Millisecond)
		c.SetSpanAttribute("payment.method", "credit_card")
		c.AddSpanEvent("payment_processed")

		// Update inventory
		time.Sleep(30 * time.Millisecond)
		c.AddSpanEvent("inventory_updated")

		orderID := "ord_" + time.Now().Format("20060102150405")
		c.SetSpanAttribute("order.id", orderID)
		c.AddSpanEvent("order_completed")

		c.JSON(http.StatusCreated, map[string]interface{}{
			"order_id": orderID,
			"status":   "created",
			"trace_id": c.TraceID(),
		})
	})

	// Error handling with tracing
	r.GET("/error/:type", func(c *router.Context) {
		errorType := c.Param("type")

		c.SetSpanAttribute("error.type", errorType)
		c.SetSpanAttribute("error.occurred", true)
		c.AddSpanEvent("error_triggered")

		switch errorType {
		case "404":
			c.JSON(http.StatusNotFound, map[string]string{
				"error":    "Resource not found",
				"trace_id": c.TraceID(),
			})
		case "500":
			c.JSON(http.StatusInternalServerError, map[string]string{
				"error":    "Internal server error",
				"trace_id": c.TraceID(),
			})
		default:
			c.JSON(http.StatusBadRequest, map[string]string{
				"error":    "Bad request",
				"trace_id": c.TraceID(),
			})
		}
	})

	// Health check (excluded from tracing)
	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
		})
	})

	log.Println("🚀 Server starting on http://localhost:8080")
	log.Println("🔍 Tracing enabled with stdout exporter")
	log.Println("\n📝 Try these commands:")
	log.Println("   curl http://localhost:8080/")
	log.Println("   curl http://localhost:8080/users/123")
	log.Println("   curl -X POST http://localhost:8080/orders")
	log.Println("   curl http://localhost:8080/error/404")
	log.Println("\n📋 Watch the console for trace output!")

	log.Fatal(http.ListenAndServe(":8080", r))
}

// initTracer initializes OpenTelemetry tracer with stdout exporter
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
