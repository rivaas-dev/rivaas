# Documentation

This directory contains images and assets for the Rivaas project.

## Main Documentation Site

All documentation has been moved to the official documentation site:

**[docs.rivaas.dev](https://docs.rivaas.dev)**

## What You'll Find There

### Architecture & Design

- **[Design Principles](https://docs.rivaas.dev/about/design-principles/)** - Core design philosophy and architectural decisions
  - Developer Experience (DX) first approach
  - Functional options pattern
  - Standalone packages architecture
  - The `app` package as integration layer

### Contributing

- **[Contributing Guide](https://docs.rivaas.dev/contributing/)** - How to contribute to Rivaas

- **[Documentation Standards](https://docs.rivaas.dev/contributing/documentation-standards/)** - Standards for writing Go code documentation (GoDoc style)
  - Documentation structure and formatting guidelines
  - Package, type, function, and example documentation patterns
  - Best practices for clear, idiomatic Go documentation

- **[Testing Standards](https://docs.rivaas.dev/contributing/testing-standards/)** - Comprehensive testing patterns and best practices
  - Unit tests, integration tests, benchmarks, and example tests
  - Ginkgo/Gomega patterns for complex integration scenarios
  - Table-driven test patterns and parallel execution guidelines

### Guides

- **[Getting Started](https://docs.rivaas.dev/getting-started/)** - Install and set up Rivaas
- **[Guides](https://docs.rivaas.dev/guides/)** - Step-by-step tutorials for all packages
- **[API Reference](https://docs.rivaas.dev/reference/)** - Complete API documentation

## Package-Specific Documentation

Each package in the Rivaas repository includes its own `README.md` with package-specific documentation:

- **[app](../app/README.md)** - Batteries-included web framework
- **[router](../router/README.md)** - High-performance HTTP router
- **[logging](../logging/README.md)** - Structured logging
- **[metrics](../metrics/README.md)** - Metrics collection
- **[tracing](../tracing/README.md)** - Distributed tracing
- **[binding](../binding/)** - Request binding and validation
- **[errors](../errors/README.md)** - Error handling
- **[openapi](../openapi/README.md)** - OpenAPI specification
- **[validation](../validation/)** - Validation utilities

## Building the Documentation Site

The documentation site is in the [`docs` repository](https://github.com/rivaas-dev/docs) and uses Hugo.

To build and run locally:

```bash
cd /path/to/docs
hugo server -D
```

Then visit http://localhost:1313

## Contributing to Documentation

When contributing to Rivaas, please follow the standards documented on the site to ensure consistency across the codebase.

For general contribution guidelines, see the [main README](../README.md#-contributing).
