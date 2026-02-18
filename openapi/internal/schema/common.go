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

package schema

import (
	"reflect"
	"strings"
)

// walkFields recursively walks struct fields, handling embedded/anonymous fields.
//
// Embedded fields are traversed recursively, allowing access to fields from
// embedded structs. This is used during schema generation to discover all
// fields that should be included in the OpenAPI schema, including those
// inherited through embedding.
//
// Example:
//
//	type Base struct { ID int }
//	type User struct { Base; Name string }
//	// walkFields on User will visit both ID (from Base) and Name
func walkFields(t reflect.Type, fn func(reflect.StructField)) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	for i := range t.NumField() {
		f := t.Field(i)

		if f.Anonymous {
			walkFields(f.Type, fn)
			continue
		}

		fn(f)
	}
}

// schemaName generates a clean, readable name for a type.
// To avoid cross-package type name collisions, the name includes the package name
// in the format "pkgname.TypeName". The package name is the last component of
// the package path (e.g., "github.com/user/api" -> "api").
func schemaName(t reflect.Type) string {
	if t.Name() == "" {
		return ""
	}

	// Get package path and extract the last component as the package name
	pkgPath := t.PkgPath()
	if pkgPath == "" {
		// Built-in types or unnamed types - just use the type name
		return t.Name()
	}

	// Extract package name from path (last component after last '/')
	// Handle both standard paths and vendor paths
	parts := strings.Split(pkgPath, "/")
	pkgName := parts[len(parts)-1]

	// If package name is empty or same as type name, just return type name
	// Otherwise, use "pkgname.TypeName" format
	if pkgName == "" || pkgName == t.Name() {
		return t.Name()
	}

	return pkgName + "." + t.Name()
}

// parseJSONName extracts the JSON field name from a tag.
//
// Examples:
//   - `json:"name"` -> "name"
//   - `json:"name,omitempty"` -> "name"
//   - `json:"-"` -> "" (empty, uses fallback)
//   - `json:""` -> "" (empty, uses fallback)
func parseJSONName(tag, fallback string) string {
	if tag == "" {
		return fallback
	}

	p := strings.Split(tag, ",")
	if p[0] != "" {
		return p[0]
	}

	return fallback
}

// isFieldRequired determines if a field is required.
func isFieldRequired(f reflect.StructField) bool {
	if f.Type.Kind() == reflect.Pointer {
		return false
	}

	return strings.Contains(f.Tag.Get("validate"), "required")
}
