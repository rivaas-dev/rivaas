# Logging

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/logging.svg)](https://pkg.go.dev/rivaas.dev/logging)
[![Go Report Card](https://goreportcard.com/badge/rivaas.dev/logging)](https://goreportcard.com/report/rivaas.dev/logging)
[![Coverage](https://codecov.io/gh/rivaas-dev/rivaas/branch/main/graph/badge.svg?component=module_logging)](https://codecov.io/gh/rivaas-dev/rivaas)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../LICENSE)

Structured logging for Rivaas using Go's standard `log/slog` package.

> **📚 [Complete documentation available on the Rivaas docs site →](https://rivaas.dev/guides/logging/)**

## Documentation

- **[Installation](https://rivaas.dev/guides/logging/installation/)** - Get started with the logging package
- **[User Guide](https://rivaas.dev/guides/logging/)** - Complete guide with tutorials and examples
- **[API Reference](https://rivaas.dev/reference/packages/logging/)** - Detailed API documentation
- **[Examples](https://rivaas.dev/guides/logging/examples/)** - Real-world usage patterns
- **[Troubleshooting](https://rivaas.dev/reference/packages/logging/troubleshooting/)** - Common issues and solutions

## Features

- Multiple output formats (JSON, text, console)
- Context-aware logging with OpenTelemetry trace correlation
- Automatic sensitive data redaction
- Log sampling for high-traffic scenarios
- Dynamic log level changes at runtime
- Convenience methods for common patterns
- Comprehensive testing utilities
- Zero external dependencies (except OpenTelemetry for tracing)

## Installation

```bash
go get rivaas.dev/logging
```

Requires Go 1.25+. See [installation guide](https://rivaas.dev/guides/logging/installation/) for details.

## Quick Start

```go
package main

import (
    "rivaas.dev/logging"
)

func main() {
    // Create logger with console output
    log := logging.MustNew(
        logging.WithConsoleHandler(),
        logging.WithDebugLevel(),
    )

    log.Info("service started", "port", 8080, "env", "production")
    log.Debug("debugging information", "key", "value")
    log.Error("operation failed", "error", "connection timeout")
}
```

## Learn More

- **[Basic Usage](https://rivaas.dev/guides/logging/basic-usage/)** - Handler types and log levels
- **[Configuration](https://rivaas.dev/guides/logging/configuration/)** - All configuration options
- **[Context Logging](https://rivaas.dev/guides/logging/context-logging/)** - Trace correlation with OpenTelemetry
- **[Best Practices](https://rivaas.dev/guides/logging/best-practices/)** - Production-ready patterns
- **[Router Integration](https://rivaas.dev/guides/logging/router-integration/)** - Using with Rivaas router
- **[Testing](https://rivaas.dev/guides/logging/testing/)** - Test utilities and patterns
- **[Migration](https://rivaas.dev/guides/logging/migration/)** - Switch from other loggers

## API Reference

For complete API documentation, see [pkg.go.dev/rivaas.dev/logging](https://pkg.go.dev/rivaas.dev/logging).

## Related Packages

- [router](../router/) - High-performance HTTP router
- [metrics](../metrics/) - OpenTelemetry metrics
- [tracing](../tracing/) - Distributed tracing
- [app](../app/) - Batteries-included framework

## Contributing

Contributions are welcome! Please see the [main repository](../) for contribution guidelines.

## License

Apache License 2.0 - see [LICENSE](../LICENSE) for details.

---

Part of the [Rivaas](https://github.com/rivaas-dev/rivaas) web framework ecosystem.
