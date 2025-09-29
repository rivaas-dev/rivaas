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

Example output:

```text
BenchmarkRivaasRouter-14        5000000    250 ns/op    64 B/op    2 allocs/op
BenchmarkGinRouter-14           3000000    450 ns/op   128 B/op    4 allocs/op
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
