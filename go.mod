// This is a multi-module repository.
// Each module has its own go.mod file in its subdirectory.
//
// Modules:
//   - rivaas.dev/app      → app/
//   - rivaas.dev/metrics  → metrics/
//   - rivaas.dev/router   → router/
//   - rivaas.dev/tracing  → tracing/
//
// Version tags follow the pattern: <module-dir>/<version>
// Examples: router/v0.1.0, metrics/v0.2.0
//
// This file helps Go's module system understand the repository structure
// and correctly resolve semantic versions for modules with custom domain paths.

module github.com/rivaas-dev/rivaas

go 1.24.0

require github.com/stretchr/testify v1.11.1 // indirect
