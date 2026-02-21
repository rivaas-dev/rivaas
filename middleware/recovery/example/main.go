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

// Package main demonstrates the recovery middleware with various configuration options.
// It shows how to handle panics gracefully in different scenarios from basic
// usage to production-ready setups with custom handlers and structured logging.
package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"rivaas.dev/router"
	"rivaas.dev/router/middleware/recovery"
)

func main() {
	r := router.MustNew()

	// Example 1: Basic recovery with default settings
	basicExample(r)

	// Example 2: Custom recovery handler
	customHandlerExample(r)

	// Example 3: Custom logger with structured logging
	customLoggerExample(r)

	// Example 4: Advanced configuration
	advancedExample(r)

	// Example 5: Production-ready setup (reference implementation)
	productionExample(r)

	log.Println("Server starting on http://localhost:8080")
	log.Println("Endpoints: /basic-panic /api/custom-panic /logged/logged-panic /advanced/advanced-panic /production/panic /safe")
	log.Fatal(http.ListenAndServe(":8080", r))
}

// Example 1: Basic recovery with default settings
func basicExample(r *router.Router) {
	// Use default recovery middleware
	r.Use(recovery.New())

	r.GET("/basic-panic", func(_ *router.Context) {
		// This will panic and be recovered
		panic("Something went wrong!")
	})

	r.GET("/safe", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "This route is safe and won't panic",
		})
	})
}

// Example 2: Custom recovery handler
func customHandlerExample(r *router.Router) {
	api := r.Group("/api")

	// Recovery with custom handler that includes request ID
	api.Use(recovery.New(
		recovery.WithHandler(func(c *router.Context, _ any) {
			// Send custom error response
			c.JSON(http.StatusInternalServerError, map[string]any{
				"error":   "Internal server error",
				"code":    "INTERNAL_ERROR",
				"message": "An unexpected error occurred. Please try again later.",
				"path":    c.Request.URL.Path,
				"method":  c.Request.Method,
			})
		}),
	))

	api.GET("/custom-panic", func(_ *router.Context) {
		// Simulate a panic
		var user map[string]string
		_ = user["name"] // This will panic: assignment to entry in nil map
	})
}

// Example 3: Custom logger with structured logging
func customLoggerExample(r *router.Router) {
	logged := r.Group("/logged")

	// Create a custom slog logger with JSON output
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	// Recovery with custom structured logger
	logged.Use(recovery.New(
		recovery.WithLogger(logger),
		recovery.WithStackTrace(true),
	))

	logged.GET("/logged-panic", func(_ *router.Context) {
		panic("This panic will be logged with structured logging")
	})
}

// Example 4: Advanced configuration
func advancedExample(r *router.Router) {
	advanced := r.Group("/advanced")

	// Recovery with multiple options
	advanced.Use(recovery.New(
		// Enable stack traces
		recovery.WithStackTrace(true),

		// Set custom stack size (8KB)
		recovery.WithStackSize(8<<10),

		// Custom handler with different responses based on error type
		recovery.WithHandler(func(c *router.Context, err any) {
			// Different responses based on panic type
			switch e := err.(type) {
			case string:
				c.JSON(http.StatusInternalServerError, map[string]any{
					"error":   "Internal server error",
					"message": "A string error occurred",
					"details": e,
				})
			case error:
				c.JSON(http.StatusInternalServerError, map[string]any{
					"error":   "Internal server error",
					"message": e.Error(),
				})
			default:
				c.JSON(http.StatusInternalServerError, map[string]any{
					"error":   "Internal server error",
					"message": "An unexpected error occurred",
				})
			}
		}),
	))

	advanced.GET("/advanced-panic", func(c *router.Context) {
		// Simulate different types of panics
		panicType := c.Query("type")
		switch panicType {
		case "string":
			panic("This is a string panic")
		case "error":
			panic(fmt.Errorf("this is an error panic"))
		case "nil":
			var data map[string]string
			_ = data["key"] // nil map panic
		default:
			panic("Default panic type")
		}
	})
}

// Example 5: Production-ready setup
func productionExample(r *router.Router) {
	prod := r.Group("/production")

	// Production-ready recovery middleware
	prod.Use(recovery.New(
		// Capture stack traces
		recovery.WithStackTrace(true),

		// Clean error response for clients
		recovery.WithHandler(func(c *router.Context, _ any) {
			c.JSON(http.StatusInternalServerError, map[string]any{
				"error":   "Internal server error",
				"message": "We're sorry, something went wrong. Please try again later.",
				"code":    "INTERNAL_ERROR",
			})
		}),
	))

	prod.GET("/panic", func(_ *router.Context) {
		panic("Production recovery example - this will be handled gracefully")
	})
}
