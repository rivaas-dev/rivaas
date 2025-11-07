// Package app provides the main application implementation for Rivaas.
package app

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/common-nighthawk/go-figure"
	"rivaas.dev/logging"
	"rivaas.dev/metrics"
	"rivaas.dev/router"
	"rivaas.dev/router/middleware/accesslog"
	"rivaas.dev/router/middleware/recovery"
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
	enabled     bool
	options     []metrics.Option
	config      *metrics.Config // Pre-initialized config
	usePrebuilt bool            // Whether to use prebuilt config
}

// tracingConfig holds tracing configuration.
type tracingConfig struct {
	enabled     bool
	options     []tracing.Option
	config      *tracing.Config // Pre-initialized config
	usePrebuilt bool            // Whether to use prebuilt config
}

// loggingConfig holds logging configuration.
type loggingConfig struct {
	enabled     bool
	options     []logging.Option
	config      *logging.Config // Pre-initialized config
	usePrebuilt bool            // Whether to use prebuilt config
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

// Validate validates the server configuration and returns all validation errors.
// This method performs comprehensive validation including:
//   - All timeouts must be positive
//   - ReadTimeout should not exceed WriteTimeout (common misconfiguration)
//   - ShutdownTimeout must be at least 1 second for proper graceful shutdown
//   - MaxHeaderBytes must be at least 1KB to handle standard HTTP headers
//
// Returns a ValidationErrors containing all validation failures, or nil if valid.
//
// Example:
//
//	cfg := &serverConfig{
//	    readTimeout:     10 * time.Second,
//	    writeTimeout:    5 * time.Second, // Invalid: read > write
//	    shutdownTimeout: 100 * time.Millisecond, // Invalid: too short
//	}
//	if err := cfg.Validate(); err != nil {
//	    // Handle validation errors
//	}
func (sc *serverConfig) Validate() *ValidationErrors {
	var errs ValidationErrors

	// Validate timeouts are positive
	if sc.readTimeout <= 0 {
		errs.Add(newTimeoutError("server.readTimeout", sc.readTimeout, "must be positive"))
	}

	if sc.writeTimeout <= 0 {
		errs.Add(newTimeoutError("server.writeTimeout", sc.writeTimeout, "must be positive"))
	}

	if sc.idleTimeout <= 0 {
		errs.Add(newTimeoutError("server.idleTimeout", sc.idleTimeout, "must be positive"))
	}

	if sc.readHeaderTimeout <= 0 {
		errs.Add(newTimeoutError("server.readHeaderTimeout", sc.readHeaderTimeout, "must be positive"))
	}

	if sc.shutdownTimeout <= 0 {
		errs.Add(newTimeoutError("server.shutdownTimeout", sc.shutdownTimeout, "must be positive"))
	}

	// Validate max header bytes
	if sc.maxHeaderBytes <= 0 {
		errs.Add(newInvalidValueError("server.maxHeaderBytes", sc.maxHeaderBytes,
			"must be positive"))
	}

	// Cross-field validation: ReadTimeout should not exceed WriteTimeout
	// This is a common misconfiguration that can cause issues where the server
	// times out reading the request body before it can write the response.
	// In practice, write operations are typically faster than read operations,
	// so write timeout should be >= read timeout.
	if sc.readTimeout > 0 && sc.writeTimeout > 0 {
		if sc.readTimeout > sc.writeTimeout {
			errs.Add(newComparisonError("server.readTimeout", "server.writeTimeout",
				sc.readTimeout, sc.writeTimeout,
				"read timeout should not exceed write timeout"))
		}
	}

	// Validate shutdown timeout is reasonable (at least 1 second)
	// Very short shutdown timeouts can cause issues with graceful shutdown,
	// as the server needs time to:
	//   - Stop accepting new connections
	//   - Wait for in-flight requests to complete
	//   - Close idle connections
	//   - Clean up resources
	if sc.shutdownTimeout > 0 && sc.shutdownTimeout < time.Second {
		errs.Add(newInvalidValueError("server.shutdownTimeout", sc.shutdownTimeout,
			"must be at least 1 second for proper graceful shutdown"))
	}

	// Validate max header bytes is reasonable (at least 1KB)
	// Very small values can cause legitimate requests to fail, as standard
	// HTTP headers (User-Agent, Accept, Cookie, etc.) can easily exceed 512 bytes.
	// 1KB is a reasonable minimum that handles most real-world scenarios.
	if sc.maxHeaderBytes > 0 && sc.maxHeaderBytes < 1024 {
		errs.Add(newInvalidValueError("server.maxHeaderBytes", sc.maxHeaderBytes,
			"must be at least 1KB (1024 bytes) to handle standard HTTP headers"))
	}

	return &errs
}

// middlewareConfig holds middleware configuration.
type middlewareConfig struct {
	functions     []router.HandlerFunc
	explicitlySet bool // Tracks if WithMiddleware was called
}

// routerConfig holds router configuration options.
type routerConfig struct {
	options []router.Option
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

// validate checks if the configuration is valid and returns structured errors.
// It collects all validation errors before returning them, allowing users to
// see all issues at once rather than one at a time.
func (c *config) validate() error {
	var errs ValidationErrors

	// Validate service name
	if c.serviceName == "" {
		errs.Add(newEmptyFieldError("serviceName"))
	}

	// Validate service version
	if c.serviceVersion == "" {
		errs.Add(newEmptyFieldError("serviceVersion"))
	}

	// Validate environment
	if c.environment != EnvironmentDevelopment && c.environment != EnvironmentProduction {
		errs.Add(newInvalidEnumError("environment", c.environment,
			[]string{EnvironmentDevelopment, EnvironmentProduction}))
	}

	// Validate server configuration
	if c.server != nil {
		// Use the dedicated Validate() method for better separation of concerns
		serverErrs := c.server.Validate()
		if serverErrs != nil && serverErrs.HasErrors() {
			// Merge server validation errors into the main error collection
			errs.Errors = append(errs.Errors, serverErrs.Errors...)
		}
	}

	// Return all errors if any exist
	return errs.ToError()
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
			functions: []router.HandlerFunc{},
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

	// Set default middleware based on environment if none explicitly provided
	// Defaults are only applied if WithMiddleware() was never called.
	// If WithMiddleware() is called (even with empty args), no defaults are added.
	if !cfg.middleware.explicitlySet {
		// Always include recovery middleware by default
		cfg.middleware.functions = append(cfg.middleware.functions, recovery.New())

		// Include accesslog in development mode by default
		if cfg.environment == EnvironmentDevelopment {
			cfg.middleware.functions = append(cfg.middleware.functions, accesslog.New())
		}
	}

	// Validate configuration
	if err := cfg.validate(); err != nil {
		// Return validation errors as-is (they're already structured)
		// Don't wrap them to preserve the structured error type
		return nil, err
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

	// Add middleware from configuration
	if len(cfg.middleware.functions) > 0 {
		app.Use(cfg.middleware.functions...)
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
//	    app.WithMetrics(),
//	    app.WithTracing(),
//	    app.WithLogging(),
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

// Any registers a route that matches all HTTP methods.
// Useful for catch-all endpoints like health checks or proxies.
//
// Note: This registers 7 separate routes internally (GET, POST, PUT, DELETE,
// PATCH, HEAD, OPTIONS). For endpoints that only need specific methods,
// use individual method registrations (GET, POST, etc.) for better performance.
//
// Example:
//
//	app.Any("/health", healthCheckHandler)
//	app.Any("/webhook/*", webhookProxyHandler)
func (a *App) Any(path string, handler router.HandlerFunc) {
	// Register the handler for all standard HTTP methods
	a.router.GET(path, handler)
	a.router.POST(path, handler)
	a.router.PUT(path, handler)
	a.router.DELETE(path, handler)
	a.router.PATCH(path, handler)
	a.router.HEAD(path, handler)
	a.router.OPTIONS(path, handler)
}

// File serves a single file at the given path.
// Common use case: serving favicon.ico, robots.txt, etc.
//
// Example:
//
//	app.File("/favicon.ico", "./static/favicon.ico")
//	app.File("/robots.txt", "./static/robots.txt")
func (a *App) File(path, filepath string) {
	a.router.StaticFile(path, filepath)
}

// StaticFS serves files from the given filesystem.
// Particularly useful with Go's embed.FS for embedding static assets.
//
// Example:
//
//	//go:embed static
//	var staticFiles embed.FS
//	app.StaticFS("/static", http.FS(staticFiles))
func (a *App) StaticFS(prefix string, fs http.FileSystem) {
	a.router.StaticFS(prefix, fs)
}

// NoRoute sets the handler for requests that don't match any registered routes.
// This allows you to customize 404 error responses instead of using the default http.NotFound.
//
// Example:
//
//	app.NoRoute(func(c *router.Context) {
//	    c.JSON(404, map[string]string{"error": "route not found"})
//	})
//
// Setting handler to nil will restore the default http.NotFound behavior.
func (a *App) NoRoute(handler router.HandlerFunc) {
	a.router.NoRoute(handler)
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

// getColorWriter returns a colorprofile.Writer configured for the app's environment.
// In production mode, ANSI colors are stripped. In development, colors are
// automatically downsampled based on terminal capabilities.
func (a *App) getColorWriter(w io.Writer) *colorprofile.Writer {
	cpw := colorprofile.NewWriter(w, os.Environ())
	// In production, explicitly strip all ANSI sequences
	if a.config.environment == EnvironmentProduction {
		cpw.Profile = colorprofile.NoTTY
	}
	return cpw
}

// printStartupBanner prints an eye-catching ASCII art startup banner with service information.
// The banner displays dynamically generated ASCII art of the service name along with version, environment, address, and routes.
func (a *App) printStartupBanner(addr, protocol string) {
	w := a.getColorWriter(os.Stdout)

	// Generate ASCII art from service name using go-figure
	// Using "standard" font as default (can be customized), strict mode disabled for safety
	myFigure := figure.NewFigure(a.config.serviceName, "", false)
	asciiLines := myFigure.Slicify()

	// Apply gradient color effect based on environment
	var gradientColors []string
	if a.config.environment == EnvironmentDevelopment {
		gradientColors = []string{"12", "14", "10", "11"} // Blue, Cyan, Green, Yellow
	} else {
		gradientColors = []string{"10", "11"} // Green, Yellow
	}

	// Create styled ASCII art with gradient effect
	var styledArt strings.Builder
	for _, line := range asciiLines {
		if strings.TrimSpace(line) == "" {
			styledArt.WriteString("\n")
			continue
		}
		for i, char := range line {
			colorIndex := i % len(gradientColors)
			color := gradientColors[colorIndex]
			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color(color)).
				Bold(true)
			styledArt.WriteString(style.Render(string(char)))
		}
		styledArt.WriteString("\n")
	}

	// Create a compact info box with vertical layout
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(12).
		Align(lipgloss.Right)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Bold(true)

	// Normalize address display: ":8080" -> "0.0.0.0:8080"
	displayAddr := addr
	if strings.HasPrefix(addr, ":") {
		displayAddr = "0.0.0.0" + addr
	}

	// Prepend scheme based on protocol
	scheme := "http://"
	if protocol == "HTTPS" {
		scheme = "https://"
	}
	displayAddr = scheme + displayAddr

	versionLabel := labelStyle.Render("Version:")
	versionValue := valueStyle.Foreground(lipgloss.Color("14")).Render(a.config.serviceVersion)
	envLabel := labelStyle.Render("Environment:")
	envValue := valueStyle.Foreground(lipgloss.Color("11")).Render(a.config.environment)
	addrLabel := labelStyle.Render("Address:")
	addrValue := valueStyle.Foreground(lipgloss.Color("10")).Render(displayAddr)

	// Build info box content
	infoLines := []string{
		versionLabel + "  " + versionValue,
		envLabel + "  " + envValue,
		addrLabel + "  " + addrValue,
	}

	// Always show observability info with status
	metricsLabel := labelStyle.Render("Metrics:")
	var metricsValue string
	if a.metrics != nil {
		metricsAddr := a.metrics.GetServerAddress()
		// Normalize metrics address: ":9090" -> "0.0.0.0:9090"
		if strings.HasPrefix(metricsAddr, ":") {
			metricsAddr = "0.0.0.0" + metricsAddr
		}
		// Prepend scheme (metrics server is always HTTP) and append path
		metricsPath := a.metrics.Path()
		if metricsPath == "" {
			metricsPath = "/metrics" // Default path
		}
		metricsAddr = "http://" + metricsAddr + metricsPath
		metricsValue = valueStyle.Foreground(lipgloss.Color("13")).Render(metricsAddr)
	} else {
		metricsValue = valueStyle.Foreground(lipgloss.Color("240")).Render("Disabled")
	}
	infoLines = append(infoLines, metricsLabel+"  "+metricsValue)

	tracingLabel := labelStyle.Render("Tracing:")
	var tracingValue string
	if a.tracing != nil {
		tracingValue = valueStyle.Foreground(lipgloss.Color("12")).Render("Enabled")
	} else {
		tracingValue = valueStyle.Foreground(lipgloss.Color("240")).Render("Disabled")
	}
	infoLines = append(infoLines, tracingLabel+"  "+tracingValue)

	// Create compact info box
	infoContent := strings.Join(infoLines, "\n")

	fmt.Fprintln(w)
	fmt.Fprint(w, styledArt.String())
	fmt.Fprintln(w)
	fmt.Fprint(w, infoContent)
	fmt.Fprintln(w)

	// Add routes section (only in development mode)
	if a.config.environment == EnvironmentDevelopment {
		routes := a.router.Routes()
		if len(routes) > 0 {
			fmt.Fprintln(w)
			a.renderRoutesTable(w, 80)
		}
	}

	fmt.Fprintln(w)
}

// renderRoutesTable renders the routes table to the given writer.
// This is an internal helper method used by both PrintRoutes and the startup banner.
// width specifies the table width (80 for banner, 120 for standalone).
func (a *App) renderRoutesTable(w io.Writer, width int) {
	routes := a.router.Routes()
	if len(routes) == 0 {
		return
	}

	// Define styles for different HTTP methods
	methodStyles := map[string]lipgloss.Style{
		"GET":     lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true), // Green
		"POST":    lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true), // Blue
		"PUT":     lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true), // Yellow
		"DELETE":  lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),  // Red
		"PATCH":   lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true), // Magenta
		"HEAD":    lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true), // Cyan
		"OPTIONS": lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Bold(true),  // Gray
	}

	// Style for version column
	versionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true) // Orange

	// Determine if we should use colors (only in development, Writer checks terminal)
	useColors := a.config.environment == EnvironmentDevelopment

	// Build table rows and calculate content width
	rows := make([][]string, 0, len(routes))
	maxMethodWidth := len("Method")
	maxVersionWidth := len("Version")
	maxPathWidth := len("Path")
	maxHandlerWidth := len("Handler")

	for _, route := range routes {
		method := route.Method
		if useColors {
			if style, ok := methodStyles[method]; ok {
				method = style.Render(method)
			}
		}

		// Format version field (show "-" if empty, style if present)
		version := route.Version
		if version == "" {
			version = "-"
		} else if useColors {
			version = versionStyle.Render(version)
		}

		// Calculate content widths (use original values, not styled ones, for accurate measurement)
		if len(route.Method) > maxMethodWidth {
			maxMethodWidth = len(route.Method)
		}

		versionLen := len(route.Version)
		if versionLen == 0 {
			versionLen = 1 // "-" is 1 char
		}
		if versionLen > maxVersionWidth {
			maxVersionWidth = versionLen
		}

		if len(route.Path) > maxPathWidth {
			maxPathWidth = len(route.Path)
		}

		if len(route.HandlerName) > maxHandlerWidth {
			maxHandlerWidth = len(route.HandlerName)
		}

		rows = append(rows, []string{
			method,
			version,
			route.Path,
			route.HandlerName,
		})
	}

	// Calculate minimum width needed: borders + separators + padding + content
	// Border chars: left (1) + right (1) = 2
	// Separators: 3 vertical bars between 4 columns = 3
	// Padding: 2 chars per column (left + right) * 4 columns = 8
	// Content: sum of max widths for each column
	minWidth := 2 + 3 + 8 + maxMethodWidth + maxVersionWidth + maxPathWidth + maxHandlerWidth

	// Try to get terminal width if available
	// First try to extract file from wrapped writer, then try os.Stdout directly
	terminalWidth := width // Use provided width as fallback

	var file *os.File
	if f, ok := w.(*os.File); ok {
		file = f
	} else {
		// Try os.Stdout as fallback (most common case)
		file = os.Stdout
	}

	if termWidth, _, err := getTerminalSize(file); err == nil && termWidth > 0 {
		terminalWidth = termWidth
	}

	// Determine final table width:
	// - Use calculated minimum if it's larger than provided width
	// - But don't exceed terminal width
	// - Ensure minimum of 60 characters
	tableWidth := minWidth
	if width > tableWidth {
		tableWidth = width
	}
	if terminalWidth > 0 && tableWidth > terminalWidth {
		tableWidth = terminalWidth
	}
	if tableWidth < 60 {
		tableWidth = 60
	}

	// Create table with lipgloss/table
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(func() lipgloss.Style {
			if useColors {
				return lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Gray border
			}
			return lipgloss.NewStyle() // No color for border
		}()).
		StyleFunc(func(row, _ int) lipgloss.Style {
			style := lipgloss.NewStyle().
				Align(lipgloss.Left).
				Padding(0, 1)

			// Header row styling
			if row == 0 && useColors {
				style = style.
					Bold(true).
					Foreground(lipgloss.Color("230")) // Light yellow/white
			}

			return style
		}).
		Headers("Method", "Version", "Path", "Handler").
		Rows(rows...).
		Width(tableWidth)

	// Write to writer
	fmt.Fprintln(w, t.Render())
}

// getTerminalSize attempts to get the terminal size using platform-specific methods.
// Returns width, height, and error.
func getTerminalSize(file *os.File) (int, int, error) {
	if file == nil {
		return 0, 0, fmt.Errorf("file is nil")
	}

	fd := int(file.Fd())

	// For Unix-like systems (Linux, macOS, BSD), use TIOCGWINSZ ioctl
	if runtime.GOOS != "windows" {
		var dimensions struct {
			rows    uint16
			cols    uint16
			xpixels uint16
			ypixels uint16
		}

		// TIOCGWINSZ constant value (0x5413) - get window size
		const TIOCGWINSZ = 0x5413

		// Use syscall to get terminal size
		_, _, errno := syscall.Syscall6(
			syscall.SYS_IOCTL,
			uintptr(fd),
			uintptr(TIOCGWINSZ),
			uintptr(unsafe.Pointer(&dimensions)),
			0, 0, 0,
		)

		if errno == 0 {
			return int(dimensions.cols), int(dimensions.rows), nil
		}
	}

	// For Windows or if ioctl fails, return error
	return 0, 0, fmt.Errorf("unable to get terminal size")
}

// PrintRoutes prints all registered routes to stdout in a formatted table.
// This is useful for development and debugging to see all available routes.
//
// Uses lipgloss/table for beautiful terminal output with color-coded HTTP methods
// and proper table formatting. A colorprofile.Writer automatically downsamples
// ANSI colors to match the terminal's capabilities (TrueColor → ANSI256 → ANSI).
// If output is not a TTY, ANSI sequences are stripped entirely. This respects
// the NO_COLOR environment variable and handles all terminal capability detection
// automatically.
//
// Colors are only enabled in development mode.
//
// Example output:
//
//	┌────────┬──────────────────┬──────────────────┐
//	│ Method │ Path             │ Handler          │
//	├────────┼──────────────────┼──────────────────┤
//	│ GET    │ /                │ handler          │
//	│ GET    │ /users/:id       │ handler          │
//	│ POST   │ /users           │ handler          │
//	└────────┴──────────────────┴──────────────────┘
func (a *App) PrintRoutes() {
	routes := a.router.Routes()
	if len(routes) == 0 {
		fmt.Println("No routes registered")
		return
	}

	// Create a writer that automatically downsamples colors based on terminal capabilities
	// Uses helper method that handles production mode (strips ANSI) and development mode (auto-detects)
	w := a.getColorWriter(os.Stdout)

	// Use internal helper with wider table for standalone use
	a.renderRoutesTable(w, 120)
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
		a.printStartupBanner(server.Addr, protocol)
		a.logStartupInfo(server.Addr, protocol)
		// Routes are now displayed as part of the startup banner

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
