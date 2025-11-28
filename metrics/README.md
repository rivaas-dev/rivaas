# Rivaas Metrics

A comprehensive metrics collection package for Go applications using OpenTelemetry. This package provides easy-to-use metrics functionality with support for multiple exporters including Prometheus, OTLP, and stdout.

## Features

- **Multiple Providers**: Prometheus, OTLP, and stdout exporters
- **Built-in HTTP Metrics**: Request duration, count, active requests, and more
- **Custom Metrics**: Support for counters, histograms, and gauges
- **Thread-Safe**: Atomic operations for optimal performance
- **Context Support**: All metrics methods accept context for cancellation and timeout support
- **Structured Logging**: Pluggable logger interface for error and warning messages
- **Router Integration**: Seamless integration with Rivaas router
- **Environment Configuration**: Automatic configuration from environment variables
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
    "rivaas.dev/router"
)

func main() {
    // Create metrics config
    metricsConfig := metrics.New(
        metrics.WithServiceName("my-api"),
        metrics.WithServiceVersion("v1.0.0"),
    )
    
    // Ensure metrics are flushed on exit
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if err := metricsConfig.Shutdown(ctx); err != nil {
            log.Printf("Metrics shutdown error: %v", err)
        }
    }()

    // Create router with metrics
    r := router.New()
    r.SetMetricsRecorder(metricsConfig)

    r.GET("/", func(c *router.Context) {
        c.JSON(http.StatusOK, map[string]string{"message": "Hello"})
    })

    log.Fatal(http.ListenAndServe(":8080", r))
}
```

### Standalone Usage

```go
package main

import (
    "context"
    
    "rivaas.dev/metrics"
    "go.opentelemetry.io/otel/attribute"
)

func main() {
    // Create metrics configuration
    config := metrics.New(
        metrics.WithServiceName("my-service"),
        metrics.WithProvider(metrics.PrometheusProvider),
    )
    
    ctx := context.Background()

    // Record custom metrics
    config.RecordMetric(ctx, "processing_duration", 1.5,
        attribute.String("operation", "create_user"),
    )
    
    config.IncrementCounter(ctx, "requests_total",
        attribute.String("status", "success"),
    )
    
    config.SetGauge(ctx, "active_connections", 42)
}
```

## Configuration Options

### Provider Options

Configure the provider **before** calling `New()`:

```go
// Prometheus (default)
metrics.New(
    metrics.WithProvider(metrics.PrometheusProvider),
    metrics.WithServiceName("my-service"),
)

// OTLP
metrics.New(
    metrics.WithProvider(metrics.OTLPProvider),
    metrics.WithOTLPEndpoint("http://localhost:4318"),
    metrics.WithServiceName("my-service"),
)

// Stdout (for development)
metrics.New(
    metrics.WithProvider(metrics.StdoutProvider),
    metrics.WithServiceName("my-service"),
)
```

### Service Configuration

```go
metrics.WithServiceName("my-service")
metrics.WithServiceVersion("v1.0.0")
```

### Prometheus-Specific Options

```go
metrics.WithPort(":9090")           // Metrics server port (default :9090)
metrics.WithPath("/metrics")        // Metrics endpoint path (default /metrics)
metrics.WithStrictPort()            // Fail if port unavailable (recommended for production)
metrics.WithServerDisabled()        // Disable auto-server
```

#### Port Configuration Behavior

**By default**, if the requested port is unavailable, the metrics server will automatically find the next available port (up to 100 ports searched). This is convenient for development but can be problematic in production.

**For production**, use `WithStrictPort()` to ensure the metrics server uses the exact port specified:

```go
// Production: Fail if port 9090 is not available
config := metrics.New(
    metrics.WithPort(":9090"),
    metrics.WithStrictPort(),  // Recommended for production
)
```

If the port is unavailable with `WithStrictPort()`, initialization will log an error and the metrics server won't start (metrics recording still works).

**Without strict mode**, if auto-discovery occurs, a **WARNING** is logged:

```text
WARN: Metrics server using different port than requested
  actual_address=:9091/metrics
  requested_port=:9090
  recommendation=use WithStrictPort() to fail instead of auto-discovering
```

To manually serve metrics when using `WithServerDisabled()`:

```go
config := metrics.New(
    metrics.WithProvider(metrics.PrometheusProvider),
    metrics.WithServerDisabled(),
)

// Get the handler (only works with Prometheus provider)
handler, err := config.GetHandler()
if err != nil {
    log.Fatalf("Failed to get metrics handler: %v", err)
}

// Serve on your own server
http.Handle("/metrics", handler)
http.ListenAndServe(":8080", nil)
```

### Filtering Options

```go
metrics.WithExcludePaths("/health", "/metrics")  // Exclude paths
metrics.WithHeaders("Authorization")             // Record headers
metrics.WithDisableParams()                      // Disable URL params
```

### Advanced Options

```go
// Use stdlib slog for logging internal events
metrics.WithLogger(slog.Default())               // Use default slog logger

// Or create a custom slog logger
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
metrics.WithLogger(logger)

// Custom event handler for advanced use cases (e.g., send errors to Sentry)
metrics.WithEventHandler(func(e metrics.Event) {
    if e.Type == metrics.EventError {
        sentry.CaptureMessage(e.Message)
    }
    slog.Default().Info(e.Message, e.Args...)
})

metrics.WithMaxCustomMetrics(1000)               // Set custom metrics limit
```

## Built-in Metrics

The package automatically collects the following HTTP metrics:

- `http_request_duration_seconds` - Request duration histogram
- `http_requests_total` - Total request count
- `http_requests_active` - Active request count
- `http_request_size_bytes` - Request size histogram
- `http_response_size_bytes` - Response size histogram
- `http_errors_total` - Error count
- `http_routes_total` - Route registration count
- `http_constraint_failures_total` - Route constraint validation failures
- `router_context_pool_hits_total` - Context pool reuse hits
- `router_context_pool_misses_total` - Context pool allocation misses
- `router_custom_metric_failures_total` - Custom metric creation failures
- `router_metrics_cas_retries_total` - CAS retry attempts (contention indicator)

## Custom Metrics

Custom metric names must follow OpenTelemetry conventions:

- Start with a letter (a-z, A-Z)
- Contain only alphanumeric characters, underscores, dots, and hyphens
- Maximum 255 characters
- **Cannot use reserved prefixes**: `__` (Prometheus), `http_`, `router_`

### Counters

```go
c.IncrementCounter("orders_total",
    attribute.String("status", "success"),
    attribute.String("type", "online"),
)
```

### Histograms

```go
c.RecordMetric("order_processing_duration_seconds", 2.5,
    attribute.String("currency", "USD"),
    attribute.String("payment_method", "card"),
)
```

### Gauges

```go
c.SetGauge("active_connections", 42,
    attribute.String("service", "api"),
)
```

### Naming Best Practices

#### Good custom metric names

```go
config.IncrementCounter(ctx, "orders_processed_total")
config.RecordMetric(ctx, "payment_processing_duration_seconds", 1.5)
config.SetGauge(ctx, "active_websocket_connections", 42)
```

#### Invalid metric names (will be rejected)

```go
config.IncrementCounter(ctx, "__internal_metric")     // Reserved: __ prefix
config.RecordMetric(ctx, "http_custom_duration", 1.0) // Reserved: http_ prefix
config.SetGauge(ctx, "router_custom_gauge", 10)       // Reserved: router_ prefix
config.IncrementCounter(ctx, "1st_metric")            // Invalid: starts with number
```

## Middleware Usage

For manual integration with other HTTP frameworks:

```go
import "rivaas.dev/metrics"

config := metrics.New(
    metrics.WithServiceName("my-service"),
)

// Create middleware
middleware := metrics.Middleware(config)

// Use with any http.Handler
http.Handle("/", middleware(yourHandler))
```

## Performance

- **Thread-Safe**: Uses atomic operations for lock-free performance
- **Memory Efficient**: Minimal allocations during request processing
- **Configurable Limits**: Set maximum custom metrics to prevent memory leaks
- **Provider-Specific Optimizations**: Each provider is optimized for its use case
- **Double-Checked Locking**: Optimized custom metric creation avoids unnecessary work
- **Idempotent Operations**: Safe to call `Shutdown()` multiple times

### Performance Characteristics

#### Lock-Free Custom Metrics

The package uses a Compare-And-Swap (CAS) based approach for managing custom metrics, which provides excellent performance under normal conditions:

- **Low contention**: Single atomic operation, extremely fast
- **Moderate contention**: Automatic retry with exponential backoff
- **High contention**: After 100 retries, falls back to logging (prevents infinite loops)

#### When High Contention Might Occur

High contention on custom metric creation is rare but can happen when:

- Many goroutines simultaneously create **new, unique** metrics (not incrementing existing ones)
- Metric names are dynamically generated with high cardinality
- Application startup creates many metrics concurrently

Under extreme contention (>100 failed CAS attempts), the operation will fail gracefully and increment `router_custom_metric_failures_total`.

#### Monitoring Contention

The package exposes `router_metrics_cas_retries_total` to track CAS retry attempts. Monitor this metric to detect contention:

- **Low values (< 100/sec)**: Normal operation, lock-free design working well
- **Medium values (100-1000/sec)**: Some contention, but within acceptable limits
- **High values (> 1000/sec)**: Significant contention, consider:
  - Reducing metric name cardinality
  - Pre-creating metrics at startup
  - Investigating if many goroutines create unique metrics concurrently

Example Prometheus alert:

```yaml
- alert: HighMetricsCASContention
  expr: rate(router_metrics_cas_retries_total[5m]) > 1000
  for: 5m
  annotations:
    summary: High CAS contention in metrics package
    description: CAS retry rate is {{ $value }}/sec, indicating lock contention
```

#### Memory Trade-offs

The CAS-based approach creates temporary map copies during updates. Under high contention:

- Failed CAS attempts create discarded map copies (GC pressure)
- Reads remain extremely fast (just pointer load + dereference)
- Trade-off: Lower latency and no lock contention vs. potential GC pressure

For most applications, this trade-off strongly favors the lock-free approach. If you observe high `router_metrics_cas_retries_total` (>1000/sec sustained), high `router_custom_metric_failures_total`, or GC pressure from metric creation, consider:

1. **Reducing metric name cardinality** - Avoid dynamically generated metric names with unbounded cardinality
2. **Pre-creating metrics at startup** - Create all expected metrics during initialization instead of on-demand
3. **Using a smaller `maxCustomMetrics` limit** - Prevents unbounded metric creation
4. **Future: Mutex-based alternative** - A `WithMutexBasedMetrics()` option may be added for extreme contention scenarios (not yet implemented)

## Important Limitations

### Global State (Non-Issue with Current Design!)

**✅ GOOD NEWS**: By default, the metrics package does NOT set the global OpenTelemetry meter provider.

This means:

- **Multiple metrics configurations can coexist** in the same process without conflicts
- The global meter provider is only set if you explicitly opt-in with `WithGlobalMeterProvider()`
- You can use custom meter providers for complete control

**Default Behavior (Recommended)**

Multiple independent configurations work out of the box:

```go
// Create independent metrics configurations (no global state!)
config1, _ := metrics.New(
    metrics.WithServiceName("service-1"),
    metrics.WithProvider(metrics.PrometheusProvider),
    metrics.WithPort(":9090"),
)

config2, _ := metrics.New(
    metrics.WithServiceName("service-2"),
    metrics.WithProvider(metrics.StdoutProvider),
)

// ✅ Both work independently without conflicts!
// ✅ No global state, no overwriting
defer config1.Shutdown(context.Background())
defer config2.Shutdown(context.Background())
```

**Opt-in to Global Registration**

If you need the meter provider to be globally registered (e.g., for third-party libraries that use the global OpenTelemetry provider):

```go
config := metrics.New(
    metrics.WithServiceName("my-service"),
    metrics.WithProvider(metrics.PrometheusProvider),
    metrics.WithGlobalMeterProvider(),  // ✅ Explicit opt-in
)
```

**Advanced: Custom Meter Provider**

For complete control over the meter provider lifecycle:

```go
// Create your own meter provider
provider := sdkmetric.NewMeterProvider(...)

config := metrics.New(
    metrics.WithMeterProvider(provider),  // Use custom provider
    metrics.WithServiceName("service-1"),
)

// Manage provider lifecycle yourself
defer provider.Shutdown(context.Background())
```

**When to use each approach:**

**Default (non-global):**
- ✅ Multiple services in one process
- ✅ Testing with isolated metrics
- ✅ Library code that shouldn't affect globals
- ✅ Most applications (recommended)

**Global registration (`WithGlobalMeterProvider`):**
- ✅ Single service application
- ✅ Integration with third-party OpenTelemetry libraries
- ✅ Existing code expects global provider

**Custom provider (`WithMeterProvider`):**
- ✅ Full control over provider lifecycle
- ✅ Complex multi-provider setups
- ✅ Integration with existing OpenTelemetry infrastructure

### Context Cancellation

All metrics recording methods (`RecordMetric`, `IncrementCounter`, `SetGauge`) respect context cancellation. If the provided context is cancelled, the operation returns early without recording the metric. This prevents unnecessary work during shutdown or request cancellation.

## Monitoring and Observability

### Key Metrics to Watch

Monitor these internal metrics to ensure healthy operation:

#### Performance Indicators

- `router_metrics_cas_retries_total` - **Most important for detecting contention**
  - Rate > 1000/sec sustained = High contention, investigate metric cardinality
  - Rate 100-1000/sec = Moderate contention, acceptable for most workloads  
  - Rate < 100/sec = Low contention, optimal performance

- `router_custom_metric_failures_total` - Metric creation failures
  - Should be zero in normal operation
  - Non-zero indicates hitting `maxCustomMetrics` limit or CAS max retries

#### Efficiency Indicators

- `router_context_pool_hits_total` / `router_context_pool_misses_total`
  - High hit ratio (> 90%) = Good pool efficiency
  - Low hit ratio = Consider increasing pool size or investigating allocation patterns

### Alerting Examples

**Prometheus Alerting Rules**:

```yaml
groups:
  - name: rivaas_metrics
    rules:
      # Alert on high CAS contention
      - alert: HighMetricsCASContention
        expr: rate(router_metrics_cas_retries_total[5m]) > 1000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: High CAS retry rate in metrics package
          description: |
            CAS retry rate is {{ $value | humanize }}/sec.
            This indicates high contention on custom metric creation.
            Consider reducing metric name cardinality or pre-creating metrics.
      
      # Alert on metric creation failures
      - alert: MetricsCreationFailures
        expr: increase(router_custom_metric_failures_total[5m]) > 0
        for: 1m
        labels:
          severity: error
        annotations:
          summary: Custom metrics failing to create
          description: |
            {{ $value }} metric creation failures in the last 5 minutes.
            Check if maxCustomMetrics limit is too low or if there's a bug.
      
      # Alert on low context pool efficiency
      - alert: LowContextPoolEfficiency
        expr: |
          (
            rate(router_context_pool_hits_total[5m]) / 
            (rate(router_context_pool_hits_total[5m]) + rate(router_context_pool_misses_total[5m]))
          ) < 0.7
        for: 10m
        labels:
          severity: info
        annotations:
          summary: Context pool hit rate below 70%
          description: Pool efficiency is {{ $value | humanizePercentage }}
```

### Grafana Dashboard Queries

**CAS Contention Panel**:

```promql
rate(router_metrics_cas_retries_total[5m])
```

**Metric Creation Success Rate**:

```promql
sum(rate(router_custom_metric_failures_total[5m])) / 
sum(rate(router_custom_metric_failures_total[5m]) + rate(router_metrics_cas_retries_total[5m]))
```

**Context Pool Efficiency**:

```promql
rate(router_context_pool_hits_total[5m]) / 
(rate(router_context_pool_hits_total[5m]) + rate(router_context_pool_misses_total[5m]))
```

## Examples

See the `examples/` directory for complete working examples:

- `standalone/` - Standalone metrics usage
- Integration examples in the main router examples

## License

MIT License - see LICENSE file for details.
