# Router Comparison Benchmarks

This directory contains comparative performance benchmarks between `rivaas/router` and other popular Go web frameworks.

## Why Separate Module?

The comparison benchmarks require dependencies on external frameworks (Gin, Echo, Chi, Fiber, Hertz, Beego) which we don't want to include in the main `rivaas/router` module. By isolating these benchmarks in a separate module, we keep the main module's dependencies clean while still maintaining the ability to run performance comparisons.

## Frameworks Compared

- **rivaas/router** - This router
- **net/http** - Go standard library ServeMux with Go 1.22+ dynamic routing (`{param}`)
- **Gin** - High-performance web framework
- **Echo** - Minimalist web framework
- **Chi** - Lightweight router
- **Fiber** - Express-inspired framework v2 (measured via net/http adaptor for httptest compatibility)
- **Fiber v3** - Same framework, v3 (measured via net/http adaptor)
- **Hertz** - CloudWeGo HTTP framework (measured via `ut.PerformRequest`, native test API; no `http.Handler`)
- **Beego** - Full-stack framework (measured via `http.Handler`)

## Methodology

- **Same route structure**: All frameworks register the same three routes: static `/`, one param `/users/:id`, and two params `/users/:id/posts/:post_id`.
- **Same response pattern**: Handlers use string concatenation only (no `fmt.Sprintf`) so allocation and CPU cost are comparable.
- **Same request path**: Each scenario (BenchmarkStatic, BenchmarkOneParam, BenchmarkTwoParams) hits the same URL across all frameworks.
- **Fiber / Fiber v3**: Measured through the net/http adaptor (`fiberadaptor.FiberApp`), which adds overhead but allows the same httptest-based loop as other frameworks.
- **Hertz**: Measured via `ut.PerformRequest(h.Engine, ...)` (Hertz’s native test API) because Hertz does not implement `http.Handler`; numbers are not directly comparable to httptest-based frameworks.
- **Beego**: May log “init global config instance failed” if `conf/app.conf` is missing; safe to ignore in benchmarks.

## Running Benchmarks

```bash
# Navigate to benchmarks directory
cd router/benchmarks

# Run all comparison benchmarks
go test -bench=.

# Run a specific scenario (e.g. one dynamic param)
go test -bench=BenchmarkOneParam

# Run with memory stats
go test -bench=. -benchmem

# Run with CPU profiling
go test -bench=. -cpuprofile=cpu.prof

# Multiple runs for benchstat comparison
go test -bench=. -count=5
```

## Understanding Results

Benchmark output shows:

- **ns/op**: Nanoseconds per operation (lower is better)
- **B/op**: Bytes allocated per operation (lower is better)
- **allocs/op**: Number of allocations per operation (lower is better)

Example output (sub-benchmark format):

```text
BenchmarkStatic/Rivaas-12       8742841   127.6 ns/op    8 B/op   1 allocs/op
BenchmarkStatic/StdMux-12     11353951   115.9 ns/op    5 B/op   1 allocs/op
BenchmarkStatic/Gin-12         8586684   152.3 ns/op   48 B/op   1 allocs/op
BenchmarkStatic/Echo-12        9520530   125.8 ns/op    8 B/op   1 allocs/op
BenchmarkStatic/Chi-12         3006007   403.1 ns/op  373 B/op   3 allocs/op
BenchmarkStatic/Fiber-12        393267  2873 ns/op  1988 B/op  20 allocs/op
BenchmarkStatic/FiberV3-12      ...
BenchmarkStatic/Hertz-12        ...
BenchmarkStatic/Beego-12        ...
BenchmarkOneParam/Rivaas-12    ...
BenchmarkOneParam/StdMux-12    ...
...
```

## Adding New Comparisons

To add a new framework:

1. Add the framework dependency to `go.mod`.
2. Add a `setupXxx()` function in `comparison_test.go` that returns an `http.Handler` with the same three routes and response pattern (direct writes via `io.WriteString`, no string concatenation or format strings).
3. Call that setup from each of `BenchmarkStatic`, `BenchmarkOneParam`, and `BenchmarkTwoParams` via a new `b.Run("Name", ...)` sub-benchmark.
4. Run `go mod tidy`.

## Notes

- Results may vary based on hardware, OS, and Go version.
- Use `go test -bench=. -count=5` and `benchstat` to compare runs or detect regressions.
