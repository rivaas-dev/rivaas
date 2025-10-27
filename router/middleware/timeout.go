package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/rivaas-dev/rivaas/router"
)

// TimeoutOption defines functional options for Timeout middleware configuration.
type TimeoutOption func(*timeoutConfig)

// timeoutConfig holds the configuration for the Timeout middleware.
type timeoutConfig struct {
	// timeout is the duration after which requests are cancelled
	timeout time.Duration

	// errorHandler is called when a timeout occurs
	errorHandler func(c *router.Context)

	// skipPaths are paths that should not have timeout applied
	skipPaths map[string]bool
}

// defaultTimeoutConfig returns the default configuration for Timeout middleware.
func defaultTimeoutConfig(timeout time.Duration) *timeoutConfig {
	return &timeoutConfig{
		timeout:      timeout,
		errorHandler: defaultTimeoutHandler,
		skipPaths:    make(map[string]bool),
	}
}

// defaultTimeoutHandler is the default timeout error handler.
func defaultTimeoutHandler(c *router.Context) {
	c.Status(http.StatusRequestTimeout)
	c.JSON(http.StatusRequestTimeout, map[string]any{
		"error": "Request timeout",
		"code":  "TIMEOUT",
	})
}

// WithTimeoutHandler sets a custom handler for timeout errors.
// This handler is called when a request exceeds the timeout duration.
//
// Example:
//
//	middleware.Timeout(30*time.Second,
//	    middleware.WithTimeoutHandler(func(c *router.Context) {
//	        c.JSON(408, map[string]string{
//	            "error": "Request took too long",
//	            "request_id": c.Response.Header().Get("X-Request-ID"),
//	        })
//	    }),
//	)
func WithTimeoutHandler(handler func(c *router.Context)) TimeoutOption {
	return func(cfg *timeoutConfig) {
		cfg.errorHandler = handler
	}
}

// WithTimeoutSkipPaths sets paths that should not have timeout applied.
// Useful for long-running endpoints like streaming or webhooks.
//
// Example:
//
//	middleware.Timeout(30*time.Second,
//	    middleware.WithTimeoutSkipPaths([]string{"/stream", "/webhook"}),
//	)
func WithTimeoutSkipPaths(paths []string) TimeoutOption {
	return func(cfg *timeoutConfig) {
		for _, path := range paths {
			cfg.skipPaths[path] = true
		}
	}
}

// Timeout returns a middleware that adds a timeout to requests.
// If a request takes longer than the specified duration, it will be cancelled
// and an error response will be sent.
//
// The middleware creates a new context with timeout and passes it to the handler.
// Handlers should respect context cancellation to properly handle timeouts.
//
// Basic usage:
//
//	r := router.New()
//	r.Use(middleware.Timeout(30 * time.Second))
//
// With custom error handler:
//
//	r.Use(middleware.Timeout(30*time.Second,
//	    middleware.WithTimeoutHandler(func(c *router.Context) {
//	        c.JSON(408, map[string]any{
//	            "error": "Operation timed out",
//	            "timeout": "30s",
//	        })
//	    }),
//	))
//
// Skip certain paths:
//
//	r.Use(middleware.Timeout(30*time.Second,
//	    middleware.WithTimeoutSkipPaths([]string{"/stream", "/events"}),
//	))
//
// Respecting timeouts in handlers:
//
//	r.GET("/slow", func(c *router.Context) {
//	    select {
//	    case <-time.After(2 * time.Second):
//	        c.JSON(200, map[string]string{"message": "Done"})
//	    case <-c.Request.Context().Done():
//	        // Request was cancelled or timed out
//	        return
//	    }
//	})
//
// Important notes:
//   - Handlers MUST check c.Request.Context().Done() for long operations
//   - Database queries should use context: db.QueryContext(c.Request.Context(), ...)
//   - HTTP calls should use context: req.WithContext(c.Request.Context())
//   - Timeouts don't interrupt running code, they cancel the context
//   - Goroutines spawned by handlers may continue after timeout until they complete
//     or check context cancellation - this is a limitation of Go's timeout mechanism
//
// Performance: This middleware adds minimal overhead (~200ns per request).
// Timeout checking is handled by Go's context package efficiently.
//
// Goroutine behavior:
//
//	The timeout middleware spawns a goroutine to execute the handler chain.
//	If a timeout occurs, the context is cancelled but the goroutine continues
//	until it naturally completes or checks c.Request.Context().Done().
//	This is expected behavior and handlers must be designed to respect context
//	cancellation to avoid goroutine leaks and unnecessary work.
func Timeout(timeout time.Duration, opts ...TimeoutOption) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultTimeoutConfig(timeout)
	for _, opt := range opts {
		opt(cfg)
	}

	return func(c *router.Context) {
		// Check if path should skip timeout
		if cfg.skipPaths[c.Request.URL.Path] {
			c.Next()
			return
		}

		// Create a context with timeout
		ctx, cancel := context.WithTimeout(c.Request.Context(), cfg.timeout)
		defer cancel()

		// Update request context
		c.Request = c.Request.WithContext(ctx)

		// Create a channel to signal completion
		done := make(chan struct{})

		// Run the handler in a goroutine
		go func() {
			c.Next()
			close(done)
		}()

		// Wait for either completion or timeout
		select {
		case <-done:
			// Request completed normally
		case <-ctx.Done():
			// Request timed out or was cancelled
			if ctx.Err() == context.DeadlineExceeded {
				cfg.errorHandler(c)
			}
		}
	}
}
