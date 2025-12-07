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

// Package handlers provides order-related HTTP handlers for the full-featured
// example application.
package handlers

import (
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"rivaas.dev/app"
)

// CreateOrder creates a new order with full request binding and validation.
// It binds the request body to CreateOrderRequest, validates the input,
// calculates the total amount, records metrics with processing time,
// and returns the created order.
//
// Example:
//
//	POST /orders
//	Body: {"user_id": 123, "items": [...], "currency": "USD"}
//	Returns the created order with calculated total and processing time.
func CreateOrder(c *app.Context) {
	var req CreateOrderRequest
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
	c.SetSpanAttribute("operation", "create_order")
	c.SetSpanAttribute("order.user_id", req.UserID)
	c.SetSpanAttribute("order.currency", req.Currency)
	c.SetSpanAttribute("order.payment_method", req.PaymentMethod)
	c.AddSpanEvent("order_creation_started")

	// Simulate order processing with timing metrics
	start := time.Now()
	time.Sleep(50 * time.Millisecond)
	processingTime := time.Since(start).Seconds()

	// Calculate total
	var totalAmount float64
	for _, item := range req.Items {
		totalAmount += item.Price * float64(item.Quantity)
	}

	// Record custom metrics
	c.RecordMetric("order_processing_duration_seconds", processingTime,
		attribute.String("currency", req.Currency),
		attribute.String("payment_method", req.PaymentMethod),
	)

	c.IncrementCounter("orders_total",
		attribute.String("status", "success"),
		attribute.String("currency", req.Currency),
		attribute.String("payment_method", req.PaymentMethod),
	)

	c.AddSpanEvent("order_created")
	if err := c.IndentedJSON(http.StatusCreated, OrderResponse{
		OrderID:        fmt.Sprintf("ord_%d", GenerateOrderID()),
		UserID:         req.UserID,
		Items:          req.Items,
		TotalAmount:    totalAmount,
		Currency:       req.Currency,
		Status:         "created",
		CreatedAt:      time.Now(),
		ProcessingTime: processingTime,
		Metadata:       req.Metadata,
	}); err != nil {
		c.Logger().Error("failed to write order response", "err", err)
	}
}

// GetOrderByID retrieves an order by ID.
// It binds the "id" parameter from the URL path, adds tracing attributes,
// and returns the order data as JSON.
//
// Example:
//
//	GET /orders/:id
//	Returns order data for the specified ID.
func GetOrderByID(c *app.Context) {
	var params struct {
		ID int `path:"id"`
	}
	if err := c.Bind(&params); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "%v", err))
		return
	}

	c.SetSpanAttribute("order.id", params.ID)
	c.SetSpanAttribute("operation", "get_order")

	// Simulate database lookup
	time.Sleep(10 * time.Millisecond)

	if err := c.JSON(http.StatusOK, OrderResponse{
		OrderID:     fmt.Sprintf("ord_%d", params.ID),
		UserID:      123,
		Items:       []OrderItem{{ProductID: 1, Quantity: 2, Price: 29.99}},
		TotalAmount: 59.98,
		Currency:    "USD",
		Status:      "completed",
		CreatedAt:   time.Now().Add(-24 * time.Hour),
	}); err != nil {
		c.Logger().Error("failed to write order response", "err", err)
	}
}
