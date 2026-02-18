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

// Package main provides examples of using the BasicAuth middleware.
package main

import (
	"fmt"
	"log"
	"net/http"

	"rivaas.dev/router"
	"rivaas.dev/router/middleware/basicauth"
)

func main() {
	r := router.MustNew()

	// Public routes - no authentication required
	r.GET("/", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Welcome! Visit /admin for protected content.",
		})
	})

	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
		})
	})

	// Protected admin routes - Basic Auth required
	admin := r.Group("/admin", basicauth.New(
		basicauth.WithUsers(map[string]string{
			"admin": "secret123",
			"user":  "password456",
		}),
		basicauth.WithRealm("Admin Panel"),
	))

	admin.GET("/dashboard", func(c *router.Context) {
		username := basicauth.Username(c)
		c.JSON(http.StatusOK, map[string]string{
			"message": fmt.Sprintf("Welcome to admin dashboard, %s!", username),
			"user":    username,
		})
	})

	admin.GET("/settings", func(c *router.Context) {
		username := basicauth.Username(c)
		c.JSON(http.StatusOK, map[string]interface{}{
			"user": username,
			"settings": map[string]any{
				"theme":         "dark",
				"notifications": true,
			},
		})
	})

	// Another protected area with different credentials
	api := r.Group("/api", basicauth.New(
		basicauth.WithUsers(map[string]string{
			"apikey1": "secret",
			"apikey2": "token",
		}),
		basicauth.WithRealm("API Access"),
		// Skip health check even within API group
		basicauth.WithSkipPaths("/api/health"),
	))

	api.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "API is healthy",
		})
	})

	api.GET("/data", func(c *router.Context) {
		username := basicauth.Username(c)
		c.JSON(http.StatusOK, map[string]interface{}{
			"authenticated_as": username,
			"data":             []string{"item1", "item2", "item3"},
		})
	})

	log.Println("Server starting on http://localhost:8080")
	log.Println("Public: GET / GET /health | Protected: /admin/* (curl -u admin:secret123) /api/data (curl -u apikey1:secret)")
	log.Fatal(http.ListenAndServe(":8080", r))
}
