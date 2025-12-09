# Rivaas

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/app.svg)](https://pkg.go.dev/rivaas.dev/app)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/rivaas.dev)](https://goreportcard.com/report/rivaas.dev)

A high-performance, modular web framework for Go with integrated observability.

| Metric | Value |
|--------|-------|
| Throughput | 6.5M+ req/sec |
| Latency | 155ns average |
| Memory | 16 bytes/request |

## Quick Start

```go
package main

import (
    "log"
    "net/http"
    "rivaas.dev/app"
)

func main() {
    a, err := app.New()
    if err != nil {
        log.Fatal(err)
    }

    a.GET("/", func(c *app.Context) {
        c.JSON(http.StatusOK, map[string]string{"message": "Hello from Rivaas!"})
    })

    a.Run(":8080")
}
```

**With observability:**

```go
a, err := app.New(
    app.WithServiceName("my-api"),
    app.WithObservability(
        app.WithLogging(logging.WithConsoleHandler()),
        app.WithMetrics(), // Prometheus on :9090/metrics
        app.WithTracing(tracing.WithStdout()),
    ),
)
```

See [full-featured example](./app/examples/02-full-featured/) for a complete production setup.

## Installation

```bash
go get rivaas.dev/app
```

Requires Go 1.25.0 or higher.

## Philosophy

The name Rivaas comes from **ریواس (Rivās)** — a [wild rhubarb plant](https://en.wikipedia.org/wiki/Rheum_ribes) native to the mountains of Iran.

This plant grows at high altitudes (1,500–3,000 meters) in harsh, rocky terrain where few other plants survive. It withstands extreme temperature swings, poor soil, and unpredictable weather — yet it thrives, providing food and medicine to mountain communities for centuries.

That's the philosophy behind Rivaas:

- **Resilient** — Built for production from day one, with graceful shutdown, health checks, and panic recovery
- **Lightweight** — Minimal overhead (155ns latency, 16 bytes/request) without sacrificing features
- **Adaptive** — Works locally, in containers, or across distributed systems with the same code
- **Self-sufficient** — Integrated observability (metrics, tracing, logging) instead of bolted-on dependencies

Like its namesake growing in the mountains, Rivaas is designed to thrive in dynamic, cloud-native environments — **lightweight yet powerful, modular yet simple**.

## Why Rivaas?

- **Production-Ready** — Graceful shutdown, health endpoints, panic recovery
- **High Performance** — 6.5M+ req/sec, 155ns latency, 16 bytes/request
- **Flexible** — Use `app` for batteries-included or `router` for full control
- **Cloud-Native** — OpenTelemetry-native with Prometheus, OTLP, Jaeger support
- **Modular** — Each package works standalone without the full framework

## Packages

### Core

| Package | Description | Docs |
|---------|-------------|------|
| [app](./app/) | Batteries-included web framework | [README](./app/README.md) |
| [router](./router/) | High-performance HTTP router | [README](./router/README.md) |

### Data Handling

| Package | Description | Docs |
|---------|-------------|------|
| [binding](./binding/) | Request binding (query, form, JSON, headers) | [doc.go](./binding/doc.go) |
| [validation](./validation/) | Struct validation with tags and JSON Schema | [doc.go](./validation/doc.go) |

### Observability

| Package | Description | Docs |
|---------|-------------|------|
| [logging](./logging/) | Structured logging with slog | [README](./logging/README.md) |
| [metrics](./metrics/) | OpenTelemetry metrics (Prometheus, OTLP) | [README](./metrics/README.md) |
| [tracing](./tracing/) | Distributed tracing with OpenTelemetry | [README](./tracing/README.md) |

### API & Errors

| Package | Description | Docs |
|---------|-------------|------|
| [openapi](./openapi/) | Automatic OpenAPI 3.0/3.1 generation | [README](./openapi/README.md) |
| [errors](./errors/) | Error formatting (RFC 9457, JSON:API) | [README](./errors/README.md) |

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     rivaas.dev/app                          │
│              Batteries-included framework                   │
├─────────────────────────────────────────────────────────────┤
│                    rivaas.dev/router                        │
│              High-performance HTTP router                   │
├───────────────┬───────────────┬─────────────────────────────┤
│   logging     │    metrics    │          tracing            │
│   binding     │   validation  │          openapi            │
│   errors      │               │                             │
└───────────────┴───────────────┴─────────────────────────────┘
```

Each package is independently usable. The `app` package integrates them with automatic service metadata propagation and lifecycle management.

## Configuration

```go
a, err := app.New(
    app.WithServiceName("my-api"),
    app.WithServiceVersion("v1.0.0"),
    app.WithObservability(
        app.WithLogging(logging.WithJSONHandler()),
        app.WithMetrics(),
        app.WithTracing(tracing.WithOTLP("localhost:4317")),
    ),
    app.WithHealthEndpoints(
        app.WithReadinessCheck("database", dbPingCheck),
    ),
)
```

See [App Documentation](./app/README.md) for complete configuration options.

## Middleware

Built-in production-ready middleware: `accesslog`, `recovery`, `cors`, `requestid`, `timeout`, `ratelimit`, `basicauth`, `bodylimit`, `compression`, `security`.

See [Middleware Documentation](./router/middleware/README.md) for usage and configuration.

## Examples

- **[App Examples](./app/examples/)** — Quick start and full-featured apps
- **[Router Examples](./router/examples/)** — Routing, middleware, versioning
- **[Logging Examples](./logging/examples/)** — Structured logging patterns
- **[Middleware Examples](./router/middleware/examples/)** — All middleware usage

## Performance

| Metric | Value |
|--------|-------|
| Throughput | 6.5M+ req/sec |
| Latency | 155ns average |
| Memory | 16 bytes/request |
| Allocations | 1 per request |

See [benchmarks](./router/benchmarks/) for detailed comparisons.

## Repository Structure

Multi-module repository — each package has its own `go.mod` and can be versioned independently.

```
rivaas/
├── app/          → rivaas.dev/app
├── router/       → rivaas.dev/router
├── binding/      → rivaas.dev/binding
├── validation/   → rivaas.dev/validation
├── logging/      → rivaas.dev/logging
├── metrics/      → rivaas.dev/metrics
├── tracing/      → rivaas.dev/tracing
├── openapi/      → rivaas.dev/openapi
├── errors/       → rivaas.dev/errors
└── go.work
```

## Documentation

| Resource | Description |
|----------|-------------|
| [App Guide](./app/README.md) | Framework documentation |
| [Router Guide](./router/README.md) | HTTP routing and binding |
| [Middleware](./router/middleware/README.md) | Middleware catalog |
| [Design Principles](./docs/DESIGN_PRINCIPLES.md) | Architecture decisions |
| [Testing Standards](./docs/TESTING_STANDARDS.md) | Testing guidelines |

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests for your changes
4. Ensure all tests pass (`go test ./...`)
5. Open a Pull Request

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.

---

Made with ❤️ for the Go community
