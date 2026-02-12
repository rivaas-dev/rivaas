// Copyright 2025 The Rivaas Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !integration

package router_test

import (
	"errors"
	"fmt"
	"log/slog"
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
		//nolint:errcheck // Example code, we don't care about the error here
		c.JSON(http.StatusOK, map[string]string{"message": "Hello World"})
	})

	fmt.Println("Router created successfully")
	// Output: Router created successfully
}

// ExampleMustNew demonstrates creating a router that panics on error.
func ExampleMustNew() {
	r := router.MustNew()

	r.GET("/health", func(c *router.Context) {
		//nolint:errcheck // Example code, we don't care about the error here
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
		//nolint:errcheck // Example code, we don't care about the error here
		c.JSON(http.StatusOK, map[string]string{"user_id": userID})
	})

	fmt.Println("GET route registered")
	// Output: GET route registered
}

// ExamplePOST demonstrates registering a POST route.
func ExampleRouter_POST() {
	r := router.MustNew()

	r.POST("/users", func(c *router.Context) {
		//nolint:errcheck // Example code, we don't care about the error here
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
			//nolint:errcheck // Example code, we don't care about the error here
			c.JSON(http.StatusOK, map[string]string{"version": "v1"})
		})
		api.POST("/users", func(c *router.Context) {
			//nolint:errcheck // Example code, we don't care about the error here
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
		//nolint:errcheck // Example code, we don't care about the error here
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
		//nolint:errcheck // Example code, we don't care about the error here
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
		//nolint:errcheck // Example code, we don't care about the error here
		c.JSON(http.StatusOK, map[string]string{
			"query": query,
			"page":  page,
		})
	})

	fmt.Println("Query parameter handling configured")
	// Output: Query parameter handling configured
}

// ExampleContext_Error demonstrates error collection.
func ExampleContext_Error() {
	r := router.MustNew()

	r.POST("/users", func(c *router.Context) {
		// Collect validation errors
		if userID := c.Param("id"); userID == "" {
			//nolint:errcheck // Example code, we don't care about the error here
			c.Error(errors.New("user ID is required"))
		}
		if email := c.Query("email"); email == "" {
			//nolint:errcheck // Example code, we don't care about the error here
			c.Error(errors.New("email is required"))
		}

		// Check if any errors were collected
		if c.HasErrors() {
			//nolint:errcheck // Example code, we don't care about the error here
			c.JSON(http.StatusBadRequest, map[string]any{
				"errors": c.Errors(),
			})

			return
		}

		//nolint:errcheck // Example code, we don't care about the error here
		c.JSON(http.StatusOK, map[string]string{"status": "created"})
	})

	fmt.Println("Error collection handler registered")
	// Output: Error collection handler registered
}

// ExampleContext_Errors demonstrates retrieving collected errors.
func ExampleContext_Errors() {
	r := router.MustNew()

	r.POST("/validate", func(c *router.Context) {
		// Collect multiple errors
		c.Error(errors.New("validation error 1"))
		c.Error(errors.New("validation error 2"))

		// Retrieve all errors
		errors := c.Errors()
		fmt.Printf("Collected %d errors\n", len(errors))

		// Process errors individually
		for i, err := range errors {
			fmt.Printf("Error %d: %v\n", i+1, err)
		}
	})

	fmt.Println("Error retrieval handler registered")
	// Output: Error retrieval handler registered
}

// ExampleContext_HasErrors demonstrates checking for errors.
func ExampleContext_HasErrors() {
	r := router.MustNew()

	r.POST("/process", func(c *router.Context) {
		// Perform validations
		if c.Query("name") == "" {
			//nolint:errcheck // Example code, we don't care about the error here
			c.Error(errors.New("name is required"))
		}

		// Check if any errors exist
		if c.HasErrors() {
			//nolint:errcheck // Example code, we don't care about the error here
			c.JSON(http.StatusBadRequest, map[string]any{
				"error": "validation failed",
			})

			return
		}

		//nolint:errcheck // Example code, we don't care about the error here
		c.JSON(http.StatusOK, map[string]string{"status": "success"})
	})

	fmt.Println("Error checking handler registered")
	// Output: Error checking handler registered
}

// ExampleContext_JSON demonstrates JSON with error handling.
func ExampleContext_JSON() {
	r := router.MustNew()

	r.GET("/data", func(c *router.Context) {
		// JSON returns error explicitly for error handling
		//nolint:errcheck // Example code, we don't care about the error here
		err := c.JSON(http.StatusOK, map[string]string{"key": "value"})
		if err != nil {
			// Handle error explicitly
			slog.ErrorContext(c.Request.Context(), "failed to write JSON", "err", err)
			c.Error(err) // Optionally collect it
			c.WriteErrorResponse(http.StatusInternalServerError, "encoding failed")

			return
		}
	})

	fmt.Println("Low-level JSON handler registered")
	// Output: Low-level JSON handler registered
}

// ExampleContext_Error_withErrorsJoin demonstrates combining errors with errors.Join.
func ExampleContext_Error_withErrorsJoin() {
	r := router.MustNew()

	r.POST("/validate", func(c *router.Context) {
		// Collect multiple errors
		c.Error(errors.New("name is required"))
		c.Error(errors.New("email is invalid"))
		c.Error(errors.New("age must be positive"))

		// Combine all errors using standard library
		if c.HasErrors() {
			joinedErr := errors.Join(c.Errors()...)
			//nolint:errcheck // Example code, we don't care about the error here
			c.JSON(http.StatusBadRequest, map[string]any{
				"error": joinedErr.Error(),
			})

			return
		}

		//nolint:errcheck // Example code, we don't care about the error here
		c.JSON(http.StatusOK, map[string]string{"status": "valid"})
	})

	fmt.Println("Error joining handler registered")
	// Output: Error joining handler registered
}
