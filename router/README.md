# Router

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/router.svg)](https://pkg.go.dev/rivaas.dev/router)
[![Go Report Card](https://goreportcard.com/badge/rivaas.dev/router)](https://goreportcard.com/report/rivaas.dev/router)
[![Coverage](https://codecov.io/gh/rivaas-dev/rivaas/branch/main/graph/badge.svg?component=module_router)](https://codecov.io/gh/rivaas-dev/rivaas)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../LICENSE)

An HTTP router for Go, built for cloud-native apps. It gives you routing, middleware, and observability in one place.

> **📚 Full docs:** For guides, examples, and API details, see the [Router Documentation](https://rivaas.dev/docs/guides/router/).

## Documentation

- **[Installation](https://rivaas.dev/docs/guides/router/installation/)** – Get the router and run your first route
- **[User Guide](https://rivaas.dev/docs/guides/router/)** – From basics to advanced use
- **[API Reference](https://rivaas.dev/docs/reference/packages/router/)** – Full API docs
- **[Examples](https://rivaas.dev/docs/guides/router/examples/)** – Working examples you can copy
- **[Troubleshooting](https://rivaas.dev/docs/reference/packages/router/troubleshooting/)** – Fix common issues

## Features

- **Fast** – 8.4M+ req/s, 119ns per route, 16B per request
- **Radix tree routing** – Compiled routes and bloom filters for quick lookups
- **Request binding** – Parses requests into structs (15+ types)
- **Validation** – Tags, interfaces, or JSON Schema
- **Content negotiation** – Handles Accept headers the standard way
- **API versioning** – Version via headers or query
- **OpenTelemetry** – Built-in tracing; no cost when you turn it off
- **Middleware** – 12 middlewares ready for production
- **Memory safe** – Context pooling with clear rules
- **Safe for concurrency** – Use it from multiple goroutines

## Installation

```bash
go get rivaas.dev/router
```

You need Go 1.25 or later.

## Quick Start

```go
package main

import (
    "net/http"
    "rivaas.dev/router"
)

func main() {
    r := router.New()
    
    // Simple route
    r.GET("/", func(c *router.Context) {
        c.JSON(http.StatusOK, map[string]string{
            "message": "Hello Rivaas!",
        })
    })
    
    // Parameter route
    r.GET("/users/:id", func(c *router.Context) {
        userID := c.Param("id")
        c.JSON(http.StatusOK, map[string]string{
            "user_id": userID,
        })
    })
    
    http.ListenAndServe(":8080", r)
}
```

## Learn More

- **[Getting Started](https://rivaas.dev/docs/guides/router/basic-usage/)** – Your first router
- **[Route Patterns](https://rivaas.dev/docs/guides/router/route-patterns/)** – Static, params, wildcards
- **[Middleware](https://rivaas.dev/docs/guides/router/middleware/)** – Built-in and custom
- **[Request Binding](https://rivaas.dev/docs/guides/router/request-binding/)** – Parse requests into structs
- **[Validation](https://rivaas.dev/docs/guides/router/validation/)** – Tags, interfaces, JSON Schema
- **[Context API](https://rivaas.dev/docs/guides/router/context/)** – Request and response
- **[Observability](https://rivaas.dev/docs/guides/router/observability/)** – OpenTelemetry tracing
- **[Testing](https://rivaas.dev/docs/guides/router/testing/)** – How to test routes
- **[Migration](https://rivaas.dev/docs/guides/router/migration/)** – From Gin, Echo, http.ServeMux

## Examples

We ship [step-by-step examples](examples/) from simple to advanced:

1. [Hello World](examples/01-hello-world/) – Basic setup
2. [Routing](examples/02-routing/) – Routes, params, groups
3. [Middleware](examples/03-middleware/) – Auth, logging, CORS
4. [REST API](examples/04-rest-api/) – Full CRUD
5. [Advanced](examples/05-advanced/) – Constraints, static files
6. [Advanced Routing](examples/06-advanced-routing/) – Versioning, wildcards
7. [Content Negotiation](examples/07-content-negotiation/) – Accept headers
8. [Request Binding](examples/08-binding/) – Parse into structs

## Benchmarks

Benchmarks live in [benchmarks/](benchmarks/). They compare this router with other Go frameworks (Gin, Echo, Chi, Fiber, Hertz, Beego, std lib). When you push a tag like `router/v0.9.2`, CI runs the benchmarks and updates the results on the docs and the website. For how we run them and how to reproduce, see [Router Performance](https://rivaas.dev/docs/reference/packages/router/performance/).

## Contributing

Contributions are welcome. See the [main repository](../) for how to contribute.

## License

Apache License 2.0. See [LICENSE](../LICENSE) for details.

---

Part of the [Rivaas](https://github.com/rivaas-dev/rivaas) web framework.
