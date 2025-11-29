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
	a, err := app.New()
	if err != nil {
		log.Fatal(err)
	}

	a.GET("/", func(c *app.Context) {
		if err := c.JSON(http.StatusOK, map[string]string{
			"message": "Hello, World!",
		}); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})

	fmt.Println("App created successfully")
	// Output: App created successfully
}

// Example_withObservability demonstrates full observability setup.
func Example_withObservability() {
	a, err := app.New(
		app.WithServiceName("example-api"),
		app.WithServiceVersion("v1.0.0"),
		app.WithObservability(
			app.WithMetrics(metrics.WithPrometheus(":9090", "/metrics")),
			app.WithTracing(tracing.WithProvider(tracing.NoopProvider)),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Service: %s\n", a.ServiceName())
	fmt.Printf("Metrics: enabled\n")
	// Output:
	// Service: example-api
	// Metrics: enabled
}

// Example_testing demonstrates testing patterns.
func Example_testing() {
	a, _ := app.New()

	a.GET("/health", func(c *app.Context) {
		if err := c.String(http.StatusOK, "ok"); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := a.Test(req)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Status: %d\n", resp.StatusCode)
	resp.Body.Close()
	// Output: Status: 200
}

// Example_routing demonstrates route registration.
func Example_routing() {
	a, _ := app.New()

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
	a, _ := app.New()

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

// Example_bindAndValidate demonstrates request binding and validation.
func Example_bindAndValidate() {
	a, _ := app.New()

	type CreateUserRequest struct {
		Name  string `json:"name" validate:"required,min=3"`
		Email string `json:"email" validate:"required,email"`
	}

	a.POST("/users", func(c *app.Context) {
		var req CreateUserRequest
		if err := c.BindAndValidate(&req); err != nil {
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

// Example_healthEndpoints demonstrates health check endpoint configuration.
func Example_healthEndpoints() {
	a, err := app.New(
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
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Health endpoints configured: %s\n", a.ServiceName())
	// Output: Health endpoints configured: example-api
}

// Example_lifecycleHooks demonstrates lifecycle hook registration.
func Example_lifecycleHooks() {
	a, _ := app.New()

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
