# OpenAPI

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/openapi.svg)](https://pkg.go.dev/rivaas.dev/openapi)
[![Go Report Card](https://goreportcard.com/badge/rivaas.dev/openapi)](https://goreportcard.com/report/rivaas.dev/openapi)
[![Coverage](https://codecov.io/gh/rivaas-dev/rivaas/branch/main/graph/badge.svg?component=module_openapi)](https://codecov.io/gh/rivaas-dev/rivaas)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../LICENSE)

Automatic OpenAPI 3.0.4 and 3.1.2 specification generation for Go applications.

> **📚 [Complete Documentation →](https://rivaas.dev/docs/guides/openapi/)**

## Documentation

This README provides a quick overview. For comprehensive guides, tutorials, and API reference:

- **[Installation Guide](https://rivaas.dev/docs/guides/openapi/installation/)** - Get started
- **[User Guide](https://rivaas.dev/docs/guides/openapi/)** - Learn the features
- **[API Reference](https://rivaas.dev/docs/reference/packages/openapi/)** - Complete API docs
- **[Examples](https://rivaas.dev/docs/guides/openapi/examples/)** - Real-world patterns
- **[Troubleshooting](https://rivaas.dev/docs/reference/packages/openapi/troubleshooting/)** - FAQs and solutions

## Features

- **Clean API** - Builder-style `API.Generate()` method
- **Type-Safe Version Selection** - `V30x` and `V31x` constants
- **Fluent HTTP Method Constructors** - `GET()`, `POST()`, `PUT()`, etc.
- **Automatic Parameter Discovery** - Extracts parameters from struct tags
- **Schema Generation** - Converts Go types to OpenAPI schemas
- **Swagger UI Configuration** - Built-in, customizable UI
- **Type-Safe Diagnostics** - `diag` package for warning control
- **Built-in Validation** - Validates against official meta-schemas

## Installation

```bash
go get rivaas.dev/openapi
```

Requires Go 1.25+

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "rivaas.dev/openapi"
)

type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

type CreateUserRequest struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

func main() {
    api := openapi.MustNew(
        openapi.WithTitle("My API", "1.0.0"),
        openapi.WithInfoDescription("API for managing users"),
        openapi.WithServer("http://localhost:8080", "Local development"),
        openapi.WithBearerAuth("bearerAuth", "JWT authentication"),
    )

    result, err := api.Generate(context.Background(),
        openapi.GET("/users/:id",
            openapi.WithSummary("Get user"),
            openapi.WithResponse(200, User{}),
            openapi.WithSecurity("bearerAuth"),
        ),
        openapi.POST("/users",
            openapi.WithSummary("Create user"),
            openapi.WithRequest(CreateUserRequest{}),
            openapi.WithResponse(201, User{}),
        ),
        openapi.DELETE("/users/:id",
            openapi.WithSummary("Delete user"),
            openapi.WithResponse(204, nil),
            openapi.WithSecurity("bearerAuth"),
        ),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Check for warnings (optional)
    if len(result.Warnings) > 0 {
        fmt.Printf("Generated with %d warnings\n", len(result.Warnings))
    }

    fmt.Println(string(result.JSON))
}
```

**[See more examples →](https://rivaas.dev/docs/guides/openapi/examples/)**

## Learn More

- **[Basic Usage](https://rivaas.dev/docs/guides/openapi/basic-usage/)** - Generate your first spec
- **[Configuration](https://rivaas.dev/docs/guides/openapi/configuration/)** - API settings and version selection
- **[Security](https://rivaas.dev/docs/guides/openapi/security/)** - Authentication schemes
- **[Operations](https://rivaas.dev/docs/guides/openapi/operations/)** - Define HTTP endpoints
- **[Auto-Discovery](https://rivaas.dev/docs/guides/openapi/auto-discovery/)** - Struct tag reference
- **[Swagger UI](https://rivaas.dev/docs/guides/openapi/swagger-ui/)** - Customize the UI

## Contributing

Contributions are welcome! Please see the [main repository](../) for contribution guidelines.

## License

Apache License 2.0 - see [LICENSE](../LICENSE) for details.

---

Part of the [Rivaas](https://github.com/rivaas-dev/rivaas) web framework ecosystem.
