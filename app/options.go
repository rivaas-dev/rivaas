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
	"fmt"
	"time"

	"rivaas.dev/errors"
	"rivaas.dev/openapi"
	"rivaas.dev/router"
)

// Option defines functional options for app configuration.
// Option functions are used to configure an App instance during creation.
type Option func(*config)

// WithServiceName sets the service name used in observability metadata.
// An empty name causes validation to fail during [New].
//
// Example:
//
//	app.New(app.WithServiceName("my-api"))
func WithServiceName(name string) Option {
	return func(c *config) {
		c.serviceName = name
	}
}

// WithServiceVersion sets the service version used in observability metadata.
// An empty version causes validation to fail during [New].
//
// Example:
//
//	app.New(app.WithServiceVersion("v1.0.0"))
func WithServiceVersion(version string) Option {
	return func(c *config) {
		c.serviceVersion = version
	}
}

// WithEnvironment sets the environment mode.
// Valid values are "development" or "production". Invalid values cause
// validation to fail during [New].
//
// Environment affects:
//   - Logging verbosity (production defaults to error-only access logs)
//   - Startup banner (development shows route table)
//   - Terminal colors (production strips ANSI sequences)
//
// Example:
//
//	app.New(app.WithEnvironment("production"))
func WithEnvironment(env string) Option {
	return func(c *config) {
		c.environment = env
	}
}

// ServerOption configures server settings.
type ServerOption func(*serverConfig)

// WithReadTimeout sets the server read timeout.
// WithReadTimeout configures how long the server waits to read the entire request.
//
// Example:
//
//	app.New(
//	    app.WithServerConfig(
//	        app.WithReadTimeout(10 * time.Second),
//	    ),
//	)
func WithReadTimeout(d time.Duration) ServerOption {
	return func(sc *serverConfig) {
		sc.readTimeout = d
	}
}

// WithWriteTimeout sets the server write timeout.
// WithWriteTimeout configures how long the server waits to write the response.
//
// Example:
//
//	app.New(
//	    app.WithServerConfig(
//	        app.WithWriteTimeout(10 * time.Second),
//	    ),
//	)
func WithWriteTimeout(d time.Duration) ServerOption {
	return func(sc *serverConfig) {
		sc.writeTimeout = d
	}
}

// WithIdleTimeout sets the server idle timeout.
// WithIdleTimeout configures how long the server waits for the next request on a keep-alive connection.
//
// Example:
//
//	app.New(
//	    app.WithServerConfig(
//	        app.WithIdleTimeout(60 * time.Second),
//	    ),
//	)
func WithIdleTimeout(d time.Duration) ServerOption {
	return func(sc *serverConfig) {
		sc.idleTimeout = d
	}
}

// WithReadHeaderTimeout sets the server read header timeout.
// WithReadHeaderTimeout configures how long the server waits to read request headers.
//
// Example:
//
//	app.New(
//	    app.WithServerConfig(
//	        app.WithReadHeaderTimeout(2 * time.Second),
//	    ),
//	)
func WithReadHeaderTimeout(d time.Duration) ServerOption {
	return func(sc *serverConfig) {
		sc.readHeaderTimeout = d
	}
}

// WithMaxHeaderBytes sets the maximum size of request headers.
// WithMaxHeaderBytes configures the maximum number of bytes allowed in request headers.
//
// Example:
//
//	app.New(
//	    app.WithServerConfig(
//	        app.WithMaxHeaderBytes(1 << 20), // 1MB
//	    ),
//	)
func WithMaxHeaderBytes(n int) ServerOption {
	return func(sc *serverConfig) {
		sc.maxHeaderBytes = n
	}
}

// WithShutdownTimeout sets the graceful shutdown timeout.
// WithShutdownTimeout configures how long the server waits for graceful shutdown to complete.
//
// Example:
//
//	app.New(
//	    app.WithServerConfig(
//	        app.WithShutdownTimeout(30 * time.Second),
//	    ),
//	)
func WithShutdownTimeout(d time.Duration) ServerOption {
	return func(sc *serverConfig) {
		sc.shutdownTimeout = d
	}
}

// WithServerConfig configures server settings using functional options.
// WithServerConfig applies options to the default server configuration.
//
// Example:
//
//	app.New(
//	    app.WithServerConfig(
//	        app.WithReadTimeout(15 * time.Second),
//	        app.WithWriteTimeout(15 * time.Second),
//	        app.WithShutdownTimeout(30 * time.Second),
//	    ),
//	)
func WithServerConfig(opts ...ServerOption) Option {
	return func(c *config) {
		// Apply options to the existing server config (which already has defaults)
		for _, opt := range opts {
			opt(c.server)
		}
	}
}

// WithMiddleware adds middleware during app initialization.
// Middleware provided here will be added before any middleware added via Use().
// Multiple calls to WithMiddleware are supported and will accumulate.
//
// Note: This does not affect default middleware (recovery). Use WithoutDefaultMiddleware()
// to disable default middleware.
//
// Example:
//
//	app.New(
//	    app.WithServiceName("my-service"),
//	    app.WithMiddleware(
//	        middleware.Logger(),
//	        middleware.Recovery(),
//	    ),
//	)
func WithMiddleware(middlewares ...HandlerFunc) Option {
	return func(c *config) {
		if c.middleware == nil {
			c.middleware = &middlewareConfig{}
		}
		c.middleware.functions = append(c.middleware.functions, middlewares...)
	}
}

// WithoutDefaultMiddleware disables the default middleware (recovery).
// Use this when you want full control over middleware and don't want the framework
// to automatically add recovery middleware.
//
// Example:
//
//	app.New(
//	    app.WithServiceName("my-service"),
//	    app.WithoutDefaultMiddleware(),
//	    app.WithMiddleware(myCustomRecovery), // Add your own
//	)
func WithoutDefaultMiddleware() Option {
	return func(c *config) {
		if c.middleware == nil {
			c.middleware = &middlewareConfig{}
		}
		c.middleware.disableDefaults = true
	}
}

// WithRouterOptions passes router options through to the underlying router.
// WithRouterOptions allows fine-tuning router settings like Bloom filter sizing,
// cancellation checks, template routing, and versioning configuration.
//
// Example:
//
//	app := app.New(
//	    app.WithServiceName("my-service"),
//	    app.WithRouterOptions(
//	        router.WithBloomFilterSize(2000),
//	        router.WithCancellationCheck(false),
//	        router.WithTemplateRouting(true),
//	        router.WithVersioning(),
//	    ),
//	)
//
// Multiple calls to WithRouterOptions accumulate options.
func WithRouterOptions(opts ...router.Option) Option {
	return func(c *config) {
		if c.router == nil {
			c.router = &routerConfig{}
		}
		c.router.options = append(c.router.options, opts...)
	}
}

// openapiConfig holds OpenAPI configuration for the app layer.
// openapiConfig stores OpenAPI settings and initialization state.
type openapiConfig struct {
	enabled bool
	config  *openapi.Config
	initErr error // Stores initialization error to be checked during validation
}

// WithOpenAPI enables OpenAPI specification generation with the given options.
// Service name and version are automatically injected from app-level configuration
// if not explicitly set via openapi.WithTitle(). Option order does not matter.
//
// Example:
//
//	app.New(
//	    app.WithServiceName("my-service"),
//	    app.WithServiceVersion("v1.0.0"),
//	    app.WithOpenAPI(
//	        openapi.WithTitle("My API", "1.0.0"),
//	        openapi.WithDescription("API description"),
//	        openapi.WithBearerAuth("bearerAuth", "JWT authentication"),
//	        openapi.WithServer("http://localhost:8080", "Local development"),
//	        openapi.WithSwaggerUI(true, "/docs"),
//	        openapi.WithUIDocExpansion(openapi.DocExpansionList),
//	        openapi.WithUISyntaxTheme(openapi.SyntaxThemeMonokai),
//	    ),
//	)
func WithOpenAPI(opts ...openapi.Option) Option {
	return func(c *config) {
		// Create OpenAPI config with options
		// Use New instead of MustNew to return errors during validation
		openapiCfg, err := openapi.New(opts...)
		if err != nil {
			// Store error to be checked during config validation.
			// We can't return errors from functional options (they're void functions),
			// so we defer error reporting until config.validate() is called.
			// This allows users to see all configuration errors at once rather than
			// failing on the first error encountered.
			c.openapi = &openapiConfig{
				enabled: true,
				initErr: fmt.Errorf("failed to initialize OpenAPI: %w", err),
			}

			return
		}

		// Note: Service name/version injection happens in New() after all options
		// are applied, so option order doesn't matter.

		c.openapi = &openapiConfig{
			enabled: true,
			config:  openapiCfg,
		}
	}
}

// WithErrorFormatter configures a single error formatter.
// WithErrorFormatter sets the formatter used for all error responses.
//
// Example:
//
//	app.New(
//	    app.WithServiceName("my-service"),
//	    app.WithErrorFormatter(&errors.RFC9457{
//	        BaseURL: "https://api.example.com/problems",
//	    }),
//	)
func WithErrorFormatter(formatter errors.Formatter) Option {
	return func(c *config) {
		if c.errors == nil {
			c.errors = &errorsConfig{}
		}
		c.errors.formatter = formatter
	}
}

// WithErrorFormatters uses the Accept header to determine which formatter is used.
//
// Example:
//
//	app.New(
//	    app.WithServiceName("my-service"),
//	    app.WithErrorFormatters(map[string]errors.Formatter{
//	        "application/problem+json": &errors.RFC9457{
//	            BaseURL: "https://api.example.com/problems",
//	        },
//	        "application/json": &errors.Simple{},
//	    }),
//	    app.WithDefaultErrorFormat("application/problem+json"),
//	)
func WithErrorFormatters(formatters map[string]errors.Formatter) Option {
	return func(c *config) {
		if c.errors == nil {
			c.errors = &errorsConfig{}
		}
		c.errors.formatters = formatters
	}
}

// WithDefaultErrorFormat sets the default format when no Accept header matches.
// WithDefaultErrorFormat is only used when WithErrorFormatters is configured.
//
// Example:
//
//	app.New(
//	    app.WithErrorFormatters(formatters),
//	    app.WithDefaultErrorFormat("application/problem+json"),
//	)
func WithDefaultErrorFormat(mediaType string) Option {
	return func(c *config) {
		if c.errors == nil {
			c.errors = &errorsConfig{}
		}
		c.errors.defaultFormat = mediaType
	}
}

// WithObservability configures all observability components: metrics, tracing, and logging.
// This is the single entry point for configuring the three pillars of observability.
//
// Components:
//   - WithLogging: enables structured logging (service name/version auto-injected)
//   - WithMetrics: enables metrics collection (Prometheus, OTLP)
//   - WithTracing: enables distributed tracing (OTLP, Jaeger)
//
// Shared settings (apply to all components):
//   - WithExcludePaths, WithExcludePrefixes, WithExcludePatterns, WithoutDefaultExclusions
//   - WithAccessLogging, WithLogOnlyErrors, WithSlowThreshold
//
// Default exclusions include common health/probe paths:
// /health, /healthz, /ready, /readyz, /live, /livez, /metrics, /debug/*
//
// Example:
//
//	app.MustNew(
//	    app.WithServiceName("orders-api"),
//	    app.WithServiceVersion("v1.0.0"),
//	    app.WithObservability(
//	        app.WithLogging(logging.WithJSONHandler(), logging.WithDebugLevel()),
//	        app.WithMetrics(), // Prometheus is default; use metrics.WithOTLP() for OTLP
//	        app.WithTracing(tracing.WithOTLP("localhost:4317")),
//	        app.WithExcludePaths("/custom-health"),
//	        app.WithExcludePrefixes("/internal/", "/admin/"),
//	        app.WithLogOnlyErrors(),
//	        app.WithSlowThreshold(500 * time.Millisecond),
//	    ),
//	)
func WithObservability(opts ...ObservabilityOption) Option {
	return func(c *config) {
		if c.observability == nil {
			c.observability = defaultObservabilitySettings()
		}
		for _, opt := range opts {
			opt(c.observability)
		}
	}
}
