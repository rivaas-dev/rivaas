# Router Comparison Benchmarks

This directory contains comparative performance benchmarks between `rivaas/router` and other popular Go web frameworks.

## Why Separate Module?

The comparison benchmarks require dependencies on external frameworks (Gin, Echo, Chi, Fiber, fasthttp) which we don't want to include in the main `rivaas/router` module. By isolating these benchmarks in a separate module, we keep the main module's dependencies clean while still maintaining the ability to run performance comparisons.

## Frameworks Compared

- **rivaas/router** - This router
- **net/http** - Go standard library
- **Gin** - High-performance web framework
- **Echo** - Minimalist web framework
- **Chi** - Lightweight router
- **Fiber** - Express-inspired framework built on fasthttp
- **fasthttp** - Fast HTTP implementation

## Running Benchmarks

```bash
# Navigate to benchmarks directory
cd benchmarks

# Run all comparison benchmarks
go test -bench=.

# Run specific benchmark
go test -bench=BenchmarkRivaasRouter

# Run with memory profiling
go test -bench=. -benchmem

# Run with CPU profiling
go test -bench=. -cpuprofile=cpu.prof

# Compare multiple runs
go test -bench=. -count=5
```

## Understanding Results

Benchmark output shows:

- **ns/op**: Nanoseconds per operation (lower is better)
- **B/op**: Bytes allocated per operation (lower is better)
- **allocs/op**: Number of allocations per operation (lower is better)

Example output (December 2025, Intel i7-1265U):

```text
BenchmarkRivaasRouter-12                10000000    119 ns/op    16 B/op    1 allocs/op
BenchmarkRivaasRouterPlainString-12     13000000     88 ns/op    16 B/op    1 allocs/op
BenchmarkRivaasRouterZeroAlloc-12       30000000     40 ns/op     0 B/op    0 allocs/op
BenchmarkStandardMux-12                 11000000    107 ns/op    16 B/op    1 allocs/op
BenchmarkSimpleRouter-12                40000000     26 ns/op    16 B/op    1 allocs/op
BenchmarkGinRouter-12                    7500000    162 ns/op    80 B/op    3 allocs/op
BenchmarkEchoRouter-12                  10000000    119 ns/op    32 B/op    2 allocs/op
BenchmarkChiRouter-12                    2900000    417 ns/op   720 B/op    5 allocs/op
BenchmarkFiberRouter-12                   800000   1446 ns/op  2064 B/op   20 allocs/op
```

## Adding New Comparisons

To add a new framework comparison:

1. Add the framework dependency to `go.mod`
2. Create a new benchmark function in `comparison_test.go`
3. Follow the existing benchmark pattern for consistency
4. Run `go mod tidy` to update dependencies

## Notes

- All benchmarks use the same route structure for fair comparison
- The `BenchmarkRivaasRouter` includes the `Warmup()` call to ensure all optimizations are enabled
- fasthttp benchmarks include both native and adaptor versions due to different APIs
- Results may vary based on hardware, OS, and Go version
