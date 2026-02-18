# Common find patterns for Go module discovery
{
  # All modules except examples
  nonExamples = "-name 'go.mod' -type f ! -path '*/examples/*' ! -path '*/example/*' -exec dirname {} \\;";

  # Only examples
  examplesOnly = "\\( -path '*/examples/*' -o -path '*/example/*' \\) -name 'go.mod' -exec dirname {} \\;";

  # All modules including examples
  allModules = "-name 'go.mod' -type f -exec dirname {} \\;";

  # Root-level and middleware modules (excludes examples/benchmarks/integration test module)
  rootModules = "-name 'go.mod' -type f ! -path '*/examples/*' ! -path '*/example/*' ! -path '*/benchmarks/*' ! -path '*/integration/*' -exec dirname {} \\;";
}
