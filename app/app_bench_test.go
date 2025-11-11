package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"rivaas.dev/router"
)

func BenchmarkNew(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_, err := New(
			WithServiceName("bench-app"),
			WithServiceVersion("1.0.0"),
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTestJSON(b *testing.B) {
	app, _ := New()
	app.GET("/test", func(c *router.Context) {
		c.JSON(200, map[string]string{"status": "ok"})
	})

	body := map[string]string{"test": "data"}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		resp, err := app.TestJSON("POST", "/test", body)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

func BenchmarkHealthCheckConcurrent(b *testing.B) {
	app, _ := New()
	_ = app.WithStandardEndpoints(StandardEndpointsOpts{
		Readiness: map[string]CheckFunc{
			"always_ready": func(ctx context.Context) error {
				return nil
			},
		},
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/readyz", nil)
			resp, err := app.Test(req)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}

func BenchmarkRouteRegistration(b *testing.B) {
	app, _ := New()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		app.GET("/test", func(c *router.Context) {
			c.String(200, "ok")
		})
	}
}

func BenchmarkRequestHandling(b *testing.B) {
	app, _ := New()
	app.GET("/test", func(c *router.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		resp, err := app.Test(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

func BenchmarkMiddlewareChain(b *testing.B) {
	app, _ := New()
	app.Use(func(c *router.Context) {
		c.Next()
	})
	app.Use(func(c *router.Context) {
		c.Next()
	})
	app.GET("/test", func(c *router.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		resp, err := app.Test(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}
