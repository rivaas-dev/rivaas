# CORS

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/middleware/cors.svg)](https://pkg.go.dev/rivaas.dev/middleware/cors)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../../LICENSE)

Control which websites can call your API from the browser. CORS (Cross-Origin Resource Sharing) lets you allow or block requests from other origins and set which methods and headers are allowed.

> **Full docs:** [Middleware Guide](https://rivaas.dev/docs/guides/router/middleware/) and [Middleware Reference](https://rivaas.dev/docs/reference/packages/router/middleware/).

## Features

- Allow specific origins (or patterns)
- Set allowed HTTP methods and headers
- Expose response headers to the client
- Support credentials (cookies, auth headers) with strict origin rules
- Handle preflight (OPTIONS) requests and cache them with Max-Age

## Installation

```bash
go get rivaas.dev/middleware/cors
```

Requires Go 1.25 or later.

## Quick Start

```go
package main

import (
    "net/http"
    "rivaas.dev/router"
    "rivaas.dev/middleware/cors"
)

func main() {
    r := router.New()
    r.Use(cors.New(
        cors.WithAllowedOrigins("https://example.com"),
        cors.WithAllowedMethods("GET", "POST", "PUT", "DELETE"),
        cors.WithAllowedHeaders("Content-Type", "Authorization"),
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
| `WithAllowedOrigins` | Origins that can call your API (supports wildcards; no wildcards if using credentials) |
| `WithAllowedMethods` | HTTP methods allowed in cross-origin requests |
| `WithAllowedHeaders` | Request headers the client may send |
| `WithExposedHeaders` | Response headers the client script can read |
| `WithAllowCredentials` | Allow cookies/auth; then you must use exact origins (no *) |
| `WithMaxAge` | How long (seconds) the browser can cache the preflight result |

Allow all origins (use only for development):

```go
r.Use(cors.New(
    cors.WithAllowedOrigins("*"),
    cors.WithAllowedMethods("GET", "POST", "PUT", "DELETE", "OPTIONS"),
))
```

## Security note

When you use credentials (cookies, Authorization), you must list exact origins. The middleware checks this for you.

## Examples

A runnable example is in the `example/` directory:

```bash
cd example
go run main.go
```

## Learn More

- [Middleware overview](../README.md) – All middleware and recommended order
- [Security middleware](../security/) – Security headers (HSTS, CSP, etc.)

## License

Apache License 2.0 – see [LICENSE](../../LICENSE) for details.
