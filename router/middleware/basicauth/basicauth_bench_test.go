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

package basicauth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"rivaas.dev/router"
)

func BenchmarkBasicAuth(b *testing.B) {
	r := router.MustNew()
	r.Use(New(
		WithUsers(map[string]string{
			"admin": "secret123",
		}),
	))
	r.GET("/test", func(c *router.Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "success")
	})

	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret123"))
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", authHeader)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkBasicAuthWithValidator(b *testing.B) {
	validUsers := map[string]string{
		"admin": "secret123",
	}

	r := router.MustNew()
	r.Use(New(
		WithValidator(func(username, password string) bool {
			expectedPassword, exists := validUsers[username]
			return exists && password == expectedPassword
		}),
	))
	r.GET("/test", func(c *router.Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "success")
	})

	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret123"))
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", authHeader)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
