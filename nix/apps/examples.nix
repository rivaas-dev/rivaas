# Example-related apps: test-examples, run-example
{ pkgs, lib }:

{
  # Build examples (standalone, with GOWORK=off)
  test-examples = {
    type = "app";
    program = toString (lib.mkModuleScript {
      name = "test-examples";
      title = "Building Examples (standalone)";
      findPattern = lib.findPatterns.examplesOnly;
      command = "${lib.go}/bin/go build -C ./$dir ./...";
      extraEnv = "env GOWORK=off";
      spinnerTitle = "Building";
      successMsg = "All $total examples build successfully!";
      failMsg = "$failed/$total examples failed to build";
    });
  };

  # Interactive example runner
  run-example = {
    type = "app";
    program = toString (pkgs.writeShellScript "rivaas-run-example" ''
      set -uo pipefail

      gum="${pkgs.gum}/bin/gum"
      $gum style --foreground ${lib.colors.header} --bold "Run Example"
      echo ""

      # Find all examples
      examples=$(${pkgs.findutils}/bin/find . ${lib.findPatterns.examplesOnly} | sed 's|^\./||' | sort)
      total=$(echo "$examples" | grep -c . || echo 0)

      if [ "$total" -eq 0 ]; then
        $gum style --foreground ${lib.colors.info} "No examples found"
        exit 0
      fi

      # Let user select an example
      selected=$($gum choose --header "Select an example to run:" $examples)

      if [ -z "$selected" ]; then
        $gum style --foreground ${lib.colors.info} "No example selected"
        exit 0
      fi

      $gum style --foreground ${lib.colors.success} "Running: $selected"
      echo ""

      # Build and run the example (standalone, with GOWORK=off)
      cd "$selected"
      GOWORK=off ${lib.go}/bin/go run .
    '');
  };
}
