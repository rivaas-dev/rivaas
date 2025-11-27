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

import "rivaas.dev/openapi/model"

// SchemaV31 represents an OpenAPI 3.1.x schema.
type SchemaV31 struct {
	Ref               string                `json:"$ref,omitempty"`
	Title             string                `json:"title,omitempty"`
	Type              any                   `json:"type,omitempty"` // string or []string
	Format            string                `json:"format,omitempty"`
	ContentEncoding   string                `json:"contentEncoding,omitempty"`
	ContentMediaType  string                `json:"contentMediaType,omitempty"`
	Description       string                `json:"description,omitempty"`
	Example           any                   `json:"example,omitempty"`
	Examples          []any                 `json:"examples,omitempty"`
	Deprecated        bool                  `json:"deprecated,omitempty"`
	ReadOnly          bool                  `json:"readOnly,omitempty"`
	WriteOnly         bool                  `json:"writeOnly,omitempty"`
	Enum              []any                 `json:"enum,omitempty"`
	Const             any                   `json:"const,omitempty"`
	MultipleOf        *float64              `json:"multipleOf,omitempty"`
	Maximum           *float64              `json:"maximum,omitempty"`
	ExclusiveMaximum  *float64              `json:"exclusiveMaximum,omitempty"`
	Minimum           *float64              `json:"minimum,omitempty"`
	ExclusiveMinimum  *float64              `json:"exclusiveMinimum,omitempty"`
	Pattern           string                `json:"pattern,omitempty"`
	MaxLength         *int                  `json:"maxLength,omitempty"`
	MinLength         *int                  `json:"minLength,omitempty"`
	Items             *SchemaV31            `json:"items,omitempty"`
	MaxItems          *int                  `json:"maxItems,omitempty"`
	MinItems          *int                  `json:"minItems,omitempty"`
	UniqueItems       bool                  `json:"uniqueItems,omitempty"`
	Properties        map[string]*SchemaV31 `json:"properties,omitempty"`
	Required          []string              `json:"required,omitempty"`
	AdditionalProps   any                   `json:"additionalProperties,omitempty"` // bool or *SchemaV31
	PatternProperties map[string]*SchemaV31 `json:"patternProperties,omitempty"`
	UnevaluatedProps  *SchemaV31            `json:"unevaluatedProperties,omitempty"`
	AllOf             []*SchemaV31          `json:"allOf,omitempty"`
	AnyOf             []*SchemaV31          `json:"anyOf,omitempty"`
	OneOf             []*SchemaV31          `json:"oneOf,omitempty"`
	Not               *SchemaV31            `json:"not,omitempty"`
	Default           any                   `json:"default,omitempty"`
	MinProperties     *int                  `json:"minProperties,omitempty"`
	MaxProperties     *int                  `json:"maxProperties,omitempty"`
	Extensions        map[string]any        `json:"-"`
}

// schema31 projects a Schema to OpenAPI 3.1.x format.
//
// Key transformations:
//   - Nullable: uses type: ["T", "null"] union
//   - Exclusive bounds: uses numeric exclusiveMinimum/Maximum
//   - Const: native support
//   - Unevaluated properties: native support
//   - Pattern properties: native support
//   - Multiple examples: native support
func schema31(s *model.Schema, warns *[]Warning, path string) *SchemaV31 {
	if s == nil {
		return nil
	}

	if s.Ref != "" {
		return &SchemaV31{Ref: s.Ref}
	}

	out := &SchemaV31{
		Title:       s.Title,
		Description: s.Description,
		Format:      s.Format,
		Deprecated:  s.Deprecated,
		ReadOnly:    s.ReadOnly,
		WriteOnly:   s.WriteOnly,
		Example:     s.Example,
		Examples:    append([]any(nil), s.Examples...),
		Enum:        append([]any(nil), s.Enum...),
		Const:       s.Const,
		Default:     s.Default,
	}

	// Type + "null" union (3.1 style)
	t := kindToString(s.Kind)
	if s.Nullable && t != "" {
		out.Type = []string{t, "null"}
	} else if t != "" {
		out.Type = t
	}

	// Binary data: contentEncoding and contentMediaType (3.1 native support)
	out.ContentEncoding = s.ContentEncoding
	out.ContentMediaType = s.ContentMediaType

	// Numeric bounds (exclusive as numeric values in 3.1)
	if s.MultipleOf != nil {
		out.MultipleOf = s.MultipleOf
	}
	if s.Maximum != nil {
		if s.Maximum.Exclusive {
			out.ExclusiveMaximum = &s.Maximum.Value
		} else {
			out.Maximum = &s.Maximum.Value
		}
	}
	if s.Minimum != nil {
		if s.Minimum.Exclusive {
			out.ExclusiveMinimum = &s.Minimum.Value
		} else {
			out.Minimum = &s.Minimum.Value
		}
	}

	// String constraints
	if s.Pattern != "" {
		out.Pattern = s.Pattern
	}
	out.MinLength, out.MaxLength = s.MinLength, s.MaxLength

	// Array constraints
	if s.Items != nil {
		out.Items = schema31(s.Items, warns, path+"/items")
	}
	out.MinItems, out.MaxItems = s.MinItems, s.MaxItems
	if s.UniqueItems {
		out.UniqueItems = true
	}

	// Object constraints
	if len(s.Properties) > 0 {
		out.Properties = make(map[string]*SchemaV31, len(s.Properties))
		for k, v := range s.Properties {
			out.Properties[k] = schema31(v, warns, path+"/properties/"+k)
		}
	}
	if len(s.Required) > 0 {
		out.Required = append([]string(nil), s.Required...)
	}

	// Additional properties
	if s.Additional != nil {
		switch {
		case s.Additional.Schema != nil:
			out.AdditionalProps = schema31(s.Additional.Schema, warns, path+"/additionalProperties")
		case s.Additional.Allow != nil:
			out.AdditionalProps = *s.Additional.Allow
		}
	}

	// Pattern properties (3.1 native support)
	if len(s.PatternProps) > 0 {
		out.PatternProperties = make(map[string]*SchemaV31, len(s.PatternProps))
		for rx, v := range s.PatternProps {
			out.PatternProperties[rx] = schema31(v, warns, path+"/patternProperties/"+rx)
		}
	}

	// Unevaluated properties (3.1 native support)
	if s.Unevaluated != nil {
		out.UnevaluatedProps = schema31(s.Unevaluated, warns, path+"/unevaluatedProperties")
	}

	// Object property count constraints
	out.MinProperties, out.MaxProperties = s.MinProperties, s.MaxProperties

	// Composition
	for _, it := range s.AllOf {
		out.AllOf = append(out.AllOf, schema31(it, warns, path+"/allOf"))
	}
	for _, it := range s.AnyOf {
		out.AnyOf = append(out.AnyOf, schema31(it, warns, path+"/anyOf"))
	}
	for _, it := range s.OneOf {
		out.OneOf = append(out.OneOf, schema31(it, warns, path+"/oneOf"))
	}
	if s.Not != nil {
		out.Not = schema31(s.Not, warns, path+"/not")
	}

	// Examples: prefer Examples array, fallback to Example
	if len(s.Examples) == 0 && s.Example != nil {
		out.Examples = []any{s.Example}
	}

	out.Extensions = copyExtensions(s.Extensions, "3.1.2")
	return out
}

// MarshalJSON implements json.Marshaler for SchemaV31 to inline extensions.
func (s *SchemaV31) MarshalJSON() ([]byte, error) {
	return marshalWithExtensions(*s, s.Extensions)
}
