package logging

import "log/slog"

// Recorder is used by router/app to access logging.
// This interface follows the same pattern as metrics.Recorder and tracing.Recorder.
type Recorder interface {
	Logger() *slog.Logger
	With(args ...any) *slog.Logger
	WithGroup(name string) *slog.Logger
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// RouterOption mirrors metrics/tracing pattern to avoid cycles.
type RouterOption func(interface{})

// WithLogging enables logging on a router.
// The router must implement SetLogger(Logger).
//
// Example:
//
//	r := router.New(
//	    logging.WithLogging(
//	        logging.WithConsoleHandler(),
//	        logging.WithDebugLevel(),
//	    ),
//	)
func WithLogging(opts ...Option) RouterOption {
	return func(router interface{}) {
		cfg := MustNew(opts...)
		if setter, ok := router.(interface{ SetLogger(Logger) }); ok {
			setter.SetLogger(cfg)
		}
	}
}

// WithLoggingFromConfig wires an existing logger into the router.
//
// Example:
//
//	logger := logging.MustNew(logging.WithJSONHandler())
//	r := router.New(
//	    logging.WithLoggingFromConfig(logger),
//	)
func WithLoggingFromConfig(cfg *Config) RouterOption {
	return func(router interface{}) {
		if setter, ok := router.(interface{ SetLogger(Logger) }); ok {
			setter.SetLogger(cfg)
		}
	}
}
