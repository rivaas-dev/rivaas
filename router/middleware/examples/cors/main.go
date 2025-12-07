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

// Package main provides examples of using the CORS middleware.
package main

import (
	"net/http"
	"os"
	"strings"

	"github.com/charmbracelet/log"

	"rivaas.dev/router"
	"rivaas.dev/router/middleware/cors"
)

func main() {
	r := router.MustNew()

	// Example 1: Basic CORS - Allow specific origins
	basicExample(r)

	// Example 2: Public API - Allow all origins
	publicAPIExample(r)

	// Example 3: Production setup with credentials
	productionExample(r)

	// Example 4: Dynamic origin validation
	dynamicOriginExample(r)

	// Example 5: Preflight request demonstration
	testPreflightExample(r)

	// Create a logger with clean, colorful output
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: false,
		ReportCaller:    false,
	})

	logger.Info("🚀 Server starting on http://localhost:8080")
	logger.Print("")
	logger.Print("📝 Available endpoints:")
	logger.Print("  GET  /basic/api/data        - Basic CORS (allow specific origins)")
	logger.Print("  POST /basic/api/data        - Create data with CORS")
	logger.Print("  GET  /public/api/public     - Public API (allow all origins)")
	logger.Print("  GET  /production/api/user/profile - Production setup with credentials")
	logger.Print("  POST /production/api/user/profile - Production setup with credentials")
	logger.Print("  GET  /dynamic/api/dynamic   - Dynamic origin validation")
	logger.Print("  POST /preflight/api/test    - Preflight request demonstration")
	logger.Print("")
	logger.Print("📋 Example commands:")
	logger.Print("  # Basic CORS")
	logger.Print("  curl -H 'Origin: https://example.com' http://localhost:8080/basic/api/data")
	logger.Print("")
	logger.Print("  # Public API (allows any origin)")
	logger.Print("  curl -H 'Origin: https://anywhere.com' http://localhost:8080/public/api/public")
	logger.Print("")
	logger.Print("  # Production with credentials")
	logger.Print("  curl -H 'Origin: https://app.example.com' -H 'Authorization: Bearer token' http://localhost:8080/production/api/user/profile")
	logger.Print("")
	logger.Print("  # Dynamic origin validation")
	logger.Print("  curl -H 'Origin: https://api.example.com' http://localhost:8080/dynamic/api/dynamic")
	logger.Print("")
	logger.Print("  # Preflight request")
	logger.Print("  curl -X OPTIONS http://localhost:8080/preflight/api/test \\")
	logger.Print("    -H 'Origin: https://example.com' \\")
	logger.Print("    -H 'Access-Control-Request-Method: POST' \\")
	logger.Print("    -v")
	logger.Print("")
	logger.Print("⚠️  WARNING: /public endpoint allows ALL origins - only use for truly public APIs!")
	logger.Print("")

	logger.Fatal(http.ListenAndServe(":8080", r))
}

// basicExample demonstrates basic CORS setup with specific origins
func basicExample(r *router.Router) {
	basic := r.Group("/basic")

	// Configure CORS to allow specific origins
	basic.Use(cors.New(
		cors.WithAllowedOrigins(
			"https://example.com",
			"https://app.example.com",
		),
	))

	basic.GET("/api/data", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]interface{}{
			"message": "This endpoint allows CORS from example.com and app.example.com",
			"data":    []string{"item1", "item2", "item3"},
		})
	})

	basic.POST("/api/data", func(c *router.Context) {
		c.JSON(http.StatusCreated, map[string]interface{}{
			"message": "Data created successfully",
			"id":      "12345",
		})
	})
}

// publicAPIExample demonstrates CORS for a public API that allows all origins
func publicAPIExample(r *router.Router) {
	public := r.Group("/public")

	// WARNING: Only use this for truly public APIs
	public.Use(cors.New(
		cors.WithAllowAllOrigins(true),
	))

	public.GET("/api/public", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]interface{}{
			"message": "This is a public API endpoint",
			"data":    "Anyone can access this",
		})
	})
}

// productionExample demonstrates production-ready CORS with all options
func productionExample(r *router.Router) {
	prod := r.Group("/production")

	// Production-ready CORS configuration
	prod.Use(cors.New(
		// Allow specific origins
		cors.WithAllowedOrigins(
			"https://example.com",
			"https://app.example.com",
			"https://admin.example.com",
		),
		// Restrict methods
		cors.WithAllowedMethods(
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
		),
		// Allow custom headers
		cors.WithAllowedHeaders(
			"Content-Type",
			"Authorization",
			"X-API-Key",
			"X-Request-ID",
		),
		// Expose custom headers to client
		cors.WithExposedHeaders(
			"X-Request-ID",
			"X-Rate-Limit-Remaining",
			"X-Rate-Limit-Reset",
		),
		// Enable credentials (cookies, auth headers)
		cors.WithAllowCredentials(true),
		// Cache preflight for 2 hours
		cors.WithMaxAge(7200),
	))

	prod.GET("/api/user/profile", func(c *router.Context) {
		c.Response.Header().Set("X-Request-ID", "req-12345")
		c.Response.Header().Set("X-Rate-Limit-Remaining", "99")
		c.JSON(http.StatusOK, map[string]interface{}{
			"user": map[string]string{
				"id":    "user123",
				"name":  "John Doe",
				"email": "john@example.com",
			},
		})
	})

	prod.POST("/api/user/profile", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]interface{}{
			"message": "Profile updated successfully",
		})
	})
}

// dynamicOriginExample demonstrates dynamic origin validation
func dynamicOriginExample(r *router.Router) {
	dynamic := r.Group("/dynamic")

	// Use a function to validate origins dynamically
	dynamic.Use(cors.New(
		cors.WithAllowOriginFunc(func(origin string) bool {
			// Allow all subdomains of example.com
			if strings.HasSuffix(origin, ".example.com") {
				return true
			}
			// Allow specific origins
			if origin == "https://example.com" || origin == "https://partner.com" {
				return true
			}
			// You could also check against a database here
			// return db.IsOriginAllowed(origin)
			return false
		}),
		cors.WithAllowCredentials(true),
	))

	dynamic.GET("/api/dynamic", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]interface{}{
			"message": "This endpoint uses dynamic origin validation",
			"allowed": "*.example.com, example.com, partner.com",
		})
	})
}

// testPreflightExample demonstrates how browsers handle preflight requests
func testPreflightExample(r *router.Router) {
	preflight := r.Group("/preflight")

	preflight.Use(cors.New(
		cors.WithAllowedOrigins("https://example.com"),
		cors.WithAllowedMethods(http.MethodGet, http.MethodPost, http.MethodPut),
		cors.WithAllowedHeaders("Content-Type", "Authorization"),
		cors.WithMaxAge(3600),
	))

	preflight.POST("/api/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]interface{}{
			"message": "POST request successful",
		})
	})
}
