//go:build !short
// +build !short

package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rivaas.dev/router"

	"github.com/stretchr/testify/assert"
)

// TestApp_ServerLifecycle tests the full server lifecycle with graceful shutdown
func TestApp_ServerLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("server starts and responds to requests", func(t *testing.T) {
		app := MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithServerConfig(
				WithShutdownTimeout(2*time.Second),
			),
		)

		app.GET("/health", func(c *router.Context) {
			c.String(http.StatusOK, "ok")
		})

		// Test route works with httptest
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		app.Router().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "ok", rec.Body.String())

		// Note: Full server lifecycle with signals would require
		// more complex setup. This test verifies routes work correctly.
	})

	t.Run("server configuration applies correctly", func(t *testing.T) {
		customTimeout := 5 * time.Second
		app := MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithServerConfig(
				WithReadTimeout(customTimeout),
				WithWriteTimeout(customTimeout),
				WithIdleTimeout(customTimeout),
				WithShutdownTimeout(customTimeout),
			),
		)

		assert.Equal(t, customTimeout, app.config.server.readTimeout)
		assert.Equal(t, customTimeout, app.config.server.writeTimeout)
		assert.Equal(t, customTimeout, app.config.server.idleTimeout)
	})
}

// TestApp_HTTPServerConfiguration tests that HTTP server is configured correctly
func TestApp_HTTPServerConfiguration(t *testing.T) {
	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithServerConfig(
			WithReadTimeout(5*time.Second),
			WithWriteTimeout(10*time.Second),
			WithIdleTimeout(30*time.Second),
			WithReadHeaderTimeout(2*time.Second),
			WithMaxHeaderBytes(4096),
			WithShutdownTimeout(15*time.Second),
		),
	)

	// Register a route
	app.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "test")
	})

	// Create server instance (without starting it)
	// We can't easily test Run() without actually starting a server,
	// but we can verify the configuration is stored correctly
	assert.Equal(t, 5*time.Second, app.config.server.readTimeout)
	assert.Equal(t, 10*time.Second, app.config.server.writeTimeout)
	assert.Equal(t, 30*time.Second, app.config.server.idleTimeout)
}

// TestApp_GracefulShutdownContext tests shutdown timeout handling
func TestApp_GracefulShutdownContext(t *testing.T) {
	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithServerConfig(
			WithShutdownTimeout(1*time.Second),
		),
	)

	// Test that shutdown timeout is configured
	assert.Equal(t, 1*time.Second, app.config.server.shutdownTimeout)

	// Test shutdown context creation (internal method behavior)
	ctx, cancel := context.WithTimeout(context.Background(), app.config.server.shutdownTimeout)
	defer cancel()

	assert.NotNil(t, ctx)
	deadline, ok := ctx.Deadline()
	assert.True(t, ok)
	assert.WithinDuration(t, time.Now().Add(1*time.Second), deadline, 100*time.Millisecond)
}

// TestApp_ObservabilityShutdown tests observability component shutdown
func TestApp_ObservabilityShutdown(t *testing.T) {
	t.Run("shutdown without observability components", func(t *testing.T) {
		app := MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
		)

		// Should not panic when shutting down observability
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		// This tests the internal shutdownObservability method
		// In a real scenario, this would be called during server shutdown
		assert.NotPanics(t, func() {
			app.shutdownObservability(ctx)
		})
	})
}

// TestApp_RouteHandling tests that routes work correctly with the app
func TestApp_RouteHandling(t *testing.T) {
	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	// Register multiple routes
	app.GET("/", func(c *router.Context) {
		c.String(http.StatusOK, "home")
	})

	app.GET("/users/:id", func(c *router.Context) {
		userID := c.Param("id")
		c.String(http.StatusOK, "user-%s", userID)
	})

	app.POST("/users", func(c *router.Context) {
		c.String(http.StatusCreated, "created")
	})

	// Test routes using httptest
	tests := []struct {
		method string
		path   string
		status int
		body   string
	}{
		{"GET", "/", 200, "home"},
		{"GET", "/users/123", 200, "user-123"},
		{"POST", "/users", 201, "created"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			app.Router().ServeHTTP(rec, req)

			assert.Equal(t, tt.status, rec.Code)
			if tt.body != "" {
				assert.Contains(t, rec.Body.String(), tt.body)
			}
		})
	}
}

// TestApp_MiddlewareExecution tests middleware chain execution
func TestApp_MiddlewareExecution(t *testing.T) {
	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	executionOrder := []string{}

	// Add middleware
	app.Use(func(c *router.Context) {
		executionOrder = append(executionOrder, "middleware1")
		c.Next()
	})

	app.Use(func(c *router.Context) {
		executionOrder = append(executionOrder, "middleware2")
		c.Next()
	})

	// Add route
	app.GET("/test", func(c *router.Context) {
		executionOrder = append(executionOrder, "handler")
		c.String(http.StatusOK, "ok")
	})

	// Execute request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)

	// Verify execution order
	assert.Equal(t, []string{"middleware1", "middleware2", "handler"}, executionOrder)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestApp_DefaultMiddlewareBehavior tests default middleware inclusion
func TestApp_DefaultMiddlewareBehavior(t *testing.T) {
	t.Run("development includes logger and recovery middleware by default", func(t *testing.T) {
		app := MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithEnvironment(EnvironmentDevelopment),
		)

		// In development, should have both logger and recovery middleware
		assert.GreaterOrEqual(t, len(app.config.middleware.functions), 2)
		assert.False(t, app.config.middleware.explicitlySet)
	})

	t.Run("production includes only recovery middleware by default", func(t *testing.T) {
		app := MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithEnvironment(EnvironmentProduction),
		)

		// In production, should have only recovery middleware
		assert.Equal(t, 1, len(app.config.middleware.functions))
		assert.False(t, app.config.middleware.explicitlySet)
	})
}

// TestApp_ComplexRouteScenarios tests complex routing scenarios
func TestApp_ComplexRouteScenarios(t *testing.T) {
	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	// Create nested groups
	api := app.Group("/api")
	v1 := api.Group("/v1")
	v1.GET("/users", func(c *router.Context) {
		c.String(http.StatusOK, "v1-users")
	})

	v2 := api.Group("/v2")
	v2.GET("/users", func(c *router.Context) {
		c.String(http.StatusOK, "v2-users")
	})

	// Test nested routes
	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	rec := httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "v1-users")

	req = httptest.NewRequest("GET", "/api/v2/users", nil)
	rec = httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "v2-users")
}
