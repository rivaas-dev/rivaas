# Development shell configuration
{ pkgs, lib }:

pkgs.mkShell {
  name = "rivaas-dev";

  buildInputs = with pkgs; [
    # Go toolchain
    go            # Go compiler
    gopls         # Go language server
    gotools       # Go tools (godoc, goimports, callgraph, digraph, etc.)
    go-tools    # Go tools (staticcheck, etc.)
    golangci-lint  # Linting and code quality
    delve         # Go debugger
    ginkgo        # Testing and benchmarking tools

    # Build tools
    gnumake
    graphviz      # Graph visualization tool

    # Version control
    git

    # Shell utilities
    jq            # JSON processing
    yq            # YAML processing
    curl          # HTTP client
    gum           # Glamorous shell scripts (charmbracelet/gum)

    # AI-powered tool for generating commit messages
    cursor-cli
  ];

  shellHook = ''
    # Set Go environment variables first (before gum output)
    export GOPATH="$HOME/go"
    export PATH="$GOPATH/bin:$PATH"
    export GO111MODULE=on
    export GOFLAGS="-buildvcs=true"
    export GOPROXY="https://proxy.golang.org,direct"
    export GOLANGCI_LINT_CACHE="''${XDG_CACHE_HOME:-$HOME/.cache}/golangci-lint"

    # Display welcome banner (skipped in CI)
    ${lib.bannerShellHook}
  '';

  # Environment variables
  # CGO is enabled for race detector support (-race flag).
  # Override with CGO_ENABLED=0 for static binaries if needed.
  CGO_ENABLED = "1";
}
