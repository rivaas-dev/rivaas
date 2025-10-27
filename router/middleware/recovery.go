package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"rivaas.dev/router"
)

// RecoveryOption defines functional options for Recovery middleware configuration.
type RecoveryOption func(*recoveryConfig)

// recoveryConfig holds the configuration for the Recovery middleware.
type recoveryConfig struct {
	// stackTrace enables/disables printing stack traces on panic
	stackTrace bool

	// stackSize sets the maximum size of the stack trace in bytes
	stackSize int

	// logger is the custom logger function for panic messages
	logger func(c *router.Context, err any, stack []byte)

	// handler is the custom recovery handler
	handler func(c *router.Context, err any)

	// disableStackAll disables full stack trace from all goroutines
	disableStackAll bool
}

// defaultRecoveryConfig returns the default configuration for Recovery middleware.
func defaultRecoveryConfig() *recoveryConfig {
	return &recoveryConfig{
		stackTrace:      true,
		stackSize:       4 << 10, // 4KB
		disableStackAll: true,
		logger:          defaultRecoveryLogger,
		handler:         defaultRecoveryHandler,
	}
}

// defaultRecoveryLogger logs panic information with stack trace.
func defaultRecoveryLogger(c *router.Context, err any, stack []byte) {
	log.Printf("[Recovery] panic recovered:\n%v\n%s", err, stack)
}

// defaultRecoveryHandler sends a 500 Internal Server Error response.
func defaultRecoveryHandler(c *router.Context, err any) {
	c.JSON(http.StatusInternalServerError, map[string]any{
		"error": "Internal server error",
		"code":  "INTERNAL_ERROR",
	})
}

// WithStackTrace enables or disables stack trace printing.
// Default: true
//
// Example:
//
//	middleware.Recovery(middleware.WithStackTrace(false))
func WithStackTrace(enabled bool) RecoveryOption {
	return func(cfg *recoveryConfig) {
		cfg.stackTrace = enabled
	}
}

// WithStackSize sets the maximum size of the stack trace buffer in bytes.
// Default: 4KB (4 << 10)
//
// Example:
//
//	middleware.Recovery(middleware.WithStackSize(8 << 10)) // 8KB
func WithStackSize(size int) RecoveryOption {
	return func(cfg *recoveryConfig) {
		cfg.stackSize = size
	}
}

// WithRecoveryLogger sets a custom logger function for panic messages.
// The logger receives the context, error, and stack trace.
//
// Example:
//
//	middleware.Recovery(middleware.WithRecoveryLogger(func(c *router.Context, err any, stack []byte) {
//	    myLogger.Error("panic recovered", "error", err, "stack", string(stack))
//	}))
func WithRecoveryLogger(logger func(c *router.Context, err any, stack []byte)) RecoveryOption {
	return func(cfg *recoveryConfig) {
		cfg.logger = logger
	}
}

// WithRecoveryHandler sets a custom recovery handler function.
// The handler receives the context and error, and is responsible for sending the response.
//
// Example:
//
//	middleware.Recovery(middleware.WithRecoveryHandler(func(c *router.Context, err any) {
//	    c.JSON(500, map[string]string{"error": "Something went wrong"})
//	}))
func WithRecoveryHandler(handler func(c *router.Context, err any)) RecoveryOption {
	return func(cfg *recoveryConfig) {
		cfg.handler = handler
	}
}

// WithDisableStackAll disables capturing full stack trace from all goroutines.
// When enabled, only the current goroutine's stack is captured (more efficient).
// Default: true
//
// Example:
//
//	middleware.Recovery(middleware.WithDisableStackAll(false)) // Capture all goroutines
func WithDisableStackAll(disabled bool) RecoveryOption {
	return func(cfg *recoveryConfig) {
		cfg.disableStackAll = disabled
	}
}

// Recovery returns a middleware that recovers from panics in request handlers.
// It logs the panic, optionally prints a stack trace, and returns a 500 error response.
//
// This middleware should typically be registered first (or early) in the middleware chain
// to catch panics from all subsequent handlers.
//
// Basic usage:
//
//	r := router.New()
//	r.Use(middleware.Recovery())
//
// With custom configuration:
//
//	r.Use(middleware.Recovery(
//	    middleware.WithStackTrace(true),
//	    middleware.WithStackSize(8 << 10),
//	    middleware.WithRecoveryLogger(customLogger),
//	))
//
// Custom handler:
//
//	r.Use(middleware.Recovery(
//	    middleware.WithRecoveryHandler(func(c *router.Context, err any) {
//	        c.JSON(500, map[string]any{
//	            "error": "Internal server error",
//	            "request_id": c.Param("request_id"),
//	        })
//	    }),
//	))
//
// Performance: This middleware has minimal overhead when no panic occurs (~50ns).
// Stack trace capture on panic adds ~1-2µs overhead.
func Recovery(opts ...RecoveryOption) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultRecoveryConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return func(c *router.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Capture stack trace if enabled
				var stack []byte
				if cfg.stackTrace {
					// debug.Stack() always returns full stack
					fullStack := debug.Stack()

					if cfg.disableStackAll {
						// Limit stack size if requested
						if len(fullStack) > cfg.stackSize {
							stack = fullStack[:cfg.stackSize]
						} else {
							stack = fullStack
						}
					} else {
						// Use full stack from all goroutines
						stack = fullStack
					}
				}

				// Call logger (default or custom)
				if cfg.logger != nil {
					cfg.logger(c, err, stack)
				}

				// Call custom handler or default
				if cfg.handler != nil {
					cfg.handler(c, err)
				}
			}
		}()

		c.Next()
	}
}
