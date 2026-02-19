# Shared helper functions for module operations (commit, release)
{
  # Shorten module path for commit message prefix
  # router/middleware/accesslog -> middleware/accesslog
  # router -> router
  shortenPrefixScript = ''
    shorten_prefix() {
      local mod="$1"
      if [[ "$mod" == router/middleware/* ]]; then
        echo "''${mod#router/}"
      else
        echo "$mod"
      fi
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

  # Filter router commits: exclude commits that only touched router/middleware/*/ subdirs
  # Reads commit lines from stdin, writes filtered lines to stdout
  # Uses $git (must be in scope when script runs)
  filterRouterCommitsScript = ''
    filter_router_commits() {
      local mod="$1"
      if [ "$mod" != "router" ]; then
        cat
        return
      fi
      while IFS= read -r commit_line; do
        [ -z "$commit_line" ] && continue
        commit_hash=$(echo "$commit_line" | cut -d' ' -f1)
        files_changed=$($git show --name-only --format="" "$commit_hash" -- "$mod/" 2>/dev/null)
        non_middleware_files=$(echo "$files_changed" | grep -v "^router/middleware/[^/]\+/" || true)
        if [ -n "$non_middleware_files" ]; then
          echo "$commit_line"
        fi
      done
    }
  '';
}
