# Validation

Flexible, multi-strategy validation for Go structs with support for struct tags, JSON Schema, and custom interfaces.

## Features

- **Multiple Validation Strategies**
  - Struct tags via [go-playground/validator](https://github.com/go-playground/validator)
  - JSON Schema (RFC-compliant)
  - Custom interfaces (`Validate()` / `ValidateContext()`)
- **Partial Validation** - For PATCH requests where only provided fields should be validated
- **Thread-Safe** - Safe for concurrent use by multiple goroutines
- **Security** - Built-in protections against deep nesting, memory exhaustion, and sensitive data exposure
- **Standalone** - Can be used independently without the full Rivaas framework
- **Built-in Validators** - Includes `username`, `slug`, and `strong_password` validators

## Installation

```bash
go get rivaas.dev/validation
```

## Quick Start

### Basic Validation

The simplest way to use this package is with the package-level `Validate` function:

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

### Custom Validator Instance

For more control, create a `Validator` instance with custom options:

```go
validator := validation.MustNew(
    validation.WithRedactor(sensitiveFieldRedactor),
    validation.WithMaxErrors(10),
    validation.WithCustomTag("phone", phoneValidator),
)

if err := validator.Validate(ctx, &user); err != nil {
    // Handle validation errors
}
```

### Partial Validation (PATCH Requests)

For PATCH requests where only provided fields should be validated:

```go
// Compute which fields are present in the JSON
presence, _ := validation.ComputePresence(rawJSON)

// Validate only the present fields
err := validator.ValidatePartial(ctx, &user, presence)
```

## Validation Strategies

The package supports three validation strategies that can be used individually or combined:

### 1. Struct Tags (go-playground/validator)

Use struct tags with go-playground/validator syntax:

```go
type User struct {
    Email    string `validate:"required,email"`
    Age      int    `validate:"min=18,max=120"`
    Username string `validate:"required,username"` // Built-in custom validator
}
```

**Built-in Custom Validators:**
- `username` - Alphanumeric and underscores, 3-20 characters
- `slug` - Lowercase alphanumeric and hyphens
- `strong_password` - Minimum 8 characters

### 2. JSON Schema

Implement the `JSONSchemaProvider` interface:

```go
type User struct {
    Email string `json:"email"`
    Age   int    `json:"age"`
}

func (u User) JSONSchema() (id, schema string) {
    return "user-schema", `{
        "type": "object",
        "properties": {
            "email": {"type": "string", "format": "email"},
            "age": {"type": "integer", "minimum": 18}
        },
        "required": ["email"]
    }`
}
```

### 3. Custom Validation Interface

Implement `ValidatorInterface` for simple validation:

```go
type User struct {
    Email string
}

func (u *User) Validate() error {
    if !strings.Contains(u.Email, "@") {
        return errors.New("email must contain @")
    }
    return nil
}

// validation.Validate will automatically call u.Validate()
err := validation.Validate(ctx, &user)
```

Or implement `ValidatorWithContext` for context-aware validation:

```go
func (u *User) ValidateContext(ctx context.Context) error {
    // Access request-scoped data from context
    tenant := ctx.Value("tenant").(string)
    // Apply tenant-specific validation rules
    return nil
}
```

## Strategy Selection

The package automatically selects the best strategy based on the type:

**Priority Order:**
1. Interface methods (`Validate()` / `ValidateContext()`)
2. Struct tags (`validate:"..."`)
3. JSON Schema (`JSONSchemaProvider`)

You can explicitly choose a strategy:

```go
err := validator.Validate(ctx, &user, validation.WithStrategy(validation.StrategyTags))
```

Or run all applicable strategies:

```go
err := validator.Validate(ctx, &user, validation.WithRunAll(true))
```

## Configuration Options

### Validator Options (at creation)

```go
validator := validation.MustNew(
    validation.WithMaxErrors(10),              // Limit errors returned
    validation.WithMaxCachedSchemas(2048),     // Schema cache size
    validation.WithRedactor(redactorFunc),     // Redact sensitive fields
    validation.WithCustomTag("phone", phoneValidatorFunc), // Custom tag
)
```

### Per-Call Options

```go
err := validator.Validate(ctx, &user,
    validation.WithStrategy(validation.StrategyTags),
    validation.WithPartial(true),
    validation.WithPresence(presenceMap),
    validation.WithMaxErrors(5),
    validation.WithDisallowUnknownFields(true),
    validation.WithCustomValidator(customFunc),
    validation.WithFieldNameMapper(mapperFunc),
)
```

## Error Handling

Validation errors are returned as a structured `*validation.Error`:

```go
err := validation.Validate(ctx, &user)
if err != nil {
    var verr *validation.Error
    if errors.As(err, &verr) {
        // Access structured field errors
        for _, fieldErr := range verr.Fields {
            fmt.Printf("Field: %s\n", fieldErr.Path)
            fmt.Printf("Code: %s\n", fieldErr.Code)
            fmt.Printf("Message: %s\n", fieldErr.Message)
            fmt.Printf("Value: %v\n", fieldErr.Value) // May be redacted
        }
        
        // Check if errors were truncated
        if verr.Truncated {
            fmt.Println("More errors exist (truncated)")
        }
    }
}
```

## Sensitive Data Redaction

Protect sensitive data in error messages:

```go
redactor := func(path string) bool {
    return strings.Contains(path, "password") || 
           strings.Contains(path, "token") ||
           strings.Contains(path, "secret")
}

validator := validation.MustNew(validation.WithRedactor(redactor))
```

## Thread Safety

`Validator` instances are safe for concurrent use by multiple goroutines. The package-level functions (`Validate`, `ValidatePartial`) use a default validator that is also thread-safe.

## Security Features

The package includes protections against:

- **Stack overflow** - Maximum nesting depth of 100 levels
- **Memory exhaustion** - Configurable limits on errors and fields
- **Sensitive data exposure** - Redaction support via `WithRedactor`
- **Schema cache DoS** - LRU eviction with configurable max size

## Examples

### Custom Tag Validator

```go
phoneRegex := regexp.MustCompile(`^\+?[1-9]\d{1,14}$`)

validator := validation.MustNew(
    validation.WithCustomTag("phone", func(fl validator.FieldLevel) bool {
        return phoneRegex.MatchString(fl.Field().String())
    }),
)

type Contact struct {
    Phone string `validate:"phone"`
}
```

### Field Name Mapping

```go
validator := validation.MustNew(
    validation.WithFieldNameMapper(func(name string) string {
        return strings.ReplaceAll(name, "_", " ")
    }),
)
```

### Custom Validator Function

```go
err := validator.Validate(ctx, &user,
    validation.WithCustomValidator(func(v any) error {
        user := v.(*User)
        if user.Age < 18 {
            return errors.New("must be 18 or older")
        }
        return nil
    }),
)
```

### Combining Strategies

```go
// Run all strategies, succeed if any one passes
err := validator.Validate(ctx, &user,
    validation.WithRunAll(true),
    validation.WithRequireAny(true),
)
```

## Performance Considerations

- **Caching** - JSON schemas are cached with LRU eviction
- **Path caching** - Field paths are cached per type
- **Type caching** - Interface implementation checks are cached
- **Lazy initialization** - Tag validator initialized on first use

## Comparison with Other Libraries

| Feature | rivaas.dev/validation | go-playground/validator | JSON Schema validators |
|---------|----------------------|------------------------|----------------------|
| Struct tags | ✅ | ✅ | ❌ |
| JSON Schema | ✅ | ❌ | ✅ |
| Custom interfaces | ✅ | ❌ | ❌ |
| Partial validation | ✅ | ❌ | ❌ |
| Multi-strategy | ✅ | ❌ | ❌ |
| Context support | ✅ | ❌ | Varies |
| Built-in redaction | ✅ | ❌ | ❌ |
| Thread-safe | ✅ | ✅ | Varies |

## Documentation

For complete API documentation, see:
- [Go package documentation](https://pkg.go.dev/rivaas.dev/validation)
- [Package doc.go](./doc.go) - Comprehensive package overview
- [Examples](./example_test.go) - Runnable examples

## Testing

The package includes comprehensive tests:

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run benchmarks
go test -bench=. -benchmem

# Run fuzz tests
go test -fuzz=FuzzValidate -fuzztime=30s
```

## Contributing

See the main [Rivaas contributing guide](../docs/) for development standards and practices.

## License

Apache License 2.0 - See [LICENSE](../LICENSE) for details.

