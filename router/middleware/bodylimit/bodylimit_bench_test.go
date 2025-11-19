package bodylimit

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"rivaas.dev/router"
)

func BenchmarkBodyLimit_ContentLengthCheck(b *testing.B) {
	r := router.MustNew()
	r.Use(New(WithLimit(1024 * 1024)))
	r.POST("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create fresh request for each iteration
		body := bytes.NewBufferString(`{"key": "value"}`)
		req := httptest.NewRequest(http.MethodPost, "/test", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Length", "18")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkBodyLimit_NoContentLength(b *testing.B) {
	r := router.MustNew()
	r.Use(New(WithLimit(1024 * 1024)))
	r.POST("/test", func(c *router.Context) {
		io.Copy(io.Discard, c.Request.Body)
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create fresh request for each iteration
		body := bytes.NewBufferString(`{"key": "value"}`)
		req := httptest.NewRequest(http.MethodPost, "/test", body)
		req.Header.Set("Content-Type", "application/json")
		// No Content-Length header

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
