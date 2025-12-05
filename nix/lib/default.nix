# Library functions and shared utilities for the Rivaas flake
{ pkgs }:

let
  colors = import ./colors.nix;
  findPatterns = import ./find-patterns.nix;
  mkModuleScript = import ./module-script.nix { inherit pkgs colors; };
in
{
  inherit colors findPatterns mkModuleScript;

  # Re-export commonly used packages for convenience
  go = pkgs.go;
  gum = pkgs.gum;
  git = pkgs.git;
  golangci-lint = pkgs.golangci-lint;
  findutils = pkgs.findutils;
}
