# Rivaas Design Principles

This document describes the core design principles and architectural decisions that guide the Rivaas project. Understanding these principles helps contributors maintain consistency and helps users understand why the framework is designed the way it is.

## Table of Contents

- [Core Philosophy](#core-philosophy)
  - [Developer Experience First](#developer-experience-first)
- [Architectural Patterns](#architectural-patterns)
  - [Functional Options Pattern](#functional-options-pattern)
  - [Separation of Concerns](#separation-of-concerns)
- [Package Architecture](#package-architecture)
  - [Standalone Packages](#standalone-packages)
  - [The `app` Package: Integration Layer](#the-app-package-integration-layer)
- [Design Decisions](#design-decisions)

---

## Core Philosophy

### Developer Experience First

Developer Experience (DX) is a primary design consideration in Rivaas. Every API decision is evaluated through the lens of how it affects the developer using the framework.

#### What This Means in Practice

**Sensible Defaults**

Every package works out of the box with zero configuration. Defaults are chosen to be safe and useful for most use cases:

```go
// Works immediately with sensible defaults
app := app.MustNew()

// Explicit configuration when needed
app := app.MustNew(
    app.WithServiceName("my-api"),
    app.WithEnvironment("production"),
)
```

**Progressive Disclosure**

Simple use cases should be simple. Advanced features are available but don't complicate the basic API:

```go
// Simple: Just works
logger := logging.MustNew()

// Intermediate: Common customization
logger := logging.MustNew(
    logging.WithJSONHandler(),
    logging.WithLevel(logging.LevelDebug),
)

// Advanced: Full control when needed
logger := logging.MustNew(
    logging.WithCustomLogger(myCustomSlogLogger),
    logging.WithSampling(logging.SamplingConfig{
        Initial:    100,
        Thereafter: 100,
        Tick:       time.Minute,
    }),
)
```

**Discoverable APIs**

Functional options make APIs self-documenting. IDE autocomplete shows all available options:

```go
metrics.MustNew(
    metrics.With...  // IDE shows: WithProvider, WithPort, WithPath, etc.
)
```

**Fail Fast with Clear Errors**

Configuration errors are caught at initialization, not at runtime:

```go
// Returns clear, structured validation errors
app, err := app.New(
    app.WithServerTimeout(-1 * time.Second), // Invalid
)
// Error: "server.readTimeout: must be positive"
```

**Convenience Without Sacrificing Control**

Provide `MustNew()` for cases where panic is acceptable (typically `main()`) and `New()` for cases where error handling is needed:

```go
// In main() - panic is fine
app := app.MustNew(...)

// In tests or libraries - handle errors
app, err := app.New(...)
if err != nil {
    return fmt.Errorf("failed to create app: %w", err)
}
```

---

## Architectural Patterns

### Functional Options Pattern

All Rivaas packages use the functional options pattern for configuration. This pattern provides:

- **Backward compatibility**: New options can be added without breaking existing code
- **Sensible defaults**: Unconfigured options use safe, useful defaults
- **Self-documenting**: Option names describe what they configure
- **Composable**: Options can be combined and reused
- **IDE-friendly**: Autocomplete shows all available options

#### Standard Implementation

Every package follows this structure:

```go
// Option type for the package
type Option func(*Config)

// Constructor with options
func New(opts ...Option) (*Config, error) {
    cfg := defaultConfig()  // Start with defaults
    
    for _, opt := range opts {
        opt(cfg)  // Apply each option
    }
    
    if err := cfg.validate(); err != nil {
        return nil, err
    }
    
    return cfg, nil
}

// Convenience constructor that panics on error
func MustNew(opts ...Option) *Config {
    cfg, err := New(opts...)
    if err != nil {
        panic(err)
    }
    return cfg
}
```

#### Option Naming Conventions

- `With<Feature>` - Enable or configure a feature
- `Without<Feature>` - Disable a feature (when default is enabled)

```go
// Enable features
metrics.WithPrometheus(":9090", "/metrics") // or WithOTLP(), WithStdout()
logging.WithJSONHandler()
app.WithServiceName("my-api")

// Disable features
metrics.WithServerDisabled()
app.WithoutDefaultMiddleware()
```

#### Examples Across Packages

**Metrics Package:**

```go
recorder := metrics.MustNew(
    metrics.WithPrometheus(":9090", "/metrics"), // or WithOTLP(), WithStdout()
    metrics.WithServiceName("my-api"),
)
```

**Logging Package:**

```go
logger := logging.MustNew(
    logging.WithJSONHandler(),
    logging.WithLevel(logging.LevelInfo),
    logging.WithServiceName("my-api"),
)
```

**Router Package:**

```go
r := router.MustNew(
    router.WithNotFoundHandler(custom404),
    router.WithMethodNotAllowedHandler(custom405),
)
```

---

### Separation of Concerns

Each package has a single, well-defined responsibility. This principle ensures:

- **Testability**: Packages can be tested in isolation
- **Maintainability**: Changes to one concern don't affect others
- **Flexibility**: Users can use only what they need
- **Clarity**: Clear boundaries make the codebase easier to understand

#### Package Responsibilities

| Package | Single Responsibility |
|---------|----------------------|
| `router` | HTTP routing and request dispatching |
| `metrics` | Metrics collection and export |
| `tracing` | Distributed tracing |
| `logging` | Structured logging |
| `binding` | Request data binding to structs |
| `validation` | Input validation |
| `errors` | Error formatting (RFC 9457, JSON:API) |
| `openapi` | OpenAPI specification generation |
| `app` | Integration and lifecycle management |

#### Clear Boundaries

Packages communicate through well-defined interfaces, not internal implementation details:

```go
// metrics package exposes a clean interface
type Recorder struct { ... }
func (r *Recorder) RecordRequest(method, path string, status int, duration time.Duration)

// app package uses the interface without knowing implementation details
app.metrics.RecordRequest(method, path, status, duration)
```

---

## Package Architecture

### Standalone Packages

**Every Rivaas package is independently usable.** You can use any package without the full framework.

This design principle ensures:

- **No vendor lock-in**: Use Rivaas packages with any Go HTTP framework
- **Gradual adoption**: Adopt packages one at a time
- **Testing flexibility**: Test with minimal dependencies
- **Microservice compatibility**: Different services can use different subsets

#### Standalone Package Requirements

Each standalone package must:

1. **Work without `app`**: No imports from `rivaas.dev/app`
2. **Have its own `go.mod`**: Independent versioning and dependencies
3. **Provide `New()` and `MustNew()`**: Standard constructors
4. **Use functional options**: Consistent configuration pattern
5. **Have sensible defaults**: Work with zero configuration
6. **Include comprehensive documentation**: README, examples, godoc

#### Standalone Usage Examples

**Metrics with Standard Library:**

```go
package main

import (
    "net/http"
    "rivaas.dev/metrics"
)

func main() {
    // Use metrics standalone with net/http
    recorder := metrics.MustNew(
        metrics.WithPrometheus(":9090", "/metrics"),
        metrics.WithServiceName("my-api"),
    )
    defer recorder.Shutdown(context.Background())

    // Create middleware for standard http.Handler
    handler := metrics.Middleware(recorder)(myHandler)
    
    http.ListenAndServe(":8080", handler)
}
```

**Logging Standalone:**

```go
package main

import "rivaas.dev/logging"

func main() {
    // Use logging anywhere - no framework needed
    logger := logging.MustNew(
        logging.WithJSONHandler(),
        logging.WithServiceName("background-worker"),
    )
    
    logger.Info("worker started", "queue", "emails")
}
```

**Binding with Any Framework:**

```go
package main

import "rivaas.dev/binding"

type CreateUserRequest struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

func handler(w http.ResponseWriter, r *http.Request) {
    // Use binding standalone
    var req CreateUserRequest
    if err := binding.JSON(r, &req); err != nil {
        // Handle error
    }
}
```

**Errors with Any Framework:**

```go
package main

import (
    "net/http"
    "rivaas.dev/errors"
)

func handler(w http.ResponseWriter, r *http.Request) {
    // Use RFC 9457 error formatting standalone
    formatter := &errors.RFC9457{}
    
    err := errors.Problem{
        Type:   "/errors/not-found",
        Title:  "User Not Found",
        Status: 404,
        Detail: "User with ID 123 does not exist",
    }
    
    formatter.Format(w, r, err)
}
```

#### Standalone Package List

| Package | Import Path | Purpose |
|---------|-------------|---------|
| `router` | `rivaas.dev/router` | HTTP routing |
| `metrics` | `rivaas.dev/metrics` | Prometheus/OTLP metrics |
| `tracing` | `rivaas.dev/tracing` | OpenTelemetry tracing |
| `logging` | `rivaas.dev/logging` | Structured logging (slog) |
| `binding` | `rivaas.dev/binding` | Request binding |
| `validation` | `rivaas.dev/validation` | Input validation |
| `errors` | `rivaas.dev/errors` | Error formatting |
| `openapi` | `rivaas.dev/openapi` | OpenAPI spec generation |

---

### The `app` Package: Integration Layer

The `app` package is the **glue** that combines standalone packages into a cohesive, batteries-included framework.

#### Role of `app`

1. **Integration**: Wires standalone packages together
2. **Lifecycle Management**: Handles startup, shutdown, graceful termination
3. **Configuration Propagation**: Shares service metadata (name, version) across packages
4. **Sensible Defaults**: Provides production-ready defaults for all integrations
5. **Convenience**: Single entry point for common use cases

#### How `app` Integrates Packages

```go
// app/app.go imports and wires standalone packages
import (
    "rivaas.dev/errors"
    "rivaas.dev/logging"
    "rivaas.dev/metrics"
    "rivaas.dev/openapi"
    "rivaas.dev/router"
    "rivaas.dev/tracing"
)

type App struct {
    router  *router.Router
    metrics *metrics.Recorder
    tracing *tracing.Config
    logging *logging.Config
    openapi *openapi.Manager
    // ...
}
```

#### Automatic Wiring

When you use `app`, packages are automatically connected:

```go
app := app.MustNew(
    app.WithServiceName("my-api"),
    app.WithObservability(
        app.WithLogging(logging.WithJSONHandler()),
        app.WithMetrics(), // Prometheus is default
        app.WithTracing(tracing.WithOTLP("localhost:4317")),
    ),
)

// Behind the scenes, app:
// 1. Creates logging with service name "my-api"
// 2. Creates metrics with service name "my-api"
// 3. Auto-wires logger to metrics (for error reporting)
// 4. Auto-wires logger to tracing (for error reporting)
// 5. Sets up unified observability recorder
// 6. Configures graceful shutdown for all components
```

#### Choose Your Level of Integration

**Full Framework (Recommended for most users):**

```go
// Use app for batteries-included experience
app := app.MustNew(
    app.WithServiceName("my-api"),
    app.WithObservability(
        app.WithLogging(),
        app.WithMetrics(),
        app.WithTracing(),
    ),
)
app.GET("/users", handlers.ListUsers)
app.Run(":8080")
```

**Standalone Packages (For advanced users or specific needs):**

```go
// Use packages individually for maximum control
r := router.MustNew()
logger := logging.MustNew()
recorder := metrics.MustNew()

// Wire them yourself
r.Use(loggingMiddleware(logger))
r.Use(metricsMiddleware(recorder))

r.GET("/users", listUsers)
http.ListenAndServe(":8080", r)
```

---

## Design Decisions

This section documents key architectural decisions and their rationale.

### Why Functional Options Over Config Structs?

**Decision**: Use functional options pattern instead of configuration structs.

**Rationale**:
- Adding new options doesn't break existing code (backward compatible)
- Defaults are implicit, not explicit empty values
- Options are self-documenting through function names
- IDE autocomplete shows available options
- Options can perform validation during application

**Example of the benefit**:

```go
// With config struct: Must update all call sites when adding fields
type Config struct {
    ServiceName string
    Port        int
    // New field added - all existing code must be reviewed
    NewFeature  bool
}

// With functional options: Existing code continues to work
metrics.MustNew(
    metrics.WithServiceName("api"),
    // New option added - existing code unaffected
)
```

### Why Standalone Packages?

**Decision**: Every package works independently without requiring the `app` framework.

**Rationale**:
- Users can adopt packages incrementally
- No vendor lock-in to the full framework
- Easier testing with minimal dependencies
- Better for library authors who want specific functionality
- Aligns with Go's composition-over-inheritance philosophy

### Why a Separate `app` Package?

**Decision**: Provide an `app` package that integrates standalone packages.

**Rationale**:
- Most users want a batteries-included experience
- Integration code shouldn't pollute standalone packages
- Centralized lifecycle management (startup, shutdown)
- Single place for cross-cutting concerns
- Consistent configuration propagation (service name, version)

### Why `New()` and `MustNew()` Pattern?

**Decision**: Provide both error-returning and panic-on-error constructors.

**Rationale**:
- `New()` for libraries and code that needs error handling
- `MustNew()` for `main()` where panic is acceptable
- Follows standard library patterns (`regexp.Compile` vs `regexp.MustCompile`)
- Reduces boilerplate in common cases while maintaining flexibility

---

## Summary

| Principle | Implementation |
|-----------|----------------|
| **DX First** | Sensible defaults, progressive disclosure, clear errors |
| **Functional Options** | All packages use `Option func(*Config)` pattern |
| **Separation of Concerns** | Each package has single responsibility |
| **Standalone Packages** | Every package works without `app` |
| **`app` as Glue** | Integration, lifecycle, configuration propagation |

These principles guide all development decisions in Rivaas. When contributing, ensure your changes align with these principles to maintain consistency across the codebase.
