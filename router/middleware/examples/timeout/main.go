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

// Package main demonstrates how to use the timeout middleware
// to set request timeout limits.
package main

import (
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/log"

	"rivaas.dev/router"
	"rivaas.dev/router/middleware/timeout"
)

func main() {
	r := router.MustNew()

	// Example 1: Basic timeout
	basicTimeoutExample(r)

	// Example 2: Custom timeout handler
	customHandlerExample(r)

	// Example 3: Skip paths (long-running operations)
	skipPathsExample(r)

	// Example 4: Context-aware handler
	contextAwareExample(r)

	// Create a logger with clean, colorful output
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: false,
		ReportCaller:    false,
	})

	logger.Info("🚀 Server starting on http://localhost:8080")
	logger.Print("")
	logger.Print("📝 Available endpoints:")
	logger.Print("  GET /basic        - Basic 2s timeout")
	logger.Print("  GET /custom       - Custom timeout handler")
	logger.Print("  GET /stream       - Skipped from timeout")
	logger.Print("  GET /slow         - Context-aware slow operation")
	logger.Print("")
	logger.Print("📋 Example commands:")
	logger.Print("  curl http://localhost:8080/basic")
	logger.Print("  curl http://localhost:8080/custom")
	logger.Print("  curl http://localhost:8080/slow")
	logger.Print("")
	logger.Print("💡 Tip: For /slow, try waiting longer than 2 seconds to see timeout")
	logger.Print("   Handlers should check c.Request.Context().Done() for cancellation")
	logger.Print("")

	logger.Fatal(http.ListenAndServe(":8080", r))
}

// Example 1: Basic timeout
func basicTimeoutExample(r *router.Router) {
	r.Use(timeout.New(2 * time.Second))

	r.GET("/basic", func(c *router.Context) {
		// Simulate work that completes quickly
		time.Sleep(500 * time.Millisecond)
		c.JSON(http.StatusOK, map[string]string{
			"message": "Request completed within timeout",
			"timeout": "2 seconds",
		})
	})
}

// Example 2: Custom timeout handler
func customHandlerExample(r *router.Router) {
	custom := r.Group("/custom")

	custom.Use(timeout.New(
		3*time.Second,
		timeout.WithHandler(func(c *router.Context) {
			c.JSON(http.StatusRequestTimeout, map[string]any{
				"error":   "Request timeout",
				"code":    "TIMEOUT",
				"message": "The request took too long to process",
				"timeout": "3 seconds",
			})
		}),
	))

	custom.GET("", func(c *router.Context) {
		// Simulate work
		time.Sleep(1 * time.Second)
		c.JSON(http.StatusOK, map[string]string{
			"message": "Request completed",
		})
	})
}

// Example 3: Skip paths for long-running operations
func skipPathsExample(r *router.Router) {
	r.Use(timeout.New(
		2*time.Second,
		timeout.WithSkipPaths("/stream", "/webhook"),
	))

	// This endpoint won't have timeout applied
	r.GET("/stream", func(c *router.Context) {
		// Simulate long-running operation (streaming, webhooks, etc.)
		time.Sleep(5 * time.Second)
		c.JSON(http.StatusOK, map[string]string{
			"message": "Long operation completed (no timeout)",
		})
	})
}

// Example 4: Context-aware handler that respects timeout
func contextAwareExample(r *router.Router) {
	slow := r.Group("/slow")

	slow.Use(timeout.New(2 * time.Second))

	slow.GET("", func(c *router.Context) {
		// Simulate slow work with context checking
		for range 5 {
			// Check if context was cancelled (timeout occurred)
			select {
			case <-c.Request.Context().Done():
				// Context was cancelled - timeout occurred
				// Don't send response (timeout handler already did)
				return
			case <-time.After(500 * time.Millisecond):
				// Continue with work
			}
		}

		// Only reached if we didn't timeout
		c.JSON(http.StatusOK, map[string]string{
			"message": "Slow operation completed successfully",
			"note":    "This would timeout if it took longer than 2 seconds",
		})
	})
}
