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

package trailingslash

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"rivaas.dev/router"
)

func TestTrailingSlash_RemovePolicy(t *testing.T) {
	r := router.MustNew()
	r.GET("/users", func(c *router.Context) {
		c.String(http.StatusOK, "users")
	})

	handler := Wrap(r, WithPolicy(PolicyRemove))

	tests := []struct {
		name             string
		url              string
		expectedStatus   int
		expectedLocation string
		expectedBody     string
	}{
		{
			name:             "redirect from /users/ to /users",
			url:              "/users/",
			expectedStatus:   http.StatusPermanentRedirect,
			expectedLocation: "/users",
		},
		{
			name:           "/users works correctly",
			url:            "/users",
			expectedStatus: http.StatusOK,
			expectedBody:   "users",
		},
		{
			name:             "query params preserved",
			url:              "/users/?page=2&sort=name",
			expectedStatus:   http.StatusPermanentRedirect,
			expectedLocation: "/users?page=2&sort=name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedLocation != "" {
				assert.Equal(t, tt.expectedLocation, w.Header().Get("Location"))
			}

			if tt.expectedBody != "" {
				assert.Equal(t, tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestTrailingSlashRootPath(t *testing.T) {
	r := router.MustNew()
	r.Use(New())

	r.GET("/", func(c *router.Context) {
		c.String(http.StatusOK, "root")
	})

	// Root path should never be redirected
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "root", w.Body.String())
}

func TestTrailingSlash_Policies(t *testing.T) {
	tests := []struct {
		name             string
		policy           Policy
		route            string
		requestURL       string
		expectedStatus   int
		expectedLocation string
		expectedBody     string
	}{
		{
			name:             "PolicyAdd - redirect to add slash",
			policy:           PolicyAdd,
			route:            "/users/",
			requestURL:       "/users",
			expectedStatus:   http.StatusPermanentRedirect,
			expectedLocation: "/users/",
		},
		{
			name:           "PolicyAdd - with slash works",
			policy:         PolicyAdd,
			route:          "/users/",
			requestURL:     "/users/",
			expectedStatus: http.StatusOK,
			expectedBody:   "users",
		},
		{
			name:           "PolicyStrict - rejects trailing slash",
			policy:         PolicyStrict,
			route:          "/users",
			requestURL:     "/users/",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "PolicyStrict - without slash works",
			policy:         PolicyStrict,
			route:          "/users",
			requestURL:     "/users",
			expectedStatus: http.StatusOK,
			expectedBody:   "users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := router.MustNew()
			if tt.policy == PolicyStrict {
				r.Use(New(WithPolicy(tt.policy)))
			}

			r.GET(tt.route, func(c *router.Context) {
				c.String(http.StatusOK, "users")
			})

			var handler http.Handler = r
			if tt.policy == PolicyAdd {
				handler = Wrap(r, WithPolicy(PolicyAdd))
			}

			req := httptest.NewRequest("GET", tt.requestURL, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedLocation != "" {
				assert.Equal(t, tt.expectedLocation, w.Header().Get("Location"))
			}

			if tt.expectedBody != "" {
				assert.Equal(t, tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestTrailingSlashTrimSuffixNotTrimRight(t *testing.T) {
	r := router.MustNew()
	r.GET("/users", func(c *router.Context) {
		c.String(http.StatusOK, "users")
	})

	handler := Wrap(r, WithPolicy(PolicyRemove))

	// Test that multiple slashes don't collapse incorrectly
	// /users// should redirect to /users/ (then to /users on second request)
	// But we only remove one slash, so /users// → /users/ → 404 (since route is /users)
	req := httptest.NewRequest("GET", "/users//", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Should redirect to /users/ (one slash removed)
	assert.Equal(t, http.StatusPermanentRedirect, w.Code)

	location := w.Header().Get("Location")
	// TrimSuffix removes exactly one slash, so /users// → /users/
	assert.Equal(t, "/users/", location)
}

func TestTrailingSlashPreservesMethod(t *testing.T) {
	r := router.MustNew()
	r.POST("/users", func(c *router.Context) {
		c.String(http.StatusOK, "created")
	})

	handler := Wrap(r, WithPolicy(PolicyRemove))

	// POST to /users/ should redirect with 308 (preserves method)
	req := httptest.NewRequest("POST", "/users/", strings.NewReader("data"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusPermanentRedirect, w.Code)

	// 308 preserves method, so client should retry POST to /users
	location := w.Header().Get("Location")
	assert.Equal(t, "/users", location)
}
