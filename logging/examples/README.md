# Logging Examples

Practical examples demonstrating the logging package features.

## Running Examples

```bash
cd <example-directory>
go run main.go
```

## Examples

### 01-quickstart — Development Basics

Getting started with logging for development environments.

**Covers:**
- Console handler initialization (human-readable output)
- Log levels: Debug, Info, Warn, Error
- Structured attributes with key-value pairs
- Dynamic level changes at runtime

```bash
cd 01-quickstart && go run main.go

# Enable debug logging
LOG_DEBUG=true go run main.go
```

### 02-production — Production Configuration

Production-ready logging setup with JSON output and high-throughput patterns.

**Covers:**
- JSON handler for structured, machine-readable logs
- Service metadata (name, version, environment)
- Configuration validation
- Batch logging for high-throughput scenarios
- Sampling to reduce log volume

```bash
cd 02-production && go run main.go
```

### 03-helper-methods — Logging Utilities

Convenience methods for common logging patterns.

**Covers:**
- `LogDuration` — timing operations
- `LogError` / `ErrorWithStack` — error handling with optional stack traces
- `LogRequest` — HTTP request logging
- Context-based request-scoped loggers
- `DebugInfo()` — diagnostic information

```bash
cd 03-helper-methods && go run main.go
```

### 04-testing — Test Utilities

Utilities for testing logging behavior in your applications.

**Covers:**
- `NewTestLogger()` — in-memory log capture
- `ParseJSONLogEntries()` — parsing logs for assertions
- Common test patterns for verifying logging behavior

```bash
cd 04-testing && go run main.go
```

## Quick Reference

### Console Logging (Development)

```go
logger := logging.MustNew(
    logging.WithConsoleHandler(),
    logging.WithLevel(logging.LevelDebug),
)
```

### JSON Logging (Production)

```go
logger := logging.MustNew(
    logging.WithJSONHandler(),
    logging.WithServiceName("my-api"),
    logging.WithServiceVersion("v1.0.0"),
    logging.WithEnvironment("production"),
)
```

### With Router

```go
logger := logging.MustNew(
    logging.WithJSONHandler(),
    logging.WithServiceName("api"),
)

r := router.MustNew()
r.SetLogger(logger)
```

### With App (Recommended)

```go
app := app.New(
    app.WithLogging(
        logging.WithJSONHandler(),
    ),
)
```

## Learn More

See the [main logging README](../README.md) for complete documentation.
