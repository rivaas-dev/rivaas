# Go Code Documentation Standards

This document defines the standards and rules for writing Go code documentation (GoDoc style) in the Rivaas router codebase.

## Primary Goal

Write clear, idiomatic Go documentation (GoDoc style) that explains **what the code does**, **how to use it**, and **its input/output behavior** — without discussing performance or algorithms.

---

## ❌ Absolutely Do NOT Mention

Do not include any references to:

### Performance Characteristics

- "fast", "low-latency", "high-performance", "efficient"
- "optimized", "quick", "rapid", "swift"
- Any comparative performance language

### Algorithmic Details or Complexity

- Big-O notation (e.g., O(1), O(n), O(log n))
- Time/space complexity descriptions
- Computational cost analysis
- Algorithm names used to imply performance

### Benchmark Results or Optimization

- "zero allocations", "allocation-free"
- "optimized for speed"
- "benchmark shows..."
- "50% faster", "X times faster"
- Any quantitative performance claims

### Memory Usage Details

- "low memory usage", "reduced allocations"
- "minimal memory footprint"
- "memory-efficient"
- Specific byte counts or memory measurements
- GC (garbage collection) pressure mentions

### Decorative or Non-Informative Comments

- Visual separators like `// ========` or `// --------`
- ASCII art or decorative boxes
- Empty comment lines for spacing (use actual blank lines instead)
- Comments that add no informational value

### TODO/FIXME Comments About Code Movement

- "TODO: move this to..."
- "FIXME: this should be in..."
- "NOTE: consider moving to..."
- Comments indicating code should be reorganized or relocated

**Rationale:** If code needs to be moved or reorganized, do it immediately rather than leaving comments about it. Comments should document the current state of the code, not future intentions for refactoring.

### Code Organization and File History Comments

- "merged from...", "moved from...", "originally in..."
- "Benchmarks from X file", "Tests merged from Y"
- File reorganization history comments
- Comments explaining code consolidation or file splitting

**Examples to avoid:**

- `// Comparison Benchmarks (merged from accept_comparison_bench_test.go)`
- `// Atomic Benchmarks (merged from atomic_bench_test.go)`
- `// Functions below moved from utils.go`
- `// This code was originally in legacy_handler.go`

**Rationale:** File organization and code history are tracked by version control (git). Comments should document what the code does and how to use it, not the history of how files were reorganized. Git blame and commit history provide better tools for understanding code evolution.

**What to do instead:**

- If grouping related code, use descriptive section comments that explain the purpose:
  - ✅ `// BenchmarkAcceptsComparison compares Accept header parsing across different scenarios.`
  - ❌ `// Comparison Benchmarks (merged from accept_comparison_bench_test.go)`
- Let the code structure and function names speak for themselves
- Remove history comments entirely if they don't explain functionality

**If the code implies such characteristics, omit them entirely.**

---

## ✔️ What You SHOULD Write

Your documentation **must** focus on:

### Purpose

- What the function, type, or method does
- Why it exists
- When it should be used

### Functionality

- What it does in simple, direct terms
- How it transforms inputs to outputs
- Step-by-step behavior (when helpful)

### Usage

- How to use it (brief examples allowed)
- Common use cases
- Integration patterns

### Usage Examples in Documentation

Public functionalities should include code examples directly in their GoDoc comments:

- Format examples using **tab-indented** code blocks with `// Example:` header
- Show **typical usage patterns** and **common integration scenarios**
- Keep examples **concise** - focus on demonstrating the API, not full applications
- Examples should be **runnable** - use valid Go code that compiles
- Place examples after the main description and before parameter/return documentation

**Important:** GoDoc requires **tab indentation** (not spaces) for code blocks to render correctly.

**Inline example format:**

```go
// FunctionName does something useful.
// It processes the input and returns a result.
//
// Example:
//
//	result := FunctionName("input")
//	fmt.Println(result)
//
// Parameters:
//   - input: description
func FunctionName(input string) string { ... }
```

**Runnable Example functions (preferred for public APIs):**

For public APIs, prefer creating `Example` functions in `*_test.go` files. These are verified by `go test` and rendered by godoc:

```go
// In example_test.go
func ExampleFunctionName() {
	result := FunctionName("input")
	fmt.Println(result)
	// Output: expected output
}
```

### Parameters and Return Values

- What each parameter represents
- What values are returned
- Error conditions and their meanings

### Behavior and Edge Cases

- Expected behavior under normal conditions
- Edge cases and how they're handled
- Side effects (if any)
- Thread safety (if relevant)

### Constraints and Requirements

- Non-performance related constraints
- Prerequisites for use
- Limitations or known issues
- Dependencies

### Error Documentation

Document error conditions and their meanings:

- List specific error types/values that may be returned
- Explain when each error occurs
- Help callers understand how to handle errors

```go
// Parse parses the input string into a Result.
// It returns an error if parsing fails.
//
// Errors:
//   - [ErrInvalidFormat]: input string is malformed
//   - [ErrEmpty]: input is an empty string
//   - [ErrTooLong]: input exceeds maximum length
func Parse(input string) (Result, error) { ... }
```

### Deprecation

Use the `Deprecated:` prefix to mark deprecated APIs:

- Start the comment with `// Deprecated:` (capital D, followed by colon)
- Explain what to use instead
- Optionally mention when it will be removed

```go
// Deprecated: Use [NewRouter] instead. This function will be removed in v2.0.
func OldRouter() *Router { ... }

// Deprecated: Use [Context.Value] with [RequestIDKey] instead.
func (c *Context) RequestID() string { ... }
```

### Interface vs Implementation Documentation

**Interfaces** should document the contract and expected behavior:

```go
// Handler handles HTTP requests.
// Implementations must be safe for concurrent use.
// Handle should not modify the request after returning.
type Handler interface {
	Handle(ctx *Context) error
}
```

**Implementations** should reference the interface and document implementation-specific details:

```go
// JSONHandler implements [Handler] for JSON request/response handling.
// It automatically parses JSON request bodies and encodes JSON responses.
type JSONHandler struct { ... }
```

### Generic Types

Document type parameter constraints and requirements:

```go
// BindInto binds values from a ValueGetter into a struct of type T.
// T must be a struct type; using non-struct types results in an error.
// T should have exported fields with appropriate struct tags.
//
// Example:
//
//	result, err := BindInto[UserRequest](getter, "query")
func BindInto[T any](getter ValueGetter, tag string) (T, error) { ... }

// Cache stores values of type V indexed by keys of type K.
// K must be comparable for use as a map key.
// V can be any type, including pointer types.
type Cache[K comparable, V any] struct { ... }
```

### Thread Safety

Document concurrency behavior when relevant. Thread safety documentation IS appropriate when:

- The type is designed for concurrent access
- Methods have synchronization requirements
- There are ordering constraints between method calls

```go
// Router is safe for concurrent use by multiple goroutines.
// Routes should be registered before calling [Router.ServeHTTP].
type Router struct { ... }

// Counter provides a thread-safe counter.
// All methods may be called concurrently from multiple goroutines.
type Counter struct { ... }

// Builder is NOT safe for concurrent use.
// Create separate Builder instances for each goroutine.
type Builder struct { ... }
```

### Cross-References

Use bracket syntax `[Symbol]` to create links to other documented symbols (Go 1.19+):

```go
// Handle processes the request using the provided [Context].
// It returns a [Response] or an error.
// See [Router.Register] for how to register handlers.
func Handle(ctx *Context) (*Response, error) { ... }

// NewRouter creates a new [Router] with the given [Config].
// Use [WithMiddleware] to add middleware to the router.
func NewRouter(cfg *Config) *Router { ... }
```

**Cross-reference targets:**

- `[FunctionName]` - links to a function in the same package
- `[TypeName]` - links to a type in the same package
- `[TypeName.MethodName]` - links to a method
- `[pkg.Symbol]` - links to a symbol in another package (e.g., `[http.Handler]`)

---

## 📌 Style Rules

### GoDoc Standards

- **Start function comments with the function/type name** (GoDoc standard)
  - ✅ `// Register adds a new route...`
  - ❌ `// This function registers...` or `// Adds a new route...`

- Use **third-person, descriptive** language
  - ✅ "Handler creates...", "Router registers...", "Context stores..."
  - ❌ "This creates...", "We register...", "I store..."

### Clarity and Conciseness

- Use **full sentences**
- Keep comments **short but meaningful**
- Avoid unnecessary verbosity
- Be direct and clear

### Language Guidelines

- **No marketing language** or adjectives like:
  - "simple", "powerful", "robust", "amazing", "excellent"
  - "best", "perfect", "ideal", "superior"
- **No superlatives** or comparative language
- Focus on **factual descriptions**

### Code Examples

See [Usage Examples in Documentation](#usage-examples-in-documentation) for detailed guidance on writing examples.

Key points:

- **Public APIs should include code examples** in their documentation
- Use **tab indentation** (not spaces) for code blocks
- Prefer **runnable Example functions** in `*_test.go` files for public APIs
- Keep examples **minimal and focused** - show one concept at a time

### Package Documentation Files (doc.go)

When package documentation is substantial (more than a few lines), use a dedicated `doc.go` file:

- **File naming**: Must be named exactly `doc.go` (lowercase)
- **Location**: Place in the package root directory
- **Content**: Should contain only the package comment and package declaration
- **Purpose**: Keeps package overview separate from implementation code

**Format requirements:**

- Start with `// Package [name]` followed by a clear, concise description
- The first sentence should be a summary (shown in package listings)
- Use markdown-style headers (`#`) for major sections
- Include code examples using indented blocks when helpful
- Cover: package purpose, main concepts, usage patterns, architecture (when relevant)

**What to include in doc.go:**

- Package overview and purpose
- Key features and capabilities
- Architecture or design decisions (when relevant)
- Quick start examples
- Common usage patterns
- Integration examples
- Links to examples directory or related packages

**What NOT to include:**

- Performance characteristics or optimization details
- Algorithmic complexity or implementation details
- File organization history or code movement comments
- Individual function/type documentation (those belong in their respective `.go` files)

**Example structure:**

```go
// Package router provides an HTTP router for Go.
//
// The router implements a routing system for cloud-native applications.
// It features path matching, parameter extraction, and comprehensive middleware support.
//
// # Key Features
//
//   - Path matching for static and parameterized routes
//   - Parameter extraction from URL paths
//   - Context pooling for request handling
//
// # Quick Start
//
//	package main
//
//	import "rivaas.dev/router"
//
//	func main() {
//	    r := router.New()
//	    r.GET("/", handler)
//	    r.Run(":8080")
//	}
//
// # Examples
//
// See the examples directory for complete working examples.
package router
```

**When to use doc.go vs inline comments:**

- **Use doc.go**: When package documentation is substantial (multiple paragraphs, sections, examples)
- **Use inline comments**: When package documentation is brief (1-3 sentences), keep it in the main package file

---

## 📝 Examples

### ✅ Good Documentation

```go
// Register adds a new route to the [Router] using the given method and pattern.
// It returns the created [Route], which can be further configured.
// Register should be called during application setup before the server starts.
func (r *Router) Register(method, pattern string) *Route { ... }

// Context represents an HTTP request context.
// It provides access to the request, response writer, and route parameters.
// Context instances are pooled and reused across requests.
// Context is NOT safe for use after the handler returns.
type Context struct { ... }

// Param returns the value of the named route parameter.
// It returns an empty string if the parameter is not found.
// Parameters are extracted from the URL path during route matching.
//
// Example:
//
//	userID := c.Param("id")
//	fmt.Println(userID)
func (c *Context) Param(name string) string { ... }

// BindInto binds values from a [ValueGetter] into a struct of type T.
// T must be a struct type with exported fields.
// It creates a new instance of T, binds values to it, and returns it.
//
// Example:
//
//	result, err := BindInto[UserRequest](getter, "query")
//
// Parameters:
//   - getter: ValueGetter that provides values
//   - tag: Struct tag name to use for field matching
//
// Errors:
//   - [ErrNotStruct]: T is not a struct type
//   - [ErrBindingFailed]: value binding failed for one or more fields
func BindInto[T any](getter ValueGetter, tag string, opts ...Option) (T, error) { ... }

// Handler defines the interface for HTTP request handlers.
// Implementations must be safe for concurrent use.
type Handler interface {
	// Handle processes an HTTP request.
	// It returns an error if the request cannot be processed.
	Handle(ctx *Context) error
}

// JSONHandler implements [Handler] for JSON request/response handling.
// It automatically parses JSON request bodies and encodes JSON responses.
type JSONHandler struct { ... }

// Deprecated: Use [NewRouter] instead. This function will be removed in v2.0.
func CreateRouter() *Router { ... }

// Router is safe for concurrent use by multiple goroutines.
// Routes must be registered before calling [Router.ServeHTTP].
type Router struct { ... }
```

### ❌ Bad Documentation

```go
// Register is a highly optimized router method with zero allocations.
// Uses O(1) lookup for fast routing.
// Extremely efficient performance characteristics.
func (r *Router) Register(method, pattern string) *Route { ... }

// Context is a fast, memory-efficient request context.
// Uses minimal allocations and provides high-performance access.
// Benchmarks show 50% faster than alternatives.
type Context struct { ... }

// Param returns the value with O(1) lookup time.
// Optimized for speed with zero allocations.
func (c *Context) Param(name string) string { ... }

// ========================================
// HTTP Context Methods
// ========================================
func (c *Context) Param(name string) string { ... }

// TODO: move this to a separate file
// Param returns the value of the named route parameter.
func (c *Context) Param(name string) string { ... }

// FIXME: this should be in context_helpers.go
// Returns parameter value
func (c *Context) Param(name string) string { ... }

//
// Empty comment lines are unnecessary
//
func (c *Context) Param(name string) string { ... }
```

---

## 🔍 Review Checklist

When writing or reviewing documentation, ensure:

### Content Rules

- [ ] No performance-related terms (fast, efficient, optimized, etc.)
- [ ] No algorithmic complexity notation (Big-O, O(1), etc.)
- [ ] No benchmark or optimization claims
- [ ] No memory usage details
- [ ] No marketing language or superlatives
- [ ] No decorative comment separators or ASCII art
- [ ] No TODO/FIXME comments about moving code to other locations
- [ ] No code organization or file history comments (merged from, moved from, etc.)
- [ ] Comments provide actual informational value

### Style Rules

- [ ] Comments start with function/type name
- [ ] Third-person, descriptive language
- [ ] Clear explanation of what the code does

### Documentation Completeness

- [ ] Parameters and return values documented
- [ ] Error return conditions documented with specific error types
- [ ] Edge cases and constraints mentioned
- [ ] Thread safety documented when relevant (concurrent access, ordering constraints)
- [ ] Generic type constraints documented when non-obvious

### Examples and References

- [ ] Public APIs include code examples (inline or Example functions)
- [ ] Code examples use tab indentation (not spaces)
- [ ] Cross-references use `[Symbol]` syntax (Go 1.19+)

### Special Cases

- [ ] Deprecated functions use `// Deprecated:` prefix with replacement guidance
- [ ] Interfaces document the contract, implementations reference the interface
- [ ] Package documentation uses `doc.go` when substantial (multiple paragraphs/sections)
- [ ] `doc.go` files start with `// Package [name]` and contain only package documentation

---

## 📚 Additional Resources

- [Go Doc Comments](https://go.dev/doc/comment) - Official guide for writing Go doc comments (includes Go 1.19+ features)
- [Effective Go - Commentary](https://go.dev/doc/effective-go#commentary) - General commentary guidelines
- [GoDoc Documentation](https://pkg.go.dev/golang.org/x/tools/cmd/godoc) - GoDoc tool reference
- [Example Functions](https://go.dev/blog/examples) - Writing testable example functions

---

## 🎯 Summary

**Remember:** Documentation should explain **what** the code does and **how** to use it, not **how well** it performs. Focus on functionality, behavior, and usage patterns. If performance characteristics are implied by the code, omit them from the documentation entirely.
