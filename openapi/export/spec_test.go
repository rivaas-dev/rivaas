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
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/openapi/model"
)

func TestProject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		spec      *model.Spec
		cfg       Config
		schema30  []byte
		schema31  []byte
		wantErr   bool
		wantWarns bool
		validate  func(t *testing.T, jsonBytes []byte, warns []Warning)
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
			validate: func(t *testing.T, jsonBytes []byte, warns []Warning) {
				var m map[string]any
				require.NoError(t, json.Unmarshal(jsonBytes, &m))
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
			validate: func(t *testing.T, jsonBytes []byte, warns []Warning) {
				var m map[string]any
				require.NoError(t, json.Unmarshal(jsonBytes, &m))
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
			validate: func(t *testing.T, jsonBytes []byte, warns []Warning) {
				assert.NotEmpty(t, warns)
				var foundSummary, foundWebhooks bool
				for _, w := range warns {
					if w.Code == DownlevelInfoSummary {
						foundSummary = true
					}
					if w.Code == DownlevelWebhooks {
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
			name: "3.0 spec without paths errors",
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
			wantErr: true,
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
			validate: func(t *testing.T, jsonBytes []byte, warns []Warning) {
				var m map[string]any
				require.NoError(t, json.Unmarshal(jsonBytes, &m))
				assert.Equal(t, "value", m["x-custom"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			jsonBytes, warns, err := Project(tt.spec, tt.cfg, tt.schema30, tt.schema31)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, jsonBytes)

			if tt.wantWarns {
				assert.NotEmpty(t, warns)
			}

			if tt.validate != nil {
				tt.validate(t, jsonBytes, warns)
			}
		})
	}
}

// mockValidator is a test implementation of JSONSchemaValidator.
type mockValidator struct {
	validateFunc func(ctx context.Context, schemaJSON, docJSON []byte) error
}

func (m *mockValidator) ValidateJSON(ctx context.Context, schemaJSON, docJSON []byte) error {
	return m.validateFunc(ctx, schemaJSON, docJSON)
}

func TestProject_WithValidator(t *testing.T) {
	t.Parallel()

	validateFunc := func(ctx context.Context, schemaJSON, docJSON []byte) error {
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
		name     string
		cfg      Config
		schema30 []byte
		schema31 []byte
		wantErr  bool
	}{
		{
			name: "3.0 with validator and schema",
			cfg: Config{
				Version:          V30,
				JSONSchemaEngine: validator,
			},
			schema30: []byte(`{}`),
			wantErr:  false,
		},
		{
			name: "3.1 with validator and schema",
			cfg: Config{
				Version:          V31,
				JSONSchemaEngine: validator,
			},
			schema31: []byte(`{}`),
			wantErr:  false,
		},
		{
			name: "3.0 with validator but no schema",
			cfg: Config{
				Version:          V30,
				JSONSchemaEngine: validator,
			},
			schema30: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			jsonBytes, _, err := Project(spec, tt.cfg, tt.schema30, tt.schema31)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, jsonBytes)
		})
	}
}

func TestProject_ValidatorError(t *testing.T) {
	t.Parallel()

	validateError := func(ctx context.Context, schemaJSON, docJSON []byte) error {
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
		Version:          V30,
		JSONSchemaEngine: validator,
	}

	jsonBytes, warns, err := Project(spec, cfg, []byte(`{}`), nil)

	require.Error(t, err)
	assert.NotNil(t, warns)
	assert.Nil(t, jsonBytes)
}

func TestVersion_Constants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, V30, Version("3.0.4"))
	assert.Equal(t, V31, Version("3.1.2"))
}
