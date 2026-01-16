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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/openapi/diag"
	"rivaas.dev/openapi/internal/model"
)

func TestSchema30(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *model.Schema
		wantWarn bool
		validate func(t *testing.T, result *SchemaV30, warns diag.Warnings)
	}{
		{
			name:  "nil schema",
			input: nil,
			validate: func(t *testing.T, result *SchemaV30, _ diag.Warnings) {
				t.Helper()
				assert.Nil(t, result)
			},
		},
		{
			name: "schema with ref",
			input: &model.Schema{
				Ref: "#/components/schemas/User",
			},
			validate: func(t *testing.T, result *SchemaV30, _ diag.Warnings) {
				t.Helper()
				require.NotNil(t, result)
				assert.Equal(t, "#/components/schemas/User", result.Ref)
			},
		},
		{
			name: "string schema",
			input: &model.Schema{
				Kind:        model.KindString,
				Title:       "Name",
				Description: "User name",
				Format:      "string",
			},
			validate: func(t *testing.T, result *SchemaV30, _ diag.Warnings) {
				t.Helper()
				require.NotNil(t, result)
				assert.Equal(t, "string", result.Type)
				assert.Equal(t, "Name", result.Title)
				assert.Equal(t, "User name", result.Description)
				assert.Equal(t, "string", result.Format)
			},
		},
		{
			name: "nullable string",
			input: &model.Schema{
				Kind:     model.KindString,
				Nullable: true,
			},
			validate: func(t *testing.T, result *SchemaV30, warns diag.Warnings) {
				t.Helper()
				require.NotNil(t, result)
				assert.Equal(t, "string", result.Type)
				assert.True(t, result.Nullable)
			},
		},
		{
			name: "integer schema with bounds",
			input: &model.Schema{
				Kind: model.KindInteger,
				Minimum: &model.Bound{
					Value:     10,
					Exclusive: false,
				},
				Maximum: &model.Bound{
					Value:     100,
					Exclusive: true,
				},
			},
			validate: func(t *testing.T, result *SchemaV30, warns diag.Warnings) {
				t.Helper()
				require.NotNil(t, result)
				assert.Equal(t, "integer", result.Type)
				require.NotNil(t, result.Minimum)
				assert.Equal(t, 10.0, *result.Minimum) //nolint:testifylint // exact integer comparison
				require.NotNil(t, result.Maximum)
				assert.Equal(t, 100.0, *result.Maximum) //nolint:testifylint // exact integer comparison
				require.NotNil(t, result.ExclusiveMaximum)
				assert.True(t, *result.ExclusiveMaximum)
			},
		},
		{
			name: "array schema",
			input: &model.Schema{
				Kind:     model.KindArray,
				MinItems: intPtr(1),
				MaxItems: intPtr(10),
				Items: &model.Schema{
					Kind: model.KindString,
				},
			},
			validate: func(t *testing.T, result *SchemaV30, warns diag.Warnings) {
				t.Helper()
				require.NotNil(t, result)
				assert.Equal(t, "array", result.Type)
				require.NotNil(t, result.MinItems)
				assert.Equal(t, 1, *result.MinItems)
				require.NotNil(t, result.MaxItems)
				assert.Equal(t, 10, *result.MaxItems)
				require.NotNil(t, result.Items)
				assert.Equal(t, "string", result.Items.Type)
			},
		},
		{
			name: "object schema with properties",
			input: &model.Schema{
				Kind: model.KindObject,
				Properties: map[string]*model.Schema{
					"name": {
						Kind: model.KindString,
					},
					"age": {
						Kind: model.KindInteger,
					},
				},
				Required: []string{"name"},
			},
			validate: func(t *testing.T, result *SchemaV30, warns diag.Warnings) {
				t.Helper()
				require.NotNil(t, result)
				assert.Equal(t, "object", result.Type)
				require.NotNil(t, result.Properties)
				assert.Contains(t, result.Properties, "name")
				assert.Contains(t, result.Properties, "age")
				assert.Contains(t, result.Required, "name")
			},
		},
		{
			name: "const converted to enum",
			input: &model.Schema{
				Kind:  model.KindString,
				Const: "fixed-value",
			},
			wantWarn: true,
			validate: func(t *testing.T, result *SchemaV30, warns diag.Warnings) {
				t.Helper()
				require.NotNil(t, result)
				require.Len(t, result.Enum, 1)
				assert.Equal(t, "fixed-value", result.Enum[0])
				var found bool
				for _, w := range warns {
					if w.Code() == diag.WarnDownlevelConstToEnum {
						found = true
						break
					}
				}
				assert.True(t, found, "should warn about const to enum conversion")
			},
		},
		{
			name: "const with existing enum",
			input: &model.Schema{
				Kind:  model.KindString,
				Enum:  []any{"value1", "value2"},
				Const: "fixed-value",
			},
			wantWarn: true,
			validate: func(t *testing.T, result *SchemaV30, warns diag.Warnings) {
				t.Helper()
				require.NotNil(t, result)
				// Enum should be preserved, const ignored
				assert.Equal(t, []any{"value1", "value2"}, result.Enum)
				var found bool
				for _, w := range warns {
					if w.Code() == diag.WarnDownlevelConstToEnumConflict {
						found = true
						break
					}
				}
				assert.True(t, found, "should warn about const/enum conflict")
			},
		},
		{
			name: "unevaluated properties dropped",
			input: &model.Schema{
				Kind: model.KindObject,
				Unevaluated: &model.Schema{
					Kind: model.KindString,
				},
			},
			wantWarn: true,
			validate: func(t *testing.T, result *SchemaV30, warns diag.Warnings) {
				t.Helper()
				require.NotNil(t, result)
				var found bool
				for _, w := range warns {
					if w.Code() == diag.WarnDownlevelUnevaluatedProperties {
						found = true
						break
					}
				}
				assert.True(t, found, "should warn about unevaluated properties")
			},
		},
		{
			name: "multiple examples reduced to one",
			input: &model.Schema{
				Kind:     model.KindString,
				Examples: []any{"example1", "example2", "example3"},
			},
			wantWarn: true,
			validate: func(t *testing.T, result *SchemaV30, warns diag.Warnings) {
				t.Helper()
				require.NotNil(t, result)
				assert.Equal(t, "example1", result.Example)
				var found bool
				for _, w := range warns {
					if w.Code() == diag.WarnDownlevelMultipleExamples {
						found = true
						break
					}
				}
				assert.True(t, found, "should warn about multiple examples")
			},
		},
		{
			name: "additional properties boolean",
			input: &model.Schema{
				Kind: model.KindObject,
				Additional: &model.Additional{
					Allow: boolPtr(false),
				},
			},
			validate: func(t *testing.T, result *SchemaV30, warns diag.Warnings) {
				t.Helper()
				require.NotNil(t, result)
				assert.Equal(t, false, result.AdditionalProps)
			},
		},
		{
			name: "additional properties schema",
			input: &model.Schema{
				Kind: model.KindObject,
				Additional: &model.Additional{
					Schema: &model.Schema{
						Kind: model.KindString,
					},
				},
			},
			validate: func(t *testing.T, result *SchemaV30, warns diag.Warnings) {
				t.Helper()
				require.NotNil(t, result)
				require.NotNil(t, result.AdditionalProps)
				schema, ok := result.AdditionalProps.(*SchemaV30)
				require.True(t, ok)
				assert.Equal(t, "string", schema.Type)
			},
		},
		{
			name: "composition allOf",
			input: &model.Schema{
				Kind: model.KindObject,
				AllOf: []*model.Schema{
					{
						Kind: model.KindObject,
						Properties: map[string]*model.Schema{
							"name": {Kind: model.KindString},
						},
					},
					{
						Kind: model.KindObject,
						Properties: map[string]*model.Schema{
							"age": {Kind: model.KindInteger},
						},
					},
				},
			},
			validate: func(t *testing.T, result *SchemaV30, warns diag.Warnings) {
				t.Helper()
				require.NotNil(t, result)
				require.Len(t, result.AllOf, 2)
			},
		},
		{
			name: "composition anyOf",
			input: &model.Schema{
				Kind: model.KindObject,
				AnyOf: []*model.Schema{
					{Kind: model.KindString},
					{Kind: model.KindInteger},
				},
			},
			validate: func(t *testing.T, result *SchemaV30, warns diag.Warnings) {
				t.Helper()
				require.NotNil(t, result)
				require.Len(t, result.AnyOf, 2)
			},
		},
		{
			name: "composition oneOf",
			input: &model.Schema{
				Kind: model.KindObject,
				OneOf: []*model.Schema{
					{Kind: model.KindString},
					{Kind: model.KindInteger},
				},
			},
			validate: func(t *testing.T, result *SchemaV30, warns diag.Warnings) {
				t.Helper()
				require.NotNil(t, result)
				require.Len(t, result.OneOf, 2)
			},
		},
		{
			name: "composition not",
			input: &model.Schema{
				Kind: model.KindObject,
				Not: &model.Schema{
					Kind: model.KindString,
				},
			},
			validate: func(t *testing.T, result *SchemaV30, warns diag.Warnings) {
				t.Helper()
				require.NotNil(t, result)
				require.NotNil(t, result.Not)
				assert.Equal(t, "string", result.Not.Type)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &proj30{}
			result := schema30(tt.input, p, "#/test")

			if tt.wantWarn {
				assert.NotEmpty(t, p.warns)
			}

			if tt.validate != nil {
				tt.validate(t, result, p.warns)
			}
		})
	}
}

func TestSchemaV30_MarshalJSON(t *testing.T) {
	t.Parallel()

	schema := &SchemaV30{
		Type:        "string",
		Description: "Test schema",
		Extensions: map[string]any{
			"x-custom": "value",
		},
	}

	data, err := json.Marshal(schema)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Equal(t, "string", m["type"])
	assert.Equal(t, "Test schema", m["description"])
	assert.Equal(t, "value", m["x-custom"])
}

func TestKindToString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind model.Kind
		want string
	}{
		{
			name: "boolean",
			kind: model.KindBoolean,
			want: "boolean",
		},
		{
			name: "integer",
			kind: model.KindInteger,
			want: "integer",
		},
		{
			name: "number",
			kind: model.KindNumber,
			want: "number",
		},
		{
			name: "string",
			kind: model.KindString,
			want: "string",
		},
		{
			name: "object",
			kind: model.KindObject,
			want: "object",
		},
		{
			name: "array",
			kind: model.KindArray,
			want: "array",
		},
		{
			name: "unknown",
			kind: model.KindUnknown,
			want: "",
		},
		{
			name: "null",
			kind: model.KindNull,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := kindToString(tt.kind)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}
