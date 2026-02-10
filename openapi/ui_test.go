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

package openapi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUIConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   UIConfig
		wantErr  bool
		contains string
	}{
		{
			name:    "default config is valid",
			config:  MustNew(WithTitle("API", "1.0.0")).UI(),
			wantErr: false,
		},
		{
			name: "zero value config is valid",
			config: UIConfig{
				DefaultModelsExpandDepth: 0,
				DefaultModelExpandDepth:  0,
				MaxDisplayedTags:         0,
			},
			wantErr: false,
		},
		{
			name: "invalid DocExpansion",
			config: UIConfig{
				DocExpansion: DocExpansionMode("invalid"),
			},
			wantErr:  true,
			contains: "docExpansion",
		},
		{
			name: "invalid DefaultModelRendering",
			config: UIConfig{
				DefaultModelRendering: ModelRenderingMode("invalid"),
			},
			wantErr:  true,
			contains: "defaultModelRendering",
		},
		{
			name: "invalid OperationsSorter",
			config: UIConfig{
				OperationsSorter: OperationsSorterMode("invalid"),
			},
			wantErr:  true,
			contains: "operationsSorter",
		},
		{
			name: "invalid TagsSorter",
			config: UIConfig{
				TagsSorter: TagsSorterMode("invalid"),
			},
			wantErr:  true,
			contains: "tagsSorter",
		},
		{
			name: "invalid SyntaxTheme",
			config: UIConfig{
				SyntaxHighlight: SyntaxHighlightConfig{
					Activated: true,
					Theme:     SyntaxTheme("invalid"),
				},
			},
			wantErr:  true,
			contains: "syntax theme",
		},
		{
			name: "invalid RequestSnippet language",
			config: UIConfig{
				RequestSnippets: RequestSnippetsConfig{
					Languages: []RequestSnippetLanguage{"invalid"},
				},
			},
			wantErr:  true,
			contains: "request snippet",
		},
		{
			name: "invalid HTTP method",
			config: UIConfig{
				SupportedSubmitMethods: []HTTPMethod{"invalid"},
			},
			wantErr:  true,
			contains: "HTTP method",
		},
		{
			name: "DefaultModelsExpandDepth less than -1",
			config: UIConfig{
				DefaultModelsExpandDepth: -2,
			},
			wantErr:  true,
			contains: "defaultModelsExpandDepth",
		},
		{
			name: "DefaultModelExpandDepth less than -1",
			config: UIConfig{
				DefaultModelExpandDepth: -2,
			},
			wantErr:  true,
			contains: "defaultModelExpandDepth",
		},
		{
			name: "MaxDisplayedTags negative",
			config: UIConfig{
				MaxDisplayedTags: -1,
			},
			wantErr:  true,
			contains: "maxDisplayedTags",
		},
		{
			name: "valid DocExpansion modes",
			config: UIConfig{
				DocExpansion: DocExpansionFull,
			},
			wantErr: false,
		},
		{
			name: "valid OperationsSorter alpha",
			config: UIConfig{
				OperationsSorter: OperationsSorterAlpha,
			},
			wantErr: false,
		},
		{
			name: "valid TagsSorter alpha",
			config: UIConfig{
				TagsSorter: TagsSorterAlpha,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.contains != "" {
					assert.ErrorContains(t, err, tt.contains)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestUIConfig_ToConfigMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   UIConfig
		specURL  string
		validate func(t *testing.T, m map[string]any)
	}{
		{
			name: "includes url and key options",
			config: UIConfig{
				DeepLinking:           true,
				DocExpansion:          DocExpansionList,
				DefaultModelRendering: ModelRenderingExample,
			},
			specURL: "https://api.example.com/openapi.json",
			validate: func(t *testing.T, m map[string]any) {
				t.Helper()
				assert.Equal(t, "https://api.example.com/openapi.json", m["url"])
				assert.Equal(t, "#swagger-ui", m["dom_id"])
				deepLinking, ok := m["deepLinking"].(bool)
				require.True(t, ok)
				assert.True(t, deepLinking)
				assert.Equal(t, "list", m["docExpansion"])
			},
		},
		{
			name: "validatorUrl nil for empty",
			config: UIConfig{
				ValidatorURL: "",
			},
			specURL: "/spec.json",
			validate: func(t *testing.T, m map[string]any) {
				t.Helper()
				assert.Nil(t, m["validatorUrl"])
			},
		},
		{
			name: "validatorUrl nil for none",
			config: UIConfig{
				ValidatorURL: ValidatorNone,
			},
			specURL: "/spec.json",
			validate: func(t *testing.T, m map[string]any) {
				t.Helper()
				assert.Nil(t, m["validatorUrl"])
			},
		},
		{
			name: "validatorUrl nil for local",
			config: UIConfig{
				ValidatorURL: ValidatorLocal,
			},
			specURL: "/spec.json",
			validate: func(t *testing.T, m map[string]any) {
				t.Helper()
				assert.Nil(t, m["validatorUrl"])
			},
		},
		{
			name: "validatorUrl set for custom URL",
			config: UIConfig{
				ValidatorURL: "https://validator.swagger.io/validator",
			},
			specURL: "/spec.json",
			validate: func(t *testing.T, m map[string]any) {
				t.Helper()
				assert.Equal(t, "https://validator.swagger.io/validator", m["validatorUrl"])
			},
		},
		{
			name: "maxDisplayedTags omitted when 0",
			config: UIConfig{
				MaxDisplayedTags: 0,
			},
			specURL: "/spec.json",
			validate: func(t *testing.T, m map[string]any) {
				t.Helper()
				_, has := m["maxDisplayedTags"]
				assert.False(t, has)
			},
		},
		{
			name: "maxDisplayedTags included when positive",
			config: UIConfig{
				MaxDisplayedTags: 10,
			},
			specURL: "/spec.json",
			validate: func(t *testing.T, m map[string]any) {
				t.Helper()
				assert.EqualValues(t, 10, m["maxDisplayedTags"])
			},
		},
		{
			name: "supportedSubmitMethods and requestSnippets when set",
			config: UIConfig{
				RequestSnippetsEnabled: true,
				RequestSnippets: RequestSnippetsConfig{
					Languages:       []RequestSnippetLanguage{SnippetCurlBash},
					DefaultExpanded: true,
				},
				SupportedSubmitMethods: []HTTPMethod{MethodGet, MethodPost},
			},
			specURL: "/spec.json",
			validate: func(t *testing.T, m map[string]any) {
				t.Helper()
				methods, ok := m["supportedSubmitMethods"].([]string)
				require.True(t, ok, "supportedSubmitMethods should be []string from ToConfigMap")
				require.Len(t, methods, 2)
				assert.Equal(t, "get", methods[0])
				assert.Equal(t, "post", methods[1])
				snippets, ok := m["requestSnippets"].(map[string]any)
				require.True(t, ok, "requestSnippets should be present when RequestSnippetsEnabled is true")
				require.NotNil(t, snippets)
				require.Contains(t, snippets, "defaultExpanded")
				langs, ok := snippets["languages"].([]string)
				require.True(t, ok, "languages should be []string from ToConfigMap")
				require.Len(t, langs, 1)
				assert.Equal(t, "curl_bash", langs[0])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := tt.config.ToConfigMap(tt.specURL)
			require.NotNil(t, m)
			tt.validate(t, m)
		})
	}
}

func TestUIConfig_ToJSON(t *testing.T) {
	t.Parallel()

	t.Run("valid config produces valid JSON", func(t *testing.T) {
		t.Parallel()

		config := MustNew(WithTitle("API", "1.0.0")).UI()
		specURL := "https://example.com/openapi.json"

		jsonStr, err := config.ToJSON(specURL)
		require.NoError(t, err)
		require.NotEmpty(t, jsonStr)

		var m map[string]any
		err = json.Unmarshal([]byte(jsonStr), &m)
		require.NoError(t, err)
		assert.Equal(t, specURL, m["url"])
	})

	t.Run("output round-trips and contains spec URL", func(t *testing.T) {
		t.Parallel()

		config := UIConfig{
			DocExpansion: DocExpansionFull,
			Filter:       true,
		}
		specURL := "/docs/openapi.json"

		jsonStr, err := config.ToJSON(specURL)
		require.NoError(t, err)

		var m map[string]any
		err = json.Unmarshal([]byte(jsonStr), &m)
		require.NoError(t, err)
		assert.Equal(t, specURL, m["url"])
		assert.Equal(t, "full", m["docExpansion"])
		filter, ok := m["filter"].(bool)
		require.True(t, ok)
		assert.True(t, filter)
	})
}
