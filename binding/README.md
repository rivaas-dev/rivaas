# Binding

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/binding.svg)](https://pkg.go.dev/rivaas.dev/binding)
[![Go Report Card](https://goreportcard.com/badge/rivaas.dev/binding)](https://goreportcard.com/report/rivaas.dev/binding)
[![Coverage](https://codecov.io/gh/rivaas-dev/rivaas/branch/main/graph/badge.svg?component=module_binding)](https://codecov.io/gh/rivaas-dev/rivaas)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../LICENSE)

High-performance request data binding for Go web applications. Maps values from various sources (query parameters, form data, JSON bodies, headers, cookies, path parameters) into Go structs using struct tags.

> **📚 [Complete Documentation →](https://rivaas.dev/docs/guides/binding/)**

## Documentation

This README provides a quick overview. For comprehensive guides, tutorials, and API reference:

- **[Installation Guide](https://rivaas.dev/docs/guides/binding/installation/)** - Get started
- **[User Guide](https://rivaas.dev/docs/guides/binding/)** - Learn the features
- **[API Reference](https://rivaas.dev/docs/reference/packages/binding/)** - Complete API docs
- **[Examples](https://rivaas.dev/docs/guides/binding/examples/)** - Real-world patterns
- **[Troubleshooting](https://rivaas.dev/docs/reference/packages/binding/troubleshooting/)** - FAQs and solutions

## Features

- **Multiple Sources** - Query, path, form, header, cookie, JSON, XML, YAML, TOML, MessagePack, Protocol Buffers
- **Type Safe** - Generic API for compile-time type safety
- **Zero Allocation** - Struct reflection info cached for performance
- **Flexible** - Nested structs, slices, maps, pointers, custom types
- **Error Context** - Detailed field-level error information with contextual hints
- **Converter Factories** - Built-in factories for common patterns (time, duration, enum, bool)
- **Extensible** - Custom type converters and value getters

> **Note:** For validation (required fields, enum constraints, etc.), use the `rivaas.dev/validation` package separately after binding.

## Installation

```bash
go get rivaas.dev/binding
```

Requires Go 1.25+

## Quick Start

### JSON Binding

```go
import "rivaas.dev/binding"

type CreateUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
    Age   int    `json:"age"`
}

user, err := binding.JSON[CreateUserRequest](body)
if err != nil {
    // Handle error
}
```

### Query Parameters

```go
type ListParams struct {
    Page   int      `query:"page" default:"1"`
    Limit  int      `query:"limit" default:"20"`
    Tags   []string `query:"tags"`
}

params, err := binding.Query[ListParams](r.URL.Query())
```

### Multi-Source Binding

```go
type CreateOrderRequest struct {
    UserID int    `path:"user_id"`
    Coupon string `query:"coupon"`
    Auth   string `header:"Authorization"`
    Items  []OrderItem `json:"items"`
}

req, err := binding.Bind[CreateOrderRequest](
    binding.FromPath(pathParams),
    binding.FromQuery(r.URL.Query()),
    binding.FromHeader(r.Header),
    binding.FromJSON(body),
)
```

### Custom Type Converters

```go
import "github.com/google/uuid"

type Request struct {
    ID       uuid.UUID `query:"id"`
    Status   Status    `query:"status"`
    Deadline time.Time `query:"deadline"`
}

binder := binding.MustNew(
    binding.WithConverter[uuid.UUID](uuid.Parse),
    binding.WithConverter(binding.TimeConverter("01/02/2006")),
    binding.WithConverter(binding.EnumConverter("active", "pending", "disabled")),
)

req, err := binder.Query[Request](r.URL.Query())
```

**[See more examples →](https://rivaas.dev/docs/guides/binding/examples/)**

## Learn More

- **[Basic Usage](https://rivaas.dev/docs/guides/binding/basic-usage/)** - Request binding fundamentals
- **[Query Parameters](https://rivaas.dev/docs/guides/binding/query-parameters/)** - URL query binding
- **[JSON Binding](https://rivaas.dev/docs/guides/binding/json-binding/)** - Request body binding
- **[Multi-Source](https://rivaas.dev/docs/guides/binding/multi-source/)** - Combine multiple sources
- **[Struct Tags](https://rivaas.dev/docs/guides/binding/struct-tags/)** - Tag syntax reference
- **[Type Support](https://rivaas.dev/docs/guides/binding/type-support/)** - All supported types
- **[Error Handling](https://rivaas.dev/docs/guides/binding/error-handling/)** - Error patterns
- **[Advanced Usage](https://rivaas.dev/docs/guides/binding/advanced-usage/)** - Custom converters and binders

## Contributing

Contributions are welcome! Please see the [main repository](../) for contribution guidelines.

## License

Apache License 2.0 - see [LICENSE](../LICENSE) for details.

---

Part of the [Rivaas](https://github.com/rivaas-dev/rivaas) web framework ecosystem.
