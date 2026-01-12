# App metadata for display in devshell banner
# Each entry defines the category, color, and description for an app
# This is the single source of truth used by both apps and devshell
[
  # Testing (green → mint → blue gradient)
  { name = "test";             category = "Testing"; color = "success"; description = "Run unit tests"; }
  { name = "test-race";        category = "Testing"; color = "accent6"; description = "Run tests with race detector"; }
  { name = "test-integration"; category = "Testing"; color = "accent1"; description = "Run integration tests"; }
  { name = "test-examples";    category = "Testing"; color = "accent2"; description = "Build examples (standalone)"; }

  # Code quality (yellow → peach gradient)
  { name = "fmt";              category = "Code Quality"; color = "accent4"; description = "Format code (gofumpt + gci)"; }
  { name = "fmt-check";        category = "Code Quality"; color = "accent4"; description = "Check formatting (CI)"; }
  { name = "lint";             category = "Code Quality"; color = "accent5"; description = "Run golangci-lint"; }
  { name = "lint-soft";        category = "Code Quality"; color = "accent5"; description = "Run optional linters (advisory)"; }
  { name = "lint-all";         category = "Code Quality"; color = "accent5"; description = "Run all linters (required + optional)"; }
  { name = "bench";            category = "Code Quality"; color = "accent4"; description = "Run benchmarks"; }
  { name = "tidy";             category = "Code Quality"; color = "accent5"; description = "Run go mod tidy"; }

  # Release (pink → lavender gradient)
  { name = "release-check";    category = "Release"; color = "accent3"; description = "Check for unreleased changes"; }
  { name = "release";          category = "Release"; color = "accent3"; description = "Interactive release (create tags)"; }
  { name = "run-example";      category = "Release"; color = "header";  description = "Interactive example runner"; }

  # Commit tools (sky blue → pale blue gradient)
  { name = "commit";           category = "Commit Tools"; color = "accent1"; description = "Interactive commit with AI messages"; }
  { name = "commit-check";     category = "Commit Tools"; color = "accent2"; description = "Check uncommitted changes"; }
]
