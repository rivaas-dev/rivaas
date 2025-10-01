package main

import (
	"fmt"
	"net/http"

	"github.com/rivaas-dev/rivaas/router"
)

func main() {
	// Create router with advanced routing features
	r := router.New(
		// Configure versioning with header and query parameter support
		router.WithVersioning(
			router.WithHeaderVersioning("API-Version"),     // Header: API-Version: v2
			router.WithQueryVersioning("version"),          // Query: ?version=v1
			router.WithDefaultVersion("v1"),                // Default version
			router.WithValidVersions("v1", "v2", "latest"), // Valid versions
		),
	)

	// Global middleware
	r.Use(Logger())

	// Standard routes (no versioning)
	r.GET("/health", healthHandler)
	r.GET("/status", statusHandler)

	// Enhanced wildcard routes
	r.GET("/files/*", fileHandler)     // Wildcard route for files
	r.GET("/static/*", staticHandler)  // Wildcard route for static assets
	r.GET("/uploads/*", uploadHandler) // Wildcard route for uploads
	r.GET("/docs/*", docsHandler)      // Wildcard route for documentation

	// Version-specific routes
	v1 := r.Version("v1")
	v1.GET("/users", getUsersV1)
	v1.GET("/users/:id", getUserV1)
	v1.POST("/users", createUserV1)

	v2 := r.Version("v2")
	v2.GET("/users", getUsersV2)
	v2.GET("/users/:id", getUserV2)
	v2.POST("/users", createUserV2)

	// Version-specific groups with middleware
	v1API := v1.Group("/api", AuthMiddleware())
	v1API.GET("/profile", getProfileV1)
	v1API.PUT("/profile", updateProfileV1)

	v2API := v2.Group("/api", AuthMiddleware(), RateLimitMiddleware())
	v2API.GET("/profile", getProfileV2)
	v2API.PUT("/profile", updateProfileV2)

	// Start server
	fmt.Println("🚀 Advanced Router Server starting on :8080")
	fmt.Println("📋 Features:")
	fmt.Println("  ✅ Wildcard routes with custom parameter names")
	fmt.Println("  ✅ Route versioning with header/query detection")
	fmt.Println("  ✅ Version-specific route groups")
	fmt.Println("  ✅ Zero-allocation performance")
	fmt.Println("")
	fmt.Println("🔗 Test endpoints:")
	fmt.Println("  GET /health")
	fmt.Println("  GET /files/static/image.jpg")
	fmt.Println("  GET /static/css/style.css")
	fmt.Println("  GET /users (with API-Version: v1 header)")
	fmt.Println("  GET /users (with ?version=v2 query)")
	fmt.Println("")

	http.ListenAndServe(":8080", r)
}

// Global middleware
func Logger() router.HandlerFunc {
	return func(c *router.Context) {
		fmt.Printf("📝 %s %s (version: %s)\n", c.Request.Method, c.Request.URL.Path, c.Version())
		c.Next()
	}
}

func AuthMiddleware() router.HandlerFunc {
	return func(c *router.Context) {
		token := c.Request.Header.Get("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "Missing Authorization header",
			})
			return
		}
		c.Next()
	}
}

func RateLimitMiddleware() router.HandlerFunc {
	return func(c *router.Context) {
		// Simple rate limiting logic
		c.Next()
	}
}

// Standard handlers
func healthHandler(c *router.Context) {
	c.JSON(http.StatusOK, map[string]string{
		"status":  "healthy",
		"version": c.Version(),
	})
}

func statusHandler(c *router.Context) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"service": "advanced-router",
		"version": c.Version(),
		"features": []string{
			"wildcard_routes",
			"route_versioning",
			"zero_allocations",
		},
	})
}

// Enhanced wildcard handlers
func fileHandler(c *router.Context) {
	filepath := c.Param("filepath")
	c.JSON(http.StatusOK, map[string]string{
		"type":    "file",
		"path":    filepath,
		"version": c.Version(),
	})
}

func staticHandler(c *router.Context) {
	asset := c.Param("filepath") // All wildcard routes use "filepath" parameter
	c.JSON(http.StatusOK, map[string]string{
		"type":    "static",
		"asset":   asset,
		"version": c.Version(),
	})
}

func uploadHandler(c *router.Context) {
	filename := c.Param("filepath") // All wildcard routes use "filepath" parameter
	c.JSON(http.StatusOK, map[string]string{
		"type":     "upload",
		"filename": filename,
		"version":  c.Version(),
	})
}

func docsHandler(c *router.Context) {
	path := c.Param("filepath") // All wildcard routes use "filepath" parameter
	c.JSON(http.StatusOK, map[string]string{
		"type":    "documentation",
		"path":    path,
		"version": c.Version(),
	})
}

// Version-specific handlers
func getUsersV1(c *router.Context) {
	c.JSON(http.StatusOK, map[string]interface{}{
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

func getUserV1(c *router.Context) {
	userID := c.Param("id")
	c.JSON(http.StatusOK, map[string]string{
		"version": "v1",
		"id":      userID,
		"name":    "John Doe",
		"email":   "john@example.com",
	})
}

func createUserV1(c *router.Context) {
	c.JSON(http.StatusCreated, map[string]string{
		"version": "v1",
		"message": "User created successfully",
		"id":      "123",
	})
}

func getUsersV2(c *router.Context) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"version": "v2",
		"data": []map[string]string{
			{"id": "1", "name": "John Doe", "email": "john@example.com"},
			{"id": "2", "name": "Jane Smith", "email": "jane@example.com"},
		},
		"meta": map[string]interface{}{
			"pagination": map[string]int{
				"page":  1,
				"limit": 10,
				"total": 2,
			},
			"timestamp": "2024-01-01T00:00:00Z",
		},
	})
}

func getUserV2(c *router.Context) {
	userID := c.Param("id")
	c.JSON(http.StatusOK, map[string]interface{}{
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

func createUserV2(c *router.Context) {
	c.JSON(http.StatusCreated, map[string]interface{}{
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

// Version-specific API handlers
func getProfileV1(c *router.Context) {
	c.JSON(http.StatusOK, map[string]string{
		"version": "v1",
		"profile": "user profile v1",
	})
}

func updateProfileV1(c *router.Context) {
	c.JSON(http.StatusOK, map[string]string{
		"version": "v1",
		"message": "Profile updated v1",
	})
}

func getProfileV2(c *router.Context) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"version": "v2",
		"data": map[string]string{
			"profile": "user profile v2",
		},
		"meta": map[string]string{
			"timestamp": "2024-01-01T00:00:00Z",
		},
	})
}

func updateProfileV2(c *router.Context) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"version": "v2",
		"data": map[string]string{
			"message": "Profile updated v2",
		},
		"meta": map[string]string{
			"timestamp": "2024-01-01T00:00:00Z",
		},
	})
}
