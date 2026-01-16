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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPrefixGetter_Has tests the Has method of prefixGetter for all getter types
func TestPrefixGetter_Has(t *testing.T) {
	t.Parallel()

	t.Run("QueryGetter", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			setup    func() (*prefixGetter, url.Values)
			key      string
			expected bool
		}{
			{
				name: "queryGetter - exact key match",
				setup: func() (*prefixGetter, url.Values) {
					values := url.Values{}
					values.Set("address.street", "Main St")
					values.Set("address.city", "NYC")
					values.Set("name", "John")
					getter := NewQueryGetter(values)

					return &prefixGetter{inner: getter, prefix: "address."}, values
				},
				key:      "street",
				expected: true,
			},
			{
				name: "queryGetter - exact key match (city)",
				setup: func() (*prefixGetter, url.Values) {
					values := url.Values{}
					values.Set("address.street", "Main St")
					values.Set("address.city", "NYC")
					values.Set("name", "John")
					getter := NewQueryGetter(values)

					return &prefixGetter{inner: getter, prefix: "address."}, values
				},
				key:      "city",
				expected: true,
			},
			{
				name: "queryGetter - key without prefix",
				setup: func() (*prefixGetter, url.Values) {
					values := url.Values{}
					values.Set("address.street", "Main St")
					values.Set("address.city", "NYC")
					values.Set("name", "John")
					getter := NewQueryGetter(values)

					return &prefixGetter{inner: getter, prefix: "address."}, values
				},
				key:      "name",
				expected: false,
			},
			{
				name: "queryGetter - prefix match with dot",
				setup: func() (*prefixGetter, url.Values) {
					values := url.Values{}
					values.Set("address.location.lat", "40.7128")
					values.Set("address.location.lng", "-74.0060")
					values.Set("address.city", "NYC")
					getter := NewQueryGetter(values)

					return &prefixGetter{inner: getter, prefix: "address."}, values
				},
				key:      "location",
				expected: true,
			},
			{
				name: "queryGetter - no matching keys",
				setup: func() (*prefixGetter, url.Values) {
					values := url.Values{}
					values.Set("user.name", "John")
					values.Set("user.email", "john@example.com")
					values.Set("other.field", "value")
					getter := NewQueryGetter(values)

					return &prefixGetter{inner: getter, prefix: "address."}, values
				},
				key:      "street",
				expected: false,
			},
			{
				name: "queryGetter - empty values",
				setup: func() (*prefixGetter, url.Values) {
					values := url.Values{}
					getter := NewQueryGetter(values)

					return &prefixGetter{inner: getter, prefix: "address."}, values
				},
				key:      "street",
				expected: false,
			},
			{
				name: "queryGetter - multiple prefix matches",
				setup: func() (*prefixGetter, url.Values) {
					values := url.Values{}
					values.Set("address.street", "Main St")
					values.Set("address.street.number", "123")
					values.Set("address.city", "NYC")
					getter := NewQueryGetter(values)

					return &prefixGetter{inner: getter, prefix: "address."}, values
				},
				key:      "street",
				expected: true,
			},
			{
				name: "queryGetter - iteration path when direct Has returns false",
				setup: func() (*prefixGetter, url.Values) {
					values := url.Values{}
					values.Set("address.location.lat", "40.7128")
					getter := NewQueryGetter(values)

					return &prefixGetter{inner: getter, prefix: "address."}, values
				},
				key:      "location",
				expected: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				pg, _ := tt.setup()
				result := pg.Has(tt.key)
				assert.Equal(t, tt.expected, result, "prefixGetter.Has(%q) = %v, want %v", tt.key, result, tt.expected)
			})
		}
	})

	t.Run("FormGetter", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			setup    func() (*prefixGetter, url.Values)
			key      string
			expected bool
		}{
			{
				name: "exact key match",
				setup: func() (*prefixGetter, url.Values) {
					values := url.Values{}
					values.Set("metadata.name", "John")
					values.Set("metadata.age", "30")
					values.Set("title", "Mr")
					getter := NewFormGetter(values)

					return &prefixGetter{inner: getter, prefix: "metadata."}, values
				},
				key:      "name",
				expected: true,
			},
			{
				name: "prefix match with dot",
				setup: func() (*prefixGetter, url.Values) {
					values := url.Values{}
					values.Set("config.database.host", "localhost")
					values.Set("config.database.port", "5432")
					values.Set("config.debug", "true")
					getter := NewFormGetter(values)

					return &prefixGetter{inner: getter, prefix: "config."}, values
				},
				key:      "database",
				expected: true,
			},
			{
				name: "no matching keys",
				setup: func() (*prefixGetter, url.Values) {
					values := url.Values{}
					values.Set("user.name", "John")
					values.Set("other.field", "value")
					getter := NewFormGetter(values)

					return &prefixGetter{inner: getter, prefix: "config."}, values
				},
				key:      "debug",
				expected: false,
			},
			{
				name: "key that starts with prefix but doesn't match",
				setup: func() (*prefixGetter, url.Values) {
					values := url.Values{}
					values.Set("addresses.street", "Main St")
					values.Set("address.city", "NYC")
					getter := NewFormGetter(values)

					return &prefixGetter{inner: getter, prefix: "address."}, values
				},
				key:      "street",
				expected: false,
			},
			{
				name: "iteration path when direct Has returns false",
				setup: func() (*prefixGetter, url.Values) {
					values := url.Values{}
					values.Set("config.database.host", "localhost")
					getter := NewFormGetter(values)

					return &prefixGetter{inner: getter, prefix: "config."}, values
				},
				key:      "database",
				expected: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				pg, _ := tt.setup()
				result := pg.Has(tt.key)
				assert.Equal(t, tt.expected, result, "prefixGetter.Has(%q) = %v, want %v", tt.key, result, tt.expected)
			})
		}
	})

	t.Run("OtherGetters", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			setup    func() (*prefixGetter, string, bool)
			validate func(t *testing.T, result bool, key string)
		}{
			{
				name: "headerGetter with prefix",
				setup: func() (*prefixGetter, string, bool) {
					headers := http.Header{}
					headers.Set("X-Meta-Tags", "tag1")
					getter := NewHeaderGetter(headers)

					return &prefixGetter{inner: getter, prefix: "X-Meta-"}, "Tags", true
				},
				validate: func(t *testing.T, result bool, key string) {
					t.Helper()
					assert.True(t, result, "Expected Has(%q) to return true", key)
				},
			},
			{
				name: "headerGetter with prefix - nonexistent",
				setup: func() (*prefixGetter, string, bool) {
					headers := http.Header{}
					headers.Set("X-Meta-Tags", "tag1")
					getter := NewHeaderGetter(headers)

					return &prefixGetter{inner: getter, prefix: "X-Meta-"}, "Nonexistent", false
				},
				validate: func(t *testing.T, result bool, key string) {
					t.Helper()
					assert.False(t, result, "Expected Has(%q) to return false", key)
				},
			},
			{
				name: "paramsGetter with prefix",
				setup: func() (*prefixGetter, string, bool) {
					params := map[string]string{"user.name": "John"}
					getter := NewPathGetter(params)

					return &prefixGetter{inner: getter, prefix: "user."}, "name", true
				},
				validate: func(t *testing.T, result bool, key string) {
					t.Helper()
					assert.True(t, result, "Expected Has(%q) to return true", key)
				},
			},
			{
				name: "cookieGetter with prefix",
				setup: func() (*prefixGetter, string, bool) {
					cookies := []*http.Cookie{
						{Name: "user.id", Value: "123"},
					}
					getter := NewCookieGetter(cookies)

					return &prefixGetter{inner: getter, prefix: "user."}, "id", true
				},
				validate: func(t *testing.T, result bool, key string) {
					t.Helper()
					assert.True(t, result, "Expected Has(%q) to return true", key)
				},
			},
			{
				name: "cookieGetter with prefix - nonexistent",
				setup: func() (*prefixGetter, string, bool) {
					cookies := []*http.Cookie{
						{Name: "user.id", Value: "123"},
					}
					getter := NewCookieGetter(cookies)

					return &prefixGetter{inner: getter, prefix: "user."}, "nonexistent", false
				},
				validate: func(t *testing.T, result bool, key string) {
					t.Helper()
					assert.False(t, result, "Expected Has(%q) to return false", key)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				pg, key, expected := tt.setup()
				result := pg.Has(key)
				assert.Equal(t, expected, result, "prefixGetter.Has(%q) = %v, want %v", key, result, expected)
				tt.validate(t, result, key)
			})
		}
	})
}

// TestPrefixGetter_GetAll tests prefixGetter.GetAll method
func TestPrefixGetter_GetAll(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func() (*prefixGetter, string)
		validate func(t *testing.T, result []string, key string)
	}{
		{
			name: "queryGetter with prefix - name",
			setup: func() (*prefixGetter, string) {
				values := url.Values{}
				values.Add("user.name", "John")
				values.Add("user.email", "john@example.com")
				values.Add("user.email", "john.doe@example.com")
				values.Add("other.field", "value")
				getter := NewQueryGetter(values)

				return &prefixGetter{inner: getter, prefix: "user."}, "name"
			},
			validate: func(t *testing.T, result []string, key string) {
				t.Helper()
				require.Len(t, result, 1, "Expected 1 value for %q", key)
				assert.Equal(t, "John", result[0], "Expected first value to be 'John'")
			},
		},
		{
			name: "queryGetter with prefix - email",
			setup: func() (*prefixGetter, string) {
				values := url.Values{}
				values.Add("user.name", "John")
				values.Add("user.email", "john@example.com")
				values.Add("user.email", "john.doe@example.com")
				values.Add("other.field", "value")
				getter := NewQueryGetter(values)

				return &prefixGetter{inner: getter, prefix: "user."}, "email"
			},
			validate: func(t *testing.T, result []string, key string) {
				t.Helper()
				require.Len(t, result, 2, "Expected 2 values for %q", key)
				assert.Equal(t, "john@example.com", result[0], "Expected first email")
				assert.Equal(t, "john.doe@example.com", result[1], "Expected second email")
			},
		},
		{
			name: "queryGetter with prefix - nonexistent",
			setup: func() (*prefixGetter, string) {
				values := url.Values{}
				values.Add("user.name", "John")
				getter := NewQueryGetter(values)

				return &prefixGetter{inner: getter, prefix: "user."}, "nonexistent"
			},
			validate: func(t *testing.T, result []string, key string) {
				t.Helper()
				assert.Nil(t, result, "Expected nil for non-existent key")
			},
		},
		{
			name: "formGetter with prefix - tags",
			setup: func() (*prefixGetter, string) {
				values := url.Values{}
				values.Add("meta.tags", "go")
				values.Add("meta.tags", "rust")
				values.Add("meta.version", "1.0")
				values.Add("other.data", "value")
				getter := NewFormGetter(values)

				return &prefixGetter{inner: getter, prefix: "meta."}, "tags"
			},
			validate: func(t *testing.T, result []string, key string) {
				t.Helper()
				require.Len(t, result, 2, "Expected 2 values for %q", key)
			},
		},
		{
			name: "formGetter with prefix - version",
			setup: func() (*prefixGetter, string) {
				values := url.Values{}
				values.Add("meta.tags", "go")
				values.Add("meta.version", "1.0")
				getter := NewFormGetter(values)

				return &prefixGetter{inner: getter, prefix: "meta."}, "version"
			},
			validate: func(t *testing.T, result []string, key string) {
				t.Helper()
				require.Len(t, result, 1, "Expected 1 value for %q", key)
				assert.Equal(t, "1.0", result[0], "Expected version to be '1.0'")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pg, key := tt.setup()
			result := pg.GetAll(key)
			tt.validate(t, result, key)
		})
	}
}

// TestBind_PrefixGetterGetAllThroughNestedBinding tests prefixGetter.GetAll through actual nested struct binding
func TestBind_PrefixGetterGetAllThroughNestedBinding(t *testing.T) {
	t.Parallel()

	// Define struct types at test level to avoid type scope issues
	type Address struct {
		Tags []string `query:"tags"`
	}
	type ParamsQuery struct {
		Address Address `query:"address"`
	}

	type Metadata struct {
		Versions []string `form:"versions"`
	}
	type FormData struct {
		Metadata Metadata `form:"meta"`
	}

	type Item struct {
		Tags []string `query:"tags"`
	}
	type Section struct {
		Items Item `query:"item"`
	}
	type ParamsDeep struct {
		Section Section `query:"section"`
	}

	tests := []struct {
		name     string
		setup    func() (ValueGetter, string, any)
		validate func(t *testing.T, params any)
	}{
		{
			name: "nested struct with slice field - query",
			setup: func() (ValueGetter, string, any) {
				values := url.Values{}
				values.Add("address.tags", "home")
				values.Add("address.tags", "work")

				return NewQueryGetter(values), TagQuery, &ParamsQuery{}
			},
			validate: func(t *testing.T, params any) {
				t.Helper()
				p, ok := params.(*ParamsQuery)
				require.True(t, ok)
				require.Len(t, p.Address.Tags, 2, "Expected 2 tags")
				assert.Equal(t, "home", p.Address.Tags[0], "Expected first tag to be 'home'")
				assert.Equal(t, "work", p.Address.Tags[1], "Expected second tag to be 'work'")
			},
		},
		{
			name: "nested struct with slice field - form",
			setup: func() (ValueGetter, string, any) {
				values := url.Values{}
				values.Add("meta.versions", "1.0")
				values.Add("meta.versions", "2.0")
				values.Add("meta.versions", "3.0")

				return NewFormGetter(values), TagForm, &FormData{}
			},
			validate: func(t *testing.T, params any) {
				t.Helper()
				p, ok := params.(*FormData)
				require.True(t, ok)
				require.Len(t, p.Metadata.Versions, 3, "Expected 3 versions")
				assert.Equal(t, "1.0", p.Metadata.Versions[0], "Expected first version")
				assert.Equal(t, "2.0", p.Metadata.Versions[1], "Expected second version")
				assert.Equal(t, "3.0", p.Metadata.Versions[2], "Expected third version")
			},
		},
		{
			name: "deeply nested struct with slice",
			setup: func() (ValueGetter, string, any) {
				values := url.Values{}
				values.Add("section.item.tags", "tag1")
				values.Add("section.item.tags", "tag2")

				return NewQueryGetter(values), TagQuery, &ParamsDeep{}
			},
			validate: func(t *testing.T, params any) {
				t.Helper()
				p, ok := params.(*ParamsDeep)
				require.True(t, ok)
				require.Len(t, p.Section.Items.Tags, 2, "Expected 2 tags")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getter, tag, params := tt.setup()
			require.NoError(t, Raw(getter, tag, params), "%s should succeed", tt.name)
			tt.validate(t, params)
		})
	}
}
