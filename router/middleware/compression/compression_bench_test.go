package compression

import (
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rivaas.dev/router"
)

func BenchmarkCompression_Enabled(b *testing.B) {
	r := router.MustNew()
	r.Use(New())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "test data"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkCompression_Disabled(b *testing.B) {
	r := router.MustNew()
	r.Use(New())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "test data"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No Accept-Encoding header

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkCompression_LargeResponse(b *testing.B) {
	r := router.MustNew()
	r.Use(New())

	largeData := strings.Repeat("benchmark data ", 1000)
	r.GET("/large", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"data": largeData})
	})

	req := httptest.NewRequest(http.MethodGet, "/large", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkCompression_BestSpeed(b *testing.B) {
	r := router.MustNew()
	r.Use(New(WithGzipLevel(gzip.BestSpeed)))
	r.GET("/test", func(c *router.Context) {
		data := strings.Repeat("data ", 100)
		c.JSON(http.StatusOK, map[string]string{"content": data})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkCompression_BestCompression(b *testing.B) {
	r := router.MustNew()
	r.Use(New(WithGzipLevel(gzip.BestCompression)))
	r.GET("/test", func(c *router.Context) {
		data := strings.Repeat("data ", 100)
		c.JSON(http.StatusOK, map[string]string{"content": data})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
