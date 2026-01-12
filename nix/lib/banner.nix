# Banner generation utilities for devshell
# Provides functions to generate the welcome banner with categorized commands
{ colors, appsMeta }:

let
  # Read ASCII banner art from external file
  bannerArt = builtins.readFile ./banner.txt;

  # Derive category order from appsMeta (preserves definition order)
  categoryOrder = builtins.foldl' (acc: app:
    if builtins.elem app.category acc
    then acc
    else acc ++ [ app.category ]
  ) [] appsMeta;

  # Group apps by their category field
  groupByCategory = apps:
    builtins.map (categoryName: {
      name = categoryName;
      apps = builtins.filter (a: a.category == categoryName) apps;
    }) categoryOrder;

  # Calculate max name length for padding (across all apps)
  maxNameLen = apps:
    builtins.foldl' (acc: app:
      let len = builtins.stringLength app.name;
      in if len > acc then len else acc
    ) 0 apps;

  # Pad a string to a given length
  padRight = str: len:
    let
      strLen = builtins.stringLength str;
      padLen = if strLen >= len then 0 else len - strLen;
      padding = builtins.concatStringsSep "" (builtins.genList (_: " ") padLen);
    in str + padding;

  # Generate a single command line
  mkCommandLine = maxLen: app:
    let
      paddedName = padRight app.name maxLen;
      colorCode = colors.${app.color};
    in
    ''printf "    %s  %s\n" "$(gum style --foreground ${colorCode} 'nix run .#${paddedName}')" "$(gum style --faint '${app.description}')"'';

  # Generate category section
  mkCategorySection = maxLen: category:
    let
      commands = builtins.map (mkCommandLine maxLen) category.apps;
      commandLines = builtins.concatStringsSep "\n    " commands;
    in
    ''
    echo ""
    gum style --foreground ${colors.header} --bold "${category.name}:"
    ${commandLines}
    '';

  # Generate the command categories script
  mkCommandCategories = apps:
    let
      maxLen = maxNameLen apps;
      categories = groupByCategory apps;
    in
    builtins.concatStringsSep "\n    " (builtins.map (mkCategorySection maxLen) categories);

  # Generate the shell hook banner script with given art and apps
  mkShellHookBannerWith = { art, apps }:
    let
      commandCategories = mkCommandCategories apps;
    in
    ''
    # Display banner only in interactive non-CI environments
    if [ -z "''${CI:-}" ]; then
      # Pretty welcome banner with gum (pastel colors)
      echo ""
      gum style --foreground ${colors.header} "${art}"

      echo ""
      gum style --foreground ${colors.info} "Go version: $(go version | cut -d' ' -f3 | tr -d 'go')"
      gum style --foreground ${colors.info} "Golangci-lint version: $(golangci-lint version --short 2>/dev/null || echo 'not found')"

      gum style --foreground ${colors.header} --bold --border rounded --padding "0 1" "Quick Commands"
      ${commandCategories}
      echo ""
    fi
    '';

in
{
  inherit groupByCategory maxNameLen padRight mkCommandLine mkCategorySection categoryOrder bannerArt;

  # Generate the command categories script
  mkBannerScript = mkCommandCategories;

  # Generate shell hook with custom banner art and apps
  mkShellHookBanner = mkShellHookBannerWith;

  # Ready-to-use shell hook with default banner art and appsMeta
  shellHook = mkShellHookBannerWith { art = bannerArt; apps = appsMeta; };
}
