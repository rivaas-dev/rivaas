package main

import (
	"log"
	"net/http"

	"rivaas.dev/router"
)

func main() {
	r := router.New()

	// Route Constraints - Validate parameters with regex patterns

	// Numeric constraint - only matches numeric IDs
	r.GET("/users/:id", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "User retrieved",
			"user_id": c.Param("id"),
		})
	}).WhereNumber("id")

	// UUID constraint - only matches valid UUIDs
	r.GET("/entities/:uuid", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Entity retrieved",
			"uuid":    c.Param("uuid"),
		})
	}).WhereUUID("uuid")

	// Alpha constraint - only matches alphabetic characters
	r.GET("/categories/:name", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message":  "Category retrieved",
			"category": c.Param("name"),
		})
	}).WhereAlpha("name")

	// AlphaNumeric constraint - matches alphanumeric characters
	r.GET("/slugs/:slug", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Slug retrieved",
			"slug":    c.Param("slug"),
		})
	}).WhereAlphaNumeric("slug")

	// Custom regex constraint
	r.GET("/files/:filename", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message":  "File retrieved",
			"filename": c.Param("filename"),
		})
	}).Where("filename", `[a-zA-Z0-9._-]+`)

	// Multiple constraints on same route
	r.GET("/posts/:id/:slug", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Post retrieved",
			"id":      c.Param("id"),
			"slug":    c.Param("slug"),
		})
	}).WhereNumber("id").WhereAlphaNumeric("slug")

	// Request/Response Helpers

	r.GET("/helpers", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]interface{}{
			"client_ip":    c.GetClientIP(),
			"is_json":      c.IsJSON(),
			"accepts_json": c.AcceptsJSON(),
			"is_secure":    c.IsSecure(),
			"user_agent":   c.Request.Header.Get("User-Agent"),
		})
	})

	// Cookie handling
	r.GET("/set-cookie", func(c *router.Context) {
		c.SetCookie("session_id", "abc123", 3600, "/", "", false, true)
		c.JSON(http.StatusOK, map[string]string{
			"message": "Cookie set",
		})
	})

	r.GET("/get-cookie", func(c *router.Context) {
		sessionID, err := c.GetCookie("session_id")
		if err != nil {
			c.JSON(http.StatusNotFound, map[string]string{
				"error": "Cookie not found",
			})
			return
		}
		c.JSON(http.StatusOK, map[string]string{
			"session_id": sessionID,
		})
	})

	// Redirect
	r.GET("/redirect", func(c *router.Context) {
		c.Redirect(http.StatusFound, "/helpers")
	})

	// Request headers (alternative to context values)
	r.GET("/context", func(c *router.Context) {
		// Set headers for demonstration
		c.Request.Header.Set("X-User-ID", "12345")
		c.Request.Header.Set("X-User-Role", "admin")

		userID := c.Request.Header.Get("X-User-ID")
		role := c.Request.Header.Get("X-User-Role")

		c.JSON(http.StatusOK, map[string]interface{}{
			"user_id": userID,
			"role":    role,
			"note":    "Using request headers instead of context values",
		})
	})

	// Static File Serving

	// Serve directory at /assets/* (create a 'public' directory for this to work)
	r.Static("/assets", "./public")

	// Serve single file
	r.StaticFile("/favicon.ico", "./favicon.ico")

	// Route Introspection

	// Get all registered routes
	r.GET("/routes", func(c *router.Context) {
		routes := r.Routes()
		c.JSON(http.StatusOK, map[string]interface{}{
			"total_routes": len(routes),
			"routes":       routes,
		})
	})

	// API with constraints
	api := r.Group("/api/v1")
	{
		// User endpoints with ID validation
		api.GET("/users/:id", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{
				"user_id": c.Param("id"),
				"name":    "John Doe",
			})
		}).WhereNumber("id")

		// File operations with filename validation
		api.GET("/files/:filename", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{
				"filename": c.Param("filename"),
			})
		}).Where("filename", `[a-zA-Z0-9._-]+`)

		// Transaction lookup with UUID
		api.GET("/transactions/:txn_id", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{
				"transaction_id": c.Param("txn_id"),
			})
		}).WhereUUID("txn_id")
	}

	// Print all routes
	log.Println("=== Registered Routes ===")
	r.PrintRoutes()

	log.Println("\n🚀 Server starting on http://localhost:8080")
	log.Println("\n📝 Route Constraints:")
	log.Println("   curl http://localhost:8080/users/123           ✓ (numeric)")
	log.Println("   curl http://localhost:8080/users/abc           ✗ (not numeric)")
	log.Println("   curl http://localhost:8080/entities/550e8400-e29b-41d4-a716-446655440000  ✓ (UUID)")
	log.Println("   curl http://localhost:8080/categories/tech     ✓ (alpha)")
	log.Println("   curl http://localhost:8080/slugs/my-post-123   ✓ (alphanumeric)")
	log.Println("   curl http://localhost:8080/files/document.pdf  ✓ (custom regex)")
	log.Println("\n📝 Helpers:")
	log.Println("   curl http://localhost:8080/helpers")
	log.Println("   curl http://localhost:8080/set-cookie")
	log.Println("   curl -b session_id=abc123 http://localhost:8080/get-cookie")
	log.Println("   curl http://localhost:8080/redirect")
	log.Println("\n📝 Introspection:")
	log.Println("   curl http://localhost:8080/routes")

	log.Fatal(http.ListenAndServe(":8080", r))
}
