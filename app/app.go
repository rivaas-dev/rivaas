// Package app provides the main application implementation for Rivaas.
package app

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

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
	router    *router.Router
	metrics   *metrics.Config
	tracing   *tracing.Config
	logging   *logging.Config
	config    *config
	hooks     *Hooks
	readiness *ReadinessManager
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

	// Cross-field validation: ReadTimeout should not exceed WriteTimeout.
	//
	// Rationale: HTTP servers must complete reading the request before writing the response.
	// If ReadTimeout > WriteTimeout, the server may successfully read the full request but
	// then immediately timeout attempting to write the response. This creates a poor user
	// experience where requests appear to succeed (from the client's perspective during upload)
	// but fail during response delivery.
	//
	// Performance consideration: Write operations to established connections are typically
	// I/O bound and faster than reads (which may involve slow clients or large payloads).
	// Setting WriteTimeout >= ReadTimeout provides a safety margin for response delivery.
	if sc.readTimeout > 0 && sc.writeTimeout > 0 {
		if sc.readTimeout > sc.writeTimeout {
			errs.Add(newComparisonError("server.readTimeout", "server.writeTimeout",
				sc.readTimeout, sc.writeTimeout,
				"read timeout should not exceed write timeout"))
		}
	}

	// Validate shutdown timeout is reasonable (at least 1 second).
	//
	// Rationale: Graceful shutdown is a multi-step process that requires coordination:
	//   1. Stop accepting new connections (~immediate)
	//   2. Wait for in-flight requests to complete (variable, request-dependent)
	//   3. Close idle keep-alive connections (requires TCP FIN/ACK exchange)
	//   4. Flush observability buffers (metrics, traces, logs)
	//   5. Clean up resources (file handles, database connections)
	//
	// A timeout < 1s is insufficient for steps 2-5 in production environments,
	// especially under load. This forces abrupt termination, potentially causing:
	// - Incomplete responses (client sees broken connections)
	// - Lost observability data (metrics/logs not flushed)
	// - Resource leaks (connections not properly closed)
	//
	// Performance impact: Longer shutdowns allow clean termination without resource
	// leaks, improving overall system stability during deployments or scaling events.
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
		router:    r,
		config:    cfg,
		hooks:     &Hooks{},
		readiness: &ReadinessManager{},
	}

	// Initialize observability with service metadata injected
	if cfg.logging != nil && cfg.logging.enabled {
		// Prepend service metadata to user options
		loggingOpts := []logging.Option{
			logging.WithServiceName(cfg.serviceName),
			logging.WithServiceVersion(cfg.serviceVersion),
		}
		loggingOpts = append(loggingOpts, cfg.logging.options...)

		logCfg, err := logging.New(loggingOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize logging: %w", err)
		}
		app.logging = logCfg
		r.SetLogger(app.logging)
	}

	if cfg.metrics != nil && cfg.metrics.enabled {
		// Prepend service metadata to user options
		metricsOpts := []metrics.Option{
			metrics.WithServiceName(cfg.serviceName),
			metrics.WithServiceVersion(cfg.serviceVersion),
		}
		metricsOpts = append(metricsOpts, cfg.metrics.options...)

		metricsCfg, err := metrics.New(metricsOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize metrics: %w", err)
		}
		app.metrics = metricsCfg
		r.SetMetricsRecorder(app.metrics)
	}

	if cfg.tracing != nil && cfg.tracing.enabled {
		// Prepend service metadata to user options
		tracingOpts := []tracing.Option{
			tracing.WithServiceName(cfg.serviceName),
			tracing.WithServiceVersion(cfg.serviceVersion),
		}
		tracingOpts = append(tracingOpts, cfg.tracing.options...)

		tracingCfg, err := tracing.New(tracingOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize tracing: %w", err)
		}
		app.tracing = tracingCfg
		r.SetTracingRecorder(app.tracing)
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

// Readiness returns the readiness manager for registering gates.
//
// Example:
//
//	type DatabaseGate struct {
//	    db *sql.DB
//	}
//	func (g *DatabaseGate) Ready() bool { return g.db.Ping() == nil }
//	func (g *DatabaseGate) Name() string { return "database" }
//
//	app.Readiness().Register("db", &DatabaseGate{db: db})
func (a *App) Readiness() *ReadinessManager {
	return a.readiness
}

// GET registers a GET route and returns the Route for chaining.
func (a *App) GET(path string, handler router.HandlerFunc) *router.Route {
	route := a.router.GET(path, handler)
	a.fireRouteHook(*route)
	return route
}

// POST registers a POST route and returns the Route for chaining.
func (a *App) POST(path string, handler router.HandlerFunc) *router.Route {
	route := a.router.POST(path, handler)
	a.fireRouteHook(*route)
	return route
}

// PUT registers a PUT route and returns the Route for chaining.
func (a *App) PUT(path string, handler router.HandlerFunc) *router.Route {
	route := a.router.PUT(path, handler)
	a.fireRouteHook(*route)
	return route
}

// DELETE registers a DELETE route and returns the Route for chaining.
func (a *App) DELETE(path string, handler router.HandlerFunc) *router.Route {
	route := a.router.DELETE(path, handler)
	a.fireRouteHook(*route)
	return route
}

// PATCH registers a PATCH route and returns the Route for chaining.
func (a *App) PATCH(path string, handler router.HandlerFunc) *router.Route {
	route := a.router.PATCH(path, handler)
	a.fireRouteHook(*route)
	return route
}

// HEAD registers a HEAD route and returns the Route for chaining.
func (a *App) HEAD(path string, handler router.HandlerFunc) *router.Route {
	route := a.router.HEAD(path, handler)
	a.fireRouteHook(*route)
	return route
}

// OPTIONS registers an OPTIONS route and returns the Route for chaining.
func (a *App) OPTIONS(path string, handler router.HandlerFunc) *router.Route {
	route := a.router.OPTIONS(path, handler)
	a.fireRouteHook(*route)
	return route
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
// This is a convenience wrapper that delegates to router.Static.
//
// Example:
//
//	app.Static("/static", "./public")
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

// Metrics returns the metrics configuration if enabled.
func (a *App) Metrics() *metrics.Config {
	return a.metrics
}

// Tracing returns the tracing configuration if enabled.
func (a *App) Tracing() *tracing.Config {
	return a.tracing
}

// Route retrieves a route by name. Returns the route and true if found.
// Panics if the router is not frozen (call after app.Run() or app.Router().Freeze()).
//
// Example:
//
//	route, ok := app.Route("users.get")
//	if ok {
//	    fmt.Printf("Route: %s %s\n", route.Method(), route.Path())
//	}
func (a *App) Route(name string) (router.Route, bool) {
	return a.router.GetRoute(name)
}

// Routes returns an immutable snapshot of all named routes.
// Panics if the router is not frozen (call after app.Run() or app.Router().Freeze()).
//
// Example:
//
//	routes := app.Routes()
//	for _, route := range routes {
//	    fmt.Printf("%s: %s %s\n", route.Name(), route.Method(), route.Path())
//	}
func (a *App) Routes() []router.Route {
	return a.router.GetRoutes()
}

// URLFor generates a URL from a route name and parameters.
// Returns an error if the route is not found or if required parameters are missing.
//
// Example:
//
//	url, err := app.URLFor("users.get", map[string]string{"id": "123"}, nil)
//	// Returns: "/users/123", nil
func (a *App) URLFor(routeName string, params map[string]string, query map[string][]string) (string, error) {
	var urlValues map[string][]string
	if query != nil {
		urlValues = query
	} else {
		urlValues = make(map[string][]string)
	}
	// Convert to url.Values
	vals := make(url.Values)
	for k, v := range urlValues {
		vals[k] = v
	}
	return a.router.URLFor(routeName, params, vals)
}

// MustURLFor generates a URL from a route name and parameters, panicking on error.
// Use this when you're certain the route exists and all parameters are provided.
//
// Example:
//
//	url := app.MustURLFor("users.get", map[string]string{"id": "123"}, nil)
//	// Returns: "/users/123"
func (a *App) MustURLFor(routeName string, params map[string]string, query map[string][]string) string {
	var urlValues map[string][]string
	if query != nil {
		urlValues = query
	} else {
		urlValues = make(map[string][]string)
	}
	// Convert to url.Values
	vals := make(url.Values)
	for k, v := range urlValues {
		vals[k] = v
	}
	return a.router.MustURLFor(routeName, params, vals)
}
