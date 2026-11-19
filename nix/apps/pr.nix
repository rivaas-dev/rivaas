# Pull request app: prepare module worktrees and open GitHub pull requests.
# Commit messages stay in nix/apps/commit.nix. Releases stay in nix/apps/release.nix.
{ pkgs, lib }:

let
  contrib = ../../scripts/contrib;
in
{
  pr = {
    type = "app";
    meta.description = "Prepare, create, and check pull requests";
    program = toString (pkgs.writeShellScript "rivaas-pr" ''
      set -uo pipefail
      export GIT=${pkgs.git}/bin/git
      export GH=${pkgs.gh}/bin/gh
      export GUM=${pkgs.gum}/bin/gum
      export JQ=${pkgs.jq}/bin/jq
      export FIND=${pkgs.findutils}/bin/find
      if [ -x ${pkgs.cursor-cli}/bin/cursor-agent ]; then
        export CURSOR_AGENT=${pkgs.cursor-cli}/bin/cursor-agent
      fi
      exec ${pkgs.bash}/bin/bash ${contrib}/pr.sh "$@"
    '');
  };
}
