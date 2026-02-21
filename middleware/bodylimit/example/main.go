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

// Package main demonstrates how to use the bodylimit middleware
// to restrict the maximum size of request bodies.
package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"rivaas.dev/binding"
	"rivaas.dev/router"
	"rivaas.dev/middleware/bodylimit"
)

func main() {
	r := router.MustNew()

	// Apply body limit middleware globally
	// Default limit is 2MB
	r.Use(bodylimit.New())

	// Example 1: Basic endpoint with body limit
	r.POST("/api/users", func(c *router.Context) {
		var user struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		if err := binding.JSONReaderTo(c.Request.Body, &user); err != nil {
			// Check if error is due to body limit
			if strings.Contains(err.Error(), "exceeds limit") {
				c.JSON(http.StatusRequestEntityTooLarge, map[string]string{
					"error": "Request body too large",
				})
				return
			}
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, map[string]string{
			"message": "User created",
			"name":    user.Name,
			"email":   user.Email,
		})
	})

	// Example 2: Custom body limit size (1MB)
	r.POST("/api/documents", bodylimit.New(
		bodylimit.WithLimit(1024*1024), // 1MB
	), func(c *router.Context) {
		var doc struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		}

		if err := binding.JSONReaderTo(c.Request.Body, &doc); err != nil {
			if strings.Contains(err.Error(), "exceeds limit") {
				c.JSON(http.StatusRequestEntityTooLarge, map[string]string{
					"error": "Document too large (max 1MB)",
				})
				return
			}
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, map[string]any{
			"message":      "Document received",
			"title":        doc.Title,
			"content_size": len(doc.Content),
		})
	})

	// Example 3: Skipping body limit for specific paths (upload endpoint)
	r.POST("/api/upload", bodylimit.New(
		bodylimit.WithLimit(10*1024*1024), // 10MB
		bodylimit.WithSkipPaths("/api/upload"),
	), func(c *router.Context) {
		// This endpoint doesn't have body limit applied due to skip path
		// Note: Skip paths must match exactly
		c.JSON(http.StatusOK, map[string]string{
			"message": "Upload endpoint - no body limit",
		})
	})

	// Actually, let's show a proper upload endpoint without skip
	r.POST("/api/upload-large", bodylimit.New(
		bodylimit.WithLimit(50*1024*1024), // 50MB for large uploads
		bodylimit.WithErrorHandler(func(c *router.Context, limit int64) {
			c.JSON(http.StatusRequestEntityTooLarge, map[string]string{
				"error":    "File too large",
				"max_size": fmt.Sprintf("%.0fMB", float64(limit)/(1024*1024)),
			})
		}),
	), func(c *router.Context) {
		var data map[string]interface{}
		if err := binding.JSONReaderTo(c.Request.Body, &data); err != nil {
			if strings.Contains(err.Error(), "exceeds limit") {
				// Error handler already called, but we can add additional logic
				return
			}
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, map[string]string{
			"message": "Large file uploaded",
		})
	})

	// Example 4: Custom error handler
	r.POST("/api/data", bodylimit.New(
		bodylimit.WithLimit(512*1024), // 512KB
		bodylimit.WithErrorHandler(func(c *router.Context, limit int64) {
			// Custom error response
			c.Header("X-Error-Type", "BodyLimitExceeded")
			c.Stringf(http.StatusRequestEntityTooLarge, "Payload too large. Maximum allowed: %d bytes", limit)
		}),
	), func(c *router.Context) {
		var data map[string]interface{}
		if err := binding.JSONReaderTo(c.Request.Body, &data); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, map[string]string{
			"message": "Data received",
		})
	})

	log.Println("Server starting on :8080")
	log.Println("\nTest the endpoints:")
	log.Println("  # Small body (should work):")
	log.Println("  curl -X POST http://localhost:8080/api/users \\")
	log.Println("    -H 'Content-Type: application/json' \\")
	log.Println("    -d '{\"name\":\"John\",\"email\":\"john@example.com\"}'")
	log.Println()
	log.Println("  # Large body (should fail with 413):")
	log.Println("  curl -X POST http://localhost:8080/api/users \\")
	log.Println("    -H 'Content-Type: application/json' \\")
	log.Println("    -d '{\"data\":\"" + strings.Repeat("x", 3*1024*1024) + "\"}'")
	log.Println()
	log.Println("  # Test with Content-Length header:")
	log.Println("  curl -X POST http://localhost:8080/api/users \\")
	log.Println("    -H 'Content-Type: application/json' \\")
	log.Println("    -H 'Content-Length: 5000000' \\")
	log.Println("    -d '{\"test\":\"data\"}'")
	log.Println()
	log.Println("  # Test custom error handler:")
	log.Println("  curl -X POST http://localhost:8080/api/data \\")
	log.Println("    -H 'Content-Type: application/json' \\")
	log.Println("    -d '{\"data\":\"" + strings.Repeat("x", 600*1024) + "\"}'")

	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
