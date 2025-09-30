# Rivaas Router Examples

This directory contains practical examples demonstrating the features and capabilities of the Rivaas router.

## 📚 Examples Overview

The examples are organized in a progressive learning path:

### 1. [Hello World](./01-hello-world/)

**Start here!** The simplest possible Rivaas application.

```bash
cd 01-hello-world && go run main.go
curl http://localhost:8080/
```

**Learn:** Basic router setup, simple JSON responses

---

### 2. [Routing](./02-routing/)

Routes, parameters, HTTP methods, and route groups.

```bash
cd 02-routing && go run main.go
curl http://localhost:8080/users/123
```

**Learn:** Path parameters, route groups, nested groups, HTTP methods (GET, POST, PUT, DELETE)

---

### 3. [Middleware](./03-middleware/)

Common middleware patterns including auth, logging, recovery, and CORS.

```bash
cd 03-middleware && go run main.go
curl -H "Authorization: Bearer token123" http://localhost:8080/api/profile
```

**Learn:** Global middleware, group middleware, middleware chaining, authentication, CORS

---

### 4. [REST API](./04-rest-api/)

Complete CRUD REST API with in-memory storage.

```bash
cd 04-rest-api && go run main.go
curl http://localhost:8080/api/v1/users
curl -X POST http://localhost:8080/api/v1/users -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com"}'
```

**Learn:** Full CRUD operations, request/response handling, error handling, JSON parsing

---

### 5. [Metrics](./05-metrics/)

Metrics collection with multiple providers (Prometheus, OTLP, Stdout).

```bash
# Prometheus (default)
cd 05-metrics && go run main.go
curl http://localhost:9090/metrics

# OTLP
METRICS_PROVIDER=otlp OTLP_ENDPOINT=http://localhost:4318 go run main.go

# Stdout (development)
METRICS_PROVIDER=stdout go run main.go
```

**Learn:** Automatic request metrics, custom metrics (counters, gauges, histograms), different providers

---

### 6. [Tracing](./06-tracing/)

Distributed tracing with OpenTelemetry.

```bash
cd 06-tracing && go run main.go
curl http://localhost:8080/users/123
# Watch console for trace output
```

**Learn:** Span attributes, events, trace IDs, OpenTelemetry integration

---

### 7. [Observability](./07-observability/)

Combined metrics and tracing for full observability.

```bash
cd 07-observability && go run main.go
curl -X POST http://localhost:8080/orders
curl http://localhost:9090/metrics
```

**Learn:** Correlating traces with metrics, trace ID in responses, comprehensive monitoring

---

### 8. [Advanced](./08-advanced/)

Advanced features: route constraints, helpers, static files, introspection.

```bash
cd 08-advanced && go run main.go
curl http://localhost:8080/users/123           # ✓ Valid (numeric)
curl http://localhost:8080/users/abc           # ✗ Invalid (not numeric)
```

**Learn:** Route validation, constraints (UUID, numeric, alpha), static files, cookie handling, introspection

---

## 🚀 Quick Start

1. **Choose an example** based on what you want to learn
2. **Navigate to the directory:**

   ```bash
   cd router/examples/01-hello-world
   ```

3. **Run the example:**

   ```bash
   go run main.go
   ```

4. **Test with curl** (commands are shown in each example's output)

## 📖 Learning Path

### For Beginners

1. Start with **01-hello-world** to understand the basics
2. Move to **02-routing** to learn about routes and parameters
3. Explore **03-middleware** for request processing
4. Build a **04-rest-api** to see everything together

### For Production

5. Add **05-metrics** for monitoring
6. Implement **06-tracing** for debugging
7. Combine with **07-observability** for full visibility
8. Use **08-advanced** features as needed

## 🔧 Common Patterns

### Creating a Router

```go
r := router.New()
```

### With Options

```go
r := router.New(
    router.WithMetrics(),
    router.WithTracing(),
    router.WithMetricsServiceName("my-api"),
)
```

### Adding Routes

```go
r.GET("/users/:id", handler)
r.POST("/users", handler)
r.PUT("/users/:id", handler)
r.DELETE("/users/:id", handler)
```

### Route Groups

```go
api := r.Group("/api/v1")
api.Use(authMiddleware)
api.GET("/users", listUsers)
```

### Middleware

```go
// Global
r.Use(logger, recovery)

// Group-specific
api.Use(authMiddleware)

// Per-route
r.GET("/admin", handler).Use(adminMiddleware)
```

### Route Constraints

```go
r.GET("/users/:id", handler).WhereNumber("id")
r.GET("/entities/:uuid", handler).WhereUUID("uuid")
r.GET("/files/:name", handler).Where("name", `[a-zA-Z0-9._-]+`)
```

## 📊 Metrics Providers

### Prometheus (Default)

```go
r := router.New(router.WithMetrics())
// Metrics at http://localhost:9090/metrics
```

### OTLP (Push to Collector)

```go
r := router.New(
    router.WithMetrics(),
    router.WithMetricsProviderOTLP("http://localhost:4318"),
)
```

### Stdout (Development)

```go
r := router.New(
    router.WithMetrics(),
    router.WithMetricsProviderStdout(),
)
```

## 🔍 Tracing

### Basic Setup

```go
r := router.New(
    router.WithTracing(),
    router.WithTracingServiceName("my-api"),
)
```

### In Handlers

```go
func handler(c *router.Context) {
    c.SetSpanAttribute("user.id", userID)
    c.AddSpanEvent("processing_started")
    
    // ... your logic ...
    
    c.JSON(http.StatusOK, map[string]string{
        "trace_id": c.TraceID(),
    })
}
```

## 🛠️ Environment Configuration

Rivaas supports environment variables for configuration:

```bash
# Service identification
export OTEL_SERVICE_NAME="my-api"
export OTEL_SERVICE_VERSION="v1.0.0"

# Metrics configuration
export RIVAAS_METRICS_PORT=":9090"
export RIVAAS_METRICS_PATH="/metrics"

# Run your app
go run main.go
```

## 🧪 Testing Examples

Each example includes curl commands in its output. Generally:

```bash
# Run the example
go run main.go

# In another terminal, test it
curl http://localhost:8080/
```

## 📝 Building Your Own

1. **Copy a similar example** as a starting point
2. **Modify routes** to match your API design
3. **Add middleware** as needed for auth, logging, etc.
4. **Enable observability** with metrics and tracing
5. **Add validation** with route constraints

## 🤝 Need Help?

- Check the [main README](../../README.md)
- Review similar examples
- Look at the router [documentation](../README.md)

## 📄 License

All examples are provided under the same license as the Rivaas project.
