{
  description = "Rivaas - Go web framework development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        
        # Use the latest stable Go version available in nixpkgs
        go = pkgs.go;
        
      in
      {
        devShells.default = pkgs.mkShell {
          name = "rivaas-dev";
          
          buildInputs = with pkgs; [
            # Go toolchain
            go
            gotools
            go-tools
            
            # Linting and code quality
            golangci-lint
            
            # Testing and benchmarking tools
            delve        # Go debugger
            
            # Build tools
            gnumake
            
            # Version control
            git
            
            # Optional: useful development tools
            gopls        # Go language server
            
            # Shell utilities
            jq           # JSON processing (useful for API testing)
            curl         # HTTP client
          ];

          shellHook = ''
            echo "🚀 Rivaas Development Environment"
            echo "================================"
            echo "Go version: $(go version)"
            echo "golangci-lint version: $(golangci-lint version --format short 2>/dev/null || echo 'not found')"
            echo ""
            echo "Quick commands (flake apps):"
            echo "  nix run .#test          - Run unit tests"
            echo "  nix run .#test-race     - Run tests with race detector"
            echo "  nix run .#test-integration - Run integration tests"
            echo "  nix run .#lint          - Run golangci-lint"
            echo "  nix run .#bench         - Run benchmarks"
            echo "  nix run .#tidy          - Run go mod tidy for all modules"
            echo ""
            echo "Direct Go commands:"
            echo "  go test ./...              - Run all tests"
            echo "  go test ./... -race        - Run tests with race detector"
            echo "  go test ./... -tags=integration - Run integration tests"
            echo "  golangci-lint run          - Run linter"
            echo "  go test ./... -bench=.     - Run benchmarks"
            echo ""
            
            # Set Go environment variables
            export GOROOT="${go}/share/go"
            export GOPATH="$HOME/go"
            export PATH="$GOPATH/bin:$PATH"
            
            # Enable Go modules
            export GO111MODULE=on
            
            # Optimize for development
            export GOFLAGS="-buildvcs=true"
            
            # Set golangci-lint cache directory
            export GOLANGCI_LINT_CACHE="$PWD/.golangci-cache"
            mkdir -p "$GOLANGCI_LINT_CACHE"
            
            echo "Environment configured ✓"
            echo ""
          '';

          # Environment variables
          CGO_ENABLED = "1"; # Enable CGO for race detector
        };

        # Scripts for common tasks that work from project directory
        apps = {
          # Run all tests
          test = {
            type = "app";
            program = toString (pkgs.writeShellScript "rivaas-test" ''
              # Test all modules in the workspace
              for dir in app binding errors logging metrics openapi router router/benchmarks telemetry tracing validation; do
                if [ -d "$dir" ]; then
                  echo "Testing $dir..."
                  (cd "$dir" && ${go}/bin/go test ./... -v -count=1) || exit 1
                fi
              done
            '');
          };

          # Run tests with race detector
          test-race = {
            type = "app";
            program = toString (pkgs.writeShellScript "rivaas-test-race" ''
              # Test all modules in the workspace with race detector
              for dir in app binding errors logging metrics openapi router router/benchmarks telemetry tracing validation; do
                if [ -d "$dir" ]; then
                  echo "Testing $dir with race detector..."
                  (cd "$dir" && ${go}/bin/go test ./... -race -v -count=1) || exit 1
                fi
              done
            '');
          };

          # Run integration tests
          test-integration = {
            type = "app";
            program = toString (pkgs.writeShellScript "rivaas-test-integration" ''
              # Test all modules in the workspace with integration tag
              for dir in app binding errors logging metrics openapi router router/benchmarks telemetry tracing validation; do
                if [ -d "$dir" ]; then
                  echo "Testing $dir (integration)..."
                  (cd "$dir" && ${go}/bin/go test ./... -tags=integration -v -count=1) || exit 1
                fi
              done
            '');
          };

          # Run golangci-lint
          lint = {
            type = "app";
            program = toString (pkgs.writeShellScript "rivaas-lint" ''
              ${pkgs.golangci-lint}/bin/golangci-lint run --config .golangci.yaml
            '');
          };

          # Run benchmarks
          bench = {
            type = "app";
            program = toString (pkgs.writeShellScript "rivaas-bench" ''
              # Run benchmarks for all modules in the workspace
              for dir in app binding errors logging metrics openapi router router/benchmarks telemetry tracing validation; do
                if [ -d "$dir" ]; then
                  echo "Benchmarking $dir..."
                  (cd "$dir" && ${go}/bin/go test ./... -bench=. -benchmem -run=^$ -count=1)
                fi
              done
            '');
          };

          # Run go mod tidy for all modules
          tidy = {
            type = "app";
            program = toString (pkgs.writeShellScript "rivaas-tidy" ''
              # Run go mod tidy for all modules in the workspace
              for dir in app binding errors logging metrics openapi router router/benchmarks telemetry tracing validation; do
                if [ -d "$dir" ] && [ -f "$dir/go.mod" ]; then
                  echo "Running go mod tidy in $dir..."
                  (cd "$dir" && ${go}/bin/go mod tidy) || exit 1
                fi
              done
              echo "✓ All modules tidied"
            '');
          };
        };
      }
    );
}
