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

// Package main provides examples of using the BasicAuth middleware.
package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/charmbracelet/log"

	"rivaas.dev/router"
	"rivaas.dev/router/middleware/basicauth"
)

func main() {
	r := router.MustNew()

	// Public routes - no authentication required
	r.GET("/", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Welcome! Visit /admin for protected content.",
		})
	})

	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
		})
	})

	// Protected admin routes - Basic Auth required
	admin := r.Group("/admin", basicauth.New(
		basicauth.WithUsers(map[string]string{
			"admin": "secret123",
			"user":  "password456",
		}),
		basicauth.WithRealm("Admin Panel"),
	))

	admin.GET("/dashboard", func(c *router.Context) {
		username := basicauth.Username(c)
		c.JSON(http.StatusOK, map[string]string{
			"message": fmt.Sprintf("Welcome to admin dashboard, %s!", username),
			"user":    username,
		})
	})

	admin.GET("/settings", func(c *router.Context) {
		username := basicauth.Username(c)
		c.JSON(http.StatusOK, map[string]interface{}{
			"user": username,
			"settings": map[string]any{
				"theme":         "dark",
				"notifications": true,
			},
		})
	})

	// Another protected area with different credentials
	api := r.Group("/api", basicauth.New(
		basicauth.WithUsers(map[string]string{
			"apikey1": "secret",
			"apikey2": "token",
		}),
		basicauth.WithRealm("API Access"),
		// Skip health check even within API group
		basicauth.WithSkipPaths("/api/health"),
	))

	api.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "API is healthy",
		})
	})

	api.GET("/data", func(c *router.Context) {
		username := basicauth.Username(c)
		c.JSON(http.StatusOK, map[string]interface{}{
			"authenticated_as": username,
			"data":             []string{"item1", "item2", "item3"},
		})
	})

	// Create a logger with clean, colorful output
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: false,
		ReportCaller:    false,
	})

	logger.Info("🚀 Server starting on http://localhost:8080")
	logger.Print("")
	logger.Print("📝 Available endpoints:")
	logger.Print("  GET /                    - Public route (no auth)")
	logger.Print("  GET /health              - Public health check")
	logger.Print("  GET /admin/dashboard     - Protected admin route")
	logger.Print("  GET /admin/settings      - Protected admin settings")
	logger.Print("  GET /api/health          - API health (no auth)")
	logger.Print("  GET /api/data            - Protected API endpoint")
	logger.Print("")
	logger.Print("📋 Example commands:")
	logger.Print("  # Public route (no auth)")
	logger.Print("  curl http://localhost:8080/")
	logger.Print("")
	logger.Print("  # Protected admin route (will prompt for credentials)")
	logger.Print("  curl http://localhost:8080/admin/dashboard")
	logger.Print("")
	logger.Print("  # With admin credentials")
	logger.Print("  curl -u admin:secret123 http://localhost:8080/admin/dashboard")
	logger.Print("")
	logger.Print("  # With user credentials")
	logger.Print("  curl -u user:password456 http://localhost:8080/admin/settings")
	logger.Print("")
	logger.Print("  # API endpoint with API key")
	logger.Print("  curl -u apikey1:secret http://localhost:8080/api/data")
	logger.Print("")
	logger.Print("  # API health check (no auth required)")
	logger.Print("  curl http://localhost:8080/api/health")
	logger.Print("")
	logger.Print("⚠️  WARNING: Basic Auth transmits credentials in base64 (not encrypted).")
	logger.Print("   Always use HTTPS in production!")
	logger.Print("")

	logger.Fatal(http.ListenAndServe(":8080", r))
}
