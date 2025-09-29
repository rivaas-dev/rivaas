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

// Package handlers provides request and response type definitions for the
// full-featured example application.
package handlers

import "time"

// CreateUserRequest represents a user creation request.
// It supports binding from both JSON request bodies and form data.
type CreateUserRequest struct {
	Name  string `json:"name" form:"name"`
	Email string `json:"email" form:"email"`
	Age   int    `json:"age" form:"age"`
}

// UserPathParams represents user-related path parameters extracted from URL paths.
type UserPathParams struct {
	ID int `params:"id"`
}

// UserResponse represents a user response returned by the API.
type UserResponse struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateOrderRequest represents an order creation request.
// It supports binding from both JSON request bodies and form data.
type CreateOrderRequest struct {
	UserID        int               `json:"user_id" form:"user_id"`
	Items         []OrderItem       `json:"items" form:"items"`
	Currency      string            `json:"currency" form:"currency" enum:"USD,EUR,GBP" default:"USD"`
	PaymentMethod string            `json:"payment_method" form:"payment_method" enum:"card,paypal,bank_transfer" default:"card"`
	Metadata      map[string]string `json:"metadata,omitempty" form:"metadata"`
}

// OrderItem represents an item in an order.
// Each item contains product information, quantity, and price.
type OrderItem struct {
	ProductID int     `json:"product_id" form:"product_id"`
	Quantity  int     `json:"quantity" form:"quantity"`
	Price     float64 `json:"price" form:"price"`
}

// OrderResponse represents an order response returned by the API.
type OrderResponse struct {
	OrderID        string            `json:"order_id"`
	UserID         int               `json:"user_id"`
	Items          []OrderItem       `json:"items"`
	TotalAmount    float64           `json:"total_amount"`
	Currency       string            `json:"currency"`
	Status         string            `json:"status"`
	CreatedAt      time.Time         `json:"created_at"`
	ProcessingTime float64           `json:"processing_time,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// SearchParams represents search query parameters extracted from URL query strings.
// It supports default values and enum validation for certain fields.
type SearchParams struct {
	Query    string   `query:"q"`
	Page     int      `query:"page" default:"1"`
	PageSize int      `query:"page_size" default:"10"`
	Tags     []string `query:"tags"`
	Active   *bool    `query:"active"`
	SortBy   string   `query:"sort_by" enum:"name,date,price" default:"name"`
}

// ProductPathParams represents product-related path parameters extracted from URL paths.
type ProductPathParams struct {
	ID string `params:"id"`
}
