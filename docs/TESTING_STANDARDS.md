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

## Assertions

**Important**: Do not use manual assertions (e.g., `if` statements with `t.Errorf()`). Always use assertion libraries:

- **Unit tests**: Use `testify/assert` or `testify/require`
- **Integration tests with Ginkgo**: Use Gomega matchers (`Expect(...).To(...)`)
- **Integration tests without Ginkgo**: Use `testify/assert` or `testify/require`

### Why Use Assertion Libraries?

- **Consistent error messages**: Better formatted, more informative failures
- **Less boilerplate**: Cleaner, more readable test code
- **Better debugging**: Assertion libraries provide helpful diff output
- **Standardization**: Consistent patterns across the codebase

### testify/assert vs testify/require

- **`assert`**: Continues test execution after failure (use when you want to check multiple assertions)
- **`require`**: Stops test execution immediately after failure (use when subsequent assertions depend on the first)

## Error Checking

When checking errors in tests, always use the appropriate `testify/assert` or `testify/require` error checking functions instead of manual error checks.

### Available Error Checking Functions

- **`assert.NoError(t, err)` / `require.NoError(t, err)`** - Verify that no error occurred
- **`assert.Error(t, err)` / `require.Error(t, err)`** - Verify that an error occurred (any error)
- **`assert.ErrorIs(t, err, target)` / `require.ErrorIs(t, err, target)`** - Verify that error wraps a specific error value (use with `errors.Is`)
- **`assert.ErrorAs(t, err, target)` / `require.ErrorAs(t, err, target)`** - Verify that error is or wraps a specific error type (use with `errors.As`)
- **`assert.ErrorContains(t, err, substring)` / `require.ErrorContains(t, err, substring)`** - Verify that error message contains a specific substring

### When to Use Each Function

#### `assert.NoError` / `require.NoError`

Use when you expect no error and want to verify success:

```go
result, err := FunctionThatShouldSucceed()
require.NoError(t, err)  // Use require if result is needed for subsequent assertions
assert.Equal(t, expected, result)
```

#### `assert.Error` / `require.Error`

Use when you expect any error but don't need to verify the specific error:

```go
_, err := FunctionThatShouldFail()
assert.Error(t, err)  // Use assert if you want to check multiple things
```

#### `assert.ErrorIs` / `require.ErrorIs`

Use when you need to verify that an error wraps a specific sentinel error value:

```go
import "errors"

var ErrNotFound = errors.New("not found")

_, err := FunctionThatReturnsWrappedError()
assert.ErrorIs(t, err, ErrNotFound)
```

#### `assert.ErrorAs` / `require.ErrorAs`

Use when you need to verify that an error is or wraps a specific error type:

```go
type ValidationError struct {
    Field string
}

_, err := FunctionThatReturnsTypedError()
var validationErr *ValidationError
require.ErrorAs(t, err, &validationErr)  // Use require if you need validationErr for subsequent checks
assert.Equal(t, "email", validationErr.Field)
```

#### `assert.ErrorContains` / `require.ErrorContains`

Use when you need to verify that an error message contains specific text:

```go
_, err := FunctionThatReturnsDescriptiveError()
assert.ErrorContains(t, err, "invalid input")
```

### When to Use `require` vs `assert` for Errors

**Use `require` for error checks when:**

1. **Setup/initialization must succeed** - If subsequent test code depends on the error check passing:

   ```go
   tmpfile, err := os.CreateTemp("", "test-*.txt")
   require.NoError(t, err)  // Must succeed to continue
   defer os.Remove(tmpfile.Name())
   
   // Can safely use tmpfile here
   ```

2. **Critical resource acquisition** - When you need a non-nil value or successful operation to proceed:

   ```go
   db, err := sql.Open("postgres", dsn)
   require.NoError(t, err)  // Must succeed
   require.NotNil(t, db)    // Must not be nil
   
   // Safe to call methods on db
   rows, err := db.Query("SELECT ...")
   ```

3. **Preventing panics** - When a nil value or failed operation would cause a panic:

   ```go
   ctx := pool.Get(5)
   require.NotNil(t, ctx, "Get(5) should not return nil")
   // Can safely use ctx here without panic
   ```

4. **Test execution that other assertions depend on** - When later assertions assume the first error check passed:

   ```go
   err := c.Format(200, data)
   require.NoError(t, err)  // Must succeed for rest of test
   
   // These assertions depend on Format succeeding
   assert.Contains(t, w.Header().Get("Content-Type"), "application/xml")
   assert.Contains(t, w.Body.String(), "<?xml")
   ```

**Use `assert` for error checks when:**

1. **Independent validations** - When you want to verify multiple things even if one fails:

   ```go
   assert.NoError(t, err)
   assert.Equal(t, expected, result)
   assert.Contains(t, message, "success")  // All will be checked even if first fails
   ```

2. **Non-critical error checks** - When failure doesn't prevent other useful checks:

   ```go
   err := optionalOperation()
   assert.NoError(t, err)  // Nice to have, but test can continue
   assert.Equal(t, http.StatusOK, w.Code)
   assert.Contains(t, w.Body.String(), "success")
   ```

### Error Checking Best Practices

1. **Always use assertion functions** - Never use manual `if err != nil` checks with `t.Errorf()`
2. **Choose the right assertion** - Use the most specific assertion that fits your needs:
   - Need exact error? → `assert.ErrorIs` or `assert.ErrorAs`
   - Need error message check? → `assert.ErrorContains`
   - Just need any error? → `assert.Error`
   - Need no error? → `assert.NoError`
3. **Use `require` for critical errors** - If subsequent test code depends on error state, use `require`:

   ```go
   require.NoError(t, err, "setup must succeed")
   // Continue with test that depends on successful setup
   ```

4. **Use `assert` for independent checks** - When you want to see all failures, use `assert`
5. **Include descriptive messages** - Add context to error assertions:

   ```go
   require.NoError(t, err, "failed to parse config file")
   assert.ErrorContains(t, err, "invalid format", "should reject malformed input")
   ```

6. **Prefer `ErrorIs` and `ErrorAs`** - When testing error types, use `ErrorIs` for sentinel errors and `ErrorAs` for error types to ensure compatibility with Go's error wrapping

### Error Checking Exceptions

**Concurrent Test Error Collection**: When testing concurrency, error collection in goroutines may use manual error handling (e.g., sending errors to channels). This is acceptable because:

- Error collection is part of the test infrastructure, not test assertions
- Channels are the idiomatic way to collect errors from goroutines
- Assertions should still be used when validating collected errors

```go
func TestConcurrentOperation(t *testing.T) {
    errors := make(chan error, numGoroutines)
    for range numGoroutines {
        go func() {
            err := doOperation()
            if err != nil {
                errors <- err  // Acceptable: error collection, not assertion
            }
        }()
    }
    close(errors)
    
    // Use assertions when validating collected errors
    for err := range errors {
        assert.NoError(t, err)  // Assertion on collected errors
    }
}
```

### Error Checking Examples

#### Table-Driven Test with Errors

```go
func TestFunctionWithErrors(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name    string
        input   string
        wantErr bool
        errType error
        errMsg  string
    }{
        {
            name:    "success case",
            input:   "valid",
            wantErr: false,
        },
        {
            name:    "not found error",
            input:   "missing",
            wantErr: true,
            errType: ErrNotFound,
        },
        {
            name:    "validation error",
            input:   "invalid",
            wantErr: true,
            errMsg:  "validation",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            result, err := Function(tt.input)
            
            if tt.wantErr {
                assert.Error(t, err)
                if tt.errType != nil {
                    assert.ErrorIs(t, err, tt.errType)
                }
                if tt.errMsg != "" {
                    assert.ErrorContains(t, err, tt.errMsg)
                }
                return
            }
            
            require.NoError(t, err)  // Use require if result is needed
            assert.NotNil(t, result)
            assert.Equal(t, expected, result)
        })
    }
}
```

#### Setup with require, Validation with assert

```go
func TestFileOperation(t *testing.T) {
    t.Parallel()

    // Setup - use require for critical operations
    tmpfile, err := os.CreateTemp("", "test-*.txt")
    require.NoError(t, err, "must create temp file")
    defer os.Remove(tmpfile.Name())

    content := []byte("test content")
    _, err = tmpfile.Write(content)
    require.NoError(t, err, "must write to temp file")
    require.NoError(t, tmpfile.Close(), "must close temp file")

    // Test execution - use require if result is needed
    result, err := ProcessFile(tmpfile.Name())
    require.NoError(t, err, "must process file successfully")
    require.NotNil(t, result, "result must not be nil")

    // Validations - use assert for independent checks
    assert.Equal(t, expected, result.Value)
    assert.Contains(t, result.Message, "success")
    assert.Len(t, result.Items, 3)
}
```

#### Error Type Checking

```go
func TestValidationError(t *testing.T) {
    t.Parallel()

    _, err := ValidateUser("invalid-email")
    
    // Check error type
    var validationErr *ValidationError
    require.ErrorAs(t, err, &validationErr, "must return ValidationError")
    
    // Now safe to use validationErr
    assert.Equal(t, "email", validationErr.Field)
    assert.ErrorContains(t, err, "invalid format")
}
```

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
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            assert.NoError(t, err)
            assert.Equal(t, tt.want, got)
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
- **Always use `testify/assert` or `testify/require`** - never use manual `if` statements with `t.Errorf()`

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

### Example Test Error Handling

**Exception**: Example tests may use `log.Fatal(err)` or similar for error handling. This is acceptable because:

- Examples demonstrate real-world usage patterns
- `log.Fatal()` is idiomatic for example code that should terminate on error
- Examples prioritize clarity and simplicity over test-specific error handling

```go
func Example() {
    a, err := app.New()
    if err != nil {
        log.Fatal(err)  // Acceptable in examples
    }
    // ... example code
}
```

## Benchmarks

Critical paths must have benchmarks in `*_bench_test.go`:

```go
func BenchmarkFunctionName(b *testing.B) {
    setup := prepareTestData()
    b.ResetTimer()
    b.ReportAllocs()

    // Preferred: Go 1.23+ syntax
    for b.Loop() {
        FunctionName(setup)
    }
    
    // Alternative: Traditional syntax (works in all Go versions)
    // for i := 0; i < b.N; i++ {
    //     FunctionName(setup)
    // }
}

func BenchmarkFunctionName_Parallel(b *testing.B) {
    setup := prepareTestData()
    b.ResetTimer()
    b.ReportAllocs()

    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            FunctionName(setup)
        }
    })
}
```

### Benchmark Guidelines

- Use `b.ResetTimer()` after setup code
- Use `b.ReportAllocs()` to track memory allocations
- **Prefer `b.Loop()`** for Go 1.23+ (simpler and more idiomatic)
- Use traditional `for i := 0; i < b.N; i++` pattern for compatibility with older Go versions
- Test both sequential and parallel execution
- Name benchmarks descriptively
- Group related benchmarks together

### Benchmark Error Handling

**Exception**: Benchmarks may use `b.Fatal(err)` or `b.Skip()` directly for setup failures. This is acceptable because:

- Benchmarks focus on performance measurement, not detailed error reporting
- Setup failures should stop the benchmark immediately
- `b.Fatal()` provides appropriate behavior for benchmark context

```go
func BenchmarkFunctionName(b *testing.B) {
    setup, err := prepareTestData()
    if err != nil {
        b.Fatal(err)  // Acceptable in benchmarks
    }
    b.ResetTimer()
    // ... benchmark code
}
```

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

## Fuzz Tests

Fuzz tests use Go's built-in fuzzing framework to find edge cases and unexpected inputs:

```go
func FuzzFunctionName(f *testing.F) {
    // Seed corpus
    f.Add("valid-input")
    f.Add("")
    f.Add("edge-case")

    f.Fuzz(func(t *testing.T, input string) {
        result, err := FunctionName(input)
        // Should never panic, even with invalid input
        if err != nil {
            var ve *ValidationError
            if !errors.As(err, &ve) {
                t.Errorf("expected ValidationError, got %T: %v", err, err)
            }
        }
    })
}
```

### Fuzz Test Guidelines

- Use `f.Add()` to seed the corpus with known good/bad inputs
- Test should never panic, even with invalid input
- Validate error types when errors are expected
- Name fuzz tests with `Fuzz` prefix

### Fuzz Test Error Handling

**Exception**: Fuzz tests may use `t.Errorf()` directly for validating error types. This is acceptable because:

- Fuzz tests validate error types and behavior, not just success cases
- Direct `t.Errorf()` provides clear feedback about unexpected error types
- Fuzz tests have different failure semantics than regular unit tests

```go
f.Fuzz(func(t *testing.T, input string) {
    _, err := FunctionName(input)
    if err != nil {
        var ve *ValidationError
        if !errors.As(err, &ve) {
            t.Errorf("expected ValidationError, got %T: %v", err, err)  // Acceptable in fuzz tests
        }
    }
})
```

## Suite Tests

For tests that require shared setup and teardown across multiple test cases, you may use the `testify/suite` pattern:

```go
package package_test

import (
    "testing"
    
    "github.com/stretchr/testify/suite"
    "rivaas.dev/package"
)

// FeatureSuite tests a feature with shared setup.
type FeatureSuite struct {
    suite.Suite
    testInstance *package.Instance
}

func (s *FeatureSuite) SetupTest() {
    // Fresh instance for each test
    instance, err := package.New()
    s.Require().NoError(err)
    s.testInstance = instance
}

func (s *FeatureSuite) TearDownTest() {
    // Cleanup if needed
    s.testInstance = nil
}

func (s *FeatureSuite) TestFeatureA() {
    s.NotNil(s.testInstance)
    // Test code using s.testInstance
}

func (s *FeatureSuite) TestFeatureB() {
    s.NotNil(s.testInstance)
    // Test code using s.testInstance
}

func TestFeatureSuite(t *testing.T) {
    suite.Run(t, new(FeatureSuite))
}
```

### Suite Test Guidelines

- Use `suite.Suite` embedded struct for shared test infrastructure
- Implement `SetupTest()` and `TearDownTest()` for per-test setup/cleanup
- Use `s.Require()` and `s.Assert()` (from suite.Suite) instead of direct testify calls
- Name suite structs descriptively (e.g., `FeatureSuite`, `IntegrationSuite`)
- Create a single `TestXxx` function that calls `suite.Run()`

### When to Use Suite Tests

Use suite tests when:

- Multiple tests share complex setup/teardown logic
- Tests need access to shared resources (databases, servers, etc.)
- Setup is expensive and should be reused across tests
- Tests are logically grouped and benefit from shared context

**Note**: For simple tests, prefer table-driven tests with `t.Parallel()`. Suite tests run sequentially by default.

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

**Important**: Ginkgo requires exactly **one `RunSpecs` call per package**. Multiple test files are allowed, but only one file should contain the `TestXxx` function that calls `RunSpecs`.

#### Suite File Pattern

Create a single `{package}_integration_suite_test.go` file as the entry point:

```go
// {package}_integration_suite_test.go
package package_test

import (
    "testing"
    
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

func TestPackageIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }
    RegisterFailHandler(Fail)
    RunSpecs(t, "Package Integration Suite")
}
```

#### Test Files with Describe Blocks

All other integration test files should **only** contain `var _ = Describe(...)` blocks. Do **not** include `TestXxx` functions or `RunSpecs` calls in these files:

```go
// integration_test.go
package package_test

import (
    "net/http"
    "net/http/httptest"
    
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

// ❌ DO NOT include TestXxx or RunSpecs in this file
```

#### Multiple Test Files Example

You can organize tests across multiple files, each with their own `Describe` blocks:

```go
// integration_test.go
package package_test

var _ = Describe("Router Integration", func() {
    // Integration tests here
})

// integration_stress_test.go
package package_test

var _ = Describe("Router Stress Tests", Label("stress"), func() {
    // Stress tests here
})

// versioning_integration_test.go
package package_test

var _ = Describe("Versioning Integration", Label("integration", "versioning"), func() {
    // Versioning tests here
})

// {package}_integration_suite_test.go - ONLY file with RunSpecs
package package_test

func TestPackageIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }
    RegisterFailHandler(Fail)
    RunSpecs(t, "Package Integration Suite")
}
```

#### Using Labels for Filtering

Use labels to organize and filter tests when running. Labels help categorize tests (e.g., `stress`, `slow`, `integration`, `versioning`) without requiring separate test binaries:

```go
// integration_stress_test.go
var _ = Describe("Router Stress Tests", Label("stress", "slow"), func() {
    It("should handle high concurrent load", Label("stress"), func() {
        // Stress test implementation
    })
})

// versioning_integration_test.go
var _ = Describe("Versioning Integration", Label("integration", "versioning"), func() {
    It("should route by version header", Label("versioning"), func() {
        // Versioning test implementation
    })
})
```

Run tests with label filters:

```bash
# Run only stress tests
ginkgo -label-filter=stress ./package

# Run everything except stress tests
ginkgo -label-filter='!stress' ./package

# Run tests with multiple labels (AND)
ginkgo -label-filter='integration && versioning' ./package

# Run tests with any of the labels (OR)
ginkgo -label-filter='stress || slow' ./package

# Using go test (requires ginkgo CLI)
go test -ginkgo.label-filter=stress ./package
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
- **Suite structure**:
  - **Exactly one** `{package}_integration_suite_test.go` file with `TestXxx` function calling `RunSpecs`
  - **Multiple** test files allowed (e.g., `integration_test.go`, `integration_stress_test.go`), each with `var _ = Describe(...)` blocks
  - **Never** call `RunSpecs` more than once per package
  - Use **labels** (`Label("stress")`, `Label("integration")`) to categorize tests for filtering
- **Test real interactions**: Test full request/response cycles, not just function calls
- **Documentation**: Document required setup in test comments
- **Isolation**: Each test should be independent and clean up after itself

## Test Helpers

Common test utilities should be in `testing.go` or `testutil` package:

```go
package package

import (
    "testing"
    
    "github.com/stretchr/testify/assert"
)

// testHelper creates a test instance with default configuration.
func testHelper(t *testing.T) *Config {
    t.Helper()
    return MustNew(WithTestDefaults())
}

// assertError checks if error matches expected.
func assertError(t *testing.T, err error, wantErr bool, msg string) {
    t.Helper()
    if wantErr {
        assert.Error(t, err, msg)
    } else {
        assert.NoError(t, err, msg)
    }
}
```

### Test Helper Guidelines

- Use `t.Helper()` in helper functions
- Provide sensible defaults for test instances
- Keep helpers simple and focused
- Document helper functions
- **Use `testify/assert` or `testify/require`** in helpers - never use manual assertions

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
2. **Assertions**: Always use `testify/assert` or `testify/require` - **never use manual assertions** (e.g., `if` statements with `t.Errorf()`)
3. **Error Messages**: Include descriptive error messages
4. **Test Isolation**: Each test should be independent
5. **Cleanup**: Use `defer` for cleanup operations
6. **Naming**: Use descriptive test and benchmark names
7. **Documentation**: Document complex test scenarios
8. **Performance**: Use benchmarks to track performance regressions

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
