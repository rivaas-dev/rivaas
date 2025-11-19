package router_test

import (
	"fmt"
	"net/http"

	"rivaas.dev/router"
)

// ExampleNew demonstrates creating a new router.
func ExampleNew() {
	r, err := router.New()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	r.GET("/", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "Hello World"})
	})

	fmt.Println("Router created successfully")
	// Output: Router created successfully
}

// ExampleMustNew demonstrates creating a router that panics on error.
func ExampleMustNew() {
	r := router.MustNew()

	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	fmt.Println("Router created")
	// Output: Router created
}

// ExampleGET demonstrates registering a GET route.
func ExampleRouter_GET() {
	r := router.MustNew()

	r.GET("/users/:id", func(c *router.Context) {
		userID := c.Param("id")
		c.JSON(http.StatusOK, map[string]string{"user_id": userID})
	})

	fmt.Println("GET route registered")
	// Output: GET route registered
}

// ExamplePOST demonstrates registering a POST route.
func ExampleRouter_POST() {
	r := router.MustNew()

	r.POST("/users", func(c *router.Context) {
		c.JSON(http.StatusCreated, map[string]string{"message": "user created"})
	})

	fmt.Println("POST route registered")
	// Output: POST route registered
}

// ExampleGroup demonstrates creating route groups.
func ExampleRouter_Group() {
	r := router.MustNew()

	// Create API v1 group
	api := r.Group("/api/v1")
	{
		api.GET("/users", func(c *router.Context) {
			c.JSON(http.StatusOK, map[string]string{"version": "v1"})
		})
		api.POST("/users", func(c *router.Context) {
			c.JSON(http.StatusCreated, map[string]string{"version": "v1"})
		})
	}

	fmt.Println("Route group created")
	// Output: Route group created
}

// ExampleUse demonstrates adding middleware.
func ExampleRouter_Use() {
	r := router.MustNew()

	// Add global middleware
	r.Use(func(c *router.Context) {
		// Log request
		fmt.Printf("Request: %s %s\n", c.Request.Method, c.Request.URL.Path)
		c.Next()
	})

	r.GET("/", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "Hello"})
	})

	fmt.Println("Middleware added")
	// Output: Middleware added
}

// ExampleStatic demonstrates serving static files.
func ExampleRouter_Static() {
	r := router.MustNew()

	r.Static("/assets", "./public")
	r.StaticFile("/favicon.ico", "./static/favicon.ico")

	fmt.Println("Static file serving configured")
	// Output: Static file serving configured
}

// ExampleContext_Param demonstrates accessing path parameters.
func ExampleContext_Param() {
	r := router.MustNew()

	r.GET("/users/:id/posts/:postId", func(c *router.Context) {
		userID := c.Param("id")
		postID := c.Param("postId")
		c.JSON(http.StatusOK, map[string]string{
			"user_id": userID,
			"post_id": postID,
		})
	})

	fmt.Println("Route with parameters registered")
	// Output: Route with parameters registered
}

// ExampleContext_Query demonstrates accessing query parameters.
func ExampleContext_Query() {
	r := router.MustNew()

	r.GET("/search", func(c *router.Context) {
		query := c.Query("q")
		page := c.QueryDefault("page", "1")
		c.JSON(http.StatusOK, map[string]string{
			"query": query,
			"page":  page,
		})
	})

	fmt.Println("Query parameter handling configured")
	// Output: Query parameter handling configured
}

// ExampleContext_JSON demonstrates JSON response.
func ExampleContext_JSON() {
	r := router.MustNew()

	r.GET("/data", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]interface{}{
			"name":  "Alice",
			"age":   30,
			"email": "alice@example.com",
		})
	})

	fmt.Println("JSON response handler registered")
	// Output: JSON response handler registered
}
