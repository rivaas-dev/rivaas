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
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	"rivaas.dev/app"
)

// Example demonstrates basic app usage.
func Example() {
	a, err := app.New()
	if err != nil {
		log.Fatal(err)
	}

	a.GET("/", func(c *app.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Hello, World!",
		})
	})

	fmt.Println("App created successfully")
	// Output: App created successfully
}

// Example_withObservability demonstrates full observability setup.
func Example_withObservability() {
	a, err := app.New(
		app.WithServiceName("example-api"),
		app.WithServiceVersion("v1.0.0"),
		app.WithMetrics(),
		app.WithTracing(),
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
		c.String(http.StatusOK, "ok")
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
		c.JSON(http.StatusOK, map[string]string{"users": "list"})
	})

	a.POST("/users", func(c *app.Context) {
		c.JSON(http.StatusCreated, map[string]string{"user": "created"})
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
		c.String(http.StatusOK, "ok")
	})

	fmt.Println("Middleware registered")
	// Output: Middleware registered
}
