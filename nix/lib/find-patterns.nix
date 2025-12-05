# Common find patterns for Go module discovery
{
  # All modules except examples
  nonExamples = "-name 'go.mod' -type f ! -path '*/examples/*' -exec dirname {} \\;";

  # Only examples
  examplesOnly = "-path '*/examples/*' -name 'go.mod' -exec dirname {} \\;";

  # All modules including examples
  allModules = "-name 'go.mod' -type f -exec dirname {} \\;";

  # Root-level modules only (depth 1, excludes examples/benchmarks/nested)
  rootModules = "-maxdepth 2 -name 'go.mod' -type f ! -path '*/examples/*' ! -path '*/benchmarks/*' -exec dirname {} \\;";
}
