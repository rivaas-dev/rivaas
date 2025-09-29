# Logging Examples

This directory contains complete examples demonstrating various logging features.

## Examples

Run any example with:

```bash
cd <example-directory>
go run main.go
```

### Basic Usage

- **01-basic-init-and-levels** - Console handler, log levels, SetLevel
- **02-structured-attrs** - Structured fields, nested-style keys, redaction-friendly keys
- **03-functional-options-and-validate** - Functional options, Validate errors
- **04-dynamic-level-change** - Level from env, runtime SetLevel

### Production Patterns

- **05-json-handler** - JSON handler in production-style setup
- **06-batch-logger** - NewBatchLogger, periodic flush, graceful close
- **07-error-with-stack** - ErrorWithStack vs regular Error
- **08-log-duration** - Duration logging for success/error paths

### Advanced Features

- **09-log-request-standalone** - Request logging without router/metrics/tracing
- **10-context-fields-only** - Add request/user IDs from context as structured fields
- **11-debug-info** - DebugInfo() diagnostic information
- **12-test-helper-standalone** - NewTestLogger() + ParseJSONLogEntries() in-memory

### HTTP Integration

- **13-http-middleware** - HTTP middleware for request/response logging

## Running All Examples

```bash
# From the examples directory
for dir in */; do
    if [ -d "$dir" ] && [ -f "$dir/main.go" ]; then
        echo "Running $dir..."
        (cd "$dir" && go run main.go)
        echo ""
    fi
done
```

## Common Patterns

### Console Logging (Development)

```go
log := logging.MustNew(
    logging.WithConsoleHandler(),
    logging.WithDebugLevel(),
)
```

### JSON Logging (Production)

```go
log := logging.MustNew(
    logging.WithJSONHandler(),
    logging.WithServiceName("my-api"),
    logging.WithServiceVersion("v1.0.0"),
    logging.WithEnvironment("production"),
)
```

### With Router

```go
r := router.New(
    logging.WithLogging(
        logging.WithJSONHandler(),
        logging.WithServiceName("api"),
        logging.WithServiceVersion("v1"),
        logging.WithEnvironment("prod"),
    ),
)
```

### With App (Recommended)

```go
app := app.New(
    app.WithLogging(
        logging.WithJSONHandler(),
    ),
)
```

## Testing

Each example can be tested individually. Start the server and use `curl` or your favorite HTTP client to test the endpoints.

## Learn More

See the [main logging README](../README.md) for complete documentation.
