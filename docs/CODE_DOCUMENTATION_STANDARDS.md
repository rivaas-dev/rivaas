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

- Format examples using indented code blocks with `// Example:` header
- Show **typical usage patterns** and **common integration scenarios**
- Keep examples **concise** - focus on demonstrating the API, not full applications
- Examples should be **runnable** - use valid Go code that compiles
- Place examples after the main description and before parameter/return documentation

Example format:

```go
// FunctionName does something useful.
// It processes the input and returns a result.
//
// Example:
//
//     result := FunctionName("input")
//     fmt.Println(result)
//
// Parameters:
//   - input: description
func FunctionName(input string) string { ... }
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

- **Public APIs should include code examples** - All exported functions, types, and methods benefit from inline code examples in their documentation
- Use code examples to demonstrate **common use cases** and **typical usage patterns**
- Format examples using indented code blocks:

  ```go
  // Example:
  //
  //     code here
  ```

- Keep examples **minimal and focused** - show one concept at a time
- Ensure examples are **correct and runnable** - they should compile and work as shown
- Examples should complement the text description, not replace it

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
// Register adds a new route to the Router using the given method and pattern.
// It returns the created Route, which can be further configured.
// Register should be called during application setup before the server starts.
func (r *Router) Register(method, pattern string) *Route { ... }

// Context represents an HTTP request context.
// It provides access to the request, response writer, and route parameters.
// Context instances are pooled and reused across requests.
type Context struct { ... }

// Param returns the value of the named route parameter.
// It returns an empty string if the parameter is not found.
// Parameters are extracted from the URL path during route matching.
//
// Example:
//
//     userID := c.Param("id")
//     fmt.Println(userID)
func (c *Context) Param(name string) string { ... }

// BindInto binds values from a ValueGetter into a struct of type T.
// It creates a new instance of T, binds values to it, and returns it.
// This is a convenience function that eliminates the need to declare a variable,
// create and pass a pointer manually.
//
// Example:
//
//     result, err := BindInto[UserRequest](getter, "query")
//
// Parameters:
//   - getter: ValueGetter that provides values
//   - tag: Struct tag name to use for field matching
//
// Returns the bound value of type T and an error if binding fails.
func BindInto[T any](getter ValueGetter, tag string, opts ...Option) (T, error) { ... }
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

- [ ] No performance-related terms (fast, efficient, optimized, etc.)
- [ ] No algorithmic complexity notation (Big-O, O(1), etc.)
- [ ] No benchmark or optimization claims
- [ ] No memory usage details
- [ ] Comments start with function/type name
- [ ] Third-person, descriptive language
- [ ] Clear explanation of what the code does
- [ ] Public functionalities include code examples in documentation when helpful
- [ ] Usage examples when helpful
- [ ] Parameters and return values documented
- [ ] Edge cases and constraints mentioned
- [ ] No marketing language or superlatives
- [ ] No decorative comment separators or ASCII art
- [ ] No TODO/FIXME comments about moving code to other locations
- [ ] No code organization or file history comments (merged from, moved from, etc.)
- [ ] Comments provide actual informational value
- [ ] Package documentation uses `doc.go` when substantial (multiple paragraphs/sections)
- [ ] `doc.go` files start with `// Package [name]` and contain only package documentation

---

## 📚 Additional Resources

- [Go Documentation Comments](https://go.dev/doc/effective-go#commentary)
- [GoDoc Documentation](https://pkg.go.dev/golang.org/x/tools/cmd/godoc)
- [Effective Go - Commentary](https://go.dev/doc/effective-go#commentary)

---

## 🎯 Summary

**Remember:** Documentation should explain **what** the code does and **how** to use it, not **how well** it performs. Focus on functionality, behavior, and usage patterns. If performance characteristics are implied by the code, omit them from the documentation entirely.
