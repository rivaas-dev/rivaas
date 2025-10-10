package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/rivaas-dev/rivaas/router"
	"github.com/rivaas-dev/rivaas/router/middleware"
)

func main() {
	// Example 1: Basic CORS - Allow specific origins
	basicExample()

	// Example 2: Public API - Allow all origins
	// publicAPIExample()

	// Example 3: Production setup with credentials
	// productionExample()

	// Example 4: Dynamic origin validation
	// dynamicOriginExample()
}

// basicExample demonstrates basic CORS setup with specific origins
func basicExample() {
	r := router.New()

	// Configure CORS to allow specific origins
	r.Use(middleware.CORS(
		middleware.WithAllowedOrigins([]string{
			"https://example.com",
			"https://app.example.com",
		}),
	))

	r.GET("/api/data", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]interface{}{
			"message": "This endpoint allows CORS from example.com and app.example.com",
			"data":    []string{"item1", "item2", "item3"},
		})
	})

	r.POST("/api/data", func(c *router.Context) {
		c.JSON(http.StatusCreated, map[string]interface{}{
			"message": "Data created successfully",
			"id":      "12345",
		})
	})

	log.Println("Basic CORS example running on :8080")
	log.Println("Try: curl -H 'Origin: https://example.com' http://localhost:8080/api/data")
	log.Fatal(http.ListenAndServe(":8080", r))
}

// publicAPIExample demonstrates CORS for a public API that allows all origins
func publicAPIExample() {
	r := router.New()

	// WARNING: Only use this for truly public APIs
	r.Use(middleware.CORS(
		middleware.WithAllowAllOrigins(true),
	))

	r.GET("/api/public", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]interface{}{
			"message": "This is a public API endpoint",
			"data":    "Anyone can access this",
		})
	})

	log.Println("Public API CORS example running on :8081")
	log.Println("Try: curl -H 'Origin: https://anywhere.com' http://localhost:8081/api/public")
	log.Fatal(http.ListenAndServe(":8081", r))
}

// productionExample demonstrates production-ready CORS with all options
func productionExample() {
	r := router.New()

	// Production-ready CORS configuration
	r.Use(middleware.CORS(
		// Allow specific origins
		middleware.WithAllowedOrigins([]string{
			"https://example.com",
			"https://app.example.com",
			"https://admin.example.com",
		}),
		// Restrict methods
		middleware.WithAllowedMethods([]string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
		}),
		// Allow custom headers
		middleware.WithAllowedHeaders([]string{
			"Content-Type",
			"Authorization",
			"X-API-Key",
			"X-Request-ID",
		}),
		// Expose custom headers to client
		middleware.WithExposedHeaders([]string{
			"X-Request-ID",
			"X-Rate-Limit-Remaining",
			"X-Rate-Limit-Reset",
		}),
		// Enable credentials (cookies, auth headers)
		middleware.WithAllowCredentials(true),
		// Cache preflight for 2 hours
		middleware.WithMaxAge(7200),
	))

	r.GET("/api/user/profile", func(c *router.Context) {
		c.Response.Header().Set("X-Request-ID", "req-12345")
		c.Response.Header().Set("X-Rate-Limit-Remaining", "99")
		c.JSON(http.StatusOK, map[string]interface{}{
			"user": map[string]string{
				"id":    "user123",
				"name":  "John Doe",
				"email": "john@example.com",
			},
		})
	})

	r.POST("/api/user/profile", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]interface{}{
			"message": "Profile updated successfully",
		})
	})

	log.Println("Production CORS example running on :8082")
	log.Println("Features: credentials, custom headers, restricted methods")
	log.Println("Try: curl -H 'Origin: https://app.example.com' -H 'Authorization: Bearer token' http://localhost:8082/api/user/profile")
	log.Fatal(http.ListenAndServe(":8082", r))
}

// dynamicOriginExample demonstrates dynamic origin validation
func dynamicOriginExample() {
	r := router.New()

	// Use a function to validate origins dynamically
	r.Use(middleware.CORS(
		middleware.WithAllowOriginFunc(func(origin string) bool {
			// Allow all subdomains of example.com
			if strings.HasSuffix(origin, ".example.com") {
				return true
			}
			// Allow specific origins
			if origin == "https://example.com" || origin == "https://partner.com" {
				return true
			}
			// You could also check against a database here
			// return db.IsOriginAllowed(origin)
			return false
		}),
		middleware.WithAllowCredentials(true),
	))

	r.GET("/api/dynamic", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]interface{}{
			"message": "This endpoint uses dynamic origin validation",
			"allowed": "*.example.com, example.com, partner.com",
		})
	})

	log.Println("Dynamic origin validation example running on :8083")
	log.Println("Allowed: *.example.com, example.com, partner.com")
	log.Println("Try: curl -H 'Origin: https://api.example.com' http://localhost:8083/api/dynamic")
	log.Println("Try: curl -H 'Origin: https://subdomain.example.com' http://localhost:8083/api/dynamic")
	log.Fatal(http.ListenAndServe(":8083", r))
}

// testPreflightExample demonstrates how browsers handle preflight requests
func testPreflightExample() {
	r := router.New()

	r.Use(middleware.CORS(
		middleware.WithAllowedOrigins([]string{"https://example.com"}),
		middleware.WithAllowedMethods([]string{http.MethodGet, http.MethodPost, http.MethodPut}),
		middleware.WithAllowedHeaders([]string{"Content-Type", "Authorization"}),
		middleware.WithMaxAge(3600),
	))

	r.POST("/api/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]interface{}{
			"message": "POST request successful",
		})
	})

	log.Println("Preflight test example running on :8084")
	log.Println("\nTo test preflight (OPTIONS) request:")
	log.Println("curl -X OPTIONS http://localhost:8084/api/test \\")
	log.Println("  -H 'Origin: https://example.com' \\")
	log.Println("  -H 'Access-Control-Request-Method: POST' \\")
	log.Println("  -H 'Access-Control-Request-Headers: Content-Type' \\")
	log.Println("  -v")
	log.Println("\nThen actual request:")
	log.Println("curl -X POST http://localhost:8084/api/test \\")
	log.Println("  -H 'Origin: https://example.com' \\")
	log.Println("  -H 'Content-Type: application/json' \\")
	log.Println("  -d '{\"data\":\"test\"}' \\")
	log.Println("  -v")
	log.Fatal(http.ListenAndServe(":8084", r))
}
