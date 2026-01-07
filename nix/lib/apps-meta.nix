# App metadata for display in devshell banner
# Each entry defines the color and description for an app
# This is the single source of truth used by both apps and devshell
[
  # Testing (green → mint → blue gradient)
  { name = "test";             color = "success"; description = "Run unit tests"; }
  { name = "test-race";        color = "accent6"; description = "Run tests with race detector"; }
  { name = "test-integration"; color = "accent1"; description = "Run integration tests"; }
  { name = "test-examples";    color = "accent2"; description = "Build examples (standalone)"; }

  # Code quality (yellow → peach gradient)
  { name = "fmt";              color = "accent4"; description = "Format code (gofumpt + gci)"; }
  { name = "fmt-check";        color = "accent4"; description = "Check formatting (CI)"; }
  { name = "lint";             color = "accent5"; description = "Run golangci-lint"; }
  { name = "lint-soft";        color = "accent5"; description = "Run optional linters (advisory)"; }
  { name = "lint-all";         color = "accent5"; description = "Run all linters (required + optional)"; }
  { name = "bench";            color = "accent4"; description = "Run benchmarks"; }
  { name = "tidy";             color = "accent5"; description = "Run go mod tidy"; }

  # Release (pink → lavender gradient)
  { name = "release-check";    color = "accent3"; description = "Check for unreleased changes"; }
  { name = "release";          color = "accent3"; description = "Interactive release (create tags)"; }
  { name = "run-example";      color = "header";  description = "Interactive example runner"; }

  # Commit tools (sky blue → pale blue gradient)
  { name = "commit";           color = "accent1"; description = "Interactive commit with AI messages"; }
  { name = "commit-check";     color = "accent2"; description = "Check uncommitted changes"; }
]
