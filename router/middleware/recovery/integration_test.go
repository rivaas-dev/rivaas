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

// This file contains integration tests for Recovery middleware with other middleware.
//
// These tests verify that Recovery works correctly when combined with AccessLog and
// RequestID middleware in realistic scenarios.

//go:build integration

package recovery_test

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

var _ = Describe("Recovery Integration", Label("integration", "recovery"), func() {
	Describe("panic recovery", func() {
		It("should catch panics from other middleware", func() {
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
				//nolint:errcheck // Test handler
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

	Describe("error handling", func() {
		It("should handle errors and panics from handlers", func() {
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
