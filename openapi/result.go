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

package openapi

import "rivaas.dev/openapi/diag"

// Result contains the generated OpenAPI specification.
type Result struct {
	// JSON is the OpenAPI spec serialized as JSON.
	JSON []byte

	// YAML is the OpenAPI spec serialized as YAML.
	YAML []byte

	// Warnings contains informational, non-fatal issues.
	// These are advisory only and do not indicate failure.
	// The spec in JSON/YAML is valid even when warnings exist.
	//
	// Import "rivaas.dev/openapi/diag" for type-safe warning code checks.
	Warnings diag.Warnings
}
