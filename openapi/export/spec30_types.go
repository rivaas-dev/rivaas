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

// SpecV30 represents an OpenAPI 3.0.4 specification.
// This type is defined here to avoid import cycles with the openapi package.
type SpecV30 struct {
	OpenAPI string `json:"openapi"`
	// Info is a pointer to ensure custom MarshalJSON methods are invoked
	// for nested structs when the parent struct uses type aliases to avoid
	// infinite recursion in MarshalJSON implementations.
	Info         *InfoV30                 `json:"info"`
	Servers      []ServerV30              `json:"servers,omitempty"`
	Paths        map[string]*PathItemV30  `json:"paths"`
	Components   *ComponentsV30           `json:"components,omitempty"`
	Tags         []TagV30                 `json:"tags,omitempty"`
	Security     []SecurityRequirementV30 `json:"security,omitempty"`
	ExternalDocs *ExternalDocsV30         `json:"externalDocs,omitempty"`
	Extensions   map[string]any           `json:"-"`
}

// InfoV30 provides metadata about the API.
type InfoV30 struct {
	Title          string         `json:"title"`
	Description    string         `json:"description,omitempty"`
	TermsOfService string         `json:"termsOfService,omitempty"`
	Version        string         `json:"version"`
	Contact        *ContactV30    `json:"contact,omitempty"`
	License        *LicenseV30    `json:"license,omitempty"`
	Extensions     map[string]any `json:"-"`
}

// ContactV30 provides contact information.
type ContactV30 struct {
	Name       string         `json:"name,omitempty"`
	URL        string         `json:"url,omitempty"`
	Email      string         `json:"email,omitempty"`
	Extensions map[string]any `json:"-"`
}

// LicenseV30 provides license information.
type LicenseV30 struct {
	Name       string         `json:"name"`
	URL        string         `json:"url,omitempty"`
	Extensions map[string]any `json:"-"`
}

// ServerV30 represents a server URL.
type ServerV30 struct {
	URL         string                        `json:"url"`
	Description string                        `json:"description,omitempty"`
	Variables   map[string]*ServerVariableV30 `json:"variables,omitempty"`
	Extensions  map[string]any                `json:"-"`
}

// ServerVariableV30 represents a server variable for URL template substitution.
type ServerVariableV30 struct {
	Enum        []string       `json:"enum,omitempty"`        // Enumeration of allowed values (SHOULD NOT be empty)
	Default     string         `json:"default"`               // REQUIRED. Default value for substitution
	Description string         `json:"description,omitempty"` // Optional description
	Extensions  map[string]any `json:"-"`
}

// PathItemV30 represents the operations available on a single path.
type PathItemV30 struct {
	Summary     string         `json:"summary,omitempty"`
	Description string         `json:"description,omitempty"`
	Get         *OperationV30  `json:"get,omitempty"`
	Put         *OperationV30  `json:"put,omitempty"`
	Post        *OperationV30  `json:"post,omitempty"`
	Delete      *OperationV30  `json:"delete,omitempty"`
	Options     *OperationV30  `json:"options,omitempty"`
	Head        *OperationV30  `json:"head,omitempty"`
	Patch       *OperationV30  `json:"patch,omitempty"`
	Parameters  []ParameterV30 `json:"parameters,omitempty"`
	Extensions  map[string]any `json:"-"`
}

// OperationV30 describes a single API operation.
type OperationV30 struct {
	Tags        []string                 `json:"tags,omitempty"`
	Summary     string                   `json:"summary,omitempty"`
	Description string                   `json:"description,omitempty"`
	OperationID string                   `json:"operationId,omitempty"`
	Parameters  []ParameterV30           `json:"parameters,omitempty"`
	RequestBody *RequestBodyV30          `json:"requestBody,omitempty"`
	Responses   map[string]*ResponseV30  `json:"responses"`
	Deprecated  bool                     `json:"deprecated,omitempty"`
	Security    []SecurityRequirementV30 `json:"security,omitempty"`
	Extensions  map[string]any           `json:"-"`
}

// ParameterV30 describes a single operation parameter.
type ParameterV30 struct {
	Name            string                   `json:"name"`
	In              string                   `json:"in"`
	Description     string                   `json:"description,omitempty"`
	Required        bool                     `json:"required,omitempty"`
	Deprecated      bool                     `json:"deprecated,omitempty"`
	AllowEmptyValue bool                     `json:"allowEmptyValue,omitempty"`
	Style           string                   `json:"style,omitempty"`
	Explode         bool                     `json:"explode,omitempty"`
	AllowReserved   bool                     `json:"allowReserved,omitempty"`
	Schema          *SchemaV30               `json:"schema,omitempty"`
	Example         any                      `json:"example,omitempty"`
	Examples        map[string]*ExampleV30   `json:"examples,omitempty"`
	Content         map[string]*MediaTypeV30 `json:"content,omitempty"`
	Extensions      map[string]any           `json:"-"`
}

// RequestBodyV30 describes a single request body.
type RequestBodyV30 struct {
	Description string                   `json:"description,omitempty"`
	Required    bool                     `json:"required,omitempty"`
	Content     map[string]*MediaTypeV30 `json:"content"`
	Extensions  map[string]any           `json:"-"`
}

// ResponseV30 describes a single response.
type ResponseV30 struct {
	Description string                   `json:"description"`
	Content     map[string]*MediaTypeV30 `json:"content,omitempty"`
	Headers     map[string]*HeaderV30    `json:"headers,omitempty"`
	Links       map[string]*LinkV30      `json:"links,omitempty"`
	Extensions  map[string]any           `json:"-"`
}

// HeaderV30 represents a response header.
type HeaderV30 struct {
	Description string                   `json:"description,omitempty"`
	Required    bool                     `json:"required,omitempty"`
	Deprecated  bool                     `json:"deprecated,omitempty"`
	Style       string                   `json:"style,omitempty"`
	Explode     bool                     `json:"explode,omitempty"`
	Schema      *SchemaV30               `json:"schema,omitempty"`
	Example     any                      `json:"example,omitempty"`
	Examples    map[string]*ExampleV30   `json:"examples,omitempty"`
	Content     map[string]*MediaTypeV30 `json:"content,omitempty"`
	Extensions  map[string]any           `json:"-"`
}

// MediaTypeV30 provides schema and examples for a specific content type.
type MediaTypeV30 struct {
	Schema     *SchemaV30              `json:"schema,omitempty"`
	Example    any                     `json:"example,omitempty"`
	Examples   map[string]*ExampleV30  `json:"examples,omitempty"`
	Encoding   map[string]*EncodingV30 `json:"encoding,omitempty"`
	Extensions map[string]any          `json:"-"`
}

// ExampleV30 represents an example value.
type ExampleV30 struct {
	Summary       string         `json:"summary,omitempty"`
	Description   string         `json:"description,omitempty"`
	Value         any            `json:"value,omitempty"`
	ExternalValue string         `json:"externalValue,omitempty"`
	Extensions    map[string]any `json:"-"`
}

// EncodingV30 describes encoding for a schema property.
type EncodingV30 struct {
	ContentType   string                `json:"contentType,omitempty"`
	Headers       map[string]*HeaderV30 `json:"headers,omitempty"`
	Style         string                `json:"style,omitempty"`
	Explode       bool                  `json:"explode,omitempty"`
	AllowReserved bool                  `json:"allowReserved,omitempty"`
	Extensions    map[string]any        `json:"-"`
}

// LinkV30 represents a design-time link for a response.
type LinkV30 struct {
	OperationRef string         `json:"operationRef,omitempty"`
	OperationId  string         `json:"operationId,omitempty"`
	Parameters   map[string]any `json:"parameters,omitempty"`
	RequestBody  any            `json:"requestBody,omitempty"`
	Description  string         `json:"description,omitempty"`
	Server       *ServerV30     `json:"server,omitempty"`
	Extensions   map[string]any `json:"-"`
}

// ComponentsV30 holds reusable components.
type ComponentsV30 struct {
	Schemas         map[string]*SchemaV30         `json:"schemas,omitempty"`
	SecuritySchemes map[string]*SecuritySchemeV30 `json:"securitySchemes,omitempty"`
	Extensions      map[string]any                `json:"-"`
}

// SecuritySchemeV30 defines a security scheme.
type SecuritySchemeV30 struct {
	Type             string         `json:"type"`
	Description      string         `json:"description,omitempty"`
	Name             string         `json:"name,omitempty"`
	In               string         `json:"in,omitempty"`
	Scheme           string         `json:"scheme,omitempty"`
	BearerFormat     string         `json:"bearerFormat,omitempty"`
	Flows            *OAuthFlowsV30 `json:"flows,omitempty"`
	OpenIdConnectUrl string         `json:"openIdConnectUrl,omitempty"`
	Extensions       map[string]any `json:"-"`
}

// OAuthFlowsV30 allows configuration of the supported OAuth Flows.
type OAuthFlowsV30 struct {
	Implicit          *OAuthFlowV30  `json:"implicit,omitempty"`
	Password          *OAuthFlowV30  `json:"password,omitempty"`
	ClientCredentials *OAuthFlowV30  `json:"clientCredentials,omitempty"`
	AuthorizationCode *OAuthFlowV30  `json:"authorizationCode,omitempty"`
	Extensions        map[string]any `json:"-"`
}

// OAuthFlowV30 contains configuration details for a supported OAuth Flow.
type OAuthFlowV30 struct {
	AuthorizationUrl string            `json:"authorizationUrl,omitempty"`
	TokenUrl         string            `json:"tokenUrl,omitempty"`
	RefreshUrl       string            `json:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes"`
	Extensions       map[string]any    `json:"-"`
}

// SecurityRequirementV30 lists the required security schemes for an operation.
type SecurityRequirementV30 map[string][]string

// TagV30 adds metadata to a single tag.
type TagV30 struct {
	Name         string           `json:"name"`
	Description  string           `json:"description,omitempty"`
	ExternalDocs *ExternalDocsV30 `json:"externalDocs,omitempty"`
	Extensions   map[string]any   `json:"-"`
}

// ExternalDocsV30 provides external documentation.
type ExternalDocsV30 struct {
	Description string         `json:"description,omitempty"`
	URL         string         `json:"url"`
	Extensions  map[string]any `json:"-"`
}

// MarshalJSON implements json.Marshaler for SpecV30 to inline extensions.
func (s *SpecV30) MarshalJSON() ([]byte, error) {
	// Use type alias to avoid infinite recursion
	type specV30 SpecV30
	return marshalWithExtensions(specV30(*s), s.Extensions)
}

// MarshalJSON implements json.Marshaler for InfoV30 to inline extensions.
func (i *InfoV30) MarshalJSON() ([]byte, error) {
	type infoV30 InfoV30
	return marshalWithExtensions(infoV30(*i), i.Extensions)
}

// MarshalJSON implements json.Marshaler for ContactV30 to inline extensions.
func (c *ContactV30) MarshalJSON() ([]byte, error) {
	type contactV30 ContactV30
	return marshalWithExtensions(contactV30(*c), c.Extensions)
}

// MarshalJSON implements json.Marshaler for LicenseV30 to inline extensions.
func (l *LicenseV30) MarshalJSON() ([]byte, error) {
	type licenseV30 LicenseV30
	return marshalWithExtensions(licenseV30(*l), l.Extensions)
}

// MarshalJSON implements json.Marshaler for ServerV30 to inline extensions.
func (s *ServerV30) MarshalJSON() ([]byte, error) {
	type serverV30 ServerV30
	return marshalWithExtensions(serverV30(*s), s.Extensions)
}

// MarshalJSON implements json.Marshaler for ServerVariableV30 to inline extensions.
func (s *ServerVariableV30) MarshalJSON() ([]byte, error) {
	type serverVariableV30 ServerVariableV30
	return marshalWithExtensions(serverVariableV30(*s), s.Extensions)
}

// MarshalJSON implements json.Marshaler for PathItemV30 to inline extensions.
func (p *PathItemV30) MarshalJSON() ([]byte, error) {
	type pathItemV30 PathItemV30
	return marshalWithExtensions(pathItemV30(*p), p.Extensions)
}

// MarshalJSON implements json.Marshaler for OperationV30 to inline extensions.
func (o *OperationV30) MarshalJSON() ([]byte, error) {
	type operationV30 OperationV30
	return marshalWithExtensions(operationV30(*o), o.Extensions)
}

// MarshalJSON implements json.Marshaler for ParameterV30 to inline extensions.
func (p *ParameterV30) MarshalJSON() ([]byte, error) {
	type parameterV30 ParameterV30
	return marshalWithExtensions(parameterV30(*p), p.Extensions)
}

// MarshalJSON implements json.Marshaler for RequestBodyV30 to inline extensions.
func (r *RequestBodyV30) MarshalJSON() ([]byte, error) {
	type requestBodyV30 RequestBodyV30
	return marshalWithExtensions(requestBodyV30(*r), r.Extensions)
}

// MarshalJSON implements json.Marshaler for ResponseV30 to inline extensions.
func (r *ResponseV30) MarshalJSON() ([]byte, error) {
	type responseV30 ResponseV30
	return marshalWithExtensions(responseV30(*r), r.Extensions)
}

// MarshalJSON implements json.Marshaler for HeaderV30 to inline extensions.
func (h *HeaderV30) MarshalJSON() ([]byte, error) {
	type headerV30 HeaderV30
	return marshalWithExtensions(headerV30(*h), h.Extensions)
}

// MarshalJSON implements json.Marshaler for MediaTypeV30 to inline extensions.
func (m *MediaTypeV30) MarshalJSON() ([]byte, error) {
	type mediaTypeV30 MediaTypeV30
	return marshalWithExtensions(mediaTypeV30(*m), m.Extensions)
}

// MarshalJSON implements json.Marshaler for ExampleV30 to inline extensions.
func (e *ExampleV30) MarshalJSON() ([]byte, error) {
	type exampleV30 ExampleV30
	return marshalWithExtensions(exampleV30(*e), e.Extensions)
}

// MarshalJSON implements json.Marshaler for EncodingV30 to inline extensions.
func (e *EncodingV30) MarshalJSON() ([]byte, error) {
	type encodingV30 EncodingV30
	return marshalWithExtensions(encodingV30(*e), e.Extensions)
}

// MarshalJSON implements json.Marshaler for LinkV30 to inline extensions.
func (l *LinkV30) MarshalJSON() ([]byte, error) {
	type linkV30 LinkV30
	return marshalWithExtensions(linkV30(*l), l.Extensions)
}

// MarshalJSON implements json.Marshaler for ComponentsV30 to inline extensions.
func (c *ComponentsV30) MarshalJSON() ([]byte, error) {
	type componentsV30 ComponentsV30
	return marshalWithExtensions(componentsV30(*c), c.Extensions)
}

// MarshalJSON implements json.Marshaler for SecuritySchemeV30 to inline extensions.
func (s *SecuritySchemeV30) MarshalJSON() ([]byte, error) {
	type securitySchemeV30 SecuritySchemeV30
	return marshalWithExtensions(securitySchemeV30(*s), s.Extensions)
}

// MarshalJSON implements json.Marshaler for OAuthFlowsV30 to inline extensions.
func (o *OAuthFlowsV30) MarshalJSON() ([]byte, error) {
	type oauthFlowsV30 OAuthFlowsV30
	return marshalWithExtensions(oauthFlowsV30(*o), o.Extensions)
}

// MarshalJSON implements json.Marshaler for OAuthFlowV30 to inline extensions.
func (o *OAuthFlowV30) MarshalJSON() ([]byte, error) {
	type oauthFlowV30 OAuthFlowV30
	return marshalWithExtensions(oauthFlowV30(*o), o.Extensions)
}

// MarshalJSON implements json.Marshaler for TagV30 to inline extensions.
func (t *TagV30) MarshalJSON() ([]byte, error) {
	type tagV30 TagV30
	return marshalWithExtensions(tagV30(*t), t.Extensions)
}

// MarshalJSON implements json.Marshaler for ExternalDocsV30 to inline extensions.
func (e *ExternalDocsV30) MarshalJSON() ([]byte, error) {
	type externalDocsV30 ExternalDocsV30
	return marshalWithExtensions(externalDocsV30(*e), e.Extensions)
}
