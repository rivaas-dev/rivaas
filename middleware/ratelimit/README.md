# RateLimit

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/router/middleware/ratelimit.svg)](https://pkg.go.dev/rivaas.dev/router/middleware/ratelimit)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../../LICENSE)

Limit how many requests each client can make. Uses a token bucket so you get a steady rate plus short bursts. Good for protecting your API from abuse and keeping usage fair.

> **Full docs:** [Middleware Guide](https://rivaas.dev/docs/guides/router/middleware/) and [Middleware Reference](https://rivaas.dev/docs/reference/packages/router/middleware/).

## Features

- Token bucket algorithm (smooth rate with configurable burst)
- Limit per client IP by default, or per user / custom key
- Skip specific paths (e.g. health checks)
- Standard rate limit headers: X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset
- Custom handler when the limit is exceeded
- Safe for concurrent use

## Installation

```bash
go get rivaas.dev/router/middleware/ratelimit
```

Requires Go 1.25 or later.

## Quick Start

```go
package main

import (
    "net/http"
    "rivaas.dev/router"
    "rivaas.dev/router/middleware/ratelimit"
)

func main() {
    r := router.New()
    r.Use(ratelimit.New(
        ratelimit.WithRequestsPerSecond(100),
        ratelimit.WithBurst(20),
    ))

    r.GET("/api/data", func(c *router.Context) {
        c.JSON(http.StatusOK, map[string]string{"data": "value"})
    })

    http.ListenAndServe(":8080", r)
}
```

## Configuration

| Option | What it does |
|--------|----------------|
| `WithRequestsPerSecond` | Average rate (tokens per second) |
| `WithBurst` | Max burst size (default: same as rate) |
| `WithKeyFunc` | How to identify the client (default: by IP) |
| `WithSkipPaths` | Paths that are not rate limited |
| `WithOnLimitExceeded` | Custom response when limit is hit |
| `WithLogger` | Logger for rate limit events |

Limit per user instead of per IP:

```go
r.Use(ratelimit.New(
    ratelimit.WithRequestsPerSecond(50),
    ratelimit.WithBurst(10),
    ratelimit.WithKeyFunc(func(c *router.Context) string {
        return basicauth.Username(c) // or from JWT, session, etc.
    }),
))
```

## Response headers

When a request is allowed, the middleware adds:

- **X-RateLimit-Limit** – Max requests in the window
- **X-RateLimit-Remaining** – Requests left in the current window
- **X-RateLimit-Reset** – When the window resets (Unix timestamp)

## Examples

A runnable example is in the `example/` directory:

```bash
cd example
go run main.go
```

Then send many requests and watch the rate limit kick in.

## Learn More

- [Middleware overview](../README.md) – All middleware and recommended order
- [BodyLimit middleware](../bodylimit/) – Limit request body size
- [Timeout middleware](../timeout/) – Limit request duration

## License

Apache License 2.0 – see [LICENSE](../../LICENSE) for details.
