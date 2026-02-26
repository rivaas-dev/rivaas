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

// Package main demonstrates advanced routing features including route constraints, wildcards,
// API versioning, and HTTP method semantics.
package main

import (
	"net/http"
	"os"

	"github.com/charmbracelet/log"

	"rivaas.dev/router"
	"rivaas.dev/router/version"
)

func main() {
	// Create router with versioning support
	r := router.MustNew(
		router.WithVersioning(
			version.WithHeaderDetection("API-Version"),
			version.WithQueryDetection("version"),
			version.WithDefault("v1"),
			version.WithValidVersions("v1", "v2", "latest"),
		),
	)

	// Route Constraints: validate parameters with patterns

	// Integer constraint - only matches integer IDs (maps to OpenAPI integer type)
	r.GET("/users/:id", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "User retrieved",
			"user_id": c.Param("id"),
		})
	}).WhereInt("id")

	// UUID constraint - only matches valid UUIDs (maps to OpenAPI string with format uuid)
	r.GET("/entities/:uuid", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Entity retrieved",
			"uuid":    c.Param("uuid"),
		})
	}).WhereUUID("uuid")

	// Regex constraint - only matches alphabetic characters
	r.GET("/categories/:name", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message":  "Category retrieved",
			"category": c.Param("name"),
		})
	}).WhereRegex("name", `[a-zA-Z]+`)

	// Regex constraint - matches alphanumeric characters
	r.GET("/slugs/:slug", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Slug retrieved",
			"slug":    c.Param("slug"),
		})
	}).WhereRegex("slug", `[a-zA-Z0-9]+`)

	// Custom regex constraint: matches filenames with allowed characters
	r.GET("/files/:filename", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message":  "File retrieved",
			"filename": c.Param("filename"),
		})
	}).WhereRegex("filename", `[a-zA-Z0-9._-]+`)

	// Multiple constraints on same route
	r.GET("/posts/:id/:slug", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Post retrieved",
			"id":      c.Param("id"),
			"slug":    c.Param("slug"),
		})
	}).WhereInt("id").WhereRegex("slug", `[a-zA-Z0-9]+`)

	// Wildcard Routes

	// Wildcard routes for flexible file serving
	r.GET("/files/*", func(c *router.Context) {
		filepath := c.Param("filepath")
		c.JSON(http.StatusOK, map[string]string{
			"type": "file",
			"path": filepath,
		})
	})

	r.GET("/static/*", func(c *router.Context) {
		asset := c.Param("filepath")
		c.JSON(http.StatusOK, map[string]string{
			"type":  "static",
			"asset": asset,
		})
	})

	r.GET("/uploads/*", func(c *router.Context) {
		filename := c.Param("filepath")
		c.JSON(http.StatusOK, map[string]string{
			"type":     "upload",
			"filename": filename,
		})
	})

	// API Versioning

	// Version-specific routes: same paths, different implementations
	v1 := r.Version("v1")
	v1.GET("/users", getUsersV1)
	v1.GET("/users/:id", getUserV1)
	v1.POST("/users", createUserV1)

	v2 := r.Version("v2")
	v2.GET("/users", getUsersV2)
	v2.GET("/users/:id", getUserV2)
	v2.POST("/users", createUserV2)

	// Version-specific groups with middleware
	v1API := v1.Group("/api")
	v1API.GET("/profile", getProfileV1)
	v1API.PUT("/profile", updateProfileV1)

	v2API := v2.Group("/api")
	v2API.GET("/profile", getProfileV2)
	v2API.PUT("/profile", updateProfileV2)

	// HTTP Method Semantics

	// GET - Retrieve resource (idempotent, safe)
	r.GET("/resource", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"method":      "GET",
			"description": "Retrieve resource(s)",
			"idempotent":  true,
			"safe":        true,
		})
	})

	r.GET("/resource/:id", func(c *router.Context) {
		id := c.Param("id")
		c.JSON(http.StatusOK, map[string]any{
			"id":     id,
			"name":   "Resource " + id,
			"status": "active",
		})
	})

	// POST - Create new resource (not idempotent)
	r.POST("/resource", func(c *router.Context) {
		c.Header("Location", "/resource/123")
		c.JSON(http.StatusCreated, map[string]any{
			"method":      "POST",
			"description": "Create new resource",
			"idempotent":  false,
			"safe":        false,
			"message":     "Resource created",
			"id":          "123",
		})
	})

	// PUT - Replace entire resource (idempotent)
	r.PUT("/resource/:id", func(c *router.Context) {
		id := c.Param("id")
		c.JSON(http.StatusOK, map[string]any{
			"method":      "PUT",
			"description": "Replace entire resource",
			"idempotent":  true,
			"safe":        false,
			"message":     "Resource replaced",
			"id":          id,
		})
	})

	// PATCH - Partially update resource (not necessarily idempotent)
	r.PATCH("/resource/:id", func(c *router.Context) {
		id := c.Param("id")
		c.JSON(http.StatusOK, map[string]any{
			"method":      "PATCH",
			"description": "Partially update resource",
			"idempotent":  false,
			"safe":        false,
			"message":     "Resource updated",
			"id":          id,
		})
	})

	// DELETE - Remove resource (idempotent)
	r.DELETE("/resource/:id", func(c *router.Context) {
		id := c.Param("id")
		c.JSON(http.StatusOK, map[string]any{
			"method":      "DELETE",
			"description": "Delete resource",
			"idempotent":  true,
			"safe":        false,
			"message":     "Resource deleted",
			"id":          id,
		})
	})

	// HEAD - Same as GET but only headers (idempotent, safe)
	r.HEAD("/resource/:id", func(c *router.Context) {
		id := c.Param("id")
		c.Header("X-Resource-ID", id)
		c.Header("X-Resource-Status", "active")
		c.Header("Content-Type", "application/json")
		c.Status(http.StatusOK)
	})

	// OPTIONS - Describe communication options (idempotent, safe)
	r.OPTIONS("/resource/:id", func(c *router.Context) {
		c.Header("Allow", "GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Status(http.StatusNoContent)
	})

	// RESTful Collection Example: complete CRUD operations
	r.GET("/items", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"items": []map[string]any{
				{"id": 1, "name": "Item 1"},
				{"id": 2, "name": "Item 2"},
			},
			"total": 2,
		})
	})

	r.POST("/items", func(c *router.Context) {
		c.Header("Location", "/items/3")
		c.JSON(http.StatusCreated, map[string]any{
			"id":   3,
			"name": "Item 3",
		})
	})

	r.GET("/items/:id", func(c *router.Context) {
		id := c.Param("id")
		c.JSON(http.StatusOK, map[string]any{
			"id":   id,
			"name": "Item " + id,
		})
	})

	r.PUT("/items/:id", func(c *router.Context) {
		id := c.Param("id")
		c.JSON(http.StatusOK, map[string]any{
			"id":      id,
			"name":    "Updated Item",
			"updated": true,
		})
	})

	r.PATCH("/items/:id", func(c *router.Context) {
		id := c.Param("id")
		c.JSON(http.StatusOK, map[string]any{
			"id":      id,
			"updated": true,
		})
	})

	r.DELETE("/items/:id", func(c *router.Context) {
		c.Status(http.StatusNoContent)
	})

	// Route Introspection: list all registered routes
	r.GET("/routes", func(c *router.Context) {
		routes := r.Routes()
		c.JSON(http.StatusOK, map[string]any{
			"total_routes": len(routes),
			"routes":       routes,
		})
	})

	// API with Constraints: combining groups and route constraints
	api := r.Group("/api/v1")
	{
		api.GET("/users/:id", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{
				"user_id": c.Param("id"),
				"name":    "John Doe",
			})
		}).WhereInt("id")

		api.GET("/files/:filename", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{
				"filename": c.Param("filename"),
			})
		}).WhereRegex("filename", `[a-zA-Z0-9._-]+`)

		api.GET("/transactions/:txn_id", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{
				"transaction_id": c.Param("txn_id"),
			})
		}).WhereUUID("txn_id")
	}

	// Documentation endpoint
	r.GET("/", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"message": "Advanced Routing Demo",
			"features": []string{
				"Route constraints (UUID, numeric, alpha, regex)",
				"API versioning (header/query based)",
				"Wildcard routes for file serving",
				"Proper HTTP method semantics",
				"Route introspection",
			},
			"endpoints": map[string]string{
				"GET /users/123":                           "Numeric constraint (valid)",
				"GET /users/abc":                           "Numeric constraint (invalid)",
				"GET /entities/:uuid":                      "UUID constraint",
				"GET /files/static/image.jpg":              "Wildcard route",
				"GET /users (with API-Version: v1 header)": "Version v1",
				"GET /users (with ?version=v2 query)":      "Version v2",
				"GET /resource/:id":                        "HTTP methods demo",
				"GET /routes":                              "Route introspection",
			},
		})
	})

	// Create a logger with clean, colorful output
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: false,
		ReportCaller:    false,
	})

	logger.Info("🚀 Server starting on http://localhost:8080")
	logger.Print("")
	logger.Print("📝 Route Constraints:")
	logger.Print("    curl http://localhost:8080/users/123           ✓ (numeric)")
	logger.Print("    curl http://localhost:8080/users/abc           ✗ (not numeric)")
	logger.Print("    curl http://localhost:8080/entities/550e8400-e29b-41d4-a716-446655440000  ✓ (UUID)")
	logger.Print("")
	logger.Print("📝 Wildcards:")
	logger.Print("    curl http://localhost:8080/files/static/image.jpg")
	logger.Print("    curl http://localhost:8080/static/css/style.css")
	logger.Print("")
	logger.Print("📝 Versioning:")
	logger.Print("    curl -H 'API-Version: v1' http://localhost:8080/users")
	logger.Print("    curl 'http://localhost:8080/users?version=v2'")
	logger.Print("")
	logger.Print("📝 HTTP Methods:")
	logger.Print("    curl http://localhost:8080/resource/123")
	logger.Print("    curl -X POST http://localhost:8080/resource")
	logger.Print("    curl -X PUT http://localhost:8080/resource/123")
	logger.Print("    curl -I http://localhost:8080/resource/123  # HEAD")
	logger.Print("    curl -X OPTIONS http://localhost:8080/resource/123")
	logger.Print("")

	logger.Fatal(http.ListenAndServe(":8080", r))
}

// Version-specific handlers demonstrate API versioning

// getUsersV1 returns v1 format users list with pagination
func getUsersV1(c *router.Context) {
	c.JSON(http.StatusOK, map[string]any{
		"version": "v1",
		"users": []map[string]string{
			{"id": "1", "name": "John Doe", "email": "john@example.com"},
			{"id": "2", "name": "Jane Smith", "email": "jane@example.com"},
		},
		"pagination": map[string]int{
			"page":  1,
			"limit": 10,
		},
	})
}

// getUserV1 returns a single user in v1 format
func getUserV1(c *router.Context) {
	userID := c.Param("id")
	c.JSON(http.StatusOK, map[string]string{
		"version": "v1",
		"id":      userID,
		"name":    "John Doe",
		"email":   "john@example.com",
	})
}

// createUserV1 creates a user and returns v1 format response
func createUserV1(c *router.Context) {
	c.JSON(http.StatusCreated, map[string]string{
		"version": "v1",
		"message": "User created successfully",
		"id":      "123",
	})
}

// getUsersV2 returns v2 format users list with metadata wrapper
func getUsersV2(c *router.Context) {
	c.JSON(http.StatusOK, map[string]any{
		"version": "v2",
		"data": []map[string]string{
			{"id": "1", "name": "John Doe", "email": "john@example.com"},
			{"id": "2", "name": "Jane Smith", "email": "jane@example.com"},
		},
		"meta": map[string]any{
			"pagination": map[string]int{
				"page":  1,
				"limit": 10,
				"total": 2,
			},
			"timestamp": "2024-01-01T00:00:00Z",
		},
	})
}

// getUserV2 returns a single user in v2 format with metadata
func getUserV2(c *router.Context) {
	userID := c.Param("id")
	c.JSON(http.StatusOK, map[string]any{
		"version": "v2",
		"data": map[string]string{
			"id":    userID,
			"name":  "John Doe",
			"email": "john@example.com",
		},
		"meta": map[string]string{
			"timestamp": "2024-01-01T00:00:00Z",
		},
	})
}

// createUserV2 creates a user and returns v2 format with metadata
func createUserV2(c *router.Context) {
	c.JSON(http.StatusCreated, map[string]any{
		"version": "v2",
		"data": map[string]string{
			"id":      "123",
			"message": "User created successfully",
		},
		"meta": map[string]string{
			"timestamp": "2024-01-01T00:00:00Z",
		},
	})
}

// getProfileV1 returns user profile in v1 format
func getProfileV1(c *router.Context) {
	c.JSON(http.StatusOK, map[string]string{
		"version": "v1",
		"profile": "user profile v1",
	})
}

// updateProfileV1 updates user profile and returns v1 format
func updateProfileV1(c *router.Context) {
	c.JSON(http.StatusOK, map[string]string{
		"version": "v1",
		"message": "Profile updated v1",
	})
}

// getProfileV2 returns user profile in v2 format with metadata
func getProfileV2(c *router.Context) {
	c.JSON(http.StatusOK, map[string]any{
		"version": "v2",
		"data": map[string]string{
			"profile": "user profile v2",
		},
		"meta": map[string]string{
			"timestamp": "2024-01-01T00:00:00Z",
		},
	})
}

// updateProfileV2 updates user profile and returns v2 format with metadata
func updateProfileV2(c *router.Context) {
	c.JSON(http.StatusOK, map[string]any{
		"version": "v2",
		"data": map[string]string{
			"message": "Profile updated v2",
		},
		"meta": map[string]string{
			"timestamp": "2024-01-01T00:00:00Z",
		},
	})
}
