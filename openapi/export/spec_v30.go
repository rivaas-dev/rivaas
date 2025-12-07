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

	"rivaas.dev/openapi/model"
)

// projectTo30 projects a spec to OpenAPI 3.0.4 format.
func projectTo30(in *model.Spec, cfg Config) (*SpecV30, []Warning, error) {
	warns := []Warning{}

	// 3.0 MUST have paths
	if len(in.Paths) == 0 {
		return nil, warns, errors.New("OpenAPI 3.0 requires 'paths'")
	}

	info := info30(in.Info, &warns)
	out := &SpecV30{
		OpenAPI: "3.0.4",
		Info:    &info,
		Servers: servers30(in.Servers),
		Paths:   paths30(in.Paths, &warns),
		Tags:    tags30(in.Tags),
	}

	// Check for summary (3.1-only field) in strict mode
	if in.Info.Summary != "" && cfg.StrictDownlevel {
		return nil, warns, errors.New("info.summary not supported in OpenAPI 3.0")
	}

	var mutualTLSSchemes []string
	if in.Components != nil {
		out.Components, mutualTLSSchemes = components30(in.Components, &warns)
		// Warn about mutualTLS schemes (3.1-only feature)
		for _, name := range mutualTLSSchemes {
			warns = append(warns, Warning{
				Code:    DownlevelMutualTLS,
				Path:    "#/components/securitySchemes/" + name,
				Message: "mutualTLS security type is 3.1-only; dropped",
			})
		}
		if len(mutualTLSSchemes) > 0 && cfg.StrictDownlevel {
			return nil, warns, errors.New("mutualTLS security type not supported in OpenAPI 3.0")
		}
	}

	if len(in.Security) > 0 {
		out.Security = security30(in.Security)
	}

	if in.ExternalDocs != nil {
		out.ExternalDocs = externalDocs30(in.ExternalDocs)
	}

	// Copy extensions
	out.Extensions = copyExtensions(in.Extensions, "3.0.4")

	// Webhooks are not in 3.0: warn if present
	if len(in.Webhooks) > 0 {
		warns = append(warns, Warning{
			Code:    DownlevelWebhooks,
			Path:    "#/webhooks",
			Message: "webhooks are 3.1-only; dropped",
		})
		if cfg.StrictDownlevel {
			return nil, warns, errors.New("webhooks not supported in OpenAPI 3.0")
		}
	}

	return out, warns, nil
}

func info30(in model.Info, warns *[]Warning) InfoV30 {
	info := InfoV30{
		Title:          in.Title,
		Description:    in.Description,
		TermsOfService: in.TermsOfService,
		Version:        in.Version,
	}
	// Drop summary if present (3.1-only field)
	if in.Summary != "" {
		*warns = append(*warns, Warning{
			Code:    DownlevelInfoSummary,
			Path:    "#/info/summary",
			Message: "info.summary is 3.1-only; dropped",
		})
	}
	if in.Contact != nil {
		info.Contact = &ContactV30{
			Name:  in.Contact.Name,
			URL:   in.Contact.URL,
			Email: in.Contact.Email,
		}
		info.Contact.Extensions = copyExtensions(in.Contact.Extensions, "3.0.4")
	}
	if in.License != nil {
		info.License = &LicenseV30{
			Name: in.License.Name,
			URL:  in.License.URL,
		}
		// Warn if identifier is present (3.1-only feature)
		if in.License.Identifier != "" {
			*warns = append(*warns, Warning{
				Code:    DownlevelLicenseIdentifier,
				Path:    "#/info/license",
				Message: "license identifier is 3.1-only; dropped (use url instead)",
			})
		}
		info.License.Extensions = copyExtensions(in.License.Extensions, "3.0.4")
	}
	info.Extensions = copyExtensions(in.Extensions, "3.0.4")
	return info
}

func servers30(in []model.Server) []ServerV30 {
	out := make([]ServerV30, len(in))
	for i, s := range in {
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
				server.Variables[name].Extensions = copyExtensions(v.Extensions, "3.0.4")
			}
		}
		server.Extensions = copyExtensions(s.Extensions, "3.0.4")
		out[i] = server
	}
	return out
}

func tags30(in []model.Tag) []TagV30 {
	out := make([]TagV30, len(in))
	for i, t := range in {
		tag := TagV30{
			Name:        t.Name,
			Description: t.Description,
		}
		if t.ExternalDocs != nil {
			tag.ExternalDocs = externalDocs30(t.ExternalDocs)
		}
		tag.Extensions = copyExtensions(t.Extensions, "3.0.4")
		out[i] = tag
	}
	return out
}

func security30(in []model.SecurityRequirement) []SecurityRequirementV30 {
	out := make([]SecurityRequirementV30, len(in))
	for i, s := range in {
		out[i] = SecurityRequirementV30(s)
	}
	return out
}

func paths30(in map[string]*model.PathItem, warns *[]Warning) map[string]*PathItemV30 {
	out := make(map[string]*PathItemV30, len(in))
	for path, item := range in {
		out[path] = pathItem30(item, warns)
	}
	return out
}

func pathItem30(in *model.PathItem, warns *[]Warning) *PathItemV30 {
	item := &PathItemV30{
		Summary:     in.Summary,
		Description: in.Description,
		Parameters:  parameters30(in.Parameters, warns),
	}
	if in.Get != nil {
		item.Get = operation30(in.Get, warns)
	}
	if in.Put != nil {
		item.Put = operation30(in.Put, warns)
	}
	if in.Post != nil {
		item.Post = operation30(in.Post, warns)
	}
	if in.Delete != nil {
		item.Delete = operation30(in.Delete, warns)
	}
	if in.Options != nil {
		item.Options = operation30(in.Options, warns)
	}
	if in.Head != nil {
		item.Head = operation30(in.Head, warns)
	}
	if in.Patch != nil {
		item.Patch = operation30(in.Patch, warns)
	}
	item.Extensions = copyExtensions(in.Extensions, "3.0.4")
	return item
}

func operation30(in *model.Operation, warns *[]Warning) *OperationV30 {
	op := &OperationV30{
		Tags:        append([]string(nil), in.Tags...),
		Summary:     in.Summary,
		Description: in.Description,
		OperationID: in.OperationID,
		Deprecated:  in.Deprecated,
		Parameters:  parameters30(in.Parameters, warns),
		Responses:   responses30(in.Responses, warns),
	}
	if in.RequestBody != nil {
		op.RequestBody = requestBody30(in.RequestBody, warns)
	}
	if len(in.Security) > 0 {
		op.Security = security30(in.Security)
	}
	op.Extensions = copyExtensions(in.Extensions, "3.0.4")
	return op
}

func parameters30(in []model.Parameter, warns *[]Warning) []ParameterV30 {
	out := make([]ParameterV30, len(in))
	for i, p := range in {
		out[i] = parameter30(p, warns)
	}
	return out
}

func parameter30(in model.Parameter, warns *[]Warning) ParameterV30 {
	p := ParameterV30{
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
		p.Schema = schema30(in.Schema, warns, "#/parameters/"+in.Name)
	}
	if len(in.Examples) > 0 {
		p.Examples = make(map[string]*ExampleV30, len(in.Examples))
		for k, v := range in.Examples {
			p.Examples[k] = example30(v)
		}
	}
	if len(in.Content) > 0 {
		p.Content = make(map[string]*MediaTypeV30, len(in.Content))
		for ct, mt := range in.Content {
			p.Content[ct] = mediaType30(mt, warns, "#/parameters/"+in.Name+"/content/"+ct)
		}
	}
	p.Extensions = copyExtensions(in.Extensions, "3.0.4")
	return p
}

func example30(in *model.Example) *ExampleV30 {
	if in == nil {
		return nil
	}
	ex := &ExampleV30{
		Summary:       in.Summary,
		Description:   in.Description,
		Value:         in.Value,
		ExternalValue: in.ExternalValue,
	}
	ex.Extensions = copyExtensions(in.Extensions, "3.0.4")
	return ex
}

func requestBody30(in *model.RequestBody, warns *[]Warning) *RequestBodyV30 {
	rb := &RequestBodyV30{
		Description: in.Description,
		Required:    in.Required,
		Content:     make(map[string]*MediaTypeV30, len(in.Content)),
	}
	for ct, mt := range in.Content {
		rb.Content[ct] = mediaType30(mt, warns, "#/requestBody/content/"+ct)
	}
	rb.Extensions = copyExtensions(in.Extensions, "3.0.4")
	return rb
}

func responses30(in map[string]*model.Response, warns *[]Warning) map[string]*ResponseV30 {
	out := make(map[string]*ResponseV30, len(in))
	for code, r := range in {
		out[code] = response30(r, warns, "#/responses/"+code)
	}
	return out
}

func response30(in *model.Response, warns *[]Warning, path string) *ResponseV30 {
	r := &ResponseV30{
		Description: in.Description,
		Content:     make(map[string]*MediaTypeV30, len(in.Content)),
	}
	for ct, mt := range in.Content {
		r.Content[ct] = mediaType30(mt, warns, path+"/content/"+ct)
	}
	if len(in.Headers) > 0 {
		r.Headers = make(map[string]*HeaderV30, len(in.Headers))
		for name, h := range in.Headers {
			r.Headers[name] = header30(h, warns, path+"/headers/"+name)
		}
	}
	if len(in.Links) > 0 {
		r.Links = make(map[string]*LinkV30, len(in.Links))
		for name, link := range in.Links {
			r.Links[name] = link30(link)
		}
	}
	r.Extensions = copyExtensions(in.Extensions, "3.0.4")
	return r
}

func mediaType30(in *model.MediaType, warns *[]Warning, path string) *MediaTypeV30 {
	if in == nil {
		return nil
	}
	mt := &MediaTypeV30{
		Example: in.Example,
	}
	if in.Schema != nil {
		mt.Schema = schema30(in.Schema, warns, path+"/schema")
	}
	if len(in.Examples) > 0 {
		mt.Examples = make(map[string]*ExampleV30, len(in.Examples))
		for k, ex := range in.Examples {
			mt.Examples[k] = example30(ex)
		}
	}
	if len(in.Encoding) > 0 {
		mt.Encoding = make(map[string]*EncodingV30, len(in.Encoding))
		for k, enc := range in.Encoding {
			mt.Encoding[k] = encoding30(enc, warns)
		}
	}
	mt.Extensions = copyExtensions(in.Extensions, "3.0.4")
	return mt
}

func encoding30(in *model.Encoding, warns *[]Warning) *EncodingV30 {
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
			enc.Headers[k] = header30(h, warns, "#/encoding/headers/"+k)
		}
	}
	enc.Extensions = copyExtensions(in.Extensions, "3.0.4")
	return enc
}

func header30(in *model.Header, warns *[]Warning, path string) *HeaderV30 {
	if in == nil {
		return nil
	}
	h := &HeaderV30{
		Description: in.Description,
		Required:    in.Required,
		Deprecated:  in.Deprecated,
		Style:       in.Style,
		Explode:     in.Explode,
		Example:     in.Example,
	}
	if in.Schema != nil {
		h.Schema = schema30(in.Schema, warns, path+"/schema")
	}
	if len(in.Examples) > 0 {
		h.Examples = make(map[string]*ExampleV30, len(in.Examples))
		for k, ex := range in.Examples {
			h.Examples[k] = example30(ex)
		}
	}
	if len(in.Content) > 0 {
		h.Content = make(map[string]*MediaTypeV30, len(in.Content))
		for ct, mt := range in.Content {
			h.Content[ct] = mediaType30(mt, warns, path+"/content/"+ct)
		}
	}
	h.Extensions = copyExtensions(in.Extensions, "3.0.4")
	return h
}

func link30(in *model.Link) *LinkV30 {
	if in == nil {
		return nil
	}
	link := &LinkV30{
		OperationRef: in.OperationRef,
		OperationID:  in.OperationID,
		Parameters:   in.Parameters,
		RequestBody:  in.RequestBody,
		Description:  in.Description,
	}
	if in.Server != nil {
		servers := servers30([]model.Server{*in.Server})
		if len(servers) > 0 {
			link.Server = &servers[0]
		}
	}
	link.Extensions = copyExtensions(in.Extensions, "3.0.4")
	return link
}

func components30(in *model.Components, warns *[]Warning) (*ComponentsV30, []string) {
	comp := &ComponentsV30{
		Schemas:         make(map[string]*SchemaV30, len(in.Schemas)),
		SecuritySchemes: make(map[string]*SecuritySchemeV30),
	}
	var mutualTLSSchemes []string

	for name, s := range in.Schemas {
		comp.Schemas[name] = schema30(s, warns, "#/components/schemas/"+name)
	}
	for name, ss := range in.SecuritySchemes {
		// Skip mutualTLS schemes (3.1-only feature)
		if ss.Type == "mutualTLS" {
			mutualTLSSchemes = append(mutualTLSSchemes, name)
			continue
		}
		comp.SecuritySchemes[name] = securityScheme30(ss)
	}
	comp.Extensions = copyExtensions(in.Extensions, "3.0.4")
	return comp, mutualTLSSchemes
}

func securityScheme30(in *model.SecurityScheme) *SecuritySchemeV30 {
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
		out.Flows = oAuthFlows30(in.Flows)
	}
	out.Extensions = copyExtensions(in.Extensions, "3.0.4")
	return out
}

func oAuthFlows30(in *model.OAuthFlows) *OAuthFlowsV30 {
	out := &OAuthFlowsV30{}
	if in.Implicit != nil {
		out.Implicit = oAuthFlow30(in.Implicit)
	}
	if in.Password != nil {
		out.Password = oAuthFlow30(in.Password)
	}
	if in.ClientCredentials != nil {
		out.ClientCredentials = oAuthFlow30(in.ClientCredentials)
	}
	if in.AuthorizationCode != nil {
		out.AuthorizationCode = oAuthFlow30(in.AuthorizationCode)
	}
	// Return nil if no flows are set
	if out.Implicit == nil && out.Password == nil && out.ClientCredentials == nil && out.AuthorizationCode == nil {
		return nil
	}
	out.Extensions = copyExtensions(in.Extensions, "3.0.4")
	return out
}

func oAuthFlow30(in *model.OAuthFlow) *OAuthFlowV30 {
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
	out.Extensions = copyExtensions(in.Extensions, "3.0.4")
	return out
}

func externalDocs30(in *model.ExternalDocs) *ExternalDocsV30 {
	if in == nil {
		return nil
	}
	ed := &ExternalDocsV30{
		Description: in.Description,
		URL:         in.URL,
	}
	ed.Extensions = copyExtensions(in.Extensions, "3.0.4")
	return ed
}
