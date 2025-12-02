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
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
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

// Package-level cached context reused across log calls.
//
// We reuse context.Background() because:
//   - It's immutable and safe for concurrent access across all goroutines
//   - slog.Logger.Log() requires a context but we don't use it for cancellation
var bgCtx = context.Background()

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

// Logger is the main logging type that provides structured logging capabilities.
//
// Thread-safety: All public methods are safe for concurrent use.
// The slogger field is accessed atomically, while mu protects initialization
// and reconfiguration operations.
type Logger struct {
	// Handler configuration
	handlerType HandlerType
	output      io.Writer
	level       Level

	// Service information (immutable after initialization)
	// These are automatically added to every log entry
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
	slogger        atomic.Pointer[slog.Logger] // Lock-free slog.Logger access
	mu             sync.Mutex                  // Protects initialization/reconfiguration only
	isShuttingDown atomic.Bool                 // Shutdown check without mutex

	// Global registration control
	registerGlobal bool // If true, sets slog.SetDefault()
}

// Option is a functional option for configuring the logger.
type Option func(*Logger)

// defaultLogger returns a Logger with default configuration.
func defaultLogger() *Logger {
	return &Logger{
		handlerType:    JSONHandler,
		output:         os.Stdout,
		level:          LevelInfo,
		serviceName:    "",
		serviceVersion: "",
		environment:    "",
		addSource:      false,
		debugMode:      false,
		registerGlobal: false, // Default: no global registration
	}
}

// New creates a new Logger with the given options.
//
// By default, this function does NOT set the global slog default logger.
// Use WithGlobalLogger() if you want to register this logger as the global default.
//
// This allows multiple Logger instances to coexist in the same process,
// and makes it easier to integrate Rivaas into larger binaries that already
// manage their own global logger.
func New(opts ...Option) (*Logger, error) {
	l := defaultLogger()

	for _, opt := range opts {
		opt(l)
	}

	if err := l.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	if err := l.initialize(); err != nil {
		return nil, err
	}
	return l, nil
}

// MustNew creates a new Logger or panics on error.
func MustNew(opts ...Option) *Logger {
	l, err := New(opts...)
	if err != nil {
		panic("logging initialization failed: " + err.Error())
	}
	return l
}

// Validate checks if the configuration is valid.
func (l *Logger) Validate() error {
	if l.output == nil {
		return errors.New("output writer cannot be nil")
	}

	if l.useCustom && l.customLogger == nil {
		return ErrNilLogger
	}

	if l.samplingConfig != nil {
		if l.samplingConfig.Initial < 0 || l.samplingConfig.Thereafter < 0 {
			return errors.New("sampling config values must be non-negative")
		}
	}

	return nil
}

// initialize sets up the logger with the configured handler.
func (l *Logger) initialize() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.initializeHandler(); err != nil {
		return err
	}

	// Start sampling ticker if configured
	if l.samplingConfig != nil && l.samplingConfig.Tick > 0 {
		l.sampleStop = make(chan struct{})
		l.sampleTicker = time.NewTicker(l.samplingConfig.Tick)
		go l.samplingResetter()
	}

	return nil
}

// samplingResetter resets the sampling counter periodically.
func (l *Logger) samplingResetter() {
	for {
		select {
		case <-l.sampleTicker.C:
			l.sampleCounter.Store(0)
		case <-l.sampleStop:
			return
		}
	}
}

// shouldSample determines if a log entry should be sampled based on the configured policy.
//
// Sampling behavior:
//   - Logs the first [SamplingConfig.Initial] entries unconditionally
//   - After that, logs 1 in every [SamplingConfig.Thereafter] entries
//   - Resets the counter every [SamplingConfig.Tick] interval
//
// Special cases:
//   - Errors (level >= ERROR) bypass sampling to ensure critical issues are never dropped
//   - If no sampling configured, returns true (all entries logged)
//   - If Thereafter is 0, logs everything after Initial
//
// Thread-safe: Safe for concurrent calls from multiple goroutines.
func (l *Logger) shouldSample(level slog.Level) bool {
	// Always log errors - critical issues should never be sampled
	if level >= slog.LevelError {
		return true
	}

	if l.samplingConfig == nil {
		return true
	}

	count := l.sampleCounter.Add(1)
	if count <= int64(l.samplingConfig.Initial) {
		return true
	}

	// If Thereafter is 0, sample everything after initial
	if l.samplingConfig.Thereafter == 0 {
		return true
	}

	return (count-int64(l.samplingConfig.Initial))%int64(l.samplingConfig.Thereafter) == 0
}

// initializeHandler creates and sets the handler (must be called with lock held).
func (l *Logger) initializeHandler() error {
	if l.useCustom {
		if l.customLogger == nil {
			return ErrNilLogger
		}
		l.slogger.Store(l.customLogger)
		if l.registerGlobal {
			slog.SetDefault(l.customLogger)
		}
		return nil
	}

	opts := &slog.HandlerOptions{
		Level:       l.level,
		AddSource:   l.addSource,
		ReplaceAttr: l.buildReplaceAttr(),
	}

	var handler slog.Handler
	switch l.handlerType {
	case JSONHandler:
		handler = slog.NewJSONHandler(l.output, opts)
	case TextHandler:
		handler = slog.NewTextHandler(l.output, opts)
	case ConsoleHandler:
		handler = newConsoleHandler(l.output, opts)
	default:
		return fmt.Errorf("%w: %s", ErrInvalidHandler, l.handlerType)
	}

	newLogger := slog.New(handler)

	// Add service metadata as default attributes if configured
	var attrs []any
	if l.serviceName != "" {
		attrs = append(attrs, "service", l.serviceName)
	}
	if l.serviceVersion != "" {
		attrs = append(attrs, "version", l.serviceVersion)
	}
	if l.environment != "" {
		attrs = append(attrs, "env", l.environment)
	}
	if len(attrs) > 0 {
		newLogger = newLogger.With(attrs...)
	}

	l.slogger.Store(newLogger)
	if l.registerGlobal {
		slog.SetDefault(newLogger)
	}
	return nil
}

// buildReplaceAttr creates the attribute replacer function.
func (l *Logger) buildReplaceAttr() func(groups []string, a slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		// Sanitize sensitive fields
		switch a.Key {
		case "password", "token", "secret", "api_key", "authorization":
			return slog.String(a.Key, "***REDACTED***")
		}
		// Call user-defined replacer if provided
		if l.replaceAttr != nil {
			return l.replaceAttr(groups, a)
		}
		return a
	}
}

// Logger returns the underlying [slog.Logger].
// This method is safe for concurrent access.
func (l *Logger) Logger() *slog.Logger {
	return l.slogger.Load()
}

// With returns a [slog.Logger] with additional attributes.
func (l *Logger) With(args ...any) *slog.Logger {
	return l.Logger().With(args...)
}

// WithGroup returns a [slog.Logger] with a group name.
func (l *Logger) WithGroup(name string) *slog.Logger {
	return l.Logger().WithGroup(name)
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
func (l *Logger) log(level slog.Level, msg string, args ...any) {
	if l.isShuttingDown.Load() {
		return
	}

	logger := l.Logger()

	// Check if level is enabled
	if !logger.Enabled(bgCtx, level) {
		return
	}

	if !l.shouldSample(level) {
		return
	}

	logger.Log(bgCtx, level, msg, args...)
}

// Debug logs a debug message with structured attributes.
// Thread-safe and safe to call concurrently.
func (l *Logger) Debug(msg string, args ...any) {
	l.log(slog.LevelDebug, msg, args...)
}

// Info logs an informational message with structured attributes.
// Thread-safe and safe to call concurrently.
func (l *Logger) Info(msg string, args ...any) {
	l.log(slog.LevelInfo, msg, args...)
}

// Warn logs a warning message with structured attributes.
// Thread-safe and safe to call concurrently.
func (l *Logger) Warn(msg string, args ...any) {
	l.log(slog.LevelWarn, msg, args...)
}

// Error logs an error message with structured attributes.
// Thread-safe and safe to call concurrently.
// Note: Errors bypass sampling and are always logged.
func (l *Logger) Error(msg string, args ...any) {
	l.log(slog.LevelError, msg, args...)
}

// Shutdown gracefully shuts down the logger.
func (l *Logger) Shutdown(_ context.Context) error {
	l.isShuttingDown.Store(true)

	// Stop sampling ticker if running
	if l.sampleTicker != nil {
		l.sampleTicker.Stop()
		close(l.sampleStop)
	}

	logger := l.Logger()
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
//   - Not supported with custom loggers (returns [ErrCannotChangeLevel])
//   - Brief initialization window where old/new levels may race
//
// Thread-safe: Safe to call concurrently, but multiple SetLevel calls will serialize.
func (l *Logger) SetLevel(level Level) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.useCustom {
		return ErrCannotChangeLevel
	}

	oldLevel := l.level
	l.level = level

	// Reinitialize handler with new level
	if err := l.initializeHandler(); err != nil {
		l.level = oldLevel // Rollback on error
		return err
	}

	return nil
}

// Level returns the current minimum log level.
// Note: This requires a lock because level can be changed dynamically via SetLevel.
func (l *Logger) Level() Level {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.level
}

// ServiceName returns the service name.
// This field is immutable after initialization, so no lock is needed.
func (l *Logger) ServiceName() string {
	return l.serviceName
}

// ServiceVersion returns the service version.
// This field is immutable after initialization, so no lock is needed.
func (l *Logger) ServiceVersion() string {
	return l.serviceVersion
}

// Environment returns the environment.
// This field is immutable after initialization, so no lock is needed.
func (l *Logger) Environment() string {
	return l.environment
}

// IsEnabled returns true if logging is enabled and not shutting down.
func (l *Logger) IsEnabled() bool {
	return !l.isShuttingDown.Load()
}

// DebugInfo returns diagnostic information about the logger.
func (l *Logger) DebugInfo() map[string]any {
	l.mu.Lock()
	defer l.mu.Unlock()

	info := map[string]any{
		"handler_type":    string(l.handlerType),
		"level":           l.level.String(),
		"service_name":    l.serviceName,
		"service_version": l.serviceVersion,
		"environment":     l.environment,
		"add_source":      l.addSource,
		"debug_mode":      l.debugMode,
		"is_custom":       l.useCustom,
		"is_shutdown":     l.isShuttingDown.Load(),
	}

	if l.samplingConfig != nil {
		info["sampling"] = map[string]any{
			"initial":    l.samplingConfig.Initial,
			"thereafter": l.samplingConfig.Thereafter,
			"tick":       l.samplingConfig.Tick.String(),
			"counter":    l.sampleCounter.Load(),
		}
	}

	return info
}
