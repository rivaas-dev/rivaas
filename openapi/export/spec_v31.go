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
	"fmt"
	"strconv"

	"rivaas.dev/openapi/model"
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
	Summary     string         `json:"summary,omitempty"`
	Description string         `json:"description,omitempty"`
	Get         *OperationV31  `json:"get,omitempty"`
	Put         *OperationV31  `json:"put,omitempty"`
	Post        *OperationV31  `json:"post,omitempty"`
	Delete      *OperationV31  `json:"delete,omitempty"`
	Options     *OperationV31  `json:"options,omitempty"`
	Head        *OperationV31  `json:"head,omitempty"`
	Patch       *OperationV31  `json:"patch,omitempty"`
	Parameters  []ParameterV31 `json:"parameters,omitempty"`
	Extensions  map[string]any `json:"-"`
}

// OperationV31 describes an API operation.
type OperationV31 struct {
	Tags        []string                 `json:"tags,omitempty"`
	Summary     string                   `json:"summary,omitempty"`
	Description string                   `json:"description,omitempty"`
	OperationID string                   `json:"operationId,omitempty"`
	Parameters  []ParameterV31           `json:"parameters,omitempty"`
	RequestBody *RequestBodyV31          `json:"requestBody,omitempty"`
	Responses   map[string]*ResponseV31  `json:"responses"`
	Callbacks   map[string]*CallbackV31  `json:"callbacks,omitempty"`
	Deprecated  bool                     `json:"deprecated,omitempty"`
	Security    []SecurityRequirementV31 `json:"security,omitempty"`
	Extensions  map[string]any           `json:"-"`
}

// ParameterV31 describes a parameter.
type ParameterV31 struct {
	Name            string                   `json:"name"`
	In              string                   `json:"in"`
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
	Summary       string         `json:"summary,omitempty"`
	Description   string         `json:"description,omitempty"`
	Value         any            `json:"value,omitempty"`
	ExternalValue string         `json:"externalValue,omitempty"`
	Extensions    map[string]any `json:"-"`
}

// RequestBodyV31 describes a request body.
type RequestBodyV31 struct {
	Description string                   `json:"description,omitempty"`
	Required    bool                     `json:"required,omitempty"`
	Content     map[string]*MediaTypeV31 `json:"content"`
	Extensions  map[string]any           `json:"-"`
}

// ResponseV31 describes a response.
type ResponseV31 struct {
	Description string                   `json:"description"`
	Content     map[string]*MediaTypeV31 `json:"content,omitempty"`
	Headers     map[string]*HeaderV31    `json:"headers,omitempty"`
	Links       map[string]*LinkV31      `json:"links,omitempty"`
	Extensions  map[string]any           `json:"-"`
}

// HeaderV31 represents a response header.
type HeaderV31 struct {
	Description string                   `json:"description,omitempty"`
	Required    bool                     `json:"required,omitempty"`
	Deprecated  bool                     `json:"deprecated,omitempty"`
	Style       string                   `json:"style,omitempty"`
	Explode     bool                     `json:"explode,omitempty"`
	Schema      *SchemaV31               `json:"schema,omitempty"`
	Example     any                      `json:"example,omitempty"`
	Examples    map[string]*ExampleV31   `json:"examples,omitempty"`
	Content     map[string]*MediaTypeV31 `json:"content,omitempty"`
	Extensions  map[string]any           `json:"-"`
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
	PathItems  map[string]*PathItemV31 `json:"-"`
	Extensions map[string]any          `json:"-"`
}

// LinkV31 represents a design-time link for a response.
type LinkV31 struct {
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
	SecuritySchemes map[string]*SecuritySchemeV31 `json:"securitySchemes,omitempty"`
	Extensions      map[string]any                `json:"-"`
}

// SecuritySchemeV31 defines a security scheme.
type SecuritySchemeV31 struct {
	Type             string         `json:"type"`
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
	for k, v := range c.Extensions {
		m[k] = v
	}
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

// projectTo31 projects a spec to OpenAPI 3.1.x format.
func projectTo31(in *model.Spec) (*SpecV31, []Warning, error) {
	warns := []Warning{}

	// 3.1 requires any of paths|components|webhooks (we assume builder guarantees at least one)
	info, err := info31(in.Info)
	if err != nil {
		return nil, warns, err
	}
	out := &SpecV31{
		OpenAPI:           "3.1.2",
		JSONSchemaDialect: "https://spec.openapis.org/oas/3.1/dialect/2024-11-10",
		Info:              &info,
		Servers:           servers31(in.Servers, &warns),
		Paths:             paths31(in.Paths, &warns),
		Tags:              tags31(in.Tags),
	}

	if in.Components != nil {
		out.Components = components31(in.Components, &warns)
	}

	if len(in.Webhooks) > 0 {
		out.Webhooks = webhooks31(in.Webhooks, &warns)
	}

	if len(in.Security) > 0 {
		out.Security = security31(in.Security)
	}

	if in.ExternalDocs != nil {
		out.ExternalDocs = externalDocs31(in.ExternalDocs)
	}

	// Copy extensions
	out.Extensions = copyExtensions(in.Extensions, "3.1.2")

	return out, warns, nil
}

func info31(in model.Info) (InfoV31, error) {
	info := InfoV31{
		Title:          in.Title,
		Summary:        in.Summary,
		Description:    in.Description,
		TermsOfService: in.TermsOfService,
		Version:        in.Version,
	}
	if in.Contact != nil {
		info.Contact = &ContactV31{
			Name:  in.Contact.Name,
			URL:   in.Contact.URL,
			Email: in.Contact.Email,
		}
		info.Contact.Extensions = copyExtensions(in.Contact.Extensions, "3.1.2")
	}
	if in.License != nil {
		// Enforce mutual exclusivity - this should never happen with proper API usage
		// but serves as a defensive check for manually constructed License structs
		if in.License.Identifier != "" && in.License.URL != "" {
			return InfoV31{}, fmt.Errorf("license identifier and url are mutually exclusive per OpenAPI spec - both fields cannot be set")
		}
		info.License = &LicenseV31{
			Name:       in.License.Name,
			Identifier: in.License.Identifier,
			URL:        in.License.URL,
		}
		info.License.Extensions = copyExtensions(in.License.Extensions, "3.1.2")
	}
	info.Extensions = copyExtensions(in.Extensions, "3.1.2")
	return info, nil
}

func servers31(in []model.Server, warns *[]Warning) []ServerV31 {
	// 3.1: inject default if empty
	if len(in) == 0 {
		return []ServerV31{{URL: "/"}}
	}
	out := make([]ServerV31, len(in))
	for i, s := range in {
		server := ServerV31{
			URL:         s.URL,
			Description: s.Description,
		}
		if len(s.Variables) > 0 {
			server.Variables = make(map[string]*ServerVariableV31, len(s.Variables))
			for name, v := range s.Variables {
				// 3.1 validation: enum MUST NOT be empty if present
				// With omitempty, empty slices won't be serialized, but we still validate
				// that if enum is provided, it must not be empty per 3.1 spec
				// Note: In Go, len(nil slice) == 0, so we check length > 0 for enum validation
				if len(v.Enum) == 0 {
					// Warn if enum is explicitly set to empty (would violate spec if serialized)
					// With omitempty, empty enum won't be in JSON, but conceptually it's wrong
					*warns = append(*warns, Warning{
						Code:    "SERVER_VARIABLE_EMPTY_ENUM",
						Path:    "#/servers/" + strconv.Itoa(i) + "/variables/" + name,
						Message: "server variable enum array MUST NOT be empty in OpenAPI 3.1 (if enum is provided, it must contain at least one value)",
					})
				}
				// 3.1 validation: default MUST exist in enum if enum is defined
				if len(v.Enum) > 0 {
					found := false
					for _, val := range v.Enum {
						if val == v.Default {
							found = true
							break
						}
					}
					if !found {
						*warns = append(*warns, Warning{
							Code:    "SERVER_VARIABLE_DEFAULT_NOT_IN_ENUM",
							Path:    "#/servers/" + strconv.Itoa(i) + "/variables/" + name,
							Message: "server variable default value MUST exist in enum values in OpenAPI 3.1",
						})
					}
				}
				server.Variables[name] = &ServerVariableV31{
					Enum:        v.Enum,
					Default:     v.Default,
					Description: v.Description,
				}
				server.Variables[name].Extensions = copyExtensions(v.Extensions, "3.1.2")
			}
		}
		server.Extensions = copyExtensions(s.Extensions, "3.1.2")
		out[i] = server
	}
	return out
}

func tags31(in []model.Tag) []TagV31 {
	out := make([]TagV31, len(in))
	for i, t := range in {
		tag := TagV31{
			Name:        t.Name,
			Description: t.Description,
		}
		if t.ExternalDocs != nil {
			tag.ExternalDocs = externalDocs31(t.ExternalDocs)
		}
		tag.Extensions = copyExtensions(t.Extensions, "3.1.2")
		out[i] = tag
	}
	return out
}

func security31(in []model.SecurityRequirement) []SecurityRequirementV31 {
	out := make([]SecurityRequirementV31, len(in))
	for i, s := range in {
		out[i] = SecurityRequirementV31(s)
	}
	return out
}

func externalDocs31(in *model.ExternalDocs) *ExternalDocsV31 {
	if in == nil {
		return nil
	}
	ed := &ExternalDocsV31{
		Description: in.Description,
		URL:         in.URL,
	}
	ed.Extensions = copyExtensions(in.Extensions, "3.1.2")
	return ed
}

func paths31(in map[string]*model.PathItem, warns *[]Warning) map[string]*PathItemV31 {
	out := make(map[string]*PathItemV31, len(in))
	for path, item := range in {
		out[path] = pathItem31(item, warns)
	}
	return out
}

func pathItem31(in *model.PathItem, warns *[]Warning) *PathItemV31 {
	item := &PathItemV31{
		Summary:     in.Summary,
		Description: in.Description,
		Parameters:  parameters31(in.Parameters, warns),
	}
	if in.Get != nil {
		item.Get = operation31(in.Get, warns)
	}
	if in.Put != nil {
		item.Put = operation31(in.Put, warns)
	}
	if in.Post != nil {
		item.Post = operation31(in.Post, warns)
	}
	if in.Delete != nil {
		item.Delete = operation31(in.Delete, warns)
	}
	if in.Options != nil {
		item.Options = operation31(in.Options, warns)
	}
	if in.Head != nil {
		item.Head = operation31(in.Head, warns)
	}
	if in.Patch != nil {
		item.Patch = operation31(in.Patch, warns)
	}
	item.Extensions = copyExtensions(in.Extensions, "3.1.2")
	return item
}

func webhooks31(in map[string]*model.PathItem, warns *[]Warning) map[string]*PathItemV31 {
	out := make(map[string]*PathItemV31, len(in))
	for path, item := range in {
		out[path] = pathItem31(item, warns)
	}
	return out
}

func operation31(in *model.Operation, warns *[]Warning) *OperationV31 {
	op := &OperationV31{
		Tags:        append([]string(nil), in.Tags...),
		Summary:     in.Summary,
		Description: in.Description,
		OperationID: in.OperationID,
		Deprecated:  in.Deprecated,
		Parameters:  parameters31(in.Parameters, warns),
		Responses:   responses31(in.Responses, warns),
	}
	if in.RequestBody != nil {
		op.RequestBody = requestBody31(in.RequestBody, warns)
	}
	if len(in.Callbacks) > 0 {
		op.Callbacks = callbacks31(in.Callbacks, warns)
	}
	if len(in.Security) > 0 {
		op.Security = security31(in.Security)
	}
	op.Extensions = copyExtensions(in.Extensions, "3.1.2")
	return op
}

func parameters31(in []model.Parameter, warns *[]Warning) []ParameterV31 {
	out := make([]ParameterV31, len(in))
	for i, p := range in {
		out[i] = parameter31(p, warns)
	}
	return out
}

func parameter31(in model.Parameter, warns *[]Warning) ParameterV31 {
	p := ParameterV31{
		Name:            in.Name,
		In:              in.In,
		Description:     in.Description,
		Required:        in.Required,
		Deprecated:      in.Deprecated,
		AllowEmptyValue: in.AllowEmptyValue,
		Style:           in.Style,
		Explode:         in.Explode,
		AllowReserved:   in.AllowReserved,
		Example:         in.Example,
	}
	if in.Schema != nil {
		p.Schema = schema31(in.Schema, warns, "#/parameters/"+in.Name)
	}
	if len(in.Examples) > 0 {
		p.Examples = make(map[string]*ExampleV31, len(in.Examples))
		for k, ex := range in.Examples {
			exV31 := &ExampleV31{
				Summary:       ex.Summary,
				Description:   ex.Description,
				Value:         ex.Value,
				ExternalValue: ex.ExternalValue,
			}
			exV31.Extensions = copyExtensions(ex.Extensions, "3.1.2")
			p.Examples[k] = exV31
		}
	}
	if len(in.Content) > 0 {
		p.Content = make(map[string]*MediaTypeV31, len(in.Content))
		for ct, mt := range in.Content {
			p.Content[ct] = mediaType31(mt, warns, "#/parameters/"+in.Name+"/content/"+ct)
		}
	}
	p.Extensions = copyExtensions(in.Extensions, "3.1.2")
	return p
}

func requestBody31(in *model.RequestBody, warns *[]Warning) *RequestBodyV31 {
	rb := &RequestBodyV31{
		Description: in.Description,
		Required:    in.Required,
		Content:     make(map[string]*MediaTypeV31, len(in.Content)),
	}
	for ct, mt := range in.Content {
		rb.Content[ct] = mediaType31(mt, warns, "#/requestBody/content/"+ct)
	}
	rb.Extensions = copyExtensions(in.Extensions, "3.1.2")
	return rb
}

func responses31(in map[string]*model.Response, warns *[]Warning) map[string]*ResponseV31 {
	out := make(map[string]*ResponseV31, len(in))
	for code, r := range in {
		out[code] = response31(r, warns, "#/responses/"+code)
	}
	return out
}

func response31(in *model.Response, warns *[]Warning, path string) *ResponseV31 {
	r := &ResponseV31{
		Description: in.Description,
		Content:     make(map[string]*MediaTypeV31, len(in.Content)),
	}
	for ct, mt := range in.Content {
		r.Content[ct] = mediaType31(mt, warns, path+"/content/"+ct)
	}
	if len(in.Headers) > 0 {
		r.Headers = make(map[string]*HeaderV31, len(in.Headers))
		for name, h := range in.Headers {
			r.Headers[name] = header31(h, warns, path+"/headers/"+name)
		}
	}
	if len(in.Links) > 0 {
		r.Links = make(map[string]*LinkV31, len(in.Links))
		for name, link := range in.Links {
			r.Links[name] = link31(link, warns)
		}
	}
	r.Extensions = copyExtensions(in.Extensions, "3.1.2")
	return r
}

func link31(in *model.Link, warns *[]Warning) *LinkV31 {
	if in == nil {
		return nil
	}
	link := &LinkV31{
		OperationRef: in.OperationRef,
		OperationID:  in.OperationID,
		Parameters:   in.Parameters,
		RequestBody:  in.RequestBody,
		Description:  in.Description,
	}
	if in.Server != nil {
		servers := servers31([]model.Server{*in.Server}, warns)
		if len(servers) > 0 {
			link.Server = &servers[0]
		}
	}
	link.Extensions = copyExtensions(in.Extensions, "3.1.2")
	return link
}

func header31(in *model.Header, warns *[]Warning, path string) *HeaderV31 {
	if in == nil {
		return nil
	}
	h := &HeaderV31{
		Description: in.Description,
		Required:    in.Required,
		Deprecated:  in.Deprecated,
		Style:       in.Style,
		Explode:     in.Explode,
		Example:     in.Example,
	}
	if in.Schema != nil {
		h.Schema = schema31(in.Schema, warns, path+"/schema")
	}
	if len(in.Examples) > 0 {
		h.Examples = make(map[string]*ExampleV31, len(in.Examples))
		for k, ex := range in.Examples {
			h.Examples[k] = &ExampleV31{
				Summary:       ex.Summary,
				Description:   ex.Description,
				Value:         ex.Value,
				ExternalValue: ex.ExternalValue,
			}
		}
	}
	if len(in.Content) > 0 {
		h.Content = make(map[string]*MediaTypeV31, len(in.Content))
		for ct, mt := range in.Content {
			h.Content[ct] = mediaType31(mt, warns, path+"/content/"+ct)
		}
	}
	h.Extensions = copyExtensions(in.Extensions, "3.1.2")
	return h
}

func mediaType31(in *model.MediaType, warns *[]Warning, path string) *MediaTypeV31 {
	if in == nil {
		return nil
	}
	mt := &MediaTypeV31{
		Example: in.Example,
	}
	if in.Schema != nil {
		mt.Schema = schema31(in.Schema, warns, path+"/schema")
	}
	if len(in.Examples) > 0 {
		mt.Examples = make(map[string]*ExampleV31, len(in.Examples))
		for k, ex := range in.Examples {
			exV31 := &ExampleV31{
				Summary:       ex.Summary,
				Description:   ex.Description,
				Value:         ex.Value,
				ExternalValue: ex.ExternalValue,
			}
			exV31.Extensions = copyExtensions(ex.Extensions, "3.1.2")
			mt.Examples[k] = exV31
		}
	}
	if len(in.Encoding) > 0 {
		mt.Encoding = make(map[string]*EncodingV31, len(in.Encoding))
		for k, enc := range in.Encoding {
			mt.Encoding[k] = encoding31(enc, warns)
		}
	}
	mt.Extensions = copyExtensions(in.Extensions, "3.1.2")
	return mt
}

func encoding31(in *model.Encoding, warns *[]Warning) *EncodingV31 {
	if in == nil {
		return nil
	}
	enc := &EncodingV31{
		ContentType:   in.ContentType,
		Style:         in.Style,
		Explode:       in.Explode,
		AllowReserved: in.AllowReserved,
	}
	if len(in.Headers) > 0 {
		enc.Headers = make(map[string]*HeaderV31, len(in.Headers))
		for k, h := range in.Headers {
			enc.Headers[k] = header31(h, warns, "#/encoding/"+k+"/headers/"+k)
		}
	}
	enc.Extensions = copyExtensions(in.Extensions, "3.1.2")
	return enc
}

func callbacks31(in map[string]*model.Callback, warns *[]Warning) map[string]*CallbackV31 {
	out := make(map[string]*CallbackV31, len(in))
	for name, cb := range in {
		out[name] = &CallbackV31{
			PathItems: make(map[string]*PathItemV31, len(cb.PathItems)),
		}
		for path, item := range cb.PathItems {
			out[name].PathItems[path] = pathItem31(item, warns)
		}
		out[name].Extensions = copyExtensions(cb.Extensions, "3.1.2")
	}
	return out
}

func components31(in *model.Components, warns *[]Warning) *ComponentsV31 {
	comp := &ComponentsV31{
		Schemas:         make(map[string]*SchemaV31, len(in.Schemas)),
		SecuritySchemes: make(map[string]*SecuritySchemeV31, len(in.SecuritySchemes)),
	}
	for name, s := range in.Schemas {
		comp.Schemas[name] = schema31(s, warns, "#/components/schemas/"+name)
	}
	for name, ss := range in.SecuritySchemes {
		comp.SecuritySchemes[name] = securityScheme31(ss)
	}
	comp.Extensions = copyExtensions(in.Extensions, "3.1.2")
	return comp
}

func securityScheme31(in *model.SecurityScheme) *SecuritySchemeV31 {
	out := &SecuritySchemeV31{
		Type:             in.Type,
		Description:      in.Description,
		Name:             in.Name,
		In:               in.In,
		Scheme:           in.Scheme,
		BearerFormat:     in.BearerFormat,
		OpenIDConnectURL: in.OpenIDConnectURL,
	}
	if in.Flows != nil {
		out.Flows = oAuthFlows31(in.Flows)
	}
	out.Extensions = copyExtensions(in.Extensions, "3.1.2")
	return out
}

func oAuthFlows31(in *model.OAuthFlows) *OAuthFlowsV31 {
	out := &OAuthFlowsV31{}
	if in.Implicit != nil {
		out.Implicit = oAuthFlow31(in.Implicit)
	}
	if in.Password != nil {
		out.Password = oAuthFlow31(in.Password)
	}
	if in.ClientCredentials != nil {
		out.ClientCredentials = oAuthFlow31(in.ClientCredentials)
	}
	if in.AuthorizationCode != nil {
		out.AuthorizationCode = oAuthFlow31(in.AuthorizationCode)
	}
	// Return nil if no flows are set
	if out.Implicit == nil && out.Password == nil && out.ClientCredentials == nil && out.AuthorizationCode == nil {
		return nil
	}
	out.Extensions = copyExtensions(in.Extensions, "3.1.2")
	return out
}

func oAuthFlow31(in *model.OAuthFlow) *OAuthFlowV31 {
	out := &OAuthFlowV31{
		AuthorizationURL: in.AuthorizationURL,
		TokenURL:         in.TokenURL,
		RefreshURL:       in.RefreshURL,
	}
	if in.Scopes != nil {
		out.Scopes = make(map[string]string, len(in.Scopes))
		for k, v := range in.Scopes {
			out.Scopes[k] = v
		}
	} else {
		out.Scopes = make(map[string]string)
	}
	out.Extensions = copyExtensions(in.Extensions, "3.1.2")
	return out
}
