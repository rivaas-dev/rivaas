package recovery

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"rivaas.dev/router"
)

func BenchmarkRecovery_NoPanic(b *testing.B) {
	r := router.MustNew()
	r.Use(New())

	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRecovery_WithPanic(b *testing.B) {
	r := router.MustNew()
	r.Use(New())

	r.GET("/panic", func(_ *router.Context) {
		panic("benchmark panic")
	})

	req := httptest.NewRequest("GET", "/panic", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
