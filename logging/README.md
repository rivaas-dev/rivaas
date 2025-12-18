# logging

Structured logging for Rivaas using Go's standard `log/slog` package.

## Features

- **Multiple Output Formats**: JSON, text, and human-friendly console output
- **Context-Aware Logging**: Automatic trace correlation with OpenTelemetry
- **Sensitive Data Redaction**: Automatic sanitization of passwords, tokens, and secrets
- **Log Sampling**: Reduce log volume in high-traffic scenarios
- **Convenience Methods**: HTTP request logging, error logging with context, duration tracking
- **Dynamic Log Levels**: Change log levels at runtime without restart
- **Functional Options API**: Clean, composable configuration
- **Router Integration**: Seamless integration following metrics/tracing patterns
- **Zero External Dependencies**: Uses only Go standard library (except OpenTelemetry for trace correlation)

## Installation

```bash
go get rivaas.dev/logging
```

## Dependencies

| Dependency | Purpose | Required |
|------------|---------|----------|
| Go stdlib (`log/slog`) | Core logging | Yes |
| `go.opentelemetry.io/otel/trace` | Trace correlation in `ContextLogger` | Optional* |
| `github.com/stretchr/testify` | Test utilities | Test only |

\* The OpenTelemetry trace dependency is only used by `NewContextLogger()` for automatic trace/span ID extraction. If you don't use context-aware logging with tracing, this dependency has no runtime impact.

## Quick Start

### Basic Usage

```go
package main

import (
    "rivaas.dev/logging"
)

func main() {
    // Create a logger with console output
    log := logging.MustNew(
        logging.WithConsoleHandler(),
        logging.WithDebugLevel(),
    )

    log.Info("service started", "port", 8080, "env", "production")
    log.Debug("debugging information", "key", "value")
    log.Error("operation failed", "error", "connection timeout")
}
```

### Output Formats

#### JSON Handler (Default)

```go
log := logging.MustNew(
    logging.WithJSONHandler(),
)

log.Info("user action", "user_id", "123", "action", "login")
// Output: {"time":"2024-01-15T10:30:45.123Z","level":"INFO","msg":"user action","user_id":"123","action":"login"}
```

#### Text Handler

```go
log := logging.MustNew(
    logging.WithTextHandler(),
)

log.Info("request processed", "method", "GET", "path", "/api/users")
// Output: time=2024-01-15T10:30:45.123Z level=INFO msg="request processed" method=GET path=/api/users
```

#### Console Handler

```go
log := logging.MustNew(
    logging.WithConsoleHandler(),
)

log.Info("server starting", "port", 8080)
// Output: 10:30:45.123 INFO  server starting port=8080
// (with colors!)
```

## Configuration Options

### Handler Types

```go
// JSON structured logging (default, best for production)
logging.WithJSONHandler()

// Text key=value logging
logging.WithTextHandler()

// Human-readable colored console (best for development)
logging.WithConsoleHandler()
```

### Log Levels

```go
// Set minimum log level
logging.WithLevel(logging.LevelDebug)
logging.WithLevel(logging.LevelInfo)
logging.WithLevel(logging.LevelWarn)
logging.WithLevel(logging.LevelError)

// Convenience function
logging.WithDebugLevel()
```

### Output Destination

```go
// Write to file
logFile, _ := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
logging.WithOutput(logFile)

// Write to custom io.Writer
var buf bytes.Buffer
logging.WithOutput(&buf)
```

### Service Information

When configured, service metadata is automatically added to every log entry:

```go
logger := logging.MustNew(
    logging.WithServiceName("my-api"),
    logging.WithServiceVersion("v1.0.0"),
    logging.WithEnvironment("production"),
)

logger.Info("server started", "port", 8080)
// Output: {"level":"INFO","msg":"server started","service":"my-api","version":"v1.0.0","env":"production","port":8080}
```

### Source Code Location

```go
// Add file:line information to logs
logging.WithSource(true)
// Output: ... (logging.go:123)
```

### Custom Attribute Replacer

```go
logging.WithReplaceAttr(func(groups []string, a slog.Attr) slog.Attr {
    // Custom logic for transforming attributes
    if a.Key == "sensitive_data" {
        return slog.String(a.Key, "***HIDDEN***")
    }
    return a
})
```

### Global Logger Registration

By default, loggers are not registered globally, allowing multiple logger instances to coexist. Use `WithGlobalLogger()` to set your logger as the `slog` default:

```go
// Register as the global slog default
logger := logging.MustNew(
    logging.WithJSONHandler(),
    logging.WithServiceName("my-api"),
    logging.WithGlobalLogger(), // Now slog.Info() uses this logger
)
defer logger.Shutdown(context.Background())

// These now use your configured logger
slog.Info("using global logger", "key", "value")
```

This is useful when:
- You want third-party libraries using `slog` to use your configured logger
- You prefer using `slog.Info()` directly instead of `logger.Info()`
- You're migrating from direct `slog` usage to Rivaas logging

**Default behavior**: Loggers are **not** registered globally. This allows multiple independent logger instances in the same process, which is useful for:
- Testing with isolated loggers
- Libraries that shouldn't affect global state
- Applications with multiple logging configurations

## Sensitive Data Redaction

The logger automatically redacts sensitive fields:

```go
log.Info("user login", 
    "username", "john",
    "password", "secret123",    // Will be redacted
    "token", "abc123",          // Will be redacted
    "api_key", "xyz789",        // Will be redacted
)

// Output: {"msg":"user login","username":"john","password":"***REDACTED***","token":"***REDACTED***","api_key":"***REDACTED***"}
```

Automatically redacted fields:

- `password`
- `token`
- `secret`
- `api_key`
- `authorization`

## Convenience Methods

The logger provides helper methods for common logging patterns.

### LogRequest - HTTP Request Logging

Automatically logs HTTP requests with standard fields:

```go
func handleRequest(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    
    // Process request...
    status := 200
    bytesWritten := 1024
    
    logger.LogRequest(r, 
        "status", status,
        "duration_ms", time.Since(start).Milliseconds(),
        "bytes", bytesWritten,
    )
}
// Output includes: method, path, remote, user_agent, query (if present)
```

### LogError - Error Logging with Context

Convenient error logging with automatic error field:

```go
if err := db.Insert(user); err != nil {
    logger.LogError(err, "database operation failed",
        "operation", "INSERT",
        "table", "users",
        "retry_count", 3,
    )
    return err
}
```

### LogDuration - Operation Timing

Track operation duration automatically:

```go
start := time.Now()
result, err := processData(data)
logger.LogDuration("data processing completed", start,
    "rows_processed", result.Count,
    "errors", result.Errors,
)
// Automatically includes: duration_ms (for filtering) and duration (human-readable)
```

### ErrorWithStack - Error Logging with Stack Traces

For critical errors that need debugging context:

```go
logger.ErrorWithStack("critical payment failure", err, true,
    "user_id", userID,
    "amount", amount,
    "payment_method", method,
)
```

**When to use stack traces:**
- ✓ Critical errors requiring debugging
- ✓ Unexpected conditions (panics, invariant violations)
- ✗ Expected errors (validation failures, not found)
- ✗ High-frequency errors where capture cost matters

## Log Sampling

Reduce log volume in high-traffic scenarios while maintaining visibility:

```go
logger := logging.MustNew(
    logging.WithJSONHandler(),
    logging.WithSampling(logging.SamplingConfig{
        Initial:    100,           // Log first 100 entries unconditionally
        Thereafter: 100,           // After that, log 1 in every 100
        Tick:       time.Minute,   // Reset counter every minute
    }),
)
```

**How it works:**
1. Logs the first `Initial` entries (e.g., first 100)
2. After that, logs 1 in every `Thereafter` entries (e.g., 1%)
3. Resets counter every `Tick` interval to ensure recent activity visibility

**Important notes:**
- Errors (level >= ERROR) **always bypass sampling** to ensure critical issues are never dropped
- Useful for high-throughput services (>1000 logs/sec)
- Helps prevent log storage costs from spiraling
- Maintains statistical sampling for debugging

Example for production API:

```go
// Log all errors, but only 1% of info/debug in steady state
logger := logging.MustNew(
    logging.WithJSONHandler(),
    logging.WithLevel(logging.LevelInfo),
    logging.WithSampling(logging.SamplingConfig{
        Initial:    1000,          // First 1000 requests fully logged
        Thereafter: 100,           // Then 1% sampling
        Tick:       5 * time.Minute, // Reset every 5 minutes
    }),
)
```

## Dynamic Log Level Changes

Change log levels at runtime without restarting your application:

```go
logger := logging.MustNew(logging.WithJSONHandler())

// Enable debug logging temporarily for troubleshooting
if err := logger.SetLevel(logging.LevelDebug); err != nil {
    log.Printf("failed to change level: %v", err)
}

// Reduce to warnings only during high traffic
if err := logger.SetLevel(logging.LevelWarn); err != nil {
    log.Printf("failed to change level: %v", err)
}

// Check current level
currentLevel := logger.Level()
```

**Use cases:**
- Enable debug logging temporarily for troubleshooting
- Reduce log volume during traffic spikes
- Runtime configuration via HTTP endpoint or signal handler

**Limitations:**
- Not supported with custom loggers (returns `ErrCannotChangeLevel`)
- Brief window where old/new levels may race during transition

**Example with HTTP endpoint:**

```go
http.HandleFunc("/admin/loglevel", func(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    levelStr := r.URL.Query().Get("level")
    var level logging.Level
    switch levelStr {
    case "debug":
        level = logging.LevelDebug
    case "info":
        level = logging.LevelInfo
    case "warn":
        level = logging.LevelWarn
    case "error":
        level = logging.LevelError
    default:
        http.Error(w, "Invalid level", http.StatusBadRequest)
        return
    }
    
    if err := logger.SetLevel(level); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, "Log level changed to %s\n", levelStr)
})
```

## Context-Aware Logging

### With OpenTelemetry Tracing

When using with tracing, logs automatically include trace and span IDs:

```go
import (
    "rivaas.dev/logging"
    "rivaas.dev/tracing"
)

// Create logger
log := logging.MustNew(logging.WithJSONHandler())

// In a traced request
func handler(ctx context.Context) {
    cl := logging.NewContextLogger(ctx, log)
    
    cl.Info("processing request", "user_id", "123")
    // Output includes: "trace_id":"abc123...", "span_id":"def456..."
}
```

### Structured Context

```go
// Add context fields that persist across log calls
contextLogger := log.With(
    "request_id", "req-123",
    "user_id", "user-456",
)

contextLogger.Info("validation started")
contextLogger.Info("validation completed")
// Both logs include request_id and user_id
```

### Grouped Attributes

```go
requestLogger := log.WithGroup("request")
requestLogger.Info("received", 
    "method", "POST",
    "path", "/api/users",
)
// Output: {"msg":"received","request":{"method":"POST","path":"/api/users"}}
```

## Router Integration

The logging package integrates with the Rivaas router via the `SetLogger` method.

### Basic Integration

```go
import (
    "rivaas.dev/router"
    "rivaas.dev/logging"
)

func main() {
    // Create logger
    logger := logging.MustNew(
        logging.WithConsoleHandler(),
        logging.WithDebugLevel(),
    )
    
    // Create router and set logger
    r := router.MustNew()
    r.SetLogger(logger)
    
    r.GET("/", func(c *router.Context) {
        c.Logger().Info("handling request")
        c.JSON(200, map[string]string{"status": "ok"})
    })
    
    r.Run(":8080")
}
```

### With Full Observability

For full observability (logging, metrics, tracing), use the `app` package which wires everything together:

```go
import (
    "rivaas.dev/app"
    "rivaas.dev/logging"
    "rivaas.dev/tracing"
)

a, err := app.New(
    app.WithServiceName("my-api"),
    app.WithObservability(
        app.WithLogging(logging.WithJSONHandler()),
        app.WithMetrics(), // Prometheus is default
        app.WithTracing(tracing.WithOTLP("localhost:4317")),
    ),
)
```

## Environment Variables

The logger respects standard OpenTelemetry environment variables:

```bash
# Service identification
export OTEL_SERVICE_NAME=my-api
export OTEL_SERVICE_VERSION=v1.0.0
export RIVAAS_ENVIRONMENT=production
```

## Advanced Usage

### Custom Logger

You can provide your own `slog.Logger`:

```go
import "log/slog"

customLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
    AddSource: true,
}))

cfg := logging.MustNew(
    logging.WithCustomLogger(customLogger),
)
```

### Multiple Loggers

Create different loggers for different purposes:

```go
// Application logger
appLog := logging.MustNew(
    logging.WithJSONHandler(),
    logging.WithLevel(logging.LevelInfo),
)

// Debug logger
debugLog := logging.MustNew(
    logging.WithConsoleHandler(),
    logging.WithDebugLevel(),
    logging.WithSource(true),
)

// Audit logger
auditFile, _ := os.OpenFile("audit.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
auditLog := logging.MustNew(
    logging.WithJSONHandler(),
    logging.WithOutput(auditFile),
)
```

### Graceful Shutdown

```go
log := logging.MustNew(logging.WithJSONHandler())

// On shutdown
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

if err := log.Shutdown(ctx); err != nil {
    // Handle shutdown error
}
```

## Best Practices

### Use Structured Logging

```go
// BAD - string concatenation
log.Info("User " + userID + " logged in from " + ipAddress)

// GOOD - structured fields
log.Info("user logged in",
    "user_id", userID,
    "ip_address", ipAddress,
    "session_id", sessionID,
)
```

### Log Appropriate Levels

```go
// DEBUG - detailed information for debugging
log.Debug("cache hit", "key", cacheKey, "ttl", ttl)

// INFO - general informational messages
log.Info("server started", "port", 8080)

// WARN - warning but not an error
log.Warn("high memory usage", "used_mb", 8192, "total_mb", 16384)

// ERROR - errors that need attention
log.Error("database connection failed", "error", err, "retry_count", retries)
```

### Don't Log in Tight Loops

```go
// BAD - logs thousands of times
for _, item := range items {
    log.Debug("processing", "item", item)
    process(item)
}

// GOOD - log once with summary
log.Info("processing batch", "count", len(items))
for _, item := range items {
    process(item)
}
log.Info("batch completed", "processed", len(items), "duration", elapsed)
```

### Include Context

```go
// Minimal context
log.Error("failed to save", "error", err)

// Better - includes relevant context
log.Error("failed to save user data",
    "error", err,
    "user_id", user.ID,
    "operation", "update_profile",
    "retry_count", retries,
)
```

## Performance Considerations

The logging package is designed for high-performance production use with minimal overhead.

### Optimization Tips

1. **Set appropriate log levels**: Debug logging has overhead; use INFO+ in production
2. **Avoid logging in tight loops**: Log batch summaries instead
3. **Use structured fields**: More efficient than string concatenation
4. **Reuse loggers**: Create once, use many times

### Memory Usage

- **Logger creation**: ~1KB per logger instance
- **No allocations**: Zero allocs for standard log calls
- **Pooling**: Consider `sync.Pool` for ContextLogger in extreme high-load scenarios

### Performance Best Practices

```go
// GOOD - Structured, efficient
logger.Info("user action",
    "user_id", userID,
    "action", "login",
    "duration_ms", elapsed.Milliseconds(),
)

// BAD - String concatenation, inefficient
logger.Info(fmt.Sprintf("User %s performed %s in %dms", userID, "login", elapsed.Milliseconds()))

// GOOD - Use router accesslog middleware for HTTP logging
// See router/middleware/accesslog for details

// GOOD - Appropriate log level for production
logger := logging.MustNew(
    logging.WithJSONHandler(),
    logging.WithLevel(logging.LevelInfo),  // Skip debug logs
)
```

### Profiling

Run benchmarks to measure performance:

```bash
# Run all benchmarks
go test -bench=. -benchmem ./logging

# Benchmark specific handler
go test -bench=BenchmarkJSONHandler -benchmem

# CPU profiling
go test -bench=BenchmarkLogging -cpuprofile=cpu.prof
go tool pprof cpu.prof

# Memory profiling
go test -bench=BenchmarkConcurrentLogging -memprofile=mem.prof
go tool pprof mem.prof
```

## Examples

See the [examples directory](./examples/) for complete working examples:

- Basic logging
- Router integration
- Context-aware logging
- Custom handlers
- Multiple output formats

## Migration Guides

Switching from other popular Go logging libraries to Rivaas logging is straightforward.

### From logrus

**logrus** is a popular structured logger, but Rivaas logging offers better performance and stdlib integration.

```go
// BEFORE (logrus)
import "github.com/sirupsen/logrus"

log := logrus.New()
log.SetFormatter(&logrus.JSONFormatter{})
log.SetLevel(logrus.InfoLevel)
log.SetOutput(os.Stdout)

log.WithFields(logrus.Fields{
    "user_id": "123",
    "action": "login",
}).Info("User logged in")

// AFTER (rivaas/logging)
import "rivaas.dev/logging"

log := logging.MustNew(
    logging.WithJSONHandler(),
    logging.WithLevel(logging.LevelInfo),
    logging.WithOutput(os.Stdout),
)

log.Info("User logged in",
    "user_id", "123",
    "action", "login",
)
```

**Key Differences**:

- Use `logging.WithJSONHandler()` instead of `&logrus.JSONFormatter{}`
- Fields are inline, not in `WithFields()` map
- `logging.LevelInfo` instead of `logrus.InfoLevel`
- Faster performance, fewer allocations

### From zap

**zap** is very fast, but Rivaas logging offers similar performance with simpler API.

```go
// BEFORE (zap)
import "go.uber.org/zap"

logger, _ := zap.NewProduction()
defer logger.Sync()

logger.Info("User logged in",
    zap.String("user_id", "123"),
    zap.String("action", "login"),
    zap.Int("status", 200),
)

// AFTER (rivaas/logging)
import "rivaas.dev/logging"

logger := logging.MustNew(logging.WithJSONHandler())
defer logger.Shutdown(context.Background())

logger.Info("User logged in",
    "user_id", "123",
    "action", "login",
    "status", 200,
)
```

**Key Differences**:

- No need for `zap.String()` wrappers - pass values directly
- `Shutdown()` instead of `Sync()`
- Simpler API, similar performance
- Standard library based

### From zerolog

**zerolog** is very fast, but Rivaas logging is simpler and uses stdlib.

```go
// BEFORE (zerolog)
import "github.com/rs/zerolog"

logger := zerolog.New(os.Stdout).With().
    Str("service", "myapp").
    Str("version", "1.0.0").
    Logger()

logger.Info().
    Str("user_id", "123").
    Str("action", "login").
    Msg("User logged in")

// AFTER (rivaas/logging)
import "rivaas.dev/logging"

logger := logging.MustNew(
    logging.WithJSONHandler(),
    logging.WithServiceName("myapp"),
    logging.WithServiceVersion("1.0.0"),
    logging.WithEnvironment("production"),
)

logger.Info("User logged in",
    "user_id", "123",
    "action", "login",
)
```

**Key Differences**:

- Simpler syntax - no chaining
- `WithServiceName()`, `WithServiceVersion()`, and `WithEnvironment()` for service metadata
- Fields passed directly, not via chainable methods
- Better integration with OpenTelemetry

### From stdlib log

**Standard library log** is simple but unstructured. Rivaas logging adds structure while using stdlib slog.

```go
// BEFORE (stdlib log)
import "log"

log.SetOutput(os.Stdout)
log.SetPrefix("[INFO] ")
log.Printf("User %s logged in from %s", userID, ipAddress)

// AFTER (rivaas/logging)
import "rivaas.dev/logging"

logger := logging.MustNew(
    logging.WithJSONHandler(),
    logging.WithLevel(logging.LevelInfo),
)

logger.Info("User logged in",
    "user_id", userID,
    "ip_address", ipAddress,
)
```

**Key Benefits**:

- Structured logging (machine parseable)
- Log levels (Debug, Info, Warn, Error)
- Automatic sensitive data redaction
- OpenTelemetry integration
- Multiple output formats

### Migration Checklist

When migrating from another logger:

- [ ] Replace logger initialization
- [ ] Update all log calls to structured format (key-value pairs)
- [ ] Replace log level constants (`logrus.InfoLevel` → `logging.LevelInfo`)
- [ ] Update context/field methods (`WithFields()` → inline fields)
- [ ] Replace typed field methods (`zap.String()` → direct values)
- [ ] Update error handling (`Sync()` → `Shutdown()`)
- [ ] Test with new logger
- [ ] Update imports
- [ ] Remove old logger dependency

## Troubleshooting

### Common Issues

#### Logs Not Appearing

```go
// Check log level - debug logs won't show at INFO level
logger := logging.MustNew(
    logging.WithDebugLevel(),  // Enable debug logs
)
```

#### Sensitive Data Not Redacted

```go
// Custom fields need custom redaction
logger := logging.MustNew(
    logging.WithReplaceAttr(func(groups []string, a slog.Attr) slog.Attr {
        if a.Key == "credit_card" {
            return slog.String(a.Key, "***REDACTED***")
        }
        return a
    }),
)
```

#### No Trace IDs in Logs

```go
// Ensure tracing is initialized and context is propagated
tracer := tracing.MustNew(tracing.WithOTLP("localhost:4317"))
ctx, span := tracer.Start(context.Background(), "operation")
defer span.End()

// Use context logger
cl := logging.NewContextLogger(logger, ctx)
cl.Info("traced message")  // Will include trace_id and span_id
```

#### Access Log Not Working

```go
// Ensure accesslog middleware is applied and logger is configured
r := router.New()
logger := logging.MustNew(logging.WithJSONHandler())
r.SetLogger(logger)
r.Use(accesslog.New())
```

#### High Memory Usage

```go
// Reduce log volume
logger := logging.MustNew(
    logging.WithLevel(logging.LevelWarn),  // Only warnings and errors
)

// Use router accesslog middleware with path exclusions
r.Use(accesslog.New(
    accesslog.WithExcludePaths("/health", "/metrics", "/ready"),
))
```

### Debugging

Enable source location to see where logs originate:

```go
logger := logging.MustNew(
    logging.WithSource(true),  // Adds file:line to logs
    logging.WithDebugLevel(),
)

// Output includes: ... "source":{"file":"main.go","line":42} ...
```

### Getting Help

- Check the [examples directory](./examples/) for working code
- Review [test files](.) for usage patterns
- See [Rivaas documentation](https://github.com/rivaas-dev/rivaas) for integration guides

## Testing

```bash
# Run all tests
go test ./logging

# Run with coverage
go test ./logging -cover

# Run with verbose output
go test ./logging -v

# Run with race detector
go test ./logging -race

# Run benchmarks
go test -bench=. -benchmem ./logging
```

## API Reference

### Core Types

- `Logger` - Main logging type (created via `New()` or `MustNew()`)
- `Option` - Functional option type
- `HandlerType` - Log output format type
- `Level` - Log level type
- `ContextLogger` - Context-aware logger with trace correlation

### Main Functions

- `New(opts ...Option) (*Logger, error)` - Create new logger
- `MustNew(opts ...Option) *Logger` - Create new logger or panic
- `NewContextLogger(ctx context.Context, logger *Logger) *ContextLogger` - Create context logger

### Logger Methods

- `Logger() *slog.Logger` - Get underlying slog logger
- `With(args ...any) *slog.Logger` - Create logger with attributes
- `WithGroup(name string) *slog.Logger` - Create logger with group
- `Debug(msg string, args ...any)` - Log debug message
- `Info(msg string, args ...any)` - Log info message
- `Warn(msg string, args ...any)` - Log warning message
- `Error(msg string, args ...any)` - Log error message
- `SetLevel(level Level) error` - Change log level dynamically
- `Level() Level` - Get current log level
- `ServiceName() string` - Get configured service name
- `ServiceVersion() string` - Get configured service version
- `Environment() string` - Get configured environment
- `IsEnabled() bool` - Check if logger is active (not shut down)
- `DebugInfo() map[string]any` - Get diagnostic information about logger state
- `Shutdown(ctx context.Context) error` - Graceful shutdown

### Convenience Methods

- `LogRequest(r *http.Request, extra ...any)` - Log HTTP request with standard fields
- `LogError(err error, msg string, extra ...any)` - Log error with context
- `LogDuration(msg string, start time.Time, extra ...any)` - Log operation duration
- `ErrorWithStack(msg string, err error, includeStack bool, extra ...any)` - Log error with optional stack trace

### ContextLogger Methods

- `Logger() *slog.Logger` - Get underlying slog logger
- `TraceID() string` - Get trace ID if available
- `SpanID() string` - Get span ID if available
- `With(args ...any) *slog.Logger` - Create logger with additional attributes
- `Debug(msg string, args ...any)` - Log debug message with context
- `Info(msg string, args ...any)` - Log info message with context
- `Warn(msg string, args ...any)` - Log warning message with context
- `Error(msg string, args ...any)` - Log error message with context

### Configuration Functions

- `WithJSONHandler()` - Use JSON output
- `WithTextHandler()` - Use text output
- `WithConsoleHandler()` - Use colored console output
- `WithLevel(level Level)` - Set minimum log level
- `WithDebugLevel()` - Enable debug logging
- `WithOutput(w io.Writer)` - Set output destination
- `WithServiceName(name string)` - Set service name
- `WithServiceVersion(version string)` - Set service version
- `WithEnvironment(env string)` - Set environment
- `WithSource(enabled bool)` - Add source location to logs
- `WithReplaceAttr(fn)` - Custom attribute replacer
- `WithCustomLogger(logger *slog.Logger)` - Use custom logger
- `WithGlobalLogger()` - Register as global slog default
- `WithSampling(cfg SamplingConfig)` - Configure log sampling

### Types

- `HandlerType` - Log output format type (JSONHandler, TextHandler, ConsoleHandler)
- `Level` - Log level type (LevelDebug, LevelInfo, LevelWarn, LevelError)
- `SamplingConfig` - Log sampling configuration
  - `Initial int` - Log first N entries unconditionally
  - `Thereafter int` - After Initial, log 1 of every M entries
  - `Tick time.Duration` - Reset sampling counter every interval

### Error Types

The package defines sentinel errors for better error handling:

- `ErrNilLogger` - Custom logger is nil (returned by `New()` with `WithCustomLogger(nil)`)
- `ErrInvalidHandler` - Invalid handler type specified
- `ErrLoggerShutdown` - Logger has been shut down (operations fail gracefully)
- `ErrInvalidLevel` - Invalid log level provided
- `ErrCannotChangeLevel` - Cannot change level on custom logger (returned by `SetLevel()`)

**Error handling example:**

```go
logger, err := logging.New(
    logging.WithCustomLogger(customSlog),
)
if err != nil {
    if errors.Is(err, logging.ErrNilLogger) {
        // Handle nil logger case
        log.Fatal("custom logger cannot be nil")
    }
    log.Fatalf("failed to create logger: %v", err)
}

// Later, trying to change level
if err := logger.SetLevel(logging.LevelDebug); err != nil {
    if errors.Is(err, logging.ErrCannotChangeLevel) {
        // Expected when using custom logger
        log.Println("cannot change level on custom logger")
    } else {
        log.Printf("unexpected error: %v", err)
    }
}
```

## License

MIT License - see LICENSE file for details

## Contributing

Contributions are welcome! Please see the main repository for contribution guidelines.

## Related Packages

- [router](../router/) - High-performance HTTP router
- [metrics](../metrics/) - OpenTelemetry metrics
- [tracing](../tracing/) - Distributed tracing
- [app](../app/) - Batteries-included framework
