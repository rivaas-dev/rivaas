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

// RouteInfo contains basic route information needed for OpenAPI generation.
// This is framework-agnostic and replaces router.RouteInfo.
type RouteInfo struct {
	Method          string                    // HTTP method (GET, POST, etc.)
	Path            string                    // URL path with parameters (e.g. "/users/:id")
	PathConstraints map[string]PathConstraint // Typed constraints for path parameters
}

// ConstraintKind represents the type of constraint on a path parameter.
// These map directly to router.ConstraintKind values.
type ConstraintKind uint8

const (
	ConstraintNone     ConstraintKind = iota
	ConstraintInt                     // OpenAPI: type integer, format int64
	ConstraintFloat                   // OpenAPI: type number, format double
	ConstraintUUID                    // OpenAPI: type string, format uuid
	ConstraintRegex                   // OpenAPI: type string, pattern
	ConstraintEnum                    // OpenAPI: type string, enum
	ConstraintDate                    // OpenAPI: type string, format date
	ConstraintDateTime                // OpenAPI: type string, format date-time
)

// PathConstraint describes a typed constraint for a path parameter.
// These constraints map directly to OpenAPI schema types.
type PathConstraint struct {
	Kind    ConstraintKind
	Pattern string   // for ConstraintRegex
	Enum    []string // for ConstraintEnum
}
