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

import "rivaas.dev/openapi/internal/schema"

// RequestMetadata contains auto-discovered information about a request struct.
//
// This is a type alias for the internal schema.RequestMetadata type.
// It's exported here for use in RouteDoc.
type RequestMetadata = schema.RequestMetadata

// ParamSpec describes a single parameter extracted from struct tags.
//
// This is a type alias for the internal schema.ParamSpec type.
// It's exported here for use in RequestMetadata.
type ParamSpec = schema.ParamSpec

// Spec represents the root OpenAPI 3.0.4 document.
//
// This is the top-level structure that represents a complete OpenAPI specification.
// It can be marshaled to JSON and served to API documentation tools and code generators.
type Spec struct {
	// OpenAPI version string (e.g., Version30 or Version31).
	OpenAPI string `json:"openapi"`

	// Info contains API metadata (title, version, description, contact, license).
	Info Info `json:"info"`

	// Servers lists available server URLs for the API.
	Servers []Server `json:"servers,omitempty"`

	// Paths maps path patterns to PathItem objects containing operations.
	Paths map[string]*PathItem `json:"paths"`

	// Components holds reusable schemas, security schemes, etc.
	Components *Components `json:"components,omitempty"`

	// Tags provides additional metadata for operations.
	Tags []Tag `json:"tags,omitempty"`

	// Security defines global security requirements applied to all operations.
	Security []SecurityRequirement `json:"security,omitempty"`

	// ExternalDocs provides external documentation links.
	ExternalDocs *ExternalDocs `json:"externalDocs,omitempty"`
}

// Info provides metadata about the API.
type Info struct {
	// Title is the API title (required).
	Title string `json:"title"`

	// Summary is a short summary of the API (OpenAPI 3.1+ only).
	// In 3.0 targets, this will be dropped with a warning.
	Summary string `json:"summary,omitempty"`

	// Description provides a detailed description of the API (supports Markdown).
	Description string `json:"description,omitempty"`

	// TermsOfService is a URL/URI for the Terms of Service.
	TermsOfService string `json:"termsOfService,omitempty"`

	// Version is the API version (required, e.g., "1.0.0").
	Version string `json:"version"`

	// Contact provides contact information for the API.
	Contact *Contact `json:"contact,omitempty"`

	// License provides license information for the API.
	License *License `json:"license,omitempty"`

	// Extensions contains specification extensions (fields prefixed with x-).
	//
	// Direct mutation of this map after New/MustNew bypasses Config.Validate().
	// However, projection-time filtering via copyExtensions still applies:
	// - Keys must start with "x-"
	// - In OpenAPI 3.1.x, keys starting with "x-oai-" or "x-oas-" are reserved and will be filtered
	//
	// Prefer using WithInfoExtension() option instead of direct mutation.
	Extensions map[string]any `json:"-"`
}

// Contact information for the API.
type Contact struct {
	// Name is the contact person or organization name.
	Name string `json:"name,omitempty"`

	// URL is the URL pointing to the contact information.
	URL string `json:"url,omitempty"`

	// Email is the email address of the contact.
	Email string `json:"email,omitempty"`

	// Extensions contains specification extensions (fields prefixed with x-).
	//
	// Direct mutation of this map after New/MustNew bypasses Config.Validate().
	// However, projection-time filtering via copyExtensions still applies:
	// - Keys must start with "x-"
	// - In OpenAPI 3.1.x, keys starting with "x-oai-" or "x-oas-" are reserved and will be filtered
	//
	// Prefer using helper options (e.g., WithContactExtension, WithLicenseExtension) instead of direct mutation.
	Extensions map[string]any `json:"-"`
}

// License information for the API.
type License struct {
	// Name is the license name (required, e.g., "MIT", "Apache 2.0").
	Name string `json:"name"`

	// Identifier is an SPDX license expression (OpenAPI 3.1+, e.g., "Apache-2.0").
	// Mutually exclusive with URL.
	Identifier string `json:"identifier,omitempty"`

	// URL is the URL to the license (OpenAPI 3.0, e.g., "https://www.apache.org/licenses/LICENSE-2.0.html").
	// Mutually exclusive with Identifier.
	URL string `json:"url,omitempty"`

	// Extensions contains specification extensions (fields prefixed with x-).
	//
	// Direct mutation of this map after New/MustNew bypasses Config.Validate().
	// However, projection-time filtering via copyExtensions still applies:
	// - Keys must start with "x-"
	// - In OpenAPI 3.1.x, keys starting with "x-oai-" or "x-oas-" are reserved and will be filtered
	//
	// Prefer using helper options (e.g., WithLicenseExtension) instead of direct mutation.
	Extensions map[string]any `json:"-"`
}

// Server represents a server URL and optional description.
type Server struct {
	// URL is the server URL (required, e.g., "https://api.example.com").
	// Supports variable substitution with {variableName} syntax.
	URL string `json:"url"`

	// Description helps distinguish between different server environments.
	Description string `json:"description,omitempty"`

	// Variables defines variable substitutions for the server URL template.
	Variables map[string]*ServerVariable `json:"variables,omitempty"`

	// Extensions contains specification extensions (fields prefixed with x-).
	//
	// Direct mutation of this map after New/MustNew bypasses Config.Validate().
	// However, projection-time filtering via copyExtensions still applies:
	// - Keys must start with "x-"
	// - In OpenAPI 3.1.x, keys starting with "x-oai-" or "x-oas-" are reserved and will be filtered
	//
	// Prefer using helper options (e.g., WithServerExtension) instead of direct mutation.
	Extensions map[string]any `json:"-"`
}

// ServerVariable represents a variable for server URL template substitution.
type ServerVariable struct {
	// Enum is an enumeration of allowed values (optional).
	// In OpenAPI 3.1, this array MUST NOT be empty if present.
	Enum []string `json:"enum,omitempty"`

	// Default is the default value for substitution (required).
	// If enum is defined, this value MUST exist in the enum (3.1) or SHOULD exist (3.0).
	Default string `json:"default"`

	// Description provides additional information about the variable.
	Description string `json:"description,omitempty"`

	// Extensions contains specification extensions (fields prefixed with x-).
	Extensions map[string]any `json:"-"`
}

// PathItem represents the operations available on a single path.
//
// Each HTTP method (GET, POST, etc.) can have its own Operation. Path-level
// parameters apply to all operations on that path.
type PathItem struct {
	// Summary provides a brief description of the path.
	Summary string `json:"summary,omitempty"`

	// Description provides detailed information about the path.
	Description string `json:"description,omitempty"`

	// Get is the GET operation for this path.
	Get *Operation `json:"get,omitempty"`

	// Put is the PUT operation for this path.
	Put *Operation `json:"put,omitempty"`

	// Post is the POST operation for this path.
	Post *Operation `json:"post,omitempty"`

	// Delete is the DELETE operation for this path.
	Delete *Operation `json:"delete,omitempty"`

	// Options is the OPTIONS operation for this path.
	Options *Operation `json:"options,omitempty"`

	// Head is the HEAD operation for this path.
	Head *Operation `json:"head,omitempty"`

	// Patch is the PATCH operation for this path.
	Patch *Operation `json:"patch,omitempty"`

	// Parameters are path-level parameters that apply to all operations.
	Parameters []Parameter `json:"parameters,omitempty"`
}

// Operation describes a single API operation on a path.
//
// An operation represents a single HTTP method on a path and contains all
// information needed to document and interact with that endpoint.
type Operation struct {
	// Tags groups operations for organization in Swagger UI.
	Tags []string `json:"tags,omitempty"`

	// Summary is a brief, one-line description of the operation.
	Summary string `json:"summary,omitempty"`

	// Description provides detailed information about the operation (supports Markdown).
	Description string `json:"description,omitempty"`

	// OperationID is a unique identifier for the operation (used by code generators).
	OperationID string `json:"operationId,omitempty"`

	// Parameters lists query, path, header, and cookie parameters.
	Parameters []Parameter `json:"parameters,omitempty"`

	// RequestBody describes the request body schema (for POST, PUT, PATCH).
	RequestBody *RequestBody `json:"requestBody,omitempty"`

	// Responses maps HTTP status codes to response definitions.
	Responses map[string]*Response `json:"responses"`

	// Deprecated marks the operation as deprecated.
	Deprecated bool `json:"deprecated,omitempty"`

	// Security overrides global security requirements for this operation.
	Security []SecurityRequirement `json:"security,omitempty"`
}

// Parameter describes a single operation parameter.
//
// Parameters can be in query, path, header, or cookie locations.
type Parameter struct {
	// Name is the parameter name (required).
	Name string `json:"name"`

	// In is the parameter location: "query", "path", "header", or "cookie" (required).
	In string `json:"in"`

	// Description provides documentation for the parameter.
	Description string `json:"description,omitempty"`

	// Required indicates if the parameter is required (always true for path parameters).
	Required bool `json:"required,omitempty"`

	// Deprecated indicates that the parameter is deprecated and should be transitioned out.
	Deprecated bool `json:"deprecated,omitempty"`

	// AllowEmptyValue allows clients to pass a zero-length string value in place of parameters
	// that would otherwise be omitted. Valid only for query parameters.
	AllowEmptyValue bool `json:"allowEmptyValue,omitempty"`

	// Style describes how the parameter value will be serialized.
	// Defaults: "form" for query/cookie, "simple" for path/header.
	Style string `json:"style,omitempty"`

	// Explode when true, generates separate parameters for each value of arrays or
	// key-value pairs of objects. Default: true for "form" style, false otherwise.
	Explode bool `json:"explode,omitempty"`

	// AllowReserved when true, uses reserved expansion (RFC6570) allowing reserved characters
	// to pass through unchanged. Only applies to query parameters.
	AllowReserved bool `json:"allowReserved,omitempty"`

	// Schema defines the parameter type and validation rules (mutually exclusive with Content).
	Schema *Schema `json:"schema,omitempty"`

	// Example provides an example value for the parameter.
	Example any `json:"example,omitempty"`

	// Examples provides multiple examples keyed by name (mutually exclusive with Example).
	Examples map[string]*ExampleSpec `json:"examples,omitempty"`

	// Content defines the media type and schema for complex serialization
	// (mutually exclusive with Schema). The map MUST contain only one entry.
	Content map[string]*MediaType `json:"content,omitempty"`
}

// RequestBody describes a single request body.
//
// Request bodies are used for POST, PUT, and PATCH operations.
type RequestBody struct {
	// Description provides documentation for the request body.
	Description string `json:"description,omitempty"`

	// Required indicates if the request body is required.
	Required bool `json:"required,omitempty"`

	// Content maps content types (e.g., "application/json") to MediaType definitions.
	Content map[string]*MediaType `json:"content"`
}

// Response describes a single response from an API operation.
//
// Each HTTP status code can have its own Response definition.
type Response struct {
	// Description describes the response (required).
	Description string `json:"description"`

	// Content maps content types (e.g., "application/json") to MediaType definitions.
	Content map[string]*MediaType `json:"content,omitempty"`

	// Headers defines response headers.
	Headers map[string]*Header `json:"headers,omitempty"`

	// Links defines operations that can be followed from the response.
	Links map[string]*Link `json:"links,omitempty"`
}

// Header represents a response header.
//
// Headers follow a similar structure to Parameters but are specific to responses.
// Headers can use either schema-based or content-based serialization.
type Header struct {
	// Description provides documentation for the header.
	Description string `json:"description,omitempty"`

	// Required indicates if the header is mandatory.
	Required bool `json:"required,omitempty"`

	// Deprecated indicates that the header is deprecated and should be transitioned out.
	Deprecated bool `json:"deprecated,omitempty"`

	// Style describes how the header value will be serialized.
	// For headers, the only valid value is "simple" (default).
	Style string `json:"style,omitempty"`

	// Explode generates comma-separated values for array/object types.
	// Default is false.
	Explode bool `json:"explode,omitempty"`

	// Schema defines the header value type (for schema-based serialization).
	Schema *Schema `json:"schema,omitempty"`

	// Example provides an example value for the header.
	Example any `json:"example,omitempty"`

	// Examples provides multiple named example values.
	Examples map[string]*ExampleSpec `json:"examples,omitempty"`

	// Content defines the media type and schema (for content-based serialization).
	// The map MUST contain only one entry.
	Content map[string]*MediaType `json:"content,omitempty"`
}

// Link represents a design-time link for a response.
//
// Links allow you to define operations that can be followed from a response.
// This is useful for describing workflows and relationships between operations.
type Link struct {
	// OperationRef references an operation using a JSON Pointer or relative reference.
	// Example: "#/paths/~1users~1{userId}/get"
	OperationRef string `json:"operationRef,omitempty"`

	// OperationID references an operation by its operationId.
	// This is an alternative to OperationRef when the operation is in the same document.
	OperationID string `json:"operationId,omitempty"`

	// Parameters to pass to the linked operation.
	// Keys are parameter names, values are expressions or values to use.
	Parameters map[string]any `json:"parameters,omitempty"`

	// RequestBody to use as the request body for the linked operation.
	RequestBody any `json:"requestBody,omitempty"`

	// Description of the link.
	Description string `json:"description,omitempty"`

	// Server to be used by the target operation.
	// If not specified, the server from the operation is used.
	Server *Server `json:"server,omitempty"`
}

// ExampleSpec represents an example value with optional description.
// This is the exported spec type. For API usage, see the [example.Example] type.
type ExampleSpec struct {
	// Summary provides a short summary of the example.
	Summary string `json:"summary,omitempty"`

	// Description provides a detailed description of the example.
	Description string `json:"description,omitempty"`

	// Value is the example value.
	Value any `json:"value,omitempty"`

	// ExternalValue is a URL pointing to an external example.
	ExternalValue string `json:"externalValue,omitempty"`
}

// Encoding describes encoding for a single schema property.
//
// Used with multipart and application/x-www-form-urlencoded media types.
// The key in the encoding map corresponds to a property name in the schema.
type Encoding struct {
	// ContentType for encoding a specific property (comma-separated list).
	// Defaults depend on the property type (see OpenAPI spec).
	ContentType string `json:"contentType,omitempty"`

	// Headers provides additional headers for the part (multipart only).
	Headers map[string]*Header `json:"headers,omitempty"`

	// Style describes how the property value will be serialized.
	// Valid values: "form", "spaceDelimited", "pipeDelimited", "deepObject".
	Style string `json:"style,omitempty"`

	// Explode generates separate parameters for array/object values.
	// Default is true when style is "form", false otherwise.
	Explode bool `json:"explode,omitempty"`

	// AllowReserved allows reserved characters per RFC6570.
	// Default is false.
	AllowReserved bool `json:"allowReserved,omitempty"`
}

// MediaType provides schema and examples for a specific content type.
//
// Used in both RequestBody and Response to define the structure of
// request/response bodies for a given content type (e.g., "application/json").
type MediaType struct {
	// Schema defines the structure of the content.
	Schema *Schema `json:"schema,omitempty"`

	// Example provides an example value for the content type.
	Example any `json:"example,omitempty"`

	// Examples provides multiple named example values (OpenAPI 3.1+, but works in 3.0 too).
	Examples map[string]*ExampleSpec `json:"examples,omitempty"`

	// Encoding defines encoding for specific schema properties.
	// Only applies to request bodies with multipart or application/x-www-form-urlencoded.
	Encoding map[string]*Encoding `json:"encoding,omitempty"`
}

// Schema represents a JSON Schema for describing data structures.
//
// Schemas are used to define parameter types, request bodies, and response bodies.
// They support the full JSON Schema specification compatible with OpenAPI 3.0.4.
type Schema struct {
	// Type is the JSON Schema type: "string", "number", "integer", "boolean", "array", "object".
	Type string `json:"type,omitempty"`

	// Format provides additional type information (e.g., "date-time", "email", "int64").
	Format string `json:"format,omitempty"`

	// Description provides documentation for the schema.
	Description string `json:"description,omitempty"`

	// Properties defines object properties (for type "object").
	Properties map[string]*Schema `json:"properties,omitempty"`

	// Required lists required property names (for type "object").
	Required []string `json:"required,omitempty"`

	// Items defines the item schema (for type "array").
	Items *Schema `json:"items,omitempty"`

	// AdditionalProperties defines the schema for additional properties (for type "object").
	AdditionalProperties *Schema `json:"additionalProperties,omitempty"`

	// Ref is a reference to a component schema (e.g., "#/components/schemas/User").
	Ref string `json:"$ref,omitempty"`

	// Enum lists allowed values for the schema.
	Enum []any `json:"enum,omitempty"`

	// Default is the default value for the schema.
	Default any `json:"default,omitempty"`

	// Example provides an example value for the schema.
	Example any `json:"example,omitempty"`

	// Nullable indicates if the value can be null.
	Nullable bool `json:"nullable,omitempty"`

	// Minimum is the minimum numeric value (for type "number" or "integer").
	Minimum *float64 `json:"minimum,omitempty"`

	// Maximum is the maximum numeric value (for type "number" or "integer").
	Maximum *float64 `json:"maximum,omitempty"`

	// MinLength is the minimum string length (for type "string").
	MinLength *int `json:"minLength,omitempty"`

	// MaxLength is the maximum string length (for type "string").
	MaxLength *int `json:"maxLength,omitempty"`

	// Pattern is a regex pattern for string validation (for type "string").
	Pattern string `json:"pattern,omitempty"`
}

// Components holds reusable components (schemas, security schemes, etc.).
//
// Components allow definitions to be referenced multiple times, reducing
// duplication and improving spec maintainability.
type Components struct {
	// Schemas maps schema names to Schema definitions.
	Schemas map[string]*Schema `json:"schemas,omitempty"`

	// SecuritySchemes maps security scheme names to SecurityScheme definitions.
	SecuritySchemes map[string]*SecurityScheme `json:"securitySchemes,omitempty"`
}

// SecurityScheme defines a security scheme that can be used by operations.
//
// Security schemes define authentication/authorization methods like Bearer tokens,
// API keys, OAuth flows, OpenID Connect, or mutual TLS.
type SecurityScheme struct {
	// Type is the security scheme type: "http", "apiKey", "oauth2", "openIdConnect", "mutualTLS" (3.1+).
	Type string `json:"type"`

	// Description provides documentation for the security scheme.
	Description string `json:"description,omitempty"`

	// Name is the parameter name (for type "apiKey").
	Name string `json:"name,omitempty"`

	// In is the parameter location: "header", "query", or "cookie" (for type "apiKey").
	In string `json:"in,omitempty"`

	// Scheme is the HTTP scheme (for type "http", e.g., "bearer").
	Scheme string `json:"scheme,omitempty"`

	// BearerFormat is the bearer token format (for type "http" with scheme "bearer", e.g., "JWT").
	BearerFormat string `json:"bearerFormat,omitempty"`

	// Flows contains OAuth2 flow configuration (for type "oauth2").
	Flows *OAuthFlows `json:"flows,omitempty"`

	// OpenIDConnectURL is the well-known URL to discover OpenID Connect provider metadata (for type "openIdConnect").
	OpenIDConnectURL string `json:"openIdConnectUrl,omitempty"`
}

// OAuthFlows allows configuration of the supported OAuth Flows.
//
// At least one flow must be configured for OAuth2 security schemes.
type OAuthFlows struct {
	// Implicit configuration for the OAuth Implicit flow.
	Implicit *OAuthFlow `json:"implicit,omitempty"`

	// Password configuration for the OAuth Resource Owner Password flow.
	Password *OAuthFlow `json:"password,omitempty"`

	// ClientCredentials configuration for the OAuth Client Credentials flow.
	ClientCredentials *OAuthFlow `json:"clientCredentials,omitempty"`

	// AuthorizationCode configuration for the OAuth Authorization Code flow.
	AuthorizationCode *OAuthFlow `json:"authorizationCode,omitempty"`
}

// OAuthFlow contains configuration details for a supported OAuth Flow.
type OAuthFlow struct {
	// AuthorizationURL is the authorization URL (REQUIRED for implicit, authorizationCode).
	AuthorizationURL string `json:"authorizationUrl,omitempty"`

	// TokenURL is the token URL (REQUIRED for password, clientCredentials, authorizationCode).
	TokenURL string `json:"tokenUrl,omitempty"`

	// RefreshURL is the URL for obtaining refresh tokens (optional).
	RefreshURL string `json:"refreshUrl,omitempty"`

	// Scopes maps scope names to their descriptions (REQUIRED, can be empty).
	Scopes map[string]string `json:"scopes"`
}

// SecurityRequirement lists the required security schemes for an operation.
//
// The map key is the security scheme name, and the value is a list of required scopes
// (empty list for schemes that don't use scopes, like Bearer or API Key).
type SecurityRequirement map[string][]string

// Tag adds metadata to a single tag that is used by the Operation Object.
//
// Tags help organize operations in Swagger UI by grouping related endpoints.
type Tag struct {
	// Name is the tag name (required).
	Name string `json:"name"`

	// Description provides documentation for the tag.
	Description string `json:"description,omitempty"`

	// ExternalDocs provides additional external documentation for this tag.
	ExternalDocs *ExternalDocs `json:"externalDocs,omitempty"`

	// Extensions contains specification extensions (fields prefixed with x-).
	//
	// Direct mutation of this map after New/MustNew bypasses Config.Validate().
	// However, projection-time filtering via copyExtensions still applies:
	// - Keys must start with "x-"
	// - In OpenAPI 3.1.x, keys starting with "x-oai-" or "x-oas-" are reserved and will be filtered
	//
	// Prefer using helper options (e.g., WithTagExtension) instead of direct mutation.
	Extensions map[string]any `json:"-"`
}

// ExternalDocs provides external documentation links.
type ExternalDocs struct {
	// Description provides a description of the target documentation (supports Markdown).
	Description string `json:"description,omitempty"`

	// URL is the URL/URI for the target documentation (required).
	URL string `json:"url"`

	// Extensions contains specification extensions (fields prefixed with x-).
	Extensions map[string]any `json:"-"`
}
