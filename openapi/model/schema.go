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

package model

// Schema represents a version-agnostic JSON Schema in the intermediate representation.
//
// This IR is a superset of features needed for both OpenAPI 3.0.x and 3.1.x.
// Version-specific differences are handled by projectors in the export package.
type Schema struct {
	// Ref is a logical reference to a component schema.
	// Projectors will resolve this to the appropriate $ref format.
	Ref string

	// Kind is the JSON Schema type kind.
	Kind Kind

	// Nullable indicates if the value can be null.
	// In 3.0: represented as nullable: true
	// In 3.1: represented as type: ["T", "null"]
	Nullable bool

	// Title provides a title for the schema.
	Title string

	// Description provides documentation for the schema.
	Description string

	// Format provides additional type information (e.g., "date-time", "email", "int64").
	Format string

	// Deprecated marks the schema as deprecated.
	Deprecated bool

	// ReadOnly indicates the value is read-only.
	ReadOnly bool

	// WriteOnly indicates the value is write-only.
	WriteOnly bool

	// Example provides a single example value (3.0 style).
	Example any

	// Examples provides multiple example values (3.1 style).
	// If both Example and Examples are set, projectors will prefer Examples for 3.1.
	Examples []any

	// Pattern is a regex pattern for string validation.
	Pattern string

	// MinLength is the minimum string length.
	MinLength *int

	// MaxLength is the maximum string length.
	MaxLength *int

	// Minimum is the minimum numeric value (with exclusive flag).
	// Projectors will convert this to version-specific format.
	Minimum *Bound

	// Maximum is the maximum numeric value (with exclusive flag).
	// Projectors will convert this to version-specific format.
	Maximum *Bound

	// MultipleOf constrains numbers to be multiples of this value.
	MultipleOf *float64

	// Items defines the item schema for arrays.
	Items *Schema

	// MinItems is the minimum number of items in an array.
	MinItems *int

	// MaxItems is the maximum number of items in an array.
	MaxItems *int

	// UniqueItems indicates array items must be unique.
	UniqueItems bool

	// Properties defines object properties.
	Properties map[string]*Schema

	// Required lists required property names (for type "object").
	Required []string

	// Additional controls additionalProperties behavior.
	// See Additional type documentation for semantics.
	Additional *Additional

	// PatternProps defines pattern-based properties (3.1 feature).
	// In 3.0, this will be dropped with a warning.
	PatternProps map[string]*Schema

	// Unevaluated defines unevaluatedProperties schema (3.1 feature).
	// In 3.0, this will be dropped with a warning.
	Unevaluated *Schema

	// MinProperties is the minimum number of properties in an object.
	MinProperties *int

	// MaxProperties is the maximum number of properties in an object.
	MaxProperties *int

	// AllOf represents an allOf composition.
	AllOf []*Schema

	// AnyOf represents an anyOf composition.
	AnyOf []*Schema

	// OneOf represents a oneOf composition.
	OneOf []*Schema

	// Not represents a not composition.
	Not *Schema

	// Enum lists allowed values for the schema.
	Enum []any

	// Const is a constant value (3.1 feature).
	// In 3.0, this will be converted to enum: [const] with a warning.
	Const any

	// Default is the default value for the schema.
	Default any

	// Discriminator is used for polymorphism (optional).
	Discriminator *Discriminator

	// XML provides XML serialization hints (optional).
	XML *XML

	// ExternalDocs provides external documentation links (optional).
	ExternalDocs *ExternalDocs

	// Extensions contains specification extensions (fields prefixed with x-).
	Extensions map[string]any
}

// Discriminator is used for polymorphism in oneOf/allOf compositions.
type Discriminator struct {
	PropertyName string
	Mapping      map[string]string
}

// XML provides XML serialization hints.
type XML struct {
	Name      string
	Namespace string
	Prefix    string
	Attribute bool
	Wrapped   bool
}

// ExternalDocs provides external documentation links.
type ExternalDocs struct {
	Description string
	URL         string
	Extensions  map[string]any
}
