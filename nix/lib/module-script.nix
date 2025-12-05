# Helper function to create module-iterating scripts with consistent UI
{ pkgs, colors }:

{
  name,
  title,
  findPattern,
  command,        # Shell command - use $dir for current module directory
  successMsg,
  failMsg,
  spinnerTitle ? "Processing",
  extraEnv ? ""
}:

pkgs.writeShellScript "rivaas-${name}" ''
  set -uo pipefail

  gum="${pkgs.gum}/bin/gum"
  $gum style --foreground ${colors.header} --bold "${title}"
  echo ""

  modules=$(${pkgs.findutils}/bin/find . ${findPattern} | sort)
  total=$(echo "$modules" | grep -c . || echo 0)
  current=0
  failed=0

  # Handle empty module list
  if [ "$total" -eq 0 ]; then
    $gum style --foreground ${colors.info} "No modules found"
    exit 0
  fi

  for dir in $modules; do
    dir=''${dir#./}
    [ -z "$dir" ] && continue
    current=$((current + 1))

    if $gum spin --spinner dot --title "[$current/$total] ${spinnerTitle} $dir..." --show-error -- ${extraEnv} ${command}; then
      $gum style --foreground ${colors.success} "✓ $dir"
    else
      $gum style --foreground ${colors.error} "✗ $dir"
      failed=$((failed + 1))
    fi
  done

  echo ""
  if [ $failed -eq 0 ]; then
    $gum style --foreground ${colors.success} --bold "✓ ${successMsg}"
  else
    $gum style --foreground ${colors.error} --bold "✗ ${failMsg}"
    exit 1
  fi
''
