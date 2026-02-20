# Module commit app: stage and commit changes per module with AI-generated messages
{ pkgs, lib }:

let
  # =============================================================================
  # Configuration Constants
  # =============================================================================
  cfg = {
    aiTimeoutSec = 60;           # Timeout for AI message generation
    diffTruncateBytes = 20000;   # ~400-500 lines, keeps AI context manageable
    complexChangeThreshold = 5;  # Files changed before using multi-line commit format
    editorWidth = 80;            # Width for multi-line commit editor
    editorHeight = 10;           # Height for multi-line commit editor
    inputWidth = 72;             # Width for single-line commit input (git convention)
  };

  # =============================================================================
  # Reusable Shell Script Helpers
  # =============================================================================

  # Count changes for a module
  # Returns: staged unstaged untracked (space-separated)
  countChangesScript = ''
    count_changes() {
      local mod="$1"
      local staged unstaged untracked
      
      # Check if this is a root category
      if is_root_category "$mod"; then
        # Get patterns for this category
        patterns=($(get_root_category_patterns "$mod"))
        staged=0
        unstaged=0
        untracked=0
        
        for pattern in "''${patterns[@]}"; do
          # Handle both files and directories
          if [[ "$pattern" == */ ]]; then
            # Directory pattern
            staged=$((staged + $($git diff --cached --name-only -- "$pattern" 2>/dev/null | wc -l)))
            unstaged=$((unstaged + $($git diff --name-only -- "$pattern" 2>/dev/null | wc -l)))
            untracked=$((untracked + $($git ls-files --others --exclude-standard -- "$pattern" 2>/dev/null | wc -l)))
          else
            # File pattern (may include wildcards)
            staged=$((staged + $($git diff --cached --name-only -- "$pattern" 2>/dev/null | wc -l)))
            unstaged=$((unstaged + $($git diff --name-only -- "$pattern" 2>/dev/null | wc -l)))
            untracked=$((untracked + $($git ls-files --others --exclude-standard -- "$pattern" 2>/dev/null | wc -l)))
          fi
        done
      # Special handling for router: exclude middleware subdirs (separate modules)
      # but include files directly in router/middleware/ (e.g. README.md)
      elif [ "$mod" = "router" ]; then
        staged=$($git diff --cached --name-only -- "$mod/" 2>/dev/null | grep -v "^router/middleware/[^/]\+/" | wc -l)
        unstaged=$($git diff --name-only -- "$mod/" 2>/dev/null | grep -v "^router/middleware/[^/]\+/" | wc -l)
        untracked=$($git ls-files --others --exclude-standard -- "$mod/" 2>/dev/null | grep -v "^router/middleware/[^/]\+/" | wc -l)
      else
        staged=$($git diff --cached --name-only -- "$mod/" 2>/dev/null | wc -l)
        unstaged=$($git diff --name-only -- "$mod/" 2>/dev/null | wc -l)
        untracked=$($git ls-files --others --exclude-standard -- "$mod/" 2>/dev/null | wc -l)
      fi
      
      echo "$staged $unstaged $untracked"
    }
  '';

  # Ensure we're at the repository root
  repoRootCheck = ''
    repo_root=$($git rev-parse --show-toplevel 2>/dev/null)
    if [ -z "$repo_root" ]; then
      $gum style --foreground ${lib.colors.error} "✗ Not in a git repository"
      exit 1
    fi
    cd "$repo_root" || exit 1
  '';

  # Discover root-level Go modules and root categories
  findModulesScript = ''
    find_modules() {
      # Root categories (virtual modules for root-level files)
      # Only shown if they have changes
      echo "ci"
      echo "nix"
      echo "dotfiles"
      echo "deps"
      echo "docs"
      
      # Go modules
      ${pkgs.findutils}/bin/find . ${lib.findPatterns.rootModules} | sed 's|^\./||' | sort
    }
  '';

  # Get file patterns for root categories
  getRootCategoryPatternsScript = ''
    get_root_category_patterns() {
      local category="$1"
      case "$category" in
        ci)
          echo ".github/" "codecov.yml"
          ;;
        nix)
          echo "nix/" "flake.nix" "flake.lock"
          ;;
        dotfiles)
          echo ".editorconfig" ".envrc" ".gitignore" ".golangci.yaml" ".golangci-soft.yaml"
          ;;
        deps)
          echo "go.work" "go.work.sum"
          ;;
        docs)
          echo "README.md" "LICENSE"
          ;;
        *)
          return 1
          ;;
      esac
    }
  '';

  # Check if a path is a root category
  isRootCategoryScript = ''
    is_root_category() {
      local mod="$1"
      case "$mod" in
        ci|nix|dotfiles|deps|docs)
          return 0
          ;;
        *)
          return 1
          ;;
      esac
    }
  '';

  # Check for changes outside Go modules and print warning if any
  # Args: $1 = newline-separated list of module paths
  checkOutsideChangesScript = ''
    check_outside_changes() {
      local modules="$1"
      local other_changes module_changes outside_changes

      other_changes=$($git status --porcelain 2>/dev/null | wc -l)
      module_changes=0

      while IFS= read -r mod; do
        [ -z "$mod" ] && continue
        count=$($git status --porcelain -- "$mod/" 2>/dev/null | wc -l)
        module_changes=$((module_changes + count))
      done <<< "$modules"

      outside_changes=$((other_changes - module_changes))
      if [ "$outside_changes" -gt 0 ]; then
        echo ""
        $gum style --foreground ${lib.colors.accent4} "  ⚠ $outside_changes file(s) changed outside Go modules"
        return 0
      fi
      return 1
    }
  '';

  # Clean AI output: strip quotes and trim whitespace
  cleanAiOutputScript = ''
    clean_ai_output() {
      echo "$1" | sed 's/^"//;s/"$//' | sed "s/^'//;s/'$//" | xargs
    }
  '';

  # Strip markdown code fences from AI output
  stripMarkdownScript = ''
    strip_markdown() {
      sed '/^```/d'
    }
  '';

  # Common AI prompt rules (shared between simple and complex prompts)
  aiPromptRules = ''
RULES:
- Start with lowercase verb (add, fix, refactor, update, etc.)
- No module name in message
- Max 50 chars for title line
- PLAIN TEXT ONLY - no markdown, no code fences, no quotes
- Just output the message, nothing else'';

in
{
  # Check which modules have uncommitted changes
  commit-check = {
    type = "app";
    meta.description = "Check modules with uncommitted changes";
    program = toString (pkgs.writeShellScript "rivaas-commit-check" ''
      set -uo pipefail

      gum="${pkgs.gum}/bin/gum"
      git="${pkgs.git}/bin/git"

      ${repoRootCheck}
      ${countChangesScript}
      ${findModulesScript}
      ${getRootCategoryPatternsScript}
      ${isRootCategoryScript}
      ${checkOutsideChangesScript}

      $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "Module Changes Status"
      echo ""

      modules=$(find_modules)

      if [ -z "$modules" ]; then
        $gum style --foreground ${lib.colors.info} "No modules found"
        exit 0
      fi

      has_changes=0
      clean=0

      for mod in $modules; do
        [ -z "$mod" ] && continue

        read -r staged unstaged untracked <<< "$(count_changes "$mod")"
        total=$((staged + unstaged + untracked))

        # Format display name for root categories
        if is_root_category "$mod"; then
          display_name="root($mod)"
        else
          display_name="$mod"
        fi

        if [ "$total" -gt 0 ]; then
          $gum style --foreground ${lib.colors.accent3} --bold "● $display_name"
          [ "$staged" -gt 0 ] && $gum style --foreground ${lib.colors.success} "  Staged: $staged file(s)"
          [ "$unstaged" -gt 0 ] && $gum style --foreground ${lib.colors.accent4} "  Modified: $unstaged file(s)"
          [ "$untracked" -gt 0 ] && $gum style --foreground ${lib.colors.accent1} "  Untracked: $untracked file(s)"
          has_changes=$((has_changes + 1))
        else
          $gum style --foreground ${lib.colors.success} "✓ $display_name"
          $gum style --faint "  Nothing to commit"
          clean=$((clean + 1))
        fi
        echo ""
      done

      # Summary
      $gum style --foreground ${lib.colors.header} --bold "Summary"
      [ $has_changes -gt 0 ] && $gum style --foreground ${lib.colors.accent3} "  ● $has_changes module(s) with uncommitted changes"
      [ $clean -gt 0 ] && $gum style --foreground ${lib.colors.success} "  ✓ $clean module(s) with nothing to commit"

      check_outside_changes "$modules"
    '');
  };

  # Interactive commit tool with AI-generated messages
  commit = {
    type = "app";
    meta.description = "Interactive commit tool with AI-generated messages per module";
    program = toString (pkgs.writeShellScript "rivaas-commit" ''
      set -uo pipefail

      gum="${pkgs.gum}/bin/gum"
      git="${pkgs.git}/bin/git"

      ${repoRootCheck}
      ${countChangesScript}
      ${findModulesScript}
      ${getRootCategoryPatternsScript}
      ${isRootCategoryScript}
      ${checkOutsideChangesScript}
      ${cleanAiOutputScript}
      ${stripMarkdownScript}
      ${lib.moduleHelpers.shortenPrefixScript}

      # Ensure workspace is trusted (one-time operation)
      ensure_workspace_trust() {
        # Create minimal workspace config to avoid trust prompt
        mkdir -p .cursor
        if [ ! -f .cursor/cli.json ]; then
          echo '{}' > .cursor/cli.json
        fi
        
        # Test if cursor-agent works (15s timeout - first request can be slow due to API latency)
        local test_output
        if test_output=$(timeout 15 ${pkgs.cursor-cli}/bin/cursor-agent --model auto -p --output-format text "test" 2>&1); then
          # Check if we got actual output (not just exit 0)
          if [ -n "$test_output" ]; then
            return 0
          else
            # Exit 0 but no output - likely API/model access issue
            return 2
          fi
        else
          return 1
        fi
      }

      # Generate AI commit message with timeout and error handling
      generate_ai_message() {
        local prompt="$1"
        local output
        if output=$(timeout ${toString cfg.aiTimeoutSec} ${pkgs.cursor-cli}/bin/cursor-agent --model auto -p --output-format text "$prompt" 2>&1); then
          echo "$output"
          return 0
        else
          return 1
        fi
      }

      # Check for cursor-agent CLI
      if ! command -v ${pkgs.cursor-cli}/bin/cursor-agent &> /dev/null; then
        $gum style --foreground ${lib.colors.error} "✗ cursor-agent CLI not found in PATH"
        exit 1
      fi

      # Check if logged in
      login_status=$(${pkgs.cursor-cli}/bin/cursor-agent status 2>&1)
      if echo "$login_status" | grep -q "Not logged in"; then
        $gum style --foreground ${lib.colors.error} "✗ cursor-agent not logged in"
        $gum style --faint "  Run: cursor-agent login"

        if $gum confirm "Login now?"; then
          NO_OPEN_BROWSER=1 ${pkgs.cursor-cli}/bin/cursor-agent login

          login_status=$(${pkgs.cursor-cli}/bin/cursor-agent status 2>&1)
          if echo "$login_status" | grep -q "Not logged in"; then
            $gum style --foreground ${lib.colors.error} "✗ Login failed"
            exit 1
          fi
          $gum style --foreground ${lib.colors.success} "✓ Logged in successfully"
          echo ""
        else
          exit 1
        fi
      fi

      # Ensure workspace is trusted (auto-accept trust prompt if needed)
      $gum style --foreground ${lib.colors.info} "Checking workspace trust..."
      trust_result=0
      ensure_workspace_trust || trust_result=$?
      
      if [ $trust_result -eq 2 ]; then
        $gum style --foreground ${lib.colors.error} "✗ Cursor Agent API not responding"
        $gum style --faint "  The cursor-agent CLI connects but returns no output."
        $gum style --faint ""
        $gum style --faint "  Possible causes:"
        $gum style --faint "    • Account lacks CLI/API entitlements (may need subscription)"
        $gum style --faint "    • Ghost mode enabled in ~/.cursor/cli-config.json"
        $gum style --faint "    • API service issue or rate limiting"
        $gum style --faint ""
        $gum style --faint "  To diagnose:"
        $gum style --faint "    1. Check: cursor-agent status"
        $gum style --faint "    2. Try: cursor-agent --model auto -p --output-format text 'hello'"
        $gum style --faint "    3. Review ~/.cursor/cli-config.json (ghostMode, privacyMode)"
        $gum style --faint ""
        $gum style --faint "  Falling back to manual commit messages..."
        echo ""
      elif [ $trust_result -ne 0 ]; then
        $gum style --foreground ${lib.colors.error} "✗ Cursor Agent connection failed"
        $gum style --faint "  Could not connect to Cursor Agent API (timeout or trust issue)"
        $gum style --faint "  Try: cursor-agent --model auto -p --output-format text 'test'"
        $gum style --faint "  If slow, the API may need a moment to warm up"
        echo ""
      fi
      
      # If trust check failed, we'll continue but AI generation will fail
      # The script will fall back to default messages

      $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "Interactive Module Commit"
      echo ""

      modules=$(find_modules)

      if [ -z "$modules" ]; then
        $gum style --foreground ${lib.colors.info} "No modules found"
        exit 0
      fi

      # Find modules with changes
      modules_with_changes=""
      for mod in $modules; do
        [ -z "$mod" ] && continue

        read -r staged unstaged untracked <<< "$(count_changes "$mod")"
        total=$((staged + unstaged + untracked))

        [ "$total" -gt 0 ] && modules_with_changes="$modules_with_changes"$'\n'"$mod"
      done

      modules_with_changes=$(echo "$modules_with_changes" | sed '/^$/d')

      if [ -z "$modules_with_changes" ]; then
        $gum style --foreground ${lib.colors.info} "Nothing to commit in Go modules"

        if check_outside_changes "$modules"; then
          $gum style --faint "  Use 'git status' to see all changes"
        fi
        exit 0
      fi

      # Display modules with changes as a table
      $gum style --foreground ${lib.colors.info} "Modules with uncommitted changes:"
      echo ""

      table_data="Module,Staged,Modified,Untracked"
      while IFS= read -r mod; do
        [ -z "$mod" ] && continue
        read -r staged unstaged untracked <<< "$(count_changes "$mod")"
        # Display root categories as root(name) for clarity
        if is_root_category "$mod"; then
          display_name="root($mod)"
        else
          display_name="$mod"
        fi
        table_data="$table_data"$'\n'"$display_name,$staged,$unstaged,$untracked"
      done <<< "$modules_with_changes"

      echo "$table_data" | $gum table --print --border.foreground ${lib.colors.accent1}
      echo ""

      # Multi-select modules to commit (pipe newline-separated list to gum choose)
      $gum style --faint "Use Space to select, Enter to confirm"
      
      # Format modules with root() prefix for display
      formatted_modules=""
      while IFS= read -r mod; do
        [ -z "$mod" ] && continue
        if is_root_category "$mod"; then
          formatted_modules="$formatted_modules"$'\n'"root($mod)|$mod"
        else
          formatted_modules="$formatted_modules"$'\n'"$mod|$mod"
        fi
      done <<< "$modules_with_changes"
      formatted_modules=$(echo "$formatted_modules" | sed '/^$/d')
      
      # Show formatted names but get original module names
      selected_display=$(echo "$formatted_modules" | cut -d'|' -f1 | $gum choose --no-limit --header "Select modules to commit:")
      
      if [ -z "$selected_display" ]; then
        $gum style --foreground ${lib.colors.info} "No modules selected"
        exit 0
      fi
      
      # Convert back to original module names
      selected=""
      while IFS= read -r display_name; do
        [ -z "$display_name" ] && continue
        # Extract original name from formatted list
        original=$(echo "$formatted_modules" | grep "^$display_name|" | cut -d'|' -f2)
        selected="$selected"$'\n'"$original"
      done <<< "$selected_display"
      selected=$(echo "$selected" | sed '/^$/d')

      # Count selected modules for progress indicator
      total_selected=$(echo "$selected" | wc -l)

      current_idx=0
      committed_count=0

      # Read all selected modules into an array first (avoids stdin issues in loop)
      readarray -t selected_modules <<< "$selected"

      # Process each selected module
      for mod in "''${selected_modules[@]}"; do
        [ -z "$mod" ] && continue
        current_idx=$((current_idx + 1))

        # Format display name for root categories
        if is_root_category "$mod"; then
          display_name="root($mod)"
        else
          display_name="$mod"
        fi

        echo ""
        $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "[$current_idx/$total_selected] $display_name"

        # Show changed files
        $gum style --foreground ${lib.colors.info} "Changed files:"
        {
          # Root category: show files matching category patterns
          if is_root_category "$mod"; then
            patterns=($(get_root_category_patterns "$mod"))
            for pattern in "''${patterns[@]}"; do
              $git diff --cached --name-only -- "$pattern" 2>/dev/null | while read -r f; do
                [ -n "$f" ] && $gum style --foreground ${lib.colors.success} "  [staged] $f"
              done
              $git diff --name-only --diff-filter=M -- "$pattern" 2>/dev/null | while read -r f; do
                [ -n "$f" ] && $gum style --foreground ${lib.colors.accent4} "  [modified] $f"
              done
              $git diff --name-only --diff-filter=D -- "$pattern" 2>/dev/null | while read -r f; do
                [ -n "$f" ] && $gum style --foreground ${lib.colors.error} "  [deleted] $f"
              done
              $git ls-files --others --exclude-standard -- "$pattern" 2>/dev/null | while read -r f; do
                [ -n "$f" ] && $gum style --foreground ${lib.colors.accent1} "  [untracked] $f"
              done
            done
          # Special exclusion for router module: exclude middleware subdirs but include router/middleware/* files
          elif [ "$mod" = "router" ]; then
            $git diff --cached --name-only -- "$mod/" 2>/dev/null | grep -v "^router/middleware/[^/]\+/" | while read -r f; do
              [ -n "$f" ] && $gum style --foreground ${lib.colors.success} "  [staged] $f"
            done
            $git diff --name-only --diff-filter=M -- "$mod/" 2>/dev/null | grep -v "^router/middleware/[^/]\+/" | while read -r f; do
              [ -n "$f" ] && $gum style --foreground ${lib.colors.accent4} "  [modified] $f"
            done
            $git diff --name-only --diff-filter=D -- "$mod/" 2>/dev/null | grep -v "^router/middleware/[^/]\+/" | while read -r f; do
              [ -n "$f" ] && $gum style --foreground ${lib.colors.error} "  [deleted] $f"
            done
            $git ls-files --others --exclude-standard -- "$mod/" 2>/dev/null | grep -v "^router/middleware/[^/]\+/" | while read -r f; do
              [ -n "$f" ] && $gum style --foreground ${lib.colors.accent1} "  [untracked] $f"
            done
          else
            $git diff --cached --name-only -- "$mod/" 2>/dev/null | while read -r f; do
              [ -n "$f" ] && $gum style --foreground ${lib.colors.success} "  [staged] $f"
            done
            $git diff --name-only --diff-filter=M -- "$mod/" 2>/dev/null | while read -r f; do
              [ -n "$f" ] && $gum style --foreground ${lib.colors.accent4} "  [modified] $f"
            done
            $git diff --name-only --diff-filter=D -- "$mod/" 2>/dev/null | while read -r f; do
              [ -n "$f" ] && $gum style --foreground ${lib.colors.error} "  [deleted] $f"
            done
            $git ls-files --others --exclude-standard -- "$mod/" 2>/dev/null | while read -r f; do
              [ -n "$f" ] && $gum style --foreground ${lib.colors.accent1} "  [untracked] $f"
            done
          fi
        }
        echo ""

        # Stage all changes for this module
        if is_root_category "$mod"; then
          # Root category: stage files matching category patterns
          patterns=($(get_root_category_patterns "$mod"))
          for pattern in "''${patterns[@]}"; do
            $git add "$pattern" 2>/dev/null || true
          done
        # Special handling for router: exclude middleware subdirs (they're separate modules)
        # but include files directly in router/middleware/ (e.g. README.md)
        # Use grep-based filtering so deleted files are excluded reliably
        elif [ "$mod" = "router" ]; then
          {
            $git diff --name-only -- "$mod/" 2>/dev/null
            $git ls-files --others --exclude-standard -- "$mod/" 2>/dev/null
            $git ls-files --deleted -- "$mod/" 2>/dev/null
          } | sort -u | grep -v "^router/middleware/[^/]\+/" | xargs -r $git add --
        else
          $git add "$mod/"
        fi

        # Get the diff for AI context (limit by bytes to avoid huge prompts)
        diff_limit=${toString cfg.diffTruncateBytes}
        if is_root_category "$mod"; then
          # For root categories, get diff for all patterns
          patterns=($(get_root_category_patterns "$mod"))
          diff_content=""
          for pattern in "''${patterns[@]}"; do
            diff_content="$diff_content"$'\n'"$($git diff --cached -- "$pattern" 2>/dev/null)"
          done
          diff_content=$(echo "$diff_content" | head -c "$diff_limit")
          diff_full_size=$(for pattern in "''${patterns[@]}"; do $git diff --cached -- "$pattern" 2>/dev/null; done | wc -c | tr -d ' ')
        elif [ "$mod" = "router" ]; then
          diff_files=$($git diff --cached --name-only -- "$mod/" 2>/dev/null | grep -v "^router/middleware/[^/]\+/")
          if [ -n "$diff_files" ]; then
            diff_content=$(echo "$diff_files" | xargs -r $git diff --cached -- 2>/dev/null | head -c "$diff_limit")
            diff_full_size=$(echo "$diff_files" | xargs -r $git diff --cached -- 2>/dev/null | wc -c | tr -d ' ')
          else
            diff_content=""
            diff_full_size=0
          fi
        else
          diff_content=$($git diff --cached -- "$mod/" 2>/dev/null | head -c "$diff_limit")
          diff_full_size=$($git diff --cached -- "$mod/" 2>/dev/null | wc -c | tr -d ' ')
        fi

        if [ "$diff_full_size" -gt "$diff_limit" ]; then
          $gum style --foreground ${lib.colors.accent4} "  (diff truncated: $(($diff_full_size / 1024))KB → $((diff_limit / 1024))KB for AI context)"
          diff_content="$diff_content"$'\n\n'"[... truncated, full diff is $(($diff_full_size / 1024))KB ...]"
        fi

        # Count changed files to determine if this is a complex change
        if is_root_category "$mod"; then
          patterns=($(get_root_category_patterns "$mod"))
          file_count=0
          for pattern in "''${patterns[@]}"; do
            file_count=$((file_count + $($git diff --cached --name-only -- "$pattern" 2>/dev/null | wc -l)))
          done
        elif [ "$mod" = "router" ]; then
          file_count=$($git diff --cached --name-only -- "$mod/" 2>/dev/null | grep -v "^router/middleware/[^/]\+/" | wc -l | tr -d ' ')
        else
          file_count=$($git diff --cached --name-only -- "$mod/" 2>/dev/null | wc -l | tr -d ' ')
        fi

        $gum style --foreground ${lib.colors.info} "Generating commit message with AI..."

        if [ "$file_count" -gt ${toString cfg.complexChangeThreshold} ]; then
          # Complex change: generate title + body
          prompt="Write a git commit message for this diff.

OUTPUT FORMAT (plain text, NO markdown):
<title line: max 50 chars, lowercase verb>

- bullet point 1
- bullet point 2

EXAMPLE OUTPUT:
refactor error handling across handlers

- update middleware error types
- fix inconsistent error messages

${aiPromptRules}
- 2-3 bullet points describing changes

Diff:
$diff_content"

          if raw_output=$(generate_ai_message "$prompt"); then
            raw_output=$(echo "$raw_output" | strip_markdown)
            ai_title=$(echo "$raw_output" | sed '/^[[:space:]]*$/d' | head -1)
            ai_body=$(echo "$raw_output" | sed '1,/^[[:space:]]*$/d' | sed '/^[[:space:]]*$/d')
          else
            ai_title=""
            ai_body=""
          fi

          if [ -z "$ai_title" ]; then
            ai_title="update $mod"
            ai_body=""
            $gum style --foreground ${lib.colors.accent4} "  (AI generation failed or timed out, using default)"
          fi

          ai_title=$(clean_ai_output "$ai_title")

          commit_prefix=$(shorten_prefix "$mod")
          ai_message_with_prefix="$commit_prefix: $ai_title"
          [ -n "$ai_body" ] && ai_message_with_prefix="$ai_message_with_prefix

$ai_body"

          $gum style --foreground ${lib.colors.info} "Edit commit message (Ctrl+D to save, Esc to cancel):"
          message=$($gum write --width ${toString cfg.editorWidth} --height ${toString cfg.editorHeight} --value "$ai_message_with_prefix" </dev/tty)
        else
          # Simple change: title only
          prompt="Write a git commit message title (max 50 chars).

EXAMPLE OUTPUT:
format imports and remove unused

${aiPromptRules}

Diff:
$diff_content"

          if raw_output=$(generate_ai_message "$prompt"); then
            ai_message=$(echo "$raw_output" | strip_markdown | sed '/^[[:space:]]*$/d' | head -1)
          else
            ai_message=""
          fi

          if [ -z "$ai_message" ]; then
            ai_message="update $mod"
            $gum style --foreground ${lib.colors.accent4} "  (AI generation failed or timed out, using default)"
          fi

          ai_message=$(clean_ai_output "$ai_message")
          commit_prefix=$(shorten_prefix "$mod")
          ai_message_with_prefix="$commit_prefix: $ai_message"

          $gum style --foreground ${lib.colors.info} "Edit commit message:"
          message=$($gum input --width ${toString cfg.inputWidth} --value "$ai_message_with_prefix" </dev/tty)
        fi

        if [ -z "$message" ]; then
          if is_root_category "$mod"; then
            patterns=($(get_root_category_patterns "$mod"))
            for pattern in "''${patterns[@]}"; do
              $git reset HEAD -- "$pattern" >/dev/null 2>&1 || true
            done
          else
            $git reset HEAD -- "$mod/" >/dev/null 2>&1 || true
          fi
          $gum style --foreground ${lib.colors.info} "Skipping $display_name (no message)"
          continue
        fi

        # Use the message as-is (prefix already included or user modified it)
        final_message="$message"

        # Show preview (handle multi-line messages nicely)
        echo ""
        if [[ "$final_message" == *$'\n'* ]]; then
          # Multi-line: show title and indicate body exists
          first_line=$(echo "$final_message" | head -1)
          body_lines=$(echo "$final_message" | tail -n +2 | grep -c . || echo 0)
          $gum style --foreground ${lib.colors.accent2} "Commit: $first_line"
          [ "$body_lines" -gt 0 ] && $gum style --faint "  (+$body_lines more lines in body)"
        else
          $gum style --foreground ${lib.colors.accent2} "Commit: $final_message"
        fi
        echo ""

        if $gum confirm "Commit?" </dev/tty; then
          # Use temp file instead of heredoc to avoid stdin issues in loop
          commit_msg_file=$(mktemp)
          printf '%s\n' "$final_message" > "$commit_msg_file"
          if $git commit -F "$commit_msg_file"; then
            rm -f "$commit_msg_file"
            $gum style --foreground ${lib.colors.success} "✓ Committed: $display_name"
            committed_count=$((committed_count + 1))
          else
            rm -f "$commit_msg_file"
            $gum style --foreground ${lib.colors.error} "✗ Failed to commit $display_name"
          fi
        else
          if is_root_category "$mod"; then
            patterns=($(get_root_category_patterns "$mod"))
            for pattern in "''${patterns[@]}"; do
              $git reset HEAD -- "$pattern" >/dev/null 2>&1 || true
            done
          else
            $git reset HEAD -- "$mod/" >/dev/null 2>&1 || true
          fi
          $gum style --foreground ${lib.colors.info} "Skipped $display_name"
        fi
      done

      # Summary
      echo ""
      $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "Summary"
      if [ $committed_count -gt 0 ]; then
        $gum style --foreground ${lib.colors.success} "✓ Created $committed_count commit(s)"
        echo ""
        $gum style --faint "Recent commits:"
        $git log --oneline -n "$committed_count" | while read -r line; do
          $gum style --faint "  $line"
        done
      else
        $gum style --foreground ${lib.colors.info} "No commits were created"
      fi
    '');
  };
}
