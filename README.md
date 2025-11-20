# Rivaas

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/rivaas.dev)](https://goreportcard.com/report/rivaas.dev)

A high-performance, modular web framework for Go with integrated observability. Rivaas provides both low-level building blocks and high-level batteries-included APIs for building modern web applications.

## Table of Contents

- [Philosophy](#-philosophy)
- [Why Rivaas?](#-why-rivaas)
- [Features](#-features)
- [Installation](#-installation)
- [Packages](#-packages)
- [Repository Structure](#️-repository-structure)
- [Quick Start](#-quick-start)
- [Architecture](#️-architecture)
- [Performance](#-performance)
- [Configuration](#-configuration)
- [Middleware](#️-middleware)
- [Observability](#-observability)
- [Testing](#-testing)
- [Production Deployment](#-production-deployment)
- [Examples](#-examples)
- [Migration Guide](#-migration-guide)
- [Contributing](#-contributing)
- [License](#-license)

## 🌿 Philosophy

The name Rivaas comes from **ریواس (Rivās)** — a [wild rhubarb plant](https://en.wikipedia.org/wiki/Rheum_ribes) that grows high in the mountains of Iran.

Rivaas survives in harsh, unpredictable climates — light, resilient, and naturally adaptive.

That's the same philosophy behind this framework.

Rivaas is built to thrive in dynamic, cloud-native environments — **lightweight yet powerful, modular yet simple**.

Like its namesake, it grows wherever the environment allows: locally, in the cloud, or across distributed systems.

## 💡 Why Rivaas?

- **For Production:** Built-in observability means you're production-ready from day one
- **For Performance:** Sub-microsecond routing with minimal memory overhead  
- **For Flexibility:** Choose between high-level convenience or low-level control
- **For Teams:** Structured logging and tracing built-in, not bolted on
- **For Cloud-Native:** OpenTelemetry-native design for modern infrastructure

## 🚀 Features

- **High Performance**: 6.9M+ requests/second, 145ns average latency
- **Modular Design**: Use only what you need
- **Integrated Observability**: Built-in metrics, tracing, and structured logging
- **Memory Efficient**: Only 16 bytes memory per request
- **Graceful Shutdown**: Production-ready server management
- **Multiple APIs**: Choose between low-level or high-level interfaces
- **OpenTelemetry Native**: First-class observability support

## 📥 Installation

**Prerequisites:** Go 1.25 or higher

```bash
# Install the high-level API (recommended for most users)
go get rivaas.dev/app

# Or install individual packages
go get rivaas.dev/router
go get rivaas.dev/logging
go get rivaas.dev/metrics
go get rivaas.dev/tracing
```

## 📦 Packages

### Core Packages

- **[router](./router/)** - High-performance HTTP router with radix tree routing ([Docs](./router/README.md))
- **[app](./app/)** - Batteries-included web framework ([Docs](./app/README.md))

### Observability Packages

- **[logging](./logging/)** - Structured logging with slog (JSON, text, console) ([Docs](./logging/README.md))
- **[metrics](./metrics/)** - OpenTelemetry metrics with Prometheus, OTLP, and stdout support ([Docs](./metrics/README.md))
- **[tracing](./tracing/)** - Distributed tracing with OpenTelemetry ([Docs](./tracing/README.md))

## 🏗️ Repository Structure

This is a **multi-module repository**. Each package is a separate Go module with its own `go.mod` file and can be versioned independently.

### Module Organization

```text
rivaas/
├── app/          → rivaas.dev/app       (batteries-included framework)
├── router/       → rivaas.dev/router    (HTTP router)
├── logging/      → rivaas.dev/logging   (structured logging)
├── metrics/      → rivaas.dev/metrics   (metrics collection)
├── tracing/      → rivaas.dev/tracing   (distributed tracing)
└── go.work       (workspace for local development)
```

### Version Tags

Each module is versioned independently using the pattern: `<module-dir>/<version>`

Examples:

- `router/v0.1.0` - Router version 0.1.0
- `app/v1.0.0` - App version 1.0.0
- `logging/v0.2.3` - Logging version 0.2.3

### Local Development

For local development with all modules together, use the workspace:

```bash
# Clone the repository
git clone https://github.com/rivaas-dev/rivaas.git
cd rivaas

# The go.work file automatically configures all modules
# No additional setup needed - just run:
go test ./...
```

## 🎯 Quick Start

### Option 1: High-Level API (Recommended)

```go
package main

import (
    "net/http"
    "rivaas.dev/app"
    "rivaas.dev/router"
)

func main() {
    // Create app with observability
    app := app.New(
        app.WithServiceName("my-api"),
        app.WithMetrics(),
        app.WithTracing(),
        app.WithLogging(),
    )

    // Register routes
    app.GET("/", func(c *router.Context) {
        c.JSON(http.StatusOK, map[string]string{
            "message": "Hello from Rivaas!",
        })
    })

    // Start server with graceful shutdown
    app.Run(":8080")
}
```

### Option 2: Low-Level API

```go
package main

import (
    "net/http"
    "rivaas.dev/metrics"
    "rivaas.dev/router"
    "rivaas.dev/tracing"
)

func main() {
    // Create router with observability
    r := router.New(
        metrics.WithMetrics(),
        tracing.WithTracing(),
    )

    r.GET("/", func(c *router.Context) {
        c.JSON(http.StatusOK, map[string]string{
            "message": "Hello from Rivaas!",
        })
    })

    http.ListenAndServe(":8080", r)
}
```

## 🏗️ Architecture

```text
┌─────────────────────────────────────────────────────────────┐
│                        Rivaas Framework                     │
├─────────────────────────────────────────────────────────────┤
│  app/          │  High-level, batteries-included framework  │
├─────────────────────────────────────────────────────────────┤
│  router/       │  Low-level, high-performance HTTP router   │
├─────────────────────────────────────────────────────────────┤
│  logging/      │  Structured logging (slog)                 │
│  metrics/      │  OpenTelemetry metrics collection          │
│  tracing/      │  OpenTelemetry distributed tracing         │
└─────────────────────────────────────────────────────────────┘
```

## 📊 Performance

- **Throughput**: 6.9M+ requests/second
- **Latency**: 145ns average per request  
- **Memory**: 16 bytes per request
- **Routing**: 145ns per operation
- **Allocations**: Only 1 allocation per request

See [benchmarks](./router/router_bench_test.go) for detailed performance comparisons.

## 🔧 Configuration

### Functional Options

```go
// App configuration
app := app.New(
    app.WithServiceName("my-api"),
    app.WithVersion("v1.0.0"),
    app.WithEnvironment("production"),
    app.WithMetrics(
        metrics.WithProvider(metrics.PrometheusProvider),
        metrics.WithPort(":9090"),
    ),
    app.WithTracing(
        tracing.WithSampleRate(0.1),
        tracing.WithExcludePaths("/health"),
    ),
    app.WithLogging(
        logging.WithJSONHandler(),
    ),
    app.WithServerConfig(
        app.WithReadTimeout(15 * time.Second),
        app.WithWriteTimeout(15 * time.Second),
    ),
)
```

### Automatic Service Metadata Injection

**The app package automatically propagates service metadata** to all observability components. Set your service information once, and it's automatically injected into logging, metrics, and tracing:

```go
app := app.New(
    app.WithServiceName("my-api"),           // Set once
    app.WithServiceVersion("v1.0.0"),        // Set once
    app.WithEnvironment("production"),       // Set once
    
    // These automatically receive service metadata:
    app.WithLogging(),   // No need to pass service name/version!
    app.WithMetrics(),   // No need to pass service name/version!
    app.WithTracing(),   // No need to pass service name/version!
)

// All logs, metrics, and traces will include:
// - service.name: my-api
// - service.version: v1.0.0
// - environment: production
```

This eliminates repetitive configuration and ensures consistency across all observability signals.

**Override when needed:**

```go
app := app.New(
    app.WithServiceName("my-api"),
    app.WithServiceVersion("v1.0.0"),
    
    // Override for a specific component if needed:
    app.WithLogging(
        logging.WithServiceName("custom-logger"),  // Overrides injection
    ),
)
```

### Individual Package Configuration

When using packages independently (without the app layer):

// Logging configuration
logging.WithLogging(
    logging.WithJSONHandler(), // or WithConsoleHandler(), WithTextHandler()
    logging.WithDebugLevel(),
    logging.WithServiceName("my-api"),
    logging.WithServiceVersion("v1.0.0"),
    logging.WithEnvironment("production"),
)

// Metrics configuration
metrics.WithMetrics(
    metrics.WithProvider(metrics.PrometheusProvider),
    metrics.WithPort(":9090"),
    metrics.WithExcludePaths("/health"),
)

// Tracing configuration
tracing.WithTracing(
    tracing.WithSampleRate(0.1),
    tracing.WithExcludePaths("/health"),
    tracing.WithHeaders("Authorization"),
)
```

## 🛠️ Middleware

### Built-in Middleware

Rivaas includes several production-ready middleware components. See the [middleware documentation](./router/middleware/README.md) for details.

```go
// Add middleware to app
app.Use(app.Logger())      // Request logging
app.Use(app.Recovery())    // Panic recovery
app.Use(app.CORS())        // CORS handling
app.Use(app.RequestID())   // Request ID generation
app.Use(app.Timeout(30*time.Second)) // Request timeout
```

### Custom Middleware

```go
func AuthMiddleware() router.HandlerFunc {
    return func(c *router.Context) {
        token := c.Request.Header.Get("Authorization")
        if !isValidToken(token) {
            c.JSON(http.StatusUnauthorized, map[string]string{
                "error": "Unauthorized",
            })
            return
        }
        c.Next()
    }
}

app.Use(AuthMiddleware())
```

## 📈 Observability

### Logging

```go
// Structured logging in handlers
c.LogInfo("processing request", "user_id", userID, "action", "fetch_profile")
c.LogDebug("validation passed", "field", "email")
c.LogWarn("rate limit approaching", "requests", 950, "limit", 1000)
c.LogError("database query failed", "error", err, "query", "SELECT * FROM users")

// Access logger directly for more control
logger := c.Logger()
if logger != nil {
    logger.Info("custom log", "key", "value")
}

// Logs automatically include trace_id and span_id when tracing is enabled
// Example output: {"time":"2024-01-15T10:30:45Z","level":"INFO","msg":"processing request","user_id":"123","trace_id":"abc...","span_id":"def..."}
```

### Metrics

```go
// Custom metrics
c.IncrementCounter("orders_total",
    attribute.String("status", "success"),
)

c.RecordMetric("processing_duration", 1.5,
    attribute.String("operation", "create_order"),
)

c.SetGauge("active_connections", 42)
```

### Tracing

```go
// Add span attributes
c.SetSpanAttribute("user.id", userID)
c.SetSpanAttribute("operation", "get_user")

// Add span events
c.AddSpanEvent("database_query_started")
c.AddSpanEvent("database_query_completed")

// Get trace information
traceID := c.TraceID()
spanID := c.SpanID()
```

## 🧪 Testing

Rivaas follows comprehensive testing standards documented in [Testing Standards](./docs/TESTING_STANDARDS.md). All packages include unit tests, integration tests, benchmarks, and example tests.

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run benchmarks
go test -bench=. ./router

# Run specific benchmark
go test -bench=BenchmarkRouter -benchmem ./router
```

## 🚀 Production Deployment

### Docker

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o main .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
CMD ["./main"]
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: rivaas-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: rivaas-app
  template:
    metadata:
      labels:
        app: rivaas-app
    spec:
      containers:
      - name: app
        image: rivaas-app:latest
        ports:
        - containerPort: 8080
```

## 📚 Examples

See the [examples directories](./app/examples/) for complete, runnable examples:

### App Examples

- [Quick Start](./app/examples/01-quick-start/) - Minimal setup
- [Full Featured](./app/examples/02-full-featured/) - Complete application with all features

### Router Examples

- [Hello World](./router/examples/01-hello-world/)
- [Routing Basics](./router/examples/02-routing/)
- [Complete REST API](./router/examples/03-complete-rest-api/)
- [Middleware Stack](./router/examples/04-middleware-stack/)
- [Advanced Routing](./router/examples/05-advanced-routing/)
- [Content Negotiation](./router/examples/06-content-and-rendering/)

### Logging Examples

- [Basic Init and Levels](./logging/examples/01-basic-init-and-levels/)
- [Structured Attributes](./logging/examples/02-structured-attrs/)
- [Functional Options](./logging/examples/03-functional-options-and-validate/)
- [Dynamic Level Change](./logging/examples/04-dynamic-level-change/)
- [JSON Handler](./logging/examples/05-json-handler/)
- [Batch Logger](./logging/examples/06-batch-logger/)
- [Error with Stack](./logging/examples/07-error-with-stack/)
- [Log Duration](./logging/examples/08-log-duration/)
- [HTTP Middleware](./logging/examples/13-http-middleware/)

### Middleware Examples

- [Basic Auth](./router/middleware/examples/basic_auth/)
- [Body Limit](./router/middleware/examples/body_limit/)
- [Compression](./router/middleware/examples/compression/)
- [CORS](./router/middleware/examples/cors/)
- [Logger](./router/middleware/examples/logger/)
- [Rate Limit](./router/middleware/examples/ratelimit/)
- [Recovery](./router/middleware/examples/recovery/)
- [Request ID](./router/middleware/examples/request_id/)
- [Security Headers](./router/middleware/examples/security/)
- [Timeout](./router/middleware/examples/timeout/)

### Code Examples

#### Basic API

```go
package main

import (
    "net/http"
    "rivaas.dev/app"
    "rivaas.dev/router"
)

func main() {
    app := app.New(
        app.WithMetrics(),
        app.WithTracing(),
        app.WithLogging(),
    )

    app.GET("/users/:id", func(c *router.Context) {
        userID := c.Param("id")
        c.JSON(http.StatusOK, map[string]interface{}{
            "user_id": userID,
            "name":    "John Doe",
        })
    })

    app.POST("/users", func(c *router.Context) {
        c.JSON(http.StatusCreated, map[string]string{
            "message": "User created",
        })
    })

    app.Run(":8080")
}
```

#### With Database

```go
package main

import (
    "database/sql"
    "net/http"
    "rivaas.dev/app"
    "rivaas.dev/router"
    _ "github.com/lib/pq"
)

func main() {
    db, _ := sql.Open("postgres", "postgres://...")
    
    app := app.New(
        app.WithMetrics(),
        app.WithTracing(),
        app.WithLogging(),
    )

    app.GET("/users/:id", func(c *router.Context) {
        userID := c.Param("id")
        
        var name string
        err := db.QueryRow("SELECT name FROM users WHERE id = $1", userID).Scan(&name)
        if err != nil {
            c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
            return
        }
        
        c.JSON(http.StatusOK, map[string]interface{}{
            "user_id": userID,
            "name":    name,
        })
    })

    app.Run(":8080")
}
```

## 🔄 Migration Guide

### From Gin/Echo

```go
// Before (Gin)
r := gin.Default()
r.GET("/users/:id", func(c *gin.Context) {
    c.JSON(200, gin.H{"user_id": c.Param("id")})
})

// After (Rivaas)
app := app.New()
app.GET("/users/:id", func(c *router.Context) {
    c.JSON(http.StatusOK, map[string]string{"user_id": c.Param("id")})
})
```

### From Standard Library

```go
// Before
http.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
    // Manual parsing, JSON encoding, etc.
})

// After
app := app.New()
app.GET("/users/:id", func(c *router.Context) {
    c.JSON(http.StatusOK, map[string]string{"user_id": c.Param("id")})
})
```

## 🤝 Contributing

We welcome contributions! Please see our contributing guidelines:

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests for your changes
4. Ensure all tests pass (`go test ./...`)
5. Commit your changes (`git commit -m 'Add some amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built on top of OpenTelemetry for observability
- Inspired by the performance characteristics of modern web frameworks
- Thanks to the Go community for excellent libraries and tools

## 📞 Support

- 📖 [Router Documentation](./router/README.md)
- 📖 [App Documentation](./app/README.md)
- 📖 [Middleware Documentation](./router/middleware/README.md)
- 🐛 [Report Issues](https://github.com/rivaas-dev/rivaas/issues)
- 💬 [GitHub Discussions](https://github.com/rivaas-dev/rivaas/discussions)

---

Made with ❤️ for the Go community
