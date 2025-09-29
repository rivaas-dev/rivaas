# Versioning Package

Comprehensive API versioning support for the Rivaas router with lifecycle management, deprecation warnings, and sunset enforcement.

## Features

- **Multiple Detection Strategies**: Path, header, query parameter, and Accept-header based versioning
- **Version Validation**: Whitelist allowed versions
- **Deprecation Warnings**: RFC 8594 Deprecation and Sunset headers
- **Sunset Enforcement**: Automatic 410 Gone responses for expired versions
- **Warning Headers**: RFC 7234 Warning: 299 for deprecated APIs
- **Link Headers**: Documentation URLs with rel=deprecation and rel=sunset
- **Version Headers**: Optional X-API-Version response header
- **Monitoring Hooks**: Async callbacks for metrics collection
- **High Performance**: Lock-free atomic operations, optimized fast paths

## Quick Start

```go
import (
    "time"
    "rivaas.dev/router"
    "rivaas.dev/router/versioning"
)

func main() {
    sunsetDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

    r := router.New(
        router.WithVersioning(
            versioning.WithPathVersioning("/v{version}/"),
            versioning.WithDefaultVersion("v2"),
            versioning.WithDeprecatedVersion("v1", sunsetDate),
            versioning.WithSunsetEnforcement(),
            versioning.WithVersionHeader(),
            versioning.WithWarning299(),
        ),
    )

    // Define version-specific routes
    r.Version("v1").GET("/users", handleUsersV1)
    r.Version("v2").GET("/users", handleUsersV2)

    r.Run(":8080")
}
```

## Version Detection Strategies

### Path-Based Versioning

Extract version from URL path:

```go
versioning.WithPathVersioning("/v{version}/")
// /v1/users -> version: v1
// /v2/users -> version: v2
```

### Header-Based Versioning

Extract version from HTTP header:

```go
versioning.WithHeaderVersioning("API-Version")
// Header: API-Version: v2 -> version: v2
```

### Query Parameter Versioning

Extract version from query string:

```go
versioning.WithQueryVersioning("v")
// /users?v=v2 -> version: v2
```

### Accept-Header Versioning (Content Negotiation)

Extract version from Accept header:

```go
versioning.WithAcceptVersioning("application/vnd.myapi.v{version}+json")
// Accept: application/vnd.myapi.v2+json -> version: v2
```

### Custom Version Detection

Implement custom version detection logic:

```go
versioning.WithCustomVersionDetector(func(req *http.Request) string {
    // Extract version from JWT token, custom header, etc.
    token := req.Header.Get("Authorization")
    return extractVersionFromToken(token)
})
```

## API Lifecycle Management

### Deprecation Warnings

Mark versions as deprecated with automatic headers:

```go
sunsetDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

versioning.New(
    versioning.WithPathVersioning("/v{version}/"),
    versioning.WithDeprecatedVersion("v1", sunsetDate),
    versioning.WithVersionHeader(),
    versioning.WithWarning299(),
    versioning.WithDeprecationLink("v1", "https://docs.example.com/migration/v1-to-v2"),
)
```

**Automatic headers for v1 requests:**

```http
HTTP/1.1 200 OK
Deprecation: true
Sunset: Sat, 31 Dec 2025 00:00:00 GMT
Link: <https://docs.example.com/migration/v1-to-v2>; rel="deprecation", rel="sunset"
Warning: 299 - "API v1 is deprecated and will be removed on 2025-12-31T00:00:00Z. Please upgrade to a supported version."
X-API-Version: v1
```

### Sunset Enforcement

Automatically return 410 Gone for versions past their sunset date:

```go
versioning.New(
    versioning.WithPathVersioning("/v{version}/"),
    versioning.WithDeprecatedVersion("v1", sunsetDate),
    versioning.WithSunsetEnforcement(),  // Return 410 Gone after sunset
)
```

**Response after sunset date:**

```http
HTTP/1.1 410 Gone
X-API-Version: v1
Sunset: Sat, 31 Dec 2025 00:00:00 GMT
Link: <https://docs.example.com/migration/v1-to-v2>; rel="sunset"

API v1 was removed. Please upgrade to a supported version.
```

### Monitoring and Metrics

Track deprecated API usage with async callbacks:

```go
versioning.New(
    versioning.WithPathVersioning("/v{version}/"),
    versioning.WithDeprecatedVersion("v1", sunsetDate),
    versioning.WithDeprecatedUseCallback(func(version, route string) {
        // Called asynchronously for each deprecated API request
        metrics.DeprecatedAPIUsage.WithLabels(version, route).Inc()
        log.Warn("deprecated API used",
            "version", version,
            "route", route,
            "sunset_date", sunsetDate)
    }),
)
```

The callback is invoked asynchronously (goroutine) to avoid blocking the request.

## Configuration Options

### Core Options

| Option | Description |
|--------|-------------|
| `WithPathVersioning(pattern)` | Enable path-based version detection |
| `WithHeaderVersioning(headerName)` | Enable header-based version detection |
| `WithQueryVersioning(paramName)` | Enable query parameter version detection |
| `WithAcceptVersioning(pattern)` | Enable Accept-header version detection |
| `WithDefaultVersion(version)` | Set fallback version when none detected |
| `WithValidVersions(...versions)` | Whitelist allowed versions |
| `WithCustomVersionDetector(fn)` | Custom version detection function |

### Lifecycle Management Options

| Option | Description |
|--------|-------------|
| `WithDeprecatedVersion(version, sunsetDate)` | Mark version as deprecated |
| `WithDeprecationLink(version, url)` | Set migration documentation URL |
| `WithSunsetEnforcement()` | Return 410 Gone for expired versions |
| `WithVersionHeader()` | Send X-API-Version response header |
| `WithWarning299()` | Send Warning: 299 header for deprecated versions |
| `WithDeprecatedUseCallback(fn)` | Async callback for monitoring |
| `WithClock(nowFn)` | Inject time function for testing |

### Observability Options

```go
versioning.WithObserver(
    versioning.WithOnDetected(func(version, method string) {
        metrics.RecordVersionUsage(version, method)
    }),
    versioning.WithOnMissing(func() {
        log.Warn("client using default version")
    }),
    versioning.WithOnInvalid(func(attempted string) {
        metrics.RecordInvalidVersion(attempted)
    }),
)
```

## Complete Example

```go
package main

import (
    "log/slog"
    "time"
    "rivaas.dev/router"
    "rivaas.dev/router/versioning"
)

func main() {
    // Define sunset dates
    v1SunsetDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

    // Create router with comprehensive versioning
    r := router.New(
        router.WithVersioning(
            // Detection strategy
            versioning.WithPathVersioning("/v{version}/"),
            versioning.WithDefaultVersion("v2"),
            versioning.WithValidVersions("v1", "v2", "v3"),

            // Lifecycle management
            versioning.WithDeprecatedVersion("v1", v1SunsetDate),
            versioning.WithDeprecationLink("v1", "https://docs.example.com/migration/v1-to-v2"),
            versioning.WithSunsetEnforcement(),

            // Response headers
            versioning.WithVersionHeader(),
            versioning.WithWarning299(),

            // Monitoring
            versioning.WithDeprecatedUseCallback(func(version, route string) {
                slog.Warn("deprecated API usage",
                    "version", version,
                    "route", route,
                    "action", "migrate_to_v2")
            }),

            // Observability
            versioning.WithObserver(
                versioning.WithOnDetected(func(version, method string) {
                    slog.Info("version detected",
                        "version", version,
                        "method", method)
                }),
                versioning.WithOnInvalid(func(attempted string) {
                    slog.Warn("invalid version attempted",
                        "version", attempted)
                }),
            ),
        ),
    )

    // v1 API (deprecated)
    v1 := r.Version("v1")
    v1.GET("/users", func(c *router.Context) {
        c.JSON(200, map[string]any{
            "version": "v1",
            "users":   []string{"alice", "bob"},
        })
    })
    v1.GET("/posts", func(c *router.Context) {
        c.JSON(200, map[string]any{
            "version": "v1",
            "posts":   []string{"post1", "post2"},
        })
    })

    // v2 API (current stable)
    v2 := r.Version("v2")
    v2.GET("/users", func(c *router.Context) {
        c.JSON(200, map[string]any{
            "version": "v2",
            "users": []map[string]any{
                {"id": 1, "name": "alice", "email": "alice@example.com"},
                {"id": 2, "name": "bob", "email": "bob@example.com"},
            },
        })
    })
    v2.GET("/posts", func(c *router.Context) {
        c.JSON(200, map[string]any{
            "version": "v2",
            "posts": []map[string]any{
                {"id": 1, "title": "Post 1", "author": "alice"},
                {"id": 2, "title": "Post 2", "author": "bob"},
            },
        })
    })

    // v3 API (experimental)
    v3 := r.Version("v3")
    v3.GET("/users", func(c *router.Context) {
        c.JSON(200, map[string]any{
            "version": "v3",
            "users": []map[string]any{
                {
                    "id":       1,
                    "name":     "alice",
                    "email":    "alice@example.com",
                    "avatar":   "https://example.com/avatars/alice.png",
                    "verified": true,
                },
            },
        })
    })

    slog.Info("server starting", "port", 8080)
    r.Run(":8080")
}
```

## Testing

### Deterministic Time Testing

Use `WithClock()` to inject a fake time source:

```go
func TestSunsetEnforcement(t *testing.T) {
    sunsetDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
    
    engine, _ := versioning.New(
        versioning.WithPathVersioning("/v{version}/"),
        versioning.WithDeprecatedVersion("v1", sunsetDate),
        versioning.WithSunsetEnforcement(),
        versioning.WithClock(func() time.Time {
            return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)  // After sunset
        }),
    )

    w := httptest.NewRecorder()
    isSunset := engine.SetLifecycleHeaders(w, "v1", "/api/users")
    
    assert.True(t, isSunset)  // Should return 410 Gone
}
```

## Performance

The versioning engine is optimized for production use:

- **Optimized fast paths** for common operations
- **Lock-free atomic operations** for version tree selection
- **O(1) map lookups** for deprecation checks
- **Async callbacks** for monitoring (non-blocking)
- **Compiled route tables** for versioned routes
- **Single-pass header parsing** for efficiency

## Standards and RFCs

This package implements:

- **RFC 8594**: HTTP Sunset Header - https://www.rfc-editor.org/rfc/rfc8594
- **RFC 7234**: HTTP Warning Header - https://www.rfc-editor.org/rfc/rfc7234#section-5.5
- **RFC 7231**: HTTP Accept Content Negotiation - https://www.rfc-editor.org/rfc/rfc7231#section-5.3.2

## Best Practices

1. **Always set a default version** for fallback behavior
2. **Use semantic versioning** (v1, v2, v3)
3. **Document migration paths** in deprecation links
4. **Give clients 6-12 months** between deprecation and sunset
5. **Monitor deprecated API usage** to track migration progress
6. **Test sunset behavior** before enforcement date
7. **Use version headers** to help clients debug issues
8. **Avoid breaking changes** within the same major version

## Migration from Middleware

If you were previously using `router/middleware/versioning`, this package provides all the same functionality in a more integrated way:

**Old approach (middleware):**
```go
r := router.New(
    router.WithVersioning(versioning.WithPathVersioning("/v{version}/")),
)
r.Use(middleware_versioning.WithVersioning(middleware_versioning.Options{
    Versions: []middleware_versioning.VersionInfo{
        {Version: "v1", Deprecated: &deprecatedDate, Sunset: &sunsetDate, DocsURL: docURL},
    },
    EnforceSunset: true,
}))
```

**New approach (integrated):**
```go
r := router.New(
    router.WithVersioning(
        versioning.WithPathVersioning("/v{version}/"),
        versioning.WithDeprecatedVersion("v1", sunsetDate),
        versioning.WithDeprecationLink("v1", docURL),
        versioning.WithSunsetEnforcement(),
        versioning.WithVersionHeader(),
        versioning.WithWarning299(),
    ),
)
```

Benefits of the integrated approach:
- ✅ No middleware overhead
- ✅ Headers set during routing (earlier in request lifecycle)
- ✅ Cleaner configuration
- ✅ Better performance (integrated checks)
- ✅ Single source of truth for versioning logic

## License

Apache License 2.0 - See LICENSE file for details.

