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

//go:build !integration

package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/router"
)

func TestWithMCP_enablesMCP(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithMCP(),
	)
	require.NoError(t, err)
	require.NotNil(t, a)

	assert.NotNil(t, a.config.mcp)
	assert.True(t, a.config.mcp.enabled)
	assert.Equal(t, "/mcp", a.config.mcp.prefix)
}

func TestWithMCP_nilOptionReturnsError(t *testing.T) {
	t.Parallel()

	_, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithMCP(nil),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MCP option")
	assert.Contains(t, err.Error(), "cannot be nil")

	var ce *ConfigErrors
	require.True(t, errors.As(err, &ce))
}

func TestWithMCPPrefix(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithMCP(
			WithMCPPrefix("/api/mcp"),
		),
	)
	require.NoError(t, err)
	assert.Equal(t, "/api/mcp", a.config.mcp.prefix)
}

func TestWithMCPTool_registersToolOnServer(t *testing.T) {
	t.Parallel()

	handler := func(ctx context.Context, args MCPToolArgs) (any, error) {
		return map[string]string{"result": "ok"}, nil
	}

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithMCP(
			WithMCPTool("test_tool", "A test tool", handler,
				WithMCPStringInput("name", "The name", MCPRequired()),
			),
		),
	)
	require.NoError(t, err)

	// POST to /mcp should be handled (not 404)
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	rec := httptest.NewRecorder()
	a.Router().ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusNotFound, rec.Code)
}

func TestWithMCPResource_registersResourceOnServer(t *testing.T) {
	t.Parallel()

	handler := func(ctx context.Context) (any, error) {
		return map[string]string{"data": "hello"}, nil
	}

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithMCP(
			WithMCPResource("test://data", "Test Data", "Some test data", handler),
		),
	)
	require.NoError(t, err)

	// POST to /mcp should be handled (not 404)
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	rec := httptest.NewRecorder()
	a.Router().ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusNotFound, rec.Code)
}

func TestWithMCPIf_conditionTrue(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithMCPIf(true),
	)
	require.NoError(t, err)
	assert.NotNil(t, a.config.mcp)
	assert.True(t, a.config.mcp.enabled)
}

func TestWithMCPIf_conditionFalse(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithMCPIf(false),
	)
	require.NoError(t, err)
	assert.Nil(t, a.config.mcp)
}

func TestWithMCP_routeCollision(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)
	require.NoError(t, err)

	a.Router().POST("/mcp", func(c *router.Context) {
		_, writeErr := c.Response.Write([]byte("collision"))
		require.NoError(t, writeErr)
	})
	a.Router().Freeze()

	err = a.registerMCPEndpoints(&mcpSettings{
		enabled: true,
		prefix:  "/mcp",
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "route already registered")
}

func TestMCPToolArgs_accessors(t *testing.T) {
	t.Parallel()

	args := NewMCPToolArgs(map[string]any{
		"name":   "alice",
		"age":    float64(30),
		"score":  float64(99.5),
		"active": true,
		"tags":   []any{"a", "b"},
		"meta":   map[string]any{"k": "v"},
	})

	assert.Equal(t, "alice", args.String("name"))
	assert.Equal(t, "", args.String("missing"))
	assert.Equal(t, "default", args.StringDefault("missing", "default"))
	assert.Equal(t, "alice", args.StringDefault("name", "default"))

	assert.Equal(t, 30, args.Int("age"))
	assert.Equal(t, 0, args.Int("missing"))

	assert.InDelta(t, 99.5, args.Float("score"), 0.001)
	assert.InDelta(t, 0.0, args.Float("missing"), 0.001)

	assert.True(t, args.Bool("active"))
	assert.False(t, args.Bool("missing"))

	assert.Equal(t, []any{"a", "b"}, args.Slice("tags"))
	assert.Nil(t, args.Slice("missing"))

	assert.Equal(t, map[string]any{"k": "v"}, args.Map("meta"))
	assert.Nil(t, args.Map("missing"))
}

func TestMCPToolArgs_requireAccessors(t *testing.T) {
	t.Parallel()

	args := NewMCPToolArgs(map[string]any{
		"name":   "alice",
		"age":    float64(30),
		"score":  float64(99.5),
		"active": true,
	})

	s, err := args.RequireString("name")
	require.NoError(t, err)
	assert.Equal(t, "alice", s)

	_, err = args.RequireString("missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required argument")

	_, err = args.RequireString("age")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected string")

	i, err := args.RequireInt("age")
	require.NoError(t, err)
	assert.Equal(t, 30, i)

	_, err = args.RequireInt("missing")
	assert.Error(t, err)

	f, err := args.RequireFloat("score")
	require.NoError(t, err)
	assert.InDelta(t, 99.5, f, 0.001)

	_, err = args.RequireFloat("missing")
	assert.Error(t, err)

	b, err := args.RequireBool("active")
	require.NoError(t, err)
	assert.True(t, b)

	_, err = args.RequireBool("missing")
	assert.Error(t, err)
}

func TestWithMCPTool_nilHandlerReturnsError(t *testing.T) {
	t.Parallel()

	_, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithMCP(
			WithMCPTool("broken", "A tool with nil handler", nil),
		),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "handler cannot be nil")
}

func TestWithMCPTool_nilInputReturnsError(t *testing.T) {
	t.Parallel()

	handler := func(ctx context.Context, args MCPToolArgs) (any, error) {
		return "ok", nil
	}

	_, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithMCP(
			WithMCPTool("broken", "A tool with nil input", handler, nil),
		),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "input option")
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestWithMCPResource_nilHandlerReturnsError(t *testing.T) {
	t.Parallel()

	_, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithMCP(
			WithMCPResource("test://data", "Test", "desc", nil),
		),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "handler cannot be nil")
}

func TestWithMCPTool_duplicateNameReturnsError(t *testing.T) {
	t.Parallel()

	handler := func(ctx context.Context, args MCPToolArgs) (any, error) {
		return "ok", nil
	}

	_, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithMCP(
			WithMCPTool("dup", "First", handler),
			WithMCPTool("dup", "Second", handler),
		),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate tool name")
}

func TestWithMCPStringInput_nilParamOptionReturnsError(t *testing.T) {
	t.Parallel()

	handler := func(ctx context.Context, args MCPToolArgs) (any, error) {
		return "ok", nil
	}

	_, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithMCP(
			WithMCPTool("test_tool", "desc", handler,
				WithMCPStringInput("name", "The name", nil),
			),
		),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "param option")
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestMCPToolInput_allTypes(t *testing.T) {
	t.Parallel()

	handler := func(ctx context.Context, args MCPToolArgs) (any, error) {
		return "ok", nil
	}

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithMCP(
			WithMCPTool("multi_input", "Tool with all input types", handler,
				WithMCPStringInput("q", "query", MCPRequired(), MCPMinLength(1), MCPMaxLength(100), MCPPattern("^[a-z]+$")),
				WithMCPStringInput("cat", "category", MCPEnum("a", "b", "c")),
				WithMCPNumberInput("price", "price", MCPMinimum(0), MCPMaximum(1000), MCPDefault(10.0)),
				WithMCPIntegerInput("page", "page", MCPMinimum(1)),
				WithMCPBooleanInput("active", "filter", MCPDefault(true)),
				WithMCPArrayInput("tags", "tags", MCPItems(map[string]any{"type": "string"})),
				WithMCPObjectInput("meta", "metadata", MCPProperties(map[string]any{"key": map[string]any{"type": "string"}})),
				WithMCPNumberInput("rate", "rate", MCPExclusiveMaximum(100.0)),
			),
		),
	)
	require.NoError(t, err)
	require.NotNil(t, a)

	assert.Len(t, a.config.mcp.tools, 1)
	assert.Len(t, a.config.mcp.tools[0].params, 8)
}
