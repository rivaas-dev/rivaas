# Security

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/router/middleware/security.svg)](https://pkg.go.dev/rivaas.dev/router/middleware/security)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../../LICENSE)

Add security-related HTTP headers to protect your app from common issues like clickjacking, MIME sniffing, and XSS. One middleware sets several headers that follow common best practices.

> **Full docs:** [Middleware Guide](https://rivaas.dev/docs/guides/router/middleware/) and [Middleware Reference](https://rivaas.dev/docs/reference/packages/router/middleware/).

## Features

- X-Content-Type-Options: nosniff (stops MIME sniffing)
- X-Frame-Options (reduces clickjacking risk)
- X-XSS-Protection (legacy browsers)
- Referrer-Policy and Permissions-Policy
- Optional HSTS (force HTTPS)
- Optional Content-Security-Policy (CSP)
- Configurable per header

## Installation

```bash
go get rivaas.dev/router/middleware/security
```

Requires Go 1.25 or later.

## Quick Start

```go
package main

import (
    "net/http"
    "rivaas.dev/router"
    "rivaas.dev/router/middleware/security"
)

func main() {
    r := router.New()
    r.Use(security.New())

    r.GET("/", func(c *router.Context) {
        c.String(http.StatusOK, "Hello")
    })

    http.ListenAndServe(":8080", r)
}
```

## Configuration

| Option | What it does |
|--------|----------------|
| `WithFrameOptions` | X-Frame-Options (e.g. DENY, SAMEORIGIN) |
| `WithContentTypeNosniff` | X-Content-Type-Options: nosniff (default: true) |
| `WithXSSProtection` | X-XSS-Protection value |
| `WithReferrerPolicy` | Referrer-Policy value |
| `WithPermissionsPolicy` | Permissions-Policy value |
| `WithHSTS` | HTTP Strict Transport Security (maxAge, includeSubdomains, preload) |
| `WithContentSecurityPolicy` | Content-Security-Policy value |

Example with HSTS and CSP:

```go
r.Use(security.New(
    security.WithHSTS(31536000, true, true), // 1 year, subdomains, preload
    security.WithContentSecurityPolicy("default-src 'self'; script-src 'self'"),
))
```

## Security note

Use HTTPS in production and set HSTS when you are sure all traffic should be HTTPS.

## Examples

A runnable example is in the `example/` directory:

```bash
cd example
go run main.go
```

## Learn More

- [Middleware overview](../README.md) – All middleware and recommended order
- [CORS middleware](../cors/) – Cross-origin request handling
- [BasicAuth middleware](../basicauth/) – Username/password protection

## License

Apache License 2.0 – see [LICENSE](../../LICENSE) for details.
