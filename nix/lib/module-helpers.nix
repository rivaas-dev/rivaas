# Shared helper functions for module operations (commit, release)
{
  # Shorten module path for commit message prefix (no special cases; modules are at root)
  shortenPrefixScript = ''
    shorten_prefix() {
      echo "$1"
    }
  '';

  # Filter commits by conventional commit scope
  # Keeps commits whose scope matches the module or have no scope
  # Excludes commits scoped to parent modules
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
}
