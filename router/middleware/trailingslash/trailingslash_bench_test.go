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

package trailingslash

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"rivaas.dev/router"
)

func BenchmarkTrailingSlash_Remove(b *testing.B) {
	r := router.MustNew()
	r.Use(New(WithPolicy(PolicyRemove)))

	r.GET("/users", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/users/", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkTrailingSlash_Add(b *testing.B) {
	r := router.MustNew()
	r.Use(New(WithPolicy(PolicyAdd)))

	r.GET("/users/", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkTrailingSlash_Strict(b *testing.B) {
	r := router.MustNew()
	r.Use(New(WithPolicy(PolicyStrict)))

	r.GET("/users", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkTrailingSlash_NoChange(b *testing.B) {
	r := router.MustNew()
	r.Use(New(WithPolicy(PolicyRemove)))

	r.GET("/users", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkTrailingSlash_RootPath(b *testing.B) {
	r := router.MustNew()
	r.Use(New(WithPolicy(PolicyRemove)))

	r.GET("/", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkTrailingSlash_Wrap(b *testing.B) {
	r := router.MustNew()
	r.GET("/users", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	handler := Wrap(r, WithPolicy(PolicyRemove))

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/users/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkTrailingSlash_Parallel(b *testing.B) {
	r := router.MustNew()
	r.Use(New(WithPolicy(PolicyRemove)))

	r.GET("/users", func(c *router.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/users/", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})
}
