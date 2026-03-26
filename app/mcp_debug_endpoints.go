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
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"rivaas.dev/router"
	"rivaas.dev/router/route"
)

// registerMCPDebugEndpoints wires built-in runtime tools and resources into
// an mcp-go server and mounts it under {settings.prefix}/mcp.
func (a *App) registerMCPDebugEndpoints(settings *debugSettings) error {
	s := settings.mcpDebug
	base := settings.prefix + "/mcp"

	methods := []string{"GET", "POST", "DELETE"}
	for _, m := range methods {
		if a.router.RouteExists(m, base) {
			return fmt.Errorf("route already registered: %s %s", m, base)
		}
	}

	srv := server.NewMCPServer(
		a.config.serviceName+" (debug)",
		a.config.serviceVersion,
		server.WithRecovery(),
	)

	startTime := time.Now()
	serviceName := a.config.serviceName
	serviceVersion := a.config.serviceVersion

	if s.runtime {
		runtimeStatsTool := mcp.NewTool("runtime_stats",
			mcp.WithDescription("Returns Go runtime statistics including goroutine count, memory usage, GC stats, and uptime. Includes AI-useful signals for anomaly detection."),
		)
		if err := safeAddTool(srv, runtimeStatsTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data := collectRuntimeStats(serviceName, serviceVersion, startTime)
			jsonBytes, err := json.Marshal(data)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to marshal runtime stats: %v", err)), nil
			}
			return mcp.NewToolResultText(string(jsonBytes)), nil
		}); err != nil {
			return fmt.Errorf("mcp-debug: failed to add tool runtime_stats: %w", err)
		}

		goroutineProfileTool := mcp.NewTool("goroutine_profile",
			mcp.WithDescription("Returns a snapshot of all goroutine stacks with state summary. Useful for detecting goroutine leaks and deadlocks."),
		)
		if err := safeAddTool(srv, goroutineProfileTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data := collectGoroutineProfile()
			jsonBytes, err := json.Marshal(data)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to marshal goroutine profile: %v", err)), nil
			}
			return mcp.NewToolResultText(string(jsonBytes)), nil
		}); err != nil {
			return fmt.Errorf("mcp-debug: failed to add tool goroutine_profile: %w", err)
		}

		gcAnalysisTool := mcp.NewTool("gc_analysis",
			mcp.WithDescription("Returns detailed garbage collection statistics including pause times, CPU fraction, and live object counts. Includes signals for high GC pause detection."),
		)
		if err := safeAddTool(srv, gcAnalysisTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data := collectGCStats()
			jsonBytes, err := json.Marshal(data)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to marshal GC stats: %v", err)), nil
			}
			return mcp.NewToolResultText(string(jsonBytes)), nil
		}); err != nil {
			return fmt.Errorf("mcp-debug: failed to add tool gc_analysis: %w", err)
		}

		runtimeResource := mcp.Resource{
			URI:         "rivaas://runtime/overview",
			Name:        "Runtime Overview",
			Description: "Live snapshot of Go runtime statistics for the running service",
			MIMEType:    "application/json",
		}
		if err := safeAddResource(srv, runtimeResource, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			data := collectRuntimeStats(serviceName, serviceVersion, startTime)
			jsonBytes, err := json.Marshal(data)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal runtime overview: %w", err)
			}
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      "rivaas://runtime/overview",
					MIMEType: "application/json",
					Text:     string(jsonBytes),
				},
			}, nil
		}); err != nil {
			return fmt.Errorf("mcp-debug: failed to add resource rivaas://runtime/overview: %w", err)
		}
	}

	if s.config {
		configResource := mcp.Resource{
			URI:         "rivaas://config",
			Name:        "Application Config",
			Description: "Sanitized application configuration summary (no secrets)",
			MIMEType:    "application/json",
		}
		if err := safeAddResource(srv, configResource, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			cfg := a.config
			summary := map[string]any{
				"service_name":    cfg.serviceName,
				"service_version": cfg.serviceVersion,
				"environment":     cfg.environment,
				"server": map[string]any{
					"port":             cfg.server.port,
					"host":             cfg.server.host,
					"read_timeout":     cfg.server.readTimeout.String(),
					"write_timeout":    cfg.server.writeTimeout.String(),
					"idle_timeout":     cfg.server.idleTimeout.String(),
					"shutdown_timeout": cfg.server.shutdownTimeout.String(),
					"max_header_bytes": cfg.server.maxHeaderBytes,
					"tls_enabled":      cfg.server.tlsCertFile != "",
					"mtls_enabled":     len(cfg.server.mtlsServerCert.Certificate) > 0,
				},
				"health_enabled":  cfg.health != nil && cfg.health.enabled,
				"debug_enabled":   cfg.debug != nil && cfg.debug.enabled,
				"openapi_enabled": cfg.openapi != nil && cfg.openapi.enabled,
			}
			jsonBytes, err := json.Marshal(summary)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal config: %w", err)
			}
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      "rivaas://config",
					MIMEType: "application/json",
					Text:     string(jsonBytes),
				},
			}, nil
		}); err != nil {
			return fmt.Errorf("mcp-debug: failed to add resource rivaas://config: %w", err)
		}
	}

	if s.build {
		buildResource := mcp.Resource{
			URI:         "rivaas://build",
			Name:        "Build Info",
			Description: "Go build information including module path, Go version, and dependency versions",
			MIMEType:    "application/json",
		}
		if err := safeAddResource(srv, buildResource, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			data := collectBuildInfo()
			jsonBytes, err := json.Marshal(data)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal build info: %w", err)
			}
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      "rivaas://build",
					MIMEType: "application/json",
					Text:     string(jsonBytes),
				},
			}, nil
		}); err != nil {
			return fmt.Errorf("mcp-debug: failed to add resource rivaas://build: %w", err)
		}
	}

	httpServer := server.NewStreamableHTTPServer(srv)

	handler := func(c *router.Context) {
		httpServer.ServeHTTP(c.Response, c.Request)
	}

	a.router.GET(base, handler)
	a.router.POST(base, handler)
	a.router.DELETE(base, handler)

	for _, m := range methods {
		a.router.UpdateRouteInfo(m, base, "", func(info *route.Info) {
			info.HandlerName = "[builtin] mcp-debug"
		})
	}

	return nil
}
