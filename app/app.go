package app

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"rivaas.dev/logging"
	"rivaas.dev/metrics"
	"rivaas.dev/router"
	"rivaas.dev/router/middleware"
	"rivaas.dev/tracing"
)

// Default configuration values.
const (
	DefaultServiceName       = "rivaas-app"
	DefaultVersion           = "1.0.0"
	DefaultEnvironment       = "development"
	DefaultReadTimeout       = 10 * time.Second
	DefaultWriteTimeout      = 10 * time.Second
	DefaultIdleTimeout       = 60 * time.Second
	DefaultReadHeaderTimeout = 2 * time.Second
	DefaultMaxHeaderBytes    = 1 << 20 // 1MB
	DefaultShutdownTimeout   = 30 * time.Second

	// Environment constants
	EnvironmentDevelopment = "development"
	EnvironmentProduction  = "production"
)

// App represents the high-level application framework.
// It wraps the router with integrated observability and common middleware.
type App struct {
	router  *router.Router
	metrics *metrics.Config
	tracing *tracing.Config
	logging *logging.Config
	config  *config
}

// config holds the internal application configuration.
// All fields are private to maintain encapsulation.
type config struct {
	serviceName    string
	serviceVersion string
	environment    string
	metrics        *metricsConfig
	tracing        *tracingConfig
	logging        *loggingConfig
	server         *serverConfig
	middleware     *middlewareConfig
	router         *routerConfig
}

// metricsConfig holds metrics configuration.
type metricsConfig struct {
	enabled       bool
	options       []metrics.Option
	config        *metrics.Config // Pre-initialized config
	usePrebuilt   bool            // Whether to use prebuilt config
	needsMetadata bool            // Whether metadata should be injected at assembly time
}

// tracingConfig holds tracing configuration.
type tracingConfig struct {
	enabled       bool
	options       []tracing.Option
	config        *tracing.Config // Pre-initialized config
	usePrebuilt   bool            // Whether to use prebuilt config
	needsMetadata bool            // Whether metadata should be injected at assembly time
}

// loggingConfig holds logging configuration.
type loggingConfig struct {
	enabled       bool
	options       []logging.Option
	config        *logging.Config // Pre-initialized config
	usePrebuilt   bool            // Whether to use prebuilt config
	needsMetadata bool            // Whether metadata should be injected at assembly time
}

// serverConfig holds server configuration.
type serverConfig struct {
	readTimeout       time.Duration
	writeTimeout      time.Duration
	idleTimeout       time.Duration
	readHeaderTimeout time.Duration
	maxHeaderBytes    int
	shutdownTimeout   time.Duration
}

// middlewareConfig holds middleware configuration.
type middlewareConfig struct {
	includeLogger   bool
	includeRecovery bool
}

// routerConfig holds router configuration options.
type routerConfig struct {
	options []router.Option
}

// ServerConfig is the public server configuration struct used by functional options.
type ServerConfig struct {
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	MaxHeaderBytes    int
	ShutdownTimeout   time.Duration
}

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
func WithMetrics(opts ...metrics.Option) Option {
	return func(c *config) {
		c.metrics = &metricsConfig{
			enabled: true,
			options: opts,
		}
	}
}

// WithTracing enables tracing with the given options.
func WithTracing(opts ...tracing.Option) Option {
	return func(c *config) {
		c.tracing = &tracingConfig{
			enabled: true,
			options: opts,
		}
	}
}

// WithLogging enables logging with the given options.
func WithLogging(opts ...logging.Option) Option {
	return func(c *config) {
		c.logging = &loggingConfig{
			enabled: true,
			options: opts,
		}
	}
}

// WithObservability enables metrics, tracing, and logging with default options.
// Metadata (service name, version, environment) is injected at assembly time,
// making option order irrelevant.
func WithObservability() Option {
	return func(c *config) {
		c.metrics = &metricsConfig{
			enabled:       true,
			options:       []metrics.Option{},
			needsMetadata: true,
		}
		c.tracing = &tracingConfig{
			enabled:       true,
			options:       []tracing.Option{},
			needsMetadata: true,
		}
		c.logging = &loggingConfig{
			enabled: true,
			options: []logging.Option{
				logging.WithJSONHandler(),
			},
			needsMetadata: true,
		}
	}
}

// WithServerConfig sets server configuration.
// Any fields not set (zero values) will use the default values.
func WithServerConfig(serverCfg *ServerConfig) Option {
	return func(c *config) {
		if serverCfg == nil {
			return
		}

		// Use provided values, or fall back to defaults for zero values
		readTimeout := serverCfg.ReadTimeout
		if readTimeout == 0 {
			readTimeout = c.server.readTimeout
		}

		writeTimeout := serverCfg.WriteTimeout
		if writeTimeout == 0 {
			writeTimeout = c.server.writeTimeout
		}

		idleTimeout := serverCfg.IdleTimeout
		if idleTimeout == 0 {
			idleTimeout = c.server.idleTimeout
		}

		readHeaderTimeout := serverCfg.ReadHeaderTimeout
		if readHeaderTimeout == 0 {
			readHeaderTimeout = c.server.readHeaderTimeout
		}

		maxHeaderBytes := serverCfg.MaxHeaderBytes
		if maxHeaderBytes == 0 {
			maxHeaderBytes = c.server.maxHeaderBytes
		}

		shutdownTimeout := serverCfg.ShutdownTimeout
		if shutdownTimeout == 0 {
			shutdownTimeout = c.server.shutdownTimeout
		}

		c.server = &serverConfig{
			readTimeout:       readTimeout,
			writeTimeout:      writeTimeout,
			idleTimeout:       idleTimeout,
			readHeaderTimeout: readHeaderTimeout,
			maxHeaderBytes:    maxHeaderBytes,
			shutdownTimeout:   shutdownTimeout,
		}
	}
}

// WithMiddleware configures which default middleware to include.
// By default, development mode includes logger, production includes neither.
// Both include recovery middleware unless explicitly disabled.
func WithMiddleware(includeLogger, includeRecovery bool) Option {
	return func(c *config) {
		c.middleware.includeLogger = includeLogger
		c.middleware.includeRecovery = includeRecovery
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

// WithLoggingConfig uses a pre-initialized logging configuration instead of
// creating a new one. This allows you to manage the logger lifecycle yourself
// and avoid global state registration conflicts.
//
// Example:
//
//	logger := logging.MustNew(logging.WithJSONHandler())
//	app := app.New(
//	    app.WithLoggingConfig(logger),
//	)
func WithLoggingConfig(logCfg *logging.Config) Option {
	return func(c *config) {
		if logCfg == nil {
			return
		}
		c.logging = &loggingConfig{
			enabled:     true,
			config:      logCfg,
			usePrebuilt: true,
		}
	}
}

// WithMetricsConfig uses a pre-initialized metrics configuration instead of
// creating a new one. This allows you to manage the metrics lifecycle yourself
// and avoid global state registration conflicts.
//
// Example:
//
//	metricsConfig := metrics.MustNew(metrics.WithProvider(metrics.PrometheusProvider))
//	app := app.New(
//	    app.WithMetricsConfig(metricsConfig),
//	)
func WithMetricsConfig(metricsCfg *metrics.Config) Option {
	return func(c *config) {
		if metricsCfg == nil {
			return
		}
		c.metrics = &metricsConfig{
			enabled:     true,
			config:      metricsCfg,
			usePrebuilt: true,
		}
	}
}

// WithTracingConfig uses a pre-initialized tracing configuration instead of
// creating a new one. This allows you to manage the tracing lifecycle yourself
// and avoid global state registration conflicts.
//
// Example:
//
//	tracingConfig := tracing.MustNew(tracing.WithProvider(tracing.OTLPProvider))
//	app := app.New(
//	    app.WithTracingConfig(tracingConfig),
//	)
func WithTracingConfig(tracingCfg *tracing.Config) Option {
	return func(c *config) {
		if tracingCfg == nil {
			return
		}
		c.tracing = &tracingConfig{
			enabled:     true,
			config:      tracingCfg,
			usePrebuilt: true,
		}
	}
}

// validate checks if the configuration is valid.
func (c *config) validate() error {
	if c.serviceName == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	if c.serviceVersion == "" {
		return fmt.Errorf("service version cannot be empty")
	}

	if c.environment != EnvironmentDevelopment && c.environment != EnvironmentProduction {
		return fmt.Errorf("environment must be '%s' or '%s', got: %s",
			EnvironmentDevelopment, EnvironmentProduction, c.environment)
	}

	if c.server.readTimeout <= 0 {
		return fmt.Errorf("read timeout must be positive, got: %s", c.server.readTimeout)
	}

	if c.server.writeTimeout <= 0 {
		return fmt.Errorf("write timeout must be positive, got: %s", c.server.writeTimeout)
	}

	if c.server.idleTimeout <= 0 {
		return fmt.Errorf("idle timeout must be positive, got: %s", c.server.idleTimeout)
	}

	if c.server.readHeaderTimeout <= 0 {
		return fmt.Errorf("read header timeout must be positive, got: %s", c.server.readHeaderTimeout)
	}

	if c.server.maxHeaderBytes <= 0 {
		return fmt.Errorf("max header bytes must be positive, got: %d", c.server.maxHeaderBytes)
	}

	if c.server.shutdownTimeout <= 0 {
		return fmt.Errorf("shutdown timeout must be positive, got: %s", c.server.shutdownTimeout)
	}

	return nil
}

// injectObservabilityMetadata injects service metadata into observability configs
// that were marked as needing metadata injection. This allows option order to be
// irrelevant when using WithObservability().
func injectObservabilityMetadata(cfg *config) {
	if cfg.metrics != nil && cfg.metrics.enabled && cfg.metrics.needsMetadata && !cfg.metrics.usePrebuilt {
		cfg.metrics.options = append(cfg.metrics.options,
			metrics.WithServiceName(cfg.serviceName),
			metrics.WithServiceVersion(cfg.serviceVersion),
		)
	}

	if cfg.tracing != nil && cfg.tracing.enabled && cfg.tracing.needsMetadata && !cfg.tracing.usePrebuilt {
		cfg.tracing.options = append(cfg.tracing.options,
			tracing.WithServiceName(cfg.serviceName),
			tracing.WithServiceVersion(cfg.serviceVersion),
		)
	}

	if cfg.logging != nil && cfg.logging.enabled && cfg.logging.needsMetadata && !cfg.logging.usePrebuilt {
		cfg.logging.options = append(cfg.logging.options,
			logging.WithServiceInfo(cfg.serviceName, cfg.serviceVersion, cfg.environment),
		)
	}
}

// defaultConfig returns a configuration with sensible defaults.
func defaultConfig() *config {
	return &config{
		serviceName:    DefaultServiceName,
		serviceVersion: DefaultVersion,
		environment:    DefaultEnvironment,
		server: &serverConfig{
			readTimeout:       DefaultReadTimeout,
			writeTimeout:      DefaultWriteTimeout,
			idleTimeout:       DefaultIdleTimeout,
			readHeaderTimeout: DefaultReadHeaderTimeout,
			maxHeaderBytes:    DefaultMaxHeaderBytes,
			shutdownTimeout:   DefaultShutdownTimeout,
		},
		middleware: &middlewareConfig{
			includeLogger:   false, // Set based on environment in New()
			includeRecovery: true,
		},
	}
}

// New creates a new App instance with the given options.
// Returns an error if the configuration is invalid or initialization fails.
func New(opts ...Option) (*App, error) {
	// Start with default configuration
	cfg := defaultConfig()

	// Apply user options
	for _, opt := range opts {
		opt(cfg)
	}

	// Inject metadata into observability configs that need it
	// This must happen after all options are applied so option order doesn't matter
	injectObservabilityMetadata(cfg)

	// Set middleware defaults based on environment if not explicitly configured
	if cfg.environment == EnvironmentDevelopment && !cfg.middleware.includeLogger {
		cfg.middleware.includeLogger = true
	}

	// Validate configuration
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create router with options if provided
	var routerOpts []router.Option
	if cfg.router != nil {
		routerOpts = cfg.router.options
	}
	r := router.New(routerOpts...)

	// Create app
	app := &App{
		router: r,
		config: cfg,
	}

	// Initialize observability
	if cfg.logging != nil && cfg.logging.enabled {
		if cfg.logging.usePrebuilt {
			// Use pre-provided logging config
			app.logging = cfg.logging.config
			r.SetLogger(app.logging)
		} else {
			// Initialize new logging config
			loggingConfig, err := logging.New(cfg.logging.options...)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize logging: %w", err)
			}
			app.logging = loggingConfig
			r.SetLogger(app.logging)
		}
	}

	if cfg.metrics != nil && cfg.metrics.enabled {
		if cfg.metrics.usePrebuilt {
			// Use pre-provided metrics config
			app.metrics = cfg.metrics.config
			r.SetMetricsRecorder(app.metrics)
		} else {
			// Initialize new metrics config
			metricsConfig, err := metrics.New(cfg.metrics.options...)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize metrics: %w", err)
			}
			app.metrics = metricsConfig
			r.SetMetricsRecorder(app.metrics)
		}
	}

	if cfg.tracing != nil && cfg.tracing.enabled {
		if cfg.tracing.usePrebuilt {
			// Use pre-provided tracing config
			app.tracing = cfg.tracing.config
			r.SetTracingRecorder(app.tracing)
		} else {
			// Initialize new tracing config
			tracingConfig, err := tracing.New(cfg.tracing.options...)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize tracing: %w", err)
			}
			app.tracing = tracingConfig
			r.SetTracingRecorder(app.tracing)
		}
	}

	// Add default middleware based on configuration
	if cfg.middleware.includeLogger {
		app.Use(middleware.Logger())
	}
	if cfg.middleware.includeRecovery {
		app.Use(middleware.Recovery())
	}

	return app, nil
}

// MustNew creates a new App instance or panics on error.
// Use this for convenience when you want to fail fast on initialization errors.
//
// Example:
//
//	app := app.MustNew(
//	    app.WithServiceName("my-service"),
//	    app.WithObservability(),
//	)
func MustNew(opts ...Option) *App {
	app, err := New(opts...)
	if err != nil {
		panic(fmt.Sprintf("app initialization failed: %v", err))
	}
	return app
}

// Router returns the underlying router for advanced usage.
func (a *App) Router() *router.Router {
	return a.router
}

// GET registers a GET route.
func (a *App) GET(path string, handler router.HandlerFunc) {
	a.router.GET(path, handler)
}

// POST registers a POST route.
func (a *App) POST(path string, handler router.HandlerFunc) {
	a.router.POST(path, handler)
}

// PUT registers a PUT route.
func (a *App) PUT(path string, handler router.HandlerFunc) {
	a.router.PUT(path, handler)
}

// DELETE registers a DELETE route.
func (a *App) DELETE(path string, handler router.HandlerFunc) {
	a.router.DELETE(path, handler)
}

// PATCH registers a PATCH route.
func (a *App) PATCH(path string, handler router.HandlerFunc) {
	a.router.PATCH(path, handler)
}

// HEAD registers a HEAD route.
func (a *App) HEAD(path string, handler router.HandlerFunc) {
	a.router.HEAD(path, handler)
}

// OPTIONS registers an OPTIONS route.
func (a *App) OPTIONS(path string, handler router.HandlerFunc) {
	a.router.OPTIONS(path, handler)
}

// Use adds middleware to the app.
func (a *App) Use(middleware ...router.HandlerFunc) {
	a.router.Use(middleware...)
}

// Group creates a new route group.
func (a *App) Group(prefix string, middleware ...router.HandlerFunc) *router.Group {
	return a.router.Group(prefix, middleware...)
}

// Static serves static files from the given directory.
func (a *App) Static(prefix, root string) {
	a.router.Static(prefix, root)
}

// serverStartFunc defines the function type for starting a server.
type serverStartFunc func() error

// logLifecycleEvent logs a lifecycle event using structured logging if available,
// otherwise falls back to the standard library log package.
func (a *App) logLifecycleEvent(level slog.Level, msg string, args ...any) {
	if a.logging != nil {
		logger := a.logging.Logger()
		if logger.Enabled(context.Background(), level) {
			logger.Log(context.Background(), level, msg, args...)
		}
	} else {
		// Fall back to stdlib log for backwards compatibility
		if len(args) == 0 {
			log.Println(msg)
		} else {
			// Format key-value pairs for stdlib log
			logMsg := msg
			for i := 0; i < len(args)-1; i += 2 {
				if key, ok := args[i].(string); ok {
					logMsg += fmt.Sprintf(" %s=%v", key, args[i+1])
				}
			}
			log.Println(logMsg)
		}
	}
}

// logStartupInfo logs startup information including address, environment, and observability status.
func (a *App) logStartupInfo(addr, protocol string) {
	attrs := []any{
		"address", addr,
		"environment", a.config.environment,
		"protocol", protocol,
	}

	if a.metrics != nil {
		attrs = append(attrs, "metrics_enabled", true, "metrics_address", a.metrics.GetServerAddress())
	}

	a.logLifecycleEvent(slog.LevelInfo, "server starting", attrs...)

	if a.tracing != nil {
		a.logLifecycleEvent(slog.LevelInfo, "tracing enabled")
	}
}

// printRoutesIfDev prints registered routes if in development mode.
func (a *App) printRoutesIfDev() {
	if a.config.environment == EnvironmentDevelopment {
		if a.logging != nil {
			logger := a.logging.Logger()
			logger.Info("registered routes", "routes_header", "\n📋 Registered Routes:")
		} else {
			log.Println("\n📋 Registered Routes:")
		}
		a.router.PrintRoutes()
		log.Println()
	}
}

// shutdownObservability gracefully shuts down all enabled observability components.
func (a *App) shutdownObservability(ctx context.Context) {
	// Shutdown metrics if running
	if a.metrics != nil {
		if err := a.metrics.Shutdown(ctx); err != nil {
			a.logLifecycleEvent(slog.LevelWarn, "metrics shutdown failed", "error", err)
		}
	}

	// Shutdown tracing if running
	if a.tracing != nil {
		if err := a.tracing.Shutdown(ctx); err != nil {
			a.logLifecycleEvent(slog.LevelWarn, "tracing shutdown failed", "error", err)
		}
	}
}

// runServer handles the common lifecycle logic for starting and shutting down an HTTP server.
// It accepts an http.Server and a startFunc (either ListenAndServe or ListenAndServeTLS).
func (a *App) runServer(server *http.Server, startFunc serverStartFunc, protocol string) error {
	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		a.logStartupInfo(server.Addr, protocol)
		a.printRoutesIfDev()

		if err := startFunc(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("%s server failed to start: %w", protocol, err)
		}
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return err
	case <-quit:
		a.logLifecycleEvent(slog.LevelInfo, "server shutting down", "protocol", protocol)
	}

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), a.config.server.shutdownTimeout)
	defer cancel()

	// Shutdown the server
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("%s server forced to shutdown: %w", protocol, err)
	}

	// Shutdown observability components (metrics and tracing)
	a.shutdownObservability(ctx)

	a.logLifecycleEvent(slog.LevelInfo, "server exited", "protocol", protocol)
	return nil
}

// Run starts the HTTP server with graceful shutdown.
func (a *App) Run(addr string) error {
	server := &http.Server{
		Addr:              addr,
		Handler:           a.router,
		ReadTimeout:       a.config.server.readTimeout,
		WriteTimeout:      a.config.server.writeTimeout,
		IdleTimeout:       a.config.server.idleTimeout,
		ReadHeaderTimeout: a.config.server.readHeaderTimeout,
		MaxHeaderBytes:    a.config.server.maxHeaderBytes,
	}

	return a.runServer(server, server.ListenAndServe, "HTTP")
}

// RunTLS starts the HTTPS server with graceful shutdown.
func (a *App) RunTLS(addr, certFile, keyFile string) error {
	server := &http.Server{
		Addr:              addr,
		Handler:           a.router,
		ReadTimeout:       a.config.server.readTimeout,
		WriteTimeout:      a.config.server.writeTimeout,
		IdleTimeout:       a.config.server.idleTimeout,
		ReadHeaderTimeout: a.config.server.readHeaderTimeout,
		MaxHeaderBytes:    a.config.server.maxHeaderBytes,
	}

	return a.runServer(server, func() error {
		return server.ListenAndServeTLS(certFile, keyFile)
	}, "HTTPS")
}

// GetMetricsHandler returns the metrics HTTP handler if metrics are enabled.
// Returns an error if metrics are not enabled or if using a non-Prometheus provider.
func (a *App) GetMetricsHandler() (http.Handler, error) {
	if a.metrics == nil {
		return nil, fmt.Errorf("metrics not enabled, use WithMetrics() to enable metrics")
	}
	return a.metrics.GetHandler()
}

// GetMetricsServerAddress returns the metrics server address if metrics are enabled.
func (a *App) GetMetricsServerAddress() string {
	if a.metrics == nil {
		return ""
	}
	return a.metrics.GetServerAddress()
}

// ServiceName returns the service name.
func (a *App) ServiceName() string {
	return a.config.serviceName
}

// ServiceVersion returns the service version.
func (a *App) ServiceVersion() string {
	return a.config.serviceVersion
}

// Environment returns the current environment (development/production).
func (a *App) Environment() string {
	return a.config.environment
}

// GetMetrics returns the metrics configuration if enabled.
func (a *App) GetMetrics() *metrics.Config {
	return a.metrics
}

// GetTracing returns the tracing configuration if enabled.
func (a *App) GetTracing() *tracing.Config {
	return a.tracing
}
