package app

import (
	"bytes"
	"os"
	"testing"

	"rivaas.dev/router"

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
	app.GET("/", func(c *router.Context) {
		c.String(200, "root")
	})
	app.GET("/users/:id", func(c *router.Context) {
		c.String(200, "user")
	})
	app.POST("/users", func(c *router.Context) {
		c.String(201, "created")
	})

	// Capture output
	var buf bytes.Buffer
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Print routes
	app.PrintRoutes()

	// Restore stdout
	w.Close()
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

	app.GET("/test", func(c *router.Context) {
		c.String(200, "ok")
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
		tt := tt
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
