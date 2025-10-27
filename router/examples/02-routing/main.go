package main

import (
	"log"
	"net/http"

	"rivaas.dev/router"
)

func main() {
	r := router.New()

	// Basic routes
	r.GET("/", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Welcome to routing examples!",
		})
	})

	// Route with parameters
	r.GET("/users/:id", func(c *router.Context) {
		userID := c.Param("id")
		c.JSON(http.StatusOK, map[string]string{
			"user_id": userID,
			"name":    "John Doe",
		})
	})

	// Multiple parameters
	r.GET("/users/:id/posts/:post_id", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"user_id": c.Param("id"),
			"post_id": c.Param("post_id"),
		})
	})

	// Different HTTP methods
	r.POST("/users", func(c *router.Context) {
		c.JSON(http.StatusCreated, map[string]string{
			"message": "User created",
		})
	})

	r.PUT("/users/:id", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "User updated",
			"user_id": c.Param("id"),
		})
	})

	r.DELETE("/users/:id", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "User deleted",
			"user_id": c.Param("id"),
		})
	})

	// Route groups
	api := r.Group("/api/v1")
	{
		api.GET("/products", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]interface{}{
				"products": []string{"Product 1", "Product 2", "Product 3"},
			})
		})

		api.GET("/products/:id", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{
				"product_id": c.Param("id"),
				"name":       "Product Name",
			})
		})
	}

	// Admin routes group
	admin := r.Group("/admin")
	{
		admin.GET("/users", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{
				"message": "Admin: List all users",
			})
		})

		admin.POST("/users", func(c *router.Context) {
			c.JSON(http.StatusCreated, map[string]string{
				"message": "Admin: User created",
			})
		})

		admin.GET("/settings", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{
				"message": "Admin: Settings",
			})
		})
	}

	log.Println("🚀 Server starting on http://localhost:8080")
	log.Println("\n📝 Try these commands:")
	log.Println("   curl http://localhost:8080/")
	log.Println("   curl http://localhost:8080/users/123")
	log.Println("   curl http://localhost:8080/users/123/posts/456")
	log.Println("   curl -X POST http://localhost:8080/users")
	log.Println("   curl http://localhost:8080/api/v1/products")
	log.Println("   curl http://localhost:8080/admin/users")

	log.Fatal(http.ListenAndServe(":8080", r))
}
