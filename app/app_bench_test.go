// Copyright 2025 The Rivaas Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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
	app, err := New()
	if err != nil {
		b.Fatal(err)
	}
	app.GET("/test", func(c *Context) {
		if err := c.JSON(http.StatusOK, map[string]string{"status": "ok"}); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	body := map[string]string{"test": "data"}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		resp, err := app.TestJSON(http.MethodPost, "/test", body)
		if err != nil {
			b.Fatal(err)
		}
		_ = resp.Body.Close() //nolint:errcheck // Benchmark cleanup
	}
}

func BenchmarkHealthCheckConcurrent(b *testing.B) {
	app, err := New(
		WithHealthEndpoints(
			WithReadinessCheck("always_ready", func(ctx context.Context) error {
				return nil
			}),
		),
	)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			resp, err := app.Test(req)
			if err != nil {
				b.Fatal(err)
			}
			_ = resp.Body.Close() //nolint:errcheck // Benchmark cleanup
		}
	})
}

func BenchmarkRouteRegistration(b *testing.B) {
	app, err := New()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		app.GET("/test", func(c *Context) {
			if err := c.String(http.StatusOK, "ok"); err != nil {
				c.Logger().Error("failed to write response", "err", err)
			}
		})
	}
}

func BenchmarkRequestHandling(b *testing.B) {
	app, err := New()
	if err != nil {
		b.Fatal(err)
	}
	app.GET("/test", func(c *Context) {
		if err := c.String(http.StatusOK, "ok"); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		resp, err := app.Test(req)
		if err != nil {
			b.Fatal(err)
		}
		_ = resp.Body.Close() //nolint:errcheck // Benchmark cleanup
	}
}

func BenchmarkMiddlewareChain(b *testing.B) {
	app, err := New()
	if err != nil {
		b.Fatal(err)
	}
	app.Use(func(c *Context) {
		c.Next()
	})
	app.Use(func(c *Context) {
		c.Next()
	})
	app.GET("/test", func(c *Context) {
		if err := c.String(http.StatusOK, "ok"); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		resp, err := app.Test(req)
		if err != nil {
			b.Fatal(err)
		}
		_ = resp.Body.Close() //nolint:errcheck // Benchmark cleanup
	}
}
