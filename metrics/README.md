# Metrics

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/metrics.svg)](https://pkg.go.dev/rivaas.dev/metrics)
[![Go Report Card](https://goreportcard.com/badge/rivaas.dev/metrics)](https://goreportcard.com/report/rivaas.dev/metrics)
[![Coverage](https://codecov.io/gh/rivaas-dev/rivaas/branch/main/graph/badge.svg?component=module_metrics)](https://codecov.io/gh/rivaas-dev/rivaas)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../LICENSE)

A metrics collection package for Go applications using OpenTelemetry. This package provides metrics functionality with support for multiple exporters including Prometheus, OTLP, and stdout.

> 📚 **[Complete Documentation](https://rivaas.dev/docs/guides/metrics/)** | **[API Reference](https://rivaas.dev/docs/reference/packages/metrics/)** | **[Examples](https://rivaas.dev/docs/guides/metrics/examples/)**

## Documentation

- **[Installation](https://rivaas.dev/docs/guides/metrics/installation/)** - Get started with the metrics package
- **[User Guide](https://rivaas.dev/docs/guides/metrics/)** - Comprehensive usage guide
- **[API Reference](https://rivaas.dev/docs/reference/packages/metrics/)** - Complete API documentation
- **[Examples](https://rivaas.dev/docs/guides/metrics/examples/)** - Real-world usage patterns
- **[Troubleshooting](https://rivaas.dev/docs/reference/packages/metrics/troubleshooting/)** - Common issues and solutions

## Features

- **Multiple Providers**: Prometheus, OTLP, and stdout exporters
- **Built-in HTTP Metrics**: Automatic request metrics via middleware
- **Custom Metrics**: Counters, histograms, and gauges with error handling
- **Thread-Safe**: All methods safe for concurrent use
- **Security**: Automatic filtering of sensitive headers
- **Testing Utilities**: Built-in support for unit tests

## Installation

```bash
go get rivaas.dev/metrics
```

Requires Go 1.25+

## Quick Start

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os/signal"
    "time"
    
    "rivaas.dev/metrics"
)

func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    recorder, err := metrics.New(
        metrics.WithPrometheus(":9090", "/metrics"),
        metrics.WithServiceName("my-api"),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    if err := recorder.Start(ctx); err != nil {
        log.Fatal(err)
    }
    
    defer func() {
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        recorder.Shutdown(shutdownCtx)
    }()

    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, World!"))
    })

    handler := metrics.Middleware(recorder,
        metrics.WithExcludePaths("/health"),
    )(mux)

    http.ListenAndServe(":8080", handler)
}
```

## Learn More

- **[Basic Usage](https://rivaas.dev/docs/guides/metrics/basic-usage/)** - Fundamentals of metrics collection
- **[Providers](https://rivaas.dev/docs/guides/metrics/providers/)** - Prometheus, OTLP, and stdout setup
- **[Configuration](https://rivaas.dev/docs/guides/metrics/configuration/)** - Advanced configuration options
- **[Custom Metrics](https://rivaas.dev/docs/guides/metrics/custom-metrics/)** - Recording your own metrics
- **[Middleware](https://rivaas.dev/docs/guides/metrics/middleware/)** - HTTP metrics integration
- **[Testing](https://rivaas.dev/docs/guides/metrics/testing/)** - Testing with metrics

## Contributing

Contributions are welcome! Please see the [main repository](../) for contribution guidelines.

## License

Apache License 2.0 - see [LICENSE](../LICENSE) for details.

---

Part of the [Rivaas](https://github.com/rivaas-dev/rivaas) web framework ecosystem.
