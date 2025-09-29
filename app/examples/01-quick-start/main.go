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
	"log"
	"net/http"

	"rivaas.dev/app"
)

func main() {
	// Create a new app with default settings
	a, err := app.New()
	if err != nil {
		log.Fatalf("Failed to create app: %v", err)
	}

	// Register routes
	a.GET("/", func(c *app.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Hello from Rivaas App!",
		})
	})

	a.GET("/health", func(c *app.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
		})
	})

	// Start the server with error handling
	if err := a.Run(":8080"); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
