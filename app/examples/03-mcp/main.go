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
			),
		),

		// Business MCP: developer-defined tools and resources
		// Available at: http://localhost:8080/mcp
		app.WithMCP(
			app.WithMCPTool("search_products", "Search the product catalog",
				func(ctx context.Context, args app.MCPToolArgs) (any, error) {
					query, err := args.RequireString("query")
					if err != nil {
						return nil, err
					}
					category := args.StringDefault("category", "all")
					minPrice := args.Float("min_price")
					inStockOnly := args.Bool("in_stock_only")
					page := args.Int("page")
					if page == 0 {
						page = 1
					}

					return map[string]any{
						"query":     query,
						"category":  category,
						"min_price": minPrice,
						"in_stock":  inStockOnly,
						"page":      page,
						"results": []map[string]any{
							{"id": "P001", "name": "Laptop", "price": 999.99, "in_stock": true},
							{"id": "P002", "name": "Mouse", "price": 29.99, "in_stock": true},
						},
						"total": 2,
					}, nil
				},
				app.WithMCPStringInput("query", "Search query text", app.MCPRequired(), app.MCPMinLength(1)),
				app.WithMCPStringInput("category", "Product category filter", app.MCPEnum("electronics", "clothing", "books", "all")),
				app.WithMCPNumberInput("min_price", "Minimum price in USD", app.MCPMinimum(0), app.MCPDefault(0.0)),
				app.WithMCPBooleanInput("in_stock_only", "Only show in-stock items", app.MCPDefault(false)),
				app.WithMCPIntegerInput("page", "Page number (1-based)", app.MCPMinimum(1), app.MCPDefault(1.0)),
			),

			app.WithMCPTool("get_product", "Get a product by ID",
				func(ctx context.Context, args app.MCPToolArgs) (any, error) {
					id, err := args.RequireString("product_id")
					if err != nil {
						return nil, err
					}
					return map[string]any{
						"id":          id,
						"name":        "Laptop Pro",
						"price":       1299.99,
						"description": "A powerful laptop for professionals",
						"in_stock":    true,
						"tags":        []string{"electronics", "computers"},
					}, nil
				},
				app.WithMCPStringInput("product_id", "The product ID", app.MCPRequired(), app.MCPPattern("^P[0-9]+$")),
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
