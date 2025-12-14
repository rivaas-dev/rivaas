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

// Spec represents a version-agnostic OpenAPI specification.
//
// This model supports all features from both OpenAPI 3.0.x and 3.1.x. Version-specific
// differences are handled by projectors in the export package.
type Spec struct {
	// Info contains API metadata (title, version, description, contact, license).
	Info Info

	// Servers lists available server URLs for the API.
	Servers []Server

	// Paths maps path patterns to PathItem objects containing operations.
	Paths map[string]*PathItem

	// Components holds reusable schemas, security schemes, etc.
	Components *Components

	// Webhooks defines webhook endpoints (3.1 feature).
	// In 3.0, this will be dropped with a warning.
	Webhooks map[string]*PathItem

	// Tags provides additional metadata for operations.
	Tags []Tag

	// Security defines global security requirements applied to all operations.
	Security []SecurityRequirement

	// ExternalDocs provides external documentation links.
	ExternalDocs *ExternalDocs

	// Extensions contains specification extensions (fields prefixed with x-).
	Extensions map[string]any
}

// Info provides metadata about the API.
type Info struct {
	Title          string
	Summary        string // 3.1+ only: short summary of the API
	Description    string
	TermsOfService string
	Version        string
	Contact        *Contact
	License        *License
	Extensions     map[string]any
}

// Contact provides contact information for the API.
type Contact struct {
	Name       string
	URL        string
	Email      string
	Extensions map[string]any
}

// License provides license information for the API.
type License struct {
	Name       string // REQUIRED. The license name used for the API.
	Identifier string // SPDX license expression (OpenAPI 3.1+). Mutually exclusive with URL.
	URL        string // A URI for the license (OpenAPI 3.0). Mutually exclusive with Identifier.
	Extensions map[string]any
}

// Server represents a server URL and optional description.
type Server struct {
	URL         string
	Description string
	Variables   map[string]*ServerVariable // Server variable substitution for URL template
	Extensions  map[string]any
}

// ServerVariable represents a variable for server URL template substitution.
type ServerVariable struct {
	Enum        []string // Enumeration of allowed values (optional)
	Default     string   // REQUIRED. Default value for substitution
	Description string   // Optional description
	Extensions  map[string]any
}

// PathItem represents the operations available on a single path.
type PathItem struct {
	Ref         string // If set, this PathItem is a reference ($ref)
	Summary     string
	Description string
	Get         *Operation
	Put         *Operation
	Post        *Operation
	Delete      *Operation
	Options     *Operation
	Head        *Operation
	Patch       *Operation
	Trace       *Operation // HTTP TRACE method
	Parameters  []Parameter
	Extensions  map[string]any
}

// Operation describes a single API operation on a path.
type Operation struct {
	Tags         []string
	Summary      string
	Description  string
	ExternalDocs *ExternalDocs
	OperationID  string
	Parameters   []Parameter
	RequestBody  *RequestBody
	Responses    map[string]*Response
	Callbacks    map[string]*Callback
	Deprecated   bool
	Security     []SecurityRequirement
	Servers      []Server
	Extensions   map[string]any
}

// Parameter describes a single operation parameter.
type Parameter struct {
	Ref             string // If set, this is a $ref
	Name            string
	In              string // query, path, header, cookie
	Description     string
	Required        bool
	Deprecated      bool
	AllowEmptyValue bool
	Style           string
	Explode         bool
	AllowReserved   bool
	Schema          *Schema
	Example         any
	Examples        map[string]*Example // 3.1 style
	Content         map[string]*MediaType
	Extensions      map[string]any
}

// Example represents an example value with optional description.
type Example struct {
	Ref           string // If set, this is a $ref
	Summary       string
	Description   string
	Value         any
	ExternalValue string
	Extensions    map[string]any
}

// RequestBody describes a single request body.
type RequestBody struct {
	Ref         string // If set, this is a $ref
	Description string
	Required    bool
	Content     map[string]*MediaType
	Extensions  map[string]any
}

// Response describes a single response from an API operation.
type Response struct {
	Ref         string // If set, this is a $ref
	Description string
	Content     map[string]*MediaType
	Headers     map[string]*Header
	Links       map[string]*Link
	Extensions  map[string]any
}

// Link represents a possible design-time link for a response.
type Link struct {
	Ref          string // If set, this is a $ref
	OperationRef string
	OperationID  string
	Parameters   map[string]any
	RequestBody  any
	Description  string
	Server       *Server
	Extensions   map[string]any
}

// Header represents a response header.
type Header struct {
	Ref             string // If set, this is a $ref
	Description     string
	Required        bool
	Deprecated      bool
	AllowEmptyValue bool
	Style           string
	Explode         bool
	Schema          *Schema
	Example         any
	Examples        map[string]*Example
	Content         map[string]*MediaType
	Extensions      map[string]any
}

// MediaType provides schema and examples for a specific content type.
type MediaType struct {
	Schema     *Schema
	Example    any
	Examples   map[string]*Example
	Encoding   map[string]*Encoding
	Extensions map[string]any
}

// Encoding describes encoding for a single schema property.
type Encoding struct {
	ContentType   string
	Headers       map[string]*Header
	Style         string
	Explode       bool
	AllowReserved bool
	Extensions    map[string]any
}

// Callback represents a callback definition (3.0+).
type Callback struct {
	Ref        string // If set, this is a $ref
	PathItems  map[string]*PathItem
	Extensions map[string]any
}

// Components holds reusable components.
type Components struct {
	Schemas         map[string]*Schema
	Responses       map[string]*Response
	Parameters      map[string]*Parameter
	Examples        map[string]*Example
	RequestBodies   map[string]*RequestBody
	Headers         map[string]*Header
	SecuritySchemes map[string]*SecurityScheme
	Links           map[string]*Link
	Callbacks       map[string]*Callback
	PathItems       map[string]*PathItem // 3.1 only
	Extensions      map[string]any
}

// SecurityScheme defines a security scheme.
type SecurityScheme struct {
	Ref              string // If set, this is a $ref
	Type             string // http, apiKey, oauth2, openIdConnect, mutualTLS (3.1+)
	Description      string
	Name             string      // for apiKey
	In               string      // header, query, cookie (for apiKey)
	Scheme           string      // bearer, basic (for http)
	BearerFormat     string      // JWT (for http bearer)
	Flows            *OAuthFlows // for oauth2
	OpenIDConnectURL string      // for openIdConnect
	Extensions       map[string]any
}

// OAuthFlows allows configuration of the supported OAuth Flows.
type OAuthFlows struct {
	Implicit          *OAuthFlow // Configuration for the OAuth Implicit flow
	Password          *OAuthFlow // Configuration for the OAuth Resource Owner Password flow
	ClientCredentials *OAuthFlow // Configuration for the OAuth Client Credentials flow
	AuthorizationCode *OAuthFlow // Configuration for the OAuth Authorization Code flow
	Extensions        map[string]any
}

// OAuthFlow contains configuration details for a supported OAuth Flow.
type OAuthFlow struct {
	AuthorizationURL string            // REQUIRED for implicit, authorizationCode
	TokenURL         string            // REQUIRED for password, clientCredentials, authorizationCode
	RefreshURL       string            // Optional URL for obtaining refresh tokens
	Scopes           map[string]string // REQUIRED. Map of scope name to description (can be empty)
	Extensions       map[string]any
}

// SecurityRequirement lists required security schemes for an operation.
type SecurityRequirement map[string][]string

// Tag adds metadata to a tag.
type Tag struct {
	Name         string
	Description  string
	ExternalDocs *ExternalDocs
	Extensions   map[string]any
}

// ExternalDocs provides external documentation links.
type ExternalDocs struct {
	Description string
	URL         string
	Extensions  map[string]any
}
