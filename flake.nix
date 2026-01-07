{
  description = "Rivaas - Go web framework development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          config.allowUnfree = true;
        };

        # Import library functions
        lib = import ./nix/lib { inherit pkgs; };
      in
      {
        # Development shell
        devShells.default = import ./nix/devshell.nix { inherit pkgs lib; };

        # All apps (test, lint, format, release, etc.)
        # Run with: nix run .#<app-name>
        # List all: nix flake show
        apps = import ./nix/apps { inherit pkgs lib; };
      }
    );
}
