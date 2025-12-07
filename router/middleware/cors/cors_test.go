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

package cors

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/router"
)

func TestCORS_NoCORSRequest(t *testing.T) {
	t.Parallel()
	r, err := router.New()
	require.NoError(t, err)
	r.Use(New(WithAllowAllOrigins(true)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// No Origin header means no CORS headers should be set
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"), "No CORS headers for non-CORS request")
}

func TestCORS_AllowAllOrigins(t *testing.T) {
	t.Parallel()
	r, err := router.New()
	require.NoError(t, err)
	r.Use(New(WithAllowAllOrigins(true)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_AllowedOrigins(t *testing.T) {
	t.Parallel()
	r, err := router.New()
	require.NoError(t, err)
	r.Use(New(WithAllowedOrigins("https://example.com", "https://app.example.com")))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	tests := []struct {
		name           string
		origin         string
		expectedOrigin string
	}{
		{
			name:           "allowed origin 1",
			origin:         "https://example.com",
			expectedOrigin: "https://example.com",
		},
		{
			name:           "allowed origin 2",
			origin:         "https://app.example.com",
			expectedOrigin: "https://app.example.com",
		},
		{
			name:           "disallowed origin",
			origin:         "https://evil.com",
			expectedOrigin: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Origin", tt.origin)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			got := w.Header().Get("Access-Control-Allow-Origin")
			assert.Equal(t, tt.expectedOrigin, got)
		})
	}
}

func TestCORS_AllowOriginFunc(t *testing.T) {
	t.Parallel()
	r, err := router.New()
	require.NoError(t, err)
	r.Use(New(WithAllowOriginFunc(func(origin string) bool {
		return strings.HasSuffix(origin, ".example.com")
	})))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	tests := []struct {
		name           string
		origin         string
		expectedOrigin string
	}{
		{
			name:           "subdomain allowed",
			origin:         "https://app.example.com",
			expectedOrigin: "https://app.example.com",
		},
		{
			name:           "another subdomain allowed",
			origin:         "https://api.example.com",
			expectedOrigin: "https://api.example.com",
		},
		{
			name:           "different domain disallowed",
			origin:         "https://evil.com",
			expectedOrigin: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Origin", tt.origin)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			got := w.Header().Get("Access-Control-Allow-Origin")
			assert.Equal(t, tt.expectedOrigin, got)
		})
	}
}

func TestCORS_Preflight(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(
		WithAllowedOrigins("https://example.com"),
		WithAllowedMethods("GET", "POST", "PUT"),
		WithAllowedHeaders("Content-Type", "Authorization"),
		WithMaxAge(7200),
	))
	r.POST("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})
	// Register OPTIONS handler for preflight
	r.OPTIONS("/test", func(_ *router.Context) {
		// CORS middleware will handle the response
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, Authorization", w.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "7200", w.Header().Get("Access-Control-Max-Age"))
}

func TestCORS_PreflightDisallowedOrigin(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(WithAllowedOrigins("https://example.com")))
	r.POST("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})
	// Register OPTIONS handler for preflight
	r.OPTIONS("/test", func(_ *router.Context) {
		// CORS middleware will handle the response
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should not set CORS headers for disallowed origin
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_Credentials(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(
		WithAllowedOrigins("https://example.com"),
		WithAllowCredentials(true),
	))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCORS_CredentialsWithAllOrigins(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(
		WithAllowAllOrigins(true),
		WithAllowCredentials(true),
	))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// When credentials are enabled, should return specific origin instead of *
	assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCORS_ExposedHeaders(t *testing.T) {
	t.Parallel()
	r, err := router.New()
	require.NoError(t, err)
	r.Use(New(
		WithAllowedOrigins("https://example.com"),
		WithExposedHeaders("X-Request-ID", "X-Rate-Limit"),
	))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, "X-Request-ID, X-Rate-Limit", w.Header().Get("Access-Control-Expose-Headers"))
}

func TestCORS_DefaultConfig(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New()) // Use default config
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Default config has no allowed origins, so no CORS headers
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_ActualRequest(t *testing.T) {
	t.Parallel()
	r, err := router.New()
	require.NoError(t, err)
	r.Use(New(WithAllowedOrigins("https://example.com")))
	r.POST("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "created"})
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"data":"test"}`))
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))

	// Should not have preflight headers on actual request
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Methods"))
}
