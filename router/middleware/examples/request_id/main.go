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
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"rivaas.dev/logging"
	"rivaas.dev/router"
	"rivaas.dev/router/middleware/accesslog"
	"rivaas.dev/router/middleware/requestid"
)

func main() {
	r := router.MustNew()

	// Set up logging for accesslog middleware
	logCfg := logging.MustNew(
		logging.WithConsoleHandler(),
		logging.WithDebugLevel(),
	)
	r.SetLogger(logCfg)

	// Example 1: Default request ID
	defaultRequestIDExample(r)

	// Example 2: Custom header name
	customHeaderExample(r)

	// Example 3: Custom ID generator
	customGeneratorExample(r)

	// Example 4: Integration with logger
	loggerIntegrationExample(r)

	// Example 5: Reject client-provided IDs
	rejectClientIDExample(r)

	// Create a logger with clean, colorful output
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: false,
		ReportCaller:    false,
	})

	logger.Info("🚀 Server starting on http://localhost:8080")
	logger.Print("")
	logger.Print("📝 Available endpoints:")
	logger.Print("  GET /default      - Default request ID (X-Request-ID)")
	logger.Print("  GET /custom       - Custom header name (X-Trace-ID)")
	logger.Print("  GET /generator    - Custom ID generator")
	logger.Print("  GET /logged       - With logger integration")
	logger.Print("  GET /secure       - Reject client-provided IDs")
	logger.Print("")
	logger.Print("📋 Example commands:")
	logger.Print("  curl http://localhost:8080/default")
	logger.Print("  curl http://localhost:8080/custom")
	logger.Print("  curl -H 'X-Request-ID: my-custom-id' http://localhost:8080/default")
	logger.Print("  curl http://localhost:8080/logged")
	logger.Print("")
	logger.Print("💡 Tip: Request IDs are included in response headers and context")
	logger.Print("   Use requestid.Get(c) to retrieve the ID in handlers")
	logger.Print("")

	logger.Fatal(http.ListenAndServe(":8080", r))
}

// Example 1: Default request ID
func defaultRequestIDExample(r *router.Router) {
	r.Use(requestid.New())

	r.GET("/default", func(c *router.Context) {
		reqID := requestid.Get(c)
		c.JSON(http.StatusOK, map[string]any{
			"message":    "Request with default ID",
			"request_id": reqID,
			"header":     "X-Request-ID",
		})
	})
}

// Example 2: Custom header name
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

// Example 3: Custom ID generator
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

// Example 4: Integration with accesslog
func loggerIntegrationExample(r *router.Router) {
	logged := r.Group("/logged")

	// RequestID must come before accesslog
	logged.Use(requestid.New())
	logged.Use(accesslog.New())

	logged.GET("", func(c *router.Context) {
		reqID := requestid.Get(c)
		c.JSON(http.StatusOK, map[string]any{
			"message":    "Request ID is automatically included in logs",
			"request_id": reqID,
		})
	})
}

// Example 5: Reject client-provided IDs (more secure)
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
