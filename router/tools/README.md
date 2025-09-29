# Router Development Tools

This directory contains development tools for the Rivaas Router package. These tools are **not part of the runtime** and are used during development, CI/CD, and performance optimization.

---

## Available Tools

### `verify_layout.go` - Memory Layout Verification Tool

**Purpose:** Verifies the memory layout of the `Context` struct to ensure optimal CPU cache performance.

**What it does:**

- Analyzes the memory layout of the `Context` struct
- Checks field alignment and offsets
- Verifies that "hot-path" fields (accessed on every request) fit within a single 64-byte CPU cache line
- Validates memory layout assumptions documented in `context.go`
- Outputs a detailed report showing field sizes, offsets, and cache line usage

**Why it matters:**
The router's `Context` struct is highly optimized for performance. Modern CPUs load memory in 64-byte cache lines, so grouping frequently-accessed fields together minimizes cache misses. This tool ensures the optimization remains correct as the codebase evolves.

**Usage:**

```bash
# Run the tool
cd router/tools
go run verify_layout.go
```

**Example Output:**

```text
=== Context Field Layout Analysis ===

Field Sizes:
  Request:      8 bytes (offset:   0)
  Response:     8 bytes (offset:   8)
  handlers:    24 bytes (offset:  16)
  router:       8 bytes (offset:  24)
  index:        4 bytes (offset:  32)
  paramCount:   4 bytes (offset:  36)
  paramKeys:  128 bytes (offset:  40)
  paramValues: 128 bytes (offset: 168)

"Hot Path" Fields Total: 48 bytes
First Cache Line Ends At: 40 bytes
✅ Hot fields fit in one cache line (64 bytes)

Total Context Size: 296 bytes
```

**When to run:**

- After modifying the `Context` struct in `context.go`
- When adding new fields to `Context`
- Before committing changes that affect `Context` memory layout
- During code review of performance-critical changes

**CI/CD Integration:**

Add to your CI pipeline to catch layout regressions:

```yaml
# .github/workflows/test.yml
- name: Verify Context Memory Layout
  run: |
    cd router/tools
    go run verify_layout.go | tee layout.txt
    grep "✅ Hot fields fit in one cache line" layout.txt
```

**Interpretation:**

- ✅ **Green check**: Hot-path fields fit in 64 bytes (optimal)
- ❌ **Red cross**: Hot-path fields exceed 64 bytes (performance regression)

If you see ❌, consider:

1. Reordering fields to pack hot-path fields tightly
2. Moving cold-path fields (rarely accessed) to the end
3. Using smaller data types where appropriate
4. Consulting the team before proceeding

---

## Development Guidelines

### Adding New Tools

When adding a new development tool to this directory:

1. **Create a standalone Go program** with `package main`
2. **Add comprehensive documentation** in this README
3. **Include usage examples** and expected output
4. **Consider CI/CD integration** if the tool catches regressions
5. **Use build tags** if the tool has heavy dependencies:

   ```go
   //go:build tools
   // +build tools
   ```

### Tool Categories

Tools in this directory should fall into one of these categories:

- **Verification tools** - Verify invariants and assumptions (like `verify_layout.go`)
- **Code generation** - Generate boilerplate or optimized code
- **Benchmarking helpers** - Tools to analyze or visualize benchmark results
- **Profiling utilities** - Tools to analyze CPU/memory profiles

### What NOT to Include

- Runtime dependencies (use `internal/` instead)
- Test utilities (use `internal/testutil/` or `_test.go` files)
- Production binaries (use `cmd/` instead)
- General-purpose scripts (use repository root `scripts/` directory)

---

## Related Documentation

- **Context Memory Layout**: See `router/context.go` for detailed memory layout documentation
- **Performance Optimization**: See `router/README.md` for optimization details
- **Development Workflow**: See repository root `CONTRIBUTING.md` for development practices

---

## Questions?

If you have questions about these tools or need to add a new development tool, please:

1. Check the existing tools for similar functionality
2. Review the development guidelines above
3. Open a discussion in the project's issue tracker
4. Consult with the team before adding heavy dependencies
