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

  # Run golangci-lint (single command with all modules for better performance)
  lint = {
    type = "app";
    meta.description = "Run golangci-lint on all modules";
    program = toString (pkgs.writeShellScript "rivaas-lint" ''
      set -uo pipefail

      gum="${pkgs.gum}/bin/gum"
      $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "Running Linter"
      echo ""

      # Collect all module paths (excluding examples) and append /...
      modules=$(${pkgs.findutils}/bin/find . ${lib.findPatterns.nonExamples} | sed 's|^\./||' | sed 's|$|/...|' | tr '\n' ' ')

      if [ -z "$modules" ]; then
        $gum style --foreground ${lib.colors.info} "No modules found"
        exit 0
      fi

      if $gum spin --spinner dot --title "Linting codebase..." --show-error -- ${pkgs.golangci-lint}/bin/golangci-lint run --config .golangci.yaml $modules; then
        $gum style --foreground ${lib.colors.success} --bold "✓ No linting issues found!"
      else
        $gum style --foreground ${lib.colors.error} --bold "✗ Linting issues detected"
        exit 1
      fi
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
        output=$($gum spin --spinner dot --title "Benchmarking..." --show-output -- ${lib.go}/bin/go test -C "./$dir" ./... -bench=. -benchmem -run=^$ -count=1)
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
      command = "${lib.go}/bin/go mod tidy -C ./$dir";
      spinnerTitle = "Tidying";
      successMsg = "All $total modules tidied!";
      failMsg = "$failed/$total modules failed to tidy";
    });
  };
}
