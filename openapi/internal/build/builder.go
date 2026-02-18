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

// Package build provides OpenAPI specification building from route metadata.
//
// The Builder type accumulates API metadata (servers, tags, security schemes)
// and generates version-agnostic specifications from enriched route information.
// The specifications can then be projected to any OpenAPI version using the export package.
package build

import (
	"fmt"
	"maps"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"rivaas.dev/openapi/internal/model"
	"rivaas.dev/openapi/internal/schema"
)

// Builder builds OpenAPI specifications from enriched route information.
//
// A Builder accumulates API metadata (servers, tags, security schemes) and
// generates a version-agnostic specification from route definitions.
// The specification can then be projected to any OpenAPI version using the export package.
type Builder struct {
	info            model.Info
	servers         []model.Server
	tags            []model.Tag
	securitySchemes map[string]*model.SecurityScheme
	globalSecurity  []model.SecurityRequirement
	externalDocs    *model.ExternalDocs
}

// NewBuilder creates a new builder with the given API info.
func NewBuilder(info model.Info) *Builder {
	return &Builder{
		info:            info,
		securitySchemes: map[string]*model.SecurityScheme{},
	}
}

// AddServer adds a server URL to the specification.
func (b *Builder) AddServer(url, desc string) *Builder {
	server := model.Server{URL: url, Description: desc}
	b.servers = append(b.servers, server)

	return b
}

// AddServerWithExtensions adds a server URL with extensions to the specification.
func (b *Builder) AddServerWithExtensions(url, desc string, extensions map[string]any) *Builder {
	server := model.Server{URL: url, Description: desc}
	if len(extensions) > 0 {
		server.Extensions = make(map[string]any, len(extensions))
		maps.Copy(server.Extensions, extensions)
	}
	b.servers = append(b.servers, server)

	return b
}

// AddServerVariable adds a variable to the last added server for URL template substitution.
//
// IMPORTANT: AddServerVariable must be called AFTER AddServer. It applies to the most
// recently added server. Validation occurs when Build() is called.
func (b *Builder) AddServerVariable(name string, variable *model.ServerVariable) *Builder {
	if len(b.servers) == 0 {
		// Create a placeholder server - validation will catch this in Build()
		b.servers = append(b.servers, model.Server{})
	}
	server := &b.servers[len(b.servers)-1]
	if server.Variables == nil {
		server.Variables = make(map[string]*model.ServerVariable)
	}
	server.Variables[name] = variable

	return b
}

// AddTag adds a tag definition to the specification.
func (b *Builder) AddTag(name, desc string) *Builder {
	b.tags = append(b.tags, model.Tag{Name: name, Description: desc})
	return b
}

// AddTagWithExtensions adds a tag definition with extensions to the specification.
func (b *Builder) AddTagWithExtensions(name, desc string, extensions map[string]any) *Builder {
	tag := model.Tag{Name: name, Description: desc}
	if len(extensions) > 0 {
		tag.Extensions = make(map[string]any, len(extensions))
		maps.Copy(tag.Extensions, extensions)
	}
	b.tags = append(b.tags, tag)

	return b
}

// AddTagWithExternalDocs adds a tag definition with external docs and extensions.
func (b *Builder) AddTagWithExternalDocs(name, desc string, extDocs *model.ExternalDocs, extensions map[string]any) *Builder {
	tag := model.Tag{Name: name, Description: desc, ExternalDocs: extDocs}
	if len(extensions) > 0 {
		tag.Extensions = make(map[string]any, len(extensions))
		maps.Copy(tag.Extensions, extensions)
	}
	b.tags = append(b.tags, tag)

	return b
}

// AddSecurityScheme adds a security scheme to the specification.
func (b *Builder) AddSecurityScheme(name string, s *model.SecurityScheme) *Builder {
	b.securitySchemes[name] = s
	return b
}

// SetGlobalSecurity sets global security requirements applied to all operations.
func (b *Builder) SetGlobalSecurity(reqs []model.SecurityRequirement) *Builder {
	b.globalSecurity = reqs
	return b
}

// SetExternalDocs sets external documentation links.
func (b *Builder) SetExternalDocs(docs *model.ExternalDocs) *Builder {
	b.externalDocs = docs
	return b
}

// Build builds the complete specification from enriched routes.
func (b *Builder) Build(routes []EnrichedRoute) (*model.Spec, error) {
	// Validate servers: variables require a server URL
	for i, server := range b.servers {
		if len(server.Variables) > 0 && server.URL == "" {
			return nil, fmt.Errorf("server[%d]: variables require a server URL - call AddServer before AddServerVariable", i)
		}
	}

	spec := &model.Spec{
		Info:         b.info,
		Servers:      b.servers,
		Tags:         b.tags,
		Paths:        map[string]*model.PathItem{},
		Security:     b.globalSecurity,
		ExternalDocs: b.externalDocs,
		Components: &model.Components{
			Schemas:         map[string]*model.Schema{},
			SecuritySchemes: b.securitySchemes,
		},
	}

	sg := schema.NewSchemaGenerator()

	// Group routes by path
	byPath := map[string][]EnrichedRoute{}
	for _, r := range routes {
		p := convertPath(r.RouteInfo.Path)
		byPath[p] = append(byPath[p], r)
	}

	seenOps := map[string]int{}

	for path, group := range byPath {
		item := &model.PathItem{}

		for _, r := range group {
			op, err := b.buildOperation(r, sg, seenOps)
			if err != nil {
				return nil, fmt.Errorf("failed to build operation for %s %s: %w", r.RouteInfo.Method, r.RouteInfo.Path, err)
			}

			switch strings.ToUpper(r.RouteInfo.Method) {
			case http.MethodGet:
				item.Get = op
			case http.MethodPost:
				item.Post = op
			case http.MethodPut:
				item.Put = op
			case http.MethodDelete:
				item.Delete = op
			case http.MethodPatch:
				item.Patch = op
			case http.MethodOptions:
				item.Options = op
			case http.MethodHead:
				item.Head = op
			}
		}

		spec.Paths[path] = item
	}

	// Add component schemas
	spec.Components.Schemas = sg.GetComponentSchemas()

	sortSpec(spec)

	return spec, nil
}

// buildOperation builds an Operation from an enriched route.
func (b *Builder) buildOperation(er EnrichedRoute, sg *schema.SchemaGenerator, seen map[string]int) (*model.Operation, error) {
	op := &model.Operation{
		Responses:  map[string]*model.Response{},
		Parameters: []model.Parameter{},
	}

	ri, doc := er.RouteInfo, er.Doc

	// Baseline OperationID
	opID := generateOperationID(ri)

	if doc != nil && doc.OperationID != "" {
		opID = doc.OperationID
	}

	// Ensure uniqueness - return error if duplicate found
	if seen[opID] > 0 {
		return nil, fmt.Errorf("duplicate operation ID: %s (used by %s %s and another route)", opID, ri.Method, ri.Path)
	}
	seen[opID] = 1
	op.OperationID = opID

	if doc == nil {
		op.Responses["200"] = &model.Response{Description: "OK"}
		// Extract path params from route as fallback
		op.Parameters = append(op.Parameters, extractPathParams(ri.Path, ri.PathConstraints)...)

		return op, nil
	}

	op.Summary = doc.Summary
	op.Description = doc.Description
	op.Tags = doc.Tags
	op.Deprecated = doc.Deprecated

	// Copy operation extensions
	if len(doc.Extensions) > 0 {
		op.Extensions = make(map[string]any, len(doc.Extensions))
		maps.Copy(op.Extensions, doc.Extensions)
	}

	for _, s := range doc.Security {
		op.Security = append(op.Security, model.SecurityRequirement{s.Scheme: s.Scopes})
	}

	// Extract path parameters from route with typed constraints
	pathParams := extractPathParams(ri.Path, ri.PathConstraints)

	// Parameters from request metadata (query, path, header, cookie)
	if md := doc.RequestMetadata; md != nil {
		// Track which path params are already in metadata
		seenPathParams := make(map[string]bool)
		for _, p := range md.Parameters {
			if p.In == "path" {
				seenPathParams[p.Name] = true
			}
			op.Parameters = append(op.Parameters, paramSpecToParameter(p, sg))
		}

		// Add any path params from route that weren't in metadata
		for _, p := range pathParams {
			if !seenPathParams[p.Name] {
				op.Parameters = append(op.Parameters, p)
			}
		}
	} else {
		// No metadata - use path params from route
		op.Parameters = append(op.Parameters, pathParams...)
	}

	// Request body: project only JSON-tagged fields
	if md := doc.RequestMetadata; md != nil && md.HasBody {
		ct := first(doc.Consumes, "application/json")
		bodySchema := sg.GenerateProjected(doc.RequestType, func(f reflect.StructField) bool {
			jt := f.Tag.Get("json")
			return jt != "" && jt != "-"
		})

		mt := &model.MediaType{
			Schema: bodySchema,
		}

		// Handle examples: single example OR named examples (mutually exclusive)
		if len(doc.RequestNamedExamples) > 0 {
			// Use named examples (plural "examples" field)
			mt.Examples = make(map[string]*model.Example, len(doc.RequestNamedExamples))
			for _, ex := range doc.RequestNamedExamples {
				example := &model.Example{
					Summary:       ex.Summary,
					Description:   ex.Description,
					Value:         ex.Value,
					ExternalValue: ex.ExternalValue,
				}
				mt.Examples[ex.Name] = example
			}
		} else if doc.RequestExample != nil {
			// Use single example (singular "example" field)
			mt.Example = doc.RequestExample
		}

		op.RequestBody = &model.RequestBody{
			Required: true,
			Content: map[string]*model.MediaType{
				ct: mt,
			},
		}
	}

	// Responses
	outCT := first(doc.Produces, "application/json")
	for status, rt := range doc.ResponseTypes {
		rs := &model.Response{Description: httpStatusText(status)}

		if status != 204 && rt != nil {
			mt := &model.MediaType{
				Schema: sg.Generate(rt),
			}

			// Handle examples: single example OR named examples (mutually exclusive)
			if namedExamples, hasNamed := doc.ResponseNamedExamples[status]; hasNamed && len(namedExamples) > 0 {
				// Use named examples (plural "examples" field)
				mt.Examples = make(map[string]*model.Example, len(namedExamples))
				for _, ex := range namedExamples {
					example := &model.Example{
						Summary:       ex.Summary,
						Description:   ex.Description,
						Value:         ex.Value,
						ExternalValue: ex.ExternalValue,
					}
					mt.Examples[ex.Name] = example
				}
			} else if singleExample, hasSingle := doc.ResponseExample[status]; hasSingle {
				// Use single example (singular "example" field)
				mt.Example = singleExample
			}

			rs.Content = map[string]*model.MediaType{
				outCT: mt,
			}
		}

		op.Responses[strconv.Itoa(status)] = rs
	}

	if len(op.Responses) == 0 {
		op.Responses[strconv.Itoa(http.StatusOK)] = &model.Response{Description: httpStatusText(http.StatusOK)}
	}

	return op, nil
}

// paramSpecToParameter converts a ParamSpec to a Parameter.
func paramSpecToParameter(ps schema.ParamSpec, sg *schema.SchemaGenerator) model.Parameter {
	s := sg.Generate(ps.Type)

	if ps.Default != nil {
		s.Default = ps.Default
	}

	if len(ps.Enum) > 0 {
		s.Enum = make([]any, 0, len(ps.Enum))
		for _, v := range ps.Enum {
			s.Enum = append(s.Enum, v)
		}
	}

	if ps.Format != "" {
		s.Format = ps.Format
	}

	param := model.Parameter{
		Name:        ps.Name,
		In:          ps.In,
		Description: ps.Description,
		Required:    ps.Required,
		Schema:      s,
		Example:     ps.Example,
	}

	// Apply style if specified
	if ps.Style != "" {
		param.Style = ps.Style
	}

	// Apply explode if explicitly set
	if ps.Explode != nil {
		param.Explode = *ps.Explode
	}

	return param
}

// extractPathParams extracts path parameters from a route path with optional type constraints.
func extractPathParams(path string, constraints map[string]PathConstraint) []model.Parameter {
	var out []model.Parameter

	for seg := range strings.SplitSeq(path, "/") {
		if name, found := strings.CutPrefix(seg, ":"); found {
			param := model.Parameter{
				Name:     name,
				In:       "path",
				Required: true,
				Schema:   constraintToSchema(constraints[name]),
			}
			out = append(out, param)
		}
	}

	return out
}

// constraintToSchema converts a PathConstraint to an OpenAPI schema.
func constraintToSchema(c PathConstraint) *model.Schema {
	switch c.Kind {
	case ConstraintInt:
		return &model.Schema{Kind: model.KindInteger, Format: "int64"}
	case ConstraintFloat:
		return &model.Schema{Kind: model.KindNumber, Format: "double"}
	case ConstraintUUID:
		return &model.Schema{Kind: model.KindString, Format: "uuid"}
	case ConstraintDate:
		return &model.Schema{Kind: model.KindString, Format: "date"}
	case ConstraintDateTime:
		return &model.Schema{Kind: model.KindString, Format: "date-time"}
	case ConstraintRegex:
		return &model.Schema{Kind: model.KindString, Pattern: c.Pattern}
	case ConstraintEnum:
		enum := make([]any, 0, len(c.Enum))
		for _, v := range c.Enum {
			enum = append(enum, v)
		}

		return &model.Schema{Kind: model.KindString, Enum: enum}
	default:
		return &model.Schema{Kind: model.KindString}
	}
}

// convertPath converts a router path pattern to OpenAPI path format.
func convertPath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if after, found := strings.CutPrefix(part, ":"); found {
			parts[i] = "{" + after + "}"
		}
	}

	return strings.Join(parts, "/")
}

// generateOperationID generates a semantic operation ID from method and path.
func generateOperationID(ri RouteInfo) string {
	return generateFromMethodAndPath(ri.Method, ri.Path)
}

// generateFromMethodAndPath creates a semantic operationId from HTTP method and path.
func generateFromMethodAndPath(method, path string) string {
	// Normalize method to uppercase for consistent comparison
	method = strings.ToUpper(method)
	verb := methodToVerb(method)

	// Parse path segments
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) == 0 || (len(segments) == 1 && segments[0] == "") {
		// Root path or empty
		return verb + "Root"
	}

	var resourceParts []string
	var lastParam string

	// Process segments to build resource hierarchy
	for i := range segments {
		seg := segments[i]
		if seg == "" {
			continue
		}

		if strings.HasPrefix(seg, ":") {
			// It's a path parameter - store for "By{Param}" suffix
			// The resource name was already added in the else branch
			lastParam = seg[1:] // Remove ':'
		} else {
			// It's a resource name
			// Check if next segment is a parameter
			if i+1 < len(segments) && strings.HasPrefix(segments[i+1], ":") {
				// Next is a parameter, so this is a specific item (use singular)
				singular := singularize(seg)
				resourceParts = append(resourceParts, capitalize(singular))
			} else {
				// No parameter after, it's a collection or action endpoint
				// Use plural for GET/DELETE (collections), singular for others (actions)
				if method == http.MethodGet || method == http.MethodDelete {
					resourceParts = append(resourceParts, capitalize(seg))
				} else {
					resourceParts = append(resourceParts, capitalize(singularize(seg)))
				}
			}
		}
	}

	// Build the base operationId
	result := verb + strings.Join(resourceParts, "")

	// Add parameter suffix if we have a specific parameter
	if lastParam != "" {
		// For generic "id", use "ById"
		// For specific params like "orderId", use "ByOrderId"
		result += "By" + capitalize(lastParam)
	}

	return result
}

// methodToVerb converts HTTP method to semantic verb.
func methodToVerb(method string) string {
	switch strings.ToUpper(method) {
	case http.MethodGet:
		return "get"
	case http.MethodPost:
		return "create"
	case http.MethodPut:
		return "replace"
	case http.MethodPatch:
		return "update"
	case http.MethodDelete:
		return "delete"
	case http.MethodHead:
		return "head"
	case http.MethodOptions:
		return "options"
	default:
		return strings.ToLower(method)
	}
}

// singularize converts plural words to singular (simple implementation).
func singularize(word string) string {
	if len(word) == 0 {
		return word
	}

	// Handle common plural patterns
	if strings.HasSuffix(word, "ies") && len(word) > 3 {
		return word[:len(word)-3] + "y" // cities -> city
	}
	if strings.HasSuffix(word, "ses") && len(word) > 3 {
		return word[:len(word)-2] // classes -> class
	}
	if strings.HasSuffix(word, "ches") && len(word) > 4 {
		return word[:len(word)-2] // matches -> match
	}
	if strings.HasSuffix(word, "xes") && len(word) > 3 {
		return word[:len(word)-2] // boxes -> box
	}
	if strings.HasSuffix(word, "s") && len(word) > 1 {
		return word[:len(word)-1] // users -> user
	}

	return word
}

// capitalize capitalizes the first letter of a string.
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])

	return string(runes)
}

// httpStatusText returns a description for an HTTP status code.
func httpStatusText(code int) string {
	if text := http.StatusText(code); text != "" {
		return text
	}

	return "Response"
}

// first returns the first element in a slice, or a default if empty.
func first[S ~[]E, E any](s S, def E) E {
	if len(s) > 0 {
		return s[0]
	}

	return def
}

// sortSpec sorts paths, tags, and components for deterministic output.
func sortSpec(s *model.Spec) {
	// Sort paths
	paths := make([]string, 0, len(s.Paths))
	for p := range s.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	// Create sorted paths map
	sortedPaths := make(map[string]*model.PathItem, len(paths))
	for _, p := range paths {
		sortedPaths[p] = s.Paths[p]
	}
	s.Paths = sortedPaths

	// Sort tags
	sort.Slice(s.Tags, func(i, j int) bool {
		return s.Tags[i].Name < s.Tags[j].Name
	})

	// Sort component schemas
	if s.Components != nil && s.Components.Schemas != nil {
		schemaNames := make([]string, 0, len(s.Components.Schemas))
		for n := range s.Components.Schemas {
			schemaNames = append(schemaNames, n)
		}
		sort.Strings(schemaNames)

		sortedSchemas := make(map[string]*model.Schema, len(schemaNames))
		for _, n := range schemaNames {
			sortedSchemas[n] = s.Components.Schemas[n]
		}
		s.Components.Schemas = sortedSchemas
	}
}
