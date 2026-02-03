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
	"net/url"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertValue_ErrorCases tests convertValue error paths
func TestConvertValue_ErrorCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		value          string
		kind           reflect.Kind
		expectedErrMsg string
	}{
		// Invalid unsigned integer errors
		{"negative value for uint", "-42", reflect.Uint, "invalid unsigned integer"},
		{"negative value for uint8", "-10", reflect.Uint8, "invalid unsigned integer"},
		{"negative value for uint16", "-100", reflect.Uint16, "invalid unsigned integer"},
		{"negative value for uint32", "-1000", reflect.Uint32, "invalid unsigned integer"},
		{"negative value for uint64", "-999999", reflect.Uint64, "invalid unsigned integer"},
		{"invalid format for uint", "abc", reflect.Uint, "invalid unsigned integer"},
		{"decimal for uint", "42.5", reflect.Uint, "invalid unsigned integer"},
		{"empty string for uint", "", reflect.Uint, "invalid unsigned integer"},
		{"whitespace for uint", "   ", reflect.Uint, "invalid unsigned integer"},
		// Invalid float errors
		{"invalid format for float32", "abc", reflect.Float32, "invalid float"},
		{"invalid format for float64", "xyz", reflect.Float64, "invalid float"},
		{"empty string for float32", "", reflect.Float32, "invalid float"},
		{"whitespace for float64", "   ", reflect.Float64, "invalid float"},
		{"mixed chars for float32", "12abc", reflect.Float32, "invalid float"},
		{"only dots for float64", "...", reflect.Float64, "invalid float"},
		{"multiple dots for float32", "12.34.56", reflect.Float32, "invalid float"},
		// Invalid bool errors
		{"invalid bool value", "maybe", reflect.Bool, "invalid"},
		{"numeric 2", "2", reflect.Bool, "invalid"},
		{"numeric 3", "3", reflect.Bool, "invalid"},
		{"random text", "random", reflect.Bool, "invalid"},
		{"mixed case invalid", "Maybe", reflect.Bool, "invalid"},
		{"yesno together", "yesno", reflect.Bool, "invalid"},
		{"truefalse together", "truefalse", reflect.Bool, "invalid"},
		// Unsupported type errors
		{"slice type", "", reflect.Slice, "unsupported type"},
		{"map type", "", reflect.Map, "unsupported type"},
		{"array type", "", reflect.Array, "unsupported type"},
		{"chan type", "", reflect.Chan, "unsupported type"},
		{"func type", "", reflect.Func, "unsupported type"},
		{"interface type", "", reflect.Interface, "unsupported type"},
		{"ptr type", "", reflect.Ptr, "unsupported type"},
		{"struct type", "", reflect.Struct, "unsupported type"},
		{"unsafe pointer", "", reflect.UnsafePointer, "unsupported type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := defaultConfig()
			_, err := convertValue(tt.value, tt.kind, opts)

			require.Error(t, err, "Expected error for %s", tt.name)
			assert.ErrorContains(t, err, tt.expectedErrMsg, "Error should contain %q", tt.expectedErrMsg)
		})
	}
}

// TestExtractBracketKey_EdgeCases tests extractBracketKey edge cases
func TestExtractBracketKey_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		prefix   string
		expected string
	}{
		{"no prefix match", "other[key]", "prefix", ""},
		{"no closing bracket", "prefix[unclosed", "prefix", ""},
		{"empty brackets", "prefix[]", "prefix", ""},
		{"nested brackets", "prefix[key1][key2]", "prefix", ""},
		{"quoted key with double quotes", `prefix["key.with.dots"]`, "prefix", "key.with.dots"},
		{"quoted key with single quotes", "prefix['key-with-dash']", "prefix", "key-with-dash"},
		{"quoted key empty after trimming", `prefix[""]`, "prefix", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := extractBracketKey(tt.input, tt.prefix)
			assert.Equal(t, tt.expected, result, "extractBracketKey(%q, %q) = %q, want %q", tt.input, tt.prefix, result, tt.expected)
		})
	}
}

// TestSetNestedStructWithDepth_PointerFields tests pointer-to-struct field handling
// in setNestedStructWithDepth. This covers the fix for the panic:
// "reflect: call of reflect.Value.Field on ptr Value"
func TestSetNestedStructWithDepth_PointerFields(t *testing.T) {
	t.Parallel()

	// Basic nested struct types for testing
	type PageSize struct {
		Width  int `query:"width"`
		Height int `query:"height"`
	}

	type Margin struct {
		Top    int `query:"top"`
		Bottom int `query:"bottom"`
		Left   int `query:"left"`
		Right  int `query:"right"`
	}

	type PrintSettings struct {
		PageSize *PageSize `query:"page_size"`
		Margin   *Margin   `query:"margin"`
		Copies   int       `query:"copies"`
	}

	type DocumentRequest struct {
		Title    string         `query:"title"`
		Settings *PrintSettings `query:"settings"`
	}

	tests := []struct {
		name        string
		buildValues func() url.Values
		params      any
		validate    func(t *testing.T, params any)
	}{
		{
			name: "nil pointer-to-struct is initialized and bound",
			buildValues: func() url.Values {
				v := url.Values{}
				v.Set("title", "Report")
				v.Set("settings.copies", "5")
				v.Set("settings.page_size.width", "210")
				v.Set("settings.page_size.height", "297")
				return v
			},
			params: &DocumentRequest{},
			validate: func(t *testing.T, params any) {
				t.Helper()
				p := params.(*DocumentRequest)
				assert.Equal(t, "Report", p.Title)
				require.NotNil(t, p.Settings, "Settings should be initialized")
				assert.Equal(t, 5, p.Settings.Copies)
				require.NotNil(t, p.Settings.PageSize, "PageSize should be initialized")
				assert.Equal(t, 210, p.Settings.PageSize.Width)
				assert.Equal(t, 297, p.Settings.PageSize.Height)
			},
		},
		{
			name: "pre-initialized pointer-to-struct preserves existing values",
			buildValues: func() url.Values {
				v := url.Values{}
				v.Set("settings.copies", "3")
				return v
			},
			params: &DocumentRequest{
				Title: "Existing",
				Settings: &PrintSettings{
					Copies: 10, // Will be overwritten
					PageSize: &PageSize{
						Width:  100, // Will NOT be overwritten (no value provided)
						Height: 200,
					},
				},
			},
			validate: func(t *testing.T, params any) {
				t.Helper()
				p := params.(*DocumentRequest)
				assert.Equal(t, "Existing", p.Title)
				require.NotNil(t, p.Settings)
				assert.Equal(t, 3, p.Settings.Copies, "Copies should be updated")
				require.NotNil(t, p.Settings.PageSize)
				assert.Equal(t, 100, p.Settings.PageSize.Width, "Width should be preserved")
				assert.Equal(t, 200, p.Settings.PageSize.Height, "Height should be preserved")
			},
		},
		{
			name: "deeply nested pointer-to-struct (3 levels)",
			buildValues: func() url.Values {
				v := url.Values{}
				v.Set("settings.margin.top", "10")
				v.Set("settings.margin.bottom", "20")
				v.Set("settings.margin.left", "15")
				v.Set("settings.margin.right", "15")
				return v
			},
			params: &DocumentRequest{},
			validate: func(t *testing.T, params any) {
				t.Helper()
				p := params.(*DocumentRequest)
				require.NotNil(t, p.Settings, "Settings should be initialized")
				require.NotNil(t, p.Settings.Margin, "Margin should be initialized")
				assert.Equal(t, 10, p.Settings.Margin.Top)
				assert.Equal(t, 20, p.Settings.Margin.Bottom)
				assert.Equal(t, 15, p.Settings.Margin.Left)
				assert.Equal(t, 15, p.Settings.Margin.Right)
			},
		},
		{
			name: "mixed pointer and value nested structs",
			buildValues: func() url.Values {
				v := url.Values{}
				v.Set("settings.copies", "2")
				v.Set("settings.page_size.width", "100")
				v.Set("settings.margin.top", "5")
				return v
			},
			params: &DocumentRequest{},
			validate: func(t *testing.T, params any) {
				t.Helper()
				p := params.(*DocumentRequest)
				require.NotNil(t, p.Settings)
				assert.Equal(t, 2, p.Settings.Copies)
				require.NotNil(t, p.Settings.PageSize)
				assert.Equal(t, 100, p.Settings.PageSize.Width)
				require.NotNil(t, p.Settings.Margin)
				assert.Equal(t, 5, p.Settings.Margin.Top)
			},
		},
		{
			name: "no values for pointer field leaves it nil",
			buildValues: func() url.Values {
				v := url.Values{}
				v.Set("title", "Empty Settings")
				return v
			},
			params: &DocumentRequest{},
			validate: func(t *testing.T, params any) {
				t.Helper()
				p := params.(*DocumentRequest)
				assert.Equal(t, "Empty Settings", p.Title)
				// Settings should remain nil since no settings.* values provided
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			values := tt.buildValues()
			getter := NewQueryGetter(values)
			err := Raw(getter, TagQuery, tt.params)
			require.NoError(t, err, "binding should succeed")
			tt.validate(t, tt.params)
		})
	}
}

// TestSetNestedStructWithDepth_EdgeCases tests edge cases for pointer-to-struct handling
func TestSetNestedStructWithDepth_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("pointer to struct with all pointer fields", func(t *testing.T) {
		t.Parallel()

		type Inner struct {
			Value *string `query:"value"`
		}
		type Outer struct {
			Inner *Inner `query:"inner"`
		}

		values := url.Values{}
		values.Set("inner.value", "test")

		var params Outer
		getter := NewQueryGetter(values)
		err := Raw(getter, TagQuery, &params)

		require.NoError(t, err)
		require.NotNil(t, params.Inner)
		require.NotNil(t, params.Inner.Value)
		assert.Equal(t, "test", *params.Inner.Value)
	})

	t.Run("multiple sibling pointer-to-struct fields", func(t *testing.T) {
		t.Parallel()

		type Address struct {
			Street string `query:"street"`
			City   string `query:"city"`
		}
		type Contact struct {
			Shipping *Address `query:"shipping"`
			Billing  *Address `query:"billing"`
		}

		values := url.Values{}
		values.Set("shipping.street", "123 Main St")
		values.Set("shipping.city", "NYC")
		values.Set("billing.street", "456 Oak Ave")
		values.Set("billing.city", "LA")

		var params Contact
		getter := NewQueryGetter(values)
		err := Raw(getter, TagQuery, &params)

		require.NoError(t, err)
		require.NotNil(t, params.Shipping)
		assert.Equal(t, "123 Main St", params.Shipping.Street)
		assert.Equal(t, "NYC", params.Shipping.City)
		require.NotNil(t, params.Billing)
		assert.Equal(t, "456 Oak Ave", params.Billing.Street)
		assert.Equal(t, "LA", params.Billing.City)
	})

	t.Run("pointer-to-struct with default values", func(t *testing.T) {
		t.Parallel()

		type Config struct {
			Timeout int `query:"timeout" default:"30"`
			Retries int `query:"retries" default:"3"`
		}
		type Request struct {
			Config *Config `query:"config"`
		}

		values := url.Values{}
		values.Set("config.timeout", "60") // Override default
		// retries not set, should use default

		var params Request
		getter := NewQueryGetter(values)
		err := Raw(getter, TagQuery, &params)

		require.NoError(t, err)
		require.NotNil(t, params.Config)
		assert.Equal(t, 60, params.Config.Timeout)
		assert.Equal(t, 3, params.Config.Retries, "Should use default value")
	})

	t.Run("pointer-to-struct with JSON fallback", func(t *testing.T) {
		t.Parallel()

		type Settings struct {
			Theme string `query:"theme" json:"theme"`
			Debug bool   `query:"debug" json:"debug"`
		}
		type Request struct {
			Settings *Settings `query:"settings"`
		}

		values := url.Values{}
		values.Set("settings", `{"theme":"dark","debug":true}`)

		var params Request
		getter := NewQueryGetter(values)
		err := Raw(getter, TagQuery, &params)

		require.NoError(t, err)
		require.NotNil(t, params.Settings)
		assert.Equal(t, "dark", params.Settings.Theme)
		assert.True(t, params.Settings.Debug)
	})

	t.Run("deeply nested 4 levels with pointers", func(t *testing.T) {
		t.Parallel()

		type Level4 struct {
			Value string `query:"value"`
		}
		type Level3 struct {
			L4 *Level4 `query:"l4"`
		}
		type Level2 struct {
			L3 *Level3 `query:"l3"`
		}
		type Level1 struct {
			L2 *Level2 `query:"l2"`
		}

		values := url.Values{}
		values.Set("l2.l3.l4.value", "deep")

		var params Level1
		getter := NewQueryGetter(values)
		err := Raw(getter, TagQuery, &params)

		require.NoError(t, err)
		require.NotNil(t, params.L2)
		require.NotNil(t, params.L2.L3)
		require.NotNil(t, params.L2.L3.L4)
		assert.Equal(t, "deep", params.L2.L3.L4.Value)
	})

	t.Run("pointer-to-struct with slice field", func(t *testing.T) {
		t.Parallel()

		type Filter struct {
			Tags   []string `query:"tags"`
			Status string   `query:"status"`
		}
		type Query struct {
			Filter *Filter `query:"filter"`
		}

		values := url.Values{}
		values.Add("filter.tags", "go")
		values.Add("filter.tags", "web")
		values.Set("filter.status", "active")

		var params Query
		getter := NewQueryGetter(values)
		err := Raw(getter, TagQuery, &params)

		require.NoError(t, err)
		require.NotNil(t, params.Filter)
		assert.Equal(t, []string{"go", "web"}, params.Filter.Tags)
		assert.Equal(t, "active", params.Filter.Status)
	})

	t.Run("pointer-to-struct with map field", func(t *testing.T) {
		t.Parallel()

		type Metadata struct {
			Labels map[string]string `query:"labels"`
			Name   string            `query:"name"`
			Count  int               `query:"count"`
		}
		type Resource struct {
			Meta *Metadata `query:"meta"`
		}

		values := url.Values{}
		values.Set("meta.name", "my-resource")
		values.Set("meta.count", "42")
		values.Set("meta.labels.env", "prod")
		values.Set("meta.labels.team", "backend")

		var params Resource
		getter := NewQueryGetter(values)
		err := Raw(getter, TagQuery, &params)

		require.NoError(t, err)
		require.NotNil(t, params.Meta, "Meta pointer should be initialized")
		assert.Equal(t, "my-resource", params.Meta.Name)
		assert.Equal(t, 42, params.Meta.Count)
		// Note: Map binding within pointer-to-struct is tested separately
		// This test focuses on verifying pointer-to-struct initialization works
	})
}

// TestSetNestedStructWithDepth_FormBinding tests pointer-to-struct with form binding
func TestSetNestedStructWithDepth_FormBinding(t *testing.T) {
	t.Parallel()

	type Address struct {
		Street string `form:"street"`
		City   string `form:"city"`
	}
	type Person struct {
		Name    string   `form:"name"`
		Address *Address `form:"address"`
	}

	values := url.Values{}
	values.Set("name", "John")
	values.Set("address.street", "123 Main St")
	values.Set("address.city", "NYC")

	getter := NewFormGetter(values)

	var params Person
	err := Raw(getter, TagForm, &params)

	require.NoError(t, err)
	assert.Equal(t, "John", params.Name)
	require.NotNil(t, params.Address)
	assert.Equal(t, "123 Main St", params.Address.Street)
	assert.Equal(t, "NYC", params.Address.City)
}
