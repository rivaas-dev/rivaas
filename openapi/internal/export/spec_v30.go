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
	"errors"
	"maps"

	"rivaas.dev/openapi/diag"
	"rivaas.dev/openapi/internal/model"
)

// proj30 carries projection state for OpenAPI 3.0.x
type proj30 struct {
	warns  diag.Warnings
	config Config
}

// warn adds a warning to the projection
func (p *proj30) warn(code diag.WarningCode, path, msg string) {
	p.warns = append(p.warns, newWarning(code, path, msg))
}

// ext copies extensions with version context
func (p *proj30) ext(src map[string]any) map[string]any {
	return copyExtensions(src, string(V30))
}

// projectTo30 projects a spec to OpenAPI 3.0.4 format.
func projectTo30(in *model.Spec, cfg Config) (*SpecV30, diag.Warnings, error) {
	p := &proj30{config: cfg, warns: diag.Warnings{}}
	spec, err := p.project(in)
	return spec, p.warns, err
}

// project performs the actual projection
func (p *proj30) project(in *model.Spec) (*SpecV30, error) {
	// 3.0 MUST have paths
	if len(in.Paths) == 0 {
		return nil, errors.New("OpenAPI 3.0 requires 'paths'")
	}

	info := p.info(in.Info)
	out := &SpecV30{
		OpenAPI: string(V30),
		Info:    info,
		Servers: p.servers(in.Servers),
		Paths:   p.paths(in.Paths),
		Tags:    p.tags(in.Tags),
	}

	// Check for summary (3.1-only field) in strict mode
	if in.Info.Summary != "" && p.config.StrictDownlevel {
		return nil, errors.New("info.summary not supported in OpenAPI 3.0")
	}

	var mutualTLSSchemes []string
	if in.Components != nil {
		out.Components, mutualTLSSchemes = p.components(in.Components)
		// Warn about mutualTLS schemes (3.1-only feature)
		for _, name := range mutualTLSSchemes {
			p.warn(diag.WarnDownlevelMutualTLS, "#/components/securitySchemes/"+name,
				"mutualTLS security type is 3.1-only; dropped")
		}
		if len(mutualTLSSchemes) > 0 && p.config.StrictDownlevel {
			return nil, errors.New("mutualTLS security type not supported in OpenAPI 3.0")
		}
	}

	if len(in.Security) > 0 {
		out.Security = p.security(in.Security)
	}

	if in.ExternalDocs != nil {
		out.ExternalDocs = p.externalDocs(in.ExternalDocs)
	}

	out.Extensions = p.ext(in.Extensions)

	// Webhooks are not in 3.0: warn if present
	if len(in.Webhooks) > 0 {
		p.warn(diag.WarnDownlevelWebhooks, "#/webhooks", "webhooks are 3.1-only; dropped")
		if p.config.StrictDownlevel {
			return nil, errors.New("webhooks not supported in OpenAPI 3.0")
		}
	}

	return out, nil
}

func (p *proj30) info(in model.Info) *InfoV30 {
	info := &InfoV30{
		Title:          in.Title,
		Description:    in.Description,
		TermsOfService: in.TermsOfService,
		Version:        in.Version,
	}
	// Drop summary if present (3.1-only field)
	if in.Summary != "" {
		p.warn(diag.WarnDownlevelInfoSummary, "#/info/summary", "info.summary is 3.1-only; dropped")
	}
	if in.Contact != nil {
		info.Contact = &ContactV30{
			Name:  in.Contact.Name,
			URL:   in.Contact.URL,
			Email: in.Contact.Email,
		}
		info.Contact.Extensions = p.ext(in.Contact.Extensions)
	}
	if in.License != nil {
		info.License = &LicenseV30{
			Name: in.License.Name,
			URL:  in.License.URL,
		}
		// Warn if identifier is present (3.1-only feature)
		if in.License.Identifier != "" {
			p.warn(diag.WarnDownlevelLicenseIdentifier, "#/info/license",
				"license identifier is 3.1-only; dropped (use url instead)")
		}
		info.License.Extensions = p.ext(in.License.Extensions)
	}
	info.Extensions = p.ext(in.Extensions)

	return info
}

func (p *proj30) servers(in []model.Server) []ServerV30 {
	out := make([]ServerV30, 0, len(in))
	for _, s := range in {
		server := ServerV30{
			URL:         s.URL,
			Description: s.Description,
		}
		if len(s.Variables) > 0 {
			server.Variables = make(map[string]*ServerVariableV30, len(s.Variables))
			for name, v := range s.Variables {
				server.Variables[name] = &ServerVariableV30{
					Enum:        v.Enum,
					Default:     v.Default,
					Description: v.Description,
				}
				server.Variables[name].Extensions = p.ext(v.Extensions)
			}
		}
		server.Extensions = p.ext(s.Extensions)
		out = append(out, server)
	}

	return out
}

func (p *proj30) tags(in []model.Tag) []TagV30 {
	out := make([]TagV30, 0, len(in))
	for _, t := range in {
		tag := TagV30{
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

func (p *proj30) security(in []model.SecurityRequirement) []SecurityRequirementV30 {
	out := make([]SecurityRequirementV30, 0, len(in))
	for _, s := range in {
		out = append(out, SecurityRequirementV30(s))
	}

	return out
}

func (p *proj30) paths(in map[string]*model.PathItem) map[string]*PathItemV30 {
	out := make(map[string]*PathItemV30, len(in))
	for path, item := range in {
		out[path] = p.pathItem(item)
	}

	return out
}

func (p *proj30) pathItem(in *model.PathItem) *PathItemV30 {
	// Handle $ref case
	if in.Ref != "" {
		return &PathItemV30{Ref: in.Ref}
	}

	item := &PathItemV30{
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

func (p *proj30) operation(in *model.Operation) *OperationV30 {
	op := &OperationV30{
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
	if len(in.Security) > 0 {
		op.Security = p.security(in.Security)
	}
	if len(in.Servers) > 0 {
		op.Servers = p.servers(in.Servers)
	}
	if len(in.Callbacks) > 0 {
		op.Callbacks = p.callbacks(in.Callbacks)
	}
	op.Extensions = p.ext(in.Extensions)

	return op
}

func (p *proj30) parameters(in []model.Parameter) []ParameterV30 {
	out := make([]ParameterV30, 0, len(in))
	for _, param := range in {
		out = append(out, p.parameter(param))
	}

	return out
}

func (p *proj30) parameter(in model.Parameter) ParameterV30 {
	// Handle $ref case
	if in.Ref != "" {
		return ParameterV30{Ref: in.Ref}
	}

	param := ParameterV30{
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
		param.Schema = schema30(in.Schema, p, "#/parameters/"+in.Name)
	}
	if len(in.Examples) > 0 {
		param.Examples = make(map[string]*ExampleV30, len(in.Examples))
		for k, v := range in.Examples {
			param.Examples[k] = p.example(v)
		}
	}
	if len(in.Content) > 0 {
		param.Content = make(map[string]*MediaTypeV30, len(in.Content))
		for ct, mt := range in.Content {
			param.Content[ct] = p.mediaType(mt, "#/parameters/"+in.Name+"/content/"+ct)
		}
	}
	param.Extensions = p.ext(in.Extensions)

	return param
}

func (p *proj30) example(in *model.Example) *ExampleV30 {
	if in == nil {
		return nil
	}
	// Handle $ref case
	if in.Ref != "" {
		return &ExampleV30{Ref: in.Ref}
	}
	ex := &ExampleV30{
		Summary:       in.Summary,
		Description:   in.Description,
		Value:         in.Value,
		ExternalValue: in.ExternalValue,
	}
	ex.Extensions = p.ext(in.Extensions)

	return ex
}

func (p *proj30) requestBody(in *model.RequestBody) *RequestBodyV30 {
	// Handle $ref case
	if in.Ref != "" {
		return &RequestBodyV30{Ref: in.Ref}
	}

	rb := &RequestBodyV30{
		Description: in.Description,
		Required:    in.Required,
		Content:     make(map[string]*MediaTypeV30, len(in.Content)),
	}
	for ct, mt := range in.Content {
		rb.Content[ct] = p.mediaType(mt, "#/requestBody/content/"+ct)
	}
	rb.Extensions = p.ext(in.Extensions)

	return rb
}

func (p *proj30) responses(in map[string]*model.Response) map[string]*ResponseV30 {
	out := make(map[string]*ResponseV30, len(in))
	for code, r := range in {
		out[code] = p.response(r, "#/responses/"+code)
	}

	return out
}

func (p *proj30) response(in *model.Response, path string) *ResponseV30 {
	// Handle $ref case
	if in.Ref != "" {
		return &ResponseV30{Ref: in.Ref}
	}

	r := &ResponseV30{
		Description: in.Description,
		Content:     make(map[string]*MediaTypeV30, len(in.Content)),
	}
	for ct, mt := range in.Content {
		r.Content[ct] = p.mediaType(mt, path+"/content/"+ct)
	}
	if len(in.Headers) > 0 {
		r.Headers = make(map[string]*HeaderV30, len(in.Headers))
		for name, h := range in.Headers {
			r.Headers[name] = p.header(h, path+"/headers/"+name)
		}
	}
	if len(in.Links) > 0 {
		r.Links = make(map[string]*LinkV30, len(in.Links))
		for name, link := range in.Links {
			r.Links[name] = p.link(link)
		}
	}
	r.Extensions = p.ext(in.Extensions)

	return r
}

func (p *proj30) mediaType(in *model.MediaType, path string) *MediaTypeV30 {
	if in == nil {
		return nil
	}
	mt := &MediaTypeV30{
		Example: in.Example,
	}
	if in.Schema != nil {
		mt.Schema = schema30(in.Schema, p, path+"/schema")
	}
	if len(in.Examples) > 0 {
		mt.Examples = make(map[string]*ExampleV30, len(in.Examples))
		for k, ex := range in.Examples {
			mt.Examples[k] = p.example(ex)
		}
	}
	if len(in.Encoding) > 0 {
		mt.Encoding = make(map[string]*EncodingV30, len(in.Encoding))
		for k, enc := range in.Encoding {
			mt.Encoding[k] = p.encoding(enc)
		}
	}
	mt.Extensions = p.ext(in.Extensions)

	return mt
}

func (p *proj30) encoding(in *model.Encoding) *EncodingV30 {
	if in == nil {
		return nil
	}
	enc := &EncodingV30{
		ContentType:   in.ContentType,
		Style:         in.Style,
		Explode:       in.Explode,
		AllowReserved: in.AllowReserved,
	}
	if len(in.Headers) > 0 {
		enc.Headers = make(map[string]*HeaderV30, len(in.Headers))
		for k, h := range in.Headers {
			enc.Headers[k] = p.header(h, "#/encoding/headers/"+k)
		}
	}
	enc.Extensions = p.ext(in.Extensions)

	return enc
}

func (p *proj30) header(in *model.Header, path string) *HeaderV30 {
	if in == nil {
		return nil
	}
	// Handle $ref case
	if in.Ref != "" {
		return &HeaderV30{Ref: in.Ref}
	}

	h := &HeaderV30{
		Description:     in.Description,
		Required:        in.Required,
		Deprecated:      in.Deprecated,
		AllowEmptyValue: in.AllowEmptyValue,
		Style:           in.Style,
		Explode:         in.Explode,
		Example:         in.Example,
	}
	if in.Schema != nil {
		h.Schema = schema30(in.Schema, p, path+"/schema")
	}
	if len(in.Examples) > 0 {
		h.Examples = make(map[string]*ExampleV30, len(in.Examples))
		for k, ex := range in.Examples {
			h.Examples[k] = p.example(ex)
		}
	}
	if len(in.Content) > 0 {
		h.Content = make(map[string]*MediaTypeV30, len(in.Content))
		for ct, mt := range in.Content {
			h.Content[ct] = p.mediaType(mt, path+"/content/"+ct)
		}
	}
	h.Extensions = p.ext(in.Extensions)

	return h
}

func (p *proj30) link(in *model.Link) *LinkV30 {
	if in == nil {
		return nil
	}
	// Handle $ref case
	if in.Ref != "" {
		return &LinkV30{Ref: in.Ref}
	}

	link := &LinkV30{
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

func (p *proj30) callbacks(in map[string]*model.Callback) map[string]*CallbackV30 {
	out := make(map[string]*CallbackV30, len(in))
	for name, cb := range in {
		out[name] = p.callback(cb)
	}
	return out
}

func (p *proj30) components(in *model.Components) (*ComponentsV30, []string) {
	comp := &ComponentsV30{
		Schemas:         make(map[string]*SchemaV30, len(in.Schemas)),
		Responses:       make(map[string]*ResponseV30, len(in.Responses)),
		Parameters:      make(map[string]*ParameterV30, len(in.Parameters)),
		Examples:        make(map[string]*ExampleV30, len(in.Examples)),
		RequestBodies:   make(map[string]*RequestBodyV30, len(in.RequestBodies)),
		Headers:         make(map[string]*HeaderV30, len(in.Headers)),
		SecuritySchemes: make(map[string]*SecuritySchemeV30, len(in.SecuritySchemes)),
		Links:           make(map[string]*LinkV30, len(in.Links)),
		Callbacks:       make(map[string]*CallbackV30, len(in.Callbacks)),
	}
	var mutualTLSSchemes []string

	for name, s := range in.Schemas {
		comp.Schemas[name] = schema30(s, p, "#/components/schemas/"+name)
	}
	for name, r := range in.Responses {
		comp.Responses[name] = p.response(r, "#/components/responses/"+name)
	}
	for name, param := range in.Parameters {
		pv := p.parameter(*param)
		comp.Parameters[name] = &pv
	}
	for name, ex := range in.Examples {
		comp.Examples[name] = p.example(ex)
	}
	for name, rb := range in.RequestBodies {
		comp.RequestBodies[name] = p.requestBody(rb)
	}
	for name, h := range in.Headers {
		comp.Headers[name] = p.header(h, "#/components/headers/"+name)
	}
	for name, ss := range in.SecuritySchemes {
		// Skip mutualTLS schemes (3.1-only feature)
		if ss.Type == "mutualTLS" {
			mutualTLSSchemes = append(mutualTLSSchemes, name)
			continue
		}
		comp.SecuritySchemes[name] = p.securityScheme(ss)
	}
	for name, link := range in.Links {
		comp.Links[name] = p.link(link)
	}
	for name, cb := range in.Callbacks {
		comp.Callbacks[name] = p.callback(cb)
	}
	// PathItems is 3.1-only, drop with warning if present
	if len(in.PathItems) > 0 {
		p.warn(diag.WarnDownlevelPathItems, "#/components/pathItems",
			"pathItems in components are 3.1-only; dropped")
	}
	comp.Extensions = p.ext(in.Extensions)

	return comp, mutualTLSSchemes
}

func (p *proj30) callback(in *model.Callback) *CallbackV30 {
	if in == nil {
		return nil
	}
	// Handle $ref case
	if in.Ref != "" {
		return &CallbackV30{Ref: in.Ref}
	}

	cb := &CallbackV30{
		PathItems: make(map[string]*PathItemV30, len(in.PathItems)),
	}
	for path, item := range in.PathItems {
		cb.PathItems[path] = p.pathItem(item)
	}
	cb.Extensions = p.ext(in.Extensions)

	return cb
}

func (p *proj30) securityScheme(in *model.SecurityScheme) *SecuritySchemeV30 {
	// Handle $ref case
	if in.Ref != "" {
		return &SecuritySchemeV30{Ref: in.Ref}
	}

	out := &SecuritySchemeV30{
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

func (p *proj30) oAuthFlows(in *model.OAuthFlows) *OAuthFlowsV30 {
	out := &OAuthFlowsV30{}
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

func (p *proj30) oAuthFlow(in *model.OAuthFlow) *OAuthFlowV30 {
	out := &OAuthFlowV30{
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

func (p *proj30) externalDocs(in *model.ExternalDocs) *ExternalDocsV30 {
	if in == nil {
		return nil
	}
	ed := &ExternalDocsV30{
		Description: in.Description,
		URL:         in.URL,
	}
	ed.Extensions = p.ext(in.Extensions)

	return ed
}
