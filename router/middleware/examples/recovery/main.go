package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"rivaas.dev/router"
	"rivaas.dev/router/middleware"
)

func main() {
	r := router.New()

	// Example 1: Basic recovery with default settings
	basicExample(r)

	// Example 2: Custom recovery handler
	customHandlerExample(r)

	// Example 3: Custom logger with structured logging
	customLoggerExample(r)

	// Example 4: Advanced configuration
	advancedExample(r)

	fmt.Println("Server starting on :8080")
	fmt.Println("Try these endpoints:")
	fmt.Println("  - GET /basic-panic - Basic panic recovery")
	fmt.Println("  - GET /custom-panic - Custom recovery handler")
	fmt.Println("  - GET /logged-panic - Custom logger")
	fmt.Println("  - GET /advanced-panic - Advanced configuration")
	fmt.Println("  - GET /safe - No panic, normal response")

	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}

// Example 1: Basic recovery with default settings
func basicExample(r *router.Router) {
	// Use default recovery middleware
	r.Use(middleware.Recovery())

	r.GET("/basic-panic", func(c *router.Context) {
		// This will panic and be recovered
		panic("Something went wrong!")
	})

	r.GET("/safe", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "This route is safe and won't panic",
		})
	})
}

// Example 2: Custom recovery handler
func customHandlerExample(r *router.Router) {
	api := r.Group("/api")

	// Recovery with custom handler that includes request ID
	api.Use(middleware.Recovery(
		middleware.WithRecoveryHandler(func(c *router.Context, err any) {
			// Send custom error response
			c.JSON(http.StatusInternalServerError, map[string]any{
				"error":   "Internal server error",
				"code":    "INTERNAL_ERROR",
				"message": "An unexpected error occurred. Please try again later.",
				"path":    c.Request.URL.Path,
				"method":  c.Request.Method,
			})
		}),
	))

	api.GET("/custom-panic", func(c *router.Context) {
		// Simulate a panic
		var user map[string]string
		_ = user["name"] // This will panic: assignment to entry in nil map
	})
}

// Example 3: Custom logger with structured logging
func customLoggerExample(r *router.Router) {
	logged := r.Group("/logged")

	// Recovery with custom structured logger
	logged.Use(middleware.Recovery(
		middleware.WithRecoveryLogger(func(c *router.Context, err any, stack []byte) {
			// Structured logging (you could use your own logger here)
			log.Printf(`{"level":"error","type":"panic_recovered","error":%q,"path":"%s","method":"%s","client_ip":"%s","timestamp":"%s","stack":"%s"}`,
				fmt.Sprint(err),
				c.Request.URL.Path,
				c.Request.Method,
				c.ClientIP(),
				time.Now().Format(time.RFC3339),
				string(stack),
			)
		}),
		middleware.WithStackTrace(true),
	))

	logged.GET("/logged-panic", func(c *router.Context) {
		panic("This panic will be logged with structured logging")
	})
}

// Example 4: Advanced configuration
func advancedExample(r *router.Router) {
	advanced := r.Group("/advanced")

	// Recovery with multiple options
	advanced.Use(middleware.Recovery(
		// Enable stack traces
		middleware.WithStackTrace(true),

		// Set custom stack size (8KB)
		middleware.WithStackSize(8<<10),

		// Custom logger
		middleware.WithRecoveryLogger(func(c *router.Context, err any, stack []byte) {
			log.Printf("[PANIC RECOVERED] Error: %v, Path: %s", err, c.Request.URL.Path)
		}),

		// Custom handler with different responses based on error type
		middleware.WithRecoveryHandler(func(c *router.Context, err any) {
			// Different responses based on panic type
			switch e := err.(type) {
			case string:
				c.JSON(http.StatusInternalServerError, map[string]any{
					"error":   "Internal server error",
					"message": "A string error occurred",
					"details": e,
				})
			case error:
				c.JSON(http.StatusInternalServerError, map[string]any{
					"error":   "Internal server error",
					"message": e.Error(),
				})
			default:
				c.JSON(http.StatusInternalServerError, map[string]any{
					"error":   "Internal server error",
					"message": "An unexpected error occurred",
				})
			}
		}),
	))

	advanced.GET("/advanced-panic", func(c *router.Context) {
		// Simulate different types of panics
		panicType := c.Query("type")
		switch panicType {
		case "string":
			panic("This is a string panic")
		case "error":
			panic(fmt.Errorf("this is an error panic"))
		case "nil":
			var data map[string]string
			_ = data["key"] // nil map panic
		default:
			panic("Default panic type")
		}
	})
}

// Example of production-ready setup
func productionExample() *router.Router {
	r := router.New()

	// Production-ready recovery middleware
	r.Use(middleware.Recovery(
		// Capture stack traces
		middleware.WithStackTrace(true),

		// Custom logger (integrate with your logging system)
		middleware.WithRecoveryLogger(func(c *router.Context, err any, stack []byte) {
			// Send to your logging system (e.g., Sentry, DataDog, etc.)
			log.Printf("[PRODUCTION PANIC] Error: %v, Path: %s, Method: %s, IP: %s",
				err,
				c.Request.URL.Path,
				c.Request.Method,
				c.ClientIP(),
			)
		}),

		// Clean error response for clients
		middleware.WithRecoveryHandler(func(c *router.Context, err any) {
			c.JSON(http.StatusInternalServerError, map[string]any{
				"error":   "Internal server error",
				"message": "We're sorry, something went wrong. Please try again later.",
				"code":    "INTERNAL_ERROR",
			})
		}),
	))

	return r
}
