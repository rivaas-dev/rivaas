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

import "fmt"

// MCPDebugOption configures the debug MCP server that exposes Go runtime
// internals to AI tools. These options are used inside WithDebugEndpoints.
type MCPDebugOption func(*mcpDebugSettings)

// mcpDebugSettings holds debug MCP server configuration.
type mcpDebugSettings struct {
	enabled          bool
	runtime          bool // runtime_stats, goroutine_profile, gc_analysis, memory_profile tools + rivaas://runtime/overview resource
	config           bool // rivaas://config resource
	build            bool // rivaas://build resource
	routes           bool // rivaas://routes resource
	health           bool // health_status tool
	openapi          bool // rivaas://openapi resource
	validationErrors []error
}

// WithMCPDebug enables the debug MCP server that exposes Go runtime information
// to AI tools via the Model Context Protocol. The MCP server is mounted at
// {debug.prefix}/mcp (default: /_internal/debug/mcp).
//
// Security rationale: the debug MCP server is disabled by default and requires
// explicit opt-in because it exposes sensitive runtime information:
//
// Attack vectors:
//   - Runtime stats reveal memory usage patterns useful for resource exhaustion attacks
//   - Goroutine dumps reveal internal logic and potential race conditions
//   - Config summaries may leak internal service topology
//   - Build info reveals dependency versions with known vulnerabilities
//
// Safe usage patterns:
//  1. Development: enable all features unconditionally
//  2. Staging: enable behind VPN or IP allowlist
//  3. Production: enable only with proper authentication middleware
//
// Example:
//
//	app.MustNew(
//	    app.WithDebugEndpoints(
//	        app.WithMCPDebug(
//	            app.WithMCPDebugRuntime(),
//	            app.WithMCPDebugConfig(),
//	            app.WithMCPDebugBuild(),
//	            app.WithMCPDebugRoutes(),
//	            app.WithMCPDebugHealth(),
//	            app.WithMCPDebugOpenAPI(),
//	        ),
//	    ),
//	)
//	// MCP debug server: http://localhost:8080/_internal/debug/mcp
func WithMCPDebug(opts ...MCPDebugOption) DebugOption {
	return func(s *debugSettings) {
		if s.mcpDebug == nil {
			s.mcpDebug = &mcpDebugSettings{enabled: true}
		}
		for i, opt := range opts {
			if opt == nil {
				s.mcpDebug.validationErrors = append(s.mcpDebug.validationErrors,
					fmt.Errorf("app: MCP debug option at index %d cannot be nil", i))
				continue
			}
			opt(s.mcpDebug)
		}
		// When called without sub-options, enable all features as sensible defaults
		// rather than creating an empty MCP server with no tools or resources.
		if !s.mcpDebug.runtime && !s.mcpDebug.config && !s.mcpDebug.build &&
			!s.mcpDebug.routes && !s.mcpDebug.health && !s.mcpDebug.openapi {
			s.mcpDebug.runtime = true
			s.mcpDebug.config = true
			s.mcpDebug.build = true
			s.mcpDebug.routes = true
			s.mcpDebug.health = true
			s.mcpDebug.openapi = true
		}
	}
}

// WithMCPDebugRuntime enables runtime introspection tools in the debug MCP server.
//
// Tools registered:
//   - runtime_stats: returns goroutine count, memory usage, GC stats, and uptime
//   - goroutine_profile: returns a snapshot of all goroutine stacks
//   - gc_analysis: returns detailed garbage collection statistics
//
// Resources registered:
//   - rivaas://runtime/overview: live snapshot of runtime statistics
//
// Example:
//
//	app.WithDebugEndpoints(
//	    app.WithMCPDebug(
//	        app.WithMCPDebugRuntime(),
//	    ),
//	)
func WithMCPDebugRuntime() MCPDebugOption {
	return func(s *mcpDebugSettings) {
		s.runtime = true
	}
}

// WithMCPDebugConfig enables the config resource in the debug MCP server.
//
// Resources registered:
//   - rivaas://config: sanitized application configuration summary
//
// Example:
//
//	app.WithDebugEndpoints(
//	    app.WithMCPDebug(
//	        app.WithMCPDebugConfig(),
//	    ),
//	)
func WithMCPDebugConfig() MCPDebugOption {
	return func(s *mcpDebugSettings) {
		s.config = true
	}
}

// WithMCPDebugBuild enables the build info resource in the debug MCP server.
//
// Resources registered:
//   - rivaas://build: Go build information from runtime/debug.ReadBuildInfo()
//
// Example:
//
//	app.WithDebugEndpoints(
//	    app.WithMCPDebug(
//	        app.WithMCPDebugBuild(),
//	    ),
//	)
func WithMCPDebugBuild() MCPDebugOption {
	return func(s *mcpDebugSettings) {
		s.build = true
	}
}

// WithMCPDebugRoutes enables the routes resource in the debug MCP server.
//
// Resources registered:
//   - rivaas://routes: registered route table (method, path, handler, middleware, constraints)
//
// Example:
//
//	app.WithDebugEndpoints(
//	    app.WithMCPDebug(
//	        app.WithMCPDebugRoutes(),
//	    ),
//	)
func WithMCPDebugRoutes() MCPDebugOption {
	return func(s *mcpDebugSettings) {
		s.routes = true
	}
}

// WithMCPDebugHealth enables the health status tool in the debug MCP server.
//
// Tools registered:
//   - health_status: runs all registered liveness and readiness checks
//
// Example:
//
//	app.WithDebugEndpoints(
//	    app.WithMCPDebug(
//	        app.WithMCPDebugHealth(),
//	    ),
//	)
func WithMCPDebugHealth() MCPDebugOption {
	return func(s *mcpDebugSettings) {
		s.health = true
	}
}

// WithMCPDebugOpenAPI enables the OpenAPI spec resource in the debug MCP server.
// If OpenAPI is not enabled on the app, the resource returns an informative error.
//
// Resources registered:
//   - rivaas://openapi: generated OpenAPI specification (JSON)
//
// Example:
//
//	app.WithDebugEndpoints(
//	    app.WithMCPDebug(
//	        app.WithMCPDebugOpenAPI(),
//	    ),
//	)
func WithMCPDebugOpenAPI() MCPDebugOption {
	return func(s *mcpDebugSettings) {
		s.openapi = true
	}
}

// WithMCPDebugIf conditionally enables the debug MCP server.
// When cond is false, the returned option is a no-op.
//
// Example:
//
//	app.WithDebugEndpoints(
//	    app.WithMCPDebugIf(os.Getenv("MCP_DEBUG") == "true",
//	        app.WithMCPDebugRuntime(),
//	    ),
//	)
func WithMCPDebugIf(cond bool, opts ...MCPDebugOption) DebugOption {
	if !cond {
		return func(s *debugSettings) {}
	}
	return WithMCPDebug(opts...)
}
