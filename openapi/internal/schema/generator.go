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

// Package schema provides schema generation from Go types using reflection.
//
// The SchemaGenerator type converts Go types to OpenAPI schemas, handling
// structs, slices, maps, primitives, and validation constraints from struct tags.
// It tracks seen types to avoid infinite recursion and creates component schema
// references for complex types.
package schema

import (
	"reflect"
	"strconv"
	"strings"
	"time"

	"rivaas.dev/openapi/internal/model"
)

// SchemaGenerator generates Schema from Go types using reflection.
//
// It handles Go's type system and converts it to the version-agnostic model format,
// including support for structs, slices, maps, primitives, and common types like
// time.Time. The generator tracks seen types to avoid infinite recursion and
// creates component schema references for complex types.
type SchemaGenerator struct {
	schemas map[string]*model.Schema
	seen    map[reflect.Type]bool
}

// NewSchemaGenerator creates a new schema generator.
//
// The generator maintains internal state to track seen types and generated
// component schemas. A new generator should be created for each spec build.
func NewSchemaGenerator() *SchemaGenerator {
	return &SchemaGenerator{
		schemas: make(map[string]*model.Schema),
		seen:    make(map[reflect.Type]bool),
	}
}

// Generate generates a Schema for the given Go type.
func (sg *SchemaGenerator) Generate(t reflect.Type) *model.Schema {
	if t == nil {
		return &model.Schema{Kind: model.KindObject}
	}

	if t == reflect.TypeFor[time.Time]() {
		return &model.Schema{
			Kind:    model.KindString,
			Format:  "date-time",
			Example: time.Now().Format(time.RFC3339),
		}
	}

	// Handle []byte as base64-encoded binary data
	if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
		return &model.Schema{
			Kind:            model.KindString,
			ContentEncoding: "base64",
		}
	}

	if t.Kind() == reflect.Pointer {
		s := sg.Generate(t.Elem())
		s.Nullable = true

		return s
	}

	if sg.seen[t] {
		if name := schemaName(t); name != "" {
			return &model.Schema{Ref: "#/components/schemas/" + name}
		}

		return &model.Schema{Kind: model.KindObject}
	}

	switch t.Kind() {
	case reflect.String:
		return &model.Schema{Kind: model.KindString}
	case reflect.Bool:
		return &model.Schema{Kind: model.KindBoolean}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return &model.Schema{Kind: model.KindInteger, Format: "int32"}
	case reflect.Int64, reflect.Uint64:
		return &model.Schema{Kind: model.KindInteger, Format: "int64"}
	case reflect.Float32:
		return &model.Schema{Kind: model.KindNumber, Format: "float"}
	case reflect.Float64:
		return &model.Schema{Kind: model.KindNumber, Format: "double"}
	case reflect.Slice, reflect.Array:
		return &model.Schema{
			Kind:  model.KindArray,
			Items: sg.Generate(t.Elem()),
		}
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return &model.Schema{Kind: model.KindObject}
		}

		return &model.Schema{
			Kind:       model.KindObject,
			Additional: model.AdditionalPropsSchema(sg.Generate(t.Elem())),
		}
	case reflect.Interface:
		return &model.Schema{Kind: model.KindObject}
	case reflect.Struct:
		return sg.structSchema(t)
	default:
		return &model.Schema{Kind: model.KindObject}
	}
}

// structSchema generates a schema for a struct type.
func (sg *SchemaGenerator) structSchema(t reflect.Type) *model.Schema {
	name := schemaName(t)
	if name != "" {
		if _, ok := sg.schemas[name]; ok {
			return &model.Schema{Ref: "#/components/schemas/" + name}
		}
	}

	sg.seen[t] = true
	defer func() {
		delete(sg.seen, t)
	}()

	s := &model.Schema{
		Kind:       model.KindObject,
		Properties: map[string]*model.Schema{},
	}

	var required []string

	walkFields(t, func(f reflect.StructField) {
		if !f.IsExported() {
			return
		}

		jsonTag := f.Tag.Get("json")
		if jsonTag == "-" {
			return
		}

		fieldName := parseJSONName(jsonTag, f.Name)

		fs := sg.Generate(f.Type)

		if doc := f.Tag.Get("doc"); doc != "" {
			fs.Description = doc
		}

		if ex := f.Tag.Get("example"); ex != "" {
			fs.Example = ex
		}

		applyValidationConstraints(fs, f)

		s.Properties[fieldName] = fs

		if isFieldRequired(f) && !strings.Contains(jsonTag, "omitempty") {
			required = append(required, fieldName)
		}
	})

	if len(required) > 0 {
		s.Required = required
	}

	if name != "" {
		sg.schemas[name] = s
		return &model.Schema{Ref: "#/components/schemas/" + name}
	}

	return s
}

// GenerateProjected builds a schema containing ONLY fields that satisfy include(f).
func (sg *SchemaGenerator) GenerateProjected(t reflect.Type, include func(reflect.StructField) bool) *model.Schema {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return sg.Generate(t)
	}

	name := schemaName(t) + "Body"
	if name != "" {
		if _, ok := sg.schemas[name]; ok {
			return &model.Schema{Ref: "#/components/schemas/" + name}
		}
	}

	s := &model.Schema{
		Kind:       model.KindObject,
		Properties: map[string]*model.Schema{},
	}

	var required []string

	walkFields(t, func(f reflect.StructField) {
		if !f.IsExported() || !include(f) {
			return
		}

		jsonTag := f.Tag.Get("json")
		if jsonTag == "-" {
			return
		}

		fieldName := parseJSONName(jsonTag, f.Name)

		fs := sg.Generate(f.Type)

		if doc := f.Tag.Get("doc"); doc != "" {
			fs.Description = doc
		}

		if ex := f.Tag.Get("example"); ex != "" {
			fs.Example = ex
		}

		applyValidationConstraints(fs, f)

		s.Properties[fieldName] = fs

		if isFieldRequired(f) && !strings.Contains(jsonTag, "omitempty") {
			required = append(required, fieldName)
		}
	})

	if len(required) > 0 {
		s.Required = required
	}

	if name != "" {
		sg.schemas[name] = s
		return &model.Schema{Ref: "#/components/schemas/" + name}
	}

	return s
}

// GetComponentSchemas returns all generated component schemas.
func (sg *SchemaGenerator) GetComponentSchemas() map[string]*model.Schema {
	return sg.schemas
}

// applyValidationConstraints applies validation constraints from struct tags to a schema.
func applyValidationConstraints(s *model.Schema, f reflect.StructField) {
	v := f.Tag.Get("validate")
	if v == "" {
		return
	}

	// Handle format constraints (email, url, uuid)
	switch {
	case strings.Contains(v, "email"):
		s.Format = "email"
	case strings.Contains(v, "url"):
		s.Format = "uri"
	case strings.Contains(v, "uuid"):
		s.Format = "uuid"
	}

	// Handle pattern-based validators
	if strings.Contains(v, "alphanum") {
		s.Pattern = "^[a-zA-Z0-9]+$"
	}

	// Parse all validation parts
	for part := range strings.SplitSeq(v, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		switch {
		case strings.HasPrefix(part, "min="):
			if x, err := strconv.ParseFloat(strings.TrimPrefix(part, "min="), 64); err == nil {
				s.Minimum = &model.Bound{Value: x, Exclusive: false}
			}
		case strings.HasPrefix(part, "max="):
			if x, err := strconv.ParseFloat(strings.TrimPrefix(part, "max="), 64); err == nil {
				s.Maximum = &model.Bound{Value: x, Exclusive: false}
			}
		case strings.HasPrefix(part, "gte="):
			if x, err := strconv.ParseFloat(strings.TrimPrefix(part, "gte="), 64); err == nil {
				s.Minimum = &model.Bound{Value: x, Exclusive: false}
			}
		case strings.HasPrefix(part, "lte="):
			if x, err := strconv.ParseFloat(strings.TrimPrefix(part, "lte="), 64); err == nil {
				s.Maximum = &model.Bound{Value: x, Exclusive: false}
			}
		case strings.HasPrefix(part, "gt="):
			if x, err := strconv.ParseFloat(strings.TrimPrefix(part, "gt="), 64); err == nil {
				s.Minimum = &model.Bound{Value: x, Exclusive: true}
			}
		case strings.HasPrefix(part, "lt="):
			if x, err := strconv.ParseFloat(strings.TrimPrefix(part, "lt="), 64); err == nil {
				s.Maximum = &model.Bound{Value: x, Exclusive: true}
			}
		case strings.HasPrefix(part, "minlen=") || strings.HasPrefix(part, "minLength="):
			if x, err := strconv.Atoi(strings.TrimPrefix(strings.TrimPrefix(part, "minlen="), "minLength=")); err == nil {
				s.MinLength = &x
			}
		case strings.HasPrefix(part, "maxlen=") || strings.HasPrefix(part, "maxLength="):
			if x, err := strconv.Atoi(strings.TrimPrefix(strings.TrimPrefix(part, "maxlen="), "maxLength=")); err == nil {
				s.MaxLength = &x
			}
		case strings.HasPrefix(part, "len="):
			if x, err := strconv.Atoi(strings.TrimPrefix(part, "len=")); err == nil {
				s.MinLength = &x
				s.MaxLength = &x
			}
		case strings.HasPrefix(part, "oneof="):
			vals := strings.Fields(strings.TrimPrefix(part, "oneof="))
			s.Enum = make([]any, 0, len(vals))
			for _, v := range vals {
				s.Enum = append(s.Enum, v)
			}
		}
	}
}
