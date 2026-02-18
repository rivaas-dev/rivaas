# MethodOverride

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/router/middleware/methodoverride.svg)](https://pkg.go.dev/rivaas.dev/router/middleware/methodoverride)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../../LICENSE)

Let clients send PUT, PATCH, or DELETE using a POST request plus a header or form field. Useful when the client cannot use real HTTP methods (for example HTML forms, which only support GET and POST).

> **Full docs:** [Middleware Guide](https://rivaas.dev/docs/guides/router/middleware/) and [Middleware Reference](https://rivaas.dev/docs/reference/packages/router/middleware/).

## Features

- Override method via `X-HTTP-Method-Override` header (default)
- Override via form field (e.g. `_method`) for POST form submissions
- Choose which methods can be overridden (default: PUT, PATCH, DELETE)
- Optional: only allow override on POST requests
- Optional: require CSRF token when using form-based override

## Installation

```bash
go get rivaas.dev/router/middleware/methodoverride
```

Requires Go 1.25 or later.

## Quick Start

```go
package main

import (
    "net/http"
    "rivaas.dev/router"
    "rivaas.dev/router/middleware/methodoverride"
)

func main() {
    r := router.New()
    r.Use(methodoverride.New())

    r.DELETE("/users/:id", func(c *router.Context) {
        id := c.Param("id")
        c.JSON(http.StatusOK, map[string]string{"deleted": id})
    })

    http.ListenAndServe(":8080", r)
}
```

Clients can send a POST with the real method in a header:

```bash
curl -X POST http://localhost:8080/users/123 \
  -H "X-HTTP-Method-Override: DELETE"
```

## Configuration

| Option | What it does |
|--------|----------------|
| `WithHeader` | Header name for method override (default: X-HTTP-Method-Override) |
| `WithQueryParam` | Query or form field name (default: _method); set empty to disable |
| `WithAllow` | Methods that can be set via override (default: PUT, PATCH, DELETE) |
| `WithOnlyOn` | Only treat override when the request method is one of these (default: POST) |
| `WithRequireCSRFToken` | When true, form-based override only if request is considered CSRF-verified |

Example with custom header and form field:

```go
r.Use(methodoverride.New(
    methodoverride.WithHeader("X-Method-Override"),
    methodoverride.WithQueryParam("_method"),
))
```

## Example in HTML forms

```html
<form method="POST" action="/users/123">
    <input type="hidden" name="_method" value="DELETE">
    <button type="submit">Delete user</button>
</form>
```

## Security note

Use method override only when you need it (e.g. form limitations). For form-based override, consider CSRF protection; the middleware can require CSRF verification via `WithRequireCSRFToken`.

## Examples

A runnable example is in the `example/` directory:

```bash
cd example
go run main.go
```

## Learn More

- [Middleware overview](../README.md) – All middleware and recommended order
- [TrailingSlash middleware](../trailingslash/) – Normalize trailing slashes in URLs

## License

Apache License 2.0 – see [LICENSE](../../LICENSE) for details.
