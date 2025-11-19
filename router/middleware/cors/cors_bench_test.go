package cors

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rivaas.dev/router"
)

func BenchmarkCORS_SimpleRequest(b *testing.B) {
	r := router.MustNew()
	r.Use(New(WithAllowAllOrigins(true)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkCORS_Preflight(b *testing.B) {
	r := router.MustNew()
	r.Use(New(WithAllowedOrigins("https://example.com")))
	r.POST("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkCORS_OriginValidation(b *testing.B) {
	r := router.MustNew()
	r.Use(New(WithAllowedOrigins(
		"https://example.com",
		"https://app.example.com",
		"https://api.example.com",
		"https://admin.example.com",
		"https://dashboard.example.com",
	)))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://api.example.com")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkCORS_OriginFunc(b *testing.B) {
	r := router.MustNew()
	r.Use(New(WithAllowOriginFunc(func(origin string) bool {
		return strings.HasSuffix(origin, ".example.com")
	})))
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://api.example.com")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
