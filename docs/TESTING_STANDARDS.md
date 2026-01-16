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

## Test Naming Conventions

Use consistent naming patterns for tests:

| Pattern | Use Case | Example |
|---------|----------|---------|
| `TestFunctionName` | Basic function test | `TestParseConfig` |
| `TestFunctionName_Scenario` | Specific scenario | `TestParseConfig_EmptyInput` |
| `TestFunctionName_ErrorCase` | Error scenarios | `TestParseConfig_InvalidJSON` |
| `TestType_MethodName` | Method test | `TestRouter_ServeHTTP` |
| `TestType_MethodName_Scenario` | Method with scenario | `TestRouter_ServeHTTP_NotFound` |

### Subtest Naming

For table-driven tests, use descriptive names that explain the scenario:

```go
tests := []struct {
    name string
    // ...
}{
    {name: "valid email address"},           // ✅ Descriptive
    {name: "empty string returns error"},    // ✅ Describes expected behavior
    {name: "test1"},                         // ❌ Not descriptive
    {name: "case 1"},                        // ❌ Not helpful
}
```

### Grouping with Subtests

Use nested `t.Run()` for logical grouping of related tests:

```go
func TestUser(t *testing.T) {
    t.Parallel()

    t.Run("Create", func(t *testing.T) {
        t.Parallel()
        t.Run("valid input succeeds", func(t *testing.T) {
            t.Parallel()
            // test code
        })
        t.Run("invalid email returns error", func(t *testing.T) {
            t.Parallel()
            // test code
        })
    })

    t.Run("Delete", func(t *testing.T) {
        t.Parallel()
        t.Run("existing user succeeds", func(t *testing.T) {
            t.Parallel()
            // test code
        })
    })
}
```

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

## Test Data Management

### The `testdata` Directory

Go has special handling for `testdata/` directories:

- Automatically ignored by `go build`
- Used for test fixtures, golden files, and sample data
- Accessible via relative path from test files

```
package/
├── handler.go
├── handler_test.go
└── testdata/
    ├── fixtures/
    │   ├── valid_request.json
    │   └── invalid_request.json
    └── golden/
        ├── expected_output.json
        └── expected_error.txt
```

### Loading Test Data

```go
func TestHandler(t *testing.T) {
    t.Parallel()

    // Load test fixture
    input, err := os.ReadFile("testdata/fixtures/valid_request.json")
    require.NoError(t, err)

    // Use in test
    result, err := ProcessRequest(input)
    require.NoError(t, err)

    // Compare with golden file
    expected, err := os.ReadFile("testdata/golden/expected_output.json")
    require.NoError(t, err)
    assert.JSONEq(t, string(expected), string(result))
}
```

### Golden File Testing

Golden files store expected output for comparison. Use the `-update` flag pattern to regenerate:

```go
var updateGolden = flag.Bool("update", false, "update golden files")

func TestOutput_Golden(t *testing.T) {
    result := GenerateOutput()
    goldenPath := "testdata/golden/output.txt"

    if *updateGolden {
        err := os.WriteFile(goldenPath, []byte(result), 0644)
        require.NoError(t, err)
        return
    }

    expected, err := os.ReadFile(goldenPath)
    require.NoError(t, err)
    assert.Equal(t, string(expected), result)
}
```

Update golden files with:

```bash
go test -update ./...
```

### Test Data Guidelines

- Keep test data files small and focused
- Use descriptive file names that indicate their purpose
- Document the structure of complex test data files
- Version control all test data (it's part of the test suite)
- For large datasets, consider generating them programmatically

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
- **Use `b.Context()`** instead of `context.Background()` (Go 1.24+)

### Benchmark Context (Go 1.24+)

In Go 1.24 and later, benchmarks should use `b.Context()` instead of `context.Background()`:

```go
func BenchmarkWithContext(b *testing.B) {
    b.ResetTimer()
    b.ReportAllocs()

    for b.Loop() {
        // ✅ Preferred: Use b.Context()
        service.Operation(b.Context())
        
        // ❌ Avoid: context.Background()
        // service.Operation(context.Background())
    }
}
```

**Benefits of `b.Context()`:**

- Automatically cancelled when the benchmark ends
- Consistent with test context usage (`t.Context()`)
- Caught by the `usetesting` linter

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

### Running Fuzz Tests

```bash
# Run fuzz test for 30 seconds
go test -fuzz=FuzzFunctionName -fuzztime=30s ./package

# Run with specific corpus entry (to reproduce a failure)
go test -run=FuzzFunctionName/corpus_entry_name ./package

# Run all fuzz tests as regular tests (uses seed corpus only)
go test -run=Fuzz ./...
```

### Reproducing Fuzz Failures

When a fuzz test fails, Go saves the failing input to `testdata/fuzz/{FuzzTestName}/`:

```
package/
└── testdata/
    └── fuzz/
        └── FuzzFunctionName/
            └── 8a4f2b3c...  # Failing input
```

To reproduce:

```bash
# Run the specific failing case
go test -run=FuzzFunctionName/8a4f2b3c ./package

# Or run all corpus entries
go test -run=FuzzFunctionName ./package
```

**Important**: Commit failing corpus entries to version control to prevent regressions.

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

Integration tests should be in `integration_test.go` or `{feature}_integration_test.go`.

### Build Tags for Test Separation

We use Go build tags to separate unit tests from integration tests at compile time:

| Test Type | Build Tag | Run Command |
|-----------|-----------|-------------|
| Unit tests | `//go:build !integration` | `go test ./...` |
| Integration tests | `//go:build integration` | `go test -tags=integration ./...` |

**Why build tags instead of `testing.Short()`?**

- **Compile-time separation**: Tests are excluded at build time, not skipped at runtime
- **Cleaner coverage reports**: Unit and integration coverage are truly separate
- **Faster unit test runs**: Integration test code isn't even compiled
- **CI/CD friendly**: Easy to run different test suites in parallel pipelines

### Standard Integration Tests

For simple integration scenarios, use standard testing with the `integration` build tag:

```go
//go:build integration

package package_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "rivaas.dev/package"
)

func TestIntegration(t *testing.T) {
    r := package.MustNew()
    // Integration test code
}
```

**Important**: The `//go:build integration` tag must appear after the license header and before the `package` declaration, with a blank line before and after.

### Ginkgo Integration Tests

For complex integration scenarios with multiple phases, nested contexts, or BDD-style organization, use Ginkgo:

**Important**: Ginkgo requires exactly **one `RunSpecs` call per package**. Multiple test files are allowed, but only one file should contain the `TestXxx` function that calls `RunSpecs`.

#### Suite File Pattern

Create a single `{package}_integration_suite_test.go` file as the entry point:

```go
// {package}_integration_suite_test.go

//go:build integration

package package_test

import (
    "testing"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

func TestPackageIntegration(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Package Integration Suite")
}
```

#### Test Files with Describe Blocks

All other integration test files should **only** contain `var _ = Describe(...)` blocks. Do **not** include `TestXxx` functions or `RunSpecs` calls in these files:

```go
// integration_test.go

//go:build integration

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

You can organize tests across multiple files, each with their own `Describe` blocks. All files must have the `//go:build integration` tag:

```go
// integration_test.go
//go:build integration

package package_test

var _ = Describe("Router Integration", func() {
    // Integration tests here
})
```

```go
// integration_stress_test.go
//go:build integration

package package_test

var _ = Describe("Router Stress Tests", Label("stress"), func() {
    // Stress tests here
})
```

```go
// versioning_integration_test.go
//go:build integration

package package_test

var _ = Describe("Versioning Integration", Label("versioning"), func() {
    // Versioning tests here
})
```

```go
// {package}_integration_suite_test.go - ONLY file with RunSpecs
//go:build integration

package package_test

import (
    "testing"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

func TestPackageIntegration(t *testing.T) {
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
- **Build tag**: All integration test files must have `//go:build integration` tag
- **Framework choice**:
  - Use standard `testing` for simple integration tests
  - Use **Ginkgo/Gomega** for complex scenarios with:
    - Multiple test phases
    - Nested contexts and shared setup
    - BDD-style organization
    - Complex assertions with Gomega matchers
- **Suite structure** (for Ginkgo tests):
  - **Exactly one** `{package}_integration_suite_test.go` file with `TestXxx` function calling `RunSpecs`
  - **Multiple** test files allowed (e.g., `integration_test.go`, `integration_stress_test.go`), each with `var _ = Describe(...)` blocks
  - **All files** must have `//go:build integration` tag
  - **Never** call `RunSpecs` more than once per package
  - Use **labels** (`Label("stress")`, `Label("versioning")`) to further categorize tests for filtering within integration tests
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

## Mocking and Test Doubles

### Interface-Based Mocking (Preferred)

Go's implicit interface implementation makes interface-based mocking the idiomatic approach:

```go
// Define interface for the dependency
type UserRepository interface {
    FindByID(ctx context.Context, id string) (*User, error)
    Save(ctx context.Context, user *User) error
}

// Production implementation
type PostgresUserRepository struct {
    db *sql.DB
}

// Test implementation (fake)
type fakeUserRepository struct {
    users map[string]*User
    err   error
}

func (f *fakeUserRepository) FindByID(ctx context.Context, id string) (*User, error) {
    if f.err != nil {
        return nil, f.err
    }
    return f.users[id], nil
}

func (f *fakeUserRepository) Save(ctx context.Context, user *User) error {
    if f.err != nil {
        return f.err
    }
    f.users[user.ID] = user
    return nil
}

// Test using the fake
func TestUserService_GetUser(t *testing.T) {
    t.Parallel()

    repo := &fakeUserRepository{
        users: map[string]*User{
            "123": {ID: "123", Name: "Test User"},
        },
    }
    service := NewUserService(repo)

    user, err := service.GetUser(context.Background(), "123")
    require.NoError(t, err)
    assert.Equal(t, "Test User", user.Name)
}
```

### Using testify/mock

For complex mocking scenarios, use `testify/mock`:

```go
import "github.com/stretchr/testify/mock"

type MockUserRepository struct {
    mock.Mock
}

func (m *MockUserRepository) FindByID(ctx context.Context, id string) (*User, error) {
    args := m.Called(ctx, id)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*User), args.Error(1)
}

func (m *MockUserRepository) Save(ctx context.Context, user *User) error {
    args := m.Called(ctx, user)
    return args.Error(0)
}

func TestUserService_GetUser_WithMock(t *testing.T) {
    t.Parallel()

    mockRepo := new(MockUserRepository)
    mockRepo.On("FindByID", mock.Anything, "123").Return(&User{ID: "123", Name: "Test"}, nil)

    service := NewUserService(mockRepo)
    user, err := service.GetUser(context.Background(), "123")

    require.NoError(t, err)
    assert.Equal(t, "Test", user.Name)
    mockRepo.AssertExpectations(t)
}
```

### Types of Test Doubles

| Type | Purpose | When to Use |
|------|---------|-------------|
| **Fake** | Working implementation with shortcuts | Database in-memory, simplified logic |
| **Stub** | Returns predetermined responses | Fixed return values for specific inputs |
| **Mock** | Verifies interactions occurred | When you need to verify method calls |
| **Spy** | Records calls for later verification | When you need call history |

### Mocking Guidelines

1. **Prefer fakes over mocks** - Fakes are simpler and test real behavior
2. **Mock at boundaries** - Mock external services, not internal components
3. **Keep interfaces small** - Smaller interfaces are easier to mock
4. **Don't mock what you don't own** - Wrap third-party APIs with your own interface
5. **Use `mock.Anything`** for unimportant arguments
6. **Always call `AssertExpectations(t)`** when using testify/mock

### HTTP Client Mocking

For HTTP clients, use `httptest.Server`:

```go
func TestAPIClient_FetchData(t *testing.T) {
    t.Parallel()

    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "/api/data", r.URL.Path)
        assert.Equal(t, "Bearer token123", r.Header.Get("Authorization"))

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte(`{"id": "123", "name": "test"}`))
    }))
    t.Cleanup(server.Close)

    client := NewAPIClient(server.URL, "token123")
    data, err := client.FetchData(context.Background())

    require.NoError(t, err)
    assert.Equal(t, "123", data.ID)
}
```

## HTTP Testing Patterns

### Testing HTTP Handlers

Use `httptest` for testing HTTP handlers:

```go
func TestHandler_GetUser(t *testing.T) {
    t.Parallel()

    handler := NewUserHandler(mockRepo)

    req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
    req.Header.Set("Content-Type", "application/json")

    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
    assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

    var response User
    err := json.NewDecoder(w.Body).Decode(&response)
    require.NoError(t, err)
    assert.Equal(t, "123", response.ID)
}
```

### Testing with Request Body

```go
func TestHandler_CreateUser(t *testing.T) {
    t.Parallel()

    body := strings.NewReader(`{"name": "Test User", "email": "test@example.com"}`)
    req := httptest.NewRequest(http.MethodPost, "/users", body)
    req.Header.Set("Content-Type", "application/json")

    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)

    assert.Equal(t, http.StatusCreated, w.Code)
}
```

### Testing Middleware

```go
func TestAuthMiddleware(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name           string
        authHeader     string
        wantStatusCode int
    }{
        {
            name:           "valid token",
            authHeader:     "Bearer valid-token",
            wantStatusCode: http.StatusOK,
        },
        {
            name:           "missing header",
            authHeader:     "",
            wantStatusCode: http.StatusUnauthorized,
        },
        {
            name:           "invalid token",
            authHeader:     "Bearer invalid",
            wantStatusCode: http.StatusUnauthorized,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusOK)
            })

            handler := AuthMiddleware(nextHandler)

            req := httptest.NewRequest(http.MethodGet, "/protected", nil)
            if tt.authHeader != "" {
                req.Header.Set("Authorization", tt.authHeader)
            }

            w := httptest.NewRecorder()
            handler.ServeHTTP(w, req)

            assert.Equal(t, tt.wantStatusCode, w.Code)
        })
    }
}
```

### Testing with Cookies

```go
func TestHandler_WithSession(t *testing.T) {
    t.Parallel()

    req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
    req.AddCookie(&http.Cookie{
        Name:  "session_id",
        Value: "valid-session-123",
    })

    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)

    // Check response cookies
    cookies := w.Result().Cookies()
    assert.Len(t, cookies, 1)
    assert.Equal(t, "session_id", cookies[0].Name)
}
```

## Context and Timeout Patterns

### Testing with Context Timeout

```go
func TestService_WithTimeout(t *testing.T) {
    t.Parallel()

    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    t.Cleanup(cancel)

    result, err := service.SlowOperation(ctx)

    // If operation is expected to complete
    require.NoError(t, err)
    assert.NotNil(t, result)

    // Or if testing timeout behavior
    // assert.ErrorIs(t, err, context.DeadlineExceeded)
}
```

### Testing Context Cancellation

```go
func TestService_ContextCancellation(t *testing.T) {
    t.Parallel()

    ctx, cancel := context.WithCancel(context.Background())

    // Start operation in goroutine
    errCh := make(chan error, 1)
    go func() {
        _, err := service.LongRunningOperation(ctx)
        errCh <- err
    }()

    // Cancel after short delay
    time.Sleep(10 * time.Millisecond)
    cancel()

    // Verify cancellation was handled
    err := <-errCh
    assert.ErrorIs(t, err, context.Canceled)
}
```

### Testing with Context Values

```go
func TestHandler_WithContextValue(t *testing.T) {
    t.Parallel()

    ctx := context.WithValue(context.Background(), userIDKey, "user-123")
    req := httptest.NewRequest(http.MethodGet, "/profile", nil).WithContext(ctx)

    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
}
```

### Using Test Context (Go 1.24+)

In Go 1.24 and later, tests should use `t.Context()` instead of `context.Background()`. The test context is automatically cancelled when the test completes, ensuring proper cleanup:

```go
func TestWithContext(t *testing.T) {
    t.Parallel()

    // ✅ Preferred: Use t.Context() - automatically cancelled when test ends
    ctx := t.Context()
    
    // ❌ Avoid: context.Background() doesn't benefit from test lifecycle
    // ctx := context.Background()

    result, err := service.Operation(ctx)
    require.NoError(t, err)
    assert.NotNil(t, result)
}
```

**Benefits of `t.Context()`:**

- Automatically cancelled when the test ends (including on failure)
- Helps detect operations that don't respect context cancellation
- Ensures goroutines started during tests are properly cleaned up
- Caught by the `usetesting` linter

**When to use `context.Background()` instead:**

- When you need a context that outlives the test (rare)
- When testing behavior that specifically requires a non-cancellable context
- In `testing.AllocsPerRun` callbacks (which don't have access to `t`)

## Test Coverage Requirements

- **Unit tests**: All public APIs must have unit tests
- **Example tests**: All public APIs must have example tests
- **Benchmarks**: Critical paths must have benchmarks
- **Integration tests**: Required for packages with external dependencies

### Coverage Thresholds

| Package Type | Minimum Coverage | Target Coverage |
|--------------|------------------|-----------------|
| Core packages (`router`, `binding`, `validation`) | 80% | 90% |
| Utility packages (`logging`, `metrics`, `tracing`) | 75% | 85% |
| Integration packages | 70% | 80% |

### Measuring Coverage

```bash
# Package coverage
go test -cover ./package

# Detailed coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Coverage by function
go tool cover -func=coverage.out

# Check coverage threshold (CI)
go test -coverprofile=coverage.out ./...
COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
if (( $(echo "$COVERAGE < 80" | bc -l) )); then
    echo "Coverage $COVERAGE% is below 80% threshold"
    exit 1
fi
```

### What to Cover

**Must cover:**
- All public APIs (exported functions, methods, types)
- Error paths and edge cases
- Critical business logic
- Security-sensitive code

**May skip:**
- Generated code
- Simple getters/setters
- Panic recovery (test separately)
- Platform-specific code (use build tags)

## Flaky Test Handling

### Identifying Flaky Tests

Flaky tests pass and fail intermittently without code changes. Common causes:

- **Race conditions** - Concurrent access to shared state
- **Timing dependencies** - `time.Sleep()`, network timeouts
- **External dependencies** - Network, filesystem, databases
- **Test pollution** - Tests affecting each other's state
- **Resource exhaustion** - File descriptors, goroutines, memory

### Handling Flaky Tests

1. **Mark with comment** - Document the flakiness:

   ```go
   // FLAKY: Sometimes fails due to timing - see issue #123
   func TestFlaky(t *testing.T) {
       // ...
   }
   ```

2. **Skip temporarily** while investigating:

   ```go
   func TestFlaky(t *testing.T) {
       t.Skip("FLAKY: Skipping until #123 is resolved")
       // ...
   }
   ```

3. **Use retry for integration tests** (sparingly):

   ```go
   func TestWithRetry(t *testing.T) {
       var lastErr error
       for i := 0; i < 3; i++ {
           if err := runTest(); err == nil {
               return
           } else {
               lastErr = err
               t.Logf("Attempt %d failed: %v", i+1, err)
           }
       }
       t.Fatalf("Test failed after 3 attempts: %v", lastErr)
   }
   ```

### Preventing Flaky Tests

1. **Use `t.Parallel()`** - Forces tests to be independent
2. **Avoid `time.Sleep()`** - Use channels, sync primitives, or polling
3. **Use `t.Cleanup()`** - Ensures cleanup runs even on failure
4. **Isolate test data** - Each test should use unique data
5. **Mock external services** - Don't rely on real networks in unit tests
6. **Use `-race` flag** - Detect race conditions early

### Flaky Test Policy

- **Never merge known flaky tests** to main branch
- **Fix or skip** - Either fix the root cause or skip with an issue reference
- **Track flaky tests** - Create issues for each flaky test
- **Review regularly** - Periodically check skipped tests

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

5. **Cleanup with `t.Cleanup()`**: Prefer `t.Cleanup()` over `defer` for test cleanup:

   ```go
   func TestWithResource(t *testing.T) {
       t.Parallel()

       // t.Cleanup is preferred - works correctly in subtests and parallel tests
       resource := createResource()
       t.Cleanup(func() {
           resource.Close()
       })

       // Use resource...
   }
   ```

   **Why `t.Cleanup()` over `defer`:**
   - Runs after the test completes, including all subtests
   - Works correctly with `t.Parallel()`
   - Cleanup functions run in LIFO order
   - Runs even if the test calls `t.FailNow()` or `t.Fatal()`

6. **Naming**: Use descriptive test and benchmark names (see [Test Naming Conventions](#test-naming-conventions))

7. **Documentation**: Document complex test scenarios

8. **Performance**: Use benchmarks to track performance regressions

9. **Race Detection**: Always run tests with race detector in CI:

   ```bash
   go test -race ./...
   ```

10. **Deterministic Tests**: Avoid tests that depend on:
    - Current time (use clock injection)
    - Random values (use fixed seeds or mocks)
    - Network availability (use mocks)
    - Filesystem state (use temp directories)

## Running Tests

```bash
# Run unit tests only (default, excludes integration tests)
go test ./...

# Run unit tests with verbose output
go test -v ./...

# Run unit tests with race detection (REQUIRED in CI)
go test -race ./...

# Run integration tests (with race detection)
go test -tags=integration -race ./...

# Run unit tests with coverage
go test -cover ./...

# Run unit tests with coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Run integration tests with coverage report
go test -tags=integration -coverprofile=coverage-integration.out ./...

# Run benchmarks
go test -bench=. -benchmem ./...

# Run benchmarks with count for statistical significance
go test -bench=. -benchmem -count=10 ./...

# Run specific test by name
go test -run TestFunctionName ./...

# Run tests matching pattern
go test -run "TestUser.*" ./...

# Run specific Ginkgo suite
go test -run TestFeatureIntegration ./...

# Run example tests
go test -run=Example ./...

# Run fuzz tests (30 second limit)
go test -fuzz=FuzzFunctionName -fuzztime=30s ./package

# Run tests with timeout
go test -timeout 5m ./...

# Run tests in specific package
go test ./router/...

# List tests without running
go test -list ".*" ./...
```

### CI Pipeline Commands

```bash
# Unit tests with race detection and coverage (CI)
go test -race -coverprofile=coverage.out -timeout 10m ./...

# Integration tests with race detection and coverage (CI)
go test -tags=integration -race -coverprofile=coverage-integration.out -timeout 10m ./...

# Verify coverage threshold
go tool cover -func=coverage.out | grep total

# Generate coverage badge data
go tool cover -func=coverage.out | grep total | awk '{print $3}'
```

### Nix Commands (Recommended)

If using the Nix development environment:

```bash
# Run unit tests (fast, no coverage)
nix run .#test

# Run unit tests with race detection and coverage
nix run .#test-race

# Run integration tests with race detection and coverage
nix run .#test-integration
```

## Continuous Integration

All tests must pass in CI:

- Unit tests (with race detection)
- Example tests
- Benchmarks (for performance tracking)
- Integration tests (if applicable)
- Coverage threshold checks

### CI Checklist

| Check | Command | Required |
|-------|---------|----------|
| Unit tests | `go test -race ./...` | ✅ Yes |
| Integration tests | `go test -tags=integration -race ./...` | ✅ Yes |
| Coverage | `go test -coverprofile=coverage.out ./...` | ✅ Yes |
| Benchmarks | `go test -bench=. ./...` | ⚠️ Recommended |
| Lint | `golangci-lint run` | ✅ Yes |
| Fuzz (seed corpus) | `go test -run=Fuzz ./...` | ⚠️ Recommended |

### Example CI Workflow

```yaml
jobs:
  unit-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Run unit tests with race detection
        run: go test -race -coverprofile=coverage.out -timeout 10m ./...

      - name: Upload coverage
        uses: actions/upload-artifact@v4
        with:
          name: coverage-unit
          path: coverage.out

  integration-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Run integration tests with race detection
        run: go test -tags=integration -race -coverprofile=coverage-integration.out -timeout 10m ./...

      - name: Upload coverage
        uses: actions/upload-artifact@v4
        with:
          name: coverage-integration
          path: coverage-integration.out

  coverage-report:
    needs: [unit-test, integration-test]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Download coverage artifacts
        uses: actions/download-artifact@v4

      - name: Check coverage threshold
        run: |
          COVERAGE=$(go tool cover -func=coverage-unit/coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          echo "Unit test coverage: $COVERAGE%"
```
