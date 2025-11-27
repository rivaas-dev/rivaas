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

package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"rivaas.dev/router"
	"rivaas.dev/router/version"
)

// This example demonstrates comprehensive API versioning strategies.
// It shows all versioning methods, migration patterns, and deprecation handling.
func main() {
	// Create router with all versioning methods enabled
	r := router.MustNew(
		router.WithVersioning(
			// Header-based versioning (recommended for APIs)
			version.WithHeaderDetection("API-Version"),

			// Query parameter versioning (convenient for testing)
			version.WithQueryDetection("version"),

			// Path-based versioning (e.g., /v1/users, /v2/users)
			version.WithPathDetection("/v{version}/"),

			// Accept header versioning (content negotiation)
			version.WithAcceptDetection("application/vnd.myapi.v{version}+json"),

			// Default version when none specified
			version.WithDefault("v2"),

			// Valid versions (validation)
			version.WithValidVersions("v1", "v2", "v3"),

			// Response headers
			version.WithResponseHeaders(),
			version.WithWarning299(),

			// Observability callbacks
			version.WithObserver(
				version.OnDetected(func(ver, method string) {
					log.Printf("📊 Version detected: %s (method: %s)", ver, method)
				}),
				version.OnMissing(func() {
					log.Println("⚠️  No version specified - using default")
				}),
				version.OnInvalid(func(attempted string) {
					log.Printf("❌ Invalid version attempted: %s", attempted)
				}),
			),
		),
	)

	// ============================================================================
	// Version 1 API (Deprecated)
	// Use lifecycle options on the version to mark it as deprecated
	// ============================================================================
	v1 := r.Version("v1",
		version.Deprecated(),
		version.Sunset(time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)),
		version.MigrationDocs("https://docs.example.com/migrate/v1-to-v2"),
	)
	v1.GET("/users", listUsersV1)
	v1.GET("/users/:id", getUserV1)
	v1.POST("/users", createUserV1)

	// ============================================================================
	// Version 2 API (Current Stable)
	// ============================================================================
	v2 := r.Version("v2")
	v2.GET("/users", listUsersV2)
	v2.GET("/users/:id", getUserV2)
	v2.POST("/users", createUserV2)
	v2.PATCH("/users/:id", updateUserV2) // New in v2: PATCH support

	// ============================================================================
	// Version 3 API (Beta/Preview)
	// ============================================================================
	v3 := r.Version("v3")
	v3.GET("/users", listUsersV3)
	v3.GET("/users/:id", getUserV3)
	v3.POST("/users", createUserV3)
	v3.PATCH("/users/:id", updateUserV3)
	v3.DELETE("/users/:id", deleteUserV3) // New in v3: DELETE support

	// ============================================================================
	// Shared/Unversioned Endpoints
	// ============================================================================
	r.GET("/health", func(c *router.Context) {
		c.JSON(200, map[string]string{"status": "healthy"})
	})

	r.GET("/", showHelp)

	// Start server
	fmt.Println("🚀 API Versioning Demo")
	fmt.Println("====================")
	fmt.Println()
	fmt.Println("Available versions:")
	fmt.Println("  • v1 (deprecated - sunset 2025-12-31)")
	fmt.Println("  • v2 (current stable)")
	fmt.Println("  • v3 (beta)")
	fmt.Println()
	fmt.Println("Versioning Methods:")
	fmt.Println()
	fmt.Println("1. Header-based (recommended):")
	fmt.Println("   curl -H 'API-Version: v2' http://localhost:8080/users")
	fmt.Println()
	fmt.Println("2. Query parameter:")
	fmt.Println("   curl 'http://localhost:8080/users?version=v2'")
	fmt.Println()
	fmt.Println("3. Path-based:")
	fmt.Println("   curl http://localhost:8080/v2/users")
	fmt.Println()
	fmt.Println("4. Accept header (content negotiation):")
	fmt.Println("   curl -H 'Accept: application/vnd.myapi.v2+json' http://localhost:8080/users")
	fmt.Println()
	fmt.Println("5. Default (no version specified - uses v2):")
	fmt.Println("   curl http://localhost:8080/users")
	fmt.Println()
	fmt.Println("Test endpoints:")
	fmt.Println("  GET    /users            - List users")
	fmt.Println("  GET    /users/:id        - Get user")
	fmt.Println("  POST   /users            - Create user")
	fmt.Println("  PATCH  /users/:id        - Update user (v2+)")
	fmt.Println("  DELETE /users/:id        - Delete user (v3 only)")
	fmt.Println()
	fmt.Println("Server listening on :8080")

	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}

// Version 1 Handlers (Original API - Deprecated)

func listUsersV1(c *router.Context) {
	c.JSON(200, map[string]any{
		"version": "v1",
		"users": []map[string]any{
			{"id": 1, "name": "Alice"},
			{"id": 2, "name": "Bob"},
		},
		"deprecation_warning": "API v1 is deprecated and will be sunset on 2025-12-31",
	})
}

func getUserV1(c *router.Context) {
	c.JSON(200, map[string]any{
		"version": "v1",
		"user": map[string]any{
			"id":   c.Param("id"),
			"name": "Alice",
		},
		"deprecation_warning": "API v1 is deprecated and will be sunset on 2025-12-31",
	})
}

func createUserV1(c *router.Context) {
	var req map[string]any
	if err := c.BindStrict(&req, router.BindOptions{MaxBytes: 1 << 20}); err != nil {
		return // Error already written
	}

	c.JSON(201, map[string]any{
		"version": "v1",
		"user": map[string]any{
			"id":   123,
			"name": req["name"],
		},
		"deprecation_warning": "API v1 is deprecated and will be sunset on 2025-12-31",
	})
}

// Version 2 Handlers (Current Stable API)

type UserV2 struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"` // New in v2: email field
	Address struct {
		City    string `json:"city"`
		Country string `json:"country"`
	} `json:"address"` // New in v2: nested address
}

func listUsersV2(c *router.Context) {
	users := []UserV2{
		{ID: 1, Name: "Alice", Email: "alice@example.com"},
		{ID: 2, Name: "Bob", Email: "bob@example.com"},
	}
	users[0].Address.City = "San Francisco"
	users[0].Address.Country = "USA"
	users[1].Address.City = "London"
	users[1].Address.Country = "UK"

	c.JSON(200, map[string]any{
		"version": "v2",
		"users":   users,
		"meta": map[string]any{
			"total": 2,
			"page":  1,
		},
	})
}

func getUserV2(c *router.Context) {
	user := UserV2{
		ID:    1,
		Name:  "Alice",
		Email: "alice@example.com",
	}
	user.Address.City = "San Francisco"
	user.Address.Country = "USA"

	c.JSON(200, map[string]any{
		"version": "v2",
		"user":    user,
	})
}

func createUserV2(c *router.Context) {
	var user UserV2
	if err := c.BindStrict(&user, router.BindOptions{MaxBytes: 1 << 20}); err != nil {
		return // Error already written
	}

	user.ID = 123

	c.JSON(201, map[string]any{
		"version": "v2",
		"user":    user,
		"message": "User created successfully",
	})
}

func updateUserV2(c *router.Context) {
	var updates map[string]any
	if err := c.BindStrict(&updates, router.BindOptions{MaxBytes: 1 << 20}); err != nil {
		return // Error already written
	}

	c.JSON(200, map[string]any{
		"version": "v2",
		"message": "User updated successfully",
		"user": map[string]any{
			"id":      c.Param("id"),
			"updates": updates,
		},
	})
}

// Version 3 Handlers (Beta API)

type UserV3 struct {
	ID       int            `json:"id"`
	Name     string         `json:"name"`
	Email    string         `json:"email"`
	Phone    string         `json:"phone"`              // New in v3: phone field
	Tags     []string       `json:"tags"`               // New in v3: tags
	Metadata map[string]any `json:"metadata,omitempty"` // New in v3: flexible metadata
	Address  struct {
		Street  string `json:"street"` // New in v3: street
		City    string `json:"city"`
		State   string `json:"state"` // New in v3: state
		Zip     string `json:"zip"`   // New in v3: zip
		Country string `json:"country"`
	} `json:"address"`
}

func listUsersV3(c *router.Context) {
	users := []UserV3{
		{
			ID:    1,
			Name:  "Alice",
			Email: "alice@example.com",
			Phone: "+1-555-0100",
			Tags:  []string{"premium", "verified"},
		},
		{
			ID:    2,
			Name:  "Bob",
			Email: "bob@example.com",
			Phone: "+44-555-0200",
			Tags:  []string{"verified"},
		},
	}

	c.JSON(200, map[string]any{
		"version": "v3",
		"users":   users,
		"meta": map[string]any{
			"total":     2,
			"page":      1,
			"page_size": 10,
		},
		"_links": map[string]string{
			"self": "/users",
			"next": "/users?page=2",
		},
	})
}

func getUserV3(c *router.Context) {
	user := UserV3{
		ID:    1,
		Name:  "Alice",
		Email: "alice@example.com",
		Phone: "+1-555-0100",
		Tags:  []string{"premium", "verified"},
		Metadata: map[string]any{
			"signup_date": "2024-01-15",
			"last_login":  "2025-01-20",
			"preferences": map[string]any{
				"theme":         "dark",
				"notifications": true,
			},
		},
	}
	user.Address.Street = "123 Main St"
	user.Address.City = "San Francisco"
	user.Address.State = "CA"
	user.Address.Zip = "94105"
	user.Address.Country = "USA"

	c.JSON(200, map[string]any{
		"version": "v3",
		"user":    user,
		"_links": map[string]string{
			"self":   fmt.Sprintf("/users/%s", c.Param("id")),
			"update": fmt.Sprintf("/users/%s", c.Param("id")),
			"delete": fmt.Sprintf("/users/%s", c.Param("id")),
		},
	})
}

func createUserV3(c *router.Context) {
	var user UserV3
	if err := c.BindStrict(&user, router.BindOptions{MaxBytes: 1 << 20}); err != nil {
		return // Error already written
	}

	user.ID = 123

	c.JSON(201, map[string]any{
		"version": "v3",
		"user":    user,
		"message": "User created successfully",
		"_links": map[string]string{
			"self": fmt.Sprintf("/users/%d", user.ID),
		},
	})
}

func updateUserV3(c *router.Context) {
	var updates UserV3
	if err := c.BindStrict(&updates, router.BindOptions{MaxBytes: 1 << 20}); err != nil {
		return // Error already written
	}

	c.JSON(200, map[string]any{
		"version": "v3",
		"message": "User updated successfully",
		"user": map[string]any{
			"id":      c.Param("id"),
			"updates": updates,
		},
	})
}

func deleteUserV3(c *router.Context) {
	c.JSON(200, map[string]any{
		"version": "v3",
		"message": "User deleted successfully",
		"user_id": c.Param("id"),
	})
}

// Help
func showHelp(c *router.Context) {
	c.JSON(200, map[string]any{
		"message": "API Versioning Demo",
		"versions": map[string]any{
			"v1": map[string]any{
				"status":      "deprecated",
				"sunset_date": "2025-12-31",
				"description": "Original API with basic user fields",
			},
			"v2": map[string]any{
				"status":      "stable",
				"description": "Current API with email and address support",
				"features":    []string{"email", "nested address", "PATCH support"},
			},
			"v3": map[string]any{
				"status":      "beta",
				"description": "Next-gen API with enhanced fields and HATEOAS",
				"features":    []string{"phone", "tags", "metadata", "HATEOAS links", "DELETE support"},
			},
		},
		"versioning_methods": map[string]string{
			"header":  "API-Version: v2",
			"query":   "?version=v2",
			"path":    "/v2/users",
			"accept":  "Accept: application/vnd.myapi.v2+json",
			"default": "defaults to v2 if not specified",
		},
	})
}
