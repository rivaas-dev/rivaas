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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestBinder(t *testing.T) {
	t.Parallel()

	t.Run("default options returns usable binder", func(t *testing.T) {
		t.Parallel()

		binder := TestBinder(t)
		require.NotNil(t, binder)

		type Params struct {
			Name string `query:"name"`
		}
		values := url.Values{}
		values.Set("name", "alice")
		var out Params
		err := binder.QueryTo(values, &out)
		require.NoError(t, err)
		assert.Equal(t, "alice", out.Name)
	})

	t.Run("with extra options overrides defaults", func(t *testing.T) {
		t.Parallel()

		binder := TestBinder(t, WithMaxDepth(8))
		require.NotNil(t, binder)

		type Params struct {
			Name string `query:"name"`
		}
		values := url.Values{}
		values.Set("name", "bob")
		var out Params
		err := binder.QueryTo(values, &out)
		require.NoError(t, err)
		assert.Equal(t, "bob", out.Name)
	})
}

func TestTestQueryGetter(t *testing.T) {
	t.Parallel()

	t.Run("valid pairs creates getter that binds", func(t *testing.T) {
		t.Parallel()

		getter := TestQueryGetter(t, "name", "john", "age", "30")
		type Params struct {
			Name string `query:"name"`
			Age  int    `query:"age"`
		}
		var out Params
		err := Raw(getter, TagQuery, &out)
		require.NoError(t, err)
		assert.Equal(t, "john", out.Name)
		assert.Equal(t, 30, out.Age)
	})
}

func TestTestQueryGetterMulti(t *testing.T) {
	t.Parallel()

	getter := TestQueryGetterMulti(t, map[string][]string{
		"tags": {"go", "rust", "python"},
		"page": {"1"},
	})
	type Params struct {
		Tags []string `query:"tags"`
		Page int      `query:"page"`
	}
	var out Params
	err := Raw(getter, TagQuery, &out)
	require.NoError(t, err)
	assert.Equal(t, []string{"go", "rust", "python"}, out.Tags)
	assert.Equal(t, 1, out.Page)
}

func TestTestFormGetter(t *testing.T) {
	t.Parallel()

	t.Run("valid pairs creates getter that binds", func(t *testing.T) {
		t.Parallel()

		getter := TestFormGetter(t, "username", "testuser", "password", "secret")
		type Form struct {
			Username string `form:"username"`
			Password string `form:"password"`
		}
		var out Form
		err := Raw(getter, TagForm, &out)
		require.NoError(t, err)
		assert.Equal(t, "testuser", out.Username)
		assert.Equal(t, "secret", out.Password)
	})
}

func TestTestPathGetter(t *testing.T) {
	t.Parallel()

	t.Run("valid pairs creates getter that binds", func(t *testing.T) {
		t.Parallel()

		getter := TestPathGetter(t, "user_id", "123", "slug", "hello-world")
		type Params struct {
			UserID string `path:"user_id"`
			Slug   string `path:"slug"`
		}
		var out Params
		err := Raw(getter, TagPath, &out)
		require.NoError(t, err)
		assert.Equal(t, "123", out.UserID)
		assert.Equal(t, "hello-world", out.Slug)
	})
}

func TestTestHeaderGetter(t *testing.T) {
	t.Parallel()

	t.Run("valid pairs creates getter that binds", func(t *testing.T) {
		t.Parallel()

		getter := TestHeaderGetter(t, "Authorization", "Bearer token", "X-Request-ID", "123")
		type Headers struct {
			Auth  string `header:"Authorization"`
			ReqID string `header:"X-Request-ID"`
		}
		var out Headers
		err := Raw(getter, TagHeader, &out)
		require.NoError(t, err)
		assert.Equal(t, "Bearer token", out.Auth)
		assert.Equal(t, "123", out.ReqID)
	})
}

func TestTestCookieGetter(t *testing.T) {
	t.Parallel()

	t.Run("valid pairs creates getter that binds", func(t *testing.T) {
		t.Parallel()

		getter := TestCookieGetter(t, "session_id", "abc123", "theme", "dark")
		type Cookies struct {
			Session string `cookie:"session_id"`
			Theme   string `cookie:"theme"`
		}
		var out Cookies
		err := Raw(getter, TagCookie, &out)
		require.NoError(t, err)
		assert.Equal(t, "abc123", out.Session)
		assert.Equal(t, "dark", out.Theme)
	})
}

func TestAssertBindError(t *testing.T) {
	t.Parallel()

	t.Run("returns BindError when binding fails with expected field", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			Age int `query:"age"`
		}
		values := url.Values{}
		values.Set("age", "invalid")
		var out Params
		err := Raw(NewQueryGetter(values), TagQuery, &out)
		require.Error(t, err)

		bindErr := AssertBindError(t, err, "Age")
		require.NotNil(t, bindErr)
		assert.Equal(t, "Age", bindErr.Field)
		assert.Equal(t, SourceQuery, bindErr.Source)
		assert.Equal(t, "invalid", bindErr.Value)
	})
}

func TestAssertNoBindError(t *testing.T) {
	t.Parallel()

	t.Run("does nothing when err is nil", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			Name string `query:"name"`
		}
		values := url.Values{}
		values.Set("name", "ok")
		var out Params
		err := Raw(NewQueryGetter(values), TagQuery, &out)
		require.NoError(t, err)
		AssertNoBindError(t, err)
	})
}

func TestMustBind(t *testing.T) {
	t.Parallel()

	t.Run("success returns bound value", func(t *testing.T) {
		t.Parallel()

		getter := TestQueryGetter(t, "name", "alice", "age", "25")
		type Params struct {
			Name string `query:"name"`
			Age  int    `query:"age"`
		}
		result := MustBind[Params](t, getter, TagQuery)
		assert.Equal(t, "alice", result.Name)
		assert.Equal(t, 25, result.Age)
	})
}

func TestMustBindJSON(t *testing.T) {
	t.Parallel()

	t.Run("success returns bound value", func(t *testing.T) {
		t.Parallel()

		type User struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}
		result := MustBindJSON[User](t, `{"name":"Jane","age":30}`)
		assert.Equal(t, "Jane", result.Name)
		assert.Equal(t, 30, result.Age)
	})
}

func TestMustBindQuery(t *testing.T) {
	t.Parallel()

	t.Run("success returns bound value", func(t *testing.T) {
		t.Parallel()

		type Params struct {
			Q    string `query:"q"`
			Page int    `query:"page"`
		}
		values := url.Values{}
		values.Set("q", "golang")
		values.Set("page", "2")
		result := MustBindQuery[Params](t, values)
		assert.Equal(t, "golang", result.Q)
		assert.Equal(t, 2, result.Page)
	})
}

func TestMustBindForm(t *testing.T) {
	t.Parallel()

	t.Run("success returns bound value", func(t *testing.T) {
		t.Parallel()

		type Form struct {
			Username string `form:"username"`
			Active   bool   `form:"active"`
		}
		values := url.Values{}
		values.Set("username", "testuser")
		values.Set("active", "true")
		result := MustBindForm[Form](t, values)
		assert.Equal(t, "testuser", result.Username)
		assert.True(t, result.Active)
	})
}

func TestTestValidator_Validate(t *testing.T) {
	t.Parallel()

	t.Run("nil ValidateFunc returns nil", func(t *testing.T) {
		t.Parallel()

		v := NewTestValidator(nil)
		err := v.Validate("anything")
		assert.NoError(t, err)
	})

	t.Run("ValidateFunc returning nil passes", func(t *testing.T) {
		t.Parallel()

		v := NewTestValidator(func(any) error { return nil })
		err := v.Validate("anything")
		assert.NoError(t, err)
	})

	t.Run("ValidateFunc returning error fails", func(t *testing.T) {
		t.Parallel()

		v := NewTestValidator(func(any) error { return assert.AnError })
		err := v.Validate("anything")
		assert.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
	})
}

func TestAlwaysFailValidator(t *testing.T) {
	t.Parallel()

	v := AlwaysFailValidator("validation failed")
	err := v.Validate(nil)
	require.Error(t, err)
	var bindErr *BindError
	require.ErrorAs(t, err, &bindErr)
	assert.Equal(t, "validation failed", bindErr.Reason)
	assert.Equal(t, SourceUnknown, bindErr.Source)
}

func TestNeverFailValidator(t *testing.T) {
	t.Parallel()

	v := NeverFailValidator()
	err := v.Validate(nil)
	assert.NoError(t, err)
}

func TestMustBindJSON_ReaderEquivalent(t *testing.T) {
	t.Parallel()

	type User struct {
		Name string `json:"name"`
	}
	// Ensure JSON path is used (bytes)
	result := MustBindJSON[User](t, `{"name":"reader-test"}`)
	assert.Equal(t, "reader-test", result.Name)
}

func TestAssertBindError_WithFormSource(t *testing.T) {
	t.Parallel()

	type Params struct {
		Count int `form:"count"`
	}
	values := url.Values{}
	values.Set("count", "not-a-number")
	var out Params
	err := Raw(NewFormGetter(values), TagForm, &out)
	require.Error(t, err)
	bindErr := AssertBindError(t, err, "Count")
	assert.Equal(t, SourceForm, bindErr.Source)
}

func TestAssertNoBindError_WithJSON(t *testing.T) {
	t.Parallel()

	type User struct {
		Name string `json:"name"`
	}
	var out User
	err := JSONTo([]byte(`{"name":"ok"}`), &out)
	require.NoError(t, err)
	AssertNoBindError(t, err)
}
