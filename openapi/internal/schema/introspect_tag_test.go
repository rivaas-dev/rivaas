package schema

import (
	"net"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test extractParamsFromTag function directly
//
//nolint:paralleltest // Some subtests share state
func TestExtractParamsFromTag(t *testing.T) {
	t.Parallel()

	t.Run("extracts parameters by location", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name     string
			structFn func() (reflect.Type, string, string)
			validate func(t *testing.T, params []ParamSpec)
		}{
			{
				name: "query parameters",
				structFn: func() (reflect.Type, string, string) {
					type Request struct {
						Page  int    `query:"page" default:"1" example:"1"`
						Limit int    `query:"limit" default:"10" example:"10"`
						Sort  string `query:"sort" default:"asc" example:"asc"`
					}

					return reflect.TypeFor[Request](), tagQuery, "query"
				},
				validate: func(t *testing.T, params []ParamSpec) {
					require.Len(t, params, 3)
					paramMap := make(map[string]ParamSpec)
					for _, p := range params {
						paramMap[p.Name] = p
					}

					assert.Equal(t, "query", paramMap["page"].In)
					assert.Equal(t, reflect.TypeFor[int](), paramMap["page"].Type)
					assert.Equal(t, int64(1), paramMap["page"].Default)
					assert.Equal(t, int64(1), paramMap["page"].Example)
					assert.Equal(t, "query", paramMap["limit"].In)
					assert.Equal(t, reflect.TypeFor[int](), paramMap["limit"].Type)
					assert.Equal(t, int64(10), paramMap["limit"].Default)
					assert.Equal(t, int64(10), paramMap["limit"].Example)
					assert.Equal(t, "query", paramMap["sort"].In)
					assert.Equal(t, reflect.TypeFor[string](), paramMap["sort"].Type)
					assert.Equal(t, "asc", paramMap["sort"].Default)
					assert.Equal(t, "asc", paramMap["sort"].Example)
				},
			},
			{
				name: "path parameters",
				structFn: func() (reflect.Type, string, string) {
					type Request struct {
						ID   int    `path:"id"`
						Name string `path:"name"`
					}

					return reflect.TypeFor[Request](), tagPath, "path"
				},
				validate: func(t *testing.T, params []ParamSpec) {
					require.Len(t, params, 2)
					paramMap := make(map[string]ParamSpec)
					for _, p := range params {
						paramMap[p.Name] = p
						// Path parameters are always required
						assert.True(t, p.Required, "path param %s should be required", p.Name)
					}
					assert.Equal(t, "path", paramMap["id"].In)
					assert.Equal(t, "path", paramMap["name"].In)
				},
			},
			{
				name: "header parameters",
				structFn: func() (reflect.Type, string, string) {
					type Request struct {
						APIKey    string `header:"X-API-Key"` //nolint:tagliatelle // Standard HTTP header format
						UserAgent string `header:"User-Agent"`
					}

					return reflect.TypeFor[Request](), tagHeader, "header"
				},
				validate: func(t *testing.T, params []ParamSpec) {
					require.Len(t, params, 2)
					paramMap := make(map[string]ParamSpec)
					for _, p := range params {
						paramMap[p.Name] = p
					}
					assert.Equal(t, "header", paramMap["X-API-Key"].In)
					assert.Equal(t, "header", paramMap["User-Agent"].In)
				},
			},
			{
				name: "cookie parameters",
				structFn: func() (reflect.Type, string, string) {
					type Request struct {
						Session string `cookie:"session"`
						Token   string `cookie:"token"`
					}

					return reflect.TypeFor[Request](), tagCookie, "cookie"
				},
				validate: func(t *testing.T, params []ParamSpec) {
					require.Len(t, params, 2)
					paramMap := make(map[string]ParamSpec)
					for _, p := range params {
						paramMap[p.Name] = p
					}
					assert.Equal(t, "cookie", paramMap["session"].In)
					assert.Equal(t, "cookie", paramMap["token"].In)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				typ, tagName, location := tt.structFn()
				params := extractParamsFromTag(typ, tagName, location)
				tt.validate(t, params)
			})
		}
	})

	t.Run("handles tag value variations", func(t *testing.T) {
		tests := []struct {
			name     string
			structFn func() reflect.Type
			expected int
			validate func(t *testing.T, params []ParamSpec)
		}{
			{
				name: "empty tag value and dash are ignored",
				structFn: func() reflect.Type {
					type Request struct {
						Field1 string `query:""`
						Field2 string `query:"-"`
						Field3 string `query:"valid"`
					}

					return reflect.TypeFor[Request]()
				},
				expected: 1,
				validate: func(t *testing.T, params []ParamSpec) {
					assert.Equal(t, "valid", params[0].Name)
				},
			},
			{
				name: "uses field name when tag value is empty after comma",
				structFn: func() reflect.Type {
					type Request struct {
						FieldName string `query:",omitempty"`
					}

					return reflect.TypeFor[Request]()
				},
				expected: 1,
				validate: func(t *testing.T, params []ParamSpec) {
					// When tag value is empty after comma, should use field name
					assert.Equal(t, "FieldName", params[0].Name)
				},
			},
			{
				name: "trims whitespace from tag names",
				structFn: func() reflect.Type {
					type Request struct {
						Field string `query:" field_name "`
					}

					return reflect.TypeFor[Request]()
				},
				expected: 1,
				validate: func(t *testing.T, params []ParamSpec) {
					assert.Equal(t, "field_name", params[0].Name)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				typ := tt.structFn()
				params := extractParamsFromTag(typ, tagQuery, "query")
				require.Len(t, params, tt.expected)
				if tt.validate != nil {
					tt.validate(t, params)
				}
			})
		}
	})

	t.Run("extracts metadata from tags", func(t *testing.T) {
		tests := []struct {
			name     string
			structFn func() reflect.Type
			validate func(t *testing.T, params []ParamSpec)
		}{
			{
				name: "description from doc tag",
				structFn: func() reflect.Type {
					type Request struct {
						ID int `query:"id" doc:"User identifier"`
					}

					return reflect.TypeFor[Request]()
				},
				validate: func(t *testing.T, params []ParamSpec) {
					require.Len(t, params, 1)
					assert.Equal(t, "User identifier", params[0].Description)
				},
			},
			{
				name: "example from example tag",
				structFn: func() reflect.Type {
					type Request struct {
						ID   int    `query:"id" example:"123"`
						Name string `query:"name" example:"John"`
					}

					return reflect.TypeFor[Request]()
				},
				validate: func(t *testing.T, params []ParamSpec) {
					require.Len(t, params, 2)
					paramMap := make(map[string]ParamSpec)
					for _, p := range params {
						paramMap[p.Name] = p
					}
					assert.Equal(t, int64(123), paramMap["id"].Example)
					assert.Equal(t, "John", paramMap["name"].Example)
				},
			},
			{
				name: "default from default tag",
				structFn: func() reflect.Type {
					type Request struct {
						Page int    `query:"page" default:"1"`
						Sort string `query:"sort" default:"asc"`
					}

					return reflect.TypeFor[Request]()
				},
				validate: func(t *testing.T, params []ParamSpec) {
					require.Len(t, params, 2)
					paramMap := make(map[string]ParamSpec)
					for _, p := range params {
						paramMap[p.Name] = p
					}
					assert.Equal(t, int64(1), paramMap["page"].Default)
					assert.Equal(t, "asc", paramMap["sort"].Default)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				typ := tt.structFn()
				params := extractParamsFromTag(typ, tagQuery, "query")
				tt.validate(t, params)
			})
		}
	})

	t.Run("extracts enum values", func(t *testing.T) {
		tests := []struct {
			name     string
			structFn func() reflect.Type
			validate func(t *testing.T, params []ParamSpec)
		}{
			{
				name: "from enum tag",
				structFn: func() reflect.Type {
					type Request struct {
						Status string `query:"status" enum:"pending,active,completed"`
					}

					return reflect.TypeFor[Request]()
				},
				validate: func(t *testing.T, params []ParamSpec) {
					require.Len(t, params, 1)
					assert.Equal(t, []string{"pending", "active", "completed"}, params[0].Enum)
				},
			},
			{
				name: "from validate oneof tag",
				structFn: func() reflect.Type {
					type Request struct {
						Color string `query:"color" validate:"oneof=red green blue"`
					}

					return reflect.TypeFor[Request]()
				},
				validate: func(t *testing.T, params []ParamSpec) {
					require.Len(t, params, 1)
					assert.Contains(t, params[0].Enum, "red")
					assert.Contains(t, params[0].Enum, "green")
					assert.Contains(t, params[0].Enum, "blue")
				},
			},
			{
				name: "combines enum and oneof",
				structFn: func() reflect.Type {
					type Request struct {
						Status string `query:"status" enum:"pending,active" validate:"oneof=pending active completed"`
					}

					return reflect.TypeFor[Request]()
				},
				validate: func(t *testing.T, params []ParamSpec) {
					require.Len(t, params, 1)
					// Should have all values from both enum and oneof
					assert.Contains(t, params[0].Enum, "pending")
					assert.Contains(t, params[0].Enum, "active")
					assert.Contains(t, params[0].Enum, "completed")
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				typ := tt.structFn()
				params := extractParamsFromTag(typ, tagQuery, "query")
				tt.validate(t, params)
			})
		}
	})

	t.Run("skips unexported fields", func(t *testing.T) {
		type Request struct {
			Public  string `query:"public"`
			private string `query:"private"` //nolint:unused // Testing that unexported fields are skipped
		}

		typ := reflect.TypeFor[Request]()
		params := extractParamsFromTag(typ, tagQuery, "query")

		require.Len(t, params, 1)
		assert.Equal(t, "public", params[0].Name)
	})

	t.Run("handles special characters in tag names", func(t *testing.T) {
		type Request struct {
			Field1 string `query:"field-name"`
			Field2 string `query:"field_name"`
			Field3 string `query:"field.name"`
		}

		typ := reflect.TypeFor[Request]()
		params := extractParamsFromTag(typ, tagQuery, "query")

		require.Len(t, params, 3)
		paramMap := make(map[string]ParamSpec)
		for _, p := range params {
			paramMap[p.Name] = p
		}

		assert.Contains(t, paramMap, "field-name")
		assert.Contains(t, paramMap, "field_name")
		assert.Contains(t, paramMap, "field.name")
	})

	t.Run("handles embedded structs", func(t *testing.T) {
		type BaseParams struct {
			ID int `path:"id"`
		}

		type Request struct {
			BaseParams
			Page int `query:"page"`
		}

		typ := reflect.TypeFor[Request]()
		pathParams := extractParamsFromTag(typ, tagPath, "path")
		queryParams := extractParamsFromTag(typ, tagQuery, "query")

		require.Len(t, pathParams, 1)
		assert.Equal(t, "id", pathParams[0].Name)

		require.Len(t, queryParams, 1)
		assert.Equal(t, "page", queryParams[0].Name)
	})

	t.Run("handles pointer types", func(t *testing.T) {
		type Request struct {
			ID   *int    `query:"id"`
			Name *string `query:"name"`
		}

		typ := reflect.TypeFor[Request]()
		params := extractParamsFromTag(typ, tagQuery, "query")

		require.Len(t, params, 2)
		paramMap := make(map[string]ParamSpec)
		for _, p := range params {
			paramMap[p.Name] = p
			// Pointer types should not be required (unless path param)
			assert.False(t, p.Required)
		}
	})

	t.Run("handles various field types", func(t *testing.T) {
		type Request struct {
			IntField     int     `query:"int"`
			Int8Field    int8    `query:"int8"`
			Int16Field   int16   `query:"int16"`
			Int32Field   int32   `query:"int32"`
			Int64Field   int64   `query:"int64"`
			UintField    uint    `query:"uint"`
			Uint8Field   uint8   `query:"uint8"`
			Uint16Field  uint16  `query:"uint16"`
			Uint32Field  uint32  `query:"uint32"`
			Uint64Field  uint64  `query:"uint64"`
			Float32Field float32 `query:"float32"`
			Float64Field float64 `query:"float64"`
			BoolField    bool    `query:"bool"`
			StringField  string  `query:"string"`
		}

		typ := reflect.TypeFor[Request]()
		params := extractParamsFromTag(typ, tagQuery, "query")

		require.Len(t, params, 14)
		paramMap := make(map[string]ParamSpec)
		for _, p := range params {
			paramMap[p.Name] = p
		}

		tests := []struct {
			name      string
			paramName string
			expected  reflect.Type
		}{
			{"int type", "int", reflect.TypeFor[int]()},
			{"int8 type", "int8", reflect.TypeFor[int8]()},
			{"string type", "string", reflect.TypeFor[string]()},
			{"bool type", "bool", reflect.TypeFor[bool]()},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, tt.expected, paramMap[tt.paramName].Type)
			})
		}
	})
}

func TestParseValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		typ      reflect.Type
		expected any
	}{
		// String types
		{"string value", "hello", reflect.TypeFor[string](), "hello"},
		{"string with spaces", "hello world", reflect.TypeFor[string](), "hello world"},

		// Integer types
		{"int", "42", reflect.TypeFor[int](), int64(42)},
		{"int8", "127", reflect.TypeFor[int8](), int64(127)},
		{"int16", "32767", reflect.TypeFor[int16](), int64(32767)},
		{"int32", "2147483647", reflect.TypeFor[int32](), int64(2147483647)},
		{"int64", "9223372036854775807", reflect.TypeFor[int64](), int64(9223372036854775807)},
		{"negative int", "-42", reflect.TypeFor[int](), int64(-42)},

		// Unsigned integer types
		{"uint", "42", reflect.TypeFor[uint](), uint64(42)},
		{"uint8", "255", reflect.TypeFor[uint8](), uint64(255)},
		{"uint16", "65535", reflect.TypeFor[uint16](), uint64(65535)},
		{"uint32", "4294967295", reflect.TypeFor[uint32](), uint64(4294967295)},
		{"uint64", "18446744073709551615", reflect.TypeFor[uint64](), uint64(18446744073709551615)},

		// Float types
		{"float32", "3.14", reflect.TypeFor[float32](), 3.14},
		{"float64", "3.14159", reflect.TypeFor[float64](), 3.14159},
		{"negative float", "-3.14", reflect.TypeFor[float64](), -3.14},
		{"scientific notation", "1.5e2", reflect.TypeFor[float64](), 150.0},

		// Bool types
		{"bool true", "true", reflect.TypeFor[bool](), true},
		{"bool false", "false", reflect.TypeFor[bool](), false},
		{"bool 1", "1", reflect.TypeFor[bool](), true},
		{"bool 0", "0", reflect.TypeFor[bool](), false},

		// Pointer types
		{"pointer to int", "42", reflect.TypeFor[*int](), int64(42)},
		{"pointer to string", "hello", reflect.TypeFor[*string](), "hello"},

		// Invalid values (should return string)
		{"invalid int", "not-a-number", reflect.TypeFor[int](), "not-a-number"},
		{"invalid float", "not-a-float", reflect.TypeFor[float64](), "not-a-float"},
		{"invalid bool", "maybe", reflect.TypeFor[bool](), "maybe"},

		// Empty input
		{"empty input", "", reflect.TypeFor[int](), nil},
		{"empty input string", "", reflect.TypeFor[string](), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := parseValue(tt.input, tt.typ)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseEnumValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"simple enum", "red,green,blue", []string{"red", "green", "blue"}},
		{"enum with spaces", "red, green, blue", []string{"red", "green", "blue"}},
		{"enum with extra spaces", "red , green , blue", []string{"red", "green", "blue"}},
		{"single value", "red", []string{"red"}},
		{"empty string", "", []string{}},
		{"empty values filtered", "red,,blue", []string{"red", "blue"}},
		{"whitespace only filtered", "red,  ,blue", []string{"red", "blue"}},
		{"enum with special chars", "field-name,field_name,field.name", []string{"field-name", "field_name", "field.name"}},
		{"enum with numbers", "1,2,3", []string{"1", "2", "3"}},
		{"enum with mixed", "pending,active,completed", []string{"pending", "active", "completed"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := parseEnumValues(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInferFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		structFn func() reflect.StructField
		expected string
	}{
		{
			name: "explicit format tag",
			structFn: func() reflect.StructField {
				type Request struct {
					Field string `query:"field" format:"custom-format"`
				}
				typ := reflect.TypeFor[Request]()
				field, _ := typ.FieldByName("Field")

				return field
			},
			expected: "custom-format",
		},
		{
			name: "email from validate tag",
			structFn: func() reflect.StructField {
				type Request struct {
					Email string `query:"email" validate:"email"`
				}
				typ := reflect.TypeFor[Request]()
				field, _ := typ.FieldByName("Email")

				return field
			},
			expected: "email",
		},
		{
			name: "uri from validate tag",
			structFn: func() reflect.StructField {
				type Request struct {
					URL string `query:"url" validate:"url"`
				}
				typ := reflect.TypeFor[Request]()
				field, _ := typ.FieldByName("URL")

				return field
			},
			expected: "uri",
		},
		{
			name: "uuid from validate tag",
			structFn: func() reflect.StructField {
				type Request struct {
					ID string `query:"id" validate:"uuid"`
				}
				typ := reflect.TypeFor[Request]()
				field, _ := typ.FieldByName("ID")

				return field
			},
			expected: "uuid",
		},
		{
			name: "ipv4 from validate tag",
			structFn: func() reflect.StructField {
				type Request struct {
					IP string `query:"ip" validate:"ipv4"`
				}
				typ := reflect.TypeFor[Request]()
				field, _ := typ.FieldByName("IP")

				return field
			},
			expected: "ipv4",
		},
		{
			name: "ipv6 from validate tag",
			structFn: func() reflect.StructField {
				type Request struct {
					IP string `query:"ip" validate:"ipv6"`
				}
				typ := reflect.TypeFor[Request]()
				field, _ := typ.FieldByName("IP")

				return field
			},
			expected: "ipv6",
		},
		{
			name: "date-time from time.Time",
			structFn: func() reflect.StructField {
				type Request struct {
					Created time.Time `query:"created"`
				}
				typ := reflect.TypeFor[Request]()
				field, _ := typ.FieldByName("Created")

				return field
			},
			expected: "date-time",
		},
		{
			name: "uri from url.URL",
			structFn: func() reflect.StructField {
				type Request struct {
					URL url.URL `query:"url"`
				}
				typ := reflect.TypeFor[Request]()
				field, _ := typ.FieldByName("URL")

				return field
			},
			expected: "uri",
		},
		{
			name: "ip from net.IP",
			structFn: func() reflect.StructField {
				type Request struct {
					IP net.IP `query:"ip"`
				}
				typ := reflect.TypeFor[Request]()
				field, _ := typ.FieldByName("IP")

				return field
			},
			expected: "ip",
		},
		{
			name: "pointer to time.Time",
			structFn: func() reflect.StructField {
				type Request struct {
					Created *time.Time `query:"created"`
				}
				typ := reflect.TypeFor[Request]()
				field, _ := typ.FieldByName("Created")

				return field
			},
			expected: "date-time",
		},
		{
			name: "no format when no tags",
			structFn: func() reflect.StructField {
				type Request struct {
					Field string `query:"field"`
				}
				typ := reflect.TypeFor[Request]()
				field, _ := typ.FieldByName("Field")

				return field
			},
			expected: "",
		},
		{
			name: "format tag takes precedence",
			structFn: func() reflect.StructField {
				type Request struct {
					Field string `query:"field" format:"custom" validate:"email"`
				}
				typ := reflect.TypeFor[Request]()
				field, _ := typ.FieldByName("Field")

				return field
			},
			expected: "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			field := tt.structFn()
			result := inferFormat(field)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsParamRequired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		structFn func() (reflect.StructField, string)
		expected bool
	}{
		{
			name: "path params always required (non-pointer)",
			structFn: func() (reflect.StructField, string) {
				type Request struct {
					ID1 string `path:"id1"`
				}
				typ := reflect.TypeFor[Request]()
				field, _ := typ.FieldByName("ID1")

				return field, tagPath
			},
			expected: true,
		},
		{
			name: "path params always required (pointer)",
			structFn: func() (reflect.StructField, string) {
				type Request struct {
					ID2 *int `path:"id2"`
				}
				typ := reflect.TypeFor[Request]()
				field, _ := typ.FieldByName("ID2")

				return field, tagPath
			},
			expected: true,
		},
		{
			name: "pointer types are optional",
			structFn: func() (reflect.StructField, string) {
				type Request struct {
					ID *int `query:"id"`
				}
				typ := reflect.TypeFor[Request]()
				field, _ := typ.FieldByName("ID")

				return field, tagQuery
			},
			expected: false,
		},
		{
			name: "explicit required validation",
			structFn: func() (reflect.StructField, string) {
				type Request struct {
					Required string `query:"req" validate:"required"`
				}
				typ := reflect.TypeFor[Request]()
				field, _ := typ.FieldByName("Required")

				return field, tagQuery
			},
			expected: true,
		},
		{
			name: "no explicit required is optional",
			structFn: func() (reflect.StructField, string) {
				type Request struct {
					Optional string `query:"opt"`
				}
				typ := reflect.TypeFor[Request]()
				field, _ := typ.FieldByName("Optional")

				return field, tagQuery
			},
			expected: false,
		},
		{
			name: "required in multiple validations",
			structFn: func() (reflect.StructField, string) {
				type Request struct {
					Field string `query:"field" validate:"email,required,min=5"`
				}
				typ := reflect.TypeFor[Request]()
				field, _ := typ.FieldByName("Field")

				return field, tagQuery
			},
			expected: true,
		},
		{
			name: "non-pointer without required is optional",
			structFn: func() (reflect.StructField, string) {
				type Request struct {
					Field string `query:"field"`
				}
				typ := reflect.TypeFor[Request]()
				field, _ := typ.FieldByName("Field")

				return field, tagQuery
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			field, tagName := tt.structFn()
			result := isParamRequired(field, tagName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIntrospectRequest_TagVariations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		structFn func() reflect.Type
		validate func(t *testing.T, meta *RequestMetadata)
	}{
		{
			name: "query tag with empty value is ignored",
			structFn: func() reflect.Type {
				type Request struct {
					Page int `query:""`
				}

				return reflect.TypeFor[Request]()
			},
			validate: func(t *testing.T, meta *RequestMetadata) {
				assert.Empty(t, meta.Parameters)
			},
		},
		{
			name: "query tag with dash ignores field",
			structFn: func() reflect.Type {
				type Request struct {
					Ignored string `query:"-"`
					Valid   string `query:"valid"`
				}

				return reflect.TypeFor[Request]()
			},
			validate: func(t *testing.T, meta *RequestMetadata) {
				require.Len(t, meta.Parameters, 1)
				assert.Equal(t, "valid", meta.Parameters[0].Name)
			},
		},
		{
			name: "json tag with dash does not create body",
			structFn: func() reflect.Type {
				type Request struct {
					Ignored string `json:"-"`
					Valid   string `json:"valid"`
				}

				return reflect.TypeFor[Request]()
			},
			validate: func(t *testing.T, meta *RequestMetadata) {
				assert.True(t, meta.HasBody) // Has valid json tag
			},
		},
		{
			name: "json tag with empty value does not create body",
			structFn: func() reflect.Type {
				type Request struct {
					Field string `json:""` //nolint:tagliatelle // testing empty tag
				}

				return reflect.TypeFor[Request]()
			},
			validate: func(t *testing.T, meta *RequestMetadata) {
				assert.False(t, meta.HasBody)
			},
		},
		{
			name: "multiple tag types on same struct",
			structFn: func() reflect.Type {
				type Request struct {
					ID      int    `path:"id"`
					Page    int    `query:"page"`
					APIKey  string `header:"X-API-Key"` //nolint:tagliatelle // Standard HTTP header format
					Session string `cookie:"session"`
					Name    string `json:"name"`
				}

				return reflect.TypeFor[Request]()
			},
			validate: func(t *testing.T, meta *RequestMetadata) {
				assert.True(t, meta.HasBody)
				assert.Len(t, meta.Parameters, 4)

				paramMap := make(map[string]ParamSpec)
				for _, p := range meta.Parameters {
					paramMap[p.Name] = p
				}

				assert.Equal(t, "path", paramMap["id"].In)
				assert.Equal(t, "query", paramMap["page"].In)
				assert.Equal(t, "header", paramMap["X-API-Key"].In)
				assert.Equal(t, "cookie", paramMap["session"].In)
			},
		},
		{
			name: "tag with comma-separated options",
			structFn: func() reflect.Type {
				type Request struct {
					Field string `query:"field,omitempty"`
				}

				return reflect.TypeFor[Request]()
			},
			validate: func(t *testing.T, meta *RequestMetadata) {
				require.Len(t, meta.Parameters, 1)
				assert.Equal(t, "field", meta.Parameters[0].Name)
			},
		},

		{
			name: "all tag types with all metadata",
			structFn: func() reflect.Type {
				type Request struct {
					ID      int    `path:"id" doc:"User ID" example:"123"`
					Page    int    `query:"page" doc:"Page number" default:"1" example:"1"`
					APIKey  string `header:"X-API-Key" validate:"required" doc:"API key"` //nolint:tagliatelle // Standard HTTP header format
					Session string `cookie:"session" doc:"Session token" example:"abc123"`
					Status  string `query:"status" enum:"pending,active" validate:"oneof=pending active completed" doc:"Status"`
				}

				return reflect.TypeFor[Request]()
			},
			validate: func(t *testing.T, meta *RequestMetadata) {
				assert.Len(t, meta.Parameters, 5)

				paramMap := make(map[string]ParamSpec)
				for _, p := range meta.Parameters {
					paramMap[p.Name] = p
				}

				// Check path param
				id := paramMap["id"]
				assert.Equal(t, "path", id.In)
				assert.True(t, id.Required)
				assert.Equal(t, "User ID", id.Description)
				assert.Equal(t, int64(123), id.Example)

				// Check query param
				page := paramMap["page"]
				assert.Equal(t, "query", page.In)
				assert.Equal(t, "Page number", page.Description)
				assert.Equal(t, int64(1), page.Default)
				assert.Equal(t, int64(1), page.Example)

				// Check header param
				apiKey := paramMap["X-API-Key"]
				assert.Equal(t, "header", apiKey.In)
				assert.True(t, apiKey.Required)
				assert.Equal(t, "API key", apiKey.Description)

				// Check cookie param
				session := paramMap["session"]
				assert.Equal(t, "cookie", session.In)
				assert.Equal(t, "Session token", session.Description)
				assert.Equal(t, "abc123", session.Example)

				// Check enum param
				status := paramMap["status"]
				assert.Contains(t, status.Enum, "pending")
				assert.Contains(t, status.Enum, "active")
				assert.Contains(t, status.Enum, "completed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			typ := tt.structFn()
			meta := IntrospectRequest(typ)
			require.NotNil(t, meta)
			if tt.validate != nil {
				tt.validate(t, meta)
			}
		})
	}
}

func TestIntrospectRequest_JSONBodyDetection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		structFn func() reflect.Type
		expected bool
		validate func(t *testing.T, meta *RequestMetadata)
	}{
		{
			name: "detects body from json tag",
			structFn: func() reflect.Type {
				type Request struct {
					Name string `json:"name"`
				}

				return reflect.TypeFor[Request]()
			},
			expected: true,
			validate: func(t *testing.T, meta *RequestMetadata) {
				// BodyType should be set to the struct type
				require.NotNil(t, meta.BodyType)
				assert.Equal(t, reflect.Struct, meta.BodyType.Kind())
			},
		},
		{
			name: "detects body from json tag with omitempty",
			structFn: func() reflect.Type {
				type Request struct {
					Name string `json:"name,omitempty"`
				}

				return reflect.TypeFor[Request]()
			},
			expected: true,
			validate: nil,
		},
		{
			name: "no body when only params",
			structFn: func() reflect.Type {
				type Request struct {
					ID int `path:"id"`
				}

				return reflect.TypeFor[Request]()
			},
			expected: false,
			validate: nil,
		},
		{
			name: "no body when json tag is dash",
			structFn: func() reflect.Type {
				type Request struct {
					Field string `json:"-"`
				}

				return reflect.TypeFor[Request]()
			},
			expected: false,
			validate: nil,
		},
		{
			name: "body detected from embedded struct",
			structFn: func() reflect.Type {
				type Body struct {
					Name string `json:"name"`
				}
				type Request struct {
					Body
					ID int `path:"id"`
				}

				return reflect.TypeFor[Request]()
			},
			expected: true,
			validate: nil,
		},
		{
			name: "body detected from pointer json field",
			structFn: func() reflect.Type {
				type Request struct {
					Name *string `json:"name"`
				}

				return reflect.TypeFor[Request]()
			},
			expected: true,
			validate: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			typ := tt.structFn()
			meta := IntrospectRequest(typ)
			require.NotNil(t, meta)
			assert.Equal(t, tt.expected, meta.HasBody)
			if tt.validate != nil {
				tt.validate(t, meta)
			}
		})
	}
}
