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
	"log/slog"
	"time"

	"rivaas.dev/errors"
	"rivaas.dev/metrics"
	"rivaas.dev/openapi"
	"rivaas.dev/router"
	"rivaas.dev/tracing"
)

// Option defines functional options for app configuration.
// Option functions are used to configure an App instance during creation.
type Option func(*config)

// WithServiceName sets the service name.
// WithServiceName configures the service name used in observability metadata.
//
// Example:
//
//	app.New(app.WithServiceName("my-api"))
func WithServiceName(name string) Option {
	return func(c *config) {
		c.serviceName = name
	}
}

// WithServiceVersion sets the service version.
// WithServiceVersion configures the service version used in observability metadata.
//
// Example:
//
//	app.New(app.WithServiceVersion("v1.0.0"))
func WithServiceVersion(version string) Option {
	return func(c *config) {
		c.serviceVersion = version
	}
}

// WithEnvironment sets the environment (development/production).
// WithEnvironment accepts "development" or "production" as valid values.
//
// Example:
//
//	app.New(app.WithEnvironment("production"))
func WithEnvironment(env string) Option {
	return func(c *config) {
		c.environment = env
	}
}

// WithMetrics enables metrics with the given options.
// Service name and version are automatically injected from app-level configuration.
//
// Example:
//
//	app.New(
//	    app.WithServiceName("my-service"),
//	    app.WithServiceVersion("v1.0.0"),
//	    app.WithMetrics(metrics.WithProvider(metrics.PrometheusProvider)),
//	)
func WithMetrics(opts ...metrics.Option) Option {
	return func(c *config) {
		c.metrics = &metricsConfig{
			enabled: true,
			options: opts,
		}
	}
}

// WithTracing enables tracing with the given options.
// Service name and version are automatically injected from app-level configuration.
//
// Example:
//
//	app.New(
//	    app.WithServiceName("my-service"),
//	    app.WithServiceVersion("v1.0.0"),
//	    app.WithTracing(tracing.WithProvider(tracing.OTLPProvider)),
//	)
func WithTracing(opts ...tracing.Option) Option {
	return func(c *config) {
		c.tracing = &tracingConfig{
			enabled: true,
			options: opts,
		}
	}
}

// WithLogger sets the base logger for the application.
// The logger should be a configured *slog.Logger instance.
//
// If not provided, a no-op logger is used (logs are discarded).
//
// The app automatically derives request-scoped loggers that include:
//   - HTTP metadata (method, route, target path, client IP)
//   - Request ID (if X-Request-ID header is present)
//   - Trace/span IDs (if OpenTelemetry tracing is enabled)
//
// Example with rivaas.dev/logging (recommended):
//
//	base := logging.MustNew(
//	    logging.WithJSONHandler(),
//	    logging.WithServiceName("orders-api"),
//	    logging.WithServiceVersion("v1.4.2"),
//	    logging.WithRedaction(logging.DefaultSensitiveKeys...),
//	)
//	app.New(app.WithLogger(base))
//
// Example with plain slog:
//
//	base := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
//	    Level: slog.LevelInfo,
//	}))
//	app.New(app.WithLogger(base))
func WithLogger(logger *slog.Logger) Option {
	return func(c *config) {
		c.baseLogger = logger
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
		c.middleware.explicitlySet = true
		c.middleware.functions = append(c.middleware.functions, middlewares...)
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
// WithOpenAPI automatically injects service name and version from app-level configuration if not provided.
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

		// If title/version not set, use app-level defaults
		if openapiCfg.Info.Title == "API" && c.serviceName != "" {
			openapiCfg.Info.Title = c.serviceName
		}
		if openapiCfg.Info.Version == "1.0.0" && c.serviceVersion != "" {
			openapiCfg.Info.Version = c.serviceVersion
		}

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

// WithErrorLogger logs errors before formatting and returning them to the client.
//
// Example:
//
//	app.New(
//	    app.WithServiceName("my-service"),
//	    app.WithErrorLogger(slog.Default()),
//	)
func WithErrorLogger(logger *slog.Logger) Option {
	return func(c *config) {
		if c.errors == nil {
			c.errors = &errorsConfig{}
		}
		c.errors.logger = logger
	}
}
