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
	"bytes"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_InvalidOptions(t *testing.T) {
	t.Parallel()

	t.Run("negative maxDepth returns error", func(t *testing.T) {
		t.Parallel()

		_, err := New(WithMaxDepth(-1))
		require.Error(t, err)
		assert.ErrorContains(t, err, "maxDepth")
	})

	t.Run("negative maxMapSize returns error", func(t *testing.T) {
		t.Parallel()

		_, err := New(WithMaxMapSize(-1))
		require.Error(t, err)
		assert.ErrorContains(t, err, "maxMapSize")
	})

	t.Run("negative maxSliceLen returns error", func(t *testing.T) {
		t.Parallel()

		_, err := New(WithMaxSliceLen(-1))
		require.Error(t, err)
		assert.ErrorContains(t, err, "maxSliceLen")
	})
}

func TestMustNew_PanicsOnInvalid(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() {
		MustNew(WithMaxDepth(-1))
	})
}

func TestBinder_QueryTo(t *testing.T) {
	t.Parallel()

	binder, err := New()
	require.NoError(t, err)

	type Params struct {
		Name string `query:"name"`
		Page int    `query:"page"`
	}
	values := url.Values{}
	values.Set("name", "alice")
	values.Set("page", "2")
	var out Params
	err = binder.QueryTo(values, &out)
	require.NoError(t, err)
	assert.Equal(t, "alice", out.Name)
	assert.Equal(t, 2, out.Page)
}

func TestBinder_PathTo(t *testing.T) {
	t.Parallel()

	binder, err := New()
	require.NoError(t, err)

	type Params struct {
		ID   string `path:"id"`
		Slug string `path:"slug"`
	}
	params := map[string]string{"id": "123", "slug": "hello"}
	var out Params
	err = binder.PathTo(params, &out)
	require.NoError(t, err)
	assert.Equal(t, "123", out.ID)
	assert.Equal(t, "hello", out.Slug)
}

func TestBinder_FormTo(t *testing.T) {
	t.Parallel()

	binder, err := New()
	require.NoError(t, err)

	type Form struct {
		Username string `form:"username"`
		Active   bool   `form:"active"`
	}
	values := url.Values{}
	values.Set("username", "testuser")
	values.Set("active", "true")
	var out Form
	err = binder.FormTo(values, &out)
	require.NoError(t, err)
	assert.Equal(t, "testuser", out.Username)
	assert.True(t, out.Active)
}

func TestBinder_HeaderTo(t *testing.T) {
	t.Parallel()

	binder, err := New()
	require.NoError(t, err)

	type Headers struct {
		Auth  string `header:"Authorization"`
		ReqID string `header:"X-Request-ID"`
	}
	h := http.Header{}
	h.Set("Authorization", "Bearer token")
	h.Set("X-Request-ID", "abc")
	var out Headers
	err = binder.HeaderTo(h, &out)
	require.NoError(t, err)
	assert.Equal(t, "Bearer token", out.Auth)
	assert.Equal(t, "abc", out.ReqID)
}

func TestBinder_CookieTo(t *testing.T) {
	t.Parallel()

	binder, err := New()
	require.NoError(t, err)

	type Cookies struct {
		Session string `cookie:"session_id"`
		Theme   string `cookie:"theme"`
	}
	cookies := []*http.Cookie{
		{Name: "session_id", Value: "xyz"},
		{Name: "theme", Value: "dark"},
	}
	var out Cookies
	err = binder.CookieTo(cookies, &out)
	require.NoError(t, err)
	assert.Equal(t, "xyz", out.Session)
	assert.Equal(t, "dark", out.Theme)
}

func TestBinder_JSONTo(t *testing.T) {
	t.Parallel()

	binder, err := New()
	require.NoError(t, err)

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	body := []byte(`{"name":"bob","age":25}`)
	var out User
	err = binder.JSONTo(body, &out)
	require.NoError(t, err)
	assert.Equal(t, "bob", out.Name)
	assert.Equal(t, 25, out.Age)
}

func TestBinder_JSONReaderTo(t *testing.T) {
	t.Parallel()

	binder, err := New()
	require.NoError(t, err)

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	body := []byte(`{"name":"charlie","age":30}`)
	var out User
	err = binder.JSONReaderTo(bytes.NewReader(body), &out)
	require.NoError(t, err)
	assert.Equal(t, "charlie", out.Name)
	assert.Equal(t, 30, out.Age)
}

func TestBinder_BindTo(t *testing.T) {
	t.Parallel()

	binder, err := New()
	require.NoError(t, err)

	type Request struct {
		ID   int    `path:"id"`
		Page int    `query:"page"`
		Name string `query:"name"`
	}
	var out Request
	err = binder.BindTo(&out,
		FromPath(map[string]string{"id": "42"}),
		FromQuery(url.Values{"page": {"3"}, "name": {"dave"}}),
	)
	require.NoError(t, err)
	assert.Equal(t, 42, out.ID)
	assert.Equal(t, 3, out.Page)
	assert.Equal(t, "dave", out.Name)
}

func TestQueryWith(t *testing.T) {
	t.Parallel()

	binder, err := New()
	require.NoError(t, err)

	type Params struct {
		Q    string `query:"q"`
		Page int    `query:"page"`
	}
	values := url.Values{}
	values.Set("q", "golang")
	values.Set("page", "1")
	result, err := QueryWith[Params](binder, values)
	require.NoError(t, err)
	assert.Equal(t, "golang", result.Q)
	assert.Equal(t, 1, result.Page)
}

func TestPathWith(t *testing.T) {
	t.Parallel()

	binder, err := New()
	require.NoError(t, err)

	type Params struct {
		ID string `path:"id"`
	}
	result, err := PathWith[Params](binder, map[string]string{"id": "99"})
	require.NoError(t, err)
	assert.Equal(t, "99", result.ID)
}

func TestFormWith(t *testing.T) {
	t.Parallel()

	binder, err := New()
	require.NoError(t, err)

	type Form struct {
		Email string `form:"email"`
	}
	values := url.Values{}
	values.Set("email", "a@b.com")
	result, err := FormWith[Form](binder, values)
	require.NoError(t, err)
	assert.Equal(t, "a@b.com", result.Email)
}

func TestHeaderWith(t *testing.T) {
	t.Parallel()

	binder, err := New()
	require.NoError(t, err)

	type Headers struct {
		APIKey string `header:"X-Api-Key"`
	}
	h := http.Header{}
	h.Set("X-Api-Key", "secret")
	result, err := HeaderWith[Headers](binder, h)
	require.NoError(t, err)
	assert.Equal(t, "secret", result.APIKey)
}

func TestCookieWith(t *testing.T) {
	t.Parallel()

	binder, err := New()
	require.NoError(t, err)

	type Cookies struct {
		Session string `cookie:"session"`
	}
	cookies := []*http.Cookie{{Name: "session", Value: "abc123"}}
	result, err := CookieWith[Cookies](binder, cookies)
	require.NoError(t, err)
	assert.Equal(t, "abc123", result.Session)
}

func TestJSONWith(t *testing.T) {
	t.Parallel()

	binder, err := New()
	require.NoError(t, err)

	type User struct {
		Name string `json:"name"`
	}
	body := []byte(`{"name":"eve"}`)
	result, err := JSONWith[User](binder, body)
	require.NoError(t, err)
	assert.Equal(t, "eve", result.Name)
}

func TestJSONReaderWith(t *testing.T) {
	t.Parallel()

	binder, err := New()
	require.NoError(t, err)

	type User struct {
		Name string `json:"name"`
	}
	body := []byte(`{"name":"frank"}`)
	result, err := JSONReaderWith[User](binder, bytes.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, "frank", result.Name)
}

func TestBindWith(t *testing.T) {
	t.Parallel()

	binder, err := New()
	require.NoError(t, err)

	type Request struct {
		ID   int `path:"id"`
		Page int `query:"page"`
	}
	result, err := BindWith[Request](binder,
		FromPath(map[string]string{"id": "10"}),
		FromQuery(url.Values{"page": {"5"}}),
	)
	require.NoError(t, err)
	assert.Equal(t, 10, result.ID)
	assert.Equal(t, 5, result.Page)
}

func TestBinder_BindTo_WithPerCallOptions(t *testing.T) {
	t.Parallel()

	binder, err := New(WithMaxDepth(4))
	require.NoError(t, err)

	type Nested struct {
		Inner struct {
			Value string `query:"value"`
		} `query:"inner"`
	}
	values := url.Values{}
	values.Set("inner.value", "ok")
	var out Nested
	err = binder.BindTo(&out, FromQuery(values))
	require.NoError(t, err)
	assert.Equal(t, "ok", out.Inner.Value)
}

func TestBinder_JSONReaderTo_Error(t *testing.T) {
	t.Parallel()

	binder, err := New()
	require.NoError(t, err)

	type User struct {
		Name string `json:"name"`
	}
	var out User
	err = binder.JSONReaderTo(bytes.NewReader([]byte("invalid")), &out)
	require.Error(t, err)
}
