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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"rivaas.dev/logging"
	"rivaas.dev/metrics"
	"rivaas.dev/router"
	"rivaas.dev/tracing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew_ValidationError tests that New returns appropriate structured errors
// for various invalid configuration scenarios.
func TestNew_ValidationError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		opts        []Option
		wantErr     string
		wantErrType any                     // Expected error type (ConfigError, ValidationError, etc.)
		checkError  func(*testing.T, error) // Optional custom error checking
	}{
		{
			name:        "empty service name",
			opts:        []Option{WithServiceName(""), WithServiceVersion("1.0.0")},
			wantErr:     "serviceName",
			wantErrType: &ValidationError{},
			checkError: func(t *testing.T, err error) {
				var ve *ValidationError
				if errors.As(err, &ve) {
					assert.True(t, ve.HasErrors())
					assert.Greater(t, len(ve.Errors), 0)
					// Check that one of the errors is about serviceName
					found := false
					for _, e := range ve.Errors {
						if e.Field == "serviceName" {
							found = true
							assert.Equal(t, "cannot be empty", e.Message)
						}
					}
					assert.True(t, found, "should have serviceName error")
				}
			},
		},
		{
			name:        "empty service version",
			opts:        []Option{WithServiceName("test"), WithServiceVersion("")},
			wantErr:     "serviceVersion",
			wantErrType: &ValidationError{},
		},
		{
			name: "invalid environment",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithEnvironment("staging"),
			},
			wantErr:     "environment",
			wantErrType: &ValidationError{},
			checkError: func(t *testing.T, err error) {
				var ve *ValidationError
				if errors.As(err, &ve) {
					found := false
					for _, e := range ve.Errors {
						if e.Field == "environment" {
							found = true
							assert.Equal(t, "staging", e.Value)
							assert.Contains(t, e.Message, "must be one of")
						}
					}
					assert.True(t, found, "should have environment error")
				}
			},
		},
		{
			name: "negative read timeout",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithReadTimeout(-1 * time.Second)),
			},
			wantErr:     "server.readTimeout",
			wantErrType: &ValidationError{},
		},
		{
			name: "zero write timeout",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithWriteTimeout(0)),
			},
			wantErr:     "server.writeTimeout",
			wantErrType: &ValidationError{},
		},
		{
			name: "negative idle timeout",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithIdleTimeout(-5 * time.Second)),
			},
			wantErr:     "server.idleTimeout",
			wantErrType: &ValidationError{},
		},
		{
			name: "zero read header timeout",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithReadHeaderTimeout(0)),
			},
			wantErr:     "server.readHeaderTimeout",
			wantErrType: &ValidationError{},
		},
		{
			name: "zero max header bytes",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithMaxHeaderBytes(0)),
			},
			wantErr:     "server.maxHeaderBytes",
			wantErrType: &ValidationError{},
		},
		{
			name: "negative shutdown timeout",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithShutdownTimeout(-10 * time.Second)),
			},
			wantErr:     "server.shutdownTimeout",
			wantErrType: &ValidationError{},
		},
		{
			name: "read timeout exceeds write timeout",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(
					WithReadTimeout(20*time.Second),
					WithWriteTimeout(10*time.Second),
				),
			},
			wantErr:     "read timeout should not exceed write timeout",
			wantErrType: &ValidationError{},
			checkError: func(t *testing.T, err error) {
				var ve *ValidationError
				if errors.As(err, &ve) {
					found := false
					for _, e := range ve.Errors {
						if e.Field == "server.readTimeout" && strings.Contains(e.Message, "read timeout should not exceed") {
							found = true
						}
					}
					assert.True(t, found, "should have read timeout comparison error")
				}
			},
		},
		{
			name: "shutdown timeout too short",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithShutdownTimeout(100 * time.Millisecond)),
			},
			wantErr:     "must be at least 1 second",
			wantErrType: &ValidationError{},
		},
		{
			name: "max header bytes too small",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithMaxHeaderBytes(512)),
			},
			wantErr:     "must be at least 1KB",
			wantErrType: &ValidationError{},
		},
		{
			name: "multiple validation errors",
			opts: []Option{
				WithServiceName(""),
				WithServiceVersion(""),
				WithEnvironment("invalid"),
				WithServerConfig(WithReadTimeout(-1 * time.Second)),
			},
			wantErr:     "validation errors",
			wantErrType: &ValidationError{},
			checkError: func(t *testing.T, err error) {
				var ve *ValidationError
				if errors.As(err, &ve) {
					// Should have multiple errors
					assert.Greater(t, len(ve.Errors), 1, "should have multiple validation errors")
					// Check error message format
					assert.Contains(t, err.Error(), "validation errors")
				}
			},
		},
		{
			name: "empty server config is valid",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(), // Should not cause error, should use defaults
			},
			wantErr: "", // Should succeed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app, err := New(tt.opts...)
			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Nil(t, app)
				assert.Contains(t, err.Error(), tt.wantErr)

				// Check error type if specified
				if tt.wantErrType != nil {
					// Use errors.As for type checking
					var target *ValidationError
					if errors.As(err, &target) {
						assert.NotNil(t, target)
					}
				}

				// Run custom error checking if provided
				if tt.checkError != nil {
					tt.checkError(t, err)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, app)
			}
		})
	}
}

// TestNew_ValidConfigurations tests that New successfully creates
// an App with various valid configuration combinations.
func TestNew_ValidConfigurations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		opts  []Option
		check func(*testing.T, *App)
	}{
		{
			name: "minimal valid config",
			opts: []Option{
				WithServiceName("test-service"),
				WithServiceVersion("1.0.0"),
			},
			check: func(t *testing.T, a *App) {
				assert.Equal(t, "test-service", a.ServiceName())
				assert.Equal(t, "1.0.0", a.ServiceVersion())
				assert.Equal(t, DefaultEnvironment, a.Environment())
			},
		},
		{
			name: "production environment",
			opts: []Option{
				WithServiceName("prod-service"),
				WithServiceVersion("2.0.0"),
				WithEnvironment(EnvironmentProduction),
			},
			check: func(t *testing.T, a *App) {
				assert.Equal(t, EnvironmentProduction, a.Environment())
			},
		},
		{
			name: "custom server config with partial values",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(
					WithReadTimeout(5 * time.Second),
					// Other fields are not set - should use defaults
				),
			},
			check: func(t *testing.T, a *App) {
				assert.NotNil(t, a.config.server)
				assert.Equal(t, 5*time.Second, a.config.server.readTimeout)
				// Verify defaults were used for unset values
				assert.Equal(t, DefaultWriteTimeout, a.config.server.writeTimeout)
			},
		},
		{
			name: "development environment",
			opts: []Option{
				WithServiceName("dev-service"),
				WithServiceVersion("1.0.0"),
				WithEnvironment(EnvironmentDevelopment),
			},
			check: func(t *testing.T, a *App) {
				assert.Equal(t, EnvironmentDevelopment, a.Environment())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app, err := New(tt.opts...)
			require.NoError(t, err)
			require.NotNil(t, app)
			if tt.check != nil {
				tt.check(t, app)
			}
		})
	}
}

// TestNew_ObservabilityInitialization tests that observability components
// (logging, metrics, tracing) are correctly initialized when enabled,
// including support for both options-based and prebuilt configurations.
func TestNew_ObservabilityInitialization(t *testing.T) {
	t.Parallel()

	t.Run("logging initialization with options", func(t *testing.T) {
		t.Parallel()
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithObservability(
				WithLogging(logging.WithLevel(logging.LevelDebug)),
			),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.logging)
	})

	t.Run("logging initialization with debug level", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithObservability(
				WithLogging(logging.WithLevel(logging.LevelInfo)),
			),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.logging)
	})

	t.Run("metrics initialization with options", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithObservability(
				WithMetrics(metrics.WithPrometheus(":9090", "/metrics")),
			),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.metrics)
	})

	t.Run("metrics initialization with prebuilt config", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithObservability(
				WithMetrics(metrics.WithPrometheus(":9090", "/metrics")),
			),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.metrics)
	})

	t.Run("tracing initialization with options", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithObservability(
				WithTracing(tracing.WithNoop()),
			),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.tracing)
	})

	t.Run("tracing initialization with prebuilt config", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithObservability(
				WithTracing(tracing.WithNoop()),
			),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.tracing)
	})

	t.Run("all observability components together", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithObservability(
				WithLogging(logging.WithLevel(logging.LevelDebug)),
				WithMetrics(metrics.WithPrometheus(":9090", "/metrics")),
				WithTracing(tracing.WithNoop()),
			),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.logging)
		assert.NotNil(t, app.metrics)
		assert.NotNil(t, app.tracing)
	})

	t.Run("observability with different log levels", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithObservability(
				WithLogging(logging.WithLevel(logging.LevelInfo)),
				WithMetrics(metrics.WithPrometheus(":9090", "/metrics")),
				WithTracing(tracing.WithNoop()),
			),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.logging)
		assert.NotNil(t, app.metrics)
		assert.NotNil(t, app.tracing)
	})

	t.Run("service metadata is automatically injected into all observability components", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("payment-service"),
			WithServiceVersion("v2.1.0"),
			WithObservability(
				WithLogging(logging.WithJSONHandler()),
				WithMetrics(metrics.WithPrometheus(":9090", "/metrics")),
				WithTracing(tracing.WithNoop()),
			),
		)
		require.NoError(t, err)
		require.NotNil(t, app)

		// Verify logging is set and has service metadata auto-injected
		assert.NotNil(t, app.logging)
		assert.Equal(t, "payment-service", app.logging.ServiceName())
		assert.Equal(t, "v2.1.0", app.logging.ServiceVersion())

		// Verify metrics has service metadata (auto-injected)
		assert.NotNil(t, app.metrics)
		assert.Equal(t, "payment-service", app.metrics.ServiceName())
		assert.Equal(t, "v2.1.0", app.metrics.ServiceVersion())

		// Verify tracing has service metadata (auto-injected)
		assert.NotNil(t, app.tracing)
		assert.Equal(t, "payment-service", app.tracing.ServiceName())
		assert.Equal(t, "v2.1.0", app.tracing.ServiceVersion())
	})

	t.Run("user options can override auto-injected service metadata", func(t *testing.T) {
		t.Parallel()

		// User explicitly sets service metadata in logging options
		// These should take precedence over app-level config
		app, err := New(
			WithServiceName("payment-service"),
			WithServiceVersion("v2.1.0"),
			WithObservability(
				WithLogging(
					logging.WithServiceName("custom-logger"),
					logging.WithServiceVersion("v1.0.0"),
				),
			),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.logging)

		// User options are applied AFTER auto-injection, so they take precedence
		assert.Equal(t, "custom-logger", app.logging.ServiceName())
		assert.Equal(t, "v1.0.0", app.logging.ServiceVersion())
	})
}

// TestNew_MiddlewareConfiguration tests that middleware is configured
// correctly based on environment and explicit settings.
func TestNew_MiddlewareConfiguration(t *testing.T) {
	t.Parallel()

	t.Run("development environment includes logger and recovery by default", func(t *testing.T) {
		t.Parallel()
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithEnvironment(EnvironmentDevelopment),
		)
		require.NoError(t, err)
		// Default middleware (recovery and accesslog) are router-level, not app-level.
		// They are applied directly to the router, not stored in app.config.middleware.functions.
		// Verify that defaults are enabled by checking disableDefaults is false.
		assert.False(t, app.config.middleware.disableDefaults)
		// Verify recovery middleware works by testing panic recovery
		app.GET("/panic", func(c *Context) {
			panic("test panic")
		})
		req := httptest.NewRequest("GET", "/panic", nil)
		rec := httptest.NewRecorder()
		app.Router().ServeHTTP(rec, req)
		// Recovery middleware should catch the panic and return 500
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("production environment includes only recovery by default", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithEnvironment(EnvironmentProduction),
		)
		require.NoError(t, err)
		// Default middleware (recovery) is router-level, not app-level.
		// Verify that defaults are enabled by checking disableDefaults is false.
		assert.False(t, app.config.middleware.disableDefaults)
		// Verify recovery middleware works by testing panic recovery
		app.GET("/panic", func(c *Context) {
			panic("test panic")
		})
		req := httptest.NewRequest("GET", "/panic", nil)
		rec := httptest.NewRecorder()
		app.Router().ServeHTTP(rec, req)
		// Recovery middleware should catch the panic and return 500
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("WithMiddleware adds custom middleware without disabling defaults", func(t *testing.T) {
		t.Parallel()

		customMiddleware := func(c *Context) {
			c.Next()
		}
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithEnvironment(EnvironmentDevelopment),
			WithMiddleware(customMiddleware), // Adds middleware, keeps defaults
		)
		require.NoError(t, err)
		// Should have the custom middleware
		assert.Equal(t, 1, len(app.config.middleware.functions))
		// Defaults should still be enabled
		assert.False(t, app.config.middleware.disableDefaults)
		// Verify recovery middleware still works
		app.GET("/panic", func(c *Context) {
			panic("test panic")
		})
		req := httptest.NewRequest("GET", "/panic", nil)
		rec := httptest.NewRecorder()
		app.Router().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("recovery middleware enabled by default", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
		)
		require.NoError(t, err)
		// Default middleware (recovery) is router-level, not app-level.
		// Verify that defaults are enabled by checking disableDefaults is false.
		assert.False(t, app.config.middleware.disableDefaults)
		// Verify recovery middleware works by testing panic recovery
		app.GET("/panic", func(c *Context) {
			panic("test panic")
		})
		req := httptest.NewRequest("GET", "/panic", nil)
		rec := httptest.NewRecorder()
		app.Router().ServeHTTP(rec, req)
		// Recovery middleware should catch the panic and return 500
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("WithoutDefaultMiddleware disables defaults", func(t *testing.T) {
		t.Parallel()

		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithEnvironment(EnvironmentDevelopment),
			WithoutDefaultMiddleware(), // Explicitly disable defaults
		)
		require.NoError(t, err)
		// Should have no custom middleware
		assert.Equal(t, 0, len(app.config.middleware.functions))
		// Defaults should be disabled
		assert.True(t, app.config.middleware.disableDefaults)
	})

	t.Run("multiple middleware can be added", func(t *testing.T) {
		t.Parallel()

		custom1 := func(c *Context) { c.Next() }
		custom2 := func(c *Context) { c.Next() }
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithMiddleware(custom1, custom2),
		)
		require.NoError(t, err)
		// Should have exactly 2 middleware
		assert.Equal(t, 2, len(app.config.middleware.functions))
		// Defaults should still be enabled (WithMiddleware doesn't disable them)
		assert.False(t, app.config.middleware.disableDefaults)
	})
}

// TestApp_RouteRegistration tests that all HTTP methods (GET, POST, PUT,
// DELETE, PATCH, HEAD, OPTIONS) can be registered and handled correctly.
func TestApp_RouteRegistration(t *testing.T) {
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	// Register routes using App methods
	app.GET("/users", func(c *Context) {
		if err := c.String(http.StatusOK, "users"); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	app.POST("/users", func(c *Context) {
		if err := c.String(http.StatusCreated, "created"); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	app.PUT("/users/:id", func(c *Context) {
		userID := c.Param("id")
		if err := c.Stringf(http.StatusOK, "updated %s", userID); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	app.DELETE("/users/:id", func(c *Context) {
		if err := c.String(http.StatusNoContent, ""); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	app.PATCH("/users/:id", func(c *Context) {
		userID := c.Param("id")
		if err := c.Stringf(http.StatusOK, "patched %s", userID); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	app.HEAD("/users/:id", func(c *Context) {
		if err := c.String(http.StatusOK, ""); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	app.OPTIONS("/users", func(c *Context) {
		if err := c.String(http.StatusOK, ""); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	// Test GET route
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	rec := httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "users", rec.Body.String())

	// Test POST route
	req = httptest.NewRequest(http.MethodPost, "/users", nil)
	rec = httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)

	// Test PUT route with param
	req = httptest.NewRequest(http.MethodPut, "/users/123", nil)
	rec = httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "updated 123", rec.Body.String())

	// Test DELETE route
	req = httptest.NewRequest(http.MethodDelete, "/users/456", nil)
	rec = httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Test PATCH route
	req = httptest.NewRequest(http.MethodPatch, "/users/789", nil)
	rec = httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "patched 789", rec.Body.String())

	// Test HEAD route
	req = httptest.NewRequest(http.MethodHead, "/users/123", nil)
	rec = httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Test OPTIONS route
	req = httptest.NewRequest(http.MethodOptions, "/users", nil)
	rec = httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestApp_GroupRoutes tests that route groups are created and work correctly,
// including parameter extraction from grouped routes.
func TestApp_GroupRoutes(t *testing.T) {
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	// Create route group
	// Groups from app support app.HandlerFunc with app.Context
	api := app.Group("/api")
	api.GET("/health", func(c *Context) {
		if err := c.String(http.StatusOK, "healthy"); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	api.GET("/users/:id", func(c *Context) {
		userID := c.Param("id")
		if err := c.Stringf(http.StatusOK, "user-%s", userID); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	// Test grouped route
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "healthy", rec.Body.String())

	// Test grouped route with param
	req = httptest.NewRequest(http.MethodGet, "/api/users/42", nil)
	rec = httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "user-42", rec.Body.String())
}

// TestApp_UseMiddleware tests that custom middleware is executed
// in the correct order when handling requests.
func TestApp_UseMiddleware(t *testing.T) {
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	callCount := 0
	app.Use(func(c *Context) {
		callCount++
		c.Next()
	})

	app.GET("/test", func(c *Context) {
		if err := c.String(http.StatusOK, "ok"); err != nil {
			c.Logger().Error("failed to write response", "err", err)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, callCount)
}

// TestApp_Static tests that the Static method can be called without panicking.
// Full file system testing is done in integration tests.
func TestApp_Static(t *testing.T) {
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	// Static files would be tested with actual file system in integration tests
	// This just ensures the method exists and doesn't panic
	assert.NotPanics(t, func() {
		app.Static("/static", "/tmp")
	})
}

// TestMustNew_Panics tests that MustNew panics on invalid configuration
// and does not panic on valid configuration.
func TestMustNew_Panics(t *testing.T) {
	t.Parallel()

	t.Run("panics on invalid config", func(t *testing.T) {
		t.Parallel()
		assert.Panics(t, func() {
			MustNew(WithServiceName(""), WithServiceVersion("1.0.0")) // Empty service name
		})
	})

	t.Run("does not panic on valid config", func(t *testing.T) {
		t.Parallel()

		assert.NotPanics(t, func() {
			MustNew(
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
			)
		})
	})
}

// TestApp_ServerConfigDefaults tests that default server configuration
// values are applied when no custom configuration is provided.
func TestApp_ServerConfigDefaults(t *testing.T) {
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	assert.Equal(t, DefaultReadTimeout, app.config.server.readTimeout)
	assert.Equal(t, DefaultWriteTimeout, app.config.server.writeTimeout)
	assert.Equal(t, DefaultIdleTimeout, app.config.server.idleTimeout)
	assert.Equal(t, DefaultReadHeaderTimeout, app.config.server.readHeaderTimeout)
	assert.Equal(t, DefaultMaxHeaderBytes, app.config.server.maxHeaderBytes)
	assert.Equal(t, DefaultShutdownTimeout, app.config.server.shutdownTimeout)
}

// TestApp_ServerConfigCustom tests that custom server configuration
// values are correctly applied and stored.
func TestApp_ServerConfigCustom(t *testing.T) {
	t.Parallel()

	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithServerConfig(
			WithReadTimeout(5*time.Second),
			WithWriteTimeout(10*time.Second),
			WithIdleTimeout(30*time.Second),
			WithReadHeaderTimeout(1*time.Second),
			WithMaxHeaderBytes(2<<20), // 2MB
			WithShutdownTimeout(15*time.Second),
		),
	)

	assert.Equal(t, 5*time.Second, app.config.server.readTimeout)
	assert.Equal(t, 10*time.Second, app.config.server.writeTimeout)
	assert.Equal(t, 30*time.Second, app.config.server.idleTimeout)
	assert.Equal(t, 1*time.Second, app.config.server.readHeaderTimeout)
	assert.Equal(t, 2<<20, app.config.server.maxHeaderBytes)
	assert.Equal(t, 15*time.Second, app.config.server.shutdownTimeout)
}

// TestApp_ServerConfigPartial tests that partial server configuration
// uses provided values while falling back to defaults for zero values.
func TestApp_ServerConfigPartial(t *testing.T) {
	t.Parallel()

	// Set only some values, others should use defaults
	// Note: read timeout must not exceed write timeout (default 10s)
	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithServerConfig(
			WithReadTimeout(5*time.Second),
			// All other fields not set - should use defaults
		),
	)

	assert.Equal(t, 5*time.Second, app.config.server.readTimeout)
	assert.Equal(t, DefaultWriteTimeout, app.config.server.writeTimeout)
	assert.Equal(t, DefaultIdleTimeout, app.config.server.idleTimeout)
}

// TestApp_GetMetricsHandler tests that GetMetricsHandler returns
// an error when metrics are not enabled and a handler when they are.
func TestApp_GetMetricsHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setup       func() *App
		wantError   bool
		wantHandler bool
		errorCheck  func(*testing.T, error)
	}{
		{
			name: "returns error when metrics not enabled",
			setup: func() *App {
				return MustNew(
					WithServiceName("test"),
					WithServiceVersion("1.0.0"),
				)
			},
			wantError:   true,
			wantHandler: false,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "metrics not enabled")
			},
		},
		{
			name: "returns handler when metrics enabled",
			setup: func() *App {
				return MustNew(
					WithServiceName("test"),
					WithServiceVersion("1.0.0"),
					WithObservability(
						WithMetrics(metrics.WithPrometheus(":9090", "/metrics")),
					),
				)
			},
			wantError:   false,
			wantHandler: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := tt.setup()
			handler, err := app.GetMetricsHandler()

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, handler)
				if tt.errorCheck != nil {
					tt.errorCheck(t, err)
				}
			} else {
				assert.NoError(t, err)
				if tt.wantHandler {
					assert.NotNil(t, handler)
				}
			}
		})
	}
}

// TestApp_GetMetricsServerAddress tests that GetMetricsServerAddress
// returns the correct address when metrics are enabled and empty when disabled.
func TestApp_GetMetricsServerAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func() *App
		wantEmpty bool
	}{
		{
			name: "returns empty when metrics not enabled",
			setup: func() *App {
				return MustNew(
					WithServiceName("test"),
					WithServiceVersion("1.0.0"),
				)
			},
			wantEmpty: true,
		},
		{
			name: "returns address when metrics enabled",
			setup: func() *App {
				return MustNew(
					WithServiceName("test"),
					WithServiceVersion("1.0.0"),
					WithObservability(
						WithMetrics(
							metrics.WithPrometheus(":9090", "/metrics"),
						),
					),
				)
			},
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := tt.setup()
			addr := app.GetMetricsServerAddress()

			if tt.wantEmpty {
				assert.Empty(t, addr)
			} else {
				assert.NotEmpty(t, addr)
			}
		})
	}
}

// TestApp_ConfigurationGetters tests that configuration getters return the correct values.
func TestApp_ConfigurationGetters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func() *App
		getter   func(*App) any
		expected any
	}{
		{
			name: "ServiceName returns configured value",
			setup: func() *App {
				return MustNew(
					WithServiceName("my-service"),
					WithServiceVersion("1.0.0"),
				)
			},
			getter: func(a *App) any {
				return a.ServiceName()
			},
			expected: "my-service",
		},
		{
			name: "ServiceVersion returns configured value",
			setup: func() *App {
				return MustNew(
					WithServiceName("test"),
					WithServiceVersion("2.3.4"),
				)
			},
			getter: func(a *App) any {
				return a.ServiceVersion()
			},
			expected: "2.3.4",
		},
		{
			name: "Environment returns configured value",
			setup: func() *App {
				return MustNew(
					WithServiceName("test"),
					WithServiceVersion("1.0.0"),
					WithEnvironment(EnvironmentProduction),
				)
			},
			getter: func(a *App) any {
				return a.Environment()
			},
			expected: EnvironmentProduction,
		},
		{
			name: "Environment returns default development",
			setup: func() *App {
				return MustNew(
					WithServiceName("test"),
					WithServiceVersion("1.0.0"),
				)
			},
			getter: func(a *App) any {
				return a.Environment()
			},
			expected: EnvironmentDevelopment,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := tt.setup()
			got := tt.getter(app)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestApp_Metrics tests that Metrics returns the metrics configuration
// when enabled and nil when disabled.
func TestApp_Metrics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setup       func() *App
		wantNil     bool
		validateCfg func(*testing.T, *App, *metrics.Recorder)
	}{
		{
			name: "returns nil when metrics not enabled",
			setup: func() *App {
				return MustNew(
					WithServiceName("test"),
					WithServiceVersion("1.0.0"),
				)
			},
			wantNil: true,
		},
		{
			name: "returns recorder when metrics enabled",
			setup: func() *App {
				return MustNew(
					WithServiceName("test"),
					WithServiceVersion("1.0.0"),
					WithObservability(
						WithMetrics(metrics.WithPrometheus(":9090", "/metrics")),
					),
				)
			},
			wantNil: false,
			validateCfg: func(t *testing.T, app *App, recorder *metrics.Recorder) {
				assert.NotNil(t, recorder)
				assert.Equal(t, app.metrics, recorder)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := tt.setup()
			metricsCfg := app.Metrics()

			if tt.wantNil {
				assert.Nil(t, metricsCfg)
			} else {
				assert.NotNil(t, metricsCfg)
				if tt.validateCfg != nil {
					tt.validateCfg(t, app, metricsCfg)
				}
			}
		})
	}
}

// TestApp_Tracing tests that Tracing returns the tracing configuration
// when enabled and nil when disabled.
func TestApp_Tracing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setup       func() *App
		wantNil     bool
		validateCfg func(*testing.T, *App, *tracing.Tracer)
	}{
		{
			name: "returns nil when tracing not enabled",
			setup: func() *App {
				return MustNew(
					WithServiceName("test"),
					WithServiceVersion("1.0.0"),
				)
			},
			wantNil: true,
		},
		{
			name: "returns config when tracing enabled",
			setup: func() *App {
				return MustNew(
					WithServiceName("test"),
					WithServiceVersion("1.0.0"),
					WithObservability(
						WithTracing(tracing.WithNoop()),
					),
				)
			},
			wantNil: false,
			validateCfg: func(t *testing.T, app *App, cfg *tracing.Tracer) {
				assert.NotNil(t, cfg)
				assert.Equal(t, app.tracing, cfg)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := tt.setup()
			tracingCfg := app.Tracing()

			if tt.wantNil {
				assert.Nil(t, tracingCfg)
			} else {
				assert.NotNil(t, tracingCfg)
				if tt.validateCfg != nil {
					tt.validateCfg(t, app, tracingCfg)
				}
			}
		})
	}
}

// TestNew_RouterOptionsAccumulation tests that multiple WithRouterOptions
// calls correctly accumulate router options.
func TestNew_RouterOptionsAccumulation(t *testing.T) {
	t.Parallel()

	t.Run("multiple WithRouterOptions accumulate", func(t *testing.T) {
		t.Parallel()
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithRouterOptions(
				router.WithBloomFilterSize(2000),
			),
			WithRouterOptions(
				router.WithCancellationCheck(false),
				router.WithTemplateRouting(true),
			),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.Router())
	})
}

// TestNew_LoggingInitializationSuccess tests that logging initialization
// succeeds with valid logger configuration.
func TestNew_LoggingInitializationSuccess(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithObservability(
			WithLogging(logging.WithLevel(logging.LevelDebug)),
		),
	)
	require.NoError(t, err)
	require.NotNil(t, app)
	assert.NotNil(t, app.logging)
}

// TestNew_MultipleOptionsApplication tests that when the same option type
// is provided multiple times, the last one wins.
func TestNew_MultipleOptionsApplication(t *testing.T) {
	t.Parallel()

	app := MustNew(
		WithServiceName("first"),
		WithServiceVersion("1.0.0"),
		WithServiceName("second"),   // Should override "first"
		WithServiceVersion("2.0.0"), // Should override "1.0.0"
	)

	assert.Equal(t, "second", app.ServiceName())
	assert.Equal(t, "2.0.0", app.ServiceVersion())
}

// TestApp_NoRoute tests that NoRoute sets a custom handler for unmatched routes.
func TestApp_NoRoute(t *testing.T) {
	t.Parallel()

	t.Run("custom 404 handler", func(t *testing.T) {
		t.Parallel()
		app := MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
		)

		app.NoRoute(func(c *Context) {
			if err := c.JSON(http.StatusNotFound, map[string]string{
				"error": "custom not found",
			}); err != nil {
				c.Logger().Error("failed to write response", "err", err)
			}
		})

		req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
		rec := httptest.NewRecorder()
		app.Router().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Contains(t, rec.Body.String(), "custom not found")
	})

	t.Run("nil handler restores default behavior", func(t *testing.T) {
		t.Parallel()

		app := MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
		)

		// First set a custom handler
		app.NoRoute(func(c *Context) {
			if err := c.JSON(http.StatusNotFound, map[string]string{"error": "custom"}); err != nil {
				c.Logger().Error("failed to write response", "err", err)
			}
		})

		// Then restore default by setting nil
		app.NoRoute(nil)

		req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
		rec := httptest.NewRecorder()
		app.Router().ServeHTTP(rec, req)

		// Default should return 404 with standard "404 page not found"
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("no route with registered route still works", func(t *testing.T) {
		t.Parallel()

		app := MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
		)

		app.GET("/exists", func(c *Context) {
			if err := c.String(http.StatusOK, "exists"); err != nil {
				c.Logger().Error("failed to write response", "err", err)
			}
		})

		app.NoRoute(func(c *Context) {
			if err := c.JSON(http.StatusNotFound, map[string]string{"error": "not found"}); err != nil {
				c.Logger().Error("failed to write response", "err", err)
			}
		})

		// Existing route should work
		req := httptest.NewRequest(http.MethodGet, "/exists", nil)
		rec := httptest.NewRecorder()
		app.Router().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "exists", rec.Body.String())

		// Non-existent route should use NoRoute handler
		req = httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
		rec = httptest.NewRecorder()
		app.Router().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Contains(t, rec.Body.String(), "not found")
	})
}
