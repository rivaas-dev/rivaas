# Module commit app: stage and commit changes per module with AI-generated messages
{ pkgs, lib }:

{
  # Check which modules have uncommitted changes
  commit-check = {
    type = "app";
    meta.description = "Check modules with uncommitted changes";
    program = toString (pkgs.writeShellScript "rivaas-commit-check" ''
      set -uo pipefail

      gum="${pkgs.gum}/bin/gum"
      git="${pkgs.git}/bin/git"

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
        staged=$($git diff --cached --name-only -- "$mod/" 2>/dev/null | wc -l)
        unstaged=$($git diff --name-only -- "$mod/" 2>/dev/null | wc -l)
        untracked=$($git ls-files --others --exclude-standard -- "$mod/" 2>/dev/null | wc -l)
        total=$((staged + unstaged + untracked))

        if [ "$total" -gt 0 ]; then
          $gum style --foreground ${lib.colors.accent3} --bold "● $mod"
          [ "$staged" -gt 0 ] && $gum style --foreground ${lib.colors.success} "  Staged: $staged file(s)"
          [ "$unstaged" -gt 0 ] && $gum style --foreground ${lib.colors.accent4} "  Modified: $unstaged file(s)"
          [ "$untracked" -gt 0 ] && $gum style --foreground ${lib.colors.accent1} "  Untracked: $untracked file(s)"
          has_changes=$((has_changes + 1))
        else
          $gum style --foreground ${lib.colors.success} "✓ $mod"
          $gum style --faint "  No changes"
          clean=$((clean + 1))
        fi
        echo ""
      done

      # Summary
      $gum style --foreground ${lib.colors.header} --bold "Summary"
      [ $has_changes -gt 0 ] && $gum style --foreground ${lib.colors.accent3} "  ● $has_changes module(s) with changes"
      [ $clean -gt 0 ] && $gum style --foreground ${lib.colors.success} "  ✓ $clean module(s) clean"
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

      # Check for cursor-agent CLI
      if ! command -v cursor-agent &> /dev/null; then
        $gum style --foreground ${lib.colors.error} "✗ cursor-agent CLI not found in PATH"
        exit 1
      fi

      # Check if logged in
      login_status=$(cursor-agent status 2>&1)
      if echo "$login_status" | grep -q "Not logged in"; then
        $gum style --foreground ${lib.colors.error} "✗ cursor-agent not logged in"
        $gum style --faint "  Run: cursor-agent login"

        if $gum confirm "Login now?"; then
          # Prevent browser auto-open, show URL in terminal instead
          NO_OPEN_BROWSER=1 cursor-agent login

          # Re-check after login attempt
          login_status=$(cursor-agent status 2>&1)
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

      # Find modules with changes
      modules_with_changes=""
      for mod in $modules; do
        [ -z "$mod" ] && continue

        staged=$($git diff --cached --name-only -- "$mod/" 2>/dev/null | wc -l)
        unstaged=$($git diff --name-only -- "$mod/" 2>/dev/null | wc -l)
        untracked=$($git ls-files --others --exclude-standard -- "$mod/" 2>/dev/null | wc -l)
        total=$((staged + unstaged + untracked))

        [ "$total" -gt 0 ] && modules_with_changes="$modules_with_changes $mod"
      done

      modules_with_changes=$(echo "$modules_with_changes" | xargs)

      if [ -z "$modules_with_changes" ]; then
        $gum style --foreground ${lib.colors.success} "✓ All modules are clean!"
        exit 0
      fi

      # Display modules with changes as a table
      $gum style --foreground ${lib.colors.info} "Modules with uncommitted changes:"
      echo ""

      table_data="Module,Staged,Modified,Untracked"
      for mod in $modules_with_changes; do
        staged=$($git diff --cached --name-only -- "$mod/" 2>/dev/null | wc -l)
        unstaged=$($git diff --name-only -- "$mod/" 2>/dev/null | wc -l)
        untracked=$($git ls-files --others --exclude-standard -- "$mod/" 2>/dev/null | wc -l)
        table_data="$table_data
$mod,$staged,$unstaged,$untracked"
      done

      echo "$table_data" | $gum table --print --border.foreground ${lib.colors.accent1}
      echo ""

      # Multi-select modules to commit
      $gum style --faint "Use Space to select, Enter to confirm"
      selected=$($gum choose --no-limit --header "Select modules to commit:" $modules_with_changes)

      if [ -z "$selected" ]; then
        $gum style --foreground ${lib.colors.info} "No modules selected"
        exit 0
      fi

      committed_count=0

      # Process each selected module
      for mod in $selected; do
        echo ""
        $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "$mod"

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

        # Get the diff for AI context (limit to avoid huge prompts)
        diff_content=$($git diff --cached -- "$mod/" 2>/dev/null | head -500)

        # Generate commit message using cursor-agent
        $gum style --foreground ${lib.colors.info} "Generating commit message with AI..."

        ai_message=$($gum spin --spinner dot --title "Cursor is thinking..." -- \
          cursor-agent -p --output-format text \
          "Generate a concise git commit message (one line, max 72 chars) for these changes.
Only output the commit message text, nothing else. No quotes, no prefixes.

Module: $mod

Diff:
$diff_content" 2>/dev/null || echo "")

        # Fallback if cursor-agent failed
        if [ -z "$ai_message" ]; then
          ai_message="update $mod"
          $gum style --foreground ${lib.colors.accent4} "  (AI generation failed, using default)"
        fi

        # Clean up the message (remove quotes, trim whitespace)
        ai_message=$(echo "$ai_message" | sed 's/^"//;s/"$//' | sed "s/^'//;s/'$//" | xargs)

        # Let user edit the message
        $gum style --foreground ${lib.colors.info} "Edit commit message:"
        message=$($gum input --width 72 --value "$ai_message")

        if [ -z "$message" ]; then
          $git reset HEAD -- "$mod/" >/dev/null 2>&1 || true
          $gum style --foreground ${lib.colors.info} "Skipping $mod (no message)"
          continue
        fi

        # Prepend module prefix if not present
        if [[ "$message" != "$mod:"* ]]; then
          final_message="$mod: $message"
        else
          final_message="$message"
        fi

        # Show preview
        echo ""
        $gum style --foreground ${lib.colors.accent2} "Commit: $final_message"
        echo ""

        if $gum confirm "Commit?"; then
          if $git commit -m "$final_message"; then
            $gum style --foreground ${lib.colors.success} "✓ Committed: $mod"
            committed_count=$((committed_count + 1))
          else
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
