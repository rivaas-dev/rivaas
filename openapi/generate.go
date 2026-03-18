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

import (
	"context"
	"fmt"
	"maps"

	"rivaas.dev/openapi/internal/build"
	"rivaas.dev/openapi/internal/export"
	"rivaas.dev/openapi/internal/schema"
	"rivaas.dev/openapi/validate"
)

// Shared validator instance for all generation (compiled once, reused)
var sharedValidator = validate.MustNew()

// Spec produces an OpenAPI specification from the API's current configuration and
// operations (from [WithOperations] and/or [API.AddOperation]). Pure function of
// current API state; no side effects. Caching is the caller's responsibility.
//
// Example:
//
//	api := openapi.MustNew(
//	    openapi.WithTitle("My API", "1.0.0"),
//	    openapi.WithOperations(
//	        openapi.WithGET("/users/:id", openapi.WithSummary("Get user"), openapi.WithResponse(200, User{})),
//	        openapi.WithPOST("/users", openapi.WithSummary("Create user"), openapi.WithRequest(CreateUserRequest{}), openapi.WithResponse(201, User{})),
//	    ),
//	)
//	spec, err := api.Spec(ctx)
//	// or: api.AddOperation(openapi.WithGET(...)); spec, err := api.Spec(ctx)
func (a *API) Spec(ctx context.Context) (*Result, error) {
	a.operationsMu.RLock()
	ops := make([]Operation, 0, len(a.operations))
	ops = append(ops, a.operations...)
	a.operationsMu.RUnlock()

	builder := createBuilder(a)
	enriched := make([]build.EnrichedRoute, 0, len(ops))
	for _, op := range ops {
		enriched = append(enriched, convertOperation(op))
	}

	// Build spec
	spec, err := builder.Build(enriched)
	if err != nil {
		return nil, fmt.Errorf("failed to build OpenAPI spec: %w", err)
	}

	// Copy extensions from API to model Spec
	if len(a.extensions) > 0 {
		spec.Extensions = make(map[string]any, len(a.extensions))
		maps.Copy(spec.Extensions, a.extensions)
	}

	// Project to target version
	var exportVersion export.Version
	switch a.version {
	case V31x:
		exportVersion = export.V31
	case V30x:
		exportVersion = export.V30
	default:
		exportVersion = export.V30 // Default to 3.0.4
	}
	exportCfg := export.Config{
		Version:         exportVersion,
		StrictDownlevel: a.strictDownlevel,
	}

	// Enable validation if configured (use shared validator for performance)
	if a.validateSpec {
		exportCfg.Validator = sharedValidator
	}

	result, err := export.Project(ctx, spec, exportCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to project OpenAPI spec: %w", err)
	}

	return &Result{
		JSON:     result.JSON,
		YAML:     result.YAML,
		Warnings: result.Warnings,
	}, nil
}

// AddOperation adds one or more operations to the API. Safe for concurrent use.
// Call [Spec] to generate the spec including these operations.
func (a *API) AddOperation(ops ...Operation) {
	if len(ops) == 0 {
		return
	}
	a.operationsMu.Lock()
	defer a.operationsMu.Unlock()
	a.operations = append(a.operations, ops...)
}

// createBuilder creates a Builder from API.
func createBuilder(a *API) *build.Builder {
	b := build.NewBuilder(a.info)

	if a.externalDocs != nil {
		b.SetExternalDocs(a.externalDocs)
	}

	for _, srv := range a.servers {
		if len(srv.Extensions) > 0 {
			b.AddServerWithExtensions(srv.URL, srv.Description, srv.Extensions)
		} else {
			b.AddServer(srv.URL, srv.Description)
		}
		for name, v := range srv.Variables {
			b.AddServerVariable(name, v)
		}
	}

	for _, tag := range a.tags {
		if tag.ExternalDocs != nil {
			b.AddTagWithExternalDocs(tag.Name, tag.Description, tag.ExternalDocs, tag.Extensions)
		} else if len(tag.Extensions) > 0 {
			b.AddTagWithExtensions(tag.Name, tag.Description, tag.Extensions)
		} else {
			b.AddTag(tag.Name, tag.Description)
		}
	}

	for name, ss := range a.securitySchemes {
		b.AddSecurityScheme(name, ss)
	}

	if len(a.defaultSecurity) > 0 {
		b.SetGlobalSecurity(a.defaultSecurity)
	}

	return b
}

// convertOperation converts an Operation to build.EnrichedRoute.
func convertOperation(op Operation) build.EnrichedRoute {
	var buildDoc *build.RouteDoc

	// Check if there's meaningful documentation
	if op.doc.Summary != "" || op.doc.Description != "" || len(op.doc.ResponseTypes) > 0 {
		// Convert request examples
		requestNamedExamples := make([]build.ExampleData, 0, len(op.doc.RequestNamedExamples))
		for _, ex := range op.doc.RequestNamedExamples {
			requestNamedExamples = append(requestNamedExamples, build.ExampleData{
				Name:          ex.Name(),
				Summary:       ex.Summary(),
				Description:   ex.Description(),
				Value:         ex.Value(),
				ExternalValue: ex.ExternalValue(),
			})
		}

		// Convert response examples
		responseNamedExamples := make(map[int][]build.ExampleData)
		for status, examples := range op.doc.ResponseNamedExamples {
			responseNamedExamples[status] = make([]build.ExampleData, 0, len(examples))
			for _, ex := range examples {
				responseNamedExamples[status] = append(responseNamedExamples[status], build.ExampleData{
					Name:          ex.Name(),
					Summary:       ex.Summary(),
					Description:   ex.Description(),
					Value:         ex.Value(),
					ExternalValue: ex.ExternalValue(),
				})
			}
		}

		// Introspect request metadata if type is set
		var requestMetadata *schema.RequestMetadata
		if op.doc.RequestType != nil {
			requestMetadata = schema.IntrospectRequest(op.doc.RequestType)
		}

		// Convert consumes/produces with defaults
		consumes := op.doc.Consumes
		if len(consumes) == 0 {
			consumes = []string{"application/json"}
		}
		produces := op.doc.Produces
		if len(produces) == 0 {
			produces = []string{"application/json"}
		}

		buildDoc = &build.RouteDoc{
			Summary:               op.doc.Summary,
			Description:           op.doc.Description,
			OperationID:           op.doc.OperationID,
			Tags:                  op.doc.Tags,
			Deprecated:            op.doc.Deprecated,
			Consumes:              consumes,
			Produces:              produces,
			RequestType:           op.doc.RequestType,
			RequestMetadata:       requestMetadata,
			RequestExample:        op.doc.RequestExample,
			RequestNamedExamples:  requestNamedExamples,
			ResponseTypes:         op.doc.ResponseTypes,
			ResponseExample:       op.doc.ResponseExample,
			ResponseNamedExamples: responseNamedExamples,
			Security:              convertSecurityReqsToBuild(op.doc.Security),
			Extensions:            op.doc.Extensions,
		}
	}

	return build.EnrichedRoute{
		RouteInfo: build.RouteInfo{
			Method:          op.Method,
			Path:            op.Path,
			PathConstraints: nil, // Path constraints are handled separately
		},
		Doc: buildDoc,
	}
}

// convertSecurityReqsToBuild converts openapi.SecurityReq to build.SecurityReq.
func convertSecurityReqsToBuild(reqs []SecurityReq) []build.SecurityReq {
	if len(reqs) == 0 {
		return nil
	}
	result := make([]build.SecurityReq, 0, len(reqs))
	for _, r := range reqs {
		result = append(result, build.SecurityReq{
			Scheme: r.Scheme,
			Scopes: r.Scopes,
		})
	}
	return result
}
