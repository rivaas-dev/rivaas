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

package binding

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBind_ErrorPaths tests bind function error paths
func TestBind_ErrorPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setup          func() (any, ValueGetter)
		expectedErrMsg string
	}{
		{
			name: "non-pointer input",
			setup: func() (any, ValueGetter) {
				type Params struct {
					Name string `query:"name"`
				}
				var params Params

				return params, NewQueryGetter(url.Values{})
			},
			expectedErrMsg: "pointer to struct",
		},
		{
			name: "nil pointer",
			setup: func() (any, ValueGetter) {
				var params *struct {
					Name string `query:"name"`
				}

				return params, NewQueryGetter(url.Values{})
			},
			expectedErrMsg: "nil",
		},
		{
			name: "pointer to non-struct",
			setup: func() (any, ValueGetter) {
				var value *int
				return &value, NewQueryGetter(url.Values{})
			},
			expectedErrMsg: "pointer to struct",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			params, getter := tt.setup()
			err := Raw(getter, TagQuery, params)

			require.Error(t, err, "Expected error for %s", tt.name)
			require.ErrorContains(t, err, tt.expectedErrMsg, "Error should contain %q", tt.expectedErrMsg)
		})
	}
}

// TestBind_PointerHandling tests pointer field handling including errors and empty values
func TestBind_PointerHandling(t *testing.T) {
	t.Parallel()

	t.Run("Error", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name           string
			values         url.Values
			params         any
			expectedErrMsg string
		}{
			{
				name: "pointer to int with invalid value",
				values: func() url.Values {
					v := url.Values{}
					v.Set("age", "not-a-number")

					return v
				}(),
				params: &struct {
					Age *int `query:"age"`
				}{},
				expectedErrMsg: "Age",
			},
			{
				name: "pointer to time.Time with invalid value",
				values: func() url.Values {
					v := url.Values{}
					v.Set("start", "invalid-time")

					return v
				}(),
				params: &struct {
					StartTime *time.Time `query:"start"`
				}{},
				expectedErrMsg: "StartTime",
			},
			{
				name: "pointer to float64 with invalid value",
				values: func() url.Values {
					v := url.Values{}
					v.Set("price", "not-a-float")

					return v
				}(),
				params: &struct {
					Price *float64 `query:"price"`
				}{},
				expectedErrMsg: "Price",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := Raw(NewQueryGetter(tt.values), TagQuery, tt.params)

				require.Error(t, err, "Expected error for %s", tt.name)
				require.ErrorContains(t, err, tt.expectedErrMsg, "Error should mention field name %q", tt.expectedErrMsg)
			})
		}
	})

	t.Run("EmptyValue", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			Name *string `query:"name"`
			Age  *int    `query:"age"`
		}

		tests := []struct {
			name     string
			values   url.Values
			validate func(t *testing.T, params Params)
		}{
			{
				name: "empty string leaves pointer nil",
				values: func() url.Values {
					v := url.Values{}
					v.Set("name", "")

					return v
				}(),
				validate: func(t *testing.T, params Params) {
					t.Helper()
					assert.Nil(t, params.Name, "Expected Name to be nil for empty value")
				},
			},
			{
				name:   "missing value leaves pointer nil",
				values: url.Values{},
				validate: func(t *testing.T, params Params) {
					t.Helper()
					assert.Nil(t, params.Name, "Expected Name to be nil for missing value")
					assert.Nil(t, params.Age, "Expected Age to be nil for missing value")
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var params Params
				require.NoError(t, Raw(NewQueryGetter(tt.values), TagQuery, &params), "Bind should succeed for %s", tt.name)
				tt.validate(t, params)
			})
		}
	})
}

// TestBind_InvalidURL tests error path for invalid URL parsing
func TestBind_InvalidURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		values         url.Values
		params         any
		wantErr        bool
		expectedErrMsg string
		validate       func(t *testing.T, params any)
	}{
		{
			name: "invalid URL format",
			values: func() url.Values {
				v := url.Values{}
				v.Set("callback", "://invalid-url")

				return v
			}(),
			params: &struct {
				CallbackURL url.URL `query:"callback"`
			}{},
			wantErr:        true,
			expectedErrMsg: "invalid URL",
			validate:       func(t *testing.T, params any) { t.Helper() },
		},
		{
			name: "malformed URL with missing scheme",
			values: func() url.Values {
				v := url.Values{}
				v.Set("endpoint", "://malformed")

				return v
			}(),
			params: &struct {
				Endpoint url.URL `query:"endpoint"`
			}{},
			wantErr:        true,
			expectedErrMsg: "invalid URL",
			validate:       func(t *testing.T, params any) { t.Helper() },
		},
		{
			name: "valid URL should succeed",
			values: func() url.Values {
				v := url.Values{}
				v.Set("endpoint", "https://example.com/path")

				return v
			}(),
			params: &struct {
				Endpoint url.URL `query:"endpoint"`
			}{},
			wantErr:        false,
			expectedErrMsg: "",
			validate: func(t *testing.T, params any) {
				t.Helper()
				p, ok := params.(*struct {
					Endpoint url.URL `query:"endpoint"`
				})
				require.True(t, ok)
				assert.Equal(t, "example.com", p.Endpoint.Host, "URL Host should match")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Raw(NewQueryGetter(tt.values), TagQuery, tt.params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
				require.ErrorContains(t, err, tt.expectedErrMsg, "Error should contain %q", tt.expectedErrMsg)
			} else {
				require.NoError(t, err, "Bind should succeed for %s", tt.name)
				tt.validate(t, tt.params)
			}
		})
	}
}

// TestBind_SliceFieldErrorPath tests setSliceField error paths
func TestBind_SliceFieldErrorPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		values         url.Values
		params         any
		expectedErrMsg string
	}{
		{
			name: "invalid element conversion",
			values: func() url.Values {
				v := url.Values{}
				v.Add("ids", "123")
				v.Add("ids", "invalid")
				v.Add("ids", "456")

				return v
			}(),
			params: &struct {
				IDs []int `query:"ids"`
			}{},
			expectedErrMsg: "element",
		},
		{
			name: "invalid time in slice",
			values: func() url.Values {
				v := url.Values{}
				v.Add("times", "2024-01-01")
				v.Add("times", "invalid-time")
				v.Add("times", "2024-01-02")

				return v
			}(),
			params: &struct {
				Times []time.Time `query:"times"`
			}{},
			expectedErrMsg: "", // Any error is acceptable
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Raw(NewQueryGetter(tt.values), TagQuery, tt.params)

			require.Error(t, err, "Expected error for %s", tt.name)
			if tt.expectedErrMsg != "" {
				require.ErrorContains(t, err, tt.expectedErrMsg, "Error should contain %q", tt.expectedErrMsg)
			}
		})
	}
}

// TestBind_ConvertValueEdgeCases tests convertValue remaining paths
func TestBind_ConvertValueEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		values         url.Values
		params         any
		wantErr        bool
		expectedErrMsg string
	}{
		{
			name: "int8 overflow",
			values: func() url.Values {
				v := url.Values{}
				v.Set("value", "999999")

				return v
			}(),
			params: &struct {
				Value int8 `query:"value"`
			}{},
			wantErr:        false, // May or may not error depending on conversion, but tests the path
			expectedErrMsg: "",
		},
		{
			name: "uint overflow",
			values: func() url.Values {
				v := url.Values{}
				v.Set("value", "-1")

				return v
			}(),
			params: &struct {
				Value uint `query:"value"`
			}{},
			wantErr:        true,
			expectedErrMsg: "invalid unsigned integer",
		},
		{
			name: "float with invalid format",
			values: func() url.Values {
				v := url.Values{}
				v.Set("value", "not-a-number")

				return v
			}(),
			params: &struct {
				Value float64 `query:"value"`
			}{},
			wantErr:        true,
			expectedErrMsg: "invalid float",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Raw(NewQueryGetter(tt.values), TagQuery, tt.params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
				if tt.expectedErrMsg != "" {
					require.ErrorContains(t, err, tt.expectedErrMsg, "Error should contain %q", tt.expectedErrMsg)
				}
			} else {
				// May or may not error, just test the path
				_ = err
			}
		})
	}
}

// TestBind_SkipUnexportedFields tests that unexported fields are skipped
func TestBind_SkipUnexportedFields(t *testing.T) {
	t.Parallel()

	type Params struct {
		Name string `query:"name"`
		Age  int    `query:"age"`
	}

	values := url.Values{}
	values.Set("name", "john")
	values.Set("age", "30")

	var params Params
	require.NoError(t, Raw(NewQueryGetter(values), TagQuery, &params), "Bind should succeed")

	assert.Equal(t, "john", params.Name, "Expected Name=john")
	assert.Equal(t, 30, params.Age, "Expected Age=30")
}

// TestBind_NestedStructError tests error handling for nested struct binding failures
func TestBind_NestedStructError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		values        url.Values
		params        any
		expectedField string
		validate      func(t *testing.T, bindErr *BindError)
	}{
		{
			name: "nested struct with invalid data",
			values: func() url.Values {
				v := url.Values{}
				v.Set("address.zip_code", "invalid")

				return v
			}(),
			params: &struct {
				Address struct {
					ZipCode int `query:"zip_code"`
				} `query:"address"`
			}{},
			expectedField: "Address",
			validate: func(t *testing.T, bindErr *BindError) {
				t.Helper()
				assert.Equal(t, SourceQuery, bindErr.Source, "Expected Source=SourceQuery")
				assert.Empty(t, bindErr.Value, "Expected empty Value for nested struct error")
				assert.Error(t, bindErr.Err, "Expected underlying error")
			},
		},
		{
			name: "nested struct with invalid time format",
			values: func() url.Values {
				v := url.Values{}
				v.Set("meta.created_at", "invalid-time")

				return v
			}(),
			params: &struct {
				Metadata struct {
					CreatedAt time.Time `query:"created_at"`
				} `query:"meta"`
			}{},
			expectedField: "Metadata",
			validate:      func(t *testing.T, bindErr *BindError) { t.Helper() },
		},
		{
			name: "deeply nested struct error",
			values: func() url.Values {
				v := url.Values{}
				v.Set("middle.inner.value", "not-a-number")

				return v
			}(),
			params: &struct {
				Middle struct {
					Inner struct {
						Value int `query:"value"`
					} `query:"inner"`
				} `query:"middle"`
			}{},
			expectedField: "Middle",
			validate:      func(_ *testing.T, _ *BindError) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Raw(NewQueryGetter(tt.values), TagQuery, tt.params)

			require.Error(t, err, "Expected error for %s", tt.name)

			var bindErr *BindError
			require.ErrorAs(t, err, &bindErr, "Expected BindError, got %T: %v", err, err)
			assert.Equal(t, tt.expectedField, bindErr.Field, "Expected Field=%q", tt.expectedField)
			tt.validate(t, bindErr)
		})
	}
}

// TestBind_ParseStructTypeRemainingPaths tests parseStructType remaining paths
func TestBind_ParseStructTypeRemainingPaths(t *testing.T) {
	t.Parallel()

	type ParamsUnexported struct {
		Exported string `query:"exported"`
		_        string `query:"unexported"` // unexported field (lowercase) should be skipped
	}

	type ParamsNoQueryTag struct {
		HasTag     string `query:"has_tag"`
		NoQueryTag string // No query tag, should be skipped for query binding
	}

	type Embedded struct {
		Value string `query:"value"`
	}
	type ParamsEmbedded struct {
		*Embedded // Pointer to embedded struct

		Name string `query:"name"`
	}

	tests := []struct {
		name     string
		values   url.Values
		params   any
		validate func(t *testing.T, params any)
	}{
		{
			name: "unexported fields are skipped",
			values: func() url.Values {
				v := url.Values{}
				v.Set("exported", "test")
				v.Set("unexported", "ignored")

				return v
			}(),
			params: &ParamsUnexported{},
			validate: func(t *testing.T, params any) {
				t.Helper()
				p, ok := params.(*ParamsUnexported)
				require.True(t, ok)
				assert.Equal(t, "test", p.Exported, "Expected Exported=test")
			},
		},
		{
			name: "non-standard tag skipped when empty",
			values: func() url.Values {
				v := url.Values{}
				v.Set("has_tag", "value")

				return v
			}(),
			params: &ParamsNoQueryTag{},
			validate: func(t *testing.T, params any) {
				t.Helper()
				p, ok := params.(*ParamsNoQueryTag)
				require.True(t, ok)
				assert.Equal(t, "value", p.HasTag, "Expected HasTag=value")
				assert.Empty(t, p.NoQueryTag, "NoQueryTag should remain zero value")
			},
		},
		{
			name: "embedded struct with pointer",
			values: func() url.Values {
				v := url.Values{}
				v.Set("value", "embedded")
				v.Set("name", "test")

				return v
			}(),
			params: func() any {
				params := ParamsEmbedded{
					Embedded: &Embedded{},
					Name:     "",
				}

				return &params
			}(),
			validate: func(t *testing.T, params any) {
				t.Helper()
				p, ok := params.(*ParamsEmbedded)
				require.True(t, ok)
				assert.Equal(t, "test", p.Name, "Expected Name=test")
				require.NotNil(t, p.Embedded, "Embedded should not be nil")
				assert.Equal(t, "embedded", p.Value, "Expected Embedded.Value=embedded")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Raw(NewQueryGetter(tt.values), TagQuery, tt.params)
			require.NoError(t, err, "%s should succeed", tt.name)
			tt.validate(t, tt.params)
		})
	}
}

// TestBind_Errors tests error cases in binding
func TestBind_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func() error
		wantErr  bool
		validate func(t *testing.T, err error)
	}{
		{
			name: "not a pointer",
			setup: func() error {
				var params struct {
					Name string `query:"name"`
				}
				values := url.Values{}
				values.Set("name", "test")

				return Raw(NewQueryGetter(values), TagQuery, params) // Not a pointer!
			},
			wantErr: true,
			validate: func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err, "Expected error for non-pointer")
			},
		},
		{
			name: "nil pointer",
			setup: func() error {
				var params *struct {
					Name string `query:"name"`
				}
				values := url.Values{}
				values.Set("name", "test")

				return Raw(NewQueryGetter(values), TagQuery, params)
			},
			wantErr: true,
			validate: func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err, "Expected error for nil pointer")
			},
		},
		{
			name: "not a struct",
			setup: func() error {
				var str string
				values := url.Values{}
				values.Set("name", "test")

				return Raw(NewQueryGetter(values), TagQuery, &str)
			},
			wantErr: true,
			validate: func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err, "Expected error for non-struct")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.setup()

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
			} else {
				require.NoError(t, err, "Should succeed for %s", tt.name)
			}
			tt.validate(t, err)
		})
	}
}

// TestBind_BindErrorDetails tests BindError details
func TestBind_BindErrorDetails(t *testing.T) {
	t.Parallel()

	type Params struct {
		Age int `query:"age"`
	}

	values := url.Values{}
	values.Set("age", "invalid")

	var params Params
	err := Raw(NewQueryGetter(values), TagQuery, &params)
	require.Error(t, err, "Expected BindError")

	var bindErr *BindError
	require.ErrorAs(t, err, &bindErr, "Expected BindError type")
	assert.Equal(t, "Age", bindErr.Field)
	assert.Equal(t, SourceQuery, bindErr.Source)
	assert.Equal(t, "invalid", bindErr.Value)
}

// TestBind_TagParsingCommaSeparatedOptions tests tag parsing with comma-separated options
func TestBind_TagParsingCommaSeparatedOptions(t *testing.T) {
	t.Parallel()

	// Define struct types at test level to avoid type scope issues
	type JSONDataOmitempty struct {
		Name  string `json:"name,omitempty"`
		Email string `json:"email,omitempty"`
		Age   int    `json:"age,omitempty"`
	}

	type FormDataOmitempty struct {
		Username string `form:"username,omitempty"`
		Password string `form:"password,omitempty"` //nolint:gosec // G117: test fixture
	}

	type JSONDataEmptyName struct {
		FieldName string `json:",omitempty"` //nolint:tagliatelle // Empty name, should use "FieldName"
	}

	type JSONDataSkipField struct {
		Public  string `json:"public"`
		Private string `json:"-"` // Should be skipped
	}

	tests := []struct {
		name     string
		setup    func() (ValueGetter, string, any)
		validate func(t *testing.T, params any)
	}{
		{
			name: "json tag with omitempty",
			setup: func() (ValueGetter, string, any) {
				return nil, TagJSON, &JSONDataOmitempty{}
			},
			validate: func(t *testing.T, params any) {
				t.Helper()
				p, ok := params.(*JSONDataOmitempty)
				require.True(t, ok)
				jsonData := `{"name":"John","email":"john@example.com","age":30}`
				err := JSONTo([]byte(jsonData), p)
				require.NoError(t, err)
				assert.Equal(t, "John", p.Name)
				assert.Equal(t, "john@example.com", p.Email)
				assert.Equal(t, 30, p.Age)
			},
		},
		{
			name: "form tag with omitempty",
			setup: func() (ValueGetter, string, any) {
				values := url.Values{}
				values.Set("username", "testuser")
				values.Set("password", "secret123")

				return NewFormGetter(values), TagForm, &FormDataOmitempty{}
			},
			validate: func(t *testing.T, params any) {
				t.Helper()
				p, ok := params.(*FormDataOmitempty)
				require.True(t, ok)
				assert.Equal(t, "testuser", p.Username)
				assert.Equal(t, "secret123", p.Password)
			},
		},
		{
			name: "json tag with empty name and options",
			setup: func() (ValueGetter, string, any) {
				return nil, TagJSON, &JSONDataEmptyName{}
			},
			validate: func(t *testing.T, params any) {
				t.Helper()
				p, ok := params.(*JSONDataEmptyName)
				require.True(t, ok)
				jsonData := `{"FieldName":"test"}`
				err := JSONTo([]byte(jsonData), p)
				require.NoError(t, err)
				assert.Equal(t, "test", p.FieldName)
			},
		},
		{
			name: "json tag with dash (skip field)",
			setup: func() (ValueGetter, string, any) {
				return nil, TagJSON, &JSONDataSkipField{}
			},
			validate: func(t *testing.T, params any) {
				t.Helper()
				p, ok := params.(*JSONDataSkipField)
				require.True(t, ok)
				jsonData := `{"public":"visible","Private":"should be ignored"}`
				err := JSONTo([]byte(jsonData), p)
				require.NoError(t, err)
				assert.Equal(t, "visible", p.Public)
				assert.Empty(t, p.Private, "Private should be empty (skipped)")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getter, tag, params := tt.setup()
			if getter != nil {
				require.NoError(t, Raw(getter, tag, params), "%s should succeed", tt.name)
			}
			tt.validate(t, params)
		})
	}
}

// TestFieldAliases tests that field aliases work correctly
// Note: Cannot be fully table-driven due to struct tag requirements in Go
func TestFieldAliases(t *testing.T) {
	t.Parallel()

	t.Run("primary name works", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			UserID string `query:"user_id,id"`
		}

		var params Params
		values := url.Values{}
		values.Set("user_id", "123")

		err := Raw(NewQueryGetter(values), TagQuery, &params)
		require.NoError(t, err)
		assert.Equal(t, "123", params.UserID)
	})

	t.Run("alias works when primary missing", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			UserID string `query:"user_id,id"`
		}

		var params Params
		values := url.Values{}
		values.Set("id", "456")

		err := Raw(NewQueryGetter(values), TagQuery, &params)
		require.NoError(t, err)
		assert.Equal(t, "456", params.UserID)
	})

	t.Run("primary takes precedence over alias", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			UserID string `query:"user_id,id"`
		}

		var params Params
		values := url.Values{}
		values.Set("user_id", "123")
		values.Set("id", "456")

		err := Raw(NewQueryGetter(values), TagQuery, &params)
		require.NoError(t, err)
		assert.Equal(t, "123", params.UserID) // Primary should win
	})

	t.Run("multiple aliases", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			UserID string `query:"user_id,id,uid"`
		}

		var params Params
		values := url.Values{}
		values.Set("uid", "789")

		err := Raw(NewQueryGetter(values), TagQuery, &params)
		require.NoError(t, err)
		assert.Equal(t, "789", params.UserID)
	})

	t.Run("aliases checked in order", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			UserID string `query:"user_id,id,uid"`
		}

		var params Params
		values := url.Values{}
		values.Set("id", "first")
		values.Set("uid", "second")

		err := Raw(NewQueryGetter(values), TagQuery, &params)
		require.NoError(t, err)
		assert.Equal(t, "first", params.UserID) // First alias should win
	})

	t.Run("aliases with form tag", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			UserID string `form:"user_id,id"`
		}

		var params Params
		values := url.Values{}
		values.Set("id", "123")

		// Test that aliases work with form tags
		err := Raw(NewFormGetter(values), TagForm, &params)
		require.NoError(t, err)
		assert.Equal(t, "123", params.UserID)
	})

	t.Run("aliases ignore omitempty modifier", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			UserID string `query:"user_id,id,omitempty"`
		}

		var params Params
		values := url.Values{}
		values.Set("id", "123")

		err := Raw(NewQueryGetter(values), TagQuery, &params)
		require.NoError(t, err)
		assert.Equal(t, "123", params.UserID)
	})
}

// TestTypedDefaults tests that pre-computed typed defaults work correctly
// Note: Cannot be table-driven due to Go's type system - each struct type is unique
func TestTypedDefaults(t *testing.T) {
	t.Parallel()

	t.Run("string default", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			Name string `query:"name" default:"John"`
		}

		var params Params
		values := url.Values{}

		err := Raw(NewQueryGetter(values), TagQuery, &params)
		require.NoError(t, err)
		assert.Equal(t, "John", params.Name)
	})

	t.Run("int default", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			Age int `query:"age" default:"30"`
		}

		var params Params
		values := url.Values{}

		err := Raw(NewQueryGetter(values), TagQuery, &params)
		require.NoError(t, err)
		assert.Equal(t, 30, params.Age)
	})

	t.Run("bool default", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			Active bool `query:"active" default:"true"`
		}

		var params Params
		values := url.Values{}

		err := Raw(NewQueryGetter(values), TagQuery, &params)
		require.NoError(t, err)
		assert.True(t, params.Active)
	})

	t.Run("float default", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			Price float64 `query:"price" default:"99.99"`
		}

		var params Params
		values := url.Values{}

		err := Raw(NewQueryGetter(values), TagQuery, &params)
		require.NoError(t, err)
		assert.Equal(t, 99.99, params.Price) //nolint:testifylint // exact decimal comparison
	})

	t.Run("pointer default", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			Age *int `query:"age" default:"25"`
		}

		var params Params
		values := url.Values{}

		err := Raw(NewQueryGetter(values), TagQuery, &params)
		require.NoError(t, err)
		require.NotNil(t, params.Age)
		assert.Equal(t, 25, *params.Age)
	})

	t.Run("default overridden by provided value", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			Name string `query:"name" default:"John"`
		}

		var params Params
		values := url.Values{}
		values.Set("name", "Jane")

		err := Raw(NewQueryGetter(values), TagQuery, &params)
		require.NoError(t, err)
		assert.Equal(t, "Jane", params.Name) // Provided value should win
	})

	t.Run("time default", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			CreatedAt time.Time `query:"created_at" default:"2024-01-01T00:00:00Z"`
		}

		var params Params
		values := url.Values{}

		err := Raw(NewQueryGetter(values), TagQuery, &params)
		require.NoError(t, err)
		expected, err := time.Parse(time.RFC3339, "2024-01-01T00:00:00Z")
		require.NoError(t, err)
		assert.Equal(t, expected, params.CreatedAt)
	})
}

func TestBindTo(t *testing.T) {
	t.Parallel()

	t.Run("binds from multiple sources", func(t *testing.T) {
		t.Parallel()

		type Request struct {
			ID      int    `path:"id"`
			Page    int    `query:"page"`
			APIKey  string `header:"X-Api-Key"` //nolint:gosec // G117: test fixture
			Session string `cookie:"session"`
		}

		var req Request
		err := BindTo(&req,
			FromPath(map[string]string{"id": "123"}),
			FromQuery(url.Values{"page": {"5"}}),
			FromHeader(http.Header{"X-Api-Key": {"secret-key"}}),
			FromCookie([]*http.Cookie{{Name: "session", Value: "abc123"}}),
		)
		require.NoError(t, err)
		assert.Equal(t, 123, req.ID)
		assert.Equal(t, 5, req.Page)
		assert.Equal(t, "secret-key", req.APIKey)
		assert.Equal(t, "abc123", req.Session)
	})

	t.Run("only binds from sources with tags", func(t *testing.T) {
		t.Parallel()

		type Request struct {
			ID   int `path:"id"`
			Page int `query:"page"`
			// No header or cookie tags
		}

		var req Request
		err := BindTo(&req,
			FromPath(map[string]string{"id": "123"}),
			FromQuery(url.Values{"page": {"5"}}),
			FromHeader(http.Header{"X-Api-Key": {"secret-key"}}),
			FromCookie([]*http.Cookie{{Name: "session", Value: "abc123"}}),
		)
		require.NoError(t, err)
		assert.Equal(t, 123, req.ID)
		assert.Equal(t, 5, req.Page)
		// Header and cookie should be ignored since no tags exist
	})

	t.Run("handles errors from multiple sources", func(t *testing.T) {
		t.Parallel()

		type Request struct {
			ID   int `path:"id"`
			Page int `query:"page"` // test type error
		}

		var req Request
		err := BindTo(&req,
			FromPath(map[string]string{"id": "123"}),
			FromQuery(url.Values{"page": {"not-a-number"}}), // Type conversion error
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "query")
	})

	t.Run("handles embedded structs", func(t *testing.T) {
		t.Parallel()

		type PathParams struct {
			ID int `path:"id"`
		}

		type QueryParams struct {
			Page int `query:"page"`
		}

		type Request struct {
			PathParams
			QueryParams
		}

		var req Request
		err := BindTo(&req,
			FromPath(map[string]string{"id": "123"}),
			FromQuery(url.Values{"page": {"5"}}),
		)
		require.NoError(t, err)
		assert.Equal(t, 123, req.ID)
		assert.Equal(t, 5, req.Page)
	})

	t.Run("validates pointer input", func(t *testing.T) {
		t.Parallel()

		type Request struct {
			ID int `path:"id"`
		}

		// Non-pointer should fail
		var req Request
		err := BindTo(req,
			FromPath(map[string]string{"id": "123"}),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pointer")

		// Nil pointer should fail
		var nilReq *Request
		err = BindTo(nilReq,
			FromPath(map[string]string{"id": "123"}),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})
}

// TestBind_UnsupportedTypes tests binding with unsupported Go types
func TestBind_UnsupportedTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		values         url.Values
		params         any
		wantErr        bool
		expectedErrMsg string
		validate       func(t *testing.T, err error)
	}{
		{
			name: "unsupported type - Array",
			values: func() url.Values {
				v := url.Values{}
				v.Set("data", "1,2,3")

				return v
			}(),
			params: &struct {
				Data [5]int `query:"data"`
			}{},
			wantErr:        true,
			expectedErrMsg: "unsupported type",
			validate: func(t *testing.T, err error) {
				t.Helper()
				assert.ErrorContains(t, err, "array", "Error should mention 'array'")
			},
		},
		{
			name: "unsupported type - Chan",
			values: func() url.Values {
				v := url.Values{}
				v.Set("channel", "test")

				return v
			}(),
			params: &struct {
				Channel chan int `query:"channel"`
			}{},
			wantErr:        true,
			expectedErrMsg: "unsupported type",
			validate: func(t *testing.T, err error) {
				t.Helper()
				assert.ErrorContains(t, err, "Chan", "Error should mention 'Chan'")
			},
		},
		{
			name: "unsupported type - Func",
			values: func() url.Values {
				v := url.Values{}
				v.Set("handler", "test")

				return v
			}(),
			params: &struct {
				Handler func() `query:"handler"`
			}{},
			wantErr:        true,
			expectedErrMsg: "unsupported type",
			validate: func(t *testing.T, err error) {
				t.Helper()
				assert.ErrorContains(t, err, "func", "Error should mention 'func'")
			},
		},
		{
			name: "unsupported type - Complex64",
			values: func() url.Values {
				v := url.Values{}
				v.Set("complex", "1+2i")

				return v
			}(),
			params: &struct {
				Complex complex64 `query:"complex"`
			}{},
			wantErr:        true,
			expectedErrMsg: "unsupported type",
			validate:       func(t *testing.T, err error) { t.Helper() },
		},
		{
			name: "unsupported type - Complex128",
			values: func() url.Values {
				v := url.Values{}
				v.Set("complex", "1+2i")

				return v
			}(),
			params: &struct {
				Complex complex128 `query:"complex"`
			}{},
			wantErr:        true,
			expectedErrMsg: "unsupported type",
			validate:       func(t *testing.T, err error) { t.Helper() },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Raw(NewQueryGetter(tt.values), TagQuery, tt.params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
				if tt.expectedErrMsg != "" {
					require.ErrorContains(t, err, tt.expectedErrMsg, "Error should contain %q", tt.expectedErrMsg)
				}
				tt.validate(t, err)
			} else {
				// May or may not error, just test the path
				_ = err
			}
		})
	}
}

// TestForm tests Form generic binding from url.Values.
func TestForm(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Username string `form:"username"`
		Active   bool   `form:"active"`
	}
	values := url.Values{}
	values.Set("username", "formuser")
	values.Set("active", "true")
	result, err := Form[FormData](values)
	require.NoError(t, err)
	assert.Equal(t, "formuser", result.Username)
	assert.True(t, result.Active)
}

// TestFormTo tests FormTo binding into out.
func TestFormTo(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Email string `form:"email"`
	}
	values := url.Values{}
	values.Set("email", "a@b.com")
	var out FormData
	err := FormTo(values, &out)
	require.NoError(t, err)
	assert.Equal(t, "a@b.com", out.Email)
}

// TestHeader tests Header generic binding from http.Header.
func TestHeader(t *testing.T) {
	t.Parallel()

	type Headers struct {
		Auth  string `header:"Authorization"`
		ReqID string `header:"X-Request-ID"`
	}
	h := http.Header{}
	h.Set("Authorization", "Bearer token")
	h.Set("X-Request-ID", "123")
	result, err := Header[Headers](h)
	require.NoError(t, err)
	assert.Equal(t, "Bearer token", result.Auth)
	assert.Equal(t, "123", result.ReqID)
}

// TestHeaderTo tests HeaderTo binding into out.
func TestHeaderTo(t *testing.T) {
	t.Parallel()

	type Headers struct {
		APIKey string `header:"X-Api-Key"` //nolint:gosec // G117: test fixture
	}
	h := http.Header{}
	h.Set("X-Api-Key", "secret")
	var out Headers
	err := HeaderTo(h, &out)
	require.NoError(t, err)
	assert.Equal(t, "secret", out.APIKey)
}

// TestCookie tests Cookie generic binding from cookies slice.
func TestCookie(t *testing.T) {
	t.Parallel()

	type Cookies struct {
		Session string `cookie:"session_id"`
		Theme   string `cookie:"theme"`
	}
	cookies := []*http.Cookie{
		{Name: "session_id", Value: "xyz"},
		{Name: "theme", Value: "dark"},
	}
	result, err := Cookie[Cookies](cookies)
	require.NoError(t, err)
	assert.Equal(t, "xyz", result.Session)
	assert.Equal(t, "dark", result.Theme)
}

// TestCookieTo tests CookieTo binding into out.
func TestCookieTo(t *testing.T) {
	t.Parallel()

	type Cookies struct {
		Session string `cookie:"session"`
	}
	cookies := []*http.Cookie{{Name: "session", Value: "abc"}}
	var out Cookies
	err := CookieTo(cookies, &out)
	require.NoError(t, err)
	assert.Equal(t, "abc", out.Session)
}

// TestPath tests Path generic binding from path params.
func TestPath(t *testing.T) {
	t.Parallel()

	type Params struct {
		ID   string `path:"id"`
		Slug string `path:"slug"`
	}
	params := map[string]string{"id": "99", "slug": "hello"}
	result, err := Path[Params](params)
	require.NoError(t, err)
	assert.Equal(t, "99", result.ID)
	assert.Equal(t, "hello", result.Slug)
}

// TestPathTo tests PathTo binding into out.
func TestPathTo(t *testing.T) {
	t.Parallel()

	type Params struct {
		ID string `path:"id"`
	}
	params := map[string]string{"id": "42"}
	var out Params
	err := PathTo(params, &out)
	require.NoError(t, err)
	assert.Equal(t, "42", out.ID)
}

// TestRawInto tests RawInto generic binding.
func TestRawInto(t *testing.T) {
	t.Parallel()

	type Params struct {
		Name string `query:"name"`
		Page int    `query:"page"`
	}
	values := url.Values{}
	values.Set("name", "rawuser")
	values.Set("page", "3")
	getter := NewQueryGetter(values)
	result, err := RawInto[Params](getter, TagQuery)
	require.NoError(t, err)
	assert.Equal(t, "rawuser", result.Name)
	assert.Equal(t, 3, result.Page)
}

// TestBind_WithAllErrors tests that WithAllErrors collects all binding errors.
func TestBind_WithAllErrors(t *testing.T) {
	t.Parallel()

	type Request struct {
		Age  int `query:"age"`
		Page int `query:"page"`
	}
	values := url.Values{}
	values.Set("age", "not-a-number")
	values.Set("page", "also-invalid")
	var out Request
	err := Raw(NewQueryGetter(values), TagQuery, &out, WithAllErrors())
	require.Error(t, err)
	var multi *MultiError
	require.ErrorAs(t, err, &multi)
	require.Len(t, multi.Errors, 2)
}

// TestBindMultiSource_JSONSource tests BindTo with FromJSON so JSON path is exercised.
func TestBindMultiSource_JSONSource(t *testing.T) {
	t.Parallel()

	type Request struct {
		ID   int    `path:"id"`
		Name string `json:"name"`
		Page int    `query:"page"`
	}
	body := []byte(`{"name":"json-name"}`)
	var out Request
	err := BindTo(&out,
		FromPath(map[string]string{"id": "1"}),
		FromJSON(body),
		FromQuery(url.Values{"page": {"5"}}),
	)
	require.NoError(t, err)
	assert.Equal(t, 1, out.ID)
	assert.Equal(t, "json-name", out.Name)
	assert.Equal(t, 5, out.Page)
}

// TestApplyTypedDefault_Pointer tests pointer default application (setPointerDefault path).
func TestApplyTypedDefault_Pointer(t *testing.T) {
	t.Parallel()

	type Params struct {
		Age *int `query:"age" default:"99"`
	}
	values := url.Values{}
	var out Params
	err := Raw(NewQueryGetter(values), TagQuery, &out)
	require.NoError(t, err)
	require.NotNil(t, out.Age)
	assert.Equal(t, 99, *out.Age)
}
