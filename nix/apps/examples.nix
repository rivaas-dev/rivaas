# Example-related apps: test-examples, run-example
{ pkgs, lib }:

{
  # Build examples (standalone, with GOWORK=off)
  test-examples = {
    type = "app";
    meta.description = "Build all examples (standalone)";
    program = toString (lib.mkModuleScript {
      name = "test-examples";
      title = "Building Examples (standalone)";
      findPattern = lib.findPatterns.examplesOnly;
      command = "${pkgs.go}/bin/go build -C ./$dir ./...";
      extraEnv = "env GOWORK=off";
      spinnerTitle = "Building";
      successMsg = "All $total examples build successfully!";
      failMsg = "$failed/$total examples failed to build";
    });
  };

  # Interactive example runner
  run-example = {
    type = "app";
    meta.description = "Interactive example runner";
    program = toString (pkgs.writeShellScript "rivaas-run-example" ''
      set -uo pipefail

      gum="${pkgs.gum}/bin/gum"
      $gum style --foreground ${lib.colors.header} --bold --border rounded --padding "0 1" "Run Example"
      echo ""

      # Find all examples
      examples=$(${pkgs.findutils}/bin/find . ${lib.findPatterns.examplesOnly} | sed 's|^\./||' | sort)
      total=$(echo "$examples" | grep -c . || echo 0)

      if [ "$total" -eq 0 ]; then
        $gum style --foreground ${lib.colors.info} "No examples found"
        exit 0
      fi

      # Let user select an example (with fuzzy search)
      selected=$(echo "$examples" | tr ' ' '\n' | $gum filter --header "Select an example (type to filter):" --fuzzy --select-if-one)

      if [ -z "$selected" ]; then
        $gum style --foreground ${lib.colors.info} "No example selected"
        exit 0
      fi

      $gum style --foreground ${lib.colors.success} "Running: $selected"
      echo ""

      # Build and run the example (standalone, with GOWORK=off)
      cd "$selected"
      GOWORK=off ${pkgs.go}/bin/go run .
    '');
  };
}
