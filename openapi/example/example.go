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

// Package example provides types and constructors for OpenAPI Example Objects.
//
// Examples can be attached to request bodies and responses to help API consumers
// understand expected data formats. Both inline values and external URLs are supported.
//
// Usage:
//
//	import "rivaas.dev/openapi/example"
//
//	// Inline example
//	example.New("success", UserResponse{ID: 123, Name: "John"})
//
//	// With metadata
//	example.New("admin", UserResponse{ID: 1, Role: "admin"},
//		example.WithSummary("Admin user response"),
//		example.WithDescription("Users with admin role have elevated permissions"),
//	)
//
//	// External URL example
//	example.NewExternal("large", "https://api.example.com/examples/large.json",
//		example.WithSummary("Large dataset"),
//	)
package example

// Example represents an OpenAPI Example Object.
// See: https://spec.openapis.org/oas/v3.1.0#example-object
//
// An Example can contain either an inline value or a reference to an external URL.
// These are mutually exclusive per the OpenAPI specification.
type Example struct {
	name          string // Unique key in the examples map (required)
	summary       string // Short description (optional)
	description   string // Long description, CommonMark supported (optional)
	value         any    // Inline example value (mutually exclusive with externalValue)
	externalValue string // URL to external example (mutually exclusive with value)
}

// Option configures an Example using the functional options pattern.
type Option func(*Example)

// New creates a named example with an inline value.
//
// The name is used as the key in the OpenAPI examples map. It must be unique
// within the same request/response context.
//
// Parameters:
//   - name: Unique identifier for this example
//   - value: The example value (will be serialized to JSON in the spec)
//   - opts: Optional configuration (summary, description)
//
// Example:
//
//	example.New("success", UserResponse{ID: 123, Name: "John"})
//
//	example.New("admin", UserResponse{ID: 1, Role: "admin"},
//		example.WithSummary("Admin user"),
//		example.WithDescription("Has elevated permissions"),
//	)
func New(name string, value any, opts ...Option) Example {
	e := Example{
		name:  name,
		value: value,
	}
	for _, opt := range opts {
		opt(&e)
	}
	return e
}

// NewExternal creates a named example pointing to an external URL.
//
// Use this for:
//   - Large examples that would bloat the spec
//   - Examples in formats that cannot be embedded (XML, binary)
//   - Shared examples across multiple specifications
//
// Parameters:
//   - name: Unique identifier for this example
//   - url: URL pointing to the example content
//   - opts: Optional configuration (summary, description)
//
// Example:
//
//	example.NewExternal("large", "https://api.example.com/examples/large.json")
//
//	example.NewExternal("xml-format", "https://api.example.com/examples/user.xml",
//		example.WithSummary("XML format response"),
//	)
func NewExternal(name, url string, opts ...Option) Example {
	e := Example{
		name:          name,
		externalValue: url,
	}
	for _, opt := range opts {
		opt(&e)
	}
	return e
}

// WithSummary sets a short description for the example.
//
// The summary appears as the example title in Swagger UI and other documentation tools.
// It should be concise (one line).
//
// Example:
//
//	example.New("success", data, example.WithSummary("Successful response"))
func WithSummary(summary string) Option {
	return func(e *Example) {
		e.summary = summary
	}
}

// WithDescription sets a detailed description for the example.
//
// CommonMark syntax is supported for rich text formatting. Use this for
// longer explanations that wouldn't fit in a summary.
//
// Example:
//
//	example.New("admin", data,
//		example.WithDescription("Users with admin role have full system access.\n\n**Note:** Admin users can modify all resources."),
//	)
func WithDescription(description string) Option {
	return func(e *Example) {
		e.description = description
	}
}

// Name returns the unique identifier for this example.
func (e Example) Name() string { return e.name }

// Summary returns the short description.
func (e Example) Summary() string { return e.summary }

// Description returns the detailed description.
func (e Example) Description() string { return e.description }

// Value returns the inline example value.
// Returns nil for external examples.
func (e Example) Value() any { return e.value }

// ExternalValue returns the URL to an external example.
// Returns empty string for inline examples.
func (e Example) ExternalValue() string { return e.externalValue }

// IsExternal returns true if this example uses an external URL.
func (e Example) IsExternal() bool { return e.externalValue != "" }
