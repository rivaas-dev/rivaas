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
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValueGetter_GetAll tests all GetAll implementations
func TestValueGetter_GetAll(t *testing.T) {
	t.Parallel()

	t.Run("queryGetter", func(t *testing.T) {
		t.Parallel()

		values := url.Values{}
		values.Add("tags", "go")
		values.Add("tags", "rust")
		values.Add("tags", "python")

		getter := NewQueryGetter(values)

		all := getter.GetAll("tags")
		assert.Equal(t, []string{"go", "rust", "python"}, all)

		none := getter.GetAll("nonexistent")
		assert.Nil(t, none)
	})

	t.Run("paramsGetter", func(t *testing.T) {
		t.Parallel()

		params := map[string]string{"id": "123"}
		getter := NewParamsGetter(params)

		all := getter.GetAll("id")
		assert.Equal(t, []string{"123"}, all)

		none := getter.GetAll("nonexistent")
		assert.Nil(t, none)
	})

	t.Run("cookieGetter", func(t *testing.T) {
		t.Parallel()

		cookies := []*http.Cookie{
			{Name: "session", Value: url.QueryEscape("abc123")},
			{Name: "session", Value: url.QueryEscape("def456")},
		}
		getter := NewCookieGetter(cookies)

		all := getter.GetAll("session")
		assert.Len(t, all, 2)

		none := getter.GetAll("nonexistent")
		assert.Empty(t, none)
	})

	t.Run("headerGetter", func(t *testing.T) {
		t.Parallel()

		headers := http.Header{}
		headers.Add("X-Tags", "tag1")
		headers.Add("X-Tags", "tag2")
		headers.Add("X-Tags", "tag3")

		getter := NewHeaderGetter(headers)

		all := getter.GetAll("X-Tags")
		assert.Len(t, all, 3)
	})

	t.Run("formGetter", func(t *testing.T) {
		t.Parallel()

		values := url.Values{}
		values.Add("items", "item1")
		values.Add("items", "item2")

		getter := NewFormGetter(values)

		all := getter.GetAll("items")
		assert.Len(t, all, 2)
	})
}

// TestCookieGetter_Has tests the Has method
func TestCookieGetter_Has(t *testing.T) {
	t.Parallel()

	cookies := []*http.Cookie{
		{Name: "session", Value: "abc123"},
		{Name: "user_id", Value: "42"},
	}

	getter := NewCookieGetter(cookies)

	assert.True(t, getter.Has("session"))
	assert.True(t, getter.Has("user_id"))
	assert.False(t, getter.Has("nonexistent"))
}

// TestCookieGetter_Get_UnescapingError tests cookieGetter.Get with URL unescaping error
func TestCookieGetter_Get_UnescapingError(t *testing.T) {
	t.Parallel()

	cookies := []*http.Cookie{
		{Name: "data", Value: "%ZZ"}, // Invalid percent encoding
	}

	getter := NewCookieGetter(cookies)

	// Should fallback to raw cookie value on unescaping error
	value := getter.Get("data")
	assert.Equal(t, "%ZZ", value, "Expected raw value %%ZZ on unescaping error")
}

// TestCookieGetter_Get_NotFound tests cookieGetter.Get when cookie is not found
func TestCookieGetter_Get_NotFound(t *testing.T) {
	t.Parallel()

	cookies := []*http.Cookie{
		{Name: "session_id", Value: "abc123"},
		{Name: "theme", Value: "dark"},
	}

	getter := NewCookieGetter(cookies)

	// Should return empty string when cookie key is not found
	value := getter.Get("nonexistent")
	assert.Empty(t, value, "Expected empty string for nonexistent cookie")

	// Verify existing cookies still work
	session := getter.Get("session_id")
	assert.Equal(t, "abc123", session, "Expected session_id to be 'abc123'")
}

// TestParamsGetter_GetAll_NonExistent tests paramsGetter.GetAll for non-existent key
func TestParamsGetter_GetAll_NonExistent(t *testing.T) {
	t.Parallel()

	params := map[string]string{"id": "123"}
	getter := NewParamsGetter(params)

	// Test non-existent key returns nil
	none := getter.GetAll("nonexistent")
	assert.Nil(t, none, "Expected nil for non-existent key")

	// Test existing key returns slice
	all := getter.GetAll("id")
	require.Len(t, all, 1, "Expected slice with 1 element")
	assert.Equal(t, "123", all[0], "Expected first element to be '123'")
}

// TestBind_Cookies tests cookie binding functionality
func TestBind_Cookies(t *testing.T) {
	t.Parallel()

	type SessionCookies struct {
		SessionID  string `cookie:"session_id"`
		Theme      string `cookie:"theme"`
		RememberMe bool   `cookie:"remember_me"`
	}

	tests := []struct {
		name     string
		cookies  []*http.Cookie
		params   any
		validate func(t *testing.T, params any)
	}{
		{
			name: "valid cookies",
			cookies: []*http.Cookie{
				{Name: "session_id", Value: "abc123"},
				{Name: "theme", Value: "dark"},
				{Name: "remember_me", Value: "true"},
			},
			params: &SessionCookies{},
			validate: func(t *testing.T, params any) {
				cookies, ok := params.(*SessionCookies)
				require.True(t, ok)
				assert.Equal(t, "abc123", cookies.SessionID)
				assert.Equal(t, "dark", cookies.Theme)
				assert.True(t, cookies.RememberMe)
			},
		},
		{
			name: "URL encoded cookies",
			cookies: []*http.Cookie{
				{Name: "session_id", Value: url.QueryEscape("value with spaces")},
			},
			params: &struct {
				SessionID string `cookie:"session_id"`
			}{},
			validate: func(t *testing.T, params any) {
				cookies, ok := params.(*struct {
					SessionID string `cookie:"session_id"`
				})
				require.True(t, ok)
				assert.Equal(t, "value with spaces", cookies.SessionID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getter := NewCookieGetter(tt.cookies)
			err := Bind(tt.params, getter, TagCookie)

			require.NoError(t, err)
			tt.validate(t, tt.params)
		})
	}
}

// TestBind_Headers tests HTTP header binding functionality
func TestBind_Headers(t *testing.T) {
	t.Parallel()

	type RequestHeaders struct {
		UserAgent string `header:"User-Agent"`
		Token     string `header:"Authorization"`
		Accept    string `header:"Accept"`
	}

	tests := []struct {
		name     string
		headers  http.Header
		params   any
		validate func(t *testing.T, params any)
	}{
		{
			name: "valid headers",
			headers: func() http.Header {
				h := http.Header{}
				h.Set("User-Agent", "Mozilla/5.0")
				h.Set("Authorization", "Bearer token123")
				h.Set("Accept", "application/json")
				return h
			}(),
			params: &RequestHeaders{},
			validate: func(t *testing.T, params any) {
				headers, ok := params.(*RequestHeaders)
				require.True(t, ok)
				assert.Equal(t, "Mozilla/5.0", headers.UserAgent)
				assert.Equal(t, "Bearer token123", headers.Token)
				assert.Equal(t, "application/json", headers.Accept)
			},
		},
		{
			name: "case insensitive",
			headers: func() http.Header {
				h := http.Header{}
				h.Set("User-Agent", "Test")
				return h
			}(),
			params: &struct {
				UserAgent string `header:"User-Agent"`
			}{},
			validate: func(t *testing.T, params any) {
				headers, ok := params.(*struct {
					UserAgent string `header:"User-Agent"`
				})
				require.True(t, ok)
				assert.Equal(t, "Test", headers.UserAgent)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getter := NewHeaderGetter(tt.headers)
			err := Bind(tt.params, getter, TagHeader)

			require.NoError(t, err)
			tt.validate(t, tt.params)
		})
	}
}

// TestBind_GetAll tests GetAll functionality through actual binding for all getter types
func TestBind_GetAll(t *testing.T) {
	t.Parallel()

	t.Run("ParamsGetter", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			ID string `params:"id"`
		}

		params := map[string]string{"id": "123"}
		getter := NewParamsGetter(params)

		var p Params
		require.NoError(t, Bind(&p, getter, TagParams), "BindParams should succeed")
		assert.Equal(t, "123", p.ID, "Expected ID=123")

		// Test that GetAll is used internally for slices
		type ParamsWithSlice struct {
			IDs []string `params:"id"`
		}

		var paramsSlice ParamsWithSlice
		params = map[string]string{"id": "456"}
		getter = NewParamsGetter(params)
		require.NoError(t, Bind(&paramsSlice, getter, TagParams), "BindParams should succeed for slice")
		require.Len(t, paramsSlice.IDs, 1, "Expected 1 ID")
		assert.Equal(t, "456", paramsSlice.IDs[0], "Expected first ID to be '456'")
	})

	t.Run("CookieGetter", func(t *testing.T) {
		t.Parallel()

		type CookiesWithSession struct {
			Session []string `cookie:"session"`
		}

		type CookiesWithData struct {
			Data []string `cookie:"data"`
		}

		tests := []struct {
			name     string
			setup    func() []*http.Cookie
			params   any
			validate func(t *testing.T, cookies any)
		}{
			{
				name: "multiple cookies with same name",
				setup: func() []*http.Cookie {
					return []*http.Cookie{
						{Name: "session", Value: "abc123"},
						{Name: "session", Value: "def456"},
					}
				},
				params: &CookiesWithSession{},
				validate: func(t *testing.T, cookies any) {
					c, ok := cookies.(*CookiesWithSession)
					require.True(t, ok)
					require.Len(t, c.Session, 2, "Expected 2 session cookies")
					assert.Equal(t, "abc123", c.Session[0], "Expected first session")
					assert.Equal(t, "def456", c.Session[1], "Expected second session")
				},
			},
			{
				name: "URL unescaping error path",
				setup: func() []*http.Cookie {
					return []*http.Cookie{{Name: "data", Value: "%ZZ"}} // Invalid percent encoding
				},
				params: &CookiesWithData{},
				validate: func(t *testing.T, cookies any) {
					c, ok := cookies.(*CookiesWithData)
					require.True(t, ok)
					require.Len(t, c.Data, 1, "Expected 1 data cookie")
					assert.Equal(t, "%ZZ", c.Data[0], "Expected raw value %%ZZ")
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				cookies := tt.setup()
				getter := NewCookieGetter(cookies)

				require.NoError(t, Bind(tt.params, getter, TagCookie), "BindCookies should succeed")
				tt.validate(t, tt.params)
			})
		}
	})

	t.Run("HeaderGetter", func(t *testing.T) {
		t.Parallel()

		type Headers struct {
			Tags []string `header:"X-Tags"`
		}

		headers := http.Header{}
		headers.Add("X-Tags", "tag1")
		headers.Add("X-Tags", "tag2")
		headers.Add("X-Tags", "tag3")

		getter := NewHeaderGetter(headers)

		var h Headers
		require.NoError(t, Bind(&h, getter, TagHeader), "BindHeaders should succeed")

		require.Len(t, h.Tags, 3, "Expected 3 tags")
		assert.Equal(t, "tag1", h.Tags[0], "Expected first tag")
		assert.Equal(t, "tag2", h.Tags[1], "Expected second tag")
		assert.Equal(t, "tag3", h.Tags[2], "Expected third tag")
	})
}

// TestValueGetter_HasSemantics tests that Has() correctly distinguishes empty vs missing
func TestValueGetter_HasSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupGetter func() ValueGetter
		key         string
		wantHas     bool
		wantGet     string
		validate    func(t *testing.T, getter ValueGetter)
	}{
		{
			name: "query getter - empty value",
			setupGetter: func() ValueGetter {
				values := url.Values{}
				values.Set("name", "") // Empty but present
				return NewQueryGetter(values)
			},
			key:     "name",
			wantHas: true,
			wantGet: "",
		},
		{
			name: "query getter - missing key",
			setupGetter: func() ValueGetter {
				values := url.Values{}
				values.Set("other", "value")
				return NewQueryGetter(values)
			},
			key:     "name",
			wantHas: false,
			wantGet: "",
		},
		{
			name: "form getter - empty value",
			setupGetter: func() ValueGetter {
				values := url.Values{}
				values.Set("email", "")
				return NewFormGetter(values)
			},
			key:     "email",
			wantHas: true,
			wantGet: "",
		},
		{
			name: "params getter - empty value",
			setupGetter: func() ValueGetter {
				params := map[string]string{
					"id": "",
				}
				return NewParamsGetter(params)
			},
			key:     "id",
			wantHas: true,
			wantGet: "",
			validate: func(t *testing.T, getter ValueGetter) {
				assert.False(t, getter.Has("missing"), "Has() should return false for missing key")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getter := tt.setupGetter()
			assert.Equal(t, tt.wantHas, getter.Has(tt.key), "Has() should return %v for key %q", tt.wantHas, tt.key)
			assert.Equal(t, tt.wantGet, getter.Get(tt.key), "Get() should return %q for key %q", tt.wantGet, tt.key)
			if tt.validate != nil {
				tt.validate(t, getter)
			}
		})
	}
}
