package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rivaas-dev/rivaas/router"
)

func TestSecurity_DefaultHeaders(t *testing.T) {
	r := router.New()
	r.Use(Security())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Check all default security headers
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("Expected X-Frame-Options: DENY, got %s", w.Header().Get("X-Frame-Options"))
	}

	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("Expected X-Content-Type-Options: nosniff")
	}

	if w.Header().Get("X-XSS-Protection") != "1; mode=block" {
		t.Error("Expected X-XSS-Protection: 1; mode=block")
	}

	if w.Header().Get("Content-Security-Policy") != "default-src 'self'" {
		t.Error("Expected Content-Security-Policy: default-src 'self'")
	}

	if w.Header().Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
		t.Error("Expected Referrer-Policy: strict-origin-when-cross-origin")
	}
}

func TestSecurity_CustomFrameOptions(t *testing.T) {
	r := router.New()
	r.Use(Security(WithFrameOptions("SAMEORIGIN")))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Errorf("Expected X-Frame-Options: SAMEORIGIN, got %s", w.Header().Get("X-Frame-Options"))
	}
}

func TestSecurity_HSTS_HTTPSOnly(t *testing.T) {
	r := router.New()
	r.Use(Security(WithHSTS(31536000, true, true)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	// Test without TLS (HTTP)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS should not be set on HTTP requests")
	}

	// Test with TLS (HTTPS)
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.TLS = &tls.ConnectionState{} // Simulate HTTPS
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	hsts := w.Header().Get("Strict-Transport-Security")
	if !strings.Contains(hsts, "max-age=31536000") {
		t.Error("HSTS should contain max-age")
	}

	if !strings.Contains(hsts, "includeSubDomains") {
		t.Error("HSTS should contain includeSubDomains")
	}

	if !strings.Contains(hsts, "preload") {
		t.Error("HSTS should contain preload")
	}
}

func TestSecurity_DisableHSTS(t *testing.T) {
	r := router.New()
	r.Use(Security(WithHSTS(0, false, false)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS should not be set when maxAge is 0")
	}
}

func TestSecurity_CustomCSP(t *testing.T) {
	customCSP := "default-src 'self'; script-src 'self' https://cdn.example.com; style-src 'self' 'unsafe-inline'"

	r := router.New()
	r.Use(Security(WithContentSecurityPolicy(customCSP)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Header().Get("Content-Security-Policy") != customCSP {
		t.Errorf("Expected custom CSP, got %s", w.Header().Get("Content-Security-Policy"))
	}
}

func TestSecurity_ReferrerPolicy(t *testing.T) {
	policies := []string{
		"no-referrer",
		"same-origin",
		"strict-origin",
		"strict-origin-when-cross-origin",
	}

	for _, policy := range policies {
		t.Run(policy, func(t *testing.T) {
			r := router.New()
			r.Use(Security(WithReferrerPolicy(policy)))
			r.GET("/test", func(c *router.Context) {
				c.JSON(http.StatusOK, map[string]string{"message": "ok"})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Header().Get("Referrer-Policy") != policy {
				t.Errorf("Expected Referrer-Policy: %s, got %s", policy, w.Header().Get("Referrer-Policy"))
			}
		})
	}
}

func TestSecurity_PermissionsPolicy(t *testing.T) {
	policy := "geolocation=(), microphone=(), camera=()"

	r := router.New()
	r.Use(Security(WithPermissionsPolicy(policy)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Header().Get("Permissions-Policy") != policy {
		t.Errorf("Expected Permissions-Policy: %s, got %s", policy, w.Header().Get("Permissions-Policy"))
	}
}

func TestSecurity_CustomHeaders(t *testing.T) {
	r := router.New()
	r.Use(Security(
		WithCustomHeader("X-Custom-Security", "custom-value"),
		WithCustomHeader("X-Another-Header", "another-value"),
	))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Header().Get("X-Custom-Security") != "custom-value" {
		t.Error("Expected custom header to be set")
	}

	if w.Header().Get("X-Another-Header") != "another-value" {
		t.Error("Expected another custom header to be set")
	}
}

func TestSecurity_DisableContentTypeNosniff(t *testing.T) {
	r := router.New()
	r.Use(Security(WithContentTypeNosniff(false)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Header().Get("X-Content-Type-Options") != "" {
		t.Error("X-Content-Type-Options should not be set when disabled")
	}
}

func TestSecurity_MultipleOptions(t *testing.T) {
	r := router.New()
	r.Use(Security(
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

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Verify all headers are set
	if w.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Error("X-Frame-Options not set correctly")
	}

	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("X-Content-Type-Options not set correctly")
	}

	if w.Header().Get("Content-Security-Policy") == "" {
		t.Error("Content-Security-Policy should be set")
	}

	if w.Header().Get("Referrer-Policy") != "same-origin" {
		t.Error("Referrer-Policy not set correctly")
	}

	if w.Header().Get("Permissions-Policy") != "geolocation=()" {
		t.Error("Permissions-Policy not set correctly")
	}

	if w.Header().Get("X-Custom") != "value" {
		t.Error("Custom header not set correctly")
	}
}

func TestSecureHeaders_Convenience(t *testing.T) {
	r := router.New()
	r.Use(SecureHeaders())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should have default security headers
	if w.Header().Get("X-Frame-Options") == "" {
		t.Error("SecureHeaders should set X-Frame-Options")
	}

	if w.Header().Get("X-Content-Type-Options") == "" {
		t.Error("SecureHeaders should set X-Content-Type-Options")
	}
}

func TestDevelopmentSecurity(t *testing.T) {
	r := router.New()
	r.Use(DevelopmentSecurity())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should have relaxed CSP
	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "unsafe-inline") || !strings.Contains(csp, "unsafe-eval") {
		t.Error("Development security should allow unsafe-inline and unsafe-eval")
	}

	// Should not have HSTS
	if w.Header().Get("Strict-Transport-Security") != "" {
		t.Error("Development security should not set HSTS")
	}

	// Should have SAMEORIGIN instead of DENY
	if w.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Error("Development security should use SAMEORIGIN")
	}
}

func TestNoSecurityHeaders(t *testing.T) {
	r := router.New()
	r.Use(NoSecurityHeaders())
	r.GET("/test", func(c *router.Context) {
		// Manually set a security header
		c.Response.Header().Set("X-Frame-Options", "DENY")
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// All security headers should be removed
	if w.Header().Get("X-Frame-Options") != "" {
		t.Error("NoSecurityHeaders should remove X-Frame-Options")
	}

	if w.Header().Get("X-Content-Type-Options") != "" {
		t.Error("NoSecurityHeaders should remove X-Content-Type-Options")
	}
}

func TestSecurity_CombinedWithOtherMiddleware(t *testing.T) {
	r := router.New()
	r.Use(RequestID())
	r.Use(Security())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should have both request ID and security headers
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("Should have request ID")
	}

	if w.Header().Get("X-Frame-Options") == "" {
		t.Error("Should have security headers")
	}
}

func TestSecurity_EmptyCSP(t *testing.T) {
	r := router.New()
	r.Use(Security(WithContentSecurityPolicy("")))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Header().Get("Content-Security-Policy") != "" {
		t.Error("Empty CSP should not set header")
	}
}

func TestSecurity_EmptyFrameOptions(t *testing.T) {
	r := router.New()
	r.Use(Security(WithFrameOptions("")))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Header().Get("X-Frame-Options") != "" {
		t.Error("Empty frame options should not set header")
	}
}

// Benchmark tests
func BenchmarkSecurity_Default(b *testing.B) {
	r := router.New()
	r.Use(Security())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkSecurity_HTTPS(b *testing.B) {
	r := router.New()
	r.Use(Security())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.TLS = &tls.ConnectionState{}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkSecurity_AllOptions(b *testing.B) {
	r := router.New()
	r.Use(Security(
		WithFrameOptions("SAMEORIGIN"),
		WithContentTypeNosniff(true),
		WithXSSProtection("1; mode=block"),
		WithHSTS(31536000, true, true),
		WithContentSecurityPolicy("default-src 'self'; script-src 'self' https://cdn.example.com"),
		WithReferrerPolicy("same-origin"),
		WithPermissionsPolicy("geolocation=(), microphone=()"),
		WithCustomHeader("X-Custom-1", "value1"),
		WithCustomHeader("X-Custom-2", "value2"),
	))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.TLS = &tls.ConnectionState{}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
