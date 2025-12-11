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

package methodoverride

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"rivaas.dev/router"
)

func TestMethodOverride_Basic(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		originalMethod   string
		overrideHeader   string
		overrideQuery    string
		expectedMethod   string
		expectedOriginal string
	}{
		{
			name:             "header override",
			originalMethod:   "POST",
			overrideHeader:   "DELETE",
			expectedMethod:   "DELETE",
			expectedOriginal: "POST",
		},
		{
			name:             "query param override",
			originalMethod:   "POST",
			overrideQuery:    "PATCH",
			expectedMethod:   "PATCH",
			expectedOriginal: "POST",
		},
		{
			name:             "header takes precedence over query",
			originalMethod:   "POST",
			overrideHeader:   "PUT",
			overrideQuery:    "PATCH",
			expectedMethod:   "PUT",
			expectedOriginal: "POST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := New()

			url := "/test"
			if tt.overrideQuery != "" {
				url += "?_method=" + tt.overrideQuery
			}

			req := httptest.NewRequest(tt.originalMethod, url, nil)
			if tt.overrideHeader != "" {
				req.Header.Set("X-Http-Method-Override", tt.overrideHeader)
			}
			w := httptest.NewRecorder()

			c := router.NewContext(w, req)
			handler(c)

			assert.Equal(t, tt.expectedMethod, c.Request.Method)
			assert.Equal(t, tt.expectedOriginal, GetOriginalMethod(c))
		})
	}
}

func TestMethodOverride_OnlyOnFiltering(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		originalMethod string
		override       string
		shouldOverride bool
		expectedMethod string
	}{
		{
			name:           "GET request not in OnlyOn list",
			originalMethod: "GET",
			override:       "PUT",
			shouldOverride: false,
			expectedMethod: "GET",
		},
		{
			name:           "POST request in OnlyOn list",
			originalMethod: "POST",
			override:       "PUT",
			shouldOverride: true,
			expectedMethod: "PUT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := New(WithOnlyOn("POST"))
			req := httptest.NewRequest(tt.originalMethod, "/test", nil)
			req.Header.Set("X-Http-Method-Override", tt.override)
			w := httptest.NewRecorder()

			c := router.NewContext(w, req)
			handler(c)

			assert.Equal(t, tt.expectedMethod, c.Request.Method)
		})
	}
}

func TestMethodOverride_AllowList(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		override       string
		shouldOverride bool
		expectedMethod string
	}{
		{
			name:           "PATCH not in allow list",
			override:       "PATCH",
			shouldOverride: false,
			expectedMethod: "POST",
		},
		{
			name:           "PUT in allow list",
			override:       "PUT",
			shouldOverride: true,
			expectedMethod: "PUT",
		},
		{
			name:           "DELETE in allow list",
			override:       "DELETE",
			shouldOverride: true,
			expectedMethod: "DELETE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := New(WithAllow("PUT", "DELETE"))
			req := httptest.NewRequest(http.MethodPost, "/test", nil)
			req.Header.Set("X-Http-Method-Override", tt.override)
			w := httptest.NewRecorder()

			c := router.NewContext(w, req)
			handler(c)

			assert.Equal(t, tt.expectedMethod, c.Request.Method)
		})
	}
}

func TestMethodOverride_CaseInsensitive(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		override       string
		expectedMethod string
	}{
		{"lowercase", "delete", "DELETE"},
		{"uppercase", "DELETE", "DELETE"},
		{"mixed case", "DeLeTe", "DELETE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := New()
			req := httptest.NewRequest(http.MethodPost, "/test", nil)
			req.Header.Set("X-Http-Method-Override", tt.override)
			w := httptest.NewRecorder()

			c := router.NewContext(w, req)
			handler(c)

			assert.Equal(t, tt.expectedMethod, c.Request.Method)
		})
	}
}

func TestMethodOverride_RespectBody(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		contentLength  int64
		expectedMethod string
	}{
		{
			name:           "POST without body - should not override",
			contentLength:  0,
			expectedMethod: "POST",
		},
		{
			name:           "POST with body - should override",
			contentLength:  10,
			expectedMethod: "PUT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := New(WithRespectBody(true))
			req := httptest.NewRequest(http.MethodPost, "/test", nil)
			req.ContentLength = tt.contentLength
			req.Header.Set("X-Http-Method-Override", "PUT")
			w := httptest.NewRecorder()

			c := router.NewContext(w, req)
			handler(c)

			assert.Equal(t, tt.expectedMethod, c.Request.Method)
		})
	}
}

func TestMethodOverride_CSRFRequired(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		csrfVerified   bool
		expectedMethod string
	}{
		{
			name:           "without CSRF verification - should not override",
			csrfVerified:   false,
			expectedMethod: "POST",
		},
		{
			name:           "with CSRF verification - should override",
			csrfVerified:   true,
			expectedMethod: "DELETE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := New(WithRequireCSRFToken(true))
			req := httptest.NewRequest(http.MethodPost, "/test", nil)
			req.Header.Set("X-Http-Method-Override", "DELETE")

			if tt.csrfVerified {
				req = req.WithContext(context.WithValue(req.Context(), CSRFVerifiedKey, true))
			}

			w := httptest.NewRecorder()
			c := router.NewContext(w, req)
			handler(c)

			assert.Equal(t, tt.expectedMethod, c.Request.Method)
		})
	}
}

func TestMethodOverride_CustomHeader(t *testing.T) {
	t.Parallel()
	handler := New(WithHeader("X-HTTP-Method"))
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-Http-Method", "DELETE")
	w := httptest.NewRecorder()

	c := router.NewContext(w, req)
	handler(c)

	assert.Equal(t, "DELETE", c.Request.Method)
}

func TestMethodOverride_DisabledQueryParam(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		url            string
		header         string
		expectedMethod string
	}{
		{
			name:           "query param ignored when disabled",
			url:            "/test?_method=DELETE",
			header:         "",
			expectedMethod: "POST",
		},
		{
			name:           "header still works when query disabled",
			url:            "/test",
			header:         "DELETE",
			expectedMethod: "DELETE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := New(WithQueryParam(""))
			req := httptest.NewRequest(http.MethodPost, tt.url, nil)
			if tt.header != "" {
				req.Header.Set("X-Http-Method-Override", tt.header)
			}
			w := httptest.NewRecorder()

			c := router.NewContext(w, req)
			handler(c)

			assert.Equal(t, tt.expectedMethod, c.Request.Method)
		})
	}
}

func TestMethodOverride_EdgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		override       string
		expectedMethod string
	}{
		{
			name:           "empty override",
			override:       "",
			expectedMethod: "POST",
		},
		{
			name:           "whitespace trimmed",
			override:       "  DELETE  ",
			expectedMethod: "DELETE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := New()
			req := httptest.NewRequest(http.MethodPost, "/test", nil)
			req.Header.Set("X-Http-Method-Override", tt.override)
			w := httptest.NewRecorder()

			c := router.NewContext(w, req)
			handler(c)

			assert.Equal(t, tt.expectedMethod, c.Request.Method)
		})
	}
}

func TestGetOriginalMethod(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		originalMethod string
		override       string
		expectedMethod string
		expectedOrig   string
	}{
		{
			name:           "overridden method",
			originalMethod: "POST",
			override:       "DELETE",
			expectedMethod: "DELETE",
			expectedOrig:   "POST",
		},
		{
			name:           "no override",
			originalMethod: "GET",
			override:       "",
			expectedMethod: "GET",
			expectedOrig:   "GET",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := New()
			req := httptest.NewRequest(tt.originalMethod, "/test", nil)
			if tt.override != "" {
				req.Header.Set("X-Http-Method-Override", tt.override)
			}
			w := httptest.NewRecorder()

			c := router.NewContext(w, req)
			handler(c)

			assert.Equal(t, tt.expectedMethod, c.Request.Method)
			assert.Equal(t, tt.expectedOrig, GetOriginalMethod(c))
		})
	}
}

func TestMethodOverride_DefaultConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		url            string
		header         string
		expectedMethod string
	}{
		{
			name:           "default header",
			url:            "/test",
			header:         "DELETE",
			expectedMethod: "DELETE",
		},
		{
			name:           "default query param",
			url:            "/test?_method=DELETE",
			header:         "",
			expectedMethod: "DELETE",
		},
		{
			name:           "default allow list - PUT",
			url:            "/put",
			header:         "PUT",
			expectedMethod: "PUT",
		},
		{
			name:           "default allow list - PATCH",
			url:            "/patch",
			header:         "PATCH",
			expectedMethod: "PATCH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := New()
			req := httptest.NewRequest(http.MethodPost, tt.url, nil)
			if tt.header != "" {
				req.Header.Set("X-Http-Method-Override", tt.header)
			}
			w := httptest.NewRecorder()

			c := router.NewContext(w, req)
			handler(c)

			assert.Equal(t, tt.expectedMethod, c.Request.Method)
		})
	}
}
