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

// Kind represents a JSON Schema type kind.
type Kind uint8

const (
	KindUnknown Kind = iota
	KindNull
	KindBoolean
	KindInteger
	KindNumber
	KindString
	KindObject
	KindArray
)

// Bound represents a numeric bound (minimum or maximum) with exclusive flag.
//
// In OpenAPI 3.0, exclusive bounds are represented as boolean flags:
//   - minimum: 10, exclusiveMinimum: true
//
// In OpenAPI 3.1, exclusive bounds are represented as numeric values:
//   - exclusiveMinimum: 10 (instead of minimum: 10)
type Bound struct {
	Value     float64
	Exclusive bool
}

// Additional represents additionalProperties configuration for objects.
//
// The semantics are:
//   - nil => not specified (JSON Schema default: true)
//   - Allow != nil && *Allow == false && Schema == nil => additionalProperties: false (strict)
//   - Allow != nil && *Allow == true && Schema == nil => additionalProperties: true (explicit allow-all)
//   - Schema != nil => additionalProperties: <schema> (takes precedence over Allow)
type Additional struct {
	// Allow controls whether additional properties are allowed.
	// If nil, the behavior is unspecified (defaults to true in JSON Schema).
	// If Schema is non-nil, it takes precedence over Allow.
	Allow *bool

	// Schema defines the schema for additional properties.
	// If set, this takes precedence over Allow.
	Schema *Schema
}

// NoAdditionalProps returns an Additional that disallows additional properties.
func NoAdditionalProps() *Additional {
	f := false
	return &Additional{Allow: &f}
}

// AdditionalPropsSchema returns an Additional that allows additional properties matching the given schema.
func AdditionalPropsSchema(s *Schema) *Additional {
	return &Additional{Schema: s}
}
