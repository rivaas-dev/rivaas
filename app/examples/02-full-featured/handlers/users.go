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

// Package handlers provides user-related HTTP handlers for the full-featured
// example application.
package handlers

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"rivaas.dev/app"
)

// GetUserByID retrieves a user by ID using path parameter binding.
// It binds the "id" parameter from the URL path, adds tracing attributes,
// records metrics, and returns the user data as JSON.
//
// Example:
//
//	GET /users/:id
//	Returns user data for the specified ID, or 404 if not found.
func GetUserByID(c *app.Context) {
	var params UserPathParams
	if err := c.Bind(&params); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "%v", err))
		return
	}

	// Add tracing attributes
	c.SetSpanAttribute("user.id", params.ID)
	c.SetSpanAttribute("operation.type", "read")
	c.AddSpanEvent("user_lookup_started")

	// Simulate database lookup
	time.Sleep(10 * time.Millisecond)

	// Record custom metrics
	c.IncrementCounter("user_lookups_total",
		attribute.Int("user_id", params.ID),
		attribute.String("result", "success"),
	)

	// Simulate user not found scenario
	if params.ID == 999 {
		c.SetSpanAttribute("error", true)
		c.SetSpanAttribute("error.type", "user_not_found")
		HandleError(c, ErrUserNotFound)
		return
	}

	c.AddSpanEvent("user_found")
	c.JSON(http.StatusOK, UserResponse{
		ID:        params.ID,
		Name:      "John Doe",
		Email:     "john@example.com",
		CreatedAt: time.Now().Add(-30 * 24 * time.Hour), // 30 days ago
	})
}

// CreateUser creates a new user using request body binding.
// It binds the request body to CreateUserRequest, validates the input,
// adds tracing attributes, records metrics, and returns the created user.
//
// Example:
//
//	POST /users
//	Body: {"name": "John Doe", "email": "john@example.com", "age": 30}
//	Returns the created user with generated ID and timestamp.
func CreateUser(c *app.Context) {
	var req CreateUserRequest
	if err := c.Bind(&req); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "failed to parse request body: %v", err))
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		HandleError(c, err)
		return
	}

	// Add tracing
	c.SetSpanAttribute("operation", "create_user")
	c.SetSpanAttribute("user.email", req.Email)
	c.AddSpanEvent("user_creation_started")

	// Simulate user creation
	time.Sleep(20 * time.Millisecond)

	// Record metrics
	c.IncrementCounter("users_created_total",
		attribute.String("result", "success"),
	)

	c.AddSpanEvent("user_created")
	c.JSON(http.StatusCreated, UserResponse{
		ID:        GenerateUserID(),
		Name:      req.Name,
		Email:     req.Email,
		CreatedAt: time.Now(),
	})
}

// GetUserOrders retrieves orders for a user by user ID.
// It binds the "id" parameter from the URL path and returns a list of orders
// associated with that user.
//
// Example:
//
//	GET /users/:id/orders
//	Returns a list of orders for the specified user ID.
func GetUserOrders(c *app.Context) {
	var params UserPathParams
	if err := c.Bind(&params); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "%v", err))
		return
	}

	c.SetSpanAttribute("user.id", params.ID)
	c.SetSpanAttribute("operation", "list_user_orders")

	// Simulate database query
	time.Sleep(15 * time.Millisecond)

	c.JSON(http.StatusOK, map[string]any{
		"user_id": params.ID,
		"orders":  []OrderResponse{}, // Empty for demo
		"total":   0,
	})
}
