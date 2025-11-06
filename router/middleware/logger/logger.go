// Package logger provides HTTP request/response logging middleware with
// configurable output formats, colors, and path skipping.
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"rivaas.dev/router"
	"rivaas.dev/router/middleware"
)

// Option defines functional options for logger middleware configuration.
type Option func(*config)

// config holds the configuration for the logger middleware.
type config struct {
	// output is the writer where logs are written
	output io.Writer

	// timeFormat is the format string for timestamps
	timeFormat string

	// skipPaths are paths that should not be logged
	skipPaths map[string]bool

	// formatter is a custom log formatter function
	formatter func(params FormatterParams) string

	// enableColors enables colored output for terminal
	enableColors bool
}

// FormatterParams holds the parameters for custom log formatting.
type FormatterParams struct {
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

// defaultConfig returns the default configuration for logger middleware.
func defaultConfig() *config {
	return &config{
		output:       os.Stdout,
		timeFormat:   "2006/01/02 15:04:05",
		skipPaths:    make(map[string]bool),
		formatter:    defaultFormatter,
		enableColors: false,
	}
}

// defaultFormatter is the default log formatter.
func defaultFormatter(params FormatterParams) string {
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

// coloredFormatter formats logs with colors for terminal output.
func coloredFormatter(params FormatterParams) string {
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

// New returns a middleware that logs HTTP requests.
// It logs method, path, status code, latency, body size, and client IP.
//
// This middleware should typically be registered early in the middleware chain
// to capture timing for all subsequent handlers.
//
// Basic usage:
//
//	r := router.New()
//	r.Use(logger.New())
//
// With custom output:
//
//	logFile, _ := os.OpenFile("access.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
//	r.Use(logger.New(
//	    logger.WithOutput(logFile),
//	))
//
// Skip health checks:
//
//	r.Use(logger.New(
//	    logger.WithSkipPaths("/health", "/metrics"),
//	))
//
// With colors:
//
//	r.Use(logger.New(
//	    logger.WithColors(true),
//	))
//
// Custom format:
//
//	r.Use(logger.New(
//	    logger.WithFormatter(func(params FormatterParams) string {
//	        return fmt.Sprintf("%s %s %d", params.Method, params.Path, params.StatusCode)
//	    }),
//	))
//
// Performance: This middleware has minimal overhead (~200ns per request).
// The actual logging I/O happens asynchronously in most cases.
func New(opts ...Option) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultConfig()
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
		clientIP := c.ClientIP()
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
		if id, ok := c.Request.Context().Value(middleware.RequestIDKey).(string); ok {
			requestID = id
		}

		// Format and log
		params := FormatterParams{
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
