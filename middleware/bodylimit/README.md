# BodyLimit

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/middleware/bodylimit.svg)](https://pkg.go.dev/rivaas.dev/middleware/bodylimit)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../../LICENSE)

Limit how big request bodies can be. Stops huge payloads before they are fully read, which helps prevent abuse and protects your server from running out of memory.

> **Full docs:** [Middleware Guide](https://rivaas.dev/docs/guides/router/middleware/) and [Middleware Reference](https://rivaas.dev/docs/reference/packages/router/middleware/).

## Features

- Set a maximum request body size in bytes
- Reject oversized requests early (saves bandwidth and memory)
- Skip limits on specific paths (e.g. file uploads)
- Custom error response when the limit is exceeded
- Returns 413 Payload Too Large by default

## Installation

```bash
go get rivaas.dev/middleware/bodylimit
```

Requires Go 1.25 or later.

## Quick Start

```go
package main

import (
    "net/http"
    "rivaas.dev/router"
    "rivaas.dev/middleware/bodylimit"
)

func main() {
    r := router.New()

    // Limit body size to 10MB (default is 2MB)
    r.Use(bodylimit.New(
        bodylimit.WithLimit(10 * 1024 * 1024),
    ))

    r.POST("/upload", func(c *router.Context) {
        // Request body is already limited
        c.JSON(http.StatusOK, map[string]string{"status": "ok"})
    })

    http.ListenAndServe(":8080", r)
}
```

## Configuration

| Option | What it does |
|--------|----------------|
| `WithLimit` | Max body size in bytes (required; default 2MB if you use the zero value) |
| `WithSkipPaths` | Paths that do not apply the limit (e.g. large uploads) |
| `WithErrorHandler` | Custom response when body is too large (default: 413 JSON) |

Custom error handler:

```go
r.Use(bodylimit.New(
    bodylimit.WithLimit(5 * 1024 * 1024), // 5MB
    bodylimit.WithErrorHandler(func(c *router.Context, limit int64) {
        c.JSON(http.StatusRequestEntityTooLarge, map[string]string{
            "error":    "Request body too large",
            "max_size": fmt.Sprintf("%d bytes", limit),
        })
    }),
))
```

## Examples

A runnable example is in the `example/` directory:

```bash
cd example
go run main.go
```

## Learn More

- [Middleware overview](../README.md) – All middleware and recommended order
- [Timeout middleware](../timeout/) – Limit how long requests can run
- [RateLimit middleware](../ratelimit/) – Limit request rate per client

## License

Apache License 2.0 – see [LICENSE](../../LICENSE) for details.
