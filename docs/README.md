# Documentation

This directory contains images and assets for the Rivaas project.

## Main Documentation Site

All documentation has been moved to the official documentation site:

**[rivaas.dev/docs](https://rivaas.dev/docs)**

## What You'll Find There

### Architecture & Design

- **[Design Principles](https://rivaas.dev/docs/about/design-principles/)** - Core design philosophy and architectural decisions
  - Developer Experience (DX) first approach
  - Functional options pattern
  - Standalone packages architecture
  - The `app` package as integration layer

### Contributing

- **[Contributing Guide](https://rivaas.dev/docs/contributing/)** - How to contribute to Rivaas

- **[Documentation Standards](https://rivaas.dev/docs/contributing/documentation-standards/)** - Standards for writing Go code documentation (GoDoc style)
  - Documentation structure and formatting guidelines
  - Package, type, function, and example documentation patterns
  - Best practices for clear, idiomatic Go documentation

- **[Testing Standards](https://rivaas.dev/docs/contributing/testing-standards/)** - Comprehensive testing patterns and best practices
  - Unit tests, integration tests, benchmarks, and example tests
  - Ginkgo/Gomega patterns for complex integration scenarios
  - Table-driven test patterns and parallel execution guidelines

### Guides

- **[Getting Started](https://rivaas.dev/docs/getting-started/)** – Install and set up Rivaas
- **[Guides](https://rivaas.dev/docs/guides/)** - Step-by-step tutorials for all packages
- **[API Reference](https://rivaas.dev/docs/reference/)** - Complete API documentation

## Package-Specific Documentation

Each package in the Rivaas repository includes its own `README.md` with package-specific documentation:

- **[app](../app/README.md)** - Batteries-included web framework
- **[router](../router/README.md)** - High-performance HTTP router
- **[logging](../logging/README.md)** - Structured logging
- **[metrics](../metrics/README.md)** – Metrics collection
- **[tracing](../tracing/README.md)** – Distributed tracing
- **[binding](../binding/README.md)** – Request binding and validation
- **[errors](../errors/README.md)** - Error handling
- **[openapi](../openapi/README.md)** - OpenAPI specification
- **[validation](../validation/README.md)** - Validation utilities

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

For general contribution guidelines, see the [main README](../README.md#contributing).
