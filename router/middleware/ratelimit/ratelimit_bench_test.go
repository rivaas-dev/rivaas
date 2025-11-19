package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"rivaas.dev/router"
)

func BenchmarkRateLimit(b *testing.B) {
	r := router.MustNew()

	r.Use(New(
		WithRequestsPerSecond(1000000), // Very high limit to avoid rate limiting in benchmark
		WithBurst(1000000),
	))

	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRateLimit_ParallelSameKey(b *testing.B) {
	r := router.MustNew()

	r.Use(New(
		WithRequestsPerSecond(1000000),
		WithBurst(1000000),
	))

	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "ok")
	})

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest("GET", "/test", nil)

		for pb.Next() {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})
}

func BenchmarkRateLimit_ParallelDifferentKeys(b *testing.B) {
	r := router.MustNew()

	r.Use(New(
		WithRequestsPerSecond(1000000),
		WithBurst(1000000),
		WithKeyFunc(func(c *router.Context) string {
			return c.Request.Header.Get("X-User-ID")
		}),
	))

	r.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "ok")
	})

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		userID := 0

		for pb.Next() {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("X-User-ID", string(rune(userID)))
			userID++

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})
}
