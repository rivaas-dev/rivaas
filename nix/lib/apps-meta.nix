# App metadata for display in devshell banner
# Each entry defines the color and description for an app
# This is the single source of truth used by both apps and devshell
[
  { name = "test";             color = "success"; description = "Run unit tests"; }
  { name = "test-race";        color = "accent1"; description = "Run tests with race detector"; }
  { name = "test-integration"; color = "accent2"; description = "Run integration tests"; }
  { name = "test-examples";    color = "accent3"; description = "Build examples (standalone)"; }
  { name = "lint";             color = "accent4"; description = "Run golangci-lint"; }
  { name = "bench";            color = "accent5"; description = "Run benchmarks"; }
  { name = "tidy";             color = "accent6"; description = "Run go mod tidy for all modules"; }
  { name = "release-check";    color = "accent2"; description = "Check modules for unreleased changes"; }
  { name = "release";          color = "accent3"; description = "Interactive release (create tags)"; }
  { name = "run-example";      color = "accent4"; description = "Interactive example runner"; }
]
