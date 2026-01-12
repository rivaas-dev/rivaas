# Testing-related apps (run outside sandbox with network access)
{ pkgs, lib }:

{
  # Run all tests (excludes examples)
  test = {
    type = "app";
    meta.description = "Run unit tests for all modules";
    program = toString (lib.mkModuleScript {
      name = "test";
      title = "Running Tests";
      findPattern = lib.findPatterns.nonExamples;
      command = "${pkgs.go}/bin/go test -C ./$dir ./... -count=1";
      spinnerTitle = "Testing";
      successMsg = "All $total modules passed!";
      failMsg = "$failed/$total modules failed";
    });
  };

  # Run tests with race detector (excludes examples)
  test-race = {
    type = "app";
    meta.description = "Run tests with race detector";
    program = toString (lib.mkModuleScript {
      name = "test-race";
      title = "Running Tests with Race Detector";
      findPattern = lib.findPatterns.nonExamples;
      command = "${pkgs.go}/bin/go test -C ./$dir ./... -race -count=1";
      spinnerTitle = "Testing";
      successMsg = "All $total modules passed race detection!";
      failMsg = "$failed/$total modules have race conditions";
    });
  };

  # Run integration tests (excludes examples)
  test-integration = {
    type = "app";
    meta.description = "Run integration tests for all modules";
    program = toString (lib.mkModuleScript {
      name = "test-integration";
      title = "Running Integration Tests";
      findPattern = lib.findPatterns.nonExamples;
      command = "${pkgs.go}/bin/go test -C ./$dir ./... -tags=integration -count=1";
      spinnerTitle = "Testing";
      successMsg = "All $total modules passed integration tests!";
      failMsg = "$failed/$total modules failed integration tests";
    });
  };

  # Run tests with coverage and generate report
  test-coverage = {
    type = "app";
    meta.description = "Run tests and generate coverage report for Codecov";
    program = toString (pkgs.writeShellScript "test-coverage" ''
      set -euo pipefail

      gum="${pkgs.gum}/bin/gum"

      $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "Generating Coverage Report"
      echo ""

      # Get absolute path of current directory
      ROOT_DIR=$(pwd)

      # Find all modules (excluding examples and benchmarks)
      modules=$(find . -name 'go.mod' \
        -not -path '*/examples/*' \
        -not -path '*/benchmarks/*' \
        -exec dirname {} \; | sort)

      # Count modules for progress
      total=$(echo "$modules" | wc -w)
      current=0

      # Create temp directory for coverage files
      mkdir -p "$ROOT_DIR/.coverage"
      rm -f "$ROOT_DIR/.coverage"/*.out

      # Run tests with coverage for each module
      for dir in $modules; do
        current=$((current + 1))
        module_name=$(echo "$dir" | sed 's|^\./||' | tr '/' '-')
        [ "$module_name" = "." ] && module_name="root"

        $gum style --foreground ${lib.colors.info} "[$current/$total] $dir"

        # Run tests with coverage using spinner
        $gum spin --spinner dot --title "Testing..." --show-error -- \
          sh -c "cd '$dir' && ${pkgs.go}/bin/go test ./... \
            -coverprofile='$ROOT_DIR/.coverage/$module_name.out' \
            -covermode=atomic \
            -count=1 2>/dev/null" || true
      done

      echo ""
      $gum style --foreground ${lib.colors.header} --bold "Merging Coverage Files"

      # Merge all coverage files into one
      echo "mode: atomic" > "$ROOT_DIR/coverage.out"
      for f in "$ROOT_DIR/.coverage"/*.out; do
        if [ -f "$f" ] && [ -s "$f" ]; then
          # Skip the mode line and append
          tail -n +2 "$f" >> "$ROOT_DIR/coverage.out" 2>/dev/null || true
        fi
      done

      echo ""
      $gum style --foreground ${lib.colors.header} --bold "Per-Package Coverage"
      echo ""
      ${pkgs.go}/bin/go tool cover -func="$ROOT_DIR/coverage.out" 2>/dev/null | grep -E "^[a-z]" | head -50 || true

      echo ""
      $gum style --foreground ${lib.colors.header} --bold "Total Coverage"
      echo ""
      total_coverage=$(${pkgs.go}/bin/go tool cover -func="$ROOT_DIR/coverage.out" 2>/dev/null | grep "total:" | awk '{print $3}')
      if [ -n "$total_coverage" ]; then
        $gum style --foreground ${lib.colors.success} --bold "  $total_coverage"
      else
        $gum style --foreground ${lib.colors.warning} "  No coverage data"
      fi

      # Cleanup temp files
      rm -rf "$ROOT_DIR/.coverage"

      echo ""
      $gum style --foreground ${lib.colors.success} --bold "✓ Coverage report saved to coverage.out"
    '');
  };
}
