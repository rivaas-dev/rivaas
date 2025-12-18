# Binding

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/binding.svg)](https://pkg.go.dev/rivaas.dev/binding)
[![Go Report Card](https://goreportcard.com/badge/rivaas.dev/binding)](https://goreportcard.com/report/rivaas.dev/binding)

High-performance request data binding for Go web applications. Maps values from various sources (query parameters, form data, JSON bodies, headers, cookies, path parameters) into Go structs using struct tags.

## Features

- **Multiple Sources** - Query, path, form, header, cookie, JSON, XML, YAML, TOML, MessagePack, Protocol Buffers
- **Type Safe** - Generic API for compile-time type safety
- **Zero Allocation** - Struct reflection info cached for performance
- **Flexible** - Nested structs, slices, maps, pointers, custom types
- **Validation** - Built-in enum validation, required fields, custom validators
- **Error Context** - Detailed field-level error information
- **Extensible** - Custom type converters and value getters

## Installation

```bash
go get rivaas.dev/binding
```

Requires Go 1.25.0 or higher.

## Quick Start

### JSON Binding

```go
import "rivaas.dev/binding"

type CreateUserRequest struct {
    Name  string `json:"name" required:"true"`
    Email string `json:"email" required:"true"`
    Age   int    `json:"age"`
}

// Generic API (preferred)
user, err := binding.JSON[CreateUserRequest](body)
if err != nil {
    // Handle error
}

// Non-generic API (when type comes from variable)
var user CreateUserRequest
err := binding.JSONTo(body, &user)
```

### Query Parameters

```go
type ListParams struct {
    Page   int      `query:"page" default:"1"`
    Limit  int      `query:"limit" default:"20"`
    Tags   []string `query:"tags"`
    SortBy string   `query:"sort_by" enum:"name,date,popularity"`
}

params, err := binding.Query[ListParams](r.URL.Query())
```

### Multi-Source Binding

Combine data from multiple sources:

```go
type CreateOrderRequest struct {
    // From path parameters
    UserID int `path:"user_id"`
    
    // From query string
    Coupon string `query:"coupon"`
    
    // From headers
    Auth string `header:"Authorization"`
    
    // From JSON body
    Items []OrderItem `json:"items"`
    Total float64     `json:"total"`
}

req, err := binding.Bind[CreateOrderRequest](
    binding.FromPath(pathParams),
    binding.FromQuery(r.URL.Query()),
    binding.FromHeader(r.Header),
    binding.FromJSON(body),
)
```

### Reusable Binder

Create a configured binder for shared settings:

```go
import "github.com/google/uuid"

binder := binding.MustNew(
    binding.WithConverter[uuid.UUID](uuid.Parse),
    binding.WithTimeLayouts("2006-01-02", "01/02/2006"),
    binding.WithRequired(),
    binding.WithMaxDepth(16),
)

// Use across handlers
user, err := binder.JSON[CreateUserRequest](body)
params, err := binder.Query[ListParams](r.URL.Query())
```

## Supported Sources

| Source | Function | Description |
|--------|----------|-------------|
| Query | `Query[T]()` | URL query parameters (`?name=value`) |
| Path | `Path[T]()` | URL path parameters (`/users/:id`) |
| Form | `Form[T]()` | Form data (`application/x-www-form-urlencoded`) |
| Header | `Header[T]()` | HTTP headers |
| Cookie | `Cookie[T]()` | HTTP cookies |
| JSON | `JSON[T]()` | JSON body |
| XML | `XML[T]()` | XML body |
| YAML | `yaml.YAML[T]()` | YAML body (sub-package) |
| TOML | `toml.TOML[T]()` | TOML body (sub-package) |
| MessagePack | `msgpack.MsgPack[T]()` | MessagePack body (sub-package) |
| Protocol Buffers | `proto.Proto[T]()` | Protobuf body (sub-package) |

## Struct Tags

### Basic Tags

```go
type Request struct {
    // Query parameters
    Page  int    `query:"page"`
    Limit int    `query:"limit"`
    
    // Path parameters
    UserID string `path:"user_id"`
    
    // Headers
    Auth string `header:"Authorization"`
    
    // Cookies
    SessionID string `cookie:"session_id"`
    
    // JSON body
    Name  string `json:"name"`
    Email string `json:"email"`
}
```

### Special Tags

```go
type Config struct {
    // Default value when field is not present
    Port int `query:"port" default:"8080"`
    
    // Enum validation
    Status string `query:"status" enum:"active,pending,disabled"`
    
    // Required field (when WithRequired() is used)
    APIKey string `header:"X-API-Key" required:"true"`
    
    // Tag aliases for multiple lookup names
    UserID int `query:"user_id,id"`
}
```

## Type Support

### Built-in Types

- **Primitives**: `string`, `int`, `int8`, `int16`, `int32`, `int64`, `uint`, `uint8`, `uint16`, `uint32`, `uint64`, `float32`, `float64`, `bool`
- **Time**: `time.Time`, `time.Duration`
- **Network**: `net.IP`, `net.IPNet`, `url.URL`
- **Regex**: `regexp.Regexp`
- **Collections**: `[]T` (slices), `map[string]T` (maps)
- **Pointers**: `*T` for any supported type
- **Nested**: Nested structs with dot notation

### Custom Types

Register custom type converters:

```go
import "github.com/shopspring/decimal"

binder := binding.MustNew(
    binding.WithConverter[uuid.UUID](uuid.Parse),
    binding.WithConverter[decimal.Decimal](decimal.NewFromString),
)
```

### TextUnmarshaler

Any type implementing `encoding.TextUnmarshaler` is automatically supported:

```go
type CustomID struct {
    value string
}

func (c *CustomID) UnmarshalText(text []byte) error {
    c.value = string(text)
    return nil
}

type Request struct {
    ID CustomID `query:"id"`
}
```

## Configuration Options

### Security Limits

```go
user, err := binding.JSON[User](body,
    binding.WithMaxDepth(16),        // Max struct nesting (default: 32)
    binding.WithMaxSliceLen(1000),   // Max slice elements (default: 10,000)
    binding.WithMaxMapSize(500),     // Max map entries (default: 1,000)
)
```

### Unknown Fields

```go
user, err := binding.JSON[User](body,
    binding.WithUnknownFields(binding.UnknownError), // Fail on unknown fields
    // Or: binding.UnknownWarn  - Log warnings
    // Or: binding.UnknownIgnore - Ignore (default)
)
```

### Required Fields

```go
type User struct {
    Name  string `json:"name" required:"true"`
    Email string `json:"email" required:"true"`
}

user, err := binding.JSON[User](body,
    binding.WithRequired(), // Enforce required:"true" tags
)
```

### Slice Parsing

```go
params, err := binding.Query[Params](values,
    binding.WithSliceMode(binding.SliceCSV), // Parse "tags=go,rust,python"
    // Or: binding.SliceRepeat - Parse "tags=go&tags=rust" (default)
)
```

### Time Formats

```go
binder := binding.MustNew(
    binding.WithTimeLayouts(
        "2006-01-02",           // Date only
        "01/02/2006",           // US format
        time.RFC3339,           // ISO 8601
        "2006-01-02 15:04:05",  // Custom format
    ),
)
```

### Error Collection

```go
user, err := binding.JSON[User](body,
    binding.WithAllErrors(), // Collect all errors instead of failing on first
)

if err != nil {
    var multi *binding.MultiError
    if errors.As(err, &multi) {
        for _, e := range multi.Errors {
            fmt.Printf("Field: %s, Error: %v\n", e.Field, e.Err)
        }
    }
}
```

### Validation Integration

```go
binder := binding.MustNew(
    binding.WithValidator(myValidator),
)

user, err := binder.JSON[User](body)
// Validator is called after successful binding
```

### Observability

```go
binder := binding.MustNew(
    binding.WithEvents(binding.Events{
        FieldBound: func(name, tag string) {
            log.Printf("Bound field %s from %s", name, tag)
        },
        UnknownField: func(name string) {
            log.Printf("Unknown field: %s", name)
        },
        Done: func(stats binding.Stats) {
            log.Printf("Bound %d fields, %d errors", 
                stats.FieldsBound, stats.ErrorCount)
        },
    }),
)
```

## Error Handling

### BindError

Detailed field-level error information:

```go
user, err := binding.JSON[User](body)
if err != nil {
    var bindErr *binding.BindError
    if errors.As(err, &bindErr) {
        fmt.Printf("Field: %s\n", bindErr.Field)
        fmt.Printf("Source: %s\n", bindErr.Source)
        fmt.Printf("Value: %s\n", bindErr.Value)
        fmt.Printf("Type: %s\n", bindErr.Type)
        fmt.Printf("Reason: %s\n", bindErr.Reason)
        
        // Check error type
        if bindErr.IsRequired() {
            // Missing required field
        }
        if bindErr.IsType() {
            // Type conversion failed
        }
        if bindErr.IsEnum() {
            // Invalid enum value
        }
    }
}
```

### UnknownFieldError

Returned when strict JSON decoding encounters unknown fields:

```go
user, err := binding.JSON[User](body,
    binding.WithUnknownFields(binding.UnknownError),
)

if err != nil {
    var unknownErr *binding.UnknownFieldError
    if errors.As(err, &unknownErr) {
        fmt.Printf("Unknown fields: %v\n", unknownErr.Fields)
    }
}
```

### MultiError

Multiple errors collected with `WithAllErrors()`:

```go
user, err := binding.JSON[User](body, binding.WithAllErrors())
if err != nil {
    var multi *binding.MultiError
    if errors.As(err, &multi) {
        for _, e := range multi.Errors {
            // Handle each error
        }
    }
}
```

## Advanced Usage

### Custom ValueGetter

Implement custom binding sources:

```go
type CustomGetter struct {
    data map[string]string
}

func (g *CustomGetter) Get(key string) string {
    return g.data[key]
}

func (g *CustomGetter) GetAll(key string) []string {
    if val, ok := g.data[key]; ok {
        return []string{val}
    }
    return nil
}

func (g *CustomGetter) Has(key string) bool {
    _, ok := g.data[key]
    return ok
}

// Use with Raw/RawInto
getter := &CustomGetter{data: myData}
result, err := binding.Raw[MyStruct](getter, "custom")
```

### GetterFunc Adapter

Use a function as a ValueGetter:

```go
getter := binding.GetterFunc(func(key string) ([]string, bool) {
    if val, ok := myMap[key]; ok {
        return []string{val}, true
    }
    return nil, false
})

result, err := binding.Raw[MyStruct](getter, "custom")
```

### Streaming with io.Reader

For large payloads, use Reader variants to avoid loading entire body into memory:

```go
// JSON from reader
user, err := binding.JSONReader[User](r.Body)

// XML from reader
doc, err := binding.XMLReader[Document](r.Body)

// YAML from reader
config, err := yaml.YAMLReader[Config](r.Body)
```

### Nested Structs

```go
type Address struct {
    Street string `json:"street"`
    City   string `json:"city"`
    Zip    string `json:"zip"`
}

type User struct {
    Name    string  `json:"name"`
    Address Address `json:"address"` // Nested struct
}

// Query parameters with dot notation
// ?address.street=123+Main&address.city=Boston
params, err := binding.Query[User](r.URL.Query())
```

### Slices and Maps

```go
type Request struct {
    // Slice from repeated parameters: ?tags=go&tags=rust
    Tags []string `query:"tags"`
    
    // Slice from CSV: ?tags=go,rust,python
    TagsCSV []string `query:"tags_csv"`
    
    // Map from dot notation: ?meta.key1=val1&meta.key2=val2
    Meta map[string]string `query:"meta"`
}

params, err := binding.Query[Request](values,
    binding.WithSliceMode(binding.SliceCSV), // For TagsCSV field
)
```

## Sub-Packages

### YAML

```go
import "rivaas.dev/binding/yaml"

type Config struct {
    Name  string `yaml:"name"`
    Port  int    `yaml:"port"`
    Debug bool   `yaml:"debug"`
}

config, err := yaml.YAML[Config](body)

// With strict mode
config, err := yaml.YAML[Config](body, yaml.WithStrict())
```

### TOML

```go
import "rivaas.dev/binding/toml"

type Config struct {
    Name  string `toml:"name"`
    Port  int    `toml:"port"`
}

config, err := toml.TOML[Config](body)
```

### MessagePack

```go
import "rivaas.dev/binding/msgpack"

type Message struct {
    ID   int    `msgpack:"id"`
    Data []byte `msgpack:"data"`
}

msg, err := msgpack.MsgPack[Message](body)
```

### Protocol Buffers

```go
import "rivaas.dev/binding/proto"
import pb "myapp/proto"

user, err := proto.Proto[*pb.User](body)
```

## Performance

### Caching

Struct reflection information is cached automatically for optimal performance:

- First binding of a type: ~500ns overhead for reflection
- Subsequent bindings: ~50ns overhead (cache lookup)
- Cache is thread-safe and has no size limit

### Memory Allocation

- **Query/Path/Form/Header/Cookie**: Zero allocations for primitive types
- **JSON/XML**: Allocations depend on encoding/json and encoding/xml
- **Nested structs**: One allocation per nesting level
- **Slices/Maps**: Pre-allocated with capacity hints when possible

### Benchmarks

```
BenchmarkQuery-8           8,000,000    150 ns/op     0 B/op    0 allocs/op
BenchmarkJSON-8            2,000,000    800 ns/op   256 B/op    3 allocs/op
BenchmarkMultiSource-8     1,000,000  1,200 ns/op   384 B/op    5 allocs/op
```

### Best Practices

1. **Reuse Binder instances** - Create once, use across handlers
2. **Use generic API** - Compile-time type safety with zero runtime overhead
3. **Set security limits** - Prevent resource exhaustion with MaxDepth/MaxSliceLen/MaxMapSize
4. **Cache struct tags** - Happens automatically, but avoid dynamic struct generation
5. **Use Reader variants** - For large payloads (>1MB) to avoid memory spikes

## Integration Examples

### With net/http

```go
func CreateUserHandler(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)
    defer r.Body.Close()
    
    user, err := binding.JSON[CreateUserRequest](body)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // Process user...
}
```

### With rivaas.dev/router

```go
import "rivaas.dev/router"

func CreateUserHandler(c *router.Context) {
    user, err := binding.JSON[CreateUserRequest](c.Body())
    if err != nil {
        c.Error(err, http.StatusBadRequest)
        return
    }
    
    c.JSON(http.StatusCreated, user)
}
```

### With rivaas.dev/app

```go
import "rivaas.dev/app"

func CreateUserHandler(c *app.Context) {
    var user CreateUserRequest
    if err := c.Bind(&user); err != nil {
        return // Error automatically handled
    }
    
    c.JSON(http.StatusCreated, user)
}
```

## Comparison with Other Libraries

| Feature | rivaas.dev/binding | go-playground/form | gorilla/schema |
|---------|-------------------|-------------------|----------------|
| Generic API | ✅ | ❌ | ❌ |
| Multi-source | ✅ | ❌ | ❌ |
| JSON/XML | ✅ | ❌ | ❌ |
| Custom converters | ✅ | ✅ | ✅ |
| Enum validation | ✅ | ❌ | ❌ |
| Error context | ✅ | ⚠️ | ⚠️ |
| Zero allocation | ✅ | ❌ | ❌ |
| Nested structs | ✅ | ✅ | ✅ |

## Documentation

- [Package Documentation](https://pkg.go.dev/rivaas.dev/binding) - Full API reference
- [Examples](./example_test.go) - Runnable examples
- [Tests](.) - Comprehensive test suite

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Write tests for your changes
4. Ensure all tests pass: `go test ./...`
5. Submit a pull request

## License

Apache License 2.0 — see [LICENSE](../LICENSE) for details.

---

Part of the [Rivaas](https://github.com/rivaas-dev/rivaas) web framework ecosystem.

