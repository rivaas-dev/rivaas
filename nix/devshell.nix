# Development shell configuration with banner
{ pkgs, lib }:

pkgs.mkShell {
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
    gum          # Glamorous shell scripts (charmbracelet/gum)
  ];

  shellHook = ''
    # Set Go environment variables first (before gum output)
    export GOROOT="${lib.go}/share/go"
    export GOPATH="$HOME/go"
    export PATH="$GOPATH/bin:$PATH"
    export GO111MODULE=on
    export GOFLAGS="-buildvcs=true"
    export GOLANGCI_LINT_CACHE="$PWD/.golangci-cache"
    mkdir -p "$GOLANGCI_LINT_CACHE"

    # Pretty welcome banner with gum (pastel colors)
    echo ""
    gum style --foreground ${lib.colors.header} "
██████╗ ██╗██╗   ██╗ █████╗  █████╗ ███████╗
██╔══██╗██║██║   ██║██╔══██╗██╔══██╗██╔════╝
██████╔╝██║██║   ██║███████║███████║███████╗
██╔══██╗██║╚██╗ ██╔╝██╔══██║██╔══██║╚════██║
██║  ██║██║ ╚████╔╝ ██║  ██║██║  ██║███████║
╚═╝  ╚═╝╚═╝  ╚═══╝  ╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝
    "

    echo ""
    gum style --foreground ${lib.colors.info} "Go version: $(go version | cut -d' ' -f3 | tr -d 'go')"
    gum style --foreground ${lib.colors.info} "Golangci-lint version: $(golangci-lint version --short 2>/dev/null || echo 'not found')"
    echo ""

    gum style --foreground ${lib.colors.header} --bold "Quick commands (flake apps):"
    printf "  %s  %s\n" "$(gum style --foreground ${lib.colors.success} 'nix run .#test            ')" "$(gum style --faint 'Run unit tests')"
    printf "  %s  %s\n" "$(gum style --foreground ${lib.colors.accent1} 'nix run .#test-race       ')" "$(gum style --faint 'Run tests with race detector')"
    printf "  %s  %s\n" "$(gum style --foreground ${lib.colors.accent2} 'nix run .#test-integration')" "$(gum style --faint 'Run integration tests')"
    printf "  %s  %s\n" "$(gum style --foreground ${lib.colors.accent3} 'nix run .#test-examples   ')" "$(gum style --faint 'Build examples (standalone)')"
    printf "  %s  %s\n" "$(gum style --foreground ${lib.colors.accent4} 'nix run .#lint            ')" "$(gum style --faint 'Run golangci-lint')"
    printf "  %s  %s\n" "$(gum style --foreground ${lib.colors.accent5} 'nix run .#bench           ')" "$(gum style --faint 'Run benchmarks')"
    printf "  %s  %s\n" "$(gum style --foreground ${lib.colors.accent6} 'nix run .#tidy            ')" "$(gum style --faint 'Run go mod tidy for all modules')"
    printf "  %s  %s\n" "$(gum style --foreground ${lib.colors.accent2} 'nix run .#release-check   ')" "$(gum style --faint 'Check modules for unreleased changes')"
    printf "  %s  %s\n" "$(gum style --foreground ${lib.colors.accent3} 'nix run .#release         ')" "$(gum style --faint 'Interactive release (create tags)')"
    printf "  %s  %s\n" "$(gum style --foreground ${lib.colors.accent4} 'nix run .#run-example     ')" "$(gum style --faint 'Interactive example runner')"
    echo ""

    gum style --foreground ${lib.colors.success} "Environment configured ✓"
    echo ""
  '';

  # Environment variables
  # CGO is enabled for race detector support (-race flag).
  # Override with CGO_ENABLED=0 for static binaries if needed.
  CGO_ENABLED = "1";
}
