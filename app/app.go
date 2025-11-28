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
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"rivaas.dev/errors"
	"rivaas.dev/logging"
	"rivaas.dev/metrics"
	"rivaas.dev/openapi"
	"rivaas.dev/router"
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

// HandlerFunc defines a handler function that receives an [Context].
// HandlerFunc provides access to both router functionality and app-level features
// like [Context.Bind] and [Context.BindAndValidate].
type HandlerFunc func(*Context)

// noopLogger is a singleton no-op logger used when no logger is configured.
// noopLogger discards all log messages.
var noopLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// App represents the high-level application framework.
// App wraps the router with integrated observability and common middleware.
// Create an App using [New] or [MustNew].
type App struct {
	router      *router.Router
	metrics     *metrics.Config
	tracing     *tracing.Config
	logging     *logging.Config // Logging configuration (can be nil, uses noopLogger fallback)
	config      *config
	hooks       *Hooks
	readiness   *ReadinessManager
	openapi     *openapi.Manager
	contextPool *contextPool
}

// config holds the internal application configuration.
// config maintains encapsulation by keeping all fields private.
type config struct {
	serviceName    string
	serviceVersion string
	environment    string
	server         *serverConfig
	middleware     *middlewareConfig
	router         *routerConfig
	openapi        *openapiConfig
	errors         *errorsConfig
	observability  *observabilitySettings // Unified observability settings (metrics, tracing, logging)
	health         *healthSettings        // Health endpoint settings (healthz, readyz)
	debug          *debugSettings         // Debug endpoint settings (pprof)
}

// metricsConfig holds metrics configuration settings.
type metricsConfig struct {
	enabled bool
	options []metrics.Option
}

// tracingConfig holds tracing configuration settings.
type tracingConfig struct {
	enabled bool
	options []tracing.Option
}

// serverConfig holds server configuration settings.
type serverConfig struct {
	readTimeout       time.Duration
	writeTimeout      time.Duration
	idleTimeout       time.Duration
	readHeaderTimeout time.Duration
	maxHeaderBytes    int
	shutdownTimeout   time.Duration
}

// Validate validates the server configuration and returns all validation errors.
// Validate performs comprehensive validation including:
//   - All timeouts must be positive
//   - ReadTimeout should not exceed WriteTimeout (common misconfiguration)
//   - ShutdownTimeout must be at least 1 second for proper graceful shutdown
//   - MaxHeaderBytes must be at least 1KB to handle standard HTTP headers
//
// Validate returns a ValidationErrors containing all validation failures, or nil if valid.
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
func (sc *serverConfig) Validate() *ValidationError {
	var errs ValidationError

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
	// Write operations to established connections are typically I/O bound and may involve
	// different network conditions than reads (which may involve slow clients or large payloads).
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
	// Longer shutdowns allow clean termination without resource leaks, improving
	// overall system stability during deployments or scaling events.
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

// middlewareConfig holds middleware configuration settings.
type middlewareConfig struct {
	functions       []HandlerFunc
	disableDefaults bool // If true, default middleware (recovery) is not applied
}

// errorsConfig holds error formatting configuration settings.
type errorsConfig struct {
	// Single formatter mode
	formatter errors.Formatter

	// Multi-formatter mode with content negotiation
	formatters    map[string]errors.Formatter
	defaultFormat string
}

// routerConfig holds router configuration options.
// routerConfig stores options that are passed to the underlying router.
type routerConfig struct {
	options []router.Option
}

// validate checks if the configuration is valid and returns structured errors.
// validate collects all validation errors before returning them, allowing users to
// see all issues at once rather than one at a time.
func (c *config) validate() error {
	var errs ValidationError

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

	// Validate OpenAPI configuration
	if c.openapi != nil && c.openapi.enabled && c.openapi.initErr != nil {
		errs.Add(newInvalidValueError("openapi", nil, c.openapi.initErr.Error()))
	}

	// Return all errors if any exist
	return errs.ToError()
}

// shouldApplyDefaultMiddleware determines if default middleware should be applied.
// shouldApplyDefaultMiddleware returns true unless WithoutDefaultMiddleware() was called.
func shouldApplyDefaultMiddleware(cfg *config) bool {
	return !cfg.middleware.disableDefaults
}

// applyDefaultMiddleware applies default router middleware based on environment.
// applyDefaultMiddleware always includes recovery middleware for panic recovery.
// These are router middleware, applied directly to the router,
// not through app.Use() to ensure they run at the correct position in the chain.
func applyDefaultMiddleware(r *router.Router, environment string) {
	// Always include recovery middleware by default (router middleware)
	r.Use(recovery.New())

	// NOTE: Access logging is now handled by the unified ObservabilityRecorder
	// (see app.New() for configuration)
}

// defaultConfig returns a configuration with default values.
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
			functions: []HandlerFunc{},
		},
		errors: &errorsConfig{
			formatter: &errors.RFC9457{}, // Default to RFC 9457
		},
	}
}

// New creates a new App instance with the given options.
// New returns an error if the configuration is invalid or initialization fails.
//
// Example:
//
//	app, err := app.New(
//	    app.WithServiceName("my-service"),
//	    app.WithObservability(
//	        app.WithMetrics(metrics.WithProvider(metrics.PrometheusProvider)),
//	        app.WithTracing(tracing.WithProvider(tracing.OTLPProvider)),
//	    ),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
func New(opts ...Option) (*App, error) {
	// Start with default configuration
	cfg := defaultConfig()

	// Apply user options
	for _, opt := range opts {
		opt(cfg)
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
	r, err := router.New(routerOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}

	// Finalize OpenAPI config: inject service name/version if not explicitly set.
	// This happens after all options are applied, so option order doesn't matter.
	if cfg.openapi != nil && cfg.openapi.enabled && cfg.openapi.config != nil {
		openapiCfg := cfg.openapi.config
		if openapiCfg.Info.Title == "API" && cfg.serviceName != "" {
			openapiCfg.Info.Title = cfg.serviceName
		}
		if openapiCfg.Info.Version == "1.0.0" && cfg.serviceVersion != "" {
			openapiCfg.Info.Version = cfg.serviceVersion
		}
	}

	// Create app
	var openapiMgr *openapi.Manager
	if cfg.openapi != nil && cfg.openapi.enabled && cfg.openapi.config != nil {
		openapiMgr = openapi.NewManager(cfg.openapi.config)
	}

	app := &App{
		router: r,
		config: cfg,
		hooks:  &Hooks{},
		readiness: &ReadinessManager{
			gates: make(map[string]Gate),
		},
		openapi:     openapiMgr,
		contextPool: newContextPool(),
	}

	// Apply default router middleware if not explicitly set
	if shouldApplyDefaultMiddleware(cfg) {
		applyDefaultMiddleware(r, cfg.environment)
	}

	// Get observability settings (use defaults if not configured)
	obsSettings := cfg.observability
	if obsSettings == nil {
		obsSettings = defaultObservabilitySettings()
	}

	// Check for observability configuration errors
	if len(obsSettings.validationErrors) > 0 {
		return nil, obsSettings.validationErrors[0]
	}

	// Initialize logging if configured
	var loggingCfg *logging.Config
	if obsSettings.logging != nil && obsSettings.logging.enabled {
		// Prepend service metadata to user options (same pattern as metrics/tracing)
		loggingOpts := []logging.Option{
			logging.WithServiceName(cfg.serviceName),
			logging.WithServiceVersion(cfg.serviceVersion),
			logging.WithEnvironment(cfg.environment),
		}
		loggingOpts = append(loggingOpts, obsSettings.logging.options...)

		loggingCfg, err = logging.New(loggingOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize logging: %w", err)
		}
		app.logging = loggingCfg
	}

	// Initialize observability components (metrics, tracing)
	var metricsCfg *metrics.Config
	var tracingCfg *tracing.Config

	if obsSettings.metrics != nil && obsSettings.metrics.enabled {
		// Prepend service metadata to user options
		metricsOpts := []metrics.Option{
			metrics.WithServiceName(cfg.serviceName),
			metrics.WithServiceVersion(cfg.serviceVersion),
		}

		// Auto-wire logger to metrics if logging is enabled
		if loggingCfg != nil {
			metricsOpts = append(metricsOpts, metrics.WithLogger(loggingCfg.Logger()))
		}

		metricsOpts = append(metricsOpts, obsSettings.metrics.options...)

		// Configure metrics server based on user choice
		if obsSettings.metricsOnMainRouter {
			// Mount on main router: disable separate server
			metricsOpts = append(metricsOpts, metrics.WithServerDisabled())
		} else if obsSettings.metricsSeparateServer {
			// Custom separate server configuration
			if obsSettings.metricsSeparateAddr != "" {
				metricsOpts = append(metricsOpts, metrics.WithPort(obsSettings.metricsSeparateAddr))
			}
			if obsSettings.metricsSeparatePath != "" {
				metricsOpts = append(metricsOpts, metrics.WithPath(obsSettings.metricsSeparatePath))
			}
		}
		// Default: use metrics package defaults (:9090/metrics)

		metricsCfg, err = metrics.New(metricsOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize metrics: %w", err)
		}
		app.metrics = metricsCfg

		// Mount metrics endpoint on main router if configured
		if obsSettings.metricsOnMainRouter {
			handler, handlerErr := metricsCfg.GetHandler()
			if handlerErr != nil {
				return nil, fmt.Errorf("failed to get metrics handler: %w", handlerErr)
			}
			r.GET(obsSettings.metricsMainRouterPath, func(c *router.Context) {
				handler.ServeHTTP(c.Response, c.Request)
			})
		}
	}

	if obsSettings.tracing != nil && obsSettings.tracing.enabled {
		// Prepend service metadata to user options
		tracingOpts := []tracing.Option{
			tracing.WithServiceName(cfg.serviceName),
			tracing.WithServiceVersion(cfg.serviceVersion),
		}

		// Auto-wire logger to tracing if logging is enabled
		if loggingCfg != nil {
			tracingOpts = append(tracingOpts, tracing.WithLogger(loggingCfg.Logger()))
		}

		tracingOpts = append(tracingOpts, obsSettings.tracing.options...)

		tracingCfg, err = tracing.New(tracingOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize tracing: %w", err)
		}
		app.tracing = tracingCfg
	}

	// Create unified observability recorder if any observability is enabled
	if metricsCfg != nil || tracingCfg != nil || loggingCfg != nil {
		// In production, default to logging errors only
		logErrorsOnly := obsSettings.logErrorsOnly
		if cfg.environment == EnvironmentProduction && !obsSettings.logErrorsOnly {
			logErrorsOnly = true
		}

		// Get the *slog.Logger from logging config (if available)
		var logger *slog.Logger
		if loggingCfg != nil {
			logger = loggingCfg.Logger()
		}

		obsRecorder := newObservabilityRecorder(&observabilityConfig{
			Metrics:           metricsCfg,
			Tracing:           tracingCfg,
			Logger:            logger,
			PathFilter:        obsSettings.pathFilter,
			LogAccessRequests: obsSettings.accessLogging,
			LogErrorsOnly:     logErrorsOnly,
			SlowThreshold:     obsSettings.slowThreshold,
		})
		r.SetObservabilityRecorder(obsRecorder)
	}

	// Register health endpoints if configured
	if cfg.health != nil && cfg.health.enabled {
		if err := app.registerHealthEndpoints(cfg.health); err != nil {
			return nil, fmt.Errorf("failed to register health endpoints: %w", err)
		}
	}

	// Register debug endpoints if configured
	if cfg.debug != nil && cfg.debug.enabled {
		if err := app.registerDebugEndpoints(cfg.debug); err != nil {
			return nil, fmt.Errorf("failed to register debug endpoints: %w", err)
		}
	}

	// Add middleware from configuration
	if len(cfg.middleware.functions) > 0 {
		app.Use(cfg.middleware.functions...)
	}

	return app, nil
}

// MustNew creates a new [App] instance or panics on error.
// MustNew is a convenience function that panics if initialization fails, useful for initialization in main() functions.
//
// Example:
//
//	app := app.MustNew(
//	    app.WithServiceName("my-service"),
//	    app.WithServiceVersion("v1.0.0"),
//	    app.WithObservability(
//	        app.WithLogging(logging.WithJSONHandler()),
//	        app.WithMetrics(metrics.WithProvider(metrics.PrometheusProvider)),
//	        app.WithTracing(tracing.WithProvider(tracing.OTLPProvider)),
//	    ),
//	)
func MustNew(opts ...Option) *App {
	app, err := New(opts...)
	if err != nil {
		panic(fmt.Sprintf("app initialization failed: %v", err))
	}
	return app
}

// Router returns the underlying router for advanced usage.
// Router provides access to router-level features that are not exposed through App.
//
// Example:
//
//	app.Router().Freeze() // Manually freeze router
//	app.Router().SetObservabilityRecorder(customRecorder)
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

// wrapRouteWithOpenAPI creates a RouteWrapper that combines router.Route and OpenAPI metadata.
// wrapRouteWithOpenAPI is used internally when registering routes.
func (a *App) wrapRouteWithOpenAPI(route *router.Route, method, path string) *RouteWrapper {
	var oapi *openapi.RouteWrapper
	if a.openapi != nil {
		// Register route with OpenAPI and get wrapper
		oapi = a.openapi.OnRouteAdded(route)
	}
	return &RouteWrapper{
		route:   route,
		openapi: oapi,
	}
}

// WrapHandler wraps an app.HandlerFunc to convert it to a router.HandlerFunc.
// WrapHandler creates an app.Context from the router.Context and manages pooling.
//
// The context is guaranteed to be returned to the pool even if the handler panics,
// ensuring no context leaks occur. The recovery middleware will still catch panics
// for proper error handling, but this ensures resource cleanup.
//
// WrapHandler is useful when you need to use router-level features (like route constraints)
// while still using app.HandlerFunc with full app.Context support.
//
// Example:
//
//	a.Router().GET("/users/:id", a.WrapHandler(handlers.GetUserByID)).WhereInt("id")
func (a *App) WrapHandler(handler HandlerFunc) router.HandlerFunc {
	return a.wrapHandler(handler)
}

// wrapHandler wraps an app.HandlerFunc to convert it to a router.HandlerFunc.
// wrapHandler creates an app.Context from the router.Context and manages pooling.
//
// The context is guaranteed to be returned to the pool even if the handler panics,
// ensuring no context leaks occur. The recovery middleware will still catch panics
// for proper error handling, but this ensures resource cleanup.
func (a *App) wrapHandler(handler HandlerFunc) router.HandlerFunc {
	return func(rc *router.Context) {
		// Get app context from pool
		ac := a.contextPool.Get()

		// Ensure cleanup even on panic
		defer func() {
			// Clear references before returning to pool
			ac.Context = nil
			ac.app = nil
			ac.bindingMeta = nil
			ac.logger = nil
			a.contextPool.Put(ac)
		}()

		// Initialize context
		ac.Context = rc
		ac.app = a
		ac.bindingMeta = nil

		// Build request-scoped logger (never nil)
		ac.logger = buildRequestLogger(a.BaseLogger(), rc)

		// Call the handler
		handler(ac)
	}
}

// GET registers a GET route and returns a RouteWrapper for constraints and OpenAPI documentation.
//
// If OpenAPI is enabled, the RouteWrapper can be used for fluent documentation.
// If OpenAPI is disabled, the RouteWrapper still supports route constraints.
//
// Example:
//
//	app.GET("/users/:id", handler).
//	    Doc("Get user", "Retrieves a user by ID").
//	    Response(200, UserResponse{}).
//	    WhereInt("id")
//
//	// With inline middleware
//	app.GET("/users/:id", Auth(), GetUser)
func (a *App) GET(path string, handlers ...HandlerFunc) *RouteWrapper {
	routerHandlers := make([]router.HandlerFunc, len(handlers))
	for i, h := range handlers {
		routerHandlers[i] = a.wrapHandler(h)
	}
	route := a.router.GET(path, routerHandlers...)
	a.fireRouteHook(route)
	return a.wrapRouteWithOpenAPI(route, http.MethodGet, path)
}

// POST registers a POST route and returns a RouteWrapper for constraints and OpenAPI documentation.
//
// Example:
//
//	app.POST("/users", handler).
//	    Doc("Create user", "Creates a new user").
//	    Request(CreateUserRequest{}).
//	    Response(201, UserResponse{})
//
//	// With inline middleware
//	app.POST("/users", Validate(), CreateUser)
func (a *App) POST(path string, handlers ...HandlerFunc) *RouteWrapper {
	routerHandlers := make([]router.HandlerFunc, len(handlers))
	for i, h := range handlers {
		routerHandlers[i] = a.wrapHandler(h)
	}
	route := a.router.POST(path, routerHandlers...)
	a.fireRouteHook(route)
	return a.wrapRouteWithOpenAPI(route, http.MethodPost, path)
}

// PUT registers a PUT route and returns a RouteWrapper for constraints and OpenAPI documentation.
//
// Example:
//
//	app.PUT("/users/:id", handler).
//	    Doc("Update user", "Updates an existing user").
//	    WhereInt("id")
func (a *App) PUT(path string, handlers ...HandlerFunc) *RouteWrapper {
	routerHandlers := make([]router.HandlerFunc, len(handlers))
	for i, h := range handlers {
		routerHandlers[i] = a.wrapHandler(h)
	}
	route := a.router.PUT(path, routerHandlers...)
	a.fireRouteHook(route)
	return a.wrapRouteWithOpenAPI(route, http.MethodPut, path)
}

// DELETE registers a DELETE route and returns a RouteWrapper for constraints and OpenAPI documentation.
//
// Example:
//
//	app.DELETE("/users/:id", handler).
//	    Doc("Delete user", "Deletes a user by ID").
//	    WhereInt("id")
func (a *App) DELETE(path string, handlers ...HandlerFunc) *RouteWrapper {
	routerHandlers := make([]router.HandlerFunc, len(handlers))
	for i, h := range handlers {
		routerHandlers[i] = a.wrapHandler(h)
	}
	route := a.router.DELETE(path, routerHandlers...)
	a.fireRouteHook(route)
	return a.wrapRouteWithOpenAPI(route, http.MethodDelete, path)
}

// PATCH registers a PATCH route and returns a RouteWrapper for constraints and OpenAPI documentation.
//
// Example:
//
//	app.PATCH("/users/:id", handler).
//	    Doc("Partially update user", "Updates specific user fields").
//	    WhereInt("id")
func (a *App) PATCH(path string, handlers ...HandlerFunc) *RouteWrapper {
	routerHandlers := make([]router.HandlerFunc, len(handlers))
	for i, h := range handlers {
		routerHandlers[i] = a.wrapHandler(h)
	}
	route := a.router.PATCH(path, routerHandlers...)
	a.fireRouteHook(route)
	return a.wrapRouteWithOpenAPI(route, http.MethodPatch, path)
}

// HEAD registers a HEAD route and returns a RouteWrapper for constraints and OpenAPI documentation.
//
// Example:
//
//	app.HEAD("/users/:id", handler).
//	    WhereInt("id")
func (a *App) HEAD(path string, handlers ...HandlerFunc) *RouteWrapper {
	routerHandlers := make([]router.HandlerFunc, len(handlers))
	for i, h := range handlers {
		routerHandlers[i] = a.wrapHandler(h)
	}
	route := a.router.HEAD(path, routerHandlers...)
	a.fireRouteHook(route)
	return a.wrapRouteWithOpenAPI(route, http.MethodHead, path)
}

// OPTIONS registers an OPTIONS route and returns a RouteWrapper for constraints and OpenAPI documentation.
//
// Example:
//
//	app.OPTIONS("/users", handler)
func (a *App) OPTIONS(path string, handlers ...HandlerFunc) *RouteWrapper {
	routerHandlers := make([]router.HandlerFunc, len(handlers))
	for i, h := range handlers {
		routerHandlers[i] = a.wrapHandler(h)
	}
	route := a.router.OPTIONS(path, routerHandlers...)
	a.fireRouteHook(route)
	return a.wrapRouteWithOpenAPI(route, http.MethodOptions, path)
}

// Use adds middleware to the app.
// Use adds middleware that will be executed for all routes.
//
// Example:
//
//	app.Use(AuthMiddleware(), LoggingMiddleware())
//	app.GET("/users", handler) // Will execute auth + logging + handler
func (a *App) Use(middleware ...HandlerFunc) {
	routerMiddleware := make([]router.HandlerFunc, len(middleware))
	for i, m := range middleware {
		routerMiddleware[i] = a.wrapHandler(m)
	}
	a.router.Use(routerMiddleware...)
}

// Group creates a new route group.
// Group creates groups that support [HandlerFunc] (with [Context]),
// providing access to binding and validation features.
//
// Example:
//
//	api := app.Group("/api/v1", AuthMiddleware())
//	api.GET("/users", handler)    // handler receives *app.Context
//	api.POST("/users", handler)    // handler receives *app.Context
func (a *App) Group(prefix string, middleware ...HandlerFunc) *Group {
	// Create router group without middleware (we handle it at route registration time)
	routerGroup := a.router.Group(prefix)
	return &Group{
		app:        a,
		router:     routerGroup,
		prefix:     prefix,
		middleware: middleware,
	}
}

// Version creates a version group that supports [HandlerFunc].
// Version allows using [Context] features (binding, validation, logging) with router versioning.
//
// Routes registered in a version group are automatically scoped to that version.
// The version is detected from the request path, headers, query parameters, or other
// configured versioning strategies.
//
// Example:
//
//	v1 := app.Version("v1")
//	v1.GET("/status", handlers.Status)
//	v1.POST("/users", handlers.CreateUser)
func (a *App) Version(version string) *VersionGroup {
	routerVersion := a.router.Version(version)
	return &VersionGroup{
		app:           a,
		versionRouter: routerVersion,
	}
}

// Static serves static files from the given directory.
// Static is a convenience wrapper that delegates to router.Static.
//
// Example:
//
//	app.Static("/static", "./public")
func (a *App) Static(prefix, root string) {
	a.router.Static(prefix, root)
}

// Any registers a route that matches all HTTP methods.
// Any is useful for catch-all endpoints like health checks or proxies.
//
// Any registers 7 separate routes internally (GET, POST, PUT, DELETE,
// PATCH, HEAD, OPTIONS). For endpoints that only need specific methods,
// use individual method registrations (GET, POST, etc.).
//
// Returns the RouteWrapper for the GET route (most common for docs/constraints).
//
// Example:
//
//	app.Any("/health", healthCheckHandler)
//	app.Any("/webhook/*", webhookProxyHandler)
func (a *App) Any(path string, handlers ...HandlerFunc) *RouteWrapper {
	// Register the handler for all standard HTTP methods
	// Use the individual method helpers to ensure hooks and OpenAPI integration
	rw := a.GET(path, handlers...)
	a.POST(path, handlers...)
	a.PUT(path, handlers...)
	a.DELETE(path, handlers...)
	a.PATCH(path, handlers...)
	a.HEAD(path, handlers...)
	a.OPTIONS(path, handlers...)
	return rw
}

// File serves a single file at the given path.
// File is commonly used for serving favicon.ico, robots.txt, etc.
//
// Example:
//
//	app.File("/favicon.ico", "./static/favicon.ico")
//	app.File("/robots.txt", "./static/robots.txt")
func (a *App) File(path, filepath string) {
	a.router.StaticFile(path, filepath)
}

// StaticFS serves files from the given filesystem.
// StaticFS is particularly useful with Go's embed.FS for embedding static assets.
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
// NoRoute allows customizing 404 error responses instead of using the default http.NotFound.
//
// Example:
//
//	app.NoRoute(func(c *Context) {
//	    c.JSON(http.StatusNotFound, map[string]string{"error": "route not found"})
//	})
//
// Setting handler to nil restores the default http.NotFound behavior.
func (a *App) NoRoute(handler HandlerFunc) {
	// If handler is nil, pass nil directly to router to restore default behavior.
	// Don't wrap nil handlers as wrapHandler will panic when trying to call them.
	if handler == nil {
		a.router.NoRoute(nil)
		return
	}
	a.router.NoRoute(a.wrapHandler(handler))
}

// GetMetricsHandler returns the metrics HTTP handler if metrics are enabled.
// GetMetricsHandler returns an error if metrics are not enabled or if using a non-Prometheus provider.
//
// Example:
//
//	handler, err := app.GetMetricsHandler()
//	if err != nil {
//	    return err
//	}
//	http.Handle("/metrics", handler)
func (a *App) GetMetricsHandler() (http.Handler, error) {
	if a.metrics == nil {
		return nil, fmt.Errorf("metrics not enabled, use WithObservability(WithMetrics(...)) to enable metrics")
	}
	return a.metrics.GetHandler()
}

// GetMetricsServerAddress returns the metrics server address if metrics are enabled.
// GetMetricsServerAddress returns an empty string if metrics are not enabled.
//
// Example:
//
//	addr := app.GetMetricsServerAddress()
//	if addr != "" {
//	    fmt.Printf("Metrics available at http://%s/metrics\n", addr)
//	}
func (a *App) GetMetricsServerAddress() string {
	if a.metrics == nil {
		return ""
	}
	return a.metrics.GetServerAddress()
}

// ServiceName returns the configured service name.
//
// Example:
//
//	name := app.ServiceName()
//	fmt.Printf("Service: %s\n", name)
func (a *App) ServiceName() string {
	return a.config.serviceName
}

// ServiceVersion returns the configured service version.
//
// Example:
//
//	version := app.ServiceVersion()
//	fmt.Printf("Version: %s\n", version)
func (a *App) ServiceVersion() string {
	return a.config.serviceVersion
}

// Environment returns the current environment (development or production).
//
// Example:
//
//	if app.Environment() == "production" {
//	    // Enable production-only features
//	}
func (a *App) Environment() string {
	return a.config.environment
}

// Metrics returns the metrics configuration if enabled.
// Metrics returns nil if metrics are not enabled.
//
// Example:
//
//	if cfg := app.Metrics(); cfg != nil {
//	    // Access metrics configuration
//	}
func (a *App) Metrics() *metrics.Config {
	return a.metrics
}

// Tracing returns the tracing configuration if enabled.
// Tracing returns nil if tracing is not enabled.
//
// Example:
//
//	if cfg := app.Tracing(); cfg != nil {
//	    // Access tracing configuration
//	}
func (a *App) Tracing() *tracing.Config {
	return a.tracing
}

// Route retrieves a route by name.
// Route returns the route and true if found, false otherwise.
// Route panics if the router is not frozen (call after app.Run() or app.Router().Freeze()).
//
// Example:
//
//	route, ok := app.Route("users.get")
//	if ok {
//	    fmt.Printf("Route: %s %s\n", route.Method(), route.Path())
//	}
func (a *App) Route(name string) (*router.Route, bool) {
	return a.router.GetRoute(name)
}

// Routes returns an immutable snapshot of all named routes.
// Routes panics if the router is not frozen (call after app.Run() or app.Router().Freeze()).
//
// Example:
//
//	routes := app.Routes()
//	for _, route := range routes {
//	    fmt.Printf("%s: %s %s\n", route.Name(), route.Method(), route.Path())
//	}
func (a *App) Routes() []*router.Route {
	return a.router.GetRoutes()
}

// URLFor generates a URL from a route name and parameters.
// URLFor returns an error if the route is not found or if required parameters are missing.
//
// Example:
//
//	url, err := app.URLFor("users.get", map[string]string{"id": "123"}, nil)
//	// Returns: "/users/123", nil
func (a *App) URLFor(routeName string, params map[string]string, query map[string][]string) (string, error) {
	vals := url.Values(query)
	if vals == nil {
		vals = make(url.Values)
	}
	return a.router.URLFor(routeName, params, vals)
}

// MustURLFor generates a URL from a route name and parameters, panicking on error.
// MustURLFor should be used when you're certain the route exists and all parameters are provided.
//
// Example:
//
//	url := app.MustURLFor("users.get", map[string]string{"id": "123"}, nil)
//	// Returns: "/users/123"
func (a *App) MustURLFor(routeName string, params map[string]string, query map[string][]string) string {
	vals := url.Values(query)
	if vals == nil {
		vals = make(url.Values)
	}
	return a.router.MustURLFor(routeName, params, vals)
}

// BaseLogger returns the application's base logger without request-specific context.
// BaseLogger should be used for background jobs, startup/shutdown logging, or other non-request contexts.
//
// BaseLogger never returns nil - if no logger is configured, a no-op logger is returned.
//
// Example:
//
//	app := app.New(...)
//	app.BaseLogger().Info("application started",
//	    slog.String("port", "8080"),
//	    slog.String("environment", "production"),
//	)
//
//	// Background job
//	go func() {
//	    app.BaseLogger().Info("background job started")
//	    // ... do work
//	}()
func (a *App) BaseLogger() *slog.Logger {
	if a.logging != nil {
		return a.logging.Logger()
	}
	return noopLogger
}
