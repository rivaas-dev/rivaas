# TrailingSlash

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/middleware/trailingslash.svg)](https://pkg.go.dev/rivaas.dev/middleware/trailingslash)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../../LICENSE)

Make URLs consistent by always adding or always removing a trailing slash. Redirects clients to the chosen form so you avoid duplicate content and keep routing predictable.

> **Full docs:** [Middleware Guide](https://rivaas.dev/docs/guides/router/middleware/) and [Middleware Reference](https://rivaas.dev/docs/reference/packages/router/middleware/).

## Features

- **PolicyRemove** – Redirect /users/ to /users (default, good for APIs)
- **PolicyAdd** – Redirect /users to /users/
- **PolicyStrict** – Return 404 for wrong trailing slash (strict APIs)
- Uses 308 Permanent Redirect so the method is preserved
- Root path "/" is never redirected

## Installation

```bash
go get rivaas.dev/middleware/trailingslash
```

Requires Go 1.25 or later.

## Quick Start

```go
package main

import (
    "net/http"
    "rivaas.dev/router"
    "rivaas.dev/middleware/trailingslash"
)

func main() {
    r := router.New()

    // Remove trailing slashes: /users/ -> /users (default)
    r.Use(trailingslash.New())

    r.GET("/users", func(c *router.Context) {
        c.JSON(http.StatusOK, map[string]string{"list": "users"})
    })

    http.ListenAndServe(":8080", r)
}
```

**Important:** Trailing slash handling must run before route matching. Use the **Wrap** function at the top level instead of `Use` when you need correct behavior for all routes:

```go
r := router.New()
// Register routes first, then wrap for trailing slash
r.GET("/users", handler)
http.ListenAndServe(":8080", trailingslash.Wrap(r, trailingslash.WithPolicy(trailingslash.PolicyRemove)))
```

See the package doc and example for when to use `Use` vs `Wrap`.

## Configuration

| Option | What it does |
|--------|----------------|
| `WithPolicy` | PolicyRemove (default), PolicyAdd, or PolicyStrict |

Require trailing slashes (e.g. for a static site):

```go
r.Use(trailingslash.New(trailingslash.WithPolicy(trailingslash.PolicyAdd)))
```

Strict mode (404 when trailing slash does not match):

```go
r.Use(trailingslash.New(trailingslash.WithPolicy(trailingslash.PolicyStrict)))
```

## Examples

A runnable example is in the `example/` directory:

```bash
cd example
go run main.go
```

Try `/users` and `/users/` to see redirects.

## Learn More

- [Middleware overview](../README.md) – All middleware and recommended order
- [MethodOverride middleware](../methodoverride/) – Override HTTP method from POST

## License

Apache License 2.0 – see [LICENSE](../../LICENSE) for details.
