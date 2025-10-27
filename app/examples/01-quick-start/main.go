package main

import (
	"log"
	"net/http"

	"github.com/rivaas-dev/rivaas/app"
	"github.com/rivaas-dev/rivaas/router"
)

func main() {
	// Create a new app with default settings
	a, err := app.New()
	if err != nil {
		log.Fatalf("Failed to create app: %v", err)
	}

	// Register routes
	a.GET("/", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Hello from Rivaas App!",
		})
	})

	a.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
		})
	})

	// Start the server with error handling
	if err := a.Run(":8080"); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
