# Development shell configuration with banner
{ pkgs, lib }:

let
  # Calculate max name length for padding
  maxNameLen = builtins.foldl' (acc: app: 
    let len = builtins.stringLength app.name; 
    in if len > acc then len else acc
  ) 0 lib.appsMeta;

  # Pad a string to a given length (dynamically generates exact padding)
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
    ''printf "  %s  %s\n" "$(gum style --foreground ${colorCode} 'nix run .#${paddedName}')" "$(gum style --faint '${app.description}')"'';

  # Generate all command lines
  commandLines = builtins.concatStringsSep "\n    " (builtins.map mkCommandLine lib.appsMeta);
in

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
    ${commandLines}
    echo ""

    gum style --foreground ${lib.colors.success} "Environment configured ✓"
    echo ""
  '';

  # Environment variables
  # CGO is enabled for race detector support (-race flag).
  # Override with CGO_ENABLED=0 for static binaries if needed.
  CGO_ENABLED = "1";
}
