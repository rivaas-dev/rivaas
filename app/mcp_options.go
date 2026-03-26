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

// mcpToolDef holds a tool registration.
type mcpToolDef struct {
	name             string
	description      string
	handler          MCPToolHandler
	params           []*mcpInputParam
	validationErrors []error
}

// mcpResourceDef holds a resource registration.
type mcpResourceDef struct {
	uri         string
	name        string
	description string
	handler     MCPResourceHandler
}

// MCPToolHandler is the handler signature for MCP tools.
// ctx carries the request context. args provides type-safe access to the tool's
// input arguments. Return any JSON-serializable value, or an error.
type MCPToolHandler func(ctx context.Context, args MCPToolArgs) (any, error)

// MCPResourceHandler is the handler signature for MCP resources.
// ctx carries the request context. Return any JSON-serializable value, or an error.
type MCPResourceHandler func(ctx context.Context) (any, error)

// MCPToolArgs provides type-safe access to tool input arguments.
// Developers never interact with the underlying map directly.
type MCPToolArgs struct {
	raw map[string]any
}

// NewMCPToolArgs creates an MCPToolArgs from a raw argument map.
// Useful in tests to construct tool arguments without going through mcp-go.
func NewMCPToolArgs(args map[string]any) MCPToolArgs {
	return MCPToolArgs{raw: args}
}

// String returns the string argument with the given name, or "" if missing or wrong type.
func (a MCPToolArgs) String(name string) string {
	if v, ok := a.raw[name].(string); ok {
		return v
	}
	return ""
}

// StringDefault returns the string argument with the given name, or def if missing.
func (a MCPToolArgs) StringDefault(name, def string) string {
	if v, ok := a.raw[name].(string); ok {
		return v
	}
	return def
}

// Float returns the float64 argument with the given name, or 0 if missing or wrong type.
func (a MCPToolArgs) Float(name string) float64 {
	if v, ok := a.raw[name].(float64); ok {
		return v
	}
	return 0
}

// Int returns the integer argument with the given name, or 0 if missing or wrong type.
// JSON numbers are float64; this truncates to int.
func (a MCPToolArgs) Int(name string) int {
	if v, ok := a.raw[name].(float64); ok {
		return int(v)
	}
	return 0
}

// Bool returns the boolean argument with the given name, or false if missing or wrong type.
func (a MCPToolArgs) Bool(name string) bool {
	if v, ok := a.raw[name].(bool); ok {
		return v
	}
	return false
}

// Slice returns the array argument with the given name, or nil if missing or wrong type.
func (a MCPToolArgs) Slice(name string) []any {
	if v, ok := a.raw[name].([]any); ok {
		return v
	}
	return nil
}

// Map returns the object argument with the given name, or nil if missing or wrong type.
func (a MCPToolArgs) Map(name string) map[string]any {
	if v, ok := a.raw[name].(map[string]any); ok {
		return v
	}
	return nil
}

// RequireString returns the string argument or an error if missing or wrong type.
func (a MCPToolArgs) RequireString(name string) (string, error) {
	v, ok := a.raw[name]
	if !ok {
		return "", fmt.Errorf("missing required argument: %s", name)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("argument %s: expected string, got %T", name, v)
	}
	return s, nil
}

// RequireFloat returns the float64 argument or an error if missing or wrong type.
func (a MCPToolArgs) RequireFloat(name string) (float64, error) {
	v, ok := a.raw[name]
	if !ok {
		return 0, fmt.Errorf("missing required argument: %s", name)
	}
	f, ok := v.(float64)
	if !ok {
		return 0, fmt.Errorf("argument %s: expected number, got %T", name, v)
	}
	return f, nil
}

// RequireInt returns the integer argument or an error if missing or wrong type.
func (a MCPToolArgs) RequireInt(name string) (int, error) {
	v, ok := a.raw[name]
	if !ok {
		return 0, fmt.Errorf("missing required argument: %s", name)
	}
	f, ok := v.(float64)
	if !ok {
		return 0, fmt.Errorf("argument %s: expected integer, got %T", name, v)
	}
	return int(f), nil
}

// RequireBool returns the boolean argument or an error if missing or wrong type.
func (a MCPToolArgs) RequireBool(name string) (bool, error) {
	v, ok := a.raw[name]
	if !ok {
		return false, fmt.Errorf("missing required argument: %s", name)
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("argument %s: expected boolean, got %T", name, v)
	}
	return b, nil
}

// --- Input parameter types ---

// MCPInputOption adds an input parameter definition to a tool.
type MCPInputOption func(*mcpToolDef)

// mcpInputParam holds the definition of a single input parameter.
type mcpInputParam struct {
	name        string
	description string
	paramType   string // "string", "number", "integer", "boolean", "array", "object"
	required    bool

	// Universal
	defaultValue any

	// String-specific
	enum      []string
	minLength *int
	maxLength *int
	pattern   string

	// Number/Integer-specific
	minimum          *float64
	maximum          *float64
	exclusiveMaximum *float64

	// Array-specific
	items map[string]any

	// Object-specific
	properties map[string]any
}

// WithMCPStringInput defines a string input parameter.
//
// Example:
//
//	app.WithMCPStringInput("name", "The user name", app.MCPRequired(), app.MCPMinLength(1))
func WithMCPStringInput(name, description string, opts ...MCPParamOption) MCPInputOption {
	return func(t *mcpToolDef) {
		p := &mcpInputParam{name: name, description: description, paramType: "string"}
		for i, opt := range opts {
			if opt == nil {
				t.validationErrors = append(t.validationErrors,
					fmt.Errorf("app: MCP param option at index %d for input %q cannot be nil", i, name))
				continue
			}
			opt(p)
		}
		t.params = append(t.params, p)
	}
}

// WithMCPNumberInput defines a number (float64) input parameter.
//
// Example:
//
//	app.WithMCPNumberInput("price", "Product price", app.MCPMinimum(0))
func WithMCPNumberInput(name, description string, opts ...MCPParamOption) MCPInputOption {
	return func(t *mcpToolDef) {
		p := &mcpInputParam{name: name, description: description, paramType: "number"}
		for i, opt := range opts {
			if opt == nil {
				t.validationErrors = append(t.validationErrors,
					fmt.Errorf("app: MCP param option at index %d for input %q cannot be nil", i, name))
				continue
			}
			opt(p)
		}
		t.params = append(t.params, p)
	}
}

// WithMCPIntegerInput defines an integer input parameter.
//
// Example:
//
//	app.WithMCPIntegerInput("page", "Page number", app.MCPMinimum(1))
func WithMCPIntegerInput(name, description string, opts ...MCPParamOption) MCPInputOption {
	return func(t *mcpToolDef) {
		p := &mcpInputParam{name: name, description: description, paramType: "integer"}
		for i, opt := range opts {
			if opt == nil {
				t.validationErrors = append(t.validationErrors,
					fmt.Errorf("app: MCP param option at index %d for input %q cannot be nil", i, name))
				continue
			}
			opt(p)
		}
		t.params = append(t.params, p)
	}
}

// WithMCPBooleanInput defines a boolean input parameter.
//
// Example:
//
//	app.WithMCPBooleanInput("active", "Filter active only", app.MCPDefault(false))
func WithMCPBooleanInput(name, description string, opts ...MCPParamOption) MCPInputOption {
	return func(t *mcpToolDef) {
		p := &mcpInputParam{name: name, description: description, paramType: "boolean"}
		for i, opt := range opts {
			if opt == nil {
				t.validationErrors = append(t.validationErrors,
					fmt.Errorf("app: MCP param option at index %d for input %q cannot be nil", i, name))
				continue
			}
			opt(p)
		}
		t.params = append(t.params, p)
	}
}

// WithMCPArrayInput defines an array input parameter.
//
// Example:
//
//	app.WithMCPArrayInput("tags", "Filter by tags", app.MCPItems(map[string]any{"type": "string"}))
func WithMCPArrayInput(name, description string, opts ...MCPParamOption) MCPInputOption {
	return func(t *mcpToolDef) {
		p := &mcpInputParam{name: name, description: description, paramType: "array"}
		for i, opt := range opts {
			if opt == nil {
				t.validationErrors = append(t.validationErrors,
					fmt.Errorf("app: MCP param option at index %d for input %q cannot be nil", i, name))
				continue
			}
			opt(p)
		}
		t.params = append(t.params, p)
	}
}

// WithMCPObjectInput defines an object input parameter.
//
// Example:
//
//	app.WithMCPObjectInput("filters", "Query filters")
func WithMCPObjectInput(name, description string, opts ...MCPParamOption) MCPInputOption {
	return func(t *mcpToolDef) {
		p := &mcpInputParam{name: name, description: description, paramType: "object"}
		for i, opt := range opts {
			if opt == nil {
				t.validationErrors = append(t.validationErrors,
					fmt.Errorf("app: MCP param option at index %d for input %q cannot be nil", i, name))
				continue
			}
			opt(p)
		}
		t.params = append(t.params, p)
	}
}

// --- MCPParamOption modifiers ---

// MCPParamOption modifies a parameter definition.
type MCPParamOption func(*mcpInputParam)

// MCPRequired marks the parameter as required.
func MCPRequired() MCPParamOption {
	return func(p *mcpInputParam) { p.required = true }
}

// MCPDefault sets a default value for the parameter.
// Applies to: string, number, integer, boolean.
func MCPDefault(v any) MCPParamOption {
	return func(p *mcpInputParam) { p.defaultValue = v }
}

// MCPEnum restricts the parameter to a set of allowed string values.
// Applies to: string only.
func MCPEnum(values ...string) MCPParamOption {
	return func(p *mcpInputParam) { p.enum = values }
}

// MCPMinLength sets the minimum string length.
// Applies to: string only.
func MCPMinLength(n int) MCPParamOption {
	return func(p *mcpInputParam) { p.minLength = &n }
}

// MCPMaxLength sets the maximum string length.
// Applies to: string only.
func MCPMaxLength(n int) MCPParamOption {
	return func(p *mcpInputParam) { p.maxLength = &n }
}

// MCPPattern sets a regex pattern for string validation.
// Applies to: string only.
func MCPPattern(pattern string) MCPParamOption {
	return func(p *mcpInputParam) { p.pattern = pattern }
}

// MCPMinimum sets the minimum value for numeric parameters.
// Applies to: number, integer.
func MCPMinimum(v float64) MCPParamOption {
	return func(p *mcpInputParam) { p.minimum = &v }
}

// MCPMaximum sets the maximum value for numeric parameters.
// Applies to: number, integer.
func MCPMaximum(v float64) MCPParamOption {
	return func(p *mcpInputParam) { p.maximum = &v }
}

// MCPExclusiveMaximum sets the exclusive maximum for number parameters.
// Applies to: number only.
func MCPExclusiveMaximum(v float64) MCPParamOption {
	return func(p *mcpInputParam) { p.exclusiveMaximum = &v }
}

// MCPItems sets the schema for array items.
// Applies to: array only.
func MCPItems(schema map[string]any) MCPParamOption {
	return func(p *mcpInputParam) { p.items = schema }
}

// MCPProperties sets the schema for object properties.
// Applies to: object only.
func MCPProperties(schema map[string]any) MCPParamOption {
	return func(p *mcpInputParam) { p.properties = schema }
}

// --- Tool and resource registration ---

// WithMCPTool registers a tool on the business MCP server.
//
// Example:
//
//	app.WithMCP(
//	    app.WithMCPTool("get_order", "Get an order by ID",
//	        func(ctx context.Context, args app.MCPToolArgs) (any, error) {
//	            id, _ := args.RequireString("order_id")
//	            return orderService.GetByID(ctx, id)
//	        },
//	        app.WithMCPStringInput("order_id", "The order ID", app.MCPRequired()),
//	    ),
//	)
func WithMCPTool(name, description string, handler MCPToolHandler, inputs ...MCPInputOption) MCPOption {
	return func(s *mcpSettings) {
		if handler == nil {
			s.validationErrors = append(s.validationErrors,
				fmt.Errorf("app: MCP tool %q handler cannot be nil", name))
		}
		td := mcpToolDef{
			name:        name,
			description: description,
			handler:     handler,
		}
		for i, input := range inputs {
			if input == nil {
				s.validationErrors = append(s.validationErrors,
					fmt.Errorf("app: MCP tool %q input option at index %d cannot be nil", name, i))
				continue
			}
			input(&td)
		}
		s.validationErrors = append(s.validationErrors, td.validationErrors...)
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
// They never import mcp-go directly.
//
// The MCP server is mounted at /mcp by default (configurable with [WithMCPPrefix]).
// It uses the Streamable HTTP transport (GET + POST + DELETE on a single endpoint).
//
// Example:
//
//	app.MustNew(
//	    app.WithMCP(
//	        app.WithMCPTool("get_order", "Get an order by ID",
//	            func(ctx context.Context, args app.MCPToolArgs) (any, error) {
//	                id, _ := args.RequireString("order_id")
//	                return orderService.GetByID(ctx, id)
//	            },
//	            app.WithMCPStringInput("order_id", "The order ID", app.MCPRequired()),
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
