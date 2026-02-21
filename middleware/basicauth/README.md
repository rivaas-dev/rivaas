# BasicAuth

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/middleware/basicauth.svg)](https://pkg.go.dev/rivaas.dev/middleware/basicauth)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../../LICENSE)

Protect routes with a username and password. The middleware checks credentials and blocks access when they don't match. Good for admin areas or simple API protection.

> **Full docs:** [Middleware Guide](https://rivaas.dev/docs/guides/router/middleware/) and [Middleware Reference](https://rivaas.dev/docs/reference/packages/router/middleware/).

## Features

- HTTP Basic Authentication (browser shows a login prompt)
- Static user/password list or your own validator (e.g. database)
- Skip specific paths (e.g. health checks)
- Get the logged-in username in your handlers
- Constant-time password comparison to reduce timing attacks

## Installation

```bash
go get rivaas.dev/middleware/basicauth
```

Requires Go 1.25 or later.

## Quick Start

```go
package main

import (
    "net/http"
    "rivaas.dev/router"
    "rivaas.dev/middleware/basicauth"
)

func main() {
    r := router.New()

    r.Use(basicauth.New(
        basicauth.WithUsers(map[string]string{
            "admin": "secret123",
            "user":  "password456",
        }),
        basicauth.WithRealm("Admin Area"),
    ))

    r.GET("/", func(c *router.Context) {
        username := basicauth.Username(c)
        c.JSON(http.StatusOK, map[string]string{
            "message": "Hello, " + username,
        })
    })

    http.ListenAndServe(":8080", r)
}
```

## Configuration

| Option | What it does |
|--------|----------------|
| `WithUsers` | Map of username to password (simple setup) |
| `WithValidator` | Your own function to check username/password (e.g. against a DB) |
| `WithRealm` | Text shown in the browser login box (default: "Restricted") |
| `WithSkipPaths` | Paths that do not require auth (e.g. `/health`) |
| `WithUnauthorizedHandler` | Custom response when auth fails |

Using a custom validator:

```go
r.Use(basicauth.New(
    basicauth.WithValidator(func(username, password string) bool {
        return db.ValidateUser(username, password)
    }),
    basicauth.WithRealm("My API"),
))
```

## Getting the username in handlers

After a successful login, the username is stored in the context:

```go
username := basicauth.Username(c)
if username == "" {
    c.JSON(http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
    return
}
```

## Security note

Basic Auth sends credentials with every request (base64-encoded, not encrypted). Use HTTPS in production. For APIs, consider tokens or OAuth as well.

## Examples

A full example with multiple protected areas is in the `example/` directory:

```bash
cd example
go run main.go
```

Then try the endpoints with and without credentials (e.g. `curl -u admin:secret123 http://localhost:8080/admin/dashboard`).

## Learn More

- [Middleware overview](../README.md) – All middleware and recommended order
- [Security middleware](../security/) – Security headers
- [AccessLog middleware](../accesslog/) – Log requests with request IDs

## License

Apache License 2.0 – see [LICENSE](../../LICENSE) for details.
