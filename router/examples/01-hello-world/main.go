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

// Package main demonstrates the simplest possible Rivaas router setup.
package main

import (
	"net/http"
	"os"

	"github.com/charmbracelet/log"
	"rivaas.dev/router"
)

func main() {
	// Create a new router
	r := router.MustNew()

	// Define a simple GET route
	r.GET("/", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Hello, Rivaas!",
		})
	})

	// Create a logger with clean, colorful output
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: false,
		ReportCaller:    false,
	})

	logger.Info("🚀 Server starting on http://localhost:8080")
	logger.Print("")
	logger.Print("📝 Available endpoint:")
	logger.Print("  GET /")
	logger.Print("")
	logger.Print("📋 Example command:")
	logger.Print("  curl http://localhost:8080/")
	logger.Print("")

	logger.Fatal(http.ListenAndServe(":8080", r))
}
