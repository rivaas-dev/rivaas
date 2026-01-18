# Router

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/router.svg)](https://pkg.go.dev/rivaas.dev/router)
[![Go Report Card](https://goreportcard.com/badge/rivaas.dev/router)](https://goreportcard.com/report/rivaas.dev/router)
[![Coverage](https://codecov.io/gh/rivaas-dev/rivaas/branch/main/graph/badge.svg?component=module_router)](https://codecov.io/gh/rivaas-dev/rivaas)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../LICENSE)

An HTTP router for Go, designed for cloud-native applications with comprehensive routing, middleware, and observability features.

> **📚 Full Documentation:** For comprehensive documentation, guides, and examples, see the [Router Documentation](https://rivaas.dev/docs/guides/router/).

## Documentation

- **[Installation](https://rivaas.dev/docs/guides/router/installation/)** - Get started with the router
- **[User Guide](https://rivaas.dev/docs/guides/router/)** - Complete learning path from basics to advanced features
- **[API Reference](https://rivaas.dev/docs/reference/packages/router/)** - Detailed API documentation
- **[Examples](https://rivaas.dev/docs/guides/router/examples/)** - Complete working examples
- **[Troubleshooting](https://rivaas.dev/docs/reference/packages/router/troubleshooting/)** - Common issues and solutions

## Features

- **High Performance**: 8.4M+ req/s throughput, 119ns/op routing, 16B/req memory
- **Radix Tree Routing**: Compiled routes with bloom filters for static lookups
- **Request Binding**: Automatic parsing to structs with 15+ type categories
- **Comprehensive Validation**: Multiple strategies (tags, interface, JSON Schema)
- **Content Negotiation**: RFC 7231 compliant Accept header handling
- **API Versioning**: Built-in header/query-based versioning
- **OpenTelemetry Native**: Zero-overhead tracing when disabled
- **Built-in Middleware**: 12 production-ready middlewares
- **Memory Safe**: Context pooling with clear lifecycle rules
- **Thread Safe**: Concurrent-safe operations

## Installation

```bash
go get rivaas.dev/router
```

Requires Go 1.25+

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

- **[Getting Started](https://rivaas.dev/docs/guides/router/basic-usage/)** - Your first router
- **[Route Patterns](https://rivaas.dev/docs/guides/router/route-patterns/)** - Static, parameter, and wildcard routes
- **[Middleware](https://rivaas.dev/docs/guides/router/middleware/)** - Built-in and custom middleware
- **[Request Binding](https://rivaas.dev/docs/guides/router/request-binding/)** - Automatic request parsing
- **[Validation](https://rivaas.dev/docs/guides/router/validation/)** - Multiple validation strategies
- **[Context API](https://rivaas.dev/docs/guides/router/context/)** - Request/response handling
- **[Observability](https://rivaas.dev/docs/guides/router/observability/)** - OpenTelemetry tracing
- **[Testing](https://rivaas.dev/docs/guides/router/testing/)** - Test your routes
- **[Migration](https://rivaas.dev/docs/guides/router/migration/)** - From Gin, Echo, http.ServeMux

## Examples

The router includes [progressive examples](examples/) from basic to advanced usage:

1. [Hello World](examples/01-hello-world/) - Basic router setup
2. [Routing](examples/02-routing/) - Routes, parameters, groups
3. [Middleware](examples/03-middleware/) - Auth, logging, CORS
4. [REST API](examples/04-rest-api/) - Full CRUD implementation
5. [Advanced](examples/05-advanced/) - Constraints, static files
6. [Advanced Routing](examples/06-advanced-routing/) - Versioning, wildcards
7. [Content Negotiation](examples/07-content-negotiation/) - Accept headers
8. [Request Binding](examples/08-binding/) - Automatic parsing

## Contributing

Contributions are welcome! Please see the [main repository](../) for contribution guidelines.

## License

Apache License 2.0 - see [LICENSE](../LICENSE) for details.

---

Part of the [Rivaas](https://github.com/rivaas-dev/rivaas) web framework ecosystem.
