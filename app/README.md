# App

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/app.svg)](https://pkg.go.dev/rivaas.dev/app)
[![Go Report Card](https://goreportcard.com/badge/rivaas.dev/app)](https://goreportcard.com/report/rivaas.dev/app)
[![Coverage](https://codecov.io/gh/rivaas-dev/rivaas/branch/main/graph/badge.svg?component=module_app)](https://codecov.io/gh/rivaas-dev/rivaas)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../LICENSE)

A batteries-included web framework built on the Rivaas router with integrated observability, lifecycle management, and sensible defaults for building production-ready applications.

## 📚 Documentation

**[📖 Full Documentation](https://rivaas.dev/docs/guides/app/)** | **[🔧 Installation](https://rivaas.dev/docs/guides/app/installation/)** | **[📘 User Guide](https://rivaas.dev/docs/guides/app/basic-usage/)** | **[📑 API Reference](https://rivaas.dev/docs/reference/packages/app/)** | **[💡 Examples](https://rivaas.dev/docs/guides/app/examples/)** | **[🐛 Troubleshooting](https://rivaas.dev/docs/reference/packages/app/troubleshooting/)**

## Features

- **Batteries-Included** - Pre-configured with sensible defaults for rapid development
- **Integrated Observability** - Built-in metrics (Prometheus/OTLP), tracing (OpenTelemetry), and structured logging (slog)
- **Request Binding & Validation** - Automatic request parsing with comprehensive validation strategies
- **OpenAPI Generation** - Automatic OpenAPI spec generation with Swagger UI
- **Lifecycle Hooks** - OnStart, OnReady, OnShutdown, OnStop for initialization and cleanup
- **Health Endpoints** - Kubernetes-compatible liveness and readiness probes
- **Graceful Shutdown** - Proper server shutdown with configurable timeouts
- **Environment-Aware** - Development and production modes with appropriate defaults

## Installation

```bash
go get rivaas.dev/app
```

Requires Go 1.25+

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
        c.JSON(http.StatusOK, map[string]string{
            "message": "Hello from Rivaas App!",
        })
    })

    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    if err := a.Start(ctx); err != nil {
        log.Fatal(err)
    }
}
```

## Learn More

- **[Installation Guide](https://rivaas.dev/docs/guides/app/installation/)** - Set up the app package
- **[Basic Usage](https://rivaas.dev/docs/guides/app/basic-usage/)** - Learn the fundamentals
- **[Configuration](https://rivaas.dev/docs/guides/app/configuration/)** - Configure your application
- **[Observability](https://rivaas.dev/docs/guides/app/observability/)** - Integrate metrics, tracing, and logging
- **[Context](https://rivaas.dev/docs/guides/app/context/)** - Request binding and validation
- **[Lifecycle](https://rivaas.dev/docs/guides/app/lifecycle/)** - Use lifecycle hooks
- **[Health Endpoints](https://rivaas.dev/docs/guides/app/health-endpoints/)** - Configure health checks
- **[Examples](https://rivaas.dev/docs/guides/app/examples/)** - Complete working examples
- **[Migration Guide](https://rivaas.dev/docs/guides/app/migration/)** - Migrate from router package

## API Reference

See [pkg.go.dev/rivaas.dev/app](https://pkg.go.dev/rivaas.dev/app) for complete API documentation.

## Contributing

Contributions are welcome! Please see the [main repository](../) for contribution guidelines.

## License

Apache License 2.0 - see [LICENSE](../LICENSE) for details.

---

Part of the [Rivaas](https://github.com/rivaas-dev/rivaas) web framework ecosystem.
