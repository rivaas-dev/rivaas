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
	"fmt"
	"maps"
	"slices"
	"strconv"

	"rivaas.dev/openapi/diag"
	"rivaas.dev/openapi/internal/model"
)

// OpenAPI 3.1.2 constants
const (
	dialect31 = "https://spec.openapis.org/oas/3.1/dialect/2024-11-10"
)

// proj31 carries projection state for OpenAPI 3.1.x
type proj31 struct {
	warns diag.Warnings
}

// warn adds a warning to the projection
func (p *proj31) warn(code diag.WarningCode, path, msg string) {
	p.warns = append(p.warns, newWarning(code, path, msg))
}

// ext copies extensions with version context
func (p *proj31) ext(src map[string]any) map[string]any {
	return copyExtensions(src, string(V31))
}

// projectTo31 projects a spec to OpenAPI 3.1.x format.
func projectTo31(in *model.Spec) (*SpecV31, diag.Warnings, error) {
	p := &proj31{warns: diag.Warnings{}}
	spec, err := p.project(in)
	return spec, p.warns, err
}

// project performs the actual projection
func (p *proj31) project(in *model.Spec) (*SpecV31, error) {
	// 3.1 requires any of paths|components|webhooks (we assume builder guarantees at least one)
	info, err := p.info(in.Info)
	if err != nil {
		return nil, err
	}
	out := &SpecV31{
		OpenAPI:           string(V31),
		JSONSchemaDialect: dialect31,
		Info:              info,
		Servers:           p.servers(in.Servers),
		Paths:             p.paths(in.Paths),
		Tags:              p.tags(in.Tags),
	}

	if in.Components != nil {
		out.Components = p.components(in.Components)
	}

	if len(in.Webhooks) > 0 {
		out.Webhooks = p.webhooks(in.Webhooks)
	}

	if len(in.Security) > 0 {
		out.Security = p.security(in.Security)
	}

	if in.ExternalDocs != nil {
		out.ExternalDocs = p.externalDocs(in.ExternalDocs)
	}

	out.Extensions = p.ext(in.Extensions)
	return out, nil
}

func (p *proj31) info(in model.Info) (*InfoV31, error) {
	info := &InfoV31{
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
		info.Contact.Extensions = p.ext(in.Contact.Extensions)
	}
	if in.License != nil {
		// Enforce mutual exclusivity - this should never happen with proper API usage
		// but serves as a defensive check for manually constructed License structs
		if in.License.Identifier != "" && in.License.URL != "" {
			return nil, fmt.Errorf("license identifier and url are mutually exclusive per OpenAPI spec - both fields cannot be set")
		}
		info.License = &LicenseV31{
			Name:       in.License.Name,
			Identifier: in.License.Identifier,
			URL:        in.License.URL,
		}
		info.License.Extensions = p.ext(in.License.Extensions)
	}
	info.Extensions = p.ext(in.Extensions)

	return info, nil
}

func (p *proj31) servers(in []model.Server) []ServerV31 {
	// 3.1: inject default if empty
	if len(in) == 0 {
		return []ServerV31{{URL: "/"}}
	}
	out := make([]ServerV31, 0, len(in))
	for i, s := range in {
		server := ServerV31{
			URL:         s.URL,
			Description: s.Description,
		}
		if len(s.Variables) > 0 {
			server.Variables = p.convertServerVariables(s.Variables, i)
		}
		server.Extensions = p.ext(s.Extensions)
		out = append(out, server)
	}

	return out
}

// convertServerVariables converts and validates server variables for OpenAPI 3.1.
func (p *proj31) convertServerVariables(vars map[string]*model.ServerVariable, serverIdx int) map[string]*ServerVariableV31 {
	out := make(map[string]*ServerVariableV31, len(vars))

	for name, v := range vars {
		p.validateServerVariableEnum(v, serverIdx, name)

		out[name] = &ServerVariableV31{
			Enum:        v.Enum,
			Default:     v.Default,
			Description: v.Description,
		}
		out[name].Extensions = p.ext(v.Extensions)
	}

	return out
}

// validateServerVariableEnum validates enum constraints per OpenAPI 3.1 spec.
func (p *proj31) validateServerVariableEnum(v *model.ServerVariable, serverIdx int, name string) {
	path := "#/servers/" + strconv.Itoa(serverIdx) + "/variables/" + name

	// 3.1 validation: enum MUST NOT be empty if present
	if len(v.Enum) == 0 {
		p.warn("SERVER_VARIABLE_EMPTY_ENUM", path,
			"server variable enum array MUST NOT be empty in OpenAPI 3.1 (if enum is provided, it must contain at least one value)")
		return
	}

	// 3.1 validation: default MUST exist in enum if enum is defined
	if slices.Contains(v.Enum, v.Default) {
		return // Found, valid
	}

	p.warn("SERVER_VARIABLE_DEFAULT_NOT_IN_ENUM", path,
		"server variable default value MUST exist in enum values in OpenAPI 3.1")
}

func (p *proj31) tags(in []model.Tag) []TagV31 {
	out := make([]TagV31, 0, len(in))
	for _, t := range in {
		tag := TagV31{
			Name:        t.Name,
			Description: t.Description,
		}
		if t.ExternalDocs != nil {
			tag.ExternalDocs = p.externalDocs(t.ExternalDocs)
		}
		tag.Extensions = p.ext(t.Extensions)
		out = append(out, tag)
	}

	return out
}

func (p *proj31) security(in []model.SecurityRequirement) []SecurityRequirementV31 {
	out := make([]SecurityRequirementV31, 0, len(in))
	for _, s := range in {
		out = append(out, SecurityRequirementV31(s))
	}

	return out
}

func (p *proj31) externalDocs(in *model.ExternalDocs) *ExternalDocsV31 {
	if in == nil {
		return nil
	}
	ed := &ExternalDocsV31{
		Description: in.Description,
		URL:         in.URL,
	}
	ed.Extensions = p.ext(in.Extensions)

	return ed
}

func (p *proj31) paths(in map[string]*model.PathItem) map[string]*PathItemV31 {
	out := make(map[string]*PathItemV31, len(in))
	for path, item := range in {
		out[path] = p.pathItem(item)
	}

	return out
}

func (p *proj31) pathItem(in *model.PathItem) *PathItemV31 {
	// Handle $ref case
	if in.Ref != "" {
		return &PathItemV31{Ref: in.Ref}
	}

	item := &PathItemV31{
		Summary:     in.Summary,
		Description: in.Description,
		Parameters:  p.parameters(in.Parameters),
	}
	if in.Get != nil {
		item.Get = p.operation(in.Get)
	}
	if in.Put != nil {
		item.Put = p.operation(in.Put)
	}
	if in.Post != nil {
		item.Post = p.operation(in.Post)
	}
	if in.Delete != nil {
		item.Delete = p.operation(in.Delete)
	}
	if in.Options != nil {
		item.Options = p.operation(in.Options)
	}
	if in.Head != nil {
		item.Head = p.operation(in.Head)
	}
	if in.Patch != nil {
		item.Patch = p.operation(in.Patch)
	}
	if in.Trace != nil {
		item.Trace = p.operation(in.Trace)
	}
	item.Extensions = p.ext(in.Extensions)

	return item
}

func (p *proj31) webhooks(in map[string]*model.PathItem) map[string]*PathItemV31 {
	out := make(map[string]*PathItemV31, len(in))
	for path, item := range in {
		out[path] = p.pathItem(item)
	}

	return out
}

func (p *proj31) operation(in *model.Operation) *OperationV31 {
	op := &OperationV31{
		Tags:        append([]string(nil), in.Tags...),
		Summary:     in.Summary,
		Description: in.Description,
		OperationID: in.OperationID,
		Deprecated:  in.Deprecated,
		Parameters:  p.parameters(in.Parameters),
		Responses:   p.responses(in.Responses),
	}
	if in.ExternalDocs != nil {
		op.ExternalDocs = p.externalDocs(in.ExternalDocs)
	}
	if in.RequestBody != nil {
		op.RequestBody = p.requestBody(in.RequestBody)
	}
	if len(in.Callbacks) > 0 {
		op.Callbacks = p.callbacks(in.Callbacks)
	}
	if len(in.Security) > 0 {
		op.Security = p.security(in.Security)
	}
	if len(in.Servers) > 0 {
		op.Servers = p.servers(in.Servers)
	}
	op.Extensions = p.ext(in.Extensions)

	return op
}

func (p *proj31) parameters(in []model.Parameter) []ParameterV31 {
	out := make([]ParameterV31, 0, len(in))
	for _, param := range in {
		out = append(out, p.parameter(param))
	}

	return out
}

func (p *proj31) parameter(in model.Parameter) ParameterV31 {
	// Handle $ref case
	if in.Ref != "" {
		return ParameterV31{Ref: in.Ref}
	}

	param := ParameterV31{
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
		param.Schema = schema31(in.Schema, p, "#/parameters/"+in.Name)
	}
	if len(in.Examples) > 0 {
		param.Examples = make(map[string]*ExampleV31, len(in.Examples))
		for k, ex := range in.Examples {
			// Handle $ref case in examples
			if ex.Ref != "" {
				param.Examples[k] = &ExampleV31{Ref: ex.Ref}
				continue
			}
			exV31 := &ExampleV31{
				Summary:       ex.Summary,
				Description:   ex.Description,
				Value:         ex.Value,
				ExternalValue: ex.ExternalValue,
			}
			exV31.Extensions = p.ext(ex.Extensions)
			param.Examples[k] = exV31
		}
	}
	if len(in.Content) > 0 {
		param.Content = make(map[string]*MediaTypeV31, len(in.Content))
		for ct, mt := range in.Content {
			param.Content[ct] = p.mediaType(mt, "#/parameters/"+in.Name+"/content/"+ct)
		}
	}
	param.Extensions = p.ext(in.Extensions)

	return param
}

func (p *proj31) requestBody(in *model.RequestBody) *RequestBodyV31 {
	// Handle $ref case
	if in.Ref != "" {
		return &RequestBodyV31{Ref: in.Ref}
	}

	rb := &RequestBodyV31{
		Description: in.Description,
		Required:    in.Required,
		Content:     make(map[string]*MediaTypeV31, len(in.Content)),
	}
	for ct, mt := range in.Content {
		rb.Content[ct] = p.mediaType(mt, "#/requestBody/content/"+ct)
	}
	rb.Extensions = p.ext(in.Extensions)

	return rb
}

func (p *proj31) responses(in map[string]*model.Response) map[string]*ResponseV31 {
	out := make(map[string]*ResponseV31, len(in))
	for code, r := range in {
		out[code] = p.response(r, "#/responses/"+code)
	}

	return out
}

func (p *proj31) response(in *model.Response, path string) *ResponseV31 {
	// Handle $ref case
	if in.Ref != "" {
		return &ResponseV31{Ref: in.Ref}
	}

	r := &ResponseV31{
		Description: in.Description,
		Content:     make(map[string]*MediaTypeV31, len(in.Content)),
	}
	for ct, mt := range in.Content {
		r.Content[ct] = p.mediaType(mt, path+"/content/"+ct)
	}
	if len(in.Headers) > 0 {
		r.Headers = make(map[string]*HeaderV31, len(in.Headers))
		for name, h := range in.Headers {
			r.Headers[name] = p.header(h, path+"/headers/"+name)
		}
	}
	if len(in.Links) > 0 {
		r.Links = make(map[string]*LinkV31, len(in.Links))
		for name, link := range in.Links {
			r.Links[name] = p.link(link)
		}
	}
	r.Extensions = p.ext(in.Extensions)

	return r
}

func (p *proj31) link(in *model.Link) *LinkV31 {
	if in == nil {
		return nil
	}
	// Handle $ref case
	if in.Ref != "" {
		return &LinkV31{Ref: in.Ref}
	}

	link := &LinkV31{
		OperationRef: in.OperationRef,
		OperationID:  in.OperationID,
		Parameters:   in.Parameters,
		RequestBody:  in.RequestBody,
		Description:  in.Description,
	}
	if in.Server != nil {
		servers := p.servers([]model.Server{*in.Server})
		if len(servers) > 0 {
			link.Server = &servers[0]
		}
	}
	link.Extensions = p.ext(in.Extensions)

	return link
}

func (p *proj31) header(in *model.Header, path string) *HeaderV31 {
	if in == nil {
		return nil
	}
	// Handle $ref case
	if in.Ref != "" {
		return &HeaderV31{Ref: in.Ref}
	}

	h := &HeaderV31{
		Description:     in.Description,
		Required:        in.Required,
		Deprecated:      in.Deprecated,
		AllowEmptyValue: in.AllowEmptyValue,
		Style:           in.Style,
		Explode:         in.Explode,
		Example:         in.Example,
	}
	if in.Schema != nil {
		h.Schema = schema31(in.Schema, p, path+"/schema")
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
			h.Content[ct] = p.mediaType(mt, path+"/content/"+ct)
		}
	}
	h.Extensions = p.ext(in.Extensions)

	return h
}

func (p *proj31) mediaType(in *model.MediaType, path string) *MediaTypeV31 {
	if in == nil {
		return nil
	}
	mt := &MediaTypeV31{
		Example: in.Example,
	}
	if in.Schema != nil {
		mt.Schema = schema31(in.Schema, p, path+"/schema")
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
			exV31.Extensions = p.ext(ex.Extensions)
			mt.Examples[k] = exV31
		}
	}
	if len(in.Encoding) > 0 {
		mt.Encoding = make(map[string]*EncodingV31, len(in.Encoding))
		for k, enc := range in.Encoding {
			mt.Encoding[k] = p.encoding(enc)
		}
	}
	mt.Extensions = p.ext(in.Extensions)

	return mt
}

func (p *proj31) encoding(in *model.Encoding) *EncodingV31 {
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
			enc.Headers[k] = p.header(h, "#/encoding/"+k+"/headers/"+k)
		}
	}
	enc.Extensions = p.ext(in.Extensions)

	return enc
}

func (p *proj31) callbacks(in map[string]*model.Callback) map[string]*CallbackV31 {
	out := make(map[string]*CallbackV31, len(in))
	for name, cb := range in {
		// Handle $ref case
		if cb.Ref != "" {
			out[name] = &CallbackV31{Ref: cb.Ref}
			continue
		}

		out[name] = &CallbackV31{
			PathItems: make(map[string]*PathItemV31, len(cb.PathItems)),
		}
		for path, item := range cb.PathItems {
			out[name].PathItems[path] = p.pathItem(item)
		}
		out[name].Extensions = p.ext(cb.Extensions)
	}

	return out
}

func (p *proj31) components(in *model.Components) *ComponentsV31 {
	comp := &ComponentsV31{
		Schemas:         make(map[string]*SchemaV31, len(in.Schemas)),
		Responses:       make(map[string]*ResponseV31, len(in.Responses)),
		Parameters:      make(map[string]*ParameterV31, len(in.Parameters)),
		Examples:        make(map[string]*ExampleV31, len(in.Examples)),
		RequestBodies:   make(map[string]*RequestBodyV31, len(in.RequestBodies)),
		Headers:         make(map[string]*HeaderV31, len(in.Headers)),
		SecuritySchemes: make(map[string]*SecuritySchemeV31, len(in.SecuritySchemes)),
		Links:           make(map[string]*LinkV31, len(in.Links)),
		Callbacks:       make(map[string]*CallbackV31, len(in.Callbacks)),
		PathItems:       make(map[string]*PathItemV31, len(in.PathItems)),
	}
	for name, s := range in.Schemas {
		comp.Schemas[name] = schema31(s, p, "#/components/schemas/"+name)
	}
	for name, r := range in.Responses {
		comp.Responses[name] = p.response(r, "#/components/responses/"+name)
	}
	for name, param := range in.Parameters {
		pv := p.parameter(*param)
		comp.Parameters[name] = &pv
	}
	for name, ex := range in.Examples {
		// Handle $ref case
		if ex.Ref != "" {
			comp.Examples[name] = &ExampleV31{Ref: ex.Ref}
			continue
		}
		comp.Examples[name] = &ExampleV31{
			Summary:       ex.Summary,
			Description:   ex.Description,
			Value:         ex.Value,
			ExternalValue: ex.ExternalValue,
		}
		comp.Examples[name].Extensions = p.ext(ex.Extensions)
	}
	for name, rb := range in.RequestBodies {
		comp.RequestBodies[name] = p.requestBody(rb)
	}
	for name, h := range in.Headers {
		comp.Headers[name] = p.header(h, "#/components/headers/"+name)
	}
	for name, ss := range in.SecuritySchemes {
		comp.SecuritySchemes[name] = p.securityScheme(ss)
	}
	for name, link := range in.Links {
		comp.Links[name] = p.link(link)
	}
	for name, cb := range in.Callbacks {
		// Handle $ref case
		if cb.Ref != "" {
			comp.Callbacks[name] = &CallbackV31{Ref: cb.Ref}
			continue
		}
		comp.Callbacks[name] = &CallbackV31{
			PathItems: make(map[string]*PathItemV31, len(cb.PathItems)),
		}
		for path, item := range cb.PathItems {
			comp.Callbacks[name].PathItems[path] = p.pathItem(item)
		}
		comp.Callbacks[name].Extensions = p.ext(cb.Extensions)
	}
	for name, pi := range in.PathItems {
		comp.PathItems[name] = p.pathItem(pi)
	}
	comp.Extensions = p.ext(in.Extensions)

	return comp
}

func (p *proj31) securityScheme(in *model.SecurityScheme) *SecuritySchemeV31 {
	// Handle $ref case
	if in.Ref != "" {
		return &SecuritySchemeV31{Ref: in.Ref}
	}

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
		out.Flows = p.oAuthFlows(in.Flows)
	}
	out.Extensions = p.ext(in.Extensions)

	return out
}

func (p *proj31) oAuthFlows(in *model.OAuthFlows) *OAuthFlowsV31 {
	out := &OAuthFlowsV31{}
	if in.Implicit != nil {
		out.Implicit = p.oAuthFlow(in.Implicit)
	}
	if in.Password != nil {
		out.Password = p.oAuthFlow(in.Password)
	}
	if in.ClientCredentials != nil {
		out.ClientCredentials = p.oAuthFlow(in.ClientCredentials)
	}
	if in.AuthorizationCode != nil {
		out.AuthorizationCode = p.oAuthFlow(in.AuthorizationCode)
	}
	// Return nil if no flows are set
	if out.Implicit == nil && out.Password == nil && out.ClientCredentials == nil && out.AuthorizationCode == nil {
		return nil
	}
	out.Extensions = p.ext(in.Extensions)

	return out
}

func (p *proj31) oAuthFlow(in *model.OAuthFlow) *OAuthFlowV31 {
	out := &OAuthFlowV31{
		AuthorizationURL: in.AuthorizationURL,
		TokenURL:         in.TokenURL,
		RefreshURL:       in.RefreshURL,
	}
	if in.Scopes != nil {
		out.Scopes = make(map[string]string, len(in.Scopes))
		maps.Copy(out.Scopes, in.Scopes)
	} else {
		out.Scopes = make(map[string]string)
	}
	out.Extensions = p.ext(in.Extensions)

	return out
}
