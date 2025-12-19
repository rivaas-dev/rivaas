# Tracing

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/tracing.svg)](https://pkg.go.dev/rivaas.dev/tracing)
[![Go Report Card](https://goreportcard.com/badge/rivaas.dev/tracing)](https://goreportcard.com/report/rivaas.dev/tracing)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../LICENSE)

A distributed tracing package for Go applications using OpenTelemetry. This package provides easy-to-use tracing functionality with support for various exporters and seamless integration with HTTP frameworks.

## Features

- **OpenTelemetry Integration**: Full OpenTelemetry tracing support
- **Context Propagation**: Automatic trace context propagation
- **Span Management**: Easy span creation and management
- **HTTP Middleware**: Standalone middleware for any HTTP framework
- **Custom Attributes**: Add custom attributes and events to spans
- **Path Filtering**: Exclude specific paths from tracing (via middleware options)
- **Consistent API**: Same design patterns as the metrics package

## Installation

```bash
go get rivaas.dev/tracing
```

Requires Go 1.25+

## Quick Start

The tracing package provides a unified API consistent with the metrics package.

### Basic Usage

```go
import (
    "context"
    "log"
    "rivaas.dev/tracing"
)

func main() {
    // Create tracer with convenient provider options
    tracer, err := tracing.New(
        tracing.WithServiceName("my-service"),
        tracing.WithServiceVersion("v1.0.0"),
        tracing.WithStdout(), // or WithOTLP, WithNoop
    )
    if err != nil {
        log.Fatal(err)
    }
    defer tracer.Shutdown(context.Background())

    // Use with your application...
}
```

### Provider Options

Choose the provider that fits your needs:

#### Stdout Provider (Development)

```go
tracer := tracing.MustNew(
    tracing.WithServiceName("my-service"),
    tracing.WithStdout(),
)
defer tracer.Shutdown(context.Background())
```

#### OTLP Provider (Production)

```go
// OTLP gRPC
tracer := tracing.MustNew(
    tracing.WithServiceName("my-service"),
    tracing.WithOTLP("localhost:4317"),
)

// With insecure connection (local dev)
tracer := tracing.MustNew(
    tracing.WithServiceName("my-service"),
    tracing.WithOTLP("localhost:4317", tracing.OTLPInsecure()),
)

// OTLP HTTP (consistent with metrics package)
tracer := tracing.MustNew(
    tracing.WithServiceName("my-service"),
    tracing.WithOTLPHTTP("http://localhost:4318"),
)
```

#### Noop Provider (Default/Testing)

```go
tracer := tracing.MustNew(
    tracing.WithServiceName("my-service"),
    tracing.WithNoop(),
)
```

### HTTP Middleware

```go
import (
    "net/http"
    "rivaas.dev/tracing"
)

func main() {
    tracer := tracing.MustNew(
        tracing.WithServiceName("my-api"),
        tracing.WithOTLP("localhost:4317"),
    )
    defer tracer.Shutdown(context.Background())

    mux := http.NewServeMux()
    mux.HandleFunc("/api/users", handleUsers)

    // Wrap with tracing middleware using MustMiddleware (panics on error)
    handler := tracing.MustMiddleware(tracer,
        tracing.WithExcludePaths("/health", "/metrics"),
        tracing.WithHeaders("X-Request-ID", "X-Correlation-ID"),
    )(mux)

    // Or use Middleware with error handling
    middleware, err := tracing.Middleware(tracer,
        tracing.WithExcludePaths("/health", "/metrics"),
    )
    if err != nil {
        log.Fatal(err)
    }
    handler = middleware(mux)

    http.ListenAndServe(":8080", handler)
}
```

### Comparison with Metrics Package

The tracing package follows the same design pattern as the metrics package:

| Aspect | Metrics Package | Tracing Package |
|--------|----------------|-----------------|
| Main Type | `Recorder` | `Tracer` |
| Provider Options | `WithPrometheus()`, `WithOTLP()` | `WithOTLP()`, `WithStdout()`, `WithNoop()` |
| Constructor | `New(opts...) (*Recorder, error)` | `New(opts...) (*Tracer, error)` |
| Panic Version | `MustNew(opts...) *Recorder` | `MustNew(opts...) *Tracer` |
| Middleware | `Middleware(recorder, opts...) (func, error)` | `Middleware(tracer, opts...) (func, error)` |
| Panic Middleware | `MustMiddleware(recorder, opts...)` | `MustMiddleware(tracer, opts...)` |
| Path Exclusion | `MiddlewareOption` | `MiddlewareOption` |
| Header Recording | `MiddlewareOption` | `MiddlewareOption` |

## Middleware Options

Path filtering and header recording are configured via `MiddlewareOption`:

### Path Exclusion

```go
handler := tracing.MustMiddleware(tracer,
    // Exact paths
    tracing.WithExcludePaths("/health", "/metrics", "/ready"),
    // Prefix matching
    tracing.WithExcludePrefixes("/debug/", "/internal/"),
    // Regex patterns (invalid patterns return error from Middleware)
    tracing.WithExcludePatterns(`^/v[0-9]+/internal/.*`),
)(mux)
```

### Header Recording

```go
handler := tracing.MustMiddleware(tracer,
    tracing.WithHeaders("X-Request-ID", "X-Correlation-ID", "User-Agent"),
)(mux)
```

**Security Note**: Sensitive headers (Authorization, Cookie, API keys) are automatically filtered.

### Parameter Recording

```go
// Record only specific parameters
handler := tracing.MustMiddleware(tracer,
    tracing.WithRecordParams("user_id", "page"),
)(mux)

// Exclude sensitive parameters
handler := tracing.MustMiddleware(tracer,
    tracing.WithExcludeParams("password", "token", "api_key"),
)(mux)

// Disable all parameter recording
handler := tracing.MustMiddleware(tracer,
    tracing.WithoutParams(),
)(mux)
```

## Tracer Configuration Options

### Basic Configuration

```go
tracing.WithServiceName("my-service")
tracing.WithServiceVersion("v1.0.0")
tracing.WithSampleRate(0.1)  // Sample 10% of requests
```

### Span Lifecycle Hooks

```go
startHook := func(ctx context.Context, span trace.Span, req *http.Request) {
    if tenantID := req.Header.Get("X-Tenant-ID"); tenantID != "" {
        span.SetAttributes(attribute.String("tenant.id", tenantID))
    }
}

finishHook := func(span trace.Span, statusCode int) {
    if statusCode >= 500 {
        metrics.IncrementServerErrors()
    }
}

tracer := tracing.MustNew(
    tracing.WithServiceName("my-service"),
    tracing.WithSpanStartHook(startHook),
    tracing.WithSpanFinishHook(finishHook),
)
```

### Logging Integration

```go
// Use stdlib slog
tracer := tracing.MustNew(
    tracing.WithServiceName("my-service"),
    tracing.WithLogger(slog.Default()),
)

// Custom event handler
tracer := tracing.MustNew(
    tracing.WithServiceName("my-service"),
    tracing.WithEventHandler(func(e tracing.Event) {
        if e.Type == tracing.EventError {
            sentry.CaptureMessage(e.Message)
        }
    }),
)
```

### Custom Tracer Provider

```go
// Use your own OpenTelemetry tracer provider
tp := sdktrace.NewTracerProvider(...)

tracer := tracing.MustNew(
    tracing.WithServiceName("my-service"),
    tracing.WithTracerProvider(tp),
)
// Note: You manage tp.Shutdown() yourself
```

## Manual Span Management

```go
// Start a span
ctx, span := tracer.StartSpan(ctx, "database-query")
defer tracer.FinishSpan(span, http.StatusOK)

// Add attributes
tracer.SetSpanAttribute(span, "db.query", "SELECT * FROM users")
tracer.SetSpanAttribute(span, "db.rows", 10)

// Add events
tracer.AddSpanEvent(span, "cache_hit",
    attribute.String("key", "user:123"),
)
```

## Context Helpers

```go
// Get trace/span IDs from context
traceID := tracing.TraceID(ctx)
spanID := tracing.SpanID(ctx)

// Set attributes from context
tracing.SetSpanAttributeFromContext(ctx, "key", "value")

// Add events from context
tracing.AddSpanEventFromContext(ctx, "event-name")
```

## Context Propagation

```go
// Extract trace context from incoming request
ctx = tracer.ExtractTraceContext(ctx, req.Header)

// Inject trace context into outgoing request
tracer.InjectTraceContext(ctx, outgoingReq.Header)
```

## Available Options

### Tracer Options

| Option | Description | Example |
|--------|-------------|---------|
| `WithServiceName(name)` | Set service name | `WithServiceName("my-api")` |
| `WithServiceVersion(version)` | Set service version | `WithServiceVersion("v1.0.0")` |
| `WithOTLP(endpoint, opts...)` | OTLP gRPC provider | `WithOTLP("localhost:4317")` |
| `WithOTLPHTTP(endpoint)` | OTLP HTTP provider | `WithOTLPHTTP("http://localhost:4318")` |
| `WithStdout()` | Stdout provider | `WithStdout()` |
| `WithNoop()` | Noop provider | `WithNoop()` |
| `WithSampleRate(rate)` | Sampling rate (0.0-1.0) | `WithSampleRate(0.1)` |
| `WithTracerProvider(provider)` | Custom tracer provider | `WithTracerProvider(tp)` |
| `WithCustomTracer(tracer)` | Custom tracer | `WithCustomTracer(t)` |
| `WithCustomPropagator(prop)` | Custom propagator | `WithCustomPropagator(b3.New())` |
| `WithLogger(logger)` | Set slog logger | `WithLogger(slog.Default())` |
| `WithEventHandler(handler)` | Custom event handler | `WithEventHandler(fn)` |
| `WithSpanStartHook(hook)` | Span start callback | `WithSpanStartHook(fn)` |
| `WithSpanFinishHook(hook)` | Span finish callback | `WithSpanFinishHook(fn)` |
| `WithGlobalTracerProvider()` | Register globally | `WithGlobalTracerProvider()` |

### Middleware Functions

| Function | Description | Example |
|----------|-------------|---------|
| `Middleware(tracer, opts...)` | Creates middleware, returns error | `Middleware(tracer, opts...)` |
| `MustMiddleware(tracer, opts...)` | Creates middleware, panics on error | `MustMiddleware(tracer, opts...)` |

### Middleware Options

| Option | Description | Example |
|--------|-------------|---------|
| `WithExcludePaths(paths...)` | Exclude exact paths | `WithExcludePaths("/health")` |
| `WithExcludePrefixes(prefixes...)` | Exclude by prefix | `WithExcludePrefixes("/debug/")` |
| `WithExcludePatterns(patterns...)` | Exclude by regex (errors on invalid) | `WithExcludePatterns("^/v[0-9]+/")` |
| `WithHeaders(headers...)` | Record headers | `WithHeaders("X-Request-ID")` |
| `WithRecordParams(params...)` | Whitelist params | `WithRecordParams("user_id")` |
| `WithExcludeParams(params...)` | Blacklist params | `WithExcludeParams("password")` |
| `WithoutParams()` | Disable all params | `WithoutParams()` |

## Production Configuration

```go
tracer := tracing.MustNew(
    tracing.WithServiceName("my-api"),
    tracing.WithServiceVersion("v1.0.0"),
    tracing.WithOTLP("collector:4317"),
    tracing.WithSampleRate(0.1), // 10% sampling
)
defer tracer.Shutdown(context.Background())

handler := tracing.MustMiddleware(tracer,
    tracing.WithExcludePaths("/health", "/metrics", "/ready", "/live"),
    tracing.WithExcludeParams("password", "token", "api_key"),
)(mux)
```

## Development Configuration

```go
tracer := tracing.MustNew(
    tracing.WithServiceName("my-api"),
    tracing.WithServiceVersion("dev"),
    tracing.WithStdout(),
    tracing.WithSampleRate(1.0), // 100% sampling
)
defer tracer.Shutdown(context.Background())

handler := tracing.MustMiddleware(tracer)(mux)
```

## Performance

| Operation | Time | Memory | Allocations |
|-----------|------|--------|-------------|
| Request Overhead (100% sampling) | ~1.6 µs | 2.3 KB | 23 |
| Start/Finish Span | ~160 ns | 240 B | 3 |
| Set Attribute | ~3 ns | 0 B | 0 |
| Path Exclusion (100 paths) | ~9 ns | 0 B | 0 |

## Error Handling

```go
tracer, err := tracing.New(
    tracing.WithServiceName("my-service"),
    tracing.WithOTLP("localhost:4317"),
)
if err != nil {
    log.Fatalf("Failed to setup tracing: %v", err)
}
defer tracer.Shutdown(context.Background())
```

**Common errors:**
- `"service name cannot be empty"`
- `"service version cannot be empty"`
- `"unsupported tracing provider: invalid"`

## Shutdown

Always shutdown before application exit:

```go
defer func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := tracer.Shutdown(ctx); err != nil {
        log.Printf("Error shutting down tracing: %v", err)
    }
}()
```

## API Reference

For detailed API documentation, see [pkg.go.dev/rivaas.dev/tracing](https://pkg.go.dev/rivaas.dev/tracing).

## Contributing

Contributions are welcome! Please see the [main repository](../) for contribution guidelines.

## License

Apache License 2.0 - see [LICENSE](../LICENSE) for details.

---

Part of the [Rivaas](https://github.com/rivaas-dev/rivaas) web framework ecosystem.
