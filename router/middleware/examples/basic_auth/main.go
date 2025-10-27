package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/rivaas-dev/rivaas/router"
	"github.com/rivaas-dev/rivaas/router/middleware"
)

func main() {
	r := router.New()

	// Public routes - no authentication required
	r.GET("/", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Welcome! Visit /admin for protected content.",
		})
	})

	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
		})
	})

	// Protected admin routes - Basic Auth required
	admin := r.Group("/admin", middleware.BasicAuth(
		middleware.WithBasicAuthUsers(map[string]string{
			"admin": "secret123",
			"user":  "password456",
		}),
		middleware.WithBasicAuthRealm("Admin Panel"),
	))

	admin.GET("/dashboard", func(c *router.Context) {
		username := middleware.GetAuthUsername(c)
		c.JSON(http.StatusOK, map[string]string{
			"message": fmt.Sprintf("Welcome to admin dashboard, %s!", username),
			"user":    username,
		})
	})

	admin.GET("/settings", func(c *router.Context) {
		username := middleware.GetAuthUsername(c)
		c.JSON(http.StatusOK, map[string]interface{}{
			"user": username,
			"settings": map[string]any{
				"theme":         "dark",
				"notifications": true,
			},
		})
	})

	// Another protected area with different credentials
	api := r.Group("/api", middleware.BasicAuth(
		middleware.WithBasicAuthUsers(map[string]string{
			"apikey1": "secret",
			"apikey2": "token",
		}),
		middleware.WithBasicAuthRealm("API Access"),
		// Skip health check even within API group
		middleware.WithBasicAuthSkipPaths([]string{"/api/health"}),
	))

	api.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "API is healthy",
		})
	})

	api.GET("/data", func(c *router.Context) {
		username := middleware.GetAuthUsername(c)
		c.JSON(http.StatusOK, map[string]interface{}{
			"authenticated_as": username,
			"data":             []string{"item1", "item2", "item3"},
		})
	})

	log.Println("🚀 Server starting on http://localhost:8080")
	log.Println("\n📝 Try these commands:")
	log.Println("\n  # Public route (no auth)")
	log.Println("  curl http://localhost:8080/")
	log.Println("\n  # Protected admin route (will prompt for credentials)")
	log.Println("  curl http://localhost:8080/admin/dashboard")
	log.Println("\n  # With admin credentials")
	log.Println("  curl -u admin:secret123 http://localhost:8080/admin/dashboard")
	log.Println("\n  # With user credentials")
	log.Println("  curl -u user:password456 http://localhost:8080/admin/settings")
	log.Println("\n  # API endpoint with API key")
	log.Println("  curl -u apikey1:secret http://localhost:8080/api/data")
	log.Println("\n  # API health check (no auth required)")
	log.Println("  curl http://localhost:8080/api/health")
	log.Println("\n⚠️  WARNING: Basic Auth transmits credentials in base64 (not encrypted).")
	log.Println("   Always use HTTPS in production!")

	log.Fatal(http.ListenAndServe(":8080", r))
}
