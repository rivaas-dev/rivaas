# AccessLog

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/router/middleware/accesslog.svg)](https://pkg.go.dev/rivaas.dev/router/middleware/accesslog)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../../LICENSE)

Log every HTTP request and response in a structured way. You get method, path, status, duration, client IP, and more. Handy for debugging and monitoring.

> **Full docs:** [Middleware Guide](https://rivaas.dev/docs/guides/router/middleware/) and [Middleware Reference](https://rivaas.dev/docs/reference/packages/router/middleware/).

## Features

- Structured logging with Go's `log/slog`
- Logs method, path, status, duration, client IP, user agent
- Skip noisy paths (e.g. health checks, metrics)
- Optional sampling to reduce log volume
- Works with the requestid middleware for correlation IDs
- Optional slow-request and errors-only logging

## Installation

```bash
go get rivaas.dev/router/middleware/accesslog
```

Requires Go 1.25 or later.

## Quick Start

```go
package main

import (
    "log/slog"
    "net/http"
    "os"

    "rivaas.dev/router"
    "rivaas.dev/router/middleware/accesslog"
    "rivaas.dev/router/middleware/requestid"
)

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    r := router.New()

    r.Use(requestid.New())
    r.Use(accesslog.New(accesslog.WithLogger(logger)))

    r.GET("/", func(c *router.Context) {
        c.String(http.StatusOK, "Hello, World!")
    })

    http.ListenAndServe(":8080", r)
}
```

## Configuration

Use options to tune what gets logged and where:

```go
r.Use(accesslog.New(
    accesslog.WithLogger(logger),
    accesslog.WithExcludePaths("/health", "/metrics"),
    accesslog.WithExcludePrefixes("/debug"),
    accesslog.WithSampleRate(0.1),  // Log 10% of requests
    accesslog.WithSlowThreshold(2*time.Second),
    accesslog.WithLogErrorsOnly(false),
))
```

| Option | What it does |
|--------|----------------|
| `WithLogger` | Set the slog logger (required if you want custom output) |
| `WithExcludePaths` | Do not log these exact paths |
| `WithExcludePrefixes` | Do not log paths that start with these prefixes |
| `WithSampleRate` | Log only a fraction of requests (0.1 = 10%) |
| `WithSlowThreshold` | Always log requests slower than this duration |
| `WithLogErrorsOnly` | Log only requests with status >= 400 |

## Examples

A full runnable example with different setups is in the `example/` directory:

```bash
cd example
go run main.go
```

Then try the endpoints and watch the logs.

## Learn More

- [Middleware overview](../README.md) – All middleware and recommended order
- [RequestID middleware](../requestid/) – Add request IDs for tracing
- [Recovery middleware](../recovery/) – Recover from panics

## License

Apache License 2.0 – see [LICENSE](../../LICENSE) for details.
