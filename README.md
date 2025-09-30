# Rivaas

**Cloud-Native Go Service Framework**

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.23.0-blue.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

Rivaas is a modern, cloud-native service framework for Go that provides high-performance building blocks for scalable applications. Built with performance and developer experience in mind.

## 📋 Table of Contents

- [🚀 Modules](#-modules)
- [📦 Installation](#-installation)
- [🚀 Quick Start](#-quick-start)
- [📊 Performance](#-performance)
- [🏗️ Architecture](#️-architecture)
- [📚 Documentation](#-documentation)
- [🤝 Contributing](#-contributing)
- [📜 License](#-license)

## 🚀 Modules

### 🛣️ Router

High-performance HTTP router with radix tree routing, zero-allocation static routes, and comprehensive middleware support.

- **Performance**: 294K+ req/s, 3.4µs latency, 55 bytes/req
- **Features**: Parameter binding, route groups, constraints, static files
- **Middleware**: Context-aware chain execution with pooling

```go
r := router.New()
r.GET("/users/:id", func(c *router.Context) {
    c.JSON(200, map[string]string{"user_id": c.Param("id")})
})
```

### 🔧 Config *(Coming Soon)*

Unified configuration management with environment variables, file formats, and validation.

### 📝 Logging *(Planned)*

Structured logging with multiple outputs, correlation IDs, and performance optimization.

### 📊 Metrics *(Planned)*

Built-in metrics collection with Prometheus integration and custom collectors.

### 🔍 Tracing *(Planned)*

Distributed tracing support with OpenTelemetry integration.

## 📋 Requirements

- **Go**: 1.23.0 or higher
- **Dependencies**: None (zero external dependencies)
- **Platforms**: Linux, macOS, Windows

## 📦 Installation

```bash
# Router module
go get github.com/rivaas-dev/rivaas/router

# Future modules (when available)
# go get github.com/rivaas-dev/rivaas/config
# go get github.com/rivaas-dev/rivaas/logging
```

## 🚀 Quick Start

```go
package main

import (
    "net/http"
    "github.com/rivaas-dev/rivaas/router"
)

func main() {
    r := router.New()
    
    r.GET("/", func(c *router.Context) {
        c.JSON(http.StatusOK, map[string]string{
            "service": "my-service",
            "status":  "running",
        })
    })
    
    // Route groups and middleware
    api := r.Group("/api/v1")
    api.Use(Logger()) // Use defined middleware
    api.GET("/users/:id", func(c *router.Context) {
        c.JSON(200, map[string]string{"user_id": c.Param("id")})
    })
    
    http.ListenAndServe(":8080", r)
}

func Logger() router.HandlerFunc {
    return func(c *router.Context) {
        // Simple logging middleware
        c.Next()
    }
}
```

## 📊 Performance

The router module delivers exceptional performance:

| Metric | Value |
|--------|--------|
| Throughput | 294K+ req/s |
| Latency | 3.4µs avg |
| Memory/Request | 55 bytes |
| Allocations/Request | 2 |

**Comparison with popular routers:**

- **Rivaas**: 198.0 ns/op, 55B/op, 2 allocs/op
- **Echo**: 138.0 ns/op, 61B/op, 2 allocs/op  
- **Gin**: 165.0 ns/op, 101B/op, 3 allocs/op

## 🏗️ Architecture

Rivaas follows cloud-native principles:

- **Modular Design** - Use only what you need
- **Performance First** - Optimized for high-throughput services
- **Developer Experience** - Clean APIs and comprehensive documentation
- **Production Ready** - Battle-tested with comprehensive test coverage

## 📚 Documentation

- **[Router Guide](router/README.md)** - Complete router documentation
- **[Examples](router/examples/)** - Working code examples
- **[API Reference](https://pkg.go.dev/github.com/rivaas-dev/rivaas)** - Go package docs

## 🤝 Contributing

Contributions are welcome! Please read our [contributing guidelines](CONTRIBUTING.md).

## 📜 License

Apache License 2.0 - see [LICENSE](LICENSE) for details.

---

**Building the future of Go services** ⚡
