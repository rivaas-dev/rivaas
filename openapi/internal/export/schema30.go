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

package export

import (
	"rivaas.dev/openapi/diag"
	"rivaas.dev/openapi/internal/model"
)

// SchemaV30 represents an OpenAPI 3.0.4 schema.
type SchemaV30 struct {
	Ref              string                `json:"$ref,omitempty"`
	Title            string                `json:"title,omitempty"`
	Type             string                `json:"type,omitempty"`
	Format           string                `json:"format,omitempty"`
	Description      string                `json:"description,omitempty"`
	Example          any                   `json:"example,omitempty"`
	Deprecated       bool                  `json:"deprecated,omitempty"`
	ReadOnly         bool                  `json:"readOnly,omitempty"`
	WriteOnly        bool                  `json:"writeOnly,omitempty"`
	Nullable         bool                  `json:"nullable,omitempty"`
	Discriminator    *DiscriminatorV30     `json:"discriminator,omitempty"`
	XML              *XMLV30               `json:"xml,omitempty"`
	Enum             []any                 `json:"enum,omitempty"`
	MultipleOf       *float64              `json:"multipleOf,omitempty"`
	Maximum          *float64              `json:"maximum,omitempty"`
	ExclusiveMaximum *bool                 `json:"exclusiveMaximum,omitempty"`
	Minimum          *float64              `json:"minimum,omitempty"`
	ExclusiveMinimum *bool                 `json:"exclusiveMinimum,omitempty"`
	Pattern          string                `json:"pattern,omitempty"`
	MaxLength        *int                  `json:"maxLength,omitempty"`
	MinLength        *int                  `json:"minLength,omitempty"`
	Items            *SchemaV30            `json:"items,omitempty"`
	MaxItems         *int                  `json:"maxItems,omitempty"`
	MinItems         *int                  `json:"minItems,omitempty"`
	UniqueItems      bool                  `json:"uniqueItems,omitempty"`
	Properties       map[string]*SchemaV30 `json:"properties,omitempty"`
	Required         []string              `json:"required,omitempty"`
	AdditionalProps  any                   `json:"additionalProperties,omitempty"` // bool or *SchemaV30
	AllOf            []*SchemaV30          `json:"allOf,omitempty"`
	AnyOf            []*SchemaV30          `json:"anyOf,omitempty"`
	OneOf            []*SchemaV30          `json:"oneOf,omitempty"`
	Not              *SchemaV30            `json:"not,omitempty"`
	Default          any                   `json:"default,omitempty"`
	Extensions       map[string]any        `json:"-"`
}

// DiscriminatorV30 is used for polymorphism in oneOf/allOf compositions.
type DiscriminatorV30 struct {
	PropertyName string            `json:"propertyName"`
	Mapping      map[string]string `json:"mapping,omitempty"`
}

// XMLV30 provides XML serialization hints.
type XMLV30 struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	Attribute bool   `json:"attribute,omitempty"`
	Wrapped   bool   `json:"wrapped,omitempty"`
}

// schema30 projects a Schema to OpenAPI 3.0.4 format.
//
// Key transformations:
//   - Nullable: uses nullable: true property
//   - Exclusive bounds: uses boolean exclusiveMinimum/Maximum flags
//   - Const: converts to enum: [const] with warning
//   - Unevaluated properties: dropped with warning
//   - Multiple examples: uses first example only
func schema30(s *model.Schema, p *proj30, path string) *SchemaV30 {
	if s == nil {
		return nil
	}

	if s.Ref != "" {
		return &SchemaV30{Ref: s.Ref}
	}

	out := &SchemaV30{
		Title:       s.Title,
		Description: s.Description,
		Format:      s.Format,
		Deprecated:  s.Deprecated,
		ReadOnly:    s.ReadOnly,
		WriteOnly:   s.WriteOnly,
		Example:     s.Example,
		Enum:        append([]any(nil), s.Enum...),
		Default:     s.Default,
	}

	// Type + nullable (3.0 uses nullable property)
	out.Type = kindToString(s.Kind)
	out.Nullable = s.Nullable

	// Binary data: ContentEncoding → format: byte in 3.0
	if s.ContentEncoding == "base64" || s.ContentEncoding == "base64url" {
		out.Format = "byte"
	}

	// Numeric bounds (boolean exclusivity in 3.0)
	if s.MultipleOf != nil {
		out.MultipleOf = s.MultipleOf
	}
	if s.Maximum != nil {
		out.Maximum = &s.Maximum.Value
		if s.Maximum.Exclusive {
			b := true
			out.ExclusiveMaximum = &b
		}
	}
	if s.Minimum != nil {
		out.Minimum = &s.Minimum.Value
		if s.Minimum.Exclusive {
			b := true
			out.ExclusiveMinimum = &b
		}
	}

	// String constraints
	if s.Pattern != "" {
		out.Pattern = s.Pattern
	}
	out.MinLength, out.MaxLength = s.MinLength, s.MaxLength

	// Array constraints
	if s.Items != nil {
		out.Items = schema30(s.Items, p, path+"/items")
	}
	out.MinItems, out.MaxItems = s.MinItems, s.MaxItems
	if s.UniqueItems {
		out.UniqueItems = true
	}

	// Object constraints
	if len(s.Properties) > 0 {
		out.Properties = make(map[string]*SchemaV30, len(s.Properties))
		for k, v := range s.Properties {
			out.Properties[k] = schema30(v, p, path+"/properties/"+k)
		}
	}
	if len(s.Required) > 0 {
		out.Required = append([]string(nil), s.Required...)
	}

	// Additional properties
	if s.Additional != nil {
		switch {
		case s.Additional.Schema != nil:
			out.AdditionalProps = schema30(s.Additional.Schema, p, path+"/additionalProperties")
		case s.Additional.Allow != nil:
			out.AdditionalProps = *s.Additional.Allow
		}
	}

	// Composition
	for _, it := range s.AllOf {
		out.AllOf = append(out.AllOf, schema30(it, p, path+"/allOf"))
	}
	for _, it := range s.AnyOf {
		out.AnyOf = append(out.AnyOf, schema30(it, p, path+"/anyOf"))
	}
	for _, it := range s.OneOf {
		out.OneOf = append(out.OneOf, schema30(it, p, path+"/oneOf"))
	}
	if s.Not != nil {
		out.Not = schema30(s.Not, p, path+"/not")
	}

	// Const: 3.0 has no const → convert to enum if needed
	if s.Const != nil {
		if len(out.Enum) == 0 {
			out.Enum = []any{s.Const}
			p.warn(diag.WarnDownlevelConstToEnum, path,
				"const keyword not supported in 3.0; converted to enum")
		} else {
			p.warn(diag.WarnDownlevelConstToEnumConflict, path,
				"const with enum under 3.0: kept enum, ignored const")
		}
	}

	// Unevaluated properties: 3.1-only → warn & drop
	if s.Unevaluated != nil {
		p.warn(diag.WarnDownlevelUnevaluatedProperties, path,
			"unevaluatedProperties not supported in OpenAPI 3.0; dropped")
	}

	// Pattern properties: not officially in OpenAPI 3.0 (but in JSON Schema)
	// We'll skip them for now, but could add a warning if needed

	// Examples: 3.0 uses singular example
	if len(s.Examples) > 0 {
		out.Example = s.Examples[0]
		if len(s.Examples) > 1 {
			p.warn(diag.WarnDownlevelMultipleExamples, path,
				"Multiple examples not supported in 3.0; using first only")
		}
	}

	// Discriminator
	if s.Discriminator != nil {
		out.Discriminator = &DiscriminatorV30{
			PropertyName: s.Discriminator.PropertyName,
		}
		if len(s.Discriminator.Mapping) > 0 {
			out.Discriminator.Mapping = make(map[string]string, len(s.Discriminator.Mapping))
			for k, v := range s.Discriminator.Mapping {
				out.Discriminator.Mapping[k] = v
			}
		}
	}

	// XML
	if s.XML != nil {
		out.XML = &XMLV30{
			Name:      s.XML.Name,
			Namespace: s.XML.Namespace,
			Prefix:    s.XML.Prefix,
			Attribute: s.XML.Attribute,
			Wrapped:   s.XML.Wrapped,
		}
	}

	out.Extensions = p.ext(s.Extensions)

	return out
}

// MarshalJSON implements json.Marshaler for SchemaV30 to inline extensions.
func (s *SchemaV30) MarshalJSON() ([]byte, error) {
	return marshalWithExtensions(*s, s.Extensions)
}

// kindToString converts a Kind to OpenAPI 3.0 type string.
func kindToString(k model.Kind) string {
	switch k {
	case model.KindBoolean:
		return "boolean"
	case model.KindInteger:
		return "integer"
	case model.KindNumber:
		return "number"
	case model.KindString:
		return "string"
	case model.KindObject:
		return "object"
	case model.KindArray:
		return "array"
	default:
		return ""
	}
}
