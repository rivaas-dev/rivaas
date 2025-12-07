# Aggregates all flake apps
{ pkgs, lib }:

let
  testing = import ./testing.nix { inherit pkgs lib; };
  quality = import ./quality.nix { inherit pkgs lib; };
  release = import ./release.nix { inherit pkgs lib; };
  examples = import ./examples.nix { inherit pkgs lib; };
  commit = import ./commit.nix { inherit pkgs lib; };
in
  # Merge all app sets together
  testing // quality // release // examples // commit
