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
	"net"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Struct tag names for OpenAPI parameter extraction.
// These match the conventions used by the binding package.
const (
	tagQuery  = "query"  // Query parameter struct tag
	tagPath   = "path"   // URL path parameter struct tag
	tagHeader = "header" // HTTP header struct tag
	tagCookie = "cookie" // Cookie struct tag
)

// RequestMetadata contains auto-discovered information about a request struct.
//
// This metadata is extracted from struct tags compatible with the binding
// package, enabling automatic OpenAPI parameter and body schema generation.
type RequestMetadata struct {
	Parameters []ParamSpec
	HasBody    bool
	BodyType   reflect.Type
}

// ParamSpec describes a single parameter extracted from struct tags.
//
// It contains all information needed to generate an OpenAPI Parameter,
// including location (query, path, header, cookie), type, validation rules,
// and documentation.
type ParamSpec struct {
	Name        string
	In          string
	Description string
	Format      string
	Required    bool
	Type        reflect.Type
	Default     any
	Example     any
	Enum        []string
	Style       string // "simple", "matrix", "label" for path params
	Explode     *bool  // nil = use default; true/false = explicit
}

// IntrospectRequest analyzes a request struct type and extracts OpenAPI metadata.
//
// This function automatically discovers:
//   - Query parameters from `query` tags
//   - Path parameters from `path` tags
//   - Header parameters from `header` tags
//   - Cookie parameters from `cookie` tags
//   - Request body presence from `json` tags
//
// Returns nil if the type is not a struct or pointer to struct.
func IntrospectRequest(t reflect.Type) *RequestMetadata {
	if t == nil {
		return nil
	}

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil
	}

	meta := &RequestMetadata{
		Parameters: make([]ParamSpec, 0),
	}

	// Extract parameters from tags (query, path, header, cookie)
	for _, tag := range []struct {
		name string
		loc  string
	}{
		{tagQuery, "query"},
		{tagPath, "path"},
		{tagHeader, "header"},
		{tagCookie, "cookie"},
	} {
		meta.Parameters = append(meta.Parameters, extractParamsFromTag(t, tag.name, tag.loc)...)
	}

	// Detect JSON body
	hasBody := false
	walkFields(t, func(f reflect.StructField) {
		if !f.IsExported() {
			return
		}
		if s := f.Tag.Get("json"); s != "" && s != "-" {
			hasBody = true
		}
	})

	meta.HasBody = hasBody
	meta.BodyType = t

	return meta
}

// extractParamsFromTag extracts parameters from struct fields with the given tag.
func extractParamsFromTag(t reflect.Type, tagName, location string) []ParamSpec {
	out := []ParamSpec{}

	walkFields(t, func(field reflect.StructField) {
		if !field.IsExported() {
			return
		}

		tagVal := field.Tag.Get(tagName)
		if tagVal == "" || tagVal == "-" {
			return
		}

		parts := strings.Split(tagVal, ",")
		name := strings.TrimSpace(parts[0])
		if name == "" {
			name = field.Name
		}

		spec := ParamSpec{
			Name:        name,
			In:          location,
			Type:        field.Type,
			Description: field.Tag.Get("doc"),
			Required:    isParamRequired(field, tagName),
			Example:     parseValue(field.Tag.Get("example"), field.Type),
			Format:      inferFormat(field),
		}

		// Parse style (for path parameters: simple, matrix, label)
		if style := field.Tag.Get("style"); style != "" {
			spec.Style = style
		}

		// Parse explode (true/false)
		if explode := field.Tag.Get("explode"); explode != "" {
			switch explode {
			case "true":
				trueVal := true
				spec.Explode = &trueVal
			case "false":
				falseVal := false
				spec.Explode = &falseVal
			}
		}

		if d := field.Tag.Get("default"); d != "" {
			spec.Default = parseValue(d, field.Type)
		}

		if e := field.Tag.Get("enum"); e != "" {
			spec.Enum = parseEnumValues(e)
		}

		// Also support validate:"oneof=a b c"
		if v := field.Tag.Get("validate"); strings.Contains(v, "oneof=") {
			oneof := strings.SplitN(v, "oneof=", 2)[1]
			spec.Enum = append(spec.Enum, strings.Fields(oneof)...)
		}

		out = append(out, spec)
	})

	return out
}

// isParamRequired determines if a parameter is required.
func isParamRequired(field reflect.StructField, tagName string) bool {
	// Path parameters are always required
	if tagName == tagPath {
		return true
	}

	// Pointer types are optional
	if field.Type.Kind() == reflect.Ptr {
		return false
	}

	// Explicit required validation
	if strings.Contains(field.Tag.Get("validate"), "required") {
		return true
	}

	return false
}

// parseEnumValues parses comma-separated enum values.
func parseEnumValues(s string) []string {
	parts := strings.Split(s, ",")
	out := []string{}
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}

	return out
}

// parseValue attempts to parse a string value into the target type.
func parseValue(s string, t reflect.Type) any {
	if s == "" {
		return nil
	}

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String:
		return s
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			return v
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v, err := strconv.ParseUint(s, 10, 64); err == nil {
			return v
		}
	case reflect.Float32, reflect.Float64:
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return v
		}
	case reflect.Bool:
		if v, err := strconv.ParseBool(s); err == nil {
			return v
		}
	}

	return s
}

// inferFormat infers OpenAPI format from field type and validation tags.
func inferFormat(field reflect.StructField) string {
	if f := field.Tag.Get("format"); f != "" {
		return f
	}

	v := field.Tag.Get("validate")

	switch {
	case strings.Contains(v, "email"):
		return "email"
	case strings.Contains(v, "url"):
		return "uri"
	case strings.Contains(v, "uuid"):
		return "uuid"
	case strings.Contains(v, "ipv4"):
		return "ipv4"
	case strings.Contains(v, "ipv6"):
		return "ipv6"
	}

	t := field.Type
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t {
	case reflect.TypeFor[time.Time]():
		return "date-time"
	case reflect.TypeFor[url.URL]():
		return "uri"
	case reflect.TypeFor[net.IP]():
		return "ip"
	}

	return ""
}
