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

### 02-full-featured

**What it shows:**
- **Complete production-ready application**
- Multi-exporter tracing (stdout for dev, OTLP for prod)
- Metrics collection with Prometheus
- Environment-based configuration
- Full middleware stack (logger, recovery, CORS, timeout, request ID)
- Custom metrics and tracing in handlers
- RESTful API patterns with route groups
- Graceful shutdown

**When to use:**
- Building production applications
- Need complete observability
- Want to see all features working together
- Reference implementation for your own projects

**Run in different modes:**
```bash
cd 02-full-featured

# Development mode (stdout tracing, logger enabled)
go run main.go

# Production mode (OTLP tracing to Jaeger)
ENVIRONMENT=production OTLP_ENDPOINT=localhost:4317 go run main.go

# Testing mode (no tracing)
ENVIRONMENT=testing go run main.go
```

**What you get:**
- HTTP API on `:8080`
- Prometheus metrics on `:9090`
- Traces to stdout or OTLP collector
- Request IDs on all responses
- Full observability stack

## Example Progression

1. **Start with 01** - Learn the basics
2. **Move to 02** - See production patterns
3. **Use 02 as a template** - For your own applications

## Key Differences

| Feature | 01-quick-start | 02-full-featured |
|---------|----------------|------------------|
| Lines of code | ~20 | ~250 |
| Observability | ❌ | ✅ Metrics + Tracing |
| Middleware | ❌ | ✅ Full stack |
| Environment modes | ❌ | ✅ Dev/Prod/Test |
| Configuration | ❌ | ✅ Env vars |
| Production ready | ❌ | ✅ |

## Running Requirements

### 01-quick-start
```bash
# No dependencies needed
go run main.go
```

### 02-full-featured

**Development mode:**
```bash
# Just run it - traces to stdout
go run main.go
```

**Production mode:**
```bash
# Start Jaeger first:
docker run -d -p 4317:4317 -p 16686:16686 jaegertracing/all-in-one:latest

# Then run the app:
ENVIRONMENT=production go run main.go

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

