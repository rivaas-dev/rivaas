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

// Package app provides a batteries-included, cloud-native web framework built on top
// of the Rivaas router. It features high-performance routing, comprehensive request
// binding & validation, automatic OpenAPI generation, and OpenTelemetry-native observability,
// along with lifecycle management and sensible defaults for building production-ready web applications.
//
// # Overview
//
// The app package wraps the router package with additional features:
//
//   - Integrated observability (metrics, tracing, logging)
//   - Lifecycle hooks (OnStart, OnReady, OnShutdown, OnStop)
//   - Graceful shutdown handling
//   - Server configuration management
//   - Request binding and validation
//   - OpenAPI/Swagger documentation with ETag-based caching
//   - Health check endpoints
//   - Development and production modes
//
// # When to Use
//
// Use the app package when:
//
//   - Building a complete web application with batteries included
//   - You want integrated observability configured out of the box
//   - You need development with sensible defaults
//   - Building REST APIs with common middleware patterns
//   - You prefer convention over configuration
//
// Use the router package directly when:
//
//   - Building a library or framework that needs full control
//   - You have custom observability setup already configured
//   - You need complete flexibility without any opinions
//   - Integrating into existing systems with established patterns
//
// # Constructor Pattern
//
// The app package follows a pragmatic constructor pattern:
//
//   - New() returns (*App, error) because app initialization can fail.
//     The app initializes external resources (metrics, tracing, logging) that may fail
//     to connect to backends, validate configurations, or allocate resources.
//
//   - MustNew() is provided as a convenience wrapper that panics on error.
//     This follows the standard Go idiom (like regexp.MustCompile, template.Must)
//     and is useful for initialization in main() functions where errors should abort startup.
//
//   - All configuration options use the "With" prefix for consistency.
//
//   - Grouping options (e.g., WithServer, WithRouter) accept sub-options
//     to organize related settings and reduce API surface.
//
// # Quick Start
//
// Simple application with defaults:
//
//	app, err := app.New()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	app.GET("/", func(c *app.Context) {
//	    c.JSON(http.StatusOK, map[string]string{"message": "Hello"})
//	})
//
//	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
//	defer cancel()
//
//	if err := app.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
// Full-featured application with observability, health, and debug endpoints:
//
//	app, err := app.New(
//	    app.WithServiceName("my-api"),
//	    app.WithServiceVersion("v1.0.0"),
//	    app.WithEnvironment("production"),
//	    // Observability: all three pillars in one place
//	    app.WithObservability(
//	        app.WithLogging(logging.WithJSONHandler()),
//	        app.WithMetrics(), // Prometheus is default
//	        app.WithTracing(tracing.WithOTLP("localhost:4317")),
//	    ),
//	    // Health endpoints: /healthz and /readyz
//	    app.WithHealthEndpoints(
//	        app.WithLivenessCheck("process", func(ctx context.Context) error {
//	            return nil
//	        }),
//	        app.WithReadinessCheck("database", func(ctx context.Context) error {
//	            return db.PingContext(ctx)
//	        }),
//	    ),
//	    // Debug endpoints: /debug/pprof/* (conditionally enabled)
//	    app.WithDebugEndpoints(
//	        app.WithPprofIf(os.Getenv("PPROF_ENABLED") == "true"),
//	    ),
//	    app.WithServer(
//	        app.WithReadTimeout(15 * time.Second),
//	        app.WithWriteTimeout(15 * time.Second),
//	    ),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Or using MustNew for initialization that panics on error:
//
//	app := app.MustNew(
//	    app.WithServiceName("my-service"),
//	    app.WithObservability(
//	        app.WithMetrics(), // Prometheus is default
//	    ),
//	)
//
// # Observability
//
// The app package integrates three pillars of observability:
//
//   - Metrics: Prometheus-compatible metrics with automatic HTTP request instrumentation
//   - Tracing: OpenTelemetry tracing with request context propagation
//   - Logging: Structured logging with slog, including request-scoped fields
//
// All observability features are optional and can be enabled independently:
//
//	app.New(
//	    app.WithObservability(
//	        app.WithMetrics(), // Prometheus is default; use metrics.WithOTLP() for OTLP
//	        app.WithTracing(tracing.WithOTLP("localhost:4317")),
//	        app.WithLogging(logging.WithJSONHandler()),
//	    ),
//	)
//
// # Lifecycle Hooks
//
// The app provides lifecycle hooks for application events:
//
//   - OnStart: Called before server starts (sequential, stops on first error)
//   - OnReady: Called when server is ready to accept connections (async, non-blocking)
//   - OnShutdown: Called during graceful shutdown (LIFO order)
//   - OnStop: Called after shutdown completes (best-effort)
//
// Example:
//
//	app.OnStart(func(ctx context.Context) error {
//	    return db.Connect(ctx)
//	})
//
//	app.OnReady(func() {
//	    log.Println("Server is ready!")
//	})
//
//	app.OnShutdown(func(ctx context.Context) {
//	    db.Close()
//	})
//
// # Request Handling
//
// Handlers receive an app.Context that extends router.Context with app-level features:
//
//   - Request binding (JSON, form, query parameters)
//   - Request validation
//   - Access to observability (metrics, tracing, logging)
//
// Example:
//
//	app.POST("/users", func(c *app.Context) {
//	    var req CreateUserRequest
//	    if err := c.Bind(&req); err != nil {
//	        c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
//	        return
//	    }
//
//	    if err := c.Validate(&req); err != nil {
//	        c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
//	        return
//	    }
//
//	    // Process request...
//	    c.JSON(http.StatusCreated, user)
//	})
//
// # Server Configuration
//
// Configure server address and timeouts using functional options:
//
//	app.New(
//	    app.WithPort(3000),                 // Listen on port 3000
//	    app.WithHost("127.0.0.1"),          // Bind to localhost only
//	    app.WithServer(
//	        app.WithReadTimeout(10 * time.Second),
//	        app.WithWriteTimeout(10 * time.Second),
//	        app.WithIdleTimeout(60 * time.Second),
//	        app.WithShutdownTimeout(30 * time.Second),
//	    ),
//	)
//
// Configuration is automatically validated to catch common misconfigurations.
//
// # Environment Variables
//
// The app package supports configuration via environment variables using [WithEnv]:
//
//	app.New(
//	    app.WithServiceName("orders-api"),
//	    app.WithEnv(),  // Enable RIVAAS_* environment variable overrides
//	)
//
// Environment variables override programmatic configuration. Supported variables:
//
//	Core:
//	  RIVAAS_ENV                    - Environment mode: "development" or "production"
//	  RIVAAS_SERVICE_NAME           - Service name for observability
//	  RIVAAS_SERVICE_VERSION        - Service version
//
//	Server:
//	  RIVAAS_PORT                   - HTTP server port (e.g., "8080")
//	  RIVAAS_HOST                   - HTTP server host/interface (e.g., "127.0.0.1")
//	  RIVAAS_READ_TIMEOUT           - Request read timeout (e.g., "10s")
//	  RIVAAS_WRITE_TIMEOUT          - Response write timeout (e.g., "10s")
//	  RIVAAS_SHUTDOWN_TIMEOUT       - Graceful shutdown timeout (e.g., "30s")
//
//	Logging:
//	  RIVAAS_LOG_LEVEL              - Log level: "debug", "info", "warn", "error"
//	  RIVAAS_LOG_FORMAT             - Log format: "json", "text", or "console"
//
//	Observability:
//	  RIVAAS_METRICS_EXPORTER       - Metrics exporter: "prometheus", "otlp", or "stdout"
//	  RIVAAS_METRICS_ADDR           - Prometheus address (default: ":9090")
//	  RIVAAS_METRICS_PATH           - Prometheus path (default: "/metrics")
//	  RIVAAS_METRICS_ENDPOINT       - OTLP metrics endpoint (e.g., "http://localhost:4318")
//	  RIVAAS_TRACING_EXPORTER       - Tracing exporter: "otlp", "otlp-http", or "stdout"
//	  RIVAAS_TRACING_ENDPOINT       - OTLP tracing endpoint (e.g., "localhost:4317")
//
//	Debug:
//	  RIVAAS_PPROF_ENABLED          - Enable pprof: "true" or "false"
//
// Use [WithEnvPrefix] for a custom prefix:
//
//	app.New(
//	    app.WithEnvPrefix("MYAPP_"),  // Use MYAPP_* instead of RIVAAS_*
//	)
//
// Invalid environment values cause [New] to return an error (fail-fast).
//
// # Examples
//
// See the examples directory for complete working examples:
//
//   - examples/01-quick-start: Minimal setup to get started (basic routing, JSON responses)
//   - examples/02-blog: Real-world blog API with configuration, validation, OpenAPI docs, observability, and testing
//
// # Architecture
//
// The app package is built on top of the router package:
//
//	┌─────────────────────────────────────────┐
//	│           Application Layer             │
//	│  (app package - this package)           │
//	│                                         │
//	│  • Configuration Management             │
//	│  • Lifecycle Hooks                      │
//	│  • Observability Integration            │
//	│  • Server Management                    │
//	│  • Request Binding/Validation           │
//	└──────────────┬──────────────────────────┘
//	               │
//	               ▼
//	┌─────────────────────────────────────────┐
//	│           Router Layer                  │
//	│  (router package)                       │
//	│                                         │
//	│  • HTTP Routing                         │
//	│  • Middleware Chain                     │
//	│  • Request Context                      │
//	│  • Path Parameters                      │
//	└──────────────┬──────────────────────────┘
//	               │
//	               ▼
//	┌─────────────────────────────────────────┐
//	│        Standard Library                 │
//	│  (net/http)                             │
//	└─────────────────────────────────────────┘
package app
