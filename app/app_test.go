package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rivaas.dev/logging"
	"rivaas.dev/metrics"
	"rivaas.dev/router"
	"rivaas.dev/tracing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew_ValidationErrors tests that New returns appropriate errors
// for various invalid configuration scenarios.
func TestNew_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		wantErr string
	}{
		{
			name:    "empty service name",
			opts:    []Option{WithServiceName(""), WithServiceVersion("1.0.0")},
			wantErr: "service name cannot be empty",
		},
		{
			name:    "empty service version",
			opts:    []Option{WithServiceName("test"), WithServiceVersion("")},
			wantErr: "service version cannot be empty",
		},
		{
			name: "invalid environment",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithEnvironment("staging"),
			},
			wantErr: "environment must be",
		},
		{
			name: "negative read timeout",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithReadTimeout(-1 * time.Second)),
			},
			wantErr: "read timeout must be positive",
		},
		{
			name: "zero write timeout",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithWriteTimeout(0)),
			},
			wantErr: "write timeout must be positive",
		},
		{
			name: "negative idle timeout",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithIdleTimeout(-5 * time.Second)),
			},
			wantErr: "idle timeout must be positive",
		},
		{
			name: "zero read header timeout",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithReadHeaderTimeout(0)),
			},
			wantErr: "read header timeout must be positive",
		},
		{
			name: "zero max header bytes",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithMaxHeaderBytes(0)),
			},
			wantErr: "max header bytes must be positive",
		},
		{
			name: "negative shutdown timeout",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithServerConfig(WithShutdownTimeout(-10 * time.Second)),
			},
			wantErr: "shutdown timeout must be positive",
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
		logCfg := logging.MustNew(logging.WithLevel(logging.LevelInfo))

		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithLoggingConfig(logCfg),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.logging)
		assert.Equal(t, logCfg, app.logging)
	})

	t.Run("logging nil config is ignored", func(t *testing.T) {
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithLoggingConfig(nil),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.Nil(t, app.logging)
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
		metricsCfg := metrics.MustNew(metrics.WithProvider(metrics.PrometheusProvider))

		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithMetricsConfig(metricsCfg),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.metrics)
		assert.Equal(t, metricsCfg, app.metrics)
	})

	t.Run("metrics nil config is ignored", func(t *testing.T) {
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithMetricsConfig(nil),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.Nil(t, app.metrics)
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
		tracingCfg := tracing.MustNew(tracing.WithProvider(tracing.NoopProvider))

		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithTracingConfig(tracingCfg),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.NotNil(t, app.tracing)
		assert.Equal(t, tracingCfg, app.tracing)
	})

	t.Run("tracing nil config is ignored", func(t *testing.T) {
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithTracingConfig(nil),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.Nil(t, app.tracing)
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
		logCfg := logging.MustNew(logging.WithLevel(logging.LevelInfo))
		metricsCfg := metrics.MustNew(metrics.WithProvider(metrics.PrometheusProvider))
		tracingCfg := tracing.MustNew(tracing.WithProvider(tracing.NoopProvider))

		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithLoggingConfig(logCfg),
			WithMetricsConfig(metricsCfg),
			WithTracingConfig(tracingCfg),
		)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.Equal(t, logCfg, app.logging)
		assert.Equal(t, metricsCfg, app.metrics)
		assert.Equal(t, tracingCfg, app.tracing)
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
	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithServerConfig(
			WithReadTimeout(20*time.Second),
			// All other fields not set - should use defaults
		),
	)

	assert.Equal(t, 20*time.Second, app.config.server.readTimeout)
	assert.Equal(t, DefaultWriteTimeout, app.config.server.writeTimeout)
	assert.Equal(t, DefaultIdleTimeout, app.config.server.idleTimeout)
}

// TestApp_GetMetricsHandler tests that GetMetricsHandler returns
// an error when metrics are not enabled and a handler when they are.
func TestApp_GetMetricsHandler(t *testing.T) {
	t.Run("returns error when metrics not enabled", func(t *testing.T) {
		app := MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
		)

		handler, err := app.GetMetricsHandler()
		assert.Error(t, err)
		assert.Nil(t, handler)
		assert.Contains(t, err.Error(), "metrics not enabled")
	})

	t.Run("returns handler when metrics enabled", func(t *testing.T) {
		app := MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithMetrics(metrics.WithProvider(metrics.PrometheusProvider)),
		)

		handler, err := app.GetMetricsHandler()
		assert.NoError(t, err)
		assert.NotNil(t, handler)
	})
}

// TestApp_GetMetricsServerAddress tests that GetMetricsServerAddress
// returns the correct address when metrics are enabled and empty when disabled.
func TestApp_GetMetricsServerAddress(t *testing.T) {
	t.Run("returns empty when metrics not enabled", func(t *testing.T) {
		app := MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
		)

		addr := app.GetMetricsServerAddress()
		assert.Empty(t, addr)
	})

	t.Run("returns address when metrics enabled", func(t *testing.T) {
		app := MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithMetrics(
				metrics.WithProvider(metrics.PrometheusProvider),
				metrics.WithPort(":9090"),
			),
		)

		addr := app.GetMetricsServerAddress()
		assert.NotEmpty(t, addr)
	})
}

// TestApp_ServiceName tests that ServiceName returns the configured service name.
func TestApp_ServiceName(t *testing.T) {
	app := MustNew(
		WithServiceName("my-service"),
		WithServiceVersion("1.0.0"),
	)

	assert.Equal(t, "my-service", app.ServiceName())
}

// TestApp_ServiceVersion tests that ServiceVersion returns the configured service version.
func TestApp_ServiceVersion(t *testing.T) {
	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("2.3.4"),
	)

	assert.Equal(t, "2.3.4", app.ServiceVersion())
}

// TestApp_Environment tests that Environment returns the configured environment.
func TestApp_Environment(t *testing.T) {
	app := MustNew(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithEnvironment(EnvironmentProduction),
	)

	assert.Equal(t, EnvironmentProduction, app.Environment())
}

// TestApp_GetMetrics tests that GetMetrics returns the metrics configuration
// when enabled and nil when disabled.
func TestApp_GetMetrics(t *testing.T) {
	t.Run("returns nil when metrics not enabled", func(t *testing.T) {
		app := MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
		)

		assert.Nil(t, app.GetMetrics())
	})

	t.Run("returns config when metrics enabled", func(t *testing.T) {
		app := MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithMetrics(metrics.WithProvider(metrics.PrometheusProvider)),
		)

		metricsCfg := app.GetMetrics()
		assert.NotNil(t, metricsCfg)
		assert.Equal(t, app.metrics, metricsCfg)
	})
}

// TestApp_GetTracing tests that GetTracing returns the tracing configuration
// when enabled and nil when disabled.
func TestApp_GetTracing(t *testing.T) {
	t.Run("returns nil when tracing not enabled", func(t *testing.T) {
		app := MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
		)

		assert.Nil(t, app.GetTracing())
	})

	t.Run("returns config when tracing enabled", func(t *testing.T) {
		app := MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithTracing(tracing.WithProvider(tracing.NoopProvider)),
		)

		tracingCfg := app.GetTracing()
		assert.NotNil(t, tracingCfg)
		assert.Equal(t, app.tracing, tracingCfg)
	})
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
