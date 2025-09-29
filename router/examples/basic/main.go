package main

import (
	"log"
	"net/http"
	"time"

	"github.com/rivaas-dev/rivaas/router"
)

func main() {
	r := router.New()

	// Global middleware
	r.Use(Logger(), Recovery())

	// Basic routes
	r.GET("/", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Welcome to Rivaas!",
		})
	})

	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// Parameter routes
	r.GET("/users/:id", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"user_id": c.Param("id"),
		})
	})

	r.GET("/users/:id/posts/:post_id", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"user_id": c.Param("id"),
			"post_id": c.Param("post_id"),
		})
	})

	// Route groups
	api := r.Group("/api/v1")
	api.Use(APIAuth())

	api.GET("/users", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]interface{}{
			"users": []string{"user1", "user2", "user3"},
		})
	})

	api.POST("/users", func(c *router.Context) {
		c.JSON(http.StatusCreated, map[string]string{
			"message": "User created",
		})
	})

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

// Logger middleware
func Logger() router.HandlerFunc {
	return func(c *router.Context) {
		start := time.Now()
		c.Next()
		log.Printf("[%s] %s %s %v", c.Request.Method, c.Request.URL.Path, c.Request.RemoteAddr, time.Since(start))
	}
}

// Recovery middleware
func Recovery() router.HandlerFunc {
	return func(c *router.Context) {
		defer func() {
			if err := recover(); err != nil {
				c.JSON(http.StatusInternalServerError, map[string]string{
					"error": "Internal server error",
				})
			}
		}()
		c.Next()
	}
}

// API Auth middleware
func APIAuth() router.HandlerFunc {
	return func(c *router.Context) {
		auth := c.Request.Header.Get("Authorization")
		if auth == "" {
			c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "Authorization header required",
			})
			return
		}
		c.Next()
	}
}
