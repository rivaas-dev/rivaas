# Recovery

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/middleware/recovery.svg)](https://pkg.go.dev/rivaas.dev/middleware/recovery)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../../LICENSE)

Catch panics in your handlers so the server does not crash. The middleware logs the panic (with stack trace), then returns a proper error response instead of killing the process.

> **Full docs:** [Middleware Guide](https://rivaas.dev/docs/guides/router/middleware/) and [Middleware Reference](https://rivaas.dev/docs/reference/packages/router/middleware/).

## Features

- Recovers from panics in handlers and later middleware
- Logs panic value and stack trace
- Sends a 500 response instead of closing the connection
- Optional custom recovery handler (e.g. JSON error body)
- Works with OpenTelemetry (marks span with exception info)
- Configurable stack trace size and logging

## Installation

```bash
go get rivaas.dev/middleware/recovery
```

Requires Go 1.25 or later.

## Quick Start

```go
package main

import (
    "net/http"
    "rivaas.dev/router"
    "rivaas.dev/middleware/recovery"
)

func main() {
    r := router.New()
    r.Use(recovery.New())

    r.GET("/", func(c *router.Context) {
        c.String(http.StatusOK, "Hello")
    })

    r.GET("/panic", func(c *router.Context) {
        panic("something went wrong") // Recovery catches this
    })

    http.ListenAndServe(":8080", r)
}
```

Register recovery early (or first) in the chain so it wraps all handlers below it.

## Configuration

| Option | What it does |
|--------|----------------|
| `WithStackTrace` | Include stack trace in logs (default: true) |
| `WithStackSize` | Max stack trace size in bytes (default: 4KB) |
| `WithLogger` | Custom function to log the panic |
| `WithHandler` | Custom function to write the error response |
| `WithDisableStackAll` | Do not dump all goroutine stacks |

Custom error response:

```go
r.Use(recovery.New(
    recovery.WithHandler(func(c *router.Context, err any) {
        c.JSON(http.StatusInternalServerError, map[string]any{
            "error":      "Internal server error",
            "request_id": requestid.Get(c),
        })
    }),
))
```

## OpenTelemetry

When you use tracing, the middleware records the panic on the current span (exception type, message, etc.) so it shows up in your observability tools.

## Examples

A runnable example is in the `example/` directory:

```bash
cd example
go run main.go
```

Hit a route that panics and check the logs and response.

## Learn More

- [Middleware overview](../README.md) – All middleware and recommended order
- [RequestID middleware](../requestid/) – Add request IDs for debugging
- [AccessLog middleware](../accesslog/) – Log every request

## License

Apache License 2.0 – see [LICENSE](../../LICENSE) for details.
