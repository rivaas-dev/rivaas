package logging

import "errors"

// Error types for better error handling and testing.
var (
	// ErrNilLogger indicates a nil custom logger was provided.
	ErrNilLogger = errors.New("custom logger is nil")

	// ErrInvalidHandler indicates an unsupported handler type was specified.
	ErrInvalidHandler = errors.New("invalid handler type")

	// ErrLoggerShutdown indicates the logger has been shut down and cannot log.
	ErrLoggerShutdown = errors.New("logger is shut down")

	// ErrInvalidLevel indicates an invalid log level was provided.
	ErrInvalidLevel = errors.New("invalid log level")

	// ErrCannotChangeLevel indicates log level cannot be changed (custom logger).
	ErrCannotChangeLevel = errors.New("cannot change level on custom logger")
)
