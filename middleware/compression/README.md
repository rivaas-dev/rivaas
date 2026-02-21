# Compression

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/middleware/compression.svg)](https://pkg.go.dev/rivaas.dev/middleware/compression)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../../LICENSE)

Compress HTTP responses automatically. Uses gzip, deflate, or brotli based on what the client supports. Reduces bandwidth and often speeds up responses for text and JSON.

> **Full docs:** [Middleware Guide](https://rivaas.dev/docs/guides/router/middleware/) and [Middleware Reference](https://rivaas.dev/docs/reference/packages/router/middleware/).

## Features

- Gzip, deflate, and brotli support (picks the best the client accepts)
- Compresses text, JSON, XML, and similar; skips images and other binary types by default
- Configurable compression level and minimum size
- Skip compression for specific paths (e.g. /metrics)
- No change needed in your handlers; compression happens in the middleware

## Installation

```bash
go get rivaas.dev/middleware/compression
```

Requires Go 1.25 or later.

## Quick Start

```go
package main

import (
    "net/http"
    "rivaas.dev/router"
    "rivaas.dev/middleware/compression"
)

func main() {
    r := router.New()
    r.Use(compression.New())

    r.GET("/", func(c *router.Context) {
        c.JSON(http.StatusOK, map[string]string{"message": "This response is compressed"})
    })

    http.ListenAndServe(":8080", r)
}
```

## Configuration

| Option | What it does |
|--------|----------------|
| `WithGzipLevel` | Gzip level 0–9 (higher = smaller but slower; default is standard) |
| `WithBrotliLevel` | Brotli level 0–11 (default 4 for dynamic content) |
| `WithBrotliDisabled` | Use only gzip and deflate |
| `WithMinSize` | Do not compress responses smaller than this (bytes) |
| `WithContentTypes` | Only compress these content types (default: text/*, application/json, etc.) |
| `WithExcludePaths` | Paths that are never compressed |

Example with custom settings:

```go
r.Use(compression.New(
    compression.WithGzipLevel(6),
    compression.WithMinSize(1024),        // Skip if response < 1KB
    compression.WithExcludePaths("/metrics"),
))
```

## Examples

A runnable example is in the `example/` directory:

```bash
cd example
go run main.go
```

Then send a request with `Accept-Encoding: gzip` and check the response.

## Learn More

- [Middleware overview](../README.md) – All middleware and recommended order
- [Router README](../../README.md) – Routing and request handling

## License

Apache License 2.0 – see [LICENSE](../../LICENSE) for details.
