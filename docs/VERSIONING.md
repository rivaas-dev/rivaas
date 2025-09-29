# API Versioning Guide

This guide covers comprehensive API versioning strategies using Rivaas Router, including best practices, performance characteristics, and migration patterns.

## Table of Contents

- [Why Version APIs?](#why-version-apis)
- [Versioning Strategies](#versioning-strategies)
- [Getting Started](#getting-started)
- [Version Detection Methods](#version-detection-methods)
- [Performance Characteristics](#performance-characteristics)
- [Migration Patterns](#migration-patterns)
- [Deprecation Strategy](#deprecation-strategy)
- [Best Practices](#best-practices)
- [Examples](#examples)

## Why Version APIs?

API versioning is essential for maintaining backward compatibility while evolving your API:

- **Breaking Changes**: Field removals, type changes, behavior modifications
- **New Features**: Adding required fields or changing validation rules
- **Client Control**: Clients upgrade at their own pace
- **Staged Rollouts**: Test new versions with subset of users
- **Documentation**: Clear contracts for each API version

## Versioning Strategies

Rivaas Router supports four industry-standard versioning methods:

### 1. Header-Based Versioning (Recommended)

Version specified in custom HTTP header:

```bash
curl -H 'API-Version: v2' https://api.example.com/users
```

**Advantages:**

- Clean URLs (no version pollution)
- Works with CDN caching
- Easy to route in reverse proxies
- Standard practice for modern APIs

**Use When:**

- Building public APIs
- RESTful services
- Microservice architectures

### 2. Query Parameter Versioning

Version specified as query parameter:

```bash
curl 'https://api.example.com/users?version=v2'
```

**Advantages:**

- Easy to test in browsers
- Simple to document
- No special header handling needed

**Use When:**

- Developer-friendly APIs
- Internal APIs with simple clients
- Testing and debugging scenarios

### 3. Path-Based Versioning

Version embedded in URL path:

```bash
curl https://api.example.com/v2/users
```

**Advantages:**

- Most visible and explicit
- Works with simple HTTP clients
- Easy routing in infrastructure

**Use When:**

- Very different API structures between versions
- Simple routing requirements
- Marketing wants version visibility

### 4. Accept Header Versioning (Content Negotiation)

Version in Accept header (RFC 7231):

```bash
curl -H 'Accept: application/vnd.myapi.v2+json' https://api.example.com/users
```

**Advantages:**

- Follows HTTP semantics
- Supports content negotiation
- Industry standard for hypermedia APIs

**Use When:**

- Hypermedia APIs (HATEOAS)
- Multiple content types per version
- Strict REST compliance

## Getting Started

### Basic Setup

```go
package main

import (
    "net/http"
    "time"
    
    "rivaas.dev/router"
)

func main() {
    r := router.New(
        router.WithVersioning(
            // Choose your versioning method(s)
            router.WithHeaderVersioning("API-Version"),
            
            // Set default for clients without version
            router.WithDefaultVersion("v2"),
            
            // Optional: Validate versions
            router.WithValidVersions("v1", "v2", "v3"),
        ),
    )
    
    // Version-specific routes
    v1 := r.Version("v1")
    v1.GET("/users", listUsersV1)
    
    v2 := r.Version("v2")
    v2.GET("/users", listUsersV2)
    
    http.ListenAndServe(":8080", r)
}
```

### Multiple Detection Methods

Enable multiple methods for flexibility:

```go
r := router.New(
    router.WithVersioning(
        router.WithHeaderVersioning("API-Version"),       // Primary
        router.WithQueryVersioning("version"),           // Fallback for testing
        router.WithPathVersioning("/v{version}/"),       // Legacy support
        router.WithAcceptVersioning("application/vnd.myapi.v{version}+json"),
        router.WithDefaultVersion("v2"),
    ),
)
```

**Detection Priority (first match wins):**

1. Custom detector (if configured)
2. Accept header
3. Path parameter
4. HTTP header
5. Query parameter
6. Default version

## Version Detection Methods

### Header-Based

```go
router.WithHeaderVersioning("API-Version")
```

Client usage:

```bash
curl -H 'API-Version: v2' https://api.example.com/users
```

### Query Parameter

```go
router.WithQueryVersioning("version")
```

Client usage:

```bash
curl 'https://api.example.com/users?version=v2'
```

### Path-Based

```go
router.WithPathVersioning("/v{version}/")
```

Routes automatically support path versions:

```go
// Accessed as /v2/users or /users (with header/query)
r.Version("v2").GET("/users", handler)
```

Client usage:

```bash
curl https://api.example.com/v2/users
```

### Accept Header

```go
router.WithAcceptVersioning("application/vnd.myapi.v{version}+json")
```

Client usage:

```bash
curl -H 'Accept: application/vnd.myapi.v2+json' https://api.example.com/users
```

### Custom Detector

For complex versioning logic:

```go
router.WithCustomVersionDetector(func(req *http.Request) string {
    // Custom logic - check auth, user agent, etc.
    if isLegacyClient(req) {
        return "v1"
    }
    return extractVersionSomehow(req)
})
```

## Performance Characteristics

### Benchmark Results

Version detection and routing add minimal overhead:

| Operation | Time/Request | Difference | Notes |
|-----------|--------------|------------|-------|
| Non-versioned route | 450ns | baseline | Single route table |
| Versioned route (header) | 490ns | +9% | One header lookup + map access |
| Versioned route (query) | 510ns | +13% | Parse query + map access |
| Versioned route (path) | 470ns | +4% | Path segment extraction |

### Performance Characteristics

**Version Detection:**

- Header lookup: ~30ns (single map access)
- Query parsing: ~50ns (lazy parse on first access)
- Path extraction: ~20ns (string slice operation)
- Accept parsing: ~100ns (regex match)

**Route Compilation:**

- Performed once at startup
- Uses lock-free `sync.Map` for version trees
- O(1) lookup for version-specific routes
- No regex matching—versions are exact strings

**Memory Overhead:**

- ~50 bytes per version configuration
- Separate route tree per version
- Compiled routes cached per version in `sync.Map`
- Bloom filters size: configurable (default 1000 routes)

**Concurrency:**

- Lock-free reads via atomic operations
- No mutex contention during request handling
- Safe concurrent version detection
- Version trees are immutable after compilation

### Optimization Tips

1. **Use Header Versioning**: Fastest detection method
2. **Limit Valid Versions**: Faster validation with smaller set
3. **Avoid Custom Detectors**: Unless necessary, they add overhead
4. **Share Code Between Versions**: Use helpers to reduce duplication
5. **Profile Before Optimizing**: Versioning overhead is usually negligible

## Migration Patterns

### Gradual Migration

Share business logic between versions:

```go
// Shared business logic
func getUserByID(id string) (*User, error) {
    // Database query, business rules, etc.
    return &User{ID: id, Name: "Alice"}, nil
}

// Version-specific response formatters
func listUsersV1(c *router.Context) {
    users, _ := getUsersFromDB()
    
    // V1 format: flat structure
    c.JSON(200, map[string]any{
        "users": users,
    })
}

func listUsersV2(c *router.Context) {
    users, _ := getUsersFromDB()
    
    // V2 format: with metadata
    c.JSON(200, map[string]any{
        "data": users,
        "meta": map[string]any{
            "total": len(users),
            "version": "v2",
        },
    })
}
```

### Breaking Change Migration

Example: Changing user email to required field

**Version 1 (Original):**

```go
type UserV1 struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email,omitempty"` // Optional
}
```

**Version 2 (Breaking Change):**

```go
type UserV2 struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"` // Required
}

func createUserV2(c *router.Context) {
    var user UserV2
    if err := c.Bind(&user); err != nil {
        c.JSON(400, map[string]string{
            "error": "validation failed",
            "detail": "email is required in API v2",
        })
        return
    }
    
    // Create user...
}
```

### Middleware Migration

Apply version-specific middleware:

```go
v1 := r.Version("v1")
v1.Use(legacyAuthMiddleware)
v1.GET("/users", listUsersV1)

v2 := r.Version("v2")
v2.Use(jwtAuthMiddleware)  // Different auth in v2
v2.GET("/users", listUsersV2)
```

### Schema Evolution

Example: Nested vs flat structure

```go
// V1: Flat structure
type UserV1 struct {
    ID      int    `json:"id"`
    Name    string `json:"name"`
    City    string `json:"city"`
    Country string `json:"country"`
}

// V2: Nested structure
type UserV2 struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
    Address struct {
        City    string `json:"city"`
        Country string `json:"country"`
    } `json:"address"`
}

// Conversion helper
func convertV1ToV2(v1 UserV1) UserV2 {
    v2 := UserV2{
        ID:   v1.ID,
        Name: v1.Name,
    }
    v2.Address.City = v1.City
    v2.Address.Country = v1.Country
    return v2
}
```

## Deprecation Strategy

### Marking Versions as Deprecated

```go
r := router.New(
    router.WithVersioning(
        // Mark v1 as deprecated with sunset date
        router.WithDeprecatedVersion(
            "v1",
            time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
        ),
        
        // Observability callbacks
        router.WithVersionObserver(
            router.WithOnDetected(func(version, method string) {
                // Track version usage in metrics
                metrics.RecordVersionUsage(version, method)
            }),
            router.WithOnMissing(func() {
                // Alert when no version specified
                log.Warn("client using default version")
            }),
            router.WithOnInvalid(func(attempted string) {
                // Track invalid version attempts
                metrics.RecordInvalidVersion(attempted)
            }),
        ),
    ),
)
```

### Deprecation Headers

Router automatically adds RFC 8594 headers for deprecated versions:

```http
Sunset: Wed, 31 Dec 2025 23:59:59 GMT
Deprecation: true
Link: <https://api.example.com/docs/migration>; rel="deprecation"
```

### Gradual Deprecation Process

**6 months before sunset:**

1. Announce deprecation in release notes
2. Add `Deprecation` header to v1 responses
3. Add migration guide to documentation
4. Contact top API consumers

**3 months before sunset:**

1. Add `Sunset` header with specific date
2. Send emails to active v1 users
3. Monitor v1 usage (should decline)
4. Offer migration assistance

**1 month before sunset:**

1. Final warning notifications
2. Return 410 Gone for deprecated endpoints
3. Redirect to migration guide

**After sunset:**

1. Remove v1 code and routes
2. Return 410 Gone with clear message
3. Keep migration documentation

### Monitoring Deprecation

```go
func trackVersionUsage(version, method string) {
    metrics.Counter("api_version_usage").
        WithLabel("version", version).
        WithLabel("method", method).
        Increment()
}

func alertOnDeprecatedUsage(version string) {
    if isDeprecated(version) {
        alerts.Send("Deprecated API version used", map[string]string{
            "version": version,
            "caller": requestContext.ClientID,
        })
    }
}
```

## Best Practices

### 1. Use Semantic Versioning for APIs

- **Major** (v1, v2, v3): Breaking changes
- **Minor** (v2.1, v2.2): Backward-compatible additions
- **Patch** (v2.1.1): Bug fixes (no API changes)

### 2. Version at the Right Granularity

**Don't version:**

- Bug fixes
- Performance improvements
- Internal refactoring
- Adding optional fields
- Relaxing validation

**Do version:**

- Removing fields
- Changing field types
- Required → optional or vice versa
- Changing behavior significantly
- Changing error codes/messages

### 3. Maintain Backward Compatibility

```go
// Good: Add optional field
type UserV2 struct {
    ID    int     `json:"id"`
    Name  string  `json:"name"`
    Email string  `json:"email,omitempty"` // New optional field
}

// Bad: Remove existing field (breaking change)
type UserV2 struct {
    ID   int    `json:"id"`
    // Name field removed - BREAKING!
}
```

### 4. Document Version Differences

Maintain clear API documentation for each version:

```markdown
## API Versions

### v2 (Current)
- Added `email` field (optional)
- Added `address` nested object
- Added PATCH support for partial updates

### v1 (Deprecated - Sunset 2025-12-31)
- Original API
- Limited to GET/POST/PUT/DELETE
- Flat structure only
```

### 5. Use Version Groups

Organize routes by version:

```go
v1 := r.Version("v1")
{
    v1.GET("/users", listUsersV1)
    v1.GET("/users/:id", getUserV1)
    v1.POST("/users", createUserV1)
}

v2 := r.Version("v2")
{
    v2.GET("/users", listUsersV2)
    v2.GET("/users/:id", getUserV2)
    v2.POST("/users", createUserV2)
    v2.PATCH("/users/:id", updateUserV2) // New in v2
}
```

### 6. Validate Versions

Reject invalid versions early:

```go
router.WithVersioning(
    router.WithValidVersions("v1", "v2", "v3", "beta"),
    router.WithVersionObserver(
        router.WithOnInvalid(func(attempted string) {
            // Log invalid version attempts
            log.Warn("invalid API version", "version", attempted)
        }),
    ),
)
```

### 7. Test All Versions

```go
func TestAPIVersions(t *testing.T) {
    r := setupRouter()
    
    tests := []struct{
        version string
        path    string
        want    int
    }{
        {"v1", "/users", 200},
        {"v2", "/users", 200},
        {"v3", "/users", 200},
        {"v99", "/users", 404}, // Invalid version
    }
    
    for _, tt := range tests {
        req := httptest.NewRequest("GET", tt.path, nil)
        req.Header.Set("API-Version", tt.version)
        
        w := httptest.NewRecorder()
        r.ServeHTTP(w, req)
        
        assert.Equal(t, tt.want, w.Code)
    }
}
```

## Examples

### Complete Example

See [router/examples/07-versioning](../router/examples/07-versioning/) for a comprehensive example demonstrating:

- All four versioning methods
- Version-specific handlers
- Migration patterns (v1 → v2 → v3)
- Deprecation warnings
- Observability callbacks
- Best practices

Run the example:

```bash
cd router/examples/07-versioning
go run main.go
```

Test different methods:

```bash
# Header-based
curl -H 'API-Version: v2' http://localhost:8080/users

# Query parameter
curl 'http://localhost:8080/users?version=v2'

# Path-based
curl http://localhost:8080/v2/users

# Accept header
curl -H 'Accept: application/vnd.myapi.v2+json' http://localhost:8080/users
```

### Real-World Pattern: Stripe-Style Versioning

```go
r := router.New(
    router.WithVersioning(
        router.WithHeaderVersioning("Stripe-Version"),
        router.WithDefaultVersion("2024-11-20"), // Date-based versions
        router.WithValidVersions(
            "2024-11-20",
            "2024-10-28",
            "2024-09-30",
        ),
    ),
)

// Version by date
v20241120 := r.Version("2024-11-20")
v20241120.GET("/charges", listCharges)
```

### Real-World Pattern: GitHub-Style Versioning

```go
r := router.New(
    router.WithVersioning(
        router.WithAcceptVersioning("application/vnd.github.v{version}+json"),
        router.WithDefaultVersion("v3"),
    ),
)

// Usage: Accept: application/vnd.github.v3+json
```

## Further Reading

- [RFC 7231 - Content Negotiation](https://tools.ietf.org/html/rfc7231#section-5.3)
- [RFC 8594 - Sunset Header](https://tools.ietf.org/html/rfc8594)
- [Semantic Versioning](https://semver.org/)
- [Microsoft API Versioning Guidelines](https://github.com/microsoft/api-guidelines/blob/vNext/Guidelines.md#12-versioning)
- [Roy Fielding on Versioning](https://www.infoq.com/articles/roy-fielding-on-versioning/)

## Contributing

Found an issue or have a suggestion? Please open an issue or pull request on GitHub.

## License

Copyright 2025 The Rivaas Authors. Licensed under the Apache License 2.0.
