# Timeout

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/middleware/timeout.svg)](https://pkg.go.dev/rivaas.dev/middleware/timeout)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../../LICENSE)

Stop requests that run too long. The middleware sets a deadline on the request; if the handler exceeds it, the context is canceled and the client gets a timeout response. Helps avoid stuck handlers using resources forever.

> **Full docs:** [Middleware Guide](https://rivaas.dev/docs/guides/router/middleware/) and [Middleware Reference](https://rivaas.dev/docs/reference/packages/router/middleware/).

## Features

- Set a max duration per request (default 30s)
- Request context is canceled when the timeout is reached
- Returns 408 Request Timeout by default; custom handler supported
- Skip timeout for specific paths or prefixes (e.g. streaming, webhooks)
- Optional logging when a timeout happens
- Handlers can check context and return early

## Installation

```bash
go get rivaas.dev/middleware/timeout
```

Requires Go 1.25 or later.

## Quick Start

```go
package main

import (
    "net/http"
    "time"
    "rivaas.dev/router"
    "rivaas.dev/middleware/timeout"
)

func main() {
    r := router.New()
    r.Use(timeout.New(timeout.WithDuration(10 * time.Second)))

    r.GET("/", func(c *router.Context) {
        c.String(http.StatusOK, "Hello")
    })

    http.ListenAndServe(":8080", r)
}
```

## Configuration

| Option | What it does |
|--------|----------------|
| `WithDuration` | Max time for the request (default: 30s) |
| `WithHandler` | Custom response when timeout happens (default: 408) |
| `WithSkipPaths` | Exact paths to exclude from timeout |
| `WithSkipPrefix` | Path prefixes to exclude (e.g. /stream) |
| `WithSkipSuffix` | Path suffixes to exclude |
| `WithSkip` | Custom function to skip timeout for a request |
| `WithoutLogging` | Do not log timeout events |

Skip timeout for some paths:

```go
r.Use(timeout.New(
    timeout.WithDuration(30*time.Second),
    timeout.WithSkipPaths("/webhook", "/health"),
    timeout.WithSkipPrefix("/stream"),
))
```

Handlers should respect context cancellation:

```go
func handler(c *router.Context) {
    ctx := c.Request.Context()
    select {
    case <-ctx.Done():
        return // Timeout or canceled
    case result := <-doWork(ctx):
        c.JSON(http.StatusOK, result)
    }
}
```

## Examples

A runnable example is in the `example/` directory:

```bash
cd example
go run main.go
```

## Learn More

- [Middleware overview](../README.md) – All middleware and recommended order
- [Recovery middleware](../recovery/) – Panic recovery
- [RateLimit middleware](../ratelimit/) – Limit request rate

## License

Apache License 2.0 – see [LICENSE](../../LICENSE) for details.
