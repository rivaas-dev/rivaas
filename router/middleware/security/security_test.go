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

package security

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"rivaas.dev/router"
	"rivaas.dev/router/middleware/requestid"
)

func TestSecurity_DefaultHeaders(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Check all default security headers
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "default-src 'self'", w.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
}

func TestSecurity_CustomFrameOptions(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(WithFrameOptions("SAMEORIGIN")))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, "SAMEORIGIN", w.Header().Get("X-Frame-Options"))
}

func TestSecurity_HSTS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		maxAge            int
		includeSubDomains bool
		preload           bool
		useTLS            bool
		expectHSTS        bool
		checkContains     []string
	}{
		{
			name:              "HSTS on HTTPS with full options",
			maxAge:            31536000,
			includeSubDomains: true,
			preload:           true,
			useTLS:            true,
			expectHSTS:        true,
			checkContains:     []string{"max-age=31536000", "includeSubDomains", "preload"},
		},
		{
			name:       "no HSTS on HTTP",
			maxAge:     31536000,
			useTLS:     false,
			expectHSTS: false,
		},
		{
			name:       "disabled HSTS",
			maxAge:     0,
			useTLS:     true,
			expectHSTS: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := router.MustNew()
			r.Use(New(WithHSTS(tt.maxAge, tt.includeSubDomains, tt.preload)))
			r.GET("/test", func(c *router.Context) {
				c.JSON(http.StatusOK, map[string]string{"message": "ok"})
			})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
			if tt.useTLS {
				req.TLS = &tls.ConnectionState{}
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			hsts := w.Header().Get("Strict-Transport-Security")
			if tt.expectHSTS {
				assert.NotEmpty(t, hsts)
				for _, expected := range tt.checkContains {
					assert.Contains(t, hsts, expected)
				}
			} else {
				assert.Empty(t, hsts)
			}
		})
	}
}

func TestSecurity_CustomCSP(t *testing.T) {
	t.Parallel()
	customCSP := "default-src 'self'; script-src 'self' https://cdn.example.com; style-src 'self' 'unsafe-inline'"

	r := router.MustNew()
	r.Use(New(WithContentSecurityPolicy(customCSP)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, customCSP, w.Header().Get("Content-Security-Policy"))
}

func TestSecurity_ReferrerPolicy(t *testing.T) {
	t.Parallel()
	policies := []string{
		"no-referrer",
		"same-origin",
		"strict-origin",
		"strict-origin-when-cross-origin",
	}

	for _, policy := range policies {
		t.Run(policy, func(t *testing.T) {
			t.Parallel()
			r := router.MustNew()
			r.Use(New(WithReferrerPolicy(policy)))
			r.GET("/test", func(c *router.Context) {
				c.JSON(http.StatusOK, map[string]string{"message": "ok"})
			})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, policy, w.Header().Get("Referrer-Policy"))
		})
	}
}

func TestSecurity_PermissionsPolicy(t *testing.T) {
	t.Parallel()
	policy := "geolocation=(), microphone=(), camera=()"

	r := router.MustNew()
	r.Use(New(WithPermissionsPolicy(policy)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, policy, w.Header().Get("Permissions-Policy"))
}

func TestSecurity_CustomHeaders(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(
		WithCustomHeader("X-Custom-Security", "custom-value"),
		WithCustomHeader("X-Another-Header", "another-value"),
	))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, "custom-value", w.Header().Get("X-Custom-Security"))
	assert.Equal(t, "another-value", w.Header().Get("X-Another-Header"))
}

func TestSecurity_DisableContentTypeNosniff(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(WithContentTypeNosniff(false)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("X-Content-Type-Options"), "X-Content-Type-Options should not be set when disabled")
}

func TestSecurity_MultipleOptions(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(
		WithFrameOptions("SAMEORIGIN"),
		WithContentTypeNosniff(true),
		WithXSSProtection("1; mode=block"),
		WithContentSecurityPolicy("default-src 'self'; script-src 'self' https://cdn.example.com"),
		WithReferrerPolicy("same-origin"),
		WithPermissionsPolicy("geolocation=()"),
		WithCustomHeader("X-Custom", "value"),
	))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Verify all headers are set
	assert.Equal(t, "SAMEORIGIN", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.NotEmpty(t, w.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "same-origin", w.Header().Get("Referrer-Policy"))
	assert.Equal(t, "geolocation=()", w.Header().Get("Permissions-Policy"))
	assert.Equal(t, "value", w.Header().Get("X-Custom"))
}

func TestSecureHeaders_Convenience(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should have default security headers
	assert.NotEmpty(t, w.Header().Get("X-Frame-Options"))
	assert.NotEmpty(t, w.Header().Get("X-Content-Type-Options"))
}

func TestDevelopmentSecurity(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(DevelopmentPreset()))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should have relaxed CSP
	csp := w.Header().Get("Content-Security-Policy")
	assert.Contains(t, csp, "unsafe-inline")
	assert.Contains(t, csp, "unsafe-eval")

	// Should not have HSTS
	assert.Empty(t, w.Header().Get("Strict-Transport-Security"))

	// Should have SAMEORIGIN instead of DENY
	assert.Equal(t, "SAMEORIGIN", w.Header().Get("X-Frame-Options"))
}

func TestNoSecurityHeaders(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(NoSecurityHeaders()))
	r.GET("/test", func(c *router.Context) {
		// Manually set a security header
		c.Response.Header().Set("X-Frame-Options", "DENY")
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Manually set X-Frame-Options should remain
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))

	// Middleware should not have set X-Content-Type-Options
	assert.Empty(t, w.Header().Get("X-Content-Type-Options"))
}

func TestSecurity_CombinedWithOtherMiddleware(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(requestid.New())
	r.Use(New())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should have both request ID and security headers
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
	assert.NotEmpty(t, w.Header().Get("X-Frame-Options"))
}

func TestSecurity_EmptyOptions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		option     func() Option
		headerName string
	}{
		{
			name:       "empty CSP",
			option:     func() Option { return WithContentSecurityPolicy("") },
			headerName: "Content-Security-Policy",
		},
		{
			name:       "empty frame options",
			option:     func() Option { return WithFrameOptions("") },
			headerName: "X-Frame-Options",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := router.MustNew()
			r.Use(New(tt.option()))
			r.GET("/test", func(c *router.Context) {
				c.JSON(http.StatusOK, map[string]string{"message": "ok"})
			})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Empty(t, w.Header().Get(tt.headerName))
		})
	}
}
