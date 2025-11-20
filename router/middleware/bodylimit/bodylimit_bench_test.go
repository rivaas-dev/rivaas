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
