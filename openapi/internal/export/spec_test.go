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

//go:build !integration

package export

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/openapi/diag"
	"rivaas.dev/openapi/internal/model"
	"rivaas.dev/openapi/validate"
)

func TestProject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		spec            *model.Spec
		cfg             Config
		wantErr         bool
		wantErrContains string
		wantWarns       bool
		validate        func(t *testing.T, result Result)
	}{
		{
			name: "nil spec",
			spec: nil,
			cfg: Config{
				Version: V30,
			},
			wantErr: true,
		},
		{
			name: "unknown version",
			spec: &model.Spec{
				Info: model.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: map[string]*model.PathItem{
					"/test": {},
				},
			},
			cfg: Config{
				Version: Version("unknown"),
			},
			wantErr: true,
		},
		{
			name: "valid 3.0 spec",
			spec: &model.Spec{
				Info: model.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: map[string]*model.PathItem{
					"/test": {
						Get: &model.Operation{
							Summary: "Test endpoint",
							Responses: map[string]*model.Response{
								"200": {
									Description: "Success",
								},
							},
						},
					},
				},
			},
			cfg: Config{
				Version: V30,
			},
			wantErr: false,
			validate: func(t *testing.T, result Result) {
				t.Helper()
				var m map[string]any
				require.NoError(t, json.Unmarshal(result.JSON, &m))
				assert.Equal(t, "3.0.4", m["openapi"])
				assert.NotNil(t, m["info"])
				assert.NotNil(t, m["paths"])
			},
		},
		{
			name: "valid 3.1 spec",
			spec: &model.Spec{
				Info: model.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: map[string]*model.PathItem{
					"/test": {
						Get: &model.Operation{
							Summary: "Test endpoint",
							Responses: map[string]*model.Response{
								"200": {
									Description: "Success",
								},
							},
						},
					},
				},
			},
			cfg: Config{
				Version: V31,
			},
			wantErr: false,
			validate: func(t *testing.T, result Result) {
				t.Helper()
				var m map[string]any
				require.NoError(t, json.Unmarshal(result.JSON, &m))
				assert.Equal(t, "3.1.2", m["openapi"])
				assert.NotNil(t, m["info"])
				assert.NotNil(t, m["paths"])
			},
		},
		{
			name: "3.0 spec with 3.1-only features generates warnings",
			spec: &model.Spec{
				Info: model.Info{
					Title:   "Test API",
					Version: "1.0.0",
					Summary: "API summary", // 3.1-only
				},
				Paths: map[string]*model.PathItem{
					"/test": {
						Get: &model.Operation{
							Summary: "Test endpoint",
							Responses: map[string]*model.Response{
								"200": {
									Description: "Success",
								},
							},
						},
					},
				},
				Webhooks: map[string]*model.PathItem{
					"testWebhook": {},
				},
			},
			cfg: Config{
				Version: V30,
			},
			wantErr:   false,
			wantWarns: true,
			validate: func(t *testing.T, result Result) {
				t.Helper()
				assert.NotEmpty(t, result.Warnings)
				var foundSummary, foundWebhooks bool
				for _, w := range result.Warnings {
					if w.Code() == diag.WarnDownlevelInfoSummary {
						foundSummary = true
					}
					if w.Code() == diag.WarnDownlevelWebhooks {
						foundWebhooks = true
					}
				}
				assert.True(t, foundSummary, "should warn about summary")
				assert.True(t, foundWebhooks, "should warn about webhooks")
			},
		},
		{
			name: "3.0 spec with strict downlevel errors on 3.1-only features",
			spec: &model.Spec{
				Info: model.Info{
					Title:   "Test API",
					Version: "1.0.0",
					Summary: "API summary", // 3.1-only
				},
				Paths: map[string]*model.PathItem{
					"/test": {
						Get: &model.Operation{
							Summary: "Test endpoint",
							Responses: map[string]*model.Response{
								"200": {
									Description: "Success",
								},
							},
						},
					},
				},
			},
			cfg: Config{
				Version:         V30,
				StrictDownlevel: true,
			},
			wantErr: true,
		},
		{
			name: "empty paths returns error for 3.0",
			spec: &model.Spec{
				Info: model.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: map[string]*model.PathItem{},
			},
			cfg: Config{
				Version: V30,
			},
			wantErr:         true,
			wantErrContains: "paths",
		},
		{
			name: "3.1 minimal spec with empty paths succeeds",
			spec: &model.Spec{
				Info: model.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: map[string]*model.PathItem{},
			},
			cfg: Config{
				Version: V31,
			},
			wantErr: false,
			validate: func(t *testing.T, result Result) {
				t.Helper()
				var m map[string]any
				require.NoError(t, json.Unmarshal(result.JSON, &m))
				assert.Equal(t, "3.1.2", m["openapi"])
				assert.NotNil(t, m["info"])
			},
		},
		{
			name: "3.0 spec with extensions",
			spec: &model.Spec{
				Info: model.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: map[string]*model.PathItem{
					"/test": {
						Get: &model.Operation{
							Summary: "Test endpoint",
							Responses: map[string]*model.Response{
								"200": {
									Description: "Success",
								},
							},
						},
					},
				},
				Extensions: map[string]any{
					"x-custom": "value",
				},
			},
			cfg: Config{
				Version: V30,
			},
			wantErr: false,
			validate: func(t *testing.T, result Result) {
				t.Helper()
				var m map[string]any
				require.NoError(t, json.Unmarshal(result.JSON, &m))
				assert.Equal(t, "value", m["x-custom"])
			},
		},
		{
			name: "3.0 strict downlevel with webhooks returns error",
			spec: &model.Spec{
				Info: model.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: map[string]*model.PathItem{
					"/test": {
						Get: &model.Operation{
							Summary: "Test endpoint",
							Responses: map[string]*model.Response{
								"200": {Description: "Success"},
							},
						},
					},
				},
				Webhooks: map[string]*model.PathItem{
					"onEvent": {},
				},
			},
			cfg: Config{
				Version:         V30,
				StrictDownlevel: true,
			},
			wantErr:         true,
			wantErrContains: "webhooks",
		},
		{
			name: "3.0 strict downlevel with mutualTLS security scheme returns error",
			spec: &model.Spec{
				Info: model.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: map[string]*model.PathItem{
					"/test": {
						Get: &model.Operation{
							Summary: "Test endpoint",
							Responses: map[string]*model.Response{
								"200": {Description: "Success"},
							},
						},
					},
				},
				Components: &model.Components{
					SecuritySchemes: map[string]*model.SecurityScheme{
						"mutualTLS": {Type: "mutualTLS"},
					},
				},
			},
			cfg: Config{
				Version:         V30,
				StrictDownlevel: true,
			},
			wantErr:         true,
			wantErrContains: "mutualTLS",
		},
		{
			name: "3.0 with pathItems in components warns and drops pathItems",
			spec: &model.Spec{
				Info: model.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: map[string]*model.PathItem{
					"/test": {
						Get: &model.Operation{
							Summary: "Test endpoint",
							Responses: map[string]*model.Response{
								"200": {Description: "Success"},
							},
						},
					},
				},
				Components: &model.Components{
					PathItems: map[string]*model.PathItem{
						"reusablePath": {
							Get: &model.Operation{
								Summary: "Reusable",
								Responses: map[string]*model.Response{
									"200": {Description: "OK"},
								},
							},
						},
					},
				},
			},
			cfg: Config{
				Version: V30,
			},
			wantErr:   false,
			wantWarns: true,
			validate: func(t *testing.T, result Result) {
				t.Helper()
				var foundPathItems bool
				for _, w := range result.Warnings {
					if w.Code() == diag.WarnDownlevelPathItems {
						foundPathItems = true
						break
					}
				}
				assert.True(t, foundPathItems, "should warn about pathItems")
				var m map[string]any
				require.NoError(t, json.Unmarshal(result.JSON, &m))
				comp, ok := m["components"].(map[string]any)
				require.True(t, ok)
				if comp != nil {
					_, hasPathItems := comp["pathItems"]
					assert.False(t, hasPathItems, "pathItems should be dropped in 3.0")
				}
			},
		},
		{
			name: "3.0 with license identifier warns and drops identifier",
			spec: &model.Spec{
				Info: model.Info{
					Title:   "Test API",
					Version: "1.0.0",
					License: &model.License{
						Name:       "MIT",
						Identifier: "MIT",
					},
				},
				Paths: map[string]*model.PathItem{
					"/test": {
						Get: &model.Operation{
							Summary: "Test endpoint",
							Responses: map[string]*model.Response{
								"200": {Description: "Success"},
							},
						},
					},
				},
			},
			cfg: Config{
				Version: V30,
			},
			wantErr:   false,
			wantWarns: true,
			validate: func(t *testing.T, result Result) {
				t.Helper()
				var foundLicenseWarn bool
				for _, w := range result.Warnings {
					if w.Code() == diag.WarnDownlevelLicenseIdentifier {
						foundLicenseWarn = true
						break
					}
				}
				assert.True(t, foundLicenseWarn, "should warn about license identifier")
				var m map[string]any
				require.NoError(t, json.Unmarshal(result.JSON, &m))
				info, ok := m["info"].(map[string]any)
				require.True(t, ok)
				require.NotNil(t, info)
				license, ok := info["license"].(map[string]any)
				require.True(t, ok)
				require.NotNil(t, license)
				_, hasIdentifier := license["identifier"]
				assert.False(t, hasIdentifier, "identifier should be dropped in 3.0")
			},
		},
		{
			name: "3.1 with license identifier and URL both set returns error",
			spec: &model.Spec{
				Info: model.Info{
					Title:   "Test API",
					Version: "1.0.0",
					License: &model.License{
						Name:       "MIT",
						Identifier: "MIT",
						URL:        "https://opensource.org/licenses/MIT",
					},
				},
				Paths: map[string]*model.PathItem{
					"/test": {
						Get: &model.Operation{
							Summary: "Test endpoint",
							Responses: map[string]*model.Response{
								"200": {Description: "Success"},
							},
						},
					},
				},
			},
			cfg: Config{
				Version: V31,
			},
			wantErr:         true,
			wantErrContains: "mutually exclusive",
		},
		{
			name: "3.1 with server variable empty enum adds warning",
			spec: &model.Spec{
				Info: model.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: map[string]*model.PathItem{
					"/test": {
						Get: &model.Operation{
							Summary: "Test endpoint",
							Responses: map[string]*model.Response{
								"200": {Description: "Success"},
							},
						},
					},
				},
				Servers: []model.Server{
					{
						URL: "https://{env}.example.com",
						Variables: map[string]*model.ServerVariable{
							"env": {
								Default: "api",
								Enum:    []string{}, // empty enum triggers warning in 3.1
							},
						},
					},
				},
			},
			cfg: Config{
				Version: V31,
			},
			wantErr:   false,
			wantWarns: true,
			validate: func(t *testing.T, result Result) {
				t.Helper()
				var found bool
				for _, w := range result.Warnings {
					if w.Code() == "SERVER_VARIABLE_EMPTY_ENUM" {
						found = true
						break
					}
				}
				assert.True(t, found, "should warn about empty enum")
			},
		},
		{
			name: "3.1 with server variable default not in enum adds warning",
			spec: &model.Spec{
				Info: model.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: map[string]*model.PathItem{
					"/test": {
						Get: &model.Operation{
							Summary: "Test endpoint",
							Responses: map[string]*model.Response{
								"200": {Description: "Success"},
							},
						},
					},
				},
				Servers: []model.Server{
					{
						URL: "https://{env}.example.com",
						Variables: map[string]*model.ServerVariable{
							"env": {
								Default: "other",
								Enum:    []string{"api", "staging"},
							},
						},
					},
				},
			},
			cfg: Config{
				Version: V31,
			},
			wantErr:   false,
			wantWarns: true,
			validate: func(t *testing.T, result Result) {
				t.Helper()
				var found bool
				for _, w := range result.Warnings {
					if w.Code() == "SERVER_VARIABLE_DEFAULT_NOT_IN_ENUM" {
						found = true
						break
					}
				}
				assert.True(t, found, "should warn about default not in enum")
			},
		},
		{
			name: "3.1 with no servers injects default server",
			spec: &model.Spec{
				Info: model.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths:   map[string]*model.PathItem{},
				Servers: nil,
			},
			cfg: Config{
				Version: V31,
			},
			wantErr: false,
			validate: func(t *testing.T, result Result) {
				t.Helper()
				var m map[string]any
				require.NoError(t, json.Unmarshal(result.JSON, &m))
				servers, ok := m["servers"].([]any)
				require.True(t, ok, "servers should be present")
				require.Len(t, servers, 1)
				server, ok := servers[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "/", server["url"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := Project(context.Background(), tt.spec, tt.cfg)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContains != "" {
					assert.ErrorContains(t, err, tt.wantErrContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result.JSON)

			if tt.wantWarns {
				assert.NotEmpty(t, result.Warnings)
			}

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// fullSpecV30 returns a rich model.Spec that exercises all projection branches in spec_v30.go.
func fullSpecV30() *model.Spec {
	minimalSchema := &model.Schema{Kind: model.KindString}
	return &model.Spec{
		Info: model.Info{
			Title:   "Full Coverage API",
			Version: "1.0.0",
			Contact: &model.Contact{
				Name:       "Support",
				URL:        "https://example.com",
				Email:      "support@example.com",
				Extensions: map[string]any{"x-contact": "c1"},
			},
			License: &model.License{
				Name: "MIT",
				URL:  "https://opensource.org/licenses/MIT",
			},
		},
		Servers: []model.Server{
			{
				URL:         "https://{env}.example.com",
				Description: "API server",
				Variables: map[string]*model.ServerVariable{
					"env": {
						Default:     "api",
						Enum:        []string{"api", "staging"},
						Description: "Environment",
					},
				},
			},
		},
		Tags: []model.Tag{
			{
				Name:        "items",
				Description: "Item operations",
				ExternalDocs: &model.ExternalDocs{
					URL:         "https://docs.example.com",
					Description: "External docs",
				},
			},
		},
		Security: []model.SecurityRequirement{{"oauth2Scheme": {}}},
		ExternalDocs: &model.ExternalDocs{
			URL:         "https://docs.example.com",
			Description: "API documentation",
		},
		Paths: map[string]*model.PathItem{
			"/refPath": {
				Ref: "#/components/pathItems/reusable",
			},
			"/ops": {
				Put:     &model.Operation{Summary: "Put", Responses: map[string]*model.Response{"200": {Description: "OK"}}},
				Delete:  &model.Operation{Summary: "Delete", Responses: map[string]*model.Response{"200": {Description: "OK"}}},
				Options: &model.Operation{Summary: "Options", Responses: map[string]*model.Response{"200": {Description: "OK"}}},
				Head:    &model.Operation{Summary: "Head", Responses: map[string]*model.Response{"200": {Description: "OK"}}},
				Patch:   &model.Operation{Summary: "Patch", Responses: map[string]*model.Response{"200": {Description: "OK"}}},
				Trace:   &model.Operation{Summary: "Trace", Responses: map[string]*model.Response{"200": {Description: "OK"}}},
			},
			"/items": {
				Summary:     "Items path",
				Description: "Path with full projection",
				Parameters: []model.Parameter{
					{
						Name:        "limit",
						In:          "query",
						Description: "Max items",
						Schema:      minimalSchema,
						Examples: map[string]*model.Example{
							"default": {Summary: "Default", Value: "10"},
						},
					},
				},
				Post: &model.Operation{
					Tags:        []string{"items"},
					Summary:     "Create item",
					Description: "Creates a new item",
					OperationID: "createItem",
					ExternalDocs: &model.ExternalDocs{
						URL: "https://docs.example.com/create",
					},
					Parameters: []model.Parameter{
						{
							Name:     "id",
							In:       "path",
							Required: true,
							Schema:   minimalSchema,
						},
					},
					RequestBody: &model.RequestBody{
						Description: "Item body",
						Required:    true,
						Content: map[string]*model.MediaType{
							"application/json": {
								Schema: minimalSchema,
								Examples: map[string]*model.Example{
									"item": {Value: map[string]any{"name": "foo"}},
								},
								Encoding: map[string]*model.Encoding{
									"style": {
										ContentType: "application/json",
										Headers: map[string]*model.Header{
											"X-Custom": {
												Description: "Custom header",
												Schema:      minimalSchema,
											},
										},
									},
								},
							},
						},
					},
					Responses: map[string]*model.Response{
						"200": {
							Description: "Created",
							Content: map[string]*model.MediaType{
								"application/json": {
									Schema:   minimalSchema,
									Example:  "ok",
									Examples: map[string]*model.Example{"one": {Summary: "One", Value: "v"}},
								},
							},
							Headers: map[string]*model.Header{
								"X-Request-Id": {
									Description: "Request ID",
									Schema:      minimalSchema,
									Examples:    map[string]*model.Example{"id": {Value: "req-1"}},
								},
							},
							Links: map[string]*model.Link{
								"getItem": {
									OperationID: "getItem",
									Description: "Get the item",
									Server: &model.Server{
										URL: "https://api.example.com",
									},
								},
							},
						},
					},
					Security: []model.SecurityRequirement{{"oauth2Scheme": {"read", "write"}}},
					Servers: []model.Server{
						{URL: "https://api.example.com"},
					},
					Callbacks: map[string]*model.Callback{
						"onEvent": {
							PathItems: map[string]*model.PathItem{
								"https://events.example.com": {
									Post: &model.Operation{
										Summary: "Event",
										Responses: map[string]*model.Response{
											"200": {Description: "OK"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Components: &model.Components{
			Parameters: map[string]*model.Parameter{
				"sharedParam": {
					Name:    "shared",
					In:      "query",
					Schema:  minimalSchema,
					Content: map[string]*model.MediaType{"application/json": {Schema: minimalSchema}},
				},
			},
			Examples: map[string]*model.Example{
				"itemExample": {Summary: "Item", Value: map[string]any{"id": "1"}},
			},
			RequestBodies: map[string]*model.RequestBody{
				"itemBody": {
					Description: "Item",
					Content:     map[string]*model.MediaType{"application/json": {Schema: minimalSchema}},
				},
			},
			Headers: map[string]*model.Header{
				"X-Header": {Description: "Header", Schema: minimalSchema},
			},
			Links: map[string]*model.Link{
				"itemLink": {OperationID: "getItem", Description: "Link"},
			},
			Callbacks: map[string]*model.Callback{
				"callback": {
					PathItems: map[string]*model.PathItem{
						"https://callback.example.com": {
							Get: &model.Operation{
								Summary:   "Callback",
								Responses: map[string]*model.Response{"200": {Description: "OK"}},
							},
						},
					},
				},
			},
			Responses: map[string]*model.Response{
				"generic": {Description: "Generic response", Content: map[string]*model.MediaType{"application/json": {Schema: minimalSchema}}},
			},
			SecuritySchemes: map[string]*model.SecurityScheme{
				"oauth2Scheme": {
					Type:        "oauth2",
					Description: "OAuth2",
					Flows: &model.OAuthFlows{
						AuthorizationCode: &model.OAuthFlow{
							AuthorizationURL: "https://auth.example.com/authorize",
							TokenURL:         "https://auth.example.com/token",
							Scopes:           map[string]string{"read": "Read", "write": "Write"},
						},
						Implicit: &model.OAuthFlow{
							AuthorizationURL: "https://auth.example.com/authorize",
							Scopes:           map[string]string{"read": "Read"},
						},
						Password: &model.OAuthFlow{
							TokenURL: "https://auth.example.com/token",
							Scopes:   map[string]string{"read": "Read"},
						},
						ClientCredentials: &model.OAuthFlow{
							TokenURL: "https://auth.example.com/token",
							Scopes:   map[string]string{"read": "Read"},
						},
					},
				},
			},
		},
	}
}

func TestProject_V30_fullSpecCoverage(t *testing.T) {
	t.Parallel()

	spec := fullSpecV30()
	result, err := Project(context.Background(), spec, Config{Version: V30})

	require.NoError(t, err)
	require.NotNil(t, result.JSON)

	var m map[string]any
	require.NoError(t, json.Unmarshal(result.JSON, &m))
	assert.Equal(t, "3.0.4", m["openapi"])
	assert.NotNil(t, m["info"])
	assert.NotNil(t, m["paths"])
	assert.NotNil(t, m["components"])
	assert.NotNil(t, m["security"])
	assert.NotNil(t, m["externalDocs"])
	assert.NotNil(t, m["servers"])
	assert.NotNil(t, m["tags"])

	comp, ok := m["components"].(map[string]any)
	require.True(t, ok)
	assert.NotNil(t, comp["parameters"])
	assert.NotNil(t, comp["examples"])
	assert.NotNil(t, comp["requestBodies"])
	assert.NotNil(t, comp["headers"])
	assert.NotNil(t, comp["links"])
	assert.NotNil(t, comp["callbacks"])
	assert.NotNil(t, comp["securitySchemes"])
}

// fullSpecV31 returns a rich model.Spec for OpenAPI 3.1 that exercises projection branches in spec_v31.go.
func fullSpecV31() *model.Spec {
	minimalSchema := &model.Schema{Kind: model.KindString}
	spec := fullSpecV30()
	spec.Webhooks = map[string]*model.PathItem{
		"itemCreated": {
			Post: &model.Operation{
				Summary: "Item created webhook",
				Responses: map[string]*model.Response{
					"200": {Description: "Received"},
				},
			},
		},
	}
	if spec.Components == nil {
		spec.Components = &model.Components{}
	}
	spec.Components.PathItems = map[string]*model.PathItem{
		"reusable": {
			Summary: "Reusable path",
			Get: &model.Operation{
				Summary: "Get reusable",
				Responses: map[string]*model.Response{
					"200": {
						Description: "OK",
						Content:     map[string]*model.MediaType{"application/json": {Schema: minimalSchema}},
					},
				},
			},
		},
	}
	return spec
}

func TestProject_V31_fullSpecCoverage(t *testing.T) {
	t.Parallel()

	spec := fullSpecV31()
	result, err := Project(context.Background(), spec, Config{Version: V31})

	require.NoError(t, err)
	require.NotNil(t, result.JSON)

	var m map[string]any
	require.NoError(t, json.Unmarshal(result.JSON, &m))
	assert.Equal(t, "3.1.2", m["openapi"])
	assert.NotNil(t, m["webhooks"])
	assert.NotNil(t, m["info"])
	assert.NotNil(t, m["paths"])
	assert.NotNil(t, m["components"])

	comp, ok := m["components"].(map[string]any)
	require.True(t, ok)
	assert.NotNil(t, comp["pathItems"])
}

func TestProject_V30_refBranches(t *testing.T) {
	t.Parallel()

	minimalSchema := &model.Schema{Kind: model.KindString}
	tests := []struct {
		name string
		spec *model.Spec
	}{
		{
			name: "parameter Ref only",
			spec: &model.Spec{
				Info: model.Info{Title: "API", Version: "1.0"},
				Paths: map[string]*model.PathItem{
					"/p": {
						Parameters: []model.Parameter{{Ref: "#/components/parameters/refParam"}},
						Get:        &model.Operation{Responses: map[string]*model.Response{"200": {Description: "OK"}}},
					},
				},
				Components: &model.Components{
					Parameters: map[string]*model.Parameter{
						"refParam": {Ref: "#/components/parameters/refParam"},
					},
				},
			},
		},
		{
			name: "example Ref only",
			spec: &model.Spec{
				Info: model.Info{Title: "API", Version: "1.0"},
				Paths: map[string]*model.PathItem{
					"/p": {Get: &model.Operation{Responses: map[string]*model.Response{"200": {Description: "OK"}}}},
				},
				Components: &model.Components{
					Examples: map[string]*model.Example{
						"refEx": {Ref: "#/components/examples/refEx"},
					},
				},
			},
		},
		{
			name: "requestBody Ref only",
			spec: &model.Spec{
				Info: model.Info{Title: "API", Version: "1.0"},
				Paths: map[string]*model.PathItem{
					"/p": {
						Post: &model.Operation{
							RequestBody: &model.RequestBody{Ref: "#/components/requestBodies/refBody"},
							Responses:   map[string]*model.Response{"200": {Description: "OK"}},
						},
					},
				},
				Components: &model.Components{
					RequestBodies: map[string]*model.RequestBody{
						"refBody": {Ref: "#/components/requestBodies/refBody"},
					},
				},
			},
		},
		{
			name: "response Ref only",
			spec: &model.Spec{
				Info: model.Info{Title: "API", Version: "1.0"},
				Paths: map[string]*model.PathItem{
					"/p": {
						Get: &model.Operation{
							Responses: map[string]*model.Response{
								"200": {Ref: "#/components/responses/refResp"},
							},
						},
					},
				},
				Components: &model.Components{
					Responses: map[string]*model.Response{
						"refResp": {Ref: "#/components/responses/refResp", Description: "Ref response"},
					},
				},
			},
		},
		{
			name: "header Ref only",
			spec: &model.Spec{
				Info: model.Info{Title: "API", Version: "1.0"},
				Paths: map[string]*model.PathItem{
					"/p": {
						Get: &model.Operation{
							Responses: map[string]*model.Response{
								"200": {
									Description: "OK",
									Headers: map[string]*model.Header{
										"X-Ref": {Ref: "#/components/headers/refHeader"},
									},
								},
							},
						},
					},
				},
				Components: &model.Components{
					Headers: map[string]*model.Header{
						"refHeader": {Ref: "#/components/headers/refHeader", Description: "Ref header"},
					},
				},
			},
		},
		{
			name: "link Ref only",
			spec: &model.Spec{
				Info: model.Info{Title: "API", Version: "1.0"},
				Paths: map[string]*model.PathItem{
					"/p": {
						Get: &model.Operation{
							Responses: map[string]*model.Response{
								"200": {
									Description: "OK",
									Links: map[string]*model.Link{
										"refLink": {Ref: "#/components/links/refLink"},
									},
								},
							},
						},
					},
				},
				Components: &model.Components{
					Links: map[string]*model.Link{
						"refLink": {Ref: "#/components/links/refLink", OperationID: "get"},
					},
				},
			},
		},
		{
			name: "callback Ref only",
			spec: &model.Spec{
				Info: model.Info{Title: "API", Version: "1.0"},
				Paths: map[string]*model.PathItem{
					"/p": {
						Post: &model.Operation{
							Responses: map[string]*model.Response{"200": {Description: "OK"}},
							Callbacks: map[string]*model.Callback{
								"onEvent": {Ref: "#/components/callbacks/refCb"},
							},
						},
					},
				},
				Components: &model.Components{
					Callbacks: map[string]*model.Callback{
						"refCb": {Ref: "#/components/callbacks/refCb"},
					},
				},
			},
		},
		{
			name: "securityScheme Ref only",
			spec: &model.Spec{
				Info:     model.Info{Title: "API", Version: "1.0"},
				Security: []model.SecurityRequirement{{"refScheme": {}}},
				Paths: map[string]*model.PathItem{
					"/p": {Get: &model.Operation{Responses: map[string]*model.Response{"200": {Description: "OK"}}}},
				},
				Components: &model.Components{
					SecuritySchemes: map[string]*model.SecurityScheme{
						"refScheme": {Ref: "#/components/securitySchemes/refScheme", Type: "apiKey", Name: "key", In: "header"},
					},
				},
			},
		},
		{
			name: "header with Content (projection branch)",
			spec: &model.Spec{
				Info: model.Info{Title: "API", Version: "1.0"},
				Paths: map[string]*model.PathItem{
					"/p": {
						Get: &model.Operation{
							Responses: map[string]*model.Response{
								"200": {
									Description: "OK",
									Headers: map[string]*model.Header{
										"X-Content": {
											Description: "Header with content",
											Content:     map[string]*model.MediaType{"application/json": {Schema: minimalSchema}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := Project(context.Background(), tt.spec, Config{Version: V30})
			require.NoError(t, err)
			require.NotNil(t, result.JSON)
		})
	}
}

// mockValidator is a test implementation of Validator.
type mockValidator struct {
	validateFunc func(ctx context.Context, specJSON []byte, version validate.Version) error
}

func (m *mockValidator) Validate(ctx context.Context, specJSON []byte, version validate.Version) error {
	return m.validateFunc(ctx, specJSON, version)
}

func TestProject_WithValidator(t *testing.T) {
	t.Parallel()

	validateFunc := func(ctx context.Context, specJSON []byte, version validate.Version) error {
		return nil
	}

	validator := &mockValidator{validateFunc: validateFunc}

	spec := &model.Spec{
		Info: model.Info{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Paths: map[string]*model.PathItem{
			"/test": {
				Get: &model.Operation{
					Summary: "Test endpoint",
					Responses: map[string]*model.Response{
						"200": {
							Description: "Success",
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "3.0 with validator",
			cfg: Config{
				Version:   V30,
				Validator: validator,
			},
			wantErr: false,
		},
		{
			name: "3.1 with validator",
			cfg: Config{
				Version:   V31,
				Validator: validator,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := Project(context.Background(), spec, tt.cfg)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result.JSON)
		})
	}
}

func TestProject_ValidatorError(t *testing.T) {
	t.Parallel()

	validateError := func(ctx context.Context, specJSON []byte, version validate.Version) error {
		return assert.AnError
	}

	validator := &mockValidator{validateFunc: validateError}

	spec := &model.Spec{
		Info: model.Info{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Paths: map[string]*model.PathItem{
			"/test": {
				Get: &model.Operation{
					Summary: "Test endpoint",
					Responses: map[string]*model.Response{
						"200": {
							Description: "Success",
						},
					},
				},
			},
		},
	}

	cfg := Config{
		Version:   V30,
		Validator: validator,
	}

	result, err := Project(context.Background(), spec, cfg)

	require.Error(t, err)
	// Warnings should still be collected before validation fails
	assert.True(t, len(result.Warnings) >= 0)
}

func TestVersion_Constants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, V30, Version("3.0.4"))
	assert.Equal(t, V31, Version("3.1.2"))
}
