# Middleware

Production-ready HTTP middleware for rivaas/router. Each middleware is provided in its own sub-package with comprehensive configuration options, examples, and tests.

## 📦 Available Middlewares

### Security

| Middleware | Purpose | Package |
|------------|---------|---------|
| **Security** | Sets security headers (HSTS, CSP, X-Frame-Options, etc.) | [`security/`](security/) |
| **CORS** | Cross-Origin Resource Sharing configuration | [`cors/`](cors/) |
| **BasicAuth** | HTTP Basic Authentication | [`basicauth/`](basicauth/) |

### Observability

| Middleware | Purpose | Package |
|------------|---------|---------|
| **AccessLog** | Structured HTTP access logging with sampling and filtering | [`accesslog/`](accesslog/) |
| **RequestID** | Request ID generation and tracking for distributed tracing | [`requestid/`](requestid/) |

### Reliability

| Middleware | Purpose | Package |
|------------|---------|---------|
| **Recovery** | Panic recovery with graceful error handling | [`recovery/`](recovery/) |
| **Timeout** | Request timeout handling | [`timeout/`](timeout/) |
| **RateLimit** | Token bucket rate limiting per client | [`ratelimit/`](ratelimit/) |
| **BodyLimit** | Request body size limiting to prevent DoS attacks | [`bodylimit/`](bodylimit/) |

### Performance

| Middleware | Purpose | Package |
|------------|---------|---------|
| **Compression** | Gzip/Deflate response compression | [`compression/`](compression/) |

## 🚀 Quick Start

### Basic Setup

```go
package main

import (
    "log/slog"
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
    
    r.Run(":8080")
}
```

### Production Setup

```go
package main

import (
    "log/slog"
    "os"
    "time"
    
    "rivaas.dev/router"
    "rivaas.dev/router/middleware/accesslog"
    "rivaas.dev/router/middleware/basicauth"
    "rivaas.dev/router/middleware/bodylimit"
    "rivaas.dev/router/middleware/compression"
    "rivaas.dev/router/middleware/cors"
    "rivaas.dev/router/middleware/ratelimit"
    "rivaas.dev/router/middleware/recovery"
    "rivaas.dev/router/middleware/requestid"
    "rivaas.dev/router/middleware/security"
    "rivaas.dev/router/middleware/timeout"
)

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    r := router.New()
    
    // Observability (apply first)
    r.Use(requestid.New())
    r.Use(accesslog.New(
        accesslog.WithLogger(logger),
        accesslog.WithExcludePaths("/health", "/metrics"),
    ))
    
    // Reliability
    r.Use(recovery.New())
    r.Use(timeout.New(timeout.WithDuration(30 * time.Second)))
    
    // Security
    r.Use(security.New())
    r.Use(cors.New(
        cors.WithAllowedOrigins("https://example.com"),
        cors.WithAllowedMethods("GET", "POST", "PUT", "DELETE"),
    ))
    
    // Rate limiting and body limits
    r.Use(ratelimit.New(
        ratelimit.WithRequestsPerSecond(1000),
        ratelimit.WithBurst(100),
        ratelimit.WithLogger(logger),
    ))
    r.Use(bodylimit.New(bodylimit.WithLimit(10 * 1024 * 1024))) // 10MB
    
    // Performance
    r.Use(compression.New(compression.WithLogger(logger)))
    
    // Routes
    r.GET("/public", publicHandler)
    
    // Protected routes with BasicAuth
    admin := r.Group("/admin")
    admin.Use(basicauth.New(
        basicauth.WithCredentials("admin", "secret"),
    ))
    admin.GET("/dashboard", adminHandler)
    
    r.Run(":8080")
}
```

## 🔗 Middleware Ordering

The order in which middleware is applied matters. Here's the recommended order:

```go
r := router.New()

// 1. Request ID - Generate early for logging/tracing
r.Use(requestid.New())

// 2. AccessLog - Log all requests including failed ones
r.Use(accesslog.New())

// 3. Recovery - Catch panics from all other middleware
r.Use(recovery.New())

// 4. Security/CORS - Set security headers early
r.Use(security.New())
r.Use(cors.New())

// 5. Body Limit - Reject large requests before processing
r.Use(bodylimit.New())

// 6. Rate Limit - Reject excessive requests before processing
r.Use(ratelimit.New())

// 7. Timeout - Set time limits for downstream processing
r.Use(timeout.New())

// 8. Authentication - Verify identity after rate limiting
r.Use(basicauth.New())

// 9. Compression - Compress responses (last)
r.Use(compression.New())

// 10. Your application routes
r.GET("/", handler)
```

### Why This Order?

1. **RequestID first** - Generates a unique ID that other middleware can use in logs
2. **Logger early** - Captures all activity including errors from other middleware
3. **Recovery early** - Catches panics to prevent crashes
4. **Security/CORS** - Applies security policies before business logic
5. **BodyLimit** - Prevents reading excessive request bodies (DoS protection)
6. **RateLimit** - Blocks excessive requests before expensive operations
7. **Timeout** - Sets deadlines for request processing
8. **BasicAuth** - Authenticates after rate limiting but before business logic
9. **Compression** - Compresses response bodies (should be last)

## 📚 Configuration Examples

Each middleware supports extensive configuration through functional options. See individual package documentation for details.

### AccessLog with Custom Configuration

```go
import "log/slog"

logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
r.Use(accesslog.New(
    accesslog.WithLogger(logger),
    accesslog.WithExcludePaths("/health", "/metrics"),
    accesslog.WithSampleRate(0.1), // Log 10% of requests
    accesslog.WithSlowThreshold(500 * time.Millisecond),
))
```

### CORS with Multiple Origins

```go
r.Use(cors.New(
    cors.WithAllowedOrigins("https://app.example.com", "https://admin.example.com"),
    cors.WithAllowedMethods("GET", "POST", "PUT", "DELETE"),
    cors.WithAllowedHeaders("Content-Type", "Authorization"),
    cors.WithExposedHeaders("X-Request-ID"),
    cors.WithAllowCredentials(true),
    cors.WithMaxAge(3600),
))
```

### Rate Limiting by Custom Key

```go
r.Use(ratelimit.New(
    ratelimit.WithRequestsPerSecond(100),
    ratelimit.WithBurst(20),
    ratelimit.WithKeyFunc(func(c *router.Context) string {
        // Rate limit by API key instead of IP
        return c.Request.Header.Get("X-API-Key")
    }),
))
```

## 📖 Context Values

Some middleware stores values in the request context for use by handlers or other middleware:

```go
import (
    "rivaas.dev/router/middleware/requestid"
    "rivaas.dev/router/middleware/basicauth"
)

func handler(c *router.Context) {
    // Get request ID
    id := requestid.Get(c)
    
    // Get authenticated username
    username := basicauth.GetUsername(c)
    
    c.JSON(200, map[string]string{
        "request_id": id,
        "user": username,
    })
}
```

## 🎯 Examples

Complete runnable examples demonstrating real-world usage are available in the [`examples/`](examples/) directory:

- **[Basic Auth](examples/basic_auth/)** - Authentication with multiple users
- **[CORS](examples/cors/)** - Cross-origin configuration for APIs
- **[AccessLog](examples/logger/)** - Structured access logging with sampling
- **[Rate Limit](examples/ratelimit/)** - Per-client rate limiting
- **[Recovery](examples/recovery/)** - Panic recovery with custom handlers
- **[Request ID](examples/request_id/)** - Request tracking for distributed systems
- **[Security](examples/security/)** - Security headers configuration
- **[Timeout](examples/timeout/)** - Request timeout handling
- **[Body Limit](examples/body_limit/)** - Request size limiting
- **[Compression](examples/compression/)** - Response compression

Each example includes curl commands for testing.

## 🧪 Testing

All middleware includes comprehensive unit tests. To run tests:

```bash
# Test all middleware
go test ./...

# Test specific middleware
go test ./accesslog

# With coverage
go test -cover ./...

# With race detector
go test -race ./...
```

## 🔒 Thread Safety

All middlewares are safe for concurrent use. Rate limiting and other stateful middleware use internal synchronization to handle concurrent requests safely.

## ⚡ Performance

Middleware is designed for minimal overhead:

- **Efficient memory usage** in hot paths where possible
- **Optimized string operations** and buffer reuse
- **Efficient rate limiting** using token bucket algorithm
- **Fast header manipulation** for CORS and Security

Benchmark results are included in each middleware's test file.

## 📄 API Documentation

For detailed API documentation, see:

- Individual package godoc (e.g., `go doc rivaas.dev/router/middleware/accesslog`)
- [pkg.go.dev](https://pkg.go.dev/rivaas.dev/router/middleware)
- Package-level `doc.go` files in each subdirectory

## 🤝 Contributing

When adding new middleware:

1. Create a new subdirectory under `middleware/`
2. Follow the existing pattern: `middleware.go`, `options.go`, `middleware_test.go`
3. Use functional options for configuration
4. Include comprehensive tests and benchmarks
5. Add examples to the `examples/` directory
6. Update this README with the new middleware
7. Document all exported types and functions

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](../../LICENSE) file for details.
