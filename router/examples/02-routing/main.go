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

// Package main demonstrates routing fundamentals with the Rivaas router.
// This example covers basic routes, path parameters, HTTP methods, and route groups.
package main

import (
	"net/http"
	"os"

	"github.com/charmbracelet/log"

	"rivaas.dev/router"
)

func main() {
	r := router.MustNew()

	// Basic route: simple GET endpoint returning JSON
	r.GET("/", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Welcome to routing examples!",
		})
	})

	// Path parameters: extract dynamic values from URL path
	// Example: GET /users/123 will extract id="123"
	r.GET("/users/:id", func(c *router.Context) {
		userID := c.Param("id")
		c.JSON(http.StatusOK, map[string]string{
			"user_id": userID,
			"name":    "John Doe",
		})
	})

	// Multiple path parameters: handle nested resources
	// Example: GET /users/123/posts/456 extracts both id and post_id
	r.GET("/users/:id/posts/:post_id", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"user_id": c.Param("id"),
			"post_id": c.Param("post_id"),
		})
	})

	// HTTP methods: demonstrate RESTful operations
	// POST creates a new resource (201 Created)
	r.POST("/users", func(c *router.Context) {
		c.JSON(http.StatusCreated, map[string]string{
			"message": "User created",
		})
	})

	// PUT updates an existing resource (200 OK)
	r.PUT("/users/:id", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "User updated",
			"user_id": c.Param("id"),
		})
	})

	// DELETE removes a resource (200 OK)
	r.DELETE("/users/:id", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "User deleted",
			"user_id": c.Param("id"),
		})
	})

	// Route groups: organize related routes under a common prefix
	// All routes in this group are prefixed with /api
	api := r.Group("/api")
	{
		// GET /api/products - returns a list of products
		api.GET("/products", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]any{
				"products": []string{"Product 1", "Product 2", "Product 3"},
			})
		})

		// GET /api/products/:id - returns a single product
		api.GET("/products/:id", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{
				"product_id": c.Param("id"),
				"name":       "Product Name",
			})
		})
	}

	// Nested groups: create hierarchical route structures
	// All routes here are prefixed with /admin
	admin := r.Group("/admin")
	{
		// GET /admin/users - admin-only endpoint
		admin.GET("/users", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{
				"message": "Admin: List all users",
			})
		})

		// POST /admin/users - admin-only creation endpoint
		admin.POST("/users", func(c *router.Context) {
			c.JSON(http.StatusCreated, map[string]string{
				"message": "Admin: User created",
			})
		})

		// GET /admin/settings - admin configuration endpoint
		admin.GET("/settings", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{
				"message": "Admin: Settings",
			})
		})
	}

	// Create a logger with clean, colorful output
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: false,
		ReportCaller:    false,
	})

	logger.Info("🚀 Server starting on http://localhost:8080")
	logger.Print("")
	logger.Print("📝 Available endpoints:")
	logger.Print("  GET    /")
	logger.Print("  GET    /users/:id")
	logger.Print("  GET    /users/:id/posts/:post_id")
	logger.Print("  POST   /users")
	logger.Print("  PUT    /users/:id")
	logger.Print("  DELETE /users/:id")
	logger.Print("  GET    /api/products")
	logger.Print("  GET    /api/products/:id")
	logger.Print("  GET    /admin/users")
	logger.Print("  POST   /admin/users")
	logger.Print("  GET    /admin/settings")
	logger.Print("")
	logger.Print("📋 Example commands:")
	logger.Print("  curl http://localhost:8080/")
	logger.Print("  curl http://localhost:8080/users/123")
	logger.Print("  curl http://localhost:8080/users/123/posts/456")
	logger.Print("  curl -X POST http://localhost:8080/users")
	logger.Print("  curl -X PUT http://localhost:8080/users/123")
	logger.Print("  curl http://localhost:8080/api/products")
	logger.Print("  curl http://localhost:8080/admin/users")
	logger.Print("")
	logger.Print("💡 Tip: Use app.PrintRoutes() for beautiful formatted route tables")
	logger.Print("")

	logger.Fatal(http.ListenAndServe(":8080", r))
}
