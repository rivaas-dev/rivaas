# OpenAPI Package

Automatic OpenAPI 3.0.4 and 3.1.2 specification generation and Swagger UI integration for Rivaas.

This package enables automatic generation of OpenAPI specifications from Go code using struct tags and reflection. It integrates seamlessly with the Rivaas router to provide comprehensive API documentation with minimal boilerplate.

## Features

- **Automatic Parameter Discovery** - Extracts query, path, header, and cookie parameters from struct tags
- **Schema Generation** - Converts Go types to OpenAPI schemas automatically
- **Swagger UI Integration** - Built-in, customizable Swagger UI for interactive API documentation
- **Semantic Operation IDs** - Auto-generates operation IDs from HTTP methods and paths
- **Security Schemes** - Support for Bearer, API Key, OAuth2, and OpenID Connect
- **Version Support** - Generate OpenAPI 3.0.4 or 3.1.2 specifications
- **Collision-Resistant Naming** - Schema names use `pkgname.TypeName` format to prevent collisions
- **ETag-Based Caching** - Spec serving with HTTP caching support
- **Concurrent Safe** - Thread-safe operations for concurrent use

## Quick Start

### With Rivaas App Framework

```go
package main

import (
    "rivaas.dev/app"
    "rivaas.dev/openapi"
)

func main() {
    app := app.New(
        app.WithServiceName("my-api"),
        app.WithOpenAPI(
            openapi.WithTitle("My API", "1.0.0"),
            openapi.WithDescription("API for managing users"),
            openapi.WithServer("http://localhost:8080", "Local development"),
            openapi.WithSwaggerUI(true, "/docs"),
        ),
    )

    app.GET("/users/:id", getUserHandler).
        Doc("Get user", "Retrieves a user by ID").
        Request(GetUserRequest{}).
        Response(200, UserResponse{}).
        Tags("users")

    app.Run()
}
```

### Standalone Usage

```go
package main

import (
    "rivaas.dev/openapi"
)

func main() {
    cfg := openapi.MustNew(
        openapi.WithTitle("My API", "1.0.0"),
        openapi.WithDescription("API description"),
        openapi.WithServer("http://localhost:8080", "Local development"),
    )

    manager := openapi.NewManager(cfg)
    
    manager.Register("GET", "/users/:id").
        Doc("Get user", "Retrieves a user by ID").
        Request(GetUserRequest{}).
        Response(200, UserResponse{})

    specJSON, _, err := manager.GenerateSpec()
    if err != nil {
        log.Fatal(err)
    }
    
    // Serve specJSON via HTTP or save to file
}
```

## Configuration

Configuration is done exclusively through functional options. All configuration types are private to enforce this pattern.

### Basic Configuration

```go
cfg := openapi.MustNew(
    openapi.WithTitle("My API", "1.0.0"),
    openapi.WithDescription("API description"),
    openapi.WithSummary("Short summary"), // 3.1.2 only
    openapi.WithTermsOfService("https://example.com/terms"),
    openapi.WithVersion("3.1.2"), // or "3.0.4"
)
```

### Servers

```go
openapi.WithServer("https://api.example.com", "Production"),
openapi.WithServer("http://localhost:8080", "Local development"),

// With variables
openapi.WithServerVariable("baseUrl", "https://api.example.com", 
    []string{"https://api.example.com", "https://staging.example.com"},
    "Base URL for the API"),
```

### Security Schemes

#### Bearer Authentication

```go
openapi.WithBearerAuth("bearerAuth", "JWT authentication"),
```

#### API Key Authentication

```go
openapi.WithAPIKey(
    "apiKey",
    "X-API-Key",
    openapi.ParameterLocationHeader,
    "API key for authentication",
),
```

#### OAuth2

```go
openapi.WithOAuth2(
    "oauth2",
    "OAuth2 authentication",
    openapi.OAuth2Flow{
        Type:             openapi.FlowAuthorizationCode,
        AuthorizationUrl: "https://example.com/oauth/authorize",
        TokenUrl:         "https://example.com/oauth/token",
        Scopes: map[string]string{
            "read":  "Read access",
            "write": "Write access",
        },
    },
    openapi.OAuth2Flow{
        Type:     openapi.FlowClientCredentials,
        TokenUrl: "https://example.com/oauth/token",
        Scopes:   map[string]string{"read": "Read access"},
    },
),
```

#### OpenID Connect

```go
openapi.WithOpenIDConnect(
    "openId",
    "https://example.com/.well-known/openid-configuration",
    "OpenID Connect authentication",
),
```

### Tags

```go
openapi.WithTag("users", "User management operations"),
openapi.WithTag("posts", "Post management operations"),
```

### Swagger UI Configuration

```go
openapi.WithSwaggerUI(true, "/docs"),

// UI Customization
openapi.WithUIDocExpansion(openapi.DocExpansionList),
openapi.WithUITryItOut(true),
openapi.WithUIRequestSnippets(true, 
    openapi.SnippetCurlBash,
    openapi.SnippetCurlPowerShell,
),
openapi.WithUISyntaxTheme(openapi.SyntaxThemeMonokai),
```

## Route Documentation

The package provides a fluent API for documenting routes:

```go
app.GET("/users/:id", handler).
    Doc("Get user", "Retrieves a user by ID").
    Summary("Get user").                    // Alternative to Doc
    Request(GetUserRequest{}).              // Request body/parameters
    Response(200, UserResponse{}).          // Success response
    Response(404, ErrorResponse{}).         // Error response
    Response(500, ErrorResponse{}).         // Error response
    Tags("users", "public").                // Tags
    OperationID("getUserById").             // Custom operation ID
    Security("bearerAuth").                 // Security requirement
    Deprecated()                            // Mark as deprecated
```

### Request Documentation

```go
type GetUserRequest struct {
    ID     int    `params:"id" doc:"User ID" example:"123"`
    Expand string `query:"expand" doc:"Fields to expand" enum:"profile,settings"`
    Format string `header:"Accept" doc:"Response format" enum:"json,xml"`
}

app.GET("/users/:id", handler).
    Request(GetUserRequest{})
```

### Response Documentation

```go
type UserResponse struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

type ErrorResponse struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

app.GET("/users/:id", handler).
    Response(200, UserResponse{}).
    Response(404, ErrorResponse{}).
    Response(500, ErrorResponse{})
```

### Security Requirements

```go
// Single security scheme
app.GET("/users/:id", handler).
    Security("bearerAuth")

// OAuth2 with scopes
app.POST("/users", handler).
    Security("oauth2", "read", "write")

// Multiple security schemes (OR)
app.DELETE("/users/:id", handler).
    Security("bearerAuth").
    Security("apiKey")
```

## Auto-Discovery

The package automatically discovers API parameters from struct tags compatible with the `binding` package.

### Supported Tags

- **`params:"name"`** - Path parameters (always required)
- **`query:"name"`** - Query parameters
- **`header:"name"`** - Header parameters
- **`cookie:"name"`** - Cookie parameters
- **`json:"name"`** - Request body fields

### Additional Tags

- **`doc:"description"`** - Parameter/field description
- **`example:"value"`** - Example value
- **`enum:"val1,val2"`** - Enum values (comma-separated)
- **`validate:"required"`** - Validation rules (affects `required` in OpenAPI)

### Struct Tag Examples

```go
type CreateUserRequest struct {
    // Path parameter (always required)
    UserID int `params:"id" doc:"User ID" example:"123"`
    
    // Query parameters
    Page    int    `query:"page" doc:"Page number" example:"1" validate:"min=1"`
    PerPage int    `query:"per_page" doc:"Items per page" example:"20" validate:"min=1,max=100"`
    Format  string `query:"format" doc:"Response format" enum:"json,xml"`
    
    // Header parameters
    Accept string `header:"Accept" doc:"Content type" enum:"application/json,application/xml"`
    
    // Request body fields
    Name  string `json:"name" doc:"User name" example:"John Doe" validate:"required"`
    Email string `json:"email" doc:"User email" example:"john@example.com" validate:"required,email"`
    Age   *int   `json:"age,omitempty" doc:"User age" example:"30" validate:"min=0,max=150"`
}

type UpdateUserRequest struct {
    // Path parameter
    ID int `params:"id"`
    
    // Request body (pointer makes it optional)
    Name  *string `json:"name,omitempty" doc:"Updated name"`
    Email *string `json:"email,omitempty" doc:"Updated email"`
}
```

## Schema Generation

Go types are automatically converted to OpenAPI schemas:

### Supported Types

- **Primitives**: `string`, `int`, `int64`, `float64`, `bool`
- **Pointers**: `*string` (nullable, optional)
- **Slices**: `[]string`, `[]int`
- **Maps**: `map[string]int`
- **Structs**: Custom types (become `object` schemas)
- **Time**: `time.Time` (becomes `string` with `date-time` format)
- **Embedded Structs**: Fields from embedded structs are included

### Schema Naming

Component schema names use the format `pkgname.TypeName` to prevent cross-package collisions:

```go
// In package "api"
type User struct { ... }  // Becomes "api.User"

// In package "models"
type User struct { ... }  // Becomes "models.User"
```

### Schema Examples

```go
type User struct {
    ID        int       `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    Tags      []string  `json:"tags"`
    Metadata  map[string]string `json:"metadata"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type UserList struct {
    Users []User `json:"users"`
    Total int    `json:"total"`
}
```

## Advanced Usage

### Custom Operation IDs

```go
app.GET("/users/:id", handler).
    OperationID("retrieveUser")
```

### Extensions

Add custom `x-*` extensions to various parts of the spec:

```go
// Info extensions
cfg := openapi.MustNew(
    openapi.WithInfoExtension("x-api-version", "v2"),
    openapi.WithInfoExtension("x-custom-feature", true),
)

// Server extensions
openapi.WithServerExtension("x-rate-limit", 1000)

// Tag extensions
openapi.WithTagExtension("x-priority", "high")
```

### Version Selection

```go
cfg := openapi.MustNew(
    openapi.WithVersion("3.1.2"), // or "3.0.4"
    openapi.WithStrictDownlevel(true), // Error on 3.1-only features in 3.0
)
```

### Accessing Generated Spec

```go
manager := openapi.NewManager(cfg)
// ... register routes ...

specJSON, etag, err := manager.GenerateSpec()
if err != nil {
    log.Fatal(err)
}

// Use specJSON (ETag for HTTP caching)
w.Header().Set("ETag", etag)
w.Header().Set("Content-Type", "application/json")
w.Write(specJSON)
```

### Warnings

The package generates warnings when 3.1-only features are used with a 3.0 target:

```go
specJSON, warnings, err := manager.GenerateSpec()
for _, warn := range warnings {
    log.Printf("Warning: %s at %s: %s", 
        warn.Code, warn.Path, warn.Message)
}
```

## Swagger UI Customization

### Common Options

```go
openapi.WithSwaggerUI(true, "/docs"),

// Document expansion
openapi.WithUIDocExpansion(openapi.DocExpansionList),     // list, full, none
openapi.WithUIModelsExpandDepth(1),                       // How deep to expand models
openapi.WithUIModelExpandDepth(1),                        // How deep to expand model

// Display options
openapi.WithUIDisplayOperationID(true),                   // Show operation IDs
openapi.WithUIDefaultModelRendering(openapi.ModelRenderingExample), // example, model

// Try it out
openapi.WithUITryItOut(true),                            // Enable "Try it out"
openapi.WithUIRequestSnippets(true,                      // Show request snippets
    openapi.SnippetCurlBash,
    openapi.SnippetCurlPowerShell,
    openapi.SnippetCurlCmd,
),
openapi.WithUIRequestSnippetsExpanded(true),             // Expand snippets by default
openapi.WithUIDisplayRequestDuration(true),              // Show request duration

// Filtering and sorting
openapi.WithUIFilter(true),                              // Enable filter box
openapi.WithUIMaxDisplayedTags(10),                      // Limit displayed tags
openapi.WithUIOperationsSorter(openapi.OperationsSorterAlpha), // alpha, method
openapi.WithUITagsSorter(openapi.TagsSorterAlpha),      // alpha

// Syntax highlighting
openapi.WithUISyntaxHighlight(true),                     // Enable syntax highlighting
openapi.WithUISyntaxTheme(openapi.SyntaxThemeMonokai),  // agate, monokai, etc.

// Validation
openapi.WithUIValidator("https://validator.swagger.io/validator/debug"),

// Authentication persistence
openapi.WithUIPersistAuth(true),                         // Persist auth across refreshes
openapi.WithUIWithCredentials(true),                    // Send credentials with requests
```

## Complete Example

```go
package main

import (
    "rivaas.dev/app"
    "rivaas.dev/openapi"
    "time"
)

type User struct {
    ID        int       `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    CreatedAt time.Time `json:"created_at"`
}

type CreateUserRequest struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

type GetUserRequest struct {
    ID int `params:"id" doc:"User ID"`
}

func main() {
    app := app.New(
        app.WithServiceName("user-api"),
        app.WithOpenAPI(
            openapi.WithTitle("User API", "1.0.0"),
            openapi.WithDescription("API for managing users"),
            openapi.WithServer("http://localhost:8080", "Local development"),
            openapi.WithServer("https://api.example.com", "Production"),
            openapi.WithBearerAuth("bearerAuth", "JWT authentication"),
            openapi.WithTag("users", "User management operations"),
            openapi.WithSwaggerUI(true, "/docs"),
            openapi.WithUIDocExpansion(openapi.DocExpansionList),
            openapi.WithUITryItOut(true),
        ),
    )

    // GET /users/:id
    app.GET("/users/:id", getUserHandler).
        Doc("Get user", "Retrieves a user by ID").
        Request(GetUserRequest{}).
        Response(200, User{}).
        Response(404, ErrorResponse{}).
        Tags("users").
        Security("bearerAuth")

    // POST /users
    app.POST("/users", createUserHandler).
        Doc("Create user", "Creates a new user").
        Request(CreateUserRequest{}).
        Response(201, User{}).
        Response(400, ErrorResponse{}).
        Tags("users").
        Security("bearerAuth")

    app.Run()
}
```

## API Reference

Full API documentation is available at [pkg.go.dev/rivaas.dev/openapi](https://pkg.go.dev/rivaas.dev/openapi).

### Key Types

- **`Config`** - OpenAPI configuration (created via `New()` or `MustNew()`)
- **`Manager`** - Manages spec generation and caching
- **`RouteWrapper`** - Fluent API for route documentation
- **`Option`** - Functional option for configuration

### Key Functions

- **`New(...Option) (*Config, error)`** - Create configuration
- **`MustNew(...Option) *Config`** - Create configuration (panics on error)
- **`NewManager(*Config) *Manager`** - Create manager instance
- **`Manager.Register(method, path) *RouteWrapper`** - Register route
- **`Manager.GenerateSpec() ([]byte, string, error)`** - Generate OpenAPI spec

## Troubleshooting

### Schema Name Collisions

If you have types with the same name in different packages, the package automatically prefixes schema names with the package name:

```go
// api.User becomes "api.User"
// models.User becomes "models.User"
```

### Extension Validation

Extension keys must start with `x-`. In OpenAPI 3.1.x, keys starting with `x-oai-` or `x-oas-` are reserved and will be filtered out.

### Version Compatibility

When using OpenAPI 3.0.4, some 3.1.2 features are automatically down-leveled with warnings:

- `info.summary` - Dropped (3.1-only)
- `license.identifier` - Dropped (3.1-only)
- `const` in schemas - Converted to `enum` with single value
- `webhooks` - Dropped (3.1-only)
- `mutualTLS` security - Dropped (3.1-only)

Enable `StrictDownlevel` to error instead of warn:

```go
openapi.WithStrictDownlevel(true)
```

### Concurrent Access

`Manager` and `RouteWrapper` are safe for concurrent use. However, `RouteWrapper` should be configured by a single goroutine before calling `Freeze()`.

## License

[Add license information here]
