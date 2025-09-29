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
	"bytes"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintRoutes_Output(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test-service"),
		WithServiceVersion("1.0.0"),
		WithEnvironment(EnvironmentDevelopment),
	)
	require.NoError(t, err)

	// Register some test routes
	app.GET("/", func(c *Context) {
		c.String(http.StatusOK, "root")
	})
	app.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "user")
	})
	app.POST("/users", func(c *Context) {
		c.String(http.StatusCreated, "created")
	})

	// Capture output
	var buf bytes.Buffer
	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	// Print routes
	app.PrintRoutes()

	// Restore stdout
	_ = w.Close() //nolint:errcheck // Test cleanup
	os.Stdout = originalStdout

	// Read captured output
	buf.ReadFrom(r)
	output := buf.String()

	// Verify output contains expected elements
	assert.Contains(t, output, "Method")
	assert.Contains(t, output, "Path")
	assert.Contains(t, output, "Handler")
	assert.Contains(t, output, "GET")
	assert.Contains(t, output, "POST")
	assert.Contains(t, output, "/")
	assert.Contains(t, output, "/users")
}

func TestRenderRoutesTable_EmptyRoutes(t *testing.T) {
	t.Parallel()

	app, err := New()
	require.NoError(t, err)

	var buf bytes.Buffer
	app.renderRoutesTable(&buf, 80)

	// Should produce no output for empty routes
	assert.Empty(t, buf.String())
}

func TestRenderRoutesTable_WithRoutes(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithEnvironment(EnvironmentDevelopment),
	)
	require.NoError(t, err)

	app.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "ok")
	})

	var buf bytes.Buffer
	app.renderRoutesTable(&buf, 120)

	output := buf.String()
	assert.Contains(t, output, "GET")
	assert.Contains(t, output, "/test")
	assert.Contains(t, output, "Method")
	assert.Contains(t, output, "Path")
}

func TestGetColorWriter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		environment string
	}{
		{
			name:        "production mode",
			environment: EnvironmentProduction,
		},
		{
			name:        "development mode",
			environment: EnvironmentDevelopment,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app, err := New(
				WithEnvironment(tt.environment),
			)
			require.NoError(t, err)

			w := app.getColorWriter(&bytes.Buffer{})
			assert.NotNil(t, w)
		})
	}
}
