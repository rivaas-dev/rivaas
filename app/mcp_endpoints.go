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

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"rivaas.dev/router"
	"rivaas.dev/router/route"
)

// registerMCPEndpoints translates Rivaas mcpSettings into mcp-go registrations
// and mounts the Streamable HTTP server on the router.
func (a *App) registerMCPEndpoints(s *mcpSettings) error {
	base := s.prefix
	if base == "" {
		base = "/mcp"
	}

	methods := []string{"GET", "POST", "DELETE"}
	for _, m := range methods {
		if a.router.RouteExists(m, base) {
			return fmt.Errorf("route already registered: %s %s", m, base)
		}
	}

	srv := server.NewMCPServer(
		a.config.serviceName,
		a.config.serviceVersion,
		server.WithRecovery(),
	)

	for i := range s.tools {
		t := &s.tools[i]
		mcpTool := buildMCPTool(t)
		handler := wrapToolHandler(t.handler)
		if err := safeAddTool(srv, mcpTool, handler); err != nil {
			return fmt.Errorf("mcp: failed to add tool %q: %w", t.name, err)
		}
	}

	for i := range s.resources {
		r := &s.resources[i]
		mcpResource := buildMCPResource(r)
		handler := wrapResourceHandler(r)
		if err := safeAddResource(srv, mcpResource, handler); err != nil {
			return fmt.Errorf("mcp: failed to add resource %q: %w", r.uri, err)
		}
	}

	httpServer := server.NewStreamableHTTPServer(srv)

	routeHandler := func(c *router.Context) {
		httpServer.ServeHTTP(c.Response, c.Request)
	}

	a.router.GET(base, routeHandler)
	a.router.POST(base, routeHandler)
	a.router.DELETE(base, routeHandler)

	for _, m := range methods {
		a.router.UpdateRouteInfo(m, base, "", func(info *route.Info) {
			info.HandlerName = "[builtin] mcp"
		})
	}

	return nil
}

// buildMCPTool translates a Rivaas mcpToolDef into an mcp-go mcp.Tool.
func buildMCPTool(t *mcpToolDef) mcp.Tool {
	var toolOpts []mcp.ToolOption
	toolOpts = append(toolOpts, mcp.WithDescription(t.description))

	for _, p := range t.params {
		toolOpts = append(toolOpts, buildParamOption(p))
	}

	return mcp.NewTool(t.name, toolOpts...)
}

// buildParamOption converts a Rivaas mcpInputParam into an mcp-go ToolOption.
func buildParamOption(p *mcpInputParam) mcp.ToolOption {
	var propOpts []mcp.PropertyOption

	propOpts = append(propOpts, mcp.Description(p.description))

	if p.required {
		propOpts = append(propOpts, mcp.Required())
	}

	switch p.paramType {
	case "string":
		if p.defaultValue != nil {
			if s, ok := p.defaultValue.(string); ok {
				propOpts = append(propOpts, mcp.DefaultString(s))
			}
		}
		if len(p.enum) > 0 {
			propOpts = append(propOpts, mcp.Enum(p.enum...))
		}
		if p.minLength != nil {
			propOpts = append(propOpts, mcp.MinLength(*p.minLength))
		}
		if p.maxLength != nil {
			propOpts = append(propOpts, mcp.MaxLength(*p.maxLength))
		}
		if p.pattern != "" {
			propOpts = append(propOpts, mcp.Pattern(p.pattern))
		}
		return mcp.WithString(p.name, propOpts...)

	case "number":
		if p.defaultValue != nil {
			if f, ok := p.defaultValue.(float64); ok {
				propOpts = append(propOpts, mcp.DefaultNumber(f))
			}
		}
		if p.minimum != nil {
			propOpts = append(propOpts, mcp.Min(*p.minimum))
		}
		if p.maximum != nil {
			propOpts = append(propOpts, mcp.Max(*p.maximum))
		}
		if p.exclusiveMaximum != nil {
			propOpts = append(propOpts, exclusiveMaxPropOption(*p.exclusiveMaximum))
		}
		return mcp.WithNumber(p.name, propOpts...)

	case "integer":
		if p.defaultValue != nil {
			if f, ok := p.defaultValue.(float64); ok {
				propOpts = append(propOpts, mcp.DefaultNumber(f))
			}
		}
		if p.minimum != nil {
			propOpts = append(propOpts, mcp.Min(*p.minimum))
		}
		if p.maximum != nil {
			propOpts = append(propOpts, mcp.Max(*p.maximum))
		}
		// mcp-go has no WithInteger; use WithNumber and override the type to "integer"
		propOpts = append(propOpts, integerTypeOverride())
		return mcp.WithNumber(p.name, propOpts...)

	case "boolean":
		if p.defaultValue != nil {
			if b, ok := p.defaultValue.(bool); ok {
				propOpts = append(propOpts, mcp.DefaultBool(b))
			}
		}
		return mcp.WithBoolean(p.name, propOpts...)

	case "array":
		if p.items != nil {
			propOpts = append(propOpts, mcp.Items(p.items))
		}
		return mcp.WithArray(p.name, propOpts...)

	case "object":
		if p.properties != nil {
			propOpts = append(propOpts, mcp.Properties(p.properties))
		}
		return mcp.WithObject(p.name, propOpts...)

	default:
		return mcp.WithString(p.name, propOpts...)
	}
}

// buildMCPResource translates a Rivaas mcpResourceDef into an mcp-go mcp.Resource.
func buildMCPResource(r *mcpResourceDef) mcp.Resource {
	return mcp.Resource{
		URI:         r.uri,
		Name:        r.name,
		Description: r.description,
		MIMEType:    "application/json",
	}
}

// wrapToolHandler adapts a Rivaas MCPToolHandler into an mcp-go ToolHandlerFunc.
func wrapToolHandler(handler MCPToolHandler) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := MCPToolArgs{raw: req.GetArguments()}

		result, err := handler(ctx, args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		switch v := result.(type) {
		case string:
			return mcp.NewToolResultText(v), nil
		default:
			jsonBytes, jsonErr := json.Marshal(v)
			if jsonErr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", jsonErr)), nil
			}
			return mcp.NewToolResultText(string(jsonBytes)), nil
		}
	}
}

// integerTypeOverride is a PropertyOption that changes the JSON Schema type from "number" to "integer".
func integerTypeOverride() mcp.PropertyOption {
	return func(schema map[string]any) {
		schema["type"] = "integer"
	}
}

// exclusiveMaxPropOption is a PropertyOption that sets the exclusiveMaximum constraint.
func exclusiveMaxPropOption(v float64) mcp.PropertyOption {
	return func(schema map[string]any) {
		schema["exclusiveMaximum"] = v
	}
}

// safeAddTool wraps srv.AddTool in a recover to catch panics from mcp-go
// (e.g. tool name collision with task tools) and returns them as errors.
func safeAddTool(srv *server.MCPServer, tool mcp.Tool, handler server.ToolHandlerFunc) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	srv.AddTool(tool, handler)
	return nil
}

// safeAddResource wraps srv.AddResource in a recover to catch panics from mcp-go.
func safeAddResource(srv *server.MCPServer, resource mcp.Resource, handler server.ResourceHandlerFunc) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	srv.AddResource(resource, handler)
	return nil
}

// wrapResourceHandler adapts a Rivaas MCPResourceHandler into an mcp-go ResourceHandlerFunc.
func wrapResourceHandler(r *mcpResourceDef) server.ResourceHandlerFunc {
	return func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
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

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      r.uri,
				MIMEType: "application/json",
				Text:     text,
			},
		}, nil
	}
}
