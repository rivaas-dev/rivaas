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

// This file contains BDD-style integration tests for middleware components.
//
// # Test Philosophy
//
// These tests verify that middleware components work correctly together in realistic
// scenarios. They test:
//
//   - Middleware ordering and interaction
//   - Context propagation between middleware
//   - End-to-end request/response behavior
//   - Real-world use cases and edge cases
//
// # Test Helpers
//
// testLogHandler: Captures log output for verification
//   - Thread-safe log record capture
//   - Filters by log level
//   - Extracts structured fields
//
// # Running These Tests
//
// See middleware_integration_suite_test.go for detailed instructions on running
// integration tests separately from unit tests.
package middleware_test

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
	"rivaas.dev/router/middleware/compression"
	"rivaas.dev/router/middleware/cors"
	"rivaas.dev/router/middleware/recovery"
	"rivaas.dev/router/middleware/requestid"
	"rivaas.dev/router/middleware/security"
)

// testLogHandler captures log records for testing.
// It implements slog.Handler to intercept log output and make it available for assertions.
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

func (h *testLogHandler) reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = nil
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

func (h *testLogHandler) getFields(level slog.Level) map[string]any {
	records := h.getRecords(level)
	if len(records) == 0 {
		return nil
	}
	return records[0].attrs
}

var _ = Describe("Middleware Integration", Label("integration"), func() {
	Describe("Basic Stack", func() {
		It("should integrate RequestID, AccessLog, and Recovery middleware", func() {
			handler := newTestLogHandler()
			logger := slog.New(handler)

			r := router.MustNew()
			r.Use(requestid.New())
			r.Use(accesslog.New(accesslog.WithLogger(logger)))
			r.Use(recovery.New())

			r.GET("/test", func(c *router.Context) {
				// Verify RequestID is available
				reqID := requestid.Get(c)
				Expect(reqID).NotTo(BeEmpty(), "RequestID should be available in handler")
				c.JSON(http.StatusOK, map[string]string{
					"request_id": reqID,
					"message":    "success",
				})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Verify response
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(ContainSubstring("success"))

			// Verify RequestID header is set
			requestIDHeader := w.Header().Get("X-Request-ID")
			Expect(requestIDHeader).NotTo(BeEmpty(), "RequestID header should be set")

			// Verify AccessLog captured the request
			logRecords := handler.getRecords(slog.LevelInfo)
			Expect(logRecords).To(HaveLen(1), "AccessLog should have logged the request")
			Expect(logRecords[0].msg).To(Equal("access"))

			// Verify basic log fields are present
			logFields := handler.getFields(slog.LevelInfo)
			Expect(logFields).To(HaveKey("method"), "AccessLog should include method")
			Expect(logFields).To(HaveKey("path"), "AccessLog should include path")
			Expect(logFields).To(HaveKey("status"), "AccessLog should include status")
		})

		It("should catch panics with Recovery middleware", func() {
			handler := newTestLogHandler()
			logger := slog.New(handler)

			r := router.MustNew()
			r.Use(requestid.New())
			r.Use(accesslog.New(accesslog.WithLogger(logger)))
			r.Use(recovery.New())

			r.GET("/panic", func(c *router.Context) {
				panic("test panic")
			})

			req := httptest.NewRequest(http.MethodGet, "/panic", nil)
			w := httptest.NewRecorder()

			// Should not panic - Recovery middleware should catch it
			r.ServeHTTP(w, req)

			// Verify Recovery handled the panic
			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			// Verify RequestID is still set
			requestIDHeader := w.Header().Get("X-Request-ID")
			Expect(requestIDHeader).NotTo(BeEmpty(), "RequestID should be set even when panic occurs")

			// Verify AccessLog captured the error
			logRecords := handler.getRecords(slog.LevelError)
			Expect(logRecords).To(HaveLen(1), "AccessLog should have logged the error")
			Expect(logRecords[0].msg).To(Equal("access"))

			logFields := handler.getFields(slog.LevelError)
			Expect(logFields["status"]).To(Equal(int64(http.StatusInternalServerError)))
		})
	})

	Describe("Security Stack", func() {
		It("should integrate Security, CORS, and BasicAuth middleware", func() {
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
				username := basicauth.GetUsername(c)
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

			// Verify CORS headers are set (only when Origin header is present)
			Expect(w.Header().Get("Access-Control-Allow-Origin")).To(Equal("https://example.com"))

			// Verify BasicAuth username is available
			Expect(w.Body.String()).To(ContainSubstring("admin"))
		})

		It("should reject unauthorized requests", func() {
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

	Describe("Full Production Stack", func() {
		It("should integrate all middleware types", func() {
			handler := newTestLogHandler()
			logger := slog.New(handler)

			r := router.MustNew()

			// Observability (first)
			r.Use(requestid.New())
			r.Use(accesslog.New(accesslog.WithLogger(logger)))

			// Reliability
			r.Use(recovery.New())

			// Security
			r.Use(security.New())
			r.Use(cors.New(
				cors.WithAllowedOrigins("https://example.com"),
			))

			// Performance
			r.Use(compression.New())

			r.GET("/api/users", func(c *router.Context) {
				reqID := requestid.Get(c)
				c.JSON(http.StatusOK, map[string]any{
					"request_id": reqID,
					"users":      []string{"user1", "user2"},
				})
			})

			req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Verify response
			Expect(w.Code).To(Equal(http.StatusOK))

			// Verify RequestID is set
			requestIDHeader := w.Header().Get("X-Request-ID")
			Expect(requestIDHeader).NotTo(BeEmpty(), "RequestID should be set")

			// Verify security headers
			Expect(w.Header().Get("X-Content-Type-Options")).NotTo(BeEmpty())

			// Verify AccessLog captured the request
			logRecords := handler.getRecords(slog.LevelInfo)
			Expect(logRecords).To(HaveLen(1), "AccessLog should have logged the request")
			logFields := handler.getFields(slog.LevelInfo)
			Expect(logFields).To(HaveKey("method"), "AccessLog should include method")
			Expect(logFields).To(HaveKey("path"), "AccessLog should include path")
			Expect(logFields).To(HaveKey("status"), "AccessLog should include status")
		})
	})

	Describe("Middleware Ordering", func() {
		It("should require RequestID before AccessLog", func() {
			handler := newTestLogHandler()
			logger := slog.New(handler)

			// Correct order: RequestID -> AccessLog
			r := router.MustNew()
			r.Use(requestid.New())
			r.Use(accesslog.New(accesslog.WithLogger(logger)))

			r.GET("/test", func(c *router.Context) {
				c.JSON(http.StatusOK, map[string]string{"message": "ok"})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Verify RequestID is set
			requestIDHeader := w.Header().Get("X-Request-ID")
			Expect(requestIDHeader).NotTo(BeEmpty(), "RequestID should be set")

			// Verify AccessLog captured the request
			// Note: AccessLog uses RequestID for sampling but doesn't include it in log fields
			logFields := handler.getFields(slog.LevelInfo)
			Expect(logFields).To(HaveKey("method"), "AccessLog should include method")
			Expect(logFields).To(HaveKey("path"), "AccessLog should include path")
			Expect(logFields).To(HaveKey("status"), "AccessLog should include status")
		})

		It("should catch panics from middleware with Recovery", func() {
			handler := newTestLogHandler()
			logger := slog.New(handler)

			r := router.MustNew()
			r.Use(requestid.New())
			r.Use(accesslog.New(accesslog.WithLogger(logger)))

			// Recovery should be after AccessLog to catch panics
			r.Use(recovery.New())

			// Middleware that panics
			r.Use(func(c *router.Context) {
				panic("middleware panic")
			})

			r.GET("/test", func(c *router.Context) {
				c.JSON(http.StatusOK, map[string]string{"message": "should not reach here"})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			// Should not panic - Recovery should catch it
			r.ServeHTTP(w, req)

			// Verify Recovery handled the panic
			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			// Verify RequestID is still set
			requestIDHeader := w.Header().Get("X-Request-ID")
			Expect(requestIDHeader).NotTo(BeEmpty(), "RequestID should be set even when panic occurs")

			// Verify AccessLog captured the error
			logRecords := handler.getRecords(slog.LevelError)
			Expect(logRecords).To(HaveLen(1), "AccessLog should have logged the error")
		})
	})

	Describe("Context Propagation", func() {
		It("should propagate context values across middleware", func() {
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
				capturedUsername = basicauth.GetUsername(c)
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

	Describe("Multiple Groups", func() {
		It("should apply middleware to different route groups", func() {
			handler := newTestLogHandler()
			logger := slog.New(handler)

			r := router.MustNew()

			// Global middleware
			r.Use(requestid.New())
			r.Use(accesslog.New(accesslog.WithLogger(logger)))
			r.Use(recovery.New())

			// Public routes
			r.GET("/public", func(c *router.Context) {
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
				username := basicauth.GetUsername(c)
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

	Describe("Error Handling", func() {
		It("should handle errors across multiple middleware", func() {
			handler := newTestLogHandler()
			logger := slog.New(handler)

			r := router.MustNew()
			r.Use(requestid.New())
			r.Use(accesslog.New(accesslog.WithLogger(logger)))
			r.Use(recovery.New())

			// Handler that returns error status
			r.GET("/error", func(c *router.Context) {
				c.JSON(http.StatusInternalServerError, map[string]string{
					"error": "something went wrong",
				})
			})

			// Handler that panics
			r.GET("/panic", func(c *router.Context) {
				panic("handler panic")
			})

			// Test error response
			req1 := httptest.NewRequest(http.MethodGet, "/error", nil)
			w1 := httptest.NewRecorder()
			r.ServeHTTP(w1, req1)

			Expect(w1.Code).To(Equal(http.StatusInternalServerError))
			Expect(w1.Header().Get("X-Request-ID")).NotTo(BeEmpty())

			// Verify error was logged
			logRecords := handler.getRecords(slog.LevelError)
			Expect(logRecords).To(HaveLen(1), "AccessLog should have logged the error")
			logFields := handler.getFields(slog.LevelError)
			Expect(logFields["status"]).To(Equal(int64(http.StatusInternalServerError)))

			// Reset handler
			handler.reset()

			// Test panic recovery
			req2 := httptest.NewRequest(http.MethodGet, "/panic", nil)
			w2 := httptest.NewRecorder()
			r.ServeHTTP(w2, req2)

			Expect(w2.Code).To(Equal(http.StatusInternalServerError))
			Expect(w2.Header().Get("X-Request-ID")).NotTo(BeEmpty())

			// Verify panic was logged
			logRecords = handler.getRecords(slog.LevelError)
			Expect(logRecords).To(HaveLen(1), "AccessLog should have logged the panic recovery")
		})
	})
})
