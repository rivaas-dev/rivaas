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

// Package main demonstrates how to use the MethodOverride middleware
// to allow PUT, PATCH, and DELETE via POST with a header or form field.
package main

import (
	"log"
	"net/http"

	"rivaas.dev/middleware/methodoverride"
	"rivaas.dev/router"
)

func main() {
	r := router.MustNew()

	r.Use(methodoverride.New())

	// These routes expect the real HTTP method; clients can send POST + override header
	r.PUT("/users/:id", func(c *router.Context) {
		id := c.Param("id")
		c.JSON(http.StatusOK, map[string]string{
			"message": "User updated (PUT)",
			"id":      id,
		})
	})

	r.PATCH("/users/:id", func(c *router.Context) {
		id := c.Param("id")
		c.JSON(http.StatusOK, map[string]string{
			"message": "User patched (PATCH)",
			"id":      id,
		})
	})

	r.DELETE("/users/:id", func(c *router.Context) {
		id := c.Param("id")
		c.JSON(http.StatusOK, map[string]string{
			"message": "User deleted (DELETE)",
			"id":      id,
		})
	})

	log.Println("Server starting on http://localhost:8080")
	log.Println("Use POST + X-HTTP-Method-Override: PUT|PATCH|DELETE, or _method in form body")
	log.Fatal(http.ListenAndServe(":8080", r))
}
