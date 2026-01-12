# Banner generation utilities for devshell
# Provides functions to generate the welcome banner with categorized commands
{ colors }:

let
  # Category display order (preserves intended ordering)
  categoryOrder = [ "Testing" "Code Quality" "Release" "Commit Tools" ];

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

in
{
  inherit groupByCategory maxNameLen padRight mkCommandLine mkCategorySection categoryOrder;

  # Generate the complete banner script for all categories
  # Usage: mkBannerScript appsMeta
  mkBannerScript = apps:
    let
      maxLen = maxNameLen apps;
      categories = groupByCategory apps;
    in
    builtins.concatStringsSep "\n    " (builtins.map (mkCategorySection maxLen) categories);
}
