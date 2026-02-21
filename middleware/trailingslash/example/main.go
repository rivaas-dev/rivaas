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

// Package main demonstrates how to use the TrailingSlash middleware
// to normalize URLs by adding or removing trailing slashes.
package main

import (
	"log"
	"net/http"

	"rivaas.dev/router"
	"rivaas.dev/middleware/trailingslash"
)

func main() {
	r := router.MustNew()

	r.GET("/users", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Users list (no trailing slash)",
		})
	})

	r.GET("/posts", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Posts list (no trailing slash)",
		})
	})

	// Wrap the router so trailing slash handling runs before route matching.
	// PolicyRemove: /users/ redirects to /users (308)
	handler := trailingslash.Wrap(r, trailingslash.WithPolicy(trailingslash.PolicyRemove))

	log.Println("Server starting on http://localhost:8080")
	log.Println("PolicyRemove: /users/ redirects to /users")
	log.Fatal(http.ListenAndServe(":8080", handler))
}
