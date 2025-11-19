# Testing Standards for Rivaas

This document outlines the standardized testing patterns used across all packages in the Rivaas codebase.

## File Structure

All packages must have the following test files:

1. **`*_test.go`** - Unit tests (table-driven preferred, same package)
2. **`example_test.go`** - Runnable examples for godoc (external package)
3. **`*_bench_test.go`** - Performance benchmarks (same package)
4. **`integration_test.go`** or **`{feature}_integration_test.go`** - Integration tests (external package, Ginkgo recommended for complex scenarios)
5. **`testing.go`** or **`testutil` package** - Test helpers (if needed)

## Test File Naming

- Unit tests: `{package}_test.go` or `{feature}_test.go` (package: `{package}`)
- Benchmarks: `{package}_bench_test.go` or `{feature}_bench_test.go` (package: `{package}`)
- Examples: `example_test.go` (always in `{package}_test` package)
- Integration: `integration_test.go` or `{feature}_integration_test.go` (package: `{package}_test`)
- Helpers: `testing.go` or `testutil/{helpers}.go` (package: `{package}`)

## Package Organization

### Unit Tests

- **Package**: Same as source (`package router`)
- **Access**: Can test both public and internal (unexported) APIs
- **Use case**: Testing individual functions, internal implementation details, edge cases
- **Framework**: Standard `testing` package with `testify/assert` or `testify/require`

### Integration Test Package

- **Package**: External (`package router_test`)
- **Access**: Only public APIs (black-box testing)
- **Use case**: Testing full request/response cycles, component interactions, complex scenarios
- **Framework**:
  - Standard `testing` package for simple integration tests
  - **Ginkgo/Gomega** recommended for complex scenarios with multiple phases, nested contexts, and BDD-style organization

### Example Test Package

- **Package**: External (`package router_test`)
- **Access**: Only public APIs
- **Use case**: Demonstrating public API usage in godoc

## Table-Driven Tests

All tests with multiple cases should use table-driven pattern:

```go
func TestFunctionName(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name    string
        input   any
        want    any
        wantErr bool
    }{
        {
            name:    "valid input",
            input:   "test",
            want:    "result",
            wantErr: false,
        },
        {
            name:    "invalid input",
            input:   "",
            want:    nil,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            got, err := FunctionName(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("FunctionName() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("FunctionName() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Table-Driven Test Guidelines

- Always use `t.Parallel()` for both outer and inner tests
- Use descriptive test case names
- Group related test cases together
- Use `want` and `wantErr` for expected results
- Use `got` for actual results
- Include both positive and negative test cases

## Example Tests

All public APIs must have example tests in `example_test.go`:

```go
package package_test

import (
    "fmt"
    "rivaas.dev/package"
)

// ExampleFunctionName demonstrates basic usage.
func ExampleFunctionName() {
    result := package.FunctionName("input")
    fmt.Println(result)
    // Output: expected output
}

// ExampleFunctionName_withOptions demonstrates usage with options.
func ExampleFunctionName_withOptions() {
    result := package.FunctionName("input",
        package.WithOption("value"),
    )
    fmt.Println(result)
    // Output: expected output
}
```

### Example Test Guidelines

- Package name must be `{package}_test`
- Function names must start with `Example`
- Use descriptive suffixes for variations (e.g., `_withOptions`, `_error`)
- Include `// Output:` comments for deterministic examples
- Use `// Output: (description)` for non-deterministic output

## Benchmarks

Critical paths must have benchmarks in `*_bench_test.go`:

```go
func BenchmarkFunctionName(b *testing.B) {
    setup := prepareTestData()
    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        FunctionName(setup)
    }
}

func BenchmarkFunctionName_Parallel(b *testing.B) {
    setup := prepareTestData()
    b.ResetTimer()

    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            FunctionName(setup)
        }
    })
}
```

### Benchmark Guidelines

- Use `b.ResetTimer()` after setup code
- Test both sequential and parallel execution
- Include allocation benchmarks with `b.ReportAllocs()`
- Name benchmarks descriptively
- Group related benchmarks together

### Performance Regression Tests

For performance regression tests that verify zero or minimal allocations, use `testing.AllocsPerRun`:

```go
func TestFunctionName_ZeroAlloc(t *testing.T) {
    // Note: Cannot use t.Parallel() with testing.AllocsPerRun
    allocs := testing.AllocsPerRun(100, func() {
        FunctionName(input)
    })
    
    assert.Equal(t, float64(0), allocs, "FunctionName allocated %f times, want 0", allocs)
}
```

**Important**: `testing.AllocsPerRun` cannot be used with `t.Parallel()`. Tests using `testing.AllocsPerRun` must run sequentially to ensure accurate allocation measurements.

## Integration Tests

Integration tests should be in `integration_test.go` or `{feature}_integration_test.go`:

### Standard Integration Tests

For simple integration scenarios, use standard testing:

```go
package package_test

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "rivaas.dev/package"
)

func TestIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }
    
    r := package.MustNew()
    // Integration test code
}
```

### Ginkgo Integration Tests

For complex integration scenarios with multiple phases, nested contexts, or BDD-style organization, use Ginkgo:

```go
package package_test

import (
    "net/http"
    "net/http/httptest"
    "testing"
    
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    "rivaas.dev/package"
)

var _ = Describe("Feature Integration", func() {
    var r *package.Router
    
    BeforeEach(func() {
        r = package.MustNew()
    })
    
    Describe("Scenario A", func() {
        Context("with condition X", func() {
            It("should behave correctly", func() {
                req := httptest.NewRequest("GET", "/path", nil)
                w := httptest.NewRecorder()
                r.ServeHTTP(w, req)
                
                Expect(w.Code).To(Equal(http.StatusOK))
            })
        })
    })
})

func TestFeatureIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }
    RegisterFailHandler(Fail)
    RunSpecs(t, "Feature Integration Suite")
}
```

### Integration Test Guidelines

- **Package**: Use external package (`package {package}_test`) for black-box testing
- **Skip behavior**: Use runtime skip with `testing.Short()` instead of build tags
- **Framework choice**:
  - Use standard `testing` for simple integration tests
  - Use **Ginkgo/Gomega** for complex scenarios with:
    - Multiple test phases
    - Nested contexts and shared setup
    - BDD-style organization
    - Complex assertions with Gomega matchers
- **Test real interactions**: Test full request/response cycles, not just function calls
- **Documentation**: Document required setup in test comments
- **Isolation**: Each test should be independent and clean up after itself

## Test Helpers

Common test utilities should be in `testing.go` or `testutil` package:

```go
package package

import "testing"

// testHelper creates a test instance with default configuration.
func testHelper(t *testing.T) *Config {
    t.Helper()
    return MustNew(WithTestDefaults())
}

// assertError checks if error matches expected.
func assertError(t *testing.T, err error, wantErr bool, msg string) {
    t.Helper()
    if (err != nil) != wantErr {
        t.Errorf("%s: error = %v, wantErr %v", msg, err, wantErr)
    }
}
```

### Test Helper Guidelines

- Use `t.Helper()` in helper functions
- Provide sensible defaults for test instances
- Keep helpers simple and focused
- Document helper functions

## Test Coverage Requirements

- **Unit tests**: All public APIs must have unit tests
- **Example tests**: All public APIs must have example tests
- **Benchmarks**: Critical paths must have benchmarks
- **Integration tests**: Required for packages with external dependencies

## Testing Framework Guidelines

### When to Use Standard Testing

- **Unit tests**: Always use standard `testing` package
- **Simple integration tests**: Single test function, straightforward scenarios
- **Benchmarks**: Standard `testing.B`
- **Example tests**: Standard `Example*` functions

### When to Use Ginkgo/Gomega

- **Complex integration tests** with:
  - Multiple sequential test phases
  - Shared setup/teardown across many tests
  - Nested contexts (e.g., "when X", "when Y")
  - BDD-style organization for readability
  - Complex assertions benefiting from Gomega matchers
- **Feature-focused integration suites**: When testing a specific feature with many scenarios

### Framework Dependencies

- **Standard testing**: Built-in, no dependencies
- **Ginkgo/Gomega**: Add to `go.mod` only for integration test files

  ```bash
  go get github.com/onsi/ginkgo/v2
  go get github.com/onsi/gomega
  ```

## Best Practices

1. **Parallel Execution**: Use `t.Parallel()` for all tests
   - **Exception**: Tests using `testing.AllocsPerRun` cannot use `t.Parallel()` (see Performance Regression Tests section)
2. **Error Messages**: Include descriptive error messages
3. **Test Isolation**: Each test should be independent
4. **Cleanup**: Use `defer` for cleanup operations
5. **Naming**: Use descriptive test and benchmark names
6. **Documentation**: Document complex test scenarios
7. **Performance**: Use benchmarks to track performance regressions

## Package-Specific Notes

### binding

- ✅ Has table-driven tests
- ✅ Has example_test.go
- ❌ Missing benchmarks (needs `bind_bench_test.go`)

### logging

- ✅ Has table-driven tests
- ✅ Has example_test.go
- ✅ Has extensive benchmarks

### metrics

- ✅ Has table-driven tests
- ❌ Missing example_test.go (now added)
- ✅ Has extensive benchmarks

### tracing

- ✅ Has table-driven tests
- ❌ Missing example_test.go (now added)
- ✅ Has benchmarks

### router

- ✅ Has table-driven tests
- ✅ Has example_test.go
- ✅ Has benchmarks
- ✅ Has integration tests (Ginkgo-based for complex scenarios)

### openapi

- ✅ Has table-driven tests
- ❌ Missing example_test.go (now added)
- ✅ Has some benchmarks

### validation

- ✅ Has table-driven tests
- ✅ Has example_test.go
- ✅ Has benchmarks

### errors

- ✅ Has table-driven tests
- ✅ Has example_test.go
- ❌ Missing benchmarks (consider adding if performance-critical)

## Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run benchmarks
go test -bench=. -benchmem ./...

# Run integration tests (all)
go test ./...

# Skip integration tests (short mode)
go test -short ./...

# Run specific Ginkgo suite
go test -run TestFeatureIntegration ./...

# Run example tests
go test -run=Example ./...
```

## Continuous Integration

All tests must pass in CI:

- Unit tests
- Example tests
- Benchmarks (for performance tracking)
- Integration tests (if applicable)
