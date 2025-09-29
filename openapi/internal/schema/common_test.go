package schema

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test types for walkFields testing
type BaseStruct struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type EmbeddedStruct struct {
	BaseStruct
	Email string `json:"email"`
}

type NestedEmbedded struct {
	BaseStruct
	EmbeddedStruct
	Age int `json:"age"`
}

type PointerEmbedded struct {
	*BaseStruct
	Status string `json:"status"`
}

type RegularStruct struct {
	Field1 string `json:"field1"`
	Field2 int    `json:"field2"`
}

func TestWalkFields(t *testing.T) {
	t.Parallel()

	t.Run("walks regular struct fields", func(t *testing.T) {
		t.Parallel()

		typ := reflect.TypeOf(RegularStruct{})
		var fields []string

		walkFields(typ, func(f reflect.StructField) {
			fields = append(fields, f.Name)
		})

		assert.Equal(t, []string{"Field1", "Field2"}, fields)
	})

	t.Run("walks embedded struct fields", func(t *testing.T) {
		t.Parallel()

		typ := reflect.TypeOf(EmbeddedStruct{})
		var fields []string

		walkFields(typ, func(f reflect.StructField) {
			fields = append(fields, f.Name)
		})

		// Should include fields from BaseStruct (ID, Name) and EmbeddedStruct (Email)
		assert.Contains(t, fields, "ID")
		assert.Contains(t, fields, "Name")
		assert.Contains(t, fields, "Email")
		assert.Len(t, fields, 3)
	})

	t.Run("walks nested embedded struct fields", func(t *testing.T) {
		t.Parallel()

		typ := reflect.TypeOf(NestedEmbedded{})
		var fields []string

		walkFields(typ, func(f reflect.StructField) {
			fields = append(fields, f.Name)
		})

		// Should include all fields from BaseStruct, EmbeddedStruct, and NestedEmbedded
		// Note: BaseStruct fields appear twice because it's embedded both directly
		// and through EmbeddedStruct, which is expected behavior
		assert.Contains(t, fields, "ID")
		assert.Contains(t, fields, "Name")
		assert.Contains(t, fields, "Email")
		assert.Contains(t, fields, "Age")
		// BaseStruct fields appear twice (once directly, once through EmbeddedStruct)
		assert.GreaterOrEqual(t, len(fields), 4)
	})

	t.Run("handles pointer embedded structs", func(t *testing.T) {
		typ := reflect.TypeOf(PointerEmbedded{})
		var fields []string

		walkFields(typ, func(f reflect.StructField) {
			fields = append(fields, f.Name)
		})

		// Should include fields from *BaseStruct (ID, Name) and PointerEmbedded (Status)
		assert.Contains(t, fields, "ID")
		assert.Contains(t, fields, "Name")
		assert.Contains(t, fields, "Status")
		assert.Len(t, fields, 3)
	})

	t.Run("handles pointer types", func(t *testing.T) {
		typ := reflect.TypeOf(&RegularStruct{})
		var fields []string

		walkFields(typ, func(f reflect.StructField) {
			fields = append(fields, f.Name)
		})

		assert.Equal(t, []string{"Field1", "Field2"}, fields)
	})

	t.Run("visits fields in order", func(t *testing.T) {
		typ := reflect.TypeOf(RegularStruct{})
		var fields []string

		walkFields(typ, func(f reflect.StructField) {
			fields = append(fields, f.Name)
		})

		// Fields should be visited in declaration order
		assert.Equal(t, []string{"Field1", "Field2"}, fields)
	})

	t.Run("skips anonymous fields themselves", func(t *testing.T) {
		typ := reflect.TypeOf(EmbeddedStruct{})
		var anonymousFields []string
		var regularFields []string

		walkFields(typ, func(f reflect.StructField) {
			if f.Anonymous {
				anonymousFields = append(anonymousFields, f.Name)
			} else {
				regularFields = append(regularFields, f.Name)
			}
		})

		// Anonymous field (BaseStruct) should not be in the list
		// Only its fields should be visited
		assert.Empty(t, anonymousFields)
		assert.Contains(t, regularFields, "ID")
		assert.Contains(t, regularFields, "Name")
		assert.Contains(t, regularFields, "Email")
	})

	t.Run("handles empty struct", func(t *testing.T) {
		type Empty struct{}
		typ := reflect.TypeOf(Empty{})
		var fields []string

		walkFields(typ, func(f reflect.StructField) {
			fields = append(fields, f.Name)
		})

		assert.Empty(t, fields)
	})

	t.Run("handles deeply nested embedding", func(t *testing.T) {
		type Level1 struct {
			Field1 string `json:"field1"`
		}
		type Level2 struct {
			Level1
			Field2 string `json:"field2"`
		}
		type Level3 struct {
			Level2
			Field3 string `json:"field3"`
		}

		typ := reflect.TypeOf(Level3{})
		var fields []string

		walkFields(typ, func(f reflect.StructField) {
			fields = append(fields, f.Name)
		})

		assert.Contains(t, fields, "Field1")
		assert.Contains(t, fields, "Field2")
		assert.Contains(t, fields, "Field3")
		assert.Len(t, fields, 3)
	})
}

func TestSchemaName(t *testing.T) {
	t.Run("returns type name for types in current package", func(t *testing.T) {
		typ := reflect.TypeOf(RegularStruct{})
		name := schemaName(typ)

		// Should include package name if package name differs from type name
		// The actual format depends on package structure
		assert.Contains(t, name, "RegularStruct")
	})

	t.Run("includes package name for external types", func(t *testing.T) {
		// Test with a standard library type
		typ := reflect.TypeOf("")
		name := schemaName(typ)

		// Built-in types should return just the type name
		assert.Equal(t, "string", name)
	})

	t.Run("handles built-in types", func(t *testing.T) {
		testCases := []struct {
			name     string
			typ      reflect.Type
			expected string
		}{
			{"int", reflect.TypeOf(0), "int"},
			{"string", reflect.TypeOf(""), "string"},
			{"bool", reflect.TypeOf(false), "bool"},
			{"float64", reflect.TypeOf(0.0), "float64"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := schemaName(tc.typ)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("handles pointer types", func(t *testing.T) {
		typ := reflect.TypeOf(&RegularStruct{})
		name := schemaName(typ)

		// Pointer types have no name, so should return empty string
		// The function doesn't dereference pointers
		assert.Equal(t, "", name)
	})

	t.Run("handles slice types", func(t *testing.T) {
		typ := reflect.TypeOf([]RegularStruct{})
		name := schemaName(typ)

		// Unnamed types (like slices) should return empty string
		assert.Equal(t, "", name)
	})

	t.Run("handles map types", func(t *testing.T) {
		typ := reflect.TypeOf(map[string]int{})
		name := schemaName(typ)

		// Unnamed types (like maps) should return empty string
		assert.Equal(t, "", name)
	})

	t.Run("handles unnamed types", func(t *testing.T) {
		// Anonymous struct
		anon := struct {
			Field string
		}{}
		typ := reflect.TypeOf(anon)
		name := schemaName(typ)

		// Unnamed types should return empty string
		assert.Equal(t, "", name)
	})

	t.Run("handles types with package path", func(t *testing.T) {
		// Use a type from another package to test package name extraction
		// We'll use a type from the reflect package
		typ := reflect.TypeOf(reflect.Value{})
		name := schemaName(typ)

		// Should include package name
		assert.Contains(t, name, "Value")
		// Should include package name prefix
		if typ.PkgPath() != "" {
			assert.Contains(t, name, ".")
		}
	})

	t.Run("handles empty package path", func(t *testing.T) {
		// Built-in types have empty package path
		typ := reflect.TypeOf(42)
		name := schemaName(typ)

		assert.Equal(t, "int", name)
	})

	t.Run("handles package name same as type name", func(t *testing.T) {
		// This is a bit tricky to test without creating a new package
		// But we can test the logic with current types
		typ := reflect.TypeOf(RegularStruct{})
		name := schemaName(typ)

		// If package name equals type name, should return just type name
		// This depends on the actual package structure
		assert.NotEmpty(t, name)
	})
}

func TestParseJSONName(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		fallback string
		expected string
	}{
		{
			name:     "simple json tag",
			tag:      "name",
			fallback: "field",
			expected: "name",
		},
		{
			name:     "json tag with omitempty",
			tag:      "name,omitempty",
			fallback: "field",
			expected: "name",
		},
		{
			name:     "json tag with multiple options",
			tag:      "name,omitempty,string",
			fallback: "field",
			expected: "name",
		},
		{
			name:     "empty json tag",
			tag:      "",
			fallback: "field",
			expected: "field",
		},
		{
			name:     "json tag with empty name",
			tag:      ",omitempty",
			fallback: "field",
			expected: "field",
		},
		{
			name:     "json tag with dash (ignore)",
			tag:      "-",
			fallback: "field",
			expected: "-",
		},
		{
			name:     "json tag with dash and options",
			tag:      "-,omitempty",
			fallback: "field",
			expected: "-",
		},
		{
			name:     "empty tag with empty fallback",
			tag:      "",
			fallback: "",
			expected: "",
		},
		{
			name:     "json tag with only comma",
			tag:      ",",
			fallback: "field",
			expected: "field",
		},
		{
			name:     "json tag with whitespace",
			tag:      " name ,omitempty",
			fallback: "field",
			expected: " name ",
		},
		{
			name:     "json tag with underscore",
			tag:      "user_name",
			fallback: "field",
			expected: "user_name",
		},
		{
			name:     "json tag with camelCase",
			tag:      "userName",
			fallback: "field",
			expected: "userName",
		},
		{
			name:     "json tag with numbers",
			tag:      "field123",
			fallback: "field",
			expected: "field123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseJSONName(tt.tag, tt.fallback)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsFieldRequired(t *testing.T) {
	t.Run("returns false for pointer types", func(t *testing.T) {
		type TestStruct struct {
			Field *string `validate:"required"`
		}
		typ := reflect.TypeOf(TestStruct{})
		f, _ := typ.FieldByName("Field")

		result := isFieldRequired(f)
		assert.False(t, result, "pointer types should not be required")
	})

	t.Run("returns true for non-pointer with required tag", func(t *testing.T) {
		type TestStruct struct {
			Field string `validate:"required"`
		}
		typ := reflect.TypeOf(TestStruct{})
		f, _ := typ.FieldByName("Field")

		result := isFieldRequired(f)
		assert.True(t, result)
	})

	t.Run("returns false for non-pointer without required tag", func(t *testing.T) {
		type TestStruct struct {
			Field string
		}
		typ := reflect.TypeOf(TestStruct{})
		f, _ := typ.FieldByName("Field")

		result := isFieldRequired(f)
		assert.False(t, result)
	})

	t.Run("returns false for non-pointer with other validate tags", func(t *testing.T) {
		type TestStruct struct {
			Field string `validate:"email,min=5"`
		}
		typ := reflect.TypeOf(TestStruct{})
		f, _ := typ.FieldByName("Field")

		result := isFieldRequired(f)
		assert.False(t, result)
	})

	t.Run("returns true when required is part of multiple tags", func(t *testing.T) {
		type TestStruct struct {
			Field string `validate:"email,required,min=5"`
		}
		typ := reflect.TypeOf(TestStruct{})
		f, _ := typ.FieldByName("Field")

		result := isFieldRequired(f)
		assert.True(t, result)
	})

	t.Run("handles int types", func(t *testing.T) {
		type TestStruct struct {
			Required int `validate:"required"`
			Optional int
			Ptr      *int `validate:"required"`
		}
		typ := reflect.TypeOf(TestStruct{})
		required, _ := typ.FieldByName("Required")
		optional, _ := typ.FieldByName("Optional")
		ptr, _ := typ.FieldByName("Ptr")

		assert.True(t, isFieldRequired(required))
		assert.False(t, isFieldRequired(optional))
		assert.False(t, isFieldRequired(ptr))
	})

	t.Run("handles bool types", func(t *testing.T) {
		type TestStruct struct {
			Required bool `validate:"required"`
			Optional bool
		}
		typ := reflect.TypeOf(TestStruct{})
		required, _ := typ.FieldByName("Required")
		optional, _ := typ.FieldByName("Optional")

		assert.True(t, isFieldRequired(required))
		assert.False(t, isFieldRequired(optional))
	})

	t.Run("handles slice types", func(t *testing.T) {
		type TestStruct struct {
			Required []string `validate:"required"`
			Optional []string
			Ptr      *[]string `validate:"required"`
		}
		typ := reflect.TypeOf(TestStruct{})
		required, _ := typ.FieldByName("Required")
		optional, _ := typ.FieldByName("Optional")
		ptr, _ := typ.FieldByName("Ptr")

		assert.True(t, isFieldRequired(required))
		assert.False(t, isFieldRequired(optional))
		assert.False(t, isFieldRequired(ptr))
	})

	t.Run("handles map types", func(t *testing.T) {
		type TestStruct struct {
			Required map[string]int `validate:"required"`
			Optional map[string]int
		}
		typ := reflect.TypeOf(TestStruct{})
		required, _ := typ.FieldByName("Required")
		optional, _ := typ.FieldByName("Optional")

		assert.True(t, isFieldRequired(required))
		assert.False(t, isFieldRequired(optional))
	})

	t.Run("case sensitive required check", func(t *testing.T) {
		type TestStruct struct {
			Field1 string `validate:"Required"` // uppercase
			Field2 string `validate:"REQUIRED"` // all uppercase
			Field3 string `validate:"required"` // lowercase (correct)
		}
		typ := reflect.TypeOf(TestStruct{})
		f1, _ := typ.FieldByName("Field1")
		f2, _ := typ.FieldByName("Field2")
		f3, _ := typ.FieldByName("Field3")

		// strings.Contains is case-sensitive
		assert.False(t, isFieldRequired(f1))
		assert.False(t, isFieldRequired(f2))
		assert.True(t, isFieldRequired(f3))
	})

	t.Run("handles empty validate tag", func(t *testing.T) {
		type TestStruct struct {
			Field string `validate:""`
		}
		typ := reflect.TypeOf(TestStruct{})
		f, _ := typ.FieldByName("Field")

		result := isFieldRequired(f)
		assert.False(t, result)
	})

	t.Run("handles missing validate tag", func(t *testing.T) {
		type TestStruct struct {
			Field string
		}
		typ := reflect.TypeOf(TestStruct{})
		f, _ := typ.FieldByName("Field")

		result := isFieldRequired(f)
		assert.False(t, result)
	})
}
