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

//go:build !integration

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
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

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
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "ok")
	})

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)

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
			return c.Request.Header.Get("X-User-Id")
		}),
	))

	r.GET("/test", func(c *router.Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "ok")
	})

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		userID := 0

		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-User-Id", string(rune(userID)))
			userID++

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})
}
