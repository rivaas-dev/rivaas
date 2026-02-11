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

package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ctxWithParam creates a context with a single path param for testing.
func ctxWithParam(t *testing.T, key, value string) *Context {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(w, req)
	c.SetParam(0, key, value)
	c.SetParamCount(1)
	return c
}

// ctxWithQuery creates a context with the given raw query string.
func ctxWithQuery(t *testing.T, rawQuery string) *Context {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?"+rawQuery, nil)
	return NewContext(w, req)
}

func TestContext_ParamInt(t *testing.T) {
	t.Parallel()

	t.Run("valid int", func(t *testing.T) {
		t.Parallel()
		c := ctxWithParam(t, "id", "42")
		val, err := c.ParamInt("id")
		require.NoError(t, err)
		assert.Equal(t, 42, val)
	})
	t.Run("missing param returns error", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		c := NewContext(w, req)
		c.SetParamCount(0)
		_, err := c.ParamInt("id")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrParamMissing)
	})
	t.Run("invalid int returns error", func(t *testing.T) {
		t.Parallel()
		c := ctxWithParam(t, "id", "abc")
		_, err := c.ParamInt("id")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrParamInvalid)
	})
}

func TestContext_ParamInt64(t *testing.T) {
	t.Parallel()

	c := ctxWithParam(t, "n", "9223372036854775807")
	val, err := c.ParamInt64("n")
	require.NoError(t, err)
	assert.Equal(t, int64(9223372036854775807), val)
}

func TestContext_ParamUint(t *testing.T) {
	t.Parallel()

	c := ctxWithParam(t, "n", "100")
	val, err := c.ParamUint("n")
	require.NoError(t, err)
	assert.Equal(t, uint(100), val)
}

func TestContext_ParamUint64(t *testing.T) {
	t.Parallel()

	c := ctxWithParam(t, "n", "18446744073709551615")
	val, err := c.ParamUint64("n")
	require.NoError(t, err)
	assert.Equal(t, uint64(18446744073709551615), val)
}

func TestContext_ParamFloat64(t *testing.T) {
	t.Parallel()

	c := ctxWithParam(t, "x", "3.14")
	val, err := c.ParamFloat64("x")
	require.NoError(t, err)
	assert.InDelta(t, 3.14, val, 1e-9)
}

func TestContext_ParamUUID(t *testing.T) {
	t.Parallel()

	t.Run("valid UUID", func(t *testing.T) {
		t.Parallel()
		c := ctxWithParam(t, "id", "550e8400-e29b-41d4-a716-446655440000")
		uuid, err := c.ParamUUID("id")
		require.NoError(t, err)
		assert.NotEqual(t, [16]byte{}, uuid)
	})
	t.Run("missing param returns error", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		c := NewContext(w, req)
		c.SetParamCount(0)
		_, err := c.ParamUUID("id")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrParamMissing)
	})
	t.Run("invalid length returns error", func(t *testing.T) {
		t.Parallel()
		c := ctxWithParam(t, "id", "short")
		_, err := c.ParamUUID("id")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrParamInvalid)
	})
	t.Run("invalid format returns error", func(t *testing.T) {
		t.Parallel()
		c := ctxWithParam(t, "id", "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx") // no hyphens
		_, err := c.ParamUUID("id")
		require.Error(t, err)
	})
}

func TestContext_ParamTime(t *testing.T) {
	t.Parallel()

	c := ctxWithParam(t, "date", "2025-01-15")
	val, err := c.ParamTime("date", "2006-01-02")
	require.NoError(t, err)
	assert.Equal(t, 2025, val.Year())
	assert.Equal(t, time.January, val.Month())
	assert.Equal(t, 15, val.Day())
}

func TestContext_ParamIntRange(t *testing.T) {
	t.Parallel()

	t.Run("in range", func(t *testing.T) {
		t.Parallel()
		c := ctxWithParam(t, "page", "2")
		val, err := c.ParamIntRange("page", 1, 10)
		require.NoError(t, err)
		assert.Equal(t, 2, val)
	})
	t.Run("out of range returns error", func(t *testing.T) {
		t.Parallel()
		c := ctxWithParam(t, "page", "99")
		_, err := c.ParamIntRange("page", 1, 10)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrParamInvalid)
	})
}

func TestContext_ParamStringMaxLen(t *testing.T) {
	t.Parallel()

	t.Run("within limit", func(t *testing.T) {
		t.Parallel()
		c := ctxWithParam(t, "name", "ab")
		s, err := c.ParamStringMaxLen("name", 10)
		require.NoError(t, err)
		assert.Equal(t, "ab", s)
	})
	t.Run("exceeds max returns error", func(t *testing.T) {
		t.Parallel()
		c := ctxWithParam(t, "name", "toolong")
		_, err := c.ParamStringMaxLen("name", 3)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrParamInvalid)
	})
}

func TestContext_ParamEnum(t *testing.T) {
	t.Parallel()

	t.Run("allowed value", func(t *testing.T) {
		t.Parallel()
		c := ctxWithParam(t, "sort", "asc")
		s, err := c.ParamEnum("sort", "asc", "desc")
		require.NoError(t, err)
		assert.Equal(t, "asc", s)
	})
	t.Run("not in allowed list returns error", func(t *testing.T) {
		t.Parallel()
		c := ctxWithParam(t, "sort", "invalid")
		_, err := c.ParamEnum("sort", "asc", "desc")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrParamInvalid)
	})
}

func TestContext_QueryInt(t *testing.T) {
	t.Parallel()

	c := ctxWithQuery(t, "page=3")
	assert.Equal(t, 3, c.QueryInt("page", 1))
	assert.Equal(t, 1, c.QueryInt("missing", 1))
}

func TestContext_QueryInt64(t *testing.T) {
	t.Parallel()

	c := ctxWithQuery(t, "n=999999")
	assert.Equal(t, int64(999999), c.QueryInt64("n", 0))
	assert.Equal(t, int64(5), c.QueryInt64("missing", 5))
}

func TestContext_QueryBool(t *testing.T) {
	t.Parallel()

	t.Run("true values", func(t *testing.T) {
		t.Parallel()
		for _, q := range []string{"true", "1", "yes", "on"} {
			c := ctxWithQuery(t, "x="+q)
			assert.True(t, c.QueryBool("x", false), "query x=%s", q)
		}
	})
	t.Run("false values", func(t *testing.T) {
		t.Parallel()
		for _, q := range []string{"false", "0", "no", "off"} {
			c := ctxWithQuery(t, "x="+q)
			assert.False(t, c.QueryBool("x", true), "query x=%s", q)
		}
	})
	t.Run("missing returns default", func(t *testing.T) {
		t.Parallel()
		c := ctxWithQuery(t, "")
		assert.True(t, c.QueryBool("x", true))
		assert.False(t, c.QueryBool("x", false))
	})
}

func TestContext_QueryFloat64(t *testing.T) {
	t.Parallel()

	c := ctxWithQuery(t, "ratio=2.5")
	assert.InDelta(t, 2.5, c.QueryFloat64("ratio", 0), 1e-9)
	assert.Equal(t, 1.0, c.QueryFloat64("missing", 1.0))
}

func TestContext_QueryDuration(t *testing.T) {
	t.Parallel()

	c := ctxWithQuery(t, "timeout=30s")
	assert.Equal(t, 30*time.Second, c.QueryDuration("timeout", time.Second))
	assert.Equal(t, time.Minute, c.QueryDuration("missing", time.Minute))
}

func TestContext_QueryTime(t *testing.T) {
	t.Parallel()

	c := ctxWithQuery(t, "date=2025-06-01")
	def := time.Time{}
	got, ok := c.QueryTime("date", "2006-01-02", def)
	require.True(t, ok)
	assert.Equal(t, 2025, got.Year())
	_, ok = c.QueryTime("missing", "2006-01-02", def)
	assert.False(t, ok)
}

func TestContext_QueryStrings(t *testing.T) {
	t.Parallel()

	c := ctxWithQuery(t, "tags=go,rust,python")
	got := c.QueryStrings("tags")
	assert.Equal(t, []string{"go", "rust", "python"}, got)
	assert.Nil(t, c.QueryStrings("missing"))
}

func TestContext_QueryInts(t *testing.T) {
	t.Parallel()

	t.Run("valid comma-separated ints", func(t *testing.T) {
		t.Parallel()
		c := ctxWithQuery(t, "ids=1,2,3")
		got, err := c.QueryInts("ids")
		require.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, got)
	})
	t.Run("missing returns nil nil", func(t *testing.T) {
		t.Parallel()
		c := ctxWithQuery(t, "")
		got, err := c.QueryInts("ids")
		require.NoError(t, err)
		assert.Nil(t, got)
	})
	t.Run("invalid int returns error", func(t *testing.T) {
		t.Parallel()
		c := ctxWithQuery(t, "ids=1,foo,3")
		_, err := c.QueryInts("ids")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrQueryInvalidInteger)
	})
}
