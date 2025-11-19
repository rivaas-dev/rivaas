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

package handlers

import (
	"net/http"
	"time"

	"rivaas.dev/app"
)

// ListProducts lists products with query parameter binding.
func ListProducts(c *app.Context) {
	var params SearchParams
	if err := c.Bind(&params); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "%v", err))
		return
	}

	c.SetSpanAttribute("resource", "products")
	c.SetSpanAttribute("query.page", params.Page)
	c.SetSpanAttribute("query.page_size", params.PageSize)
	c.AddSpanEvent("fetching_products")

	// Simulate work
	time.Sleep(20 * time.Millisecond)

	products := []map[string]any{
		{"id": "1", "name": "Product A", "price": 29.99},
		{"id": "2", "name": "Product B", "price": 49.99},
		{"id": "3", "name": "Product C", "price": 19.99},
	}

	c.JSON(http.StatusOK, map[string]any{
		"products":  products,
		"page":      params.Page,
		"page_size": params.PageSize,
		"total":     len(products),
		"trace_id":  c.TraceID(),
	})
}

// GetProductByID retrieves a product by ID with custom constraint.
func GetProductByID(c *app.Context) {
	var params ProductPathParams
	if err := c.Bind(&params); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "%v", err))
		return
	}

	c.SetSpanAttribute("product.id", params.ID)
	c.SetSpanAttribute("operation", "get_product")

	// Simulate database lookup
	time.Sleep(10 * time.Millisecond)

	c.JSON(http.StatusOK, map[string]any{
		"id":    params.ID,
		"name":  "Product " + params.ID,
		"price": 29.99,
	})
}
