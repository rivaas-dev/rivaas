<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./docs/images/logo-alpine-light.svg">
    <source media="(prefers-color-scheme: light)" srcset="./docs/images/logo-alpine-dark.svg">
    <img src="./docs/images/logo-alpine.svg" alt="Rivaas" width="200">
  </picture>
  <h1>Rivaas</h1>
  <p><strong>High-performance Go API framework with built-in observability</strong></p>
</div>

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![CI](https://github.com/rivaas-dev/rivaas/actions/workflows/ci.yml/badge.svg)](https://github.com/rivaas-dev/rivaas/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/rivaas-dev/rivaas/branch/main/graph/badge.svg)](https://codecov.io/gh/rivaas-dev/rivaas)

**[Website](https://rivaas.dev)** | **[Documentation](https://rivaas.dev/docs/)** | **[Blog](https://rivaas.dev/blog/)** | **[Getting Started](https://rivaas.dev/docs/getting-started/)**

> A batteries-included, cloud-native web framework for Go featuring **high-performance routing**, comprehensive request binding and validation, automatic OpenAPI generation, and OpenTelemetry-native observability.

We run benchmarks on every router release. For the latest numbers and how we measure them, see [Router Performance](https://rivaas.dev/docs/reference/packages/router/performance/).

## Why Rivaas?

- **Production-Ready** — Graceful shutdown, health endpoints, panic recovery, mTLS
- **High Performance** — Radix tree router with Bloom filter optimization
- **Flexible** — Use `app` for batteries-included or `router` for full control
- **Cloud-Native** — OpenTelemetry-native with Prometheus, OTLP, Jaeger support
- **Modular** — Each package works standalone without the full framework

## Quick Start

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"

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

    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    if err := a.Start(ctx); err != nil {
        log.Fatal(err)
    }
}
```

For HTTPS or mTLS, use `app.WithTLS` or `app.WithMTLS` at construction, then `Start(ctx)`. Default port is 8080 for HTTP and 8443 for TLS/mTLS (override with `app.WithPort(n)`).

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

See [full-featured example](./app/examples/02-blog/) for a complete production setup.

## Installation

```bash
go get rivaas.dev/app
```

Requires Go 1.25+

## Philosophy

The name Rivaas comes from **ریواس (Rivās)** — a [wild rhubarb plant](https://en.wikipedia.org/wiki/Rheum_ribes) native to the mountains of Iran.

This plant grows at high altitudes (1,500–3,000 meters) in harsh, rocky terrain where few other plants survive. It withstands extreme temperature swings, poor soil, and unpredictable weather — yet it thrives, providing food and medicine to mountain communities for centuries.

That's the philosophy behind Rivaas:

| Principle              | Description                                                                                                                                             |
|------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------|
| 🛡️ **Resilient**      | Built for production with graceful shutdown, health checks, and panic recovery                                                                          |
| ⚡ **Lightweight**      | Minimal overhead without sacrificing features — see [Performance](https://rivaas.dev/docs/reference/packages/router/performance/) for latest benchmarks |
| 🔧 **Adaptive**        | Works locally, in containers, or across distributed systems with the same code                                                                          |
| 📦 **Self-sufficient** | Integrated observability (metrics, tracing, logging) instead of bolted-on dependencies                                                                  |

Like its namesake growing in the mountains, Rivaas is designed to thrive in dynamic, cloud-native environments — **lightweight yet powerful, modular yet simple**.

## Packages

### Core

| Package            | Description                      | Docs                                                                                                                                                                                                                        |
|--------------------|----------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [app](./app)       | Batteries-included web framework | [![Go Reference](https://pkg.go.dev/badge/rivaas.dev/app.svg)](https://pkg.go.dev/rivaas.dev/app) [![Go Report](https://goreportcard.com/badge/rivaas.dev/app)](https://goreportcard.com/report/rivaas.dev/app)             |
| [router](./router) | High-performance HTTP router     | [![Go Reference](https://pkg.go.dev/badge/rivaas.dev/router.svg)](https://pkg.go.dev/rivaas.dev/router) [![Go Report](https://goreportcard.com/badge/rivaas.dev/router)](https://goreportcard.com/report/rivaas.dev/router) |

### Configuration

| Package            | Description                                                    | Docs                                                                                                                                                                                                                        |
|--------------------|----------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [config](./config) | Configuration management (files, env vars, Consul, validation) | [![Go Reference](https://pkg.go.dev/badge/rivaas.dev/config.svg)](https://pkg.go.dev/rivaas.dev/config) [![Go Report](https://goreportcard.com/badge/rivaas.dev/config)](https://goreportcard.com/report/rivaas.dev/config) |

### Data Handling

| Package                    | Description                                                             | Docs                                                                                                                                                                                                                                        |
|----------------------------|-------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [binding](./binding)       | Request binding (query, form, JSON, headers, XML, YAML, MsgPack, Proto) | [![Go Reference](https://pkg.go.dev/badge/rivaas.dev/binding.svg)](https://pkg.go.dev/rivaas.dev/binding) [![Go Report](https://goreportcard.com/badge/rivaas.dev/binding)](https://goreportcard.com/report/rivaas.dev/binding)             |
| [validation](./validation) | Struct validation with tags and JSON Schema                             | [![Go Reference](https://pkg.go.dev/badge/rivaas.dev/validation.svg)](https://pkg.go.dev/rivaas.dev/validation) [![Go Report](https://goreportcard.com/badge/rivaas.dev/validation)](https://goreportcard.com/report/rivaas.dev/validation) |

### Observability

| Package              | Description                                | Docs                                                                                                                                                                                                                            |
|----------------------|--------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [logging](./logging) | Structured logging with slog               | [![Go Reference](https://pkg.go.dev/badge/rivaas.dev/logging.svg)](https://pkg.go.dev/rivaas.dev/logging) [![Go Report](https://goreportcard.com/badge/rivaas.dev/logging)](https://goreportcard.com/report/rivaas.dev/logging) |
| [metrics](./metrics) | OpenTelemetry metrics (Prometheus, OTLP)   | [![Go Reference](https://pkg.go.dev/badge/rivaas.dev/metrics.svg)](https://pkg.go.dev/rivaas.dev/metrics) [![Go Report](https://goreportcard.com/badge/rivaas.dev/metrics)](https://goreportcard.com/report/rivaas.dev/metrics) |
| [tracing](./tracing) | Distributed tracing (OTLP, Jaeger, stdout) | [![Go Reference](https://pkg.go.dev/badge/rivaas.dev/tracing.svg)](https://pkg.go.dev/rivaas.dev/tracing) [![Go Report](https://goreportcard.com/badge/rivaas.dev/tracing)](https://goreportcard.com/report/rivaas.dev/tracing) |

### API & Errors

| Package              | Description                                          | Docs                                                                                                                                                                                                                            |
|----------------------|------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [openapi](./openapi) | Automatic OpenAPI 3.0/3.1 generation with Swagger UI | [![Go Reference](https://pkg.go.dev/badge/rivaas.dev/openapi.svg)](https://pkg.go.dev/rivaas.dev/openapi) [![Go Report](https://goreportcard.com/badge/rivaas.dev/openapi)](https://goreportcard.com/report/rivaas.dev/openapi) |
| [errors](./errors)   | Error formatting (RFC 9457, JSON:API)                | [![Go Reference](https://pkg.go.dev/badge/rivaas.dev/errors.svg)](https://pkg.go.dev/rivaas.dev/errors) [![Go Report](https://goreportcard.com/badge/rivaas.dev/errors)](https://goreportcard.com/report/rivaas.dev/errors)     |

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
│   errors      │    config     │                             │
└───────────────┴───────────────┴─────────────────────────────┘
```

Each package is independently usable with its own `go.mod`. The `app` package integrates them with automatic service metadata propagation and lifecycle management.

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

Twelve production-ready middleware included: `accesslog`, `recovery`, `cors`, `requestid`, `timeout`, `ratelimit`, `basicauth`, `bodylimit`, `compression`, `security`, `methodoverride`, `trailingslash`.

→ [Middleware Catalog](./middleware/README.md)

## Examples

| Directory                            | Description                                                     |
|--------------------------------------|-----------------------------------------------------------------|
| [App Examples](./app/examples)       | Quick start and full-featured apps                              |
| [Router Examples](./router/examples) | Routing, middleware, versioning, static files                   |
| [Middleware Examples](./middleware)  | Each package has an `example/` subdirectory with runnable demos |

## Performance

We benchmark the router on every release. For the latest numbers and comparisons with Gin, Echo, Chi, Fiber, and others, see [Router Performance](https://rivaas.dev/docs/reference/packages/router/performance/). To run the benchmarks yourself, see [router/benchmarks](./router/benchmarks/).

## Repository Structure

Multi-module repository — each package has its own `go.mod` and can be versioned independently.

```
rivaas/
├── app/          → rivaas.dev/app
├── middleware/   → rivaas.dev/middleware/*
├── router/       → rivaas.dev/router
├── binding/      → rivaas.dev/binding
├── validation/   → rivaas.dev/validation
├── config/       → rivaas.dev/config
├── logging/      → rivaas.dev/logging
├── metrics/      → rivaas.dev/metrics
├── tracing/      → rivaas.dev/tracing
├── openapi/      → rivaas.dev/openapi
├── errors/       → rivaas.dev/errors
└── go.work       → workspace configuration
```

## Documentation

| Resource                                                                     | Description                       |
|------------------------------------------------------------------------------|-----------------------------------|
| [App Guide](https://rivaas.dev/docs/guides/app/)                             | Complete framework documentation  |
| [Router Guide](https://rivaas.dev/docs/guides/router/)                       | HTTP routing and request handling |
| [Config Guide](https://rivaas.dev/docs/guides/config/)                       | Configuration management          |
| [Middleware Catalog](https://rivaas.dev/docs/guides/router/middleware/)      | All 12 middleware with examples   |
| [Design Principles](https://rivaas.dev/docs/about/design-principles/)        | Architecture and design decisions |
| [Testing Standards](https://rivaas.dev/docs/contributing/testing-standards/) | Testing guidelines and patterns   |

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests for your changes
4. Ensure all tests pass (`go test ./...`)
5. Open a Pull Request

## License

Apache License 2.0 – see [LICENSE](LICENSE) for details.

---

Made with ❤️ for the Go community
