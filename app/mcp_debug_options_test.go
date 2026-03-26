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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithMCPDebug_enablesDebugMCP(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithDebugEndpoints(
			WithMCPDebug(
				WithMCPDebugRuntime(),
			),
		),
	)
	require.NoError(t, err)
	require.NotNil(t, a)

	assert.NotNil(t, a.config.debug.mcpDebug)
	assert.True(t, a.config.debug.mcpDebug.enabled)
	assert.True(t, a.config.debug.mcpDebug.runtime)
}

func TestWithMCPDebug_allFeatureFlags(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithDebugEndpoints(
			WithMCPDebug(
				WithMCPDebugRuntime(),
				WithMCPDebugConfig(),
				WithMCPDebugBuild(),
			),
		),
	)
	require.NoError(t, err)
	require.NotNil(t, a)

	s := a.config.debug.mcpDebug
	assert.True(t, s.enabled)
	assert.True(t, s.runtime)
	assert.True(t, s.config)
	assert.True(t, s.build)
}

func TestWithMCPDebug_featureFlagsIndependent(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithDebugEndpoints(
			WithMCPDebug(
				WithMCPDebugBuild(),
			),
		),
	)
	require.NoError(t, err)
	require.NotNil(t, a)

	s := a.config.debug.mcpDebug
	assert.True(t, s.enabled)
	assert.False(t, s.runtime)
	assert.False(t, s.config)
	assert.True(t, s.build)
}

func TestWithMCPDebug_mountsAtCorrectPath(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithDebugEndpoints(
			WithMCPDebug(
				WithMCPDebugRuntime(),
			),
		),
	)
	require.NoError(t, err)

	// POST to MCP debug endpoint should be handled
	req := httptest.NewRequest(http.MethodPost, "/_internal/debug/mcp", nil)
	rec := httptest.NewRecorder()
	a.Router().ServeHTTP(rec, req)
	// MCP protocol expects JSON-RPC, but at minimum the route should exist (not 404)
	assert.NotEqual(t, http.StatusNotFound, rec.Code)
}

func TestWithMCPDebug_customPrefixShiftsMount(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithDebugEndpoints(
			WithDebugPrefix("/custom"),
			WithMCPDebug(
				WithMCPDebugRuntime(),
			),
		),
	)
	require.NoError(t, err)

	// Should be mounted at /custom/mcp
	req := httptest.NewRequest(http.MethodPost, "/custom/mcp", nil)
	rec := httptest.NewRecorder()
	a.Router().ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusNotFound, rec.Code)

	// Default path should 404
	req = httptest.NewRequest(http.MethodPost, "/_internal/debug/mcp", nil)
	rec = httptest.NewRecorder()
	a.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestWithMCPDebug_withoutOptionsEnablesAllFeatures(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithDebugEndpoints(
			WithMCPDebug(),
		),
	)
	require.NoError(t, err)

	s := a.config.debug.mcpDebug
	assert.True(t, s.runtime, "bare WithMCPDebug() should enable runtime")
	assert.True(t, s.config, "bare WithMCPDebug() should enable config")
	assert.True(t, s.build, "bare WithMCPDebug() should enable build")

	req := httptest.NewRequest(http.MethodPost, "/_internal/debug/mcp", nil)
	rec := httptest.NewRecorder()
	a.Router().ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusNotFound, rec.Code)
}

func TestWithMCPDebug_nilOptionReturnsError(t *testing.T) {
	t.Parallel()

	_, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithDebugEndpoints(
			WithMCPDebug(nil),
		),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MCP debug option")
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestWithMCPDebugIf_conditionTrue(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithDebugEndpoints(
			WithMCPDebugIf(true, WithMCPDebugRuntime()),
		),
	)
	require.NoError(t, err)
	require.NotNil(t, a.config.debug.mcpDebug)
	assert.True(t, a.config.debug.mcpDebug.enabled)
	assert.True(t, a.config.debug.mcpDebug.runtime)
}

func TestWithMCPDebugIf_conditionFalse(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithDebugEndpoints(
			WithMCPDebugIf(false, WithMCPDebugRuntime()),
		),
	)
	require.NoError(t, err)
	assert.Nil(t, a.config.debug.mcpDebug)
}
