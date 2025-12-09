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

package binding

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBind_FormBasicTypes tests binding basic form data types
func TestBind_FormBasicTypes(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Name   string  `form:"name"`
		Age    int     `form:"age"`
		Active bool    `form:"active"`
		Score  float64 `form:"score"`
	}

	values := url.Values{}
	values.Set("name", "John Doe")
	values.Set("age", "30")
	values.Set("active", "true")
	values.Set("score", "95.5")

	getter := NewFormGetter(values)

	var data FormData
	err := Raw(getter, TagForm, &data)

	require.NoError(t, err)
	assert.Equal(t, "John Doe", data.Name)
	assert.Equal(t, 30, data.Age)
	assert.True(t, data.Active)
	assert.Equal(t, 95.5, data.Score) //nolint:testifylint // exact decimal comparison
}

// TestBind_FormNestedStructs tests binding nested form structures
func TestBind_FormNestedStructs(t *testing.T) {
	t.Parallel()

	type Address struct {
		Street string `form:"street"`
		City   string `form:"city"`
		Zip    string `form:"zip"`
	}

	type User struct {
		Name    string  `form:"name"`
		Address Address `form:"address"`
	}

	values := url.Values{}
	values.Set("name", "Alice")
	values.Set("address.street", "123 Main St")
	values.Set("address.city", "Springfield")
	values.Set("address.zip", "12345")

	getter := NewFormGetter(values)

	var user User
	err := Raw(getter, TagForm, &user)

	require.NoError(t, err)
	assert.Equal(t, "Alice", user.Name)
	assert.Equal(t, "123 Main St", user.Address.Street)
	assert.Equal(t, "Springfield", user.Address.City)
	assert.Equal(t, "12345", user.Address.Zip)
}

// TestBind_FormSlices tests binding slice form data
func TestBind_FormSlices(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Tags []string `form:"tags"`
		IDs  []int    `form:"ids"`
	}

	values := url.Values{}
	values.Add("tags", "go")
	values.Add("tags", "rust")
	values.Add("tags", "python")
	values.Add("ids", "1")
	values.Add("ids", "2")
	values.Add("ids", "3")

	getter := NewFormGetter(values)

	var data FormData
	err := Raw(getter, TagForm, &data)

	require.NoError(t, err)
	assert.Equal(t, []string{"go", "rust", "python"}, data.Tags)
	assert.Equal(t, []int{1, 2, 3}, data.IDs)
}

// TestBind_FormMaps tests comprehensive map binding scenarios in forms
func TestBind_FormMaps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		values   url.Values
		params   any
		wantErr  bool
		validate func(t *testing.T, params any, err error)
	}{
		{
			name: "bracket notation - string map",
			values: func() url.Values {
				v := url.Values{}
				v.Set("metadata[key1]", "value1")
				v.Set("metadata[key2]", "value2")
				v.Set("metadata[key3]", "value3")

				return v
			}(),
			params: &struct {
				Metadata map[string]string `form:"metadata"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				t.Helper()
				p, ok := params.(*struct {
					Metadata map[string]string `form:"metadata"`
				})
				require.True(t, ok)
				require.NotNil(t, p.Metadata)
				assert.Equal(t, "value1", p.Metadata["key1"])
				assert.Equal(t, "value2", p.Metadata["key2"])
				assert.Equal(t, "value3", p.Metadata["key3"])
			},
		},
		{
			name: "dot notation - string map",
			values: func() url.Values {
				v := url.Values{}
				v.Set("metadata.name", "John")
				v.Set("metadata.age", "30")

				return v
			}(),
			params: &struct {
				Metadata map[string]string `form:"metadata"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				t.Helper()
				p, ok := params.(*struct {
					Metadata map[string]string `form:"metadata"`
				})
				require.True(t, ok)
				require.NotNil(t, p.Metadata)
				assert.Equal(t, "John", p.Metadata["name"])
				assert.Equal(t, "30", p.Metadata["age"])
			},
		},
		{
			name: "typed map - int values",
			values: func() url.Values {
				v := url.Values{}
				v.Set("intmap[count]", "42")
				v.Set("intmap[total]", "100")

				return v
			}(),
			params: &struct {
				IntMap map[string]int `form:"intmap"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				t.Helper()
				p, ok := params.(*struct {
					IntMap map[string]int `form:"intmap"`
				})
				require.True(t, ok)
				require.NotNil(t, p.IntMap)
				assert.Equal(t, 42, p.IntMap["count"])
				assert.Equal(t, 100, p.IntMap["total"])
			},
		},
		{
			name: "map[string]any",
			values: func() url.Values {
				v := url.Values{}
				v.Set("data[name]", "John")
				v.Set("data[age]", "30")
				v.Set("data[active]", "true")

				return v
			}(),
			params: &struct {
				Data map[string]any `form:"data"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				t.Helper()
				p, ok := params.(*struct {
					Data map[string]any `form:"data"`
				})
				require.True(t, ok)
				require.NotNil(t, p.Data)
				assert.Equal(t, "John", p.Data["name"])
				assert.Equal(t, "30", p.Data["age"])
				assert.Equal(t, "true", p.Data["active"])
			},
		},
		{
			name: "pointer to map",
			values: func() url.Values {
				v := url.Values{}
				v.Set("metadata.name", "John")
				v.Set("metadata.age", "30")

				return v
			}(),
			params: &struct {
				Metadata *map[string]string `form:"metadata"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				t.Helper()
				p, ok := params.(*struct {
					Metadata *map[string]string `form:"metadata"`
				})
				require.True(t, ok)
				require.NotNil(t, p.Metadata)
				assert.Equal(t, "John", (*p.Metadata)["name"])
				assert.Equal(t, "30", (*p.Metadata)["age"])
			},
		},
		{
			name: "JSON fallback - string map",
			values: func() url.Values {
				v := url.Values{}
				v.Set("metadata", `{"name":"John","age":"30","city":"NYC"}`)

				return v
			}(),
			params: &struct {
				Metadata map[string]string `form:"metadata"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				t.Helper()
				p, ok := params.(*struct {
					Metadata map[string]string `form:"metadata"`
				})
				require.True(t, ok)
				require.NotNil(t, p.Metadata)
				assert.Equal(t, "John", p.Metadata["name"])
				assert.Equal(t, "30", p.Metadata["age"])
				assert.Equal(t, "NYC", p.Metadata["city"])
			},
		},
		{
			name: "JSON fallback - int map",
			values: func() url.Values {
				v := url.Values{}
				v.Set("scores", `{"math":95,"science":88}`)

				return v
			}(),
			params: &struct {
				Scores map[string]int `form:"scores"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				t.Helper()
				p, ok := params.(*struct {
					Scores map[string]int `form:"scores"`
				})
				require.True(t, ok)
				require.NotNil(t, p.Scores)
				assert.Equal(t, 95, p.Scores["math"])
				assert.Equal(t, 88, p.Scores["science"])
			},
		},
		{
			name: "complex bracket keys",
			values: func() url.Values {
				v := url.Values{}
				v.Set("config[key.with.dots]", "value1")
				v.Set("config[key-with-dashes]", "value2")
				v.Set("config[key_with_underscores]", "value3")
				v.Set("config[123numeric]", "value4")

				return v
			}(),
			params: &struct {
				Config map[string]string `form:"config"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				t.Helper()
				p, ok := params.(*struct {
					Config map[string]string `form:"config"`
				})
				require.True(t, ok)
				require.NotNil(t, p.Config)
				assert.Len(t, p.Config, 4)
				assert.Equal(t, "value1", p.Config["key.with.dots"])
				assert.Equal(t, "value2", p.Config["key-with-dashes"])
				assert.Equal(t, "value3", p.Config["key_with_underscores"])
				assert.Equal(t, "value4", p.Config["123numeric"])
			},
		},
		{
			name: "nested maps not supported",
			values: func() url.Values {
				v := url.Values{}
				v.Set("config[database][host]", "localhost")

				return v
			}(),
			params: &struct {
				Config map[string]map[string]string `form:"config"`
			}{},
			wantErr: true,
			validate: func(t *testing.T, params any, err error) {
				t.Helper()
				assert.True(t,
					strings.Contains(err.Error(), "map") ||
						strings.Contains(err.Error(), "unsupported") ||
						strings.Contains(err.Error(), "Config"),
					"Error should mention map, unsupported, or field name: %s", err.Error())
			},
		},
		{
			name: "map[int]string not supported",
			values: func() url.Values {
				v := url.Values{}
				v.Set("items[1]", "first")
				v.Set("items[2]", "second")

				return v
			}(),
			params: &struct {
				Items map[int]string `form:"items"`
			}{},
			wantErr: true,
			validate: func(t *testing.T, params any, err error) {
				t.Helper()
				assert.True(t,
					strings.Contains(err.Error(), "unsupported") ||
						strings.Contains(err.Error(), "map[string]") ||
						strings.Contains(err.Error(), "Items"),
					"Error should mention unsupported, map[string], or field name: %s", err.Error())
			},
		},
		{
			name: "invalid map key conversion",
			values: func() url.Values {
				v := url.Values{}
				v.Set("intkeys[not-a-number]", "value")

				return v
			}(),
			params: &struct {
				IntKeys map[int]string `form:"intkeys"`
			}{},
			wantErr: true,
			validate: func(t *testing.T, params any, err error) {
				t.Helper()
				assert.True(t,
					strings.Contains(err.Error(), "not-a-number") ||
						strings.Contains(err.Error(), "IntKeys") ||
						strings.Contains(err.Error(), "unsupported") ||
						strings.Contains(err.Error(), "map[string]"),
					"Error should mention invalid key, field name, unsupported, or map[string]: %s", err.Error())
			},
		},
		{
			name: "type conversion error - invalid int",
			values: func() url.Values {
				v := url.Values{}
				v.Set("scores.math", "not-a-number")
				v.Set("scores.science", "88")

				return v
			}(),
			params: &struct {
				Scores map[string]int `form:"scores"`
			}{},
			wantErr: true,
			validate: func(t *testing.T, params any, err error) {
				t.Helper()
				assert.ErrorContains(t, err, "math", "Error should mention key 'math'")
			},
		},
		{
			name: "type conversion error - invalid float",
			values: func() url.Values {
				v := url.Values{}
				v.Set("rates[usd]", "invalid-float")

				return v
			}(),
			params: &struct {
				Rates map[string]float64 `form:"rates"`
			}{},
			wantErr: true,
			validate: func(t *testing.T, params any, err error) {
				t.Helper()
				assert.ErrorContains(t, err, "usd", "Error should mention key 'usd'")
			},
		},
		{
			name: "invalid JSON fallback - silently fails",
			values: func() url.Values {
				v := url.Values{}
				v.Set("metadata", `{invalid json}`)

				return v
			}(),
			params: &struct {
				Metadata map[string]string `form:"metadata"`
			}{},
			wantErr: false,
			validate: func(t *testing.T, params any, err error) {
				t.Helper()
				p, ok := params.(*struct {
					Metadata map[string]string `form:"metadata"`
				})
				require.True(t, ok)
				// Map should remain nil or empty since JSON parsing failed
				assert.Empty(t, p.Metadata, "Metadata should be empty when JSON is invalid")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getter := NewFormGetter(tt.values)
			err := Raw(getter, TagForm, tt.params)

			if tt.wantErr {
				require.Error(t, err, "Expected error for %s", tt.name)
			} else {
				require.NoError(t, err, "Bind should succeed for %s", tt.name)
			}
			tt.validate(t, tt.params, err)
		})
	}
}

// TestBind_FormSpecialCharacters tests form data with special characters
func TestBind_FormSpecialCharacters(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Text string `form:"text"`
	}

	values := url.Values{}
	values.Set("text", "Hello & Goodbye! @#$%^&*()")

	getter := NewFormGetter(values)

	var data FormData
	err := Raw(getter, TagForm, &data)

	require.NoError(t, err)
	assert.Equal(t, "Hello & Goodbye! @#$%^&*()", data.Text)
}

// TestBind_FormErrorCases tests error scenarios in form binding
func TestBind_FormErrorCases(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Age int `form:"age"`
	}

	values := url.Values{}
	values.Set("age", "not-a-number")

	getter := NewFormGetter(values)

	var data FormData
	err := Raw(getter, TagForm, &data)

	require.Error(t, err)
	assert.ErrorContains(t, err, "Age")
}

// TestBind_FormDuplicateKeys tests handling of duplicate form keys
func TestBind_FormDuplicateKeys(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Values []string `form:"value"`
	}

	values := url.Values{}
	values.Add("value", "first")
	values.Add("value", "second")
	values.Add("value", "third")

	getter := NewFormGetter(values)

	var data FormData
	err := Raw(getter, TagForm, &data)

	require.NoError(t, err)
	assert.Len(t, data.Values, 3)
	assert.Equal(t, []string{"first", "second", "third"}, data.Values)
}

// TestBind_FormEmptyForm tests binding with empty form data
func TestBind_FormEmptyForm(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Name string `form:"name"`
	}

	values := url.Values{}

	getter := NewFormGetter(values)

	var data FormData
	err := Raw(getter, TagForm, &data)

	require.NoError(t, err)
	assert.Empty(t, data.Name)
}

// TestBind_FormUnicodeCharacters tests form data with unicode characters
func TestBind_FormUnicodeCharacters(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Text string `form:"text"`
		Name string `form:"name"`
	}

	values := url.Values{}
	values.Set("text", "Hello 世界! 🌍")
	values.Set("name", "José María Ömer")

	getter := NewFormGetter(values)

	var data FormData
	err := Raw(getter, TagForm, &data)

	require.NoError(t, err)
	assert.Equal(t, "Hello 世界! 🌍", data.Text)
	assert.Equal(t, "José María Ömer", data.Name)
}

// TestBind_FormEmptySliceInitialization tests that empty slices are properly handled
func TestBind_FormEmptySliceInitialization(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Tags []string `form:"tags"`
	}

	values := url.Values{}

	getter := NewFormGetter(values)

	var data FormData
	err := Raw(getter, TagForm, &data)

	require.NoError(t, err)
	// Slices without values should remain nil (not cause error)
	assert.Nil(t, data.Tags)
}

// TestBind_FormEmptyMapInitialization tests that empty maps are properly initialized
func TestBind_FormEmptyMapInitialization(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Data map[string]string `form:"data"`
	}

	values := url.Values{}

	getter := NewFormGetter(values)

	var data FormData
	err := Raw(getter, TagForm, &data)

	require.NoError(t, err)
	// Maps without values should remain nil (not cause error)
	assert.Empty(t, data.Data)
}

// TestBind_FormComplexNested tests deeply nested form structures
func TestBind_FormComplexNested(t *testing.T) {
	t.Parallel()

	type Nested struct {
		Level3 string `form:"level3"`
	}

	type Middle struct {
		Level2 string `form:"level2"`
		Nested Nested `form:"nested"`
	}

	type FormData struct {
		Level1 string `form:"level1"`
		Middle Middle `form:"middle"`
	}

	values := url.Values{}
	values.Set("level1", "value1")
	values.Set("middle.level2", "value2")
	values.Set("middle.nested.level3", "value3")

	getter := NewFormGetter(values)

	var data FormData
	err := Raw(getter, TagForm, &data)

	require.NoError(t, err)
	assert.Equal(t, "value1", data.Level1)
	assert.Equal(t, "value2", data.Middle.Level2)
	assert.Equal(t, "value3", data.Middle.Nested.Level3)
}

// TestBind_FormArrayNotation tests array notation in form keys
func TestBind_FormArrayNotation(t *testing.T) {
	t.Parallel()

	type FormData struct {
		Items []string `form:"items"`
	}

	values := url.Values{}
	values.Add("items[]", "item1")
	values.Add("items[]", "item2")
	values.Add("items[]", "item3")

	getter := NewFormGetter(values)

	var data FormData
	err := Raw(getter, TagForm, &data)

	require.NoError(t, err)
	assert.Len(t, data.Items, 3)
	assert.Equal(t, []string{"item1", "item2", "item3"}, data.Items)
}

// TestBind_FormURLEncoded tests URL-encoded form data
func TestBind_FormURLEncoded(t *testing.T) {
	t.Parallel()

	type FormData struct {
		URL   string `form:"url"`
		Query string `form:"query"`
	}

	values := url.Values{}
	values.Set("url", "https://example.com/path?foo=bar")
	values.Set("query", "search term with spaces")

	getter := NewFormGetter(values)

	var data FormData
	err := Raw(getter, TagForm, &data)

	require.NoError(t, err)
	assert.Equal(t, "https://example.com/path?foo=bar", data.URL)
	assert.Equal(t, "search term with spaces", data.Query)
}

// TestBind_FormAllTypes tests type conversion for all supported types
func TestBind_FormAllTypes(t *testing.T) {
	t.Parallel()

	type AllTypes struct {
		String  string  `form:"str"`
		Int     int     `form:"int"`
		Int8    int8    `form:"int8"`
		Int16   int16   `form:"int16"`
		Int32   int32   `form:"int32"`
		Int64   int64   `form:"int64"`
		Uint    uint    `form:"uint"`
		Uint8   uint8   `form:"uint8"`
		Uint16  uint16  `form:"uint16"`
		Uint32  uint32  `form:"uint32"`
		Uint64  uint64  `form:"uint64"`
		Float32 float32 `form:"float32"`
		Float64 float64 `form:"float64"`
		Bool    bool    `form:"bool"`
	}

	values := url.Values{}
	values.Set("str", "text")
	values.Set("int", "42")
	values.Set("int8", "8")
	values.Set("int16", "16")
	values.Set("int32", "32")
	values.Set("int64", "64")
	values.Set("uint", "42")
	values.Set("uint8", "8")
	values.Set("uint16", "16")
	values.Set("uint32", "32")
	values.Set("uint64", "64")
	values.Set("float32", "3.14")
	values.Set("float64", "2.718")
	values.Set("bool", "true")

	var data AllTypes
	err := Raw(NewFormGetter(values), TagForm, &data)

	require.NoError(t, err, "Bind should succeed")
	assert.Equal(t, "text", data.String)
	assert.Equal(t, int(42), data.Int)
	assert.Equal(t, int8(8), data.Int8)
	assert.Equal(t, int16(16), data.Int16)
	assert.Equal(t, int32(32), data.Int32)
	assert.Equal(t, int64(64), data.Int64)
	assert.Equal(t, uint(42), data.Uint)
	assert.Equal(t, uint8(8), data.Uint8)
	assert.Equal(t, uint16(16), data.Uint16)
	assert.Equal(t, uint32(32), data.Uint32)
	assert.Equal(t, uint64(64), data.Uint64)
	assert.InDelta(t, float32(3.14), data.Float32, 0.001)
	assert.InDelta(t, 2.718, data.Float64, 0.001)
	assert.True(t, data.Bool)
}
