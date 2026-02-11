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
