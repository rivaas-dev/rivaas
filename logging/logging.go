// Package logging provides structured logging for Rivaas using Go's standard
// log/slog package. It supports multiple output formats (JSON, text, console),
// automatic sensitive data redaction, and seamless OpenTelemetry integration.
//
// The package follows a functional options pattern for configuration and
// integrates cleanly with the Rivaas router, metrics, and tracing packages.
//
// Basic usage:
//
//	logger := logging.MustNew(logging.WithConsoleHandler())
//	logger.Info("service started", "port", 8080)
//
// With HTTP middleware:
//
//	mw := logging.Middleware(logger,
//	    logging.WithSkipPaths("/health", "/metrics"),
//	)
//	http.Handle("/", mw(handler))
//
// With router integration:
//
//	r := router.New(
//	    logging.WithLogging(
//	        logging.WithJSONHandler(),
//	        logging.WithDebugLevel(),
//	    ),
//	)
//
// Sensitive data (password, token, secret, api_key, authorization) is
// automatically redacted from all log output. Additional sanitization can be
// configured using WithReplaceAttr.
//
// See the README for more examples and configuration options.
package logging

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// HandlerType represents the type of logging handler.
type HandlerType string

const (
	// JSONHandler outputs structured JSON logs.
	JSONHandler HandlerType = "json"
	// TextHandler outputs key=value text logs.
	TextHandler HandlerType = "text"
	// ConsoleHandler outputs human-readable colored logs.
	ConsoleHandler HandlerType = "console"
)

// Level represents log level.
type Level = slog.Level

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// Logger is an interface for structured logging.
// This interface is compatible with slog and other structured loggers.
// It provides standard logging methods with structured key-value attributes.
type Logger interface {
	// Debug logs a debug message with structured attributes
	Debug(msg string, args ...any)

	// Info logs an informational message with structured attributes
	Info(msg string, args ...any)

	// Warn logs a warning message with structured attributes
	Warn(msg string, args ...any)

	// Error logs an error message with structured attributes
	Error(msg string, args ...any)
}

// Package-level cached context to avoid repeated allocations.
// context.Background() is safe to reuse across goroutines.
var bgCtx = context.Background()

// logAttrPool provides pooled attribute slices for convenience methods.
// This reduces allocations in LogRequest, LogError, and LogDuration.
var logAttrPool = sync.Pool{
	New: func() any {
		s := make([]any, 0, 16)
		return &s
	},
}

// SamplingConfig configures log sampling to reduce volume.
type SamplingConfig struct {
	Initial    int           // Log first N occurrences
	Thereafter int           // Then log 1 of every M
	Tick       time.Duration // Reset counters every interval
}

// Config holds the logging configuration.
type Config struct {
	// Handler configuration
	handlerType HandlerType
	output      io.Writer
	level       Level

	// Service information
	serviceName    string
	serviceVersion string
	environment    string

	// Features
	addSource   bool
	debugMode   bool
	replaceAttr func(groups []string, a slog.Attr) slog.Attr

	// Sampling
	samplingConfig *SamplingConfig
	sampleCounter  atomic.Int64
	sampleTicker   *time.Ticker
	sampleStop     chan struct{}

	// Custom logger
	customLogger *slog.Logger
	useCustom    bool

	// Internal state
	logger         atomic.Pointer[slog.Logger] // Lock-free logger access
	mu             sync.Mutex                  // Only for initialization, not hot path
	isShuttingDown atomic.Bool                 // Use atomic for fast shutdown check

	// Global registration control
	registerGlobal bool // If true, sets slog.SetDefault()
}

// Option is a functional option for configuring the logger.
type Option func(*Config)

// defaultConfig returns the default configuration.
func defaultConfig() *Config {
	return &Config{
		handlerType:    JSONHandler,
		output:         os.Stdout,
		level:          LevelInfo,
		serviceName:    "rivaas",
		serviceVersion: "unknown",
		environment:    "development",
		addSource:      false,
		debugMode:      false,
		registerGlobal: false, // Default: no global registration
	}
}

// New creates a new logging configuration.
//
// By default, this function does NOT set the global slog default logger.
// Use WithGlobalLogger() if you want to register this logger as the global default.
//
// This allows multiple logging configurations to coexist in the same process,
// and makes it easier to integrate Rivaas into larger binaries that already
// manage their own global logger.
func New(opts ...Option) (*Config, error) {
	cfg := defaultConfig()
	cfg.readFromEnv()

	for _, opt := range opts {
		opt(cfg)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	if err := cfg.initialize(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// MustNew creates a new logging configuration or panics on error.
func MustNew(opts ...Option) *Config {
	cfg, err := New(opts...)
	if err != nil {
		panic("logging initialization failed: " + err.Error())
	}
	return cfg
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.output == nil {
		return errors.New("output writer cannot be nil")
	}

	if c.serviceName == "" {
		return errors.New("service name cannot be empty")
	}

	if c.useCustom && c.customLogger == nil {
		return ErrNilLogger
	}

	if c.samplingConfig != nil {
		if c.samplingConfig.Initial < 0 || c.samplingConfig.Thereafter < 0 {
			return errors.New("sampling config values must be non-negative")
		}
	}

	return nil
}

// readFromEnv reads configuration from environment variables.
func (c *Config) readFromEnv() {
	// OTEL_SERVICE_NAME
	if serviceName := os.Getenv("OTEL_SERVICE_NAME"); serviceName != "" {
		c.serviceName = serviceName
	}

	// OTEL_SERVICE_VERSION
	if serviceVersion := os.Getenv("OTEL_SERVICE_VERSION"); serviceVersion != "" {
		c.serviceVersion = serviceVersion
	}

	// RIVAAS_ENVIRONMENT
	if environment := os.Getenv("RIVAAS_ENVIRONMENT"); environment != "" {
		c.environment = environment
	}
}

// initialize sets up the logger with the configured handler.
func (c *Config) initialize() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.initializeHandler(); err != nil {
		return err
	}

	// Start sampling ticker if configured
	if c.samplingConfig != nil && c.samplingConfig.Tick > 0 {
		c.sampleStop = make(chan struct{})
		c.sampleTicker = time.NewTicker(c.samplingConfig.Tick)
		go c.samplingResetter()
	}

	return nil
}

// samplingResetter resets the sampling counter periodically.
func (c *Config) samplingResetter() {
	for {
		select {
		case <-c.sampleTicker.C:
			c.sampleCounter.Store(0)
		case <-c.sampleStop:
			return
		}
	}
}

// shouldSample determines if a log entry should be sampled.
// Errors (level >= ERROR) are always logged to ensure critical
// issues are never dropped, even under high load scenarios.
// Lower severity levels (DEBUG, INFO, WARN) respect sampling
// configuration to manage log volume.
func (c *Config) shouldSample(level slog.Level) bool {
	// Always log errors - critical issues should never be sampled
	if level >= slog.LevelError {
		return true
	}

	if c.samplingConfig == nil {
		return true
	}

	count := c.sampleCounter.Add(1)
	if count <= int64(c.samplingConfig.Initial) {
		return true
	}

	// If Thereafter is 0, sample everything after initial
	if c.samplingConfig.Thereafter == 0 {
		return true
	}

	return (count-int64(c.samplingConfig.Initial))%int64(c.samplingConfig.Thereafter) == 0
}

// initializeHandler creates and sets the handler (must be called with lock held).
func (c *Config) initializeHandler() error {
	if c.useCustom {
		if c.customLogger == nil {
			return ErrNilLogger
		}
		c.logger.Store(c.customLogger)
		if c.registerGlobal {
			slog.SetDefault(c.customLogger)
		}
		return nil
	}

	opts := &slog.HandlerOptions{
		Level:       c.level,
		AddSource:   c.addSource,
		ReplaceAttr: c.buildReplaceAttr(),
	}

	var handler slog.Handler
	switch c.handlerType {
	case JSONHandler:
		handler = slog.NewJSONHandler(c.output, opts)
	case TextHandler:
		handler = slog.NewTextHandler(c.output, opts)
	case ConsoleHandler:
		handler = newConsoleHandler(c.output, opts)
	default:
		return fmt.Errorf("%w: %s", ErrInvalidHandler, c.handlerType)
	}

	newLogger := slog.New(handler)
	c.logger.Store(newLogger)
	if c.registerGlobal {
		slog.SetDefault(newLogger)
	}
	return nil
}

// buildReplaceAttr creates the attribute replacer function.
func (c *Config) buildReplaceAttr() func(groups []string, a slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		// Sanitize sensitive fields
		switch a.Key {
		case "password", "token", "secret", "api_key", "authorization":
			return slog.String(a.Key, "***REDACTED***")
		}
		// Call user-defined replacer if provided
		if c.replaceAttr != nil {
			return c.replaceAttr(groups, a)
		}
		return a
	}
}

// Logger returns the underlying slog.Logger.
// This method is lock-free and safe for concurrent access.
func (c *Config) Logger() *slog.Logger {
	return c.logger.Load()
}

// With returns a logger with additional attributes.
func (c *Config) With(args ...any) *slog.Logger {
	return c.Logger().With(args...)
}

// WithGroup returns a logger with a group name.
func (c *Config) WithGroup(name string) *slog.Logger {
	return c.Logger().WithGroup(name)
}

// log is the internal helper method that handles common logging logic.
func (c *Config) log(level slog.Level, msg string, args ...any) {
	if c.isShuttingDown.Load() {
		return
	}

	logger := c.Logger()

	// Check if level is enabled
	if !logger.Enabled(bgCtx, level) {
		return
	}

	if !c.shouldSample(level) {
		return
	}

	logger.Log(bgCtx, level, msg, args...)
}

// Debug logs a debug message.
func (c *Config) Debug(msg string, args ...any) {
	c.log(slog.LevelDebug, msg, args...)
}

// Info logs an info message.
func (c *Config) Info(msg string, args ...any) {
	c.log(slog.LevelInfo, msg, args...)
}

// Warn logs a warning message.
func (c *Config) Warn(msg string, args ...any) {
	c.log(slog.LevelWarn, msg, args...)
}

// Error logs an error message.
func (c *Config) Error(msg string, args ...any) {
	c.log(slog.LevelError, msg, args...)
}

// ErrorWithStack logs an error with optional stack trace.
//
// Performance note: Stack capture is expensive (~100-200x slower than regular logging).
// Only enable stack traces for critical errors or debugging, not for high-frequency error logging.
// Stack capture involves runtime.Callers (~1-2µs) and frame formatting.
func (c *Config) ErrorWithStack(msg string, err error, includeStack bool, extra ...any) {
	if c.isShuttingDown.Load() {
		return
	}

	attrsPtr := logAttrPool.Get().(*[]any)
	attrs := (*attrsPtr)[:0]
	defer func() {
		*attrsPtr = (*attrsPtr)[:0]
		logAttrPool.Put(attrsPtr)
	}()

	attrs = append(attrs, "error", err.Error())

	if includeStack {
		attrs = append(attrs, "stack", captureStack(3))
	}

	attrs = append(attrs, extra...)

	c.log(slog.LevelError, msg, attrs...)
}

// captureStack captures a lightweight stack trace.
func captureStack(skip int) string {
	var buf strings.Builder
	pcs := make([]uintptr, 10)
	n := runtime.Callers(skip, pcs)
	frames := runtime.CallersFrames(pcs[:n])

	for {
		frame, more := frames.Next()
		fmt.Fprintf(&buf, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}
	return buf.String()
}

// Shutdown gracefully shuts down the logger.
func (c *Config) Shutdown(ctx context.Context) error {
	c.isShuttingDown.Store(true)

	// Stop sampling ticker if running
	if c.sampleTicker != nil {
		c.sampleTicker.Stop()
		close(c.sampleStop)
	}

	logger := c.Logger()
	if logger != nil {
		if flusher, ok := logger.Handler().(interface{ Flush() error }); ok {
			return flusher.Flush()
		}
	}
	return nil
}

// SetLevel dynamically changes the minimum log level at runtime.
// This is useful for enabling debug logging temporarily without restart.
// Returns ErrCannotChangeLevel if using a custom logger.
//
// Example:
//
//	// Enable debug logging via HTTP endpoint
//	http.HandleFunc("/debug/loglevel", func(w http.ResponseWriter, r *http.Request) {
//	    level := r.URL.Query().Get("level")
//	    if level == "debug" {
//	        logger.SetLevel(logging.LevelDebug)
//	    }
//	})
func (c *Config) SetLevel(level Level) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.useCustom {
		return ErrCannotChangeLevel
	}

	oldLevel := c.level
	c.level = level

	// Reinitialize handler with new level
	if err := c.initializeHandler(); err != nil {
		c.level = oldLevel // Rollback on error
		return err
	}

	return nil
}

// Level returns the current minimum log level.
func (c *Config) Level() Level {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.level
}

// ServiceName returns the service name.
func (c *Config) ServiceName() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.serviceName
}

// ServiceVersion returns the service version.
func (c *Config) ServiceVersion() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.serviceVersion
}

// Environment returns the environment.
func (c *Config) Environment() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.environment
}

// IsEnabled returns true if logging is enabled and not shutting down.
func (c *Config) IsEnabled() bool {
	return !c.isShuttingDown.Load()
}

// DebugInfo returns diagnostic information about the logger.
func (c *Config) DebugInfo() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()

	info := map[string]any{
		"handler_type":    string(c.handlerType),
		"level":           c.level.String(),
		"service_name":    c.serviceName,
		"service_version": c.serviceVersion,
		"environment":     c.environment,
		"add_source":      c.addSource,
		"debug_mode":      c.debugMode,
		"is_custom":       c.useCustom,
		"is_shutdown":     c.isShuttingDown.Load(),
	}

	if c.samplingConfig != nil {
		info["sampling"] = map[string]any{
			"initial":    c.samplingConfig.Initial,
			"thereafter": c.samplingConfig.Thereafter,
			"tick":       c.samplingConfig.Tick.String(),
			"counter":    c.sampleCounter.Load(),
		}
	}

	return info
}

// Convenience Methods

// LogRequest logs an HTTP request with standard fields.
// This is a convenience method for common HTTP request logging.
//
// Example:
//
//	logger.LogRequest(r, "status", 200, "duration_ms", 45)
func (c *Config) LogRequest(r *http.Request, extra ...any) {
	if c.isShuttingDown.Load() {
		return
	}

	attrsPtr := logAttrPool.Get().(*[]any)
	attrs := (*attrsPtr)[:0]
	defer func() {
		*attrsPtr = (*attrsPtr)[:0]
		logAttrPool.Put(attrsPtr)
	}()

	attrs = append(attrs,
		"method", r.Method,
		"path", r.URL.Path,
		"remote", r.RemoteAddr,
		"user_agent", r.UserAgent(),
	)
	if r.URL.RawQuery != "" {
		attrs = append(attrs, "query", r.URL.RawQuery)
	}
	attrs = append(attrs, extra...)
	c.Info("http request", attrs...)
}

// LogError logs an error with additional context.
// This is a convenience method for error logging with structured fields.
//
// Example:
//
//	logger.LogError(err, "database operation failed",
//	    "operation", "INSERT",
//	    "table", "users",
//	    "retry_count", 3,
//	)
func (c *Config) LogError(err error, msg string, extra ...any) {
	if c.isShuttingDown.Load() {
		return
	}

	attrsPtr := logAttrPool.Get().(*[]any)
	attrs := (*attrsPtr)[:0]
	defer func() {
		*attrsPtr = (*attrsPtr)[:0]
		logAttrPool.Put(attrsPtr)
	}()

	attrs = append(attrs, "error", err.Error())
	attrs = append(attrs, extra...)
	c.Error(msg, attrs...)
}

// LogDuration logs an operation duration.
// This is a convenience method for timing operations.
//
// Example:
//
//	start := time.Now()
//	// ... do work ...
//	logger.LogDuration("operation completed", start, "rows_processed", count)
func (c *Config) LogDuration(msg string, start time.Time, extra ...any) {
	if c.isShuttingDown.Load() {
		return
	}

	duration := time.Since(start)
	attrsPtr := logAttrPool.Get().(*[]any)
	attrs := (*attrsPtr)[:0]
	defer func() {
		*attrsPtr = (*attrsPtr)[:0]
		logAttrPool.Put(attrsPtr)
	}()

	attrs = append(attrs,
		"duration_ms", duration.Milliseconds(),
		"duration", duration.String(),
	)
	attrs = append(attrs, extra...)
	c.Info(msg, attrs...)
}

// Functional Options

// WithHandlerType sets the logging handler type.
func WithHandlerType(t HandlerType) Option {
	return func(c *Config) { c.handlerType = t }
}

// WithJSONHandler uses JSON structured logging (default).
func WithJSONHandler() Option {
	return WithHandlerType(JSONHandler)
}

// WithTextHandler uses text key=value logging.
func WithTextHandler() Option {
	return WithHandlerType(TextHandler)
}

// WithConsoleHandler uses human-readable console logging.
func WithConsoleHandler() Option {
	return WithHandlerType(ConsoleHandler)
}

// WithOutput sets the output writer.
func WithOutput(w io.Writer) Option {
	return func(c *Config) { c.output = w }
}

// WithLevel sets the minimum log level.
func WithLevel(l Level) Option {
	return func(c *Config) { c.level = l }
}

// WithDebugLevel enables debug logging.
func WithDebugLevel() Option {
	return WithLevel(LevelDebug)
}

// WithServiceName sets the service name.
func WithServiceName(name string) Option {
	return func(c *Config) {
		if name != "" {
			c.serviceName = name
		}
	}
}

// WithServiceVersion sets the service version.
func WithServiceVersion(version string) Option {
	return func(c *Config) {
		if version != "" {
			c.serviceVersion = version
		}
	}
}

// WithEnvironment sets the environment.
func WithEnvironment(env string) Option {
	return func(c *Config) {
		if env != "" {
			c.environment = env
		}
	}
}

// WithServiceInfo sets service metadata.
// Deprecated: Use WithServiceName, WithServiceVersion, and WithEnvironment instead.
func WithServiceInfo(name, version, env string) Option {
	return func(c *Config) {
		if name != "" {
			c.serviceName = name
		}
		if version != "" {
			c.serviceVersion = version
		}
		if env != "" {
			c.environment = env
		}
	}
}

// WithSource enables source code location in logs.
func WithSource(enabled bool) Option {
	return func(c *Config) { c.addSource = enabled }
}

// WithDebugMode enables verbose debugging information.
func WithDebugMode(enabled bool) Option {
	return func(c *Config) {
		c.debugMode = enabled
		if enabled {
			// Auto-enable source, debug level for diagnostics
			c.addSource = true
			c.level = LevelDebug
		}
	}
}

// WithReplaceAttr sets a custom attribute replacer.
func WithReplaceAttr(fn func(groups []string, a slog.Attr) slog.Attr) Option {
	return func(c *Config) { c.replaceAttr = fn }
}

// WithCustomLogger uses a custom slog.Logger.
func WithCustomLogger(l *slog.Logger) Option {
	return func(c *Config) {
		c.customLogger = l
		c.useCustom = true
	}
}

// WithGlobalLogger registers this logger as the global slog default logger.
// By default, loggers are not registered globally to allow multiple logger
// instances to coexist in the same process.
//
// Example:
//
//	logger := logging.New(
//	    logging.WithJSONHandler(),
//	    logging.WithGlobalLogger(), // Register as global default
//	)
func WithGlobalLogger() Option {
	return func(c *Config) {
		c.registerGlobal = true
	}
}

// WithSampling enables log sampling to reduce volume in high-traffic scenarios.
func WithSampling(cfg SamplingConfig) Option {
	return func(c *Config) {
		c.samplingConfig = &cfg
	}
}

// Helper functions

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Example demonstrates basic usage.
func Example() {
	logger := MustNew(
		WithConsoleHandler(),
		WithDebugLevel(),
	)

	logger.Info("service started", "port", 8080)
	logger.Debug("debugging info", "key", "value")

	// Output:
	// INFO  service started port=8080
	// DEBUG debugging info key=value
}

// Example_contextLogging demonstrates context-aware logging.
func Example_contextLogging() {
	logger := MustNew(WithJSONHandler())

	// Simulate traced context
	ctx := context.Background()
	cl := NewContextLogger(logger, ctx)

	cl.Info("processing request", "user_id", "123")
}

// Example_errorWithStack demonstrates error logging with stack traces.
func Example_errorWithStack() {
	logger := MustNew(WithConsoleHandler())

	err := errors.New("database connection failed")
	logger.ErrorWithStack("critical error", err, true,
		"database", "postgres",
		"host", "localhost",
	)
}

// NewTestLogger creates a logger for testing with in-memory buffer.
func NewTestLogger() (*Config, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	logger := MustNew(
		WithJSONHandler(),
		WithOutput(buf),
		WithLevel(LevelDebug),
	)
	return logger, buf
}

// LogEntry represents a parsed log entry for testing.
type LogEntry struct {
	Time    time.Time
	Level   string
	Message string
	Attrs   map[string]any
}

// ParseJSONLogEntries parses JSON log entries from buffer.
// It creates a copy of the buffer so the original is not consumed.
func ParseJSONLogEntries(buf *bytes.Buffer) ([]LogEntry, error) {
	// Create a copy to avoid consuming the original buffer
	data := buf.Bytes()
	reader := bytes.NewReader(data)

	var entries []LogEntry
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		var entry map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, err
		}

		le := LogEntry{
			Message: entry["msg"].(string),
			Level:   entry["level"].(string),
			Attrs:   make(map[string]any),
		}

		for k, v := range entry {
			if k != "time" && k != "level" && k != "msg" {
				le.Attrs[k] = v
			}
		}

		entries = append(entries, le)
	}
	return entries, scanner.Err()
}
