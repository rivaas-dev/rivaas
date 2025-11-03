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

// Package main provides examples of using the Logger middleware.
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"rivaas.dev/router"
	"rivaas.dev/router/middleware/logger"
	"rivaas.dev/router/middleware/requestid"
)

func main() {
	r := router.New()

	// Example 1: Default logging
	defaultLoggingExample(r)

	// Example 2: Custom format logging
	customFormatExample(r)

	// Example 3: JSON structured logging
	jsonLoggingExample(r)

	// Example 4: Skip paths (health checks)
	skipPathsExample(r)

	// Example 5: Integration with request ID
	requestIDIntegrationExample(r)

	// Create a logger with clean, colorful output
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: false,
		ReportCaller:    false,
	})

	logger.Info("🚀 Server starting on http://localhost:8080")
	logger.Print("")
	logger.Print("📝 Available endpoints:")
	logger.Print("  GET /default  - Default format logging")
	logger.Print("  GET /custom   - Custom format")
	logger.Print("  GET /json     - JSON structured logs")
	logger.Print("  GET /health   - Skipped from logs")
	logger.Print("  GET /tracked  - With request ID")
	logger.Print("")
	logger.Print("📋 Example commands:")
	logger.Print("  curl http://localhost:8080/default")
	logger.Print("  curl http://localhost:8080/custom")
	logger.Print("  curl http://localhost:8080/json")
	logger.Print("  curl http://localhost:8080/health")
	logger.Print("  curl http://localhost:8080/tracked")
	logger.Print("")
	logger.Print("💡 Tip: Check the server output to see different log formats")
	logger.Print("")

	logger.Fatal(http.ListenAndServe(":8080", r))
}

// Example 1: Default logging
func defaultLoggingExample(r *router.Router) {
	r.Use(logger.New())

	r.GET("/default", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Request logged with default format",
		})
	})
}

// Example 2: Custom format logging
func customFormatExample(r *router.Router) {
	custom := r.Group("/custom")

	custom.Use(logger.New(
		logger.WithFormatter(func(params logger.FormatterParams) string {
			return fmt.Sprintf("[%s] %s %s %d %v\n",
				params.TimeStamp.Format("2006-01-02 15:04:05"),
				params.Method,
				params.Path,
				params.StatusCode,
				params.Latency,
			)
		}),
	))

	custom.GET("", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Custom log format",
		})
	})
}

// Example 3: JSON structured logging
func jsonLoggingExample(r *router.Router) {
	json := r.Group("/json")

	json.Use(logger.New(
		logger.WithFormatter(func(params logger.FormatterParams) string {
			return fmt.Sprintf(
				`{"time":"%s","method":"%s","path":"%s","status":%d,"latency_ms":%d,"ip":"%s"}%s`,
				params.TimeStamp.Format(time.RFC3339),
				params.Method,
				params.Path,
				params.StatusCode,
				params.Latency.Milliseconds(),
				params.ClientIP,
				"\n",
			)
		}),
	))

	json.GET("", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "JSON structured logging",
		})
	})
}

// Example 4: Skip health check paths
func skipPathsExample(r *router.Router) {
	r.Use(logger.New(
		logger.WithSkipPaths("/health", "/metrics"),
	))

	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
		})
	})
}

// Example 5: Integration with RequestID middleware
func requestIDIntegrationExample(r *router.Router) {
	tracked := r.Group("/tracked")

	// RequestID middleware must come before Logger
	tracked.Use(requestid.New())
	tracked.Use(logger.New(
		logger.WithFormatter(func(params logger.FormatterParams) string {
			reqID := params.RequestID
			if reqID == "" {
				reqID = "no-id"
			}
			return fmt.Sprintf("[%s] [%s] %s %s %d\n",
				params.TimeStamp.Format("15:04:05"),
				reqID,
				params.Method,
				params.Path,
				params.StatusCode,
			)
		}),
	))

	tracked.GET("", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Request with ID tracking",
		})
	})
}
