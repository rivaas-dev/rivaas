# OpenAPI

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/openapi.svg)](https://pkg.go.dev/rivaas.dev/openapi)
[![Go Report Card](https://goreportcard.com/badge/rivaas.dev/openapi)](https://goreportcard.com/report/rivaas.dev/openapi)
[![Coverage](https://codecov.io/gh/rivaas-dev/rivaas/branch/main/graph/badge.svg?component=module_openapi)](https://codecov.io/gh/rivaas-dev/rivaas)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../LICENSE)

Automatic OpenAPI 3.0.4 and 3.1.2 specification generation for Go applications.

This package enables automatic generation of OpenAPI specifications from Go code using struct tags and reflection. It provides a clean, type-safe API for building specifications with minimal boilerplate.

## Features

- **Clean API** - Builder-style `API.Generate()` method for specification generation
- **Type-Safe Version Selection** - `V30x` and `V31x` constants with IDE autocomplete
- **Fluent HTTP Method Constructors** - `GET()`, `POST()`, `PUT()`, etc. for clean operation definitions
- **Functional Options** - Consistent `With*` pattern for all configuration
- **Type-Safe Warning Diagnostics** - `diag` package for fine-grained warning control
- **Automatic Parameter Discovery** - Extracts query, path, header, and cookie parameters from struct tags
- **Schema Generation** - Converts Go types to OpenAPI schemas automatically
- **Swagger UI Configuration** - Built-in, customizable Swagger UI settings
- **Semantic Operation IDs** - Auto-generates operation IDs from HTTP methods and paths
- **Security Schemes** - Support for Bearer, API Key, OAuth2, and OpenID Connect
- **Collision-Resistant Naming** - Schema names use `pkgname.TypeName` format to prevent collisions
- **Built-in Validation** - Validates generated specs against official OpenAPI meta-schemas
- **Standalone Validator** - Validate external OpenAPI specs with pre-compiled schemas

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

## Configuration

Configuration is done exclusively through functional options with `With*` prefix.

### Basic Configuration

```go
api := openapi.MustNew(
    openapi.WithTitle("My API", "1.0.0"),
    openapi.WithInfoDescription("API description"),
    openapi.WithInfoSummary("Short summary"), // 3.1.x only
    openapi.WithTermsOfService("https://example.com/terms"),
    openapi.WithVersion(openapi.V31x), // or openapi.V30x
)
```

### Version Selection

The package supports two OpenAPI version families:

```go
// Target OpenAPI 3.0.x (generates 3.0.4)
api := openapi.MustNew(
    openapi.WithTitle("API", "1.0.0"),
    openapi.WithVersion(openapi.V30x), // Default
)

// Target OpenAPI 3.1.x (generates 3.1.2)
api := openapi.MustNew(
    openapi.WithTitle("API", "1.0.0"),
    openapi.WithVersion(openapi.V31x),
)
```

The constants `V30x` and `V31x` represent version **families** - internally they map to specific versions (3.0.4 and 3.1.2) in the generated specification.

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
    openapi.InHeader,
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
        AuthorizationURL: "https://example.com/oauth/authorize",
        TokenURL:         "https://example.com/oauth/token",
        Scopes: map[string]string{
            "read":  "Read access",
            "write": "Write access",
        },
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

Swagger UI options are nested under `WithSwaggerUI()` for cleaner namespacing:

```go
openapi.MustNew(
    openapi.WithTitle("API", "1.0.0"),
    openapi.WithSwaggerUI("/docs",
        openapi.WithUIExpansion(openapi.DocExpansionList),
        openapi.WithUITryItOut(true),
        openapi.WithUIRequestSnippets(true, 
            openapi.SnippetCurlBash,
            openapi.SnippetCurlPowerShell,
        ),
        openapi.WithUISyntaxTheme(openapi.SyntaxThemeMonokai),
        openapi.WithUIFilter(true),
        openapi.WithUIPersistAuth(true),
    ),
)

// Or disable Swagger UI
openapi.WithoutSwaggerUI()
```

## Defining Operations

The package provides HTTP method constructors for defining operations:

### HTTP Method Constructors

```go
openapi.GET("/users/:id", opts...)
openapi.POST("/users", opts...)
openapi.PUT("/users/:id", opts...)
openapi.PATCH("/users/:id", opts...)
openapi.DELETE("/users/:id", opts...)
openapi.HEAD("/users/:id", opts...)
openapi.OPTIONS("/users", opts...)
openapi.TRACE("/debug", opts...)
```

### Operation Options

All operation options follow the `With*` naming convention for consistency:

| Function | Description |
|----------|-------------|
| `WithSummary(s)` | Set operation summary |
| `WithDescription(s)` | Set operation description |
| `WithOperationID(id)` | Set custom operation ID |
| `WithRequest(type, examples...)` | Set request body type |
| `WithResponse(status, type, examples...)` | Set response type for status code |
| `WithTags(tags...)` | Add tags to operation |
| `WithSecurity(scheme, scopes...)` | Add security requirement |
| `WithDeprecated()` | Mark operation as deprecated |
| `WithConsumes(types...)` | Set accepted content types |
| `WithProduces(types...)` | Set returned content types |
| `WithOperationExtension(key, value)` | Add operation extension |

### Complete Operation Example

```go
openapi.PUT("/users/:id",
    openapi.WithSummary("Update user"),
    openapi.WithDescription("Updates an existing user"),
    openapi.WithOperationID("updateUser"),
    openapi.WithRequest(UpdateUserRequest{}),
    openapi.WithResponse(200, User{}),
    openapi.WithResponse(404, ErrorResponse{}),
    openapi.WithResponse(400, ErrorResponse{}),
    openapi.WithTags("users"),
    openapi.WithSecurity("bearerAuth"),
    openapi.WithDeprecated(),
)
```

### Composable Operation Options

Use `WithOptions()` to create reusable option sets:

```go
// Define reusable option sets
var (
    CommonErrors = openapi.WithOptions(
        openapi.WithResponse(400, Error{}),
        openapi.WithResponse(401, Error{}),
        openapi.WithResponse(500, Error{}),
    )
    
    UserEndpoint = openapi.WithOptions(
        openapi.WithTags("users"),
        openapi.WithSecurity("jwt"),
        CommonErrors,
    )
)

// Apply to operations
openapi.GET("/users/:id",
    UserEndpoint,
    openapi.WithSummary("Get user"),
    openapi.WithResponse(200, User{}),
)

openapi.POST("/users",
    UserEndpoint,
    openapi.WithSummary("Create user"),
    openapi.WithRequest(CreateUser{}),
    openapi.WithResponse(201, User{}),
)
```

### Request Documentation

```go
type GetUserRequest struct {
    ID     int    `path:"id" doc:"User ID" example:"123"`
    Expand string `query:"expand" doc:"Fields to expand" enum:"profile,settings"`
    Format string `header:"Accept" doc:"Response format" enum:"json,xml"`
}

result, err := api.Generate(context.Background(),
    openapi.GET("/users/:id",
        openapi.WithResponse(200, User{}),
    ),
)
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

result, err := api.Generate(context.Background(),
    openapi.GET("/users/:id",
        openapi.WithResponse(200, UserResponse{}),
        openapi.WithResponse(404, ErrorResponse{}),
        openapi.WithResponse(500, ErrorResponse{}),
    ),
)
```

### Security Requirements

```go
// Single security scheme
openapi.GET("/users/:id",
    openapi.WithSecurity("bearerAuth"),
)

// OAuth2 with scopes
openapi.POST("/users",
    openapi.WithSecurity("oauth2", "read", "write"),
)

// Multiple security schemes (OR)
openapi.DELETE("/users/:id",
    openapi.WithSecurity("bearerAuth"),
    openapi.WithSecurity("apiKey"),
)
```

## Auto-Discovery

The package automatically discovers API parameters from struct tags.

### Supported Tags

- **`path:"name"`** - Path parameters (always required)
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
    UserID int `path:"id" doc:"User ID" example:"123"`
    
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

## Generating Specifications

Use `api.Generate()` with a context and variadic operation arguments:

```go
api := openapi.MustNew(
    openapi.WithTitle("My API", "1.0.0"),
)

result, err := api.Generate(context.Background(),
    openapi.GET("/users",
        openapi.WithSummary("List users"),
        openapi.WithResponse(200, []User{}),
    ),
    openapi.GET("/users/:id",
        openapi.WithSummary("Get user"),
        openapi.WithResponse(200, User{}),
    ),
    openapi.POST("/users",
        openapi.WithSummary("Create user"),
        openapi.WithRequest(CreateUserRequest{}),
        openapi.WithResponse(201, User{}),
    ),
)
if err != nil {
    log.Fatal(err)
}

// result.JSON contains the OpenAPI specification as JSON
// result.YAML contains the OpenAPI specification as YAML
// result.Warnings contains any generation warnings
```

### Working with Warnings

The package generates warnings when 3.1-only features are used with a 3.0 target:

```go
result, err := api.Generate(context.Background(), ops...)
if err != nil {
    log.Fatal(err)
}

// Basic warning check
if len(result.Warnings) > 0 {
    fmt.Printf("Generated with %d warnings\n", len(result.Warnings))
}

// Iterate through warnings
for _, warn := range result.Warnings {
    fmt.Printf("[%s] %s\n", warn.Code(), warn.Message())
}
```

### Type-Safe Warning Checks

Import the `diag` package for type-safe warning handling:

```go
import "rivaas.dev/openapi/diag"

// Check for specific warning
if result.Warnings.Has(diag.WarnDownlevelWebhooks) {
    log.Warn("webhooks not supported in OpenAPI 3.0")
}

// Filter by category
downlevelWarnings := result.Warnings.FilterCategory(diag.CategoryDownlevel)
fmt.Printf("Downlevel warnings: %d\n", len(downlevelWarnings))

// Check for any of multiple codes
if result.Warnings.HasAny(
    diag.WarnDownlevelMutualTLS,
    diag.WarnDownlevelWebhooks,
) {
    log.Warn("Some 3.1 security features were dropped")
}

// Filter specific warnings
licenseWarnings := result.Warnings.Filter(diag.WarnDownlevelLicenseIdentifier)

// Exclude expected warnings
unexpected := result.Warnings.Exclude(diag.WarnDownlevelInfoSummary)
```

## Advanced Usage

### Custom Operation IDs

```go
openapi.GET("/users/:id",
    openapi.WithOperationID("retrieveUser"),
    openapi.WithResponse(200, User{}),
)
```

### Extensions

Add custom `x-*` extensions to various parts of the spec:

```go
// Root-level extensions
api := openapi.MustNew(
    openapi.WithTitle("API", "1.0.0"),
    openapi.WithExtension("x-api-version", "v2"),
    openapi.WithExtension("x-custom-feature", true),
)
```

### Strict Downlevel Mode

By default, using 3.1-only features with a 3.0 target generates warnings. Enable strict mode to error instead:

```go
api := openapi.MustNew(
    openapi.WithTitle("API", "1.0.0"),
    openapi.WithVersion(openapi.V30x),
    openapi.WithStrictDownlevel(true), // Error on 3.1 features
    openapi.WithInfoSummary("Summary"), // This will cause an error
)

_, err := api.Generate(context.Background(), ops...)
// err will be non-nil due to strict mode violation
```

### Validation

The package provides built-in validation against official OpenAPI meta-schemas:

```go
// Enable validation (opt-in for performance)
api := openapi.MustNew(
    openapi.WithTitle("My API", "1.0.0"),
    openapi.WithValidation(true), // Enable validation
)

result, err := api.Generate(context.Background(), ops...)
if err != nil {
    log.Fatal(err) // Will fail if spec is invalid
}
```

Validation is disabled by default for performance. Enable it during development or in CI/CD pipelines.

#### Validate External Specs

```go
import "rivaas.dev/openapi/validate"

// Auto-detect version
specJSON, _ := os.ReadFile("openapi.json")
validator := validate.New()
err := validator.Validate(context.Background(), specJSON, validate.V30)
if err != nil {
    log.Fatal(err)
}
```

#### Swagger UI Validation

```go
// Use local validation (no external calls, recommended)
openapi.WithSwaggerUI("/docs",
    openapi.WithUIValidator(openapi.ValidatorLocal),
)

// Use external validator
openapi.WithSwaggerUI("/docs",
    openapi.WithUIValidator("https://validator.swagger.io/validator"),
)

// Disable validation
openapi.WithSwaggerUI("/docs",
    openapi.WithUIValidator(openapi.ValidatorNone),
)
```

## Swagger UI Customization

### Common Options

```go
openapi.WithSwaggerUI("/docs",
    // Document expansion
    openapi.WithUIExpansion(openapi.DocExpansionList),     // list, full, none
    openapi.WithUIModelsExpandDepth(1),                    // How deep to expand models
    openapi.WithUIModelExpandDepth(1),                     // How deep to expand model

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

    // Authentication persistence
    openapi.WithUIPersistAuth(true),                         // Persist auth across refreshes
    openapi.WithUIWithCredentials(true),                    // Send credentials with requests
)
```

## Complete Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "rivaas.dev/openapi"
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
    ID int `path:"id" doc:"User ID"`
}

type ErrorResponse struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

func main() {
    api := openapi.MustNew(
        openapi.WithTitle("User API", "1.0.0"),
        openapi.WithInfoDescription("API for managing users"),
        openapi.WithServer("http://localhost:8080", "Local development"),
        openapi.WithServer("https://api.example.com", "Production"),
        openapi.WithBearerAuth("bearerAuth", "JWT authentication"),
        openapi.WithTag("users", "User management operations"),
    )

    result, err := api.Generate(context.Background(),
        openapi.GET("/users/:id",
            openapi.WithSummary("Get user"),
            openapi.WithDescription("Retrieves a user by ID"),
            openapi.WithResponse(200, User{}),
            openapi.WithResponse(404, ErrorResponse{}),
            openapi.WithTags("users"),
            openapi.WithSecurity("bearerAuth"),
        ),
        openapi.POST("/users",
            openapi.WithSummary("Create user"),
            openapi.WithDescription("Creates a new user"),
            openapi.WithRequest(CreateUserRequest{}),
            openapi.WithResponse(201, User{}),
            openapi.WithResponse(400, ErrorResponse{}),
            openapi.WithTags("users"),
            openapi.WithSecurity("bearerAuth"),
        ),
        openapi.DELETE("/users/:id",
            openapi.WithSummary("Delete user"),
            openapi.WithDescription("Deletes a user"),
            openapi.WithResponse(204, nil),
            openapi.WithTags("users"),
            openapi.WithSecurity("bearerAuth"),
        ),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Print any warnings
    for _, warn := range result.Warnings {
        log.Printf("Warning: %s", warn.Message())
    }

    // Output the specification
    fmt.Println(string(result.JSON))
}
```

## API Reference

Full API documentation is available at [pkg.go.dev/rivaas.dev/openapi](https://pkg.go.dev/rivaas.dev/openapi).

### Key Types

- **`API`** - OpenAPI configuration (created via `New()` or `MustNew()`)
- **`Operation`** - An HTTP operation with method, path, and metadata
- **`Result`** - Result of spec generation (JSON + YAML + Warnings)
- **`Version`** - Type-safe OpenAPI version (`V30x` or `V31x`)

### HTTP Method Constructors

- **`GET(path, ...opts) Operation`**
- **`POST(path, ...opts) Operation`**
- **`PUT(path, ...opts) Operation`**
- **`PATCH(path, ...opts) Operation`**
- **`DELETE(path, ...opts) Operation`**
- **`HEAD(path, ...opts) Operation`**
- **`OPTIONS(path, ...opts) Operation`**
- **`TRACE(path, ...opts) Operation`**

### Key Functions

- **`New(...Option) (*API, error)`** - Create API configuration with validation
- **`MustNew(...Option) *API`** - Create API configuration (panics on error)
- **`(api *API) Generate(ctx context.Context, ...Operation) (*Result, error)`** - Generate OpenAPI spec

### Warning Diagnostics

Import `rivaas.dev/openapi/diag` for type-safe warning handling:

- **`Warning`** interface - Individual warning with `Code()`, `Message()`, `Path()`, `Category()`
- **`Warnings`** - Collection with helper methods (`Has`, `Filter`, `FilterCategory`, etc.)
- **`WarningCode`** - Type-safe warning code constants (`WarnDownlevelWebhooks`, etc.)
- **`WarningCategory`** - Warning categories (`CategoryDownlevel`, `CategorySchema`)

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

When using OpenAPI 3.0.x target, some 3.1.x features are automatically down-leveled with warnings:

- `info.summary` - Dropped (3.1-only)
- `license.identifier` - Dropped (3.1-only)
- `const` in schemas - Converted to `enum` with single value
- `examples` in schemas - Converted to single `example`
- `webhooks` - Dropped (3.1-only)
- `mutualTLS` security - Dropped (3.1-only)

Enable `StrictDownlevel` to error instead of warn:

```go
api := openapi.MustNew(
    openapi.WithTitle("API", "1.0.0"),
    openapi.WithVersion(openapi.V30x),
    openapi.WithStrictDownlevel(true),
)
```

## API Reference

For detailed API documentation, see [pkg.go.dev/rivaas.dev/openapi](https://pkg.go.dev/rivaas.dev/openapi).

## Contributing

Contributions are welcome! Please see the [main repository](../) for contribution guidelines.

## License

Apache License 2.0 - see [LICENSE](../LICENSE) for details.

---

Part of the [Rivaas](https://github.com/rivaas-dev/rivaas) web framework ecosystem.
