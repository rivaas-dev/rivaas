# Middleware Examples

This directory contains runnable examples demonstrating how to use rivaas/router middleware in real-world scenarios.

## 📋 Available Examples

### Security

- **[basic_auth/](basic_auth/)** - HTTP Basic Authentication with multiple user databases and skip paths
- **[security/](security/)** - Security headers (HSTS, CSP, X-Frame-Options, etc.)
- **[cors/](cors/)** - Cross-Origin Resource Sharing for APIs and web apps

### Observability

- **[logger/](logger/)** - Request logging with custom formats, JSON output, and request ID integration
- **[request_id/](request_id/)** - Request ID generation for distributed tracing

### Reliability

- **[recovery/](recovery/)** - Panic recovery with custom handlers and structured logging
- **[timeout/](timeout/)** - Request timeout handling
- **[body_limit/](body_limit/)** - Request body size limiting to prevent DoS attacks

### Performance

- **[compression/](compression/)** - Gzip/Deflate response compression with custom levels
- **[ratelimit/](ratelimit/)** - Token bucket rate limiting per IP or API key

## 🚀 Quick Start

Each example is a standalone Go program. To run any example:

```bash
cd router/middleware/examples/basic_auth
go run main.go
```

Then test the endpoints using curl or your browser.

## 💡 Example Structure

Each example demonstrates:

1. **Basic usage** - Simplest possible configuration
2. **Common patterns** - Real-world use cases
3. **Advanced configuration** - All available options
4. **Production setup** - Best practices for deployment

## 🔗 Middleware Combinations

Many examples show how to combine multiple middlewares effectively:

### Recommended Order

```go
r := router.New()

// 1. Request ID (first, for tracing)
r.Use(middleware.RequestID())

// 2. Logger (after request ID, before business logic)
r.Use(middleware.Logger())

// 3. Recovery (catch panics in all subsequent middleware)
r.Use(middleware.Recovery())

// 4. Security headers
r.Use(middleware.Security())
r.Use(middleware.CORS())

// 5. Body limit (prevent DoS attacks from large requests)
r.Use(middleware.BodyLimit())

// 6. Rate limiting (after logging, before business logic)
r.Use(middleware.RateLimit())

// 7. Timeout (before business logic)
r.Use(middleware.Timeout(30 * time.Second))

// 8. Business-specific middleware
r.Use(middleware.BasicAuth(...))
r.Use(middleware.Compression())
```

### Why This Order Matters

1. **RequestID first** - So all subsequent middleware can include it in logs
2. **Logger early** - To capture all request/response activity
3. **Recovery early** - To catch panics from middleware setup errors
4. **Security/CORS** - Applied globally before routing
5. **BodyLimit** - Before reading request bodies to prevent DoS attacks
6. **RateLimit** - Before expensive operations
7. **Timeout** - Before business logic that might hang
8. **Auth** - After security headers but before business logic
9. **Compression** - Last, as it modifies response bodies

## 📚 Full Documentation

See the main [middleware documentation](../) for:

- Complete API reference
- Configuration options
- Performance considerations
- Testing strategies

## 🧪 Testing Examples

Each example includes curl commands in its output. Generally:

```bash
# Run the example
go run main.go

# In another terminal, test it
curl http://localhost:8080/endpoint
```

## 🤝 Contributing

When adding new examples:

1. Follow the existing structure (basic → advanced)
2. Include curl commands for testing
3. Document all configuration options
4. Show both simple and production-ready setups
5. Use charmbracelet/log for consistent output formatting
6. Include helpful tips and warnings

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](../../LICENSE) file for details.
