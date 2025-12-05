# Release-related apps: release-check, release
{ pkgs, lib }:

{
  # Check module release status (dry-run)
  release-check = {
    type = "app";
    meta.description = "Check modules for unreleased changes";
    program = toString (pkgs.writeShellScript "rivaas-release-check" ''
      set -uo pipefail

      gum="${pkgs.gum}/bin/gum"
      git="${pkgs.git}/bin/git"

      # Fetch latest tags from remote (warn on failure, don't fail completely)
      # Keep this outside the pager so the spinner is interactive
      fetch_warning=""
      if ! $gum spin --spinner dot --title "Fetching latest tags from origin..." -- $git fetch --tags origin 2>&1; then
        fetch_warning="failed"
      fi

      # Get GitHub repo URL for commit links (convert SSH to HTTPS, strip .git)
      repo_url=$($git remote get-url origin 2>/dev/null | sed 's|ssh://git@github.com/|https://github.com/|' | sed 's|git@github.com:|https://github.com/|' | sed 's|\.git$||')

      # Root-level modules only
      modules=$(${pkgs.findutils}/bin/find . ${lib.findPatterns.rootModules} | sed 's|^\./||' | sort)
      total=$(echo "$modules" | grep -c . || true)

      # Handle empty module list
      if [ "$total" -eq 0 ]; then
        $gum style --foreground ${lib.colors.info} "No modules found"
        exit 0
      fi

      # Pipe all output through gum pager for scrollable view
      {
        $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "Module Release Status"
        echo ""

        # Show fetch warning if applicable
        if [ -n "$fetch_warning" ]; then
          $gum style --foreground ${lib.colors.error} "⚠ Warning: Could not fetch tags from origin (working offline or network issue)"
          $gum style --faint "  Continuing with local tags only..."
          echo ""
        fi

        # Helper: format commits with clickable OSC 8 hyperlinks (colored hash)
        format_commits() {
          while IFS= read -r line; do
            [ -z "$line" ] && continue
            hash=$(echo "$line" | cut -d' ' -f1)
            msg=$(echo "$line" | cut -d' ' -f2-)
            # Color the hash with gum, embed in OSC 8 hyperlink
            colored_hash=$($gum style --foreground ${lib.colors.accent1} "$hash")
            # OSC 8 hyperlink: \e]8;;URL\e\\TEXT\e]8;;\e\\
            printf "    \033]8;;%s/commit/%s\033\\%s\033]8;;\033\\ %s\n" "$repo_url" "$hash" "$colored_hash" "$msg"
          done
        }

        needs_release=0
        up_to_date=0

        for mod in $modules; do
          [ -z "$mod" ] && continue

          latest_tag=$($git tag -l "$mod/v*" --sort=-v:refname | head -1)

          if [ -z "$latest_tag" ]; then
            commit_count=$($git rev-list --count HEAD -- "$mod/" 2>/dev/null || echo "0")
            if [ "$commit_count" -gt 0 ]; then
              $gum style --foreground ${lib.colors.accent3} --bold "● $mod"
              $gum style --faint "  Tag: (none - new module)"
              $gum style --foreground ${lib.colors.accent4} "  $commit_count total commits"
              echo ""
              $git log --oneline HEAD -- "$mod/" 2>/dev/null | head -5 | format_commits
              [ "$commit_count" -gt 5 ] && $gum style --faint "    ... and $((commit_count - 5)) more"
              needs_release=$((needs_release + 1))
            fi
          else
            commits=$($git log --oneline "$latest_tag"..HEAD -- "$mod/" 2>/dev/null)
            commit_count=$(echo "$commits" | grep -c . 2>/dev/null || true)

            if [ "$commit_count" -gt 0 ] && [ -n "$commits" ]; then
              $gum style --foreground ${lib.colors.accent3} --bold "● $mod"
              $gum style --faint "  Tag: $latest_tag"
              $gum style --foreground ${lib.colors.accent4} "  $commit_count commits since tag:"
              echo ""
              echo "$commits" | head -10 | format_commits
              [ "$commit_count" -gt 10 ] && $gum style --faint "    ... and $((commit_count - 10)) more"
              needs_release=$((needs_release + 1))
            else
              $gum style --foreground ${lib.colors.success} "✓ $mod"
              $gum style --faint "  Tag: $latest_tag"
              up_to_date=$((up_to_date + 1))
            fi
          fi
          echo ""
        done

        # Summary
        $gum style --foreground ${lib.colors.header} --bold "Summary"
        if [ $needs_release -gt 0 ]; then
          $gum style --foreground ${lib.colors.accent3} "  ● $needs_release module(s) have unreleased changes"
        fi
        if [ $up_to_date -gt 0 ]; then
          $gum style --foreground ${lib.colors.success} "  ✓ $up_to_date module(s) up to date"
        fi
      } | $gum pager --show-line-numbers=false
    '');
  };

  # Interactive release tool
  release = {
    type = "app";
    meta.description = "Interactive release tool (create tags)";
    program = toString (pkgs.writeShellScript "rivaas-release" ''
      set -uo pipefail

      gum="${pkgs.gum}/bin/gum"
      git="${pkgs.git}/bin/git"

      $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "Interactive Release"
      echo ""

      # Fetch latest tags from remote
      if ! $gum spin --spinner dot --title "Fetching latest tags from origin..." -- $git fetch --tags origin 2>&1; then
        $gum style --foreground ${lib.colors.error} "⚠ Warning: Could not fetch tags from origin"
        $gum style --faint "  Continuing with local tags only..."
        echo ""
      fi

      # Get GitHub repo URL for hyperlinks (convert SSH to HTTPS, strip .git)
      repo_url=$($git remote get-url origin 2>/dev/null | sed 's|ssh://git@github.com/|https://github.com/|' | sed 's|git@github.com:|https://github.com/|' | sed 's|\.git$||')

      # Helper: bump semver version
      bump_version() {
        local version=$1 type=$2
        local major minor patch
        # Strip 'v' prefix if present
        version="''${version#v}"
        IFS='.' read -r major minor patch <<< "$version"
        # Default to 0.0.0 if parsing fails
        major=''${major:-0}
        minor=''${minor:-0}
        patch=''${patch:-0}
        case "$type" in
          major) echo "$((major + 1)).0.0" ;;
          minor) echo "$major.$((minor + 1)).0" ;;
          patch) echo "$major.$minor.$((patch + 1))" ;;
          *) echo "$version" ;;
        esac
      }

      # Root-level modules only
      modules=$(${pkgs.findutils}/bin/find . ${lib.findPatterns.rootModules} | sed 's|^\./||' | sort)

      if [ -z "$modules" ]; then
        $gum style --foreground ${lib.colors.info} "No modules found"
        exit 0
      fi

      # Find modules with unreleased changes
      modules_needing_release=""
      for mod in $modules; do
        [ -z "$mod" ] && continue
        latest_tag=$($git tag -l "$mod/v*" --sort=-v:refname | head -1)

        if [ -z "$latest_tag" ]; then
          commit_count=$($git rev-list --count HEAD -- "$mod/" 2>/dev/null || echo "0")
          [ "$commit_count" -gt 0 ] && modules_needing_release="$modules_needing_release $mod"
        else
          commit_count=$($git log --oneline "$latest_tag"..HEAD -- "$mod/" 2>/dev/null | grep -c . || true)
          [ "$commit_count" -gt 0 ] && modules_needing_release="$modules_needing_release $mod"
        fi
      done

      modules_needing_release=$(echo "$modules_needing_release" | xargs)

      if [ -z "$modules_needing_release" ]; then
        $gum style --foreground ${lib.colors.success} "✓ All modules are up to date!"
        exit 0
      fi

      # Display modules with unreleased changes as a table
      $gum style --foreground ${lib.colors.info} "Modules with unreleased changes:"
      echo ""

      # Build table data (CSV format)
      table_data="Module,Version,Commits"
      for mod in $modules_needing_release; do
        latest_tag=$($git tag -l "$mod/v*" --sort=-v:refname | head -1)
        commit_count=$($git log --oneline "''${latest_tag:-HEAD~100}"..HEAD -- "$mod/" 2>/dev/null | grep -c . || true)
        version="''${latest_tag#$mod/}"
        version="''${version:-new}"
        table_data="$table_data
$mod,$version,$commit_count"
      done

      # Display table with gum
      echo "$table_data" | $gum table --print --border.foreground ${lib.colors.accent1}
      echo ""

      # Multi-select: Space to toggle, Enter to confirm
      $gum style --faint "Use Space to select, Enter to confirm"
      selected=$($gum choose --no-limit --header "Select modules to release:" $modules_needing_release)

      if [ -z "$selected" ]; then
        $gum style --foreground ${lib.colors.info} "No modules selected"
        exit 0
      fi

      # Track created tags for later push
      created_tags=""

      # Process each selected module
      for mod in $selected; do
        echo ""
        $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "$mod"

        latest_tag=$($git tag -l "$mod/v*" --sort=-v:refname | head -1)
        current_version="''${latest_tag#$mod/v}"
        current_version="''${current_version:-0.0.0}"

        $gum style --faint "Current version: v$current_version"

        # Calculate next versions
        next_patch=$(bump_version "$current_version" patch)
        next_minor=$(bump_version "$current_version" minor)
        next_major=$(bump_version "$current_version" major)

        # Let user choose version
        version_choice=$($gum choose --header "Select new version:" \
          "v$next_patch (patch)" \
          "v$next_minor (minor)" \
          "v$next_major (major)" \
          "custom")

        if [ -z "$version_choice" ]; then
          $gum style --foreground ${lib.colors.info} "Skipping $mod"
          continue
        fi

        # Parse version choice
        case "$version_choice" in
          *patch*) new_version="$next_patch" ;;
          *minor*) new_version="$next_minor" ;;
          *major*) new_version="$next_major" ;;
          custom)
            new_version=$($gum input --placeholder "$next_patch" --header "Enter version (without v prefix):")
            if [ -z "$new_version" ]; then
              $gum style --foreground ${lib.colors.info} "Skipping $mod"
              continue
            fi
            ;;
        esac

        new_tag="$mod/v$new_version"

        # Check if tag already exists
        if $git tag -l "$new_tag" | grep -q .; then
          $gum style --foreground ${lib.colors.error} "✗ Tag $new_tag already exists!"
          continue
        fi

        # Auto-generate changelog from commits
        if [ -n "$latest_tag" ]; then
          commits=$($git log --oneline "$latest_tag"..HEAD -- "$mod/" 2>/dev/null | sed 's/^[a-f0-9]* /- /')
        else
          commits=$($git log --oneline HEAD -- "$mod/" 2>/dev/null | head -20 | sed 's/^[a-f0-9]* /- /')
        fi

        # Prepare default message
        default_message="$mod v$new_version

$commits"

        # Let user edit the message
        $gum style --foreground ${lib.colors.info} "Edit release notes (Ctrl+D or Esc to save):"
        message=$($gum write --width 80 --height 15 --value "$default_message")

        if [ -z "$message" ]; then
          message="$mod v$new_version"
        fi

        # Show preview and confirm
        echo ""
        $gum style --foreground ${lib.colors.accent2} "Tag: $new_tag"
        $gum style --foreground ${lib.colors.accent2} "Message:"
        echo "$message" | $gum style --faint
        echo ""

        if $gum confirm "Create tag $new_tag?"; then
          if $git tag -a "$new_tag" -m "$message"; then
            $gum style --foreground ${lib.colors.success} "✓ Created tag: $new_tag"
            created_tags="$created_tags $new_tag"
          else
            $gum style --foreground ${lib.colors.error} "✗ Failed to create tag: $new_tag"
          fi
        else
          $gum style --foreground ${lib.colors.info} "Skipped $new_tag"
        fi
      done

      created_tags=$(echo "$created_tags" | xargs)

      # Offer to push tags
      if [ -n "$created_tags" ]; then
        echo ""
        $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "Summary"
        $gum style --foreground ${lib.colors.success} "Created tags:"
        for tag in $created_tags; do
          $gum style --faint "  $tag"
        done
        echo ""

        if $gum confirm "Push tags to origin?"; then
          for tag in $created_tags; do
            if $gum spin --spinner dot --title "Pushing $tag..." -- $git push origin "$tag"; then
              $gum style --foreground ${lib.colors.success} "✓ Pushed: $tag"
            else
              $gum style --foreground ${lib.colors.error} "✗ Failed to push: $tag"
            fi
          done
        fi

        echo ""
        $gum style --foreground ${lib.colors.success} --bold "✓ Release complete!"
      else
        echo ""
        $gum style --foreground ${lib.colors.info} "No tags were created"
      fi
    '');
  };
}
