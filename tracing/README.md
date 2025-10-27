# Rivaas Tracing

A distributed tracing package for Go applications using OpenTelemetry. This package provides easy-to-use tracing functionality with support for various exporters and seamless integration with HTTP frameworks.

## Features

- **OpenTelemetry Integration**: Full OpenTelemetry tracing support
- **Context Propagation**: Automatic trace context propagation
- **Span Management**: Easy span creation and management
- **Router Integration**: Seamless integration with Rivaas router
- **Custom Attributes**: Add custom attributes and events to spans
- **Path Filtering**: Exclude specific paths from tracing
- **Optional Exporter Helpers**: Built-in helpers for common exporters (with build tags)

## Prerequisites

The tracing package provides two ways to use OpenTelemetry tracing:

1. **Built-in providers** (Stdout, OTLP, Noop) - Easy setup with `New()`
2. **Custom providers** - Full control with `WithTracerProvider()` for advanced use cases

By default, the package uses built-in providers which set the global OpenTelemetry tracer provider. To avoid global state, use custom providers (see "Avoiding Global State" section below).

## Quick Start

The tracing package provides a unified API consistent with the metrics package, supporting multiple providers at runtime without build tags.

**Key Features:**

- ✅ All providers available at runtime
- ✅ No build tags required
- ✅ Switch via configuration only
- ✅ Consistent API with metrics package
- ✅ Environment variable support

### Basic Usage

```go
import (
    "context"
    "log"
    "github.com/rivaas-dev/rivaas/tracing"
)

func main() {
    // Create tracing configuration
    config, err := tracing.New(
        tracing.WithServiceName("my-service"),
        tracing.WithServiceVersion("v1.0.0"),
        tracing.WithProvider(tracing.StdoutProvider),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer config.Shutdown(context.Background())

    // Use with your application...
}
```

### Provider Options

Choose the provider that fits your needs:

#### Stdout Provider (Development)

Prints traces to stdout in pretty-printed JSON format. Great for local development and debugging.

```go
config, err := tracing.New(
    tracing.WithServiceName("my-service"),
    tracing.WithServiceVersion("v1.0.0"),
    tracing.WithProvider(tracing.StdoutProvider),
)
if err != nil {
    log.Fatal(err)
}
defer config.Shutdown(context.Background())
```

#### OTLP Provider (Production)

Sends traces to an OTLP collector (Jaeger, Tempo, cloud services). Recommended for production.

```go
config, err := tracing.New(
    tracing.WithServiceName("my-service"),
    tracing.WithServiceVersion("v1.0.0"),
    tracing.WithProvider(tracing.OTLPProvider),
    tracing.WithOTLPEndpoint("jaeger:4317"),
    tracing.WithOTLPInsecure(false),  // Use TLS in production
)
if err != nil {
    log.Fatal(err)
}
defer config.Shutdown(context.Background())
```

#### Noop Provider (Default/Testing)

No-op provider that doesn't export anything. This is the default if no provider is specified.

```go
config, err := tracing.New(
    tracing.WithServiceName("my-service"),
    tracing.WithServiceVersion("v1.0.0"),
    tracing.WithProvider(tracing.NoopProvider),
)
if err != nil {
    log.Fatal(err)
}
defer config.Shutdown(context.Background())
```

### Environment Variables

The package reads standard OpenTelemetry environment variables:

```bash
export OTEL_SERVICE_NAME="my-service"
export OTEL_SERVICE_VERSION="v1.0.0"
export OTEL_TRACES_EXPORTER="otlp"  # or "stdout", "noop"
export OTEL_EXPORTER_OTLP_ENDPOINT="localhost:4317"
```

```go
// Configuration is automatically read from environment variables
config, err := tracing.New()
if err != nil {
    log.Fatal(err)
}
defer config.Shutdown(context.Background())
```

Supported environment variables:
- `OTEL_TRACES_EXPORTER`: Provider type (`otlp`, `stdout`, `noop`)
- `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` or `OTEL_EXPORTER_OTLP_ENDPOINT`: OTLP endpoint
- `OTEL_SERVICE_NAME`: Service name
- `OTEL_SERVICE_VERSION`: Service version

### Production and Development Helpers

Pre-configured setups for common scenarios:

```go
// Production configuration: OTLP with conservative sampling
config, err := tracing.NewProduction("my-service", "v1.2.3")
if err != nil {
    log.Fatal(err)
}
defer config.Shutdown(context.Background())

// Development configuration: Stdout with full sampling
config, err := tracing.NewDevelopment("my-service", "dev")
if err != nil {
    log.Fatal(err)
}
defer config.Shutdown(context.Background())
```

### Comparison with Metrics Package

The tracing package follows the same design pattern as the metrics package:

| Aspect | Metrics Package | Tracing Package |
|--------|----------------|-----------------|
| Design Pattern | `WithProvider(PrometheusProvider)` | `WithProvider(StdoutProvider)` |
| Constructor | `New(opts...Option) (*Config, error)` | `New(opts...Option) (*Config, error)` |
| Panic Version | `MustNew(opts...Option) *Config` | `MustNew(opts...Option) *Config` |
| Shutdown | `config.Shutdown(ctx) error` | `config.Shutdown(ctx) error` |
| Runtime Selection | ✅ Yes | ✅ Yes |
| Build Tags | ❌ No | ❌ No |
| Environment Variables | ✅ Yes | ✅ Yes |
| Available Options | Prometheus, OTLP, Stdout | Stdout, OTLP, Noop |
| Dependency Inclusion | All included | All included |
| Configuration | Functional options | Functional options |

### Manual Setup (Advanced)

For custom exporters or advanced configuration, you can use `WithCustomTracer()` to provide your own OpenTelemetry tracer.

## Usage Examples

### Router Integration

Integrate tracing with the Rivaas router:

```go
package main

import (
    "context"
    "log"
    "net/http"
    "github.com/rivaas-dev/rivaas/tracing"
    "github.com/rivaas-dev/rivaas/router"
)

func main() {
    // Create router with tracing
    r := router.New(
        tracing.WithTracing(
            tracing.WithServiceName("my-api"),
            tracing.WithServiceVersion("v1.0.0"),
            tracing.WithProvider(tracing.StdoutProvider),
        ),
    )

    r.GET("/", func(c *router.Context) {
        c.JSON(http.StatusOK, map[string]string{
            "message": "Hello",
            "trace_id": c.TraceID(),
        })
    })

    http.ListenAndServe(":8080", r)
}
```

### Production-Ready Configuration

For production deployments, use the `NewProduction()` helper:

```go
package main

import (
    "context"
    "log"
    "net/http"
    "github.com/rivaas-dev/rivaas/tracing"
    "github.com/rivaas-dev/rivaas/router"
)

func main() {
    // Production config: OTLP, 10% sampling, sensitive data protection
    config, err := tracing.NewProduction("my-api", "v1.0.0")
    if err != nil {
        log.Fatal(err)
    }
    defer config.Shutdown(context.Background())
    
    r := router.New()
    r.SetTracingRecorder(config)

    r.GET("/", func(c *router.Context) {
        c.JSON(http.StatusOK, map[string]string{"status": "ok"})
    })

    http.ListenAndServe(":8080", r)
}
```

### Development Configuration

For local development with maximum visibility:

```go
// Development config: Stdout, 100% sampling, all params recorded
config, err := tracing.NewDevelopment("my-api", "dev")
if err != nil {
    log.Fatal(err)
}
defer config.Shutdown(context.Background())

r := router.New()
r.SetTracingRecorder(config)
```

### Standalone Usage

```go
package main

import (
    "context"
    "log"
    "github.com/rivaas-dev/rivaas/tracing"
    "go.opentelemetry.io/otel/attribute"
)

func main() {
    // Create tracing configuration
    config, err := tracing.New(
        tracing.WithServiceName("my-service"),
        tracing.WithServiceVersion("v1.0.0"),
        tracing.WithProvider(tracing.StdoutProvider),
        tracing.WithSampleRate(1.0),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer config.Shutdown(context.Background())

    // Start a span
    ctx, span := config.StartSpan(context.Background(), "my-operation")
    defer config.FinishSpan(span, 200)

    // Add attributes
    config.SetSpanAttribute(span, "user.id", "123")
    config.AddSpanEvent(span, "processing_started")

    // Do some work...
    
    config.AddSpanEvent(span, "processing_completed")
}
```

## Configuration Options

### Convenience Helpers

For common use cases, use the built-in configuration helpers:

```go
// Production: OTLP, 10% sampling, safe defaults
config, err := tracing.NewProduction("my-service", "v1.0.0")
if err != nil {
    log.Fatal(err)
}
defer config.Shutdown(context.Background())

// Development: Stdout, 100% sampling, maximum visibility
config, err := tracing.NewDevelopment("my-service", "dev")
if err != nil {
    log.Fatal(err)
}
defer config.Shutdown(context.Background())
```

### Basic Configuration

```go
tracing.WithServiceName("my-service")
tracing.WithServiceVersion("v1.0.0")
tracing.WithSampleRate(0.1)  // Sample 10% of requests (0.0 to 1.0, automatically clamped)
```

**Note**: Sample rate values are automatically clamped to the valid range of 0.0 to 1.0. Values outside this range will be adjusted.

### Configuration Constants

You can reference the default values:

```go
tracing.DefaultServiceName      // "rivaas-service"
tracing.DefaultServiceVersion   // "1.0.0"
tracing.DefaultSampleRate       // 1.0 (100%)
tracing.MaxExcludedPaths        // 1000
```

### Path Filtering

```go
tracing.WithExcludePaths("/health", "/metrics")
```

### Header Recording

```go
tracing.WithHeaders("X-Request-ID", "User-Agent")
```

**Security Note**: Sensitive headers (Authorization, Cookie, API keys, etc.) are **automatically filtered** and will never be recorded, even if explicitly listed. This prevents accidental credential leakage.

### Custom Tracer

```go
import "go.opentelemetry.io/otel/trace"

customTracer := trace.NewNoopTracerProvider().Tracer("custom")
tracing.WithCustomTracer(customTracer)
```

### Custom Propagator

For advanced use cases like B3 propagation (Zipkin-style) or custom header formats:

```go
import (
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/contrib/propagators/b3"
)

// Use B3 propagation (Zipkin-style)
b3Prop := b3.New()
config, err := tracing.New(
    tracing.WithServiceName("my-service"),
    tracing.WithServiceVersion("v1.0.0"),
    tracing.WithCustomPropagator(b3Prop),
)
if err != nil {
    log.Fatal(err)
}
defer config.Shutdown(context.Background())

// Or use composite propagator for multiple formats
compositeProp := propagation.NewCompositeTextMapPropagator(
    propagation.TraceContext{},  // W3C Trace Context
    propagation.Baggage{},       // W3C Baggage
    b3.New(),                    // B3 format
)
config, err := tracing.New(
    tracing.WithServiceName("my-service"),
    tracing.WithServiceVersion("v1.0.0"),
    tracing.WithCustomPropagator(compositeProp),
)
if err != nil {
    log.Fatal(err)
}
defer config.Shutdown(context.Background())
```

### Disable Parameter Recording

By default, URL query parameters are recorded as span attributes. To disable this:

```go
tracing.WithDisableParams()
```

## Context Integration

### Using with Router Context

```go
r.GET("/users/:id", func(c *router.Context) {
    userID := c.Param("id")
    
    // Add span attributes
    c.SetSpanAttribute("user.id", userID)
    c.SetSpanAttribute("operation", "get_user")
    
    // Add span events
    c.AddSpanEvent("user_lookup_started")
    
    // Do work...
    
    c.AddSpanEvent("user_found")
    
    // Get trace information
    traceID := c.TraceID()
    spanID := c.SpanID()
    
    c.JSON(http.StatusOK, map[string]interface{}{
        "user_id": userID,
        "trace_id": traceID,
        "span_id": spanID,
    })
})
```

### Manual Span Management

```go
// Start a span
ctx, span := config.StartSpan(ctx, "operation-name")
defer config.FinishSpan(span, http.StatusOK)

// Add attributes
config.SetSpanAttribute(span, "key", "value")
config.AddSpanEvent(span, "event-name")

// Get trace context
traceCtx := config.TraceContext(ctx)
```

## HTTP Request Tracing

### Automatic Request Tracing

```go
// The router automatically creates spans for HTTP requests
r.GET("/api/users", func(c *router.Context) {
    // Span is automatically created and managed
    c.SetSpanAttribute("endpoint", "get_users")
    // ... handler logic
})
```

### Manual Request Tracing

```go
config := tracing.MustNew(
    tracing.WithServiceName("my-service"),
    tracing.WithServiceVersion("v1.0.0"),
)
defer config.Shutdown(context.Background())

// Start request span
ctx, span := config.StartRequestSpan(ctx, req, "/api/users", false)

// Process request
// ...

// Finish span
config.FinishRequestSpan(span, http.StatusOK)
```

## Context Propagation

### Extract Trace Context

```go
// Extract trace context from HTTP headers
ctx := config.ExtractTraceContext(ctx, req.Header)
```

### Inject Trace Context

```go
// Inject trace context into HTTP headers
config.InjectTraceContext(ctx, resp.Header)
```

## Middleware Usage

For manual integration with other HTTP frameworks:

```go
import "github.com/rivaas-dev/rivaas/tracing"

config := tracing.MustNew(
    tracing.WithServiceName("my-service"),
    tracing.WithServiceVersion("v1.0.0"),
)
defer config.Shutdown(context.Background())

// Create middleware
middleware := tracing.Middleware(config)

// Use with any http.Handler
http.Handle("/", middleware(yourHandler))
```

## Helper Functions

### Context Helpers

```go
// Get trace ID from context
traceID := tracing.TraceID(ctx)

// Get span ID from context
spanID := tracing.SpanID(ctx)

// Set span attribute from context
tracing.SetSpanAttributeFromContext(ctx, "key", "value")

// Add span event from context
tracing.AddSpanEventFromContext(ctx, "event-name")
```

## Integration Examples

### With Database Operations

```go
r.POST("/users", func(c *router.Context) {
    c.SetSpanAttribute("operation", "create_user")
    c.AddSpanEvent("validation_started")
    
    // Validate input
    // ...
    
    c.AddSpanEvent("database_insert_started")
    
    // Insert into database
    userID, err := db.InsertUser(user)
    if err != nil {
        c.SetSpanAttribute("error", true)
        c.AddSpanEvent("database_error")
        c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
        return
    }
    
    c.AddSpanEvent("database_insert_completed")
    c.SetSpanAttribute("user.id", userID)
    
    c.JSON(http.StatusCreated, map[string]interface{}{
        "user_id": userID,
        "trace_id": c.TraceID(),
    })
})
```

### With External API Calls

```go
r.GET("/users/:id", func(c *router.Context) {
    userID := c.Param("id")
    c.SetSpanAttribute("user.id", userID)
    
    // Call external API
    ctx := c.TraceContext()
    resp, err := httpClient.Get(ctx, "https://api.example.com/users/"+userID)
    
    if err != nil {
        c.SetSpanAttribute("external_api.error", true)
        c.AddSpanEvent("external_api_failed")
        c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "External API unavailable"})
        return
    }
    
    c.AddSpanEvent("external_api_success")
    c.JSON(http.StatusOK, resp)
})
```

## Type-Safe Span Attributes

The tracing package supports native type handling for span attributes, automatically selecting the most appropriate OpenTelemetry attribute type:

```go
// Supported types with native handling:
c.SetSpanAttribute("name", "John Doe")           // string
c.SetSpanAttribute("age", 30)                     // int
c.SetSpanAttribute("count", int64(1000000))       // int64
c.SetSpanAttribute("score", 95.5)                 // float64
c.SetSpanAttribute("active", true)                // bool

// Other types are automatically converted to strings:
c.SetSpanAttribute("metadata", customStruct)      // fmt.Sprintf("%v", value)
```

This applies to all span attribute methods:

- `config.SetSpanAttribute(span, key, value)`
- `c.SetSpanAttribute(key, value)` (router context)
- `tracing.SetSpanAttributeFromContext(ctx, key, value)`

## Performance Considerations

- **Sampling**: Use appropriate sampling rates in production (0.0 to 1.0)
- **Path Filtering**: Exclude health checks and metrics endpoints
- **Attribute Limits**: Be mindful of attribute cardinality
- **Context Propagation**: Minimal overhead when properly configured
- **Thread Safety**: All operations are thread-safe with minimal overhead

### Performance Benchmarks

Benchmarks on Intel Core i7-1265U (12 cores):

| Category | Operation | Time per op | Memory | Allocations |
|----------|-----------|-------------|--------|-------------|
| **Request Overhead** | No Tracing | 1,784 ns | 2,354 B | 23 allocs |
| | 100% Sampling | 1,679 ns | 2,354 B | 23 allocs |
| | 50% Sampling | 1,658 ns | 2,354 B | 23 allocs |
| **Span Operations** | Start/Finish Span | 161 ns | 240 B | 3 allocs |
| | Set String Attr | 3.1 ns | 0 B | 0 allocs |
| | Set Int Attr | 2.9 ns | 0 B | 0 allocs |
| | Add Event | 2.9 ns | 0 B | 0 allocs |
| | Add Event w/ Attrs | 67 ns | 128 B | 1 alloc |
| **Context Propagation** | Extract Context | 14.9 ns | 0 B | 0 allocs |
| | Inject Context | 46.3 ns | 48 B | 1 alloc |
| **Path Exclusion** | Check (0 paths) | 1.2 ns | 0 B | 0 allocs |
| | Check (10 paths) | 5.9 ns | 0 B | 0 allocs |
| | Check (100 paths) | 9.3 ns | 0 B | 0 allocs |

**Key Takeaways:**

- **Minimal overhead**: Tracing adds <100ns per request with noop exporter
- **Zero-allocation attributes**: Setting span attributes doesn't allocate
- **Efficient path checking**: Even with 100 excluded paths, lookup is <10ns
- **Production-ready**: Can handle millions of requests/second with minimal impact

## Limitations and Considerations

### Excluded Paths Map

The `excludePaths` map stores paths that should be excluded from tracing. Consider these points:

- The map is automatically limited to a maximum of 1000 paths to prevent unbounded memory growth
- Paths beyond the 1000 limit are silently ignored
- For most applications, this limit is more than sufficient
- Each path entry uses approximately 50-100 bytes of memory
- **Recommendation**: Configure excluded paths once during initialization, not dynamically

### Sampling Rate

- Sampling decisions are made per-request using a thread-safe random number generator (math/rand/v2)
- Once a request is sampled out, no tracing data is collected for that request
- Sampling is probabilistic - actual sampling rate may vary slightly from configured rate
- For exact control, consider using custom samplers via OpenTelemetry configuration
- The random number generator is safe for use in high-concurrency environments

### Context Propagation Details

- Trace context is extracted from incoming request headers
- If no trace context is present, a new trace is started
- Context propagation follows W3C Trace Context specification
- Custom propagators can be configured via `WithCustomPropagator()`

### Thread Safety Guarantees

- All public APIs are thread-safe
- The `responseWriter` uses mutex locks, but contention is minimal
- Random number generation for sampling uses thread-safe math/rand/v2
- No unsafe global state is modified during request processing
- Safe for use in high-concurrency environments
- Tested with race detector to ensure no data races

### Resource Management

- Spans are created and managed by OpenTelemetry SDK
- No explicit cleanup required for spans (handled by SDK)
- For long-running applications, consider configuring span processors and exporters appropriately
- **Note**: This package does not provide tracer shutdown functionality - manage tracer lifecycle at the application level

### Shutdown and Cleanup

When using the exporter setup functions, always shutdown the tracer provider before application exit:

```go
import (
    "context"
    "time"
    "github.com/rivaas-dev/rivaas/tracing"
)

// During application initialization
tp, err := tracing.SetupStdout("my-service", "v1.0.0")
if err != nil {
    log.Fatal(err)
}

// Ensure shutdown on exit
defer func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := tp.Shutdown(ctx); err != nil {
        log.Printf("Error shutting down tracer provider: %v", err)
    }
}()

// Create tracing config
config, err := tracing.New(...)
if err != nil {
    log.Fatal(err)
}
defer config.Shutdown(context.Background())
```

**Important**: Always flush or shutdown your tracer provider before application exit to ensure all spans are exported.

For manual OpenTelemetry setup (advanced users):

```go
import (
    "go.opentelemetry.io/otel"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Create your own tracer provider
tp := sdktrace.NewTracerProvider(...)
otel.SetTracerProvider(tp)

// Create tracing config with custom tracer
tracer := tp.Tracer("my-service")
config, err := tracing.New(
    tracing.WithServiceName("my-service"),
    tracing.WithServiceVersion("v1.0.0"),
    tracing.WithCustomTracer(tracer),
)
if err != nil {
    log.Fatal(err)
}
defer config.Shutdown(context.Background())

// Shutdown your tracer provider
defer tp.Shutdown(context.Background())
```

### Query Parameter Recording

- When enabled, all query parameters are recorded as span attributes
- Be cautious with sensitive data in query parameters (passwords, tokens, etc.)
- Consider using `WithDisableParams()` if query parameters contain sensitive information
- Parameter values are recorded as string slices to preserve multiple values

## Advanced Usage

### Using Custom Propagators

For interoperability with systems using different trace context formats:

```go
import (
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/contrib/propagators/b3"
)

// Single propagator (B3)
b3Prop := b3.New()
config, err := tracing.New(
    tracing.WithServiceName("my-service"),
    tracing.WithServiceVersion("v1.0.0"),
    tracing.WithCustomPropagator(b3Prop),
)
if err != nil {
    log.Fatal(err)
}
defer config.Shutdown(context.Background())

// Multiple propagators (W3C Trace Context + B3 + Baggage)
compositeProp := propagation.NewCompositeTextMapPropagator(
    propagation.TraceContext{},  // W3C Trace Context (default)
    propagation.Baggage{},       // W3C Baggage
    b3.New(),                    // B3 Single Header
)
config, err := tracing.New(
    tracing.WithServiceName("my-service"),
    tracing.WithServiceVersion("v1.0.0"),
    tracing.WithCustomPropagator(compositeProp),
)
if err != nil {
    log.Fatal(err)
}
defer config.Shutdown(context.Background())
```

**Common propagators:**

- `propagation.TraceContext{}` - W3C Trace Context (default)
- `propagation.Baggage{}` - W3C Baggage
- `b3.New()` - Zipkin B3 format
- `ot.OT{}` - OpenTracing format

### Programmatic Exporter Selection

Switch exporters based on environment or configuration:

```go
import (
    "os"
    "github.com/rivaas-dev/rivaas/tracing"
)

func setupTracing() (*tracing.TracerProvider, error) {
    serviceName := os.Getenv("SERVICE_NAME")
    serviceVersion := os.Getenv("SERVICE_VERSION")
    environment := os.Getenv("ENVIRONMENT")
    
    switch environment {
    case "production":
        return tracing.SetupOTLP(
            serviceName,
            serviceVersion,
            os.Getenv("OTLP_ENDPOINT"),
            false, // Use secure connection in production
        )
    case "development":
        return tracing.SetupStdout(serviceName, serviceVersion)
    case "test":
        return tracing.SetupExporter(serviceName, serviceVersion,
            tracing.WithExporter(tracing.NoopExporter),
        )
    default:
        return tracing.SetupStdout(serviceName, serviceVersion)
    }
}

func main() {
    tp, err := setupTracing()
    if err != nil {
        log.Fatal(err)
    }
    defer tp.Shutdown(context.Background())
    
    // Continue with application setup...
}
```

## Avoiding Global State

### Using Custom Tracer Providers

By default, the tracing package sets the global OpenTelemetry tracer provider. To avoid global state and support multiple independent tracing configurations, provide your own tracer provider:

```go
import (
    "github.com/rivaas-dev/rivaas/tracing"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
)

// Create custom tracer providers for each service
exporter1, _ := stdouttrace.New(stdouttrace.WithPrettyPrint())
provider1 := sdktrace.NewTracerProvider(
    sdktrace.WithBatcher(exporter1),
)

exporter2, _ := stdouttrace.New(stdouttrace.WithPrettyPrint())
provider2 := sdktrace.NewTracerProvider(
    sdktrace.WithBatcher(exporter2),
)

// Create independent tracing configurations
config1, _ := tracing.New(
    tracing.WithTracerProvider(provider1),
    tracing.WithServiceName("service-1"),
)

config2, _ := tracing.New(
    tracing.WithTracerProvider(provider2),
    tracing.WithServiceName("service-2"),
)

// Both work independently without conflicts!
defer provider1.Shutdown(context.Background())
defer provider2.Shutdown(context.Background())
```

**When to use custom providers:**

- ✅ Multiple independent tracing configurations in same process
- ✅ Full control over tracer provider lifecycle
- ✅ Integration with existing OpenTelemetry setups
- ✅ Testing with isolated traces
- ✅ Avoiding global state in your application

**When to use built-in providers:**

- ✅ Single tracing configuration per process (most common)
- ✅ Quick setup with sensible defaults
- ✅ Don't need fine-grained control over tracer provider

## Examples

See the `examples/` directory for complete working examples:

- `standalone/` - Standalone tracing usage
- Integration examples in the main router examples

## Exporter-Specific Configuration

### OTLP Exporter Options

The OTLP exporter supports additional configuration:

```go
// Basic setup
tp, _ := tracing.SetupOTLP("my-service", "v1.0.0", "collector:4317", true)

// With unified API and all options
tp, _ := tracing.SetupExporter("my-service", "v1.0.0",
    tracing.WithExporter(tracing.OTLPExporter),
    tracing.WithOTLPEndpoint("collector:4317"),
    tracing.WithOTLPInsecure(true),  // Use false for production with TLS
)
```

**Configuration Parameters:**

- **Endpoint**: The OTLP collector address (host:port)
  - Examples: `"localhost:4317"`, `"jaeger:4317"`, `"tempo:4317"`
  - Default: Uses OpenTelemetry SDK defaults
  
- **Insecure**: Whether to use insecure gRPC connection
  - `true`: Use for local development (no TLS)
  - `false`: Use for production (requires TLS certificates)

### Stdout Exporter Output

The stdout exporter uses pretty-printing by default:

```go
tp, _ := tracing.SetupStdout("my-service", "v1.0.0")
```

Output format:

```json
{
  "Name": "GET /api/users",
  "SpanContext": {
    "TraceID": "4bf92f3577b34da6a3ce929d0e0e4736",
    "SpanID": "00f067aa0ba902b7",
    "TraceFlags": "01"
  },
  "Attributes": [
    {"Key": "http.method", "Value": {"Type": "STRING", "Value": "GET"}},
    {"Key": "http.route", "Value": {"Type": "STRING", "Value": "/api/users"}}
  ]
}
```

### Noop Exporter Use Cases

The noop exporter creates a valid tracer provider but doesn't export any traces:

```go
tp, _ := tracing.SetupExporter("my-service", "v1.0.0",
    tracing.WithExporter(tracing.NoopExporter),
)
```

**Use cases:**

- Testing without trace output
- Temporarily disabling tracing
- Performance testing baseline

## Error Handling

All exporter setup functions return errors that should be handled:

```go
tp, err := tracing.SetupOTLP("my-service", "v1.0.0", "collector:4317", true)
if err != nil {
    log.Fatalf("Failed to setup tracing: %v", err)
}
defer tp.Shutdown(context.Background())
```

**Common errors:**

- Invalid exporter type: `"unsupported tracing exporter: invalid"`
- OTLP connection failure: Errors from gRPC dial
- Resource creation failure: Rare, usually indicates config issue

## Implementation Details

### Architecture

The tracing package is organized into focused modules:

```text
tracing/
  ├── exporters.go           # Exporter API and implementations
  ├── tracing.go             # Core tracing functionality
  ├── router.go              # Router integration
  ├── tracing_test.go        # Comprehensive tests
  ├── tracing_bench_test.go  # Performance benchmarks
  └── go.mod                 # All exporter dependencies included
```

### How It Works

1. **All exporter dependencies are included in `go.mod`** - No conditional compilation
2. **`exporters.go` contains all exporter implementations** - Stdout, OTLP, Noop
3. **User selects exporter type at runtime via options** - No build tags needed
4. **`SetupExporter` dispatches to the appropriate init function** - Clean abstraction

### Extending with New Exporters

To add a new exporter (e.g., Zipkin, Jaeger native):

1. Add dependency to `go.mod`:

   ```bash
   go get go.opentelemetry.io/otel/exporters/zipkin
   ```

2. Add exporter constant in `exporters.go`:

   ```go
   const ZipkinExporter TracingExporter = "zipkin"
   ```

3. Add init function:

   ```go
   func initZipkinExporter(cfg *ExporterConfig) (*TracerProvider, error) {
       exporter, err := zipkin.New("http://localhost:9411/api/v2/spans")
       if err != nil {
           return nil, fmt.Errorf("failed to create zipkin exporter: %w", err)
       }
       
       res := createResource(cfg.serviceName, cfg.serviceVersion)
       tp := sdktrace.NewTracerProvider(
           sdktrace.WithBatcher(exporter),
           sdktrace.WithResource(res),
       )
       
       otel.SetTracerProvider(tp)
       return &TracerProvider{provider: tp}, nil
   }
   ```

4. Add case to switch in `SetupExporter`:

   ```go
   case ZipkinExporter:
       return initZipkinExporter(cfg)
   ```

5. Optionally add convenience function:

   ```go
   func SetupZipkin(serviceName, serviceVersion, endpoint string) (*TracerProvider, error) {
       return SetupExporter(serviceName, serviceVersion,
           WithExporter(ZipkinExporter),
           // Add Zipkin-specific options here
       )
   }
   ```

## Troubleshooting

### No Traces Appearing

1. **Check exporter setup**: Ensure `SetupExporter` was called before creating Config
2. **Check sampling rate**: With `WithSampleRate(0.1)`, only 10% of requests are traced
3. **Check excluded paths**: Verify your route isn't in `WithExcludePaths()`
4. **Check OTLP connection**: For OTLP, ensure collector is running and accessible

### Performance Impact

If tracing is causing performance issues:

1. **Lower sample rate**: Use `WithSampleRate(0.1)` or lower in production
2. **Exclude health checks**: Use `WithExcludePaths("/health", "/metrics")`
3. **Disable params**: Use `WithDisableParams()` to reduce attribute count
4. **Check exporter**: Ensure OTLP collector can handle the load

### Context Propagation Not Working

1. **Check propagator**: Verify both services use compatible propagators
2. **Check headers**: Ensure `traceparent` header is being set
3. **Check extraction**: Use `config.ExtractTraceContext()` for incoming requests
4. **Check injection**: Use `config.InjectTraceContext()` for outgoing requests

## License

MIT License - see LICENSE file for details.
