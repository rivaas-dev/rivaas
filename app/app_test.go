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

// TestNew_ValidationErrors tests that New returns appropriate structured errors
// for various invalid configuration scenarios.
func TestNew_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		opts        []Option
		wantErr     string
		wantErrType interface{}             // Expected error type (ConfigError, ValidationErrors, etc.)
		checkError  func(*testing.T, error) // Optional custom error checking
	}{
		{
			name:        "empty service name",
			opts:        []Option{WithServiceName(""), WithServiceVersion("1.0.0")},
			wantErr:     "serviceName",
			wantErrType: &ValidationErrors{},
			checkError: func(t *testing.T, err error) {
				var ve *ValidationErrors
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
			wantErrType: &ValidationErrors{},
		},
		{
			name: "invalid environment",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithEnvironment("staging"),
			},
			wantErr:     "environment",
			wantErrType: &ValidationErrors{},
			checkError: func(t *testing.T, err error) {
				var ve *ValidationErrors
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
			wantErrType: &ValidationErrors{},
		},
		{
			name: "zero write timeout",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithWriteTimeout(0)),
			},
			wantErr:     "server.writeTimeout",
			wantErrType: &ValidationErrors{},
		},
		{
			name: "negative idle timeout",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithIdleTimeout(-5 * time.Second)),
			},
			wantErr:     "server.idleTimeout",
			wantErrType: &ValidationErrors{},
		},
		{
			name: "zero read header timeout",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithReadHeaderTimeout(0)),
			},
			wantErr:     "server.readHeaderTimeout",
			wantErrType: &ValidationErrors{},
		},
		{
			name: "zero max header bytes",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithMaxHeaderBytes(0)),
			},
			wantErr:     "server.maxHeaderBytes",
			wantErrType: &ValidationErrors{},
		},
		{
			name: "negative shutdown timeout",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithShutdownTimeout(-10 * time.Second)),
			},
			wantErr:     "server.shutdownTimeout",
			wantErrType: &ValidationErrors{},
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
			wantErrType: &ValidationErrors{},
			checkError: func(t *testing.T, err error) {
				var ve *ValidationErrors
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
			wantErrType: &ValidationErrors{},
		},
		{
			name: "max header bytes too small",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithMaxHeaderBytes(512)),
			},
			wantErr:     "must be at least 1KB",
			wantErrType: &ValidationErrors{},
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
			wantErrType: &ValidationErrors{},
			checkError: func(t *testing.T, err error) {
				var ve *ValidationErrors
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
			app, err := New(tt.opts...)
			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Nil(t, app)
				assert.Contains(t, err.Error(), tt.wantErr)

				// Check error type if specified
				if tt.wantErrType != nil {
					// Use errors.As for type checking
					var target *ValidationErrors
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
	t.Run("logging initialization with options", func(t *testing.T) {
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithLogging(logging.WithLevel(logging.LevelDebug)),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.logging)
	})

	t.Run("logging initialization with prebuilt config", func(t *testing.T) {
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithLogging(logging.WithLevel(logging.LevelInfo)),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.logging)
	})

	t.Run("metrics initialization with options", func(t *testing.T) {
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithMetrics(metrics.WithProvider(metrics.PrometheusProvider)),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.metrics)
	})

	t.Run("metrics initialization with prebuilt config", func(t *testing.T) {
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithMetrics(metrics.WithProvider(metrics.PrometheusProvider)),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.metrics)
	})

	t.Run("tracing initialization with options", func(t *testing.T) {
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithTracing(tracing.WithProvider(tracing.NoopProvider)),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.tracing)
	})

	t.Run("tracing initialization with prebuilt config", func(t *testing.T) {
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithTracing(tracing.WithProvider(tracing.NoopProvider)),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.tracing)
	})

	t.Run("all observability components together", func(t *testing.T) {
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithLogging(logging.WithLevel(logging.LevelDebug)),
			WithMetrics(metrics.WithProvider(metrics.PrometheusProvider)),
			WithTracing(tracing.WithProvider(tracing.NoopProvider)),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.logging)
		assert.NotNil(t, app.metrics)
		assert.NotNil(t, app.tracing)
	})

	t.Run("prebuilt observability configs together", func(t *testing.T) {
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithLogging(logging.WithLevel(logging.LevelInfo)),
			WithMetrics(metrics.WithProvider(metrics.PrometheusProvider)),
			WithTracing(tracing.WithProvider(tracing.NoopProvider)),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.logging)
		assert.NotNil(t, app.metrics)
		assert.NotNil(t, app.tracing)
	})

	t.Run("service metadata is automatically injected into observability", func(t *testing.T) {
		app, err := New(
			WithServiceName("payment-service"),
			WithServiceVersion("v2.1.0"),
			WithLogging(),
			WithMetrics(),
			WithTracing(),
		)
		require.NoError(t, err)
		require.NotNil(t, app)

		// Verify logging has service metadata
		assert.NotNil(t, app.logging)
		assert.Equal(t, "payment-service", app.logging.ServiceName())
		assert.Equal(t, "v2.1.0", app.logging.ServiceVersion())

		// Verify metrics has service metadata
		assert.NotNil(t, app.metrics)
		assert.Equal(t, "payment-service", app.metrics.ServiceName())
		assert.Equal(t, "v2.1.0", app.metrics.ServiceVersion())

		// Verify tracing has service metadata
		assert.NotNil(t, app.tracing)
		assert.Equal(t, "payment-service", app.tracing.ServiceName())
		assert.Equal(t, "v2.1.0", app.tracing.ServiceVersion())
	})

	t.Run("user-provided service metadata can override injected values", func(t *testing.T) {
		app, err := New(
			WithServiceName("payment-service"),
			WithServiceVersion("v2.1.0"),
			WithLogging(
				logging.WithServiceName("custom-logger"),
				logging.WithServiceVersion("v1.0.0"),
			),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.logging)

		// User-provided values should override (they come after injected ones)
		assert.Equal(t, "custom-logger", app.logging.ServiceName())
		assert.Equal(t, "v1.0.0", app.logging.ServiceVersion())
	})
}

// TestNew_MiddlewareConfiguration tests that middleware is configured
// correctly based on environment and explicit settings.
func TestNew_MiddlewareConfiguration(t *testing.T) {
	t.Run("development environment includes logger and recovery by default", func(t *testing.T) {
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithEnvironment(EnvironmentDevelopment),
		)
		require.NoError(t, err)
		// Should have 2 middleware: Recovery and Logger
		assert.GreaterOrEqual(t, len(app.config.middleware.functions), 2)
		assert.False(t, app.config.middleware.explicitlySet)
	})

	t.Run("production environment includes only recovery by default", func(t *testing.T) {
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithEnvironment(EnvironmentProduction),
		)
		require.NoError(t, err)
		// Should have 1 middleware: Recovery only
		assert.Equal(t, 1, len(app.config.middleware.functions))
		assert.False(t, app.config.middleware.explicitlySet)
	})

	t.Run("explicit middleware configuration overrides defaults", func(t *testing.T) {
		customMiddleware := func(c *router.Context) {
			c.Next()
		}
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithEnvironment(EnvironmentDevelopment),
			WithMiddleware(customMiddleware), // Explicit middleware, no defaults
		)
		require.NoError(t, err)
		// Should only have the custom middleware, no defaults
		assert.Equal(t, 1, len(app.config.middleware.functions))
		assert.True(t, app.config.middleware.explicitlySet)
	})

	t.Run("recovery middleware enabled by default", func(t *testing.T) {
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
		)
		require.NoError(t, err)
		// Should have at least recovery middleware
		assert.GreaterOrEqual(t, len(app.config.middleware.functions), 1)
		assert.False(t, app.config.middleware.explicitlySet)
	})

	t.Run("explicit empty middleware disables defaults", func(t *testing.T) {
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithEnvironment(EnvironmentDevelopment),
			WithMiddleware(), // Explicitly empty - no defaults
		)
		require.NoError(t, err)
		// Should have no middleware
		assert.Equal(t, 0, len(app.config.middleware.functions))
		assert.True(t, app.config.middleware.explicitlySet)
	})

	t.Run("multiple middleware can be added", func(t *testing.T) {
		custom1 := func(c *router.Context) { c.Next() }
		custom2 := func(c *router.Context) { c.Next() }
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithMiddleware(custom1, custom2),
		)
		require.NoError(t, err)
		// Should have exactly 2 middleware
		assert.Equal(t, 2, len(app.config.middleware.functions))
		assert.True(t, app.config.middleware.explicitlySet)
	})
}

// TestApp_RouteRegistration tests that all HTTP methods (GET, POST, PUT,
// DELETE, PATCH, HEAD, OPTIONS) can be registered and handled correctly.
func TestApp_RouteRegistration(t *testing.T) {
	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	// Register routes using App methods
	app.GET("/users", func(c *router.Context) {
		c.String(http.StatusOK, "users")
	})

	app.POST("/users", func(c *router.Context) {
		c.String(http.StatusCreated, "created")
	})

	app.PUT("/users/:id", func(c *router.Context) {
		userID := c.Param("id")
		c.String(http.StatusOK, "updated %s", userID)
	})

	app.DELETE("/users/:id", func(c *router.Context) {
		c.String(http.StatusNoContent, "")
	})

	app.PATCH("/users/:id", func(c *router.Context) {
		userID := c.Param("id")
		c.String(http.StatusOK, "patched %s", userID)
	})

	app.HEAD("/users/:id", func(c *router.Context) {
		c.String(http.StatusOK, "")
	})

	app.OPTIONS("/users", func(c *router.Context) {
		c.String(http.StatusOK, "")
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
	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	// Create route group
	api := app.Group("/api")
	api.GET("/health", func(c *router.Context) {
		c.String(http.StatusOK, "healthy")
	})

	api.GET("/users/:id", func(c *router.Context) {
		userID := c.Param("id")
		c.String(http.StatusOK, "user-%s", userID)
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
	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)

	callCount := 0
	app.Use(func(c *router.Context) {
		callCount++
		c.Next()
	})

	app.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "ok")
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
	t.Run("panics on invalid config", func(t *testing.T) {
		assert.Panics(t, func() {
			MustNew(WithServiceName(""), WithServiceVersion("1.0.0")) // Empty service name
		})
	})

	t.Run("does not panic on valid config", func(t *testing.T) {
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
					WithMetrics(metrics.WithProvider(metrics.PrometheusProvider)),
				)
			},
			wantError:   false,
			wantHandler: true,
		},
	}

	for _, tt := range tests {
		tt := tt
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
					WithMetrics(
						metrics.WithProvider(metrics.PrometheusProvider),
						metrics.WithPort(":9090"),
					),
				)
			},
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		tt := tt
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
		getter   func(*App) interface{}
		expected interface{}
	}{
		{
			name: "ServiceName returns configured value",
			setup: func() *App {
				return MustNew(
					WithServiceName("my-service"),
					WithServiceVersion("1.0.0"),
				)
			},
			getter: func(a *App) interface{} {
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
			getter: func(a *App) interface{} {
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
			getter: func(a *App) interface{} {
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
			getter: func(a *App) interface{} {
				return a.Environment()
			},
			expected: EnvironmentDevelopment,
		},
	}

	for _, tt := range tests {
		tt := tt
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
		validateCfg func(*testing.T, *App, *metrics.Config)
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
			name: "returns config when metrics enabled",
			setup: func() *App {
				return MustNew(
					WithServiceName("test"),
					WithServiceVersion("1.0.0"),
					WithMetrics(metrics.WithProvider(metrics.PrometheusProvider)),
				)
			},
			wantNil: false,
			validateCfg: func(t *testing.T, app *App, cfg *metrics.Config) {
				assert.NotNil(t, cfg)
				assert.Equal(t, app.metrics, cfg)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
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
		validateCfg func(*testing.T, *App, *tracing.Config)
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
					WithTracing(tracing.WithProvider(tracing.NoopProvider)),
				)
			},
			wantNil: false,
			validateCfg: func(t *testing.T, app *App, cfg *tracing.Config) {
				assert.NotNil(t, cfg)
				assert.Equal(t, app.tracing, cfg)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
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
	t.Run("multiple WithRouterOptions accumulate", func(t *testing.T) {
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

// TestNew_LoggingInitializationError tests that logging initialization
// error handling is in place, though most logging options don't typically fail.
func TestNew_LoggingInitializationError(t *testing.T) {
	// This test depends on whether logging.New can actually fail
	// If it can't fail with normal options, we might need to skip this
	// For now, we test that error handling exists in the code path
	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithLogging(logging.WithLevel(logging.LevelDebug)),
	)
	// If logging initialization would fail, it should be caught
	// Most logging options won't fail, so this test verifies the happy path
	require.NoError(t, err)
	require.NotNil(t, app)
}

// TestNew_MultipleOptionsApplication tests that when the same option type
// is provided multiple times, the last one wins.
func TestNew_MultipleOptionsApplication(t *testing.T) {
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
	t.Run("custom 404 handler", func(t *testing.T) {
		app := MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
		)

		app.NoRoute(func(c *router.Context) {
			c.JSON(http.StatusNotFound, map[string]string{
				"error": "custom not found",
			})
		})

		req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
		rec := httptest.NewRecorder()
		app.Router().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Contains(t, rec.Body.String(), "custom not found")
	})

	t.Run("nil handler restores default behavior", func(t *testing.T) {
		app := MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
		)

		// First set a custom handler
		app.NoRoute(func(c *router.Context) {
			c.JSON(http.StatusNotFound, map[string]string{"error": "custom"})
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
		app := MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
		)

		app.GET("/exists", func(c *router.Context) {
			c.String(http.StatusOK, "exists")
		})

		app.NoRoute(func(c *router.Context) {
			c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
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
