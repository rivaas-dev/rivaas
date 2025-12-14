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
	"encoding/json"
	"maps"
)

// SpecV31 represents an OpenAPI 3.1.x specification.
type SpecV31 struct {
	OpenAPI           string `json:"openapi"`
	JSONSchemaDialect string `json:"jsonSchemaDialect,omitempty"`
	// Info is a pointer to ensure custom MarshalJSON methods are invoked
	// for nested structs when the parent struct uses type aliases to avoid
	// infinite recursion in MarshalJSON implementations.
	Info         *InfoV31                 `json:"info"`
	Servers      []ServerV31              `json:"servers,omitempty"`
	Paths        map[string]*PathItemV31  `json:"paths,omitempty"`
	Components   *ComponentsV31           `json:"components,omitempty"`
	Webhooks     map[string]*PathItemV31  `json:"webhooks,omitempty"`
	Tags         []TagV31                 `json:"tags,omitempty"`
	Security     []SecurityRequirementV31 `json:"security,omitempty"`
	ExternalDocs *ExternalDocsV31         `json:"externalDocs,omitempty"`
	Extensions   map[string]any           `json:"-"`
}

// InfoV31 provides metadata about the API.
type InfoV31 struct {
	Title          string         `json:"title"`
	Summary        string         `json:"summary,omitempty"`
	Description    string         `json:"description,omitempty"`
	TermsOfService string         `json:"termsOfService,omitempty"`
	Version        string         `json:"version"`
	Contact        *ContactV31    `json:"contact,omitempty"`
	License        *LicenseV31    `json:"license,omitempty"`
	Extensions     map[string]any `json:"-"`
}

// ContactV31 provides contact information.
type ContactV31 struct {
	Name       string         `json:"name,omitempty"`
	URL        string         `json:"url,omitempty"`
	Email      string         `json:"email,omitempty"`
	Extensions map[string]any `json:"-"`
}

// LicenseV31 provides license information.
type LicenseV31 struct {
	Name       string         `json:"name"`                 // REQUIRED. The license name used for the API.
	Identifier string         `json:"identifier,omitempty"` // SPDX license expression. Mutually exclusive with url.
	URL        string         `json:"url,omitempty"`        // A URI for the license. Mutually exclusive with identifier.
	Extensions map[string]any `json:"-"`
}

// ServerV31 represents a server URL.
type ServerV31 struct {
	URL         string                        `json:"url"`
	Description string                        `json:"description,omitempty"`
	Variables   map[string]*ServerVariableV31 `json:"variables,omitempty"`
	Extensions  map[string]any                `json:"-"`
}

// ServerVariableV31 represents a server variable for URL template substitution.
type ServerVariableV31 struct {
	Enum        []string       `json:"enum,omitempty"`        // Enumeration of allowed values (MUST NOT be empty)
	Default     string         `json:"default"`               // REQUIRED. Default value for substitution
	Description string         `json:"description,omitempty"` // Optional description
	Extensions  map[string]any `json:"-"`
}

// PathItemV31 represents operations on a path.
type PathItemV31 struct {
	Ref         string         `json:"$ref,omitempty"`
	Summary     string         `json:"summary,omitempty"`
	Description string         `json:"description,omitempty"`
	Get         *OperationV31  `json:"get,omitempty"`
	Put         *OperationV31  `json:"put,omitempty"`
	Post        *OperationV31  `json:"post,omitempty"`
	Delete      *OperationV31  `json:"delete,omitempty"`
	Options     *OperationV31  `json:"options,omitempty"`
	Head        *OperationV31  `json:"head,omitempty"`
	Patch       *OperationV31  `json:"patch,omitempty"`
	Trace       *OperationV31  `json:"trace,omitempty"`
	Parameters  []ParameterV31 `json:"parameters,omitempty"`
	Extensions  map[string]any `json:"-"`
}

// OperationV31 describes an API operation.
type OperationV31 struct {
	Tags         []string                 `json:"tags,omitempty"`
	Summary      string                   `json:"summary,omitempty"`
	Description  string                   `json:"description,omitempty"`
	ExternalDocs *ExternalDocsV31         `json:"externalDocs,omitempty"`
	OperationID  string                   `json:"operationId,omitempty"`
	Parameters   []ParameterV31           `json:"parameters,omitempty"`
	RequestBody  *RequestBodyV31          `json:"requestBody,omitempty"`
	Responses    map[string]*ResponseV31  `json:"responses"`
	Callbacks    map[string]*CallbackV31  `json:"callbacks,omitempty"`
	Deprecated   bool                     `json:"deprecated,omitempty"`
	Security     []SecurityRequirementV31 `json:"security,omitempty"`
	Servers      []ServerV31              `json:"servers,omitempty"`
	Extensions   map[string]any           `json:"-"`
}

// ParameterV31 describes a parameter.
type ParameterV31 struct {
	Ref             string                   `json:"$ref,omitempty"`
	Name            string                   `json:"name,omitempty"`
	In              string                   `json:"in,omitempty"`
	Description     string                   `json:"description,omitempty"`
	Required        bool                     `json:"required,omitempty"`
	Deprecated      bool                     `json:"deprecated,omitempty"`
	AllowEmptyValue bool                     `json:"allowEmptyValue,omitempty"`
	Style           string                   `json:"style,omitempty"`
	Explode         bool                     `json:"explode,omitempty"`
	AllowReserved   bool                     `json:"allowReserved,omitempty"`
	Schema          *SchemaV31               `json:"schema,omitempty"`
	Example         any                      `json:"example,omitempty"`
	Examples        map[string]*ExampleV31   `json:"examples,omitempty"`
	Content         map[string]*MediaTypeV31 `json:"content,omitempty"`
	Extensions      map[string]any           `json:"-"`
}

// ExampleV31 represents an example.
type ExampleV31 struct {
	Ref           string         `json:"$ref,omitempty"`
	Summary       string         `json:"summary,omitempty"`
	Description   string         `json:"description,omitempty"`
	Value         any            `json:"value,omitempty"`
	ExternalValue string         `json:"externalValue,omitempty"`
	Extensions    map[string]any `json:"-"`
}

// RequestBodyV31 describes a request body.
type RequestBodyV31 struct {
	Ref         string                   `json:"$ref,omitempty"`
	Description string                   `json:"description,omitempty"`
	Required    bool                     `json:"required,omitempty"`
	Content     map[string]*MediaTypeV31 `json:"content,omitempty"`
	Extensions  map[string]any           `json:"-"`
}

// ResponseV31 describes a response.
type ResponseV31 struct {
	Ref         string                   `json:"$ref,omitempty"`
	Description string                   `json:"description,omitempty"`
	Content     map[string]*MediaTypeV31 `json:"content,omitempty"`
	Headers     map[string]*HeaderV31    `json:"headers,omitempty"`
	Links       map[string]*LinkV31      `json:"links,omitempty"`
	Extensions  map[string]any           `json:"-"`
}

// HeaderV31 represents a response header.
type HeaderV31 struct {
	Ref             string                   `json:"$ref,omitempty"`
	Description     string                   `json:"description,omitempty"`
	Required        bool                     `json:"required,omitempty"`
	Deprecated      bool                     `json:"deprecated,omitempty"`
	AllowEmptyValue bool                     `json:"allowEmptyValue,omitempty"`
	Style           string                   `json:"style,omitempty"`
	Explode         bool                     `json:"explode,omitempty"`
	Schema          *SchemaV31               `json:"schema,omitempty"`
	Example         any                      `json:"example,omitempty"`
	Examples        map[string]*ExampleV31   `json:"examples,omitempty"`
	Content         map[string]*MediaTypeV31 `json:"content,omitempty"`
	Extensions      map[string]any           `json:"-"`
}

// MediaTypeV31 provides schema and examples.
type MediaTypeV31 struct {
	Schema     *SchemaV31              `json:"schema,omitempty"`
	Example    any                     `json:"example,omitempty"`
	Examples   map[string]*ExampleV31  `json:"examples,omitempty"`
	Encoding   map[string]*EncodingV31 `json:"encoding,omitempty"`
	Extensions map[string]any          `json:"-"`
}

// EncodingV31 describes encoding for a schema property.
type EncodingV31 struct {
	ContentType   string                `json:"contentType,omitempty"`
	Headers       map[string]*HeaderV31 `json:"headers,omitempty"`
	Style         string                `json:"style,omitempty"`
	Explode       bool                  `json:"explode,omitempty"`
	AllowReserved bool                  `json:"allowReserved,omitempty"`
	Extensions    map[string]any        `json:"-"`
}

// CallbackV31 represents a callback.
type CallbackV31 struct {
	Ref        string                  `json:"$ref,omitempty"`
	PathItems  map[string]*PathItemV31 `json:"-"`
	Extensions map[string]any          `json:"-"`
}

// LinkV31 represents a design-time link for a response.
type LinkV31 struct {
	Ref          string         `json:"$ref,omitempty"`
	OperationRef string         `json:"operationRef,omitempty"`
	OperationID  string         `json:"operationId,omitempty"`
	Parameters   map[string]any `json:"parameters,omitempty"`
	RequestBody  any            `json:"requestBody,omitempty"`
	Description  string         `json:"description,omitempty"`
	Server       *ServerV31     `json:"server,omitempty"`
	Extensions   map[string]any `json:"-"`
}

// ComponentsV31 holds reusable components.
type ComponentsV31 struct {
	Schemas         map[string]*SchemaV31         `json:"schemas,omitempty"`
	Responses       map[string]*ResponseV31       `json:"responses,omitempty"`
	Parameters      map[string]*ParameterV31      `json:"parameters,omitempty"`
	Examples        map[string]*ExampleV31        `json:"examples,omitempty"`
	RequestBodies   map[string]*RequestBodyV31    `json:"requestBodies,omitempty"`
	Headers         map[string]*HeaderV31         `json:"headers,omitempty"`
	SecuritySchemes map[string]*SecuritySchemeV31 `json:"securitySchemes,omitempty"`
	Links           map[string]*LinkV31           `json:"links,omitempty"`
	Callbacks       map[string]*CallbackV31       `json:"callbacks,omitempty"`
	PathItems       map[string]*PathItemV31       `json:"pathItems,omitempty"` // 3.1 only
	Extensions      map[string]any                `json:"-"`
}

// SecuritySchemeV31 defines a security scheme.
type SecuritySchemeV31 struct {
	Ref              string         `json:"$ref,omitempty"`
	Type             string         `json:"type,omitempty"`
	Description      string         `json:"description,omitempty"`
	Name             string         `json:"name,omitempty"`
	In               string         `json:"in,omitempty"`
	Scheme           string         `json:"scheme,omitempty"`
	BearerFormat     string         `json:"bearerFormat,omitempty"`
	Flows            *OAuthFlowsV31 `json:"flows,omitempty"`
	OpenIDConnectURL string         `json:"openIdConnectUrl,omitempty"`
	Extensions       map[string]any `json:"-"`
}

// OAuthFlowsV31 allows configuration of the supported OAuth Flows.
type OAuthFlowsV31 struct {
	Implicit          *OAuthFlowV31  `json:"implicit,omitempty"`
	Password          *OAuthFlowV31  `json:"password,omitempty"`
	ClientCredentials *OAuthFlowV31  `json:"clientCredentials,omitempty"`
	AuthorizationCode *OAuthFlowV31  `json:"authorizationCode,omitempty"`
	Extensions        map[string]any `json:"-"`
}

// OAuthFlowV31 contains configuration details for a supported OAuth Flow.
type OAuthFlowV31 struct {
	AuthorizationURL string            `json:"authorizationUrl,omitempty"`
	TokenURL         string            `json:"tokenUrl,omitempty"`
	RefreshURL       string            `json:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes"`
	Extensions       map[string]any    `json:"-"`
}

// SecurityRequirementV31 lists required security schemes.
type SecurityRequirementV31 map[string][]string

// TagV31 adds metadata to a tag.
type TagV31 struct {
	Name         string           `json:"name"`
	Description  string           `json:"description,omitempty"`
	ExternalDocs *ExternalDocsV31 `json:"externalDocs,omitempty"`
	Extensions   map[string]any   `json:"-"`
}

// ExternalDocsV31 provides external documentation.
type ExternalDocsV31 struct {
	Description string         `json:"description,omitempty"`
	URL         string         `json:"url"`
	Extensions  map[string]any `json:"-"`
}

// MarshalJSON implements json.Marshaler for SpecV31 to inline extensions.
func (s *SpecV31) MarshalJSON() ([]byte, error) {
	type specV31 SpecV31
	return marshalWithExtensions(specV31(*s), s.Extensions)
}

// MarshalJSON implements json.Marshaler for InfoV31 to inline extensions.
func (i *InfoV31) MarshalJSON() ([]byte, error) {
	type infoV31 InfoV31
	return marshalWithExtensions(infoV31(*i), i.Extensions)
}

// MarshalJSON implements json.Marshaler for ContactV31 to inline extensions.
func (c *ContactV31) MarshalJSON() ([]byte, error) {
	type contactV31 ContactV31
	return marshalWithExtensions(contactV31(*c), c.Extensions)
}

// MarshalJSON implements json.Marshaler for LicenseV31 to inline extensions.
func (l *LicenseV31) MarshalJSON() ([]byte, error) {
	type licenseV31 LicenseV31
	return marshalWithExtensions(licenseV31(*l), l.Extensions)
}

// MarshalJSON implements json.Marshaler for ServerV31 to inline extensions.
func (s *ServerV31) MarshalJSON() ([]byte, error) {
	type serverV31 ServerV31
	return marshalWithExtensions(serverV31(*s), s.Extensions)
}

// MarshalJSON implements json.Marshaler for ServerVariableV31 to inline extensions.
func (s *ServerVariableV31) MarshalJSON() ([]byte, error) {
	type serverVariableV31 ServerVariableV31
	return marshalWithExtensions(serverVariableV31(*s), s.Extensions)
}

// MarshalJSON implements json.Marshaler for PathItemV31 to inline extensions.
func (p *PathItemV31) MarshalJSON() ([]byte, error) {
	type pathItemV31 PathItemV31
	return marshalWithExtensions(pathItemV31(*p), p.Extensions)
}

// MarshalJSON implements json.Marshaler for OperationV31 to inline extensions.
func (o *OperationV31) MarshalJSON() ([]byte, error) {
	type operationV31 OperationV31
	return marshalWithExtensions(operationV31(*o), o.Extensions)
}

// MarshalJSON implements json.Marshaler for ParameterV31 to inline extensions.
func (p *ParameterV31) MarshalJSON() ([]byte, error) {
	type parameterV31 ParameterV31
	return marshalWithExtensions(parameterV31(*p), p.Extensions)
}

// MarshalJSON implements json.Marshaler for ExampleV31 to inline extensions.
func (e *ExampleV31) MarshalJSON() ([]byte, error) {
	type exampleV31 ExampleV31
	return marshalWithExtensions(exampleV31(*e), e.Extensions)
}

// MarshalJSON implements json.Marshaler for RequestBodyV31 to inline extensions.
func (r *RequestBodyV31) MarshalJSON() ([]byte, error) {
	type requestBodyV31 RequestBodyV31
	return marshalWithExtensions(requestBodyV31(*r), r.Extensions)
}

// MarshalJSON implements json.Marshaler for ResponseV31 to inline extensions.
func (r *ResponseV31) MarshalJSON() ([]byte, error) {
	type responseV31 ResponseV31
	return marshalWithExtensions(responseV31(*r), r.Extensions)
}

// MarshalJSON implements json.Marshaler for HeaderV31 to inline extensions.
func (h *HeaderV31) MarshalJSON() ([]byte, error) {
	type headerV31 HeaderV31
	return marshalWithExtensions(headerV31(*h), h.Extensions)
}

// MarshalJSON implements json.Marshaler for MediaTypeV31 to inline extensions.
func (m *MediaTypeV31) MarshalJSON() ([]byte, error) {
	type mediaTypeV31 MediaTypeV31
	return marshalWithExtensions(mediaTypeV31(*m), m.Extensions)
}

// MarshalJSON implements json.Marshaler for EncodingV31 to inline extensions.
func (e *EncodingV31) MarshalJSON() ([]byte, error) {
	type encodingV31 EncodingV31
	return marshalWithExtensions(encodingV31(*e), e.Extensions)
}

// MarshalJSON implements json.Marshaler for CallbackV31.
// Callbacks are maps of path expressions to PathItems, so PathItems become the top-level keys.
func (c *CallbackV31) MarshalJSON() ([]byte, error) {
	// Start with PathItems as the base map
	m := make(map[string]any, len(c.PathItems)+len(c.Extensions))
	for k, v := range c.PathItems {
		m[k] = v
	}
	// Merge extensions
	maps.Copy(m, c.Extensions)

	return json.Marshal(m)
}

// MarshalJSON implements json.Marshaler for LinkV31 to inline extensions.
func (l *LinkV31) MarshalJSON() ([]byte, error) {
	type linkV31 LinkV31
	return marshalWithExtensions(linkV31(*l), l.Extensions)
}

// MarshalJSON implements json.Marshaler for ComponentsV31 to inline extensions.
func (c *ComponentsV31) MarshalJSON() ([]byte, error) {
	type componentsV31 ComponentsV31
	return marshalWithExtensions(componentsV31(*c), c.Extensions)
}

// MarshalJSON implements json.Marshaler for SecuritySchemeV31 to inline extensions.
func (s *SecuritySchemeV31) MarshalJSON() ([]byte, error) {
	type securitySchemeV31 SecuritySchemeV31
	return marshalWithExtensions(securitySchemeV31(*s), s.Extensions)
}

// MarshalJSON implements json.Marshaler for OAuthFlowsV31 to inline extensions.
func (o *OAuthFlowsV31) MarshalJSON() ([]byte, error) {
	type oauthFlowsV31 OAuthFlowsV31
	return marshalWithExtensions(oauthFlowsV31(*o), o.Extensions)
}

// MarshalJSON implements json.Marshaler for OAuthFlowV31 to inline extensions.
func (o *OAuthFlowV31) MarshalJSON() ([]byte, error) {
	type oauthFlowV31 OAuthFlowV31
	return marshalWithExtensions(oauthFlowV31(*o), o.Extensions)
}

// MarshalJSON implements json.Marshaler for TagV31 to inline extensions.
func (t *TagV31) MarshalJSON() ([]byte, error) {
	type tagV31 TagV31
	return marshalWithExtensions(tagV31(*t), t.Extensions)
}

// MarshalJSON implements json.Marshaler for ExternalDocsV31 to inline extensions.
func (e *ExternalDocsV31) MarshalJSON() ([]byte, error) {
	type externalDocsV31 ExternalDocsV31
	return marshalWithExtensions(externalDocsV31(*e), e.Extensions)
}
