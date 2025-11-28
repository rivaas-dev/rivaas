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

By default, the package does NOT set the global OpenTelemetry tracer provider. Use `WithGlobalTracerProvider()` if you want global registration. This allows multiple tracing configurations to coexist in the same process.

## Quick Start

The tracing package provides a unified API consistent with the metrics package, supporting multiple providers at runtime without build tags.

**Key Features:**

- ✅ All providers available at runtime
- ✅ No build tags required
- ✅ Switch via configuration only
- ✅ Consistent API with metrics package

### Basic Usage

```go
import (
    "context"
    "log"
    "rivaas.dev/tracing"
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
    "rivaas.dev/tracing"
    "rivaas.dev/router"
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
    "rivaas.dev/tracing"
    "rivaas.dev/router"
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
    "rivaas.dev/tracing"
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

### Logging Integration

Integrate tracing with your application's logging system for visibility into tracing operations:

```go
// Use stdlib slog for logging internal events
config := tracing.New(
    tracing.WithServiceName("my-service"),
    tracing.WithLogger(slog.Default()),
)

// Or create a custom slog logger
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
config := tracing.New(
    tracing.WithServiceName("my-service"),
    tracing.WithLogger(logger),
)

// For advanced use cases, use a custom event handler (e.g., send errors to Sentry)
config := tracing.New(
    tracing.WithServiceName("my-service"),
    tracing.WithEventHandler(func(e tracing.Event) {
        if e.Type == tracing.EventError {
            sentry.CaptureMessage(e.Message)
        }
        slog.Default().Info(e.Message, e.Args...)
    }),
)
```

**What gets logged:**

- **Warning**: Excluded paths limit exceeded, provider initialization issues
- **Debug**: Sampling decisions (when requests are not sampled), provider setup details
- **Error**: Shutdown failures, provider errors

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

Exclude specific paths from tracing:

```go
tracing.WithExcludePaths("/health", "/metrics")
```

**Note**: Maximum 1000 paths can be excluded. If more paths are provided, only the first 1000 will be excluded and a warning will be logged (if logger is configured).

For excluding many paths that follow a pattern, use regex patterns:

```go
// Exclude all paths starting with /internal/
tracing.WithExcludePathPattern("^/internal/.*")

// Exclude all health check endpoints
tracing.WithExcludePathPattern("^/(health|ready|live)")

// Combine exact paths and patterns
tracing.WithExcludePaths("/exact/path"),
tracing.WithExcludePathPattern("^/pattern/.*"),
```

Regex patterns are compiled once during configuration, so invalid patterns will cause `New()` to return an error.

### Header Recording

```go
tracing.WithHeaders("X-Request-ID", "User-Agent")
```

**Security Note**: Sensitive headers (Authorization, Cookie, API keys, etc.) are **automatically filtered** and will never be recorded, even if explicitly listed. This prevents accidental credential leakage.

### Granular Parameter Recording

Control which query parameters are recorded with fine-grained options:

**Whitelist specific parameters (record only these):**

```go
config := tracing.New(
    tracing.WithServiceName("my-service"),
    tracing.WithRecordParams("user_id", "request_id", "page"),
)
```

**Blacklist sensitive parameters (record all except these):**

```go
config := tracing.New(
    tracing.WithServiceName("my-service"),
    tracing.WithExcludeParams("password", "token", "api_key", "secret"),
)
```

**Combine whitelist and blacklist:**

```go
config := tracing.New(
    tracing.WithServiceName("my-service"),
    tracing.WithRecordParams("user_id", "request_id", "password"),
    tracing.WithExcludeParams("password"), // Blacklist takes precedence
)
```

**Logic:**

- If parameter is in blacklist (`WithExcludeParams`), it's **never** recorded
- If whitelist is configured (`WithRecordParams`), only listed parameters are recorded
- Otherwise, all parameters are recorded (default behavior)

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

### Span Lifecycle Hooks

Add custom logic when spans start and finish for integration with external systems, metrics, or custom attribute injection:

**Span Start Hook:**

```go
startHook := func(ctx context.Context, span trace.Span, req *http.Request) {
    // Add custom business logic attributes
    if tenantID := req.Header.Get("X-Tenant-ID"); tenantID != "" {
        span.SetAttributes(attribute.String("tenant.id", tenantID))
    }
    
    // Add user context
    if userID := extractUserID(req); userID != "" {
        span.SetAttributes(attribute.String("user.id", userID))
    }
}

config := tracing.New(
    tracing.WithServiceName("my-service"),
    tracing.WithSpanStartHook(startHook),
)
```

**Span Finish Hook:**

```go
finishHook := func(span trace.Span, statusCode int) {
    // Record custom metrics
    if statusCode >= 500 {
        metrics.IncrementServerErrors()
    }
    
    // Log slow requests
    if duration := time.Since(span.StartTime()); duration > 5*time.Second {
        log.Warn("slow request", "duration", duration)
    }
}

config := tracing.New(
    tracing.WithServiceName("my-service"),
    tracing.WithSpanFinishHook(finishHook),
)
```

**Both hooks together:**

```go
config := tracing.New(
    tracing.WithServiceName("my-service"),
    tracing.WithSpanStartHook(startHook),
    tracing.WithSpanFinishHook(finishHook),
)
```

**Use cases:**

- Adding tenant/organization context from request headers
- Recording custom metrics based on span data
- Integration with external monitoring systems
- Dynamic span configuration per request
- Logging slow requests or errors
- Post-processing trace data

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
import "rivaas.dev/tracing"

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

### Available Configuration Options

| Option | Description | Example |
|--------|-------------|---------|
| `WithServiceName(name)` | Set service name | `WithServiceName("my-api")` |
| `WithServiceVersion(version)` | Set service version | `WithServiceVersion("v1.0.0")` |
| `WithProvider(provider)` | Set trace exporter | `WithProvider(OTLPProvider)` |
| `WithOTLPEndpoint(endpoint)` | Set OTLP endpoint | `WithOTLPEndpoint("localhost:4317")` |
| `WithOTLPInsecure(bool)` | Use insecure OTLP | `WithOTLPInsecure(true)` |
| `WithSampleRate(rate)` | Set sampling rate (0.0-1.0) | `WithSampleRate(0.1)` |
| `WithExcludePaths(paths...)` | Exclude paths from tracing | `WithExcludePaths("/health")` |
| `WithHeaders(headers...)` | Record specific headers | `WithHeaders("X-Request-ID")` |
| `WithDisableParams()` | Disable all parameter recording | `WithDisableParams()` |
| `WithRecordParams(params...)` | Whitelist parameters to record | `WithRecordParams("user_id", "page")` |
| `WithExcludeParams(params...)` | Blacklist parameters | `WithExcludeParams("password", "token")` |
| `WithCustomTracer(tracer)` | Use custom tracer | `WithCustomTracer(myTracer)` |
| `WithCustomPropagator(prop)` | Use custom propagator | `WithCustomPropagator(b3.New())` |
| `WithTracerProvider(provider)` | Use custom tracer provider | `WithTracerProvider(myProvider)` |
| `WithLogger(logger)` | Set slog logger for events | `WithLogger(slog.Default())` |
| `WithEventHandler(handler)` | Set custom event handler | `WithEventHandler(myHandler)` |
| `WithSpanStartHook(hook)` | Set span start callback | `WithSpanStartHook(myStartHook)` |
| `WithSpanFinishHook(hook)` | Set span finish callback | `WithSpanFinishHook(myFinishHook)` |

## Integration Examples

### With Lifecycle Hooks

Add custom business logic to spans:

```go
startHook := func(ctx context.Context, span trace.Span, req *http.Request) {
    // Extract tenant from JWT or header
    if tenantID := extractTenantFromRequest(req); tenantID != "" {
        span.SetAttributes(
            attribute.String("tenant.id", tenantID),
            attribute.String("tenant.plan", getTenantPlan(tenantID)),
        )
    }
    
    // Add request metadata
    if requestID := req.Header.Get("X-Request-ID"); requestID != "" {
        span.SetAttributes(attribute.String("request.id", requestID))
    }
}

finishHook := func(span trace.Span, statusCode int) {
    // Record custom metrics
    if statusCode >= 500 {
        serverErrorsCounter.Inc()
    } else if statusCode >= 400 {
        clientErrorsCounter.Inc()
    }
    
    // Log high-latency requests
    ctx := span.SpanContext()
    if ctx.IsValid() {
        duration := time.Since(span.StartTime())
        if duration > 5*time.Second {
            log.Warn("slow request detected",
                "trace_id", ctx.TraceID().String(),
                "duration", duration,
            )
        }
    }
}

config := tracing.New(
    tracing.WithServiceName("my-service"),
    tracing.WithSpanStartHook(startHook),
    tracing.WithSpanFinishHook(finishHook),
)
```

### With Granular Parameter Recording

Protect sensitive data while maintaining visibility:

```go
// Scenario 1: Only record specific safe parameters
config := tracing.New(
    tracing.WithServiceName("auth-service"),
    tracing.WithRecordParams("user_id", "session_id", "redirect_uri"),
    // password, token, etc. won't be recorded
)

// Scenario 2: Record all except sensitive ones
config := tracing.New(
    tracing.WithServiceName("api-service"),
    tracing.WithExcludeParams(
        "password", "token", "api_key", "secret", 
        "credit_card", "ssn", "access_token",
    ),
)

// Scenario 3: Combination approach
config := tracing.New(
    tracing.WithServiceName("admin-service"),
    tracing.WithRecordParams("user_id", "action", "resource_id", "api_key"),
    tracing.WithExcludeParams("api_key"), // Override: don't record api_key even if whitelisted
)
```

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
    "rivaas.dev/tracing"
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

Switch exporters based on configuration:

```go
import (
    "rivaas.dev/tracing"
)

func setupTracing(environment string) (*tracing.Config, error) {
    serviceName := "my-service"
    serviceVersion := "v1.0.0"
    
    switch environment {
    case "production":
        return tracing.NewProduction(serviceName, serviceVersion)
    case "development":
        return tracing.NewDevelopment(serviceName, serviceVersion)
    case "test":
        return tracing.New(
            tracing.WithServiceName(serviceName),
            tracing.WithServiceVersion(serviceVersion),
            tracing.WithProvider(tracing.NoopProvider),
        )
    default:
        return tracing.NewDevelopment(serviceName, serviceVersion)
    }
}

func main() {
    config, err := setupTracing("production")
    if err != nil {
        log.Fatal(err)
    }
    defer config.Shutdown(context.Background())
    
    // Continue with application setup...
}
```

## Avoiding Global State

### Using Custom Tracer Providers

By default, the tracing package sets the global OpenTelemetry tracer provider. To avoid global state and support multiple independent tracing configurations, provide your own tracer provider:

```go
import (
    "rivaas.dev/tracing"
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

1. **Check provider configuration**: Ensure provider is set correctly (StdoutProvider, OTLPProvider, etc.)
2. **Check sampling rate**: With `WithSampleRate(0.1)`, only 10% of requests are traced. Default is 100% (1.0).
3. **Check excluded paths**: Verify your route isn't in `WithExcludePaths()` or matching a pattern in `WithExcludePathPattern()`
4. **Check OTLP connection**: For OTLP, ensure collector is running and accessible. Provider creation may succeed even with unreachable endpoints (connection failures happen during span export).
5. **Check context cancellation**: If the request context is cancelled before span creation, no span will be created.
6. **Verify global tracer provider**: If using `WithGlobalTracerProvider()`, ensure only one configuration is active per process.

### Performance Impact

If tracing is causing performance issues:

1. **Lower sample rate**: Use `WithSampleRate(0.1)` or lower in production (recommended: 10% for high-traffic services)
2. **Exclude health checks**: Use `WithExcludePaths("/health", "/metrics")` or `WithExcludePathPattern("^/(health|metrics|ready|live)")`
3. **Disable parameter recording**: Use `WithDisableParams()` if query parameters contain sensitive data or are high-cardinality
4. **Use regex patterns**: For many paths, use `WithExcludePathPattern()` instead of listing individual paths
5. **Monitor span creation overhead**: Use benchmarks to measure impact (see Performance Benchmarks section)

### Common Errors

#### `"validation errors: invalid regex pattern"`

- Check regex patterns passed to `WithExcludePathPattern()` for syntax errors
- Test patterns with Go's `regexp.Compile()` before using

#### `"unsupported tracing provider"`

- Ensure provider is one of: `NoopProvider`, `StdoutProvider`, `OTLPProvider`
- Check for typos in provider names

#### `"service name cannot be empty"`

- Provide service name via `WithServiceName()`

#### `"tracer provider shutdown: ..."`

- This error occurs during `Shutdown()` if the tracer provider fails to flush pending spans
- Ensure shutdown context has sufficient timeout (recommended: 5-10 seconds)
- Check OTLP collector connectivity if using OTLP provider

### Memory Usage Growing

1. **Ensure `Shutdown()` is called**: Always call `config.Shutdown(ctx)` on application exit
2. **Check for goroutine leaks**: Custom hooks should not spawn long-running goroutines
3. **Monitor span pool usage**: String pool for span names is automatically managed

### Context Cancellation

The package handles context cancellation gracefully:

- If context is cancelled before span creation, no span is created (no-op)
- Cancelled contexts are preserved and returned to callers
- This prevents unnecessary work when requests are already cancelled

### Context Propagation Not Working

1. **Check propagator**: Verify both services use compatible propagators
2. **Check headers**: Ensure `traceparent` header is being set
3. **Check extraction**: Use `config.ExtractTraceContext()` for incoming requests
4. **Check injection**: Use `config.InjectTraceContext()` for outgoing requests

## License

MIT License - see LICENSE file for details.
