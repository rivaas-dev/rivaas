# Documentation

This directory contains standards, guidelines, and documentation for the Rivaas project.

## Standards

- **[Testing Standards](./TESTING_STANDARDS.md)** - Comprehensive testing patterns and best practices for all Rivaas packages
  - Unit tests, integration tests, benchmarks, and example tests
  - Ginkgo/Gomega patterns for complex integration scenarios
  - Table-driven test patterns and parallel execution guidelines

- **[Code Documentation Standards](./CODE_DOCUMENTATION_STANDARDS.md)** - Standards and rules for writing Go code documentation (GoDoc style)
  - Documentation structure and formatting guidelines
  - Package, type, function, and example documentation patterns
  - Best practices for clear, idiomatic Go documentation

## Contributing

When contributing to Rivaas, please follow the standards documented here to ensure consistency across the codebase.

For general contribution guidelines, see the [main README](../README.md#-contributing).

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

## Planned Documentation

Additional documentation planned for this directory:

- **Code style guidelines** - Go coding standards, naming conventions, and formatting rules
- **Architecture decision records (ADRs)** - Documented architectural decisions and their rationale
- **Contribution guidelines** - Detailed process for contributing code, documentation, and issues
- **Performance benchmarks and guidelines** - Performance optimization patterns and benchmarking standards
