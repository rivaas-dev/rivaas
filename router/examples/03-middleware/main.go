package main

import (
	"log"
	"net/http"
	"time"

	"rivaas.dev/router"
)

func main() {
	r := router.New()

	// Global middleware - applies to all routes
	r.Use(Logger(), Recovery())

	// Public routes
	r.GET("/", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Public endpoint",
		})
	})

	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// Protected API with authentication middleware
	api := r.Group("/api")
	api.Use(AuthMiddleware()) // Only applies to /api/* routes
	{
		api.GET("/profile", func(c *router.Context) {
			// Get user from context (set by auth middleware)
			// Note: Context doesn't have Get/Set methods, using request headers instead
			userID := c.Request.Header.Get("X-User-ID")
			userRole := c.Request.Header.Get("X-User-Role")
			c.JSON(http.StatusOK, map[string]interface{}{
				"user_id": userID,
				"role":    userRole,
			})
		})

		api.GET("/settings", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{
				"message": "User settings",
			})
		})
	}

	// Admin routes with additional middleware
	admin := r.Group("/admin")
	admin.Use(AuthMiddleware(), AdminMiddleware()) // Multiple middleware
	{
		admin.GET("/users", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{
				"message": "Admin: All users",
			})
		})

		admin.DELETE("/users/:id", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{
				"message": "Admin: User deleted",
				"user_id": c.Param("id"),
			})
		})
	}

	// CORS example - handle CORS manually to avoid WriteHeader conflicts
	cors := r.Group("/cors")
	{
		cors.GET("/data", func(c *router.Context) {
			// Set CORS headers manually in the handler
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

			c.JSON(http.StatusOK, map[string]string{
				"message": "CORS enabled endpoint",
			})
		})

		cors.OPTIONS("/data", func(c *router.Context) {
			// Handle OPTIONS requests manually
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.String(http.StatusOK, "")
		})
	}

	// Panic recovery example
	r.GET("/panic", func(c *router.Context) {
		panic("This will be caught by Recovery middleware!")
	})

	log.Println("🚀 Server starting on http://localhost:8080")
	log.Println("\n📝 Try these commands:")
	log.Println("   curl http://localhost:8080/")
	log.Println("   curl http://localhost:8080/api/profile")
	log.Println("   curl -H 'Authorization: Bearer token123' http://localhost:8080/api/profile")
	log.Println("   curl -H 'Authorization: Bearer admin-token' http://localhost:8080/admin/users")
	log.Println("   curl http://localhost:8080/panic")

	log.Fatal(http.ListenAndServe(":8080", r))
}

// Logger middleware logs request details
func Logger() router.HandlerFunc {
	return func(c *router.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		duration := time.Since(start)
		// Note: Status code tracking would need to be implemented differently
		// For now, we'll just log the request without status code
		log.Printf("[%s] %s %s - (%v)",
			c.Request.Method,
			path,
			c.ClientIP(),
			duration,
		)
	}
}

// Recovery middleware recovers from panics
func Recovery() router.HandlerFunc {
	return func(c *router.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("❌ Panic recovered: %v", err)
				c.JSON(http.StatusInternalServerError, map[string]string{
					"error": "Internal server error",
				})
			}
		}()
		c.Next()
	}
}

// AuthMiddleware checks for authorization
func AuthMiddleware() router.HandlerFunc {
	return func(c *router.Context) {
		token := c.Request.Header.Get("Authorization")

		if token == "" {
			c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "Missing authorization header",
			})
			return
		}

		// Simulate token validation
		if token == "Bearer token123" || token == "Bearer admin-token" {
			// Set user info in request headers for downstream handlers
			c.Request.Header.Set("X-User-ID", "123")
			c.Request.Header.Set("X-User-Name", "John Doe")
			if token == "Bearer admin-token" {
				c.Request.Header.Set("X-User-Role", "admin")
			} else {
				c.Request.Header.Set("X-User-Role", "user")
			}
			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "Invalid token",
			})
		}
	}
}

// AdminMiddleware checks for admin privileges
func AdminMiddleware() router.HandlerFunc {
	return func(c *router.Context) {
		token := c.Request.Header.Get("Authorization")

		if token != "Bearer admin-token" {
			c.JSON(http.StatusForbidden, map[string]string{
				"error": "Admin access required",
			})
			return
		}

		c.Next()
	}
}

// CORS middleware adds CORS headers
func CORS() router.HandlerFunc {
	return func(c *router.Context) {
		// Set CORS headers
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			// For OPTIONS requests, handle them directly without calling Next()
			// This prevents the WriteHeader conflict
			c.String(http.StatusOK, "")
			return
		}

		// For other methods, continue to the handler
		c.Next()
	}
}
