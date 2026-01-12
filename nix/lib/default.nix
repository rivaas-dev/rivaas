# Library functions and shared utilities for the Rivaas flake
{ pkgs }:

let
  colors = import ./colors.nix;
  findPatterns = import ./find-patterns.nix;
  mkModuleScript = import ./module-script.nix { inherit pkgs colors; };
  appsMeta = import ./apps-meta.nix;
  banner = import ./banner.nix { inherit colors appsMeta; };
in
{
  inherit colors findPatterns mkModuleScript appsMeta banner;

  # Ready-to-use banner shell hook for devshell
  bannerShellHook = banner.shellHook;
}
