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
	"os"
	"os/signal"
	"syscall"

	"rivaas.dev/app"
)

// ExampleApp_OnReload demonstrates how to use the OnReload hook to reload
// configuration when SIGHUP is received.
func ExampleApp_OnReload() {
	// Create app - SIGHUP handling is automatically enabled when hooks are registered
	myApp := app.MustNew(
		app.WithServiceName("example-reload"),
	)

	// Register a reload hook to reload configuration
	// SIGHUP is automatically enabled when this hook is registered
	myApp.OnReload(func(ctx context.Context) error {
		fmt.Println("Reloading configuration...")
		// In a real application, you would:
		// - Re-read config files
		// - Rotate TLS certificates
		// - Flush caches
		// - Update connection pool settings
		return nil
	})

	myApp.GET("/", func(c *app.Context) {
		_ = c.String(200, "Hello, World!") //nolint:errcheck // Example code
	})

	// Set up signal handling for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start server - SIGHUP will trigger reload hooks automatically
	if err := myApp.Start(ctx); err != nil {
		log.Fatal(err)
	}
}

// ExampleApp_Reload demonstrates programmatic reload without SIGHUP.
func ExampleApp_Reload() {
	myApp := app.MustNew(
		app.WithServiceName("example-reload-programmatic"),
	)

	var configVersion int

	myApp.OnReload(func(ctx context.Context) error {
		configVersion++
		fmt.Printf("Config reloaded, version: %d\n", configVersion)
		return nil
	})

	// Create an admin endpoint that triggers reload
	myApp.POST("/admin/reload", func(c *app.Context) {
		if err := myApp.Reload(c.Request.Context()); err != nil {
			c.InternalError(err)
			return
		}
		_ = c.JSON(200, map[string]string{ //nolint:errcheck // Example code
			"status": "reloaded",
		})
	})

	// Programmatically trigger reload
	ctx := context.Background()
	if err := myApp.Reload(ctx); err != nil {
		log.Printf("reload failed: %v", err)
	}
	// Output: Config reloaded, version: 1
}
