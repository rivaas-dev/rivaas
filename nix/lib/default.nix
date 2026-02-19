# Library functions and shared utilities for the Rivaas flake
{ pkgs }:

let
  colors = import ./colors.nix;
  findPatterns = import ./find-patterns.nix;
  moduleHelpers = import ./module-helpers.nix;
  mkModuleScript = import ./module-script.nix { inherit pkgs colors; };
  mkCoverageTestScript = import ./coverage-test-script.nix { inherit pkgs colors findPatterns; };
  appsMeta = import ./apps-meta.nix;
  banner = import ./banner.nix { inherit colors appsMeta; };
in
{
  inherit colors findPatterns moduleHelpers mkModuleScript mkCoverageTestScript appsMeta banner;

  # Ready-to-use banner shell hook for devshell
  bannerShellHook = banner.shellHook;
}
