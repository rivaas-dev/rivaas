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

package app_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	"rivaas.dev/app"
	"rivaas.dev/metrics"
	"rivaas.dev/tracing"
)

// Example demonstrates basic app usage.
func Example() {
	a := app.MustNew()

	a.GET("/", func(c *app.Context) {
		if jsonErr := c.JSON(http.StatusOK, map[string]string{
			"message": "Hello, World!",
		}); jsonErr != nil {
			log.Printf("Failed to write response: %v", jsonErr)
		}
	})

	fmt.Println("App created successfully")
	// Output: App created successfully
}

// Example_withObservability demonstrates full observability setup.
func Example_withObservability() {
	a := app.MustNew(
		app.WithServiceName("example-api"),
		app.WithServiceVersion("v1.0.0"),
		app.WithObservability(
			app.WithMetrics(metrics.WithPrometheus(":9090", "/metrics")),
			app.WithTracing(tracing.WithNoop()),
		),
	)

	fmt.Printf("Service: %s\n", a.ServiceName())
	fmt.Printf("Metrics: enabled\n")
	// Output:
	// Service: example-api
	// Metrics: enabled
}

// Example_testing demonstrates testing patterns.
func Example_testing() {
	a := app.MustNew()

	a.GET("/health", func(c *app.Context) {
		if err := c.String(http.StatusOK, "ok"); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp, err := a.Test(req)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Status: %d\n", resp.StatusCode)
	//nolint:errcheck // example code, we don't care about the error here
	resp.Body.Close()
	// Output: Status: 200
}

// Example_routing demonstrates route registration.
func Example_routing() {
	a := app.MustNew()

	a.GET("/users", func(c *app.Context) {
		if err := c.JSON(http.StatusOK, map[string]string{"users": "list"}); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})

	a.POST("/users", func(c *app.Context) {
		if err := c.JSON(http.StatusCreated, map[string]string{"user": "created"}); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})

	fmt.Println("Routes registered")
	// Output: Routes registered
}

// Example_middleware demonstrates middleware usage.
func Example_middleware() {
	a := app.MustNew()

	a.Use(func(c *app.Context) {
		// Add custom header
		c.Header("X-Custom", "value")
		c.Next()
	})

	a.GET("/test", func(c *app.Context) {
		if err := c.String(http.StatusOK, "ok"); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})

	fmt.Println("Middleware registered")
	// Output: Middleware registered
}

// ExampleContext_Bind demonstrates basic request binding and validation.
func ExampleContext_Bind() {
	a := app.MustNew()

	type CreateUserRequest struct {
		Name  string `json:"name" validate:"required,min=3"`
		Email string `json:"email" validate:"required,email"`
	}

	a.POST("/users", func(c *app.Context) {
		var req CreateUserRequest
		if err := c.Bind(&req); err != nil {
			c.Error(err)
			return
		}

		if err := c.JSON(http.StatusCreated, map[string]string{
			"message": "User created",
			"name":    req.Name,
		}); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})

	fmt.Println("Handler with binding and validation registered")
	// Output: Handler with binding and validation registered
}

// ExampleContext_Bind_withOptions demonstrates binding with options.
func ExampleContext_Bind_withOptions() {
	a := app.MustNew()

	type CreateUserRequest struct {
		Name  string `json:"name" validate:"required,min=3"`
		Email string `json:"email" validate:"required,email"`
	}

	a.POST("/users", func(c *app.Context) {
		var req CreateUserRequest
		if err := c.Bind(&req, app.WithStrict()); err != nil {
			c.Error(err)
			return
		}

		if err := c.JSON(http.StatusCreated, map[string]string{
			"message": "User created",
		}); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})

	fmt.Println("Handler with strict binding registered")
	// Output: Handler with strict binding registered
}

// ExampleContext_MustBind demonstrates the Must pattern for binding.
func ExampleContext_MustBind() {
	a := app.MustNew()

	type CreateUserRequest struct {
		Name  string `json:"name" validate:"required,min=3"`
		Email string `json:"email" validate:"required,email"`
	}

	a.POST("/users", func(c *app.Context) {
		var req CreateUserRequest
		if !c.MustBind(&req) {
			return // Error already written
		}

		if err := c.JSON(http.StatusCreated, map[string]string{
			"message": "User created",
			"name":    req.Name,
		}); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})

	fmt.Println("Handler with MustBind registered")
	// Output: Handler with MustBind registered
}

// ExampleBind demonstrates type-safe binding with generics.
func ExampleBind() {
	a := app.MustNew()

	type CreateUserRequest struct {
		Name  string `json:"name" validate:"required,min=3"`
		Email string `json:"email" validate:"required,email"`
	}

	a.POST("/users", func(c *app.Context) {
		req, err := app.Bind[CreateUserRequest](c)
		if err != nil {
			c.Error(err)
			return
		}

		if jsonErr := c.JSON(http.StatusCreated, map[string]string{
			"message": "User created",
			"name":    req.Name,
		}); jsonErr != nil {
			log.Printf("Failed to write response: %v", jsonErr)
		}
	})

	fmt.Println("Handler with generic Bind registered")
	// Output: Handler with generic Bind registered
}

// ExampleMustBind demonstrates type-safe binding with the Must pattern.
func ExampleMustBind() {
	a := app.MustNew()

	type CreateUserRequest struct {
		Name  string `json:"name" validate:"required,min=3"`
		Email string `json:"email" validate:"required,email"`
	}

	a.POST("/users", func(c *app.Context) {
		req, ok := app.MustBind[CreateUserRequest](c)
		if !ok {
			return // Error already written
		}

		if err := c.JSON(http.StatusCreated, map[string]string{
			"message": "User created",
			"name":    req.Name,
		}); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})

	fmt.Println("Handler with MustBind registered")
	// Output: Handler with MustBind registered
}

// ExampleBindPatch demonstrates partial validation for PATCH endpoints.
func ExampleBindPatch() {
	a := app.MustNew()

	type UpdateUserRequest struct {
		Name  *string `json:"name" validate:"omitempty,min=3"`
		Email *string `json:"email" validate:"omitempty,email"`
	}

	a.PATCH("/users/:id", func(c *app.Context) {
		req, err := app.BindPatch[UpdateUserRequest](c)
		if err != nil {
			c.Error(err)
			return
		}

		userID := c.Param("id")
		if jsonErr := c.JSON(http.StatusOK, map[string]string{
			"message": "User updated",
			"id":      userID,
		}); jsonErr != nil {
			log.Printf("Failed to write response: %v", jsonErr)
		}

		_ = req // Use req for update logic
	})

	fmt.Println("PATCH handler with partial validation registered")
	// Output: PATCH handler with partial validation registered
}

// ExampleBindStrict demonstrates strict mode for typo detection.
func ExampleBindStrict() {
	a := app.MustNew()

	type CreateUserRequest struct {
		Name  string `json:"name" validate:"required,min=3"`
		Email string `json:"email" validate:"required,email"`
	}

	a.POST("/users", func(c *app.Context) {
		req, err := app.BindStrict[CreateUserRequest](c)
		if err != nil {
			c.Error(err)
			return
		}

		if jsonErr := c.JSON(http.StatusCreated, map[string]string{
			"message": "User created",
			"name":    req.Name,
		}); jsonErr != nil {
			log.Printf("Failed to write response: %v", jsonErr)
		}
	})

	fmt.Println("Handler with strict binding registered")
	// Output: Handler with strict binding registered
}

// ExampleContext_BindOnly demonstrates binding without validation.
func ExampleContext_BindOnly() {
	a := app.MustNew()

	type CreateUserRequest struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	a.POST("/users", func(c *app.Context) {
		var req CreateUserRequest
		if err := c.BindOnly(&req); err != nil {
			c.Error(err)
			return
		}

		// Custom processing before validation
		req.Email = normalizeEmail(req.Email)

		// Validate separately
		if err := c.Validate(&req); err != nil {
			c.Error(err)
			return
		}

		if err := c.JSON(http.StatusCreated, map[string]string{
			"message": "User created",
		}); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})

	fmt.Println("Handler with separate bind and validate registered")
	// Output: Handler with separate bind and validate registered
}

func normalizeEmail(email string) string {
	// Example normalization
	return email
}

// Example_healthEndpoints demonstrates health check endpoint configuration.
func Example_healthEndpoints() {
	a := app.MustNew(
		app.WithServiceName("example-api"),
		app.WithHealthEndpoints(
			app.WithLivenessCheck("process", func(ctx context.Context) error {
				// Process is alive
				return nil
			}),
			app.WithReadinessCheck("database", func(ctx context.Context) error {
				// Check database connection
				// return db.PingContext(ctx)
				return nil
			}),
		),
	)

	fmt.Printf("Health endpoints configured: %s\n", a.ServiceName())
	// Output: Health endpoints configured: example-api
}

// Example_lifecycleHooks demonstrates lifecycle hook registration.
func Example_lifecycleHooks() {
	a := app.MustNew()

	a.OnStart(func(ctx context.Context) error {
		// Initialize database, run migrations, etc.
		fmt.Println("OnStart: Initializing...")
		return nil
	})

	a.OnReady(func() {
		// Register with service discovery, warmup caches, etc.
		fmt.Println("OnReady: Server is ready")
	})

	a.OnShutdown(func(ctx context.Context) {
		// Close connections, flush buffers, etc.
		fmt.Println("OnShutdown: Cleaning up...")
	})

	fmt.Println("Lifecycle hooks registered")
	// Output: Lifecycle hooks registered
}
