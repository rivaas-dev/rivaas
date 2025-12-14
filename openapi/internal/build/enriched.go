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

package build

import (
	"reflect"

	"rivaas.dev/openapi/internal/schema"
)

// RouteInfo contains basic route information needed for OpenAPI generation.
// This avoids importing the openapi package to prevent import cycles.
type RouteInfo struct {
	Method          string                    // HTTP method (GET, POST, etc.)
	Path            string                    // URL path with parameters (e.g. "/users/:id")
	PathConstraints map[string]PathConstraint // Typed constraints for path parameters
}

// ConstraintKind represents the type of constraint on a path parameter.
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

// ExampleData holds example data to avoid import cycles with openapi package.
type ExampleData struct {
	Name          string
	Summary       string
	Description   string
	Value         any
	ExternalValue string
}

// RouteDoc holds all OpenAPI metadata for a route.
// This is a copy of the openapi.RouteDoc structure to avoid import cycles.
type RouteDoc struct {
	Summary               string
	Description           string
	OperationID           string
	Tags                  []string
	Deprecated            bool
	Consumes              []string
	Produces              []string
	RequestType           reflect.Type
	RequestMetadata       *schema.RequestMetadata
	RequestExample        any           // Single unnamed example
	RequestNamedExamples  []ExampleData // Named examples
	ResponseTypes         map[int]reflect.Type
	ResponseExample       map[int]any           // Single unnamed example per status
	ResponseNamedExamples map[int][]ExampleData // Named examples per status
	Security              []SecurityReq
	Extensions            map[string]any // Operation-level extensions (x-*)
}

// SecurityReq represents a security requirement for an operation.
type SecurityReq struct {
	Scheme string
	Scopes []string
}

// EnrichedRoute combines route information with OpenAPI documentation.
//
// This type is used to pass route data to Builder.Build() for spec generation.
// The Doc field may be nil if the route has no OpenAPI documentation.
type EnrichedRoute struct {
	RouteInfo RouteInfo
	Doc       *RouteDoc
}
