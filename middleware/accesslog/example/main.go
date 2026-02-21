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
	"log"
	"net/http"

	"rivaas.dev/router"
	"rivaas.dev/middleware/accesslog"
)

func main() {
	r := router.MustNew()

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

	log.Println("Server starting on http://localhost:8080")
	log.Println("Endpoints: /default /custom /json /health /tracked")
	log.Fatal(http.ListenAndServe(":8080", r))
}

// Example 1: Default logging
func defaultLoggingExample(r *router.Router) {
	r.Use(accesslog.New())

	r.GET("/default", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Request logged with default format",
		})
	})
}

// Example 2: Custom format logging
// Note: accesslog uses structured logging, so custom formatting is done via logging configuration
func customFormatExample(r *router.Router) {
	custom := r.Group("/custom")

	custom.Use(accesslog.New())

	custom.GET("", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Structured logging (format controlled by logging config)",
		})
	})
}

// Example 3: JSON structured logging
func jsonLoggingExample(r *router.Router) {
	json := r.Group("/json")

	json.Use(accesslog.New())

	json.GET("", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "JSON structured logging (format controlled by logging config)",
		})
	})
}

// Example 4: Skip health check paths
func skipPathsExample(r *router.Router) {
	r.Use(accesslog.New(
		accesslog.WithExcludePaths("/health", "/metrics"),
	))

	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
		})
	})
}

// Example 5: Deterministic sampling by request ID (e.g. from X-Request-ID header)
func requestIDIntegrationExample(r *router.Router) {
	tracked := r.Group("/tracked")

	tracked.Use(accesslog.New(
		accesslog.WithSampleRate(0.5),
		accesslog.WithRequestIDFunc(func(c *router.Context) string {
			return c.Request.Header.Get("X-Request-ID")
		}),
	))

	tracked.GET("", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Request with ID-based sampling (set X-Request-ID header)",
		})
	})
}
