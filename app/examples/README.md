# Rivaas App Examples

This directory contains examples demonstrating the Rivaas app package features.

## Available Examples

### 01-quick-start

**What it shows:**
- Minimal setup to get started quickly
- Basic routing and JSON responses
- Default configuration

**When to use:**
- Learning the basics
- Quick prototyping
- Simplest possible setup

**Run it:**
```bash
cd 01-quick-start
go run main.go
```

### 02-blog

**What it shows:**
- **Real-world blog API** with posts, authors, and comments
- **Configuration management** with `rivaas.dev/config` (YAML + env vars)
- **Method-based validation** (proper enum handling with `IsValid()`)
- **OpenAPI documentation** with Swagger UI
- **Full observability** (logging, metrics, tracing)
- **API versioning** with path-based routing (`/v1/stats`)
- **Integration tests** using `app/testing.go`
- **Production patterns** (slug-based URLs, status transitions, nested resources)

**When to use:**
- Building real-world RESTful APIs
- Need configuration management
- Want to see proper validation patterns
- Looking for comprehensive testing examples
- Reference implementation for your own projects

**Run it:**
```bash
cd 02-blog

# Development mode (default)
go run main.go

# Production mode with custom config
BLOG_SERVER_PORT=3000 BLOG_OBSERVABILITY_ENVIRONMENT=production go run main.go

# Run tests
go test -v
```

**What you get:**
- HTTP API on `:8080`
- OpenAPI docs at `/docs`
- Prometheus metrics on `:9090`
- Health endpoints: `/healthz`, `/readyz`
- Versioned API: `/v1/stats`, `/v1/popular`
- Full CRUD operations with validation

## Example Progression

1. **Start with 01** - Learn the basics
2. **Move to 02** - See production patterns
3. **Use 02 as a template** - For your own applications

## Key Differences

| Feature | 01-quick-start | 02-blog |
|---------|----------------|---------|
| Lines of code | ~20 | ~800 |
| Configuration | ❌ | ✅ YAML + Env vars |
| Validation | ❌ | ✅ Method-based |
| OpenAPI Docs | ❌ | ✅ Swagger UI |
| Observability | ❌ | ✅ Metrics + Tracing + Logging |
| Middleware | ❌ | ✅ Full stack |
| Testing | ❌ | ✅ Integration tests |
| API Versioning | ❌ | ✅ Path-based |
| Production ready | ❌ | ✅ |

## Running Requirements

### 01-quick-start
```bash
# No dependencies needed
go run main.go
```

### 02-blog

**Development mode:**
```bash
# Just run it - uses config.yaml
go run main.go

# Access the API:
# - API: http://localhost:8080
# - Docs: http://localhost:8080/docs
# - Metrics: http://localhost:9090/metrics
```

**Production mode:**
```bash
# Override with environment variables
BLOG_SERVER_PORT=3000 \
BLOG_OBSERVABILITY_ENVIRONMENT=production \
BLOG_OBSERVABILITY_SAMPLERATE=0.1 \
go run main.go
```

**With external services:**
```bash
# Start Jaeger for tracing:
docker run -d -p 4317:4317 -p 16686:16686 jaegertracing/all-in-one:latest

# Run with OTLP tracing:
BLOG_OBSERVABILITY_ENVIRONMENT=production OTLP_ENDPOINT=localhost:4317 go run main.go

# View traces at: http://localhost:16686
```

## Next Steps

After trying these examples:

1. Read the [App Package Documentation](../README.md)
2. Explore [Metrics Package](../../metrics/README.md)
3. Explore [Tracing Package](../../tracing/README.md)
4. Check out [Router Examples](../../router/examples/README.md)

## Design Philosophy

These examples follow the Rivaas design philosophy:

- **Simple things are simple** (Example 01)
- **Complex things are possible** (Example 02)
- **No magic** - Everything is explicit and configurable
- **Production ready** - Real-world patterns, not toys

