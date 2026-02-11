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
)

func TestCacheControl_WithPublic(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(w, req)
	c.CacheControl(WithPublic())
	assert.Contains(t, w.Header().Get("Cache-Control"), "public")
}

func TestCacheControl_WithPrivate(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(w, req)
	c.CacheControl(WithPrivate())
	assert.Contains(t, w.Header().Get("Cache-Control"), "private")
}

func TestCacheControl_WithNoStore(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(w, req)
	c.CacheControl(WithNoStore())
	assert.Contains(t, w.Header().Get("Cache-Control"), "no-store")
}

func TestCacheControl_WithNoCache(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(w, req)
	c.CacheControl(WithNoCache())
	assert.Contains(t, w.Header().Get("Cache-Control"), "no-cache")
}

func TestCacheControl_WithMaxAge(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(w, req)
	c.CacheControl(WithMaxAge(time.Minute))
	assert.Contains(t, w.Header().Get("Cache-Control"), "max-age=60")
}

func TestCacheControl_WithStaleWhileRevalidate(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(w, req)
	c.CacheControl(WithStaleWhileRevalidate(2 * time.Minute))
	assert.Contains(t, w.Header().Get("Cache-Control"), "stale-while-revalidate=120")
}

func TestCacheControl_WithStaleIfError(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(w, req)
	c.CacheControl(WithStaleIfError(5 * time.Minute))
	assert.Contains(t, w.Header().Get("Cache-Control"), "stale-if-error=300")
}

func TestCacheControl_Combined(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(w, req)
	c.CacheControl(
		WithPublic(),
		WithMaxAge(time.Minute),
		WithStaleWhileRevalidate(2*time.Minute),
	)
	cc := w.Header().Get("Cache-Control")
	assert.Contains(t, cc, "public")
	assert.Contains(t, cc, "max-age=60")
	assert.Contains(t, cc, "stale-while-revalidate=120")
}
