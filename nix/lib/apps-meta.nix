# App metadata for display in devshell banner
# Each entry defines the color and description for an app
# This is the single source of truth used by both apps and devshell
[
  # Testing (greens/blues)
  { name = "test";             color = "success"; description = "Run unit tests"; }
  { name = "test-race";        color = "accent1"; description = "Run tests with race detector"; }
  { name = "test-integration"; color = "accent2"; description = "Run integration tests"; }
  { name = "test-examples";    color = "accent6"; description = "Build examples (standalone)"; }

  # Code quality (yellows/peach)
  { name = "fmt";              color = "accent4"; description = "Format code (gofumpt + gci)"; }
  { name = "fmt-check";        color = "accent5"; description = "Check formatting (CI)"; }
  { name = "lint";             color = "accent5"; description = "Run golangci-lint"; }
  { name = "bench";            color = "accent4"; description = "Run benchmarks"; }
  { name = "tidy";             color = "header";  description = "Run go mod tidy"; }

  # Release & examples (pinks)
  { name = "release-check";    color = "accent3"; description = "Check for unreleased changes"; }
  { name = "release";          color = "accent3"; description = "Interactive release (create tags)"; }
  { name = "run-example";      color = "accent1"; description = "Interactive example runner"; }
]
