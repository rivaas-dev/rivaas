# Code quality apps: lint, fmt, bench, tidy
{ pkgs, lib }:

{
  # Format code with golangci-lint (gofumpt + gci)
  fmt = {
    type = "app";
    meta.description = "Format all Go code with gofumpt and gci";
    program = toString (pkgs.writeShellScript "rivaas-fmt" ''
      set -uo pipefail

      gum="${pkgs.gum}/bin/gum"
      $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "Formatting Code"
      echo ""

      # Collect all module paths (excluding examples) and append /...
      modules=$(${pkgs.findutils}/bin/find . ${lib.findPatterns.nonExamples} | sed 's|^\./||' | sed 's|$|/...|' | tr '\n' ' ')

      if [ -z "$modules" ]; then
        $gum style --foreground ${lib.colors.info} "No modules found"
        exit 0
      fi

      if $gum spin --spinner dot --title "Formatting codebase..." --show-error -- ${pkgs.golangci-lint}/bin/golangci-lint fmt --config .golangci.yaml $modules; then
        $gum style --foreground ${lib.colors.success} --bold "✓ Code formatted!"
      else
        $gum style --foreground ${lib.colors.error} --bold "✗ Formatting failed"
        exit 1
      fi
    '');
  };

  # Check formatting without modifying (for CI)
  fmt-check = {
    type = "app";
    meta.description = "Check if code is formatted (no changes)";
    program = toString (pkgs.writeShellScript "rivaas-fmt-check" ''
      set -uo pipefail

      gum="${pkgs.gum}/bin/gum"
      $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "Checking Formatting"
      echo ""

      modules=$(${pkgs.findutils}/bin/find . ${lib.findPatterns.nonExamples} | sed 's|^\./||' | sed 's|$|/...|' | tr '\n' ' ')

      if [ -z "$modules" ]; then
        $gum style --foreground ${lib.colors.info} "No modules found"
        exit 0
      fi

      if $gum spin --spinner dot --title "Checking format..." --show-error -- ${pkgs.golangci-lint}/bin/golangci-lint fmt --diff --config .golangci.yaml $modules; then
        $gum style --foreground ${lib.colors.success} --bold "✓ Code is properly formatted!"
      else
        $gum style --foreground ${lib.colors.error} --bold "✗ Code needs formatting - run: nix run .#fmt"
        exit 1
      fi
    '');
  };

  # Run required linters only (hard checks - CI gate)
  lint = {
    type = "app";
    meta.description = "Run required linters (critical issues only)";
    program = toString (pkgs.writeShellScript "rivaas-lint" ''
      set -uo pipefail

      gum="${pkgs.gum}/bin/gum"
      $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "Running Linter (Required Checks)"
      echo ""

      # Collect all module paths (excluding examples) and append /...
      modules=$(${pkgs.findutils}/bin/find . ${lib.findPatterns.nonExamples} | sed 's|^\./||' | sed 's|$|/...|' | tr '\n' ' ')

      if [ -z "$modules" ]; then
        $gum style --foreground ${lib.colors.info} "No modules found"
        exit 0
      fi

      # Uses .golangci.yaml (hard/required checks)
      if $gum spin --spinner dot --title "Checking critical issues..." --show-error -- ${pkgs.golangci-lint}/bin/golangci-lint run --config .golangci.yaml $modules; then
        $gum style --foreground ${lib.colors.success} --bold "✓ No critical issues found!"
      else
        $gum style --foreground ${lib.colors.error} --bold "✗ Critical issues detected"
        exit 1
      fi
    '');
  };

  # Run optional linters (soft checks - advisory only)
  lint-soft = {
    type = "app";
    meta.description = "Run optional linters (warnings & suggestions)";
    program = toString (pkgs.writeShellScript "rivaas-lint-soft" ''
      set -uo pipefail

      gum="${pkgs.gum}/bin/gum"
      $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "Running Linter (Advisory Checks)"
      echo ""

      modules=$(${pkgs.findutils}/bin/find . ${lib.findPatterns.nonExamples} | sed 's|^\./||' | sed 's|$|/...|' | tr '\n' ' ')

      if [ -z "$modules" ]; then
        $gum style --foreground ${lib.colors.info} "No modules found"
        exit 0
      fi

      # Uses .golangci-soft.yaml (warnings + info)
      if $gum spin --spinner dot --title "Checking code quality..." --show-error -- ${pkgs.golangci-lint}/bin/golangci-lint run --config .golangci-soft.yaml $modules; then
        $gum style --foreground ${lib.colors.success} --bold "✓ No issues found!"
      else
        $gum style --foreground ${lib.colors.info} --bold "ℹ Code quality suggestions available (non-blocking)"
        # Don't fail on warnings/info
        exit 0
      fi
    '');
  };

  # Run all linters (hard + soft - comprehensive review)
  lint-all = {
    type = "app";
    meta.description = "Run all linters (required + optional)";
    program = toString (pkgs.writeShellScript "rivaas-lint-all" ''
      set -uo pipefail

      gum="${pkgs.gum}/bin/gum"
      
      # Step 1: Run required checks first (must pass)
      $gum style --foreground ${lib.colors.header} --bold "Step 1/2: Required Checks"
      
      modules=$(${pkgs.findutils}/bin/find . ${lib.findPatterns.nonExamples} | sed 's|^\./||' | sed 's|$|/...|' | tr '\n' ' ')
      
      if [ -z "$modules" ]; then
        $gum style --foreground ${lib.colors.info} "No modules found"
        exit 0
      fi
      
      if ! $gum spin --spinner dot --title "Checking critical issues..." --show-error -- ${pkgs.golangci-lint}/bin/golangci-lint run --config .golangci.yaml $modules; then
        $gum style --foreground ${lib.colors.error} --bold "✗ Critical issues detected - fix these first!"
        exit 1
      fi
      
      $gum style --foreground ${lib.colors.success} "✓ No critical issues"
      echo ""
      
      # Step 2: Run advisory checks (non-blocking)
      $gum style --foreground ${lib.colors.header} --bold "Step 2/2: Advisory Checks"
      
      if $gum spin --spinner dot --title "Checking code quality..." --show-error -- ${pkgs.golangci-lint}/bin/golangci-lint run --config .golangci-soft.yaml $modules; then
        $gum style --foreground ${lib.colors.success} "✓ No quality issues"
      else
        $gum style --foreground ${lib.colors.info} "ℹ Code quality suggestions available"
      fi
      
      echo ""
      $gum style --foreground ${lib.colors.success} --bold "✓ Linting complete!"
    '');
  };

  # Run benchmarks (excludes examples) - custom output handling
  bench = {
    type = "app";
    meta.description = "Run benchmarks for all modules";
    program = toString (pkgs.writeShellScript "rivaas-bench" ''
      set -uo pipefail

      gum="${pkgs.gum}/bin/gum"
      $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "Running Benchmarks"
      echo ""

      modules=$(${pkgs.findutils}/bin/find . ${lib.findPatterns.nonExamples} | sort)
      total=$(echo "$modules" | grep -c . || echo 0)
      current=0

      # Handle empty module list
      if [ "$total" -eq 0 ]; then
        $gum style --foreground ${lib.colors.info} "No modules found"
        exit 0
      fi

      for dir in $modules; do
        dir=''${dir#./}
        [ -z "$dir" ] && continue
        current=$((current + 1))

        $gum style --foreground ${lib.colors.info} "[$current/$total] $dir"
        output=$($gum spin --spinner dot --title "Benchmarking..." --show-output -- ${pkgs.go}/bin/go test -C "./$dir" ./... -bench=. -benchmem -run=^$ -count=1)
        # Only show if there are actual benchmark results
        if echo "$output" | grep -q "Benchmark"; then
          echo "$output" | grep -E "(Benchmark|ns/op|B/op)"
        else
          $gum style --faint "  (no benchmarks)"
        fi
      done

      echo ""
      $gum style --foreground ${lib.colors.success} --bold "✓ Benchmarks complete!"
    '');
  };

  # Run go mod tidy for all modules
  tidy = {
    type = "app";
    meta.description = "Run go mod tidy for all modules";
    program = toString (lib.mkModuleScript {
      name = "tidy";
      title = "Tidying Go Modules";
      findPattern = lib.findPatterns.allModules;
      command = "${pkgs.go}/bin/go mod tidy -C ./$dir";
      spinnerTitle = "Tidying";
      successMsg = "All $total modules tidied!";
      failMsg = "$failed/$total modules failed to tidy";
    });
  };
}
