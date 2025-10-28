# Request Binding Example

This example demonstrates automatic request data binding to Go structs using the Rivaas router's `Bind*()` methods.

## What is Request Binding?

Request binding automatically maps request data (body, query params, URL params, cookies, headers) to Go struct fields using reflection and struct tags. This eliminates boilerplate code for parsing and type conversion.

## Available Methods

### `BindBody(out any) error`

Binds request body to struct based on Content-Type header:
- `application/json` → uses `json` tags
- `application/x-www-form-urlencoded` → uses `form` tags
- `multipart/form-data` → uses `form` tags

### `BindQuery(out any) error`

Binds query parameters to struct using `query` tags.

### `BindParams(out any) error`

Binds URL path parameters to struct using `params` tags.

### `BindCookies(out any) error`

Binds cookies to struct using `cookie` tags.

### `BindHeaders(out any) error`

Binds request headers to struct using `header` tags.

### `BindForm(out any) error` / `BindJSON(out any) error`

Explicit content type binding (bypass auto-detection).

## Quick Examples

### 1. JSON Body Binding

```bash
curl -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com","age":25}'
```

```go
type CreateUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
    Age   int    `json:"age"`
}

var req CreateUserRequest
if err := c.BindBody(&req); err != nil {
    // Handle error
}
// req.Name = "Alice", req.Email = "alice@example.com", req.Age = 25
```

### 2. Query Parameter Binding

```bash
curl "http://localhost:8080/api/search?q=golang&page=2&page_size=20&tags=web&tags=api&active=true"
```

```go
type SearchParams struct {
    Query    string   `query:"q"`
    Page     int      `query:"page"`
    PageSize int      `query:"page_size"`
    Tags     []string `query:"tags"`
    Active   bool     `query:"active"`
}

var params SearchParams
if err := c.BindQuery(&params); err != nil {
    // Handle error
}
// params populated from query string
```

### 3. URL Path Parameter Binding

```bash
curl http://localhost:8080/api/users/123/posts/456
```

```go
type PathParams struct {
    UserID int `params:"id"`
    PostID int `params:"post_id"`
}

// Route: /api/users/:id/posts/:post_id
var params PathParams
if err := c.BindParams(&params); err != nil {
    // Handle error
}
// params.UserID = 123, params.PostID = 456
```

### 4. Cookie Binding

```bash
curl http://localhost:8080/api/session \
  --cookie "session_id=abc123;theme=dark;remember_me=true"
```

```go
type SessionCookies struct {
    SessionID  string `cookie:"session_id"`
    Theme      string `cookie:"theme"`
    RememberMe bool   `cookie:"remember_me"`
}

var cookies SessionCookies
if err := c.BindCookies(&cookies); err != nil {
    // Handle error
}
```

### 5. Header Binding

```bash
curl http://localhost:8080/api/client-info \
  -H "User-Agent: CustomClient/1.0" \
  -H "Authorization: Bearer token123"
```

```go
type ClientHeaders struct {
    UserAgent     string `header:"User-Agent"`
    Authorization string `header:"Authorization"`
}

var headers ClientHeaders
if err := c.BindHeaders(&headers); err != nil {
    // Handle error
}
```

## Supported Types

### Primitive Types
- - `string`
- - `int`, `int8`, `int16`, `int32`, `int64`
- - `uint`, `uint8`, `uint16`, `uint32`, `uint64`
- - `float32`, `float64`
- - `bool`

### Time & Duration Types
- - **`time.Time`**: Supports RFC3339, date-only, and common formats
- - **`time.Duration`**: Parses `5s`, `10m`, `1h30m`, etc.

### Network Types
- - **`net.IP`**: IPv4 and IPv6 addresses
- - **`url.URL`**: Full URL parsing

### Custom Types
- - **`encoding.TextUnmarshaler`**: Any custom type implementing this interface

### Composite Types
- - **Slices**: `[]string`, `[]int`, `[]time.Time`, `[]net.IP`, etc.
- - **Pointers**: `*string`, `*int`, `*time.Time`, `*url.URL`, etc.
- - **Embedded Structs**: Composition and code reuse

### Boolean Values

The binding system accepts multiple boolean representations:

**True**: `true`, `1`, `yes`, `on`, `t`, `y` (case-insensitive)

**False**: `false`, `0`, `no`, `off`, `f`, `n`, `""` (case-insensitive)

## Advanced Features

### Time & Duration Types

```go
type EventParams struct {
    StartDate time.Time     `query:"start"`   // RFC3339, date-only, or other formats
    EndDate   time.Time     `query:"end"`
    Timeout   time.Duration `query:"timeout"` // e.g., "30s", "5m", "1h"
}

// Usage:
// ?start=2024-01-15T10:00:00Z&end=2024-01-20&timeout=30s
```

**Supported time formats:**
- RFC3339: `2024-01-15T10:30:00Z`
- RFC3339Nano: `2024-01-15T10:30:00.123456789Z`
- Date only: `2024-01-15`
- DateTime: `2024-01-15 10:30:00`
- RFC1123: `Mon, 15 Jan 2024 10:30:00 MST`
- And more...

**Duration format examples:**
- `5s` - 5 seconds
- `10m` - 10 minutes
- `1h30m` - 1 hour 30 minutes
- `500ms` - 500 milliseconds

### Network Types

```go
type NetworkConfig struct {
    AllowedIP net.IP     `query:"allowed_ip"`  // IPv4 or IPv6
    Subnet    net.IPNet  `query:"subnet"`      // CIDR notation
    ProxyURL  url.URL    `query:"proxy"`       // Full URL with scheme
    IPs       []net.IP   `query:"ips"`         // Multiple IPs
    Ranges    []net.IPNet `query:"ranges"`     // Multiple CIDR ranges
}

// Usage:
// ?allowed_ip=192.168.1.1&subnet=10.0.0.0/8&proxy=http://proxy.example.com:8080
// ?ips=192.168.1.1&ips=10.0.0.1
// ?ranges=10.0.0.0/8&ranges=172.16.0.0/12&ranges=192.168.0.0/16
```

### Custom Types with TextUnmarshaler

Implement `encoding.TextUnmarshaler` for custom parsing:

```go
type UUID string

func (u *UUID) UnmarshalText(text []byte) error {
    // Custom validation logic
    if !isValidUUID(string(text)) {
        return errors.New("invalid UUID format")
    }
    *u = UUID(text)
    return nil
}

type Request struct {
    ID UUID `query:"id"`  // Automatically uses UnmarshalText
}
```

### Embedded Structs

Reuse common struct patterns:

```go
type Pagination struct {
    Page     int `query:"page"`
    PageSize int `query:"page_size"`
}

type SearchRequest struct {
    Pagination  // Embedded - no field name needed
    Query string `query:"q"`
    Sort  string `query:"sort"`
}

// Usage:
var req SearchRequest
c.BindQuery(&req)
// Now req.Page, req.PageSize, req.Query, req.Sort are all populated
```

### Maps with Dot Notation

Use maps for dynamic key-value data:

```go
type FilterParams struct {
    Metadata map[string]string  `query:"metadata"`  // Dynamic metadata
    Settings map[string]any     `query:"settings"`  // Flexible settings  
    Scores   map[string]int     `query:"scores"`    // Typed map values
    Rates    map[string]float64 `query:"rates"`     // Float map values
}

// Query syntax with dot notation:
// ?metadata.name=John&metadata.age=30&metadata.city=NYC
// Results in: map[string]string{"name": "John", "age": "30", "city": "NYC"}

// Typed map values automatically converted:
// ?scores.math=95&scores.science=88
// Results in: map[string]int{"math": 95, "science": 88}

// ?rates.usd=1.0&rates.eur=0.85
// Results in: map[string]float64{"usd": 1.0, "eur": 0.85}
```

**Supported map types:**
- `map[string]string` - String values
- `map[string]int` - Integer values
- `map[string]float64` - Float values  
- `map[string]bool` - Boolean values
- `map[string]any` - Interface{} values
- `map[string]time.Time` - Time values
- `map[string]time.Duration` - Duration values
- `map[string]net.IP` - IP address values
- Any other supported value type

**Map Syntax Reference - Both Supported!**

- **Dot Notation** (clean, recommended):
```bash
?metadata.name=John&metadata.age=30
?filters.status=active&filters.category=tech
?settings.debug=true&settings.port=8080
```

- **Bracket Notation** (PHP/Laravel-style):
```bash
?metadata[name]=John&metadata[age]=30
?scores[math]=95&scores[science]=88
?config[debug]=true
```

- **Quoted Keys** (for keys with special characters):
```bash
?metadata["user.name"]=John&metadata["user-email"]=test@example.com
?config['db.host']=localhost&config['db.port']=5432
```

- **Mixed Syntax** (use both together):
```bash
?metadata.key1=value1&metadata[key2]=value2&metadata.key3=value3
# All three keys go into the same map!
```

- **Array Notation** (not supported for maps):
```bash
?metadata[]=value              # Empty brackets (use for slices, not maps)
?metadata[key1][key2]=value    # Nested brackets (not supported)
```

**Which syntax should you use?**
- **Dot notation**: Cleaner, more URL-friendly, recommended for simple keys
- **Bracket notation**: Better for keys with dots/dashes, familiar to PHP developers
- **Quoted bracket**: Required for keys containing `.` or `-` characters
- **Mixed**: Use whatever makes sense for each key!

**Example - When quotes are needed:**
```bash
# Without quotes - creates nested structure (not a map)
?config.db.host=localhost → Tries to bind to nested struct

# With quotes - creates map with dotted key
?config["db.host"]=localhost → Map key is literally "db.host"
```

### Nested Structs with Dot Notation

Organize complex data with nested structs:

```go
type Address struct {
    Street  string `query:"street"`
    City    string `query:"city"`
    ZipCode string `query:"zip_code"`
}

type UserRequest struct {
    Name    string  `query:"name"`
    Email   string  `query:"email"`
    Address Address `query:"address"`  // Nested struct
}

// Query syntax:
// ?name=John&email=john@example.com&address.street=123+Main+St&address.city=NYC&address.zip_code=10001

// Deeply nested also works:
type Location struct {
    Lat float64 `query:"lat"`
    Lng float64 `query:"lng"`
}

type FullAddress struct {
    Street   string   `query:"street"`
    Location Location `query:"location"`  // Nested in nested!
}

// Query:
// ?address.street=Main+St&address.location.lat=40.7128&address.location.lng=-74.0060
```

### Regular Expression Patterns

Bind regex patterns for search/filter:

```go
type SearchParams struct {
    Pattern regexp.Regexp `query:"pattern"`  // Compiled and ready
}

// Usage:
// ?pattern=^user-[0-9]+$

// The regex is automatically compiled and ready to use:
if params.Pattern.MatchString("user-123") {
    // Matches!
}
```

****Security Warning**: Allowing user-provided regex patterns can lead to **ReDoS** (Regular Expression Denial of Service) attacks.  Use with caution:
- - Only accept patterns from trusted sources
- - Set timeouts on regex matching
- - Consider whitelisting allowed patterns
- - Validate pattern complexity before use
- - Never allow untrusted user input without validation

### Enum Validation

Validate that values are in an allowed set using the `enum` struct tag:

```go
type UserFilter struct {
    Status   string `query:"status" enum:"active,inactive,pending"`
    Role     string `query:"role" enum:"admin,user,guest"`
    Priority string `query:"priority" enum:"low,medium,high"`
}

// Valid requests:
// ?status=active&role=admin&priority=high 

// Invalid requests:
// ?status=deleted  - Error: value "deleted" not in allowed values: active,inactive,pending
// ?role=superadmin  - Error: value "superadmin" not in allowed values: admin,user,guest
```

**Benefits:**
- - Declarative validation at struct definition
- - Automatic validation during binding
- - Clear, actionable error messages
- - No manual validation code needed
- - Self-documenting allowed values

### Default Values

Provide default values when parameters are not specified:

```go
type PaginationParams struct {
    Page     int    `query:"page" default:"1"`
    PageSize int    `query:"page_size" default:"10"`
    Sort     string `query:"sort" default:"created_at"`
    Order    string `query:"order" default:"desc"`
    Active   bool   `query:"active" default:"true"`
}

// No query params → all defaults applied:
// ?  → {Page: 1, PageSize: 10, Sort: "created_at", Order: "desc", Active: true}

// Partial params → user values override defaults:
// ?page=5&active=false  → {Page: 5, PageSize: 10, Sort: "created_at", Order: "desc", Active: false}
```

**Works with all types:**
```go
type AdvancedDefaults struct {
    Timeout  time.Duration `query:"timeout" default:"30s"`
    Created  time.Time     `query:"created" default:"2024-01-01T00:00:00Z"`
    MaxRetry int           `query:"max_retry" default:"3"`
}
```

**Benefits:**
- - Eliminate manual default value handling
- - Self-documenting API defaults
- - Works with all supported types
- - User values always take precedence
- - Clean, declarative code

### Optional Fields with Pointers

Use pointers to distinguish between "not provided" and "zero value":

```go
type ProductFilters struct {
    Category *string  `query:"category"`  // nil if not provided
    MinPrice *float64 `query:"min_price"` // nil if not provided
    InStock  *bool    `query:"in_stock"`  // nil if not provided
}

var filters ProductFilters
c.BindQuery(&filters)

if filters.Category != nil {
    // Category was explicitly provided
    fmt.Println("Category:", *filters.Category)
} else {
    // Category was not in query string
}
```

### Slices for Multiple Values

```bash
curl "http://localhost:8080/api/batch?ids=1&ids=2&ids=3&tags=go&tags=rust"
```

```go
type BatchRequest struct {
    IDs  []int    `query:"ids"`   // [1, 2, 3]
    Tags []string `query:"tags"`  // ["go", "rust"]
}
```

### Combined Sources

Bind data from multiple sources to the same struct:

```go
type UpdateRequest struct {
    UserID int    `params:"id"`       // From URL
    Notify bool   `query:"notify"`    // From query
    Name   string `json:"name"`       // From body
    Email  string `json:"email"`      // From body
}

var req UpdateRequest
c.BindParams(&req)  // Bind UserID from URL
c.BindQuery(&req)   // Bind Notify from query
c.BindBody(&req)    // Bind Name, Email from JSON body
```

## Error Handling

### Binding Errors

Binding methods return detailed errors when conversion fails:

```go
var params struct {
    Age int `query:"age"`
}

if err := c.BindQuery(&params); err != nil {
    // Error message: binding field "Age" (tag:query): failed to convert "invalid" to int: ...
    c.JSON(400, map[string]string{"error": err.Error()})
    return
}
```

### Custom Error Responses

```go
if err := c.BindBody(&req); err != nil {
    c.JSON(http.StatusBadRequest, map[string]any{
        "error":   "Invalid request data",
        "details": err.Error(),
        "code":    "BIND_ERROR",
    })
    return
}
```

## Best Practices

### 1. Always Check Errors

```go
// - BAD - ignoring errors
var req Request
c.BindBody(&req)

// - GOOD - proper error handling
var req Request
if err := c.BindBody(&req); err != nil {
    c.JSON(400, map[string]string{"error": err.Error()})
    return
}
```

### 2. Validate After Binding

Binding only handles type conversion, not business validation:

```go
var req CreateUserRequest
if err := c.BindBody(&req); err != nil {
    return err
}

// Add business validation
if req.Age < 18 {
    return errors.New("must be 18 or older")
}
if !isValidEmail(req.Email) {
    return errors.New("invalid email format")
}
```

### 3. Use Pointers for Optional Fields

```go
type Filters struct {
    Category *string `query:"category"` // Optional
    Active   bool    `query:"active"`   // Defaults to false if not provided
}
```

### 4. Provide Sensible Defaults

```go
var params SearchParams
c.BindQuery(&params)

if params.PageSize == 0 {
    params.PageSize = 10 // Default page size
}
if params.Page == 0 {
    params.Page = 1 // Default to first page
}
```

### 5. Return Meaningful Error Messages

```go
if err := c.BindParams(&params); err != nil {
    c.JSON(http.StatusBadRequest, map[string]string{
        "error": "Invalid user ID - must be a number",
    })
    return
}
```

## Performance

The binding system is optimized for performance:

- **Type cache**: Struct information is cached after first use
- **Accept header cache**: Parsed headers cached for reuse
- **Minimal allocations**: Efficient reflection usage
- **Fast paths**: Optimized for common types
- **Cache warmup**: Pre-populate caches at startup

Benchmark results (on test machine):

```text
BenchmarkBindJSON-12      124,905    12,387 ns/op    6,612 B/op    24 allocs/op
BenchmarkBindQuery-12   1,252,024     1,260 ns/op      504 B/op     8 allocs/op
BenchmarkBindParams-12  1,797,410       563 ns/op      384 B/op     5 allocs/op
BenchmarkBindQuery_Cached 1,000,000   1,033 ns/op      480 B/op     7 allocs/op
```

### Cache Warmup for Startup Optimization

Pre-parse your request structs at application startup to eliminate first-call reflection overhead:

```go
import "rivaas.dev/router"

type UserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

type SearchParams struct {
    Query string `query:"q"`
    Page  int    `query:"page"`
}

func main() {
    r := router.New()
    
    // Warmup binding cache during startup
    router.WarmupBindingCache(
        UserRequest{},
        SearchParams{},
        // Add all your request types here
    )
    
    // Cache is now populated for faster first requests
    r.GET("/api/users", handler)
    http.ListenAndServe(":8080", r)
}
```

**Benefits:**
- - Eliminates ~100-200ns first-call overhead
- - More predictable latency
- - Better for latency-sensitive APIs
- - Warmup happens once at startup

## Running the Example

```bash
# Start the server
go run main.go

# Test endpoints with the curl commands shown above
```

## Common Patterns

### REST API Endpoint

```go
r.POST("/api/users", func(c *router.Context) {
    var req CreateUserRequest
    if err := c.BindBody(&req); err != nil {
        c.JSON(400, map[string]string{"error": err.Error()})
        return
    }
    
    // Business logic
    user := createUser(req)
    c.JSON(201, user)
})
```

### Search/Filter Endpoint

```go
r.GET("/api/products", func(c *router.Context) {
    var filters ProductFilters
    if err := c.BindQuery(&filters); err != nil {
        c.JSON(400, map[string]string{"error": err.Error()})
        return
    }
    
    products := searchProducts(filters)
    c.JSON(200, products)
})
```

### Resource Update with Multiple Sources

```go
r.PUT("/api/users/:id", func(c *router.Context) {
    var req UpdateUserRequest
    
    c.BindParams(&req)  // Get user ID from URL
    c.BindQuery(&req)   // Get options from query
    c.BindBody(&req)    // Get update data from body
    
    updateUser(req)
    c.JSON(200, map[string]string{"message": "Updated"})
})
```

## See Also

- [Router README](../../README.md) - Main documentation
- [Content Negotiation Example](../07-content-negotiation/) - Format negotiation
- [REST API Example](../04-rest-api/) - Full CRUD API

