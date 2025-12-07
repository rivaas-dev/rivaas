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

// Package main demonstrates a full-featured example of the Rivaas router with advanced features.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"example.com/full-featured/handlers"
	"go.opentelemetry.io/otel/attribute"

	"rivaas.dev/app"
	"rivaas.dev/logging"
	"rivaas.dev/metrics"
	"rivaas.dev/openapi"
	"rivaas.dev/router"
	"rivaas.dev/router/middleware/cors"
	"rivaas.dev/router/middleware/requestid"
	"rivaas.dev/router/middleware/timeout"
	"rivaas.dev/router/version"
	"rivaas.dev/tracing"
)

func main() {
	// Get environment configuration
	environment := getEnv("ENVIRONMENT", "development")
	serviceName := getEnv("SERVICE_NAME", "full-featured-api")
	serviceVersion := getEnv("SERVICE_VERSION", "v1.0.0")

	// Set up tracing options based on environment
	var tracingOpts []tracing.Option
	switch environment {
	case "production":
		otlpEndpoint := getEnv("OTLP_ENDPOINT", "localhost:4317")
		tracingOpts = append(tracingOpts, tracing.WithOTLP(otlpEndpoint))
	default:
		tracingOpts = append(tracingOpts, tracing.WithStdout())
	}

	// Configure metrics port
	metricsPort := getEnv("METRICS_PORT", ":9090")

	a, err := app.New(
		app.WithServiceName(serviceName),
		app.WithServiceVersion(serviceVersion),
		app.WithEnvironment(environment),
		// Configure router with path-based versioning
		app.WithRouterOptions(
			router.WithVersioning(
				version.WithPathDetection("/v{version}/"), // Path-based versioning: /api/v1/, /api/v2/, etc.
				version.WithDefault("v1"),
				version.WithValidVersions("v1", "v2"), // Optional: validate versions
			),
		),
		// Configure observability (logging, metrics, tracing)
		// All three pillars use the same consistent pattern: pass options directly
		// Service name/version are automatically injected from app config
		app.WithObservability(
			// Logging - service name/version auto-injected
			app.WithLogging(
				logging.WithConsoleHandler(),
				// logging.WithDebugLevel(),
			),
			// Metrics - service name/version auto-injected
			// Prometheus is default; use metrics.WithOTLP() for OTLP
			app.WithMetrics(
				metrics.WithPrometheus(metricsPort, "/metrics"),
			),
			// Tracing - service name/version auto-injected
			app.WithTracing(
				append(tracingOpts, tracing.WithSampleRate(getSampleRate(environment)))...,
			),
			// Shared exclusions (apply to all observability components)
			app.WithExcludePaths("/healthz", "/readyz", "/metrics"),
		),
		// Health endpoints - consistent functional options pattern
		// Endpoints: GET /healthz (liveness), GET /readyz (readiness)
		app.WithHealthEndpoints(
			app.WithHealthTimeout(800*time.Millisecond),
			app.WithLivenessCheck("process", func(ctx context.Context) error {
				// Simple liveness check - process is alive
				return nil
			}),
			// In production, you'd add real dependency checks:
			// app.WithReadinessCheck("database", func(ctx context.Context) error {
			//     return db.PingContext(ctx)
			// }),
			// app.WithReadinessCheck("cache", func(ctx context.Context) error {
			//     return redis.Ping(ctx).Err()
			// }),
		),
		// Debug endpoints - enable pprof conditionally
		// WARNING: Only enable in development or behind authentication
		// app.WithDebugEndpoints(
		// 	app.WithPprofIf(environment == "development" || os.Getenv("PPROF_ENABLED") == "true"),
		// ),
		// Server config
		app.WithServerConfig(
			app.WithReadTimeout(15*time.Second),
			app.WithWriteTimeout(15*time.Second),
			app.WithShutdownTimeout(30*time.Second),
		),
		app.WithOpenAPI(
			openapi.WithTitle(serviceName, serviceVersion),
			openapi.WithDescription("API description"),
			openapi.WithBearerAuth("bearerAuth", "JWT authentication"),
			openapi.WithServer("http://localhost:8080", "Local development"),
			openapi.WithSwaggerUI(true, "/docs"),
			openapi.WithUIDocExpansion(openapi.DocExpansionList),
			openapi.WithUIRequestSnippets(true, openapi.SnippetCurlBash, openapi.SnippetCurlPowerShell, openapi.SnippetCurlCmd),
		),
	)
	if err != nil {
		log.Fatalf("Failed to create app: %v", err)
	}

	a.Router().Use(requestid.New())
	a.Router().Use(cors.New(cors.WithAllowAllOrigins(true)))
	a.Router().Use(timeout.New(30 * time.Second))

	// Root endpoint
	a.GET("/", func(c *app.Context) {
		if err := c.JSON(http.StatusOK, map[string]any{
			"message":     "Full Featured API",
			"service":     serviceName,
			"version":     serviceVersion,
			"environment": environment,
			"trace_id":    c.TraceID(),
			"span_id":     c.SpanID(),
			"request_id":  c.Response.Header().Get("X-Request-ID"),
		}); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	// User endpoints with typed constraints and OpenAPI documentation
	// The unified API allows chaining both constraints and OpenAPI docs
	a.GET("/users/:id", handlers.GetUserByID).
		WhereInt("id").
		Doc("Get user", "Retrieves a user by ID").
		Response(http.StatusOK, handlers.UserResponse{}).
		Response(http.StatusNotFound, handlers.APIError{}).
		Tags("users")

	a.POST("/users", handlers.CreateUser).
		Doc("Create user", "Creates a new user").
		Request(handlers.CreateUserRequest{}).
		Response(http.StatusCreated, handlers.UserResponse{}).
		Tags("users")

	a.GET("/users/:id/orders", handlers.GetUserOrders).
		WhereInt("id").
		Doc("Get user orders", "Retrieves all orders for a user").
		Response(http.StatusOK, []handlers.OrderResponse{}).
		Tags("users", "orders")

	// Order endpoints with typed constraints
	a.POST("/orders", handlers.CreateOrder).
		Doc("Create order", "Creates a new order").
		Request(handlers.CreateOrderRequest{}).
		Response(http.StatusCreated, handlers.OrderResponse{}).
		Tags("orders")

	a.GET("/orders/:id", handlers.GetOrderByID).
		WhereInt("id").
		Doc("Get order", "Retrieves an order by ID").
		Response(http.StatusOK, handlers.OrderResponse{}).
		Response(http.StatusNotFound, handlers.APIError{}).
		Tags("orders")

	// Error handling example - MOVED BEFORE VERSIONED ROUTES
	a.GET("/error", func(c *app.Context) {
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

	// API v1 routes using app versioning feature
	// Routes are registered without version prefix - router handles version detection from path
	// Pattern "/v{version}/" means paths like "/v1/status" will be routed to "/status" in v1 tree
	v1 := a.Version("v1")

	// Simple test route first - IMPORTANT: Routes are registered WITHOUT /v1 prefix
	// The router automatically detects version from URL path like /v1/test
	// Now using proper app.Version() which provides full app.Context support
	v1.GET("/test", func(c *app.Context) {
		if err := c.JSON(http.StatusOK, map[string]any{
			"message": "v1 test route works",
			"version": c.Version(),
			"path":    c.Request.URL.Path,
		}); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	v1.GET("/status", func(c *app.Context) {
		if err := c.JSON(http.StatusOK, map[string]any{
			"status":      "operational",
			"environment": environment,
			"metrics":     a.GetMetricsServerAddress(),
			"version":     c.Version(), // Returns "v1" from router context
			"api_version": "v1",
		}); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	// Products with typed constraints - showcasing the unified API
	v1.GET("/products/:id", handlers.GetProductByID).
		WhereRegex("id", `[a-zA-Z0-9-]+`). // Custom pattern for product IDs
		Doc("Get product", "Retrieves a product by ID").
		Response(http.StatusOK, map[string]any{
			"id":    "string",
			"name":  "string",
			"price": 0.0,
		}).
		Tags("products")

	// Search endpoint with advanced query binding and OpenAPI docs
	v1.GET("/search", handlers.Search).
		Doc("Search", "Search endpoint with query parameters").
		Request(handlers.SearchParams{}).
		Deprecated().
		Response(http.StatusOK, map[string]any{
			"query":     "string",
			"page":      0,
			"page_size": 0,
			"results":   []string{},
			"total":     0,
		}).
		Tags("search")

	// Example: You can easily add v2 when ready
	// v2 := a.Version("v2")
	// v2.GET("/status", func(c *app.Context) {
	// 	c.JSON(http.StatusOK, map[string]any{
	// 		"status":      "operational",
	// 		"environment": environment,
	// 		"version":     c.Version(), // Returns "v2"
	// 		"api_version": "v2",
	// 		"enhanced":    true,
	// 	})
	// })

	// Compile routes for correct route tracking
	// This ensures versioned routes show correct route templates in access logs
	a.Router().Warmup()

	// Start server with graceful shutdown
	// Health endpoints: GET /healthz, GET /readyz
	// Debug endpoints (in development): GET /debug/pprof/*
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
