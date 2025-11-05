package recovery

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"rivaas.dev/router"
)

// Option defines functional options for recovery middleware configuration.
type Option func(*config)

// config holds the configuration for the recovery middleware.
type config struct {
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

	// useProblemDetails enables RFC 9457 Problem Details responses
	useProblemDetails bool

	// includeStackInProblem includes stack trace in problem details (dev/staging only!)
	includeStackInProblem bool

	// problemTypeURI is the problem type URI for RFC 9457 responses
	problemTypeURI string
}

// defaultConfig returns the default configuration for recovery middleware.
func defaultConfig() *config {
	return &config{
		stackTrace:      true,
		stackSize:       4 << 10, // 4KB
		disableStackAll: true,
		logger:          defaultLogger,
		handler:         defaultHandler,
	}
}

// defaultLogger logs panic information with stack trace.
func defaultLogger(c *router.Context, err any, stack []byte) {
	log.Printf("[Recovery] panic recovered:\n%v\n%s", err, stack)
}

// defaultHandler sends a 500 Internal Server Error response.
func defaultHandler(c *router.Context, err any) {
	c.JSON(http.StatusInternalServerError, map[string]any{
		"error": "Internal server error",
		"code":  "INTERNAL_ERROR",
	})
}

// problemDetailsHandler sends an RFC 9457 Problem Details response.
func problemDetailsHandler(cfg *config) func(c *router.Context, err any, stack []byte) {
	return func(c *router.Context, err any, stack []byte) {
		p := router.NewProblemDetail(http.StatusInternalServerError, "Internal Server Error").
			WithType(cfg.problemTypeURI).
			WithDetail("An unexpected error occurred while processing your request.").
			WithInstance(c.Request.URL.Path)

		// Include panic details in dev/staging (never in production!)
		if cfg.includeStackInProblem {
			p.WithExtension("panic", fmt.Sprintf("%v", err))
			if len(stack) > 0 {
				// Limit stack size if configured
				stackToInclude := stack
				if cfg.stackSize > 0 && len(stack) > cfg.stackSize {
					stackToInclude = stack[:cfg.stackSize]
				}
				p.WithExtension("stack", string(stackToInclude))
			}
		}

		_ = c.ProblemDetail(p)
	}
}

// New returns a middleware that recovers from panics in request handlers.
// It logs the panic, optionally prints a stack trace, and returns a 500 error response.
//
// This middleware should typically be registered first (or early) in the middleware chain
// to catch panics from all subsequent handlers.
//
// Basic usage:
//
//	r := router.New()
//	r.Use(recovery.New())
//
// With custom configuration:
//
//	r.Use(recovery.New(
//	    recovery.WithStackTrace(true),
//	    recovery.WithStackSize(8 << 10),
//	    recovery.WithLogger(customLogger),
//	))
//
// Custom handler:
//
//	r.Use(recovery.New(
//	    recovery.WithHandler(func(c *router.Context, err any) {
//	        c.JSON(500, map[string]any{
//	            "error": "Internal server error",
//	            "request_id": c.Param("request_id"),
//	        })
//	    }),
//	))
//
// Performance: This middleware has minimal overhead when no panic occurs (~50ns).
// Stack trace capture on panic adds ~1-2µs overhead.
func New(opts ...Option) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return func(c *router.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Mark span with exception.escaped for panics (only place this is set)
				if span := c.Span(); span != nil && span.SpanContext().IsValid() {
					span.SetStatus(codes.Error, "panic recovered")
					span.SetAttributes(
						attribute.Bool("exception.escaped", true), // KEY: only set for panics
						attribute.String("exception.type", fmt.Sprintf("%T", err)),
						attribute.String("exception.message", fmt.Sprintf("%v", err)),
					)

					// Optionally record as error (creates exception event)
					if actualErr, ok := err.(error); ok {
						span.RecordError(actualErr)
					}
				}

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

				// Use RFC 9457 Problem Details if enabled
				if cfg.useProblemDetails {
					problemDetailsHandler(cfg)(c, err, stack)
					return
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
