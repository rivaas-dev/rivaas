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

// Package main demonstrates the Rivaas MCP feature with both
// debug runtime tools and business-facing MCP tools/resources.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"rivaas.dev/app"
	"rivaas.dev/logging"
)

// SearchProductsInput defines the input schema for the search_products tool.
// Schema constraints are expressed via jsonschema struct tags.
type SearchProductsInput struct {
	Query       string  `json:"query"          jsonschema:"Search query text,minLength=1"`
	Category    string  `json:"category"       jsonschema:"Product category filter,enum=electronics,enum=clothing,enum=books,enum=all"`
	MinPrice    float64 `json:"min_price"      jsonschema:"Minimum price in USD,minimum=0"`
	InStockOnly bool    `json:"in_stock_only"  jsonschema:"Only show in-stock items"`
	Page        int     `json:"page"           jsonschema:"Page number (1-based),minimum=1,default=1"`
}

// GetProductInput defines the input schema for the get_product tool.
type GetProductInput struct {
	ProductID string `json:"product_id" jsonschema:"The product ID,pattern=^P[0-9]+$"`
}

func main() {
	a := app.MustNew(
		app.WithServiceName("mcp-demo"),
		app.WithServiceVersion("1.0.0"),
		app.WithObservability(
			app.WithLogging(
				logging.WithConsoleHandler(),
			),
		),

		// Debug MCP: exposes Go runtime internals to AI tools
		// Available at: http://localhost:8080/_internal/debug/mcp
		app.WithDebugEndpoints(
			app.WithPprof(),
			app.WithMCPDebug(
				app.WithMCPDebugRuntime(),
				app.WithMCPDebugConfig(),
				app.WithMCPDebugBuild(),
				app.WithMCPDebugRoutes(),
				app.WithMCPDebugHealth(),
				app.WithMCPDebugOpenAPI(),
			),
		),

		// Business MCP: developer-defined tools and resources
		// Available at: http://localhost:8080/mcp
		app.WithMCP(
			app.WithMCPTool("search_products", "Search the product catalog",
				func(ctx context.Context, input SearchProductsInput) (any, error) {
					page := input.Page
					if page == 0 {
						page = 1
					}

					return map[string]any{
						"query":     input.Query,
						"category":  input.Category,
						"min_price": input.MinPrice,
						"in_stock":  input.InStockOnly,
						"page":      page,
						"results": []map[string]any{
							{"id": "P001", "name": "Laptop", "price": 999.99, "in_stock": true},
							{"id": "P002", "name": "Mouse", "price": 29.99, "in_stock": true},
						},
						"total": 2,
					}, nil
				},
			),

			app.WithMCPTool("get_product", "Get a product by ID",
				func(ctx context.Context, input GetProductInput) (any, error) {
					return map[string]any{
						"id":          input.ProductID,
						"name":        "Laptop Pro",
						"price":       1299.99,
						"description": "A powerful laptop for professionals",
						"in_stock":    true,
						"tags":        []string{"electronics", "computers"},
					}, nil
				},
			),

			app.WithMCPResource("products://catalog", "Product Catalog",
				"Complete product catalog with pricing and availability",
				func(ctx context.Context) (any, error) {
					return map[string]any{
						"products": []map[string]any{
							{"id": "P001", "name": "Laptop", "price": 999.99},
							{"id": "P002", "name": "Mouse", "price": 29.99},
							{"id": "P003", "name": "Keyboard", "price": 79.99},
						},
						"last_updated": "2025-01-01T00:00:00Z",
					}, nil
				},
			),
		),

		// Health check
		app.WithHealthEndpoints(
			app.WithLivenessCheck("process", func(ctx context.Context) error {
				return nil
			}),
		),
	)

	a.GET("/", func(c *app.Context) {
		endpoints := []string{
			"GET  /                          - This page",
			"GET  /livez                     - Liveness probe",
			"*    /mcp                       - Business MCP server (tools: search_products, get_product; resource: products://catalog)",
			"*    /_internal/debug/mcp       - Debug MCP server (runtime, config, build info)",
			"GET  /_internal/debug/pprof/*   - pprof profiling",
		}
		if jsonErr := c.JSON(http.StatusOK, map[string]any{
			"service":   "mcp-demo",
			"endpoints": endpoints,
			"hint":      fmt.Sprintf("Point your AI tool at http://localhost:8080/mcp\n\nEndpoints:\n%s", strings.Join(endpoints, "\n")),
		}); jsonErr != nil {
			slog.Error("failed to write response", slog.String("error", jsonErr.Error()))
		}
	})

	if err := a.Start(context.Background()); err != nil {
		slog.Error("failed to start server", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
