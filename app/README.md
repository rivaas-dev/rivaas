# App

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/app.svg)](https://pkg.go.dev/rivaas.dev/app)
[![Go Report Card](https://goreportcard.com/badge/rivaas.dev/app)](https://goreportcard.com/report/rivaas.dev/app)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../LICENSE)

A high-level, batteries-included web framework built on top of the Rivaas router. This package provides a simple, opinionated API for building web applications with integrated observability, middleware, and graceful shutdown.

## Features

- **Batteries-Included**: Pre-configured with sensible defaults
- **Integrated Observability**: Built-in metrics and tracing support
- **Common Middleware**: Logger, recovery, CORS, and more
- **Graceful Shutdown**: Proper server shutdown handling
- **Environment-Aware**: Development and production modes
- **Functional Options**: Clean, extensible configuration API
- **Type-Safe Configuration**: Validated configuration with clear error messages

## Installation

```bash
go get rivaas.dev/app
```

Requires Go 1.25+

## When to Use

### Use `app` Package When

- **Building a complete web application** - Need a full-featured framework with batteries included
- **Want integrated observability** - Metrics and tracing configured out of the box
- **Need quick development** - Sensible defaults get you started immediately
- **Building a REST API** - Pre-configured with common middleware and patterns
- **Prefer convention over configuration** - Opinionated defaults that work well together

### Use `router` Package Directly When

- **Building a library or framework** - Need full control over the routing layer
- **Have custom observability setup** - Already using specific metrics/tracing solutions
- **Maximum performance is critical** - Want zero overhead from default middleware
- **Need complete flexibility** - Don't want any opinions or defaults imposed
- **Integrating into existing systems** - Need to fit into established patterns

The `app` package adds approximately 1-2% latency overhead compared to using `router` directly (119ns → ~121-122ns), but provides significant development speed and maintainability benefits through integrated observability and sensible defaults.

## Quick Start

### Simple App

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    
    "rivaas.dev/app"
)

func main() {
    // Create app with defaults
    a, err := app.New()
    if err != nil {
        log.Fatalf("Failed to create app: %v", err)
    }

    // Register routes
    a.GET("/", func(c *app.Context) {
        c.JSON(http.StatusOK, map[string]string{
            "message": "Hello from Rivaas App!",
        })
    })

    // Setup graceful shutdown
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    // Start server with graceful shutdown
    if err := a.Start(ctx, ":8080"); err != nil {
        log.Fatalf("Server error: %v", err)
    }
}
```

### Full-Featured App

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "rivaas.dev/app"
    "rivaas.dev/logging"
    "rivaas.dev/metrics"
    "rivaas.dev/tracing"
    "rivaas.dev/router/middleware/requestid"
    "rivaas.dev/router/middleware/cors"
)

func main() {
    // Create app with full observability
    // All features use the same consistent functional options pattern
    // Service name/version are automatically injected into all components
    a, err := app.New(
        app.WithServiceName("my-api"),
        app.WithServiceVersion("v1.0.0"),
        app.WithEnvironment("production"),
        // Observability: logging, metrics, tracing
        app.WithObservability(
            app.WithLogging(logging.WithJSONHandler()),
            app.WithMetrics(), // Prometheus is default
            app.WithTracing(tracing.WithOTLP("localhost:4317")),
            app.WithExcludePaths("/healthz", "/readyz", "/metrics"),
        ),
        // Health endpoints: GET /healthz (liveness), GET /readyz (readiness)
        app.WithHealthEndpoints(
            app.WithHealthTimeout(800 * time.Millisecond),
            app.WithLivenessCheck("process", func(ctx context.Context) error {
                return nil // Process is alive
            }),
            app.WithReadinessCheck("database", func(ctx context.Context) error {
                return db.PingContext(ctx)
            }),
        ),
        // Debug endpoints: GET /debug/pprof/* (conditional)
        app.WithDebugEndpoints(
            app.WithPprofIf(os.Getenv("PPROF_ENABLED") == "true"),
        ),
        app.WithServer(
            app.WithReadTimeout(15 * time.Second),
            app.WithWriteTimeout(15 * time.Second),
        ),
    )
    if err != nil {
        log.Fatalf("Failed to create app: %v", err)
    }

    // Add middleware
    a.Use(requestid.New())
    a.Use(cors.New(cors.WithAllowAllOrigins(true)))

    // Register routes
    a.GET("/", func(c *app.Context) {
        c.JSON(http.StatusOK, map[string]any{
            "message":     "Full Featured API",
            "service":     "my-api",
            "version":     "v1.0.0",
            "trace_id":    c.TraceID(),
            "request_id":  c.Response.Header().Get("X-Request-ID"),
        })
    })

    a.GET("/users/:id", func(c *app.Context) {
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

    // Setup graceful shutdown
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    // Start server
    // Health: GET /healthz, GET /readyz
    // Debug: GET /debug/pprof/* (if enabled)
    if err := a.Start(ctx, ":8080"); err != nil {
        log.Fatalf("Server error: %v", err)
    }
}
```

## Configuration Options

### Service Configuration

```go
app.WithServiceName("my-service")
app.WithServiceVersion("v1.0.0")
app.WithEnvironment("production") // or "development"
```

### Observability

```go
// Configure individually
// Note: Service name and version are automatically injected from app-level configuration
app.WithMetrics(
    metrics.WithPrometheus(":9090", "/metrics"), // or WithOTLP(), WithStdout()
)

app.WithTracing(
    tracing.WithSampleRate(0.1),
    tracing.WithExcludePaths("/health"),
)

app.WithLogging(
    logging.WithJSONHandler(),
)
```

**Automatic Service Metadata Injection:**

Service name and version set via `WithServiceName()` and `WithServiceVersion()` are automatically injected into all observability components (logging, metrics, and tracing). You don't need to pass them explicitly:

```go
app.New(
    app.WithServiceName("my-service"),      // Set once
    app.WithServiceVersion("v1.0.0"),       // Set once
    app.WithLogging(),                       // Automatically gets service metadata
    app.WithMetrics(),                       // Automatically gets service metadata
    app.WithTracing(),                       // Automatically gets service metadata
)
```

If you need to override the service metadata for a specific component, you can still pass it explicitly (your options will override the injected ones):

```go
app.New(
    app.WithServiceName("my-service"),
    app.WithServiceVersion("v1.0.0"),
    app.WithLogging(
        logging.WithServiceName("custom-logger"), // Overrides injected value
    ),
)
```

### Important: Tracing Requires OpenTelemetry Setup

When you see "🔍 Tracing enabled" in the logs, it means tracing *configuration* is enabled, but **traces won't actually be generated or exported** until you set up an OpenTelemetry tracer provider.

The `trace_id` will be empty and no traces will appear in stdout because:

- By default, OpenTelemetry uses a **no-op tracer provider**
- You must explicitly configure a tracer provider with an exporter
- This must be done **before** creating the app

**Quick Start - Stdout Tracing (Development):**

```go
import (
    "log"
    "rivaas.dev/app"
    "rivaas.dev/logging"
    "rivaas.dev/metrics"
    "rivaas.dev/tracing"
)

// Create app with full observability
// All three pillars use the same pattern: pass options
a, err := app.New(
    app.WithServiceName("my-service"),
    app.WithServiceVersion("v1.0.0"),
    app.WithObservability(
        app.WithLogging(logging.WithConsoleHandler()),
        app.WithMetrics(), // Prometheus is default
        app.WithTracing(tracing.WithStdout()),
    ),
)
if err != nil {
    log.Fatal(err)
}
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

Configure server timeouts and limits using functional options:

```go
app.WithServer(
    app.WithReadTimeout(10 * time.Second),
    app.WithWriteTimeout(10 * time.Second),
    app.WithIdleTimeout(60 * time.Second),
    app.WithReadHeaderTimeout(2 * time.Second),
    app.WithMaxHeaderBytes(1 << 20), // 1MB
    app.WithShutdownTimeout(30 * time.Second), // Graceful shutdown timeout
)
```

**Partial Configuration:** You can set only the fields you need. Unset fields will use their default values:

```go
// Only override read and write timeouts
app.WithServer(
    app.WithReadTimeout(15 * time.Second),
    app.WithWriteTimeout(15 * time.Second),
    // Other fields (IdleTimeout, etc.) will use defaults
)
```

**Default Values:**

- `ReadTimeout`: 10s
- `WriteTimeout`: 10s  
- `IdleTimeout`: 60s
- `ReadHeaderTimeout`: 2s
- `MaxHeaderBytes`: 1MB
- `ShutdownTimeout`: 30s

**Configuration Validation:**

Server configuration is automatically validated when creating the app. The validation catches common misconfigurations:

- ✅ **All timeouts must be positive** - Prevents invalid timeout values
- ✅ **ReadTimeout should not exceed WriteTimeout** - Common misconfiguration that can cause issues where the server times out reading the request body before it can write the response
- ✅ **ShutdownTimeout must be at least 1 second** - Ensures proper graceful shutdown (needs time to stop accepting connections, wait for in-flight requests, close idle connections, and clean up resources)
- ✅ **MaxHeaderBytes must be at least 1KB** - Prevents legitimate requests from failing due to standard HTTP headers (User-Agent, Accept, Cookie, etc.) exceeding the limit

**Example - Invalid Configuration:**

```go
app, err := app.New(
    app.WithServiceName("my-api"),
    app.WithServer(
        app.WithReadTimeout(15 * time.Second),
        app.WithWriteTimeout(10 * time.Second), // ❌ Invalid: read > write
        app.WithShutdownTimeout(100 * time.Millisecond), // ❌ Invalid: too short
        app.WithMaxHeaderBytes(512), // ❌ Invalid: too small
    ),
)
// err will contain validation errors:
// validation errors (3):
//   1. configuration error in server.readTimeout: read timeout should not exceed write timeout (compared with server.writeTimeout: 10s) (constraint: server.readTimeout vs server.writeTimeout, value: 15s)
//   2. configuration error in server.shutdownTimeout: must be at least 1 second for proper graceful shutdown (value: 100ms)
//   3. configuration error in server.maxHeaderBytes: must be at least 1KB (1024 bytes) to handle standard HTTP headers (value: 512)
```

**Example - Valid Configuration:**

```go
app, err := app.New(
    app.WithServiceName("my-api"),
    app.WithServer(
        app.WithReadTimeout(10 * time.Second),
        app.WithWriteTimeout(15 * time.Second), // ✅ Valid: write >= read
        app.WithShutdownTimeout(5 * time.Second), // ✅ Valid: >= 1s
        app.WithMaxHeaderBytes(2048), // ✅ Valid: >= 1KB
    ),
)
// err will be nil - configuration is valid
```

### Health Endpoints

Configure standard Kubernetes-compatible health endpoints using the consistent functional options pattern:

```go
app.WithHealthEndpoints(
    app.WithHealthPrefix("/_system"),      // Optional: mount under prefix
    app.WithHealthTimeout(800 * time.Millisecond),
    app.WithLivenessCheck("process", func(ctx context.Context) error {
        return nil // Process is alive
    }),
    app.WithReadinessCheck("database", func(ctx context.Context) error {
        return db.PingContext(ctx)
    }),
    app.WithReadinessCheck("cache", func(ctx context.Context) error {
        return redis.Ping(ctx).Err()
    }),
)
```

**Endpoints Registered:**

- `GET /healthz` (or `/{prefix}/healthz`) - Liveness probe
  - Returns `200 "ok"` if all liveness checks pass
  - Returns `503` if any liveness check fails
  - If no liveness checks configured, always returns `200`

- `GET /readyz` (or `/{prefix}/readyz`) - Readiness probe
  - Returns `204` if all readiness checks pass
  - Returns `503` if any readiness check fails
  - If no readiness checks configured, always returns `204`

**Health Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `WithHealthPrefix(prefix)` | Mount prefix for health endpoints | `""` (root) |
| `WithHealthzPath(path)` | Custom liveness probe path | `"/healthz"` |
| `WithReadyzPath(path)` | Custom readiness probe path | `"/readyz"` |
| `WithHealthTimeout(d)` | Timeout for each health check | `1s` |
| `WithLivenessCheck(name, fn)` | Add a liveness check | - |
| `WithReadinessCheck(name, fn)` | Add a readiness check | - |

**Liveness vs Readiness:**

- **Liveness checks** should be dependency-free and fast. They indicate whether the process is alive and should be restarted if failing.
- **Readiness checks** verify external dependencies (database, cache, upstream services). Failing readiness means the service should not receive traffic but doesn't need to be restarted.

**Runtime Readiness Gates:**

For dynamic readiness state (e.g., database connection pools managing their own health), use the `ReadinessManager`:

```go
type DatabaseGate struct {
    db *sql.DB
}
func (g *DatabaseGate) Ready() bool { return g.db.Ping() == nil }
func (g *DatabaseGate) Name() string { return "database" }

// Register at runtime
app.Readiness().Register("db", &DatabaseGate{db: db})

// Unregister during shutdown
app.Readiness().Unregister("db")
```

### Debug Endpoints

Enable debug endpoints (pprof) using the consistent functional options pattern:

```go
// Development: enable pprof unconditionally
app.WithDebugEndpoints(
    app.WithPprof(),
)

// Production: enable conditionally via environment variable
app.WithDebugEndpoints(
    app.WithDebugPrefix("/_internal/debug"),
    app.WithPprofIf(os.Getenv("PPROF_ENABLED") == "true"),
)
```

**⚠️ Security Warning:**

pprof endpoints expose sensitive runtime information and should NEVER be enabled without proper security measures in production:

- **Development**: Enable unconditionally (no external exposure)
- **Staging**: Enable behind VPN or IP allowlist
- **Production**: Enable only with proper authentication middleware

**Endpoints Registered (when pprof enabled):**

- `GET /debug/pprof/` - Main pprof index
- `GET /debug/pprof/cmdline` - Command line
- `GET /debug/pprof/profile` - CPU profile
- `GET /debug/pprof/symbol` - Symbol lookup
- `POST /debug/pprof/symbol` - Symbol lookup
- `GET /debug/pprof/trace` - Execution trace
- `GET /debug/pprof/{profile}` - Named profiles (allocs, block, goroutine, heap, mutex, threadcreate)

**Debug Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `WithDebugPrefix(prefix)` | Mount prefix for debug endpoints | `"/debug"` |
| `WithPprof()` | Enable pprof endpoints | Disabled |
| `WithPprofIf(condition)` | Conditionally enable pprof | Disabled |

### Middleware Configuration

Add middleware during initialization or after app creation:

```go
// Option 1: Add middleware during initialization
a, err := app.New(
    app.WithServiceName("my-service"),
    app.WithMiddleware(
        accesslog.New(accesslog.WithLogger(logger)),
        recovery.New(),
        requestid.New(),
    ),
)

// Option 2: Add middleware after creation (more flexible)
a, err := app.New(
    app.WithServiceName("my-service"),
)
a.Use(accesslog.New(accesslog.WithLogger(logger)))
a.Use(recovery.New())
a.Use(requestid.New())

// Option 3: Mix both approaches
a, err := app.New(
    app.WithServiceName("my-service"),
    app.WithMiddleware(recovery.New()), // Core middleware
)
a.Use(accesslog.New(accesslog.WithLogger(logger)))  // Additional middleware
```

**Default Behavior:**

- **Development mode**: Automatically includes `recovery.New()` middleware and access logging via observability recorder
- **Production mode**: Automatically includes only `recovery.New()` middleware, with error-only logging via observability recorder
- **To disable defaults**: Call `app.WithMiddleware()` with no arguments (empty) to opt out of default middleware

## Built-in Middleware

The app package provides access to high-quality middleware from `router/middleware` subpackages. Each middleware is in its own subpackage for clean imports and modularity.

### Access Logging

```go
import "rivaas.dev/router/middleware/accesslog"

// Basic logger
app.Use(accesslog.New(accesslog.WithLogger(logger)))

// With options
app.Use(accesslog.New(
    accesslog.WithLogger(logger),
    accesslog.WithSkipPaths([]string{"/health", "/metrics"}),
))
```

Logs requests with timing, status codes, and client IPs. Requires a `*slog.Logger` to be provided.

**Note:** The app package automatically configures access logging through its unified observability recorder when `WithLogging()` is used. Manual access logging middleware is only needed for custom configurations.

### Recovery

```go
import "rivaas.dev/router/middleware/recovery"

// Basic recovery
app.Use(recovery.New())

// With options
app.Use(recovery.New(
    recovery.WithStackTrace(true),
))
```

Recovers from panics and returns proper error responses. Configurable stack traces and custom handlers.

**Note:** Recovery middleware is automatically included by default in both development and production modes. Use `app.WithMiddleware()` to override defaults.

### CORS

```go
import "rivaas.dev/router/middleware/cors"

// Allow all origins (development)
app.Use(cors.New(cors.WithAllowAllOrigins(true)))

// Specific origins (production)
app.Use(cors.New(
    cors.WithAllowedOrigins([]string{"https://example.com"}),
    cors.WithAllowCredentials(true),
))
```

Handles Cross-Origin Resource Sharing with flexible configuration options.

### Request ID

```go
import "rivaas.dev/router/middleware/requestid"

// Basic request ID
app.Use(requestid.New())

// With custom header
app.Use(requestid.New(
    requestid.WithRequestIDHeader("X-Correlation-ID"),
))
```

Adds unique request IDs to each request for distributed tracing.

### Timeout

```go
import "rivaas.dev/router/middleware/timeout"

// Basic timeout (uses 30s default)
app.Use(timeout.New())

// With custom duration
app.Use(timeout.New(timeout.WithDuration(5 * time.Second)))

// With options
app.Use(timeout.New(
    timeout.WithDuration(30*time.Second),
    timeout.WithSkipPaths("/stream"),
    timeout.WithSkipPrefix("/admin"),
))
```

Adds request timeout handling with context cancellation.

### Rate Limiting

```go
import "rivaas.dev/router/middleware/ratelimit"

app.Use(ratelimit.New(100, time.Minute)) // 100 requests per minute
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
// Setup signal handling for graceful shutdown
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()

// Start HTTP server
if err := app.Start(ctx, ":8080"); err != nil {
    log.Fatal(err)
}
```

### HTTPS Server

```go
// Setup signal handling
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()

// Start HTTPS server
if err := app.StartTLS(ctx, ":8443", "cert.pem", "key.pem"); err != nil {
    log.Fatal(err)
}
```

### Graceful Shutdown

The app supports graceful shutdown via context cancellation. When the context is canceled
(e.g., via OS signals), the server:

1. Stops accepting new connections
2. Executes OnShutdown hooks in LIFO order
3. Waits for in-flight requests to complete (up to shutdown timeout)
4. Shuts down observability components (metrics, tracing)
5. Executes OnStop hooks in best-effort mode

Use `signal.NotifyContext` to trigger shutdown on OS signals (SIGINT, SIGTERM):

```go
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()

if err := app.Start(ctx, ":8080"); err != nil {
    log.Fatal(err)
}
```

**Architecture:** HTTP and HTTPS servers share the same lifecycle implementation through `runServer`, which provides:

- **Unified telemetry**: Startup/shutdown events are logged through the configured slog logger with protocol identification
- **Consistent shutdown**: Both protocols use the same graceful shutdown timeout and signal handling
- **Observability teardown**: Metrics and tracing components are shut down in the correct order after the server stops accepting connections
- **Protocol abstraction**: The design uses a function parameter (`serverStartFunc`) to abstract the difference between `ListenAndServe` and `ListenAndServeTLS`, ensuring identical behavior for both protocols

This design ensures that HTTP and HTTPS deployments have identical lifecycle behavior, making it safe to switch protocols without changing shutdown logic.

## Accessing Underlying Components

### Router

```go
router := app.Router()
// Use router for advanced features
```

### Metrics

```go
metrics := app.Metrics()
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
tracing := app.Tracing()
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
import "rivaas.dev/router"

r := router.New(
    router.WithMetrics(),
    router.WithTracing(),
)
// No error handling needed
```

### After (App)

```go
import (
    "context"
    "os"
    "os/signal"
    "syscall"
    
    "rivaas.dev/app"
    "rivaas.dev/logging"
    "rivaas.dev/metrics"
    "rivaas.dev/tracing"
)

a, err := app.New(
    app.WithServiceName("my-service"),
    app.WithServiceVersion("v1.0.0"),
    app.WithObservability(
        app.WithLogging(logging.WithJSONHandler()),
        app.WithMetrics(), // Prometheus is default
        app.WithTracing(tracing.WithOTLP("localhost:4317")),
    ),
)
if err != nil {
    log.Fatalf("Failed to create app: %v", err)
}

// Setup graceful shutdown
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()

// Start server
if err := a.Start(ctx, ":8080"); err != nil {
    log.Fatal(err)
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

## Troubleshooting

### Common Issues

#### Server Won't Start

**Problem:** `app.Run()` returns an error immediately.

**Solutions:**

- Check if the port is already in use: `lsof -i :8080` or `netstat -an | grep 8080`
- Verify you have permission to bind to the port (ports < 1024 require root on Unix)
- Check firewall rules if running in a container or cloud environment

#### Configuration Validation Errors

**Problem:** `app.New()` returns validation errors.

**Solutions:**

- Ensure all timeout values are positive: `WithReadTimeout(10 * time.Second)`
- Read timeout must not exceed write timeout
- Shutdown timeout must be at least 1 second
- Max header bytes must be at least 1KB

**Example:**

```go
// ❌ Invalid
app.WithServer(
    app.WithReadTimeout(15 * time.Second),
    app.WithWriteTimeout(10 * time.Second), // Read > Write
)

// ✅ Valid
app.WithServer(
    app.WithReadTimeout(10 * time.Second),
    app.WithWriteTimeout(15 * time.Second), // Write >= Read
)
```

#### Metrics Not Appearing

**Problem:** Metrics endpoint returns 404 or empty response.

**Solutions:**

- Ensure metrics are enabled: `app.WithMetrics()`
- Check the metrics path: `app.Metrics().Path()` (default: `/metrics`)
- Verify the metrics server is running: `app.Metrics().ServerAddress()`
- In development mode, check the startup banner for metrics address

#### Tracing Not Working

**Problem:** No traces appear in your tracing backend.

**Solutions:**

- Verify tracing is enabled: `app.WithTracing()`
- Check OTLP endpoint configuration: `OTLP_ENDPOINT=jaeger:4317`
- Ensure the tracing provider is correct: `tracing.WithOTLP("localhost:4317")`
- Check network connectivity to the tracing backend
- Review logs for tracing initialization errors

#### Graceful Shutdown Not Working

**Problem:** Server doesn't shut down cleanly or takes too long.

**Solutions:**

- Increase shutdown timeout: `app.WithServer(app.WithShutdownTimeout(60 * time.Second))`
- Ensure OnShutdown hooks complete quickly (they block shutdown)
- Check for long-running requests that don't respect context cancellation
- Verify OnStop hooks don't perform blocking operations

#### Routes Not Registering

**Problem:** Routes return 404 even though they're registered.

**Solutions:**

- Ensure routes are registered before calling `app.Run()` (router is frozen on startup)
- Check route paths match exactly (case-sensitive, trailing slashes matter)
- Verify HTTP method matches (GET vs POST)
- Use `app.PrintRoutes()` to see all registered routes
- Check middleware isn't blocking requests

#### Middleware Not Executing

**Problem:** Middleware functions aren't being called.

**Solutions:**

- Ensure middleware is added before routes: `app.Use(accesslog.New(...))` before `app.GET(...)`
- Check middleware calls `c.Next()` to continue the chain
- Verify middleware isn't returning early without calling `c.Next()`
- In development mode, check logs for middleware execution

### Debugging Tips

1. **Enable Development Mode:**

   ```go
   app.WithEnvironment(EnvironmentDevelopment)
   ```

   This enables verbose logging and route table display.

2. **Print Registered Routes:**

   ```go
   app.PrintRoutes() // Shows all routes in a formatted table
   ```

3. **Check Observability Status:**

   ```go
   if app.Metrics() != nil {
       fmt.Println("Metrics enabled:", app.Metrics().ServerAddress())
   }
   if app.Tracing() != nil {
       fmt.Println("Tracing enabled")
   }
   ```

4. **Use Test Helpers:**

   ```go
   resp, err := app.Test(req) // Test requests without starting server
   ```

5. **Enable GC Tracing (for memory issues):**

   ```bash
   GODEBUG=gctrace=1 go run main.go
   ```

## API Reference

### Core Functions

| Function | Description | Returns |
|----------|-------------|---------|
| `New(...Option)` | Create a new App instance | `(*App, error)` |
| `MustNew(...Option)` | Create a new App instance (panics on error) | `*App` |

### App Methods

| Method | Description | Returns |
|--------|-------------|---------|
| `Start(ctx context.Context, addr string)` | Start HTTP server with graceful shutdown | `error` |
| `StartTLS(ctx context.Context, addr, certFile, keyFile string)` | Start HTTPS server | `error` |
| `StartMTLS(ctx context.Context, addr string, cert tls.Certificate, opts ...MTLSOption)` | Start mTLS server | `error` |
| `GET(path string, handler HandlerFunc)` | Register GET route | - |
| `POST(path string, handler HandlerFunc)` | Register POST route | - |
| `PUT(path string, handler HandlerFunc)` | Register PUT route | - |
| `DELETE(path string, handler HandlerFunc)` | Register DELETE route | - |
| `PATCH(path string, handler HandlerFunc)` | Register PATCH route | - |
| `HEAD(path string, handler HandlerFunc)` | Register HEAD route | - |
| `OPTIONS(path string, handler HandlerFunc)` | Register OPTIONS route | - |
| `Group(prefix string, middleware ...HandlerFunc)` | Create route group | `*Group` |
| `Use(middleware ...HandlerFunc)` | Add middleware | - |
| `Static(prefix, root string)` | Serve static files | - |
| `Router()` | Get underlying router | `*router.Router` |
| `Metrics()` | Get metrics recorder | `*metrics.Recorder` |
| `Tracing()` | Get tracing configuration | `*tracing.Tracer` |
| `Route(name string)` | Get route by name | `(*router.Route, bool)` |
| `Routes()` | Get all named routes | `[]*router.Route` |
| `PrintRoutes()` | Print all registered routes | - |

### App Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithServiceName(name string)` | Set service name | `"rivaas-app"` |
| `WithServiceVersion(version string)` | Set service version | `"1.0.0"` |
| `WithEnvironment(env string)` | Set environment | `"development"` |
| `WithObservability(opts ...ObservabilityOption)` | Configure observability | Disabled |
| `WithHealthEndpoints(opts ...HealthOption)` | Configure health endpoints | Disabled |
| `WithDebugEndpoints(opts ...DebugOption)` | Configure debug endpoints | Disabled |
| `WithServer(opts ...ServerOption)` | Configure server settings | See defaults below |
| `WithMiddleware(middlewares ...HandlerFunc)` | Add middleware | Auto-included in dev |
| `WithRouter(opts ...router.Option)` | Configure router | - |

### Observability Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithMetrics(opts ...metrics.Option)` | Enable metrics collection | Disabled |
| `WithTracing(opts ...tracing.Option)` | Enable distributed tracing | Disabled |
| `WithLogging(opts ...logging.Option)` | Enable structured logging | Disabled |
| `WithExcludePaths(paths ...string)` | Exclude paths from observability | Common health paths |
| `WithExcludePrefixes(prefixes ...string)` | Exclude path prefixes | - |
| `WithExcludePatterns(patterns ...string)` | Exclude paths matching regex patterns | - |
| `WithoutDefaultExclusions()` | Clear default path exclusions | - |
| `WithMetricsOnMainRouter(path string)` | Mount metrics endpoint on main router | Disabled |
| `WithMetricsSeparateServer(addr, path string)` | Configure separate metrics server | `:9090/metrics` |
| `WithAccessLogging(enabled bool)` | Enable/disable access logging | `true` |
| `WithLogOnlyErrors()` | Log only errors and slow requests | `false` |
| `WithSlowThreshold(d time.Duration)` | Mark requests as slow | `1s` |

### Health Endpoint Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithHealthPrefix(prefix string)` | Mount prefix for endpoints | `""` (root) |
| `WithHealthzPath(path string)` | Custom liveness probe path | `"/healthz"` |
| `WithReadyzPath(path string)` | Custom readiness probe path | `"/readyz"` |
| `WithHealthTimeout(d time.Duration)` | Timeout for each check | `1s` |
| `WithLivenessCheck(name string, fn CheckFunc)` | Add liveness check | - |
| `WithReadinessCheck(name string, fn CheckFunc)` | Add readiness check | - |

### Debug Endpoint Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithDebugPrefix(prefix string)` | Mount prefix for endpoints | `"/debug"` |
| `WithPprof()` | Enable pprof endpoints | Disabled |
| `WithPprofIf(condition bool)` | Conditionally enable pprof | Disabled |

### Server Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithReadTimeout(d time.Duration)` | Request read timeout | `10s` |
| `WithWriteTimeout(d time.Duration)` | Response write timeout | `10s` |
| `WithIdleTimeout(d time.Duration)` | Idle connection timeout | `60s` |
| `WithReadHeaderTimeout(d time.Duration)` | Header read timeout | `2s` |
| `WithMaxHeaderBytes(n int)` | Max request header size | `1MB` |
| `WithShutdownTimeout(d time.Duration)` | Graceful shutdown timeout | `30s` |

### Lifecycle Hooks

| Hook | Description | Execution |
|------|-------------|-----------|
| `OnStart(fn func(context.Context) error)` | Called before server starts | Sequential, fail-fast |
| `OnReady(fn func())` | Called when server is ready | Async, non-blocking |
| `OnShutdown(fn func(context.Context))` | Called during shutdown | LIFO order |
| `OnStop(fn func())` | Called after shutdown | Best-effort |
| `OnRoute(fn func(*route.Route))` | Called when route is registered | Synchronous, during registration |

## Architecture

The `app` package is built on top of the `router` package and adds:

```text
┌─────────────────────────────────────────┐
│           Application Layer             │
│  (app package - this package)           │
│                                         │
│  • Configuration Management             │
│  • Lifecycle Hooks                      │
│  • Observability Integration            │
│  • Server Management                    │
│  • Startup Banner                       │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│           Router Layer                  │
│  (router package)                       │
│                                         │
│  • HTTP Routing                         │
│  • Middleware Chain                     │
│  • Request Context                      │
│  • Path Parameters                      │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│        Standard Library                 │
│  (net/http)                             │
└─────────────────────────────────────────┘
```

**Key Design Principles:**

1. **Separation of Concerns**: App handles configuration and lifecycle; router handles HTTP routing
2. **Functional Options**: Clean, extensible configuration API
3. **Graceful Degradation**: Works with or without observability components
4. **Environment Awareness**: Different defaults for development vs production

## API Reference

For detailed API documentation, see [pkg.go.dev/rivaas.dev/app](https://pkg.go.dev/rivaas.dev/app).

## Contributing

Contributions are welcome! Please see the [main repository](../) for contribution guidelines.

## License

Apache License 2.0 - see [LICENSE](../LICENSE) for details.

---

Part of the [Rivaas](https://github.com/rivaas-dev/rivaas) web framework ecosystem.
