# Code quality apps: lint, bench, tidy
{ pkgs, lib }:

{
  # Run golangci-lint (single command with all modules for better performance)
  lint = {
    type = "app";
    program = toString (pkgs.writeShellScript "rivaas-lint" ''
      set -uo pipefail

      gum="${pkgs.gum}/bin/gum"
      $gum style --foreground ${lib.colors.header} --bold "Running Linter"
      echo ""

      # Collect all module paths (excluding examples) and append /...
      modules=$(${pkgs.findutils}/bin/find . ${lib.findPatterns.nonExamples} | sed 's|^\./||' | sed 's|$|/...|' | tr '\n' ' ')

      if [ -z "$modules" ]; then
        $gum style --foreground ${lib.colors.info} "No modules found"
        exit 0
      fi

      if $gum spin --spinner dot --title "Linting codebase..." -- ${pkgs.golangci-lint}/bin/golangci-lint run --config .golangci.yaml $modules; then
        $gum style --foreground ${lib.colors.success} --bold "✓ No linting issues found!"
      else
        $gum style --foreground ${lib.colors.error} --bold "✗ Linting issues detected"
        echo ""
        ${pkgs.golangci-lint}/bin/golangci-lint run --config .golangci.yaml $modules
        exit 1
      fi
    '');
  };

  # Run benchmarks (excludes examples) - custom output handling
  bench = {
    type = "app";
    program = toString (pkgs.writeShellScript "rivaas-bench" ''
      set -uo pipefail

      gum="${pkgs.gum}/bin/gum"
      $gum style --foreground ${lib.colors.header} --bold "Running Benchmarks"
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
    program = toString (lib.mkModuleScript {
      name = "tidy";
      title = "Tidying Go Modules";
      findPattern = lib.findPatterns.allModules;
      command = "sh -c \"cd '$dir' && ${lib.go}/bin/go mod tidy\"";
      spinnerTitle = "Tidying";
      successMsg = "All $total modules tidied!";
      failMsg = "$failed/$total modules failed to tidy";
    });
  };
}
