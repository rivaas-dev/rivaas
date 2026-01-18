# Validation

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/validation.svg)](https://pkg.go.dev/rivaas.dev/validation)
[![Go Report Card](https://goreportcard.com/badge/rivaas.dev/validation)](https://goreportcard.com/report/rivaas.dev/validation)
[![Coverage](https://codecov.io/gh/rivaas-dev/rivaas/branch/main/graph/badge.svg?component=module_validation)](https://codecov.io/gh/rivaas-dev/rivaas)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../LICENSE)

Flexible, multi-strategy validation for Go structs with support for struct tags, JSON Schema, and custom interfaces.

> **📚 Full Documentation**: https://rivaas.dev/docs/guides/validation/

## Documentation

- **[Installation](https://rivaas.dev/docs/guides/validation/installation/)** - Get started
- **[User Guide](https://rivaas.dev/docs/guides/validation/)** - Learn validation strategies
- **[API Reference](https://rivaas.dev/docs/reference/packages/validation/)** - Complete API docs
- **[Examples](https://rivaas.dev/docs/guides/validation/examples/)** - Real-world patterns
- **[Troubleshooting](https://rivaas.dev/docs/reference/packages/validation/troubleshooting/)** - Common issues

## Features

- **Multiple Validation Strategies** - Struct tags, JSON Schema, custom interfaces
- **Partial Validation** - PATCH request support with presence tracking
- **Thread-Safe** - Safe for concurrent use
- **Security** - Built-in redaction, nesting limits, memory protection
- **Structured Errors** - Field-level errors with codes and metadata
- **Extensible** - Custom tags, validators, and error messages

## Installation

```bash
go get rivaas.dev/validation
```

Requires Go 1.25+

## Quick Start

```go
import "rivaas.dev/validation"

type User struct {
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"min=18"`
}

user := User{Email: "invalid", Age: 15}
if err := validation.Validate(ctx, &user); err != nil {
    var verr *validation.Error
    if errors.As(err, &verr) {
        for _, fieldErr := range verr.Fields {
            fmt.Printf("%s: %s\n", fieldErr.Path, fieldErr.Message)
        }
    }
}
```

## Validation Strategies

### 1. Struct Tags

```go
type User struct {
    Email string `validate:"required,email"`
    Age   int    `validate:"min=18,max=120"`
}
```

### 2. JSON Schema

```go
func (u User) JSONSchema() (id, schema string) {
    return "user-v1", `{
        "type": "object",
        "properties": {
            "email": {"type": "string", "format": "email"},
            "age": {"type": "integer", "minimum": 18}
        },
        "required": ["email"]
    }`
}
```

### 3. Custom Interfaces

```go
func (u *User) Validate() error {
    if !strings.Contains(u.Email, "@") {
        return errors.New("email must contain @")
    }
    return nil
}
```

## Learn More

- [Basic Usage](https://rivaas.dev/docs/guides/validation/basic-usage/) - Fundamentals
- [Struct Tags](https://rivaas.dev/docs/guides/validation/struct-tags/) - go-playground/validator syntax
- [JSON Schema](https://rivaas.dev/docs/guides/validation/json-schema/) - Schema validation
- [Custom Interfaces](https://rivaas.dev/docs/guides/validation/custom-interfaces/) - Custom methods
- [Partial Validation](https://rivaas.dev/docs/guides/validation/partial-validation/) - PATCH requests
- [Error Handling](https://rivaas.dev/docs/guides/validation/error-handling/) - Structured errors
- [Security](https://rivaas.dev/docs/guides/validation/security/) - Redaction and limits

## Contributing

Contributions are welcome! Please see the [main repository](../) for contribution guidelines.

## License

Apache License 2.0 - see [LICENSE](../LICENSE) for details.

---

Part of the [Rivaas](https://github.com/rivaas-dev/rivaas) web framework ecosystem.
