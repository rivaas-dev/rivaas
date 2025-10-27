# Rivaas

A high-performance, modular web framework for Go with integrated observability. Rivaas provides both low-level building blocks and high-level batteries-included APIs for building modern web applications.

## 🚀 Features

- **High Performance**: 223K+ requests/second, 4.5µs average latency
- **Modular Design**: Use only what you need
- **Integrated Observability**: Built-in metrics and tracing
- **Memory Efficient**: Only 51 bytes memory per request
- **Graceful Shutdown**: Production-ready server management
- **Multiple APIs**: Choose between low-level or high-level interfaces

## 📦 Packages

### Core Packages

- **[router](./router/)** - High-performance HTTP router with radix tree routing
- **[app](./app/)** - Batteries-included web framework (recommended for most users)

### Observability Packages

- **[metrics](./metrics/)** - OpenTelemetry metrics with Prometheus, OTLP, and stdout support
- **[tracing](./tracing/)** - Distributed tracing with OpenTelemetry

## 🎯 Quick Start

### Option 1: High-Level API (Recommended)

```go
package main

import (
    "net/http"
    "rivass.dev/app"
    "rivass.dev/router"
)

func main() {
    // Create app with observability
    app := app.New(
        app.WithServiceName("my-api"),
        app.WithObservability(), // Enables metrics + tracing
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
    "rivass.dev/metrics"
    "rivass.dev/router"
    "rivass.dev/tracing"
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
│  metrics/      │  OpenTelemetry metrics collection          │
│  tracing/      │  OpenTelemetry distributed tracing         │
└─────────────────────────────────────────────────────────────┘
```

## 📊 Performance

- **Throughput**: 223K+ requests/second
- **Latency**: 4.5µs average per request
- **Memory**: 51 bytes per request
- **Routing**: Sub-100ns for static paths
- **Allocations**: Only 3 allocations per request

## 🔧 Configuration

### Environment Variables

```bash
# Service configuration
OTEL_SERVICE_NAME=my-service
OTEL_SERVICE_VERSION=v1.0.0

# Metrics configuration
OTEL_METRICS_EXPORTER=prometheus
RIVAAS_METRICS_PORT=:9090

# Tracing configuration
OTEL_TRACES_EXPORTER=jaeger
OTEL_EXPORTER_JAEGER_ENDPOINT=http://localhost:14268/api/traces
```

### Functional Options

```go
// App configuration
app := app.New(
    app.WithServiceName("my-api"),
    app.WithVersion("v1.0.0"),
    app.WithEnvironment("production"),
    app.WithObservability(),
    app.WithServerConfig(&app.ServerConfig{
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
    }),
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

## 🚀 Production Deployment

### Docker

```dockerfile
FROM golang:1.21-alpine AS builder
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
        env:
        - name: OTEL_SERVICE_NAME
          value: "rivaas-app"
        - name: OTEL_METRICS_EXPORTER
          value: "prometheus"
```

## 📚 Examples

### Basic API

```go
package main

import (
    "net/http"
    "rivass.dev/app"
    "rivass.dev/router"
)

func main() {
    app := app.New(app.WithObservability())

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

### With Database

```go
package main

import (
    "database/sql"
    "net/http"
    "rivass.dev/app"
    "rivass.dev/router"
    _ "github.com/lib/pq"
)

func main() {
    db, _ := sql.Open("postgres", "postgres://...")
    
    app := app.New(app.WithObservability())

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

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built on top of OpenTelemetry for observability
- Inspired by the performance characteristics of modern web frameworks
- Thanks to the Go community for excellent libraries and tools

## 📞 Support

- 📖 [Documentation](./docs/)
- 🐛 [Issues](https://github.com/rivaas-dev/rivaas/issues)
- 💬 [Discussions](https://github.com/rivaas-dev/rivaas/discussions)
- 📧 [Email](mailto:support@rivaas.dev)