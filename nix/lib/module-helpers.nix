# Shared helper functions for module operations (commit, release)
{
  # Shorten module path for commit message prefix (no special cases; modules are at root)
  shortenPrefixScript = ''
    shorten_prefix() {
      echo "$1"
    }
  '';

  # Keep commits whose prefix (the text before the first colon) names this
  # module. A contrib: commit is not a module release. Commits with no
  # colon stay, so older unscoped messages still count. History is not rewritten.
  filterCommitsByScopeScript = ''
    filter_commits_by_scope() {
      local mod="$1"
      local short_name base_name
      short_name=$(shorten_prefix "$mod")
      base_name=$(basename "$mod")

      while IFS= read -r line; do
        [ -z "$line" ] && continue
        msg=$(echo "$line" | sed 's/^[a-f0-9]* //')

        # Extract scope (text before first colon)
        if [[ "$msg" == *:* ]]; then
          scope="''${msg%%:*}"
          # Keep if scope matches module, short name, or base name
          case "$scope" in
            "$mod"|"$short_name"|"$base_name")
              echo "$line"
              ;;
            *)
              # Discard (parent or unrelated scope)
              ;;
          esac
        else
          # No scope prefix -- keep (generic commit)
          echo "$line"
        fi
      done
    }
  '';

  # Pass-through: middleware is at repo root, no exclusion needed per module
  filterRouterCommitsScript = ''
    filter_router_commits() {
      cat
    }
  '';

  # Log commits that can require a release of this module.
  # Example, benchmark, and integration trees belong to the parent for commits
  # and pull requests. A commit that only touches those trees does not.
  moduleLogScript = ''
    module_log() {
      local range="$1"
      local mod="$2"
      local -a spec=(
        "$mod"
        ":(exclude,glob)$mod/examples/**"
        ":(exclude,glob)$mod/**/examples/**"
        ":(exclude,glob)$mod/example/**"
        ":(exclude,glob)$mod/**/example/**"
        ":(exclude,glob)$mod/benchmarks/**"
        ":(exclude,glob)$mod/**/benchmarks/**"
        ":(exclude,glob)$mod/integration/**"
        ":(exclude,glob)$mod/**/integration/**"
      )
      if [ -n "$range" ]; then
        $git log --oneline "$range" -- "''${spec[@]}"
      else
        $git log --oneline HEAD -- "''${spec[@]}"
      fi
    }
  '';
}
