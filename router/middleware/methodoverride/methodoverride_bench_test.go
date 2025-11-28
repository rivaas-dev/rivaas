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

package methodoverride

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"rivaas.dev/router"
)

func BenchmarkMethodOverride_Header(b *testing.B) {
	r := router.MustNew()
	r.Use(New(
		WithHeader("X-HTTP-Method-Override"),
		WithAllow("PUT", "PATCH", "DELETE"),
		WithOnlyOn("POST"),
	))

	r.PUT("/users/:id", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/users/123", nil)
		req.Header.Set("X-HTTP-Method-Override", "PUT")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkMethodOverride_QueryParam(b *testing.B) {
	r := router.MustNew()
	r.Use(New(
		WithQueryParam("_method"),
		WithAllow("PUT", "PATCH", "DELETE"),
		WithOnlyOn("POST"),
	))

	r.PUT("/users/:id", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/users/123?_method=PUT", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkMethodOverride_NoOverride(b *testing.B) {
	r := router.MustNew()
	r.Use(New(
		WithHeader("X-HTTP-Method-Override"),
		WithAllow("PUT", "PATCH", "DELETE"),
		WithOnlyOn("POST"),
	))

	r.POST("/users", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/users", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkMethodOverride_WithCSRFCheck(b *testing.B) {
	r := router.MustNew()
	r.Use(New(
		WithHeader("X-HTTP-Method-Override"),
		WithAllow("PUT", "PATCH", "DELETE"),
		WithOnlyOn("POST"),
		WithRequireCSRFToken(true),
	))

	r.PUT("/users/:id", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/users/123", nil)
		req.Header.Set("X-HTTP-Method-Override", "PUT")
		// CSRF not verified, so override should be skipped
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkMethodOverride_Parallel(b *testing.B) {
	r := router.MustNew()
	r.Use(New(
		WithHeader("X-HTTP-Method-Override"),
		WithAllow("PUT", "PATCH", "DELETE"),
		WithOnlyOn("POST"),
	))

	r.PUT("/users/:id", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodPost, "/users/123", nil)
			req.Header.Set("X-HTTP-Method-Override", "PUT")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})
}
