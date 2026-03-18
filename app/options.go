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
	"crypto/tls"
	"fmt"
	"time"

	"rivaas.dev/errors"
	"rivaas.dev/openapi"
	"rivaas.dev/router"
	"rivaas.dev/validation"
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
//   - Access log scope (when unset via [WithAccessLogScope], production defaults to errors-only, development to all)
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

// WithPort sets the server listen port.
// Default is 8080 for HTTP; when using [WithTLS] or [WithMTLS] the default is 8443.
// Override with WithPort(n) in all cases. Can be overridden by RIVAAS_PORT when [WithEnv] is used.
//
// Example:
//
//	app.New(app.WithPort(3000))
func WithPort(port int) Option {
	return func(c *config) {
		c.server.port = port
	}
}

// WithHost sets the host/interface to bind the HTTP server to.
// Default is "" (all interfaces, equivalent to "0.0.0.0").
// Use "127.0.0.1" or "localhost" to restrict to local connections only.
//
// Example:
//
//	// Bind to all interfaces (default)
//	app.New(app.WithPort(8080))
//
//	// Bind to localhost only (e.g., behind reverse proxy)
//	app.New(
//	    app.WithHost("127.0.0.1"),
//	    app.WithPort(8080),
//	)
func WithHost(host string) Option {
	return func(c *config) {
		c.server.host = host
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
//	    app.WithServer(
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
//	    app.WithServer(
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
//	    app.WithServer(
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
//	    app.WithServer(
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
//	    app.WithServer(
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
//	    app.WithServer(
//	        app.WithShutdownTimeout(30 * time.Second),
//	    ),
//	)
func WithShutdownTimeout(d time.Duration) ServerOption {
	return func(sc *serverConfig) {
		sc.shutdownTimeout = d
	}
}

// WithServer configures server settings using functional options.
//
// Example:
//
//	app.New(
//	    app.WithServer(
//	        app.WithReadTimeout(15 * time.Second),
//	        app.WithWriteTimeout(15 * time.Second),
//	        app.WithShutdownTimeout(30 * time.Second),
//	    ),
//	)
func WithServer(opts ...ServerOption) Option {
	return func(c *config) {
		// Apply options to the existing server config (which already has defaults)
		for i, opt := range opts {
			if opt == nil {
				c.validationErrors = append(c.validationErrors, fmt.Errorf("app: server option at index %d cannot be nil", i))
				continue
			}
			opt(c.server)
		}
	}
}

// WithTLS configures the server to serve HTTPS using the given certificate and key files.
// Only one of WithTLS or WithMTLS may be used. Both certFile and keyFile must be non-empty.
// Default listen port is 8443 unless overridden by [WithPort] or RIVAAS_PORT when [WithEnv] is used.
//
// Example:
//
//	app.New(
//	    app.WithServiceName("my-api"),
//	    app.WithTLS("server.crt", "server.key"), // default port 8443; use WithPort(443) to override
//	)
//	// ...
//	app.Start(ctx)
func WithTLS(certFile, keyFile string) Option {
	return func(c *config) {
		c.server.tlsCertFile = certFile
		c.server.tlsKeyFile = keyFile
		if c.server.port == DefaultPort {
			c.server.port = DefaultTLSPort
		}
	}
}

// WithMTLS configures the server to serve HTTPS with mutual TLS (mTLS) using the given
// server certificate and options. Only one of WithTLS or WithMTLS may be used.
// Default listen port is 8443 unless overridden by [WithPort] or RIVAAS_PORT when [WithEnv] is used.
//
// Example:
//
//	serverCert, _ := tls.LoadX509KeyPair("server.crt", "server.key")
//	app.New(
//	    app.WithServiceName("my-api"),
//	    app.WithMTLS(serverCert,
//	        app.WithClientCAs(caCertPool),
//	        app.WithMinVersion(tls.VersionTLS13),
//	    ), // default port 8443; use WithPort(443) to override
//	)
//	// ...
//	app.Start(ctx)
func WithMTLS(serverCert tls.Certificate, opts ...MTLSOption) Option {
	return func(c *config) {
		c.server.mtlsServerCert = serverCert
		c.server.mtlsOpts = opts
		if c.server.port == DefaultPort {
			c.server.port = DefaultTLSPort
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

// WithRouter passes router options through to the underlying router.
//
// Example:
//
//	app := app.New(
//	    app.WithServiceName("my-service"),
//	    app.WithRouter(
//	        router.WithBloomFilterSize(2000),
//	        router.WithoutCancellationCheck(),
//	        router.WithTemplateRouting(true),
//	        router.WithVersioning(),
//	    ),
//	)
//
// Multiple calls to WithRouter accumulate options.
func WithRouter(opts ...router.Option) Option {
	return func(c *config) {
		if c.router == nil {
			c.router = &routerConfig{}
		}
		c.router.options = append(c.router.options, opts...)
	}
}

// WithValidationEngine sets the validation engine used by [Context.Bind] and [Context.Validate].
// When set, the app uses this engine instead of the package-level [validation.DefaultEngine].
// Use this for custom validation configuration (e.g. redaction, MaxErrors) or test isolation.
//
// Example:
//
//	engine := validation.MustNew(validation.WithRedactor(myRedactor))
//	app := app.MustNew(
//	    app.WithServiceName("my-api"),
//	    app.WithValidationEngine(engine),
//	)
func WithValidationEngine(engine *validation.Engine) Option {
	return func(c *config) {
		c.validationEngine = engine
	}
}

// openapiConfig holds OpenAPI configuration for the app layer.
// openapiConfig stores OpenAPI settings and initialization state.
// options is set by WithOpenAPI and consumed in config.validate() to build config.
type openapiConfig struct {
	enabled bool
	options []openapi.Option // raw options until finalization in validate()
	config  *openapi.API
	initErr error // Stores initialization error to be checked during validation
}

// WithOpenAPI enables OpenAPI specification generation with the given options.
// Service name and version are automatically injected from app-level configuration
// after all options are applied (in config validation), so option order does not matter.
// If not explicitly set via openapi.WithTitle(), the app's service name and version are used.
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
		c.openapi = &openapiConfig{
			enabled: true,
			options: opts,
		}
	}
}

// WithErrorFormatterFor configures an error formatter from options.
// The app builds the formatter via errors.New(opts...); invalid options are reported during config validation.
//
// Use empty mediaType ("") for a single formatter for all responses (no content negotiation).
// Use a non-empty mediaType (e.g. "application/problem+json") to register a formatter for content negotiation;
// multiple calls accumulate. Cannot mix: use either a single formatter ("") or content-negotiated formatters, not both.
//
// Example:
//
//	// Single formatter for all responses
//	app.New(
//	    app.WithServiceName("my-service"),
//	    app.WithErrorFormatterFor("", errors.WithRFC9457("https://api.example.com/problems")),
//	)
//
//	// Content negotiation by Accept header
//	app.New(
//	    app.WithServiceName("my-service"),
//	    app.WithErrorFormatterFor("application/problem+json", errors.WithRFC9457("https://api.example.com/problems")),
//	    app.WithErrorFormatterFor("application/json", errors.WithSimple()),
//	    app.WithDefaultErrorFormat("application/problem+json"),
//	)
func WithErrorFormatterFor(mediaType string, opts ...errors.Option) Option {
	return func(c *config) {
		if c.errors == nil {
			c.errors = &errorsConfig{}
		}
		formatter, err := errors.New(opts...)
		if err != nil {
			c.errors.initErr = err
			return
		}
		c.errors.initErr = nil
		if mediaType == "" {
			if len(c.errors.formatters) > 0 {
				c.errors.modeErr = fmt.Errorf("cannot use single error formatter when content-negotiated formatters are configured")
				return
			}
			c.errors.formatter = formatter
			c.errors.formatters = nil
			c.errors.singleFormatterExplicitlySet = true
			return
		}
		if c.errors.singleFormatterExplicitlySet {
			c.errors.modeErr = fmt.Errorf("cannot use content-negotiated formatters when single error formatter is configured")
			return
		}
		// Switch to content-negotiated mode: clear single formatter and add to map.
		if c.errors.formatters == nil {
			c.errors.formatters = make(map[string]errors.Formatter)
		}
		c.errors.formatters[mediaType] = formatter
		c.errors.formatter = nil
	}
}

// WithErrorFormatters configures multiple error formatters with content negotiation by Accept header.
// Advanced: use when you need to pass pre-built or custom formatters. Prefer [WithErrorFormatterFor]
// for option-based configuration.
//
// Example:
//
//	app.New(
//	    app.WithServiceName("my-service"),
//	    app.WithErrorFormatters(map[string]errors.Formatter{
//	        "application/problem+json": errors.MustNew(errors.WithRFC9457("https://api.example.com/problems")),
//	        "application/json":        errors.MustNew(errors.WithSimple()),
//	    }),
//	    app.WithDefaultErrorFormat("application/problem+json"),
//	)
func WithErrorFormatters(formatters map[string]errors.Formatter) Option {
	return func(c *config) {
		if c.errors == nil {
			c.errors = &errorsConfig{}
		}
		c.errors.formatters = formatters
		c.errors.formatter = nil
		c.errors.singleFormatterExplicitlySet = false
	}
}

// WithDefaultErrorFormat sets the default format when no Accept header matches.
// Only used when content-negotiated formatters are configured (via [WithErrorFormatterFor] with non-empty media types or [WithErrorFormatters]).
//
// Example:
//
//	app.New(
//	    app.WithErrorFormatterFor("application/problem+json", errors.WithRFC9457("...")),
//	    app.WithErrorFormatterFor("application/json", errors.WithSimple()),
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
//   - WithAccessLogging, WithAccessLogScope, WithSlowThreshold
//
// Default exclusions include common health/probe paths:
// /health, /livez, /ready, /readyz, /live, /metrics, /debug/*
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
//	        app.WithAccessLogScope(app.AccessLogScopeErrorsOnly),
//	        app.WithSlowThreshold(500 * time.Millisecond),
//	    ),
//	)
func WithObservability(opts ...ObservabilityOption) Option {
	return func(c *config) {
		if c.observability == nil {
			c.observability = defaultObservabilitySettings()
		}
		for i, opt := range opts {
			if opt == nil {
				c.observability.validationErrors = append(c.observability.validationErrors, fmt.Errorf("app: observability option at index %d cannot be nil", i))
				continue
			}
			opt(c.observability)
		}
	}
}
