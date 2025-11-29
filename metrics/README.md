# Rivaas Metrics

A comprehensive metrics collection package for Go applications using OpenTelemetry. This package provides easy-to-use metrics functionality with support for multiple exporters including Prometheus, OTLP, and stdout.

## Features

- **Multiple Providers**: Prometheus, OTLP, and stdout exporters
- **Built-in HTTP Metrics**: Request duration, count, active requests, and more
- **Custom Metrics**: Support for counters, histograms, and gauges with error handling
- **Thread-Safe**: RWMutex-based operations for optimal performance
- **Context Support**: All metrics methods accept context for cancellation
- **Structured Logging**: Pluggable logger interface for error and warning messages
- **HTTP Middleware**: Easy integration with any HTTP framework
- **Security**: Automatic filtering of sensitive headers
- **Memory Optimized**: Pre-allocated slices and efficient memory usage

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "log"
    "net/http"
    "time"
    
    "rivaas.dev/metrics"
)

func main() {
    // Create metrics recorder with Prometheus
    recorder, err := metrics.New(
        metrics.WithPrometheus(":9090", "/metrics"),
        metrics.WithServiceName("my-api"),
        metrics.WithServiceVersion("v1.0.0"),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // Ensure metrics are flushed on exit
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if err := recorder.Shutdown(ctx); err != nil {
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
    
    "rivaas.dev/metrics"
    "go.opentelemetry.io/otel/attribute"
)

func main() {
    // Create metrics recorder with Prometheus
    recorder := metrics.MustNew(
        metrics.WithPrometheus(":9090", "/metrics"),
        metrics.WithServiceName("my-service"),
    )
    defer recorder.Shutdown(context.Background())
    
    ctx := context.Background()

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

metrics.WithMaxCustomMetrics(1000)  // Set custom metrics limit
```

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

## Performance

- **Thread-Safe**: Uses RWMutex for efficient concurrent access
- **Memory Efficient**: Minimal allocations during request processing
- **Configurable Limits**: Set maximum custom metrics to prevent memory leaks
- **Provider-Specific Optimizations**: Each provider is optimized for its use case
- **Idempotent Operations**: Safe to call `Shutdown()` multiple times

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

## Examples

See the `examples/` directory for complete working examples:

- `standalone/` - Standalone metrics usage
- Integration examples in the main router examples

## License

MIT License - see LICENSE file for details.
