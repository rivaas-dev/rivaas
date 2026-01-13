# Metrics

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/metrics.svg)](https://pkg.go.dev/rivaas.dev/metrics)
[![Go Report Card](https://goreportcard.com/badge/rivaas.dev/metrics)](https://goreportcard.com/report/rivaas.dev/metrics)
[![Coverage](https://codecov.io/gh/rivaas-dev/rivaas/branch/main/graph/badge.svg?component=module_metrics)](https://codecov.io/gh/rivaas-dev/rivaas)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../LICENSE)

A metrics collection package for Go applications using OpenTelemetry. This package provides metrics functionality with support for multiple exporters including Prometheus, OTLP, and stdout.

## Features

- **Multiple Providers**: Prometheus, OTLP, and stdout exporters
- **Built-in HTTP Metrics**: Request duration, count, active requests, and more
- **Custom Metrics**: Support for counters, histograms, and gauges with error handling
- **Thread-Safe**: All methods are safe for concurrent use
- **Context Support**: All metrics methods accept context for cancellation
- **Structured Logging**: Pluggable logger interface for error and warning messages
- **HTTP Middleware**: Integration with any HTTP framework
- **Security**: Automatic filtering of sensitive headers

## Installation

```bash
go get rivaas.dev/metrics
```

Requires Go 1.25+

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os/signal"
    "time"
    
    "rivaas.dev/metrics"
)

func main() {
    // Create context for application lifecycle
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    // Create metrics recorder with Prometheus
    recorder, err := metrics.New(
        metrics.WithPrometheus(":9090", "/metrics"),
        metrics.WithServiceName("my-api"),
        metrics.WithServiceVersion("v1.0.0"),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // Start metrics server (required for Prometheus, OTLP)
    if err := recorder.Start(ctx); err != nil {
        log.Fatal(err)
    }
    
    // Ensure metrics are flushed on exit
    defer func() {
        shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer shutdownCancel()
        if err := recorder.Shutdown(shutdownCtx); err != nil {
            log.Printf("Metrics shutdown error: %v", err)
        }
    }()

    // Create HTTP handler with metrics middleware
    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`{"message": "Hello"}`))
    })

    // Wrap with metrics middleware (with optional path exclusions)
    handler := metrics.Middleware(recorder,
        metrics.WithExcludePaths("/health", "/metrics"),
    )(mux)

    log.Fatal(http.ListenAndServe(":8080", handler))
}
```

### Standalone Usage

```go
package main

import (
    "context"
    "log"
    "os/signal"
    
    "rivaas.dev/metrics"
    "go.opentelemetry.io/otel/attribute"
)

func main() {
    // Create context for application lifecycle
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    // Create metrics recorder with Prometheus
    recorder := metrics.MustNew(
        metrics.WithPrometheus(":9090", "/metrics"),
        metrics.WithServiceName("my-service"),
    )
    
    // Start metrics server
    if err := recorder.Start(ctx); err != nil {
        log.Fatal(err)
    }
    
    defer recorder.Shutdown(context.Background())

    // Record custom metrics with error handling
    if err := recorder.RecordHistogram(ctx, "processing_duration", 1.5,
        attribute.String("operation", "create_user"),
    ); err != nil {
        log.Printf("metrics error: %v", err)
    }
    
    // Or fire-and-forget (ignore errors)
    _ = recorder.IncrementCounter(ctx, "requests_total",
        attribute.String("status", "success"),
    )
    
    _ = recorder.SetGauge(ctx, "active_connections", 42)
}
```

## Configuration Options

### Provider Options

The recommended way to configure providers is with composite options:

```go
// Prometheus (recommended for production)
recorder := metrics.MustNew(
    metrics.WithPrometheus(":9090", "/metrics"),
    metrics.WithServiceName("my-service"),
)

// OTLP (for OpenTelemetry collectors)
recorder := metrics.MustNew(
    metrics.WithOTLP("http://localhost:4318"),
    metrics.WithServiceName("my-service"),
)

// Stdout (for development/debugging)
recorder := metrics.MustNew(
    metrics.WithStdout(),
    metrics.WithServiceName("my-service"),
)
```

**Note**: Only one provider option can be used. Using multiple provider options (e.g., `WithPrometheus` and `WithStdout` together) will result in a validation error.

#### Provider Initialization Timing

Different providers initialize at different times:

- **Prometheus**: Initialized immediately in `New()`, HTTP server starts when `Start(ctx)` is called
- **OTLP**: Initialization deferred to `Start(ctx)` to use lifecycle context for network connections
- **Stdout**: Initialized immediately in `New()`, works without calling `Start()`

**Important for OTLP**: You must call `Start(ctx)` before recording metrics, or metrics will be silently dropped until `Start()` is called. The lifecycle context enables proper graceful shutdown of network connections to the OTLP collector.

### Service Configuration

```go
metrics.WithServiceName("my-service")
metrics.WithServiceVersion("v1.0.0")
```

### Prometheus-Specific Options

```go
metrics.WithStrictPort()            // Fail if port unavailable (recommended for production)
metrics.WithServerDisabled()        // Disable auto-server
```

#### Port Configuration Behavior

**By default**, if the requested port is unavailable, the metrics server will automatically find the next available port (up to 100 ports searched). This is convenient for development but can be problematic in production.

**For production**, use `WithStrictPort()` to ensure the metrics server uses the exact port specified:

```go
// Production: Fail if port 9090 is not available
recorder := metrics.MustNew(
    metrics.WithPrometheus(":9090", "/metrics"),
    metrics.WithStrictPort(),  // Recommended for production
)
```

To manually serve metrics when using `WithServerDisabled()`:

```go
recorder := metrics.MustNew(
    metrics.WithPrometheus(":9090", "/metrics"),
    metrics.WithServerDisabled(),
)

// Get the handler (only works with Prometheus provider)
handler, err := recorder.Handler()
if err != nil {
    log.Fatalf("Failed to get metrics handler: %v", err)
}

// Serve on your own server
http.Handle("/metrics", handler)
http.ListenAndServe(":8080", nil)
```

### Histogram Bucket Configuration

```go
// Custom histogram buckets for request duration
metrics.WithDurationBuckets(0.001, 0.01, 0.1, 0.5, 1, 5, 10)

// Custom histogram buckets for request/response sizes
metrics.WithSizeBuckets(100, 1000, 10000, 100000, 1000000)
```

### Advanced Options

```go
// Use stdlib slog for logging internal events
metrics.WithLogger(slog.Default())

// Custom event handler for advanced use cases
metrics.WithEventHandler(func(e metrics.Event) {
    if e.Type == metrics.EventError {
        sentry.CaptureMessage(e.Message)
    }
    slog.Default().Info(e.Message, e.Args...)
})

// Set custom metrics limit (default: 1000)
metrics.WithMaxCustomMetrics(1000)

// Set export interval for OTLP and stdout (default: 30s)
// Note: Only affects push-based providers, not Prometheus
metrics.WithExportInterval(10 * time.Second)
```

#### Lifecycle Management

For proper initialization and shutdown, especially with OTLP provider:

```go
// Create context for application lifecycle
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
defer cancel()

recorder, err := metrics.New(
    metrics.WithOTLP("http://localhost:4318"),
    metrics.WithServiceName("my-api"),
)
if err != nil {
    log.Fatal(err)
}

// Start with lifecycle context (required for OTLP)
if err := recorder.Start(ctx); err != nil {
    log.Fatal(err)
}

// Ensure graceful shutdown
defer func() {
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer shutdownCancel()
    if err := recorder.Shutdown(shutdownCtx); err != nil {
        log.Printf("Metrics shutdown error: %v", err)
    }
}()
```

**Why `Start()` is important:**
- **OTLP provider**: Requires lifecycle context for network connections and graceful shutdown
- **Prometheus provider**: Starts the HTTP metrics server
- **Stdout provider**: Works without `Start()`, but calling it is harmless

#### Force Flush Metrics

For push-based providers (OTLP, stdout), you can force immediate export of pending metrics:

```go
// Before critical operation or deployment
if err := recorder.ForceFlush(ctx); err != nil {
    log.Printf("Failed to flush metrics: %v", err)
}
```

This is useful for:
- Ensuring metrics are exported before deployment
- Checkpointing during long-running operations
- Guaranteeing metrics visibility before shutdown

**Note**: For Prometheus (pull-based), this is typically a no-op as metrics are collected on-demand when scraped.

## Built-in Metrics

The package automatically collects the following HTTP metrics:

- `http_request_duration_seconds` - Request duration histogram
- `http_requests_total` - Total request count
- `http_requests_active` - Active request count
- `http_request_size_bytes` - Request size histogram
- `http_response_size_bytes` - Response size histogram
- `http_errors_total` - Error count
- `custom_metric_failures_total` - Custom metric creation failures

## Custom Metrics

Custom metric names must follow OpenTelemetry conventions:

- Start with a letter (a-z, A-Z)
- Contain only alphanumeric characters, underscores, dots, and hyphens
- Maximum 255 characters
- **Cannot use reserved prefixes**: `__` (Prometheus), `http_`, `router_`

### Counters

```go
// With error handling
if err := recorder.IncrementCounter(ctx, "orders_total",
    attribute.String("status", "success"),
    attribute.String("type", "online"),
); err != nil {
    log.Printf("metrics error: %v", err)
}

// Fire-and-forget
_ = recorder.IncrementCounter(ctx, "events_total")

// Add specific value
_ = recorder.AddCounter(ctx, "bytes_processed", 1024)
```

### Histograms

```go
_ = recorder.RecordHistogram(ctx, "order_processing_duration_seconds", 2.5,
    attribute.String("currency", "USD"),
    attribute.String("payment_method", "card"),
)
```

### Gauges

```go
_ = recorder.SetGauge(ctx, "active_connections", 42,
    attribute.String("service", "api"),
)
```

### Monitoring Custom Metrics

Track how many custom metrics have been created:

```go
count := recorder.CustomMetricCount()
log.Printf("Custom metrics created: %d/%d", count, maxLimit)
```

This is useful for:
- Monitoring metric cardinality
- Debugging metric limit issues
- Capacity planning

### Naming Best Practices

#### Good custom metric names

```go
_ = recorder.IncrementCounter(ctx, "orders_processed_total")
_ = recorder.RecordHistogram(ctx, "payment_processing_duration_seconds", 1.5)
_ = recorder.SetGauge(ctx, "active_websocket_connections", 42)
```

#### Invalid metric names (will return error)

```go
recorder.IncrementCounter(ctx, "__internal_metric")     // Reserved: __ prefix
recorder.RecordHistogram(ctx, "http_custom_duration", 1.0) // Reserved: http_ prefix
recorder.SetGauge(ctx, "router_custom_gauge", 10)       // Reserved: router_ prefix
recorder.IncrementCounter(ctx, "1st_metric")            // Invalid: starts with number
```

## Middleware Usage

For standalone HTTP integration (without the app package):

```go
import "rivaas.dev/metrics"

recorder := metrics.MustNew(
    metrics.WithPrometheus(":9090", "/metrics"),
    metrics.WithServiceName("my-api"),
)
defer recorder.Shutdown(context.Background())

mux := http.NewServeMux()
mux.HandleFunc("/", yourHandler)
mux.HandleFunc("/health", healthHandler)

// Create middleware with options
handler := metrics.Middleware(recorder,
    // Exclude paths from metrics collection
    metrics.WithExcludePaths("/health", "/metrics", "/ready"),
    // Exclude path prefixes
    metrics.WithExcludePrefixes("/debug/", "/internal/"),
    // Exclude paths matching regex patterns
    metrics.WithExcludePatterns(`^/v[0-9]+/internal/.*`),
    // Record specific headers as metric attributes
    // (sensitive headers like Authorization, Cookie are auto-filtered)
    metrics.WithHeaders("X-Request-ID", "X-Correlation-ID"),
)(mux)

http.ListenAndServe(":8080", handler)
```

### Middleware Options

| Option | Description |
|--------|-------------|
| `WithExcludePaths(paths...)` | Exclude exact paths from metrics |
| `WithExcludePrefixes(prefixes...)` | Exclude path prefixes (e.g., `/debug/`) |
| `WithExcludePatterns(patterns...)` | Exclude paths matching regex patterns |
| `WithHeaders(headers...)` | Record specific headers as attributes |

## Security

### Sensitive Header Filtering

When using `WithHeaders()` in middleware options, sensitive headers are automatically filtered out to prevent accidental exposure in metrics:

- `Authorization`
- `Cookie`
- `Set-Cookie`
- `X-API-Key`
- `X-Auth-Token`
- `Proxy-Authorization`
- `WWW-Authenticate`

```go
// Only X-Request-ID will be recorded; Authorization and Cookie are filtered
handler := metrics.Middleware(recorder,
    metrics.WithHeaders("Authorization", "X-Request-ID", "Cookie"),
)(mux)
```

## Behavior

- **Thread-Safe**: All methods are safe for concurrent use
- **Configurable Limits**: Set maximum custom metrics to prevent unbounded metric creation
- **Idempotent Shutdown**: Safe to call `Shutdown()` multiple times

## Important Limitations

### Global State (Non-Issue with Current Design!)

**✅ GOOD NEWS**: By default, the metrics package does NOT set the global OpenTelemetry meter provider.

This means:

- **Multiple Recorder instances can coexist** in the same process without conflicts
- The global meter provider is only set if you explicitly opt-in with `WithGlobalMeterProvider()`
- You can use custom meter providers for complete control

**Default Behavior (Recommended)**

Multiple independent configurations work out of the box:

```go
// Create independent metrics recorders (no global state!)
recorder1, _ := metrics.New(
    metrics.WithPrometheus(":9090", "/metrics"),
    metrics.WithServiceName("service-1"),
)

recorder2, _ := metrics.New(
    metrics.WithStdout(),
    metrics.WithServiceName("service-2"),
)

// ✅ Both work independently without conflicts!
defer recorder1.Shutdown(context.Background())
defer recorder2.Shutdown(context.Background())
```

**Opt-in to Global Registration**

If you need the meter provider to be globally registered:

```go
recorder := metrics.MustNew(
    metrics.WithPrometheus(":9090", "/metrics"),
    metrics.WithServiceName("my-service"),
    metrics.WithGlobalMeterProvider(),  // ✅ Explicit opt-in
)
```

**When to use `WithGlobalMeterProvider()`:**
- You want OpenTelemetry instrumentation libraries to use your metrics provider
- You need `otel.GetMeterProvider()` to return your provider
- You're integrating with third-party libraries that expect a global meter provider

**When NOT to use it:**
- You have multiple services in the same process (e.g., microservices in tests)
- You want to avoid global state
- You're managing your own meter provider lifecycle

## Testing

The package provides testing utilities for unit tests:

```go
import "rivaas.dev/metrics"

func TestMyHandler(t *testing.T) {
    t.Parallel()
    
    // Create test recorder (uses stdout, avoids port conflicts)
    recorder := metrics.TestingRecorder(t, "test-service")
    
    // Use recorder in tests...
    // Cleanup is automatic via t.Cleanup()
}

func TestWithPrometheus(t *testing.T) {
    t.Parallel()
    
    // Create test recorder with Prometheus (dynamic port)
    recorder := metrics.TestingRecorderWithPrometheus(t, "test-service")
    
    // Wait for server to be ready
    err := metrics.WaitForMetricsServer(t, recorder.ServerAddress(), 5*time.Second)
    if err != nil {
        t.Fatal(err)
    }
    
    // Test metrics endpoint...
}
```

See `testing.go` for more utilities.

## Troubleshooting

### Metrics not appearing (OTLP)

- Ensure you called `recorder.Start(ctx)` before recording metrics
- Check OTLP collector is reachable at the configured endpoint
- Verify export interval hasn't expired (default: 30s)
- Use `recorder.ForceFlush(ctx)` to immediately export pending metrics

### Port conflicts (Prometheus)

- Use `WithStrictPort()` to fail fast instead of auto-discovering alternative ports
- Check if another service is using the port: `lsof -i :9090`
- Use `recorder.ServerAddress()` to see the actual port used
- In tests, use `TestingRecorderWithPrometheus()` for automatic port allocation

### Custom metric limit reached

- Increase limit with `WithMaxCustomMetrics(n)` (default: 1000)
- Review metric cardinality - too many unique label combinations create separate metrics
- Use `recorder.CustomMetricCount()` to monitor current usage
- Consider using fewer labels or more general label values

### Metrics server not starting

- Check if `Start(ctx)` was called (required for Prometheus and OTLP)
- Verify the context passed to `Start()` is not already canceled
- Check logs via `WithLogger(slog.Default())` for detailed error messages
- For Prometheus, ensure the port is available or use `WithStrictPort()` for explicit errors

## Examples

See the `examples/` directory for complete working examples:

- `standalone/` - Standalone metrics usage
- Integration examples in the main router examples

## API Reference

For detailed API documentation, see [pkg.go.dev/rivaas.dev/metrics](https://pkg.go.dev/rivaas.dev/metrics).

## Contributing

Contributions are welcome! Please see the [main repository](../) for contribution guidelines.

## License

Apache License 2.0 - see [LICENSE](../LICENSE) for details.

---

Part of the [Rivaas](https://github.com/rivaas-dev/rivaas) web framework ecosystem.
