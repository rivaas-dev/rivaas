# Middleware

Production-ready HTTP middleware for rivaas/router. Each middleware is provided in its own sub-package with comprehensive configuration options, examples, and tests.

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

## Package Structure

Each middleware package typically contains:

```
middleware/
├── accesslog/
│   ├── accesslog.go              # Implementation
│   ├── accesslog_test.go         # Unit tests
│   ├── accesslog_bench_test.go   # Benchmarks
│   ├── accesslog_integration_suite_test.go
│   ├── integration_test.go      # Integration tests (some packages)
│   ├── options.go                # Configuration options
│   ├── doc.go                    # Package documentation
│   ├── go.mod                    # Module definition
│   └── example/                  # Runnable example (most packages)
│       ├── main.go
│       └── go.mod
├── basicauth/
│   └── ...
└── ...
```

Some packages have extra files (for example, `ratelimit` has `stores.go` for rate limit storage backends). Not every middleware has an `options.go` (e.g. `trailingslash` uses defaults only).

## Examples

Each middleware package includes a runnable example in an `example/` subdirectory.

```bash
# Run the BasicAuth example
cd basicauth/example
go run main.go

# Run the AccessLog example
cd accesslog/example
go run main.go
```

Each example shows basic usage, common patterns, and how to configure the middleware.

## Testing

Each middleware has unit tests; some also have integration and benchmark tests:

```bash
# Run all middleware tests
go test ./...

# Run tests for a specific middleware
go test ./basicauth/...
go test ./accesslog/...
```

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
r.Use(basicauth.New())       // 8. Authentication
r.Use(compression.New())     // 9. Compression (last)
```

## Learn More

- **[Middleware Guide](https://rivaas.dev/docs/guides/router/middleware/)** - Complete usage guide
- **Individual READMEs** - Each middleware has its own README in its package directory (e.g. [accesslog/README.md](accesslog/README.md))

## Contributing

When adding new middleware:

1. Create a new subdirectory under `middleware/`
2. Follow the existing pattern: `name.go`, `options.go`, `doc.go`, `name_test.go`, `name_bench_test.go`
3. Use functional options for configuration
4. Include unit tests and benchmarks; add integration tests if needed
5. Add a runnable example in an `example/` subdirectory
6. Create its own `go.mod` for independent versioning
7. Add a README.md in the package and update this README and the documentation site

## License

Apache License 2.0 - see [LICENSE](../../LICENSE) for details.
