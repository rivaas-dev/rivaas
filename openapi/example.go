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

package openapi

// Example represents an OpenAPI Example Object.
// Used for both request body and response body examples.
// See: https://spec.openapis.org/oas/v3.1.0#example-object
type Example struct {
	name          string // Unique key in the examples map (required)
	summary       string // Short description (optional)
	description   string // Long description, CommonMark supported (optional)
	value         any    // Inline example value (mutually exclusive with externalValue)
	externalValue string // URL to external example (mutually exclusive with value)
}

// ExampleOption is a functional option for configuring an Example.
type ExampleOption func(*Example)

// NewExample creates a named example with an inline value.
// Note: This is named NewExample instead of Example to avoid a naming conflict
// with the Example type. Go does not allow a type and function with the same name
// in the same package.
//
// Parameters:
//   - name: Unique identifier (used as key in OpenAPI examples map)
//   - value: The example value (serialized to JSON in spec)
//   - opts: Optional configuration (summary, description)
//
// Usage:
//
//	openapi.NewExample("success", UserResponse{ID: 123, Name: "John"})
//
//	openapi.NewExample("admin", UserResponse{ID: 1, Role: "admin"},
//	    openapi.WithExampleSummary("Admin user response"),
//	)
func NewExample(name string, value any, opts ...ExampleOption) Example {
	e := Example{
		name:  name,
		value: value,
	}
	for _, opt := range opts {
		opt(&e)
	}
	return e
}

// ExternalExample creates a named example pointing to an external URL.
//
// Use this for:
//   - Large examples that would bloat the spec
//   - Examples in formats that can't be embedded (XML, binary)
//   - Shared examples across multiple specs
//
// Parameters:
//   - name: Unique identifier
//   - url: URL pointing to the example content
//   - opts: Optional configuration (summary, description)
//
// Usage:
//
//	openapi.ExternalExample("large-response", "https://api.example.com/examples/large.json")
//
//	openapi.ExternalExample("xml-example", "https://api.example.com/examples/user.xml",
//	    openapi.WithExampleSummary("XML format response"),
//	)
func ExternalExample(name, url string, opts ...ExampleOption) Example {
	e := Example{
		name:          name,
		externalValue: url,
	}
	for _, opt := range opts {
		opt(&e)
	}
	return e
}

// WithExampleSummary sets a short description for the example.
// This appears as the example title in Swagger UI.
//
// Usage:
//
//	openapi.NewExample("success", data,
//	    openapi.WithExampleSummary("Successful response"),
//	)
func WithExampleSummary(summary string) ExampleOption {
	return func(e *Example) {
		e.summary = summary
	}
}

// WithExampleDescription sets a detailed description for the example.
// CommonMark syntax is supported for rich text formatting.
//
// Usage:
//
//	openapi.NewExample("success", data,
//	    openapi.WithExampleDescription("Returns when the user ID exists and is active."),
//	)
func WithExampleDescription(description string) ExampleOption {
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
func (e Example) Value() any { return e.value }

// ExternalValue returns the URL to an external example.
func (e Example) ExternalValue() string { return e.externalValue }

// IsExternal returns true if this example uses an external URL.
func (e Example) IsExternal() bool { return e.externalValue != "" }
