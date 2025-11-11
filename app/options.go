// Package app provides the main application implementation for Rivaas.
package app

import (
	"time"

	"rivaas.dev/logging"
	"rivaas.dev/metrics"
	"rivaas.dev/router"
	"rivaas.dev/tracing"
)

// Option defines functional options for app configuration.
type Option func(*config)

// WithServiceName sets the service name.
func WithServiceName(name string) Option {
	return func(c *config) {
		c.serviceName = name
	}
}

// WithServiceVersion sets the service version.
func WithServiceVersion(version string) Option {
	return func(c *config) {
		c.serviceVersion = version
	}
}

// WithEnvironment sets the environment (development/production).
// Valid values: "development", "production"
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

// WithLogging enables logging with the given options.
// Service name and version are automatically injected from app-level configuration.
//
// Example:
//
//	app.New(
//	    app.WithServiceName("my-service"),
//	    app.WithServiceVersion("v1.0.0"),
//	    app.WithLogging(logging.WithJSONHandler(), logging.WithLevel("debug")),
//	)
func WithLogging(opts ...logging.Option) Option {
	return func(c *config) {
		c.logging = &loggingConfig{
			enabled: true,
			options: opts,
		}
	}
}

// ServerOption configures server settings.
type ServerOption func(*serverConfig)

// WithReadTimeout sets the server read timeout.
func WithReadTimeout(d time.Duration) ServerOption {
	return func(sc *serverConfig) {
		sc.readTimeout = d
	}
}

// WithWriteTimeout sets the server write timeout.
func WithWriteTimeout(d time.Duration) ServerOption {
	return func(sc *serverConfig) {
		sc.writeTimeout = d
	}
}

// WithIdleTimeout sets the server idle timeout.
func WithIdleTimeout(d time.Duration) ServerOption {
	return func(sc *serverConfig) {
		sc.idleTimeout = d
	}
}

// WithReadHeaderTimeout sets the server read header timeout.
func WithReadHeaderTimeout(d time.Duration) ServerOption {
	return func(sc *serverConfig) {
		sc.readHeaderTimeout = d
	}
}

// WithMaxHeaderBytes sets the maximum size of request headers.
func WithMaxHeaderBytes(n int) ServerOption {
	return func(sc *serverConfig) {
		sc.maxHeaderBytes = n
	}
}

// WithShutdownTimeout sets the graceful shutdown timeout.
func WithShutdownTimeout(d time.Duration) ServerOption {
	return func(sc *serverConfig) {
		sc.shutdownTimeout = d
	}
}

// WithServerConfig configures server settings using functional options.
// Defaults are already set in defaultConfig(), so options are applied in place.
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
func WithMiddleware(middlewares ...router.HandlerFunc) Option {
	return func(c *config) {
		if c.middleware == nil {
			c.middleware = &middlewareConfig{}
		}
		c.middleware.explicitlySet = true
		c.middleware.functions = append(c.middleware.functions, middlewares...)
	}
}

// WithRouterOptions passes router options through to the underlying router.
// This allows fine-tuning router performance settings like Bloom filter sizing,
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
// Multiple calls to WithRouterOptions are supported and will accumulate options.
func WithRouterOptions(opts ...router.Option) Option {
	return func(c *config) {
		if c.router == nil {
			c.router = &routerConfig{}
		}
		c.router.options = append(c.router.options, opts...)
	}
}
