# Module commit app: stage and commit changes per module with AI-generated messages
{ pkgs, lib }:

let
  # Helper function to count changes for a module
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

  # Common setup: ensure we're at repo root
  repoRootCheck = ''
    # Ensure we're at the repository root
    repo_root=$($git rev-parse --show-toplevel 2>/dev/null)
    if [ -z "$repo_root" ]; then
      $gum style --foreground ${lib.colors.error} "✗ Not in a git repository"
      exit 1
    fi
    cd "$repo_root" || exit 1
  '';
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

      $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "Module Changes Status"
      echo ""

      # Root-level modules only
      modules=$(${pkgs.findutils}/bin/find . ${lib.findPatterns.rootModules} | sed 's|^\./||' | sort)

      if [ -z "$modules" ]; then
        $gum style --foreground ${lib.colors.info} "No modules found"
        exit 0
      fi

      has_changes=0
      clean=0

      for mod in $modules; do
        [ -z "$mod" ] && continue

        # Get changes for this module (staged + unstaged + untracked)
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

      # Check for changes outside Go modules
      other_changes=$($git status --porcelain 2>/dev/null | wc -l)
      module_changes=$(for mod in $modules; do
        $git status --porcelain -- "$mod/" 2>/dev/null
      done | wc -l)
      outside_changes=$((other_changes - module_changes))
      if [ "$outside_changes" -gt 0 ]; then
        echo ""
        $gum style --foreground ${lib.colors.accent4} "  ⚠ $outside_changes file(s) changed outside Go modules"
      fi
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

      # Generate AI commit message with timeout and error handling
      generate_ai_message() {
        local prompt="$1"
        local timeout_sec=30

        # Run cursor-agent with timeout, capture stdout separately from stderr
        local output
        if output=$(timeout "$timeout_sec" ${pkgs.cursor-cli}/bin/cursor-agent -p --output-format text "$prompt" 2>/dev/null); then
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
          # Prevent browser auto-open, show URL in terminal instead
          NO_OPEN_BROWSER=1 ${pkgs.cursor-cli}/bin/cursor-agent login

          # Re-check after login attempt
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

      $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "Interactive Module Commit"
      echo ""

      # Root-level modules only
      modules=$(${pkgs.findutils}/bin/find . ${lib.findPatterns.rootModules} | sed 's|^\./||' | sort)

      if [ -z "$modules" ]; then
        $gum style --foreground ${lib.colors.info} "No modules found"
        exit 0
      fi

      # Find modules with changes (store in array-like newline-separated string)
      modules_with_changes=""
      for mod in $modules; do
        [ -z "$mod" ] && continue

        read -r staged unstaged untracked <<< "$(count_changes "$mod")"
        total=$((staged + unstaged + untracked))

        [ "$total" -gt 0 ] && modules_with_changes="$modules_with_changes"$'\n'"$mod"
      done

      # Trim leading newline and check if empty
      modules_with_changes=$(echo "$modules_with_changes" | sed '/^$/d')

      if [ -z "$modules_with_changes" ]; then
        $gum style --foreground ${lib.colors.info} "Nothing to commit in Go modules"

        # Check for changes outside Go modules
        other_changes=$($git status --porcelain 2>/dev/null | wc -l)
        if [ "$other_changes" -gt 0 ]; then
          echo ""
          $gum style --foreground ${lib.colors.accent4} "Note: $other_changes file(s) changed outside Go modules"
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

      # Process each selected module
      while IFS= read -r mod; do
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
        # Use 20KB limit which is roughly 400-500 lines of code
        diff_content=$($git diff --cached -- "$mod/" 2>/dev/null | head -c 20000)
        diff_full_size=$($git diff --cached -- "$mod/" 2>/dev/null | wc -c)

        if [ "$diff_full_size" -gt 20000 ]; then
          $gum style --foreground ${lib.colors.accent4} "  (diff truncated: $(($diff_full_size / 1024))KB → 20KB for AI context)"
          # Add summary of what was truncated
          diff_content="$diff_content"$'\n\n'"[... truncated, full diff is $(($diff_full_size / 1024))KB ...]"
        fi

        # Count changed files to determine if this is a complex change
        file_count=$($git diff --cached --name-only -- "$mod/" 2>/dev/null | wc -l)

        # Generate commit message using cursor-agent
        $gum style --foreground ${lib.colors.info} "Generating commit message with AI..."

        if [ "$file_count" -gt 5 ]; then
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

RULES:
- First line is the title (max 50 chars, start with lowercase verb)
- No module name in title
- 2-3 bullet points describing changes
- PLAIN TEXT ONLY - no markdown, no code fences, no quotes

Diff:
$diff_content"

          if raw_output=$(generate_ai_message "$prompt"); then
            # Strip any markdown code fences the AI might have added
            raw_output=$(echo "$raw_output" | sed '/^```/d')

            # Extract title (first non-empty line) and body (rest)
            ai_title=$(echo "$raw_output" | sed '/^[[:space:]]*$/d' | head -1)
            ai_body=$(echo "$raw_output" | sed '1,/^[[:space:]]*$/d' | sed '/^[[:space:]]*$/d')
          else
            ai_title=""
            ai_body=""
          fi

          # Fallback if cursor-agent failed
          if [ -z "$ai_title" ]; then
            ai_title="update $mod"
            ai_body=""
            $gum style --foreground ${lib.colors.accent4} "  (AI generation failed or timed out, using default)"
          fi

          # Clean up the title
          ai_title=$(echo "$ai_title" | sed 's/^"//;s/"$//' | sed "s/^'//;s/'$//" | xargs)

          # Add module prefix
          ai_message_with_prefix="$mod: $ai_title"
          [ -n "$ai_body" ] && ai_message_with_prefix="$ai_message_with_prefix

$ai_body"

          # Let user edit with multi-line editor
          $gum style --foreground ${lib.colors.info} "Edit commit message (Ctrl+D to save, Esc to cancel):"
          message=$($gum write --width 80 --height 10 --value "$ai_message_with_prefix")
        else
          # Simple change: title only
          prompt="Write a git commit message title (max 50 chars).

EXAMPLE OUTPUT:
format imports and remove unused

RULES:
- Start with lowercase verb
- No module name
- PLAIN TEXT ONLY - no markdown, no code fences, no quotes
- Just output the message, nothing else

Diff:
$diff_content"

          if raw_output=$(generate_ai_message "$prompt"); then
            # Strip any markdown code fences and get first non-empty line
            ai_message=$(echo "$raw_output" | sed '/^```/d' | sed '/^[[:space:]]*$/d' | head -1)
          else
            ai_message=""
          fi

          # Fallback if cursor-agent failed
          if [ -z "$ai_message" ]; then
            ai_message="update $mod"
            $gum style --foreground ${lib.colors.accent4} "  (AI generation failed or timed out, using default)"
          fi

          # Clean up the message (remove quotes, trim whitespace)
          ai_message=$(echo "$ai_message" | sed 's/^"//;s/"$//' | sed "s/^'//;s/'$//" | xargs)

          # Add module prefix for editing
          ai_message_with_prefix="$mod: $ai_message"

          # Let user edit the message (with prefix visible)
          $gum style --foreground ${lib.colors.info} "Edit commit message:"
          message=$($gum input --width 72 --value "$ai_message_with_prefix")
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

        if $gum confirm "Commit?"; then
          # Use heredoc for reliable multi-line message handling
          if $git commit -F - <<EOF
$final_message
EOF
          then
            $gum style --foreground ${lib.colors.success} "✓ Committed: $mod"
            committed_count=$((committed_count + 1))
          else
            $gum style --foreground ${lib.colors.error} "✗ Failed to commit $mod"
          fi
        else
          $git reset HEAD -- "$mod/" >/dev/null 2>&1 || true
          $gum style --foreground ${lib.colors.info} "Skipped $mod"
        fi
      done <<< "$selected"

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
