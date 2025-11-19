package security

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"rivaas.dev/router"
)

func BenchmarkSecurity_Default(b *testing.B) {
	r := router.MustNew()
	r.Use(New())
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
	r := router.MustNew()
	r.Use(New())
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
	r := router.MustNew()
	r.Use(New(
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
