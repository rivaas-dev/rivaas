package schema

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type GetUserRequest struct {
	ID     int    `path:"id" doc:"User ID" example:"123"`
	Expand string `query:"expand" doc:"Fields to expand" enum:"profile,settings"`
	APIKey string `header:"X-Api-Key" validate:"required"` //nolint:tagliatelle // Standard API key header
	Token  string `cookie:"token" doc:"Auth token"`
}

type CreateUserRequest struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"email"`
	Age   int    `query:"age" doc:"User age" validate:"min=0,max=150" example:"25"`
}

type MixedRequest struct {
	ID    int    `path:"id"`
	Query string `query:"q"`
	Body  string `json:"body"`
}

func TestIntrospectRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		validate func(t *testing.T, meta *RequestMetadata)
	}{
		{
			name:  "path and query parameters",
			input: GetUserRequest{},
			validate: func(t *testing.T, meta *RequestMetadata) {
				require.NotNil(t, meta)
				assert.Len(t, meta.Parameters, 4)
				assert.False(t, meta.HasBody)

				// Check path parameter
				var idParam *ParamSpec
				for i := range meta.Parameters {
					if meta.Parameters[i].Name == "id" {
						idParam = &meta.Parameters[i]
						break
					}
				}
				require.NotNil(t, idParam)
				assert.Equal(t, "path", idParam.In)
				assert.True(t, idParam.Required)
				assert.Equal(t, "User ID", idParam.Description)
				assert.Equal(t, int64(123), idParam.Example)

				// Check query parameter with enum
				var expandParam *ParamSpec
				for i := range meta.Parameters {
					if meta.Parameters[i].Name == "expand" {
						expandParam = &meta.Parameters[i]
						break
					}
				}
				require.NotNil(t, expandParam)
				assert.Equal(t, "query", expandParam.In)
				assert.Equal(t, []string{"profile", "settings"}, expandParam.Enum)
				assert.Equal(t, "Fields to expand", expandParam.Description)

				// Check header parameter
				var apiKeyParam *ParamSpec
				for i := range meta.Parameters {
					if meta.Parameters[i].Name == "X-Api-Key" {
						apiKeyParam = &meta.Parameters[i]
						break
					}
				}
				require.NotNil(t, apiKeyParam)
				assert.Equal(t, "header", apiKeyParam.In)
				assert.True(t, apiKeyParam.Required)

				// Check cookie parameter
				var tokenParam *ParamSpec
				for i := range meta.Parameters {
					if meta.Parameters[i].Name == "token" {
						tokenParam = &meta.Parameters[i]
						break
					}
				}
				require.NotNil(t, tokenParam)
				assert.Equal(t, "cookie", tokenParam.In)
				assert.Equal(t, "Auth token", tokenParam.Description)
			},
		},
		{
			name:  "mixed json and query",
			input: CreateUserRequest{},
			validate: func(t *testing.T, meta *RequestMetadata) {
				require.NotNil(t, meta)
				assert.True(t, meta.HasBody)
				assert.Len(t, meta.Parameters, 1) // Only age query param

				ageParam := meta.Parameters[0]
				assert.Equal(t, "age", ageParam.Name)
				assert.Equal(t, "query", ageParam.In)
				assert.Equal(t, "User age", ageParam.Description)
				assert.Equal(t, int64(25), ageParam.Example)
				require.NotNil(t, ageParam.Type)
			},
		},
		{
			name:  "all parameter types",
			input: MixedRequest{},
			validate: func(t *testing.T, meta *RequestMetadata) {
				require.NotNil(t, meta)
				assert.True(t, meta.HasBody)
				assert.Len(t, meta.Parameters, 2) // id (path) and q (query)

				// Verify path param is required
				var idParam *ParamSpec
				for i := range meta.Parameters {
					if meta.Parameters[i].Name == "id" {
						idParam = &meta.Parameters[i]
						break
					}
				}
				require.NotNil(t, idParam)
				assert.True(t, idParam.Required)
			},
		},
		{
			name:  "non-struct type",
			input: "string",
			validate: func(t *testing.T, meta *RequestMetadata) {
				assert.Nil(t, meta)
			},
		},
		{
			name:  "pointer to struct",
			input: (*GetUserRequest)(nil),
			validate: func(t *testing.T, meta *RequestMetadata) {
				require.NotNil(t, meta)
				// Should work the same as non-pointer
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var meta *RequestMetadata
			if tt.input == nil {
				meta = IntrospectRequest(nil)
			} else {
				meta = IntrospectRequest(reflect.TypeOf(tt.input))
			}
			tt.validate(t, meta)
		})
	}
}

func TestIntrospectRequest_RequiredFields(t *testing.T) {
	t.Parallel()

	type RequiredTest struct {
		Required  string `path:"id"`   // path params always required
		Optional  *int   `query:"opt"` // pointer is optional
		Required2 string `query:"req" validate:"required"`
		Optional2 string `query:"opt2"` // no validate required
	}

	meta := IntrospectRequest(reflect.TypeOf(RequiredTest{}))
	require.NotNil(t, meta)

	tests := []struct {
		name     string
		param    string
		required bool
	}{
		{"path param always required", "id", true},
		{"pointer is optional", "opt", false},
		{"explicit required", "req", true},
		{"no explicit required", "opt2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var param *ParamSpec
			for i := range meta.Parameters {
				if meta.Parameters[i].Name == tt.param {
					param = &meta.Parameters[i]
					break
				}
			}
			require.NotNil(t, param, "param %s not found", tt.param)
			assert.Equal(t, tt.required, param.Required)
		})
	}
}

func TestIntrospectRequest_EmbeddedStructs(t *testing.T) {
	t.Parallel()

	type BaseParams struct {
		ID int `path:"id"`
	}

	type ExtendedRequest struct {
		BaseParams
		Query string `query:"q"`
		Body  string `json:"body"`
	}

	meta := IntrospectRequest(reflect.TypeOf(ExtendedRequest{}))
	require.NotNil(t, meta)

	// Should include fields from embedded struct
	paramNames := make(map[string]bool)
	for _, p := range meta.Parameters {
		paramNames[p.Name] = true
	}

	assert.True(t, paramNames["id"], "embedded field 'id' should be included")
	assert.True(t, paramNames["q"], "field 'q' should be included")
	assert.True(t, meta.HasBody, "should detect body from json tag")
}

func TestIntrospectRequest_DefaultValues(t *testing.T) {
	t.Parallel()

	type RequestWithDefaults struct {
		ID    int    `path:"id" default:"1"`
		Limit int    `query:"limit" default:"10"`
		Sort  string `query:"sort" default:"asc"`
	}

	meta := IntrospectRequest(reflect.TypeOf(RequestWithDefaults{}))
	require.NotNil(t, meta)

	for _, p := range meta.Parameters {
		switch p.Name {
		case "id":
			assert.Equal(t, int64(1), p.Default)
		case "limit":
			assert.Equal(t, int64(10), p.Default)
		case "sort":
			assert.Equal(t, "asc", p.Default)
		}
	}
}

func TestIntrospectRequest_EnumValues(t *testing.T) {
	t.Parallel()

	type RequestWithEnum struct {
		Status string `query:"status" enum:"pending,active,completed"`
		Type   string `query:"type" enum:"public,private"`
	}

	meta := IntrospectRequest(reflect.TypeOf(RequestWithEnum{}))
	require.NotNil(t, meta)

	for _, p := range meta.Parameters {
		switch p.Name {
		case "status":
			assert.Equal(t, []string{"pending", "active", "completed"}, p.Enum)
		case "type":
			assert.Equal(t, []string{"public", "private"}, p.Enum)
		}
	}
}

func TestIntrospectRequest_EnumFromValidate(t *testing.T) {
	t.Parallel()

	type RequestWithValidateEnum struct {
		Color string `query:"color" validate:"oneof=red green blue"`
	}

	meta := IntrospectRequest(reflect.TypeOf(RequestWithValidateEnum{}))
	require.NotNil(t, meta)

	var colorParam *ParamSpec
	for i := range meta.Parameters {
		if meta.Parameters[i].Name == "color" {
			colorParam = &meta.Parameters[i]
			break
		}
	}
	require.NotNil(t, colorParam)
	assert.Contains(t, colorParam.Enum, "red")
	assert.Contains(t, colorParam.Enum, "green")
	assert.Contains(t, colorParam.Enum, "blue")
}

func TestIntrospectRequest_EmptyStruct(t *testing.T) {
	t.Parallel()

	meta := IntrospectRequest(reflect.TypeOf(struct{}{}))
	require.NotNil(t, meta)
	assert.False(t, meta.HasBody)
	assert.Empty(t, meta.Parameters)
}

func TestIntrospectRequest_OnlyBody(t *testing.T) {
	t.Parallel()

	type BodyOnly struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	meta := IntrospectRequest(reflect.TypeOf(BodyOnly{}))
	require.NotNil(t, meta)
	assert.True(t, meta.HasBody)
	assert.Empty(t, meta.Parameters)
}

func TestIntrospectRequest_OnlyParams(t *testing.T) {
	t.Parallel()

	type ParamsOnly struct {
		ID   int    `path:"id"`
		Page int    `query:"page"`
		Key  string `header:"X-Key"`
	}

	meta := IntrospectRequest(reflect.TypeOf(ParamsOnly{}))
	require.NotNil(t, meta)
	assert.False(t, meta.HasBody)
	assert.Len(t, meta.Parameters, 3)
}

func TestIntrospectRequest_InvalidTypes(t *testing.T) {
	t.Parallel()

	// Non-struct types should return nil
	assert.Nil(t, IntrospectRequest(reflect.TypeOf("string")))
	assert.Nil(t, IntrospectRequest(reflect.TypeOf(123)))
	assert.Nil(t, IntrospectRequest(reflect.TypeOf([]string{})))
	assert.Nil(t, IntrospectRequest(nil))
}

func TestIntrospectRequest_ComplexNested(t *testing.T) {
	t.Parallel()

	type Nested struct {
		Value string `json:"value"`
	}

	type ComplexRequest struct {
		ID     int     `path:"id"`
		Query  string  `query:"q"`
		Header string  `header:"X-Header"`
		Nested *Nested `json:"nested"`
	}

	meta := IntrospectRequest(reflect.TypeOf(ComplexRequest{}))
	require.NotNil(t, meta)
	assert.True(t, meta.HasBody)
	assert.Len(t, meta.Parameters, 3)
}
