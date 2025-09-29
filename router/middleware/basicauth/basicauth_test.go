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

package basicauth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"rivaas.dev/router"
)

func TestBasicAuth(t *testing.T) {
	tests := []struct {
		name           string
		setupAuth      func() router.HandlerFunc
		authHeader     string
		expectedStatus int
		expectedBody   string
		checkHeader    bool
	}{
		{
			name: "valid credentials",
			setupAuth: func() router.HandlerFunc {
				return New(
					WithUsers(map[string]string{
						"admin": "secret",
					}),
				)
			},
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret")),
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
		{
			name: "invalid password",
			setupAuth: func() router.HandlerFunc {
				return New(
					WithUsers(map[string]string{
						"admin": "secret",
					}),
				)
			},
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:wrong")),
			expectedStatus: http.StatusUnauthorized,
			checkHeader:    true,
		},
		{
			name: "invalid username",
			setupAuth: func() router.HandlerFunc {
				return New(
					WithUsers(map[string]string{
						"admin": "secret",
					}),
				)
			},
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("nobody:secret")),
			expectedStatus: http.StatusUnauthorized,
			checkHeader:    true,
		},
		{
			name: "missing auth header",
			setupAuth: func() router.HandlerFunc {
				return New(
					WithUsers(map[string]string{
						"admin": "secret",
					}),
				)
			},
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			checkHeader:    true,
		},
		{
			name: "malformed auth header",
			setupAuth: func() router.HandlerFunc {
				return New(
					WithUsers(map[string]string{
						"admin": "secret",
					}),
				)
			},
			authHeader:     "Bearer token123",
			expectedStatus: http.StatusUnauthorized,
			checkHeader:    true,
		},
		{
			name: "invalid base64",
			setupAuth: func() router.HandlerFunc {
				return New(
					WithUsers(map[string]string{
						"admin": "secret",
					}),
				)
			},
			authHeader:     "Basic !!invalid-base64!!",
			expectedStatus: http.StatusUnauthorized,
			checkHeader:    true,
		},
		{
			name: "missing colon in credentials",
			setupAuth: func() router.HandlerFunc {
				return New(
					WithUsers(map[string]string{
						"admin": "secret",
					}),
				)
			},
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("adminonly")),
			expectedStatus: http.StatusUnauthorized,
			checkHeader:    true,
		},
		{
			name: "custom realm",
			setupAuth: func() router.HandlerFunc {
				return New(
					WithUsers(map[string]string{"user": "pass"}),
					WithRealm("Admin Area"),
				)
			},
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			checkHeader:    true,
		},
		{
			name: "multiple users",
			setupAuth: func() router.HandlerFunc {
				return New(
					WithUsers(map[string]string{
						"admin": "secret1",
						"user":  "secret2",
					}),
				)
			},
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("user:secret2")),
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := router.MustNew()
			r.Use(tt.setupAuth())
			r.GET("/test", func(c *router.Context) {
				c.String(http.StatusOK, "success")
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedBody != "" {
				assert.Equal(t, tt.expectedBody, w.Body.String())
			}

			if tt.checkHeader {
				assert.NotEmpty(t, w.Header().Get("WWW-Authenticate"))
			}
		})
	}
}

func TestBasicAuthWithValidator(t *testing.T) {
	validUsers := map[string]string{
		"admin": "password123",
		"user":  "pass456",
	}

	r := router.MustNew()
	r.Use(New(
		WithValidator(func(username, password string) bool {
			expectedPassword, exists := validUsers[username]
			return exists && password == expectedPassword
		}),
	))
	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "success")
	})

	tests := []struct {
		name           string
		credentials    string
		expectedStatus int
	}{
		{"valid credentials", "admin:password123", http.StatusOK},
		{"invalid credentials", "admin:wrong", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(tt.credentials)))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestBasicAuthSkipPaths(t *testing.T) {
	r := router.MustNew()
	r.Use(New(
		WithUsers(map[string]string{"admin": "secret"}),
		WithSkipPaths("/health", "/public"),
	))
	r.GET("/health", func(c *router.Context) {
		c.String(http.StatusOK, "healthy")
	})
	r.GET("/protected", func(c *router.Context) {
		c.String(http.StatusOK, "protected")
	})

	// Skipped path - no auth required
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Skipped path should succeed")

	// Protected path - auth required
	req = httptest.NewRequest(http.MethodGet, "/protected", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "Protected path should require auth")
}

func TestBasicAuthCustomUnauthorizedHandler(t *testing.T) {
	customCalled := false
	r := router.MustNew()
	r.Use(New(
		WithUsers(map[string]string{"admin": "secret"}),
		WithUnauthorizedHandler(func(c *router.Context) {
			customCalled = true
			c.String(http.StatusUnauthorized, "custom unauthorized")
		}),
	))
	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, customCalled, "Custom unauthorized handler should be called")
	assert.Equal(t, "custom unauthorized", w.Body.String())
}

func TestGetAuthUsername(t *testing.T) {
	r := router.MustNew()
	r.Use(New(
		WithUsers(map[string]string{"testuser": "testpass"}),
	))
	r.GET("/test", func(c *router.Context) {
		username := GetUsername(c)
		c.Stringf(http.StatusOK, "user:%s", username)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("testuser:testpass")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user:testuser", w.Body.String())
}

func TestBasicAuth_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		users          map[string]string
		credentials    string
		expectedStatus int
	}{
		{
			name:           "empty password",
			users:          map[string]string{"user": ""},
			credentials:    "user:",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "special characters",
			users:          map[string]string{"user@example.com": "p@ss:w0rd!"},
			credentials:    "user@example.com:p@ss:w0rd!",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := router.MustNew()
			r.Use(New(WithUsers(tt.users)))
			r.GET("/test", func(c *router.Context) {
				c.String(http.StatusOK, "success")
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(tt.credentials)))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}
