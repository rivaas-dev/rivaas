package logger

import "io"

// WithOutput sets the output writer for logs.
// Default: os.Stdout
//
// Example:
//
//	file, _ := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
//	logger.New(logger.WithOutput(file))
func WithOutput(output io.Writer) Option {
	return func(cfg *config) {
		cfg.output = output
	}
}

// WithTimeFormat sets the time format for log timestamps.
// Default: "2006/01/02 15:04:05"
//
// Example:
//
//	logger.New(logger.WithTimeFormat(time.RFC3339))
func WithTimeFormat(format string) Option {
	return func(cfg *config) {
		cfg.timeFormat = format
	}
}

// WithSkipPaths sets paths that should not be logged.
// Useful for health check endpoints that create log noise.
//
// Example:
//
//	logger.New(logger.WithSkipPaths("/health", "/metrics"))
func WithSkipPaths(paths ...string) Option {
	return func(cfg *config) {
		for _, path := range paths {
			cfg.skipPaths[path] = true
		}
	}
}

// WithFormatter sets a custom log formatter function.
//
// Example:
//
//	logger.New(logger.WithFormatter(func(params FormatterParams) string {
//	    return fmt.Sprintf("%s - %s %s %d", params.ClientIP, params.Method, params.Path, params.StatusCode)
//	}))
func WithFormatter(formatter func(FormatterParams) string) Option {
	return func(cfg *config) {
		cfg.formatter = formatter
	}
}

// WithColors enables colored output for terminal logging.
// Colors are based on HTTP status code ranges.
// Default: false
//
// Example:
//
//	logger.New(logger.WithColors(true))
func WithColors(enabled bool) Option {
	return func(cfg *config) {
		cfg.enableColors = enabled
		if enabled {
			cfg.formatter = coloredFormatter
		}
	}
}
