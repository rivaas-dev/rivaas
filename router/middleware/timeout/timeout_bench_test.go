package timeout

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rivaas.dev/router"
)

func BenchmarkTimeout_NoTimeout(b *testing.B) {
	r := router.MustNew()
	r.Use(New(1 * time.Second))
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

func BenchmarkTimeout_WithContextCheck(b *testing.B) {
	r := router.MustNew()
	r.Use(New(1 * time.Second))
	r.GET("/test", func(c *router.Context) {
		// Simulate handler checking context
		select {
		case <-c.Request.Context().Done():
			return
		default:
			c.JSON(http.StatusOK, map[string]string{"message": "ok"})
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
