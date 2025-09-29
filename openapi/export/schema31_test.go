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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rivaas.dev/openapi/model"
)

func TestSchema31(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *model.Schema
		validate func(t *testing.T, result *SchemaV31, warns []Warning)
	}{
		{
			name:  "nil schema",
			input: nil,
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
				assert.Nil(t, result)
			},
		},
		{
			name: "schema with ref",
			input: &model.Schema{
				Ref: "#/components/schemas/User",
			},
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
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
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
				require.NotNil(t, result)
				assert.Equal(t, "string", result.Type)
				assert.Equal(t, "Name", result.Title)
				assert.Equal(t, "User name", result.Description)
				assert.Equal(t, "string", result.Format)
			},
		},
		{
			name: "nullable string uses type union",
			input: &model.Schema{
				Kind:     model.KindString,
				Nullable: true,
			},
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
				require.NotNil(t, result)
				typeVal, ok := result.Type.([]string)
				require.True(t, ok)
				assert.Contains(t, typeVal, "string")
				assert.Contains(t, typeVal, "null")
			},
		},
		{
			name: "integer schema with exclusive bounds",
			input: &model.Schema{
				Kind: model.KindInteger,
				Minimum: &model.Bound{
					Value:     10,
					Exclusive: true,
				},
				Maximum: &model.Bound{
					Value:     100,
					Exclusive: true,
				},
			},
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
				require.NotNil(t, result)
				assert.Equal(t, "integer", result.Type)
				require.NotNil(t, result.ExclusiveMinimum)
				assert.Equal(t, 10.0, *result.ExclusiveMinimum)
				require.NotNil(t, result.ExclusiveMaximum)
				assert.Equal(t, 100.0, *result.ExclusiveMaximum)
			},
		},
		{
			name: "integer schema with inclusive bounds",
			input: &model.Schema{
				Kind: model.KindInteger,
				Minimum: &model.Bound{
					Value:     10,
					Exclusive: false,
				},
				Maximum: &model.Bound{
					Value:     100,
					Exclusive: false,
				},
			},
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
				require.NotNil(t, result)
				require.NotNil(t, result.Minimum)
				assert.Equal(t, 10.0, *result.Minimum)
				require.NotNil(t, result.Maximum)
				assert.Equal(t, 100.0, *result.Maximum)
			},
		},
		{
			name: "const supported natively",
			input: &model.Schema{
				Kind:  model.KindString,
				Const: "fixed-value",
			},
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
				require.NotNil(t, result)
				assert.Equal(t, "fixed-value", result.Const)
				assert.Equal(t, 0, len(warns), "should not warn about const in 3.1")
			},
		},
		{
			name: "unevaluated properties supported",
			input: &model.Schema{
				Kind: model.KindObject,
				Unevaluated: &model.Schema{
					Kind: model.KindString,
				},
			},
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
				require.NotNil(t, result)
				require.NotNil(t, result.UnevaluatedProps)
				assert.Equal(t, "string", result.UnevaluatedProps.Type)
				assert.Equal(t, 0, len(warns), "should not warn about unevaluated properties in 3.1")
			},
		},
		{
			name: "pattern properties supported",
			input: &model.Schema{
				Kind: model.KindObject,
				PatternProps: map[string]*model.Schema{
					"^S_": {
						Kind: model.KindString,
					},
					"^I_": {
						Kind: model.KindInteger,
					},
				},
			},
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
				require.NotNil(t, result)
				require.NotNil(t, result.PatternProperties)
				assert.Contains(t, result.PatternProperties, "^S_")
				assert.Contains(t, result.PatternProperties, "^I_")
			},
		},
		{
			name: "multiple examples supported",
			input: &model.Schema{
				Kind:     model.KindString,
				Examples: []any{"example1", "example2", "example3"},
			},
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
				require.NotNil(t, result)
				require.Len(t, result.Examples, 3)
				assert.Equal(t, []any{"example1", "example2", "example3"}, result.Examples)
				assert.Equal(t, 0, len(warns), "should not warn about multiple examples in 3.1")
			},
		},
		{
			name: "example converted to examples array",
			input: &model.Schema{
				Kind:    model.KindString,
				Example: "single-example",
			},
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
				require.NotNil(t, result)
				require.Len(t, result.Examples, 1)
				assert.Equal(t, "single-example", result.Examples[0])
			},
		},
		{
			name: "object with min/max properties",
			input: &model.Schema{
				Kind:          model.KindObject,
				MinProperties: intPtr(1),
				MaxProperties: intPtr(10),
			},
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
				require.NotNil(t, result)
				require.NotNil(t, result.MinProperties)
				assert.Equal(t, 1, *result.MinProperties)
				require.NotNil(t, result.MaxProperties)
				assert.Equal(t, 10, *result.MaxProperties)
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
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
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
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
				require.NotNil(t, result)
				assert.Equal(t, "object", result.Type)
				require.NotNil(t, result.Properties)
				assert.Contains(t, result.Properties, "name")
				assert.Contains(t, result.Properties, "age")
				assert.Contains(t, result.Required, "name")
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
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
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
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
				require.NotNil(t, result)
				require.NotNil(t, result.AdditionalProps)
				schema, ok := result.AdditionalProps.(*SchemaV31)
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
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
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
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
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
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
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
			validate: func(t *testing.T, result *SchemaV31, warns []Warning) {
				require.NotNil(t, result)
				require.NotNil(t, result.Not)
				assert.Equal(t, "string", result.Not.Type)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var warns []Warning
			result := schema31(tt.input, &warns, "#/test")

			if tt.validate != nil {
				tt.validate(t, result, warns)
			}
		})
	}
}

func TestSchemaV31_MarshalJSON(t *testing.T) {
	t.Parallel()

	schema := &SchemaV31{
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

func TestSchemaV31_TypeUnion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *model.Schema
		validate func(t *testing.T, result *SchemaV31)
	}{
		{
			name: "nullable string",
			input: &model.Schema{
				Kind:     model.KindString,
				Nullable: true,
			},
			validate: func(t *testing.T, result *SchemaV31) {
				typeVal, ok := result.Type.([]string)
				require.True(t, ok)
				assert.Len(t, typeVal, 2)
				assert.Contains(t, typeVal, "string")
				assert.Contains(t, typeVal, "null")
			},
		},
		{
			name: "non-nullable string",
			input: &model.Schema{
				Kind:     model.KindString,
				Nullable: false,
			},
			validate: func(t *testing.T, result *SchemaV31) {
				assert.Equal(t, "string", result.Type)
			},
		},
		{
			name: "nullable integer",
			input: &model.Schema{
				Kind:     model.KindInteger,
				Nullable: true,
			},
			validate: func(t *testing.T, result *SchemaV31) {
				typeVal, ok := result.Type.([]string)
				require.True(t, ok)
				assert.Len(t, typeVal, 2)
				assert.Contains(t, typeVal, "integer")
				assert.Contains(t, typeVal, "null")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var warns []Warning
			result := schema31(tt.input, &warns, "#/test")

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}
