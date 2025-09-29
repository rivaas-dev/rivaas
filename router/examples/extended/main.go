package main

import (
	"log"
	"net/http"

	"github.com/rivaas-dev/rivaas/router"
)

func main() {
	r := router.New()

	// Global middleware
	r.Use(Logger(), Recovery())

	// 1. Route Introspection
	log.Println("=== Registered Routes ===")
	// We'll print routes after adding them all

	// 2. Request/Response Helpers
	r.GET("/helpers", func(c *router.Context) {
		// Test various helper methods
		clientIP := c.GetClientIP()
		isJSON := c.IsJSON()
		isSecure := c.IsSecure()
		userAgent := c.Request.Header.Get("User-Agent")

		c.JSON(http.StatusOK, map[string]interface{}{
			"client_ip":    clientIP,
			"is_json":      isJSON,
			"is_secure":    isSecure,
			"user_agent":   userAgent,
			"accepts_json": c.AcceptsJSON(),
		})
	})

	// Cookie example
	r.GET("/set-cookie", func(c *router.Context) {
		c.SetCookie("session_id", "abc123", 3600, "/", "", false, true)
		c.JSON(http.StatusOK, map[string]string{"message": "Cookie set"})
	})

	r.GET("/get-cookie", func(c *router.Context) {
		sessionID, err := c.GetCookie("session_id")
		if err != nil {
			c.JSON(http.StatusNotFound, map[string]string{"error": "Cookie not found"})
			return
		}
		c.JSON(http.StatusOK, map[string]string{"session_id": sessionID})
	})

	// Redirect example
	r.GET("/redirect", func(c *router.Context) {
		c.Redirect(http.StatusFound, "/helpers")
	})

	// 3. Static File Serving
	r.Static("/assets", "./public")               // Serve ./public/* at /assets/*
	r.StaticFile("/favicon.ico", "./favicon.ico") // Serve single file

	// 4. Route Constraints/Validation

	// Basic numeric constraint
	r.GET("/users/:id", getUserHandler).WhereNumber("id")

	// Custom regex constraint
	r.GET("/files/:filename", getFileHandler).Where("filename", `[a-zA-Z0-9.-]+`)

	// UUID constraint
	r.GET("/entities/:uuid", getEntityHandler).WhereUUID("uuid")

	// Alpha constraint
	r.GET("/categories/:name", getCategoryHandler).WhereAlpha("name")

	// AlphaNumeric constraint
	r.GET("/slugs/:slug", getSlugHandler).WhereAlphaNumeric("slug")

	// Multiple constraints on same route
	r.GET("/posts/:id/:slug", getPostHandler).
		WhereNumber("id").
		WhereAlphaNumeric("slug")

	// API group with constraints
	api := r.Group("/api/v1")
	api.Use(JSONMiddleware())
	{
		// User endpoints with validation
		api.GET("/users/:id", apiGetUserHandler).WhereNumber("id")
		api.PUT("/users/:id", apiUpdateUserHandler).WhereNumber("id")
		api.DELETE("/users/:id", apiDeleteUserHandler).WhereNumber("id")

		// File operations with filename validation
		api.GET("/files/:filename", apiGetFileHandler).Where("filename", `[a-zA-Z0-9._-]+`)
		api.POST("/files/:filename", apiUploadFileHandler).Where("filename", `[a-zA-Z0-9._-]+`)
	}

	// Print all registered routes for demonstration
	log.Println("=== All Registered Routes ===")
	r.PrintRoutes()

	// Get routes programmatically
	routes := r.Routes()
	log.Printf("Total routes registered: %d", len(routes))

	log.Println("Server starting on :8080")
	log.Println("Try these endpoints:")
	log.Println("  GET  /helpers")
	log.Println("  GET  /users/123 (valid: numeric)")
	log.Println("  GET  /users/abc (invalid: not numeric)")
	log.Println("  GET  /files/document.pdf (valid)")
	log.Println("  GET  /files/bad@file.txt (invalid: contains @)")
	log.Println("  GET  /entities/550e8400-e29b-41d4-a716-446655440000 (valid UUID)")
	log.Println("  GET  /categories/technology (valid: alpha)")
	log.Println("  GET  /slugs/my-cool-post123 (valid: alphanumeric)")

	log.Fatal(http.ListenAndServe(":8080", r))
}

// Handler functions
func getUserHandler(c *router.Context) {
	userID := c.Param("id")
	c.JSON(http.StatusOK, map[string]string{
		"message": "User retrieved successfully",
		"user_id": userID,
	})
}

func getFileHandler(c *router.Context) {
	filename := c.Param("filename")
	c.JSON(http.StatusOK, map[string]string{
		"message":  "File retrieved successfully",
		"filename": filename,
	})
}

func getEntityHandler(c *router.Context) {
	uuid := c.Param("uuid")
	c.JSON(http.StatusOK, map[string]string{
		"message": "Entity retrieved successfully",
		"uuid":    uuid,
	})
}

func getCategoryHandler(c *router.Context) {
	name := c.Param("name")
	c.JSON(http.StatusOK, map[string]string{
		"message": "Category retrieved successfully",
		"name":    name,
	})
}

func getSlugHandler(c *router.Context) {
	slug := c.Param("slug")
	c.JSON(http.StatusOK, map[string]string{
		"message": "Slug retrieved successfully",
		"slug":    slug,
	})
}

func getPostHandler(c *router.Context) {
	id := c.Param("id")
	slug := c.Param("slug")
	c.JSON(http.StatusOK, map[string]string{
		"message": "Post retrieved successfully",
		"id":      id,
		"slug":    slug,
	})
}

// API handlers
func apiGetUserHandler(c *router.Context) {
	userID := c.Param("id")
	c.JSON(http.StatusOK, map[string]interface{}{
		"user": map[string]string{
			"id":    userID,
			"name":  "John Doe",
			"email": "john@example.com",
		},
	})
}

func apiUpdateUserHandler(c *router.Context) {
	userID := c.Param("id")
	c.JSON(http.StatusOK, map[string]string{
		"message": "User updated successfully",
		"user_id": userID,
	})
}

func apiDeleteUserHandler(c *router.Context) {
	userID := c.Param("id")
	c.JSON(http.StatusOK, map[string]string{
		"message": "User deleted successfully",
		"user_id": userID,
	})
}

func apiGetFileHandler(c *router.Context) {
	filename := c.Param("filename")
	c.JSON(http.StatusOK, map[string]string{
		"message":  "File metadata retrieved",
		"filename": filename,
		"size":     "1024 bytes",
	})
}

func apiUploadFileHandler(c *router.Context) {
	filename := c.Param("filename")
	c.JSON(http.StatusCreated, map[string]string{
		"message":  "File uploaded successfully",
		"filename": filename,
	})
}

// Middleware functions
func Logger() router.HandlerFunc {
	return func(c *router.Context) {
		log.Printf("[%s] %s %s", c.Request.Method, c.Request.URL.Path, c.GetClientIP())
		c.Next()
	}
}

func Recovery() router.HandlerFunc {
	return func(c *router.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				c.JSON(http.StatusInternalServerError, map[string]string{
					"error": "Internal server error",
				})
			}
		}()
		c.Next()
	}
}

func JSONMiddleware() router.HandlerFunc {
	return func(c *router.Context) {
		c.Header("Content-Type", "application/json")
		c.Next()
	}
}
