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

type testToolInput struct {
	Name string `json:"name" jsonschema:"The name"`
}

type emptyToolInput struct{}

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

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithMCP(
			WithMCPTool("test_tool", "A test tool",
				func(ctx context.Context, input testToolInput) (any, error) {
					return map[string]string{"result": "ok"}, nil
				},
			),
		),
	)
	require.NoError(t, err)

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

func TestWithMCPTool_nilHandlerReturnsError(t *testing.T) {
	t.Parallel()

	_, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithMCP(
			WithMCPTool[emptyToolInput]("broken", "A tool with nil handler", nil),
		),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "handler cannot be nil")
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

	handler := func(ctx context.Context, input emptyToolInput) (any, error) {
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

func TestWithMCPTool_multipleTools(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithMCP(
			WithMCPTool("tool_a", "First tool",
				func(ctx context.Context, input emptyToolInput) (any, error) {
					return "a", nil
				},
			),
			WithMCPTool("tool_b", "Second tool",
				func(ctx context.Context, input testToolInput) (any, error) {
					return input.Name, nil
				},
			),
		),
	)
	require.NoError(t, err)
	require.NotNil(t, a)

	assert.Len(t, a.config.mcp.tools, 2)
}
