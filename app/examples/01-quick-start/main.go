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

// Package main demonstrates a quick start example of the Rivaas router.
package main

import (
	"context"
	"log"
	"net/http"

	"rivaas.dev/app"
)

func main() {
	// Create a new app with default settings and health endpoints
	a, err := app.New(
		// Enable standard health endpoints
		app.WithHealthEndpoints(
			// Simple liveness check (process is alive)
			app.WithLivenessCheck("process", func(ctx context.Context) error {
				return nil // Process is alive
			}),
		),
	)
	if err != nil {
		log.Fatalf("Failed to create app: %v", err)
	}

	// Register routes
	a.GET("/", func(c *app.Context) {
		if err := c.JSON(http.StatusOK, map[string]string{
			"message": "Hello from Rivaas App!",
		}); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})

	// Start the server with error handling
	// Health endpoints are available at:
	//   GET /healthz - Liveness probe (returns 200 "ok")
	//   GET /readyz  - Readiness probe (returns 204)
	if err := a.Run(":8080"); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
