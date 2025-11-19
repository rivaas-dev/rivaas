package basicauth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"rivaas.dev/router"
)

func BenchmarkBasicAuth(b *testing.B) {
	r := router.MustNew()
	r.Use(New(
		WithUsers(map[string]string{
			"admin": "secret123",
		}),
	))
	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "success")
	})

	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret123"))
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", authHeader)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkBasicAuthWithValidator(b *testing.B) {
	validUsers := map[string]string{
		"admin": "secret123",
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

	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret123"))
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", authHeader)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
