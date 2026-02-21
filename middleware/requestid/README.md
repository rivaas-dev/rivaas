# RequestID

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/middleware/requestid.svg)](https://pkg.go.dev/rivaas.dev/middleware/requestid)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../../LICENSE)

Give every request a unique ID. You can use it in logs and across services to trace a single request from start to finish.

> **Full docs:** [Middleware Guide](https://rivaas.dev/docs/guides/router/middleware/) and [Middleware Reference](https://rivaas.dev/docs/reference/packages/router/middleware/).

## Features

- Generates a unique ID per request (UUID v7 by default, or ULID)
- Uses the client's X-Request-ID if they send one (good for tracing)
- Puts the ID in the response header so clients can log it
- Short option: ULID (26 chars) instead of UUID (36 chars)
- Custom header name and custom ID generator supported

## Installation

```bash
go get rivaas.dev/middleware/requestid
```

Requires Go 1.25 or later.

## Quick Start

```go
package main

import (
    "net/http"
    "rivaas.dev/router"
    "rivaas.dev/middleware/requestid"
)

func main() {
    r := router.New()
    r.Use(requestid.New())

    r.GET("/", func(c *router.Context) {
        id := requestid.Get(c)
        c.JSON(http.StatusOK, map[string]string{
            "request_id": id,
        })
    })

    http.ListenAndServe(":8080", r)
}
```

Register requestid early so the ID is available to accesslog and other middleware.

## Configuration

| Option | What it does |
|--------|----------------|
| `WithHeader` | Header name for request ID (default: X-Request-ID) |
| `WithULID` | Use ULID instead of UUID v7 (shorter IDs) |
| `WithGenerator` | Your own function to generate IDs |
| `WithAllowClientID` | Whether to accept an ID from the client (default: true) |

Use ULID for shorter IDs:

```go
r.Use(requestid.New(requestid.WithULID()))
```

Custom header:

```go
r.Use(requestid.New(requestid.WithHeader("X-Trace-ID")))
```

## Getting the ID in handlers

```go
id := requestid.Get(c)
logger.Info("processing request", "request_id", id, "path", c.Request.URL.Path)
```

## Examples

A runnable example is in the `example/` directory:

```bash
cd example
go run main.go
```

Check the response headers for X-Request-ID.

## Learn More

- [Middleware overview](../README.md) – All middleware and recommended order
- [AccessLog middleware](../accesslog/) – Log requests with request IDs
- [Recovery middleware](../recovery/) – Panic recovery

## License

Apache License 2.0 – see [LICENSE](../../LICENSE) for details.
