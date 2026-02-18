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

// Package main demonstrates how to use the requestid middleware
// to add unique request IDs to HTTP requests.
package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"rivaas.dev/router"
	"rivaas.dev/router/middleware/accesslog"
	"rivaas.dev/router/middleware/requestid"
)

func main() {
	r := router.MustNew()

	// Example 1: Default request ID (UUID v7)
	defaultRequestIDExample(r)

	// Example 2: ULID format
	ulidExample(r)

	// Example 3: Custom header name
	customHeaderExample(r)

	// Example 4: Custom ID generator
	customGeneratorExample(r)

	// Example 5: Integration with logger
	loggerIntegrationExample(r)

	// Example 6: Reject client-provided IDs
	rejectClientIDExample(r)

	log.Println("Server starting on http://localhost:8080")
	log.Println("Endpoints: /default /ulid /custom /generator /logged /secure")
	log.Fatal(http.ListenAndServe(":8080", r))
}

// Example 1: Default request ID (UUID v7)
func defaultRequestIDExample(r *router.Router) {
	r.Use(requestid.New())

	r.GET("/default", func(c *router.Context) {
		reqID := requestid.Get(c)
		c.JSON(http.StatusOK, map[string]any{
			"message":    "Request with UUID v7 (default)",
			"request_id": reqID,
			"format":     "UUID v7 (36 chars, RFC 9562)",
			"header":     "X-Request-ID",
		})
	})
}

// Example 2: ULID format (shorter, 26 chars)
func ulidExample(r *router.Router) {
	ulid := r.Group("/ulid")

	ulid.Use(requestid.New(
		requestid.WithULID(),
	))

	ulid.GET("", func(c *router.Context) {
		reqID := requestid.Get(c)
		c.JSON(http.StatusOK, map[string]any{
			"message":    "Request with ULID format",
			"request_id": reqID,
			"format":     "ULID (26 chars, time-ordered)",
			"header":     "X-Request-ID",
		})
	})
}

// Example 3: Custom header name
func customHeaderExample(r *router.Router) {
	custom := r.Group("/custom")

	custom.Use(requestid.New(
		requestid.WithHeader("X-Trace-ID"),
	))

	custom.GET("", func(c *router.Context) {
		reqID := requestid.Get(c)
		c.JSON(http.StatusOK, map[string]any{
			"message":    "Request with custom header",
			"request_id": reqID,
			"header":     "X-Trace-ID",
		})
	})
}

// Example 4: Custom ID generator
func customGeneratorExample(r *router.Router) {
	generator := r.Group("/generator")

	generator.Use(requestid.New(
		requestid.WithGenerator(func() string {
			// Custom format: timestamp + random suffix
			return fmt.Sprintf("req-%d-%d", time.Now().Unix(), time.Now().Nanosecond())
		}),
	))

	generator.GET("", func(c *router.Context) {
		reqID := requestid.Get(c)
		c.JSON(http.StatusOK, map[string]any{
			"message":    "Request with custom generator",
			"request_id": reqID,
			"format":     "req-{timestamp}-{nanosecond}",
		})
	})
}

// Example 5: Integration with accesslog
func loggerIntegrationExample(r *router.Router) {
	logged := r.Group("/logged")

	// Create a logger for accesslog middleware
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// RequestID must come before accesslog
	logged.Use(requestid.New())
	logged.Use(accesslog.New(accesslog.WithLogger(logger)))

	logged.GET("", func(c *router.Context) {
		reqID := requestid.Get(c)
		c.JSON(http.StatusOK, map[string]any{
			"message":    "Request ID is automatically included in logs",
			"request_id": reqID,
		})
	})
}

// Example 6: Reject client-provided IDs (more secure)
func rejectClientIDExample(r *router.Router) {
	secure := r.Group("/secure")

	secure.Use(requestid.New(
		requestid.WithAllowClientID(false), // Always generate new ID
	))

	secure.GET("", func(c *router.Context) {
		reqID := requestid.Get(c)
		c.JSON(http.StatusOK, map[string]any{
			"message":    "Client-provided IDs are ignored (security)",
			"request_id": reqID,
			"note":       "ID is always server-generated",
		})
	})
}
