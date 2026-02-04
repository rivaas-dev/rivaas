# Module commit app: stage and commit changes per module with AI-generated messages
{ pkgs, lib }:

let
  # =============================================================================
  # Configuration Constants
  # =============================================================================
  cfg = {
    aiTimeoutSec = 30;           # Timeout for AI message generation
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
      staged=$($git diff --cached --name-only -- "$mod/" 2>/dev/null | wc -l)
      unstaged=$($git diff --name-only -- "$mod/" 2>/dev/null | wc -l)
      untracked=$($git ls-files --others --exclude-standard -- "$mod/" 2>/dev/null | wc -l)
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

  # Discover root-level Go modules
  findModulesScript = ''
    find_modules() {
      ${pkgs.findutils}/bin/find . ${lib.findPatterns.rootModules} | sed 's|^\./||' | sort
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

        if [ "$total" -gt 0 ]; then
          $gum style --foreground ${lib.colors.accent3} --bold "● $mod"
          [ "$staged" -gt 0 ] && $gum style --foreground ${lib.colors.success} "  Staged: $staged file(s)"
          [ "$unstaged" -gt 0 ] && $gum style --foreground ${lib.colors.accent4} "  Modified: $unstaged file(s)"
          [ "$untracked" -gt 0 ] && $gum style --foreground ${lib.colors.accent1} "  Untracked: $untracked file(s)"
          has_changes=$((has_changes + 1))
        else
          $gum style --foreground ${lib.colors.success} "✓ $mod"
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
      ${checkOutsideChangesScript}
      ${cleanAiOutputScript}
      ${stripMarkdownScript}

      # Ensure workspace is trusted (one-time operation)
      ensure_workspace_trust() {
        # Create minimal workspace config to avoid trust prompt
        mkdir -p .cursor
        if [ ! -f .cursor/cli.json ]; then
          echo '{}' > .cursor/cli.json
        fi
        
        # Test if cursor-agent works (15s timeout - first request can be slow due to API latency)
        local test_output
        if test_output=$(timeout 15 ${pkgs.cursor-cli}/bin/cursor-agent -p --output-format text "test" 2>&1); then
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
        if output=$(timeout ${toString cfg.aiTimeoutSec} ${pkgs.cursor-cli}/bin/cursor-agent -p --output-format text "$prompt" 2>&1); then
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
        $gum style --faint "    2. Try: cursor-agent -p --output-format text 'hello'"
        $gum style --faint "    3. Review ~/.cursor/cli-config.json (ghostMode, privacyMode)"
        $gum style --faint ""
        $gum style --faint "  Falling back to manual commit messages..."
        echo ""
      elif [ $trust_result -ne 0 ]; then
        $gum style --foreground ${lib.colors.error} "✗ Cursor Agent connection failed"
        $gum style --faint "  Could not connect to Cursor Agent API (timeout or trust issue)"
        $gum style --faint "  Try: cursor-agent -p --output-format text 'test'"
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
        table_data="$table_data"$'\n'"$mod,$staged,$unstaged,$untracked"
      done <<< "$modules_with_changes"

      echo "$table_data" | $gum table --print --border.foreground ${lib.colors.accent1}
      echo ""

      # Multi-select modules to commit (pipe newline-separated list to gum choose)
      $gum style --faint "Use Space to select, Enter to confirm"
      selected=$(echo "$modules_with_changes" | $gum choose --no-limit --header "Select modules to commit:")

      if [ -z "$selected" ]; then
        $gum style --foreground ${lib.colors.info} "No modules selected"
        exit 0
      fi

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

        echo ""
        $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "[$current_idx/$total_selected] $mod"

        # Show changed files
        $gum style --foreground ${lib.colors.info} "Changed files:"
        {
          $git diff --cached --name-only -- "$mod/" 2>/dev/null | while read -r f; do
            [ -n "$f" ] && $gum style --foreground ${lib.colors.success} "  [staged] $f"
          done
          $git diff --name-only -- "$mod/" 2>/dev/null | while read -r f; do
            [ -n "$f" ] && $gum style --foreground ${lib.colors.accent4} "  [modified] $f"
          done
          $git ls-files --others --exclude-standard -- "$mod/" 2>/dev/null | while read -r f; do
            [ -n "$f" ] && $gum style --foreground ${lib.colors.accent1} "  [untracked] $f"
          done
        }
        echo ""

        # Stage all changes for this module
        $git add "$mod/"

        # Get the diff for AI context (limit by bytes to avoid huge prompts)
        diff_limit=${toString cfg.diffTruncateBytes}
        diff_content=$($git diff --cached -- "$mod/" 2>/dev/null | head -c "$diff_limit")
        diff_full_size=$($git diff --cached -- "$mod/" 2>/dev/null | wc -c | tr -d ' ')

        if [ "$diff_full_size" -gt "$diff_limit" ]; then
          $gum style --foreground ${lib.colors.accent4} "  (diff truncated: $(($diff_full_size / 1024))KB → $((diff_limit / 1024))KB for AI context)"
          diff_content="$diff_content"$'\n\n'"[... truncated, full diff is $(($diff_full_size / 1024))KB ...]"
        fi

        # Count changed files to determine if this is a complex change
        file_count=$($git diff --cached --name-only -- "$mod/" 2>/dev/null | wc -l | tr -d ' ')

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

          ai_message_with_prefix="$mod: $ai_title"
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
          ai_message_with_prefix="$mod: $ai_message"

          $gum style --foreground ${lib.colors.info} "Edit commit message:"
          message=$($gum input --width ${toString cfg.inputWidth} --value "$ai_message_with_prefix" </dev/tty)
        fi

        if [ -z "$message" ]; then
          $git reset HEAD -- "$mod/" >/dev/null 2>&1 || true
          $gum style --foreground ${lib.colors.info} "Skipping $mod (no message)"
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
            $gum style --foreground ${lib.colors.success} "✓ Committed: $mod"
            committed_count=$((committed_count + 1))
          else
            rm -f "$commit_msg_file"
            $gum style --foreground ${lib.colors.error} "✗ Failed to commit $mod"
          fi
        else
          $git reset HEAD -- "$mod/" >/dev/null 2>&1 || true
          $gum style --foreground ${lib.colors.info} "Skipped $mod"
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
