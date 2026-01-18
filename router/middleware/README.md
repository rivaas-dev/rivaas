# Middleware

Production-ready HTTP middleware for rivaas/router. Each middleware is provided in its own sub-package with comprehensive configuration options.

> **📚 Full Documentation:** For comprehensive middleware documentation, see the [Middleware Guide](https://rivaas.dev/docs/guides/router/middleware/) and [Middleware Reference](https://rivaas.dev/docs/reference/packages/router/middleware/).

## Available Middlewares

### Security

- **[Security](security/)** - Security headers (HSTS, CSP, X-Frame-Options, etc.)
- **[CORS](cors/)** - Cross-Origin Resource Sharing
- **[BasicAuth](basicauth/)** - HTTP Basic Authentication

### Observability

- **[AccessLog](accesslog/)** - Structured HTTP access logging
- **[RequestID](requestid/)** - Request ID generation and tracking

### Reliability

- **[Recovery](recovery/)** - Panic recovery with graceful error handling
- **[Timeout](timeout/)** - Request timeout handling
- **[RateLimit](ratelimit/)** - Token bucket rate limiting
- **[BodyLimit](bodylimit/)** - Request body size limiting

### Performance

- **[Compression](compression/)** - Gzip/Deflate response compression

### Other

- **[MethodOverride](methodoverride/)** - HTTP method override
- **[TrailingSlash](trailingslash/)** - Trailing slash redirect

## Quick Start

```go
package main

import (
    "log/slog"
    "net/http"
    "os"
    
    "rivaas.dev/router"
    "rivaas.dev/router/middleware/accesslog"
    "rivaas.dev/router/middleware/recovery"
    "rivaas.dev/router/middleware/requestid"
)

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    r := router.New()
    
    // Apply middleware globally
    r.Use(requestid.New())
    r.Use(accesslog.New(accesslog.WithLogger(logger)))
    r.Use(recovery.New())
    
    r.GET("/", func(c *router.Context) {
        c.String(200, "Hello, World!")
    })
    
    http.ListenAndServe(":8080", r)
}
```

## Documentation

- **[Middleware Guide](https://rivaas.dev/docs/guides/router/middleware/)** - Usage patterns and best practices
- **[Middleware Reference](https://rivaas.dev/docs/reference/packages/router/middleware/)** - Complete API reference
- **[Examples](examples/)** - Working examples for each middleware

## Middleware Ordering

Recommended middleware order:

```go
r := router.New()

r.Use(requestid.New())       // 1. Request ID
r.Use(accesslog.New())       // 2. AccessLog
r.Use(recovery.New())        // 3. Recovery
r.Use(security.New())        // 4. Security/CORS
r.Use(cors.New())            
r.Use(bodylimit.New())       // 5. Body Limit
r.Use(ratelimit.New())       // 6. Rate Limit
r.Use(timeout.New())         // 7. Timeout
r.Use(auth.New())            // 8. Authentication
r.Use(compression.New())     // 9. Compression (last)
```

## Learn More

- **[Middleware Guide](https://rivaas.dev/docs/guides/router/middleware/)** - Complete usage guide
- **[Individual READMEs](.)** - Each middleware has its own README with examples
- **[Examples Directory](examples/)** - Working examples for all middleware

## Contributing

When adding new middleware:

1. Create a new subdirectory under `middleware/`
2. Follow the existing pattern: `middleware.go`, `options.go`, `middleware_test.go`
3. Use functional options for configuration
4. Include comprehensive tests and benchmarks
5. Add examples to the `examples/` directory
6. Update this README and the documentation site

## License

Apache License 2.0 - see [LICENSE](../../LICENSE) for details.
