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

        # All apps (test, lint, release, etc.)
        apps = import ./nix/apps { inherit pkgs lib; };

        # CI checks - run with: nix flake check
        checks.default = import ./nix/checks.nix { inherit pkgs lib self; };
      }
    );
}
