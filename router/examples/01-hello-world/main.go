package main

import (
	"log"
	"net/http"

	"github.com/rivaas-dev/rivaas/router"
)

func main() {
	// Create a new router
	r := router.New()

	// Define a simple route
	r.GET("/", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Hello, Rivaas!",
		})
	})

	// Start the server
	log.Println("🚀 Server starting on http://localhost:8080")
	log.Println("📝 Try: curl http://localhost:8080/")
	log.Fatal(http.ListenAndServe(":8080", r))
}
