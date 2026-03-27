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
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"rivaas.dev/router"
	"rivaas.dev/router/route"
)

// registerMCPEndpoints translates Rivaas mcpSettings into official MCP SDK
// registrations and mounts the Streamable HTTP server on the router.
//
// MCP endpoints register directly on the router (via mountMCPServer) rather than
// through App.registerRoute so they do not participate in app-level hooks (OnRoute)
// or appear in the OpenAPI specification. This is intentional: MCP is an
// infrastructure transport, not a user-facing API surface.
func (a *App) registerMCPEndpoints(s *mcpSettings) error {
	base := s.prefix
	if base == "" {
		base = "/mcp"
	}

	srv := mcp.NewServer(
		&mcp.Implementation{
			Name:    a.config.serviceName,
			Version: a.config.serviceVersion,
		},
		nil,
	)

	for i := range s.tools {
		if err := s.tools[i].register(srv); err != nil {
			return fmt.Errorf("mcp: failed to add tool %q: %w", s.tools[i].name, err)
		}
	}

	for i := range s.resources {
		r := &s.resources[i]
		handler := wrapResourceHandler(r)
		if err := safeAddResource(srv, &mcp.Resource{
			URI:         r.uri,
			Name:        r.name,
			Description: r.description,
			MIMEType:    "application/json",
		}, handler); err != nil {
			return fmt.Errorf("mcp: failed to add resource %q: %w", r.uri, err)
		}
	}

	return a.mountMCPServer(base, srv, "mcp")
}

// mountMCPServer checks for route conflicts, creates a StreamableHTTPHandler
// for srv, registers GET/POST/DELETE on base, and sets the handler label.
func (a *App) mountMCPServer(base string, srv *mcp.Server, label string) error {
	methods := []string{"GET", "POST", "DELETE"}
	for _, m := range methods {
		if a.router.RouteExists(m, base) {
			return fmt.Errorf("route already registered: %s %s", m, base)
		}
	}

	httpHandler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return srv
	}, nil)

	routeHandler := func(c *router.Context) {
		httpHandler.ServeHTTP(c.Response, c.Request)
	}

	a.router.GET(base, routeHandler)
	a.router.POST(base, routeHandler)
	a.router.DELETE(base, routeHandler)

	handlerName := "[builtin] " + label
	for _, m := range methods {
		a.router.UpdateRouteInfo(m, base, "", func(info *route.Info) {
			info.HandlerName = handlerName
		})
	}

	return nil
}

// toolResult JSON-marshals data into a CallToolResult. On marshal failure it
// returns an error result instead of propagating the error.
func toolResult(data any) (*mcp.CallToolResult, any, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to marshal result: %v", err)}},
			IsError: true,
		}, nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(jsonBytes)}},
	}, nil, nil
}

// safeAddTool wraps mcp.AddTool in a recover to catch panics from the official
// SDK (e.g. invalid tool names, schema errors) and returns them as errors.
func safeAddTool[I, O any](srv *mcp.Server, tool *mcp.Tool, handler mcp.ToolHandlerFor[I, O]) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	mcp.AddTool(srv, tool, handler)
	return nil
}

// safeAddResource wraps srv.AddResource in a recover to catch panics from the
// official SDK (e.g. invalid URIs) and returns them as errors.
func safeAddResource(srv *mcp.Server, resource *mcp.Resource, handler mcp.ResourceHandler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	srv.AddResource(resource, handler)
	return nil
}

// wrapResourceHandler adapts a Rivaas MCPResourceHandler into an official SDK ResourceHandler.
func wrapResourceHandler(r *mcpResourceDef) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		result, err := r.handler(ctx)
		if err != nil {
			return nil, err
		}

		var text string
		switch v := result.(type) {
		case string:
			text = v
		default:
			jsonBytes, jsonErr := json.Marshal(v)
			if jsonErr != nil {
				return nil, fmt.Errorf("failed to marshal resource: %w", jsonErr)
			}
			text = string(jsonBytes)
		}

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      r.uri,
					MIMEType: "application/json",
					Text:     text,
				},
			},
		}, nil
	}
}
