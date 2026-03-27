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

package app

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPOption configures the business-facing MCP server.
type MCPOption func(*mcpSettings)

// mcpSettings holds business MCP server configuration.
type mcpSettings struct {
	enabled          bool
	prefix           string // Mount prefix (default: "/mcp")
	tools            []mcpToolDef
	resources        []mcpResourceDef
	validationErrors []error
}

// mcpToolDef holds a tool registration as a closure that registers itself
// on the MCP server. The closure captures the typed handler via generics
// at WithMCPTool call time, keeping mcpSettings non-generic.
type mcpToolDef struct {
	name     string
	register func(srv *mcp.Server) error
}

// mcpResourceDef holds a resource registration.
type mcpResourceDef struct {
	uri         string
	name        string
	description string
	handler     MCPResourceHandler
}

// MCPToolHandler is the handler signature for MCP tools.
// I is the input struct type; schema is derived from its json/jsonschema tags.
// Return any JSON-serializable value, or an error.
//
// Example:
//
//	type SearchInput struct {
//	    Query string `json:"query" jsonschema:"Search query text,minLength=1"`
//	}
//
//	func(ctx context.Context, input SearchInput) (any, error) {
//	    return productService.Search(ctx, input.Query)
//	}
type MCPToolHandler[I any] func(ctx context.Context, input I) (any, error)

// MCPResourceHandler is the handler signature for MCP resources.
// ctx carries the request context. Return any JSON-serializable value, or an error.
type MCPResourceHandler func(ctx context.Context) (any, error)

// WithMCPTool registers a tool on the business MCP server.
// The input schema is derived from the I type's json and jsonschema struct tags.
//
// Example:
//
//	type GetOrderInput struct {
//	    OrderID string `json:"order_id" jsonschema:"the order ID"`
//	}
//
//	app.WithMCP(
//	    app.WithMCPTool("get_order", "Get an order by ID",
//	        func(ctx context.Context, input GetOrderInput) (any, error) {
//	            return orderService.GetByID(ctx, input.OrderID)
//	        },
//	    ),
//	)
func WithMCPTool[I any](name, description string, handler MCPToolHandler[I]) MCPOption {
	return func(s *mcpSettings) {
		if handler == nil {
			s.validationErrors = append(s.validationErrors,
				fmt.Errorf("app: MCP tool %q handler cannot be nil", name))
			return
		}
		td := mcpToolDef{
			name: name,
			register: func(srv *mcp.Server) error {
				return safeAddTool(srv, &mcp.Tool{Name: name, Description: description},
					func(ctx context.Context, _ *mcp.CallToolRequest, input I) (*mcp.CallToolResult, any, error) {
						result, err := handler(ctx, input)
						return nil, result, err
					})
			},
		}
		s.tools = append(s.tools, td)
	}
}

// WithMCPResource registers a resource on the business MCP server.
//
// Example:
//
//	app.WithMCP(
//	    app.WithMCPResource("orders://recent", "Recent Orders",
//	        "The 10 most recently placed orders",
//	        func(ctx context.Context) (any, error) {
//	            return orderService.ListRecent(ctx, 10)
//	        },
//	    ),
//	)
func WithMCPResource(uri, name, description string, handler MCPResourceHandler) MCPOption {
	return func(s *mcpSettings) {
		if handler == nil {
			s.validationErrors = append(s.validationErrors,
				fmt.Errorf("app: MCP resource %q handler cannot be nil", uri))
		}
		s.resources = append(s.resources, mcpResourceDef{
			uri:         uri,
			name:        name,
			description: description,
			handler:     handler,
		})
	}
}

// WithMCPPrefix sets the mount prefix for the business MCP server.
// Default is "/mcp".
//
// Example:
//
//	app.WithMCP(
//	    app.WithMCPPrefix("/api/mcp"),
//	    app.WithMCPTool(...),
//	)
func WithMCPPrefix(prefix string) MCPOption {
	return func(s *mcpSettings) {
		s.prefix = prefix
	}
}

// WithMCP enables and configures the business-facing MCP server.
// Developers register their own tools and resources using Rivaas-native types.
// They never import the MCP SDK directly.
//
// The MCP server is mounted at /mcp by default (configurable with [WithMCPPrefix]).
// It uses the Streamable HTTP transport (GET + POST + DELETE on a single endpoint).
//
// Example:
//
//	type SearchInput struct {
//	    Query string `json:"query" jsonschema:"Search query text,minLength=1"`
//	}
//
//	app.MustNew(
//	    app.WithMCP(
//	        app.WithMCPTool("search", "Search products",
//	            func(ctx context.Context, input SearchInput) (any, error) {
//	                return productService.Search(ctx, input.Query)
//	            },
//	        ),
//	        app.WithMCPResource("orders://recent", "Recent Orders",
//	            "The 10 most recently placed orders",
//	            func(ctx context.Context) (any, error) {
//	                return orderService.ListRecent(ctx, 10)
//	            },
//	        ),
//	    ),
//	)
func WithMCP(opts ...MCPOption) Option {
	return func(c *config) {
		if c.mcp == nil {
			c.mcp = &mcpSettings{
				enabled: true,
				prefix:  "/mcp",
			}
		}
		for i, opt := range opts {
			if opt == nil {
				c.validationErrors = append(c.validationErrors, fmt.Errorf("app: MCP option at index %d cannot be nil", i))
				continue
			}
			opt(c.mcp)
		}
	}
}

// WithMCPIf conditionally enables the business-facing MCP server.
//
// Example:
//
//	app.WithMCPIf(os.Getenv("MCP_ENABLED") == "true",
//	    app.WithMCPTool(...),
//	)
func WithMCPIf(cond bool, opts ...MCPOption) Option {
	if !cond {
		return func(c *config) {}
	}
	return WithMCP(opts...)
}
