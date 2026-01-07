# Development shell configuration with banner
{ pkgs, lib }:

let
  # Group apps by category based on their position in apps-meta
  categories = [
    { name = "Testing"; apps = builtins.filter (a: builtins.elem a.name ["test" "test-race" "test-integration" "test-examples"]) lib.appsMeta; }
    { name = "Code Quality"; apps = builtins.filter (a: builtins.elem a.name ["fmt" "fmt-check" "lint" "lint-soft" "lint-all" "bench" "tidy"]) lib.appsMeta; }
    { name = "Release"; apps = builtins.filter (a: builtins.elem a.name ["release-check" "release" "run-example"]) lib.appsMeta; }
    { name = "Commit Tools"; apps = builtins.filter (a: builtins.elem a.name ["commit" "commit-check"]) lib.appsMeta; }
  ];

  # Calculate max name length for padding (across all apps)
  maxNameLen = builtins.foldl' (acc: app: 
    let len = builtins.stringLength app.name; 
    in if len > acc then len else acc
  ) 0 lib.appsMeta;

  # Pad a string to a given length
  padRight = str: len:
    let
      strLen = builtins.stringLength str;
      padLen = if strLen >= len then 0 else len - strLen;
      padding = builtins.concatStringsSep "" (builtins.genList (_: " ") padLen);
    in str + padding;

  # Generate a single command line
  mkCommandLine = app:
    let
      paddedName = padRight app.name maxNameLen;
      colorCode = lib.colors.${app.color};
    in
    ''printf "    %s  %s\n" "$(gum style --foreground ${colorCode} 'nix run .#${paddedName}')" "$(gum style --faint '${app.description}')"'';

  # Generate category section
  mkCategory = category:
    let
      commands = builtins.map mkCommandLine category.apps;
      commandLines = builtins.concatStringsSep "\n    " commands;
    in
    ''
    echo ""
    gum style --foreground ${lib.colors.header} --bold "${category.name}:"
    ${commandLines}
    '';

  # Generate all categories
  allCategories = builtins.concatStringsSep "\n    " (builtins.map mkCategory categories);
in

pkgs.mkShell {
  name = "rivaas-dev";

  buildInputs = with pkgs; [
    # Go toolchain
    go           # Go compiler
    gopls        # Go language server
    gotools      # Go tools (e.g., godoc, goimports, callgraph, digraph, etc.)
    go-tools   # Go tools (e.g., goimports, govet, etc.)
    golangci-lint # Linting and code quality
    delve        # Go debugger
    ginkgo       # Testing and benchmarking tools

    # Build tools
    gnumake
    graphviz     # Graph visualization tool

    # Version control
    git

    # Optional: useful development tools
    # Shell utilities
    jq           # JSON processing
    yq           # YAML processing
    curl        # HTTP client
    gum         # Glamorous shell scripts (charmbracelet/gum)

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

    # Skip banner in CI environments (CI variable is set by most CI systems)
    if [ -n "''${CI:-}" ]; then
      return
    fi

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

    gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "Quick Commands"
    ${allCategories}
    echo ""
  '';

  # Environment variables
  # CGO is enabled for race detector support (-race flag).
  # Override with CGO_ENABLED=0 for static binaries if needed.
  CGO_ENABLED = "1";
}
