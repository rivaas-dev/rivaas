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

// This file contains integration tests for AccessLog middleware with other middleware.
//
// These tests verify that AccessLog works correctly when combined with RequestID and
// Recovery middleware in realistic scenarios.

//go:build integration

package accesslog_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"rivaas.dev/router"
	"rivaas.dev/router/middleware/accesslog"
	"rivaas.dev/router/middleware/recovery"
	"rivaas.dev/router/middleware/requestid"
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

var _ = Describe("AccessLog Integration", Label("integration", "accesslog"), func() {
	Describe("with RequestID and Recovery", func() {
		It("should log requests with RequestID available", func() {
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
				//nolint:errcheck // Test handler
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
			Expect(logRecords[0].msg).To(Equal("http request"))

			// Verify basic log fields are present
			logFields := handler.getFields(slog.LevelInfo)
			Expect(logFields).To(HaveKey("method"), "AccessLog should include method")
			Expect(logFields).To(HaveKey("path"), "AccessLog should include path")
			Expect(logFields).To(HaveKey("status"), "AccessLog should include status")
		})

		It("should log error status when Recovery catches panic", func() {
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
			Expect(logRecords[0].msg).To(Equal("http request"))

			logFields := handler.getFields(slog.LevelError)
			Expect(logFields["status"]).To(Equal(int64(http.StatusInternalServerError)))
		})
	})

	Describe("middleware ordering", func() {
		It("should work correctly when RequestID comes before AccessLog", func() {
			handler := newTestLogHandler()
			logger := slog.New(handler)

			// Correct order: RequestID -> AccessLog
			r := router.MustNew()
			r.Use(requestid.New())
			r.Use(accesslog.New(accesslog.WithLogger(logger)))

			r.GET("/test", func(c *router.Context) {
				//nolint:errcheck // Test handler
				c.JSON(http.StatusOK, map[string]string{"message": "ok"})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Verify RequestID is set
			requestIDHeader := w.Header().Get("X-Request-ID")
			Expect(requestIDHeader).NotTo(BeEmpty(), "RequestID should be set")

			// Verify AccessLog captured the request
			logFields := handler.getFields(slog.LevelInfo)
			Expect(logFields).To(HaveKey("method"), "AccessLog should include method")
			Expect(logFields).To(HaveKey("path"), "AccessLog should include path")
			Expect(logFields).To(HaveKey("status"), "AccessLog should include status")
		})
	})

	Describe("error handling", func() {
		It("should log errors and panics correctly", func() {
			handler := newTestLogHandler()
			logger := slog.New(handler)

			r := router.MustNew()
			r.Use(requestid.New())
			r.Use(accesslog.New(accesslog.WithLogger(logger)))
			r.Use(recovery.New())

			// Handler that returns error status
			r.GET("/error", func(c *router.Context) {
				//nolint:errcheck // Test handler
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
