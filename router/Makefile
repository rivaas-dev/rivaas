.PHONY: help bench bench-compare bench-baseline bench-save test test-race lint clean

# Default target
help:
	@echo "Available targets:"
	@echo "  make bench           - Run benchmarks once"
	@echo "  make bench-compare   - Run benchmarks and compare with baseline"
	@echo "  make bench-baseline  - Run benchmarks and save as new baseline"
	@echo "  make bench-save      - Save current benchmark results"
	@echo "  make test            - Run all tests"
	@echo "  make test-race       - Run tests with race detector"
	@echo "  make lint            - Run linters"
	@echo "  make clean           - Clean build artifacts"

# Run benchmarks once (quick check)
bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem -run='^$$'

# Run benchmarks and compare with baseline
bench-compare:
	@echo "Running benchmarks and comparing with baseline..."
	@command -v benchstat >/dev/null 2>&1 || { \
		echo "benchstat not found. Installing..."; \
		go install golang.org/x/perf/cmd/benchstat@latest; \
	}
	@mkdir -p .benchmarks
	@if [ ! -f .benchmarks/baseline-latest.txt ]; then \
		echo "No baseline found. Run 'make bench-baseline' first."; \
		exit 1; \
	fi
	@echo "Running current benchmarks..."
	go test -bench=. -benchmem -benchtime=100000x -count=5 -run='^$$' > .benchmarks/current.txt 2>&1
	@echo ""
	@echo "=== Comparison with Baseline ==="
	@benchstat .benchmarks/baseline-latest.txt .benchmarks/current.txt
	@echo ""
	@echo "Checking for regressions (>10% time, >20% memory)..."
	@if benchstat -threshold=0.1 .benchmarks/baseline-latest.txt .benchmarks/current.txt 2>&1 | grep -E '\+[0-9]+\.[0-9]+%.*~'; then \
		echo "⚠️  Performance regression detected!"; \
		exit 1; \
	else \
		echo "✅ No significant performance regression."; \
	fi

# Run benchmarks with high accuracy and save as baseline
bench-baseline:
	@echo "Running comprehensive benchmarks for baseline..."
	@mkdir -p .benchmarks
	go test -bench=. -benchmem -benchtime=200000x -count=10 -run='^$$' | tee .benchmarks/baseline-$$(date +%Y%m%d-%H%M%S).txt
	@ln -sf baseline-$$(date +%Y%m%d-%H%M%S).txt .benchmarks/baseline-latest.txt
	@echo ""
	@echo "✅ Baseline saved to .benchmarks/baseline-latest.txt"
	@echo "Keeping last 10 baselines..."
	@ls -t .benchmarks/baseline-*.txt | tail -n +11 | xargs -r rm
	@echo "Done!"

# Save current benchmark without making it baseline
bench-save:
	@echo "Running and saving benchmarks..."
	@mkdir -p .benchmarks
	go test -bench=. -benchmem -benchtime=100000x -count=5 -run='^$$' | tee .benchmarks/run-$$(date +%Y%m%d-%H%M%S).txt
	@echo ""
	@echo "✅ Saved to .benchmarks/run-$$(date +%Y%m%d-%H%M%S).txt"

# Run comparison benchmarks (vs Gin, Echo, fasthttp)
bench-frameworks:
	@echo "Running framework comparison benchmarks..."
	go test -bench='Benchmark(Rivaas|Gin|Echo|Fasthttp|Standard|Simple)Router$$' \
		-benchmem -benchtime=200000x -count=5 -run='^$$'

# Run memory benchmarks
bench-memory:
	@echo "Running memory benchmarks..."
	go test -bench='BenchmarkMemory' -benchmem -benchtime=100000x -run='^$$'

# Run concurrency benchmarks
bench-concurrent:
	@echo "Running concurrency benchmarks..."
	go test -bench='BenchmarkConcurrent' -benchmem -benchtime=50000x -run='^$$'

# Profile CPU
profile-cpu:
	@echo "Generating CPU profile..."
	@mkdir -p .profiles
	go test -bench='BenchmarkRivaasRouter' -benchtime=1000000x -cpuprofile=.profiles/cpu.prof -run='^$$'
	@echo ""
	@echo "View profile with: go tool pprof -http=:8080 .profiles/cpu.prof"

# Profile memory
profile-mem:
	@echo "Generating memory profile..."
	@mkdir -p .profiles
	go test -bench='BenchmarkRivaasRouter' -benchtime=1000000x -memprofile=.profiles/mem.prof -run='^$$'
	@echo ""
	@echo "View profile with: go tool pprof -http=:8080 .profiles/mem.prof"

# Run all tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with short flag (skip integration tests)
test-short:
	@echo "Running short tests..."
	go test -short -v ./...

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	go test -race -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@mkdir -p .coverage
	go test -coverprofile=.coverage/coverage.out ./...
	go tool cover -html=.coverage/coverage.out -o .coverage/coverage.html
	@echo ""
	@echo "✅ Coverage report: .coverage/coverage.html"

# Run linters (requires golangci-lint)
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not found. Install from: https://golangci-lint.run/"; \
		exit 1; \
	}
	@echo "Running linters..."
	golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	go mod tidy

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf .benchmarks/current.txt
	@rm -rf .benchmarks/run-*.txt
	@rm -rf .profiles/
	@rm -rf .coverage/
	@rm -f *.prof
	@rm -f *.test
	@echo "✅ Clean complete"

# Clean everything including baselines (use with caution!)
clean-all: clean
	@echo "⚠️  Removing ALL benchmarks including baselines..."
	@rm -rf .benchmarks/
	@echo "✅ All clean"

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install golang.org/x/perf/cmd/benchstat@latest
	go install golang.org/x/tools/cmd/goimports@latest
	@echo ""
	@echo "Optional tools (install if needed):"
	@echo "  golangci-lint: https://golangci-lint.run/usage/install/"
	@echo "✅ Basic tools installed"

# CI simulation - runs the same checks as CI
ci-local:
	@echo "Running CI checks locally..."
	@echo ""
	@echo "1. Running tests..."
	$(MAKE) test
	@echo ""
	@echo "2. Running tests with race detector..."
	$(MAKE) test-race
	@echo ""
	@echo "3. Running benchmarks..."
	$(MAKE) bench-compare
	@echo ""
	@echo "✅ All CI checks passed!"

