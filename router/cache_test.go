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

func TestETag_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		etag ETag
		want string
	}{
		{
			name: "empty value returns empty string",
			etag: ETag{},
			want: "",
		},
		{
			name: "weak etag format",
			etag: ETag{Value: "abc", Weak: true},
			want: `W/"abc"`,
		},
		{
			name: "strong etag format",
			etag: ETag{Value: "xyz", Weak: false},
			want: `"xyz"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.etag.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWeakETagFromBytes(t *testing.T) {
	t.Parallel()

	t.Run("empty bytes returns empty etag", func(t *testing.T) {
		t.Parallel()
		got := WeakETagFromBytes(nil)
		assert.Equal(t, ETag{}, got)
		got = WeakETagFromBytes([]byte{})
		assert.Equal(t, ETag{}, got)
	})
	t.Run("non-empty bytes returns weak etag", func(t *testing.T) {
		t.Parallel()
		got := WeakETagFromBytes([]byte("hello"))
		require.NotEmpty(t, got.Value)
		assert.True(t, got.Weak)
	})
}

func TestStrongETagFromBytes(t *testing.T) {
	t.Parallel()

	t.Run("empty bytes returns empty etag", func(t *testing.T) {
		t.Parallel()
		got := StrongETagFromBytes(nil)
		assert.Equal(t, ETag{}, got)
	})
	t.Run("non-empty bytes returns strong etag", func(t *testing.T) {
		t.Parallel()
		got := StrongETagFromBytes([]byte("data"))
		require.NotEmpty(t, got.Value)
		assert.False(t, got.Weak)
	})
}

func TestWeakETagFromString(t *testing.T) {
	t.Parallel()

	t.Run("empty string returns empty etag", func(t *testing.T) {
		t.Parallel()
		got := WeakETagFromString("")
		assert.Equal(t, ETag{}, got)
	})
	t.Run("non-empty string returns weak etag", func(t *testing.T) {
		t.Parallel()
		got := WeakETagFromString("content")
		require.NotEmpty(t, got.Value)
		assert.True(t, got.Weak)
	})
}

func TestStrongETagFromString(t *testing.T) {
	t.Parallel()

	t.Run("empty string returns empty etag", func(t *testing.T) {
		t.Parallel()
		got := StrongETagFromString("")
		assert.Equal(t, ETag{}, got)
	})
	t.Run("non-empty string returns strong etag", func(t *testing.T) {
		t.Parallel()
		got := StrongETagFromString("content")
		require.NotEmpty(t, got.Value)
		assert.False(t, got.Weak)
	})
}

func TestContext_SetETag(t *testing.T) {
	t.Parallel()

	t.Run("empty etag value does not set header", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		c := NewContext(w, req)
		c.SetETag(ETag{})
		assert.Empty(t, w.Header().Get("ETag"))
	})
	t.Run("non-empty etag sets header", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		c := NewContext(w, req)
		c.SetETag(ETag{Value: "abc", Weak: true})
		assert.Equal(t, `W/"abc"`, w.Header().Get("ETag"))
	})
}

func TestContext_SetLastModified(t *testing.T) {
	t.Parallel()

	t.Run("zero time does not set header", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		c := NewContext(w, req)
		c.SetLastModified(time.Time{})
		assert.Empty(t, w.Header().Get("Last-Modified"))
	})
	t.Run("non-zero time sets header", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		c := NewContext(w, req)
		ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
		c.SetLastModified(ts)
		assert.NotEmpty(t, w.Header().Get("Last-Modified"))
	})
}

func TestContext_HandleConditionals(t *testing.T) {
	t.Parallel()

	etag := WeakETagFromBytes([]byte("body"))
	lastMod := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	t.Run("GET with If-None-Match matching returns 304", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("If-None-Match", etag.String())
		c := NewContext(w, req)
		handled := c.HandleConditionals(CondOpts{ETag: &etag})
		require.True(t, handled)
		assert.Equal(t, http.StatusNotModified, w.Code)
	})

	t.Run("GET with If-None-Match wildcard returns 304", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("If-None-Match", "*")
		c := NewContext(w, req)
		handled := c.HandleConditionals(CondOpts{ETag: &etag})
		require.True(t, handled)
		assert.Equal(t, http.StatusNotModified, w.Code)
	})

	t.Run("GET with If-Modified-Since at or after last modified returns 304", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		// Client says "if modified since 13:00"; resource is 12:00 -> not modified since -> 304
		req.Header.Set("If-Modified-Since", lastMod.Add(time.Hour).UTC().Format(http.TimeFormat))
		c := NewContext(w, req)
		handled := c.HandleConditionals(CondOpts{LastModified: &lastMod})
		require.True(t, handled)
		assert.Equal(t, http.StatusNotModified, w.Code)
	})

	t.Run("PUT with If-Match not matching returns 412", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/", nil)
		req.Header.Set("If-Match", `"other-etag"`)
		c := NewContext(w, req)
		handled := c.HandleConditionals(CondOpts{ETag: &etag})
		require.True(t, handled)
		assert.Equal(t, http.StatusPreconditionFailed, w.Code)
	})

	t.Run("PUT with If-Unmodified-Since before last modified returns 412", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/", nil)
		// Client says "only if not modified since 11:00"; resource is 12:00 -> 412
		req.Header.Set("If-Unmodified-Since", lastMod.Add(-time.Hour).UTC().Format(http.TimeFormat))
		c := NewContext(w, req)
		handled := c.HandleConditionals(CondOpts{LastModified: &lastMod})
		require.True(t, handled)
		assert.Equal(t, http.StatusPreconditionFailed, w.Code)
	})

	t.Run("no conditional headers returns false", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		c := NewContext(w, req)
		handled := c.HandleConditionals(CondOpts{ETag: &etag, LastModified: &lastMod})
		assert.False(t, handled)
	})
}

func TestContext_IfNoneMatch(t *testing.T) {
	t.Parallel()

	etag := WeakETagFromBytes([]byte("x"))

	t.Run("GET with matching If-None-Match returns true and 304", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("If-None-Match", etag.String())
		c := NewContext(w, req)
		ok := c.IfNoneMatch(etag)
		require.True(t, ok)
		assert.Equal(t, http.StatusNotModified, w.Code)
	})
	t.Run("POST returns false", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("If-None-Match", etag.String())
		c := NewContext(w, req)
		ok := c.IfNoneMatch(etag)
		assert.False(t, ok)
	})
}

func TestContext_IfModifiedSince(t *testing.T) {
	t.Parallel()

	ts := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	t.Run("GET with If-Modified-Since at or after last modified returns true and 304", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		// Client says "if modified since 13:00"; resource is 12:00 -> not modified -> 304
		req.Header.Set("If-Modified-Since", ts.Add(time.Hour).UTC().Format(http.TimeFormat))
		c := NewContext(w, req)
		ok := c.IfModifiedSince(ts)
		require.True(t, ok)
		assert.Equal(t, http.StatusNotModified, w.Code)
	})
	t.Run("POST returns false", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		c := NewContext(w, req)
		ok := c.IfModifiedSince(ts)
		assert.False(t, ok)
	})
}

func TestContext_IfMatch(t *testing.T) {
	t.Parallel()

	etag := WeakETagFromBytes([]byte("y"))

	t.Run("PUT with matching If-Match returns true", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/", nil)
		req.Header.Set("If-Match", etag.String())
		c := NewContext(w, req)
		ok := c.IfMatch(etag)
		require.True(t, ok)
	})
	t.Run("PUT with non-matching If-Match sends 412", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/", nil)
		req.Header.Set("If-Match", `"other"`)
		c := NewContext(w, req)
		_ = c.IfMatch(etag)
		assert.Equal(t, http.StatusPreconditionFailed, w.Code)
	})
	t.Run("GET returns true without checking", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		c := NewContext(w, req)
		ok := c.IfMatch(etag)
		assert.True(t, ok)
	})
}

func TestContext_IfUnmodifiedSince(t *testing.T) {
	t.Parallel()

	ts := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	t.Run("PUT with If-Unmodified-Since in past sends 412", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/", nil)
		req.Header.Set("If-Unmodified-Since", ts.Add(-time.Hour).UTC().Format(http.TimeFormat))
		c := NewContext(w, req)
		_ = c.IfUnmodifiedSince(ts)
		assert.Equal(t, http.StatusPreconditionFailed, w.Code)
	})
	t.Run("PUT with no header returns true", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/", nil)
		c := NewContext(w, req)
		ok := c.IfUnmodifiedSince(ts)
		assert.True(t, ok)
	})
}

func TestContext_AddVary(t *testing.T) {
	t.Parallel()

	t.Run("adds single field", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		c := NewContext(w, req)
		c.AddVary("Accept")
		assert.Equal(t, "Accept", w.Header().Get("Vary"))
	})
	t.Run("adds multiple fields deduplicated", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		c := NewContext(w, req)
		c.AddVary("Accept", "Accept-Encoding")
		vary := w.Header().Get("Vary")
		assert.Contains(t, vary, "Accept")
		assert.Contains(t, vary, "Accept-Encoding")
	})
	t.Run("empty fields no-op", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		c := NewContext(w, req)
		c.AddVary()
		assert.Empty(t, w.Header().Get("Vary"))
	})
}
