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

package schema

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/openapi/model"
)

type TestStruct struct {
	Name     string    `json:"name" doc:"User name" example:"John" validate:"required"`
	Age      int       `json:"age" validate:"required,min=0,max=150"`
	Email    string    `json:"email,omitempty" validate:"email"`
	Tags     []string  `json:"tags" validate:"required"`
	Created  time.Time `json:"created" validate:"required"`
	Optional *string   `json:"optional,omitempty"`
}

// newTestSchemaGenerator creates a new SchemaGenerator for testing.
func newTestSchemaGenerator(tb testing.TB) *SchemaGenerator {
	tb.Helper()
	return NewSchemaGenerator()
}

// generateTestSchema generates a schema for testing.
func generateTestSchema(tb testing.TB, input any) *model.Schema {
	tb.Helper()
	gen := newTestSchemaGenerator(tb)
	if input == nil {
		return gen.Generate(nil)
	}

	return gen.Generate(reflect.TypeOf(input))
}

func TestSchemaGenerator_Generate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		validate func(t *testing.T, schema *model.Schema)
	}{
		{
			name:  "string type",
			input: "",
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				assert.Equal(t, model.KindString, s.Kind)
			},
		},
		{
			name:  "int type",
			input: 0,
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				assert.Equal(t, model.KindInteger, s.Kind)
				assert.Equal(t, "int32", s.Format)
			},
		},
		{
			name:  "int64 type",
			input: int64(0),
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				assert.Equal(t, model.KindInteger, s.Kind)
				assert.Equal(t, "int64", s.Format)
			},
		},
		{
			name:  "bool type",
			input: false,
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				assert.Equal(t, model.KindBoolean, s.Kind)
			},
		},
		{
			name:  "float32 type",
			input: float32(0),
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				assert.Equal(t, model.KindNumber, s.Kind)
				assert.Equal(t, "float", s.Format)
			},
		},
		{
			name:  "float64 type",
			input: float64(0),
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				assert.Equal(t, model.KindNumber, s.Kind)
				assert.Equal(t, "double", s.Format)
			},
		},
		{
			name:  "slice type",
			input: []string{},
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				assert.Equal(t, model.KindArray, s.Kind)
				require.NotNil(t, s.Items)
				assert.Equal(t, model.KindString, s.Items.Kind)
			},
		},
		{
			name:  "array type",
			input: [5]int{},
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				assert.Equal(t, model.KindArray, s.Kind)
				require.NotNil(t, s.Items)
				assert.Equal(t, model.KindInteger, s.Items.Kind)
			},
		},
		{
			name:  "map type",
			input: map[string]int{},
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				assert.Equal(t, model.KindObject, s.Kind)
				require.NotNil(t, s.Additional)
				require.NotNil(t, s.Additional.Schema)
				assert.Equal(t, model.KindInteger, s.Additional.Schema.Kind)
			},
		},
		{
			name:  "time.Time type",
			input: time.Time{},
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				assert.Equal(t, model.KindString, s.Kind)
				assert.Equal(t, "date-time", s.Format)
				assert.NotNil(t, s.Example)
			},
		},
		{
			name:  "[]byte type (binary data)",
			input: []byte{},
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				assert.Equal(t, model.KindString, s.Kind)
				assert.Equal(t, "base64", s.ContentEncoding)
			},
		},
		{
			name:  "pointer type (nullable)",
			input: new(string),
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				assert.Equal(t, model.KindString, s.Kind)
				assert.True(t, s.Nullable)
			},
		},
		{
			name:  "struct type",
			input: TestStruct{},
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				assert.NotEmpty(t, s.Ref)
				assert.Contains(t, s.Ref, "#/components/schemas/")
			},
		},
		{
			name:  "nil type",
			input: nil,
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				assert.Equal(t, model.KindObject, s.Kind)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			schema := generateTestSchema(t, tt.input)
			require.NotNil(t, schema)
			tt.validate(t, schema)
		})
	}
}

//nolint:paralleltest // Tests schema generation
func TestSchemaGenerator_StructTags(t *testing.T) {
	sg := newTestSchemaGenerator(t)
	schema := sg.Generate(reflect.TypeFor[TestStruct]())

	// Should create component schema
	assert.NotEmpty(t, schema.Ref)

	// Check component schemas
	schemas := sg.GetComponentSchemas()
	require.Contains(t, schemas, "schema.TestStruct")

	ts := schemas["schema.TestStruct"]
	require.NotNil(t, ts)
	assert.Equal(t, model.KindObject, ts.Kind)

	// Check properties
	require.Contains(t, ts.Properties, "name")
	assert.Equal(t, "User name", ts.Properties["name"].Description)
	assert.Equal(t, "John", ts.Properties["name"].Example)

	require.Contains(t, ts.Properties, "age")
	assert.NotNil(t, ts.Properties["age"].Minimum)
	assert.InDelta(t, 0.0, ts.Properties["age"].Minimum.Value, 0.001)
	assert.NotNil(t, ts.Properties["age"].Maximum)
	assert.InDelta(t, 150.0, ts.Properties["age"].Maximum.Value, 0.001)

	require.Contains(t, ts.Properties, "email")
	assert.Equal(t, "email", ts.Properties["email"].Format)

	require.Contains(t, ts.Properties, "tags")
	assert.Equal(t, model.KindArray, ts.Properties["tags"].Kind)
	require.NotNil(t, ts.Properties["tags"].Items)
	assert.Equal(t, model.KindString, ts.Properties["tags"].Items.Kind)

	require.Contains(t, ts.Properties, "created")
	assert.Equal(t, "date-time", ts.Properties["created"].Format)

	// Check nullable
	require.Contains(t, ts.Properties, "optional")
	assert.True(t, ts.Properties["optional"].Nullable)

	// Check required
	assert.Contains(t, ts.Required, "name")
	assert.Contains(t, ts.Required, "age")
	assert.Contains(t, ts.Required, "tags")
	assert.Contains(t, ts.Required, "created")
	assert.NotContains(t, ts.Required, "email")    // omitempty
	assert.NotContains(t, ts.Required, "optional") // pointer
}

//nolint:paralleltest // Tests schema generation
func TestSchemaGenerator_GenerateProjected(t *testing.T) {
	type MixedStruct struct {
		QueryParam string `query:"q"`
		PathParam  int    `path:"id"`
		BodyField  string `json:"body"`
		Ignored    string // no tags
	}

	sg := newTestSchemaGenerator(t)
	schema := sg.GenerateProjected(reflect.TypeFor[MixedStruct](), func(f reflect.StructField) bool {
		jsonTag := f.Tag.Get("json")
		return jsonTag != "" && jsonTag != "-"
	})

	// Should create component schema
	assert.NotEmpty(t, schema.Ref)
	assert.Contains(t, schema.Ref, "Body")

	// Check component schemas
	schemas := sg.GetComponentSchemas()
	require.Contains(t, schemas, "schema.MixedStructBody")

	body := schemas["schema.MixedStructBody"]
	require.NotNil(t, body)

	// Should only have JSON-tagged fields
	assert.Contains(t, body.Properties, "body")
	assert.NotContains(t, body.Properties, "queryParam")
	assert.NotContains(t, body.Properties, "pathParam")
	assert.NotContains(t, body.Properties, "ignored")
}

//nolint:paralleltest // Tests schema generation
func TestSchemaGenerator_CircularReference(t *testing.T) {
	type Node struct {
		Value string
		Next  *Node
	}

	sg := newTestSchemaGenerator(t)
	schema := sg.Generate(reflect.TypeFor[Node]())

	// Should create component schema
	assert.NotEmpty(t, schema.Ref)

	schemas := sg.GetComponentSchemas()
	require.Contains(t, schemas, "schema.Node")

	node := schemas["schema.Node"]
	require.NotNil(t, node)
	assert.Equal(t, model.KindObject, node.Kind)

	// Next field should be a reference, not cause infinite recursion
	require.Contains(t, node.Properties, "Next")
	nextSchema := node.Properties["Next"]
	assert.True(t, nextSchema.Nullable)
	// The actual schema should be a reference when accessed
	assert.NotEmpty(t, nextSchema.Ref)
}

func TestSchemaGenerator_ValidationConstraints(t *testing.T) {
	t.Parallel()

	type ValidatedStruct struct {
		Email  string `validate:"email"`
		URL    string `validate:"url"`
		UUID   string `validate:"uuid"`
		MinLen string `validate:"minlen=5"`
		MaxLen string `validate:"maxlen=10"`
		Min    int    `validate:"min=10"`
		Max    int    `validate:"max=100"`
		Enum   string `validate:"oneof=red green blue"`
	}

	sg := newTestSchemaGenerator(t)
	_ = sg.Generate(reflect.TypeFor[ValidatedStruct]())

	schemas := sg.GetComponentSchemas()
	vs := schemas["schema.ValidatedStruct"]
	require.NotNil(t, vs)

	tests := []struct {
		field    string
		validate func(t *testing.T, s *model.Schema)
	}{
		{
			field: "Email",
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				assert.Equal(t, "email", s.Format)
			},
		},
		{
			field: "URL",
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				assert.Equal(t, "uri", s.Format)
			},
		},
		{
			field: "UUID",
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				assert.Equal(t, "uuid", s.Format)
			},
		},
		{
			field: "MinLen",
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				require.NotNil(t, s.MinLength)
				assert.Equal(t, 5, *s.MinLength)
			},
		},
		{
			field: "MaxLen",
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				require.NotNil(t, s.MaxLength)
				assert.Equal(t, 10, *s.MaxLength)
			},
		},
		{
			field: "Min",
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				require.NotNil(t, s.Minimum)
				assert.InDelta(t, 10.0, s.Minimum.Value, 0.001)
				assert.False(t, s.Minimum.Exclusive)
			},
		},
		{
			field: "Max",
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				require.NotNil(t, s.Maximum)
				assert.InDelta(t, 100.0, s.Maximum.Value, 0.001)
				assert.False(t, s.Maximum.Exclusive)
			},
		},
		{
			field: "Enum",
			validate: func(t *testing.T, s *model.Schema) {
				t.Helper()
				require.Len(t, s.Enum, 3)
				assert.Contains(t, s.Enum, "red")
				assert.Contains(t, s.Enum, "green")
				assert.Contains(t, s.Enum, "blue")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			t.Parallel()
			fieldSchema, ok := vs.Properties[tt.field]
			require.True(t, ok, "field %s not found", tt.field)
			tt.validate(t, fieldSchema)
		})
	}
}

func TestSchemaGenerator_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		validate func(t *testing.T, schema *model.Schema, schemas map[string]*model.Schema)
	}{
		{
			name:  "empty struct",
			input: struct{}{},
			validate: func(t *testing.T, schema *model.Schema, schemas map[string]*model.Schema) {
				t.Helper()
				// Empty structs without names don't create component schemas
				// They return an inline object schema
				assert.Equal(t, model.KindObject, schema.Kind)
				assert.Empty(t, schema.Properties)
			},
		},
		{
			name: "nested struct",
			input: struct {
				User struct {
					Name string `json:"name"`
					Age  int    `json:"age"`
				} `json:"user"`
			}{},
			validate: func(t *testing.T, schema *model.Schema, schemas map[string]*model.Schema) {
				t.Helper()
				// Anonymous nested structs are inlined
				assert.Equal(t, model.KindObject, schema.Kind)
				assert.Contains(t, schema.Properties, "user")
				// Nested struct should be an object
				assert.Equal(t, model.KindObject, schema.Properties["user"].Kind)
			},
		},
		{
			name:  "map with non-string key",
			input: map[int]string{},
			validate: func(t *testing.T, schema *model.Schema, schemas map[string]*model.Schema) {
				t.Helper()
				// Non-string keys should fallback to object
				assert.Equal(t, model.KindObject, schema.Kind)
			},
		},
		{
			name:  "any type",
			input: any(nil),
			validate: func(t *testing.T, schema *model.Schema, schemas map[string]*model.Schema) {
				t.Helper()
				assert.Equal(t, model.KindObject, schema.Kind)
			},
		},
		{
			name:  "slice of pointers",
			input: []*string{},
			validate: func(t *testing.T, schema *model.Schema, schemas map[string]*model.Schema) {
				t.Helper()
				assert.Equal(t, model.KindArray, schema.Kind)
				require.NotNil(t, schema.Items)
				assert.Equal(t, model.KindString, schema.Items.Kind)
				assert.True(t, schema.Items.Nullable)
			},
		},
		{
			name:  "map of slices",
			input: map[string][]int{},
			validate: func(t *testing.T, schema *model.Schema, schemas map[string]*model.Schema) {
				t.Helper()
				assert.Equal(t, model.KindObject, schema.Kind)
				require.NotNil(t, schema.Additional)
				require.NotNil(t, schema.Additional.Schema)
				assert.Equal(t, model.KindArray, schema.Additional.Schema.Kind)
			},
		},
		{
			name:  "pointer to slice",
			input: (*[]string)(nil),
			validate: func(t *testing.T, schema *model.Schema, schemas map[string]*model.Schema) {
				t.Helper()
				assert.Equal(t, model.KindArray, schema.Kind)
				assert.True(t, schema.Nullable)
				require.NotNil(t, schema.Items)
				assert.Equal(t, model.KindString, schema.Items.Kind)
			},
		},
		{
			name:  "array with fixed size",
			input: [10]int{},
			validate: func(t *testing.T, schema *model.Schema, schemas map[string]*model.Schema) {
				t.Helper()
				assert.Equal(t, model.KindArray, schema.Kind)
				require.NotNil(t, schema.Items)
				assert.Equal(t, model.KindInteger, schema.Items.Kind)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sg := newTestSchemaGenerator(t)
			var schema *model.Schema
			if tt.input == nil {
				schema = sg.Generate(nil)
			} else {
				schema = sg.Generate(reflect.TypeOf(tt.input))
			}
			require.NotNil(t, schema)
			tt.validate(t, schema, sg.GetComponentSchemas())
		})
	}
}

//nolint:paralleltest // Tests schema generation
func TestSchemaGenerator_EmbeddedStructs(t *testing.T) {
	type Base struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	type Extended struct {
		Base
		Email string `json:"email"`
	}

	sg := newTestSchemaGenerator(t)
	schema := sg.Generate(reflect.TypeFor[Extended]())

	require.NotEmpty(t, schema.Ref)
	schemas := sg.GetComponentSchemas()
	extended := schemas["schema.Extended"]
	require.NotNil(t, extended)

	// Should include fields from embedded struct
	assert.Contains(t, extended.Properties, "id")
	assert.Contains(t, extended.Properties, "name")
	assert.Contains(t, extended.Properties, "email")
}

//nolint:paralleltest // Tests schema generation
func TestSchemaGenerator_InvalidValidationTags(t *testing.T) {
	type InvalidStruct struct {
		InvalidMin    int    `validate:"min=invalid"`
		InvalidMax    int    `validate:"max=not-a-number"`
		InvalidMinLen string `validate:"minlen=abc"`
		InvalidMaxLen string `validate:"maxlen=xyz"`
		EmptyEnum     string `validate:"oneof="`
	}

	sg := newTestSchemaGenerator(t)
	schema := sg.Generate(reflect.TypeFor[InvalidStruct]())

	require.NotEmpty(t, schema.Ref)
	schemas := sg.GetComponentSchemas()
	invalid := schemas["schema.InvalidStruct"]
	require.NotNil(t, invalid)

	// Invalid validation tags should be ignored gracefully
	// Fields should still exist but without constraints
	assert.Contains(t, invalid.Properties, "InvalidMin")
	assert.Contains(t, invalid.Properties, "InvalidMax")
	assert.Contains(t, invalid.Properties, "InvalidMinLen")
	assert.Contains(t, invalid.Properties, "InvalidMaxLen")
	assert.Contains(t, invalid.Properties, "EmptyEnum")
}

//nolint:paralleltest // Tests schema generation
func TestSchemaGenerator_DefaultValues(t *testing.T) {
	type StructWithDefaults struct {
		StringField string `json:"string" example:"default"`
		IntField    int    `json:"int" example:"42"`
		BoolField   bool   `json:"bool" example:"true"`
	}

	sg := newTestSchemaGenerator(t)
	schema := sg.Generate(reflect.TypeFor[StructWithDefaults]())

	require.NotEmpty(t, schema.Ref)
	schemas := sg.GetComponentSchemas()
	s := schemas["schema.StructWithDefaults"]
	require.NotNil(t, s)

	assert.Equal(t, "default", s.Properties["string"].Example)
	// Note: example parsing might need adjustment based on actual implementation
}

//nolint:paralleltest // Tests schema generation
func TestSchemaGenerator_JSONTagVariations(t *testing.T) {
	type TagVariations struct {
		OmitEmpty  string `json:"omitempty,omitempty"`
		NoTag      string
		Ignored    string `json:"-"`
		CustomName string `json:"custom_name"` //nolint:tagliatelle // testing snake_case tag
		EmptyTag   string `json:""`            //nolint:tagliatelle // testing empty tag
		CommaOnly  string `json:","`           //nolint:tagliatelle // testing comma-only tag
	}

	sg := newTestSchemaGenerator(t)
	schema := sg.Generate(reflect.TypeFor[TagVariations]())

	require.NotEmpty(t, schema.Ref)
	schemas := sg.GetComponentSchemas()
	s := schemas["schema.TagVariations"]
	require.NotNil(t, s)

	// Fields with json:"-" should be excluded
	assert.NotContains(t, s.Properties, "Ignored")
	// Fields with custom names should use the custom name
	assert.Contains(t, s.Properties, "custom_name")
	// Fields without json tag should use field name
	assert.Contains(t, s.Properties, "NoTag")
}

func BenchmarkSchemaGenerator_Generate(b *testing.B) {
	gen := newTestSchemaGenerator(b)
	typ := reflect.TypeFor[TestStruct]()

	b.ResetTimer()
	for b.Loop() {
		_ = gen.Generate(typ)
	}
}

func BenchmarkSchemaGenerator_ComplexStruct(b *testing.B) {
	type ComplexStruct struct {
		ID       int                      `json:"id"`
		Name     string                   `json:"name"`
		Tags     []string                 `json:"tags"`
		Metadata map[string]any           `json:"metadata"`
		Nested   *ComplexStruct           `json:"nested"`
		Items    []ComplexStruct          `json:"items"`
		Data     map[string]ComplexStruct `json:"data"`
	}

	gen := newTestSchemaGenerator(b)
	typ := reflect.TypeFor[ComplexStruct]()

	b.ResetTimer()
	for b.Loop() {
		_ = gen.Generate(typ)
	}
}

func BenchmarkSchemaGenerator_GenerateProjected(b *testing.B) {
	type MixedStruct struct {
		QueryParam string `query:"q"`
		PathParam  int    `path:"id"`
		BodyField  string `json:"body"`
		Ignored    string // no tags
	}

	gen := newTestSchemaGenerator(b)
	typ := reflect.TypeFor[MixedStruct]()

	b.ResetTimer()
	for b.Loop() {
		_ = gen.GenerateProjected(typ, func(f reflect.StructField) bool {
			jsonTag := f.Tag.Get("json")
			return jsonTag != "" && jsonTag != "-"
		})
	}
}
