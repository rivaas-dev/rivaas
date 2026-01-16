# Helper function to create coverage-generating test scripts with consistent UI
{ pkgs, colors, findPatterns }:

{
  name,
  title,
  testFlags ? "",       # e.g., "-race" or "-tags=integration"
  coverageDir,          # e.g., ".coverage" or ".coverage-integration"
  outputFile,           # e.g., "coverage.out" or "coverage-integration.out"
  spinnerTitle ? "Testing",
  successMsg,
  failMsg,
}:

pkgs.writeShellScript "rivaas-${name}" ''
  set -euo pipefail

  gum="${pkgs.gum}/bin/gum"
  go="${pkgs.go}/bin/go"
  find="${pkgs.findutils}/bin/find"

  # Fix for Go toolchain auto-switching bug causing "no such tool covdata" errors
  # See: https://github.com/golang/go/issues/75031#issuecomment-3195256688
  export GOTOOLCHAIN=local

  $gum style --foreground ${colors.header} --bold --border rounded --padding "0 1" "${title}"
  echo ""

  ROOT_DIR=$(pwd)

  # Find root-level modules only (excludes examples/benchmarks modules)
  modules=$($find . ${findPatterns.rootModules} | sort)
  total=$(echo "$modules" | grep -c . || echo 0)

  if [ "$total" -eq 0 ]; then
    $gum style --foreground ${colors.info} "No modules found"
    exit 0
  fi

  current=0
  failed=0

  mkdir -p "$ROOT_DIR/${coverageDir}"
  rm -f "$ROOT_DIR/${coverageDir}"/*.out

  for dir in $modules; do
    current=$((current + 1))
    module_name=$(echo "$dir" | sed 's|^\./||' | tr '/' '-')
    [ "$module_name" = "." ] && module_name="root"

    $gum style --foreground ${colors.info} "[$current/$total] $dir"

    # Filter packages to exclude examples and benchmarks sub-packages
    if ! $gum spin --spinner dot --title "${spinnerTitle}..." --show-error -- \
      sh -c "cd '$dir' && \
        packages=\$($go list ./... | grep -v '/examples/' | grep -v '/benchmarks/') && \
        if [ -n \"\$packages\" ]; then \
          $go test \$packages ${testFlags} \
            -coverprofile='$ROOT_DIR/${coverageDir}/$module_name.out' \
            -covermode=atomic -count=1 2>&1; \
        fi"; then
      failed=$((failed + 1))
    fi
  done

  echo ""
  $gum style --foreground ${colors.header} --bold "Merging Coverage Files"

  echo "mode: atomic" > "$ROOT_DIR/${outputFile}"
  for f in "$ROOT_DIR/${coverageDir}"/*.out; do
    if [ -f "$f" ] && [ -s "$f" ]; then
      tail -n +2 "$f" >> "$ROOT_DIR/${outputFile}" 2>/dev/null || true
    fi
  done

  echo ""
  $gum style --foreground ${colors.header} --bold "Total Coverage"
  total_cov=$($go tool cover -func="$ROOT_DIR/${outputFile}" 2>/dev/null | grep "total:" | awk '{print $3}')
  if [ -n "$total_cov" ]; then
    $gum style --foreground ${colors.success} --bold "  $total_cov"
  else
    $gum style --foreground ${colors.warning} "  No coverage data"
  fi

  rm -rf "$ROOT_DIR/${coverageDir}"

  echo ""
  if [ "$failed" -eq 0 ]; then
    $gum style --foreground ${colors.success} --bold "✓ ${successMsg}"
  else
    $gum style --foreground ${colors.error} --bold "✗ ${failMsg}"
  fi
  $gum style --foreground ${colors.info} "  Coverage report: ${outputFile}"

  [ "$failed" -ne 0 ] && exit 1
  exit 0
''
