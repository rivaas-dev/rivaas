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

package route

import (
	"regexp"
	"strings"
)

// Constraint represents a compiled constraint for route parameters.
// Constraints are compiled for validation during routing.
type Constraint struct {
	Param   string         // Parameter name
	Pattern *regexp.Regexp // Compiled regex pattern
}

// ConstraintKind represents the type of constraint applied to a route parameter.
type ConstraintKind uint8

const (
	ConstraintNone ConstraintKind = iota
	ConstraintInt
	ConstraintFloat
	ConstraintUUID
	ConstraintRegex
	ConstraintEnum
	ConstraintDate     // RFC3339 full-date
	ConstraintDateTime // RFC3339 date-time
)

// ParamConstraint represents a typed constraint for a route parameter.
// This provides semantic constraint types that map directly to OpenAPI schema types.
type ParamConstraint struct {
	Kind    ConstraintKind
	Pattern string         // for ConstraintRegex
	Enum    []string       // for ConstraintEnum
	re      *regexp.Regexp // compiled regex for ConstraintRegex (lazy)
}

// Compile compiles regex patterns in typed constraints (lazy compilation).
func (pc *ParamConstraint) Compile() {
	if pc.Kind == ConstraintRegex && pc.Pattern != "" && pc.re == nil {
		if rx, err := regexp.Compile("^" + pc.Pattern + "$"); err == nil {
			pc.re = rx
		}
	}
}

// ToRegexConstraint converts a typed constraint to a regex-based Constraint
// for use with the existing validation system. This allows typed constraints to work
// with the current router architecture while preserving semantic information for OpenAPI.
func (pc *ParamConstraint) ToRegexConstraint(paramName string) *Constraint {
	var pattern string
	switch pc.Kind {
	case ConstraintInt:
		pattern = `\d+`
	case ConstraintFloat:
		pattern = `-?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?`
	case ConstraintUUID:
		pattern = `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}`
	case ConstraintRegex:
		pattern = pc.Pattern
	case ConstraintEnum:
		// Convert enum to regex: (value1|value2|value3)
		escaped := make([]string, 0, len(pc.Enum))
		for _, v := range pc.Enum {
			// Escape special regex characters in enum values
			escaped = append(escaped, regexp.QuoteMeta(v))
		}
		pattern = "(" + strings.Join(escaped, "|") + ")"
	case ConstraintDate:
		pattern = `\d{4}-\d{2}-\d{2}`
	case ConstraintDateTime:
		pattern = `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})`
	default:
		return nil // Skip unknown constraint types
	}

	// Compile regex pattern
	regex, err := regexp.Compile("^" + pattern + "$")
	if err != nil {
		// Should not happen for our predefined patterns, but handle gracefully
		return nil
	}

	return &Constraint{
		Param:   paramName,
		Pattern: regex,
	}
}

// Info contains comprehensive information about a registered route for introspection.
// This is used for debugging, documentation generation, API documentation, and monitoring.
//
// Enhanced fields provide deep insights into route configuration:
//   - Middleware: Full middleware chain for this route
//   - Constraints: Parameter validation rules
//   - IsStatic: Whether the route is static
//   - Version: API versioning information
type Info struct {
	Method      string            // HTTP method (GET, POST, etc.)
	Path        string            // Route path pattern (/users/:id)
	HandlerName string            // Name of the handler function
	Middleware  []string          // Middleware chain names (in execution order)
	Constraints map[string]string // Parameter constraints (param -> regex pattern)
	IsStatic    bool              // True if route has no dynamic parameters
	Version     string            // API version (e.g., "v1", "v2"), empty if not versioned
	ParamCount  int               // Number of URL parameters in this route
}
