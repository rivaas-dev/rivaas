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

// This file contains integration tests for BasicAuth middleware with other middleware.
//
// These tests verify that BasicAuth works correctly when combined with Security, CORS,
// and RequestID middleware in realistic scenarios.

//go:build integration

package basicauth_test

import (
	"context"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"rivaas.dev/router"
	"rivaas.dev/router/middleware/accesslog"
	"rivaas.dev/router/middleware/basicauth"
	"rivaas.dev/router/middleware/cors"
	"rivaas.dev/router/middleware/recovery"
	"rivaas.dev/router/middleware/requestid"
	"rivaas.dev/router/middleware/security"
)

// testLogHandler captures log records for testing.
type testLogHandler struct {
	mu      sync.Mutex
	records []testLogRecord
}

type testLogRecord struct {
	level slog.Level
	msg   string
	attrs map[string]any
}

func newTestLogHandler() *testLogHandler {
	return &testLogHandler{
		records: make([]testLogRecord, 0),
	}
}

func (h *testLogHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *testLogHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	attrs := make(map[string]any)
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	h.records = append(h.records, testLogRecord{
		level: r.Level,
		msg:   r.Message,
		attrs: attrs,
	})

	return nil
}

func (h *testLogHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

func (h *testLogHandler) WithGroup(_ string) slog.Handler {
	return h
}

func (h *testLogHandler) getRecords(level slog.Level) []testLogRecord {
	h.mu.Lock()
	defer h.mu.Unlock()

	var result []testLogRecord
	for _, r := range h.records {
		if r.level == level {
			result = append(result, r)
		}
	}

	return result
}

var _ = Describe("BasicAuth Integration", Label("integration", "basicauth"), func() {
	Describe("with Security and CORS", func() {
		It("should work with security headers and CORS", func() {
			r := router.MustNew()
			r.Use(security.New())
			r.Use(cors.New(
				cors.WithAllowedOrigins("https://example.com"),
				cors.WithAllowedMethods("GET", "POST"),
				cors.WithAllowedHeaders("Content-Type", "Authorization"),
			))
			r.Use(basicauth.New(
				basicauth.WithUsers(map[string]string{
					"admin": "secret",
				}),
			))

			r.GET("/protected", func(c *router.Context) {
				username := basicauth.Username(c)
				//nolint:errcheck // Test handler
				c.JSON(http.StatusOK, map[string]string{
					"user":    username,
					"message": "protected resource",
				})
			})

			// Test authenticated request with CORS origin
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secret")))
			req.Header.Set("Origin", "https://example.com")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Verify authentication succeeded
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(ContainSubstring("admin"))

			// Verify security headers are set
			Expect(w.Header().Get("X-Content-Type-Options")).NotTo(BeEmpty())
			Expect(w.Header().Get("X-Frame-Options")).NotTo(BeEmpty())

			// Verify CORS headers are set
			Expect(w.Header().Get("Access-Control-Allow-Origin")).To(Equal("https://example.com"))
		})

		It("should reject unauthorized requests with security headers", func() {
			r := router.MustNew()
			r.Use(security.New())
			r.Use(cors.New(
				cors.WithAllowedOrigins("https://example.com"),
			))
			r.Use(basicauth.New(
				basicauth.WithUsers(map[string]string{
					"admin": "secret",
				}),
			))

			r.GET("/protected", func(c *router.Context) {
				//nolint:errcheck // Test handler
				c.JSON(http.StatusOK, map[string]string{"message": "should not reach here"})
			})

			// Test unauthenticated request
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Verify authentication failed
			Expect(w.Code).To(Equal(http.StatusUnauthorized))
			Expect(w.Header().Get("WWW-Authenticate")).NotTo(BeEmpty())

			// Verify security headers are still set (even on error)
			Expect(w.Header().Get("X-Content-Type-Options")).NotTo(BeEmpty())
		})
	})

	Describe("context propagation", func() {
		It("should propagate RequestID and BasicAuth username", func() {
			r := router.MustNew()
			r.Use(requestid.New())
			r.Use(basicauth.New(
				basicauth.WithUsers(map[string]string{
					"admin": "secret",
				}),
			))

			var capturedRequestID string
			var capturedUsername string

			r.GET("/test", func(c *router.Context) {
				capturedRequestID = requestid.Get(c)
				capturedUsername = basicauth.Username(c)
				//nolint:errcheck // Test handler
				c.JSON(http.StatusOK, map[string]string{
					"request_id": capturedRequestID,
					"username":   capturedUsername,
				})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secret")))
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Verify context values are available
			Expect(capturedRequestID).NotTo(BeEmpty(), "RequestID should be available in handler")
			Expect(capturedUsername).To(Equal("admin"), "Username should be available in handler")

			// Verify response contains both values
			Expect(w.Body.String()).To(ContainSubstring(capturedRequestID))
			Expect(w.Body.String()).To(ContainSubstring("admin"))
		})
	})

	Describe("route groups", func() {
		It("should work with group-scoped authentication", func() {
			handler := newTestLogHandler()
			logger := slog.New(handler)

			r := router.MustNew()

			// Global middleware
			r.Use(requestid.New())
			r.Use(accesslog.New(accesslog.WithLogger(logger)))
			r.Use(recovery.New())

			// Public routes
			r.GET("/public", func(c *router.Context) {
				//nolint:errcheck // Test handler
				c.JSON(http.StatusOK, map[string]string{"message": "public"})
			})

			// Protected group
			protected := r.Group("/admin")
			protected.Use(basicauth.New(
				basicauth.WithUsers(map[string]string{
					"admin": "secret",
				}),
			))
			protected.GET("/dashboard", func(c *router.Context) {
				username := basicauth.Username(c)
				//nolint:errcheck // Test handler
				c.JSON(http.StatusOK, map[string]string{
					"user":    username,
					"message": "dashboard",
				})
			})

			// Test public route
			req1 := httptest.NewRequest(http.MethodGet, "/public", nil)
			w1 := httptest.NewRecorder()
			r.ServeHTTP(w1, req1)

			Expect(w1.Code).To(Equal(http.StatusOK))
			Expect(w1.Body.String()).To(ContainSubstring("public"))
			Expect(w1.Header().Get("X-Request-ID")).NotTo(BeEmpty())

			// Test protected route without auth
			req2 := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
			w2 := httptest.NewRecorder()
			r.ServeHTTP(w2, req2)

			Expect(w2.Code).To(Equal(http.StatusUnauthorized))

			// Test protected route with auth
			req3 := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
			req3.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secret")))
			w3 := httptest.NewRecorder()
			r.ServeHTTP(w3, req3)

			Expect(w3.Code).To(Equal(http.StatusOK))
			Expect(w3.Body.String()).To(ContainSubstring("admin"))
			Expect(w3.Header().Get("X-Request-ID")).NotTo(BeEmpty())

			// Verify AccessLog captured all requests
			logRecords := handler.getRecords(slog.LevelInfo)
			Expect(len(logRecords)).To(BeNumerically(">=", 2), "AccessLog should have logged multiple requests")
		})
	})
})
