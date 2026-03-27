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

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// emptyInput is used for debug tools that take no input parameters.
type emptyInput struct{}

// goroutineProfileInput defines the input schema for the goroutine_profile tool.
type goroutineProfileInput struct {
	State string `json:"state" jsonschema:"Filter by goroutine state (running/waiting/idle/all),enum=running,enum=waiting,enum=idle,enum=all,default=all"`
}

// memoryProfileInput defines the input schema for the memory_profile tool.
type memoryProfileInput struct {
	TopN int `json:"top_n" jsonschema:"Number of top allocation sites to return,minimum=1,default=20"`
}

// registerMCPDebugEndpoints wires built-in runtime tools and resources into
// an MCP server and mounts it under {settings.prefix}/mcp.
//
// Like registerMCPEndpoints, debug MCP endpoints register directly on the router
// (via mountMCPServer) and bypass App.registerRoute. They do not trigger OnRoute
// hooks or appear in the OpenAPI specification.
func (a *App) registerMCPDebugEndpoints(settings *debugSettings) error {
	s := settings.mcpDebug
	base := settings.prefix + "/mcp"

	srv := mcp.NewServer(
		&mcp.Implementation{
			Name:    a.config.serviceName + " (debug)",
			Version: a.config.serviceVersion,
		},
		nil,
	)

	startTime := time.Now()
	serviceName := a.config.serviceName
	serviceVersion := a.config.serviceVersion

	if s.runtime {
		if err := safeAddTool(srv,
			&mcp.Tool{
				Name:        "runtime_stats",
				Description: "Returns Go runtime statistics including goroutine count, memory usage, GC stats, and uptime. Includes AI-useful signals for anomaly detection.",
			},
			func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
				return toolResult(collectRuntimeStats(serviceName, serviceVersion, startTime))
			},
		); err != nil {
			return fmt.Errorf("mcp-debug: failed to add tool runtime_stats: %w", err)
		}

		if err := safeAddTool(srv,
			&mcp.Tool{
				Name:        "goroutine_profile",
				Description: "Returns a snapshot of goroutine stacks with state summary. Supports filtering by state (running/waiting/idle/all). Useful for detecting goroutine leaks and deadlocks.",
			},
			func(ctx context.Context, _ *mcp.CallToolRequest, input goroutineProfileInput) (*mcp.CallToolResult, any, error) {
				return toolResult(collectGoroutineProfile(input.State))
			},
		); err != nil {
			return fmt.Errorf("mcp-debug: failed to add tool goroutine_profile: %w", err)
		}

		if err := safeAddTool(srv,
			&mcp.Tool{
				Name:        "gc_analysis",
				Description: "Returns detailed garbage collection statistics including pause times, CPU fraction, and live object counts. Includes signals for high GC pause detection.",
			},
			func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
				return toolResult(collectGCStats())
			},
		); err != nil {
			return fmt.Errorf("mcp-debug: failed to add tool gc_analysis: %w", err)
		}

		if err := safeAddTool(srv,
			&mcp.Tool{
				Name:        "memory_profile",
				Description: "Returns top N heap allocation sites by in-use bytes. Useful for finding memory-heavy functions and potential leaks.",
			},
			func(ctx context.Context, _ *mcp.CallToolRequest, input memoryProfileInput) (*mcp.CallToolResult, any, error) {
				return toolResult(collectMemoryProfile(input.TopN))
			},
		); err != nil {
			return fmt.Errorf("mcp-debug: failed to add tool memory_profile: %w", err)
		}

		if err := safeAddResource(srv, &mcp.Resource{
			URI:         "rivaas://runtime/overview",
			Name:        "Runtime Overview",
			Description: "Live snapshot of Go runtime statistics for the running service",
			MIMEType:    "application/json",
		}, func(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			data := collectRuntimeStats(serviceName, serviceVersion, startTime)
			jsonBytes, err := json.Marshal(data)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal runtime overview: %w", err)
			}
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      "rivaas://runtime/overview",
						MIMEType: "application/json",
						Text:     string(jsonBytes),
					},
				},
			}, nil
		}); err != nil {
			return fmt.Errorf("mcp-debug: failed to add resource rivaas://runtime/overview: %w", err)
		}
	}

	if s.config {
		if err := safeAddResource(srv, &mcp.Resource{
			URI:         "rivaas://config",
			Name:        "Application Config",
			Description: "Sanitized application configuration summary (no secrets)",
			MIMEType:    "application/json",
		}, func(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
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
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      "rivaas://config",
						MIMEType: "application/json",
						Text:     string(jsonBytes),
					},
				},
			}, nil
		}); err != nil {
			return fmt.Errorf("mcp-debug: failed to add resource rivaas://config: %w", err)
		}
	}

	if s.build {
		if err := safeAddResource(srv, &mcp.Resource{
			URI:         "rivaas://build",
			Name:        "Build Info",
			Description: "Go build information including module path, Go version, and dependency versions",
			MIMEType:    "application/json",
		}, func(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			data := collectBuildInfo()
			jsonBytes, err := json.Marshal(data)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal build info: %w", err)
			}
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      "rivaas://build",
						MIMEType: "application/json",
						Text:     string(jsonBytes),
					},
				},
			}, nil
		}); err != nil {
			return fmt.Errorf("mcp-debug: failed to add resource rivaas://build: %w", err)
		}
	}

	if s.routes {
		if err := safeAddResource(srv, &mcp.Resource{
			URI:         "rivaas://routes",
			Name:        "Route Table",
			Description: "Registered HTTP routes with methods, paths, handlers, middleware, and constraints",
			MIMEType:    "application/json",
		}, func(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			data := collectRoutes(a.router.Routes())
			jsonBytes, err := json.Marshal(data)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal routes: %w", err)
			}
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      "rivaas://routes",
						MIMEType: "application/json",
						Text:     string(jsonBytes),
					},
				},
			}, nil
		}); err != nil {
			return fmt.Errorf("mcp-debug: failed to add resource rivaas://routes: %w", err)
		}
	}

	if s.health {
		healthCfg := a.config.health
		if err := safeAddTool(srv,
			&mcp.Tool{
				Name:        "health_status",
				Description: "Runs all registered liveness and readiness health checks and returns per-check pass/fail status with signals for failures.",
			},
			func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
				return toolResult(collectHealthStatus(ctx, healthCfg))
			},
		); err != nil {
			return fmt.Errorf("mcp-debug: failed to add tool health_status: %w", err)
		}
	}

	if s.openapi {
		if err := safeAddResource(srv, &mcp.Resource{
			URI:         "rivaas://openapi",
			Name:        "OpenAPI Specification",
			Description: "Generated OpenAPI specification for the service (JSON)",
			MIMEType:    "application/json",
		}, func(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			if a.openapi == nil {
				return &mcp.ReadResourceResult{
					Contents: []*mcp.ResourceContents{
						{
							URI:      "rivaas://openapi",
							MIMEType: "text/plain",
							Text:     `{"error": "OpenAPI not enabled — use app.WithOpenAPI() to enable"}`,
						},
					},
				}, nil
			}
			specJSON, _, err := a.openapi.GenerateSpec(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to generate OpenAPI spec: %w", err)
			}
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      "rivaas://openapi",
						MIMEType: "application/json",
						Text:     string(specJSON),
					},
				},
			}, nil
		}); err != nil {
			return fmt.Errorf("mcp-debug: failed to add resource rivaas://openapi: %w", err)
		}
	}

	return a.mountMCPServer(base, srv, "mcp-debug")
}
