# Config

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/config.svg)](https://pkg.go.dev/rivaas.dev/config)
[![Go Report Card](https://goreportcard.com/badge/rivaas.dev/config)](https://goreportcard.com/report/rivaas.dev/config)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../LICENSE)

A powerful and versatile configuration management package for Go that simplifies handling application settings across different environments and formats.

> **📚 [Complete Documentation →](https://rivaas.dev/docs/guides/config/)**

## Documentation

This README provides a quick overview. For comprehensive guides, tutorials, and API reference:

- **[Installation Guide](https://rivaas.dev/docs/guides/config/installation/)** - Get started
- **[User Guide](https://rivaas.dev/docs/guides/config/)** - Learn the features
- **[API Reference](https://rivaas.dev/docs/reference/packages/config/)** - Complete API docs
- **[Examples](https://rivaas.dev/docs/guides/config/examples/)** - Real-world patterns
- **[Troubleshooting](https://rivaas.dev/docs/reference/packages/config/troubleshooting/)** - FAQs and solutions

## Features

- **Easy Integration**: Simple and intuitive API
- **Flexible Sources**: Files, environment variables, Consul, custom sources
- **Format Agnostic**: JSON, YAML, TOML, and extensible codecs
- **Type Casting**: Automatic type conversion (bool, int, float, time, duration)
- **Hierarchical Merging**: Multiple sources merged with precedence
- **Struct Binding**: Automatic mapping to Go structs
- **Built-in Validation**: Struct methods, JSON Schemas, custom functions
- **Dot Notation**: Easy nested configuration access
- **Configuration Dumping**: Save effective configuration
- **Thread-Safe**: Safe for concurrent access
- **Nil-Safe**: Graceful handling of nil instances

## Installation

```shell
go get rivaas.dev/config
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    "rivaas.dev/config"
)

func main() {
    cfg := config.MustNew(
        config.WithFile("config.yaml"),
        config.WithEnv("APP_"),
    )

    if err := cfg.Load(context.Background()); err != nil {
        log.Fatalf("failed to load config: %v", err)
    }

    port := cfg.Int("server.port")
    host := cfg.StringOr("server.host", "localhost")
    
    log.Printf("Server: %s:%d", host, port)
}
```

**[See more examples →](https://rivaas.dev/docs/guides/config/examples/)**

## Learn More

- **[Basic Usage](https://rivaas.dev/docs/guides/config/basic-usage/)** - Loading files, accessing values, error handling
- **[Environment Variables](https://rivaas.dev/docs/guides/config/environment-variables/)** - 12-factor app configuration
- **[Struct Binding](https://rivaas.dev/docs/guides/config/struct-binding/)** - Type-safe configuration mapping
- **[Validation](https://rivaas.dev/docs/guides/config/validation/)** - Ensure configuration correctness
- **[Multiple Sources](https://rivaas.dev/docs/guides/config/multiple-sources/)** - Combine configurations
- **[Custom Codecs](https://rivaas.dev/docs/guides/config/custom-codecs/)** - Support custom formats

## Contributing

Contributions are welcome! Please see the [main repository](../) for contribution guidelines.

## License

Apache License 2.0 - see [LICENSE](../LICENSE) for details.
