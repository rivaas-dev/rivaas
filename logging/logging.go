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
	// LevelDebug is the debug log level.
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

// Package-level cached context reused across log calls.
//
// We reuse context.Background() because:
//   - It's immutable and safe for concurrent access across all goroutines
//   - slog.Logger.Log() requires a context but we don't use it for cancellation
var bgCtx = context.Background()

// logAttrPool provides pooled attribute slices for convenience methods.
//
// LogRequest, LogError, and LogDuration need temporary slices to build
// attribute lists. The pool reuses slices across calls. Initial capacity
// of 16 is chosen to fit most typical log entries without requiring
// slice growth.
var logAttrPool = sync.Pool{
	New: func() any {
		s := make([]any, 0, 16)
		return &s
	},
}

// SamplingConfig configures log sampling to reduce volume in high-traffic scenarios.
//
// Sampling algorithm:
//  1. Log the first 'Initial' entries unconditionally (e.g., first 100)
//  2. After that, log 1 in every 'Thereafter' entries (e.g., 1 in 100)
//  3. Reset the counter every 'Tick' interval to avoid indefinite accumulation
//
// Example: Initial=100, Thereafter=100, Tick=1m means:
//   - Always log first 100 entries
//   - Then log 1% of entries (1 in 100)
//   - Every minute, reset counter (log next 100 again)
//
// This ensures you always see some recent activity while managing log volume.
type SamplingConfig struct {
	Initial    int           // Log first N occurrences unconditionally
	Thereafter int           // After Initial, log 1 of every M entries (0 = log all)
	Tick       time.Duration // Reset sampling counter every interval (0 = never reset)
}

// Config holds the logging configuration.
//
// Thread-safety: All public methods are safe for concurrent use.
//   - logger field uses atomic.Pointer for lock-free read access
//   - mu protects initialization and reconfiguration only
//   - isShuttingDown uses atomic.Bool for shutdown checks
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
	mu             sync.Mutex                  // Protects initialization/reconfiguration only
	isShuttingDown atomic.Bool                 // Shutdown check without mutex

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

// shouldSample determines if a log entry should be sampled based on the configured policy.
//
// Sampling algorithm (with example Initial=100, Thereafter=100):
//  1. Atomically increment counter
//  2. If count <= 100 (Initial): always log
//  3. If count > 100: log only if (count-100) % 100 == 0
//
// Special cases:
//   - Errors (level >= ERROR) bypass sampling to never drop critical issues
//   - If no sampling configured, always returns true
//   - If Thereafter=0, logs everything after Initial
//
// Thread-safety: Uses atomic counter, safe for concurrent calls.
//
// Why always log errors: Critical errors must be investigated. Sampling them
// could hide production incidents. Lower-severity logs (INFO, DEBUG) are
// safe to sample since they're typically for observability, not alerting.
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
//
// Thread-safety: This method is safe for concurrent access.
// Uses atomic.Pointer for thread-safe logger access.
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
//
// This method consolidates:
//   - Shutdown check (atomic.Bool load)
//   - Level check (via slog.Logger.Enabled)
//   - Sampling decision
//
// Why centralized: Ensures consistent behavior across Debug/Info/Warn/Error.
// Single code path makes it easier to add features (e.g., rate limiting).
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

// Debug logs a debug message with structured attributes.
// Thread-safe and safe to call concurrently.
func (c *Config) Debug(msg string, args ...any) {
	c.log(slog.LevelDebug, msg, args...)
}

// Info logs an informational message with structured attributes.
// Thread-safe and safe to call concurrently.
func (c *Config) Info(msg string, args ...any) {
	c.log(slog.LevelInfo, msg, args...)
}

// Warn logs a warning message with structured attributes.
// Thread-safe and safe to call concurrently.
func (c *Config) Warn(msg string, args ...any) {
	c.log(slog.LevelWarn, msg, args...)
}

// Error logs an error message with structured attributes.
// Thread-safe and safe to call concurrently.
// Note: Errors bypass sampling and are always logged.
func (c *Config) Error(msg string, args ...any) {
	c.log(slog.LevelError, msg, args...)
}

// ErrorWithStack logs an error with optional stack trace.
//
// When to use stack traces:
//
//	✓ Critical errors that require debugging
//	✓ Unexpected error conditions (panics, invariant violations)
//	✗ Expected errors (validation failures, not found)
//	✗ High-frequency errors where stack capture cost is undesirable
//
// Thread-safe and safe to call concurrently.
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

// captureStack captures a stack trace.
//
// Skip parameter: Number of stack frames to skip.
//   - 0: includes captureStack itself
//   - 3: typical value to skip captureStack, ErrorWithStack, and caller's caller
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
func (c *Config) Shutdown(_ context.Context) error {
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
//
// Use cases:
//   - Enable debug logging temporarily for troubleshooting
//   - Reduce log volume during high traffic periods
//   - Runtime configuration via HTTP endpoint or signal handler
//
// Limitations:
//   - Not supported with custom loggers (returns ErrCannotChangeLevel)
//   - Brief initialization window where old/new levels may race
//
// Thread-safety: Uses mutex to serialize level changes. Safe to call
// concurrently, but multiple SetLevel calls will serialize.
//
// Example:
//
//	// Enable debug logging via HTTP endpoint
//	http.HandleFunc("/debug/loglevel", func(w http.ResponseWriter, r *http.Request) {
//	    level := r.URL.Query().Get("level")
//	    switch level {
//	    case "debug":
//	        if err := logger.SetLevel(logging.LevelDebug); err != nil {
//	            http.Error(w, err.Error(), http.StatusInternalServerError)
//	            return
//	        }
//	    case "info":
//	        logger.SetLevel(logging.LevelInfo)
//	    }
//	    w.WriteHeader(http.StatusOK)
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
//
// Standard fields included:
//   - method: HTTP method (GET, POST, etc.)
//   - path: Request path (without query string)
//   - remote: Client remote address
//   - user_agent: Client User-Agent header
//   - query: Query string (only if non-empty)
//
// Additional fields can be passed via 'extra' (e.g., "status", 200, "duration_ms", 45).
//
// Thread-safe and safe to call concurrently.
//
// Example:
//
//	logger.LogRequest(r, "status", 200, "duration_ms", 45, "bytes", 1024)
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

// LogError logs an error with additional context fields.
//
// Why use this instead of Error():
//   - Automatically includes "error" field with error message
//   - Convenient for error handling patterns
//   - Consistent error logging format across codebase
//
// Thread-safe and safe to call concurrently.
//
// Example:
//
//	if err := db.Insert(user); err != nil {
//	    logger.LogError(err, "database operation failed",
//	        "operation", "INSERT",
//	        "table", "users",
//	        "retry_count", 3,
//	    )
//	    return err
//	}
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

// LogDuration logs an operation duration with timing information.
//
// Automatically includes:
//   - duration_ms: Duration in milliseconds (for easy filtering/alerting)
//   - duration: Human-readable duration string (e.g., "1.5s", "250ms")
//
// Thread-safe and safe to call concurrently.
//
// Example:
//
//	start := time.Now()
//	result, err := processData(data)
//	logger.LogDuration("data processing completed", start,
//	    "rows_processed", result.Count,
//	    "errors", result.Errors,
//	)
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

// ExampleContextLogging demonstrates context-aware logging.
func ExampleContextLogging() {
	logger := MustNew(WithJSONHandler())

	// Simulate traced context
	ctx := context.Background()
	cl := NewContextLogger(ctx, logger)

	cl.Info("processing request", "user_id", "123")
}

// ExampleErrorWithStack demonstrates error logging with stack traces.
func ExampleErrorWithStack() {
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
