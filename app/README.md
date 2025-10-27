# Rivaas App

A high-level, batteries-included web framework built on top of the Rivaas router. This package provides a simple, opinionated API for building web applications with integrated observability, middleware, and graceful shutdown.

## Features

- **Batteries-Included**: Pre-configured with sensible defaults
- **Integrated Observability**: Built-in metrics and tracing support
- **Common Middleware**: Logger, recovery, CORS, and more
- **Graceful Shutdown**: Proper server shutdown handling
- **Environment-Aware**: Development and production modes
- **Functional Options**: Clean, extensible configuration API
- **Type-Safe Configuration**: Validated configuration with clear error messages

## When to Use

### Use `app` Package When:

- **Building a complete web application** - Need a full-featured framework with batteries included
- **Want integrated observability** - Metrics and tracing configured out of the box
- **Need quick development** - Sensible defaults get you started immediately
- **Building a REST API** - Pre-configured with common middleware and patterns
- **Prefer convention over configuration** - Opinionated defaults that work well together

### Use `router` Package Directly When:

- **Building a library or framework** - Need full control over the routing layer
- **Have custom observability setup** - Already using specific metrics/tracing solutions
- **Maximum performance is critical** - Want zero overhead from default middleware
- **Need complete flexibility** - Don't want any opinions or defaults imposed
- **Integrating into existing systems** - Need to fit into established patterns

The `app` package adds approximately 1-2% latency overhead compared to using `router` directly, but provides significant development speed and maintainability benefits through integrated observability and sensible defaults.

## Quick Start

### Simple App

```go
package main

import (
    "log"
    "net/http"
    "rivass.dev/app"
    "rivass.dev/router"
)

func main() {
    // Create app with defaults
    a, err := app.New()
    if err != nil {
        log.Fatalf("Failed to create app: %v", err)
    }

    // Register routes
    a.GET("/", func(c *router.Context) {
        c.JSON(http.StatusOK, map[string]string{
            "message": "Hello from Rivaas App!",
        })
    })

    // Start server with graceful shutdown
    if err := a.Run(":8080"); err != nil {
        log.Fatalf("Server error: %v", err)
    }
}
```

### Full-Featured App

```go
package main

import (
    "log"
    "net/http"
    "time"
    
    "rivass.dev/app"
    "rivass.dev/router"
    "rivass.dev/router/middleware"
    "go.opentelemetry.io/otel/attribute"
)

func main() {
    // Create app with full observability
    a, err := app.New(
        app.WithServiceName("my-api"),
        app.WithVersion("v1.0.0"),
        app.WithEnvironment("production"),
        app.WithObservability(), // Enables both metrics and tracing
        app.WithServerConfig(&app.ServerConfig{
            ReadTimeout:  15 * time.Second,
            WriteTimeout: 15 * time.Second,
        }),
    )
    if err != nil {
        log.Fatalf("Failed to create app: %v", err)
    }

    // Add middleware
    a.Use(middleware.RequestID())
    a.Use(middleware.CORS(middleware.WithAllowAllOrigins(true)))

    // Register routes
    a.GET("/", func(c *router.Context) {
        c.JSON(http.StatusOK, map[string]any{
            "message":     "Full Featured API",
            "service":     "my-api",
            "version":     "v1.0.0",
            "trace_id":    c.TraceID(),
            "request_id":  c.Response.Header().Get("X-Request-ID"),
        })
    })

    a.GET("/users/:id", func(c *router.Context) {
        userID := c.Param("id")
        
        // Add span attributes
        c.SetSpanAttribute("user.id", userID)
        c.AddSpanEvent("user_lookup_started")
        
        // Record custom metrics
        c.IncrementCounter("user_lookups_total",
            attribute.String("user_id", userID),
        )
        
        c.JSON(http.StatusOK, map[string]any{
            "user_id":    userID,
            "name":       "John Doe",
            "trace_id":   c.TraceID(),
            "request_id": c.Response.Header().Get("X-Request-ID"),
        })
    })

    // Start server
    if err := a.Run(":8080"); err != nil {
        log.Fatalf("Server error: %v", err)
    }
}
```

## Configuration Options

### Service Configuration

```go
app.WithServiceName("my-service")
app.WithVersion("v1.0.0")
app.WithEnvironment("production") // or "development"
```

### Observability

```go
// Enable both metrics and tracing with defaults
app.WithObservability()

// Or configure individually
app.WithMetrics(
    metrics.WithProvider(metrics.PrometheusProvider),
    metrics.WithPort(":9090"),
)

app.WithTracing(
    tracing.WithSampleRate(0.1),
    tracing.WithExcludePaths("/health"),
)
```

**Important: Tracing Requires OpenTelemetry Setup**

When you see "🔍 Tracing enabled" in the logs, it means tracing *configuration* is enabled, but **traces won't actually be generated or exported** until you set up an OpenTelemetry tracer provider.

The `trace_id` will be empty and no traces will appear in stdout because:
- By default, OpenTelemetry uses a **no-op tracer provider**
- You must explicitly configure a tracer provider with an exporter
- This must be done **before** creating the app

**Quick Start - Stdout Tracing (Development):**

```go
import "rivass.dev/tracing"

// Set up stdout exporter BEFORE creating app
tp, err := tracing.SetupStdout("my-service", "v1.0.0")
if err != nil {
    log.Fatal(err)
}
defer tp.Shutdown(context.Background())

// Now create app - traces will actually work!
app, _ := app.New(app.WithObservability())
```

**Just run it:**
```bash
go run main.go  # No build tags needed!
```

**Switch exporters:**
```bash
# Development: stdout tracing
ENVIRONMENT=development go run main.go

# Production: OTLP to Jaeger/Tempo
ENVIRONMENT=production OTLP_ENDPOINT=jaeger:4317 go run main.go
```

**See:** `examples/02-full-featured/` for a complete production-ready example with:
- Multi-exporter tracing (stdout/OTLP/noop)
- Environment-based configuration
- Full middleware stack
- Custom metrics and tracing in handlers

**Note:** Request IDs work independently and don't require any tracing setup.

### Server Configuration

Configure server timeouts and limits:

```go
app.WithServerConfig(&app.ServerConfig{
    ReadTimeout:       10 * time.Second,
    WriteTimeout:      10 * time.Second,
    IdleTimeout:       60 * time.Second,
    ReadHeaderTimeout: 2 * time.Second,
    MaxHeaderBytes:    1 << 20, // 1MB
    ShutdownTimeout:   30 * time.Second, // Graceful shutdown timeout
})
```

**Partial Configuration:** You can set only the fields you need. Unset fields will use their default values:

```go
// Only override read and write timeouts
app.WithServerConfig(&app.ServerConfig{
    ReadTimeout:  15 * time.Second,
    WriteTimeout: 15 * time.Second,
    // Other fields (IdleTimeout, etc.) will use defaults
})
```

**Default Values:**

- `ReadTimeout`: 10s
- `WriteTimeout`: 10s  
- `IdleTimeout`: 60s
- `ReadHeaderTimeout`: 2s
- `MaxHeaderBytes`: 1MB
- `ShutdownTimeout`: 30s

### Middleware Configuration

Control which default middleware is included:

```go
// Include logger and recovery (useful for development)
app.WithMiddleware(true, true)

// Disable logger in production (you may use custom logging)
app.WithMiddleware(false, true)

// Disable all default middleware
app.WithMiddleware(false, false)
```

By default:

- **Development mode**: Includes logger and recovery middleware
- **Production mode**: Includes only recovery middleware

## Built-in Middleware

The app package provides access to high-quality middleware from `router/middleware`:

```go
import "rivass.dev/router/middleware"
```

### Logger

```go
// Basic logger
app.Use(middleware.Logger())

// With options
app.Use(middleware.Logger(
    middleware.WithColors(true),
    middleware.WithSkipPaths([]string{"/health", "/metrics"}),
))
```

Logs requests with timing, status codes, and client IPs. Supports colored output, custom formatters, and path skipping.

### Recovery

```go
// Basic recovery
app.Use(middleware.Recovery())

// With options
app.Use(middleware.Recovery(
    middleware.WithStackTrace(true),
    middleware.WithRecoveryLogger(customLogger),
))
```

Recovers from panics and returns proper error responses. Configurable stack traces and custom handlers.

### CORS

```go
// Allow all origins (development)
app.Use(middleware.CORS(middleware.WithAllowAllOrigins(true)))

// Specific origins (production)
app.Use(middleware.CORS(
    middleware.WithAllowedOrigins([]string{"https://example.com"}),
    middleware.WithAllowCredentials(true),
))
```

Handles Cross-Origin Resource Sharing with flexible configuration options.

### Request ID

```go
// Basic request ID
app.Use(middleware.RequestID())

// With custom header
app.Use(middleware.RequestID(
    middleware.WithRequestIDHeader("X-Correlation-ID"),
))
```

Adds unique request IDs to each request for distributed tracing.

### Timeout

```go
// Basic timeout
app.Use(middleware.Timeout(30 * time.Second))

// With options
app.Use(middleware.Timeout(30*time.Second,
    middleware.WithTimeoutSkipPaths([]string{"/stream"}),
))
```

Adds request timeout handling with context cancellation.

### Rate Limiting

```go
app.Use(app.RateLimit(100, time.Minute)) // 100 requests per minute
```

Simple in-memory rate limiting. Note: This is suitable for single-instance deployments only. For production with multiple instances, use a distributed rate limiting solution.

## Routing

### Basic Routes

```go
app.GET("/users", getUsersHandler)
app.POST("/users", createUserHandler)
app.PUT("/users/:id", updateUserHandler)
app.DELETE("/users/:id", deleteUserHandler)
app.PATCH("/users/:id", patchUserHandler)
app.HEAD("/users", headUsersHandler)
app.OPTIONS("/users", optionsUsersHandler)
```

### Route Groups

```go
api := app.Group("/api/v1")
api.GET("/users", getUsersHandler)
api.POST("/users", createUserHandler)

admin := app.Group("/admin", adminMiddleware)
admin.GET("/dashboard", dashboardHandler)
```

### Static Files

```go
app.Static("/static", "./public")
```

## Server Management

### HTTP Server

```go
// Start HTTP server
app.Run(":8080")
```

### HTTPS Server

```go
// Start HTTPS server
app.RunTLS(":8443", "cert.pem", "key.pem")
```

### Graceful Shutdown

The app automatically handles graceful shutdown on SIGINT or SIGTERM signals.

## Accessing Underlying Components

### Router

```go
router := app.Router()
// Use router for advanced features
```

### Metrics

```go
metrics := app.GetMetrics()
if metrics != nil {
    handler, err := app.GetMetricsHandler()
    if err != nil {
        log.Fatalf("Failed to get metrics handler: %v", err)
    }
    // Serve metrics manually
    http.Handle("/metrics", handler)
}
```

### Tracing

```go
tracing := app.GetTracing()
if tracing != nil {
    // Access tracing configuration
}
```

## Environment Modes

### Development Mode

```go
app := app.New(
    app.WithEnvironment("development"),
)
```

- Logger middleware enabled by default
- More verbose error messages
- Development-friendly defaults

### Production Mode

```go
app := app.New(
    app.WithEnvironment("production"),
)
```

- Optimized for performance
- Minimal logging
- Production-ready defaults

## Examples

The `examples/` directory contains two examples:

### Example 01: Quick Start

Minimal setup - perfect for learning the basics:

```bash
cd examples/01-quick-start
go run main.go
```

**Shows:** Basic routing, JSON responses, default configuration (~20 lines of code)

### Example 02: Full-Featured Production App

Complete production-ready application with full observability:

```bash
cd examples/02-full-featured

# Development mode (stdout tracing)
go run main.go

# Production mode (OTLP to Jaeger)
ENVIRONMENT=production OTLP_ENDPOINT=localhost:4317 go run main.go
```

**Shows:**
- Multi-exporter tracing (stdout/OTLP/noop)
- Environment-based configuration  
- Full middleware stack
- Custom metrics and tracing
- RESTful API patterns
- Production deployment patterns

**See:** `examples/README.md` for detailed comparison and usage guide

## Migration from Router

If you're migrating from the low-level router package:

### Before (Router)

```go
import "rivass.dev/router"

r := router.New(
    router.WithMetrics(),
    router.WithTracing(),
)
// No error handling needed
```

### After (App)

```go
import "rivass.dev/app"

a, err := app.New(
    app.WithObservability(),
)
if err != nil {
    log.Fatalf("Failed to create app: %v", err)
}
// Configuration is validated at creation time
```

**Key Differences:**
- `New()` now returns `(*App, error)` for better error handling
- Configuration is validated immediately
- Invalid configurations are caught at startup, not runtime
- Use `any` instead of `interface{}` for JSON responses

## Performance

- **Minimal Overhead**: App layer adds minimal overhead over raw router
- **Optimized Defaults**: Sensible defaults for production use
- **Graceful Shutdown**: Proper resource cleanup
- **Memory Efficient**: Reuses router's memory optimizations

## License

MIT License - see LICENSE file for details.
