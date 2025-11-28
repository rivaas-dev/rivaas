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

package accesslog

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"rivaas.dev/router"
	"rivaas.dev/router/middleware/requestid"
)

func BenchmarkAccessLog_Basic(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	r := router.MustNew()
	r.Use(New(WithLogger(logger)))

	r.GET("/users/:id", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkAccessLog_WithExclusions(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	r := router.MustNew()
	r.Use(New(
		WithLogger(logger),
		WithExcludePaths("/health", "/metrics"),
	))

	r.GET("/health", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkAccessLog_WithPrefixExclusions(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	r := router.MustNew()
	r.Use(New(
		WithLogger(logger),
		WithExcludePrefixes("/static", "/assets"),
	))

	r.GET("/static/file.txt", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/static/file.txt", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkAccessLog_WithSlowThreshold(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	r := router.MustNew()
	r.Use(New(
		WithLogger(logger),
		WithSlowThreshold(100),
	))

	r.GET("/fast", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/fast", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkAccessLog_WithSampling(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	r := router.MustNew()
	r.Use(New(
		WithLogger(logger),
		WithSampleRate(0.5), // 50% sampling
	))

	r.GET("/users/:id", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkAccessLog_WithErrorsOnly(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	r := router.MustNew()
	r.Use(New(
		WithLogger(logger),
		WithErrorsOnly(),
	))

	r.GET("/users/:id", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkAccessLog_NoLogger(b *testing.B) {
	// No logger configured - middleware should skip logging
	r := router.MustNew()
	r.Use(New())

	r.GET("/users/:id", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkAccessLog_Parallel(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	r := router.MustNew()
	r.Use(New(WithLogger(logger)))

	r.GET("/users/:id", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})
}

func BenchmarkAccessLog_WithRequestID(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	r := router.MustNew()
	r.Use(requestid.New())
	r.Use(New(WithLogger(logger)))

	r.GET("/users/:id", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
