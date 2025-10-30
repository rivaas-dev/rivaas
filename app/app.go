package app

import (
	"context"
	"fmt"
	"log"
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
}

// metricsConfig holds metrics configuration.
type metricsConfig struct {
	enabled bool
	options []metrics.Option
}

// tracingConfig holds tracing configuration.
type tracingConfig struct {
	enabled bool
	options []tracing.Option
}

// loggingConfig holds logging configuration.
type loggingConfig struct {
	enabled bool
	options []logging.Option
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
func WithObservability() Option {
	return func(c *config) {
		c.metrics = &metricsConfig{
			enabled: true,
			options: []metrics.Option{
				metrics.WithServiceName(c.serviceName),
				metrics.WithServiceVersion(c.serviceVersion),
			},
		}
		c.tracing = &tracingConfig{
			enabled: true,
			options: []tracing.Option{
				tracing.WithServiceName(c.serviceName),
				tracing.WithServiceVersion(c.serviceVersion),
			},
		}
		c.logging = &loggingConfig{
			enabled: true,
			options: []logging.Option{
				logging.WithJSONHandler(),
				logging.WithServiceInfo(c.serviceName, c.serviceVersion, c.environment),
			},
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

	// Set middleware defaults based on environment if not explicitly configured
	if cfg.environment == EnvironmentDevelopment && !cfg.middleware.includeLogger {
		cfg.middleware.includeLogger = true
	}

	// Validate configuration
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create router
	r := router.New()

	// Create app
	app := &App{
		router: r,
		config: cfg,
	}

	// Initialize observability
	if cfg.logging != nil && cfg.logging.enabled {
		loggingConfig, err := logging.New(cfg.logging.options...)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize logging: %w", err)
		}
		app.logging = loggingConfig
		r.SetLogger(app.logging)
	}

	if cfg.metrics != nil && cfg.metrics.enabled {
		metricsConfig, err := metrics.New(cfg.metrics.options...)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize metrics: %w", err)
		}
		app.metrics = metricsConfig
		r.SetMetricsRecorder(app.metrics)
	}

	if cfg.tracing != nil && cfg.tracing.enabled {
		tracingConfig, err := tracing.New(cfg.tracing.options...)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize tracing: %w", err)
		}
		app.tracing = tracingConfig
		r.SetTracingRecorder(app.tracing)
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

	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("🚀 Server starting on %s (environment: %s)", addr, a.config.environment)
		if a.metrics != nil {
			log.Printf("📊 Metrics enabled: %s", a.metrics.GetServerAddress())
		}
		if a.tracing != nil {
			log.Printf("🔍 Tracing enabled")
		}

		// Print routes in development mode
		if a.config.environment == EnvironmentDevelopment {
			log.Println("\n📋 Registered Routes:")
			a.router.PrintRoutes()
			log.Println()
		}

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("server failed to start: %w", err)
		}
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return err
	case <-quit:
		log.Println("🛑 Server shutting down...")
	}

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), a.config.server.shutdownTimeout)
	defer cancel()

	// Shutdown the server
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	// Shutdown metrics if running
	if a.metrics != nil {
		if err := a.metrics.Shutdown(ctx); err != nil {
			log.Printf("Failed to shutdown metrics: %v", err)
		}
	}

	log.Println("✅ Server exited")
	return nil
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

	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("🚀 HTTPS server starting on %s (environment: %s)", addr, a.config.environment)
		if a.metrics != nil {
			log.Printf("📊 Metrics enabled: %s", a.metrics.GetServerAddress())
		}
		if a.tracing != nil {
			log.Printf("🔍 Tracing enabled")
		}

		// Print routes in development mode
		if a.config.environment == EnvironmentDevelopment {
			log.Println("\n📋 Registered Routes:")
			a.router.PrintRoutes()
			log.Println()
		}

		if err := server.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("HTTPS server failed to start: %w", err)
		}
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return err
	case <-quit:
		log.Println("🛑 HTTPS server shutting down...")
	}

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), a.config.server.shutdownTimeout)
	defer cancel()

	// Shutdown the server
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("HTTPS server forced to shutdown: %w", err)
	}

	// Shutdown metrics if running
	if a.metrics != nil {
		if err := a.metrics.Shutdown(ctx); err != nil {
			log.Printf("Failed to shutdown metrics: %v", err)
		}
	}

	log.Println("✅ HTTPS server exited")
	return nil
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
