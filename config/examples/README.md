# Config Examples

This directory contains examples demonstrating different ways to use the Config package.

## Examples Overview

### 1. [Basic Example](./basic/)

A simple example showing the most basic usage of Config package - loading configuration from a YAML file into a Go struct.

**Features:**

- File source (YAML)
- Struct binding
- Type conversion
- Nested structures
- Arrays and slices
- Time and URL types

**Best for:** Getting started with Config package, understanding basic concepts.

### 2. [Environment Variables Example](./environment/)

Demonstrates loading configuration from environment variables, following the Twelve-Factor App methodology.

**Features:**

- Environment variable source
- Struct binding
- Nested configuration
- Direct access methods
- Type conversion

**Best for:** Containerized applications, cloud deployments, following 12-factor app principles.

### 3. [Mixed Configuration Example](./mixed/)

Shows how to combine YAML files and environment variables, with environment variables overriding YAML defaults.

**Features:**

- Mixed configuration sources
- Configuration precedence
- Environment variable mapping
- Struct binding
- Direct access

**Best for:** Applications that need both default configuration files and environment-specific overrides.

### 4. [Comprehensive Example](./comprehensive/)

A complete example demonstrating advanced Config package features with a realistic web application configuration.

**Features:**

- Mixed configuration sources
- Complex nested structures
- Validation
- Comprehensive testing
- Production-ready patterns
- Docker examples

**Best for:** Production applications, learning advanced features, understanding best practices.

## Quick Start

Choose the example that best fits your needs:

```bash
# Basic YAML configuration
cd examples/basic
go run main.go

# Environment variables only
cd examples/environment
export WEBAPP_SERVER_HOST=localhost
export WEBAPP_SERVER_PORT=8080
go run main.go

# Mixed YAML + environment variables
cd examples/mixed
export WEBAPP_SERVER_PORT=8080  # Override YAML default
go run main.go

# Comprehensive example with tests
cd examples/comprehensive
go test -v
go run main.go
```

## Example Progression

The examples are designed to be progressive:

1. **Basic**: Start here to understand core concepts
2. **Environment**: Learn about environment variable configuration
3. **Mixed**: Understand configuration precedence and mixed sources
4. **Comprehensive**: See advanced features and production patterns

## Common Patterns

### Basic YAML Configuration

```go
var config MyConfig
cfg, err := config.New(
    config.WithFileSource("config.yaml", codec.TypeYAML),
    config.WithBinding(&config),
)
```

### Environment Variables Only

```go
var config MyConfig
cfg, err := config.New(
    config.WithOSEnvVarSource("APP_"),
    config.WithBinding(&config),
)
```

### Mixed Configuration (Recommended for Production)

```go
var config MyConfig
cfg, err := config.New(
    config.WithFileSource("config.yaml", codec.TypeYAML),  // Defaults
    config.WithOSEnvVarSource("APP_"),                     // Overrides
    config.WithBinding(&config),
)
```

## Environment Variable Naming

All examples use a consistent environment variable naming convention:

- **Prefix**: Configurable (e.g., `WEBAPP_`, `APP_`)
- **Hierarchy**: Underscores create nested levels
- **Conversion**: Keys are converted to lowercase

Examples:

- `WEBAPP_SERVER_PORT` → `server.port`
- `WEBAPP_DATABASE_PRIMARY_HOST` → `database.primary.host`

## Testing

Most examples include tests demonstrating different scenarios:

```bash
cd examples/comprehensive
go test -v
```

## Production Considerations

When using these examples in production:

1. **Use mixed configuration** for flexibility
2. **Implement validation** for required fields
3. **Use secrets management** for sensitive values
4. **Follow 12-factor app principles** for configuration
5. **Add comprehensive logging** for debugging

## Contributing

When adding new examples:

1. Create a new directory with a descriptive name
2. Include a comprehensive README.md
3. Add tests if applicable
4. Update this main README.md
5. Follow the existing patterns and conventions 