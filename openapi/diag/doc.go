// Copyright 2025 The Rivaas Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
Package diag provides diagnostic types for OpenAPI spec generation.

This package defines warning types and codes used throughout the openapi package.
Warnings are informational, non-fatal issues that don't prevent spec generation.

# Basic Usage

Most users don't need to import this package directly - warning types are
re-exported from the main openapi package:

	import "rivaas.dev/openapi"

	result, _ := api.Generate(ctx, ops...)
	if len(result.Warnings) > 0 {
	    fmt.Printf("Generated with %d warnings\n", len(result.Warnings))
	}

# Type-Safe Warning Checks

Import this package for type-safe warning code comparisons:

	import (
	    "rivaas.dev/openapi"
	    "rivaas.dev/openapi/diag"
	)

	result, _ := api.Generate(ctx, ops...)

	// Type-safe check with IDE autocomplete
	if result.Warnings.Has(diag.WarnDownlevelWebhooks) {
	    log.Warn("webhooks were dropped for OpenAPI 3.0")
	}

	// Filter by category
	downlevelWarnings := result.Warnings.FilterCategory(diag.CategoryDownlevel)

# Warning Categories

Warnings are grouped into categories:

  - CategoryDownlevel: Features lost when converting 3.1 → 3.0
  - CategoryDeprecation: Using deprecated OpenAPI features

Validation issues are ERRORS, not warnings.
*/
package diag
