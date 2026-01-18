# Tracing

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/tracing.svg)](https://pkg.go.dev/rivaas.dev/tracing)
[![Go Report Card](https://goreportcard.com/badge/rivaas.dev/tracing)](https://goreportcard.com/report/rivaas.dev/tracing)
[![Coverage](https://codecov.io/gh/rivaas-dev/rivaas/branch/main/graph/badge.svg?component=module_tracing)](https://codecov.io/gh/rivaas-dev/rivaas)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../LICENSE)

A distributed tracing package for Go applications using OpenTelemetry. This package provides easy-to-use tracing functionality with support for various exporters and seamless integration with HTTP frameworks.

> **📚 Full documentation available at [rivaas.dev/docs](https://rivaas.dev/docs/guides/tracing/)**

## Documentation

- **[Installation](https://rivaas.dev/docs/guides/tracing/installation/)** - Get started with the tracing package
- **[User Guide](https://rivaas.dev/docs/guides/tracing/)** - Learn tracing fundamentals and best practices
- **[API Reference](https://rivaas.dev/docs/reference/packages/tracing/)** - Complete API documentation
- **[Examples](https://rivaas.dev/docs/guides/tracing/examples/)** - Real-world usage patterns
- **[Troubleshooting](https://rivaas.dev/docs/reference/packages/tracing/troubleshooting/)** - Common issues and solutions

## Features

- **OpenTelemetry Integration** - Full OpenTelemetry tracing support
- **Context Propagation** - Automatic trace context propagation across services
- **Multiple Providers** - Stdout, OTLP (gRPC and HTTP), and Noop exporters
- **HTTP Middleware** - Standalone middleware for any HTTP framework
- **Span Management** - Easy span creation and management with lifecycle hooks
- **Path Filtering** - Exclude specific paths from tracing via middleware options
- **Consistent API** - Same design patterns as the metrics package

## Installation

```bash
go get rivaas.dev/tracing
```

Requires Go 1.25+

## Quick Start

```go
package main

import (
    "context"
    "log"
    "net/http"
    
    "rivaas.dev/tracing"
)

func main() {
    // Create tracer
    tracer := tracing.MustNew(
        tracing.WithServiceName("my-service"),
        tracing.WithServiceVersion("v1.0.0"),
        tracing.WithOTLP("localhost:4317"),
    )
    
    // Start tracer (required for OTLP)
    if err := tracer.Start(context.Background()); err != nil {
        log.Fatal(err)
    }
    defer tracer.Shutdown(context.Background())

    // Create HTTP handler
    mux := http.NewServeMux()
    mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello"))
    })

    // Wrap with tracing middleware
    handler := tracing.MustMiddleware(tracer,
        tracing.WithExcludePaths("/health", "/metrics"),
    )(mux)

    log.Fatal(http.ListenAndServe(":8080", handler))
}
```

## Learn More

- **[Installation Guide](https://rivaas.dev/docs/guides/tracing/installation/)** - Get started
- **[Basic Usage](https://rivaas.dev/docs/guides/tracing/basic-usage/)** - Learn the fundamentals
- **[Providers](https://rivaas.dev/docs/guides/tracing/providers/)** - Choose the right exporter
- **[Middleware](https://rivaas.dev/docs/guides/tracing/middleware/)** - HTTP integration
- **[Context Propagation](https://rivaas.dev/docs/guides/tracing/context-propagation/)** - Distributed tracing
- **[Examples](https://rivaas.dev/docs/guides/tracing/examples/)** - Production-ready configurations

## Contributing

Contributions are welcome! Please see the [main repository](../) for contribution guidelines.

## License

Apache License 2.0 - see [LICENSE](../LICENSE) for details.

---

Part of the [Rivaas](https://github.com/rivaas-dev/rivaas) web framework ecosystem.
